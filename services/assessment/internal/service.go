package assessment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (s *AssessmentService) Create(
	ctx context.Context,
	actor Actor,
	input CreateAssessmentInput,
) (*Assessment, error) {
	if input.OfferingID == uuid.Nil {
		return nil, newValidationError("offering_id", "must be a valid UUID")
	}
	if err := s.authorizeManageOffering(ctx, actor, input.OfferingID); err != nil {
		return nil, err
	}
	input.Title = strings.TrimSpace(input.Title)
	if len(input.Title) == 0 || len(input.Title) > 200 {
		return nil, newValidationError("title", "must contain between 1 and 200 characters")
	}
	if len(input.Description) > 10000 {
		return nil, newValidationError("description", "must not exceed 10000 characters")
	}
	switch input.ComponentKind {
	case ComponentCA, ComponentFinalExam, ComponentOther:
	default:
		return nil, newValidationError("component_kind", "must be ca, final_exam, or other")
	}
	if input.DueAt.IsZero() {
		return nil, newValidationError("due_at", "is required")
	}
	if !validHundredth(input.MaxMark) || input.MaxMark <= 0 || input.MaxMark > 999999.99 {
		return nil, newValidationError("max_mark", "must be greater than zero and at most 999999.99 with at most two decimal places")
	}
	if !validHundredth(input.WeightPercent) || input.WeightPercent <= 0 || input.WeightPercent > 100 {
		return nil, newValidationError("weight_percent", "must be greater than zero and at most 100")
	}
	if input.MaxAttempts < 1 || input.MaxAttempts > 100 {
		return nil, newValidationError("max_attempts", "must be between 1 and 100")
	}
	if input.LatePolicy != LateAccept && input.LatePolicy != LateReject {
		return nil, newValidationError("late_policy", "must be accept or reject")
	}
	if len(input.AllowedFileTypes) == 0 {
		input.AllowedFileTypes = DefaultAllowedFileTypes()
	}
	seenExtensions := make(map[string]struct{}, len(input.AllowedFileTypes))
	for i := range input.AllowedFileTypes {
		fileType := &input.AllowedFileTypes[i]
		fileType.Extension = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(fileType.Extension)), ".")
		fileType.MediaType = strings.ToLower(strings.TrimSpace(fileType.MediaType))
		expected, ok := s.allowedFiles[fileType.Extension]
		if !ok || expected != fileType.MediaType {
			return nil, newValidationError("allowed_file_types", "contains a file type that is not enabled by the service")
		}
		if _, exists := seenExtensions[fileType.Extension]; exists {
			return nil, newValidationError("allowed_file_types", "contains a duplicate extension")
		}
		seenExtensions[fileType.Extension] = struct{}{}
	}

	item := &Assessment{
		ID: uuid.New(), OfferingID: input.OfferingID, Title: input.Title,
		Description: input.Description, ComponentKind: input.ComponentKind,
		DueAt: input.DueAt.UTC(), MaxMark: input.MaxMark,
		WeightPercent: input.WeightPercent, MaxAttempts: input.MaxAttempts,
		LatePolicy: input.LatePolicy, AllowedFileTypes: input.AllowedFileTypes,
		Status: AssessmentDraft, Version: 1, CreatedByUserID: actor.UserID,
	}
	if err := s.store.CreateAssessment(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *AssessmentService) ListAnalyticsEvents(
	ctx context.Context, after time.Time, afterID uuid.UUID, limit int,
) (*AnalyticsEventPage, error) {
	if limit < 1 || limit > 500 {
		return nil, newValidationError("limit", "must be between 1 and 500")
	}
	return s.store.ListAnalyticsEvents(ctx, after, afterID, limit)
}

