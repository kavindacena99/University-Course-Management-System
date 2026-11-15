package analytics

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound       = errors.New("analytics record not found")
	ErrRebuildRunning = errors.New("analytics rebuild is already running")
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Ping(ctx context.Context) error {
	return r.db.Ping(ctx)
}

func (r *Repository) IngestEvent(ctx context.Context, event EventEnvelope) (bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin Analytics event transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := tx.QueryRow(ctx,
		`SELECT singleton FROM analytics_projection_state WHERE singleton = true FOR SHARE`,
	).Scan(new(bool)); err != nil {
		return false, fmt.Errorf("lock Analytics projection state: %w", err)
	}
	tag, err := tx.Exec(ctx,
		`INSERT INTO analytics_inbox_events (event_id, event_type, payload)
		 VALUES ($1, $2, $3) ON CONFLICT (event_id) DO NOTHING`,
		event.EventID, event.EventType, event.Payload,
	)
	if err != nil {
		return false, fmt.Errorf("persist Analytics inbox event: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}

	switch event.EventType {
	case "GradePublished":
		_, err = tx.Exec(ctx, `
			INSERT INTO component_grade_projection (
				grade_id, submission_id, assessment_id, offering_id,
				student_user_id, attempt_no, component_kind, mark, max_mark,
				weight_percent, aggregate_version, released_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
			ON CONFLICT (grade_id) DO UPDATE SET
				submission_id = EXCLUDED.submission_id,
				assessment_id = EXCLUDED.assessment_id,
				offering_id = EXCLUDED.offering_id,
				student_user_id = EXCLUDED.student_user_id,
				attempt_no = EXCLUDED.attempt_no,
				component_kind = EXCLUDED.component_kind,
				mark = EXCLUDED.mark,
				max_mark = EXCLUDED.max_mark,
				weight_percent = EXCLUDED.weight_percent,
				aggregate_version = EXCLUDED.aggregate_version,
				released_at = EXCLUDED.released_at
			WHERE component_grade_projection.aggregate_version < EXCLUDED.aggregate_version`,
			event.GradeID, event.SubmissionID, event.AssessmentID, event.OfferingID,
			event.StudentUserID, event.AttemptNo, event.ComponentKind, event.Mark,
			event.MaxMark, event.WeightPercent, event.version(), event.ReleasedAt,
		)
	case "FinalResultImported":
		_, err = tx.Exec(ctx, `
			INSERT INTO final_result_projection (
				result_id, offering_id, student_user_id, final_mark, max_mark,
				aggregate_version, imported_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7)
			ON CONFLICT (result_id) DO UPDATE SET
				offering_id = EXCLUDED.offering_id,
				student_user_id = EXCLUDED.student_user_id,
				final_mark = EXCLUDED.final_mark,
				max_mark = EXCLUDED.max_mark,
				aggregate_version = EXCLUDED.aggregate_version,
				imported_at = EXCLUDED.imported_at
			WHERE final_result_projection.aggregate_version < EXCLUDED.aggregate_version`,
			event.ResultID, event.OfferingID, event.StudentUserID,
			event.FinalMark, event.MaxMark, event.version(), event.ImportedAt,
		)
	}
	if err != nil {
		return false, fmt.Errorf("update Analytics projection: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit Analytics event: %w", err)
	}
	return true, nil
}

func (r *Repository) ProjectionRevision(ctx context.Context) (int64, error) {
	var available bool
	var revision int64
	if err := r.db.QueryRow(ctx,
		`SELECT available, revision
		   FROM analytics_projection_state WHERE singleton = true`,
	).Scan(&available, &revision); err != nil {
		return 0, fmt.Errorf("read Analytics projection state: %w", err)
	}
	if !available {
		return 0, ErrProjectionUnavailable
	}
	return revision, nil
}

func (r *Repository) ConfirmProjectionRevision(ctx context.Context, expected int64) error {
	var available bool
	var revision int64
	if err := r.db.QueryRow(ctx,
		`SELECT available, revision
		   FROM analytics_projection_state WHERE singleton = true`,
	).Scan(&available, &revision); err != nil {
		return fmt.Errorf("confirm Analytics projection state: %w", err)
	}
	if !available || revision != expected {
		return ErrProjectionUnavailable
	}
	return nil
}

func (r *Repository) BeginRebuild(ctx context.Context) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin Analytics rebuild: %w", err)
	}
	defer tx.Rollback(ctx)
	var rebuilding bool
	if err := tx.QueryRow(ctx, `
		UPDATE analytics_projection_state
		   SET rebuilding = true, available = false,
		       revision = revision + 1, updated_at = now()
		 WHERE singleton = true AND rebuilding = false
		RETURNING rebuilding
	`).Scan(&rebuilding); errors.Is(err, pgx.ErrNoRows) {
		return ErrRebuildRunning
	} else if err != nil {
		return fmt.Errorf("mark Analytics rebuild started: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM final_result_projection`); err != nil {
		return fmt.Errorf("clear final result projection: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM component_grade_projection`); err != nil {
		return fmt.Errorf("clear component grade projection: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM analytics_inbox_events`); err != nil {
		return fmt.Errorf("clear Analytics event archive: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit Analytics rebuild start: %w", err)
	}
	return nil
}

func (r *Repository) FinishRebuild(ctx context.Context) error {
	if _, err := r.db.Exec(ctx, `
		UPDATE analytics_projection_state
		   SET rebuilding = false, available = true, updated_at = now()
		 WHERE singleton = true
	`); err != nil {
		return fmt.Errorf("mark Analytics rebuild complete: %w", err)
	}
	return nil
}

func (r *Repository) FailRebuild(ctx context.Context) error {
	if _, err := r.db.Exec(ctx, `
		UPDATE analytics_projection_state
		   SET rebuilding = false, available = false, updated_at = now()
		 WHERE singleton = true
	`); err != nil {
		return fmt.Errorf("mark Analytics rebuild failed: %w", err)
	}
	return nil
}

func (r *Repository) StudentResults(
	ctx context.Context, studentID uuid.UUID,
) ([]FinalResult, error) {
	rows, err := r.db.Query(ctx, `
		SELECT result_id, offering_id, student_user_id, final_mark, max_mark, imported_at
		  FROM final_result_projection
		 WHERE student_user_id = $1
		 ORDER BY imported_at, result_id
	`, studentID)
	if err != nil {
		return nil, fmt.Errorf("list student final results: %w", err)
	}
	defer rows.Close()
	results := make([]FinalResult, 0)
	for rows.Next() {
		var result FinalResult
		if err := rows.Scan(&result.ResultID, &result.OfferingID,
			&result.StudentUserID, &result.FinalMark, &result.MaxMark, &result.ImportedAt); err != nil {
			return nil, fmt.Errorf("scan student final result: %w", err)
		}
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate student final results: %w", err)
	}
	return results, nil
}

func (r *Repository) OfferingResults(
	ctx context.Context, offeringID uuid.UUID,
) ([]FinalResult, error) {
	rows, err := r.db.Query(ctx, `
		SELECT result_id, offering_id, student_user_id, final_mark, max_mark, imported_at
		  FROM final_result_projection
		 WHERE offering_id = $1
		 ORDER BY student_user_id, imported_at, result_id
	`, offeringID)
	if err != nil {
		return nil, fmt.Errorf("list offering final results: %w", err)
	}
	defer rows.Close()
	results := make([]FinalResult, 0)
	for rows.Next() {
		var result FinalResult
		if err := rows.Scan(&result.ResultID, &result.OfferingID,
			&result.StudentUserID, &result.FinalMark, &result.MaxMark, &result.ImportedAt); err != nil {
			return nil, fmt.Errorf("scan offering final result: %w", err)
		}
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate offering final results: %w", err)
	}
	return results, nil
}

func (r *Repository) StudentComponents(
	ctx context.Context, studentID uuid.UUID,
) ([]ComponentGrade, error) {
	return r.components(ctx, `student_user_id = $1`, studentID)
}

func (r *Repository) OfferingComponents(
	ctx context.Context, offeringID uuid.UUID,
) ([]ComponentGrade, error) {
	return r.components(ctx, `offering_id = $1`, offeringID)
}

func (r *Repository) components(
	ctx context.Context, where string, id uuid.UUID,
) ([]ComponentGrade, error) {
	rows, err := r.db.Query(ctx, `
		SELECT grade_id, submission_id, assessment_id, offering_id, student_user_id,
		       attempt_no, component_kind, mark, max_mark, weight_percent, released_at
		  FROM component_grade_projection
		 WHERE `+where+`
		 ORDER BY released_at, grade_id
	`, id)
	if err != nil {
		return nil, fmt.Errorf("list component grades: %w", err)
	}
	defer rows.Close()
	items := make([]ComponentGrade, 0)
	for rows.Next() {
		var item ComponentGrade
		if err := rows.Scan(&item.GradeID, &item.SubmissionID, &item.AssessmentID,
			&item.OfferingID, &item.StudentUserID, &item.AttemptNo,
			&item.ComponentKind, &item.Mark, &item.MaxMark, &item.WeightPercent,
			&item.ReleasedAt); err != nil {
			return nil, fmt.Errorf("scan component grade: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate component grades: %w", err)
	}
	return items, nil
}

func (r *Repository) GradeBands(ctx context.Context) ([]GradeBand, error) {
	rows, err := r.db.Query(ctx, `
		SELECT grade_code, minimum_percent, maximum_percent, grade_points, display_order
		  FROM grade_bands ORDER BY display_order
	`)
	if err != nil {
		return nil, fmt.Errorf("list grade bands: %w", err)
	}
	defer rows.Close()
	bands := make([]GradeBand, 0)
	for rows.Next() {
		var band GradeBand
		if err := rows.Scan(&band.Code, &band.MinimumPercent, &band.MaximumPercent,
			&band.GradePoints, &band.DisplayOrder); err != nil {
			return nil, fmt.Errorf("scan grade band: %w", err)
		}
		bands = append(bands, band)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate grade bands: %w", err)
	}
	return bands, nil
}

func (r *Repository) ReplaceGradeBands(ctx context.Context, bands []GradeBand) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin grade-band update: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM grade_bands`); err != nil {
		return fmt.Errorf("clear grade bands: %w", err)
	}
	for _, band := range bands {
		if _, err := tx.Exec(ctx, `
			INSERT INTO grade_bands (
				grade_code, minimum_percent, maximum_percent, grade_points, display_order
			) VALUES ($1,$2,$3,$4,$5)
		`, band.Code, band.MinimumPercent, band.MaximumPercent,
			band.gpaValue(), band.DisplayOrder); err != nil {
			return fmt.Errorf("save grade band %q: %w", band.Code, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit grade-band update: %w", err)
	}
	return nil
}

func (r *Repository) ListProgrammes(ctx context.Context) ([]Programme, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, code, name, category, repeat_policy, failed_result_policy,
		       pass_grade_points, active
		  FROM programmes ORDER BY category, name
	`)
	if err != nil {
		return nil, fmt.Errorf("list programmes: %w", err)
	}
	defer rows.Close()
	items := make([]Programme, 0)
	for rows.Next() {
		var item Programme
		if err := rows.Scan(&item.ID, &item.Code, &item.Name, &item.Category,
			&item.RepeatPolicy, &item.FailedResultPolicy, &item.PassGradePoints,
			&item.Active); err != nil {
			return nil, fmt.Errorf("scan programme: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate programmes: %w", err)
	}
	return items, nil
}

func (r *Repository) CreateProgramme(ctx context.Context, input CreateProgrammeInput) (*Programme, error) {
	item := Programme{ID: uuid.New(), Code: input.Code, Name: input.Name,
		Category: input.Category, RepeatPolicy: input.RepeatPolicy,
		FailedResultPolicy: input.FailedResultPolicy,
		PassGradePoints: input.PassGradePoints, Active: true}
	err := r.db.QueryRow(ctx, `
		INSERT INTO programmes (
			id, code, name, category, repeat_policy, failed_result_policy, pass_grade_points
		) VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING active
	`, item.ID, item.Code, item.Name, item.Category, item.RepeatPolicy,
		item.FailedResultPolicy, item.PassGradePoints).Scan(&item.Active)
	if err != nil {
		return nil, fmt.Errorf("create programme: %w", err)
	}
	return &item, nil
}

func (r *Repository) UpdateProgramme(
	ctx context.Context, id uuid.UUID, input CreateProgrammeInput,
) (*Programme, error) {
	var item Programme
	err := r.db.QueryRow(ctx, `
		UPDATE programmes
		   SET code = $2, name = $3, category = $4, repeat_policy = $5,
		       failed_result_policy = $6, pass_grade_points = $7,
		       updated_at = now()
		 WHERE id = $1
		RETURNING id, code, name, category, repeat_policy, failed_result_policy,
		          pass_grade_points, active
	`, id, input.Code, input.Name, input.Category, input.RepeatPolicy,
		input.FailedResultPolicy, input.PassGradePoints).Scan(&item.ID,
		&item.Code, &item.Name, &item.Category, &item.RepeatPolicy,
		&item.FailedResultPolicy, &item.PassGradePoints, &item.Active)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update programme: %w", err)
	}
	return &item, nil
}

func (r *Repository) AssignProgramme(
	ctx context.Context, studentID, programmeID, actorID uuid.UUID,
) error {
	tag, err := r.db.Exec(ctx, `
		INSERT INTO student_programme_assignments (
			student_user_id, programme_id, assigned_by_user_id
		)
		SELECT $1, id, $3 FROM programmes WHERE id = $2 AND active = true
		ON CONFLICT (student_user_id) DO UPDATE SET
			programme_id = EXCLUDED.programme_id,
			assigned_by_user_id = EXCLUDED.assigned_by_user_id,
			assigned_at = now()
	`, studentID, programmeID, actorID)
	if err != nil {
		return fmt.Errorf("assign student programme: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) StudentProgramme(ctx context.Context, studentID uuid.UUID) (*Programme, error) {
	var item Programme
	err := r.db.QueryRow(ctx, `
		SELECT p.id, p.code, p.name, p.category, p.repeat_policy,
		       p.failed_result_policy, p.pass_grade_points, p.active
		  FROM student_programme_assignments a
		  JOIN programmes p ON p.id = a.programme_id
		 WHERE a.student_user_id = $1 AND p.active = true
	`, studentID).Scan(&item.ID, &item.Code, &item.Name, &item.Category,
		&item.RepeatPolicy, &item.FailedResultPolicy, &item.PassGradePoints,
		&item.Active)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read student programme: %w", err)
	}
	return &item, nil
}

func (r *Repository) AdminSummary(ctx context.Context) (*AdminSummary, error) {
	var total int64
	var average *float64
	if err := r.db.QueryRow(ctx, `
		SELECT count(*), avg(final_mark / max_mark * 100.0)
		  FROM final_result_projection
	`).Scan(&total, &average); err != nil {
		return nil, fmt.Errorf("aggregate final results: %w", err)
	}
	rows, err := r.db.Query(ctx, `
		SELECT COALESCE(b.grade_code, ''), count(r.result_id)
		  FROM final_result_projection r
		  LEFT JOIN grade_bands b
		    ON (r.final_mark / r.max_mark * 100.0) >= b.minimum_percent
		   AND ((r.final_mark / r.max_mark * 100.0) < b.maximum_percent
		        OR (b.maximum_percent = 100 AND r.final_mark / r.max_mark * 100.0 <= 100))
		 GROUP BY b.grade_code, b.display_order
		 ORDER BY b.display_order NULLS LAST, b.grade_code
	`)
	if err != nil {
		return nil, fmt.Errorf("aggregate grade distribution: %w", err)
	}
	defer rows.Close()
	distribution := make([]GradeSummary, 0)
	for rows.Next() {
		var item GradeSummary
		if err := rows.Scan(&item.GradeCode, &item.Count); err != nil {
			return nil, fmt.Errorf("scan grade distribution: %w", err)
		}
		distribution = append(distribution, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate grade distribution: %w", err)
	}
	rows.Close()
	programmeRows, err := r.db.Query(ctx, `
		SELECT p.code, p.name, p.category, count(r.result_id),
		       avg(r.final_mark / r.max_mark * 100.0)
		  FROM final_result_projection r
		  JOIN student_programme_assignments a ON a.student_user_id = r.student_user_id
		  JOIN programmes p ON p.id = a.programme_id
		 GROUP BY p.id, p.code, p.name, p.category
		 ORDER BY p.category, p.name
	`)
	if err != nil {
		return nil, fmt.Errorf("aggregate programme results: %w", err)
	}
	defer programmeRows.Close()
	programmes := make([]ProgrammeSummary, 0)
	for programmeRows.Next() {
		var item ProgrammeSummary
		if err := programmeRows.Scan(&item.Code, &item.Name, &item.Category,
			&item.TotalResults, &item.AveragePercent); err != nil {
			return nil, fmt.Errorf("scan programme aggregation: %w", err)
		}
		programmes = append(programmes, item)
	}
	if err := programmeRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate programme aggregations: %w", err)
	}
	return &AdminSummary{TotalResults: total, AveragePercent: average,
		Distribution: distribution, Programmes: programmes}, nil
}
