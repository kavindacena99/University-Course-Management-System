package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

const testPassword = "StrongPass1!"

func TestServiceLoginAndRefreshRotatesTokens(t *testing.T) {
	service, _, pool, _ := newTestAuthService(t)
	ctx := context.Background()

	user, err := service.ProvisionUser(ctx, ProvisionUserInput{
		UserID:    uuid.New(),
		Email:     uniqueTestEmail(),
		FirstName: "Login",
		LastName:  "User",
		Version:   1,
		Password:  testPassword,
		Roles:     []string{RoleStudent},
	})
	if err != nil {
		t.Fatalf("provision user: %v", err)
	}
	t.Cleanup(func() { cleanupAuthUser(t, pool, user.UserID) })

	login, err := service.Login(ctx, LoginInput{
		Email:    user.Email,
		Password: testPassword,
		Client: ClientInfo{
			UserAgent: "test-agent",
			IPAddress: "127.0.0.1",
		},
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if login.AccessToken == "" || login.RefreshToken == "" {
		t.Fatal("expected access and refresh tokens")
	}
	if login.CSRFToken == "" {
		t.Fatal("expected a CSRF token")
	}

	claims, err := service.tokens.VerifyAccessToken(login.AccessToken)
	if err != nil {
		t.Fatalf("verify access token: %v", err)
	}
	sessions, err := service.Sessions(ctx, claims)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 1 || !sessions[0].Current {
		t.Fatalf("unexpected sessions: %+v", sessions)
	}

	refreshed, err := service.Refresh(ctx, login.RefreshToken)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if refreshed.RefreshToken == login.RefreshToken {
		t.Fatal("expected refresh token rotation")
	}

	if _, err := service.Refresh(ctx, login.RefreshToken); !errors.Is(err, ErrRefreshReuse) {
		t.Fatalf("expected ErrRefreshReuse, got %v", err)
	}
	if _, err := service.Refresh(ctx, refreshed.RefreshToken); !errors.Is(err, ErrRefreshInvalid) {
		t.Fatalf("expected ErrRefreshInvalid after reuse revocation, got %v", err)
	}

	sessions, err = service.Sessions(ctx, claims)
	if err != nil {
		t.Fatalf("list sessions after revocation: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected no active sessions, got %+v", sessions)
	}
}

func TestServiceLoginFailuresLockAndUnlockAccount(t *testing.T) {
	service, _, pool, _ := newTestAuthService(t)
	ctx := context.Background()

	user, err := service.ProvisionUser(ctx, ProvisionUserInput{
		UserID:    uuid.New(),
		Email:     uniqueTestEmail(),
		FirstName: "Locked",
		LastName:  "User",
		Version:   1,
		Password:  testPassword,
		Roles:     []string{RoleStudent},
	})
	if err != nil {
		t.Fatalf("provision user: %v", err)
	}
	t.Cleanup(func() { cleanupAuthUser(t, pool, user.UserID) })

	if _, err := service.Login(ctx, LoginInput{
		Email: "missing-" + user.Email, Password: testPassword,
	}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials for unknown email, got %v", err)
	}

	for i := 0; i < 3; i++ {
		if _, err := service.Login(ctx, LoginInput{
			Email: user.Email, Password: "WrongPass1!",
		}); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("attempt %d: expected ErrInvalidCredentials, got %v", i+1, err)
		}
	}

	if _, err := service.Login(ctx, LoginInput{
		Email: user.Email, Password: testPassword,
	}); !errors.Is(err, ErrAccountLocked) {
		t.Fatalf("expected ErrAccountLocked, got %v", err)
	}

	if _, err := service.UnlockUser(ctx, user.UserID); err != nil {
		t.Fatalf("unlock user: %v", err)
	}
	if _, err := service.Login(ctx, LoginInput{
		Email: user.Email, Password: testPassword,
	}); err != nil {
		t.Fatalf("login after unlock: %v", err)
	}
}

func TestServiceEventActivationFlow(t *testing.T) {
	service, repo, pool, notifications := newTestAuthService(t)
	ctx := context.Background()

	userID := uuid.New()
	email := uniqueTestEmail()
	eventID := uuid.New()
	event := UserEvent{
		EventID:    eventID,
		EventType:  EventUserApproved,
		OccurredAt: time.Now().UTC(),
		User: UserEventPayload{
			UserID:             userID,
			Email:              email,
			FirstName:          "Activation",
			LastName:           "User",
			RegistrationStatus: StatusApproved,
			Version:            1,
		},
	}
	t.Cleanup(func() {
		cleanupAuthUser(t, pool, userID)
		cleanupInboxEvent(t, pool, eventID)
	})

	if _, err := service.ProcessUserEvent(ctx, event); err != nil {
		t.Fatalf("process approved event: %v", err)
	}

	token, notifiedUser := notifications.activation()
	if token == "" || notifiedUser == nil || notifiedUser.UserID != userID {
		t.Fatalf("expected an activation notification, got user=%v token=%q",
			notifiedUser, token)
	}

	if _, err := service.Login(ctx, LoginInput{
		Email: email, Password: testPassword,
	}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials before activation, got %v", err)
	}

	activated, err := service.Activate(ctx, token, testPassword)
	if err != nil {
		t.Fatalf("activate account: %v", err)
	}
	if activated.ActivatedAt == nil {
		t.Fatal("expected activated_at to be populated")
	}
	if !activated.HasRole(RoleStudent) {
		t.Fatalf("expected default student role, got %v", activated.Roles)
	}

	if _, err := service.Login(ctx, LoginInput{
		Email: email, Password: testPassword,
	}); err != nil {
		t.Fatalf("login after activation: %v", err)
	}

	if _, err := service.Activate(ctx, token, testPassword); !errors.Is(err, ErrActionTokenUsed) {
		t.Fatalf("expected ErrActionTokenUsed, got %v", err)
	}

	duplicate, err := service.ProcessUserEvent(ctx, event)
	if err != nil {
		t.Fatalf("process duplicate event: %v", err)
	}
	if !duplicate.Duplicate {
		t.Fatal("expected duplicate event to be detected")
	}

	staleEventID := uuid.New()
	t.Cleanup(func() { cleanupInboxEvent(t, pool, staleEventID) })
	stale, err := service.ProcessUserEvent(ctx, UserEvent{
		EventID:    staleEventID,
		EventType:  EventUserProfileUpdated,
		OccurredAt: time.Now().UTC(),
		User: UserEventPayload{
			UserID:             userID,
			Email:              email,
			FirstName:          "Stale",
			LastName:           "User",
			RegistrationStatus: StatusApproved,
			Version:            1,
		},
	})
	if err != nil {
		t.Fatalf("process stale event: %v", err)
	}
	if !stale.Stale {
		t.Fatal("expected stale event to be detected")
	}

	updateEventID := uuid.New()
	t.Cleanup(func() { cleanupInboxEvent(t, pool, updateEventID) })
	if _, err := service.ProcessUserEvent(ctx, UserEvent{
		EventID:    updateEventID,
		EventType:  EventUserProfileUpdated,
		OccurredAt: time.Now().UTC(),
		User: UserEventPayload{
			UserID:             userID,
			Email:              email,
			FirstName:          "Updated",
			LastName:           "Name",
			RegistrationStatus: StatusApproved,
			Version:            2,
		},
	}); err != nil {
		t.Fatalf("process profile update: %v", err)
	}

	updated, err := repo.GetUserByID(ctx, userID)
	if err != nil {
		t.Fatalf("get updated user: %v", err)
	}
	if updated.FirstName != "Updated" || updated.LastName != "Name" {
		t.Fatalf("unexpected updated user: %+v", updated)
	}
	if updated.UserVersion != 2 {
		t.Fatalf("user version = %d", updated.UserVersion)
	}
}

func TestServiceChangePassword(t *testing.T) {
	service, _, pool, _ := newTestAuthService(t)
	ctx := context.Background()

	user, err := service.ProvisionUser(ctx, ProvisionUserInput{
		UserID:    uuid.New(),
		Email:     uniqueTestEmail(),
		FirstName: "Password",
		LastName:  "User",
		Version:   1,
		Password:  testPassword,
		Roles:     []string{RoleStudent},
	})
	if err != nil {
		t.Fatalf("provision user: %v", err)
	}
	t.Cleanup(func() { cleanupAuthUser(t, pool, user.UserID) })

	login, err := service.Login(ctx, LoginInput{
		Email: user.Email, Password: testPassword,
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	claims, err := service.tokens.VerifyAccessToken(login.AccessToken)
	if err != nil {
		t.Fatalf("verify access token: %v", err)
	}

	if err := service.ChangePassword(ctx, claims, ChangePasswordInput{
		CurrentPassword: testPassword,
		NewPassword:     testPassword,
	}); !errors.Is(err, ErrPasswordReuse) {
		t.Fatalf("expected ErrPasswordReuse, got %v", err)
	}

	newPassword := "EvenStronger2@"
	if err := service.ChangePassword(ctx, claims, ChangePasswordInput{
		CurrentPassword: testPassword,
		NewPassword:     newPassword,
	}); err != nil {
		t.Fatalf("change password: %v", err)
	}

	if _, err := service.Login(ctx, LoginInput{
		Email: user.Email, Password: testPassword,
	}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected old password to fail, got %v", err)
	}
	if _, err := service.Login(ctx, LoginInput{
		Email: user.Email, Password: newPassword,
	}); err != nil {
		t.Fatalf("login with new password: %v", err)
	}

	if _, err := service.Refresh(ctx, login.RefreshToken); err != nil {
		t.Fatalf("expected current session refresh to remain valid: %v", err)
	}
}

func TestServiceForgotAndResetPassword(t *testing.T) {
	service, _, pool, notifications := newTestAuthService(t)
	ctx := context.Background()

	user, err := service.ProvisionUser(ctx, ProvisionUserInput{
		UserID:    uuid.New(),
		Email:     uniqueTestEmail(),
		FirstName: "Reset",
		LastName:  "User",
		Version:   1,
		Password:  testPassword,
		Roles:     []string{RoleStudent},
	})
	if err != nil {
		t.Fatalf("provision user: %v", err)
	}
	t.Cleanup(func() { cleanupAuthUser(t, pool, user.UserID) })

	if err := service.ForgotPassword(ctx, "missing-"+user.Email); err != nil {
		t.Fatalf("forgot password for unknown email: %v", err)
	}

	if err := service.ForgotPassword(ctx, user.Email); err != nil {
		t.Fatalf("forgot password: %v", err)
	}
	token, notifiedUser := notifications.reset()
	if token == "" || notifiedUser == nil || notifiedUser.UserID != user.UserID {
		t.Fatalf("expected reset notification, got user=%v token=%q",
			notifiedUser, token)
	}

	newPassword := "ResetPassword3#"
	resetUser, err := service.ResetPassword(ctx, token, newPassword)
	if err != nil {
		t.Fatalf("reset password: %v", err)
	}
	if resetUser.UserID != user.UserID {
		t.Fatalf("reset user = %s, want %s", resetUser.UserID, user.UserID)
	}

	if _, err := service.Login(ctx, LoginInput{
		Email: user.Email, Password: testPassword,
	}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected old password to fail, got %v", err)
	}
	if _, err := service.Login(ctx, LoginInput{
		Email: user.Email, Password: newPassword,
	}); err != nil {
		t.Fatalf("login with reset password: %v", err)
	}

	if _, err := service.ResetPassword(ctx, token, "AnotherPass4$"); !errors.Is(err, ErrActionTokenUsed) {
		t.Fatalf("expected ErrActionTokenUsed, got %v", err)
	}
}

func TestServiceRoleUpdatesAndLastAdminGuard(t *testing.T) {
	service, _, pool, _ := newTestAuthService(t)
	ctx := context.Background()

	firstAdmin, err := service.ProvisionUser(ctx, ProvisionUserInput{
		UserID:    uuid.New(),
		Email:     uniqueTestEmail(),
		FirstName: "First",
		LastName:  "Admin",
		Version:   1,
		Password:  testPassword,
		Roles:     []string{RoleAdmin},
	})
	if err != nil {
		t.Fatalf("provision first admin: %v", err)
	}
	t.Cleanup(func() { cleanupAuthUser(t, pool, firstAdmin.UserID) })

	adminClaims := &AccessClaims{
		Subject: firstAdmin.UserID.String(),
		Roles:   []string{RoleAdmin},
	}
	if _, err := service.SetUserRoles(ctx, adminClaims, firstAdmin.UserID, []string{RoleStudent}); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("expected ErrLastAdmin, got %v", err)
	}

	secondAdmin, err := service.ProvisionUser(ctx, ProvisionUserInput{
		UserID:    uuid.New(),
		Email:     uniqueTestEmail(),
		FirstName: "Second",
		LastName:  "Admin",
		Version:   1,
		Password:  testPassword,
		Roles:     []string{RoleAdmin},
	})
	if err != nil {
		t.Fatalf("provision second admin: %v", err)
	}
	t.Cleanup(func() { cleanupAuthUser(t, pool, secondAdmin.UserID) })

	updated, err := service.SetUserRoles(ctx, adminClaims, firstAdmin.UserID,
		[]string{RoleLecturer, RoleLecturer, RoleStudent})
	if err != nil {
		t.Fatalf("update roles: %v", err)
	}
	if len(updated.Roles) != 2 {
		t.Fatalf("expected deduplicated roles, got %v", updated.Roles)
	}
	if updated.HasRole(RoleAdmin) {
		t.Fatalf("expected admin role to be removed, got %v", updated.Roles)
	}
}

func TestServicePermissionsAndAudit(t *testing.T) {
	service, _, pool, _ := newTestAuthService(t)
	ctx := context.Background()

	admin, err := service.ProvisionUser(ctx, ProvisionUserInput{
		UserID:    uuid.New(),
		Email:     uniqueTestEmail(),
		FirstName: "Audit",
		LastName:  "Admin",
		Version:   1,
		Password:  testPassword,
		Roles:     []string{RoleAdmin},
	})
	if err != nil {
		t.Fatalf("provision admin: %v", err)
	}
	t.Cleanup(func() { cleanupAuthUser(t, pool, admin.UserID) })

	permissions, err := service.ListPermissions(ctx)
	if err != nil {
		t.Fatalf("list permissions: %v", err)
	}
	if len(permissions) != len(AllPermissions) {
		t.Fatalf("expected %d permissions, got %d", len(AllPermissions), len(permissions))
	}

	adminPermissions, err := service.GetRolePermissions(ctx, RoleAdmin)
	if err != nil {
		t.Fatalf("get admin permissions: %v", err)
	}
	if len(adminPermissions) != len(AllPermissions) {
		t.Fatalf("expected admin to have all permissions, got %v", adminPermissions)
	}

	claims := &AccessClaims{
		Subject: admin.UserID.String(),
		Roles:   []string{RoleAdmin},
	}
	updated, err := service.SetRolePermissions(ctx, claims, RoleStudent,
		[]string{PermUsersRead, PermUsersRead})
	if err != nil {
		t.Fatalf("set student permissions: %v", err)
	}
	if len(updated) != 1 || updated[0].Code != PermUsersRead {
		t.Fatalf("unexpected student permissions: %v", updated)
	}
	t.Cleanup(func() {
		_, _ = service.SetRolePermissions(context.Background(), claims, RoleStudent, nil)
	})

	if _, err := service.SetRolePermissions(ctx, claims, RoleStudent,
		[]string{"not:a:permission"}); !errors.Is(err, ErrInvalidPermission) {
		t.Fatalf("expected ErrInvalidPermission, got %v", err)
	}

	events, total, err := service.ListAuditEvents(ctx, 50, 0)
	if err != nil {
		t.Fatalf("list audit events: %v", err)
	}
	if total == 0 || len(events) == 0 {
		t.Fatalf("expected audit events, got total=%d len=%d", total, len(events))
	}
}

func TestServiceLoginFailsClosedWhenEligibilityUnavailable(t *testing.T) {
	service, _, pool, _ := newTestAuthService(t)
	ctx := context.Background()

	user, err := service.ProvisionUser(ctx, ProvisionUserInput{
		UserID:    uuid.New(),
		Email:     uniqueTestEmail(),
		FirstName: "Eligible",
		LastName:  "Check",
		Version:   1,
		Password:  testPassword,
		Roles:     []string{RoleStudent},
	})
	if err != nil {
		t.Fatalf("provision user: %v", err)
	}
	t.Cleanup(func() { cleanupAuthUser(t, pool, user.UserID) })

	service.eligibility = &testEligibilityChecker{err: ErrEligibilityUnavailable}
	if _, err := service.Login(ctx, LoginInput{
		Email: user.Email, Password: testPassword,
	}); !errors.Is(err, ErrEligibilityUnavailable) {
		t.Fatalf("expected ErrEligibilityUnavailable, got %v", err)
	}

	service.eligibility = &testEligibilityChecker{err: ErrUserIneligible}
	if _, err := service.Login(ctx, LoginInput{
		Email: user.Email, Password: testPassword,
	}); !errors.Is(err, ErrUserIneligible) {
		t.Fatalf("expected ErrUserIneligible, got %v", err)
	}
}

func TestServiceProvisionRejectsDuplicateEmail(t *testing.T) {
	service, _, pool, _ := newTestAuthService(t)
	ctx := context.Background()

	first, err := service.ProvisionUser(ctx, ProvisionUserInput{
		UserID:    uuid.New(),
		Email:     uniqueTestEmail(),
		FirstName: "First",
		LastName:  "User",
		Version:   1,
		Password:  testPassword,
		Roles:     []string{RoleStudent},
	})
	if err != nil {
		t.Fatalf("provision first user: %v", err)
	}
	t.Cleanup(func() { cleanupAuthUser(t, pool, first.UserID) })

	_, err = service.ProvisionUser(ctx, ProvisionUserInput{
		UserID:    uuid.New(),
		Email:     first.Email,
		FirstName: "Second",
		LastName:  "User",
		Version:   1,
		Password:  testPassword,
		Roles:     []string{RoleStudent},
	})
	if !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("expected ErrEmailTaken, got %v", err)
	}
}
