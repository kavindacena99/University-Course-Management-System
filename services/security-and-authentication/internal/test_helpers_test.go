package auth

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set; skipping PostgreSQL integration test")
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse test database URL: %v", err)
	}
	if cfg.ConnConfig.Database != "auth_test" {
		t.Fatalf("refusing to run repository tests against database %q; want auth_test",
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
	return pool
}

func newTestAuthRepository(t *testing.T) (*Repository, *pgxpool.Pool) {
	t.Helper()

	pool := newTestPool(t)
	repo := NewRepository(pool, 24*time.Hour, 30*time.Minute, time.Hour)
	return repo, pool
}

func newTestAuthService(t *testing.T) (
	*Service,
	*Repository,
	*pgxpool.Pool,
	*testNotificationSender,
) {
	t.Helper()

	repo, pool := newTestAuthRepository(t)
	hasher, err := NewPasswordHasher(8192, 1, 1, 16, 16)
	if err != nil {
		t.Fatalf("create password hasher: %v", err)
	}

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate token key: %v", err)
	}
	tokens, err := NewTokenManager(
		privateKey,
		"test-key",
		"university-auth",
		"university-api",
		time.Minute,
		0,
	)
	if err != nil {
		t.Fatalf("create token manager: %v", err)
	}

	notifications := &testNotificationSender{}
	service, err := NewService(
		repo,
		hasher,
		tokens,
		notifications,
		&testEligibilityChecker{},
		time.Minute,
		time.Hour,
		24*time.Hour,
		30*time.Minute,
		3,
		time.Minute,
	)
	if err != nil {
		t.Fatalf("create auth service: %v", err)
	}

	return service, repo, pool, notifications
}

type testEligibilityChecker struct {
	err error
}

func (c *testEligibilityChecker) CheckEligibility(
	_ context.Context,
	_ uuid.UUID,
) error {
	return c.err
}

type testNotificationSender struct {
	mu              sync.Mutex
	activationToken string
	activationUser  *AuthUser
	resetToken      string
	resetUser       *AuthUser
}

func (s *testNotificationSender) SendActivation(
	_ context.Context,
	user *AuthUser,
	token string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activationToken = token
	s.activationUser = user
	return nil
}

func (s *testNotificationSender) SendPasswordReset(
	_ context.Context,
	user *AuthUser,
	token string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resetToken = token
	s.resetUser = user
	return nil
}

func (s *testNotificationSender) activation() (string, *AuthUser) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.activationToken, s.activationUser
}

func (s *testNotificationSender) reset() (string, *AuthUser) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.resetToken, s.resetUser
}

func uniqueTestEmail() string {
	return fmt.Sprintf("test-%s@example.com", uuid.NewString())
}

func cleanupAuthUser(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID) {
	t.Helper()

	if _, err := pool.Exec(context.Background(),
		`DELETE FROM auth_users WHERE user_id = $1`, userID,
	); err != nil {
		t.Fatalf("clean up auth user: %v", err)
	}
}

func cleanupInboxEvent(t *testing.T, pool *pgxpool.Pool, eventID uuid.UUID) {
	t.Helper()

	if _, err := pool.Exec(context.Background(),
		`DELETE FROM auth_inbox_events WHERE event_id = $1`, eventID,
	); err != nil {
		t.Fatalf("clean up inbox event: %v", err)
	}
}
