package enrollment

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type stubStore struct {
	create func(*Enrollment, *int) error
	get    func(uuid.UUID) (*Enrollment, error)
	active func(uuid.UUID, uuid.UUID) (*Enrollment, error)
	cancel func(uuid.UUID, int64, uuid.UUID) (*Enrollment, error)
	list   func(EnrollmentFilter) (*EnrollmentList, error)
}

func (s stubStore) Create(_ context.Context, e *Enrollment, capacity *int) error {
	if s.create == nil {
		return errors.New("unexpected Create")
	}
	return s.create(e, capacity)
}
func (s stubStore) GetByID(_ context.Context, id uuid.UUID) (*Enrollment, error) {
	if s.get == nil {
		return nil, errors.New("unexpected GetByID")
	}
	return s.get(id)
}
func (s stubStore) GetActive(_ context.Context, studentID, offeringID uuid.UUID) (*Enrollment, error) {
	if s.active == nil {
		return nil, errors.New("unexpected GetActive")
	}
	return s.active(studentID, offeringID)
}
func (s stubStore) Cancel(_ context.Context, id uuid.UUID, version int64, actorID uuid.UUID) (*Enrollment, error) {
	if s.cancel == nil {
		return nil, errors.New("unexpected Cancel")
	}
	return s.cancel(id, version, actorID)
}
func (s stubStore) List(_ context.Context, filter EnrollmentFilter) (*EnrollmentList, error) {
	if s.list == nil {
		return nil, errors.New("unexpected List")
	}
	return s.list(filter)
}

type stubUser func(uuid.UUID) error

func (f stubUser) EnsureApproved(_ context.Context, id uuid.UUID) error { return f(id) }

type stubRoles struct {
	student  func(uuid.UUID) error
	lecturer func(uuid.UUID) error
}

func (s stubRoles) EnsureActiveStudent(_ context.Context, id uuid.UUID) error {
	return s.student(id)
}
func (s stubRoles) EnsureActiveLecturer(_ context.Context, id uuid.UUID) error {
	return s.lecturer(id)
}

type stubOffering func(uuid.UUID) (*OfferingFacts, error)

func (f stubOffering) GetOffering(_ context.Context, id uuid.UUID) (*OfferingFacts, error) {
	return f(id)
}

func testService(
	store stubStore, user stubUser, roles stubRoles, offering stubOffering,
) *Service {
	return NewService(store, user, roles, offering)
}

func TestCreateEnrollmentChecksAuthoritiesWindowAndCapacity(t *testing.T) {
	studentID, offeringID := uuid.New(), uuid.New()
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	opens, closes, capacity := now.Add(-time.Hour), now.Add(time.Hour), 2
	calls := make([]string, 0)
	service := testService(
		stubStore{create: func(item *Enrollment, received *int) error {
			calls = append(calls, "store")
			if item.StudentUserID != studentID || item.OfferingID != offeringID ||
				item.CreatedByUserID != studentID || received == nil || *received != capacity {
				t.Fatalf("unexpected enrollment or capacity: %+v, %v", item, received)
			}
			item.Version = 1
			return nil
		}},
		func(id uuid.UUID) error { calls = append(calls, "user"); return nil },
		stubRoles{student: func(id uuid.UUID) error { calls = append(calls, "auth"); return nil }},
		func(id uuid.UUID) (*OfferingFacts, error) {
			calls = append(calls, "course")
			return &OfferingFacts{
				ID: offeringID, Status: "published", Version: 1,
				EnrollmentOpensAt: &opens, EnrollmentClosesAt: &closes,
				EnrollmentCapacity: &capacity,
			}, nil
		},
	)
	service.now = func() time.Time { return now }
	item, err := service.Create(context.Background(),
		Actor{UserID: studentID, Roles: []string{RoleStudent}}, studentID, offeringID)
	if err != nil || item.Version != 1 {
		t.Fatalf("create enrollment: %+v, %v", item, err)
	}
	if got := len(calls); got != 4 ||
		calls[0] != "user" || calls[1] != "auth" ||
		calls[2] != "course" || calls[3] != "store" {
		t.Fatalf("verification order: %v", calls)
	}
}

