package assessment

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/google/uuid"
)

const (
	RoleAdmin    = "ADMIN"
	RoleLecturer = "LECTURER"
	RoleStudent  = "STUDENT"

	PermAssessmentRead         = "assessments:read"
	PermAssessmentManage       = "assessments:manage"
	PermAssessmentGrade        = "assessments:grade"
	PermAssessmentRelease      = "assessments:release"
	PermAssessmentSubmit       = "assessments:submit"
	PermAssessmentStudentRead  = "assessments:student:read"
	PermAssessmentResultImport = "assessments:results:import"
)

type AssessmentStatus string

const (
	AssessmentDraft     AssessmentStatus = "draft"
	AssessmentPublished AssessmentStatus = "published"
	AssessmentClosed    AssessmentStatus = "closed"
)

type ComponentKind string

const (
	ComponentCA        ComponentKind = "ca"
	ComponentFinalExam ComponentKind = "final_exam"
	ComponentOther     ComponentKind = "other"
)

type LatePolicy string

const (
	LateReject LatePolicy = "reject"
	LateAccept LatePolicy = "accept"
)

type SubmissionStatus string

const (
	SubmissionOnTime SubmissionStatus = "submitted"
	SubmissionLate   SubmissionStatus = "late"
)

type GradeStatus string

const (
	GradeDraft    GradeStatus = "draft"
	GradeReleased GradeStatus = "released"
)

type Actor struct {
	UserID      uuid.UUID
	Roles       []string
	Permissions []string
}

func (a Actor) HasPermission(permission string) bool {
	for _, candidate := range a.Permissions {
		if candidate == permission {
			return true
		}
	}
	return false
}

func (a Actor) HasRole(role string) bool {
	for _, candidate := range a.Roles {
		if candidate == role {
			return true
		}
	}
	return false
}

type AllowedFileType struct {
	Extension string `json:"extension"`
	MediaType string `json:"media_type"`
}

