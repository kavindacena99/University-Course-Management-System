package enrollment

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

const (
	RoleAdmin             = "ADMIN"
	RoleLecturer          = "LECTURER"
	RoleStudent           = "STUDENT"
	PermEnrollmentsRead   = "enrollments:read"
	PermEnrollmentsManage = "enrollments:manage"
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

type OfferingFacts struct {
	ID                 uuid.UUID  `json:"id"`
	Status             string     `json:"status"`
	EnrollmentOpensAt  *time.Time `json:"enrollment_opens_at"`
	EnrollmentClosesAt *time.Time `json:"enrollment_closes_at"`
	EnrollmentCapacity *int       `json:"enrollment_capacity"`
	LecturerUserID     *uuid.UUID `json:"lecturer_user_id"`
	Version            int64      `json:"version"`
}

type EnrollmentStore interface {
	Create(ctx context.Context, item *Enrollment, capacity *int) error
	GetByID(ctx context.Context, id uuid.UUID) (*Enrollment, error)
	GetActive(ctx context.Context, studentID, offeringID uuid.UUID) (*Enrollment, error)
	Cancel(ctx context.Context, id uuid.UUID, expectedVersion int64, actorID uuid.UUID) (*Enrollment, error)
	List(ctx context.Context, filter EnrollmentFilter) (*EnrollmentList, error)
}

type UserEligibility interface {
	EnsureApproved(ctx context.Context, userID uuid.UUID) error
}

type RoleEligibility interface {
	EnsureActiveStudent(ctx context.Context, userID uuid.UUID) error
	EnsureActiveLecturer(ctx context.Context, userID uuid.UUID) error
}

type OfferingReader interface {
	GetOffering(ctx context.Context, offeringID uuid.UUID) (*OfferingFacts, error)
}

type Service struct {
	store     EnrollmentStore
	users     UserEligibility
	roles     RoleEligibility
	offerings OfferingReader
	now       func() time.Time
}

func NewService(
	store EnrollmentStore,
	users UserEligibility,
	roles RoleEligibility,
	offerings OfferingReader,
) *Service {
	return &Service{
		store: store, users: users, roles: roles, offerings: offerings,
		now: time.Now,
	}
}

func (s *Service) Create(
	ctx context.Context, actor Actor, studentID, offeringID uuid.UUID,
) (*Enrollment, error) {
	if studentID == uuid.Nil {
		return nil, newValidationError("student_user_id", "must be a valid UUID")
	}
	if offeringID == uuid.Nil {
		return nil, newValidationError("offering_id", "must be a valid UUID")
	}
	if actor.UserID == uuid.Nil {
		return nil, ErrUnauthorized
	}
	if !(actor.HasRole(RoleAdmin) && actor.HasPermission(PermEnrollmentsManage)) &&
		(!actor.HasRole(RoleStudent) || actor.UserID != studentID) {
		return nil, ErrForbidden
	}
	if err := s.users.EnsureApproved(ctx, studentID); err != nil {
		return nil, err
	}
	if err := s.roles.EnsureActiveStudent(ctx, studentID); err != nil {
		return nil, err
	}
	offering, err := s.offerings.GetOffering(ctx, offeringID)
	if err != nil {
		return nil, err
	}
	if offering == nil || offering.ID != offeringID {
		return nil, ErrDependencyUnavailable
	}
	if offering.Status != "published" {
		return nil, ErrOfferingIneligible
	}
	now := s.now().UTC()
	if offering.EnrollmentOpensAt != nil && now.Before(*offering.EnrollmentOpensAt) {
		return nil, ErrOfferingIneligible
	}
	if offering.EnrollmentClosesAt != nil && !now.Before(*offering.EnrollmentClosesAt) {
		return nil, ErrOfferingIneligible
	}
	item := &Enrollment{
		ID:              uuid.New(),
		StudentUserID:   studentID,
		OfferingID:      offeringID,
		Status:          EnrollmentActive,
		CreatedByUserID: actor.UserID,
	}
	if err := s.store.Create(ctx, item, offering.EnrollmentCapacity); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) Get(ctx context.Context, actor Actor, id uuid.UUID) (*Enrollment, error) {
	if actor.UserID == uuid.Nil {
		return nil, ErrUnauthorized
	}
	item, err := s.store.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if actor.HasRole(RoleAdmin) && actor.HasPermission(PermEnrollmentsRead) ||
		actor.UserID == item.StudentUserID && actor.HasRole(RoleStudent) {
		return item, nil
	}
	if actor.HasRole(RoleLecturer) && actor.HasPermission(PermEnrollmentsRead) {
		if err := s.authorizeOfferingLecturer(ctx, actor.UserID, item.OfferingID); err != nil {
			return nil, err
		}
		return item, nil
	}
	return nil, ErrForbidden
}

func (s *Service) ListForStudent(
	ctx context.Context, actor Actor, studentID uuid.UUID, filter EnrollmentFilter,
) (*EnrollmentList, error) {
	if studentID == uuid.Nil {
		return nil, newValidationError("student_user_id", "must be a valid UUID")
	}
	if actor.UserID == uuid.Nil {
		return nil, ErrUnauthorized
	}
	if !(actor.HasRole(RoleAdmin) && actor.HasPermission(PermEnrollmentsRead)) &&
		(!actor.HasRole(RoleStudent) || actor.UserID != studentID) {
		return nil, ErrForbidden
	}
	if err := validateEnrollmentFilter(&filter); err != nil {
		return nil, err
	}
	filter.StudentUserID = &studentID
	return s.store.List(ctx, filter)
}

func (s *Service) ListForOffering(
	ctx context.Context, actor Actor, offeringID uuid.UUID, filter EnrollmentFilter,
) (*EnrollmentList, error) {
	if offeringID == uuid.Nil {
		return nil, newValidationError("offering_id", "must be a valid UUID")
	}
	if actor.UserID == uuid.Nil {
		return nil, ErrUnauthorized
	}
	if !(actor.HasRole(RoleAdmin) && actor.HasPermission(PermEnrollmentsRead)) {
		if !actor.HasRole(RoleLecturer) || !actor.HasPermission(PermEnrollmentsRead) {
			return nil, ErrForbidden
		}
		if err := s.authorizeOfferingLecturer(ctx, actor.UserID, offeringID); err != nil {
			return nil, err
		}
	}
	if err := validateEnrollmentFilter(&filter); err != nil {
		return nil, err
	}
	filter.OfferingID = &offeringID
	return s.store.List(ctx, filter)
}

func (s *Service) Cancel(
	ctx context.Context, actor Actor, id uuid.UUID, expectedVersion int64,
) (*Enrollment, error) {
	if id == uuid.Nil {
		return nil, newValidationError("id", "must be a valid UUID")
	}
	if expectedVersion < 1 {
		return nil, newValidationError("version", "must be positive")
	}
	if actor.UserID == uuid.Nil {
		return nil, ErrUnauthorized
	}
	item, err := s.store.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !(actor.HasRole(RoleAdmin) && actor.HasPermission(PermEnrollmentsManage)) &&
		(!actor.HasRole(RoleStudent) || actor.UserID != item.StudentUserID) {
		return nil, ErrForbidden
	}
	return s.store.Cancel(ctx, id, expectedVersion, actor.UserID)
}

func (s *Service) Active(
	ctx context.Context, studentID, offeringID uuid.UUID,
) (bool, error) {
	if studentID == uuid.Nil || offeringID == uuid.Nil {
		return false, newValidationError("id", "must be a valid UUID")
	}
	_, err := s.store.GetActive(ctx, studentID, offeringID)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := s.users.EnsureApproved(ctx, studentID); err != nil {
		if errors.Is(err, ErrStudentIneligible) {
			return false, nil
		}
		return false, err
	}
	if err := s.roles.EnsureActiveStudent(ctx, studentID); err != nil {
		if errors.Is(err, ErrStudentIneligible) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ActiveLecturer verifies the Course-owned assignment and both live account
// authorities. It stores no lecturer assignment record.
func (s *Service) ActiveLecturer(
	ctx context.Context, lecturerID, offeringID uuid.UUID,
) (bool, error) {
	if lecturerID == uuid.Nil || offeringID == uuid.Nil {
		return false, newValidationError("id", "must be a valid UUID")
	}
	offering, err := s.offerings.GetOffering(ctx, offeringID)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if offering == nil || offering.ID != offeringID {
		return false, ErrDependencyUnavailable
	}
	if offering.LecturerUserID == nil || *offering.LecturerUserID != lecturerID {
		return false, nil
	}
	if err := s.users.EnsureApproved(ctx, lecturerID); err != nil {
		if errors.Is(err, ErrStudentIneligible) {
			return false, nil
		}
		return false, err
	}
	if err := s.roles.EnsureActiveLecturer(ctx, lecturerID); err != nil {
		if errors.Is(err, ErrStudentIneligible) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *Service) authorizeOfferingLecturer(
	ctx context.Context, lecturerID, offeringID uuid.UUID,
) error {
	active, err := s.ActiveLecturer(ctx, lecturerID, offeringID)
	if err != nil {
		return err
	}
	if !active {
		return ErrForbidden
	}
	return nil
}

func validateEnrollmentFilter(filter *EnrollmentFilter) error {
	if filter.Status != "" && !filter.Status.Valid() {
		return newValidationError("status", "is not a valid enrollment status")
	}
	if filter.Limit == 0 {
		filter.Limit = 20
	}
	if filter.Limit < 1 || filter.Limit > 100 {
		return newValidationError("limit", "must be between 1 and 100")
	}
	if filter.Offset < 0 || filter.Offset > 1_000_000 {
		return newValidationError("offset", "must be between 0 and 1000000")
	}
	return nil
}
