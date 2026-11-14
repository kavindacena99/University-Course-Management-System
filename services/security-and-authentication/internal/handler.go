package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"security-and-authentication/internal/config"
)

type authService interface {
	Login(ctx context.Context, input LoginInput) (*LoginResult, error)
	Refresh(ctx context.Context, rawRefreshToken string) (*RefreshResult, error)
	Logout(ctx context.Context, rawRefreshToken string) (bool, error)
	LogoutAll(ctx context.Context, claims *AccessClaims) error
	Sessions(ctx context.Context, claims *AccessClaims) ([]Session, error)
	RevokeSession(ctx context.Context, claims *AccessClaims, sessionID uuid.UUID) error
	Me(ctx context.Context, claims *AccessClaims) (*UserSummary, error)
	Activate(ctx context.Context, token, password string) (*AuthUser, error)
	ChangePassword(ctx context.Context, claims *AccessClaims, input ChangePasswordInput) error
	ForgotPassword(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, token, newPassword string) (*AuthUser, error)
	ProvisionUser(ctx context.Context, input ProvisionUserInput) (*AuthUser, error)
	ProcessUserEvent(ctx context.Context, event UserEvent) (InboxResult, error)
	LecturerEligibility(ctx context.Context, userID uuid.UUID) (bool, error)
	StudentEligibility(ctx context.Context, userID uuid.UUID) (bool, error)
	ListRoles(ctx context.Context) ([]Role, error)
	ListPermissions(ctx context.Context) ([]Permission, error)
	GetRolePermissions(ctx context.Context, roleCode string) ([]string, error)
	SetRolePermissions(ctx context.Context, claims *AccessClaims, roleCode string, permissions []string) ([]Permission, error)
	SetUserRoles(ctx context.Context, claims *AccessClaims, userID uuid.UUID, roles []string) (*AuthUser, error)
	UnlockUser(ctx context.Context, userID uuid.UUID) (*AuthUser, error)
	RevokeUserSessions(ctx context.Context, userID uuid.UUID) error
	ListAuditEvents(ctx context.Context, limit, offset int) ([]AuditEvent, int64, error)
}

type Handler struct {
	service authService
	cfg     *config.Config
	logger  *slog.Logger
}

func NewHandler(
	service authService,
	cfg *config.Config,
	logger *slog.Logger,
) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, cfg: cfg, logger: logger}
}

type RouteDependencies struct {
	Auth            gin.HandlerFunc
	Internal        gin.HandlerFunc
	LoginRateLimit  gin.HandlerFunc
	ForgotRateLimit gin.HandlerFunc
}

