package user

import (
	"time"

	"github.com/google/uuid"
)

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

func (s RegistrationStatus) CanTransitionTo(next RegistrationStatus) bool {
	switch s {
	case StatusPending:
		return next == StatusApproved || next == StatusRejected
	case StatusApproved:
		return next == StatusSuspended
	case StatusSuspended:
		return next == StatusApproved
	default:
		return false
	}
}

type User struct {
	ID                 uuid.UUID          `json:"id"`
	Email              string             `json:"email"`
	FirstName          string             `json:"first_name"`
	LastName           string             `json:"last_name"`
	RegistrationStatus RegistrationStatus `json:"registration_status"`
	StatusReason       string             `json:"status_reason,omitempty"`
	ReviewedBy         *uuid.UUID         `json:"reviewed_by,omitempty"`
	ReviewedAt         *time.Time         `json:"reviewed_at,omitempty"`
	Version            int64              `json:"version"`
	CreatedAt          time.Time          `json:"created_at"`
	UpdatedAt          time.Time          `json:"updated_at"`
}

func (u *User) FullName() string {
	return u.FirstName + " " + u.LastName
}

type StatusHistory struct {
	ID         int64               `json:"id"`
	UserID     uuid.UUID           `json:"user_id"`
	FromStatus *RegistrationStatus `json:"from_status,omitempty"`
	ToStatus   RegistrationStatus  `json:"to_status"`
	Reason     string              `json:"reason,omitempty"`
	ChangedBy  *uuid.UUID          `json:"changed_by,omitempty"`
	Version    int64               `json:"version"`
	ChangedAt  time.Time           `json:"changed_at"`
}

type ListUsersFilter struct {
	Status RegistrationStatus
	Search string
	Limit  int
	Offset int
}

type UserList struct {
	Users  []User `json:"users"`
	Total  int64  `json:"total"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

const (
	EventUserApproved       = "user.approved"
	EventUserRejected       = "user.rejected"
	EventUserSuspended      = "user.suspended"
	EventUserReinstated     = "user.reinstated"
	EventUserProfileUpdated = "user.profile_updated"
)

func EventTypeForTransition(from, to RegistrationStatus) (string, bool) {
	switch {
	case from == StatusPending && to == StatusApproved:
		return EventUserApproved, true
	case from == StatusPending && to == StatusRejected:
		return EventUserRejected, true
	case from == StatusApproved && to == StatusSuspended:
		return EventUserSuspended, true
	case from == StatusSuspended && to == StatusApproved:
		return EventUserReinstated, true
	default:
		return "", false
	}
}

type UserEventPayload struct {
	UserID             uuid.UUID          `json:"user_id"`
	Email              string             `json:"email"`
	FirstName          string             `json:"first_name"`
	LastName           string             `json:"last_name"`
	RegistrationStatus RegistrationStatus `json:"registration_status"`
	Version            int64              `json:"version"`
}

func NewUserEventPayload(u *User) UserEventPayload {
	return UserEventPayload{
		UserID:             u.ID,
		Email:              u.Email,
		FirstName:          u.FirstName,
		LastName:           u.LastName,
		RegistrationStatus: u.RegistrationStatus,
		Version:            u.Version,
	}
}

type UserEvent struct {
	EventID    uuid.UUID        `json:"event_id"`
	EventType  string           `json:"event_type"`
	OccurredAt time.Time        `json:"occurred_at"`
	User       UserEventPayload `json:"user"`
}

const (
	OutboxStatusPending   = "pending"
	OutboxStatusDelivered = "delivered"
	OutboxStatusDead      = "dead"
)

type OutboxEvent struct {
	ID         uuid.UUID
	EventType  string
	Payload    []byte
	Attempts   int
	CreatedAt  time.Time
	NextAction time.Time
}

type BootstrapUser struct {
	Email     string
	FirstName string
	LastName  string
	Password  string
}
