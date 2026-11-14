package course

import (
	"context"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

type CourseStore interface {
	Ping(ctx context.Context) error
	CreateCourse(ctx context.Context, item *Course) error
	GetCourse(ctx context.Context, id uuid.UUID) (*Course, error)
	UpdateCourse(ctx context.Context, input UpdateCourseRecord) (*Course, error)
	ChangeCourseStatus(ctx context.Context, input ChangeCourseStatusRecord) (*Course, error)
	ListCourses(ctx context.Context, filter CourseFilter) (*CourseList, error)

	GetOffering(ctx context.Context, id uuid.UUID) (*CourseOffering, error)
	CreateOffering(ctx context.Context, item *CourseOffering) error
	UpdateOffering(ctx context.Context, input UpdateOfferingRecord) (*CourseOffering, error)
	ChangeOfferingStatus(ctx context.Context, input ChangeOfferingStatusRecord) (*CourseOffering, error)
	AssignLecturer(ctx context.Context, input AssignLecturerRecord) (*CourseOffering, error)
	ListOfferings(ctx context.Context, filter OfferingFilter) (*OfferingList, error)
}

type LecturerEligibility interface {
	EnsureEligibleLecturer(ctx context.Context, userID uuid.UUID) error
}

type Service struct {
	store     CourseStore
	lecturers LecturerEligibility
}

func NewService(store CourseStore, lecturers LecturerEligibility) *Service {
	return &Service{store: store, lecturers: lecturers}
}

type CreateCourseInput struct {
	Code        string
	Title       string
	Description string
	Credits     int
}

func (s *Service) CreateCourse(
	ctx context.Context,
	claims *AccessClaims,
	input CreateCourseInput,
) (*Course, error) {
	actorID, err := authorizePermission(claims, PermCoursesManage)
	if err != nil {
		return nil, err
	}

	code, title, description, err := validateCourseFields(
		input.Code, input.Title, input.Description, input.Credits,
	)
	if err != nil {
		return nil, err
	}

	item := &Course{
		ID:          uuid.New(),
		Code:        code,
		Title:       title,
		Description: description,
		Credits:     input.Credits,
		Status:      CourseDraft,
		CreatedBy:   actorID,
		UpdatedBy:   actorID,
	}

	if err := s.store.CreateCourse(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) GetCourse(
	ctx context.Context,
	claims *AccessClaims,
	courseID uuid.UUID,
) (*Course, error) {
	if _, err := requireClaims(claims); err != nil {
		return nil, err
	}

	item, err := s.store.GetCourse(ctx, courseID)
	if err != nil {
		return nil, err
	}
	if !canReadAllCourses(claims) && item.Status != CourseActive {
		return nil, ErrCourseNotFound
	}
	return item, nil
}

func (s *Service) ListCourses(
	ctx context.Context,
	claims *AccessClaims,
	filter CourseFilter,
) (*CourseList, error) {
	if _, err := requireClaims(claims); err != nil {
		return nil, err
	}
	if !canReadAllCourses(claims) {
		if filter.Status != "" && filter.Status != CourseActive {
			return nil, ErrForbidden
		}
		filter.Status = CourseActive
	}
	if filter.Status != "" && !filter.Status.Valid() {
		return nil, newValidationError("status", "is not a valid course status")
	}
	if utf8.RuneCountInString(filter.Search) > 100 {
		return nil, newValidationError("search", "must not exceed 100 characters")
	}

	applyPagination(&filter.Limit, &filter.Offset)
	var err error
	if filter.Sort, err = validateCourseSort(filter.Sort); err != nil {
		return nil, err
	}
	if filter.Order, err = validateOrder(filter.Order); err != nil {
		return nil, err
	}

	return s.store.ListCourses(ctx, filter)
}

type UpdateCourseInput struct {
	ExpectedVersion int64
	Code            string
	Title           string
	Description     string
	Credits         int
}

func (s *Service) UpdateCourse(
	ctx context.Context,
	claims *AccessClaims,
	courseID uuid.UUID,
	input UpdateCourseInput,
) (*Course, error) {
	actorID, err := authorizePermission(claims, PermCoursesManage)
	if err != nil {
		return nil, err
	}
	if input.ExpectedVersion <= 0 {
		return nil, newValidationError("version", "must be provided and positive")
	}

	current, err := s.store.GetCourse(ctx, courseID)
	if err != nil {
		return nil, err
	}
	if current.Status == CourseArchived {
		return nil, ErrCourseArchived
	}

	code, title, description, err := validateCourseFields(
		input.Code, input.Title, input.Description, input.Credits,
	)
	if err != nil {
		return nil, err
	}
	if code != current.Code && current.Status != CourseDraft {
		return nil, ErrCourseCodeLocked
	}

	return s.store.UpdateCourse(ctx, UpdateCourseRecord{
		CourseID:        courseID,
		ActorID:         actorID,
		ExpectedVersion: input.ExpectedVersion,
		Code:            code,
		Title:           title,
		Description:     description,
		Credits:         input.Credits,
		Previous:        *current,
	})
}

func (s *Service) ChangeCourseStatus(
	ctx context.Context,
	claims *AccessClaims,
	courseID uuid.UUID,
	action string,
	expectedVersion int64,
) (*Course, error) {
	actorID, err := authorizePermission(claims, PermCoursesManage)
	if err != nil {
		return nil, err
	}
	if expectedVersion <= 0 {
		return nil, newValidationError("version", "must be provided and positive")
	}

	next, ok := courseTransitionForAction(action)
	if !ok {
		return nil, newValidationError("action", "is not supported")
	}
	if action == "restore" {
		next = CourseActive
	}

	current, err := s.store.GetCourse(ctx, courseID)
	if err != nil {
		return nil, err
	}
	if current.Version != expectedVersion {
		return nil, ErrVersionConflict
	}
	if action == "restore" && current.Status != CourseArchived {
		return nil, ErrInvalidTransition
	}
	if !current.Status.CanTransitionTo(next) {
		return nil, ErrInvalidTransition
	}

	return s.store.ChangeCourseStatus(ctx, ChangeCourseStatusRecord{
		CourseID:        courseID,
		ActorID:         actorID,
		ExpectedVersion: expectedVersion,
		FromStatus:      current.Status,
		ToStatus:        next,
		Action:          action,
	})
}

type CreateOfferingInput struct {
	CourseID           uuid.UUID
	AcademicYear       string
	Semester           string
	StartDate          string
	EndDate            string
	EnrollmentOpensAt  *time.Time
	EnrollmentClosesAt *time.Time
	EnrollmentCapacity *int
	LecturerUserID     *uuid.UUID
}

func (s *Service) CreateOffering(
	ctx context.Context,
	claims *AccessClaims,
	input CreateOfferingInput,
) (*CourseOffering, error) {
	actorID, err := authorizePermission(claims, PermCoursesManage)
	if err != nil {
		return nil, err
	}
	if input.CourseID == uuid.Nil {
		return nil, newValidationError("course_id", "must be a valid UUID")
	}

	course, err := s.store.GetCourse(ctx, input.CourseID)
	if err != nil {
		return nil, err
	}
	if course.Status == CourseArchived {
		return nil, ErrCourseArchived
	}

	fields, err := validateOfferingFields(offeringInput{
		AcademicYear:       input.AcademicYear,
		Semester:           input.Semester,
		StartDate:          input.StartDate,
		EndDate:            input.EndDate,
		EnrollmentOpensAt:  input.EnrollmentOpensAt,
		EnrollmentClosesAt: input.EnrollmentClosesAt,
		EnrollmentCapacity: input.EnrollmentCapacity,
	})
	if err != nil {
		return nil, err
	}

	if input.LecturerUserID != nil {
		if *input.LecturerUserID == uuid.Nil {
			return nil, newValidationError("lecturer_user_id", "must be a valid UUID")
		}
		if err := s.ensureEligibleLecturer(ctx, *input.LecturerUserID); err != nil {
			return nil, err
		}
	}

	item := &CourseOffering{
		ID:                 uuid.New(),
		CourseID:           input.CourseID,
		CourseCode:         course.Code,
		CourseTitle:        course.Title,
		AcademicYear:       fields.AcademicYear,
		Semester:           fields.Semester,
		StartDate:          fields.StartDate,
		EndDate:            fields.EndDate,
		EnrollmentOpensAt:  fields.EnrollmentOpensAt,
		EnrollmentClosesAt: fields.EnrollmentClosesAt,
		EnrollmentCapacity: fields.EnrollmentCapacity,
		Status:             OfferingDraft,
		CreatedBy:          actorID,
		UpdatedBy:          actorID,
	}
	if input.LecturerUserID != nil {
		assignedAt := time.Now().UTC()
		item.LecturerUserID = input.LecturerUserID
		item.LecturerAssignedBy = &actorID
		item.LecturerAssignedAt = &assignedAt
	}

	if err := s.store.CreateOffering(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) GetOffering(
	ctx context.Context,
	claims *AccessClaims,
	offeringID uuid.UUID,
) (*CourseOffering, error) {
	actorID, err := requireClaims(claims)
	if err != nil {
		return nil, err
	}

	item, err := s.store.GetOffering(ctx, offeringID)
	if err != nil {
		return nil, err
	}

	privileged := canReadAllOfferings(claims)
	assigned := HasAnyRole(claims, RoleLecturer) &&
		HasAnyPermission(claims, PermOfferingsManage) &&
		item.LecturerUserID != nil && *item.LecturerUserID == actorID
	if !privileged && !assigned && item.Status != OfferingPublished {
		return nil, ErrOfferingNotFound
	}
	return item, nil
}

// GetOfferingFacts serves only the HMAC-authenticated Enrollment integration.
func (s *Service) GetOfferingFacts(
	ctx context.Context, offeringID uuid.UUID,
) (*OfferingFacts, error) {
	item, err := s.store.GetOffering(ctx, offeringID)
	if err != nil {
		return nil, err
	}
	return &OfferingFacts{
		ID: item.ID, Status: item.Status,
		EnrollmentOpensAt:  item.EnrollmentOpensAt,
		EnrollmentClosesAt: item.EnrollmentClosesAt,
		EnrollmentCapacity: item.EnrollmentCapacity,
		LecturerUserID:     item.LecturerUserID,
		Version:            item.Version,
	}, nil
}

func (s *Service) ListCourseOfferings(
	ctx context.Context,
	claims *AccessClaims,
	courseID uuid.UUID,
	filter OfferingFilter,
) (*OfferingList, error) {
	if _, err := s.GetCourse(ctx, claims, courseID); err != nil {
		return nil, err
	}
	filter.CourseID = &courseID
	return s.ListOfferings(ctx, claims, filter)
}

func (s *Service) ListOfferings(
	ctx context.Context,
	claims *AccessClaims,
	filter OfferingFilter,
) (*OfferingList, error) {
	actorID, err := requireClaims(claims)
	if err != nil {
		return nil, err
	}

	privileged := canReadAllOfferings(claims)
	if filter.AssignedToMe {
		if !privileged && (!HasAnyRole(claims, RoleLecturer) ||
			!HasAnyPermission(claims, PermOfferingsManage)) {
			return nil, ErrForbidden
		}
		filter.AssignedToUserID = &actorID
		filter.PublishedOnly = false
	} else if !privileged {
		if filter.Status != "" && filter.Status != OfferingPublished {
			return nil, ErrForbidden
		}
		filter.Status = OfferingPublished
		filter.PublishedOnly = true
	}

	if filter.Status != "" && !filter.Status.Valid() {
		return nil, newValidationError("status", "is not a valid offering status")
	}
	if utf8.RuneCountInString(filter.AcademicYear) > 20 {
		return nil, newValidationError("academic_year", "must not exceed 20 characters")
	}
	if utf8.RuneCountInString(filter.Semester) > 20 {
		return nil, newValidationError("semester", "must not exceed 20 characters")
	}
	if utf8.RuneCountInString(filter.Search) > 100 {
		return nil, newValidationError("search", "must not exceed 100 characters")
	}

	applyPagination(&filter.Limit, &filter.Offset)
	if filter.Sort, err = validateOfferingSort(filter.Sort); err != nil {
		return nil, err
	}
	if filter.Order, err = validateOrder(filter.Order); err != nil {
		return nil, err
	}

	return s.store.ListOfferings(ctx, filter)
}

type UpdateOfferingInput struct {
	ExpectedVersion    int64
	AcademicYear       string
	Semester           string
	StartDate          string
	EndDate            string
	EnrollmentOpensAt  *time.Time
	EnrollmentClosesAt *time.Time
	EnrollmentCapacity *int
}

func (s *Service) UpdateOffering(
	ctx context.Context,
	claims *AccessClaims,
	offeringID uuid.UUID,
	input UpdateOfferingInput,
) (*CourseOffering, error) {
	actorID, err := authorizePermission(claims, PermOfferingsManage)
	if err != nil {
		return nil, err
	}
	if input.ExpectedVersion <= 0 {
		return nil, newValidationError("version", "must be provided and positive")
	}

	current, err := s.store.GetOffering(ctx, offeringID)
	if err != nil {
		return nil, err
	}
	if !HasAnyRole(claims, RoleAdmin) &&
		(current.LecturerUserID == nil || *current.LecturerUserID != actorID) {
		return nil, ErrForbidden
	}
	if !HasAnyRole(claims, RoleAdmin) && !HasAnyRole(claims, RoleLecturer) {
		return nil, ErrForbidden
	}
	if current.Version != input.ExpectedVersion {
		return nil, ErrVersionConflict
	}
	switch current.Status {
	case OfferingClosed, OfferingArchived:
		return nil, ErrOfferingImmutable
	}
	if !HasAnyRole(claims, RoleAdmin) {
		if err := s.ensureEligibleLecturer(ctx, actorID); err != nil {
			return nil, err
		}
	}

	fields, err := validateOfferingFields(offeringInput{
		AcademicYear:       input.AcademicYear,
		Semester:           input.Semester,
		StartDate:          input.StartDate,
		EndDate:            input.EndDate,
		EnrollmentOpensAt:  input.EnrollmentOpensAt,
		EnrollmentClosesAt: input.EnrollmentClosesAt,
		EnrollmentCapacity: input.EnrollmentCapacity,
	})
	if err != nil {
		return nil, err
	}

	if current.Status == OfferingPublished {
		if err := validatePublishedOfferingUpdate(current, fields); err != nil {
			return nil, err
		}
	}

	return s.store.UpdateOffering(ctx, UpdateOfferingRecord{
		OfferingID:         offeringID,
		ActorID:            actorID,
		ExpectedVersion:    input.ExpectedVersion,
		AcademicYear:       fields.AcademicYear,
		Semester:           fields.Semester,
		StartDate:          fields.StartDate,
		EndDate:            fields.EndDate,
		EnrollmentOpensAt:  fields.EnrollmentOpensAt,
		EnrollmentClosesAt: fields.EnrollmentClosesAt,
		EnrollmentCapacity: fields.EnrollmentCapacity,
		Previous:           *current,
	})
}

type AssignLecturerInput struct {
	ExpectedVersion int64
	LecturerUserID  uuid.UUID
}

func (s *Service) AssignLecturer(
	ctx context.Context,
	claims *AccessClaims,
	offeringID uuid.UUID,
	input AssignLecturerInput,
) (*CourseOffering, error) {
	actorID, err := authorizePermission(claims, PermCoursesManage)
	if err != nil {
		return nil, err
	}
	if input.ExpectedVersion <= 0 {
		return nil, newValidationError("version", "must be provided and positive")
	}
	if input.LecturerUserID == uuid.Nil {
		return nil, newValidationError("lecturer_user_id", "must be a valid UUID")
	}

	current, err := s.store.GetOffering(ctx, offeringID)
	if err != nil {
		return nil, err
	}
	if current.Version != input.ExpectedVersion {
		return nil, ErrVersionConflict
	}
	switch current.Status {
	case OfferingClosed, OfferingArchived:
		return nil, ErrOfferingImmutable
	}
	if current.LecturerUserID != nil && *current.LecturerUserID == input.LecturerUserID {
		return current, nil
	}

	if err := s.ensureEligibleLecturer(ctx, input.LecturerUserID); err != nil {
		return nil, err
	}

	return s.store.AssignLecturer(ctx, AssignLecturerRecord{
		OfferingID:      offeringID,
		ActorID:         actorID,
		ExpectedVersion: input.ExpectedVersion,
		LecturerUserID:  input.LecturerUserID,
		Previous:        *current,
	})
}

func (s *Service) ChangeOfferingStatus(
	ctx context.Context,
	claims *AccessClaims,
	offeringID uuid.UUID,
	action string,
	expectedVersion int64,
) (*CourseOffering, error) {
	actorID, err := requireClaims(claims)
	if err != nil {
		return nil, err
	}
	if expectedVersion <= 0 {
		return nil, newValidationError("version", "must be provided and positive")
	}

	next, ok := offeringTransitionForAction(action)
	if !ok {
		return nil, newValidationError("action", "is not supported")
	}
	if action == "restore" {
		if _, err := authorizePermission(claims, PermCoursesManage); err != nil {
			return nil, err
		}
	} else if _, err := authorizePermission(claims, PermOfferingsManage); err != nil {
		return nil, err
	}

	current, err := s.store.GetOffering(ctx, offeringID)
	if err != nil {
		return nil, err
	}
	if current.Version != expectedVersion {
		return nil, ErrVersionConflict
	}

	admin := HasAnyRole(claims, RoleAdmin)
	if !admin && !HasAnyRole(claims, RoleLecturer) {
		return nil, ErrForbidden
	}
	if !admin && action != "restore" &&
		(current.LecturerUserID == nil || *current.LecturerUserID != actorID) {
		return nil, ErrForbidden
	}

	if !current.Status.CanTransitionTo(next) {
		return nil, ErrInvalidTransition
	}
	if action == "restore" && current.Status != OfferingArchived {
		return nil, ErrInvalidTransition
	}

	course, err := s.store.GetCourse(ctx, current.CourseID)
	if err != nil {
		return nil, err
	}
	switch action {
	case "publish":
		if course.Status != CourseActive {
			return nil, ErrCourseNotActive
		}
		if current.LecturerUserID == nil {
			return nil, newValidationError(
				"lecturer_user_id", "must be assigned before publication",
			)
		}
		if err := s.ensureEligibleLecturer(ctx, *current.LecturerUserID); err != nil {
			return nil, err
		}
	case "restore":
		if course.Status == CourseArchived {
			return nil, ErrCourseArchived
		}
	}
	if !admin && action != "publish" && action != "restore" {
		if err := s.ensureEligibleLecturer(ctx, actorID); err != nil {
			return nil, err
		}
	}

	return s.store.ChangeOfferingStatus(ctx, ChangeOfferingStatusRecord{
		OfferingID:      offeringID,
		ActorID:         actorID,
		ExpectedVersion: expectedVersion,
		FromStatus:      current.Status,
		ToStatus:        next,
		Action:          action,
	})
}

func (s *Service) ensureEligibleLecturer(
	ctx context.Context,
	userID uuid.UUID,
) error {
	if s.lecturers == nil {
		return ErrUserServiceUnavailable
	}
	return s.lecturers.EnsureEligibleLecturer(ctx, userID)
}

type offeringInput struct {
	AcademicYear       string
	Semester           string
	StartDate          string
	EndDate            string
	EnrollmentOpensAt  *time.Time
	EnrollmentClosesAt *time.Time
	EnrollmentCapacity *int
}

type offeringFields struct {
	AcademicYear       string
	Semester           string
	StartDate          Date
	EndDate            Date
	EnrollmentOpensAt  *time.Time
	EnrollmentClosesAt *time.Time
	EnrollmentCapacity *int
}

func validateOfferingFields(input offeringInput) (offeringFields, error) {
	academicYear, err := validatePeriodValue("academic_year", input.AcademicYear)
	if err != nil {
		return offeringFields{}, err
	}
	semester, err := validatePeriodValue("semester", input.Semester)
	if err != nil {
		return offeringFields{}, err
	}

	start, err := ParseDate(input.StartDate)
	if err != nil {
		return offeringFields{}, newValidationError("start_date", "must use YYYY-MM-DD format")
	}
	end, err := ParseDate(input.EndDate)
	if err != nil {
		return offeringFields{}, newValidationError("end_date", "must use YYYY-MM-DD format")
	}
	if end.Before(start.Time) {
		return offeringFields{}, newValidationError(
			"end_date", "must not precede start_date",
		)
	}

	opens := normalizeOptionalTime(input.EnrollmentOpensAt)
	closes := normalizeOptionalTime(input.EnrollmentClosesAt)
	if opens != nil && closes != nil && !closes.After(*opens) {
		return offeringFields{}, newValidationError(
			"enrollment_closes_at", "must be later than enrollment_opens_at",
		)
	}

	if input.EnrollmentCapacity != nil &&
		(*input.EnrollmentCapacity < 1 || *input.EnrollmentCapacity > 100_000) {
		return offeringFields{}, newValidationError(
			"enrollment_capacity", "must be between 1 and 100000 when provided",
		)
	}

	return offeringFields{
		AcademicYear:       academicYear,
		Semester:           semester,
		StartDate:          start,
		EndDate:            end,
		EnrollmentOpensAt:  opens,
		EnrollmentClosesAt: closes,
		EnrollmentCapacity: input.EnrollmentCapacity,
	}, nil
}

func validatePublishedOfferingUpdate(
	current *CourseOffering,
	next offeringFields,
) error {
	if next.AcademicYear != current.AcademicYear ||
		next.Semester != current.Semester ||
		!sameDate(next.StartDate, current.StartDate) ||
		!sameDate(next.EndDate, current.EndDate) {
		return ErrFieldLocked
	}
	if !sameOptionalTime(next.EnrollmentOpensAt, current.EnrollmentOpensAt) {
		return ErrFieldLocked
	}
	if !sameOptionalTime(next.EnrollmentClosesAt, current.EnrollmentClosesAt) {
		if current.EnrollmentClosesAt != nil &&
			(next.EnrollmentClosesAt == nil ||
				next.EnrollmentClosesAt.Before(*current.EnrollmentClosesAt)) {
			return ErrFieldLocked
		}
	}
	if current.EnrollmentCapacity == nil {
		if next.EnrollmentCapacity != nil {
			return ErrFieldLocked
		}
	} else if next.EnrollmentCapacity == nil ||
		*next.EnrollmentCapacity < *current.EnrollmentCapacity {
		return ErrFieldLocked
	}
	return nil
}

func validateCourseFields(
	code, title, description string,
	credits int,
) (string, string, string, error) {
	normalizedCode, err := normalizeCourseCode(code)
	if err != nil {
		return "", "", "", err
	}

	title = strings.TrimSpace(title)
	if title == "" {
		return "", "", "", newValidationError("title", "is required")
	}
	if utf8.RuneCountInString(title) > 200 {
		return "", "", "", newValidationError("title", "must not exceed 200 characters")
	}
	if containsControl(title, false) {
		return "", "", "", newValidationError("title", "must not contain control characters")
	}

	description = strings.TrimSpace(description)
	if utf8.RuneCountInString(description) > 5000 {
		return "", "", "", newValidationError(
			"description", "must not exceed 5000 characters",
		)
	}
	if containsControl(description, true) {
		return "", "", "", newValidationError(
			"description", "must not contain unsupported control characters",
		)
	}

	if credits < 1 || credits > 30 {
		return "", "", "", newValidationError(
			"credits", "must be between 1 and 30",
		)
	}

	return normalizedCode, title, description, nil
}

func normalizeCourseCode(value string) (string, error) {
	value = strings.ToUpper(strings.TrimSpace(value))
	if len(value) < 2 || len(value) > 32 {
		return "", newValidationError("code", "must be between 2 and 32 characters")
	}
	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-', r == '_':
		default:
			return "", newValidationError(
				"code", "must contain only letters, digits, hyphens, and underscores",
			)
		}
	}
	return value, nil
}

func validatePeriodValue(field, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", newValidationError(field, "is required")
	}
	if utf8.RuneCountInString(value) > 20 {
		return "", newValidationError(field, "must not exceed 20 characters")
	}
	if containsControl(value, false) {
		return "", newValidationError(field, "must not contain control characters")
	}
	return value, nil
}

