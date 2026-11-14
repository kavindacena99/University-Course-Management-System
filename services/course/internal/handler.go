package course

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type courseService interface {
	CreateCourse(ctx context.Context, claims *AccessClaims, input CreateCourseInput) (*Course, error)
	GetCourse(ctx context.Context, claims *AccessClaims, courseID uuid.UUID) (*Course, error)
	ListCourses(ctx context.Context, claims *AccessClaims, filter CourseFilter) (*CourseList, error)
	UpdateCourse(ctx context.Context, claims *AccessClaims, courseID uuid.UUID, input UpdateCourseInput) (*Course, error)
	ChangeCourseStatus(ctx context.Context, claims *AccessClaims, courseID uuid.UUID, action string, expectedVersion int64) (*Course, error)

	CreateOffering(ctx context.Context, claims *AccessClaims, input CreateOfferingInput) (*CourseOffering, error)
	GetOffering(ctx context.Context, claims *AccessClaims, offeringID uuid.UUID) (*CourseOffering, error)
	ListCourseOfferings(ctx context.Context, claims *AccessClaims, courseID uuid.UUID, filter OfferingFilter) (*OfferingList, error)
	ListOfferings(ctx context.Context, claims *AccessClaims, filter OfferingFilter) (*OfferingList, error)
	UpdateOffering(ctx context.Context, claims *AccessClaims, offeringID uuid.UUID, input UpdateOfferingInput) (*CourseOffering, error)
	AssignLecturer(ctx context.Context, claims *AccessClaims, offeringID uuid.UUID, input AssignLecturerInput) (*CourseOffering, error)
	ChangeOfferingStatus(ctx context.Context, claims *AccessClaims, offeringID uuid.UUID, action string, expectedVersion int64) (*CourseOffering, error)
}

type Handler struct {
	service courseService
	logger  *slog.Logger
}

func NewHandler(service courseService, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, logger: logger}
}

func (h *Handler) RegisterRoutes(router gin.IRouter, auth gin.HandlerFunc) {
	api := router.Group("/api/v1")
	authenticated := func(handler gin.HandlerFunc) []gin.HandlerFunc {
		return withMiddleware([]gin.HandlerFunc{auth}, handler)
	}
	permitted := func(permission string, handler gin.HandlerFunc) []gin.HandlerFunc {
		return withMiddleware(
			[]gin.HandlerFunc{auth, RequirePermissions(permission)},
			handler,
		)
	}

	api.GET("/courses", authenticated(h.ListCourses)...)
	api.GET("/courses/:id", authenticated(h.GetCourse)...)
	api.GET("/courses/:id/offerings", authenticated(h.ListCourseOfferings)...)

	api.GET("/offerings", authenticated(h.ListOfferings)...)
	api.GET("/offerings/assigned", authenticated(h.ListAssignedOfferings)...)
	api.GET("/offerings/:id", authenticated(h.GetOffering)...)
	api.PUT("/offerings/:id", permitted(PermOfferingsManage, h.UpdateOffering)...)
	api.POST("/offerings/:id/publish",
		permitted(PermOfferingsManage, h.ChangeOfferingStatus("publish"))...)
	api.POST("/offerings/:id/close",
		permitted(PermOfferingsManage, h.ChangeOfferingStatus("close"))...)
	api.POST("/offerings/:id/archive",
		permitted(PermOfferingsManage, h.ChangeOfferingStatus("archive"))...)

	api.POST("/admin/courses",
		permitted(PermCoursesManage, h.CreateCourse)...)
	api.PUT("/admin/courses/:id",
		permitted(PermCoursesManage, h.UpdateCourse)...)
	api.POST("/admin/courses/:id/publish",
		permitted(PermCoursesManage, h.ChangeCourseStatus("publish"))...)
	api.POST("/admin/courses/:id/archive",
		permitted(PermCoursesManage, h.ChangeCourseStatus("archive"))...)
	api.POST("/admin/courses/:id/restore",
		permitted(PermCoursesManage, h.ChangeCourseStatus("restore"))...)

	api.POST("/admin/offerings",
		permitted(PermCoursesManage, h.CreateOffering)...)
	api.PUT("/admin/offerings/:id/lecturer",
		permitted(PermCoursesManage, h.AssignLecturer)...)
	api.POST("/admin/offerings/:id/restore",
		permitted(PermCoursesManage, h.ChangeOfferingStatus("restore"))...)
}