func TestCreateEnrollmentRejectsCrossStudentAndUnverifiedDependencies(t *testing.T) {
	studentID, otherID, offeringID := uuid.New(), uuid.New(), uuid.New()
	dependencyDown := errors.New("dependency down")
	for _, tc := range []struct {
		name  string
		actor Actor
		user  error
		role  error
		offer error
		want  error
	}{
		{"cross student", Actor{UserID: otherID, Roles: []string{RoleStudent}}, nil, nil, nil, ErrForbidden},
		{"unapproved", Actor{UserID: studentID, Roles: []string{RoleStudent}}, ErrStudentIneligible, nil, nil, ErrStudentIneligible},
		{"auth unavailable", Actor{UserID: studentID, Roles: []string{RoleStudent}}, nil, dependencyDown, nil, dependencyDown},
		{"course unavailable", Actor{UserID: studentID, Roles: []string{RoleStudent}}, nil, nil, dependencyDown, dependencyDown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := testService(stubStore{},
				func(uuid.UUID) error { return tc.user },
				stubRoles{student: func(uuid.UUID) error { return tc.role }},
				func(uuid.UUID) (*OfferingFacts, error) { return nil, tc.offer },
			)
			_, err := service.Create(context.Background(), tc.actor, studentID, offeringID)
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestCreateEnrollmentRequiresPublishedOpenOffering(t *testing.T) {
	studentID, offeringID := uuid.New(), uuid.New()
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name   string
		status string
		opens  *time.Time
		closes *time.Time
	}{
		{"draft", "draft", nil, nil},
		{"closed", "closed", nil, nil},
		{"early", "published", ptrTime(now.Add(time.Minute)), nil},
		{"late", "published", nil, ptrTime(now)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := testService(stubStore{},
				func(uuid.UUID) error { return nil },
				stubRoles{student: func(uuid.UUID) error { return nil }},
				func(uuid.UUID) (*OfferingFacts, error) {
					return &OfferingFacts{
						ID: offeringID, Status: tc.status, Version: 1,
						EnrollmentOpensAt: tc.opens, EnrollmentClosesAt: tc.closes,
					}, nil
				},
			)
			service.now = func() time.Time { return now }
			_, err := service.Create(context.Background(),
				Actor{UserID: studentID, Roles: []string{RoleStudent}},
				studentID, offeringID)
			if !errors.Is(err, ErrOfferingIneligible) {
				t.Fatalf("got %v, want offering ineligible", err)
			}
		})
	}
}

func ptrTime(value time.Time) *time.Time { return &value }

func TestLecturerAccessRequiresCourseAssignmentAndLiveEligibility(t *testing.T) {
	lecturerID, otherID, offeringID := uuid.New(), uuid.New(), uuid.New()
	var approved bool = true
	service := testService(
		stubStore{list: func(filter EnrollmentFilter) (*EnrollmentList, error) {
			if filter.OfferingID == nil || *filter.OfferingID != offeringID {
				t.Fatal("offering filter missing")
			}
			return &EnrollmentList{}, nil
		}},
		func(uuid.UUID) error {
			if !approved {
				return ErrStudentIneligible
			}
			return nil
		},
		stubRoles{lecturer: func(uuid.UUID) error { return nil }},
		func(uuid.UUID) (*OfferingFacts, error) {
			return &OfferingFacts{ID: offeringID, LecturerUserID: &lecturerID, Version: 1}, nil
		},
	)
	actor := Actor{UserID: lecturerID, Roles: []string{RoleLecturer},
		Permissions: []string{PermEnrollmentsRead}}
	if _, err := service.ListForOffering(context.Background(), actor, offeringID, EnrollmentFilter{}); err != nil {
		t.Fatalf("assigned lecturer denied: %v", err)
	}
	other := Actor{UserID: otherID, Roles: []string{RoleLecturer},
		Permissions: []string{PermEnrollmentsRead}}
	if _, err := service.ListForOffering(context.Background(), other, offeringID, EnrollmentFilter{}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("unrelated lecturer: %v", err)
	}
	approved = false
	if _, err := service.ListForOffering(context.Background(), actor, offeringID, EnrollmentFilter{}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("unapproved lecturer: %v", err)
	}
}

func TestCancelEnrollmentChecksOwnershipAndVersion(t *testing.T) {
	studentID, otherID, enrollmentID := uuid.New(), uuid.New(), uuid.New()
	service := testService(stubStore{
		get: func(id uuid.UUID) (*Enrollment, error) {
			return &Enrollment{ID: id, StudentUserID: studentID}, nil
		},
		cancel: func(id uuid.UUID, version int64, actorID uuid.UUID) (*Enrollment, error) {
			if version != 1 || actorID != studentID {
				t.Fatal("wrong cancellation arguments")
			}
			return nil, ErrVersionConflict
		},
	}, nil, stubRoles{}, nil)
	if _, err := service.Cancel(context.Background(),
		Actor{UserID: otherID, Roles: []string{RoleStudent}}, enrollmentID, 1); !errors.Is(err, ErrForbidden) {
		t.Fatalf("cross-student cancellation: %v", err)
	}
	if _, err := service.Cancel(context.Background(),
		Actor{UserID: studentID, Roles: []string{RoleStudent}}, enrollmentID, 1); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("version conflict: %v", err)
	}
}

