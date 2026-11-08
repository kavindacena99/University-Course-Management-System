package user

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newTestRepository(t *testing.T) (*Repository, *pgxpool.Pool) {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set; skipping PostgreSQL integration test")
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse test database URL: %v", err)
	}

	if cfg.ConnConfig.Database != "users_test" {
		t.Fatalf("refusing to run repository tests against database %q; want users_test",
			cfg.ConnConfig.Database)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping test database: %v", err)
	}

	t.Cleanup(pool.Close)

	return NewRepository(pool), pool
}

func uniqueTestEmail() string {
	return fmt.Sprintf("test-%s@example.com", uuid.NewString())
}

func cleanupUser(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID) {
	t.Helper()

	ctx := context.Background()
	if _, err := pool.Exec(ctx,
		`DELETE FROM outbox_events WHERE aggregate_id = $1`, userID,
	); err != nil {
		t.Fatalf("clean up outbox events: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID); err != nil {
		t.Fatalf("clean up user: %v", err)
	}
}

func TestRepositoryCreatePendingAndGetByID(t *testing.T) {
	repo, pool := newTestRepository(t)
	ctx := context.Background()

	email := uniqueTestEmail()
	created := &User{
		Email:     "  " + strings.ToUpper(email) + "  ",
		FirstName: "  Test  ",
		LastName:  "  Student  ",
	}

	if err := repo.CreatePending(ctx, created); err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() { cleanupUser(t, pool, created.ID) })

	if created.ID == uuid.Nil {
		t.Fatal("expected repository to assign a UUID")
	}
	if created.Email != email {
		t.Errorf("expected normalized email %q, got %q", email, created.Email)
	}
	if created.FirstName != "Test" || created.LastName != "Student" {
		t.Errorf("expected trimmed names, got %q %q", created.FirstName, created.LastName)
	}
	if created.RegistrationStatus != StatusPending {
		t.Errorf("expected status %q, got %q", StatusPending, created.RegistrationStatus)
	}
	if created.Version != 1 {
		t.Errorf("expected version 1, got %d", created.Version)
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatal("expected database timestamps to be populated")
	}

	fetched, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("get created user: %v", err)
	}
	if fetched.Email != email || fetched.RegistrationStatus != StatusPending {
		t.Fatalf("unexpected fetched user: %+v", fetched)
	}

	byEmail, err := repo.GetByEmail(ctx, strings.ToUpper(email))
	if err != nil {
		t.Fatalf("get user by email: %v", err)
	}
	if byEmail.ID != created.ID {
		t.Errorf("expected email lookup to return %s, got %s", created.ID, byEmail.ID)
	}

	history, total, err := repo.GetHistory(ctx, created.ID, 10, 0)
	if err != nil {
		t.Fatalf("get user history: %v", err)
	}
	if total != 1 || len(history) != 1 {
		t.Fatalf("expected one history entry, got total=%d len=%d", total, len(history))
	}
	if history[0].FromStatus != nil || history[0].ToStatus != StatusPending {
		t.Fatalf("unexpected initial history entry: %+v", history[0])
	}
}

func TestRepositoryRejectsDuplicateEmailIgnoringCase(t *testing.T) {
	repo, pool := newTestRepository(t)
	ctx := context.Background()

	email := uniqueTestEmail()
	first := &User{Email: email, FirstName: "First", LastName: "Student"}
	if err := repo.CreatePending(ctx, first); err != nil {
		t.Fatalf("create first user: %v", err)
	}
	t.Cleanup(func() { cleanupUser(t, pool, first.ID) })

	second := &User{
		Email:     strings.ToUpper(email),
		FirstName: "Second",
		LastName:  "Student",
	}
	err := repo.CreatePending(ctx, second)
	if !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("expected ErrEmailTaken, got %v", err)
	}
}