func containsControl(value string, allowWhitespace bool) bool {
	for _, r := range value {
		if !unicode.IsControl(r) {
			continue
		}
		if allowWhitespace && (r == '\n' || r == '\r' || r == '\t') {
			continue
		}
		return true
	}
	return false
}

func normalizeOptionalTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	normalized := value.UTC()
	return &normalized
}

func sameOptionalTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}

func sameDate(a, b Date) bool {
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}

func validateCourseSort(value string) (string, error) {
	switch value {
	case "", "code", "title", "created_at", "updated_at":
		if value == "" {
			return "code", nil
		}
		return value, nil
	default:
		return "", newValidationError("sort", "is not supported")
	}
}

func validateOfferingSort(value string) (string, error) {
	switch value {
	case "", "start_date", "created_at", "updated_at", "academic_year", "course_code":
		if value == "" {
			return "start_date", nil
		}
		return value, nil
	default:
		return "", newValidationError("sort", "is not supported")
	}
}

func validateOrder(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "asc":
		return "asc", nil
	case "desc":
		return "desc", nil
	default:
		return "", newValidationError("order", "must be asc or desc")
	}
}

func applyPagination(limit, offset *int) {
	if *limit == 0 {
		*limit = 20
	}
	if *limit < 1 {
		*limit = 1
	}
	if *limit > 100 {
		*limit = 100
	}
	if *offset < 0 {
		*offset = 0
	}
}

func requireClaims(claims *AccessClaims) (uuid.UUID, error) {
	if claims == nil {
		return uuid.Nil, ErrUnauthorized
	}
	userID, err := claims.UserID()
	if err != nil {
		return uuid.Nil, ErrUnauthorized
	}
	return userID, nil
}

func authorizePermission(
	claims *AccessClaims,
	permission string,
) (uuid.UUID, error) {
	userID, err := requireClaims(claims)
	if err != nil {
		return uuid.Nil, err
	}
	if !HasAnyPermission(claims, permission) {
		return uuid.Nil, ErrForbidden
	}
	if permission == PermCoursesManage && !HasAnyRole(claims, RoleAdmin) {
		return uuid.Nil, ErrForbidden
	}
	if permission == PermOfferingsManage &&
		!HasAnyRole(claims, RoleAdmin, RoleLecturer) {
		return uuid.Nil, ErrForbidden
	}
	return userID, nil
}

func canReadAllCourses(claims *AccessClaims) bool {
	return HasAnyRole(claims, RoleAdmin)
}

func canReadAllOfferings(claims *AccessClaims) bool {
	return canReadAllCourses(claims)
}
