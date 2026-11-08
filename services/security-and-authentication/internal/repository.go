package auth

import (
	"context"
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

const authUserColumns = `
	user_id,
	email,
	first_name,
	last_name,
	registration_status,
	user_version,
	password_hash,
	password_updated_at,
	activated_at,
	disabled,
	failed_login_attempts,
	locked_until,
	created_at,
	updated_at
`

type Repository struct {
	db            *pgxpool.Pool
	activationTTL time.Duration
	resetTTL      time.Duration
	refreshTTL    time.Duration
}

func NewRepository(
	db *pgxpool.Pool,
	activationTTL time.Duration,
	resetTTL time.Duration,
	refreshTTL time.Duration,
) *Repository {
	return &Repository{
		db:            db,
		activationTTL: activationTTL,
		resetTTL:      resetTTL,
		refreshTTL:    refreshTTL,
	}
}

func (r *Repository) Ping(ctx context.Context) error {
	return r.db.Ping(ctx)
}

func (r *Repository) GetUserByID(ctx context.Context, userID uuid.UUID) (*AuthUser, error) {
	return r.getUser(ctx, r.db,
		`SELECT `+authUserColumns+` FROM auth_users WHERE user_id = $1`, userID)
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*AuthUser, error) {
	email = normalizeEmail(email)
	return r.getUser(ctx, r.db,
		`SELECT `+authUserColumns+` FROM auth_users WHERE LOWER(email) = $1`, email)
}

func (r *Repository) RecordLoginFailure(
	ctx context.Context,
	userID uuid.UUID,
	maxAttempts int,
	lockedUntil time.Time,
) error {
	_, err := r.db.Exec(ctx, `
		UPDATE auth_users
		SET failed_login_attempts = failed_login_attempts + 1,
			locked_until = CASE
				WHEN failed_login_attempts + 1 >= $2 THEN $3
				ELSE locked_until
			END,
			updated_at = NOW()
		WHERE user_id = $1
	`, userID, maxAttempts, lockedUntil)
	if err != nil {
		return fmt.Errorf("record login failure: %w", err)
	}
	return nil
}

func (r *Repository) RecordLoginSuccess(
	ctx context.Context,
	userID uuid.UUID,
	rehashedPassword string,
) error {
	_, err := r.db.Exec(ctx, `
		UPDATE auth_users
		SET failed_login_attempts = 0,
			locked_until = NULL,
			password_hash = CASE WHEN $2 <> '' THEN $2 ELSE password_hash END,
			password_updated_at = CASE WHEN $2 <> '' THEN NOW() ELSE password_updated_at END,
			updated_at = NOW()
		WHERE user_id = $1
	`, userID, rehashedPassword)
	if err != nil {
		return fmt.Errorf("record login success: %w", err)
	}
	return nil
}

type NewSession struct {
	ID                    uuid.UUID
	UserID                uuid.UUID
	UserAgent             string
	IPAddress             string
	AbsoluteExpiresAt     time.Time
	RefreshTokenID        uuid.UUID
	RefreshTokenHash      []byte
	RefreshTokenExpiresAt time.Time
}

func (r *Repository) CreateSession(ctx context.Context, session NewSession) error {
	return r.withTx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO auth_sessions (
				id, user_id, absolute_expires_at, user_agent, ip_address
			)
			VALUES ($1, $2, $3, $4, $5)
		`, session.ID, session.UserID, session.AbsoluteExpiresAt,
			session.UserAgent, session.IPAddress)
		if err != nil {
			return fmt.Errorf("insert session: %w", err)
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO auth_refresh_tokens (
				id, session_id, token_hash, expires_at
			)
			VALUES ($1, $2, $3, $4)
		`, session.RefreshTokenID, session.ID,
			session.RefreshTokenHash, session.RefreshTokenExpiresAt)
		if err != nil {
			return fmt.Errorf("insert refresh token: %w", err)
		}

		return nil
	})
}

type RotationResult struct {
	User          *AuthUser
	SessionID     uuid.UUID
	RefreshExpiry time.Time
}

