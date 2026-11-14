package course

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type fakeStore struct {
	createCourseFn         func(context.Context, *Course) error
	getCourseFn            func(context.Context, uuid.UUID) (*Course, error)
	updateCourseFn         func(context.Context, UpdateCourseRecord) (*Course, error)
	changeCourseStatusFn   func(context.Context, ChangeCourseStatusRecord) (*Course, error)
	listCoursesFn          func(context.Context, CourseFilter) (*CourseList, error)
	getOfferingFn          func(context.Context, uuid.UUID) (*CourseOffering, error)
	createOfferingFn       func(context.Context, *CourseOffering) error
	updateOfferingFn       func(context.Context, UpdateOfferingRecord) (*CourseOffering, error)
	changeOfferingStatusFn func(context.Context, ChangeOfferingStatusRecord) (*CourseOffering, error)
	assignLecturerFn       func(context.Context, AssignLecturerRecord) (*CourseOffering, error)
	listOfferingsFn        func(context.Context, OfferingFilter) (*OfferingList, error)
}

func (f *fakeStore) Ping(context.Context) error {
	return errors.New("unexpected Ping call")
}

func (f *fakeStore) CreateCourse(ctx context.Context, item *Course) error {
	if f.createCourseFn == nil {
		return errors.New("unexpected CreateCourse call")
	}
	return f.createCourseFn(ctx, item)
}

func (f *fakeStore) GetCourse(ctx context.Context, id uuid.UUID) (*Course, error) {
	if f.getCourseFn == nil {
		return nil, errors.New("unexpected GetCourse call")
	}
	return f.getCourseFn(ctx, id)
}

func (f *fakeStore) UpdateCourse(ctx context.Context, input UpdateCourseRecord) (*Course, error) {
	if f.updateCourseFn == nil {
		return nil, errors.New("unexpected UpdateCourse call")
	}
	return f.updateCourseFn(ctx, input)
}

func (f *fakeStore) ChangeCourseStatus(ctx context.Context, input ChangeCourseStatusRecord) (*Course, error) {
	if f.changeCourseStatusFn == nil {
		return nil, errors.New("unexpected ChangeCourseStatus call")
	}
	return f.changeCourseStatusFn(ctx, input)
}

func (f *fakeStore) ListCourses(ctx context.Context, filter CourseFilter) (*CourseList, error) {
	if f.listCoursesFn == nil {
		return nil, errors.New("unexpected ListCourses call")
	}
	return f.listCoursesFn(ctx, filter)
}

func (f *fakeStore) GetOffering(ctx context.Context, id uuid.UUID) (*CourseOffering, error) {
	if f.getOfferingFn == nil {
		return nil, errors.New("unexpected GetOffering call")
	}
	return f.getOfferingFn(ctx, id)
}

func (f *fakeStore) CreateOffering(ctx context.Context, item *CourseOffering) error {
	if f.createOfferingFn == nil {
		return errors.New("unexpected CreateOffering call")
	}
	return f.createOfferingFn(ctx, item)
}

func (f *fakeStore) UpdateOffering(ctx context.Context, input UpdateOfferingRecord) (*CourseOffering, error) {
	if f.updateOfferingFn == nil {
		return nil, errors.New("unexpected UpdateOffering call")
	}
	return f.updateOfferingFn(ctx, input)
}

func (f *fakeStore) ChangeOfferingStatus(ctx context.Context, input ChangeOfferingStatusRecord) (*CourseOffering, error) {
	if f.changeOfferingStatusFn == nil {
		return nil, errors.New("unexpected ChangeOfferingStatus call")
	}
	return f.changeOfferingStatusFn(ctx, input)
}

func (f *fakeStore) AssignLecturer(ctx context.Context, input AssignLecturerRecord) (*CourseOffering, error) {
	if f.assignLecturerFn == nil {
		return nil, errors.New("unexpected AssignLecturer call")
	}
	return f.assignLecturerFn(ctx, input)
}

func (f *fakeStore) ListOfferings(ctx context.Context, filter OfferingFilter) (*OfferingList, error) {
	if f.listOfferingsFn == nil {
		return nil, errors.New("unexpected ListOfferings call")
	}
	return f.listOfferingsFn(ctx, filter)
}

type fakeEligibility struct {
	err   error
	calls []uuid.UUID
}

