package assessment

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type apiErrorResponse struct {
	Error struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"request_id,omitempty"`
	} `json:"error"`
}

func AbortWithError(c *gin.Context, status int, code, message string) {
	response := apiErrorResponse{}
	response.Error.Code = code
	response.Error.Message = message
	response.Error.RequestID = RequestID(c)
	c.AbortWithStatusJSON(status, response)
}

func RespondValidationError(c *gin.Context, field, message string) {
	AbortWithError(c, http.StatusBadRequest, "validation_failed",
		(&ValidationError{Field: field, Message: message}).Error())
}

func BindJSON(c *gin.Context, destination any) bool {
	mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || mediaType != "application/json" {
		AbortWithError(c, http.StatusUnsupportedMediaType, "unsupported_media_type",
			"Content-Type must be application/json.")
		return false
	}
	if c.Request.Body == nil {
		AbortWithError(c, http.StatusBadRequest, "invalid_request", "A JSON request body is required.")
		return false
	}
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			AbortWithError(c, http.StatusRequestEntityTooLarge, "payload_too_large",
				"The request body is too large.")
			return false
		}
		AbortWithError(c, http.StatusBadRequest, "invalid_request", "The JSON request body is invalid.")
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		AbortWithError(c, http.StatusBadRequest, "invalid_request", "Only one JSON value is allowed.")
		return false
	}
	return true
}

func RespondServiceError(c *gin.Context, err error) {
	var validation *ValidationError
	if errors.As(err, &validation) {
		RespondValidationError(c, validation.Field, validation.Message)
		return
	}
	switch {
	case errors.Is(err, ErrNotFound):
		AbortWithError(c, http.StatusNotFound, "not_found", "The requested resource was not found.")
	case errors.Is(err, ErrForbidden):
		AbortWithError(c, http.StatusForbidden, "forbidden", "You do not have permission to perform this action.")
	case errors.Is(err, ErrUnauthorized), errors.Is(err, ErrInvalidToken), errors.Is(err, ErrTokenExpired):
		AbortWithError(c, http.StatusUnauthorized, "unauthorized", "Authentication is required.")
	case errors.Is(err, ErrStudentIneligible):
		AbortWithError(c, http.StatusUnprocessableEntity, "student_ineligible", "The student is not eligible.")
	case errors.Is(err, ErrDependencyUnavailable):
		AbortWithError(c, http.StatusServiceUnavailable, "dependency_unavailable",
			"Eligibility cannot be verified at this time.")
	case errors.Is(err, ErrVersionConflict):
		AbortWithError(c, http.StatusConflict, "version_conflict",
			"The resource changed; refresh and try again.")
	case errors.Is(err, ErrInvalidTransition):
		AbortWithError(c, http.StatusConflict, "invalid_status_transition",
			"The resource cannot make that transition.")
	case errors.Is(err, ErrAttemptLimitReached):
		AbortWithError(c, http.StatusConflict, "attempt_limit_reached",
			"The maximum number of attempts has been reached.")
	case errors.Is(err, ErrIdempotencyConflict):
		AbortWithError(c, http.StatusConflict, "idempotency_conflict",
			"The idempotency key was already used for a different file.")
	case errors.Is(err, ErrSubmissionClosed):
		AbortWithError(c, http.StatusConflict, "submission_closed",
			"The submission window is closed.")
	default:
		AbortWithError(c, http.StatusInternalServerError, "internal_error",
			"An internal error occurred.")
	}
}

func respondError(c *gin.Context, logger *slog.Logger, err error) {
	var validation *ValidationError
	if !errors.As(err, &validation) {
		expected := []error{
			ErrNotFound, ErrForbidden, ErrUnauthorized, ErrInvalidToken, ErrTokenExpired,
			ErrStudentIneligible, ErrDependencyUnavailable, ErrVersionConflict,
			ErrInvalidTransition, ErrAttemptLimitReached, ErrIdempotencyConflict,
			ErrSubmissionClosed,
		}
		known := false
		for _, candidate := range expected {
			if errors.Is(err, candidate) {
				known = true
				break
			}
		}
		if !known {
			logger.Error("request_failed", "error", err, "path", c.Request.URL.Path,
				"request_id", RequestID(c))
		}
	}
	RespondServiceError(c, err)
}

func parsePathUUID(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil || id == uuid.Nil {
		RespondValidationError(c, name, "must be a valid UUID")
		return uuid.Nil, false
	}
	return id, true
}

func actorOrReject(c *gin.Context) (Actor, bool) {
	claims, ok := ClaimsFromContext(c)
	actor, valid := actorFromClaims(claims)
	if !ok || !valid {
		AbortWithError(c, http.StatusUnauthorized, "unauthorized", "Authentication is required.")
		return Actor{}, false
	}
	return actor, true
}

func parsePagination(c *gin.Context) (int, int, bool) {
	limit := 20
	offset := 0
	if value := c.Query("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			RespondValidationError(c, "limit", "must be an integer")
			return 0, 0, false
		}
		limit = parsed
	}
	if value := c.Query("offset"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			RespondValidationError(c, "offset", "must be an integer")
			return 0, 0, false
		}
		offset = parsed
	}
	if limit < 1 || limit > 100 {
		RespondValidationError(c, "limit", "must be between 1 and 100")
		return 0, 0, false
	}
	if offset < 0 || offset > 1_000_000 {
		RespondValidationError(c, "offset", "must be between 0 and 1000000")
		return 0, 0, false
	}
	return limit, offset, true
}
