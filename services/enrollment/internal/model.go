package enrollment

import (
	"time"

	"github.com/google/uuid"
)

type EnrollmentStatus string

const (
	EnrollmentActive    EnrollmentStatus = "active"
	EnrollmentCancelled EnrollmentStatus = "cancelled"
)

func (s EnrollmentStatus) Valid() bool {
	return s == EnrollmentActive || s == EnrollmentCancelled
}

func (s EnrollmentStatus) CanTransitionTo(next EnrollmentStatus) bool {
	return s == EnrollmentActive && next == EnrollmentCancelled
}

// Enrollment references a CourseOffering and User by their external UUIDs.
// Neither external entity is stored or foreign-keyed in this database.
type Enrollment struct {
	ID              uuid.UUID        `json:"id"`
	StudentUserID   uuid.UUID        `json:"student_user_id"`
	OfferingID      uuid.UUID        `json:"offering_id"`
	Status          EnrollmentStatus `json:"status"`
	Version         int64            `json:"version"`
	EnrolledAt      time.Time        `json:"enrolled_at"`
	EndedAt         *time.Time       `json:"ended_at,omitempty"`
	CreatedByUserID uuid.UUID        `json:"created_by_user_id"`
	EndedByUserID   *uuid.UUID       `json:"ended_by_user_id,omitempty"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
}

type EnrollmentFilter struct {
	StudentUserID *uuid.UUID
	OfferingID    *uuid.UUID
	Status        EnrollmentStatus
	Limit         int
	Offset        int
}

type EnrollmentList struct {
	Enrollments []Enrollment `json:"enrollments"`
	Total       int64        `json:"total"`
	Limit       int          `json:"limit"`
	Offset      int          `json:"offset"`
}
