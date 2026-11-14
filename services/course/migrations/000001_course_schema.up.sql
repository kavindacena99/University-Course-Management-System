CREATE TABLE courses (
    id UUID PRIMARY KEY,
    code VARCHAR(32) NOT NULL,
    title VARCHAR(200) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    credits INTEGER NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    version BIGINT NOT NULL DEFAULT 1,
    created_by UUID NOT NULL,
    updated_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    archived_at TIMESTAMPTZ,
    CONSTRAINT courses_code_uq UNIQUE (code),
    CONSTRAINT courses_code_format_chk CHECK (
        code = upper(code)
        AND code ~ '^[A-Z0-9_-]{2,32}$'
    ),
    CONSTRAINT courses_title_chk CHECK (length(btrim(title)) BETWEEN 1 AND 200),
    CONSTRAINT courses_credits_chk CHECK (credits BETWEEN 1 AND 30),
    CONSTRAINT courses_status_chk CHECK (status IN ('draft', 'active', 'archived')),
    CONSTRAINT courses_version_chk CHECK (version > 0),
    CONSTRAINT courses_archive_state_chk CHECK (
        (status = 'archived' AND archived_at IS NOT NULL)
        OR (status <> 'archived' AND archived_at IS NULL)
    )
);

CREATE INDEX courses_status_code_idx ON courses (status, code);
CREATE INDEX courses_created_at_idx ON courses (created_at DESC);

CREATE TABLE course_offerings (
    id UUID PRIMARY KEY,
    course_id UUID NOT NULL REFERENCES courses (id) ON DELETE RESTRICT,
    academic_year VARCHAR(20) NOT NULL,
    semester VARCHAR(20) NOT NULL,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    enrollment_opens_at TIMESTAMPTZ,
    enrollment_closes_at TIMESTAMPTZ,
    enrollment_capacity INTEGER,
    lecturer_user_id UUID,
    lecturer_assigned_by UUID,
    lecturer_assigned_at TIMESTAMPTZ,
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    version BIGINT NOT NULL DEFAULT 1,
    created_by UUID NOT NULL,
    updated_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    archived_at TIMESTAMPTZ,
    CONSTRAINT course_offerings_academic_year_chk CHECK (
        length(btrim(academic_year)) BETWEEN 1 AND 20
    ),
    CONSTRAINT course_offerings_semester_chk CHECK (
        length(btrim(semester)) BETWEEN 1 AND 20
    ),
    CONSTRAINT course_offerings_dates_chk CHECK (end_date >= start_date),
    CONSTRAINT course_offerings_enrollment_window_chk CHECK (
        enrollment_opens_at IS NULL
        OR enrollment_closes_at IS NULL
        OR enrollment_closes_at > enrollment_opens_at
    ),
    CONSTRAINT course_offerings_capacity_chk CHECK (
        enrollment_capacity IS NULL
        OR enrollment_capacity BETWEEN 1 AND 100000
    ),
    CONSTRAINT course_offerings_lecturer_assignment_chk CHECK (
        (lecturer_user_id IS NULL
            AND lecturer_assigned_by IS NULL
            AND lecturer_assigned_at IS NULL)
        OR (lecturer_user_id IS NOT NULL
            AND lecturer_assigned_by IS NOT NULL
            AND lecturer_assigned_at IS NOT NULL)
    ),
    CONSTRAINT course_offerings_status_chk CHECK (
        status IN ('draft', 'published', 'closed', 'archived')
    ),
    CONSTRAINT course_offerings_version_chk CHECK (version > 0),
    CONSTRAINT course_offerings_archive_state_chk CHECK (
        (status = 'archived' AND archived_at IS NOT NULL)
        OR (status <> 'archived' AND archived_at IS NULL)
    )
);

CREATE INDEX course_offerings_course_idx ON course_offerings (course_id, created_at DESC);
CREATE INDEX course_offerings_lecturer_idx ON course_offerings (lecturer_user_id, status);
CREATE INDEX course_offerings_status_start_idx ON course_offerings (status, start_date);
CREATE INDEX course_offerings_period_idx ON course_offerings (academic_year, semester);

CREATE TABLE course_audit_events (
    id BIGSERIAL PRIMARY KEY,
    course_id UUID NOT NULL REFERENCES courses (id) ON DELETE RESTRICT,
    offering_id UUID REFERENCES course_offerings (id) ON DELETE RESTRICT,
    actor_id UUID NOT NULL,
    event VARCHAR(64) NOT NULL,
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX course_audit_events_course_idx
    ON course_audit_events (course_id, created_at DESC);
CREATE INDEX course_audit_events_offering_idx
    ON course_audit_events (offering_id, created_at DESC)
    WHERE offering_id IS NOT NULL;
