package enrollment

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testRepository(t *testing.T) (*Repository, *pgxpool.Pool) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set; skipping PostgreSQL test")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ConnConfig.Database != "enrollments_test" {
		t.Fatalf("refusing to use database %q; want enrollments_test", cfg.ConnConfig.Database)
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return NewRepository(pool), pool
}

func cleanOffering(t *testing.T, pool *pgxpool.Pool, offeringID uuid.UUID) {
	t.Helper()
	t.Cleanup(func() {
		ctx := context.Background()
		if _, err := pool.Exec(ctx, `DELETE FROM enrollments WHERE offering_id = $1`, offeringID); err != nil {
			t.Errorf("delete test enrollments: %v", err)
		}
		if _, err := pool.Exec(ctx, `DELETE FROM enrollment_offering_guards WHERE offering_id = $1`, offeringID); err != nil {
			t.Errorf("delete test guard: %v", err)
		}
	})
}

func TestRepositoryUniqueActiveHistoryAndVersion(t *testing.T) {
	repo, pool := testRepository(t)
	offeringID, studentID, actorID := uuid.New(), uuid.New(), uuid.New()
	cleanOffering(t, pool, offeringID)
	ctx := context.Background()
	capacity := 2
	first := &Enrollment{
		ID: uuid.New(), StudentUserID: studentID, OfferingID: offeringID,
		CreatedByUserID: actorID,
	}
	if err := repo.Create(ctx, first, &capacity); err != nil {
		t.Fatal(err)
	}
	if first.Status != EnrollmentActive || first.Version != 1 || first.EnrolledAt.IsZero() {
		t.Fatalf("unexpected created enrollment: %+v", first)
	}
	duplicate := &Enrollment{
		ID: uuid.New(), StudentUserID: studentID, OfferingID: offeringID,
		CreatedByUserID: actorID,
	}
	if err := repo.Create(ctx, duplicate, &capacity); !errors.Is(err, ErrDuplicateActive) {
		t.Fatalf("duplicate active enrollment: %v", err)
	}
	if _, err := repo.Cancel(ctx, first.ID, 2, actorID); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("stale version: %v", err)
	}
	cancelled, err := repo.Cancel(ctx, first.ID, 1, actorID)
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.Status != EnrollmentCancelled || cancelled.Version != 2 ||
		cancelled.EndedAt == nil || cancelled.EndedByUserID == nil {
		t.Fatalf("unexpected cancelled enrollment: %+v", cancelled)
	}
	if _, err := repo.GetActive(ctx, studentID, offeringID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cancelled enrollment is still active: %v", err)
	}
	if err := repo.Create(ctx, duplicate, &capacity); err != nil {
		t.Fatalf("re-enrollment after cancellation: %v", err)
	}
	result, err := repo.List(ctx, EnrollmentFilter{StudentUserID: &studentID, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 2 || len(result.Enrollments) != 2 {
		t.Fatalf("history was not preserved: %+v", result)
	}
}

func TestRepositoryConcurrentCapacityAllowsOneWinner(t *testing.T) {
	repo, pool := testRepository(t)
	offeringID := uuid.New()
	cleanOffering(t, pool, offeringID)
	capacity := 1
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			studentID := uuid.New()
			results <- repo.Create(context.Background(), &Enrollment{
				ID: uuid.New(), StudentUserID: studentID, OfferingID: offeringID,
				CreatedByUserID: studentID,
			}, &capacity)
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	success, full := 0, 0
	for err := range results {
		switch {
		case err == nil:
			success++
		case errors.Is(err, ErrCapacityReached):
			full++
		default:
			t.Fatalf("unexpected concurrency error: %v", err)
		}
	}
	if success != 1 || full != 1 {
		t.Fatalf("success=%d, capacity_reached=%d", success, full)
	}
}

func TestDatabaseEnrollmentConstraints(t *testing.T) {
	_, pool := testRepository(t)
	offeringID := uuid.New()
	cleanOffering(t, pool, offeringID)
	_, err := pool.Exec(context.Background(),
		`INSERT INTO enrollments (
			id, student_user_id, offering_id, status, enrolled_at,
			created_by_user_id
		) VALUES ($1, $2, $3, 'cancelled', now(), $2)`,
		uuid.New(), uuid.New(), offeringID,
	)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Fatalf("expected check constraint violation, got %v", err)
	}
}
