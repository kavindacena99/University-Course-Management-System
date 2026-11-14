package enrollment

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

const enrollmentColumns = `
	id, student_user_id, offering_id, status, version, enrolled_at,
	ended_at, created_by_user_id, ended_by_user_id, created_at, updated_at
`

func scanEnrollment(row pgx.Row) (*Enrollment, error) {
	var item Enrollment
	err := row.Scan(
		&item.ID, &item.StudentUserID, &item.OfferingID, &item.Status,
		&item.Version, &item.EnrolledAt, &item.EndedAt,
		&item.CreatedByUserID, &item.EndedByUserID,
		&item.CreatedAt, &item.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan enrollment: %w", err)
	}
	return &item, nil
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*Enrollment, error) {
	return scanEnrollment(r.db.QueryRow(ctx,
		`SELECT `+enrollmentColumns+` FROM enrollments WHERE id = $1`, id))
}

func (r *Repository) GetActive(
	ctx context.Context, studentID, offeringID uuid.UUID,
) (*Enrollment, error) {
	return scanEnrollment(r.db.QueryRow(ctx,
		`SELECT `+enrollmentColumns+`
		   FROM enrollments
		  WHERE student_user_id = $1 AND offering_id = $2 AND status = 'active'`,
		studentID, offeringID))
}

// Create serializes all seat-consuming writes for an Offering using a local
// guard row. Course owns capacity configuration; Enrollment owns seat counts.
func (r *Repository) Create(
	ctx context.Context, item *Enrollment, capacity *int,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin enrollment transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := lockOfferingGuard(ctx, tx, item.OfferingID); err != nil {
		return err
	}
	if capacity != nil {
		var occupied int
		if err := tx.QueryRow(ctx,
			`SELECT count(*) FROM enrollments
			  WHERE offering_id = $1 AND status = 'active'`,
			item.OfferingID,
		).Scan(&occupied); err != nil {
			return fmt.Errorf("count occupied seats: %w", err)
		}
		if occupied >= *capacity {
			return ErrCapacityReached
		}
	}

	row := tx.QueryRow(ctx,
		`INSERT INTO enrollments (
			id, student_user_id, offering_id, status, version,
			enrolled_at, created_by_user_id
		) VALUES ($1, $2, $3, 'active', 1, now(), $4)
		RETURNING `+enrollmentColumns,
		item.ID, item.StudentUserID, item.OfferingID, item.CreatedByUserID,
	)
	saved, err := scanEnrollment(row)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDuplicateActive
		}
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit enrollment: %w", err)
	}
	*item = *saved
	return nil
}

func (r *Repository) Cancel(
	ctx context.Context, id uuid.UUID, expectedVersion int64, actorID uuid.UUID,
) (*Enrollment, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin cancellation transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var offeringID uuid.UUID
	if err := tx.QueryRow(ctx,
		`SELECT offering_id FROM enrollments WHERE id = $1`, id,
	).Scan(&offeringID); errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, fmt.Errorf("read enrollment offering: %w", err)
	}
	if err := lockOfferingGuard(ctx, tx, offeringID); err != nil {
		return nil, err
	}

	var status EnrollmentStatus
	var version int64
	if err := tx.QueryRow(ctx,
		`SELECT status, version FROM enrollments WHERE id = $1 FOR UPDATE`, id,
	).Scan(&status, &version); err != nil {
		return nil, fmt.Errorf("lock enrollment: %w", err)
	}
	if version != expectedVersion {
		return nil, ErrVersionConflict
	}
	if !status.CanTransitionTo(EnrollmentCancelled) {
		return nil, ErrInvalidTransition
	}
	item, err := scanEnrollment(tx.QueryRow(ctx,
		`UPDATE enrollments
		    SET status = 'cancelled', version = version + 1,
		        ended_at = now(), ended_by_user_id = $2, updated_at = now()
		  WHERE id = $1
		RETURNING `+enrollmentColumns,
		id, actorID,
	))
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit cancellation: %w", err)
	}
	return item, nil
}

func lockOfferingGuard(ctx context.Context, tx pgx.Tx, offeringID uuid.UUID) error {
	if _, err := tx.Exec(ctx,
		`INSERT INTO enrollment_offering_guards (offering_id)
		 VALUES ($1) ON CONFLICT DO NOTHING`, offeringID,
	); err != nil {
		return fmt.Errorf("create offering guard: %w", err)
	}
	if err := tx.QueryRow(ctx,
		`SELECT offering_id FROM enrollment_offering_guards
		  WHERE offering_id = $1 FOR UPDATE`, offeringID,
	).Scan(new(uuid.UUID)); err != nil {
		return fmt.Errorf("lock offering guard: %w", err)
	}
	return nil
}

func (r *Repository) List(
	ctx context.Context, filter EnrollmentFilter,
) (*EnrollmentList, error) {
	where := "WHERE ($1::uuid IS NULL OR student_user_id = $1) " +
		"AND ($2::uuid IS NULL OR offering_id = $2) " +
		"AND ($3::text IS NULL OR status = $3)"
	var studentID, offeringID any
	if filter.StudentUserID != nil {
		studentID = *filter.StudentUserID
	}
	if filter.OfferingID != nil {
		offeringID = *filter.OfferingID
	}
	var status any
	if filter.Status != "" {
		status = filter.Status
	}
	args := []any{studentID, offeringID, status}
	result := &EnrollmentList{Limit: filter.Limit, Offset: filter.Offset}
	if err := r.db.QueryRow(ctx,
		"SELECT count(*) FROM enrollments "+where, args...,
	).Scan(&result.Total); err != nil {
		return nil, fmt.Errorf("count enrollments: %w", err)
	}
	rows, err := r.db.Query(ctx,
		"SELECT "+enrollmentColumns+" FROM enrollments "+where+
			" ORDER BY created_at DESC, id DESC LIMIT $4 OFFSET $5",
		append(args, filter.Limit, filter.Offset)...,
	)
	if err != nil {
		return nil, fmt.Errorf("list enrollments: %w", err)
	}
	defer rows.Close()
	result.Enrollments = make([]Enrollment, 0)
	for rows.Next() {
		item, err := scanEnrollment(rows)
		if err != nil {
			return nil, err
		}
		result.Enrollments = append(result.Enrollments, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate enrollments: %w", err)
	}
	return result, nil
}
