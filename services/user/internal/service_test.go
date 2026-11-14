package user

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
	pingFn           func(context.Context) error
	createPendingFn  func(context.Context, *User) error
	createApprovedFn func(context.Context, *User, string, *uuid.UUID) error
	getByIDFn        func(context.Context, uuid.UUID) (*User, error)
	getByEmailFn     func(context.Context, string) (*User, error)
	updateProfileFn  func(context.Context, uuid.UUID, int64, string, string) (*User, error)
	changeStatusFn   func(context.Context, ChangeStatusInput) (*User, error)
	listUsersFn      func(context.Context, ListUsersFilter) (*UserList, error)
	getHistoryFn     func(context.Context, uuid.UUID, int, int) ([]StatusHistory, int64, error)
}

func (f *fakeStore) Ping(ctx context.Context) error {
	if f.pingFn == nil {
		return nil
	}
	return f.pingFn(ctx)
}

func (f *fakeStore) CreatePending(ctx context.Context, u *User) error {
	if f.createPendingFn == nil {
		return errors.New("unexpected CreatePending call")
	}
	return f.createPendingFn(ctx, u)
}

func (f *fakeStore) CreateApproved(ctx context.Context, u *User, reason string, reviewer *uuid.UUID) error {
	if f.createApprovedFn == nil {
		return errors.New("unexpected CreateApproved call")
	}
	return f.createApprovedFn(ctx, u, reason, reviewer)
}

func (f *fakeStore) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	if f.getByIDFn == nil {
		return nil, errors.New("unexpected GetByID call")
	}
	return f.getByIDFn(ctx, id)
}

func (f *fakeStore) GetByEmail(ctx context.Context, email string) (*User, error) {
	if f.getByEmailFn == nil {
		return nil, errors.New("unexpected GetByEmail call")
	}
	return f.getByEmailFn(ctx, email)
}

func (f *fakeStore) UpdateProfile(ctx context.Context, id uuid.UUID, version int64, first, last string) (*User, error) {
	if f.updateProfileFn == nil {
		return nil, errors.New("unexpected UpdateProfile call")
	}
	return f.updateProfileFn(ctx, id, version, first, last)
}

func (f *fakeStore) ChangeStatus(ctx context.Context, input ChangeStatusInput) (*User, error) {
	if f.changeStatusFn == nil {
		return nil, errors.New("unexpected ChangeStatus call")
	}
	return f.changeStatusFn(ctx, input)
}

func (f *fakeStore) ListUsers(ctx context.Context, filter ListUsersFilter) (*UserList, error) {
	if f.listUsersFn == nil {
		return nil, errors.New("unexpected ListUsers call")
	}
	return f.listUsersFn(ctx, filter)
}

func (f *fakeStore) GetHistory(ctx context.Context, userID uuid.UUID, limit, offset int) ([]StatusHistory, int64, error) {
	if f.getHistoryFn == nil {
		return nil, 0, errors.New("unexpected GetHistory call")
	}
	return f.getHistoryFn(ctx, userID, limit, offset)
}

type fakeProvisioner struct {
	calls int
	err   error
	user  *User
	pass  string
}

func (f *fakeProvisioner) ProvisionAdmin(_ context.Context, u *User, password string) error {
	f.calls++
	f.user = u
	f.pass = password
	return f.err
}

func TestServiceRegisterNormalizesInput(t *testing.T) {
	var created *User
	store := &fakeStore{
		createPendingFn: func(_ context.Context, u *User) error {
			u.ID = uuid.New()
			u.Version = 1
			u.CreatedAt = time.Now()
			u.UpdatedAt = time.Now()
			created = u
			return nil
		},
	}
	service := NewService(store, nil)

	u, err := service.Register(context.Background(), RegisterInput{
		Email:     "  Student@Example.COM ",
		FirstName: "  Test ",
		LastName:  " Student  ",
	})
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	if created == nil {
		t.Fatal("store was not called")
	}
	if u.Email != "student@example.com" {
		t.Fatalf("email = %q", u.Email)
	}
	if u.FirstName != "Test" || u.LastName != "Student" {
		t.Fatalf("names = %q %q", u.FirstName, u.LastName)
	}
	if u.RegistrationStatus != StatusPending {
		t.Fatalf("status = %q", u.RegistrationStatus)
	}
}

func TestServiceRegisterValidation(t *testing.T) {
	service := NewService(&fakeStore{}, nil)

	cases := []RegisterInput{
		{Email: "not-an-email", FirstName: "Test", LastName: "Student"},
		{Email: "student@example.com", FirstName: "   ", LastName: "Student"},
		{Email: "student@example.com", FirstName: "Test", LastName: strings.Repeat("a", 101)},
	}

	for _, input := range cases {
		_, err := service.Register(context.Background(), input)
		var validation *ValidationError
		if !errors.As(err, &validation) {
			t.Fatalf("expected validation error for %+v, got %v", input, err)
		}
	}
}