type Assessment struct {
	ID               uuid.UUID         `json:"id"`
	OfferingID       uuid.UUID         `json:"offering_id"`
	Title            string            `json:"title"`
	Description      string            `json:"description"`
	ComponentKind    ComponentKind     `json:"component_kind"`
	DueAt            time.Time         `json:"due_at"`
	MaxMark          float64           `json:"max_mark"`
	WeightPercent    float64           `json:"weight_percent"`
	MaxAttempts      int               `json:"max_attempts"`
	LatePolicy       LatePolicy        `json:"late_policy"`
	AllowedFileTypes []AllowedFileType `json:"allowed_file_types"`
	Status           AssessmentStatus  `json:"status"`
	Version          int64             `json:"version"`
	CreatedByUserID  uuid.UUID         `json:"created_by_user_id"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
	PublishedAt      *time.Time        `json:"published_at,omitempty"`
	ClosedAt         *time.Time        `json:"closed_at,omitempty"`
}

type CreateAssessmentInput struct {
	OfferingID       uuid.UUID
	Title            string
	Description      string
	ComponentKind    ComponentKind
	DueAt            time.Time
	MaxMark          float64
	WeightPercent    float64
	MaxAttempts      int
	LatePolicy       LatePolicy
	AllowedFileTypes []AllowedFileType
}

type AssessmentFilter struct {
	OfferingID uuid.UUID
	Status     AssessmentStatus
	Limit      int
	Offset     int
}

type AssessmentList struct {
	Assessments []Assessment `json:"assessments"`
	Total       int64        `json:"total"`
	Limit       int          `json:"limit"`
	Offset      int          `json:"offset"`
}

type Submission struct {
	ID               uuid.UUID        `json:"id"`
	AssessmentID     uuid.UUID        `json:"assessment_id"`
	StudentUserID    uuid.UUID        `json:"student_user_id"`
	AttemptNo        int              `json:"attempt_no"`
	IdempotencyKey   string           `json:"-"`
	ObjectKey        string           `json:"-"`
	OriginalFilename string           `json:"original_filename"`
	MediaType        string           `json:"media_type"`
	SizeBytes        int64            `json:"size_bytes"`
	ContentSHA256    string           `json:"content_sha256"`
	Status           SubmissionStatus `json:"status"`
	SubmittedAt      time.Time        `json:"submitted_at"`
}

type SubmissionList struct {
	Submissions []Submission `json:"submissions"`
	Total       int64        `json:"total"`
	Limit       int          `json:"limit"`
	Offset      int          `json:"offset"`
}

type FinalResult struct {
	ID            uuid.UUID `json:"id"`
	OfferingID    uuid.UUID `json:"offering_id"`
	StudentUserID uuid.UUID `json:"student_user_id"`
	FinalMark     float64   `json:"final_mark"`
	MaxMark       float64   `json:"max_mark"`
	Version       int64     `json:"version"`
	ImportBatchID uuid.UUID `json:"import_batch_id"`
	ImportedBy    uuid.UUID `json:"imported_by_user_id"`
	ImportedAt    time.Time `json:"imported_at"`
}

type FinalResultImportBatch struct {
	ID               uuid.UUID `json:"id"`
	OfferingID       uuid.UUID `json:"offering_id"`
	ImportedBy       uuid.UUID `json:"imported_by_user_id"`
	RowCount         int       `json:"row_count"`
	ImportedAt       time.Time `json:"imported_at"`
	AlreadyProcessed bool      `json:"already_processed"`
}

type FinalResultList struct {
	Results []FinalResult `json:"results"`
	Total   int64         `json:"total"`
	Limit   int           `json:"limit"`
	Offset  int           `json:"offset"`
}

type AnalyticsEventRecord struct {
	EventID    uuid.UUID       `json:"event_id"`
	OccurredAt time.Time       `json:"occurred_at"`
	Payload    json.RawMessage `json:"payload"`
}

type AnalyticsEventPage struct {
	Events         []AnalyticsEventRecord `json:"events"`
	HasMore        bool                   `json:"has_more"`
	NextOccurredAt *time.Time             `json:"next_occurred_at,omitempty"`
	NextEventID    *uuid.UUID             `json:"next_event_id,omitempty"`
}

type Grade struct {
	ID           uuid.UUID   `json:"id"`
	SubmissionID uuid.UUID   `json:"submission_id"`
	GradedBy     uuid.UUID   `json:"graded_by_user_id"`
	Mark         float64     `json:"mark"`
	Feedback     string      `json:"feedback"`
	Status       GradeStatus `json:"status"`
	Version      int64       `json:"version"`
	GradedAt     time.Time   `json:"graded_at"`
	ReleasedAt   *time.Time  `json:"released_at,omitempty"`
	UpdatedAt    time.Time   `json:"updated_at"`
}

type AssessmentStore interface {
	CreateAssessment(context.Context, *Assessment) error
	GetAssessment(context.Context, uuid.UUID) (*Assessment, error)
	ListAssessments(context.Context, AssessmentFilter) (*AssessmentList, error)
	ChangeAssessmentStatus(context.Context, uuid.UUID, int64, AssessmentStatus) (*Assessment, error)
	CreateExtension(context.Context, uuid.UUID, uuid.UUID, time.Time, string, uuid.UUID) error
	GetExtensionDeadline(context.Context, uuid.UUID, uuid.UUID) (*time.Time, error)
	CreateSubmission(context.Context, *Submission, int) (*Submission, error)
	GetSubmission(context.Context, uuid.UUID) (*Submission, error)
	ListSubmissions(context.Context, uuid.UUID, int, int) (*SubmissionList, error)
	ImportFinalResults(context.Context, uuid.UUID, uuid.UUID, string, string, []FinalResultInput, string) (*FinalResultImportBatch, error)
	ListFinalResults(context.Context, uuid.UUID, uuid.UUID, int, int) (*FinalResultList, error)
	GetGrade(context.Context, uuid.UUID) (*Grade, error)
	SaveGrade(context.Context, *Grade, int64) (*Grade, error)
	ReleaseGrade(context.Context, uuid.UUID, int64, uuid.UUID, string) (*Grade, error)
	ListAnalyticsEvents(context.Context, time.Time, uuid.UUID, int) (*AnalyticsEventPage, error)
}

type EnrollmentReader interface {
	ActiveStudent(context.Context, uuid.UUID, uuid.UUID) (bool, error)
	ActiveLecturer(context.Context, uuid.UUID, uuid.UUID) (bool, error)
}

type ObjectStore interface {
	Put(context.Context, string, string, int64, io.Reader) error
	Get(context.Context, string) (io.ReadCloser, string, error)
	Delete(context.Context, string) error
	Ready(context.Context) error
}

type AssessmentService struct {
	store          AssessmentStore
	enrollments    EnrollmentReader
	objects        ObjectStore
	maxUploadBytes int64
	allowedFiles   map[string]string
	now            func() time.Time
}

func NewAssessmentService(
	store AssessmentStore,
	enrollments EnrollmentReader,
	objects ObjectStore,
	maxUploadBytes int64,
	allowedFiles []AllowedFileType,
) *AssessmentService {
	fileTypes := make(map[string]string, len(allowedFiles))
	for _, fileType := range allowedFiles {
		fileTypes[fileType.Extension] = fileType.MediaType
	}
	return &AssessmentService{
		store: store, enrollments: enrollments, objects: objects,
		maxUploadBytes: maxUploadBytes, allowedFiles: fileTypes, now: time.Now,
	}
}

type Upload struct {
	Filename  string
	MediaType string
	SizeBytes int64
	SHA256    string
	Reader    io.Reader
}

type FinalResultInput struct {
	StudentUserID uuid.UUID
	FinalMark     float64
	MaxMark       float64
}

type CreateExtensionInput struct {
	AssessmentID uuid.UUID
	StudentID    uuid.UUID
	RevisedDueAt time.Time
	Reason       string
}
