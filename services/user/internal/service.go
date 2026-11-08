package user

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

type UserStore interface {
	Ping(ctx context.Context) error
	CreatePending(ctx context.Context, u *User) error
	CreateApproved(ctx context.Context, u *User, reason string, reviewedBy *uuid.UUID) error
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	UpdateProfile(ctx context.Context, id uuid.UUID, expectedVersion int64, firstName, lastName string) (*User, error)
	ChangeStatus(ctx context.Context, input ChangeStatusInput) (*User, error)
	ListUsers(ctx context.Context, filter ListUsersFilter) (*UserList, error)
	GetHistory(ctx context.Context, userID uuid.UUID, limit, offset int) ([]StatusHistory, int64, error)
}

type AuthProvisioner interface {
	ProvisionAdmin(ctx context.Context, u *User, password string) error
}

type Service struct {
	store       UserStore
	provisioner AuthProvisioner
}

func NewService(store UserStore, provisioner AuthProvisioner) *Service {
	return &Service{store: store, provisioner: provisioner}
}

type RegisterInput struct {
	Email     string
	FirstName string
	LastName  string
}

func (s *Service) Register(ctx context.Context, input RegisterInput) (*User, error) {
	email, err := validateEmail(input.Email)
	if err != nil {
		return nil, err
	}
	firstName, err := validateName("first_name", input.FirstName)
	if err != nil {
		return nil, err
	}
	lastName, err := validateName("last_name", input.LastName)
	if err != nil {
		return nil, err
	}

	u := &User{
		Email:              email,
		FirstName:          firstName,
		LastName:           lastName,
		RegistrationStatus: StatusPending,
	}

	if err := s.store.CreatePending(ctx, u); err != nil {
		return nil, err
	}

	return u, nil
}

func (s *Service) GetUser(
	ctx context.Context,
	claims *AccessClaims,
	userID uuid.UUID,
) (*User, error) {
	if err := s.authorizeUserRead(claims, userID); err != nil {
		return nil, err
	}
	return s.store.GetByID(ctx, userID)
}

func (s *Service) InternalGetUser(ctx context.Context, userID uuid.UUID) (*User, error) {
	return s.store.GetByID(ctx, userID)
}

func (s *Service) GetMe(ctx context.Context, claims *AccessClaims) (*User, error) {
	if claims == nil {
		return nil, ErrUnauthorized
	}
	userID, err := claims.UserID()
	if err != nil {
		return nil, ErrUnauthorized
	}
	return s.store.GetByID(ctx, userID)
}

type UpdateProfileInput struct {
	Version   int64
	FirstName string
	LastName  string
}

func (s *Service) UpdateMe(
	ctx context.Context,
	claims *AccessClaims,
	input UpdateProfileInput,
) (*User, error) {
	if claims == nil {
		return nil, ErrUnauthorized
	}
	userID, err := claims.UserID()
	if err != nil {
		return nil, ErrUnauthorized
	}
	if input.Version <= 0 {
		return nil, newValidationError("version", "must be provided and positive")
	}
	firstName, err := validateName("first_name", input.FirstName)
	if err != nil {
		return nil, err
	}
	lastName, err := validateName("last_name", input.LastName)
	if err != nil {
		return nil, err
	}

	return s.store.UpdateProfile(ctx, userID, input.Version, firstName, lastName)
}

func (s *Service) ListUsers(
	ctx context.Context,
	filter ListUsersFilter,
) (*UserList, error) {
	if filter.Status != "" && !filter.Status.Valid() {
		return nil, newValidationError("registration_status", "is not a valid status")
	}
	if utf8.RuneCountInString(filter.Search) > 100 {
		return nil, newValidationError("search", "must not exceed 100 characters")
	}

	filter.Limit = clamp(filter.Limit, 1, 100, 20)
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	if filter.Offset > 1_000_000 {
		return nil, newValidationError("offset", "is too large")
	}

	return s.store.ListUsers(ctx, filter)
}

func (s *Service) GetHistory(
	ctx context.Context,
	userID uuid.UUID,
	limit int,
	offset int,
) ([]StatusHistory, int64, error) {
	if _, err := s.store.GetByID(ctx, userID); err != nil {
		return nil, 0, err
	}
	limit = clamp(limit, 1, 200, 50)
	if offset < 0 {
		offset = 0
	}
	return s.store.GetHistory(ctx, userID, limit, offset)
}

type StatusAction string

const (
	ActionApprove   StatusAction = "approve"
	ActionReject    StatusAction = "reject"
	ActionSuspend   StatusAction = "suspend"
	ActionReinstate StatusAction = "reinstate"
)

type ChangeStatusRequest struct {
	UserID          uuid.UUID
	Action          StatusAction
	Reason          string
	ExpectedVersion int64
}

