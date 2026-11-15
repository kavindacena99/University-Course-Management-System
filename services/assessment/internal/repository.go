package assessment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

var (
	_ AssessmentStore = (*Repository)(nil)
	_ OutboxStore     = (*Repository)(nil)
)

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

const assessmentColumns = `
	id, offering_id, title, description, component_kind, due_at, max_mark,
	weight_percent, max_attempts, late_policy, status, version,
	created_by_user_id, created_at, updated_at, published_at, closed_at
`

const submissionColumns = `
	id, assessment_id, student_user_id, attempt_no, object_key,
	original_filename, media_type, size_bytes, content_sha256, status, submitted_at
`

const gradeColumns = `
	id, submission_id, graded_by_user_id, mark, feedback, status,
	version, graded_at, released_at, updated_at
`

func scanAssessment(row pgx.Row) (*Assessment, error) {
	var item Assessment
	err := row.Scan(
		&item.ID, &item.OfferingID, &item.Title, &item.Description,
		&item.ComponentKind, &item.DueAt, &item.MaxMark, &item.WeightPercent,
		&item.MaxAttempts, &item.LatePolicy, &item.Status, &item.Version,
		&item.CreatedByUserID, &item.CreatedAt, &item.UpdatedAt,
		&item.PublishedAt, &item.ClosedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan assessment: %w", err)
	}
	return &item, nil
}

func scanSubmission(row pgx.Row) (*Submission, error) {
	var item Submission
	err := row.Scan(
		&item.ID, &item.AssessmentID, &item.StudentUserID, &item.AttemptNo,
		&item.ObjectKey, &item.OriginalFilename, &item.MediaType,
		&item.SizeBytes, &item.ContentSHA256, &item.Status, &item.SubmittedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan submission: %w", err)
	}
	return &item, nil
}

func scanGrade(row pgx.Row) (*Grade, error) {
	var item Grade
	err := row.Scan(
		&item.ID, &item.SubmissionID, &item.GradedBy, &item.Mark, &item.Feedback,
		&item.Status, &item.Version, &item.GradedAt, &item.ReleasedAt, &item.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan grade: %w", err)
	}
	return &item, nil
}

func (r *Repository) CreateAssessment(ctx context.Context, item *Assessment) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin assessment creation: %w", err)
	}
	defer tx.Rollback(ctx)

	saved, err := scanAssessment(tx.QueryRow(ctx,
		`INSERT INTO assessments (
			id, offering_id, title, description, component_kind, due_at,
			max_mark, weight_percent, max_attempts, late_policy, status,
			version, created_by_user_id
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'draft',1,$11)
		RETURNING `+assessmentColumns,
		item.ID, item.OfferingID, item.Title, item.Description, item.ComponentKind,
		item.DueAt, item.MaxMark, item.WeightPercent, item.MaxAttempts,
		item.LatePolicy, item.CreatedByUserID,
	))
	if err != nil {
		return err
	}
	for _, fileType := range item.AllowedFileTypes {
		if _, err := tx.Exec(ctx,
			`INSERT INTO assessment_allowed_file_types (assessment_id, extension, media_type)
			 VALUES ($1,$2,$3)`,
			item.ID, fileType.Extension, fileType.MediaType,
		); err != nil {
			return fmt.Errorf("insert allowed file type: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit assessment creation: %w", err)
	}
	fileTypes := append([]AllowedFileType(nil), item.AllowedFileTypes...)
	*item = *saved
	item.AllowedFileTypes = fileTypes
	return nil
}

func (r *Repository) GetAssessment(ctx context.Context, id uuid.UUID) (*Assessment, error) {
	item, err := scanAssessment(r.db.QueryRow(ctx,
		`SELECT `+assessmentColumns+` FROM assessments WHERE id = $1`, id))
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Query(ctx,
		`SELECT extension, media_type
		   FROM assessment_allowed_file_types
		  WHERE assessment_id = $1
		  ORDER BY extension`, id)
	if err != nil {
		return nil, fmt.Errorf("read allowed file types: %w", err)
	}
	defer rows.Close()
	item.AllowedFileTypes = make([]AllowedFileType, 0)
	for rows.Next() {
		var fileType AllowedFileType
		if err := rows.Scan(&fileType.Extension, &fileType.MediaType); err != nil {
			return nil, fmt.Errorf("scan allowed file type: %w", err)
		}
		item.AllowedFileTypes = append(item.AllowedFileTypes, fileType)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate allowed file types: %w", err)
	}
	return item, nil
}