func (s *AssessmentService) Get(
	ctx context.Context,
	actor Actor,
	assessmentID uuid.UUID,
) (*Assessment, error) {
	item, err := s.store.GetAssessment(ctx, assessmentID)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeReadAssessment(ctx, actor, item, false); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *AssessmentService) ListForOffering(
	ctx context.Context,
	actor Actor,
	offeringID uuid.UUID,
	filter AssessmentFilter,
) (*AssessmentList, error) {
	if offeringID == uuid.Nil {
		return nil, newValidationError("offering_id", "must be a valid UUID")
	}
	filter.OfferingID = offeringID
	if filter.Limit == 0 {
		filter.Limit = 20
	}
	if filter.Limit < 1 || filter.Limit > 100 {
		return nil, newValidationError("limit", "must be between 1 and 100")
	}
	if filter.Offset < 0 || filter.Offset > 1_000_000 {
		return nil, newValidationError("offset", "must be between 0 and 1000000")
	}
	if actor.UserID == uuid.Nil {
		return nil, ErrUnauthorized
	}
	switch {
	case actor.HasRole(RoleAdmin) && actor.HasPermission(PermAssessmentRead):
	case actor.HasRole(RoleLecturer) && actor.HasPermission(PermAssessmentRead):
		if err := s.ensureAssignedLecturer(ctx, actor.UserID, offeringID); err != nil {
			return nil, err
		}
	case actor.HasRole(RoleStudent) && actor.HasPermission(PermAssessmentSubmit):
		active, err := s.enrollments.ActiveStudent(ctx, actor.UserID, offeringID)
		if err != nil {
			return nil, err
		}
		if !active {
			return nil, ErrForbidden
		}
		filter.Status = AssessmentPublished
	default:
		return nil, ErrForbidden
	}
	return s.store.ListAssessments(ctx, filter)
}

func (s *AssessmentService) ChangeStatus(
	ctx context.Context,
	actor Actor,
	assessmentID uuid.UUID,
	expectedVersion int64,
	next AssessmentStatus,
) (*Assessment, error) {
	if expectedVersion < 1 {
		return nil, newValidationError("version", "must be positive")
	}
	item, err := s.store.GetAssessment(ctx, assessmentID)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeManageOffering(ctx, actor, item.OfferingID); err != nil {
		return nil, err
	}
	if !item.Status.canTransitionTo(next) {
		return nil, ErrInvalidTransition
	}
	return s.store.ChangeAssessmentStatus(ctx, assessmentID, expectedVersion, next)
}

func (s *AssessmentService) CreateExtension(
	ctx context.Context,
	actor Actor,
	input CreateExtensionInput,
) error {
	item, err := s.store.GetAssessment(ctx, input.AssessmentID)
	if err != nil {
		return err
	}
	if err := s.authorizeManageOffering(ctx, actor, item.OfferingID); err != nil {
		return err
	}
	if item.Status != AssessmentPublished {
		return ErrInvalidTransition
	}
	if input.StudentID == uuid.Nil {
		return newValidationError("student_user_id", "must be a valid UUID")
	}
	input.Reason = strings.TrimSpace(input.Reason)
	if len(input.Reason) == 0 || len(input.Reason) > 2000 {
		return newValidationError("reason", "must contain between 1 and 2000 characters")
	}
	if !input.RevisedDueAt.After(item.DueAt) {
		return newValidationError("revised_due_at", "must be later than the original deadline")
	}
	active, err := s.enrollments.ActiveStudent(ctx, input.StudentID, item.OfferingID)
	if err != nil {
		return err
	}
	if !active {
		return ErrStudentIneligible
	}
	return s.store.CreateExtension(
		ctx, item.ID, input.StudentID, input.RevisedDueAt.UTC(), input.Reason, actor.UserID,
	)
}

