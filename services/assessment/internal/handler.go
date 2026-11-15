package assessment

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	service        *AssessmentService
	logger         *slog.Logger
	maxUploadBytes int64
	allowedFiles   map[string]string
}

func NewHandler(
	service *AssessmentService,
	logger *slog.Logger,
	maxUploadBytes int64,
	allowedFiles []AllowedFileType,
) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	types := make(map[string]string, len(allowedFiles))
	for _, fileType := range allowedFiles {
		types[fileType.Extension] = fileType.MediaType
	}
	return &Handler{service: service, logger: logger, maxUploadBytes: maxUploadBytes, allowedFiles: types}
}

func (h *Handler) RegisterRoutes(
	router gin.IRouter, auth, internal gin.HandlerFunc,
) {
	api := router.Group("/api/v1")
	api.POST("/offerings/:offering_id/assessments", auth, h.Create)
	api.GET("/offerings/:offering_id/assessments", auth, h.ListForOffering)
	api.GET("/offerings/:offering_id/results", auth, h.ListFinalResults)
	api.POST("/offerings/:offering_id/results/import", auth, h.ImportFinalResults)
	api.GET("/assessments/:id", auth, h.Get)
	api.POST("/assessments/:id/publish", auth, h.ChangeStatus(AssessmentPublished))
	api.POST("/assessments/:id/close", auth, h.ChangeStatus(AssessmentClosed))
	api.POST("/assessments/:id/extensions", auth, h.CreateExtension)
	api.POST("/assessments/:id/submissions", auth, h.Submit)
	api.GET("/assessments/:id/submissions", auth, h.ListSubmissions)
	api.GET("/submissions/:id", auth, h.GetSubmission)
	api.GET("/submissions/:id/file", auth, h.DownloadSubmission)
	api.PUT("/submissions/:id/grade", auth, h.SaveGrade)
	api.POST("/submissions/:id/release", auth, h.ReleaseGrade)
	router.GET("/internal/analytics/events",
		internal, RequireInternalCaller("analytics-service"), h.ListAnalyticsEvents)
}

