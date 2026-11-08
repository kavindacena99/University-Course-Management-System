package user

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
	CodeInvalidRequest      = "invalid_request"
	CodeValidationFailed    = "validation_failed"
	CodeUnauthorized        = "unauthorized"
	CodeForbidden           = "forbidden"
	CodeNotFound            = "not_found"
	CodeConflict            = "conflict"
	CodeInvalidTransition   = "invalid_status_transition"
	CodeVersionConflict     = "version_conflict"
	CodeRateLimited         = "rate_limited"
	CodeInternal            = "internal_error"
	CodeServiceUnavailable  = "service_unavailable"
	CodeInvalidCredentials  = "invalid_credentials"
	CodeAccountUnavailable  = "account_unavailable"
	CodePasswordPolicy      = "password_policy"
	CodeInvalidBootstrap    = "invalid_bootstrap_token"
	CodeBootstrapDisabled   = "bootstrap_disabled"
	CodeCSRFFailed          = "csrf_validation_failed"
	CodeInvalidServiceAuth  = "invalid_service_authentication"
	CodeUnsupportedMedia    = "unsupported_media_type"
	CodePayloadTooLarge     = "payload_too_large"
	CodeMethodNotAllowed    = "method_not_allowed"
	CodeTooManyRequestsText = "too many requests"
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
	case errors.Is(err, ErrNotFound):
		AbortWithError(c, http.StatusNotFound, CodeNotFound, "User not found.")
	case errors.Is(err, ErrEmailTaken):
		AbortWithError(c, http.StatusConflict, CodeConflict,
			"An account with this email address already exists.")
	case errors.Is(err, ErrInvalidTransition):
		AbortWithError(c, http.StatusConflict, CodeInvalidTransition,
			"The requested status change is not allowed for this user.")
	case errors.Is(err, ErrVersionConflict):
		AbortWithError(c, http.StatusConflict, CodeVersionConflict,
			"The user was modified by another request. Refresh and try again.")
	case errors.Is(err, ErrForbidden):
		AbortWithError(c, http.StatusForbidden, CodeForbidden,
			"You do not have permission to perform this action.")
	case errors.Is(err, ErrUnauthorized), errors.Is(err, ErrInvalidToken):
		AbortWithError(c, http.StatusUnauthorized, CodeUnauthorized, "Authentication is required.")
	case errors.Is(err, ErrTokenExpired):
		AbortWithError(c, http.StatusUnauthorized, CodeUnauthorized, "The access token has expired.")
	case errors.Is(err, ErrBootstrapDisabled):
		AbortWithError(c, http.StatusServiceUnavailable, CodeBootstrapDisabled,
			"Admin bootstrap is not configured on this service.")
	case errors.Is(err, ErrBootstrapForbidden):
		AbortWithError(c, http.StatusForbidden, CodeInvalidBootstrap, "Invalid bootstrap token.")
	case errors.Is(err, ErrPasswordTooWeak):
		AbortWithError(c, http.StatusBadRequest, CodePasswordPolicy,
			"The password does not meet the required policy.")
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

// BindJSON decodes a single JSON object from the request body and rejects
// unknown fields and trailing data. It writes an error response on failure.
func BindJSON(c *gin.Context, destination any) bool {
	if c.Request.Body == nil {
		AbortWithError(c, http.StatusBadRequest, CodeInvalidRequest, "A JSON request body is required.")
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
