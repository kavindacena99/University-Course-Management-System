CREATE TABLE auth_permissions (
    code VARCHAR(50) PRIMARY KEY,
    description TEXT NOT NULL DEFAULT ''
);

INSERT INTO auth_permissions (code, description) VALUES
    ('users:read', 'View user directory and user details'),
    ('users:manage', 'Approve, reject, suspend, and reinstate users'),
    ('roles:manage', 'Manage roles and role permission assignments'),
    ('sessions:read', 'View active sessions'),
    ('sessions:revoke', 'Revoke user sessions'),
    ('account:unlock', 'Unlock locked user accounts'),
    ('audit:read', 'View authentication and security audit events');

CREATE TABLE auth_role_permissions (
    role_code VARCHAR(20) NOT NULL
        REFERENCES auth_roles (code) ON DELETE CASCADE,
    permission_code VARCHAR(50) NOT NULL
        REFERENCES auth_permissions (code) ON DELETE CASCADE,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (role_code, permission_code)
);

CREATE INDEX idx_auth_role_permissions_permission
    ON auth_role_permissions (permission_code);

INSERT INTO auth_role_permissions (role_code, permission_code)
SELECT 'ADMIN', code FROM auth_permissions;

INSERT INTO auth_role_permissions (role_code, permission_code) VALUES
    ('STAFF', 'users:read'),
    ('STAFF', 'sessions:read'),
    ('STAFF', 'sessions:revoke'),
    ('STAFF', 'account:unlock'),
    ('LECTURER', 'users:read');

CREATE TABLE auth_audit_events (
    id BIGSERIAL PRIMARY KEY,
    actor_id UUID NULL
        REFERENCES auth_users (user_id) ON DELETE SET NULL,
    subject_id UUID NULL
        REFERENCES auth_users (user_id) ON DELETE SET NULL,
    event_type VARCHAR(100) NOT NULL,
    success BOOLEAN NOT NULL,
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_auth_audit_events_created
    ON auth_audit_events (created_at DESC, id DESC);

CREATE INDEX idx_auth_audit_events_actor
    ON auth_audit_events (actor_id, created_at DESC)
    WHERE actor_id IS NOT NULL;

CREATE INDEX idx_auth_audit_events_subject
    ON auth_audit_events (subject_id, created_at DESC)
    WHERE subject_id IS NOT NULL;