type createCourseRequest struct {
	Code        string `json:"code"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Credits     int    `json:"credits"`
}

func (h *Handler) CreateCourse(c *gin.Context) {
	claims, ok := claimsOrReject(c)
	if !ok {
		return
	}
	var req createCourseRequest
	if !BindJSON(c, &req) {
		return
	}

	item, err := h.service.CreateCourse(c.Request.Context(), claims, CreateCourseInput{
		Code:        req.Code,
		Title:       req.Title,
		Description: req.Description,
		Credits:     req.Credits,
	})
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"course": item})
}

type updateCourseRequest struct {
	Version     int64  `json:"version"`
	Code        string `json:"code"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Credits     int    `json:"credits"`
}

func (h *Handler) UpdateCourse(c *gin.Context) {
	claims, ok := claimsOrReject(c)
	if !ok {
		return
	}
	courseID, ok := parseIDParam(c, "id", "course")
	if !ok {
		return
	}
	var req updateCourseRequest
	if !BindJSON(c, &req) {
		return
	}

	item, err := h.service.UpdateCourse(c.Request.Context(), claims, courseID,
		UpdateCourseInput{
			ExpectedVersion: req.Version,
			Code:            req.Code,
			Title:           req.Title,
			Description:     req.Description,
			Credits:         req.Credits,
		})
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"course": item})
}

func (h *Handler) GetCourse(c *gin.Context) {
	claims, ok := claimsOrReject(c)
	if !ok {
		return
	}
	courseID, ok := parseIDParam(c, "id", "course")
	if !ok {
		return
	}

	item, err := h.service.GetCourse(c.Request.Context(), claims, courseID)
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"course": item})
}

func (h *Handler) ListCourses(c *gin.Context) {
	claims, ok := claimsOrReject(c)
	if !ok {
		return
	}
	limit, offset, ok := parsePagination(c)
	if !ok {
		return
	}

	result, err := h.service.ListCourses(c.Request.Context(), claims, CourseFilter{
		Status: CourseStatus(c.Query("status")),
		Search: c.Query("search"),
		Limit:  limit,
		Offset: offset,
		Sort:   c.Query("sort"),
		Order:  c.Query("order"),
	})
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) ChangeCourseStatus(action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := claimsOrReject(c)
		if !ok {
			return
		}
		courseID, ok := parseIDParam(c, "id", "course")
		if !ok {
			return
		}
		expectedVersion, ok := parseVersionBody(c)
		if !ok {
			return
		}

		item, err := h.service.ChangeCourseStatus(
			c.Request.Context(), claims, courseID, action, expectedVersion,
		)
		if err != nil {
			h.respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"course": item})
	}
}

type createOfferingRequest struct {
	CourseID           uuid.UUID  `json:"course_id"`
	AcademicYear       string     `json:"academic_year"`
	Semester           string     `json:"semester"`
	StartDate          string     `json:"start_date"`
	EndDate            string     `json:"end_date"`
	EnrollmentOpensAt  *time.Time `json:"enrollment_opens_at"`
	EnrollmentClosesAt *time.Time `json:"enrollment_closes_at"`
	EnrollmentCapacity *int       `json:"enrollment_capacity"`
	LecturerUserID     *uuid.UUID `json:"lecturer_user_id"`
}