func TestServiceGetUserAuthorization(t *testing.T) {
	ownerID := uuid.New()
	otherID := uuid.New()
	store := &fakeStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*User, error) {
			return &User{ID: id, RegistrationStatus: StatusApproved}, nil
		},
	}
	service := NewService(store, nil)

	ownerClaims := claimsFor(ownerID, RoleStudent)
	if _, err := service.GetUser(context.Background(), ownerClaims, ownerID); err != nil {
		t.Fatalf("owner read failed: %v", err)
	}
	if _, err := service.GetUser(context.Background(), ownerClaims, otherID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}

	adminClaims := claimsFor(uuid.New(), RoleAdmin)
	if _, err := service.GetUser(context.Background(), adminClaims, otherID); err != nil {
		t.Fatalf("admin read failed: %v", err)
	}
}

func TestServiceUpdateMeRequiresVersion(t *testing.T) {
	service := NewService(&fakeStore{}, nil)
	claims := claimsFor(uuid.New(), RoleStudent)

	_, err := service.UpdateMe(context.Background(), claims, UpdateProfileInput{
		Version:   0,
		FirstName: "Test",
		LastName:  "Student",
	})
	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestServiceChangeStatusBuildsEvent(t *testing.T) {
	userID := uuid.New()
	adminID := uuid.New()
	store := &fakeStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*User, error) {
			return &User{
				ID:                 id,
				RegistrationStatus: StatusPending,
				Version:            1,
			}, nil
		},
		changeStatusFn: func(_ context.Context, input ChangeStatusInput) (*User, error) {
			if input.FromStatus != StatusPending || input.ToStatus != StatusApproved {
				t.Fatalf("unexpected transition %+v", input)
			}
			if input.EventType != EventUserApproved {
				t.Fatalf("event type = %q", input.EventType)
			}
			if input.ChangedBy != adminID {
				t.Fatalf("changed by = %s", input.ChangedBy)
			}
			return &User{
				ID:                 input.UserID,
				RegistrationStatus: StatusApproved,
				Version:            2,
			}, nil
		},
	}
	service := NewService(store, nil)

	updated, err := service.ChangeStatus(
		context.Background(),
		claimsFor(adminID, RoleAdmin),
		ChangeStatusRequest{
			UserID:          userID,
			Action:          ActionApprove,
			ExpectedVersion: 1,
		},
	)
	if err != nil {
		t.Fatalf("change status: %v", err)
	}
	if updated.RegistrationStatus != StatusApproved {
		t.Fatalf("status = %q", updated.RegistrationStatus)
	}
}

func TestServiceChangeStatusRejectsInvalidTransitions(t *testing.T) {
	userID := uuid.New()
	store := &fakeStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*User, error) {
			return &User{
				ID:                 id,
				RegistrationStatus: StatusApproved,
				Version:            2,
			}, nil
		},
	}
	service := NewService(store, nil)
	claims := claimsFor(uuid.New(), RoleAdmin)

	_, err := service.ChangeStatus(context.Background(), claims, ChangeStatusRequest{
		UserID:          userID,
		Action:          ActionApprove,
		ExpectedVersion: 2,
	})
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected invalid transition, got %v", err)
	}
}

func TestServiceChangeStatusRequiresReasonForReject(t *testing.T) {
	userID := uuid.New()
	store := &fakeStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*User, error) {
			return &User{
				ID:                 id,
				RegistrationStatus: StatusPending,
				Version:            1,
			}, nil
		},
	}
	service := NewService(store, nil)

	_, err := service.ChangeStatus(
		context.Background(),
		claimsFor(uuid.New(), RoleAdmin),
		ChangeStatusRequest{
			UserID:          userID,
			Action:          ActionReject,
			ExpectedVersion: 1,
		},
	)
	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestServiceChangeStatusRejectsStaleVersion(t *testing.T) {
	store := &fakeStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*User, error) {
			return &User{
				ID:                 id,
				RegistrationStatus: StatusPending,
				Version:            2,
			}, nil
		},
	}
	service := NewService(store, nil)

	_, err := service.ChangeStatus(
		context.Background(),
		claimsFor(uuid.New(), RoleAdmin),
		ChangeStatusRequest{
			UserID:          uuid.New(),
			Action:          ActionApprove,
			ExpectedVersion: 1,
		},
	)
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("expected version conflict, got %v", err)
	}
}

func TestServiceListUsersClampsPagination(t *testing.T) {
	var captured ListUsersFilter
	store := &fakeStore{
		listUsersFn: func(_ context.Context, filter ListUsersFilter) (*UserList, error) {
			captured = filter
			return &UserList{Limit: filter.Limit, Offset: filter.Offset}, nil
		},
	}
	service := NewService(store, nil)

	if _, err := service.ListUsers(context.Background(), ListUsersFilter{
		Limit:  500,
		Offset: -5,
	}); err != nil {
		t.Fatalf("list users: %v", err)
	}
	if captured.Limit != 100 {
		t.Fatalf("limit = %d", captured.Limit)
	}
	if captured.Offset != 0 {
		t.Fatalf("offset = %d", captured.Offset)
	}
}

