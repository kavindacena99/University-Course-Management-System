package enrollment

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	"github.com/gin-gonic/gin"
)

const requestIDContextKey = "request_id"

const (
	CodeInvalidRequest     = "invalid_request"
	CodeValidationFailed   = "validation_failed"
	CodeUnauthorized       = "unauthorized"
	CodeForbidden          = "forbidden"
	CodeNotFound           = "not_found"
	CodeDuplicateActive    = "duplicate_active_enrollment"
	CodeCapacityReached    = "capacity_reached"
	CodeVersionConflict    = "version_conflict"
	CodeInvalidTransition  = "invalid_status_transition"
	CodeStudentIneligible  = "student_ineligible"
	CodeOfferingIneligible = "offering_ineligible"
	CodeDependencyFailed   = "dependency_unavailable"
	CodeInvalidServiceAuth = "invalid_service_auth"
	CodeInternal           = "internal_error"
	CodeCSRFFailed         = "csrf_validation_failed"
	CodePayloadTooLarge    = "payload_too_large"
	CodeUnsupportedMedia   = "unsupported_media_type"
)

type apiErrorDetail struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

type apiErrorResponse struct {
	Error apiErrorDetail `json:"error"`
}

func RespondError(c *gin.Context, status int, code, message string) {
	c.JSON(status, apiErrorResponse{Error: apiErrorDetail{
		Code: code, Message: message, RequestID: RequestID(c),
	}})
}

func AbortWithError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, apiErrorResponse{Error: apiErrorDetail{
		Code: code, Message: message, RequestID: RequestID(c),
	}})
}

func RespondValidationError(c *gin.Context, field, message string) {
	AbortWithError(c, http.StatusBadRequest, CodeValidationFailed,
		(&ValidationError{Field: field, Message: message}).Error())
}

func RespondServiceError(c *gin.Context, err error) {
	var validation *ValidationError
	if errors.As(err, &validation) {
		RespondValidationError(c, validation.Field, validation.Message)
		return
	}
	switch {
	case errors.Is(err, ErrNotFound):
		AbortWithError(c, http.StatusNotFound, CodeNotFound, "Enrollment or offering not found.")
	case errors.Is(err, ErrDuplicateActive):
		AbortWithError(c, http.StatusConflict, CodeDuplicateActive, "The student is already actively enrolled.")
	case errors.Is(err, ErrCapacityReached):
		AbortWithError(c, http.StatusConflict, CodeCapacityReached, "The offering has no available seats.")
	case errors.Is(err, ErrVersionConflict):
		AbortWithError(c, http.StatusConflict, CodeVersionConflict, "The enrollment changed; refresh and try again.")
	case errors.Is(err, ErrInvalidTransition):
		AbortWithError(c, http.StatusConflict, CodeInvalidTransition, "The enrollment cannot make that transition.")
	case errors.Is(err, ErrForbidden):
		AbortWithError(c, http.StatusForbidden, CodeForbidden, "You do not have permission to perform this action.")
	case errors.Is(err, ErrUnauthorized), errors.Is(err, ErrInvalidToken), errors.Is(err, ErrTokenExpired):
		AbortWithError(c, http.StatusUnauthorized, CodeUnauthorized, "Authentication is required.")
	case errors.Is(err, ErrStudentIneligible):
		AbortWithError(c, http.StatusUnprocessableEntity, CodeStudentIneligible, "The student is not eligible.")
	case errors.Is(err, ErrOfferingIneligible):
		AbortWithError(c, http.StatusUnprocessableEntity, CodeOfferingIneligible, "The offering is not open for enrollment.")
	case errors.Is(err, ErrDependencyUnavailable):
		AbortWithError(c, http.StatusServiceUnavailable, CodeDependencyFailed, "Eligibility cannot be verified at this time.")
	default:
		AbortWithError(c, http.StatusInternalServerError, CodeInternal, "An internal error occurred.")
	}
}

func RequestID(c *gin.Context) string {
	value, _ := c.Get(requestIDContextKey)
	id, _ := value.(string)
	return id
}

func SetRequestID(c *gin.Context, id string) {
	c.Set(requestIDContextKey, id)
}

func BindJSON(c *gin.Context, destination any) bool {
	mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || mediaType != "application/json" {
		AbortWithError(c, http.StatusUnsupportedMediaType, CodeUnsupportedMedia,
			"Content-Type must be application/json.")
		return false
	}
	if c.Request.Body == nil {
		AbortWithError(c, http.StatusBadRequest, CodeInvalidRequest,
			"A JSON request body is required.")
		return false
	}
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			AbortWithError(c, http.StatusRequestEntityTooLarge, CodePayloadTooLarge,
				"The request body is too large.")
			return false
		}
		AbortWithError(c, http.StatusBadRequest, CodeInvalidRequest,
			"The JSON request body is invalid.")
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		AbortWithError(c, http.StatusBadRequest, CodeInvalidRequest,
			"Only one JSON value is allowed.")
		return false
	}
	return true
}

func isUnsafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return true
	}
}