func (h *Handler) CreateOffering(c *gin.Context) {
	claims, ok := claimsOrReject(c)
	if !ok {
		return
	}
	var req createOfferingRequest
	if !BindJSON(c, &req) {
		return
	}

	item, err := h.service.CreateOffering(c.Request.Context(), claims,
		CreateOfferingInput{
			CourseID:           req.CourseID,
			AcademicYear:       req.AcademicYear,
			Semester:           req.Semester,
			StartDate:          req.StartDate,
			EndDate:            req.EndDate,
			EnrollmentOpensAt:  req.EnrollmentOpensAt,
			EnrollmentClosesAt: req.EnrollmentClosesAt,
			EnrollmentCapacity: req.EnrollmentCapacity,
			LecturerUserID:     req.LecturerUserID,
		})
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"offering": item})
}

type updateOfferingRequest struct {
	Version            int64      `json:"version"`
	AcademicYear       string     `json:"academic_year"`
	Semester           string     `json:"semester"`
	StartDate          string     `json:"start_date"`
	EndDate            string     `json:"end_date"`
	EnrollmentOpensAt  *time.Time `json:"enrollment_opens_at"`
	EnrollmentClosesAt *time.Time `json:"enrollment_closes_at"`
	EnrollmentCapacity *int       `json:"enrollment_capacity"`
}

func (h *Handler) UpdateOffering(c *gin.Context) {
	claims, ok := claimsOrReject(c)
	if !ok {
		return
	}
	offeringID, ok := parseIDParam(c, "id", "offering")
	if !ok {
		return
	}
	var req updateOfferingRequest
	if !BindJSON(c, &req) {
		return
	}

	item, err := h.service.UpdateOffering(c.Request.Context(), claims,
		offeringID, UpdateOfferingInput{
			ExpectedVersion:    req.Version,
			AcademicYear:       req.AcademicYear,
			Semester:           req.Semester,
			StartDate:          req.StartDate,
			EndDate:            req.EndDate,
			EnrollmentOpensAt:  req.EnrollmentOpensAt,
			EnrollmentClosesAt: req.EnrollmentClosesAt,
			EnrollmentCapacity: req.EnrollmentCapacity,
		})
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"offering": item})
}

func (h *Handler) GetOffering(c *gin.Context) {
	claims, ok := claimsOrReject(c)
	if !ok {
		return
	}
	offeringID, ok := parseIDParam(c, "id", "offering")
	if !ok {
		return
	}

	item, err := h.service.GetOffering(c.Request.Context(), claims, offeringID)
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"offering": item})
}

func (h *Handler) ListOfferings(c *gin.Context) {
	h.listOfferings(c, nil, false)
}

func (h *Handler) ListAssignedOfferings(c *gin.Context) {
	h.listOfferings(c, nil, true)
}

func (h *Handler) ListCourseOfferings(c *gin.Context) {
	courseID, ok := parseIDParam(c, "id", "course")
	if !ok {
		return
	}
	h.listOfferings(c, &courseID, false)
}

func (h *Handler) listOfferings(
	c *gin.Context,
	courseID *uuid.UUID,
	assignedToMe bool,
) {
	claims, ok := claimsOrReject(c)
	if !ok {
		return
	}
	limit, offset, ok := parsePagination(c)
	if !ok {
		return
	}
	lecturerID, ok := parseOptionalUUIDQuery(c, "lecturer_user_id", "lecturer_user_id")
	if !ok {
		return
	}
	assigned, ok := parseOptionalBoolQuery(c, "assigned_to_me")
	if !ok {
		return
	}

	filter := OfferingFilter{
		CourseID:     courseID,
		AcademicYear: c.Query("academic_year"),
		Semester:     c.Query("semester"),
		Status:       OfferingStatus(c.Query("status")),
		LecturerID:   lecturerID,
		AssignedToMe: assignedToMe || assigned,
		Search:       c.Query("search"),
		Limit:        limit,
		Offset:       offset,
		Sort:         c.Query("sort"),
		Order:        c.Query("order"),
	}

	var (
		result *OfferingList
		err    error
	)
	if courseID != nil {
		result, err = h.service.ListCourseOfferings(
			c.Request.Context(), claims, *courseID, filter,
		)
	} else {
		result, err = h.service.ListOfferings(c.Request.Context(), claims, filter)
	}
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

type assignLecturerRequest struct {
	Version        int64     `json:"version"`
	LecturerUserID uuid.UUID `json:"lecturer_user_id"`
}

func (h *Handler) AssignLecturer(c *gin.Context) {
	claims, ok := claimsOrReject(c)
	if !ok {
		return
	}
	offeringID, ok := parseIDParam(c, "id", "offering")
	if !ok {
		return
	}
	var req assignLecturerRequest
	if !BindJSON(c, &req) {
		return
	}

	item, err := h.service.AssignLecturer(c.Request.Context(), claims,
		offeringID, AssignLecturerInput{
			ExpectedVersion: req.Version,
			LecturerUserID:  req.LecturerUserID,
		})
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"offering": item})
}

