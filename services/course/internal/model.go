package course

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	RoleAdmin    = "ADMIN"
	RoleStaff    = "STAFF"
	RoleLecturer = "LECTURER"
	RoleStudent  = "STUDENT"

	PermCoursesManage   = "courses:manage"
	PermOfferingsManage = "offerings:manage"
)

type CourseStatus string

const (
	CourseDraft    CourseStatus = "draft"
	CourseActive   CourseStatus = "active"
	CourseArchived CourseStatus = "archived"
)

func (s CourseStatus) Valid() bool {
	switch s {
	case CourseDraft, CourseActive, CourseArchived:
		return true
	default:
		return false
	}
}

func (s CourseStatus) CanTransitionTo(next CourseStatus) bool {
	switch s {
	case CourseDraft:
		return next == CourseActive || next == CourseArchived
	case CourseActive:
		return next == CourseArchived
	case CourseArchived:
		return next == CourseActive
	default:
		return false
	}
}

type OfferingStatus string

const (
	OfferingDraft     OfferingStatus = "draft"
	OfferingPublished OfferingStatus = "published"
	OfferingClosed    OfferingStatus = "closed"
	OfferingArchived  OfferingStatus = "archived"
)

func (s OfferingStatus) Valid() bool {
	switch s {
	case OfferingDraft, OfferingPublished, OfferingClosed, OfferingArchived:
		return true
	default:
		return false
	}
}

func (s OfferingStatus) CanTransitionTo(next OfferingStatus) bool {
	switch s {
	case OfferingDraft:
		return next == OfferingPublished || next == OfferingArchived
	case OfferingPublished:
		return next == OfferingClosed || next == OfferingArchived
	case OfferingClosed:
		return next == OfferingArchived
	case OfferingArchived:
		return next == OfferingDraft
	default:
		return false
	}
}

// Date is a calendar date serialized as YYYY-MM-DD.
type Date struct {
	time.Time
}

func NewDate(value time.Time) Date {
	return Date{Time: time.Date(
		value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC,
	)}
}

func ParseDate(value string) (Date, error) {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return Date{}, err
	}
	return NewDate(parsed), nil
}

func (d Date) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Format("2006-01-02"))
}

func (d *Date) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	parsed, err := ParseDate(value)
	if err != nil {
		return fmt.Errorf("date must use YYYY-MM-DD format")
	}
	*d = parsed
	return nil
}

func (d Date) String() string {
	return d.Format("2006-01-02")
}

type Course struct {
	ID          uuid.UUID    `json:"id"`
	Code        string       `json:"code"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Credits     int          `json:"credits"`
	Status      CourseStatus `json:"status"`
	Version     int64        `json:"version"`
	CreatedBy   uuid.UUID    `json:"created_by"`
	UpdatedBy   uuid.UUID    `json:"updated_by"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	ArchivedAt  *time.Time   `json:"archived_at,omitempty"`
}

type CourseOffering struct {
	ID                 uuid.UUID      `json:"id"`
	CourseID           uuid.UUID      `json:"course_id"`
	CourseCode         string         `json:"course_code"`
	CourseTitle        string         `json:"course_title"`
	AcademicYear       string         `json:"academic_year"`
	Semester           string         `json:"semester"`
	StartDate          Date           `json:"start_date"`
	EndDate            Date           `json:"end_date"`
	EnrollmentOpensAt  *time.Time     `json:"enrollment_opens_at,omitempty"`
	EnrollmentClosesAt *time.Time     `json:"enrollment_closes_at,omitempty"`
	EnrollmentCapacity *int           `json:"enrollment_capacity,omitempty"`
	LecturerUserID     *uuid.UUID     `json:"lecturer_user_id,omitempty"`
	LecturerAssignedBy *uuid.UUID     `json:"lecturer_assigned_by,omitempty"`
	LecturerAssignedAt *time.Time     `json:"lecturer_assigned_at,omitempty"`
	Status             OfferingStatus `json:"status"`
	Version            int64          `json:"version"`
	CreatedBy          uuid.UUID      `json:"created_by"`
	UpdatedBy          uuid.UUID      `json:"updated_by"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	ArchivedAt         *time.Time     `json:"archived_at,omitempty"`
}

type CourseFilter struct {
	Status CourseStatus
	Search string
	Limit  int
	Offset int
	Sort   string
	Order  string
}

type CourseList struct {
	Courses []Course `json:"courses"`
	Total   int64    `json:"total"`
	Limit   int      `json:"limit"`
	Offset  int      `json:"offset"`
}

type OfferingFilter struct {
	CourseID         *uuid.UUID
	AcademicYear     string
	Semester         string
	Status           OfferingStatus
	LecturerID       *uuid.UUID
	AssignedToMe     bool
	AssignedToUserID *uuid.UUID
	Search           string
	PublishedOnly    bool
	Limit            int
	Offset           int
	Sort             string
	Order            string
}

type OfferingList struct {
	Offerings []CourseOffering `json:"offerings"`
	Total     int64            `json:"total"`
	Limit     int              `json:"limit"`
	Offset    int              `json:"offset"`
}

func courseTransitionForAction(action string) (CourseStatus, bool) {
	switch action {
	case "publish":
		return CourseActive, true
	case "archive":
		return CourseArchived, true
	case "restore":
		return CourseActive, true
	default:
		return "", false
	}
}

func offeringTransitionForAction(action string) (OfferingStatus, bool) {
	switch action {
	case "publish":
		return OfferingPublished, true
	case "close":
		return OfferingClosed, true
	case "archive":
		return OfferingArchived, true
	case "restore":
		return OfferingDraft, true
	default:
		return "", false
	}
}