func (r *Repository) RotateRefreshToken(
	ctx context.Context,
	rawHash []byte,
	newTokenID uuid.UUID,
	newHash []byte,
	now time.Time,
) (*RotationResult, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin refresh rotation: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var tokenID, sessionID, userID uuid.UUID
	var tokenExpiresAt, sessionExpiresAt time.Time
	var usedAt, tokenRevokedAt, sessionRevokedAt pgtype.Timestamptz

	err = tx.QueryRow(ctx, `
		SELECT
			rt.id,
			rt.session_id,
			rt.expires_at,
			rt.used_at,
			rt.revoked_at,
			s.user_id,
			s.absolute_expires_at,
			s.revoked_at
		FROM auth_refresh_tokens rt
		JOIN auth_sessions s ON s.id = rt.session_id
		WHERE rt.token_hash = $1
		FOR UPDATE OF rt, s
	`, rawHash).Scan(
		&tokenID,
		&sessionID,
		&tokenExpiresAt,
		&usedAt,
		&tokenRevokedAt,
		&userID,
		&sessionExpiresAt,
		&sessionRevokedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRefreshInvalid
		}
		return nil, fmt.Errorf("load refresh token: %w", err)
	}

	if usedAt.Valid {
		if err := revokeSessionTx(ctx, tx, sessionID, "refresh token reuse detected"); err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit reuse revocation: %w", err)
		}
		return nil, ErrRefreshReuse
	}

	if tokenRevokedAt.Valid || sessionRevokedAt.Valid {
		return nil, ErrRefreshInvalid
	}
	if !now.Before(tokenExpiresAt) {
		return nil, ErrRefreshInvalid
	}
	if !now.Before(sessionExpiresAt) {
		if err := revokeSessionTx(ctx, tx, sessionID, "session expired"); err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit session expiry: %w", err)
		}
		return nil, ErrSessionExpired
	}

	newExpiry := now.Add(r.refreshTTL)
	if newExpiry.After(sessionExpiresAt) {
		newExpiry = sessionExpiresAt
	}

	if _, err := tx.Exec(ctx, `
		UPDATE auth_refresh_tokens
		SET used_at = $2, replaced_by_id = $3
		WHERE id = $1
	`, tokenID, now, newTokenID); err != nil {
		return nil, fmt.Errorf("mark refresh token used: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO auth_refresh_tokens (
			id, session_id, token_hash, expires_at
		)
		VALUES ($1, $2, $3, $4)
	`, newTokenID, sessionID, newHash, newExpiry); err != nil {
		return nil, fmt.Errorf("insert rotated refresh token: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE auth_sessions SET last_used_at = $2 WHERE id = $1
	`, sessionID, now); err != nil {
		return nil, fmt.Errorf("update session last used: %w", err)
	}

	user, err := r.getUser(ctx, tx,
		`SELECT `+authUserColumns+` FROM auth_users WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit refresh rotation: %w", err)
	}

	return &RotationResult{
		User:          user,
		SessionID:     sessionID,
		RefreshExpiry: newExpiry,
	}, nil
}

func (r *Repository) RevokeSessionByRefreshHash(
	ctx context.Context,
	rawHash []byte,
	reason string,
) (bool, error) {
	var sessionID uuid.UUID
	err := r.db.QueryRow(ctx, `
		SELECT session_id FROM auth_refresh_tokens WHERE token_hash = $1
	`, rawHash).Scan(&sessionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("find refresh token: %w", err)
	}

	if err := r.RevokeSession(ctx, sessionID, uuid.Nil, reason, true); err != nil {
		return false, err
	}
	return true, nil
}

func (r *Repository) RevokeSession(
	ctx context.Context,
	sessionID uuid.UUID,
	userID uuid.UUID,
	reason string,
	skipOwnership bool,
) error {
	return r.withTx(ctx, func(tx pgx.Tx) error {
		if !skipOwnership {
			var owner uuid.UUID
			err := tx.QueryRow(ctx,
				`SELECT user_id FROM auth_sessions WHERE id = $1`, sessionID,
			).Scan(&owner)
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			if err != nil {
				return fmt.Errorf("load session owner: %w", err)
			}
			if owner != userID {
				return ErrForbidden
			}
		}

		return revokeSessionTx(ctx, tx, sessionID, reason)
	})
}

func (r *Repository) RevokeAllSessions(
	ctx context.Context,
	userID uuid.UUID,
	exceptSessionID *uuid.UUID,
	reason string,
) error {
	return r.withTx(ctx, func(tx pgx.Tx) error {
		var except any
		if exceptSessionID != nil {
			except = *exceptSessionID
		}

		rows, err := tx.Query(ctx, `
			UPDATE auth_sessions
			SET revoked_at = NOW(), revoked_reason = $2
			WHERE user_id = $1
				AND revoked_at IS NULL
				AND ($3::uuid IS NULL OR id <> $3::uuid)
			RETURNING id
		`, userID, reason, except)
		if err != nil {
			return fmt.Errorf("revoke sessions: %w", err)
		}
		sessionIDs := make([]uuid.UUID, 0)
		for rows.Next() {
			var sessionID uuid.UUID
			if err := rows.Scan(&sessionID); err != nil {
				rows.Close()
				return fmt.Errorf("scan revoked session: %w", err)
			}
			sessionIDs = append(sessionIDs, sessionID)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate revoked sessions: %w", err)
		}

		if len(sessionIDs) == 0 {
			return nil
		}

		_, err = tx.Exec(ctx, `
			UPDATE auth_refresh_tokens
			SET revoked_at = NOW()
			WHERE session_id = ANY($1) AND revoked_at IS NULL
		`, sessionIDs)
		if err != nil {
			return fmt.Errorf("revoke refresh tokens: %w", err)
		}

		return nil
	})
}

func (r *Repository) ListSessions(
	ctx context.Context,
	userID uuid.UUID,
	currentSessionID uuid.UUID,
) ([]Session, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			id, user_id, created_at, last_used_at, absolute_expires_at,
			revoked_at, user_agent, ip_address
		FROM auth_sessions
		WHERE user_id = $1
			AND revoked_at IS NULL
			AND absolute_expires_at > NOW()
		ORDER BY last_used_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	sessions := make([]Session, 0)
	for rows.Next() {
		var session Session
		var revokedAt pgtype.Timestamptz
		if err := rows.Scan(
			&session.ID,
			&session.UserID,
			&session.CreatedAt,
			&session.LastUsedAt,
			&session.AbsoluteExpiresAt,
			&revokedAt,
			&session.UserAgent,
			&session.IPAddress,
		); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		if revokedAt.Valid {
			value := revokedAt.Time
			session.RevokedAt = &value
		}
		session.Current = session.ID == currentSessionID
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sessions: %w", err)
	}

	return sessions, nil
}

func (r *Repository) ChangePassword(
	ctx context.Context,
	userID uuid.UUID,
	newPasswordHash string,
	revokeSessionsExcept *uuid.UUID,
) error {
	return r.withTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE auth_users
			SET password_hash = $2,
				password_updated_at = NOW(),
				failed_login_attempts = 0,
				locked_until = NULL,
				updated_at = NOW()
			WHERE user_id = $1
		`, userID, newPasswordHash)
		if err != nil {
			return fmt.Errorf("update password: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}

		return revokeAllSessionsTx(ctx, tx, userID, revokeSessionsExcept,
			"password changed")
	})
}

