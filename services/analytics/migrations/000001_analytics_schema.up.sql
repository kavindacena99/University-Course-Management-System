CREATE TABLE analytics_inbox_events (
    event_id UUID PRIMARY KEY,
    event_type VARCHAR(64) NOT NULL,
    payload JSONB NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX analytics_inbox_events_received_idx
    ON analytics_inbox_events (received_at, event_id);

CREATE TABLE component_grade_projection (
    grade_id UUID PRIMARY KEY,
    submission_id UUID NOT NULL,
    assessment_id UUID NOT NULL,
    offering_id UUID NOT NULL,
    student_user_id UUID NOT NULL,
    attempt_no INTEGER NOT NULL,
    component_kind VARCHAR(30) NOT NULL,
    mark NUMERIC(8, 2) NOT NULL,
    max_mark NUMERIC(8, 2) NOT NULL,
    weight_percent NUMERIC(5, 2) NOT NULL,
    aggregate_version BIGINT NOT NULL,
    released_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT analytics_component_attempt_chk CHECK (attempt_no > 0),
    CONSTRAINT analytics_component_mark_chk CHECK (
        mark >= 0 AND max_mark > 0 AND mark <= max_mark
    ),
    CONSTRAINT analytics_component_weight_chk CHECK (
        weight_percent > 0 AND weight_percent <= 100
    ),
    CONSTRAINT analytics_component_version_chk CHECK (aggregate_version > 0)
);

CREATE INDEX analytics_component_student_idx
    ON component_grade_projection (student_user_id, released_at DESC);
CREATE INDEX analytics_component_offering_idx
    ON component_grade_projection (offering_id, released_at DESC);

CREATE TABLE final_result_projection (
    result_id UUID PRIMARY KEY,
    offering_id UUID NOT NULL,
    student_user_id UUID NOT NULL,
    final_mark NUMERIC(8, 2) NOT NULL,
    max_mark NUMERIC(8, 2) NOT NULL,
    aggregate_version BIGINT NOT NULL,
    imported_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT analytics_final_mark_chk CHECK (
        final_mark >= 0 AND max_mark > 0 AND final_mark <= max_mark
    ),
    CONSTRAINT analytics_final_version_chk CHECK (aggregate_version > 0)
);

CREATE INDEX analytics_result_student_idx
    ON final_result_projection (student_user_id, imported_at DESC);
CREATE INDEX analytics_result_offering_idx
    ON final_result_projection (offering_id, imported_at DESC);

CREATE TABLE grade_bands (
    grade_code VARCHAR(12) PRIMARY KEY,
    minimum_percent NUMERIC(5, 2) NOT NULL,
    maximum_percent NUMERIC(5, 2) NOT NULL,
    grade_points NUMERIC(4, 2) NOT NULL,
    display_order INTEGER NOT NULL,
    CONSTRAINT grade_bands_min_chk CHECK (minimum_percent >= 0),
    CONSTRAINT grade_bands_max_chk CHECK (
        maximum_percent > minimum_percent AND maximum_percent <= 100
    ),
    CONSTRAINT grade_bands_points_chk CHECK (grade_points BETWEEN 0 AND 4),
    CONSTRAINT grade_bands_order_chk CHECK (display_order > 0)
);

CREATE TABLE programmes (
    id UUID PRIMARY KEY,
    code VARCHAR(32) NOT NULL UNIQUE,
    name VARCHAR(160) NOT NULL,
    category VARCHAR(100) NOT NULL,
    repeat_policy VARCHAR(16) NOT NULL,
    failed_result_policy VARCHAR(16) NOT NULL,
    pass_grade_points NUMERIC(4, 2) NOT NULL,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT programmes_code_chk CHECK (code = upper(code) AND length(btrim(code)) BETWEEN 1 AND 32),
    CONSTRAINT programmes_name_chk CHECK (length(btrim(name)) BETWEEN 1 AND 160),
    CONSTRAINT programmes_category_chk CHECK (length(btrim(category)) BETWEEN 1 AND 100),
    CONSTRAINT programmes_repeat_policy_chk CHECK (repeat_policy IN ('best', 'latest', 'all')),
    CONSTRAINT programmes_failed_policy_chk CHECK (failed_result_policy IN ('include', 'exclude')),
    CONSTRAINT programmes_pass_points_chk CHECK (pass_grade_points BETWEEN 0 AND 4)
);

CREATE INDEX programmes_category_idx ON programmes (category, name);

CREATE TABLE student_programme_assignments (
    student_user_id UUID PRIMARY KEY,
    programme_id UUID NOT NULL REFERENCES programmes (id) ON DELETE RESTRICT,
    assigned_by_user_id UUID NOT NULL,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE analytics_projection_state (
    singleton BOOLEAN PRIMARY KEY DEFAULT true CHECK (singleton),
    rebuilding BOOLEAN NOT NULL DEFAULT false,
    available BOOLEAN NOT NULL DEFAULT true,
    revision BIGINT NOT NULL DEFAULT 0 CHECK (revision >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO analytics_projection_state (singleton) VALUES (true);

CREATE TABLE internal_request_nonces (
    nonce VARCHAR(128) PRIMARY KEY,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX internal_request_nonces_expiry_idx
    ON internal_request_nonces (expires_at);
