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

type User struct {
	ID                 uuid.UUID          `json:"id"`
	Email              string             `json:"email"`
	FirstName          string             `json:"first_name"`
	LastName           string             `json:"last_name"`
	RegistrationStatus RegistrationStatus `json:"registration_status"`
	CreatedAt          time.Time          `json:"created_at"`
	UpdatedAt          time.Time          `json:"updated_at"`
}