func (h *Handler) RegisterRoutes(router gin.IRouter, deps RouteDependencies) {
	api := router.Group("/api/v1")
	auth := api.Group("/auth")

	auth.POST("/login",
		withMiddleware([]gin.HandlerFunc{deps.LoginRateLimit}, h.Login)...)
	auth.POST("/refresh", h.Refresh)
	auth.POST("/logout", h.Logout)

	auth.POST("/logout-all",
		withMiddleware([]gin.HandlerFunc{deps.Auth}, h.LogoutAll)...)
	auth.GET("/sessions",
		withMiddleware([]gin.HandlerFunc{deps.Auth}, h.Sessions)...)
	auth.DELETE("/sessions/:id",
		withMiddleware([]gin.HandlerFunc{deps.Auth}, h.RevokeSession)...)
	auth.GET("/me",
		withMiddleware([]gin.HandlerFunc{deps.Auth}, h.Me)...)
	auth.POST("/activate", h.Activate)
	auth.POST("/password/change",
		withMiddleware([]gin.HandlerFunc{deps.Auth}, h.ChangePassword)...)
	auth.POST("/password/forgot",
		withMiddleware([]gin.HandlerFunc{deps.ForgotRateLimit}, h.ForgotPassword)...)
	auth.POST("/password/reset", h.ResetPassword)

	withPermission := func(permission string) []gin.HandlerFunc {
		return []gin.HandlerFunc{deps.Auth, RequirePermissions(permission)}
	}

	api.GET("/admin/auth/roles",
		withMiddleware(withPermission(PermRolesManage), h.ListRoles)...)
	api.GET("/admin/auth/permissions",
		withMiddleware(withPermission(PermRolesManage), h.ListPermissions)...)
	api.GET("/admin/auth/roles/:code/permissions",
		withMiddleware(withPermission(PermRolesManage), h.GetRolePermissions)...)
	api.PUT("/admin/auth/roles/:code/permissions",
		withMiddleware(withPermission(PermRolesManage), h.SetRolePermissions)...)
	api.PUT("/admin/auth/users/:id/roles",
		withMiddleware(withPermission(PermRolesManage), h.SetUserRoles)...)
	api.POST("/admin/auth/users/:id/unlock",
		withMiddleware(withPermission(PermAccountUnlock), h.UnlockUser)...)
	api.DELETE("/admin/auth/users/:id/sessions",
		withMiddleware(withPermission(PermSessionsRevoke), h.RevokeUserSessions)...)
	api.GET("/admin/auth/audit",
		withMiddleware(withPermission(PermAuditRead), h.ListAuditEvents)...)

	router.POST("/internal/users",
		withMiddleware([]gin.HandlerFunc{deps.Internal, RequireInternalCaller("user-service")}, h.InternalProvisionUser)...)
	router.POST("/internal/events/users",
		withMiddleware([]gin.HandlerFunc{deps.Internal, RequireInternalCaller("user-service")}, h.InternalUserEvent)...)
	router.GET("/internal/users/:id/lecturer-eligibility",
		withMiddleware([]gin.HandlerFunc{deps.Internal, RequireInternalCaller("course-service", "enrollment-service")}, h.InternalLecturerEligibility)...)
	router.GET("/internal/users/:id/student-eligibility",
		withMiddleware([]gin.HandlerFunc{deps.Internal, RequireInternalCaller("enrollment-service")}, h.InternalStudentEligibility)...)
}

func (h *Handler) InternalStudentEligibility(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil || userID == uuid.Nil {
		RespondValidationError(c, "id", "must be a valid UUID")
		return
	}
	eligible, err := h.service.StudentEligibility(c.Request.Context(), userID)
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"user_id": userID, "eligible": eligible})
}

func (h *Handler) InternalLecturerEligibility(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil || userID == uuid.Nil {
		RespondValidationError(c, "id", "must be a valid UUID")
		return
	}
	eligible, err := h.service.LecturerEligibility(c.Request.Context(), userID)
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"user_id": userID, "eligible": eligible})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if !BindJSON(c, &req) {
		return
	}

	result, err := h.service.Login(c.Request.Context(), LoginInput{
		Email:    req.Email,
		Password: req.Password,
		Client: ClientInfo{
			UserAgent: c.Request.UserAgent(),
			IPAddress: c.ClientIP(),
		},
	})
	if err != nil {
		h.respondError(c, err)
		return
	}

	h.setAuthCookies(
		c,
		result.AccessToken,
		result.RefreshToken,
		result.CSRFToken,
		result.AccessExpiry,
		result.RefreshExpiry,
	)
	c.JSON(http.StatusOK, h.authResponse(
		result.User,
		result.AccessToken,
		result.RefreshToken,
		result.CSRFToken,
		result.AccessExpiry,
		result.RefreshExpiry,
	))
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *Handler) Refresh(c *gin.Context) {
	rawToken, ok := h.readRefreshToken(c)
	if !ok {
		return
	}
	if rawToken == "" {
		AbortWithError(c, http.StatusUnauthorized, CodeRefreshInvalid,
			"The refresh token is invalid.")
		return
	}

	result, err := h.service.Refresh(c.Request.Context(), rawToken)
	if err != nil {
		h.respondError(c, err)
		return
	}

	h.setAuthCookies(
		c,
		result.AccessToken,
		result.RefreshToken,
		result.CSRFToken,
		result.AccessExpiry,
		result.RefreshExpiry,
	)
	c.JSON(http.StatusOK, h.authResponse(
		result.User,
		result.AccessToken,
		result.RefreshToken,
		result.CSRFToken,
		result.AccessExpiry,
		result.RefreshExpiry,
	))
}