func (h *Handler) ListAnalyticsEvents(c *gin.Context) {
	after := time.Unix(0, 0).UTC()
	if value := strings.TrimSpace(c.Query("after_occurred_at")); value != "" {
		parsed, err := time.Parse(time.RFC3339Nano, value)
		if err != nil {
			respondError(c, h.logger, newValidationError(
				"after_occurred_at", "must be a valid RFC3339 timestamp",
			))
			return
		}
		after = parsed
	}
	afterID := uuid.Nil
	if value := strings.TrimSpace(c.Query("after_event_id")); value != "" {
		parsed, err := uuid.Parse(value)
		if err != nil || parsed == uuid.Nil {
			respondError(c, h.logger, newValidationError(
				"after_event_id", "must be a valid UUID",
			))
			return
		}
		afterID = parsed
	}
	limit := 250
	if value := strings.TrimSpace(c.Query("limit")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 500 {
			respondError(c, h.logger, newValidationError(
				"limit", "must be between 1 and 500",
			))
			return
		}
		limit = parsed
	}
	page, err := h.service.ListAnalyticsEvents(
		c.Request.Context(), after, afterID, limit,
	)
	if err != nil {
		respondError(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, page)
}

type createAssessmentRequest struct {
	Title            string            `json:"title"`
	Description      string            `json:"description"`
	ComponentKind    ComponentKind     `json:"component_kind"`
	DueAt            time.Time         `json:"due_at"`
	MaxMark          float64           `json:"max_mark"`
	WeightPercent    float64           `json:"weight_percent"`
	MaxAttempts      int               `json:"max_attempts"`
	LatePolicy       LatePolicy        `json:"late_policy"`
	AllowedFileTypes []AllowedFileType `json:"allowed_file_types"`
}

func (h *Handler) Create(c *gin.Context) {
	actor, ok := actorOrReject(c)
	if !ok {
		return
	}
	offeringID, ok := parsePathUUID(c, "offering_id")
	if !ok {
		return
	}
	var request createAssessmentRequest
	if !BindJSON(c, &request) {
		return
	}
	item, err := h.service.Create(c.Request.Context(), actor, CreateAssessmentInput{
		OfferingID: offeringID, Title: request.Title, Description: request.Description,
		ComponentKind: request.ComponentKind, DueAt: request.DueAt,
		MaxMark: request.MaxMark, WeightPercent: request.WeightPercent,
		MaxAttempts: request.MaxAttempts, LatePolicy: request.LatePolicy,
		AllowedFileTypes: request.AllowedFileTypes,
	})
	if err != nil {
		respondError(c, h.logger, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"assessment": item})
}

func (h *Handler) ListForOffering(c *gin.Context) {
	actor, ok := actorOrReject(c)
	if !ok {
		return
	}
	offeringID, ok := parsePathUUID(c, "offering_id")
	if !ok {
		return
	}
	limit, offset, ok := parsePagination(c)
	if !ok {
		return
	}
	filter := AssessmentFilter{Status: AssessmentStatus(c.Query("status")), Limit: limit, Offset: offset}
	if filter.Status != "" && filter.Status != AssessmentDraft &&
		filter.Status != AssessmentPublished && filter.Status != AssessmentClosed {
		RespondValidationError(c, "status", "is not a valid assessment status")
		return
	}
	result, err := h.service.ListForOffering(c.Request.Context(), actor, offeringID, filter)
	if err != nil {
		respondError(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) Get(c *gin.Context) {
	actor, ok := actorOrReject(c)
	if !ok {
		return
	}
	id, ok := parsePathUUID(c, "id")
	if !ok {
		return
	}
	item, err := h.service.Get(c.Request.Context(), actor, id)
	if err != nil {
		respondError(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"assessment": item})
}

func (h *Handler) ListFinalResults(c *gin.Context) {
	actor, ok := actorOrReject(c)
	if !ok {
		return
	}
	offeringID, ok := parsePathUUID(c, "offering_id")
	if !ok {
		return
	}
	limit, offset, ok := parsePagination(c)
	if !ok {
		return
	}
	result, err := h.service.ListFinalResults(
		c.Request.Context(), actor, offeringID, limit, offset,
	)
	if err != nil {
		respondError(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) ImportFinalResults(c *gin.Context) {
	actor, ok := actorOrReject(c)
	if !ok {
		return
	}
	offeringID, ok := parsePathUUID(c, "offering_id")
	if !ok {
		return
	}
	idempotencyKey := c.GetHeader("Idempotency-Key")
	if idempotencyKey == "" {
		RespondValidationError(c, "Idempotency-Key", "is required")
		return
	}
	if err := c.Request.ParseMultipartForm(1 << 20); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			AbortWithError(c, http.StatusRequestEntityTooLarge, "payload_too_large",
				"The request body is too large.")
		} else {
			RespondValidationError(c, "file", "multipart form data is required")
		}
		return
	}
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		RespondValidationError(c, "file", "a CSV file is required")
		return
	}
	defer file.Close()
	if !strings.EqualFold(filepath.Ext(header.Filename), ".csv") {
		RespondValidationError(c, "file", "filename must use the .csv extension")
		return
	}
	if header.Size < 1 {
		RespondValidationError(c, "file", "must be a non-empty CSV file")
		return
	}
	if header.Size > maxFinalResultImportBytes {
		AbortWithError(c, http.StatusRequestEntityTooLarge, "payload_too_large",
			"The CSV file exceeds the 5 MiB size limit.")
		return
	}
	content, err := io.ReadAll(io.LimitReader(file, maxFinalResultImportBytes+1))
	if err != nil {
		respondError(c, h.logger, fmt.Errorf("read final result CSV: %w", err))
		return
	}
	if int64(len(content)) > maxFinalResultImportBytes {
		RespondValidationError(c, "file", "must be no larger than 5 MiB")
		return
	}
	batch, err := h.service.ImportFinalResults(
		c.Request.Context(), actor, offeringID, idempotencyKey, content, RequestID(c),
	)
	if err != nil {
		respondError(c, h.logger, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"import": batch})
}

func (h *Handler) ChangeStatus(next AssessmentStatus) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := actorOrReject(c)
		if !ok {
			return
		}
		id, ok := parsePathUUID(c, "id")
		if !ok {
			return
		}
		var request struct {
			Version int64 `json:"version"`
		}
		if !BindJSON(c, &request) {
			return
		}
		item, err := h.service.ChangeStatus(c.Request.Context(), actor, id, request.Version, next)
		if err != nil {
			respondError(c, h.logger, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"assessment": item})
	}
}

func (h *Handler) CreateExtension(c *gin.Context) {
	actor, ok := actorOrReject(c)
	if !ok {
		return
	}
	assessmentID, ok := parsePathUUID(c, "id")
	if !ok {
		return
	}
	var request struct {
		StudentUserID uuid.UUID `json:"student_user_id"`
		RevisedDueAt  time.Time `json:"revised_due_at"`
		Reason        string    `json:"reason"`
	}
	if !BindJSON(c, &request) {
		return
	}
	if err := h.service.CreateExtension(c.Request.Context(), actor, CreateExtensionInput{
		AssessmentID: assessmentID, StudentID: request.StudentUserID,
		RevisedDueAt: request.RevisedDueAt, Reason: request.Reason,
	}); err != nil {
		respondError(c, h.logger, err)
		return
	}
	c.Status(http.StatusCreated)
}

