CREATE TABLE final_result_import_batches (
    id UUID PRIMARY KEY,
    offering_id UUID NOT NULL,
    imported_by_user_id UUID NOT NULL,
    idempotency_key VARCHAR(128) NOT NULL,
    content_sha256 CHAR(64) NOT NULL,
    row_count INTEGER NOT NULL,
    imported_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT final_result_import_row_count_chk CHECK (row_count BETWEEN 1 AND 1000),
    CONSTRAINT final_result_import_key_uq UNIQUE (offering_id, idempotency_key)
);

CREATE TABLE final_course_results (
    id UUID PRIMARY KEY,
    offering_id UUID NOT NULL,
    student_user_id UUID NOT NULL,
    final_mark NUMERIC(8, 2) NOT NULL,
    max_mark NUMERIC(8, 2) NOT NULL,
    version BIGINT NOT NULL DEFAULT 1,
    import_batch_id UUID NOT NULL
        REFERENCES final_result_import_batches (id) ON DELETE RESTRICT,
    imported_by_user_id UUID NOT NULL,
    imported_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT final_course_result_mark_chk CHECK (final_mark >= 0),
    CONSTRAINT final_course_result_max_mark_chk CHECK (max_mark > 0),
    CONSTRAINT final_course_result_mark_max_chk CHECK (final_mark <= max_mark),
    CONSTRAINT final_course_result_version_chk CHECK (version > 0),
    CONSTRAINT final_course_result_student_uq UNIQUE (offering_id, student_user_id)
);

CREATE INDEX final_course_results_student_idx
    ON final_course_results (student_user_id, imported_at DESC, id);

CREATE TABLE final_result_revisions (
    id UUID PRIMARY KEY,
    final_result_id UUID NOT NULL
        REFERENCES final_course_results (id) ON DELETE RESTRICT,
    revision_no INTEGER NOT NULL,
    changed_by_user_id UUID NOT NULL,
    old_final_mark NUMERIC(8, 2) NOT NULL,
    old_max_mark NUMERIC(8, 2) NOT NULL,
    new_final_mark NUMERIC(8, 2) NOT NULL,
    new_max_mark NUMERIC(8, 2) NOT NULL,
    import_batch_id UUID NOT NULL
        REFERENCES final_result_import_batches (id) ON DELETE RESTRICT,
    changed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT final_result_revision_no_chk CHECK (revision_no > 0),
    CONSTRAINT final_result_revision_uq UNIQUE (final_result_id, revision_no)
);