func TestRepositoryGetByIDNotFound(t *testing.T) {
	repo, _ := newTestRepository(t)

	_, err := repo.GetByID(context.Background(), uuid.New())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestRepositoryCreateApproved(t *testing.T) {
	repo, pool := newTestRepository(t)
	ctx := context.Background()

	user := &User{
		Email:     uniqueTestEmail(),
		FirstName: "Approved",
		LastName:  "User",
	}
	if err := repo.CreateApproved(ctx, user, "bootstrap", nil); err != nil {
		t.Fatalf("create approved user: %v", err)
	}
	t.Cleanup(func() { cleanupUser(t, pool, user.ID) })

	if user.RegistrationStatus != StatusApproved {
		t.Fatalf("status = %q", user.RegistrationStatus)
	}
	if user.ReviewedAt == nil {
		t.Fatal("expected reviewed_at to be set")
	}
	if user.StatusReason != "bootstrap" {
		t.Fatalf("status reason = %q", user.StatusReason)
	}
}

func TestRepositoryUpdateProfileAndOutbox(t *testing.T) {
	repo, pool := newTestRepository(t)
	ctx := context.Background()

	user := &User{Email: uniqueTestEmail(), FirstName: "Old", LastName: "Name"}
	if err := repo.CreatePending(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() { cleanupUser(t, pool, user.ID) })

	updated, err := repo.UpdateProfile(ctx, user.ID, user.Version, "New", "Name")
	if err != nil {
		t.Fatalf("update profile: %v", err)
	}
	if updated.Version != 2 {
		t.Fatalf("version = %d, want 2", updated.Version)
	}
	if updated.FirstName != "New" || updated.LastName != "Name" {
		t.Fatalf("unexpected names: %+v", updated)
	}

	if _, err := repo.UpdateProfile(ctx, user.ID, 1, "Again", "Name"); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("expected ErrVersionConflict, got %v", err)
	}
	if _, err := repo.UpdateProfile(ctx, uuid.New(), 1, "Missing", "User"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	var eventType string
	var payload []byte
	err = pool.QueryRow(ctx, `
		SELECT event_type, payload
		FROM outbox_events
		WHERE aggregate_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, user.ID).Scan(&eventType, &payload)
	if err != nil {
		t.Fatalf("load outbox event: %v", err)
	}
	if eventType != EventUserProfileUpdated {
		t.Fatalf("event type = %q", eventType)
	}

	var event UserEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		t.Fatalf("decode event payload: %v", err)
	}
	if event.User.UserID != user.ID || event.User.Version != 2 {
		t.Fatalf("unexpected event payload: %+v", event)
	}
}

func TestRepositoryChangeStatusHistoryAndOutbox(t *testing.T) {
	repo, pool := newTestRepository(t)
	ctx := context.Background()

	admin := &User{
		Email:     uniqueTestEmail(),
		FirstName: "Admin",
		LastName:  "User",
	}
	if err := repo.CreateApproved(ctx, admin, "test admin", nil); err != nil {
		t.Fatalf("create admin: %v", err)
	}
	t.Cleanup(func() { cleanupUser(t, pool, admin.ID) })

	user := &User{Email: uniqueTestEmail(), FirstName: "Pending", LastName: "User"}
	if err := repo.CreatePending(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() { cleanupUser(t, pool, user.ID) })

	approved, err := repo.ChangeStatus(ctx, ChangeStatusInput{
		UserID:          user.ID,
		ExpectedVersion: user.Version,
		FromStatus:      StatusPending,
		ToStatus:        StatusApproved,
		Reason:          "verified",
		ChangedBy:       admin.ID,
		EventType:       EventUserApproved,
	})
	if err != nil {
		t.Fatalf("change status: %v", err)
	}

	if approved.RegistrationStatus != StatusApproved {
		t.Fatalf("status = %q", approved.RegistrationStatus)
	}
	if approved.Version != 2 {
		t.Fatalf("version = %d", approved.Version)
	}
	if approved.ReviewedBy == nil || *approved.ReviewedBy != admin.ID {
		t.Fatalf("unexpected reviewer: %+v", approved.ReviewedBy)
	}
	if approved.StatusReason != "verified" {
		t.Fatalf("reason = %q", approved.StatusReason)
	}

	history, total, err := repo.GetHistory(ctx, user.ID, 10, 0)
	if err != nil {
		t.Fatalf("get history: %v", err)
	}
	if total != 2 || len(history) != 2 {
		t.Fatalf("expected two history entries, got total=%d len=%d", total, len(history))
	}
	if history[0].ToStatus != StatusApproved || history[0].FromStatus == nil ||
		*history[0].FromStatus != StatusPending {
		t.Fatalf("unexpected history entry: %+v", history[0])
	}

	var eventType string
	var payload []byte
	if err := pool.QueryRow(ctx, `
		SELECT event_type, payload
		FROM outbox_events
		WHERE aggregate_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, user.ID).Scan(&eventType, &payload); err != nil {
		t.Fatalf("load outbox event: %v", err)
	}
	if eventType != EventUserApproved {
		t.Fatalf("event type = %q", eventType)
	}
	var event UserEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		t.Fatalf("decode event: %v", err)
	}
	if event.User.RegistrationStatus != StatusApproved ||
		event.User.Version != approved.Version {
		t.Fatalf("unexpected event payload: %+v", event)
	}

	_, err = repo.ChangeStatus(ctx, ChangeStatusInput{
		UserID:          user.ID,
		ExpectedVersion: approved.Version,
		FromStatus:      StatusPending,
		ToStatus:        StatusRejected,
		Reason:          "stale",
		ChangedBy:       admin.ID,
		EventType:       EventUserRejected,
	})
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("expected ErrVersionConflict, got %v", err)
	}

	_, err = repo.ChangeStatus(ctx, ChangeStatusInput{
		UserID:          uuid.New(),
		ExpectedVersion: 1,
		FromStatus:      StatusPending,
		ToStatus:        StatusApproved,
		ChangedBy:       admin.ID,
		EventType:       EventUserApproved,
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestRepositoryListUsersFilters(t *testing.T) {
	repo, pool := newTestRepository(t)
	ctx := context.Background()

	token := strings.ReplaceAll(uuid.NewString(), "-", "")
	email := "list-" + token + "@example.com"
	user := &User{Email: email, FirstName: "Listed", LastName: "Student"}
	if err := repo.CreatePending(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() { cleanupUser(t, pool, user.ID) })

	result, err := repo.ListUsers(ctx, ListUsersFilter{
		Status: StatusPending,
		Search: token,
		Limit:  10,
		Offset: 0,
	})
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	if result.Total != 1 || len(result.Users) != 1 {
		t.Fatalf("expected one user, got total=%d len=%d", result.Total, len(result.Users))
	}
	if result.Users[0].ID != user.ID {
		t.Fatalf("unexpected user: %+v", result.Users[0])
	}

	result, err = repo.ListUsers(ctx, ListUsersFilter{
		Status: StatusApproved,
		Search: token,
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("list approved users: %v", err)
	}
	if result.Total != 0 || len(result.Users) != 0 {
		t.Fatalf("expected no approved users, got total=%d", result.Total)
	}
}

func TestRepositoryOutboxClaimAndCompletion(t *testing.T) {
	repo, pool := newTestRepository(t)
	ctx := context.Background()

	user := &User{Email: uniqueTestEmail(), FirstName: "Outbox", LastName: "User"}
	if err := repo.CreatePending(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() { cleanupUser(t, pool, user.ID) })

	if _, err := repo.UpdateProfile(ctx, user.ID, user.Version, "Outbox", "Updated"); err != nil {
		t.Fatalf("update profile: %v", err)
	}

	var target *OutboxEvent
	for i := 0; i < 10 && target == nil; i++ {
		events, err := repo.ClaimOutboxBatch(ctx, 100)
		if err != nil {
			t.Fatalf("claim outbox batch: %v", err)
		}
		if len(events) == 0 {
			break
		}
		for index := range events {
			if events[index].ID == uuid.Nil {
				t.Fatal("claimed event has empty id")
			}
			var payload UserEvent
			if err := json.Unmarshal(events[index].Payload, &payload); err != nil {
				t.Fatalf("decode claimed payload: %v", err)
			}
			if payload.User.UserID == user.ID {
				target = &events[index]
				break
			}
			if err := repo.MarkOutboxDelivered(ctx, events[index].ID); err != nil {
				t.Fatalf("mark unrelated event delivered: %v", err)
			}
		}
	}
	if target == nil {
		t.Fatal("did not find the user outbox event")
	}

	if err := repo.MarkOutboxDelivered(ctx, target.ID); err != nil {
		t.Fatalf("mark delivered: %v", err)
	}

	var status string
	if err := pool.QueryRow(ctx,
		`SELECT status FROM outbox_events WHERE id = $1`, target.ID,
	).Scan(&status); err != nil {
		t.Fatalf("query outbox status: %v", err)
	}
	if status != OutboxStatusDelivered {
		t.Fatalf("status = %q", status)
	}

	if _, err := repo.UpdateProfile(ctx, user.ID, user.Version+1, "Outbox", "Again"); err != nil {
		t.Fatalf("update profile again: %v", err)
	}
	var failed *OutboxEvent
	events, err := repo.ClaimOutboxBatch(ctx, 100)
	if err != nil {
		t.Fatalf("claim outbox batch: %v", err)
	}
	for index := range events {
		var payload UserEvent
		if err := json.Unmarshal(events[index].Payload, &payload); err != nil {
			continue
		}
		if payload.User.UserID == user.ID {
			failed = &events[index]
			break
		}
	}
	if failed == nil {
		t.Fatal("did not find the second outbox event")
	}

	if err := repo.MarkOutboxFailed(ctx, failed.ID, true, "permanent failure", time.Now()); err != nil {
		t.Fatalf("mark failed: %v", err)
	}
	var attempts int
	if err := pool.QueryRow(ctx,
		`SELECT attempts FROM outbox_events WHERE id = $1`, failed.ID,
	).Scan(&attempts); err != nil {
		t.Fatalf("query attempts: %v", err)
	}
	if attempts < 1 {
		t.Fatalf("attempts = %d", attempts)
	}
}