func (r *Repository) ListAssessments(
	ctx context.Context,
	filter AssessmentFilter,
) (*AssessmentList, error) {
	result := &AssessmentList{Limit: filter.Limit, Offset: filter.Offset}
	var status any
	if filter.Status != "" {
		status = filter.Status
	}
	if err := r.db.QueryRow(ctx,
		`SELECT count(*) FROM assessments
		  WHERE offering_id = $1 AND ($2::text IS NULL OR status = $2)`,
		filter.OfferingID, status,
	).Scan(&result.Total); err != nil {
		return nil, fmt.Errorf("count assessments: %w", err)
	}
	rows, err := r.db.Query(ctx,
		`SELECT `+assessmentColumns+`
		   FROM assessments
		  WHERE offering_id = $1 AND ($2::text IS NULL OR status = $2)
		  ORDER BY due_at, id
		  LIMIT $3 OFFSET $4`,
		filter.OfferingID, status, filter.Limit, filter.Offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list assessments: %w", err)
	}
	defer rows.Close()
	result.Assessments = make([]Assessment, 0)
	for rows.Next() {
		item, err := scanAssessment(rows)
		if err != nil {
			return nil, err
		}
		result.Assessments = append(result.Assessments, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate assessments: %w", err)
	}
	for i := range result.Assessments {
		if err := r.loadAllowedFileTypes(ctx, &result.Assessments[i]); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (r *Repository) loadAllowedFileTypes(ctx context.Context, item *Assessment) error {
	rows, err := r.db.Query(ctx,
		`SELECT extension, media_type
		   FROM assessment_allowed_file_types
		  WHERE assessment_id = $1 ORDER BY extension`, item.ID)
	if err != nil {
		return fmt.Errorf("read allowed file types: %w", err)
	}
	defer rows.Close()
	item.AllowedFileTypes = make([]AllowedFileType, 0)
	for rows.Next() {
		var fileType AllowedFileType
		if err := rows.Scan(&fileType.Extension, &fileType.MediaType); err != nil {
			return fmt.Errorf("scan allowed file type: %w", err)
		}
		item.AllowedFileTypes = append(item.AllowedFileTypes, fileType)
	}
	return rows.Err()
}

func (r *Repository) ChangeAssessmentStatus(
	ctx context.Context,
	id uuid.UUID,
	expectedVersion int64,
	next AssessmentStatus,
) (*Assessment, error) {
	var publishedAt, closedAt any
	switch next {
	case AssessmentPublished:
		publishedAt = time.Now().UTC()
	case AssessmentClosed:
		closedAt = time.Now().UTC()
	default:
		return nil, ErrInvalidTransition
	}
	item, err := scanAssessment(r.db.QueryRow(ctx,
		`UPDATE assessments
		    SET status = $3, version = version + 1, updated_at = now(),
		        published_at = COALESCE(published_at, $4),
		        closed_at = COALESCE($5, closed_at)
		  WHERE id = $1 AND version = $2
		RETURNING `+assessmentColumns,
		id, expectedVersion, next, publishedAt, closedAt,
	))
	if errors.Is(err, ErrNotFound) {
		var exists bool
		if queryErr := r.db.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM assessments WHERE id = $1)`, id,
		).Scan(&exists); queryErr != nil {
			return nil, fmt.Errorf("check assessment version: %w", queryErr)
		}
		if exists {
			return nil, ErrVersionConflict
		}
	}
	if err != nil {
		return nil, err
	}
	if err := r.loadAllowedFileTypes(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (r *Repository) CreateExtension(
	ctx context.Context,
	assessmentID, studentID uuid.UUID,
	revisedDueAt time.Time,
	reason string,
	actorID uuid.UUID,
) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO assessment_extensions (
			id, assessment_id, student_user_id, revised_due_at, reason, authorized_by_user_id
		) VALUES ($1,$2,$3,$4,$5,$6)`,
		uuid.New(), assessmentID, studentID, revisedDueAt, reason, actorID,
	)
	if err != nil {
		return fmt.Errorf("create assessment extension: %w", err)
	}
	return nil
}

func (r *Repository) GetExtensionDeadline(
	ctx context.Context,
	assessmentID, studentID uuid.UUID,
) (*time.Time, error) {
	var result *time.Time
	if err := r.db.QueryRow(ctx,
		`SELECT max(revised_due_at)
		   FROM assessment_extensions
		  WHERE assessment_id = $1 AND student_user_id = $2`,
		assessmentID, studentID,
	).Scan(&result); err != nil {
		return nil, fmt.Errorf("read assessment extension: %w", err)
	}
	return result, nil
}

func (r *Repository) CreateSubmission(
	ctx context.Context,
	item *Submission,
	maxAttempts int,
) (*Submission, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin submission: %w", err)
	}
	defer tx.Rollback(ctx)

	var status AssessmentStatus
	if err := tx.QueryRow(ctx,
		`SELECT status FROM assessments WHERE id = $1 FOR UPDATE`,
		item.AssessmentID,
	).Scan(&status); errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, fmt.Errorf("lock assessment for submission: %w", err)
	}
	if status != AssessmentPublished {
		return nil, ErrSubmissionClosed
	}

	existing, err := scanSubmission(tx.QueryRow(ctx,
		`SELECT `+submissionColumns+`
		   FROM submissions
		  WHERE assessment_id = $1 AND student_user_id = $2 AND idempotency_key = $3`,
		item.AssessmentID, item.StudentUserID, item.IdempotencyKey,
	))
	if err == nil {
		if existing.ContentSHA256 != item.ContentSHA256 {
			return nil, ErrIdempotencyConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit idempotent submission lookup: %w", err)
		}
		return existing, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	var attempts int
	if err := tx.QueryRow(ctx,
		`SELECT count(*) FROM submissions
		  WHERE assessment_id = $1 AND student_user_id = $2`,
		item.AssessmentID, item.StudentUserID,
	).Scan(&attempts); err != nil {
		return nil, fmt.Errorf("count submission attempts: %w", err)
	}
	if attempts >= maxAttempts {
		return nil, ErrAttemptLimitReached
	}
	item.AttemptNo = attempts + 1
	saved, err := scanSubmission(tx.QueryRow(ctx,
		`INSERT INTO submissions (
			id, assessment_id, student_user_id, attempt_no, idempotency_key,
			object_key, original_filename, media_type, size_bytes,
			content_sha256, status, submitted_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING `+submissionColumns,
		item.ID, item.AssessmentID, item.StudentUserID, item.AttemptNo,
		item.IdempotencyKey, item.ObjectKey, item.OriginalFilename, item.MediaType,
		item.SizeBytes, item.ContentSHA256, item.Status, item.SubmittedAt,
	))
	if err != nil {
		return nil, fmt.Errorf("insert submission: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit submission: %w", err)
	}
	return saved, nil
}

func (r *Repository) GetSubmission(ctx context.Context, id uuid.UUID) (*Submission, error) {
	return scanSubmission(r.db.QueryRow(ctx,
		`SELECT `+submissionColumns+` FROM submissions WHERE id = $1`, id))
}

func (r *Repository) ListSubmissions(
	ctx context.Context,
	assessmentID uuid.UUID,
	limit, offset int,
) (*SubmissionList, error) {
	result := &SubmissionList{Limit: limit, Offset: offset}
	if err := r.db.QueryRow(ctx,
		`SELECT count(*) FROM submissions WHERE assessment_id = $1`, assessmentID,
	).Scan(&result.Total); err != nil {
		return nil, fmt.Errorf("count submissions: %w", err)
	}
	rows, err := r.db.Query(ctx,
		`SELECT `+submissionColumns+`
		   FROM submissions
		  WHERE assessment_id = $1
		  ORDER BY student_user_id, attempt_no, id
		  LIMIT $2 OFFSET $3`,
		assessmentID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list submissions: %w", err)
	}
	defer rows.Close()
	result.Submissions = make([]Submission, 0)
	for rows.Next() {
		item, err := scanSubmission(rows)
		if err != nil {
			return nil, err
		}
		result.Submissions = append(result.Submissions, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate submissions: %w", err)
	}
	return result, nil
}

func (r *Repository) ImportFinalResults(
	ctx context.Context,
	offeringID, actorID uuid.UUID,
	idempotencyKey, contentSHA256 string,
	results []FinalResultInput,
	correlationID string,
) (*FinalResultImportBatch, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin final result import: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`,
		offeringID.String(),
	); err != nil {
		return nil, fmt.Errorf("lock final result import for offering: %w", err)
	}
	batch := &FinalResultImportBatch{ID: uuid.New(), OfferingID: offeringID,
		ImportedBy: actorID, RowCount: len(results)}
	err = tx.QueryRow(ctx,
		`INSERT INTO final_result_import_batches (
			id, offering_id, imported_by_user_id, idempotency_key, content_sha256, row_count
		) VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (offering_id, idempotency_key) DO NOTHING
		RETURNING imported_at`,
		batch.ID, offeringID, actorID, idempotencyKey, contentSHA256, len(results),
	).Scan(&batch.ImportedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		var existingSHA256 string
		if err := tx.QueryRow(ctx,
			`SELECT id, imported_by_user_id, content_sha256, row_count, imported_at
			   FROM final_result_import_batches
			  WHERE offering_id = $1 AND idempotency_key = $2`,
			offeringID, idempotencyKey,
		).Scan(&batch.ID, &batch.ImportedBy, &existingSHA256, &batch.RowCount, &batch.ImportedAt); err != nil {
			return nil, fmt.Errorf("read prior final result import: %w", err)
		}
		if existingSHA256 != contentSHA256 {
			return nil, ErrIdempotencyConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit idempotent final result import: %w", err)
		}
		batch.AlreadyProcessed = true
		return batch, nil
	}
	if err != nil {
		return nil, fmt.Errorf("create final result import batch: %w", err)
	}

	for _, input := range results {
		var current FinalResult
		err := tx.QueryRow(ctx,
			`SELECT id, version, final_mark, max_mark
			   FROM final_course_results
			  WHERE offering_id = $1 AND student_user_id = $2
			  FOR UPDATE`,
			offeringID, input.StudentUserID,
		).Scan(&current.ID, &current.Version, &current.FinalMark, &current.MaxMark)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			current.ID = uuid.New()
			current.Version = 1
			_, err = tx.Exec(ctx,
				`INSERT INTO final_course_results (
					id, offering_id, student_user_id, final_mark, max_mark, version,
					import_batch_id, imported_by_user_id
				) VALUES ($1,$2,$3,$4,$5,1,$6,$7)`,
				current.ID, offeringID, input.StudentUserID, input.FinalMark,
				input.MaxMark, batch.ID, actorID,
			)
			if err != nil {
				return nil, fmt.Errorf("insert final course result: %w", err)
			}
		case err != nil:
			return nil, fmt.Errorf("read existing final course result: %w", err)
		default:
			if current.FinalMark == input.FinalMark && current.MaxMark == input.MaxMark {
				continue
			}
			var revisionNo int
			if err := tx.QueryRow(ctx,
				`SELECT COALESCE(max(revision_no), 0) + 1
				   FROM final_result_revisions
				  WHERE final_result_id = $1`,
				current.ID,
			).Scan(&revisionNo); err != nil {
				return nil, fmt.Errorf("allocate final result revision: %w", err)
			}
			if _, err := tx.Exec(ctx,
				`INSERT INTO final_result_revisions (
					id, final_result_id, revision_no, changed_by_user_id,
					old_final_mark, old_max_mark, new_final_mark, new_max_mark, import_batch_id
				) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
				uuid.New(), current.ID, revisionNo, actorID, current.FinalMark,
				current.MaxMark, input.FinalMark, input.MaxMark, batch.ID,
			); err != nil {
				return nil, fmt.Errorf("record final result revision: %w", err)
			}
			current.Version++
			if _, err := tx.Exec(ctx,
				`UPDATE final_course_results
				    SET final_mark = $2, max_mark = $3, version = $4,
				        import_batch_id = $5, imported_by_user_id = $6, imported_at = now()
				  WHERE id = $1`,
				current.ID, input.FinalMark, input.MaxMark, current.Version, batch.ID, actorID,
			); err != nil {
				return nil, fmt.Errorf("update final course result: %w", err)
			}
		}
		if err != nil {
			return nil, fmt.Errorf("persist final course result: %w", err)
		}
		var importedAt time.Time
		if err := tx.QueryRow(ctx,
			`SELECT imported_at FROM final_course_results WHERE id = $1`, current.ID,
		).Scan(&importedAt); err != nil {
			return nil, fmt.Errorf("read final result import time: %w", err)
		}
		eventID := uuid.New()
		eventPayload := struct {
			EventID          uuid.UUID `json:"event_id"`
			EventType        string    `json:"event_type"`
			SchemaVersion    int       `json:"schema_version"`
			OccurredAt       time.Time `json:"occurred_at"`
			Producer         string    `json:"producer_service"`
			AggregateID      uuid.UUID `json:"aggregate_id"`
			AggregateVersion int64     `json:"aggregate_version"`
			CorrelationID    string    `json:"correlation_id"`
			ResultID         uuid.UUID `json:"result_id"`
			OfferingID       uuid.UUID `json:"offering_id"`
			StudentUserID    uuid.UUID `json:"student_user_id"`
			FinalMark        float64   `json:"final_mark"`
			MaxMark          float64   `json:"max_mark"`
			ImportBatchID    uuid.UUID `json:"import_batch_id"`
			ImportedBy       uuid.UUID `json:"imported_by_user_id"`
			ImportedAt       time.Time `json:"imported_at"`
		}{
			EventID: eventID, EventType: "FinalResultImported", SchemaVersion: 1,
			OccurredAt: importedAt, Producer: "assessment-service",
			AggregateID: current.ID, AggregateVersion: current.Version,
			CorrelationID: correlationID, ResultID: current.ID,
			OfferingID: offeringID, StudentUserID: input.StudentUserID,
			FinalMark: input.FinalMark, MaxMark: input.MaxMark,
			ImportBatchID: batch.ID, ImportedBy: actorID, ImportedAt: importedAt,
		}
		payload, err := json.Marshal(eventPayload)
		if err != nil {
			return nil, fmt.Errorf("encode final result event: %w", err)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO assessment_outbox_events (
				event_id, event_type, schema_version, aggregate_id, aggregate_version,
				occurred_at, correlation_id, payload
			) VALUES ($1,'FinalResultImported',1,$2,$3,$4,$5,$6)`,
			eventID, current.ID, current.Version, importedAt, correlationID, payload,
		); err != nil {
			return nil, fmt.Errorf("write final result event to outbox: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit final result import: %w", err)
	}
	return batch, nil
}

func (r *Repository) ListFinalResults(
	ctx context.Context,
	offeringID, studentID uuid.UUID,
	limit, offset int,
) (*FinalResultList, error) {
	result := &FinalResultList{Limit: limit, Offset: offset}
	var studentFilter any
	if studentID != uuid.Nil {
		studentFilter = studentID
	}
	if err := r.db.QueryRow(ctx,
		`SELECT count(*) FROM final_course_results
		  WHERE offering_id = $1 AND ($2::uuid IS NULL OR student_user_id = $2)`,
		offeringID, studentFilter,
	).Scan(&result.Total); err != nil {
		return nil, fmt.Errorf("count final course results: %w", err)
	}
	rows, err := r.db.Query(ctx,
		`SELECT id, offering_id, student_user_id, final_mark, max_mark,
		        version, import_batch_id, imported_by_user_id, imported_at
		   FROM final_course_results
		  WHERE offering_id = $1 AND ($2::uuid IS NULL OR student_user_id = $2)
		  ORDER BY student_user_id, id
		  LIMIT $3 OFFSET $4`,
		offeringID, studentFilter, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list final course results: %w", err)
	}
	defer rows.Close()
	result.Results = make([]FinalResult, 0)
	for rows.Next() {
		var item FinalResult
		if err := rows.Scan(
			&item.ID, &item.OfferingID, &item.StudentUserID, &item.FinalMark,
			&item.MaxMark, &item.Version, &item.ImportBatchID, &item.ImportedBy,
			&item.ImportedAt,
		); err != nil {
			return nil, fmt.Errorf("scan final course result: %w", err)
		}
		result.Results = append(result.Results, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate final course results: %w", err)
	}
	return result, nil
}

func (r *Repository) GetGrade(ctx context.Context, submissionID uuid.UUID) (*Grade, error) {
	return scanGrade(r.db.QueryRow(ctx,
		`SELECT `+gradeColumns+` FROM grades WHERE submission_id = $1`, submissionID))
}

func (r *Repository) SaveGrade(
	ctx context.Context,
	item *Grade,
	expectedVersion int64,
) (*Grade, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin grade save: %w", err)
	}
	defer tx.Rollback(ctx)

	var maxMark float64
	if err := tx.QueryRow(ctx,
		`SELECT a.max_mark
		   FROM submissions s
		   JOIN assessments a ON a.id = s.assessment_id
		  WHERE s.id = $1 FOR UPDATE OF s`,
		item.SubmissionID,
	).Scan(&maxMark); errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, fmt.Errorf("read submission maximum mark: %w", err)
	}
	if item.Mark > maxMark {
		return nil, newValidationError("mark", "must not exceed the assessment maximum")
	}

	existing, err := scanGrade(tx.QueryRow(ctx,
		`SELECT `+gradeColumns+` FROM grades WHERE submission_id = $1 FOR UPDATE`,
		item.SubmissionID,
	))
	if errors.Is(err, ErrNotFound) {
		if expectedVersion != 0 {
			return nil, ErrVersionConflict
		}
		saved, err := scanGrade(tx.QueryRow(ctx,
			`INSERT INTO grades (
				id, submission_id, graded_by_user_id, mark, feedback, status, version
			) VALUES ($1,$2,$3,$4,$5,'draft',1)
			RETURNING `+gradeColumns,
			item.ID, item.SubmissionID, item.GradedBy, item.Mark, item.Feedback,
		))
		if err != nil {
			return nil, fmt.Errorf("insert grade: %w", mapDatabaseConflict(err))
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit new grade: %w", err)
		}
		return saved, nil
	}
	if err != nil {
		return nil, err
	}
	if expectedVersion == 0 || existing.Version != expectedVersion {
		return nil, ErrVersionConflict
	}
	if existing.Status == GradeReleased {
		var revisionNo int
		if err := tx.QueryRow(ctx,
			`SELECT COALESCE(max(revision_no), 0) + 1
			   FROM grade_revisions WHERE grade_id = $1`, existing.ID,
		).Scan(&revisionNo); err != nil {
			return nil, fmt.Errorf("allocate grade revision: %w", err)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO grade_revisions (
				id, grade_id, revision_no, changed_by_user_id,
				old_mark, new_mark, old_feedback, new_feedback
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
			uuid.New(), existing.ID, revisionNo, item.GradedBy,
			existing.Mark, item.Mark, existing.Feedback, item.Feedback,
		); err != nil {
			return nil, fmt.Errorf("record grade revision: %w", err)
		}
	}
	saved, err := scanGrade(tx.QueryRow(ctx,
		`UPDATE grades
		    SET graded_by_user_id = $2, mark = $3, feedback = $4,
		        status = 'draft', released_at = NULL,
		        version = version + 1, updated_at = now()
		  WHERE id = $1 AND version = $5
		RETURNING `+gradeColumns,
		existing.ID, item.GradedBy, item.Mark, item.Feedback, expectedVersion,
	))
	if err != nil {
		return nil, fmt.Errorf("update grade: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit grade update: %w", err)
	}
	return saved, nil
}

func (r *Repository) ReleaseGrade(
	ctx context.Context,
	submissionID uuid.UUID,
	expectedVersion int64,
	actorID uuid.UUID,
	correlationID string,
) (*Grade, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin grade release: %w", err)
	}
	defer tx.Rollback(ctx)

	grade, err := scanGrade(tx.QueryRow(ctx,
		`SELECT `+gradeColumns+` FROM grades WHERE submission_id = $1 FOR UPDATE`,
		submissionID,
	))
	if err != nil {
		return nil, err
	}
	if grade.Version != expectedVersion {
		return nil, ErrVersionConflict
	}
	if grade.Status != GradeDraft {
		return nil, ErrInvalidTransition
	}
	saved, err := scanGrade(tx.QueryRow(ctx,
		`UPDATE grades
		    SET status = 'released', released_at = now(),
		        version = version + 1, updated_at = now()
		  WHERE id = $1
		RETURNING `+gradeColumns,
		grade.ID,
	))
	if err != nil {
		return nil, fmt.Errorf("release grade: %w", err)
	}
	var offeringID, studentID, assessmentID uuid.UUID
	var componentKind string
	var attemptNo int
	var maxMark, weightPercent float64
	if err := tx.QueryRow(ctx,
		`SELECT a.offering_id, s.student_user_id, a.id, s.attempt_no, a.component_kind,
		        a.max_mark, a.weight_percent
		   FROM submissions s
		   JOIN assessments a ON a.id = s.assessment_id
		  WHERE s.id = $1`,
		submissionID,
	).Scan(&offeringID, &studentID, &assessmentID, &attemptNo, &componentKind,
		&maxMark, &weightPercent); err != nil {
		return nil, fmt.Errorf("read released grade context: %w", err)
	}
	eventID := uuid.New()
	eventPayload := struct {
		EventID          uuid.UUID `json:"event_id"`
		EventType        string    `json:"event_type"`
		SchemaVersion    int       `json:"schema_version"`
		OccurredAt       time.Time `json:"occurred_at"`
		Producer          string    `json:"producer_service"`
		AggregateID      uuid.UUID `json:"aggregate_id"`
		Version          int64     `json:"aggregate_version"`
		CorrelationID    string    `json:"correlation_id"`
		GradeID          uuid.UUID `json:"grade_id"`
		SubmissionID     uuid.UUID `json:"submission_id"`
		StudentID        uuid.UUID `json:"student_user_id"`
		AssessmentID     uuid.UUID `json:"assessment_id"`
		OfferingID       uuid.UUID `json:"offering_id"`
		AttemptNo        int       `json:"attempt_no"`
		ComponentKind    string    `json:"component_kind"`
		Mark             float64   `json:"mark"`
		MaxMark          float64   `json:"max_mark"`
		WeightPercent    float64   `json:"weight_percent"`
		ReleasedByUserID uuid.UUID `json:"released_by_user_id"`
		ReleasedAt       time.Time `json:"released_at"`
	}{
		EventID: eventID, EventType: "GradePublished", SchemaVersion: 1,
		OccurredAt: *saved.ReleasedAt, Producer: "assessment-service",
		AggregateID: saved.ID, Version: saved.Version, CorrelationID: correlationID,
		GradeID: saved.ID, SubmissionID: submissionID, StudentID: studentID,
		AssessmentID: assessmentID, OfferingID: offeringID, AttemptNo: attemptNo,
		ComponentKind: componentKind,
		Mark: saved.Mark, MaxMark: maxMark, WeightPercent: weightPercent,
		ReleasedByUserID: actorID, ReleasedAt: *saved.ReleasedAt,
	}
	payload, err := json.Marshal(eventPayload)
	if err != nil {
		return nil, fmt.Errorf("encode grade event: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO assessment_outbox_events (
			event_id, event_type, schema_version, aggregate_id, aggregate_version,
			occurred_at, correlation_id, payload
		) VALUES ($1,'GradePublished',1,$2,$3,$4,$5,$6)`,
		eventID, saved.ID, saved.Version, *saved.ReleasedAt, correlationID, payload,
	); err != nil {
		return nil, fmt.Errorf("write grade event to outbox: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit grade release: %w", err)
	}
	return saved, nil
}

