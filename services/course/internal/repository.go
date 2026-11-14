package course

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const courseColumns = `
	id,
	code,
	title,
	description,
	credits,
	status,
	version,
	created_by,
	updated_by,
	created_at,
	updated_at,
	archived_at
`

const offeringColumns = `
	o.id,
	o.course_id,
	c.code,
	c.title,
	o.academic_year,
	o.semester,
	o.start_date,
	o.end_date,
	o.enrollment_opens_at,
	o.enrollment_closes_at,
	o.enrollment_capacity,
	o.lecturer_user_id,
	o.lecturer_assigned_by,
	o.lecturer_assigned_at,
	o.status,
	o.version,
	o.created_by,
	o.updated_by,
	o.created_at,
	o.updated_at,
	o.archived_at
`

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Ping(ctx context.Context) error {
	return r.db.Ping(ctx)
}

type UpdateCourseRecord struct {
	CourseID        uuid.UUID
	ActorID         uuid.UUID
	ExpectedVersion int64
	Code            string
	Title           string
	Description     string
	Credits         int
	Previous        Course
}

type ChangeCourseStatusRecord struct {
	CourseID        uuid.UUID
	ActorID         uuid.UUID
	ExpectedVersion int64
	FromStatus      CourseStatus
	ToStatus        CourseStatus
	Action          string
}

type UpdateOfferingRecord struct {
	OfferingID         uuid.UUID
	ActorID            uuid.UUID
	ExpectedVersion    int64
	AcademicYear       string
	Semester           string
	StartDate          Date
	EndDate            Date
	EnrollmentOpensAt  *time.Time
	EnrollmentClosesAt *time.Time
	EnrollmentCapacity *int
	Previous           CourseOffering
}

type ChangeOfferingStatusRecord struct {
	OfferingID      uuid.UUID
	ActorID         uuid.UUID
	ExpectedVersion int64
	FromStatus      OfferingStatus
	ToStatus        OfferingStatus
	Action          string
}

type AssignLecturerRecord struct {
	OfferingID      uuid.UUID
	ActorID         uuid.UUID
	ExpectedVersion int64
	LecturerUserID  uuid.UUID
	Previous        CourseOffering
}

