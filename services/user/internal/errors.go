package user

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound            = errors.New("user not found")
	ErrEmailTaken          = errors.New("email is already registered")
	ErrInvalidTransition   = errors.New("invalid registration status transition")
	ErrVersionConflict     = errors.New("user was modified by another request")
	ErrUnauthorized        = errors.New("unauthorized")
	ErrForbidden           = errors.New("forbidden")
	ErrInvalidToken        = errors.New("invalid access token")
	ErrTokenExpired        = errors.New("access token expired")
	ErrInvalidServiceAuth  = errors.New("invalid internal service authentication")
	ErrBootstrapDisabled   = errors.New("admin bootstrap is not configured")
	ErrBootstrapForbidden  = errors.New("invalid admin bootstrap token")
	ErrPasswordTooWeak     = errors.New("password does not meet the required policy")
	ErrInvalidStatusChange = errors.New("invalid status change request")
)

// ValidationError describes an invalid field supplied by a client.
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
