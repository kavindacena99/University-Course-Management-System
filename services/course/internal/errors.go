package course

import (
	"errors"
	"fmt"
)

var (
	ErrCourseNotFound           = errors.New("course not found")
	ErrOfferingNotFound         = errors.New("course offering not found")
	ErrCourseCodeTaken          = errors.New("course code is already in use")
	ErrCourseCodeLocked         = errors.New("course code cannot be changed after offerings exist")
	ErrCourseArchived           = errors.New("archived course cannot be modified")
	ErrCourseNotActive          = errors.New("parent course is not active")
	ErrCourseHasActiveOfferings = errors.New("course has draft or published offerings")
	ErrOfferingImmutable        = errors.New("course offering is closed or archived")
	ErrFieldLocked              = errors.New("field cannot be changed after publication")
	ErrInvalidTransition        = errors.New("invalid lifecycle transition")
	ErrVersionConflict          = errors.New("resource was modified by another request")
	ErrUnauthorized             = errors.New("unauthorized")
	ErrForbidden                = errors.New("forbidden")
	ErrInvalidToken             = errors.New("invalid access token")
	ErrTokenExpired             = errors.New("access token expired")
	ErrInvalidServiceAuth       = errors.New("invalid internal service authentication")
	ErrLecturerNotFound         = errors.New("lecturer user not found")
	ErrLecturerIneligible       = errors.New("lecturer user is not eligible")
	ErrUserServiceUnavailable   = errors.New("user service is unavailable")
	ErrAuthServiceUnavailable   = errors.New("auth service is unavailable")
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