func (r *Repository) CreateCourse(ctx context.Context, item *Course) error {
	return r.withTx(ctx, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			INSERT INTO courses (
				id, code, title, description, credits, status, created_by, updated_by
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
			RETURNING version, created_at, updated_at
		`,
			item.ID, item.Code, item.Title, item.Description, item.Credits,
			item.Status, item.CreatedBy,
		).Scan(&item.Version, &item.CreatedAt, &item.UpdatedAt)
		if err != nil {
			return mapCourseWriteError(err)
		}

		return insertCourseAudit(ctx, tx, item.ID, nil, item.CreatedBy,
			"course.created", map[string]any{
				"code":    item.Code,
				"status":  item.Status,
				"version": item.Version,
			})
	})
}

func (r *Repository) GetCourse(
	ctx context.Context,
	id uuid.UUID,
) (*Course, error) {
	item, err := scanCourse(r.db.QueryRow(ctx,
		`SELECT `+courseColumns+` FROM courses WHERE id = $1`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCourseNotFound
		}
		return nil, fmt.Errorf("get course: %w", err)
	}
	return item, nil
}

func (r *Repository) UpdateCourse(
	ctx context.Context,
	input UpdateCourseRecord,
) (*Course, error) {
	var updated *Course

	err := r.withTx(ctx, func(tx pgx.Tx) error {
		current, err := getCourseForUpdate(ctx, tx, input.CourseID)
		if err != nil {
			return err
		}
		if current.Version != input.ExpectedVersion {
			return ErrVersionConflict
		}
		if current.Status == CourseArchived {
			return ErrCourseArchived
		}

		if current.Code != input.Code {
			if current.Status != CourseDraft {
				return ErrCourseCodeLocked
			}
			var offeringCount int
			if err := tx.QueryRow(ctx, `
				SELECT count(*) FROM course_offerings WHERE course_id = $1
			`, input.CourseID).Scan(&offeringCount); err != nil {
				return fmt.Errorf("count course offerings: %w", err)
			}
			if offeringCount > 0 {
				return ErrCourseCodeLocked
			}
		}

		item, err := scanCourse(tx.QueryRow(ctx, `
			UPDATE courses
			SET code = $3,
				title = $4,
				description = $5,
				credits = $6,
				version = version + 1,
				updated_by = $2,
				updated_at = NOW()
			WHERE id = $1 AND version = $7
			RETURNING `+courseColumns,
			input.CourseID, input.ActorID, input.Code, input.Title,
			input.Description, input.Credits, input.ExpectedVersion,
		))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrVersionConflict
			}
			return mapCourseWriteError(err)
		}

		details := map[string]any{
			"previous_version": current.Version,
			"new_version":      item.Version,
			"code_changed":     current.Code != item.Code,
		}
		if err := insertCourseAudit(ctx, tx, item.ID, nil, input.ActorID,
			"course.updated", details); err != nil {
			return err
		}

		updated = item
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (r *Repository) ChangeCourseStatus(
	ctx context.Context,
	input ChangeCourseStatusRecord,
) (*Course, error) {
	var updated *Course

	err := r.withTx(ctx, func(tx pgx.Tx) error {
		current, err := getCourseForUpdate(ctx, tx, input.CourseID)
		if err != nil {
			return err
		}
		if current.Version != input.ExpectedVersion || current.Status != input.FromStatus {
			return ErrVersionConflict
		}
		if !current.Status.CanTransitionTo(input.ToStatus) {
			return ErrInvalidTransition
		}

		if input.ToStatus == CourseArchived {
			var activeOfferings int
			if err := tx.QueryRow(ctx, `
				SELECT count(*)
				FROM course_offerings
				WHERE course_id = $1 AND status IN ('draft', 'published')
			`, input.CourseID).Scan(&activeOfferings); err != nil {
				return fmt.Errorf("count active course offerings: %w", err)
			}
			if activeOfferings > 0 {
				return ErrCourseHasActiveOfferings
			}
		}

		item, err := scanCourse(tx.QueryRow(ctx, `
			UPDATE courses
			SET status = $3::varchar,
				version = version + 1,
				updated_by = $2,
				updated_at = NOW(),
				archived_at = CASE
					WHEN $3::varchar = 'archived' THEN NOW()
					ELSE NULL
				END
			WHERE id = $1 AND version = $4
			RETURNING `+courseColumns,
			input.CourseID, input.ActorID, input.ToStatus, input.ExpectedVersion,
		))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrVersionConflict
			}
			return fmt.Errorf("change course status: %w", err)
		}

		event := "course.updated"
		switch input.Action {
		case "publish":
			event = "course.published"
		case "archive":
			event = "course.archived"
		case "restore":
			event = "course.restored"
		}

		if err := insertCourseAudit(ctx, tx, item.ID, nil, input.ActorID,
			event, map[string]any{
				"from_status": input.FromStatus,
				"to_status":   input.ToStatus,
				"new_version": item.Version,
			}); err != nil {
			return err
		}

		updated = item
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (r *Repository) ListCourses(
	ctx context.Context,
	filter CourseFilter,
) (*CourseList, error) {
	where := []string{"1 = 1"}
	args := make([]any, 0, 4)

	if filter.Status != "" {
		args = append(args, filter.Status)
		where = append(where, fmt.Sprintf("status = $%d", len(args)))
	}
	if search := strings.TrimSpace(filter.Search); search != "" {
		args = append(args, search)
		pattern := fmt.Sprintf("$%d", len(args))
		where = append(where, fmt.Sprintf(
			"(code ILIKE '%%' || %s || '%%' OR title ILIKE '%%' || %s || '%%')",
			pattern, pattern,
		))
	}
	whereSQL := strings.Join(where, " AND ")

	var total int64
	if err := r.db.QueryRow(ctx,
		`SELECT count(*) FROM courses WHERE `+whereSQL, args...,
	).Scan(&total); err != nil {
		return nil, fmt.Errorf("count courses: %w", err)
	}

	orderColumn := courseOrderColumn(filter.Sort)
	order := strings.ToUpper(filter.Order)
	if order != "DESC" {
		order = "ASC"
	}
	args = append(args, filter.Limit, filter.Offset)
	query := fmt.Sprintf(`
		SELECT %s
		FROM courses
		WHERE %s
		ORDER BY %s %s, id ASC
		LIMIT $%d OFFSET $%d
	`, courseColumns, whereSQL, orderColumn, order, len(args)-1, len(args))

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list courses: %w", err)
	}
	defer rows.Close()

	items := make([]Course, 0, filter.Limit)
	for rows.Next() {
		item, err := scanCourse(rows)
		if err != nil {
			return nil, fmt.Errorf("scan course: %w", err)
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate courses: %w", err)
	}

	return &CourseList{
		Courses: items,
		Total:   total,
		Limit:   filter.Limit,
		Offset:  filter.Offset,
	}, nil
}

func (r *Repository) CreateOffering(
	ctx context.Context,
	item *CourseOffering,
) error {
	return r.withTx(ctx, func(tx pgx.Tx) error {
		var courseStatus CourseStatus
		var courseCode, courseTitle string
		err := tx.QueryRow(ctx, `
			SELECT status, code, title
			FROM courses
			WHERE id = $1
			FOR UPDATE
		`, item.CourseID).Scan(&courseStatus, &courseCode, &courseTitle)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrCourseNotFound
			}
			return fmt.Errorf("lock offering course: %w", err)
		}
		if courseStatus == CourseArchived {
			return ErrCourseArchived
		}

		err = tx.QueryRow(ctx, `
			INSERT INTO course_offerings (
				id, course_id, academic_year, semester, start_date, end_date,
				enrollment_opens_at, enrollment_closes_at, enrollment_capacity,
				lecturer_user_id, lecturer_assigned_by, lecturer_assigned_at,
				status, created_by, updated_by
			)
			VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $14
			)
			RETURNING version, created_at, updated_at
		`,
			item.ID, item.CourseID, item.AcademicYear, item.Semester,
			item.StartDate.Time, item.EndDate.Time,
			timeArg(item.EnrollmentOpensAt), timeArg(item.EnrollmentClosesAt),
			intArg(item.EnrollmentCapacity), uuidArg(item.LecturerUserID),
			uuidArg(item.LecturerAssignedBy), timeArg(item.LecturerAssignedAt),
			item.Status, item.CreatedBy,
		).Scan(&item.Version, &item.CreatedAt, &item.UpdatedAt)
		if err != nil {
			return fmt.Errorf("create course offering: %w", err)
		}
		item.CourseCode = courseCode
		item.CourseTitle = courseTitle

		return insertCourseAudit(ctx, tx, item.CourseID, &item.ID, item.CreatedBy,
			"offering.created", map[string]any{
				"academic_year": item.AcademicYear,
				"semester":      item.Semester,
				"status":        item.Status,
				"version":       item.Version,
			})
	})
}

func (r *Repository) GetOffering(
	ctx context.Context,
	id uuid.UUID,
) (*CourseOffering, error) {
	item, err := scanOffering(r.db.QueryRow(ctx, `
		SELECT `+offeringColumns+`
		FROM course_offerings o
		JOIN courses c ON c.id = o.course_id
		WHERE o.id = $1
	`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrOfferingNotFound
		}
		return nil, fmt.Errorf("get course offering: %w", err)
	}
	return item, nil
}

func (r *Repository) UpdateOffering(
	ctx context.Context,
	input UpdateOfferingRecord,
) (*CourseOffering, error) {
	var updated *CourseOffering

	err := r.withTx(ctx, func(tx pgx.Tx) error {
		current, err := getOfferingForUpdate(ctx, tx, input.OfferingID)
		if err != nil {
			return err
		}
		if current.Version != input.ExpectedVersion {
			return ErrVersionConflict
		}
		switch current.Status {
		case OfferingClosed, OfferingArchived:
			return ErrOfferingImmutable
		case OfferingPublished:
			if err := validatePublishedOfferingUpdateRecord(current, input); err != nil {
				return err
			}
		}

		var newVersion int64
		err = tx.QueryRow(ctx, `
			UPDATE course_offerings
			SET academic_year = $3,
				semester = $4,
				start_date = $5,
				end_date = $6,
				enrollment_opens_at = $7,
				enrollment_closes_at = $8,
				enrollment_capacity = $9,
				version = version + 1,
				updated_by = $2,
				updated_at = NOW()
			WHERE id = $1 AND version = $10
			RETURNING version
		`,
			input.OfferingID, input.ActorID, input.AcademicYear, input.Semester,
			input.StartDate.Time, input.EndDate.Time,
			timeArg(input.EnrollmentOpensAt), timeArg(input.EnrollmentClosesAt),
			intArg(input.EnrollmentCapacity), input.ExpectedVersion,
		).Scan(&newVersion)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrVersionConflict
			}
			return fmt.Errorf("update course offering: %w", err)
		}
		item, err := getOfferingTx(ctx, tx, input.OfferingID)
		if err != nil {
			return err
		}

		if err := insertCourseAudit(ctx, tx, item.CourseID, &item.ID, input.ActorID,
			"offering.updated", map[string]any{
				"previous_version": current.Version,
				"new_version":      item.Version,
			}); err != nil {
			return err
		}

		updated = item
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (r *Repository) ChangeOfferingStatus(
	ctx context.Context,
	input ChangeOfferingStatusRecord,
) (*CourseOffering, error) {
	var updated *CourseOffering

	err := r.withTx(ctx, func(tx pgx.Tx) error {
		current, err := getOfferingForUpdate(ctx, tx, input.OfferingID)
		if err != nil {
			return err
		}
		if current.Version != input.ExpectedVersion ||
			current.Status != input.FromStatus {
			return ErrVersionConflict
		}
		if !current.Status.CanTransitionTo(input.ToStatus) {
			return ErrInvalidTransition
		}

		if input.Action == "publish" || input.Action == "restore" {
			var courseStatus CourseStatus
			err := tx.QueryRow(ctx, `
				SELECT status FROM courses WHERE id = $1 FOR UPDATE
			`, current.CourseID).Scan(&courseStatus)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return ErrCourseNotFound
				}
				return fmt.Errorf("lock parent course: %w", err)
			}
			if input.Action == "publish" {
				if courseStatus != CourseActive {
					return ErrCourseNotActive
				}
				if current.LecturerUserID == nil {
					return ErrInvalidTransition
				}
			}
			if input.Action == "restore" && courseStatus == CourseArchived {
				return ErrCourseArchived
			}
		}

		var newVersion int64
		err = tx.QueryRow(ctx, `
			UPDATE course_offerings
			SET status = $3::varchar,
				version = version + 1,
				updated_by = $2,
				updated_at = NOW(),
				archived_at = CASE
					WHEN $3::varchar = 'archived' THEN NOW()
					ELSE NULL
				END
			WHERE id = $1 AND version = $4
			RETURNING version
		`, input.OfferingID, input.ActorID, input.ToStatus,
			input.ExpectedVersion,
		).Scan(&newVersion)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrVersionConflict
			}
			return fmt.Errorf("change offering status: %w", err)
		}
		item, err := getOfferingTx(ctx, tx, input.OfferingID)
		if err != nil {
			return err
		}

		event := "offering.updated"
		switch input.Action {
		case "publish":
			event = "offering.published"
		case "close":
			event = "offering.closed"
		case "archive":
			event = "offering.archived"
		case "restore":
			event = "offering.restored"
		}
		if err := insertCourseAudit(ctx, tx, item.CourseID, &item.ID, input.ActorID,
			event, map[string]any{
				"from_status": input.FromStatus,
				"to_status":   input.ToStatus,
				"new_version": item.Version,
			}); err != nil {
			return err
		}

		updated = item
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (r *Repository) AssignLecturer(
	ctx context.Context,
	input AssignLecturerRecord,
) (*CourseOffering, error) {
	var updated *CourseOffering

	err := r.withTx(ctx, func(tx pgx.Tx) error {
		current, err := getOfferingForUpdate(ctx, tx, input.OfferingID)
		if err != nil {
			return err
		}
		if current.Version != input.ExpectedVersion {
			return ErrVersionConflict
		}
		switch current.Status {
		case OfferingClosed, OfferingArchived:
			return ErrOfferingImmutable
		}

		var newVersion int64
		err = tx.QueryRow(ctx, `
			UPDATE course_offerings
			SET lecturer_user_id = $3,
				lecturer_assigned_by = $2,
				lecturer_assigned_at = NOW(),
				version = version + 1,
				updated_by = $2,
				updated_at = NOW()
			WHERE id = $1 AND version = $4
			RETURNING version
		`, input.OfferingID, input.ActorID, input.LecturerUserID,
			input.ExpectedVersion,
		).Scan(&newVersion)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrVersionConflict
			}
			return fmt.Errorf("assign lecturer: %w", err)
		}
		item, err := getOfferingTx(ctx, tx, input.OfferingID)
		if err != nil {
			return err
		}

		event := "offering.lecturer_assigned"
		if current.LecturerUserID != nil {
			event = "offering.lecturer_reassigned"
		}
		if err := insertCourseAudit(ctx, tx, item.CourseID, &item.ID, input.ActorID,
			event, map[string]any{
				"previous_lecturer_user_id": uuidValue(current.LecturerUserID),
				"lecturer_user_id":          item.LecturerUserID,
				"new_version":               item.Version,
			}); err != nil {
			return err
		}

		updated = item
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (r *Repository) ListOfferings(
	ctx context.Context,
	filter OfferingFilter,
) (*OfferingList, error) {
	where := []string{"1 = 1"}
	args := make([]any, 0, 8)

	add := func(column string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf("%s = $%d", column, len(args)))
	}

	if filter.CourseID != nil {
		add("o.course_id", *filter.CourseID)
	}
	if filter.AcademicYear != "" {
		add("o.academic_year", filter.AcademicYear)
	}
	if filter.Semester != "" {
		add("o.semester", filter.Semester)
	}
	if filter.Status != "" {
		add("o.status", filter.Status)
	}
	if filter.LecturerID != nil {
		add("o.lecturer_user_id", *filter.LecturerID)
	}
	if filter.AssignedToUserID != nil {
		add("o.lecturer_user_id", *filter.AssignedToUserID)
	}
	if filter.PublishedOnly {
		where = append(where, "o.status = 'published'")
	}
	if search := strings.TrimSpace(filter.Search); search != "" {
		args = append(args, search)
		pattern := fmt.Sprintf("$%d", len(args))
		where = append(where, fmt.Sprintf(
			"(c.code ILIKE '%%' || %s || '%%' OR c.title ILIKE '%%' || %s || '%%')",
			pattern, pattern,
		))
	}

	whereSQL := strings.Join(where, " AND ")
	fromSQL := `
		FROM course_offerings o
		JOIN courses c ON c.id = o.course_id
		WHERE ` + whereSQL

	var total int64
	if err := r.db.QueryRow(ctx,
		`SELECT count(*) `+fromSQL, args...,
	).Scan(&total); err != nil {
		return nil, fmt.Errorf("count course offerings: %w", err)
	}

	orderColumn := offeringOrderColumn(filter.Sort)
	order := strings.ToUpper(filter.Order)
	if order != "DESC" {
		order = "ASC"
	}
	args = append(args, filter.Limit, filter.Offset)
	query := fmt.Sprintf(`
		SELECT %s
		%s
		ORDER BY %s %s, o.id ASC
		LIMIT $%d OFFSET $%d
	`, offeringColumns, fromSQL, orderColumn, order, len(args)-1, len(args))

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list course offerings: %w", err)
	}
	defer rows.Close()

	items := make([]CourseOffering, 0, filter.Limit)
	for rows.Next() {
		item, err := scanOffering(rows)
		if err != nil {
			return nil, fmt.Errorf("scan course offering: %w", err)
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate course offerings: %w", err)
	}

	return &OfferingList{
		Offerings: items,
		Total:     total,
		Limit:     filter.Limit,
		Offset:    filter.Offset,
	}, nil
}

func (r *Repository) withTx(
	ctx context.Context,
	fn func(pgx.Tx) error,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanCourse(row rowScanner) (*Course, error) {
	var item Course
	var status string
	var archivedAt pgtype.Timestamptz

	if err := row.Scan(
		&item.ID,
		&item.Code,
		&item.Title,
		&item.Description,
		&item.Credits,
		&status,
		&item.Version,
		&item.CreatedBy,
		&item.UpdatedBy,
		&item.CreatedAt,
		&item.UpdatedAt,
		&archivedAt,
	); err != nil {
		return nil, err
	}

	item.Status = CourseStatus(status)
	item.ArchivedAt = timePtr(archivedAt)
	return &item, nil
}

func scanOffering(row rowScanner) (*CourseOffering, error) {
	var item CourseOffering
	var startDate, endDate time.Time
	var status string
	var enrollmentOpensAt, enrollmentClosesAt pgtype.Timestamptz
	var enrollmentCapacity pgtype.Int4
	var lecturerUserID, lecturerAssignedBy pgtype.UUID
	var lecturerAssignedAt, archivedAt pgtype.Timestamptz

	if err := row.Scan(
		&item.ID,
		&item.CourseID,
		&item.CourseCode,
		&item.CourseTitle,
		&item.AcademicYear,
		&item.Semester,
		&startDate,
		&endDate,
		&enrollmentOpensAt,
		&enrollmentClosesAt,
		&enrollmentCapacity,
		&lecturerUserID,
		&lecturerAssignedBy,
		&lecturerAssignedAt,
		&status,
		&item.Version,
		&item.CreatedBy,
		&item.UpdatedBy,
		&item.CreatedAt,
		&item.UpdatedAt,
		&archivedAt,
	); err != nil {
		return nil, err
	}

	item.StartDate = NewDate(startDate)
	item.EndDate = NewDate(endDate)
	item.EnrollmentOpensAt = timePtr(enrollmentOpensAt)
	item.EnrollmentClosesAt = timePtr(enrollmentClosesAt)
	item.EnrollmentCapacity = intPtr(enrollmentCapacity)
	item.LecturerUserID = uuidPtr(lecturerUserID)
	item.LecturerAssignedBy = uuidPtr(lecturerAssignedBy)
	item.LecturerAssignedAt = timePtr(lecturerAssignedAt)
	item.Status = OfferingStatus(status)
	item.ArchivedAt = timePtr(archivedAt)
	return &item, nil
}

func getCourseForUpdate(
	ctx context.Context,
	tx pgx.Tx,
	id uuid.UUID,
) (*Course, error) {
	item, err := scanCourse(tx.QueryRow(ctx,
		`SELECT `+courseColumns+` FROM courses WHERE id = $1 FOR UPDATE`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCourseNotFound
		}
		return nil, fmt.Errorf("lock course: %w", err)
	}
	return item, nil
}

func getOfferingForUpdate(
	ctx context.Context,
	tx pgx.Tx,
	id uuid.UUID,
) (*CourseOffering, error) {
	item, err := scanOffering(tx.QueryRow(ctx, `
		SELECT `+offeringColumns+`
		FROM course_offerings o
		JOIN courses c ON c.id = o.course_id
		WHERE o.id = $1
		FOR UPDATE OF o
	`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrOfferingNotFound
		}
		return nil, fmt.Errorf("lock course offering: %w", err)
	}
	return item, nil
}

func getOfferingTx(
	ctx context.Context,
	tx pgx.Tx,
	id uuid.UUID,
) (*CourseOffering, error) {
	item, err := scanOffering(tx.QueryRow(ctx, `
		SELECT `+offeringColumns+`
		FROM course_offerings o
		JOIN courses c ON c.id = o.course_id
		WHERE o.id = $1
	`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrOfferingNotFound
		}
		return nil, fmt.Errorf("get course offering: %w", err)
	}
	return item, nil
}

func validatePublishedOfferingUpdateRecord(
	current *CourseOffering,
	input UpdateOfferingRecord,
) error {
	if current.AcademicYear != input.AcademicYear ||
		current.Semester != input.Semester ||
		!sameDate(current.StartDate, input.StartDate) ||
		!sameDate(current.EndDate, input.EndDate) {
		return ErrFieldLocked
	}
	if !sameOptionalTime(current.EnrollmentOpensAt, input.EnrollmentOpensAt) {
		return ErrFieldLocked
	}
	if !sameOptionalTime(current.EnrollmentClosesAt, input.EnrollmentClosesAt) {
		if current.EnrollmentClosesAt != nil &&
			(input.EnrollmentClosesAt == nil ||
				input.EnrollmentClosesAt.Before(*current.EnrollmentClosesAt)) {
			return ErrFieldLocked
		}
	}
	if current.EnrollmentCapacity == nil && input.EnrollmentCapacity != nil {
		return ErrFieldLocked
	}
	if current.EnrollmentCapacity != nil &&
		(input.EnrollmentCapacity == nil ||
			*input.EnrollmentCapacity < *current.EnrollmentCapacity) {
		return ErrFieldLocked
	}
	return nil
}

func insertCourseAudit(
	ctx context.Context,
	tx pgx.Tx,
	courseID uuid.UUID,
	offeringID *uuid.UUID,
	actorID uuid.UUID,
	event string,
	details map[string]any,
) error {
	payload, err := json.Marshal(details)
	if err != nil {
		return fmt.Errorf("marshal audit details: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO course_audit_events (
			course_id, offering_id, actor_id, event, details
		)
		VALUES ($1, $2, $3, $4, $5)
	`, courseID, uuidArg(offeringID), actorID, event, payload)
	if err != nil {
		return fmt.Errorf("insert course audit event: %w", err)
	}
	return nil
}