func (h *Handler) ChangeOfferingStatus(action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := claimsOrReject(c)
		if !ok {
			return
		}
		offeringID, ok := parseIDParam(c, "id", "offering")
		if !ok {
			return
		}
		expectedVersion, ok := parseVersionBody(c)
		if !ok {
			return
		}

		item, err := h.service.ChangeOfferingStatus(
			c.Request.Context(), claims, offeringID, action, expectedVersion,
		)
		if err != nil {
			h.respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"offering": item})
	}
}

type statusRequest struct {
	Version int64 `json:"version"`
}

func parseVersionBody(c *gin.Context) (int64, bool) {
	var req statusRequest
	if !BindJSON(c, &req) {
		return 0, false
	}
	if req.Version <= 0 {
		AbortWithError(c, http.StatusBadRequest, CodeValidationFailed,
			"version: must be provided and positive.")
		return 0, false
	}
	return req.Version, true
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
		ErrCourseNotFound,
		ErrOfferingNotFound,
		ErrCourseCodeTaken,
		ErrCourseCodeLocked,
		ErrCourseArchived,
		ErrCourseNotActive,
		ErrCourseHasActiveOfferings,
		ErrOfferingImmutable,
		ErrFieldLocked,
		ErrInvalidTransition,
		ErrVersionConflict,
		ErrUnauthorized,
		ErrForbidden,
		ErrInvalidToken,
		ErrTokenExpired,
		ErrLecturerNotFound,
		ErrLecturerIneligible,
		ErrUserServiceUnavailable,
		ErrAuthServiceUnavailable,
	} {
		if errors.Is(err, sentinel) {
			return true
		}
	}
	return false
}

func claimsOrReject(c *gin.Context) (*AccessClaims, bool) {
	claims, ok := ClaimsFromContext(c)
	if !ok {
		AbortWithError(c, http.StatusUnauthorized, CodeUnauthorized,
			"Authentication is required.")
		return nil, false
	}
	return claims, true
}

func parseIDParam(c *gin.Context, name, label string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		AbortWithError(c, http.StatusBadRequest, CodeValidationFailed,
			"The "+label+" id must be a valid UUID.")
		return uuid.Nil, false
	}
	return id, true
}

func parseOptionalUUIDQuery(
	c *gin.Context,
	queryName string,
	label string,
) (*uuid.UUID, bool) {
	value := c.Query(queryName)
	if value == "" {
		return nil, true
	}
	id, err := uuid.Parse(value)
	if err != nil {
		AbortWithError(c, http.StatusBadRequest, CodeValidationFailed,
			"The "+label+" parameter must be a valid UUID.")
		return nil, false
	}
	return &id, true
}

func parseOptionalBoolQuery(c *gin.Context, queryName string) (bool, bool) {
	value := c.Query(queryName)
	if value == "" {
		return false, true
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		AbortWithError(c, http.StatusBadRequest, CodeValidationFailed,
			"The "+queryName+" parameter must be true or false.")
		return false, false
	}
	return parsed, true
}

func parsePagination(c *gin.Context) (int, int, bool) {
	limit := 20
	offset := 0

	if value := c.Query("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 100 {
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
