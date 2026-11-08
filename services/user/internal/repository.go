package user

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const userColumns = `
	id,
	email,
	first_name,
	last_name,
	registration_status,
	status_reason,
	reviewed_by,
	reviewed_at,
	version,
	created_at,
	updated_at
`

// Repository persists users, status history, and outbox events in PostgreSQL.
type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Ping(ctx context.Context) error {
	return r.db.Ping(ctx)
}

func (r *Repository) CreatePending(ctx context.Context, u *User) error {
	normalizeUser(u)
	u.RegistrationStatus = StatusPending
	u.StatusReason = ""
	u.ReviewedBy = nil
	u.ReviewedAt = nil

	return r.withTx(ctx, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			INSERT INTO users (
				id, email, first_name, last_name, registration_status, status_reason
			)
			VALUES ($1, $2, $3, $4, $5, '')
			RETURNING version, created_at, updated_at
		`,
			u.ID, u.Email, u.FirstName, u.LastName, u.RegistrationStatus,
		).Scan(&u.Version, &u.CreatedAt, &u.UpdatedAt)
		if err != nil {
			return mapUserWriteError(err)
		}

		return insertStatusHistory(ctx, tx, u.ID, nil, u.RegistrationStatus, "", nil, u.Version)
	})
}

// CreateApproved inserts an already-approved user without emitting an outbox
// event. It is used by the one-time administrator bootstrap flow, which
// provisions credentials directly with the Authentication Service.
func (r *Repository) CreateApproved(
	ctx context.Context,
	u *User,
	reason string,
	reviewedBy *uuid.UUID,
) error {
	normalizeUser(u)
	u.RegistrationStatus = StatusApproved
	u.StatusReason = reason

	return r.withTx(ctx, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			INSERT INTO users (
				id, email, first_name, last_name, registration_status,
				status_reason, reviewed_by, reviewed_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
			RETURNING version, reviewed_at, created_at, updated_at
		`,
			u.ID, u.Email, u.FirstName, u.LastName, u.RegistrationStatus,
			u.StatusReason, reviewedBy,
		).Scan(&u.Version, &u.ReviewedAt, &u.CreatedAt, &u.UpdatedAt)
		if err != nil {
			return mapUserWriteError(err)
		}

		return insertStatusHistory(
			ctx, tx, u.ID, nil, u.RegistrationStatus, reason, reviewedBy, u.Version,
		)
	})
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	row := r.db.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id)
	u, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return u, nil
}

func (r *Repository) GetByEmail(ctx context.Context, email string) (*User, error) {
	email = normalizeEmail(email)
	row := r.db.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE LOWER(email) = $1`, email)
	u, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return u, nil
}

func (r *Repository) UpdateProfile(
	ctx context.Context,
	id uuid.UUID,
	expectedVersion int64,
	firstName string,
	lastName string,
) (*User, error) {
	returnValue := &User{}

	err := r.withTx(ctx, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			UPDATE users
			SET first_name = $3,
				last_name = $4,
				version = version + 1,
				updated_at = NOW()
			WHERE id = $1 AND version = $2
			RETURNING `+userColumns,
			id, expectedVersion, firstName, lastName,
		)

		u, err := scanUser(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return classifyUserUpdateMiss(ctx, tx, id)
			}
			return fmt.Errorf("update user profile: %w", err)
		}

		if err := enqueueOutboxEvent(
			ctx, tx, EventUserProfileUpdated, u, u.UpdatedAt,
		); err != nil {
			return err
		}

		*returnValue = *u
		return nil
	})

	if err != nil {
		return nil, err
	}

	return returnValue, nil
}

type ChangeStatusInput struct {
	UserID          uuid.UUID
	ExpectedVersion int64
	FromStatus      RegistrationStatus
	ToStatus        RegistrationStatus
	Reason          string
	ChangedBy       uuid.UUID
	EventType       string
}

