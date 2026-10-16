CREATE TABLE users (
    id UUID PRIMARY KEY,
    email VARCHAR(254) NOT NULL,
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    registration_status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_users_email UNIQUE (email),

    CONSTRAINT chk_users_registration_status
        CHECK (
            registration_status IN (
                'pending',
                'approved',
                'rejected',
                'suspended'
            )
        )
);

CREATE INDEX idx_users_registration_status
    ON users (registration_status);
