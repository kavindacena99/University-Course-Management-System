package auth

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
	CodeInvalidCredentials = "invalid_credentials"
	CodeAccountLocked      = "account_locked"
	CodeAccountUnavailable = "account_unavailable"
	CodeAccountActivation  = "account_not_activated"
	CodeNotFound           = "not_found"
	CodeConflict           = "conflict"
	CodeLastAdmin          = "last_admin"
	CodePasswordPolicy     = "password_policy"
	CodePasswordReuse      = "password_reuse"
	CodeInvalidToken       = "invalid_token"
	CodeTokenExpired       = "token_expired"
	CodeRefreshInvalid     = "invalid_refresh_token"
	CodeRefreshReuse       = "refresh_token_reuse"
	CodeSessionExpired     = "session_expired"
	CodeActionTokenInvalid = "invalid_action_token"
	CodeActionTokenExpired = "expired_action_token"
	CodeActionTokenUsed    = "used_action_token"
	CodeRateLimited        = "rate_limited"
	CodeInternal           = "internal_error"
	CodeServiceAuth        = "invalid_service_authentication"
	CodeCSRFFailed         = "csrf_validation_failed"
	CodePayloadTooLarge    = "payload_too_large"
	CodeDisabledFeature    = "feature_disabled"
	CodeInvalidRole        = "invalid_role"
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
		AbortWithError(c, http.StatusNotFound, CodeNotFound, "Account not found.")
	case errors.Is(err, ErrEmailTaken):
		AbortWithError(c, http.StatusConflict, CodeConflict,
			"An account with this email address already exists.")
	case errors.Is(err, ErrInvalidCredentials):
		AbortWithError(c, http.StatusUnauthorized, CodeInvalidCredentials,
			"Invalid email or password.")
	case errors.Is(err, ErrAccountLocked):
		AbortWithError(c, http.StatusTooManyRequests, CodeAccountLocked,
			"Too many failed attempts. Try again later.")
	case errors.Is(err, ErrAccountNotActivated):
		AbortWithError(c, http.StatusForbidden, CodeAccountActivation,
			"The account has not been activated.")
	case errors.Is(err, ErrAccountUnavailable):
		AbortWithError(c, http.StatusForbidden, CodeAccountUnavailable,
			"The account is not available.")
	case errors.Is(err, ErrInvalidToken):
		AbortWithError(c, http.StatusUnauthorized, CodeInvalidToken,
			"The access token is invalid.")
	case errors.Is(err, ErrTokenExpired):
		AbortWithError(c, http.StatusUnauthorized, CodeTokenExpired,
			"The access token has expired.")
	case errors.Is(err, ErrRefreshInvalid):
		AbortWithError(c, http.StatusUnauthorized, CodeRefreshInvalid,
			"The refresh token is invalid.")
	case errors.Is(err, ErrRefreshReuse):
		AbortWithError(c, http.StatusUnauthorized, CodeRefreshReuse,
			"The refresh token has already been used.")
	case errors.Is(err, ErrSessionExpired):
		AbortWithError(c, http.StatusUnauthorized, CodeSessionExpired,
			"The session has expired.")
	case errors.Is(err, ErrActionTokenInvalid):
		AbortWithError(c, http.StatusBadRequest, CodeActionTokenInvalid,
			"The action token is invalid.")
	case errors.Is(err, ErrActionTokenExpired):
		AbortWithError(c, http.StatusGone, CodeActionTokenExpired,
			"The action token has expired.")
	case errors.Is(err, ErrActionTokenUsed):
		AbortWithError(c, http.StatusConflict, CodeActionTokenUsed,
			"The action token has already been used.")
	case errors.Is(err, ErrLastAdmin):
		AbortWithError(c, http.StatusConflict, CodeLastAdmin,
			"At least one administrator must remain.")
	case errors.Is(err, ErrPasswordReuse):
		AbortWithError(c, http.StatusBadRequest, CodePasswordReuse,
			"The new password must differ from the current password.")
	case errors.Is(err, ErrInvalidRole):
		AbortWithError(c, http.StatusBadRequest, CodeInvalidRole,
			"One or more roles are invalid.")
	case errors.Is(err, ErrUnauthorized), errors.Is(err, ErrForbidden):
		AbortWithError(c, http.StatusForbidden, CodeForbidden,
			"You do not have permission to perform this action.")
	case errors.Is(err, ErrDisabledFeature):
		AbortWithError(c, http.StatusServiceUnavailable, CodeDisabledFeature,
			"This feature is not configured on the service.")
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

func BindOptionalJSON(c *gin.Context, destination any) bool {
	if c.Request.Body == nil {
		return true
	}

	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(destination); err != nil {
		if errors.Is(err, io.EOF) {
			return true
		}
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			AbortWithError(c, http.StatusRequestEntityTooLarge, CodePayloadTooLarge,
				"The request body is too large.")
			return false
		}
		AbortWithError(c, http.StatusBadRequest, CodeInvalidRequest,
			"The JSON request body is invalid.")
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
