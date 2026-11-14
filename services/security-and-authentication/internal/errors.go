package auth

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound               = errors.New("account not found")
	ErrEmailTaken             = errors.New("email is already registered")
	ErrInvalidCredentials     = errors.New("invalid email or password")
	ErrAccountLocked          = errors.New("account is temporarily locked")
	ErrAccountUnavailable     = errors.New("account is not available")
	ErrAccountNotActivated    = errors.New("account has not been activated")
	ErrInvalidToken           = errors.New("invalid access token")
	ErrTokenExpired           = errors.New("access token expired")
	ErrRefreshInvalid         = errors.New("invalid refresh token")
	ErrRefreshReuse           = errors.New("refresh token reuse detected")
	ErrSessionExpired         = errors.New("session expired")
	ErrActionTokenInvalid     = errors.New("invalid action token")
	ErrActionTokenExpired     = errors.New("action token expired")
	ErrActionTokenUsed        = errors.New("action token already used")
	ErrUnauthorized           = errors.New("unauthorized")
	ErrForbidden              = errors.New("forbidden")
	ErrInvalidServiceAuth     = errors.New("invalid internal service authentication")
	ErrConflict               = errors.New("conflict")
	ErrLastAdmin              = errors.New("cannot remove the last administrator")
	ErrPasswordPolicy         = errors.New("password does not meet the required policy")
	ErrPasswordReuse          = errors.New("new password must differ from the current password")
	ErrInvalidRole            = errors.New("invalid role")
	ErrInvalidPermission      = errors.New("invalid permission")
	ErrDisabledFeature        = errors.New("feature is not configured")
	ErrUserIneligible         = errors.New("user is not eligible for authentication")
	ErrEligibilityUnavailable = errors.New("user eligibility cannot be verified")
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
