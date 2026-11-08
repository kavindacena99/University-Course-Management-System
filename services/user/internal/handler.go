package user

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type userService interface {
	Register(ctx context.Context, input RegisterInput) (*User, error)
	GetMe(ctx context.Context, claims *AccessClaims) (*User, error)
	UpdateMe(ctx context.Context, claims *AccessClaims, input UpdateProfileInput) (*User, error)
	GetUser(ctx context.Context, claims *AccessClaims, userID uuid.UUID) (*User, error)
	ListUsers(ctx context.Context, filter ListUsersFilter) (*UserList, error)
	ChangeStatus(ctx context.Context, claims *AccessClaims, request ChangeStatusRequest) (*User, error)
	GetHistory(ctx context.Context, userID uuid.UUID, limit, offset int) ([]StatusHistory, int64, error)
	InternalGetUser(ctx context.Context, userID uuid.UUID) (*User, error)
	Eligibility(ctx context.Context, userID uuid.UUID) (*Eligibility, error)
	BootstrapAdmin(ctx context.Context, input BootstrapInput) (*User, error)
}

type Handler struct {
	service userService
	logger  *slog.Logger
}

func NewHandler(service userService, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, logger: logger}
}

type RouteDependencies struct {
	Auth                  gin.HandlerFunc
	Admin                 gin.HandlerFunc
	Internal              gin.HandlerFunc
	Bootstrap             gin.HandlerFunc
	RegistrationRateLimit gin.HandlerFunc
}

func (h *Handler) RegisterRoutes(router gin.IRouter, deps RouteDependencies) {
	api := router.Group("/api/v1")

	api.POST("/users",
		withMiddleware([]gin.HandlerFunc{deps.RegistrationRateLimit}, h.Create)...)

	api.GET("/users/me",
		withMiddleware([]gin.HandlerFunc{deps.Auth}, h.GetMe)...)
	api.PATCH("/users/me",
		withMiddleware([]gin.HandlerFunc{deps.Auth}, h.UpdateMe)...)
	api.GET("/users/:id",
		withMiddleware([]gin.HandlerFunc{deps.Auth}, h.GetUser)...)

	admin := []gin.HandlerFunc{deps.Auth, deps.Admin}

	api.GET("/admin/users",
		withMiddleware(admin, h.ListUsers)...)
	api.GET("/admin/users/:id",
		withMiddleware(admin, h.GetUser)...)
	api.GET("/admin/users/:id/history",
		withMiddleware(admin, h.GetHistory)...)
	api.POST("/admin/users/:id/approve",
		withMiddleware(admin, h.ChangeStatus(ActionApprove))...)
	api.POST("/admin/users/:id/reject",
		withMiddleware(admin, h.ChangeStatus(ActionReject))...)
	api.POST("/admin/users/:id/suspend",
		withMiddleware(admin, h.ChangeStatus(ActionSuspend))...)
	api.POST("/admin/users/:id/reinstate",
		withMiddleware(admin, h.ChangeStatus(ActionReinstate))...)

	internal := []gin.HandlerFunc{deps.Internal}

	router.GET("/internal/users/:id",
		withMiddleware(internal, h.InternalGetUser)...)
	router.GET("/internal/users/:id/eligibility",
		withMiddleware(internal, h.InternalEligibility)...)
	router.POST("/internal/bootstrap",
		withMiddleware([]gin.HandlerFunc{deps.Bootstrap}, h.Bootstrap)...)
}

type createUserRequest struct {
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

func (h *Handler) Create(c *gin.Context) {
	var req createUserRequest
	if !BindJSON(c, &req) {
		return
	}

	u, err := h.service.Register(c.Request.Context(), RegisterInput{
		Email:     req.Email,
		FirstName: req.FirstName,
		LastName:  req.LastName,
	})
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"user": u})
}

func (h *Handler) GetMe(c *gin.Context) {
	claims, ok := ClaimsFromContext(c)
	if !ok {
		AbortWithError(c, http.StatusUnauthorized, CodeUnauthorized,
			"Authentication is required.")
		return
	}

	u, err := h.service.GetMe(c.Request.Context(), claims)
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"user": u})
}

