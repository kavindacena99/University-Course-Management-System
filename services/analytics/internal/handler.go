package analytics

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(
	router gin.IRouter, auth, internal gin.HandlerFunc,
) {
	router.POST("/internal/events",
		internal, RequireInternalCaller("assessment-service"), h.IngestEvent)

	api := router.Group("/api/v1/analytics")
	api.GET("/students/me", auth, h.StudentReport)
	api.GET("/lecturer/offerings/:id", auth, h.LecturerOfferingReport)
	api.GET("/admin/summary", auth, h.AdminSummary)
	api.GET("/admin/grade-bands", auth, h.ListGradeBands)
	api.PUT("/admin/grade-bands", auth, h.ReplaceGradeBands)
	api.GET("/admin/programmes", auth, h.ListProgrammes)
	api.POST("/admin/programmes", auth, h.CreateProgramme)
	api.PUT("/admin/programmes/:id", auth, h.UpdateProgramme)
	api.PUT("/admin/students/:id/programme", auth, h.AssignProgramme)
	api.GET("/admin/students/:id", auth, h.AdminStudentReport)
	api.POST("/admin/rebuild", auth, h.Rebuild)
}

func (h *Handler) IngestEvent(c *gin.Context) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20+1))
	if err != nil || len(body) > 1<<20 {
		writeError(c, http.StatusRequestEntityTooLarge, "payload_too_large", "The event is too large.")
		return
	}
	processed, err := h.service.IngestEvent(c.Request.Context(), body)
	if err != nil {
		if errors.Is(err, ErrInvalidEvent) {
			writeError(c, http.StatusBadRequest, "invalid_event", err.Error())
			return
		}
		writeError(c, http.StatusInternalServerError, "internal_error", "Unable to process the event.")
		return
	}
	if processed {
		c.JSON(http.StatusOK, gin.H{"result": "processed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": "duplicate"})
}

func (h *Handler) StudentReport(c *gin.Context) {
	_, userID, ok := requirePermission(c, RoleStudent, PermissionStudentRead)
	if !ok {
		return
	}
	report, err := h.service.StudentReport(c.Request.Context(), userID)
	if err != nil {
		writeReportError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"report": report})
}

func (h *Handler) LecturerOfferingReport(c *gin.Context) {
	_, userID, ok := requirePermission(c, RoleLecturer, PermissionLecturerRead)
	if !ok {
		return
	}
	offeringID, ok := parsePathUUID(c, "id")
	if !ok {
		return
	}
	report, err := h.service.LecturerOfferingReport(
		c.Request.Context(), offeringID, userID,
	)
	if errors.Is(err, ErrNotFound) {
		writeError(c, http.StatusNotFound, "not_found", "The requested offering was not found.")
		return
	}
	if err != nil {
		writeReportError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"report": report})
}

func (h *Handler) AdminSummary(c *gin.Context) {
	if _, _, ok := requirePermission(c, RoleAdmin, PermissionAdminRead); !ok {
		return
	}
	report, err := h.service.AdminSummary(c.Request.Context())
	if err != nil {
		writeReportError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"summary": report})
}