func (h *Handler) Logout(c *gin.Context) {
	rawToken, ok := h.readRefreshToken(c)
	if !ok {
		return
	}

	if _, err := h.service.Logout(c.Request.Context(), rawToken); err != nil {
		h.respondError(c, err)
		return
	}

	h.clearAuthCookies(c)
	c.Status(http.StatusNoContent)
}

func (h *Handler) LogoutAll(c *gin.Context) {
	claims, ok := ClaimsFromContext(c)
	if !ok {
		AbortWithError(c, http.StatusUnauthorized, CodeUnauthorized,
			"Authentication is required.")
		return
	}

	if err := h.service.LogoutAll(c.Request.Context(), claims); err != nil {
		h.respondError(c, err)
		return
	}

	h.clearAuthCookies(c)
	c.Status(http.StatusNoContent)
}

func (h *Handler) Sessions(c *gin.Context) {
	claims, ok := ClaimsFromContext(c)
	if !ok {
		AbortWithError(c, http.StatusUnauthorized, CodeUnauthorized,
			"Authentication is required.")
		return
	}

	sessions, err := h.service.Sessions(c.Request.Context(), claims)
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"sessions": sessions})
}

func (h *Handler) RevokeSession(c *gin.Context) {
	claims, ok := ClaimsFromContext(c)
	if !ok {
		AbortWithError(c, http.StatusUnauthorized, CodeUnauthorized,
			"Authentication is required.")
		return
	}

	sessionID, ok := parseUUIDParam(c, "id", "session id")
	if !ok {
		return
	}

	if err := h.service.RevokeSession(c.Request.Context(), claims, sessionID); err != nil {
		h.respondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) Me(c *gin.Context) {
	claims, ok := ClaimsFromContext(c)
	if !ok {
		AbortWithError(c, http.StatusUnauthorized, CodeUnauthorized,
			"Authentication is required.")
		return
	}

	user, err := h.service.Me(c.Request.Context(), claims)
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"user": user})
}

type activateRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

func (h *Handler) Activate(c *gin.Context) {
	var req activateRequest
	if !BindJSON(c, &req) {
		return
	}

	user, err := h.service.Activate(c.Request.Context(), req.Token, req.Password)
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"user": user.Summary()})
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (h *Handler) ChangePassword(c *gin.Context) {
	claims, ok := ClaimsFromContext(c)
	if !ok {
		AbortWithError(c, http.StatusUnauthorized, CodeUnauthorized,
			"Authentication is required.")
		return
	}

	var req changePasswordRequest
	if !BindJSON(c, &req) {
		return
	}

	if err := h.service.ChangePassword(c.Request.Context(), claims, ChangePasswordInput{
		CurrentPassword: req.CurrentPassword,
		NewPassword:     req.NewPassword,
	}); err != nil {
		h.respondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

type forgotPasswordRequest struct {
	Email string `json:"email"`
}

func (h *Handler) ForgotPassword(c *gin.Context) {
	var req forgotPasswordRequest
	if !BindJSON(c, &req) {
		return
	}

	if err := h.service.ForgotPassword(c.Request.Context(), req.Email); err != nil {
		h.logger.Error("forgot_password_failed",
			"error", err,
			"request_id", RequestID(c),
		)
	}

	c.JSON(http.StatusAccepted, gin.H{"status": "accepted"})
}

type resetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

func (h *Handler) ResetPassword(c *gin.Context) {
	var req resetPasswordRequest
	if !BindJSON(c, &req) {
		return
	}

	user, err := h.service.ResetPassword(
		c.Request.Context(), req.Token, req.NewPassword,
	)
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"user": user.Summary()})
}

