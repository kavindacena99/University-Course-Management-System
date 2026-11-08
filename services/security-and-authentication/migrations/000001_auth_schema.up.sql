CREATE TABLE auth_users (
    user_id UUID PRIMARY KEY,
    email TEXT NOT NULL,
    first_name TEXT NOT NULL DEFAULT '',
    last_name TEXT NOT NULL DEFAULT '',
    registration_status VARCHAR(20) NOT NULL,
    user_version BIGINT NOT NULL DEFAULT 1,
    password_hash TEXT NULL,
    password_updated_at TIMESTAMPTZ NULL,
    activated_at TIMESTAMPTZ NULL,
    disabled BOOLEAN NOT NULL DEFAULT FALSE,
    failed_login_attempts INTEGER NOT NULL DEFAULT 0,
    locked_until TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_auth_users_status
        CHECK (
            registration_status IN (
                'pending',
                'approved',
                'rejected',
                'suspended'
            )
        ),

    CONSTRAINT chk_auth_users_version
        CHECK (user_version >= 1),

    CONSTRAINT chk_auth_users_failed_attempts
        CHECK (failed_login_attempts >= 0)
);

CREATE UNIQUE INDEX uq_auth_users_email_lower
    ON auth_users (LOWER(email));

CREATE TABLE auth_roles (
    code VARCHAR(20) PRIMARY KEY,
    description TEXT NOT NULL DEFAULT ''
);

INSERT INTO auth_roles (code, description) VALUES
    ('ADMIN', 'System administrator'),
    ('STAFF', 'University staff member'),
    ('LECTURER', 'Lecturer'),
    ('STUDENT', 'Student');

CREATE TABLE auth_user_roles (
    user_id UUID NOT NULL REFERENCES auth_users (user_id) ON DELETE CASCADE,
    role_code VARCHAR(20) NOT NULL REFERENCES auth_roles (code) ON DELETE RESTRICT,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (user_id, role_code)
);

CREATE INDEX idx_auth_user_roles_role
    ON auth_user_roles (role_code);

CREATE TABLE auth_sessions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES auth_users (user_id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    absolute_expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ NULL,
    revoked_reason TEXT NOT NULL DEFAULT '',
    user_agent TEXT NOT NULL DEFAULT '',
    ip_address TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_auth_sessions_user_active
    ON auth_sessions (user_id, last_used_at DESC)
    WHERE revoked_at IS NULL;

CREATE TABLE auth_refresh_tokens (
    id UUID PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES auth_sessions (id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ NULL,
    revoked_at TIMESTAMPTZ NULL,
    replaced_by_id UUID NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_auth_refresh_tokens_hash UNIQUE (token_hash)
);

CREATE INDEX idx_auth_refresh_tokens_session
    ON auth_refresh_tokens (session_id, created_at DESC);

CREATE TABLE auth_action_tokens (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES auth_users (user_id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL,
    purpose VARCHAR(30) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_auth_action_tokens_hash UNIQUE (token_hash),

    CONSTRAINT chk_auth_action_tokens_purpose
        CHECK (purpose IN ('activation', 'password_reset'))
);

CREATE INDEX idx_auth_action_tokens_user_purpose
    ON auth_action_tokens (user_id, purpose, expires_at);

CREATE TABLE auth_inbox_events (
    event_id UUID PRIMARY KEY,
    event_type VARCHAR(100) NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ NULL,
    result VARCHAR(20) NOT NULL DEFAULT 'pending',

    CONSTRAINT chk_auth_inbox_events_result
        CHECK (result IN ('pending', 'processed', 'stale'))
);

CREATE INDEX idx_auth_inbox_events_received
    ON auth_inbox_events (received_at);