func TestAssessmentMembershipChecksFailClosed(t *testing.T) {
	studentID, lecturerID, offeringID := uuid.New(), uuid.New(), uuid.New()
	dependencyDown := errors.New("authority unavailable")
	studentApproved := true
	lecturerApproved := true
	service := testService(
		stubStore{active: func(student, offering uuid.UUID) (*Enrollment, error) {
			if student != studentID || offering != offeringID {
				t.Fatal("unexpected enrollment lookup")
			}
			return &Enrollment{Status: EnrollmentActive}, nil
		}},
		func(id uuid.UUID) error {
			if id == studentID && !studentApproved || id == lecturerID && !lecturerApproved {
				return ErrStudentIneligible
			}
			return nil
		},
		stubRoles{
			student:  func(uuid.UUID) error { return nil },
			lecturer: func(uuid.UUID) error { return nil },
		},
		func(id uuid.UUID) (*OfferingFacts, error) {
			return &OfferingFacts{ID: id, LecturerUserID: &lecturerID, Version: 1}, nil
		},
	)
	if active, err := service.Active(context.Background(), studentID, offeringID); err != nil || !active {
		t.Fatalf("eligible student: active=%t, err=%v", active, err)
	}
	if active, err := service.ActiveLecturer(context.Background(), lecturerID, offeringID); err != nil || !active {
		t.Fatalf("eligible lecturer: active=%t, err=%v", active, err)
	}
	studentApproved = false
	if active, err := service.Active(context.Background(), studentID, offeringID); err != nil || active {
		t.Fatalf("unapproved student: active=%t, err=%v", active, err)
	}
	lecturerApproved = false
	if active, err := service.ActiveLecturer(context.Background(), lecturerID, offeringID); err != nil || active {
		t.Fatalf("unapproved lecturer: active=%t, err=%v", active, err)
	}
	service.users = stubUser(func(uuid.UUID) error { return dependencyDown })
	if active, err := service.Active(context.Background(), studentID, offeringID); active || !errors.Is(err, dependencyDown) {
		t.Fatalf("student authority outage: active=%t, err=%v", active, err)
	}
	if active, err := service.ActiveLecturer(context.Background(), lecturerID, offeringID); active || !errors.Is(err, dependencyDown) {
		t.Fatalf("lecturer authority outage: active=%t, err=%v", active, err)
	}
}
