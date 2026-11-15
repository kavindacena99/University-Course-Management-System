package analytics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	repository *Repository
	course     *CourseClient
	assessment *AssessmentClient
}

func NewService(
	repository *Repository, course *CourseClient, assessment *AssessmentClient,
) *Service {
	return &Service{repository: repository, course: course, assessment: assessment}
}

func (s *Service) IngestEvent(ctx context.Context, payload []byte) (bool, error) {
	var event EventEnvelope
	if err := json.Unmarshal(payload, &event); err != nil {
		return false, fmt.Errorf("%w: decode event: %v", ErrInvalidEvent, err)
	}
	event.Payload = payload
	if err := event.validate(); err != nil {
		return false, err
	}
	return s.repository.IngestEvent(ctx, event)
}

func (s *Service) StudentReport(
	ctx context.Context, studentID uuid.UUID,
) (*StudentReport, error) {
	revision, err := s.repository.ProjectionRevision(ctx)
	if err != nil {
		return nil, err
	}
	results, err := s.repository.StudentResults(ctx, studentID)
	if err != nil {
		return nil, err
	}
	components, err := s.repository.StudentComponents(ctx, studentID)
	if err != nil {
		return nil, err
	}
	bands, err := s.repository.GradeBands(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.repository.ConfirmProjectionRevision(ctx, revision); err != nil {
		return nil, err
	}
	report := &StudentReport{
		StudentUserID: studentID,
		Terms:         make([]TermGPA, 0),
		Results:       make([]ReportCourse, 0, len(results)),
		Components:    components,
		GPAStatus:     "no_results",
	}
	if len(results) == 0 {
		return report, nil
	}
	if len(bands) == 0 {
		report.GPAStatus = "grade_scale_unconfigured"
	}
	factsByOffering := make(map[uuid.UUID]*CourseFacts)
	for _, result := range results {
		facts := factsByOffering[result.OfferingID]
		if facts == nil {
			facts, err = s.course.GetOfferingFacts(ctx, result.OfferingID)
			if err != nil {
				return nil, fmt.Errorf("read Course facts for offering %s: %w", result.OfferingID, err)
			}
			factsByOffering[result.OfferingID] = facts
		}
		percent := result.FinalMark / result.MaxMark * 100
		course := ReportCourse{
			OfferingID: result.OfferingID, CourseID: facts.CourseID,
			AcademicYear: facts.AcademicYear, Semester: facts.Semester,
			Credits: facts.Credits, FinalMark: result.FinalMark, MaxMark: result.MaxMark,
			Percent: percent, ImportedAt: result.ImportedAt,
		}
		for _, band := range bands {
			if band.matches(percent) {
				course.GradeCode = band.Code
				points := band.gpaValue()
				course.GradePoints = &points
				break
			}
		}
		if course.GradePoints == nil && len(bands) > 0 {
			report.GPAStatus = "grade_scale_incomplete"
		}
		report.Results = append(report.Results, course)
	}
	programme, err := s.repository.StudentProgramme(ctx, studentID)
	if errors.Is(err, ErrNotFound) {
		if report.GPAStatus != "grade_scale_unconfigured" &&
			report.GPAStatus != "grade_scale_incomplete" {
			report.GPAStatus = "programme_unassigned"
		}
		return report, nil
	}
	if err != nil {
		return nil, err
	}
	report.ProgrammeCode = programme.Code
	if report.GPAStatus == "grade_scale_unconfigured" ||
		report.GPAStatus == "grade_scale_incomplete" {
		return report, nil
	}
	report.CumulativeGPA, _ = calculateGPA(
		report.Results, programme.RepeatPolicy,
		programme.FailedResultPolicy, programme.PassGradePoints,
	)
	report.Terms = calculateTermGPAs(
		report.Results, programme.RepeatPolicy,
		programme.FailedResultPolicy, programme.PassGradePoints,
	)
	report.GPAStatus = "ready"
	if report.CumulativeGPA == nil {
		report.GPAStatus = "no_eligible_results"
	}
	return report, nil
}

type OfferingReport struct {
	Offering   *CourseFacts     `json:"offering"`
	Total      int              `json:"total_results"`
	Results    []ReportStudent  `json:"results"`
	Components []ComponentGrade `json:"component_grades"`
}

type ReportStudent struct {
	StudentUserID uuid.UUID `json:"student_user_id"`
	FinalMark     float64   `json:"final_mark"`
	MaxMark       float64   `json:"max_mark"`
	Percent       float64   `json:"percent"`
	GradeCode     string    `json:"grade_code,omitempty"`
	ImportedAt    time.Time `json:"imported_at"`
}

func (s *Service) LecturerOfferingReport(
	ctx context.Context, offeringID, lecturerID uuid.UUID,
) (*OfferingReport, error) {
	revision, err := s.repository.ProjectionRevision(ctx)
	if err != nil {
		return nil, err
	}
	results, err := s.repository.OfferingResults(ctx, offeringID)
	if err != nil {
		return nil, err
	}
	bands, err := s.repository.GradeBands(ctx)
	if err != nil {
		return nil, err
	}
	components, err := s.repository.OfferingComponents(ctx, offeringID)
	if err != nil {
		return nil, err
	}
	if err := s.repository.ConfirmProjectionRevision(ctx, revision); err != nil {
		return nil, err
	}
	facts, err := s.course.GetOfferingFacts(ctx, offeringID)
	if err != nil {
		return nil, fmt.Errorf("read Course offering facts: %w", err)
	}
	if facts.LecturerID == nil || *facts.LecturerID != lecturerID {
		return nil, ErrNotFound
	}
	report := &OfferingReport{
		Offering: facts, Total: len(results),
		Results: make([]ReportStudent, 0, len(results)),
	}
	for _, result := range results {
		percent := result.FinalMark / result.MaxMark * 100
		item := ReportStudent{
			StudentUserID: result.StudentUserID,
			FinalMark: result.FinalMark, MaxMark: result.MaxMark,
			Percent: percent, ImportedAt: result.ImportedAt,
		}
		for _, band := range bands {
			if band.matches(percent) {
				item.GradeCode = band.Code
				break
			}
		}
		report.Results = append(report.Results, item)
	}
	report.Components = components
	return report, nil
}

func (s *Service) AdminSummary(ctx context.Context) (*AdminSummary, error) {
	revision, err := s.repository.ProjectionRevision(ctx)
	if err != nil {
		return nil, err
	}
	summary, err := s.repository.AdminSummary(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.repository.ConfirmProjectionRevision(ctx, revision); err != nil {
		return nil, err
	}
	return summary, nil
}

func (s *Service) GradeBands(ctx context.Context) ([]GradeBand, error) {
	return s.repository.GradeBands(ctx)
}

func (s *Service) ReplaceGradeBands(ctx context.Context, bands []GradeBand) error {
	if err := validateGradeBands(bands); err != nil {
		return err
	}
	return s.repository.ReplaceGradeBands(ctx, bands)
}

func (s *Service) ListProgrammes(ctx context.Context) ([]Programme, error) {
	return s.repository.ListProgrammes(ctx)
}

func (s *Service) SaveProgramme(
	ctx context.Context, input CreateProgrammeInput, id *uuid.UUID,
) (*Programme, error) {
	input.Code = strings.ToUpper(strings.TrimSpace(input.Code))
	input.Name = strings.TrimSpace(input.Name)
	input.Category = strings.TrimSpace(input.Category)
	if err := input.validate(); err != nil {
		return nil, err
	}
	if id == nil {
		return s.repository.CreateProgramme(ctx, input)
	}
	return s.repository.UpdateProgramme(ctx, *id, input)
}

func (s *Service) AssignProgramme(
	ctx context.Context, studentID, programmeID, assignedBy uuid.UUID,
) error {
	if studentID == uuid.Nil || programmeID == uuid.Nil || assignedBy == uuid.Nil {
		return fmt.Errorf("student_id and programme_id must be valid UUIDs")
	}
	return s.repository.AssignProgramme(ctx, studentID, programmeID, assignedBy)
}

func (s *Service) Rebuild(ctx context.Context) error {
	if err := s.repository.BeginRebuild(ctx); err != nil {
		return err
	}
	fail := func(cause error) error {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := s.repository.FailRebuild(cleanupCtx); err != nil {
			return fmt.Errorf("%w; mark rebuild failed: %v", cause, err)
		}
		return cause
	}
	after := time.Unix(0, 0).UTC()
	afterID := uuid.Nil
	for {
		page, err := s.assessment.ListEvents(ctx, after, afterID)
		if err != nil {
			return fail(fmt.Errorf("read Assessment event history: %w", err))
		}
		for _, event := range page.Events {
			if _, err := s.IngestEvent(ctx, event.Payload); err != nil {
				return fail(fmt.Errorf("replay Assessment event %s: %w", event.EventID, err))
			}
		}
		if !page.HasMore {
			break
		}
		after, afterID = *page.NextOccurredAt, *page.NextEventID
	}
	if err := s.repository.FinishRebuild(ctx); err != nil {
		return fail(err)
	}
	return nil
}

func calculateTermGPAs(
	results []ReportCourse, repeatPolicy, failedPolicy string, passPoints float64,
) []TermGPA {
	periods := make(map[string][]ReportCourse)
	for _, result := range results {
		key := result.AcademicYear + "\x00" + result.Semester
		periods[key] = append(periods[key], result)
	}
	keys := make([]string, 0, len(periods))
	for key := range periods {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	terms := make([]TermGPA, 0, len(keys))
	for _, key := range keys {
		parts := strings.SplitN(key, "\x00", 2)
		gpa, count := calculateGPA(
			periods[key], repeatPolicy, failedPolicy, passPoints,
		)
		terms = append(terms, TermGPA{
			AcademicYear: parts[0], Semester: parts[1],
			GPA: gpa, CourseCount: count,
		})
	}
	return terms
}

func calculateGPA(
	results []ReportCourse, repeatPolicy, failedPolicy string, passPoints float64,
) (*float64, int) {
	selected := selectRepeatedResults(results, repeatPolicy)
	var qualityPoints, credits float64
	count := 0
	for _, result := range selected {
		if result.GradePoints == nil ||
			(failedPolicy == "exclude" && *result.GradePoints < passPoints) {
			continue
		}
		qualityPoints += *result.GradePoints * float64(result.Credits)
		credits += float64(result.Credits)
		count++
	}
	if credits == 0 {
		return nil, count
	}
	gpa := math.Round((qualityPoints/credits)*100) / 100
	return &gpa, count
}

func selectRepeatedResults(results []ReportCourse, repeatPolicy string) []ReportCourse {
	if repeatPolicy == "all" {
		return results
	}
	selected := make(map[uuid.UUID]ReportCourse, len(results))
	for _, result := range results {
		current, exists := selected[result.CourseID]
		if !exists {
			selected[result.CourseID] = result
			continue
		}
		replace := result.ImportedAt.After(current.ImportedAt)
		if repeatPolicy == "best" {
			replace = result.GradePoints != nil &&
				(current.GradePoints == nil || *result.GradePoints > *current.GradePoints ||
					(*result.GradePoints == *current.GradePoints && result.ImportedAt.After(current.ImportedAt)))
		}
		if replace {
			selected[result.CourseID] = result
		}
	}
	result := make([]ReportCourse, 0, len(selected))
	for _, item := range selected {
		result = append(result, item)
	}
	return result
}