type updateProfileRequest struct {
	Version   int64  `json:"version"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

func (h *Handler) UpdateMe(c *gin.Context) {
	claims, ok := ClaimsFromContext(c)
	if !ok {
		AbortWithError(c, http.StatusUnauthorized, CodeUnauthorized,
			"Authentication is required.")
		return
	}

	var req updateProfileRequest
	if !BindJSON(c, &req) {
		return
	}

	u, err := h.service.UpdateMe(c.Request.Context(), claims, UpdateProfileInput{
		Version:   req.Version,
		FirstName: req.FirstName,
		LastName:  req.LastName,
	})
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"user": u})
}

func (h *Handler) GetUser(c *gin.Context) {
	claims, ok := ClaimsFromContext(c)
	if !ok {
		AbortWithError(c, http.StatusUnauthorized, CodeUnauthorized,
			"Authentication is required.")
		return
	}

	userID, ok := parseUserID(c)
	if !ok {
		return
	}

	u, err := h.service.GetUser(c.Request.Context(), claims, userID)
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"user": u})
}

func (h *Handler) ListUsers(c *gin.Context) {
	limit, offset, ok := parsePagination(c, 20, 100)
	if !ok {
		return
	}

	filter := ListUsersFilter{
		Status: RegistrationStatus(c.Query("status")),
		Search: c.Query("search"),
		Limit:  limit,
		Offset: offset,
	}

	result, err := h.service.ListUsers(c.Request.Context(), filter)
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

type changeStatusRequest struct {
	Version int64  `json:"version"`
	Reason  string `json:"reason"`
}

func (h *Handler) ChangeStatus(action StatusAction) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := ClaimsFromContext(c)
		if !ok {
			AbortWithError(c, http.StatusUnauthorized, CodeUnauthorized,
				"Authentication is required.")
			return
		}

		userID, ok := parseUserID(c)
		if !ok {
			return
		}

		var req changeStatusRequest
		if !BindJSON(c, &req) {
			return
		}

		u, err := h.service.ChangeStatus(c.Request.Context(), claims, ChangeStatusRequest{
			UserID:          userID,
			Action:          action,
			Reason:          req.Reason,
			ExpectedVersion: req.Version,
		})
		if err != nil {
			h.respondError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{"user": u})
	}
}

func (h *Handler) GetHistory(c *gin.Context) {
	userID, ok := parseUserID(c)
	if !ok {
		return
	}

	limit, offset, ok := parsePagination(c, 50, 200)
	if !ok {
		return
	}

	history, total, err := h.service.GetHistory(c.Request.Context(), userID, limit, offset)
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"history": history,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

func (h *Handler) InternalGetUser(c *gin.Context) {
	userID, ok := parseUserID(c)
	if !ok {
		return
	}

	u, err := h.service.InternalGetUser(c.Request.Context(), userID)
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"user": u})
}

func (h *Handler) InternalEligibility(c *gin.Context) {
	userID, ok := parseUserID(c)
	if !ok {
		return
	}

	eligibility, err := h.service.Eligibility(c.Request.Context(), userID)
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, eligibility)
}

type bootstrapRequest struct {
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Password  string `json:"password"`
}

func (h *Handler) Bootstrap(c *gin.Context) {
	var req bootstrapRequest
	if !BindJSON(c, &req) {
		return
	}

	u, err := h.service.BootstrapAdmin(c.Request.Context(), BootstrapInput{
		Email:     req.Email,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Password:  req.Password,
	})
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"user": u})
}

func (h *Handler) respondError(c *gin.Context, err error) {
	if !isExpectedError(err) {
		h.logger.Error("request_failed",
			"error", err,
			"path", c.Request.URL.Path,
			"request_id", RequestID(c),
		)
	}
	RespondServiceError(c, err)
}

func isExpectedError(err error) bool {
	var validation *ValidationError
	if errors.As(err, &validation) {
		return true
	}

	for _, sentinel := range []error{
		ErrNotFound,
		ErrEmailTaken,
		ErrInvalidTransition,
		ErrVersionConflict,
		ErrUnauthorized,
		ErrForbidden,
		ErrInvalidToken,
		ErrTokenExpired,
		ErrBootstrapDisabled,
		ErrBootstrapForbidden,
		ErrPasswordTooWeak,
	} {
		if errors.Is(err, sentinel) {
			return true
		}
	}

	return false
}

func parseUserID(c *gin.Context) (uuid.UUID, bool) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		AbortWithError(c, http.StatusBadRequest, CodeValidationFailed,
			"The user id must be a valid UUID.")
		return uuid.Nil, false
	}
	return userID, true
}

func parsePagination(c *gin.Context, defaultLimit, maxLimit int) (int, int, bool) {
	limit := defaultLimit
	offset := 0

	if value := c.Query("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > maxLimit {
			AbortWithError(c, http.StatusBadRequest, CodeValidationFailed,
				"The limit parameter must be a positive integer within the allowed range.")
			return 0, 0, false
		}
		limit = parsed
	}

	if value := c.Query("offset"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 {
			AbortWithError(c, http.StatusBadRequest, CodeValidationFailed,
				"The offset parameter must be a non-negative integer.")
			return 0, 0, false
		}
		offset = parsed
	}

	return limit, offset, true
}

func withMiddleware(
	middlewares []gin.HandlerFunc,
	handler gin.HandlerFunc,
) []gin.HandlerFunc {
	chain := make([]gin.HandlerFunc, 0, len(middlewares)+1)
	for _, middleware := range middlewares {
		if middleware != nil {
			chain = append(chain, middleware)
		}
	}
	return append(chain, handler)
}
