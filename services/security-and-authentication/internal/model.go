package auth

import (
	"time"

	"github.com/google/uuid"
)

const (
	RoleAdmin    = "ADMIN"
	RoleStaff    = "STAFF"
	RoleLecturer = "LECTURER"
	RoleStudent  = "STUDENT"
)

var AllRoles = []string{RoleAdmin, RoleStaff, RoleLecturer, RoleStudent}

func IsValidRole(role string) bool {
	for _, candidate := range AllRoles {
		if role == candidate {
			return true
		}
	}
	return false
}

type RegistrationStatus string

const (
	StatusPending   RegistrationStatus = "pending"
	StatusApproved  RegistrationStatus = "approved"
	StatusRejected  RegistrationStatus = "rejected"
	StatusSuspended RegistrationStatus = "suspended"
)

func (s RegistrationStatus) Valid() bool {
	switch s {
	case StatusPending, StatusApproved, StatusRejected, StatusSuspended:
		return true
	default:
		return false
	}
}

type AuthUser struct {
	UserID              uuid.UUID
	Email               string
	FirstName           string
	LastName            string
	RegistrationStatus  RegistrationStatus
	UserVersion         int64
	PasswordHash        string
	PasswordUpdatedAt   *time.Time
	ActivatedAt         *time.Time
	Disabled            bool
	FailedLoginAttempts int
	LockedUntil         *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
	Roles               []string
}

func (u *AuthUser) IsActive() bool {
	return u.RegistrationStatus == StatusApproved &&
		!u.Disabled &&
		u.PasswordHash != "" &&
		u.ActivatedAt != nil
}

func (u *AuthUser) IsLocked(now time.Time) bool {
	return u.LockedUntil != nil && u.LockedUntil.After(now)
}

func (u *AuthUser) HasRole(role string) bool {
	for _, candidate := range u.Roles {
		if candidate == role {
			return true
		}
	}
	return false
}

func (u *AuthUser) Summary() UserSummary {
	return UserSummary{
		ID:                 u.UserID,
		Email:              u.Email,
		FirstName:          u.FirstName,
		LastName:           u.LastName,
		RegistrationStatus: u.RegistrationStatus,
		Roles:              append([]string(nil), u.Roles...),
	}
}

type UserSummary struct {
	ID                 uuid.UUID          `json:"id"`
	Email              string             `json:"email"`
	FirstName          string             `json:"first_name"`
	LastName           string             `json:"last_name"`
	RegistrationStatus RegistrationStatus `json:"registration_status"`
	Roles              []string           `json:"roles"`
}

type Session struct {
	ID                uuid.UUID  `json:"id"`
	UserID            uuid.UUID  `json:"user_id"`
	CreatedAt         time.Time  `json:"created_at"`
	LastUsedAt        time.Time  `json:"last_used_at"`
	AbsoluteExpiresAt time.Time  `json:"absolute_expires_at"`
	RevokedAt         *time.Time `json:"revoked_at,omitempty"`
	UserAgent         string     `json:"user_agent,omitempty"`
	IPAddress         string     `json:"ip_address,omitempty"`
	Current           bool       `json:"current"`
}

const (
	EventUserApproved       = "user.approved"
	EventUserRejected       = "user.rejected"
	EventUserSuspended      = "user.suspended"
	EventUserReinstated     = "user.reinstated"
	EventUserProfileUpdated = "user.profile_updated"
)

type UserEventPayload struct {
	UserID             uuid.UUID          `json:"user_id"`
	Email              string             `json:"email"`
	FirstName          string             `json:"first_name"`
	LastName           string             `json:"last_name"`
	RegistrationStatus RegistrationStatus `json:"registration_status"`
	Version            int64              `json:"version"`
}

type UserEvent struct {
	EventID    uuid.UUID        `json:"event_id"`
	EventType  string           `json:"event_type"`
	OccurredAt time.Time        `json:"occurred_at"`
	User       UserEventPayload `json:"user"`
}

const (
	ActionTokenActivation    = "activation"
	ActionTokenPasswordReset = "password_reset"
)

type ActionToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash []byte
	Purpose   string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

type Role struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

type LoginResult struct {
	User          *AuthUser
	AccessToken   string
	RefreshToken  string
	SessionID     uuid.UUID
	AccessExpiry  time.Time
	RefreshExpiry time.Time
	CSRFToken     string
}

type RefreshResult struct {
	User          *AuthUser
	AccessToken   string
	RefreshToken  string
	SessionID     uuid.UUID
	AccessExpiry  time.Time
	RefreshExpiry time.Time
	CSRFToken     string
}

type ProvisionUserInput struct {
	UserID    uuid.UUID
	Email     string
	FirstName string
	LastName  string
	Version   int64
	Password  string
	Roles     []string
}

type InboxResult struct {
	Duplicate bool
	Stale     bool
}
