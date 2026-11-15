package assessment

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound              = errors.New("assessment resource not found")
	ErrForbidden             = errors.New("forbidden")
	ErrUnauthorized          = errors.New("unauthorized")
	ErrInvalidToken          = errors.New("invalid access token")
	ErrTokenExpired          = errors.New("access token expired")
	ErrInvalidServiceAuth    = errors.New("invalid internal service authentication")
	ErrDependencyUnavailable = errors.New("dependency is unavailable")
	ErrStudentIneligible     = errors.New("student is not eligible")
	ErrVersionConflict       = errors.New("version conflict")
	ErrInvalidTransition     = errors.New("invalid status transition")
	ErrAttemptLimitReached   = errors.New("assessment attempt limit reached")
	ErrIdempotencyConflict   = errors.New("idempotency key conflict")
	ErrSubmissionClosed      = errors.New("submission window is closed")
)

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

func newValidationError(field, message string) error {
	return &ValidationError{Field: field, Message: message}
}