func (r *Repository) CreatePasswordResetToken(
	ctx context.Context,
	userID uuid.UUID,
	tokenHash []byte,
	expiresAt time.Time,
) error {
	return r.withTx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			DELETE FROM auth_action_tokens
			WHERE user_id = $1 AND purpose = $2 AND used_at IS NULL
		`, userID, ActionTokenPasswordReset)
		if err != nil {
			return fmt.Errorf("delete previous reset tokens: %w", err)
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO auth_action_tokens (
				id, user_id, token_hash, purpose, expires_at
			)
			VALUES ($1, $2, $3, $4, $5)
		`, uuid.New(), userID, tokenHash, ActionTokenPasswordReset, expiresAt)
		if err != nil {
			return fmt.Errorf("insert reset token: %w", err)
		}

		return nil
	})
}

func (r *Repository) ConsumeActionTokenAndSetPassword(
	ctx context.Context,
	tokenHash []byte,
	purpose string,
	newPasswordHash string,
	now time.Time,
) (*AuthUser, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin action token transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var tokenID, userID uuid.UUID
	var expiresAt time.Time
	var usedAt pgtype.Timestamptz

	err = tx.QueryRow(ctx, `
		SELECT id, user_id, expires_at, used_at
		FROM auth_action_tokens
		WHERE token_hash = $1 AND purpose = $2
		FOR UPDATE
	`, tokenHash, purpose).Scan(&tokenID, &userID, &expiresAt, &usedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrActionTokenInvalid
	}
	if err != nil {
		return nil, fmt.Errorf("load action token: %w", err)
	}
	if usedAt.Valid {
		return nil, ErrActionTokenUsed
	}
	if !now.Before(expiresAt) {
		return nil, ErrActionTokenExpired
	}

	user, err := r.getUser(ctx, tx,
		`SELECT `+authUserColumns+` FROM auth_users WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	if user.RegistrationStatus != StatusApproved || user.Disabled {
		return nil, ErrAccountUnavailable
	}

	activatedAt := "NOW()"
	if purpose != ActionTokenActivation {
		activatedAt = "COALESCE(activated_at, NOW())"
	}

	_, err = tx.Exec(ctx, `
		UPDATE auth_users
		SET password_hash = $2,
			password_updated_at = NOW(),
			activated_at = `+activatedAt+`,
			failed_login_attempts = 0,
			locked_until = NULL,
			updated_at = NOW()
		WHERE user_id = $1
	`, userID, newPasswordHash)
	if err != nil {
		return nil, fmt.Errorf("set password from action token: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE auth_action_tokens SET used_at = $2 WHERE id = $1
	`, tokenID, now); err != nil {
		return nil, fmt.Errorf("mark action token used: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		DELETE FROM auth_action_tokens
		WHERE user_id = $1 AND purpose = $2 AND used_at IS NULL
	`, userID, purpose); err != nil {
		return nil, fmt.Errorf("delete other action tokens: %w", err)
	}

	if err := revokeAllSessionsTx(ctx, tx, userID, nil, "password changed"); err != nil {
		return nil, err
	}

	user, err = r.getUser(ctx, tx,
		`SELECT `+authUserColumns+` FROM auth_users WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit action token transaction: %w", err)
	}

	return user, nil
}

func (r *Repository) ProvisionUser(
	ctx context.Context,
	input ProvisionUserInput,
	passwordHash string,
) (*AuthUser, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin provisioning transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	version := input.Version
	if version < 1 {
		version = 1
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO auth_users (
			user_id, email, first_name, last_name, registration_status,
			user_version, password_hash, password_updated_at, activated_at,
			disabled, failed_login_attempts, locked_until
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW(), FALSE, 0, NULL)
		ON CONFLICT (user_id) DO UPDATE SET
			email = EXCLUDED.email,
			first_name = EXCLUDED.first_name,
			last_name = EXCLUDED.last_name,
			registration_status = EXCLUDED.registration_status,
			user_version = GREATEST(auth_users.user_version, EXCLUDED.user_version),
			password_hash = EXCLUDED.password_hash,
			password_updated_at = NOW(),
			activated_at = COALESCE(auth_users.activated_at, NOW()),
			disabled = FALSE,
			failed_login_attempts = 0,
			locked_until = NULL,
			updated_at = NOW()
	`, input.UserID, normalizeEmail(input.Email), input.FirstName,
		input.LastName, StatusApproved, version, passwordHash)
	if err != nil {
		return nil, mapEmailConflict(err)
	}

	for _, role := range input.Roles {
		if !IsValidRole(role) {
			return nil, fmt.Errorf("%w: %s", ErrInvalidRole, role)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO auth_user_roles (user_id, role_code)
			VALUES ($1, $2)
			ON CONFLICT DO NOTHING
		`, input.UserID, role); err != nil {
			return nil, fmt.Errorf("grant provisioning role: %w", err)
		}
	}

	if err := revokeAllSessionsTx(ctx, tx, input.UserID, nil,
		"credentials provisioned"); err != nil {
		return nil, err
	}

	user, err := r.getUser(ctx, tx,
		`SELECT `+authUserColumns+` FROM auth_users WHERE user_id = $1`, input.UserID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit provisioning transaction: %w", err)
	}

	return user, nil
}