func (s *AssessmentService) Submit(
	ctx context.Context,
	actor Actor,
	assessmentID uuid.UUID,
	idempotencyKey string,
	upload Upload,
) (*Submission, error) {
	if actor.UserID == uuid.Nil {
		return nil, ErrUnauthorized
	}
	if !actor.HasRole(RoleStudent) || !actor.HasPermission(PermAssessmentSubmit) {
		return nil, ErrForbidden
	}
	if !validIdempotencyKey(idempotencyKey) {
		return nil, newValidationError("Idempotency-Key", "must contain 8 to 128 safe characters")
	}
	item, err := s.store.GetAssessment(ctx, assessmentID)
	if err != nil {
		return nil, err
	}
	if item.Status != AssessmentPublished {
		return nil, ErrSubmissionClosed
	}
	active, err := s.enrollments.ActiveStudent(ctx, actor.UserID, item.OfferingID)
	if err != nil {
		return nil, err
	}
	if !active {
		return nil, ErrStudentIneligible
	}
	if upload.Reader == nil || upload.SizeBytes < 1 || upload.SizeBytes > s.maxUploadBytes {
		return nil, newValidationError("file", "must be a non-empty file within the configured size limit")
	}
	filename := strings.TrimSpace(upload.Filename)
	if filename == "" || len(filename) > 255 || strings.ContainsAny(filename, "\r\n/\\\x00") ||
		filepath.Base(filename) != filename || filename == "." || filename == ".." {
		return nil, newValidationError("file", "filename is invalid")
	}
	extension := strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
	allowedMediaType, enabled := s.allowedFiles[extension]
	if !enabled || allowedMediaType != upload.MediaType {
		return nil, newValidationError("file", "file type is not allowed")
	}
	if len(item.AllowedFileTypes) > 0 {
		assessmentAllows := false
		for _, fileType := range item.AllowedFileTypes {
			if fileType.Extension == extension && fileType.MediaType == upload.MediaType {
				assessmentAllows = true
				break
			}
		}
		if !assessmentAllows {
			return nil, newValidationError("file", "file type is not allowed for this assessment")
		}
	}

	now := s.now().UTC()
	deadline := item.DueAt
	extensionDeadline, err := s.store.GetExtensionDeadline(ctx, item.ID, actor.UserID)
	if err != nil {
		return nil, err
	}
	if extensionDeadline != nil && extensionDeadline.After(deadline) {
		deadline = *extensionDeadline
	}
	status := SubmissionOnTime
	if now.After(deadline) {
		if item.LatePolicy == LateReject {
			return nil, ErrSubmissionClosed
		}
		status = SubmissionLate
	}

	id := uuid.New()
	objectKey := item.ID.String() + "/" + actor.UserID.String() + "/" + id.String()
	hash := sha256.New()
	reader := io.TeeReader(upload.Reader, hash)
	if err := s.objects.Put(ctx, objectKey, upload.MediaType, upload.SizeBytes, reader); err != nil {
		return nil, fmt.Errorf("store submission object: %w", err)
	}

	submission := &Submission{
		ID: id, AssessmentID: item.ID, StudentUserID: actor.UserID,
		IdempotencyKey: idempotencyKey,
		ObjectKey: objectKey, OriginalFilename: filename, MediaType: upload.MediaType,
		SizeBytes: upload.SizeBytes, ContentSHA256: hex.EncodeToString(hash.Sum(nil)),
		Status: status, SubmittedAt: now,
	}
	saved, err := s.store.CreateSubmission(ctx, submission, item.MaxAttempts)
	if err != nil {
		if errors.Is(err, ErrIdempotencyConflict) || errors.Is(err, ErrAttemptLimitReached) ||
			errors.Is(err, ErrSubmissionClosed) || errors.Is(err, ErrNotFound) {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if cleanupErr := s.objects.Delete(cleanupCtx, objectKey); cleanupErr != nil {
				return nil, errors.Join(err, fmt.Errorf("remove rejected submission object: %w", cleanupErr))
			}
			return nil, err
		}
		return nil, fmt.Errorf("persist submission metadata (stored object %s may need cleanup): %w",
			objectKey, err)
	}
	if saved.ID != id {
		if err := s.objects.Delete(ctx, objectKey); err != nil {
			return nil, fmt.Errorf("remove duplicate submission object: %w", err)
		}
	}
	return saved, nil
}

