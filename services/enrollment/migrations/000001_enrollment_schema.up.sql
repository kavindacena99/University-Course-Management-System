CREATE TABLE enrollment_offering_guards (
    offering_id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE enrollments (
    id UUID PRIMARY KEY,
    student_user_id UUID NOT NULL,
    offering_id UUID NOT NULL,
    status VARCHAR(20) NOT NULL,
    version BIGINT NOT NULL DEFAULT 1,
    enrolled_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ,
    created_by_user_id UUID NOT NULL,
    ended_by_user_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT enrollments_status_chk CHECK (status IN ('active', 'cancelled')),
    CONSTRAINT enrollments_version_chk CHECK (version > 0),
    CONSTRAINT enrollments_end_state_chk CHECK (
        (status = 'active' AND ended_at IS NULL AND ended_by_user_id IS NULL)
        OR (status = 'cancelled' AND ended_at IS NOT NULL AND ended_by_user_id IS NOT NULL)
    ),
    CONSTRAINT enrollments_end_time_chk CHECK (ended_at IS NULL OR ended_at >= enrolled_at)
);

CREATE UNIQUE INDEX enrollments_one_active_per_student_offering_idx
    ON enrollments (student_user_id, offering_id)
    WHERE status = 'active';
CREATE INDEX enrollments_student_created_idx
    ON enrollments (student_user_id, created_at DESC, id DESC);
CREATE INDEX enrollments_offering_created_idx
    ON enrollments (offering_id, created_at DESC, id DESC);
CREATE INDEX enrollments_offering_active_idx
    ON enrollments (offering_id)
    WHERE status = 'active';
