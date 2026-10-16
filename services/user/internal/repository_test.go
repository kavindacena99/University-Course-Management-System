package user

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newTestRepository(t *testing.T) *Repository {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set; skipping PostgreSQL integration test")
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse test database URL: %v", err)
	}

	// Fail closed: never run integration tests against another database.
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

	return NewRepository(pool)
}

func uniqueTestEmail() string {
	return fmt.Sprintf("test-%s@example.com", uuid.NewString())
}

func TestRepositoryCreateAndGetByID(t *testing.T) {
	repo := newTestRepository(t)
	ctx := context.Background()

	email := uniqueTestEmail()
	created := &User{
		Email:              "  " + strings.ToUpper(email) + "  ",
		FirstName:          "  Test  ",
		LastName:           "  Student  ",
		RegistrationStatus: "",
	}

	if err := repo.Create(ctx, created); err != nil {
		t.Fatalf("create user: %v", err)
	}

	if created.ID == uuid.Nil {
		t.Fatal("expected repository to assign a UUID")
	}
	if created.Email != email {
		t.Errorf("expected normalized email %q, got %q", email, created.Email)
	}
	if created.FirstName != "Test" {
		t.Errorf("expected trimmed first name, got %q", created.FirstName)
	}
	if created.LastName != "Student" {
		t.Errorf("expected trimmed last name, got %q", created.LastName)
	}
	if created.RegistrationStatus != StatusPending {
		t.Errorf("expected status %q, got %q", StatusPending, created.RegistrationStatus)
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatal("expected database timestamps to be populated")
	}

	fetched, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("get created user: %v", err)
	}

	if fetched.ID != created.ID {
		t.Errorf("expected ID %s, got %s", created.ID, fetched.ID)
	}
	if fetched.Email != email {
		t.Errorf("expected email %q, got %q", email, fetched.Email)
	}
	if fetched.RegistrationStatus != StatusPending {
		t.Errorf("expected status %q, got %q",
			StatusPending, fetched.RegistrationStatus)
	}
}

func TestRepositoryRejectsDuplicateEmailIgnoringCase(t *testing.T) {
	repo := newTestRepository(t)
	ctx := context.Background()

	email := uniqueTestEmail()

	first := &User{
		Email:     email,
		FirstName: "First",
		LastName:  "Student",
	}
	if err := repo.Create(ctx, first); err != nil {
		t.Fatalf("create first user: %v", err)
	}

	second := &User{
		Email:     strings.ToUpper(email),
		FirstName: "Second",
		LastName:  "Student",
	}
	err := repo.Create(ctx, second)

	if !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("expected ErrEmailTaken, got %v", err)
	}
}

func TestRepositoryGetByIDNotFound(t *testing.T) {
	repo := newTestRepository(t)

	_, err := repo.GetByID(context.Background(), uuid.New())

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