func (f *fakeEligibility) EnsureEligibleLecturer(_ context.Context, userID uuid.UUID) error {
	f.calls = append(f.calls, userID)
	return f.err
}

func testAccessClaims(
	userID uuid.UUID,
	role string,
	permissions ...string,
) *AccessClaims {
	return &AccessClaims{
		SessionID:   uuid.NewString(),
		Roles:       []string{role},
		Permissions: permissions,
		TokenType:   "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			Issuer:    "university-auth",
			Audience:  jwt.ClaimStrings{"university-api"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
}

func testDate(t *testing.T, value string) Date {
	t.Helper()
	parsed, err := ParseDate(value)
	if err != nil {
		t.Fatalf("parse date: %v", err)
	}
	return parsed
}

func TestStudentCannotUseAccidentallyGrantedManagementPermissions(t *testing.T) {
	studentID := uuid.New()
	claims := testAccessClaims(studentID, RoleStudent, PermCoursesManage, PermOfferingsManage)
	offering := &CourseOffering{ID: uuid.New(), LecturerUserID: &studentID, Status: OfferingDraft, Version: 1}
	store := &fakeStore{getOfferingFn: func(context.Context, uuid.UUID) (*CourseOffering, error) {
		return offering, nil
	}}
	service := NewService(store, &fakeEligibility{})
	if _, err := service.CreateCourse(context.Background(), claims, CreateCourseInput{}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("student create course: %v", err)
	}
	if _, err := service.GetOffering(context.Background(), claims, offering.ID); !errors.Is(err, ErrOfferingNotFound) {
		t.Fatalf("student assigned draft read: %v", err)
	}
	if _, err := service.ChangeOfferingStatus(context.Background(), claims, offering.ID, "archive", 1); !errors.Is(err, ErrForbidden) {
		t.Fatalf("student offering mutation: %v", err)
	}
}

func TestCreateCourseNormalizesAndAuthorizes(t *testing.T) {
	actorID := uuid.New()
	var created *Course
	store := &fakeStore{
		createCourseFn: func(_ context.Context, item *Course) error {
			created = item
			item.Version = 1
			return nil
		},
	}
	service := NewService(store, nil)
	claims := testAccessClaims(actorID, RoleAdmin, PermCoursesManage)

	item, err := service.CreateCourse(context.Background(), claims, CreateCourseInput{
		Code:        "  cs-101  ",
		Title:       "  Introduction to Computer Science  ",
		Description: "  Foundations  ",
		Credits:     3,
	})
	if err != nil {
		t.Fatalf("create course: %v", err)
	}
	if created == nil || item != created {
		t.Fatal("expected store to receive the created course")
	}
	if item.Code != "CS-101" {
		t.Errorf("code = %q", item.Code)
	}
	if item.Title != "Introduction to Computer Science" {
		t.Errorf("title = %q", item.Title)
	}
	if item.Description != "Foundations" {
		t.Errorf("description = %q", item.Description)
	}
	if item.Status != CourseDraft {
		t.Errorf("status = %q", item.Status)
	}
	if item.CreatedBy != actorID || item.UpdatedBy != actorID {
		t.Error("expected actor to be recorded as creator and updater")
	}

	_, err = service.CreateCourse(context.Background(),
		testAccessClaims(actorID, RoleLecturer, PermOfferingsManage),
		CreateCourseInput{Code: "CS-102", Title: "Title", Credits: 3})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestCreateCourseValidation(t *testing.T) {
	service := NewService(&fakeStore{}, nil)
	claims := testAccessClaims(uuid.New(), RoleAdmin, PermCoursesManage)

	tests := []struct {
		name  string
		input CreateCourseInput
		field string
	}{
		{
			name:  "invalid code",
			input: CreateCourseInput{Code: "CS 101", Title: "Title", Credits: 3},
			field: "code",
		},
		{
			name:  "short code",
			input: CreateCourseInput{Code: "A", Title: "Title", Credits: 3},
			field: "code",
		},
		{
			name:  "empty title",
			input: CreateCourseInput{Code: "CS-101", Title: "   ", Credits: 3},
			field: "title",
		},
		{
			name:  "invalid credits",
			input: CreateCourseInput{Code: "CS-101", Title: "Title", Credits: 0},
			field: "credits",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := service.CreateCourse(context.Background(), claims, test.input)
			var validation *ValidationError
			if !errors.As(err, &validation) {
				t.Fatalf("expected validation error, got %v", err)
			}
			if validation.Field != test.field {
				t.Errorf("field = %q, want %q", validation.Field, test.field)
			}
		})
	}
}

func TestCourseReadVisibility(t *testing.T) {
	store := &fakeStore{
		getCourseFn: func(context.Context, uuid.UUID) (*Course, error) {
			return &Course{ID: uuid.New(), Status: CourseDraft}, nil
		},
	}
	service := NewService(store, nil)
	student := testAccessClaims(uuid.New(), RoleStudent)

	if _, err := service.GetCourse(context.Background(), student, uuid.New()); !errors.Is(err, ErrCourseNotFound) {
		t.Fatalf("expected hidden draft course, got %v", err)
	}

	admin := testAccessClaims(uuid.New(), RoleAdmin, PermCoursesManage)
	if _, err := service.GetCourse(context.Background(), admin, uuid.New()); err != nil {
		t.Fatalf("admin should read draft course: %v", err)
	}

	if _, err := service.ListCourses(context.Background(), student, CourseFilter{
		Status: CourseDraft,
		Limit:  20,
	}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden status filter, got %v", err)
	}

	var captured CourseFilter
	store.listCoursesFn = func(_ context.Context, filter CourseFilter) (*CourseList, error) {
		captured = filter
		return &CourseList{Limit: filter.Limit, Offset: filter.Offset}, nil
	}
	if _, err := service.ListCourses(context.Background(), student, CourseFilter{}); err != nil {
		t.Fatalf("list courses: %v", err)
	}
	if captured.Status != CourseActive {
		t.Errorf("expected catalogue status active, got %q", captured.Status)
	}
	if captured.Limit != 20 {
		t.Errorf("expected default limit 20, got %d", captured.Limit)
	}
}

func TestCreateOfferingValidatesFieldsAndLecturer(t *testing.T) {
	courseID := uuid.New()
	lecturerID := uuid.New()
	actorID := uuid.New()
	course := &Course{
		ID:     courseID,
		Code:   "CS-101",
		Title:  "Introduction",
		Status: CourseActive,
	}
	var created *CourseOffering
	store := &fakeStore{
		getCourseFn: func(context.Context, uuid.UUID) (*Course, error) {
			return course, nil
		},
		createOfferingFn: func(_ context.Context, item *CourseOffering) error {
			created = item
			return nil
		},
	}
	eligibility := &fakeEligibility{}
	service := NewService(store, eligibility)
	claims := testAccessClaims(actorID, RoleAdmin, PermCoursesManage)

	input := CreateOfferingInput{
		CourseID:           courseID,
		AcademicYear:       "2026-2027",
		Semester:           "Fall",
		StartDate:          "2026-09-01",
		EndDate:            "2026-12-20",
		EnrollmentCapacity: intPointer(100),
		LecturerUserID:     &lecturerID,
	}
	item, err := service.CreateOffering(context.Background(), claims, input)
	if err != nil {
		t.Fatalf("create offering: %v", err)
	}
	if created == nil || item != created {
		t.Fatal("expected store to receive created offering")
	}
	if item.Status != OfferingDraft || item.CourseCode != course.Code {
		t.Errorf("unexpected offering: %+v", item)
	}
	if len(eligibility.calls) != 1 || eligibility.calls[0] != lecturerID {
		t.Fatalf("expected eligibility check for %s, got %v", lecturerID, eligibility.calls)
	}

	eligibility.err = ErrLecturerIneligible
	_, err = service.CreateOffering(context.Background(), claims, input)
	if !errors.Is(err, ErrLecturerIneligible) {
		t.Fatalf("expected ineligible lecturer, got %v", err)
	}

	invalidDateInput := input
	invalidDateInput.StartDate = "2026-12-21"
	if _, err := service.CreateOffering(
		context.Background(), claims, invalidDateInput,
	); err == nil {
		t.Fatal("expected invalid dates to be rejected")
	}

	invalidCapacityInput := input
	invalidCapacityInput.EnrollmentCapacity = intPointer(0)
	if _, err := service.CreateOffering(
		context.Background(), claims, invalidCapacityInput,
	); err == nil {
		t.Fatal("expected invalid capacity to be rejected")
	}

	course.Status = CourseArchived
	if _, err := service.CreateOffering(
		context.Background(), claims, input,
	); !errors.Is(err, ErrCourseArchived) {
		t.Fatalf("expected archived course rejection, got %v", err)
	}
}

func TestUpdatePublishedOfferingEnforcesFieldLocks(t *testing.T) {
	lecturerID := uuid.New()
	opensAt := time.Now().UTC().Add(24 * time.Hour)
	closesAt := opensAt.Add(14 * 24 * time.Hour)
	current := &CourseOffering{
		ID:                 uuid.New(),
		CourseID:           uuid.New(),
		AcademicYear:       "2026-2027",
		Semester:           "Fall",
		StartDate:          testDate(t, "2026-09-01"),
		EndDate:            testDate(t, "2026-12-20"),
		EnrollmentOpensAt:  &opensAt,
		EnrollmentClosesAt: &closesAt,
		EnrollmentCapacity: intPointer(100),
		LecturerUserID:     &lecturerID,
		Status:             OfferingPublished,
		Version:            2,
	}
	store := &fakeStore{
		getOfferingFn: func(context.Context, uuid.UUID) (*CourseOffering, error) {
			return current, nil
		},
		updateOfferingFn: func(_ context.Context, input UpdateOfferingRecord) (*CourseOffering, error) {
			return current, nil
		},
	}
	service := NewService(store, &fakeEligibility{})
	claims := testAccessClaims(lecturerID, RoleLecturer, PermOfferingsManage)

	baseInput := UpdateOfferingInput{
		ExpectedVersion:    2,
		AcademicYear:       current.AcademicYear,
		Semester:           current.Semester,
		StartDate:          current.StartDate.String(),
		EndDate:            current.EndDate.String(),
		EnrollmentOpensAt:  current.EnrollmentOpensAt,
		EnrollmentClosesAt: current.EnrollmentClosesAt,
		EnrollmentCapacity: intPointer(120),
	}
	if _, err := service.UpdateOffering(
		context.Background(), claims, current.ID, baseInput,
	); err != nil {
		t.Fatalf("capacity increase should be allowed: %v", err)
	}

	extended := baseInput
	extendedClose := closesAt.Add(7 * 24 * time.Hour)
	extended.EnrollmentClosesAt = &extendedClose
	if _, err := service.UpdateOffering(
		context.Background(), claims, current.ID, extended,
	); err != nil {
		t.Fatalf("enrollment extension should be allowed: %v", err)
	}

	shortened := baseInput
	earlierClose := closesAt.Add(-time.Hour)
	shortened.EnrollmentClosesAt = &earlierClose
	if _, err := service.UpdateOffering(
		context.Background(), claims, current.ID, shortened,
	); !errors.Is(err, ErrFieldLocked) {
		t.Fatalf("expected shortened enrollment to be locked, got %v", err)
	}

	decreased := baseInput
	decreased.EnrollmentCapacity = intPointer(50)
	if _, err := service.UpdateOffering(
		context.Background(), claims, current.ID, decreased,
	); !errors.Is(err, ErrFieldLocked) {
		t.Fatalf("expected capacity decrease to be locked, got %v", err)
	}

	current.EnrollmentCapacity = nil
	fromUnlimited := baseInput
	fromUnlimited.EnrollmentCapacity = intPointer(120)
	if _, err := service.UpdateOffering(
		context.Background(), claims, current.ID, fromUnlimited,
	); !errors.Is(err, ErrFieldLocked) {
		t.Fatalf("expected unlimited-to-finite capacity to be locked, got %v", err)
	}
	current.EnrollmentCapacity = intPointer(100)

	changedDates := baseInput
	changedDates.StartDate = "2026-09-02"
	if _, err := service.UpdateOffering(
		context.Background(), claims, current.ID, changedDates,
	); !errors.Is(err, ErrFieldLocked) {
		t.Fatalf("expected start date change to be locked, got %v", err)
	}

	unlockedOpens := baseInput
	unlockedOpens.EnrollmentOpensAt = nil
	if _, err := service.UpdateOffering(
		context.Background(), claims, current.ID, unlockedOpens,
	); !errors.Is(err, ErrFieldLocked) {
		t.Fatalf("expected enrollment opening change to be locked, got %v", err)
	}

	otherLecturer := testAccessClaims(uuid.New(), RoleLecturer, PermOfferingsManage)
	if _, err := service.UpdateOffering(
		context.Background(), otherLecturer, current.ID, baseInput,
	); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected other lecturer to be forbidden, got %v", err)
	}

	current.Status = OfferingClosed
	if _, err := service.UpdateOffering(
		context.Background(), claims, current.ID, baseInput,
	); !errors.Is(err, ErrOfferingImmutable) {
		t.Fatalf("expected closed offering to be immutable, got %v", err)
	}
}

func TestPublishOfferingRequirements(t *testing.T) {
	lecturerID := uuid.New()
	courseID := uuid.New()
	current := &CourseOffering{
		ID:             uuid.New(),
		CourseID:       courseID,
		LecturerUserID: &lecturerID,
		Status:         OfferingDraft,
		Version:        1,
	}
	course := &Course{ID: courseID, Status: CourseActive}
	var transition ChangeOfferingStatusRecord
	store := &fakeStore{
		getOfferingFn: func(context.Context, uuid.UUID) (*CourseOffering, error) {
			return current, nil
		},
		getCourseFn: func(context.Context, uuid.UUID) (*Course, error) {
			return course, nil
		},
		changeOfferingStatusFn: func(_ context.Context, input ChangeOfferingStatusRecord) (*CourseOffering, error) {
			transition = input
			return current, nil
		},
	}
	service := NewService(store, &fakeEligibility{})
	claims := testAccessClaims(lecturerID, RoleLecturer, PermOfferingsManage)

	if _, err := service.ChangeOfferingStatus(
		context.Background(), claims, current.ID, "publish", 1,
	); err != nil {
		t.Fatalf("publish offering: %v", err)
	}
	if transition.FromStatus != OfferingDraft ||
		transition.ToStatus != OfferingPublished {
		t.Fatalf("unexpected transition: %+v", transition)
	}

	course.Status = CourseDraft
	if _, err := service.ChangeOfferingStatus(
		context.Background(), claims, current.ID, "publish", 1,
	); !errors.Is(err, ErrCourseNotActive) {
		t.Fatalf("expected inactive course rejection, got %v", err)
	}
	course.Status = CourseActive

	current.LecturerUserID = nil
	if _, err := service.ChangeOfferingStatus(
		context.Background(), claims, current.ID, "publish", 1,
	); err == nil {
		t.Fatal("expected missing lecturer rejection")
	}
	current.LecturerUserID = &lecturerID

	current.Status = OfferingClosed
	if _, err := service.ChangeOfferingStatus(
		context.Background(), claims, current.ID, "publish", 1,
	); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected invalid transition, got %v", err)
	}

	current.Status = OfferingDraft
	if _, err := service.ChangeOfferingStatus(
		context.Background(), claims, current.ID, "publish", 2,
	); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("expected version conflict, got %v", err)
	}

	otherClaims := testAccessClaims(uuid.New(), RoleLecturer, PermOfferingsManage)
	if _, err := service.ChangeOfferingStatus(
		context.Background(), otherClaims, current.ID, "publish", 1,
	); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected unassigned lecturer rejection, got %v", err)
	}
}