func (h *Handler) Submit(c *gin.Context) {
	actor, ok := actorOrReject(c)
	if !ok {
		return
	}
	assessmentID, ok := parsePathUUID(c, "id")
	if !ok {
		return
	}
	key := c.GetHeader("Idempotency-Key")
	if key == "" {
		RespondValidationError(c, "Idempotency-Key", "is required")
		return
	}
	if err := c.Request.ParseMultipartForm(1 << 20); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			AbortWithError(c, http.StatusRequestEntityTooLarge, "payload_too_large",
				"The request body is too large.")
		} else {
			RespondValidationError(c, "file", "multipart form data is required")
		}
		return
	}
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		RespondValidationError(c, "file", "a file is required")
		return
	}
	defer file.Close()
	if header.Size < 1 {
		RespondValidationError(c, "file", "must be a non-empty file")
		return
	}
	if header.Size > h.maxUploadBytes {
		AbortWithError(c, http.StatusRequestEntityTooLarge, "payload_too_large",
			"The uploaded file exceeds the configured size limit.")
		return
	}
	prefix := make([]byte, 3072)
	read, readErr := io.ReadFull(file, prefix)
	if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
		respondError(c, h.logger, fmt.Errorf("read submission header: %w", readErr))
		return
	}
	mediaType, _, err := mime.ParseMediaType(mimetype.Detect(prefix[:read]).String())
	if err != nil {
		RespondValidationError(c, "file", "file content type could not be determined")
		return
	}
	extension := strings.TrimPrefix(strings.ToLower(filepath.Ext(header.Filename)), ".")
	expected, allowed := h.allowedFiles[extension]
	if !allowed || expected != mediaType {
		RespondValidationError(c, "file", "file content does not match an allowed file type")
		return
	}
	reader := io.MultiReader(bytes.NewReader(prefix[:read]), file)
	item, err := h.service.Submit(c.Request.Context(), actor, assessmentID, key, Upload{
		Filename: header.Filename, MediaType: mediaType, SizeBytes: header.Size, Reader: reader,
	})
	if err != nil {
		respondError(c, h.logger, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"submission": item})
}

func (h *Handler) ListSubmissions(c *gin.Context) {
	actor, ok := actorOrReject(c)
	if !ok {
		return
	}
	assessmentID, ok := parsePathUUID(c, "id")
	if !ok {
		return
	}
	limit, offset, ok := parsePagination(c)
	if !ok {
		return
	}
	result, err := h.service.ListSubmissions(c.Request.Context(), actor, assessmentID, limit, offset)
	if err != nil {
		respondError(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) GetSubmission(c *gin.Context) {
	actor, ok := actorOrReject(c)
	if !ok {
		return
	}
	id, ok := parsePathUUID(c, "id")
	if !ok {
		return
	}
	submission, grade, err := h.service.GetSubmission(c.Request.Context(), actor, id)
	if err != nil {
		respondError(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"submission": submission, "grade": grade})
}

func (h *Handler) DownloadSubmission(c *gin.Context) {
	actor, ok := actorOrReject(c)
	if !ok {
		return
	}
	id, ok := parsePathUUID(c, "id")
	if !ok {
		return
	}
	submission, body, mediaType, err := h.service.DownloadSubmission(c.Request.Context(), actor, id)
	if err != nil {
		respondError(c, h.logger, err)
		return
	}
	defer body.Close()
	disposition := mime.FormatMediaType("attachment", map[string]string{
		"filename": submission.OriginalFilename,
	})
	c.Header("Content-Type", mediaType)
	c.Header("Content-Disposition", disposition)
	c.Header("Content-Length", strconv.FormatInt(submission.SizeBytes, 10))
	c.Header("X-Content-Type-Options", "nosniff")
	if _, err := io.Copy(c.Writer, body); err != nil {
		h.logger.Error("submission_download_failed", "error", err,
			"submission_id", submission.ID, "request_id", RequestID(c))
	}
}

func (h *Handler) SaveGrade(c *gin.Context) {
	actor, ok := actorOrReject(c)
	if !ok {
		return
	}
	id, ok := parsePathUUID(c, "id")
	if !ok {
		return
	}
	var request struct {
		Version  int64   `json:"version"`
		Mark     float64 `json:"mark"`
		Feedback string  `json:"feedback"`
	}
	if !BindJSON(c, &request) {
		return
	}
	grade, err := h.service.SaveGrade(
		c.Request.Context(), actor, id, request.Mark, request.Feedback, request.Version,
	)
	if err != nil {
		respondError(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"grade": grade})
}

func (h *Handler) ReleaseGrade(c *gin.Context) {
	actor, ok := actorOrReject(c)
	if !ok {
		return
	}
	id, ok := parsePathUUID(c, "id")
	if !ok {
		return
	}
	var request struct {
		Version int64 `json:"version"`
	}
	if !BindJSON(c, &request) {
		return
	}
	grade, err := h.service.ReleaseGrade(
		c.Request.Context(), actor, id, request.Version, RequestID(c),
	)
	if err != nil {
		respondError(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"grade": grade})
}

