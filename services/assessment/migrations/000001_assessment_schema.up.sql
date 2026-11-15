CREATE TABLE assessments (
    id UUID PRIMARY KEY,
    offering_id UUID NOT NULL,
    title VARCHAR(200) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    component_kind VARCHAR(30) NOT NULL,
    due_at TIMESTAMPTZ NOT NULL,
    max_mark NUMERIC(8, 2) NOT NULL,
    weight_percent NUMERIC(5, 2) NOT NULL,
    max_attempts INTEGER NOT NULL,
    late_policy VARCHAR(20) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    version BIGINT NOT NULL DEFAULT 1,
    created_by_user_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ,
    closed_at TIMESTAMPTZ,
    CONSTRAINT assessments_title_chk CHECK (length(btrim(title)) BETWEEN 1 AND 200),
    CONSTRAINT assessments_component_kind_chk CHECK (
        component_kind IN ('ca', 'final_exam', 'other')
    ),
    CONSTRAINT assessments_mark_chk CHECK (max_mark > 0),
    CONSTRAINT assessments_weight_chk CHECK (weight_percent > 0 AND weight_percent <= 100),
    CONSTRAINT assessments_attempts_chk CHECK (max_attempts BETWEEN 1 AND 100),
    CONSTRAINT assessments_late_policy_chk CHECK (late_policy IN ('reject', 'accept')),
    CONSTRAINT assessments_status_chk CHECK (status IN ('draft', 'published', 'closed')),
    CONSTRAINT assessments_version_chk CHECK (version > 0),
    CONSTRAINT assessments_published_chk CHECK (
        (status = 'draft' AND published_at IS NULL)
        OR (status <> 'draft' AND published_at IS NOT NULL)
    )
);

CREATE INDEX assessments_offering_status_due_idx
    ON assessments (offering_id, status, due_at, id);

CREATE TABLE assessment_allowed_file_types (
    assessment_id UUID NOT NULL REFERENCES assessments (id) ON DELETE CASCADE,
    extension VARCHAR(16) NOT NULL,
    media_type VARCHAR(127) NOT NULL,
    PRIMARY KEY (assessment_id, extension),
    CONSTRAINT assessment_file_extension_chk CHECK (extension ~ '^[a-z0-9]{1,15}$'),
    CONSTRAINT assessment_file_media_type_chk CHECK (position('/' IN media_type) > 1)
);

CREATE TABLE assessment_extensions (
    id UUID PRIMARY KEY,
    assessment_id UUID NOT NULL REFERENCES assessments (id) ON DELETE RESTRICT,
    student_user_id UUID NOT NULL,
    revised_due_at TIMESTAMPTZ NOT NULL,
    reason TEXT NOT NULL,
    authorized_by_user_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT assessment_extensions_reason_chk CHECK (length(btrim(reason)) BETWEEN 1 AND 2000)
);

CREATE INDEX assessment_extensions_student_assessment_idx
    ON assessment_extensions (assessment_id, student_user_id, revised_due_at DESC);

CREATE TABLE submissions (
    id UUID PRIMARY KEY,
    assessment_id UUID NOT NULL REFERENCES assessments (id) ON DELETE RESTRICT,
    student_user_id UUID NOT NULL,
    attempt_no INTEGER NOT NULL,
    idempotency_key VARCHAR(128) NOT NULL,
    object_key TEXT NOT NULL UNIQUE,
    original_filename VARCHAR(255) NOT NULL,
    media_type VARCHAR(127) NOT NULL,
    size_bytes BIGINT NOT NULL,
    content_sha256 CHAR(64) NOT NULL,
    status VARCHAR(20) NOT NULL,
    submitted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT submissions_attempt_chk CHECK (attempt_no > 0),
    CONSTRAINT submissions_size_chk CHECK (size_bytes > 0),
    CONSTRAINT submissions_status_chk CHECK (status IN ('submitted', 'late')),
    CONSTRAINT submissions_idempotency_uq UNIQUE (assessment_id, student_user_id, idempotency_key),
    CONSTRAINT submissions_attempt_uq UNIQUE (assessment_id, student_user_id, attempt_no)
);

CREATE INDEX submissions_assessment_student_idx
    ON submissions (assessment_id, student_user_id, attempt_no DESC);
CREATE INDEX submissions_student_created_idx
    ON submissions (student_user_id, submitted_at DESC, id);

CREATE TABLE grades (
    id UUID PRIMARY KEY,
    submission_id UUID NOT NULL UNIQUE REFERENCES submissions (id) ON DELETE RESTRICT,
    graded_by_user_id UUID NOT NULL,
    mark NUMERIC(8, 2) NOT NULL,
    feedback TEXT NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    version BIGINT NOT NULL DEFAULT 1,
    graded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    released_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT grades_mark_chk CHECK (mark >= 0),
    CONSTRAINT grades_status_chk CHECK (status IN ('draft', 'released')),
    CONSTRAINT grades_version_chk CHECK (version > 0),
    CONSTRAINT grades_release_chk CHECK (
        (status = 'draft' AND released_at IS NULL)
        OR (status = 'released' AND released_at IS NOT NULL)
    )
);

CREATE INDEX grades_released_idx ON grades (released_at DESC, id)
    WHERE status = 'released';

CREATE TABLE grade_revisions (
    id UUID PRIMARY KEY,
    grade_id UUID NOT NULL REFERENCES grades (id) ON DELETE RESTRICT,
    revision_no INTEGER NOT NULL,
    changed_by_user_id UUID NOT NULL,
    old_mark NUMERIC(8, 2),
    new_mark NUMERIC(8, 2) NOT NULL,
    old_feedback TEXT,
    new_feedback TEXT NOT NULL,
    changed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT grade_revisions_no_chk CHECK (revision_no > 0),
    CONSTRAINT grade_revisions_grade_revision_uq UNIQUE (grade_id, revision_no)
);

CREATE TABLE assessment_outbox_events (
    event_id UUID PRIMARY KEY,
    event_type VARCHAR(100) NOT NULL,
    schema_version INTEGER NOT NULL DEFAULT 1,
    aggregate_id UUID NOT NULL,
    aggregate_version BIGINT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    correlation_id VARCHAR(128) NOT NULL,
    payload JSONB NOT NULL,
    delivery_status VARCHAR(20) NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    delivered_at TIMESTAMPTZ,
    last_error TEXT NOT NULL DEFAULT '',
    CONSTRAINT assessment_outbox_schema_chk CHECK (schema_version > 0),
    CONSTRAINT assessment_outbox_version_chk CHECK (aggregate_version > 0),
    CONSTRAINT assessment_outbox_attempts_chk CHECK (attempts >= 0),
    CONSTRAINT assessment_outbox_status_chk CHECK (
        delivery_status IN ('pending', 'delivered', 'dead')
    )
);

CREATE INDEX assessment_outbox_pending_idx
    ON assessment_outbox_events (next_attempt_at, occurred_at)
    WHERE delivery_status = 'pending';