func (r *Repository) MarkOutboxDelivered(
	ctx context.Context,
	eventID uuid.UUID,
) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE assessment_outbox_events
		    SET delivery_status = 'delivered', delivered_at = now()
		  WHERE event_id = $1 AND delivery_status IN ('pending', 'delivered')`,
		eventID,
	)
	if err != nil {
		return fmt.Errorf("mark event delivered: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) ListPendingOutboxEvents(
	ctx context.Context,
	limit int,
) ([]OutboxEvent, error) {
	rows, err := r.db.Query(ctx,
		`SELECT event_id, payload, correlation_id, attempts
		   FROM assessment_outbox_events
		  WHERE delivery_status = 'pending' AND next_attempt_at <= now()
		  ORDER BY occurred_at, event_id
		  LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list pending outbox events: %w", err)
	}
	defer rows.Close()
	events := make([]OutboxEvent, 0)
	for rows.Next() {
		var event OutboxEvent
		if err := rows.Scan(
			&event.ID, &event.Payload, &event.CorrelationID, &event.Attempts,
		); err != nil {
			return nil, fmt.Errorf("scan pending outbox event: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending outbox events: %w", err)
	}
	return events, nil
}

func (r *Repository) ListAnalyticsEvents(
	ctx context.Context, after time.Time, afterID uuid.UUID, limit int,
) (*AnalyticsEventPage, error) {
	rows, err := r.db.Query(ctx, `
		SELECT event_id, occurred_at, payload
		  FROM assessment_outbox_events
		 WHERE (occurred_at, event_id) > ($1, $2)
		   AND event_type IN ('GradePublished', 'FinalResultImported')
		 ORDER BY occurred_at, event_id
		 LIMIT $3
	`, after, afterID, limit+1)
	if err != nil {
		return nil, fmt.Errorf("list Analytics source events: %w", err)
	}
	defer rows.Close()
	page := &AnalyticsEventPage{Events: make([]AnalyticsEventRecord, 0, limit)}
	for rows.Next() {
		var event AnalyticsEventRecord
		if err := rows.Scan(&event.EventID, &event.OccurredAt, &event.Payload); err != nil {
			return nil, fmt.Errorf("scan Analytics source event: %w", err)
		}
		if len(page.Events) == limit {
			page.HasMore = true
			break
		}
		page.Events = append(page.Events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate Analytics source events: %w", err)
	}
	if page.HasMore && len(page.Events) > 0 {
		last := page.Events[len(page.Events)-1]
		page.NextOccurredAt = &last.OccurredAt
		page.NextEventID = &last.EventID
	}
	return page, nil
}

func (r *Repository) RecordOutboxFailure(
	ctx context.Context,
	eventID uuid.UUID,
	attempts int,
	lastError string,
) error {
	nextAttempt := attempts + 1
	delay := time.Minute * time.Duration(1<<min(nextAttempt-1, 10))
	status := "pending"
	if nextAttempt >= 20 {
		status = "dead"
	}
	if len(lastError) > 1000 {
		lastError = lastError[:1000]
	}
	tag, err := r.db.Exec(ctx,
		`UPDATE assessment_outbox_events
		    SET attempts = attempts + 1,
		        delivery_status = $2,
		        next_attempt_at = $3,
		        last_error = $4
		  WHERE event_id = $1 AND delivery_status = 'pending'`,
		eventID, status, time.Now().UTC().Add(delay), lastError,
	)
	if err != nil {
		return fmt.Errorf("record outbox delivery failure: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func mapDatabaseConflict(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrVersionConflict
	}
	return err
}
