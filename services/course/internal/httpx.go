package course

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const requestIDContextKey = "request_id"

const (
	CodeInvalidRequest     = "invalid_request"
	CodeValidationFailed   = "validation_failed"
	CodeUnauthorized       = "unauthorized"
	CodeForbidden          = "forbidden"
	CodeNotFound           = "not_found"
	CodeConflict           = "conflict"
	CodeInvalidTransition  = "invalid_status_transition"
	CodeVersionConflict    = "version_conflict"
	CodeCourseCodeConflict = "course_code_conflict"
	CodeCourseCodeLocked   = "course_code_locked"
	CodeCourseArchived     = "course_archived"
	CodeCourseNotActive    = "course_not_active"
	CodeCourseHasOfferings = "course_has_active_offerings"
	CodeOfferingImmutable  = "offering_immutable"
	CodeFieldLocked        = "field_locked"
	CodeLecturerIneligible = "lecturer_ineligible"
	CodeDependencyFailed   = "dependency_unavailable"
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
	c.JSON(status, apiErrorResponse{
		Error: apiErrorDetail{
			Code:      code,
			Message:   message,
			RequestID: RequestID(c),
		},
	})
}

func AbortWithError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, apiErrorResponse{
		Error: apiErrorDetail{
			Code:      code,
			Message:   message,
			RequestID: RequestID(c),
		},
	})
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
	case errors.Is(err, ErrCourseNotFound):
		AbortWithError(c, http.StatusNotFound, CodeNotFound, "Course not found.")
	case errors.Is(err, ErrOfferingNotFound):
		AbortWithError(c, http.StatusNotFound, CodeNotFound, "Course offering not found.")
	case errors.Is(err, ErrCourseCodeTaken):
		AbortWithError(c, http.StatusConflict, CodeCourseCodeConflict,
			"A course with this code already exists.")
	case errors.Is(err, ErrCourseCodeLocked):
		AbortWithError(c, http.StatusConflict, CodeCourseCodeLocked,
			"The course code cannot be changed after offerings exist.")
	case errors.Is(err, ErrCourseArchived):
		AbortWithError(c, http.StatusConflict, CodeCourseArchived,
			"The archived course must be restored first.")
	case errors.Is(err, ErrCourseNotActive):
		AbortWithError(c, http.StatusConflict, CodeCourseNotActive,
			"The parent course must be active before the offering is published.")
	case errors.Is(err, ErrCourseHasActiveOfferings):
		AbortWithError(c, http.StatusConflict, CodeCourseHasOfferings,
			"Archive or close the course's draft and published offerings first.")
	case errors.Is(err, ErrOfferingImmutable):
		AbortWithError(c, http.StatusConflict, CodeOfferingImmutable,
			"Closed or archived offerings cannot be modified.")
	case errors.Is(err, ErrFieldLocked):
		AbortWithError(c, http.StatusConflict, CodeFieldLocked,
			"One or more fields cannot be changed after publication.")
	case errors.Is(err, ErrInvalidTransition):
		AbortWithError(c, http.StatusConflict, CodeInvalidTransition,
			"The requested lifecycle change is not allowed.")
	case errors.Is(err, ErrVersionConflict):
		AbortWithError(c, http.StatusConflict, CodeVersionConflict,
			"The resource was modified by another request. Refresh and try again.")
	case errors.Is(err, ErrForbidden):
		AbortWithError(c, http.StatusForbidden, CodeForbidden,
			"You do not have permission to perform this action.")
	case errors.Is(err, ErrUnauthorized), errors.Is(err, ErrInvalidToken):
		AbortWithError(c, http.StatusUnauthorized, CodeUnauthorized,
			"Authentication is required.")
	case errors.Is(err, ErrTokenExpired):
		AbortWithError(c, http.StatusUnauthorized, CodeUnauthorized,
			"The access token has expired.")
	case errors.Is(err, ErrLecturerNotFound), errors.Is(err, ErrLecturerIneligible):
		AbortWithError(c, http.StatusUnprocessableEntity, CodeLecturerIneligible,
			"The lecturer must be an approved user.")
	case errors.Is(err, ErrUserServiceUnavailable), errors.Is(err, ErrAuthServiceUnavailable):
		AbortWithError(c, http.StatusServiceUnavailable, CodeDependencyFailed,
			"Lecturer eligibility cannot be verified at this time.")
	default:
		AbortWithError(c, http.StatusInternalServerError, CodeInternal,
			"An internal error occurred.")
	}
}

func RequestID(c *gin.Context) string {
	value, _ := c.Get(requestIDContextKey)
	if id, ok := value.(string); ok {
		return id
	}
	return ""
}

func SetRequestID(c *gin.Context, id string) {
	c.Set(requestIDContextKey, id)
}

// BindJSON decodes a single JSON object and rejects unknown fields and trailing
// data. It writes an error response on failure.
func BindJSON(c *gin.Context, destination any) bool {
	if c.Request.Body == nil {
		AbortWithError(c, http.StatusBadRequest, CodeInvalidRequest,
			"A JSON request body is required.")
		return false
	}

	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(destination); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			AbortWithError(c, http.StatusRequestEntityTooLarge, CodePayloadTooLarge,
				"The request body is too large.")
			return false
		}
		message := "The JSON request body is invalid."
		if errors.Is(err, io.ErrUnexpectedEOF) {
			message = "The JSON request body is incomplete."
		}
		AbortWithError(c, http.StatusBadRequest, CodeInvalidRequest, message)
		return false
	}

	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		AbortWithError(c, http.StatusBadRequest, CodeInvalidRequest,
			"The request body must contain exactly one JSON object.")
		return false
	}

	return true
}

func isUnsafeMethod(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return true
	}
}
