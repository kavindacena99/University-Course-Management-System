package user

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrEmailTaken = errors.New("email already exists")
	ErrNotFound   = errors.New("user not found")
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetByID(
	ctx context.Context,
	id uuid.UUID,
) (*User, error) {
	const query = `
		SELECT
			id,
			email,
			first_name,
			last_name,
			registration_status,
			created_at,
			updated_at
		FROM users
		WHERE id = $1
	`

	var u User

	err := r.db.QueryRow(ctx, query, id).Scan(
		&u.ID,
		&u.Email,
		&u.FirstName,
		&u.LastName,
		&u.RegistrationStatus,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("get user by ID: %w", err)
	}

	return &u, nil
}

func (r *Repository) Create(ctx context.Context, u *User) error {
	u.Email = strings.ToLower(strings.TrimSpace(u.Email))
	u.FirstName = strings.TrimSpace(u.FirstName)
	u.LastName = strings.TrimSpace(u.LastName)

	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}

	if u.RegistrationStatus == "" {
		u.RegistrationStatus = StatusPending
	}

	const query = `
		INSERT INTO users (
			id, email, first_name, last_name, registration_status
		)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING created_at, updated_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		u.ID,
		u.Email,
		u.FirstName,
		u.LastName,
		u.RegistrationStatus,
	).Scan(&u.CreatedAt, &u.UpdatedAt)

	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) &&
			pgErr.Code == "23505" &&
			(pgErr.ConstraintName == "uq_users_email" ||
				pgErr.ConstraintName == "uq_users_email_lower") {
			return ErrEmailTaken
		}

		return fmt.Errorf("create user: %w", err)
	}

	return nil
}