func TestServiceEligibility(t *testing.T) {
	userID := uuid.New()
	store := &fakeStore{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*User, error) {
			return &User{
				ID:                 id,
				RegistrationStatus: StatusApproved,
				Version:            3,
			}, nil
		},
	}
	service := NewService(store, nil)

	eligibility, err := service.Eligibility(context.Background(), userID)
	if err != nil {
		t.Fatalf("eligibility: %v", err)
	}
	if !eligibility.Eligible {
		t.Fatal("expected eligible user")
	}
}

func TestServiceBootstrapAdminCreatesAndProvisionsUser(t *testing.T) {
	provisioner := &fakeProvisioner{}
	store := &fakeStore{
		getByEmailFn: func(context.Context, string) (*User, error) {
			return nil, ErrNotFound
		},
		createApprovedFn: func(_ context.Context, u *User, reason string, reviewer *uuid.UUID) error {
			u.Version = 1
			u.ReviewedAt = timePointer(time.Now())
			return nil
		},
	}
	service := NewService(store, provisioner)

	u, err := service.BootstrapAdmin(context.Background(), BootstrapInput{
		Email:     "admin@example.com",
		FirstName: "Admin",
		LastName:  "User",
		Password:  "StrongPassword1!",
	})
	if err != nil {
		t.Fatalf("bootstrap admin: %v", err)
	}
	if u.RegistrationStatus != StatusApproved {
		t.Fatalf("status = %q", u.RegistrationStatus)
	}
	if provisioner.calls != 1 {
		t.Fatalf("provisioner calls = %d", provisioner.calls)
	}
	if provisioner.pass != "StrongPassword1!" {
		t.Fatal("provisioner did not receive the password")
	}
}

func TestServiceBootstrapAdminIsIdempotentForApprovedUser(t *testing.T) {
	existing := &User{
		ID:                 uuid.New(),
		Email:              "admin@example.com",
		RegistrationStatus: StatusApproved,
		Version:            1,
	}
	provisioner := &fakeProvisioner{}
	store := &fakeStore{
		getByEmailFn: func(context.Context, string) (*User, error) {
			return existing, nil
		},
		createApprovedFn: func(context.Context, *User, string, *uuid.UUID) error {
			t.Fatal("CreateApproved must not be called for an existing approved user")
			return nil
		},
	}
	service := NewService(store, provisioner)

	u, err := service.BootstrapAdmin(context.Background(), BootstrapInput{
		Email:     "admin@example.com",
		FirstName: "Admin",
		LastName:  "User",
		Password:  "StrongPassword1!",
	})
	if err != nil {
		t.Fatalf("bootstrap admin: %v", err)
	}
	if u.ID != existing.ID {
		t.Fatalf("user id = %s", u.ID)
	}
	if provisioner.calls != 1 {
		t.Fatalf("provisioner calls = %d", provisioner.calls)
	}
}

func TestServiceBootstrapAdminRejectsUnapprovedExistingUser(t *testing.T) {
	store := &fakeStore{
		getByEmailFn: func(context.Context, string) (*User, error) {
			return &User{ID: uuid.New(), RegistrationStatus: StatusPending}, nil
		},
	}
	service := NewService(store, &fakeProvisioner{})

	_, err := service.BootstrapAdmin(context.Background(), BootstrapInput{
		Email:     "admin@example.com",
		FirstName: "Admin",
		LastName:  "User",
		Password:  "StrongPassword1!",
	})
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected invalid transition, got %v", err)
	}
}

func TestServiceBootstrapAdminValidatesPassword(t *testing.T) {
	service := NewService(&fakeStore{}, &fakeProvisioner{})

	for _, password := range []string{
		"short",
		"alllowercase1!",
		"ALLUPPERCASE1!",
		"NoDigitsHere!",
		"NoSymbols12345",
		"Has Space 123!A",
	} {
		_, err := service.BootstrapAdmin(context.Background(), BootstrapInput{
			Email:     "admin@example.com",
			FirstName: "Admin",
			LastName:  "User",
			Password:  password,
		})
		var validation *ValidationError
		if !errors.As(err, &validation) {
			t.Fatalf("expected validation error for password %q, got %v", password, err)
		}
	}
}

func claimsFor(userID uuid.UUID, role string) *AccessClaims {
	permissions := []string{}
	if role == RoleAdmin {
		permissions = []string{PermUsersRead, PermUsersManage}
	}
	return &AccessClaims{
		SessionID:   "session",
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

func timePointer(value time.Time) *time.Time {
	return &value
}