func (r *Repository) ChangeStatus(
	ctx context.Context,
	input ChangeStatusInput,
) (*User, error) {
	returnValue := &User{}

	err := r.withTx(ctx, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			UPDATE users
			SET registration_status = $4,
				status_reason = $5,
				reviewed_by = $6,
				reviewed_at = NOW(),
				version = version + 1,
				updated_at = NOW()
			WHERE id = $1
				AND version = $2
				AND registration_status = $3
			RETURNING `+userColumns,
			input.UserID,
			input.ExpectedVersion,
			input.FromStatus,
			input.ToStatus,
			input.Reason,
			input.ChangedBy,
		)

		u, err := scanUser(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return classifyUserUpdateMiss(ctx, tx, input.UserID)
			}
			return fmt.Errorf("change user status: %w", err)
		}

		if err := insertStatusHistory(
			ctx, tx, u.ID, &input.FromStatus, input.ToStatus,
			input.Reason, &input.ChangedBy, u.Version,
		); err != nil {
			return err
		}

		if err := enqueueOutboxEvent(
			ctx, tx, input.EventType, u, u.UpdatedAt,
		); err != nil {
			return err
		}

		*returnValue = *u
		return nil
	})

	if err != nil {
		return nil, err
	}

	return returnValue, nil
}

func (r *Repository) ListUsers(ctx context.Context, filter ListUsersFilter) (*UserList, error) {
	conditions := []string{"1 = 1"}
	args := make([]any, 0, 5)

	if filter.Status != "" {
		args = append(args, filter.Status)
		conditions = append(conditions,
			fmt.Sprintf("registration_status = $%d", len(args)))
	}

	if search := strings.TrimSpace(filter.Search); search != "" {
		pattern := "%" + strings.ToLower(search) + "%"
		args = append(args, pattern)
		conditions = append(conditions, fmt.Sprintf(
			"(LOWER(email) LIKE $%d OR LOWER(first_name || ' ' || last_name) LIKE $%d)",
			len(args), len(args),
		))
	}

	where := strings.Join(conditions, " AND ")

	var total int64
	if err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM users WHERE `+where, args...,
	).Scan(&total); err != nil {
		return nil, fmt.Errorf("count users: %w", err)
	}

	args = append(args, filter.Limit, filter.Offset)
	query := fmt.Sprintf(`
		SELECT %s
		FROM users
		WHERE %s
		ORDER BY created_at DESC, id
		LIMIT $%d OFFSET $%d
	`, userColumns, where, len(args)-1, len(args))

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	users := make([]User, 0, filter.Limit)
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("scan listed user: %w", err)
		}
		users = append(users, *u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}

	return &UserList{
		Users:  users,
		Total:  total,
		Limit:  filter.Limit,
		Offset: filter.Offset,
	}, nil
}

func (r *Repository) GetHistory(
	ctx context.Context,
	userID uuid.UUID,
	limit int,
	offset int,
) ([]StatusHistory, int64, error) {
	var total int64
	if err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM user_status_history WHERE user_id = $1`, userID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count user history: %w", err)
	}

	rows, err := r.db.Query(ctx, `
		SELECT
			id, user_id, from_status, to_status, reason, changed_by, version, changed_at
		FROM user_status_history
		WHERE user_id = $1
		ORDER BY changed_at DESC, id DESC
		LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list user history: %w", err)
	}
	defer rows.Close()

	history := make([]StatusHistory, 0, limit)
	for rows.Next() {
		var item StatusHistory
		var fromStatus pgtype.Text
		var changedBy pgtype.UUID

		if err := rows.Scan(
			&item.ID,
			&item.UserID,
			&fromStatus,
			&item.ToStatus,
			&item.Reason,
			&changedBy,
			&item.Version,
			&item.ChangedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan user history: %w", err)
		}

		if fromStatus.Valid {
			status := RegistrationStatus(fromStatus.String)
			item.FromStatus = &status
		}
		if changedBy.Valid {
			id := uuid.UUID(changedBy.Bytes)
			item.ChangedBy = &id
		}

		history = append(history, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate user history: %w", err)
	}

	return history, total, nil
}

