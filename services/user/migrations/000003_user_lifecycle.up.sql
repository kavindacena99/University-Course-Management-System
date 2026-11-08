ALTER TABLE users
    ADD COLUMN status_reason TEXT NOT NULL DEFAULT '',
    ADD COLUMN reviewed_by UUID NULL REFERENCES users (id) ON DELETE SET NULL,
    ADD COLUMN reviewed_at TIMESTAMPTZ NULL,
    ADD COLUMN version BIGINT NOT NULL DEFAULT 1;

ALTER TABLE users
    ADD CONSTRAINT chk_users_version_positive CHECK (version >= 1);

CREATE TABLE user_status_history (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    from_status VARCHAR(20) NULL,
    to_status VARCHAR(20) NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    changed_by UUID NULL REFERENCES users (id) ON DELETE SET NULL,
    version BIGINT NOT NULL,
    changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_user_status_history_from
        CHECK (
            from_status IS NULL OR from_status IN (
                'pending',
                'approved',
                'rejected',
                'suspended'
            )
        ),

    CONSTRAINT chk_user_status_history_to
        CHECK (
            to_status IN (
                'pending',
                'approved',
                'rejected',
                'suspended'
            )
        )
);

CREATE INDEX idx_user_status_history_user_changed
    ON user_status_history (user_id, changed_at DESC, id DESC);