func (s *AssessmentService) ListSubmissions(
	ctx context.Context,
	actor Actor,
	assessmentID uuid.UUID,
	limit, offset int,
) (*SubmissionList, error) {
	item, err := s.store.GetAssessment(ctx, assessmentID)
	if err != nil {
		return nil, err
	}
	if actor.UserID == uuid.Nil {
		return nil, ErrUnauthorized
	}
	if actor.HasRole(RoleAdmin) && actor.HasPermission(PermAssessmentStudentRead) {
	} else if actor.HasRole(RoleLecturer) && actor.HasPermission(PermAssessmentRead) {
		if err := s.ensureAssignedLecturer(ctx, actor.UserID, item.OfferingID); err != nil {
			return nil, err
		}
	} else {
		return nil, ErrForbidden
	}
	if limit == 0 {
		limit = 20
	}
	if limit < 1 || limit > 100 {
		return nil, newValidationError("limit", "must be between 1 and 100")
	}
	if offset < 0 || offset > 1_000_000 {
		return nil, newValidationError("offset", "must be between 0 and 1000000")
	}
	return s.store.ListSubmissions(ctx, assessmentID, limit, offset)
}

func (s *AssessmentService) GetSubmission(
	ctx context.Context,
	actor Actor,
	submissionID uuid.UUID,
) (*Submission, *Grade, error) {
	submission, err := s.store.GetSubmission(ctx, submissionID)
	if err != nil {
		return nil, nil, err
	}
	if actor.HasRole(RoleStudent) && submission.StudentUserID != actor.UserID {
		return nil, nil, ErrNotFound
	}
	assessment, err := s.store.GetAssessment(ctx, submission.AssessmentID)
	if err != nil {
		return nil, nil, err
	}
	if err := s.authorizeReadAssessment(ctx, actor, assessment, true); err != nil {
		return nil, nil, err
	}
	grade, err := s.store.GetGrade(ctx, submission.ID)
	if errors.Is(err, ErrNotFound) {
		return submission, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	if actor.HasRole(RoleStudent) && grade.Status != GradeReleased {
		return submission, nil, nil
	}
	return submission, grade, nil
}

func (s *AssessmentService) DownloadSubmission(
	ctx context.Context,
	actor Actor,
	submissionID uuid.UUID,
) (*Submission, io.ReadCloser, string, error) {
	submission, _, err := s.GetSubmission(ctx, actor, submissionID)
	if err != nil {
		return nil, nil, "", err
	}
	body, mediaType, err := s.objects.Get(ctx, submission.ObjectKey)
	if err != nil {
		return nil, nil, "", fmt.Errorf("read submission object: %w", err)
	}
	return submission, body, mediaType, nil
}

func (s *AssessmentService) SaveGrade(
	ctx context.Context,
	actor Actor,
	submissionID uuid.UUID,
	mark float64,
	feedback string,
	expectedVersion int64,
) (*Grade, error) {
	submission, err := s.store.GetSubmission(ctx, submissionID)
	if err != nil {
		return nil, err
	}
	assessment, err := s.store.GetAssessment(ctx, submission.AssessmentID)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeGradeOffering(ctx, actor, assessment.OfferingID); err != nil {
		return nil, err
	}
	if !validHundredth(mark) || mark < 0 || mark > assessment.MaxMark {
		return nil, newValidationError("mark", "must be between zero and the assessment maximum with at most two decimal places")
	}
	if len(feedback) > 10000 {
		return nil, newValidationError("feedback", "must not exceed 10000 characters")
	}
	if expectedVersion < 0 {
		return nil, newValidationError("version", "must not be negative")
	}
	grade := &Grade{
		ID: uuid.New(), SubmissionID: submission.ID, GradedBy: actor.UserID,
		Mark: mark, Feedback: feedback, Status: GradeDraft, Version: 1,
	}
	return s.store.SaveGrade(ctx, grade, expectedVersion)
}

func (s *AssessmentService) ReleaseGrade(
	ctx context.Context,
	actor Actor,
	submissionID uuid.UUID,
	expectedVersion int64,
	correlationID string,
) (*Grade, error) {
	submission, err := s.store.GetSubmission(ctx, submissionID)
	if err != nil {
		return nil, err
	}
	assessment, err := s.store.GetAssessment(ctx, submission.AssessmentID)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeReleaseOffering(ctx, actor, assessment.OfferingID); err != nil {
		return nil, err
	}
	if expectedVersion < 1 {
		return nil, newValidationError("version", "must be positive")
	}
	if strings.TrimSpace(correlationID) == "" {
		correlationID = uuid.NewString()
	}
	return s.store.ReleaseGrade(ctx, submissionID, expectedVersion, actor.UserID, correlationID)
}

func (s *AssessmentService) authorizeManageOffering(
	ctx context.Context,
	actor Actor,
	offeringID uuid.UUID,
) error {
	if actor.UserID == uuid.Nil {
		return ErrUnauthorized
	}
	if actor.HasRole(RoleAdmin) && actor.HasPermission(PermAssessmentManage) {
		return nil
	}
	if actor.HasRole(RoleLecturer) && actor.HasPermission(PermAssessmentManage) {
		return s.ensureAssignedLecturer(ctx, actor.UserID, offeringID)
	}
	return ErrForbidden
}

func (s *AssessmentService) authorizeGradeOffering(
	ctx context.Context,
	actor Actor,
	offeringID uuid.UUID,
) error {
	if actor.UserID == uuid.Nil {
		return ErrUnauthorized
	}
	if actor.HasRole(RoleAdmin) && actor.HasPermission(PermAssessmentGrade) {
		return nil
	}
	if actor.HasRole(RoleLecturer) && actor.HasPermission(PermAssessmentGrade) {
		return s.ensureAssignedLecturer(ctx, actor.UserID, offeringID)
	}
	return ErrForbidden
}

func (s *AssessmentService) authorizeReleaseOffering(
	ctx context.Context,
	actor Actor,
	offeringID uuid.UUID,
) error {
	if actor.UserID == uuid.Nil {
		return ErrUnauthorized
	}
	if actor.HasRole(RoleAdmin) && actor.HasPermission(PermAssessmentRelease) {
		return nil
	}
	if actor.HasRole(RoleLecturer) && actor.HasPermission(PermAssessmentRelease) {
		return s.ensureAssignedLecturer(ctx, actor.UserID, offeringID)
	}
	return ErrForbidden
}

func (s *AssessmentService) authorizeReadAssessment(
	ctx context.Context,
	actor Actor,
	item *Assessment,
	submissionRead bool,
) error {
	if actor.UserID == uuid.Nil {
		return ErrUnauthorized
	}
	if actor.HasRole(RoleAdmin) {
		if submissionRead {
			if actor.HasPermission(PermAssessmentStudentRead) {
				return nil
			}
			return ErrForbidden
		}
		if actor.HasPermission(PermAssessmentRead) {
			return nil
		}
	}
	if actor.HasRole(RoleLecturer) && actor.HasPermission(PermAssessmentRead) {
		return s.ensureAssignedLecturer(ctx, actor.UserID, item.OfferingID)
	}
	if actor.HasRole(RoleStudent) && actor.HasPermission(PermAssessmentSubmit) {
		if item.Status != AssessmentPublished && !submissionRead {
			return ErrNotFound
		}
		if !submissionRead {
			active, err := s.enrollments.ActiveStudent(ctx, actor.UserID, item.OfferingID)
			if err != nil {
				return err
			}
			if !active {
				return ErrForbidden
			}
		}
		return nil
	}
	return ErrForbidden
}

func (s *AssessmentService) ensureAssignedLecturer(
	ctx context.Context,
	lecturerID uuid.UUID,
	offeringID uuid.UUID,
) error {
	active, err := s.enrollments.ActiveLecturer(ctx, lecturerID, offeringID)
	if err != nil {
		return err
	}
	if !active {
		return ErrForbidden
	}
	return nil
}

func validHundredth(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) &&
		math.Abs(value*100-math.Round(value*100)) < 1e-8
}

func validIdempotencyKey(value string) bool {
	if len(value) < 8 || len(value) > 128 {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			continue
		}
		return false
	}
	return true
}

func (s AssessmentStatus) canTransitionTo(next AssessmentStatus) bool {
	switch s {
	case AssessmentDraft:
		return next == AssessmentPublished
	case AssessmentPublished:
		return next == AssessmentClosed
	default:
		return false
	}
}