// ClaimOutboxBatch leases a batch of pending events so that concurrent
// dispatchers do not deliver the same event twice.
func (r *Repository) ClaimOutboxBatch(ctx context.Context, limit int) ([]OutboxEvent, error) {
	rows, err := r.db.Query(ctx, `
		WITH claimed AS (
			SELECT id
			FROM outbox_events
			WHERE status = 'pending'
				AND next_attempt_at <= NOW()
			ORDER BY next_attempt_at, created_at
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE outbox_events AS e
		SET next_attempt_at = NOW() + make_interval(secs => $2)
		FROM claimed
		WHERE e.id = claimed.id
		RETURNING e.id, e.event_type, e.payload, e.attempts, e.created_at, e.next_attempt_at
	`, limit, 30)
	if err != nil {
		return nil, fmt.Errorf("claim outbox batch: %w", err)
	}
	defer rows.Close()

	events := make([]OutboxEvent, 0, limit)
	for rows.Next() {
		var event OutboxEvent
		if err := rows.Scan(
			&event.ID,
			&event.EventType,
			&event.Payload,
			&event.Attempts,
			&event.CreatedAt,
			&event.NextAction,
		); err != nil {
			return nil, fmt.Errorf("scan outbox event: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate outbox events: %w", err)
	}

	return events, nil
}

func (r *Repository) MarkOutboxDelivered(ctx context.Context, eventID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		UPDATE outbox_events
		SET status = 'delivered',
			delivered_at = NOW(),
			attempts = attempts + 1,
			last_error = ''
		WHERE id = $1
	`, eventID)
	if err != nil {
		return fmt.Errorf("mark outbox delivered: %w", err)
	}
	return nil
}

func (r *Repository) MarkOutboxFailed(
	ctx context.Context,
	eventID uuid.UUID,
	dead bool,
	lastError string,
	retryAt time.Time,
) error {
	status := OutboxStatusPending
	if dead {
		status = OutboxStatusDead
	}
	if len(lastError) > 2000 {
		lastError = lastError[:2000]
	}

	_, err := r.db.Exec(ctx, `
		UPDATE outbox_events
		SET status = $2,
			attempts = attempts + 1,
			last_error = $3,
			next_attempt_at = $4
		WHERE id = $1
	`, eventID, status, lastError, retryAt)
	if err != nil {
		return fmt.Errorf("mark outbox failed: %w", err)
	}
	return nil
}

func (r *Repository) PurgeDeliveredOutbox(ctx context.Context, olderThan time.Time) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM outbox_events
		WHERE status = 'delivered' AND delivered_at < $1
	`, olderThan)
	if err != nil {
		return fmt.Errorf("purge delivered outbox events: %w", err)
	}
	return nil
}

func (r *Repository) withTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

type rowScanner interface {
	Scan(destination ...any) error
}

func scanUser(row rowScanner) (*User, error) {
	var u User
	var reviewedBy pgtype.UUID
	var reviewedAt pgtype.Timestamptz

	if err := row.Scan(
		&u.ID,
		&u.Email,
		&u.FirstName,
		&u.LastName,
		&u.RegistrationStatus,
		&u.StatusReason,
		&reviewedBy,
		&reviewedAt,
		&u.Version,
		&u.CreatedAt,
		&u.UpdatedAt,
	); err != nil {
		return nil, err
	}

	if reviewedBy.Valid {
		id := uuid.UUID(reviewedBy.Bytes)
		u.ReviewedBy = &id
	}
	if reviewedAt.Valid {
		value := reviewedAt.Time
		u.ReviewedAt = &value
	}

	return &u, nil
}

func insertStatusHistory(
	ctx context.Context,
	tx pgx.Tx,
	userID uuid.UUID,
	fromStatus *RegistrationStatus,
	toStatus RegistrationStatus,
	reason string,
	changedBy *uuid.UUID,
	version int64,
) error {
	var from any
	if fromStatus != nil {
		from = string(*fromStatus)
	}
	var changedByValue any
	if changedBy != nil {
		changedByValue = *changedBy
	}

	_, err := tx.Exec(ctx, `
		INSERT INTO user_status_history (
			user_id, from_status, to_status, reason, changed_by, version
		)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, userID, from, toStatus, reason, changedByValue, version)
	if err != nil {
		return fmt.Errorf("insert status history: %w", err)
	}
	return nil
}

func enqueueOutboxEvent(
	ctx context.Context,
	tx pgx.Tx,
	eventType string,
	u *User,
	occurredAt time.Time,
) error {
	event := UserEvent{
		EventID:    uuid.New(),
		EventType:  eventType,
		OccurredAt: occurredAt.UTC(),
		User:       NewUserEventPayload(u),
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal outbox event: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO outbox_events (
			id, aggregate_type, aggregate_id, event_type, payload
		)
		VALUES ($1, 'user', $2, $3, $4)
	`, event.EventID, u.ID, eventType, payload)
	if err != nil {
		return fmt.Errorf("insert outbox event: %w", err)
	}
	return nil
}

func classifyUserUpdateMiss(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	var exists bool
	err := tx.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM users WHERE id = $1)`, userID,
	).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check user existence: %w", err)
	}
	if !exists {
		return ErrNotFound
	}
	return ErrVersionConflict
}

func mapUserWriteError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "uq_users_email", "uq_users_email_lower":
			return ErrEmailTaken
		}
	}
	return err
}

func normalizeUser(u *User) {
	u.Email = normalizeEmail(u.Email)
	u.FirstName = strings.TrimSpace(u.FirstName)
	u.LastName = strings.TrimSpace(u.LastName)
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
