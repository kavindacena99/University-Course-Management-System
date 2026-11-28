package analytics

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidEvent          = errors.New("invalid Analytics event")
	ErrValidation            = errors.New("validation error")
	ErrProjectionUnavailable = errors.New("Analytics projection is unavailable")
	ErrDependencyUnavailable = errors.New("Course service is unavailable")
)

const (
	RoleAdmin    = "ADMIN"
	RoleLecturer = "LECTURER"
	RoleStudent  = "STUDENT"

	PermissionStudentRead      = "analytics:student:read"
	PermissionLecturerRead     = "analytics:lecturer:read"
	PermissionAdminRead        = "analytics:admin:read"
	PermissionAdminStudentRead = "analytics:admin:student:read"
	PermissionManage           = "analytics:manage"
)

type EventEnvelope struct {
	EventID          uuid.UUID `json:"event_id"`
	EventType        string    `json:"event_type"`
	SchemaVersion    int       `json:"schema_version"`
	OccurredAt       time.Time `json:"occurred_at"`
	Producer         string    `json:"producer_service"`
	AggregateID      uuid.UUID `json:"aggregate_id"`
	AggregateVersion int64     `json:"aggregate_version"`
	Version          int64     `json:"version"`
	Payload          []byte    `json:"-"`
	GradeID          uuid.UUID `json:"grade_id"`
	ResultID         uuid.UUID `json:"result_id"`
	SubmissionID     uuid.UUID `json:"submission_id"`
	AssessmentID     uuid.UUID `json:"assessment_id"`
	OfferingID       uuid.UUID `json:"offering_id"`
	StudentUserID    uuid.UUID `json:"student_user_id"`
	AttemptNo        int       `json:"attempt_no"`
	ComponentKind    string    `json:"component_kind"`
	Mark             float64   `json:"mark"`
	MaxMark          float64   `json:"max_mark"`
	WeightPercent    float64   `json:"weight_percent"`
	FinalMark        float64   `json:"final_mark"`
	ImportBatchID    uuid.UUID `json:"import_batch_id"`
	ImportedAt       time.Time `json:"imported_at"`
	ReleasedAt       time.Time `json:"released_at"`
}

func (e EventEnvelope) version() int64 {
	if e.AggregateVersion != 0 {
		return e.AggregateVersion
	}
	return e.Version
}

func (e EventEnvelope) validate() error {
	if e.EventID == uuid.Nil || e.Producer != "assessment-service" ||
		e.SchemaVersion != 1 || e.AggregateID == uuid.Nil || e.version() <= 0 {
		return fmt.Errorf("%w: event metadata is invalid", ErrInvalidEvent)
	}
	switch e.EventType {
	case "GradePublished":
		if e.GradeID == uuid.Nil || e.SubmissionID == uuid.Nil ||
			e.AssessmentID == uuid.Nil || e.OfferingID == uuid.Nil ||
			e.StudentUserID == uuid.Nil || e.AttemptNo <= 0 ||
			e.ComponentKind == "" || e.MaxMark <= 0 || e.Mark < 0 ||
			e.Mark > e.MaxMark || e.WeightPercent <= 0 ||
			e.WeightPercent > 100 || e.ReleasedAt.IsZero() {
			return fmt.Errorf("%w: published grade payload is invalid", ErrInvalidEvent)
		}
	case "FinalResultImported":
		if e.ResultID == uuid.Nil || e.OfferingID == uuid.Nil ||
			e.StudentUserID == uuid.Nil || e.MaxMark <= 0 ||
			e.FinalMark < 0 || e.FinalMark > e.MaxMark ||
			e.ImportedAt.IsZero() {
			return fmt.Errorf("%w: final result payload is invalid", ErrInvalidEvent)
		}
	default:
		return fmt.Errorf("%w: unsupported event type", ErrInvalidEvent)
	}
	return nil
}

type ComponentGrade struct {
	GradeID       uuid.UUID `json:"grade_id"`
	SubmissionID  uuid.UUID `json:"submission_id"`
	AssessmentID  uuid.UUID `json:"assessment_id"`
	OfferingID    uuid.UUID `json:"offering_id"`
	StudentUserID uuid.UUID `json:"student_user_id"`
	AttemptNo     int       `json:"attempt_no"`
	ComponentKind string    `json:"component_kind"`
	Mark          float64   `json:"mark"`
	MaxMark       float64   `json:"max_mark"`
	WeightPercent float64   `json:"weight_percent"`
	ReleasedAt    time.Time `json:"released_at"`
}

type FinalResult struct {
	ResultID      uuid.UUID `json:"result_id"`
	OfferingID    uuid.UUID `json:"offering_id"`
	StudentUserID uuid.UUID `json:"student_user_id"`
	FinalMark     float64   `json:"final_mark"`
	MaxMark       float64   `json:"max_mark"`
	ImportedAt    time.Time `json:"imported_at"`
}

type GradeBand struct {
	Code           string  `json:"grade_code"`
	MinimumPercent float64 `json:"minimum_percent"`
	MaximumPercent float64 `json:"maximum_percent"`
	GradePoints    float64 `json:"grade_points"`
	DisplayOrder   int     `json:"display_order"`
}