func TestRestoreOfferingRequiresCourseManagement(t *testing.T) {
	courseID := uuid.New()
	current := &CourseOffering{
		ID:       uuid.New(),
		CourseID: courseID,
		Status:   OfferingArchived,
		Version:  3,
	}
	course := &Course{ID: courseID, Status: CourseActive}
	var called bool
	store := &fakeStore{
		getOfferingFn: func(context.Context, uuid.UUID) (*CourseOffering, error) {
			return current, nil
		},
		getCourseFn: func(context.Context, uuid.UUID) (*Course, error) {
			return course, nil
		},
		changeOfferingStatusFn: func(_ context.Context, input ChangeOfferingStatusRecord) (*CourseOffering, error) {
			called = true
			return current, nil
		},
	}
	service := NewService(store, nil)

	lecturer := testAccessClaims(uuid.New(), RoleLecturer, PermOfferingsManage)
	if _, err := service.ChangeOfferingStatus(
		context.Background(), lecturer, current.ID, "restore", 3,
	); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected restore to require course management, got %v", err)
	}
	if called {
		t.Fatal("store should not be called for forbidden restore")
	}

	admin := testAccessClaims(uuid.New(), RoleAdmin, PermCoursesManage, PermOfferingsManage)
	if _, err := service.ChangeOfferingStatus(
		context.Background(), admin, current.ID, "restore", 3,
	); err != nil {
		t.Fatalf("restore offering: %v", err)
	}
	if !called {
		t.Fatal("expected offering restore to be persisted")
	}

	course.Status = CourseArchived
	if _, err := service.ChangeOfferingStatus(
		context.Background(), admin, current.ID, "restore", 3,
	); !errors.Is(err, ErrCourseArchived) {
		t.Fatalf("expected archived course rejection, got %v", err)
	}
}

