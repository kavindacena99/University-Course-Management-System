package enrollment

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound              = errors.New("enrollment not found")
	ErrDuplicateActive       = errors.New("student is already actively enrolled")
	ErrCapacityReached       = errors.New("offering capacity reached")
	ErrVersionConflict       = errors.New("enrollment version conflict")
	ErrInvalidTransition     = errors.New("invalid enrollment transition")
	ErrForbidden             = errors.New("forbidden")
	ErrUnauthorized          = errors.New("unauthorized")
	ErrInvalidToken          = errors.New("invalid access token")
	ErrTokenExpired          = errors.New("access token expired")
	ErrInvalidServiceAuth    = errors.New("invalid internal service authentication")
	ErrStudentIneligible     = errors.New("student is not eligible")
	ErrOfferingIneligible    = errors.New("offering is not eligible for enrollment")
	ErrDependencyUnavailable = errors.New("dependency is unavailable")
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
