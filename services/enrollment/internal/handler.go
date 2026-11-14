package enrollment

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type enrollmentService interface {
	Create(ctx context.Context, actor Actor, studentID, offeringID uuid.UUID) (*Enrollment, error)
	Get(ctx context.Context, actor Actor, id uuid.UUID) (*Enrollment, error)
	ListForStudent(ctx context.Context, actor Actor, studentID uuid.UUID, filter EnrollmentFilter) (*EnrollmentList, error)
	ListForOffering(ctx context.Context, actor Actor, offeringID uuid.UUID, filter EnrollmentFilter) (*EnrollmentList, error)
	Cancel(ctx context.Context, actor Actor, id uuid.UUID, expectedVersion int64) (*Enrollment, error)
	Active(ctx context.Context, studentID, offeringID uuid.UUID) (bool, error)
	ActiveLecturer(ctx context.Context, lecturerID, offeringID uuid.UUID) (bool, error)
}

type Handler struct {
	service enrollmentService
	logger  *slog.Logger
}

func NewHandler(service enrollmentService, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, logger: logger}
}

func (h *Handler) RegisterRoutes(router gin.IRouter, auth, internal gin.HandlerFunc) {
	api := router.Group("/api/v1")
	api.POST("/enrollments", auth, h.Create)
	api.GET("/enrollments/:id", auth, h.Get)
	api.POST("/enrollments/:id/cancel", auth, h.Cancel)
	api.GET("/students/:id/enrollments", auth, h.ListStudent)
	api.GET("/offerings/:id/enrollments", auth, h.ListOffering)

	assessment := []gin.HandlerFunc{internal, RequireInternalCaller("assessment-service")}
	router.GET("/internal/offerings/:id/enrollments/:student_id/active",
		append(assessment, h.ActiveEnrollment)...)
	router.GET("/internal/offerings/:id/lecturers/:lecturer_id/active",
		append(assessment, h.ActiveLecturer)...)
}

type createEnrollmentRequest struct {
	StudentUserID *uuid.UUID `json:"student_user_id"`
	OfferingID    uuid.UUID  `json:"offering_id"`
}

func (h *Handler) Create(c *gin.Context) {
	actor, ok := actorOrReject(c)
	if !ok {
		return
	}
	var request createEnrollmentRequest
	if !BindJSON(c, &request) {
		return
	}
	studentID := actor.UserID
	if request.StudentUserID != nil {
		studentID = *request.StudentUserID
	}
	if actor.HasRole(RoleAdmin) && request.StudentUserID == nil {
		RespondValidationError(c, "student_user_id", "is required for an administrator")
		return
	}
	item, err := h.service.Create(c.Request.Context(), actor, studentID, request.OfferingID)
	if err != nil {
		RespondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"enrollment": item})
}

func (h *Handler) Get(c *gin.Context) {
	actor, ok := actorOrReject(c)
	if !ok {
		return
	}
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	item, err := h.service.Get(c.Request.Context(), actor, id)
	if err != nil {
		RespondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"enrollment": item})
}

func (h *Handler) ListStudent(c *gin.Context) {
	actor, ok := actorOrReject(c)
	if !ok {
		return
	}
	studentID, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	filter, ok := queryFilter(c)
	if !ok {
		return
	}
	result, err := h.service.ListForStudent(c.Request.Context(), actor, studentID, filter)
	if err != nil {
		RespondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) ListOffering(c *gin.Context) {
	actor, ok := actorOrReject(c)
	if !ok {
		return
	}
	offeringID, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	filter, ok := queryFilter(c)
	if !ok {
		return
	}
	result, err := h.service.ListForOffering(c.Request.Context(), actor, offeringID, filter)
	if err != nil {
		RespondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

type cancelEnrollmentRequest struct {
	Version int64 `json:"version"`
}

func (h *Handler) Cancel(c *gin.Context) {
	actor, ok := actorOrReject(c)
	if !ok {
		return
	}
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	var request cancelEnrollmentRequest
	if !BindJSON(c, &request) {
		return
	}
	item, err := h.service.Cancel(c.Request.Context(), actor, id, request.Version)
	if err != nil {
		RespondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"enrollment": item})
}

func (h *Handler) ActiveEnrollment(c *gin.Context) {
	offeringID, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	studentID, ok := pathUUID(c, "student_id")
	if !ok {
		return
	}
	active, err := h.service.Active(c.Request.Context(), studentID, offeringID)
	if err != nil {
		RespondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"student_user_id": studentID, "offering_id": offeringID, "active": active,
	})
}

func (h *Handler) ActiveLecturer(c *gin.Context) {
	offeringID, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	lecturerID, ok := pathUUID(c, "lecturer_id")
	if !ok {
		return
	}
	active, err := h.service.ActiveLecturer(c.Request.Context(), lecturerID, offeringID)
	if err != nil {
		RespondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"lecturer_user_id": lecturerID, "offering_id": offeringID, "active": active,
	})
}

func actorOrReject(c *gin.Context) (Actor, bool) {
	claims, ok := ClaimsFromContext(c)
	if !ok {
		AbortWithError(c, http.StatusUnauthorized, CodeUnauthorized, "Authentication is required.")
		return Actor{}, false
	}
	userID, err := claims.UserID()
	if err != nil || userID == uuid.Nil {
		AbortWithError(c, http.StatusUnauthorized, CodeUnauthorized, "Authentication is required.")
		return Actor{}, false
	}
	return Actor{UserID: userID, Roles: claims.Roles, Permissions: claims.Permissions}, true
}

func pathUUID(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil || id == uuid.Nil {
		RespondValidationError(c, name, "must be a valid UUID")
		return uuid.Nil, false
	}
	return id, true
}

func queryFilter(c *gin.Context) (EnrollmentFilter, bool) {
	filter := EnrollmentFilter{Status: EnrollmentStatus(c.Query("status"))}
	if raw := c.Query("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			RespondValidationError(c, "limit", "must be an integer")
			return EnrollmentFilter{}, false
		}
		filter.Limit = value
	}
	if raw := c.Query("offset"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			RespondValidationError(c, "offset", "must be an integer")
			return EnrollmentFilter{}, false
		}
		filter.Offset = value
	}
	if err := validateEnrollmentFilter(&filter); err != nil {
		RespondServiceError(c, err)
		return EnrollmentFilter{}, false
	}
	return filter, true
}