func TestAssignLecturerChecksEligibility(t *testing.T) {
	actorID := uuid.New()
	lecturerID := uuid.New()
	current := &CourseOffering{
		ID:      uuid.New(),
		Status:  OfferingDraft,
		Version: 1,
	}
	var assigned AssignLecturerRecord
	store := &fakeStore{
		getOfferingFn: func(context.Context, uuid.UUID) (*CourseOffering, error) {
			return current, nil
		},
		assignLecturerFn: func(_ context.Context, input AssignLecturerRecord) (*CourseOffering, error) {
			assigned = input
			current.LecturerUserID = &input.LecturerUserID
			current.Version++
			return current, nil
		},
	}
	eligibility := &fakeEligibility{}
	service := NewService(store, eligibility)
	claims := testAccessClaims(actorID, RoleAdmin, PermCoursesManage)

	if _, err := service.AssignLecturer(context.Background(), claims, current.ID,
		AssignLecturerInput{ExpectedVersion: 1, LecturerUserID: lecturerID},
	); err != nil {
		t.Fatalf("assign lecturer: %v", err)
	}
	if assigned.LecturerUserID != lecturerID || assigned.ActorID != actorID {
		t.Fatalf("unexpected assignment: %+v", assigned)
	}

	eligibility.err = ErrLecturerIneligible
	otherID := uuid.New()
	if _, err := service.AssignLecturer(context.Background(), claims, current.ID,
		AssignLecturerInput{ExpectedVersion: 2, LecturerUserID: otherID},
	); !errors.Is(err, ErrLecturerIneligible) {
		t.Fatalf("expected ineligible lecturer, got %v", err)
	}

	current.Status = OfferingClosed
	if _, err := service.AssignLecturer(context.Background(), claims, current.ID,
		AssignLecturerInput{ExpectedVersion: 2, LecturerUserID: lecturerID},
	); !errors.Is(err, ErrOfferingImmutable) {
		t.Fatalf("expected immutable closed offering, got %v", err)
	}
}