func mapCourseWriteError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" &&
		pgErr.ConstraintName == "courses_code_uq" {
		return ErrCourseCodeTaken
	}
	return err
}

func courseOrderColumn(sort string) string {
	switch sort {
	case "title":
		return "title"
	case "created_at":
		return "created_at"
	case "updated_at":
		return "updated_at"
	case "code", "":
		return "code"
	default:
		return "code"
	}
}

func offeringOrderColumn(sort string) string {
	switch sort {
	case "created_at":
		return "o.created_at"
	case "updated_at":
		return "o.updated_at"
	case "academic_year":
		return "o.academic_year"
	case "course_code":
		return "c.code"
	case "start_date", "":
		return "o.start_date"
	default:
		return "o.start_date"
	}
}

func timeArg(value *time.Time) any {
	if value == nil {
		return nil
	}
	return *value
}

func intArg(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func uuidArg(value *uuid.UUID) any {
	if value == nil {
		return nil
	}
	return *value
}

func timePtr(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time
	return &result
}

func intPtr(value pgtype.Int4) *int {
	if !value.Valid {
		return nil
	}
	result := int(value.Int32)
	return &result
}

func uuidPtr(value pgtype.UUID) *uuid.UUID {
	if !value.Valid {
		return nil
	}
	result := uuid.UUID(value.Bytes)
	return &result
}

func uuidValue(value *uuid.UUID) any {
	if value == nil {
		return nil
	}
	return value.String()
}