func validateGradeBands(bands []GradeBand) error {
	if len(bands) > 20 {
		return fmt.Errorf("%w: grade_bands must contain no more than 20 bands", ErrValidation)
	}
	ordered := append([]GradeBand(nil), bands...)
	for _, band := range ordered {
		if band.Code == "" || strings.TrimSpace(band.Code) != band.Code || len(band.Code) > 12 ||
			band.MinimumPercent < 0 || band.MaximumPercent > 100 ||
			band.MaximumPercent <= band.MinimumPercent ||
			band.GradePoints < 0 || band.GradePoints > 4 ||
			band.DisplayOrder <= 0 ||
			!hasTwoDecimalPlaces(band.MinimumPercent) ||
			!hasTwoDecimalPlaces(band.MaximumPercent) ||
			!hasTwoDecimalPlaces(band.GradePoints) {
			return fmt.Errorf("%w: grade band values are invalid", ErrValidation)
		}
	}
	for i := 0; i < len(ordered); i++ {
		for j := i + 1; j < len(ordered); j++ {
			if ordered[i].Code == ordered[j].Code ||
				ordered[i].DisplayOrder == ordered[j].DisplayOrder {
				return fmt.Errorf("%w: grade codes and display orders must be unique", ErrValidation)
			}
			if ordered[i].MinimumPercent < ordered[j].MaximumPercent &&
				ordered[j].MinimumPercent < ordered[i].MaximumPercent {
				return fmt.Errorf("%w: grade bands must not overlap", ErrValidation)
			}
		}
	}
	return nil
}

func hasTwoDecimalPlaces(value float64) bool {
	return math.Abs(value*100-math.Round(value*100)) < 1e-8
}

func (b GradeBand) matches(percent float64) bool {
	return percent >= b.MinimumPercent &&
		(percent < b.MaximumPercent ||
			(b.MaximumPercent == 100 && percent <= 100))
}

func (b GradeBand) gpaValue() float64 {
	return math.Round(b.GradePoints*100) / 100
}

type Programme struct {
	ID                 uuid.UUID `json:"id"`
	Code               string    `json:"code"`
	Name               string    `json:"name"`
	Category           string    `json:"category"`
	RepeatPolicy       string    `json:"repeat_policy"`
	FailedResultPolicy string    `json:"failed_result_policy"`
	PassGradePoints    float64   `json:"pass_grade_points"`
	Active             bool      `json:"active"`
}

type CreateProgrammeInput struct {
	Code               string  `json:"code"`
	Name               string  `json:"name"`
	Category           string  `json:"category"`
	RepeatPolicy       string  `json:"repeat_policy"`
	FailedResultPolicy string  `json:"failed_result_policy"`
	PassGradePoints    float64 `json:"pass_grade_points"`
}

func (p CreateProgrammeInput) validate() error {
	if p.Code == "" || len(p.Code) > 32 || p.Name == "" || len(p.Name) > 160 ||
		p.Category == "" || len(p.Category) > 100 ||
		(p.RepeatPolicy != "best" && p.RepeatPolicy != "latest" && p.RepeatPolicy != "all") ||
		(p.FailedResultPolicy != "include" && p.FailedResultPolicy != "exclude") ||
		p.PassGradePoints < 0 || p.PassGradePoints > 4 ||
		!hasTwoDecimalPlaces(p.PassGradePoints) || !validProgrammeCode(p.Code) {
		return fmt.Errorf("%w: programme values are invalid", ErrValidation)
	}
	return nil
}

func validProgrammeCode(code string) bool {
	for _, char := range code {
		if (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') ||
			char == '_' || char == '-' {
			continue
		}
		return false
	}
	return true
}

type CourseFacts struct {
	ID           uuid.UUID  `json:"id"`
	CourseID     uuid.UUID  `json:"course_id"`
	Credits      int        `json:"credits"`
	AcademicYear string     `json:"academic_year"`
	Semester     string     `json:"semester"`
	LecturerID   *uuid.UUID `json:"lecturer_user_id"`
}

type GradeSummary struct {
	GradeCode string `json:"grade_code"`
	Count     int64  `json:"count"`
}

type AdminSummary struct {
	TotalResults   int64              `json:"total_results"`
	AveragePercent *float64           `json:"average_percent,omitempty"`
	Distribution   []GradeSummary     `json:"grade_distribution"`
	Programmes     []ProgrammeSummary `json:"programme_breakdown"`
}

type ProgrammeSummary struct {
	Code           string  `json:"programme_code"`
	Name           string  `json:"programme_name"`
	Category       string  `json:"category"`
	TotalResults   int64   `json:"total_results"`
	AveragePercent float64 `json:"average_percent"`
}

type ReportCourse struct {
	OfferingID   uuid.UUID `json:"offering_id"`
	CourseID     uuid.UUID `json:"course_id"`
	AcademicYear string    `json:"academic_year"`
	Semester     string    `json:"semester"`
	Credits      int       `json:"credits"`
	FinalMark    float64   `json:"final_mark"`
	MaxMark      float64   `json:"max_mark"`
	Percent      float64   `json:"percent"`
	GradeCode    string    `json:"grade_code,omitempty"`
	GradePoints  *float64  `json:"grade_points,omitempty"`
	ImportedAt   time.Time `json:"imported_at"`
}

type TermGPA struct {
	AcademicYear string   `json:"academic_year"`
	Semester     string   `json:"semester"`
	GPA          *float64 `json:"gpa,omitempty"`
	CourseCount  int      `json:"course_count"`
}

type StudentReport struct {
	StudentUserID uuid.UUID        `json:"student_user_id"`
	ProgrammeCode string           `json:"programme_code,omitempty"`
	GPAStatus     string           `json:"gpa_status"`
	CumulativeGPA *float64         `json:"cumulative_gpa,omitempty"`
	Terms         []TermGPA        `json:"terms"`
	Results       []ReportCourse   `json:"results"`
	Components    []ComponentGrade `json:"component_grades"`
}