func (s *Service) ChangeStatus(
	ctx context.Context,
	claims *AccessClaims,
	request ChangeStatusRequest,
) (*User, error) {
	if claims == nil {
		return nil, ErrUnauthorized
	}
	adminID, err := claims.UserID()
	if err != nil {
		return nil, ErrUnauthorized
	}
	if request.ExpectedVersion <= 0 {
		return nil, newValidationError("version", "must be provided and positive")
	}

	reason := strings.TrimSpace(request.Reason)
	if utf8.RuneCountInString(reason) > 500 {
		return nil, newValidationError("reason", "must not exceed 500 characters")
	}

	toStatus, err := statusForAction(request.Action)
	if err != nil {
		return nil, err
	}
	if (request.Action == ActionReject || request.Action == ActionSuspend) && reason == "" {
		return nil, newValidationError("reason", "is required for this action")
	}

	target, err := s.store.GetByID(ctx, request.UserID)
	if err != nil {
		return nil, err
	}
	if target.Version != request.ExpectedVersion {
		return nil, ErrVersionConflict
	}
	if !target.RegistrationStatus.CanTransitionTo(toStatus) {
		return nil, ErrInvalidTransition
	}

	eventType, ok := EventTypeForTransition(target.RegistrationStatus, toStatus)
	if !ok {
		return nil, ErrInvalidTransition
	}

	return s.store.ChangeStatus(ctx, ChangeStatusInput{
		UserID:          target.ID,
		ExpectedVersion: request.ExpectedVersion,
		FromStatus:      target.RegistrationStatus,
		ToStatus:        toStatus,
		Reason:          reason,
		ChangedBy:       adminID,
		EventType:       eventType,
	})
}

type Eligibility struct {
	UserID             uuid.UUID          `json:"user_id"`
	Eligible           bool               `json:"eligible"`
	RegistrationStatus RegistrationStatus `json:"registration_status"`
	StatusReason       string             `json:"status_reason,omitempty"`
	Version            int64              `json:"version"`
}

func (s *Service) Eligibility(ctx context.Context, userID uuid.UUID) (*Eligibility, error) {
	u, err := s.store.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &Eligibility{
		UserID:             u.ID,
		Eligible:           u.RegistrationStatus == StatusApproved,
		RegistrationStatus: u.RegistrationStatus,
		StatusReason:       u.StatusReason,
		Version:            u.Version,
	}, nil
}

type BootstrapInput struct {
	Email     string
	FirstName string
	LastName  string
	Password  string
}

func (s *Service) BootstrapAdmin(ctx context.Context, input BootstrapInput) (*User, error) {
	email, err := validateEmail(input.Email)
	if err != nil {
		return nil, err
	}
	firstName, err := validateName("first_name", input.FirstName)
	if err != nil {
		return nil, err
	}
	lastName, err := validateName("last_name", input.LastName)
	if err != nil {
		return nil, err
	}
	if err := validatePassword(input.Password); err != nil {
		return nil, err
	}
	if s.provisioner == nil {
		return nil, ErrBootstrapDisabled
	}

	u := &User{
		Email:     email,
		FirstName: firstName,
		LastName:  lastName,
	}

	existing, err := s.store.GetByEmail(ctx, email)
	switch {
	case err == nil:
		if existing.RegistrationStatus != StatusApproved {
			return nil, fmt.Errorf("%w: the existing account is not approved",
				ErrInvalidTransition)
		}
		u = existing
	case err != nil && !isNotFound(err):
		return nil, err
	default:
		u.ID = uuid.New()
		u.RegistrationStatus = StatusApproved
		if err := s.store.CreateApproved(
			ctx, u, "initial administrator bootstrap", nil,
		); err != nil {
			return nil, err
		}
	}

	if err := s.provisioner.ProvisionAdmin(ctx, u, input.Password); err != nil {
		return nil, fmt.Errorf("provision administrator credentials: %w", err)
	}

	return u, nil
}

func (s *Service) authorizeUserRead(claims *AccessClaims, userID uuid.UUID) error {
	if claims == nil {
		return ErrUnauthorized
	}
	if HasAnyRole(claims, RoleAdmin, RoleStaff) {
		return nil
	}
	ownID, err := claims.UserID()
	if err != nil {
		return ErrUnauthorized
	}
	if ownID != userID {
		return ErrForbidden
	}
	return nil
}

func statusForAction(action StatusAction) (RegistrationStatus, error) {
	switch action {
	case ActionApprove:
		return StatusApproved, nil
	case ActionReject:
		return StatusRejected, nil
	case ActionSuspend:
		return StatusSuspended, nil
	case ActionReinstate:
		return StatusApproved, nil
	default:
		return "", newValidationError("action", "is not supported")
	}
}

func validateEmail(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", newValidationError("email", "is required")
	}
	if len(value) > 254 {
		return "", newValidationError("email", "must not exceed 254 characters")
	}
	if strings.ContainsAny(value, " \t\r\n") {
		return "", newValidationError("email", "must not contain whitespace")
	}

	address, err := mail.ParseAddress(value)
	if err != nil || !strings.EqualFold(address.Address, value) {
		return "", newValidationError("email", "must be a valid email address")
	}
	if !strings.Contains(address.Address, "@") {
		return "", newValidationError("email", "must be a valid email address")
	}

	return strings.ToLower(address.Address), nil
}

func validateName(field, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", newValidationError(field, "is required")
	}
	if utf8.RuneCountInString(value) > 100 {
		return "", newValidationError(field, "must not exceed 100 characters")
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return "", newValidationError(field, "must not contain control characters")
		}
	}
	return value, nil
}

func validatePassword(password string) error {
	if len(password) < 12 || len(password) > 128 {
		return newValidationError("password",
			"must be between 12 and 128 characters")
	}

	var hasLower, hasUpper, hasDigit, hasSymbol bool
	for _, r := range password {
		switch {
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsSpace(r):
			return newValidationError("password", "must not contain spaces")
		default:
			hasSymbol = true
		}
	}

	if !hasLower || !hasUpper || !hasDigit || !hasSymbol {
		return newValidationError("password",
			"must include lowercase, uppercase, digit, and symbol characters")
	}
	return nil
}

func clamp(value, minValue, maxValue, fallback int) int {
	if value == 0 {
		return fallback
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func isNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}