func TestCourseLifecyclePermissionsAndTransitions(t *testing.T) {
	courseID := uuid.New()
	current := &Course{ID: courseID, Status: CourseDraft, Version: 1}
	var transition ChangeCourseStatusRecord
	store := &fakeStore{
		getCourseFn: func(context.Context, uuid.UUID) (*Course, error) {
			return current, nil
		},
		changeCourseStatusFn: func(_ context.Context, input ChangeCourseStatusRecord) (*Course, error) {
			transition = input
			current.Status = input.ToStatus
			current.Version++
			return current, nil
		},
	}
	service := NewService(store, nil)

	lecturer := testAccessClaims(uuid.New(), RoleLecturer, PermOfferingsManage)
	if _, err := service.ChangeCourseStatus(
		context.Background(), lecturer, courseID, "publish", 1,
	); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected course management permission, got %v", err)
	}

	admin := testAccessClaims(uuid.New(), RoleAdmin, PermCoursesManage, PermOfferingsManage)
	if _, err := service.ChangeCourseStatus(
		context.Background(), admin, courseID, "publish", 1,
	); err != nil {
		t.Fatalf("publish course: %v", err)
	}
	if transition.FromStatus != CourseDraft || transition.ToStatus != CourseActive {
		t.Fatalf("unexpected transition: %+v", transition)
	}

	if _, err := service.ChangeCourseStatus(
		context.Background(), admin, courseID, "publish", 2,
	); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected invalid transition, got %v", err)
	}

	if _, err := service.ChangeCourseStatus(
		context.Background(), admin, courseID, "archive", 2,
	); err != nil {
		t.Fatalf("archive course: %v", err)
	}
	if transition.ToStatus != CourseArchived {
		t.Fatalf("to status = %q", transition.ToStatus)
	}

	if _, err := service.ChangeCourseStatus(
		context.Background(), admin, courseID, "restore", 3,
	); err != nil {
		t.Fatalf("restore course: %v", err)
	}
	if transition.ToStatus != CourseActive {
		t.Fatalf("to status = %q", transition.ToStatus)
	}
}