func (h *Handler) ListGradeBands(c *gin.Context) {
	if !requireAdminManage(c) {
		return
	}
	bands, err := h.service.GradeBands(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "Unable to read the grade scheme.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"grade_bands": bands})
}

func (h *Handler) ReplaceGradeBands(c *gin.Context) {
	if !requireAdminManage(c) {
		return
	}
	var request struct {
		GradeBands []GradeBand `json:"grade_bands"`
	}
	if !bindJSON(c, &request) {
		return
	}
	if err := h.service.ReplaceGradeBands(c.Request.Context(), request.GradeBands); err != nil {
		if errors.Is(err, ErrValidation) {
			writeError(c, http.StatusUnprocessableEntity, "validation_error", err.Error())
			return
		}
		writeError(c, http.StatusInternalServerError, "internal_error", "Unable to update the grade scheme.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"grade_bands": request.GradeBands})
}

func (h *Handler) ListProgrammes(c *gin.Context) {
	if !requireAdminManage(c) {
		return
	}
	items, err := h.service.ListProgrammes(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "Unable to read programmes.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"programmes": items})
}

func (h *Handler) CreateProgramme(c *gin.Context) {
	if !requireAdminManage(c) {
		return
	}
	var input CreateProgrammeInput
	if !bindJSON(c, &input) {
		return
	}
	item, err := h.service.SaveProgramme(c.Request.Context(), input, nil)
	if err != nil {
		writeProgrammeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"programme": item})
}

func (h *Handler) UpdateProgramme(c *gin.Context) {
	if !requireAdminManage(c) {
		return
	}
	id, ok := parsePathUUID(c, "id")
	if !ok {
		return
	}
	var input CreateProgrammeInput
	if !bindJSON(c, &input) {
		return
	}
	item, err := h.service.SaveProgramme(c.Request.Context(), input, &id)
	if errors.Is(err, ErrNotFound) {
		writeError(c, http.StatusNotFound, "not_found", "The requested programme was not found.")
		return
	}
	if err != nil {
		writeProgrammeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"programme": item})
}

func (h *Handler) AssignProgramme(c *gin.Context) {
	_, actorID, ok := requireAdminManageClaims(c)
	if !ok {
		return
	}
	studentID, ok := parsePathUUID(c, "id")
	if !ok {
		return
	}
	var request struct {
		ProgrammeID uuid.UUID `json:"programme_id"`
	}
	if !bindJSON(c, &request) {
		return
	}
	if err := h.service.AssignProgramme(c.Request.Context(), studentID,
		request.ProgrammeID, actorID); err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "The requested programme was not found.")
			return
		}
		writeError(c, http.StatusInternalServerError, "internal_error", "Unable to assign the programme.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"student_user_id": studentID, "programme_id": request.ProgrammeID})
}

func (h *Handler) AdminStudentReport(c *gin.Context) {
	_, _, ok := requirePermission(c, RoleAdmin, PermissionAdminStudentRead)
	if !ok {
		return
	}
	studentID, ok := parsePathUUID(c, "id")
	if !ok {
		return
	}
	report, err := h.service.StudentReport(c.Request.Context(), studentID)
	if err != nil {
		writeReportError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"report": report})
}

func (h *Handler) Rebuild(c *gin.Context) {
	if !requireAdminManage(c) {
		return
	}
	if err := h.service.Rebuild(c.Request.Context()); err != nil {
		if errors.Is(err, ErrRebuildRunning) {
			writeError(c, http.StatusConflict, "rebuild_in_progress", "An Analytics rebuild is already running.")
			return
		}
		writeError(c, http.StatusInternalServerError, "rebuild_failed", "The Analytics projection rebuild failed.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": "rebuilt"})
}

func requirePermission(
	c *gin.Context, role, permission string,
) (*AccessClaims, uuid.UUID, bool) {
	claims, ok := ClaimsFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized", "Authentication is required.")
		return nil, uuid.Nil, false
	}
	userID, err := claims.UserID()
	if err != nil || userID == uuid.Nil {
		writeError(c, http.StatusUnauthorized, "unauthorized", "The access token is invalid.")
		return nil, uuid.Nil, false
	}
	if !HasRole(claims, role) || !HasPermission(claims, permission) {
		writeError(c, http.StatusForbidden, "forbidden", "You do not have permission to access this report.")
		return nil, uuid.Nil, false
	}
	return claims, userID, true
}

func requireAdminManage(c *gin.Context) bool {
	_, _, ok := requireAdminManageClaims(c)
	return ok
}

func requireAdminManageClaims(c *gin.Context) (*AccessClaims, uuid.UUID, bool) {
	return requirePermission(c, RoleAdmin, PermissionManage)
}

func parsePathUUID(c *gin.Context, key string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(key))
	if err != nil || id == uuid.Nil {
		writeError(c, http.StatusBadRequest, "validation_error", "The ID must be a valid UUID.")
		return uuid.Nil, false
	}
	return id, true
}

func bindJSON(c *gin.Context, target any) bool {
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "The JSON request body is invalid.")
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		writeError(c, http.StatusBadRequest, "invalid_request", "The request must contain one JSON object.")
		return false
	}
	return true
}

func writeProgrammeError(c *gin.Context, err error) {
	if errors.Is(err, ErrValidation) {
		writeError(c, http.StatusUnprocessableEntity, "validation_error", err.Error())
		return
	}
	writeError(c, http.StatusInternalServerError, "internal_error", "Unable to save the programme.")
}

func writeReportError(c *gin.Context, err error) {
	if errors.Is(err, ErrProjectionUnavailable) {
		writeError(c, http.StatusServiceUnavailable, "projection_unavailable", "The Analytics projection is unavailable until a rebuild completes.")
		return
	}
	if errors.Is(err, ErrDependencyUnavailable) {
		writeError(c, http.StatusServiceUnavailable, "dependency_unavailable", "Course facts are temporarily unavailable.")
		return
	}
	writeError(c, http.StatusInternalServerError, "internal_error", "Unable to build the Analytics report.")
}