func (r *Repository) ListRoles(ctx context.Context) ([]Role, error) {
	rows, err := r.db.Query(ctx, `
		SELECT code, description FROM auth_roles ORDER BY code
	`)
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	defer rows.Close()

	roles := make([]Role, 0)
	for rows.Next() {
		var role Role
		if err := rows.Scan(&role.Code, &role.Description); err != nil {
			return nil, fmt.Errorf("scan role: %w", err)
		}
		roles = append(roles, role)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate roles: %w", err)
	}

	return roles, nil
}

func (r *Repository) SetUserRoles(
	ctx context.Context,
	userID uuid.UUID,
	roles []string,
) (*AuthUser, error) {
	for _, role := range roles {
		if !IsValidRole(role) {
			return nil, fmt.Errorf("%w: %s", ErrInvalidRole, role)
		}
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin role update: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if _, err := tx.Exec(ctx, `
		SELECT code FROM auth_roles WHERE code = $1 FOR UPDATE
	`, RoleAdmin); err != nil {
		return nil, fmt.Errorf("lock admin role: %w", err)
	}

	existing, err := loadRoles(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	removingAdmin := contains(existing, RoleAdmin) && !contains(roles, RoleAdmin)
	if removingAdmin {
		var adminCount int
		if err := tx.QueryRow(ctx, `
			SELECT COUNT(DISTINCT user_id)
			FROM auth_user_roles
			WHERE role_code = $1
		`, RoleAdmin).Scan(&adminCount); err != nil {
			return nil, fmt.Errorf("count administrators: %w", err)
		}
		if adminCount <= 1 {
			return nil, ErrLastAdmin
		}
	}

	if _, err := tx.Exec(ctx,
		`DELETE FROM auth_user_roles WHERE user_id = $1`, userID,
	); err != nil {
		return nil, fmt.Errorf("delete user roles: %w", err)
	}

	for _, role := range roles {
		if _, err := tx.Exec(ctx, `
			INSERT INTO auth_user_roles (user_id, role_code)
			VALUES ($1, $2)
		`, userID, role); err != nil {
			return nil, fmt.Errorf("insert user role: %w", err)
		}
	}

	if err := revokeAllSessionsTx(ctx, tx, userID, nil, "roles updated"); err != nil {
		return nil, err
	}

	user, err := r.getUser(ctx, tx,
		`SELECT `+authUserColumns+` FROM auth_users WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit role update: %w", err)
	}

	return user, nil
}

func (r *Repository) UnlockUser(ctx context.Context, userID uuid.UUID) (*AuthUser, error) {
	tag, err := r.db.Exec(ctx, `
		UPDATE auth_users
		SET failed_login_attempts = 0, locked_until = NULL, updated_at = NOW()
		WHERE user_id = $1
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("unlock user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	return r.GetUserByID(ctx, userID)
}

type NotifyFunc func(user *AuthUser, token string) error

func (r *Repository) ProcessUserEvent(
	ctx context.Context,
	event UserEvent,
	now time.Time,
	notify NotifyFunc,
) (InboxResult, error) {
	if err := validateUserEvent(event); err != nil {
		return InboxResult{}, err
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return InboxResult{}, fmt.Errorf("begin event transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	tag, err := tx.Exec(ctx, `
		INSERT INTO auth_inbox_events (event_id, event_type)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, event.EventID, event.EventType)
	if err != nil {
		return InboxResult{}, fmt.Errorf("insert inbox event: %w", err)
	}
	if tag.RowsAffected() == 0 {
		result := InboxResult{Duplicate: true}
		if err := tx.Commit(ctx); err != nil {
			return InboxResult{}, fmt.Errorf("commit duplicate event: %w", err)
		}
		return result, nil
	}

	existing, err := r.getUser(ctx, tx,
		`SELECT `+authUserColumns+` FROM auth_users WHERE user_id = $1 FOR UPDATE`,
		event.User.UserID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return InboxResult{}, err
	}
	if existing != nil && event.User.Version <= existing.UserVersion {
		if _, err := tx.Exec(ctx, `
			UPDATE auth_inbox_events
			SET processed_at = NOW(), result = 'stale'
			WHERE event_id = $1
		`, event.EventID); err != nil {
			return InboxResult{}, fmt.Errorf("mark stale event: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return InboxResult{}, fmt.Errorf("commit stale event: %w", err)
		}
		return InboxResult{Stale: true}, nil
	}

	status := event.User.RegistrationStatus
	disabled := status != StatusApproved
	email := normalizeEmail(event.User.Email)

	if existing == nil {
		_, err = tx.Exec(ctx, `
			INSERT INTO auth_users (
				user_id, email, first_name, last_name, registration_status,
				user_version, disabled
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, event.User.UserID, email, event.User.FirstName,
			event.User.LastName, status, event.User.Version, disabled)
		if err != nil {
			return InboxResult{}, mapEmailConflict(err)
		}
	} else {
		_, err = tx.Exec(ctx, `
			UPDATE auth_users
			SET email = $2,
				first_name = $3,
				last_name = $4,
				registration_status = $5,
				user_version = $6,
				disabled = $7,
				updated_at = NOW()
			WHERE user_id = $1
		`, event.User.UserID, email, event.User.FirstName,
			event.User.LastName, status, event.User.Version, disabled)
		if err != nil {
			return InboxResult{}, mapEmailConflict(err)
		}
	}

	if err := ensureDefaultRole(ctx, tx, event.User.UserID); err != nil {
		return InboxResult{}, err
	}

	activationToken := ""
	if status == StatusApproved {
		user, err := r.getUser(ctx, tx,
			`SELECT `+authUserColumns+` FROM auth_users WHERE user_id = $1`,
			event.User.UserID)
		if err != nil {
			return InboxResult{}, err
		}

		if user.PasswordHash == "" && user.ActivatedAt == nil {
			if _, err := tx.Exec(ctx, `
				DELETE FROM auth_action_tokens
				WHERE user_id = $1 AND purpose = $2 AND used_at IS NULL
			`, user.UserID, ActionTokenActivation); err != nil {
				return InboxResult{}, fmt.Errorf("delete previous activation tokens: %w", err)
			}

			raw, hash, err := GenerateOpaqueToken()
			if err != nil {
				return InboxResult{}, err
			}
			_, err = tx.Exec(ctx, `
				INSERT INTO auth_action_tokens (
					id, user_id, token_hash, purpose, expires_at
				)
				VALUES ($1, $2, $3, $4, $5)
			`, uuid.New(), user.UserID, hash, ActionTokenActivation,
				now.Add(r.activationTTL))
			if err != nil {
				return InboxResult{}, fmt.Errorf("insert activation token: %w", err)
			}

			activationToken = raw
		}
	}

	if status == StatusRejected || status == StatusSuspended {
		if err := revokeAllSessionsTx(ctx, tx, event.User.UserID, nil,
			"account "+string(status)); err != nil {
			return InboxResult{}, err
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM auth_action_tokens
			WHERE user_id = $1 AND used_at IS NULL
		`, event.User.UserID); err != nil {
			return InboxResult{}, fmt.Errorf("delete unused action tokens: %w", err)
		}
	}

	if activationToken != "" && notify != nil {
		user, err := r.getUser(ctx, tx,
			`SELECT `+authUserColumns+` FROM auth_users WHERE user_id = $1`,
			event.User.UserID)
		if err != nil {
			return InboxResult{}, err
		}
		if err := notify(user, activationToken); err != nil {
			return InboxResult{}, fmt.Errorf("send activation notification: %w", err)
		}
	}

	if _, err := tx.Exec(ctx, `
		UPDATE auth_inbox_events
		SET processed_at = NOW(), result = 'processed'
		WHERE event_id = $1
	`, event.EventID); err != nil {
		return InboxResult{}, fmt.Errorf("mark inbox processed: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return InboxResult{}, fmt.Errorf("commit event: %w", err)
	}

	return InboxResult{}, nil
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

func (r *Repository) getUser(
	ctx context.Context,
	q querier,
	query string,
	args ...any,
) (*AuthUser, error) {
	row := q.QueryRow(ctx, query, args...)
	user, err := scanAuthUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("load auth user: %w", err)
	}

	roles, err := loadRoles(ctx, q, user.UserID)
	if err != nil {
		return nil, err
	}
	user.Roles = roles

	return user, nil
}

type querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type rowScanner interface {
	Scan(destination ...any) error
}

func scanAuthUser(row rowScanner) (*AuthUser, error) {
	var user AuthUser
	var passwordHash pgtype.Text
	var passwordUpdatedAt pgtype.Timestamptz
	var activatedAt pgtype.Timestamptz
	var lockedUntil pgtype.Timestamptz

	if err := row.Scan(
		&user.UserID,
		&user.Email,
		&user.FirstName,
		&user.LastName,
		&user.RegistrationStatus,
		&user.UserVersion,
		&passwordHash,
		&passwordUpdatedAt,
		&activatedAt,
		&user.Disabled,
		&user.FailedLoginAttempts,
		&lockedUntil,
		&user.CreatedAt,
		&user.UpdatedAt,
	); err != nil {
		return nil, err
	}

	if passwordHash.Valid {
		user.PasswordHash = passwordHash.String
	}
	if passwordUpdatedAt.Valid {
		value := passwordUpdatedAt.Time
		user.PasswordUpdatedAt = &value
	}
	if activatedAt.Valid {
		value := activatedAt.Time
		user.ActivatedAt = &value
	}
	if lockedUntil.Valid {
		value := lockedUntil.Time
		user.LockedUntil = &value
	}

	return &user, nil
}

func loadRoles(ctx context.Context, q querier, userID uuid.UUID) ([]string, error) {
	rows, err := q.Query(ctx, `
		SELECT role_code FROM auth_user_roles WHERE user_id = $1 ORDER BY role_code
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("load user roles: %w", err)
	}
	defer rows.Close()

	roles := make([]string, 0, len(AllRoles))
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return nil, fmt.Errorf("scan user role: %w", err)
		}
		roles = append(roles, role)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user roles: %w", err)
	}
	return roles, nil
}

func ensureDefaultRole(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	var count int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM auth_user_roles WHERE user_id = $1
	`, userID).Scan(&count); err != nil {
		return fmt.Errorf("count user roles: %w", err)
	}
	if count > 0 {
		return nil
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO auth_user_roles (user_id, role_code)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, userID, RoleStudent); err != nil {
		return fmt.Errorf("grant default role: %w", err)
	}
	return nil
}

func revokeSessionTx(
	ctx context.Context,
	tx pgx.Tx,
	sessionID uuid.UUID,
	reason string,
) error {
	if _, err := tx.Exec(ctx, `
		UPDATE auth_sessions
		SET revoked_at = COALESCE(revoked_at, NOW()), revoked_reason = $2
		WHERE id = $1
	`, sessionID, reason); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE auth_refresh_tokens
		SET revoked_at = COALESCE(revoked_at, NOW())
		WHERE session_id = $1
	`, sessionID); err != nil {
		return fmt.Errorf("revoke session refresh tokens: %w", err)
	}
	return nil
}

func revokeAllSessionsTx(
	ctx context.Context,
	tx pgx.Tx,
	userID uuid.UUID,
	exceptSessionID *uuid.UUID,
	reason string,
) error {
	var except any
	if exceptSessionID != nil {
		except = *exceptSessionID
	}

	rows, err := tx.Query(ctx, `
		UPDATE auth_sessions
		SET revoked_at = NOW(), revoked_reason = $2
		WHERE user_id = $1
			AND revoked_at IS NULL
			AND ($3::uuid IS NULL OR id <> $3::uuid)
		RETURNING id
	`, userID, reason, except)
	if err != nil {
		return fmt.Errorf("revoke user sessions: %w", err)
	}
	sessionIDs := make([]uuid.UUID, 0)
	for rows.Next() {
		var sessionID uuid.UUID
		if err := rows.Scan(&sessionID); err != nil {
			rows.Close()
			return fmt.Errorf("scan revoked session: %w", err)
		}
		sessionIDs = append(sessionIDs, sessionID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate revoked sessions: %w", err)
	}

	if len(sessionIDs) == 0 {
		return nil
	}

	if _, err := tx.Exec(ctx, `
		UPDATE auth_refresh_tokens
		SET revoked_at = NOW()
		WHERE session_id = ANY($1) AND revoked_at IS NULL
	`, sessionIDs); err != nil {
		return fmt.Errorf("revoke user refresh tokens: %w", err)
	}
	return nil
}

func validateUserEvent(event UserEvent) error {
	if event.EventID == uuid.Nil {
		return fmt.Errorf("event id is required")
	}
	switch event.EventType {
	case EventUserApproved, EventUserRejected, EventUserSuspended,
		EventUserReinstated, EventUserProfileUpdated:
	default:
		return fmt.Errorf("unsupported event type %q", event.EventType)
	}
	if event.User.UserID == uuid.Nil {
		return fmt.Errorf("user id is required")
	}
	if event.User.Version < 1 {
		return fmt.Errorf("user version must be positive")
	}
	if !event.User.RegistrationStatus.Valid() {
		return fmt.Errorf("invalid registration status")
	}
	if normalizeEmail(event.User.Email) == "" {
		return fmt.Errorf("user email is required")
	}
	return nil
}

func mapEmailConflict(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "uq_auth_users_email", "uq_auth_users_email_lower":
			return ErrEmailTaken
		}
	}
	return err
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