func TestUpdateCourseLocksCodeAfterDraft(t *testing.T) {
	courseID := uuid.New()
	current := &Course{
		ID:      courseID,
		Code:    "CS-101",
		Title:   "Old",
		Credits: 3,
		Status:  CourseActive,
		Version: 2,
	}
	store := &fakeStore{
		getCourseFn: func(context.Context, uuid.UUID) (*Course, error) {
			return current, nil
		},
		updateCourseFn: func(_ context.Context, input UpdateCourseRecord) (*Course, error) {
			return current, nil
		},
	}
	service := NewService(store, nil)
	claims := testAccessClaims(uuid.New(), RoleAdmin, PermCoursesManage)

	input := UpdateCourseInput{
		ExpectedVersion: 2,
		Code:            "CS-101",
		Title:           "New title",
		Credits:         3,
	}
	if _, err := service.UpdateCourse(
		context.Background(), claims, courseID, input,
	); err != nil {
		t.Fatalf("update non-code field: %v", err)
	}

	input.Code = "CS-102"
	if _, err := service.UpdateCourse(
		context.Background(), claims, courseID, input,
	); !errors.Is(err, ErrCourseCodeLocked) {
		t.Fatalf("expected locked course code, got %v", err)
	}

	input.Code = "CS-101"
	current.Status = CourseArchived
	if _, err := service.UpdateCourse(
		context.Background(), claims, courseID, input,
	); !errors.Is(err, ErrCourseArchived) {
		t.Fatalf("expected archived course rejection, got %v", err)
	}
}