type provisionUserRequest struct {
	UserID    uuid.UUID `json:"user_id"`
	Email     string    `json:"email"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Version   int64     `json:"version"`
	Password  string    `json:"password"`
	Roles     []string  `json:"roles"`
}

func (h *Handler) InternalProvisionUser(c *gin.Context) {
	var req provisionUserRequest
	if !BindJSON(c, &req) {
		return
	}

	user, err := h.service.ProvisionUser(c.Request.Context(), ProvisionUserInput{
		UserID:    req.UserID,
		Email:     req.Email,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Version:   req.Version,
		Password:  req.Password,
		Roles:     req.Roles,
	})
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"user": user.Summary()})
}

func (h *Handler) InternalUserEvent(c *gin.Context) {
	var event UserEvent
	if !BindJSON(c, &event) {
		return
	}

	result, err := h.service.ProcessUserEvent(c.Request.Context(), event)
	if err != nil {
		h.respondError(c, err)
		return
	}

	status := "processed"
	if result.Duplicate {
		status = "duplicate"
	} else if result.Stale {
		status = "stale"
	}

	c.JSON(http.StatusOK, gin.H{"status": status})
}

func (h *Handler) ListRoles(c *gin.Context) {
	roles, err := h.service.ListRoles(c.Request.Context())
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"roles": roles})
}

func (h *Handler) ListPermissions(c *gin.Context) {
	permissions, err := h.service.ListPermissions(c.Request.Context())
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"permissions": permissions})
}

func (h *Handler) GetRolePermissions(c *gin.Context) {
	permissions, err := h.service.GetRolePermissions(
		c.Request.Context(), c.Param("code"),
	)
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"role":        c.Param("code"),
		"permissions": permissions,
	})
}

type setRolePermissionsRequest struct {
	Permissions []string `json:"permissions"`
}

func (h *Handler) SetRolePermissions(c *gin.Context) {
	claims, ok := ClaimsFromContext(c)
	if !ok {
		AbortWithError(c, http.StatusUnauthorized, CodeUnauthorized,
			"Authentication is required.")
		return
	}

	var req setRolePermissionsRequest
	if !BindJSON(c, &req) {
		return
	}

	permissions, err := h.service.SetRolePermissions(
		c.Request.Context(), claims, c.Param("code"), req.Permissions,
	)
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"role":        c.Param("code"),
		"permissions": permissions,
	})
}

type setUserRolesRequest struct {
	Roles []string `json:"roles"`
}

func (h *Handler) SetUserRoles(c *gin.Context) {
	claims, ok := ClaimsFromContext(c)
	if !ok {
		AbortWithError(c, http.StatusUnauthorized, CodeUnauthorized,
			"Authentication is required.")
		return
	}

	userID, ok := parseUUIDParam(c, "id", "user id")
	if !ok {
		return
	}

	var req setUserRolesRequest
	if !BindJSON(c, &req) {
		return
	}

	user, err := h.service.SetUserRoles(
		c.Request.Context(), claims, userID, req.Roles,
	)
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"user": user.Summary()})
}

func (h *Handler) ListAuditEvents(c *gin.Context) {
	limit, offset, ok := parseAuditPagination(c)
	if !ok {
		return
	}

	events, total, err := h.service.ListAuditEvents(
		c.Request.Context(), limit, offset,
	)
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"events": events,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func parseAuditPagination(c *gin.Context) (int, int, bool) {
	limit := 50
	offset := 0

	if value := c.Query("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 200 {
			AbortWithError(c, http.StatusBadRequest, CodeValidationFailed,
				"The limit parameter must be between 1 and 200.")
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

func (h *Handler) UnlockUser(c *gin.Context) {
	userID, ok := parseUUIDParam(c, "id", "user id")
	if !ok {
		return
	}

	user, err := h.service.UnlockUser(c.Request.Context(), userID)
	if err != nil {
		h.respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"user": user.Summary()})
}

func (h *Handler) RevokeUserSessions(c *gin.Context) {
	userID, ok := parseUUIDParam(c, "id", "user id")
	if !ok {
		return
	}

	if err := h.service.RevokeUserSessions(c.Request.Context(), userID); err != nil {
		h.respondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

type authResponseBody struct {
	User             UserSummary `json:"user"`
	AccessToken      string      `json:"access_token,omitempty"`
	RefreshToken     string      `json:"refresh_token,omitempty"`
	TokenType        string      `json:"token_type"`
	ExpiresIn        int         `json:"expires_in"`
	RefreshExpiresIn int         `json:"refresh_expires_in"`
	CSRFToken        string      `json:"csrf_token,omitempty"`
}

func (h *Handler) authResponse(
	user *AuthUser,
	accessToken string,
	refreshToken string,
	csrfToken string,
	accessExpiry time.Time,
	refreshExpiry time.Time,
) authResponseBody {
	body := authResponseBody{
		User:             user.Summary(),
		TokenType:        "Bearer",
		ExpiresIn:        secondsUntil(accessExpiry),
		RefreshExpiresIn: secondsUntil(refreshExpiry),
	}
	if h.cfg.TokenDelivery == "body" || h.cfg.TokenDelivery == "both" {
		body.AccessToken = accessToken
		body.RefreshToken = refreshToken
	}
	if h.cfg.CookieEnabled {
		body.CSRFToken = csrfToken
	}
	return body
}

func (h *Handler) readRefreshToken(c *gin.Context) (string, bool) {
	var req refreshRequest
	if c.Request.ContentLength != 0 {
		if !BindOptionalJSON(c, &req) {
			return "", false
		}
	}

	token := strings.TrimSpace(req.RefreshToken)
	if token == "" && h.cfg.CookieEnabled {
		if cookieValue, err := c.Cookie(h.cfg.RefreshTokenCookieName); err == nil {
			token = strings.TrimSpace(cookieValue)
		}
	}
	return token, true
}

func (h *Handler) setAuthCookies(
	c *gin.Context,
	accessToken string,
	refreshToken string,
	csrfToken string,
	accessExpiry time.Time,
	refreshExpiry time.Time,
) {
	if !h.cfg.CookieEnabled {
		return
	}

	c.SetSameSite(h.sameSite())
	c.SetCookie(
		h.cfg.AccessTokenCookieName,
		accessToken,
		secondsUntil(accessExpiry),
		"/",
		h.cfg.CookieDomain,
		h.cfg.CookieSecure,
		true,
	)
	c.SetCookie(
		h.cfg.RefreshTokenCookieName,
		refreshToken,
		secondsUntil(refreshExpiry),
		"/",
		h.cfg.CookieDomain,
		h.cfg.CookieSecure,
		true,
	)
	c.SetCookie(
		h.cfg.CSRFCookieName,
		csrfToken,
		secondsUntil(refreshExpiry),
		"/",
		h.cfg.CookieDomain,
		h.cfg.CookieSecure,
		false,
	)
}

func (h *Handler) clearAuthCookies(c *gin.Context) {
	if !h.cfg.CookieEnabled {
		return
	}

	c.SetSameSite(h.sameSite())
	for _, name := range []string{
		h.cfg.AccessTokenCookieName,
		h.cfg.RefreshTokenCookieName,
		h.cfg.CSRFCookieName,
	} {
		c.SetCookie(name, "", -1, "/", h.cfg.CookieDomain,
			h.cfg.CookieSecure, name != h.cfg.CSRFCookieName)
	}
}

func (h *Handler) sameSite() http.SameSite {
	switch strings.ToLower(h.cfg.CookieSameSite) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
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
		ErrInvalidCredentials,
		ErrAccountLocked,
		ErrAccountUnavailable,
		ErrAccountNotActivated,
		ErrInvalidToken,
		ErrTokenExpired,
		ErrRefreshInvalid,
		ErrRefreshReuse,
		ErrSessionExpired,
		ErrActionTokenInvalid,
		ErrActionTokenExpired,
		ErrActionTokenUsed,
		ErrUnauthorized,
		ErrForbidden,
		ErrLastAdmin,
		ErrPasswordReuse,
		ErrInvalidRole,
		ErrInvalidPermission,
		ErrDisabledFeature,
		ErrUserIneligible,
		ErrEligibilityUnavailable,
	} {
		if errors.Is(err, sentinel) {
			return true
		}
	}

	return false
}

func parseUUIDParam(
	c *gin.Context,
	param string,
	label string,
) (uuid.UUID, bool) {
	value, err := uuid.Parse(c.Param(param))
	if err != nil {
		AbortWithError(c, http.StatusBadRequest, CodeValidationFailed,
			"The "+label+" must be a valid UUID.")
		return uuid.Nil, false
	}
	return value, true
}

func secondsUntil(expiry time.Time) int {
	remaining := time.Until(expiry)
	if remaining <= 0 {
		return 0
	}
	seconds := int(remaining / time.Second)
	if seconds < 1 {
		return 1
	}
	return seconds
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