func TestCreateOfferingFailsClosedWithoutEligibilityClient(t *testing.T) {
	lecturerID := uuid.New()
	store := &fakeStore{
		getCourseFn: func(context.Context, uuid.UUID) (*Course, error) {
			return &Course{ID: uuid.New(), Status: CourseActive}, nil
		},
	}
	service := NewService(store, nil)
	claims := testAccessClaims(uuid.New(), RoleAdmin, PermCoursesManage)

	_, err := service.CreateOffering(context.Background(), claims, CreateOfferingInput{
		CourseID:       uuid.New(),
		AcademicYear:   "2026-2027",
		Semester:       "Fall",
		StartDate:      "2026-09-01",
		EndDate:        "2026-12-20",
		LecturerUserID: &lecturerID,
	})
	if !errors.Is(err, ErrUserServiceUnavailable) {
		t.Fatalf("expected fail-closed dependency error, got %v", err)
	}
}

func TestListOfferingsVisibility(t *testing.T) {
	student := testAccessClaims(uuid.New(), RoleStudent)
	var captured OfferingFilter
	store := &fakeStore{
		listOfferingsFn: func(_ context.Context, filter OfferingFilter) (*OfferingList, error) {
			captured = filter
			return &OfferingList{}, nil
		},
	}
	service := NewService(store, nil)

	if _, err := service.ListOfferings(context.Background(), student,
		OfferingFilter{Status: OfferingDraft},
	); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden draft filter, got %v", err)
	}

	if _, err := service.ListOfferings(context.Background(), student,
		OfferingFilter{},
	); err != nil {
		t.Fatalf("list published offerings: %v", err)
	}
	if !captured.PublishedOnly || captured.Status != OfferingPublished {
		t.Fatalf("expected published-only filter, got %+v", captured)
	}

	if _, err := service.ListOfferings(context.Background(), student,
		OfferingFilter{AssignedToMe: true},
	); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected assigned_to_me permission check, got %v", err)
	}

	lecturer := testAccessClaims(uuid.New(), RoleLecturer, PermOfferingsManage)
	if _, err := service.ListOfferings(context.Background(), lecturer,
		OfferingFilter{AssignedToMe: true},
	); err != nil {
		t.Fatalf("list assigned offerings: %v", err)
	}
	if captured.AssignedToUserID == nil ||
		*captured.AssignedToUserID != mustUserID(t, lecturer) {
		t.Fatalf("unexpected assigned filter: %+v", captured)
	}
	if strings.TrimSpace(captured.Sort) == "" {
		t.Fatal("expected default offering sort")
	}
}

func mustUserID(t *testing.T, claims *AccessClaims) uuid.UUID {
	t.Helper()
	id, err := claims.UserID()
	if err != nil {
		t.Fatalf("parse claims user id: %v", err)
	}
	return id
}

func intPointer(value int) *int {
	return &value
}
