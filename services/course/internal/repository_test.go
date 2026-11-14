package course

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newTestRepository(t *testing.T) (*Repository, *pgxpool.Pool) {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set; skipping PostgreSQL integration test")
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse test database URL: %v", err)
	}
	if cfg.ConnConfig.Database != "courses_test" {
		t.Fatalf("refusing to test database %q; want courses_test", cfg.ConnConfig.Database)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping test database: %v", err)
	}
	t.Cleanup(pool.Close)

	return NewRepository(pool), pool
}

func uniqueCourseCode() string {
	return "TST-" + strings.ToUpper(uuid.NewString()[:8])
}

func cleanupCourse(t *testing.T, pool *pgxpool.Pool, courseID uuid.UUID) {
	t.Helper()

	ctx := context.Background()
	for _, statement := range []string{
		`DELETE FROM course_audit_events WHERE course_id = $1`,
		`DELETE FROM course_offerings WHERE course_id = $1`,
		`DELETE FROM courses WHERE id = $1`,
	} {
		if _, err := pool.Exec(ctx, statement, courseID); err != nil {
			t.Fatalf("clean up course: %v", err)
		}
	}
}

func createTestCourse(t *testing.T, repo *Repository, pool *pgxpool.Pool) (*Course, uuid.UUID) {
	t.Helper()

	actorID := uuid.New()
	item := &Course{
		ID:          uuid.New(),
		Code:        uniqueCourseCode(),
		Title:       "Test Course",
		Description: "Course used by integration tests",
		Credits:     3,
		Status:      CourseDraft,
		CreatedBy:   actorID,
		UpdatedBy:   actorID,
	}
	if err := repo.CreateCourse(context.Background(), item); err != nil {
		t.Fatalf("create course: %v", err)
	}
	t.Cleanup(func() { cleanupCourse(t, pool, item.ID) })
	return item, actorID
}

func createTestOffering(
	t *testing.T,
	repo *Repository,
	course *Course,
	actorID uuid.UUID,
	withLecturer bool,
) *CourseOffering {
	t.Helper()

	var lecturerID *uuid.UUID
	if withLecturer {
		id := uuid.New()
		lecturerID = &id
	}
	assignedBy := actorID
	assignedAt := time.Now().UTC()
	opensAt := time.Now().UTC().Add(24 * time.Hour)
	closesAt := opensAt.Add(14 * 24 * time.Hour)
	capacity := 100

	item := &CourseOffering{
		ID:                 uuid.New(),
		CourseID:           course.ID,
		AcademicYear:       "2026-2027",
		Semester:           "Fall",
		StartDate:          testDate(t, "2026-09-01"),
		EndDate:            testDate(t, "2026-12-20"),
		EnrollmentOpensAt:  &opensAt,
		EnrollmentClosesAt: &closesAt,
		EnrollmentCapacity: &capacity,
		LecturerUserID:     lecturerID,
		Status:             OfferingDraft,
		CreatedBy:          actorID,
		UpdatedBy:          actorID,
	}
	if withLecturer {
		item.LecturerAssignedBy = &assignedBy
		item.LecturerAssignedAt = &assignedAt
	}
	if err := repo.CreateOffering(context.Background(), item); err != nil {
		t.Fatalf("create offering: %v", err)
	}
	return item
}

func TestRepositoryCoursePersistenceAndConflicts(t *testing.T) {
	repo, pool := newTestRepository(t)
	ctx := context.Background()
	course, actorID := createTestCourse(t, repo, pool)

	if course.Version != 1 || course.CreatedAt.IsZero() || course.UpdatedAt.IsZero() {
		t.Fatalf("unexpected created course: %+v", course)
	}
	fetched, err := repo.GetCourse(ctx, course.ID)
	if err != nil {
		t.Fatalf("get course: %v", err)
	}
	if fetched.Code != course.Code || fetched.Status != CourseDraft {
		t.Fatalf("unexpected fetched course: %+v", fetched)
	}

	duplicate := &Course{
		ID:        uuid.New(),
		Code:      course.Code,
		Title:     "Duplicate",
		Credits:   3,
		Status:    CourseDraft,
		CreatedBy: actorID,
		UpdatedBy: actorID,
	}
	if err := repo.CreateCourse(ctx, duplicate); !errors.Is(err, ErrCourseCodeTaken) {
		t.Fatalf("expected duplicate code error, got %v", err)
	}

	updated, err := repo.UpdateCourse(ctx, UpdateCourseRecord{
		CourseID:        course.ID,
		ActorID:         actorID,
		ExpectedVersion: 1,
		Code:            course.Code,
		Title:           "Updated Title",
		Description:     "Updated",
		Credits:         4,
	})
	if err != nil {
		t.Fatalf("update course: %v", err)
	}
	if updated.Version != 2 || updated.Title != "Updated Title" {
		t.Fatalf("unexpected updated course: %+v", updated)
	}

	_, err = repo.UpdateCourse(ctx, UpdateCourseRecord{
		CourseID:        course.ID,
		ActorID:         actorID,
		ExpectedVersion: 1,
		Code:            course.Code,
		Title:           "Stale",
		Credits:         3,
	})
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("expected version conflict, got %v", err)
	}

	var auditCount int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM course_audit_events WHERE course_id = $1`,
		course.ID,
	).Scan(&auditCount); err != nil {
		t.Fatalf("count audit events: %v", err)
	}
	if auditCount < 2 {
		t.Fatalf("expected audit events, got %d", auditCount)
	}
}

func TestRepositoryConcurrentUpdatesAllowOnlyOneWinner(t *testing.T) {
	repo, pool := newTestRepository(t)
	course, actorID := createTestCourse(t, repo, pool)

	var waitGroup sync.WaitGroup
	results := make(chan error, 2)
	for _, title := range []string{"First", "Second"} {
		title := title
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			_, err := repo.UpdateCourse(context.Background(), UpdateCourseRecord{
				CourseID:        course.ID,
				ActorID:         actorID,
				ExpectedVersion: 1,
				Code:            course.Code,
				Title:           title,
				Credits:         3,
			})
			results <- err
		}()
	}
	waitGroup.Wait()
	close(results)

	successes := 0
	conflicts := 0
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrVersionConflict):
			conflicts++
		default:
			t.Fatalf("unexpected concurrent update error: %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d", successes, conflicts)
	}

	fetched, err := repo.GetCourse(context.Background(), course.ID)
	if err != nil {
		t.Fatalf("get course: %v", err)
	}
	if fetched.Version != 2 {
		t.Fatalf("version = %d, want 2", fetched.Version)
	}
}

func TestRepositoryLifecycleRules(t *testing.T) {
	repo, pool := newTestRepository(t)
	ctx := context.Background()
	course, actorID := createTestCourse(t, repo, pool)
	offering := createTestOffering(t, repo, course, actorID, true)

	_, err := repo.ChangeCourseStatus(ctx, ChangeCourseStatusRecord{
		CourseID:        course.ID,
		ActorID:         actorID,
		ExpectedVersion: 1,
		FromStatus:      CourseDraft,
		ToStatus:        CourseArchived,
		Action:          "archive",
	})
	if !errors.Is(err, ErrCourseHasActiveOfferings) {
		t.Fatalf("expected active offerings to block archive, got %v", err)
	}

	publishedCourse, err := repo.ChangeCourseStatus(ctx, ChangeCourseStatusRecord{
		CourseID:        course.ID,
		ActorID:         actorID,
		ExpectedVersion: 1,
		FromStatus:      CourseDraft,
		ToStatus:        CourseActive,
		Action:          "publish",
	})
	if err != nil {
		t.Fatalf("publish course: %v", err)
	}
	if publishedCourse.Version != 2 || publishedCourse.Status != CourseActive {
		t.Fatalf("unexpected published course: %+v", publishedCourse)
	}

	_, err = repo.ChangeCourseStatus(ctx, ChangeCourseStatusRecord{
		CourseID:        course.ID,
		ActorID:         actorID,
		ExpectedVersion: 2,
		FromStatus:      CourseActive,
		ToStatus:        CourseActive,
		Action:          "publish",
	})
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected invalid transition, got %v", err)
	}

	publishedOffering, err := repo.ChangeOfferingStatus(ctx, ChangeOfferingStatusRecord{
		OfferingID:      offering.ID,
		ActorID:         actorID,
		ExpectedVersion: 1,
		FromStatus:      OfferingDraft,
		ToStatus:        OfferingPublished,
		Action:          "publish",
	})
	if err != nil {
		t.Fatalf("publish offering: %v", err)
	}
	if publishedOffering.Status != OfferingPublished || publishedOffering.Version != 2 {
		t.Fatalf("unexpected published offering: %+v", publishedOffering)
	}

	_, err = repo.ChangeCourseStatus(ctx, ChangeCourseStatusRecord{
		CourseID:        course.ID,
		ActorID:         actorID,
		ExpectedVersion: 2,
		FromStatus:      CourseActive,
		ToStatus:        CourseArchived,
		Action:          "archive",
	})
	if !errors.Is(err, ErrCourseHasActiveOfferings) {
		t.Fatalf("publish offering should block archive, got %v", err)
	}

	closed, err := repo.ChangeOfferingStatus(ctx, ChangeOfferingStatusRecord{
		OfferingID:      offering.ID,
		ActorID:         actorID,
		ExpectedVersion: 2,
		FromStatus:      OfferingPublished,
		ToStatus:        OfferingClosed,
		Action:          "close",
	})
	if err != nil {
		t.Fatalf("close offering: %v", err)
	}
	if closed.Status != OfferingClosed || closed.Version != 3 {
		t.Fatalf("unexpected closed offering: %+v", closed)
	}

	archivedOffering, err := repo.ChangeOfferingStatus(ctx, ChangeOfferingStatusRecord{
		OfferingID:      offering.ID,
		ActorID:         actorID,
		ExpectedVersion: 3,
		FromStatus:      OfferingClosed,
		ToStatus:        OfferingArchived,
		Action:          "archive",
	})
	if err != nil {
		t.Fatalf("archive offering: %v", err)
	}
	if archivedOffering.Status != OfferingArchived || archivedOffering.ArchivedAt == nil {
		t.Fatalf("unexpected archived offering: %+v", archivedOffering)
	}

	archivedCourse, err := repo.ChangeCourseStatus(ctx, ChangeCourseStatusRecord{
		CourseID:        course.ID,
		ActorID:         actorID,
		ExpectedVersion: 2,
		FromStatus:      CourseActive,
		ToStatus:        CourseArchived,
		Action:          "archive",
	})
	if err != nil {
		t.Fatalf("archive course: %v", err)
	}
	if archivedCourse.Status != CourseArchived || archivedCourse.ArchivedAt == nil {
		t.Fatalf("unexpected archived course: %+v", archivedCourse)
	}

	_, err = repo.ChangeOfferingStatus(ctx, ChangeOfferingStatusRecord{
		OfferingID:      offering.ID,
		ActorID:         actorID,
		ExpectedVersion: 4,
		FromStatus:      OfferingArchived,
		ToStatus:        OfferingDraft,
		Action:          "restore",
	})
	if !errors.Is(err, ErrCourseArchived) {
		t.Fatalf("expected archived course to block offering restore, got %v", err)
	}

	restoredCourse, err := repo.ChangeCourseStatus(ctx, ChangeCourseStatusRecord{
		CourseID:        course.ID,
		ActorID:         actorID,
		ExpectedVersion: 3,
		FromStatus:      CourseArchived,
		ToStatus:        CourseActive,
		Action:          "restore",
	})
	if err != nil {
		t.Fatalf("restore course: %v", err)
	}
	if restoredCourse.Status != CourseActive || restoredCourse.ArchivedAt != nil {
		t.Fatalf("unexpected restored course: %+v", restoredCourse)
	}

	restoredOffering, err := repo.ChangeOfferingStatus(ctx, ChangeOfferingStatusRecord{
		OfferingID:      offering.ID,
		ActorID:         actorID,
		ExpectedVersion: 4,
		FromStatus:      OfferingArchived,
		ToStatus:        OfferingDraft,
		Action:          "restore",
	})
	if err != nil {
		t.Fatalf("restore offering: %v", err)
	}
	if restoredOffering.Status != OfferingDraft || restoredOffering.ArchivedAt != nil {
		t.Fatalf("unexpected restored offering: %+v", restoredOffering)
	}

	_, err = repo.ChangeCourseStatus(ctx, ChangeCourseStatusRecord{
		CourseID:        course.ID,
		ActorID:         actorID,
		ExpectedVersion: 4,
		FromStatus:      CourseActive,
		ToStatus:        CourseArchived,
		Action:          "archive",
	})
	if !errors.Is(err, ErrCourseHasActiveOfferings) {
		t.Fatalf("restored draft offering should block archive, got %v", err)
	}
}

func TestRepositoryOfferingMutationRulesAndAudit(t *testing.T) {
	repo, pool := newTestRepository(t)
	ctx := context.Background()
	course, actorID := createTestCourse(t, repo, pool)
	offering := createTestOffering(t, repo, course, actorID, true)

	updated, err := repo.UpdateCourse(ctx, UpdateCourseRecord{
		CourseID:        course.ID,
		ActorID:         actorID,
		ExpectedVersion: 1,
		Code:            course.Code,
		Title:           "Updated",
		Credits:         3,
	})
	if err != nil {
		t.Fatalf("update course: %v", err)
	}

	_, err = repo.UpdateCourse(ctx, UpdateCourseRecord{
		CourseID:        course.ID,
		ActorID:         actorID,
		ExpectedVersion: updated.Version,
		Code:            "TST-" + strings.ToUpper(uuid.NewString()[:8]),
		Title:           updated.Title,
		Credits:         updated.Credits,
	})
	if !errors.Is(err, ErrCourseCodeLocked) {
		t.Fatalf("expected offering to lock course code, got %v", err)
	}

	if _, err := repo.ChangeCourseStatus(ctx, ChangeCourseStatusRecord{
		CourseID:        course.ID,
		ActorID:         actorID,
		ExpectedVersion: updated.Version,
		FromStatus:      CourseDraft,
		ToStatus:        CourseActive,
		Action:          "publish",
	}); err != nil {
		t.Fatalf("publish course: %v", err)
	}

	published, err := repo.ChangeOfferingStatus(ctx, ChangeOfferingStatusRecord{
		OfferingID:      offering.ID,
		ActorID:         actorID,
		ExpectedVersion: 1,
		FromStatus:      OfferingDraft,
		ToStatus:        OfferingPublished,
		Action:          "publish",
	})
	if err != nil {
		t.Fatalf("publish offering: %v", err)
	}

	shrunkCapacity := 50
	_, err = repo.UpdateOffering(ctx, UpdateOfferingRecord{
		OfferingID:         offering.ID,
		ActorID:            actorID,
		ExpectedVersion:    published.Version,
		AcademicYear:       published.AcademicYear,
		Semester:           published.Semester,
		StartDate:          published.StartDate,
		EndDate:            published.EndDate,
		EnrollmentOpensAt:  published.EnrollmentOpensAt,
		EnrollmentClosesAt: published.EnrollmentClosesAt,
		EnrollmentCapacity: &shrunkCapacity,
	})
	if !errors.Is(err, ErrFieldLocked) {
		t.Fatalf("expected capacity decrease lock, got %v", err)
	}

	extendedClose := published.EnrollmentClosesAt.Add(24 * time.Hour)
	grownCapacity := 150
	updatedOffering, err := repo.UpdateOffering(ctx, UpdateOfferingRecord{
		OfferingID:         offering.ID,
		ActorID:            actorID,
		ExpectedVersion:    published.Version,
		AcademicYear:       published.AcademicYear,
		Semester:           published.Semester,
		StartDate:          published.StartDate,
		EndDate:            published.EndDate,
		EnrollmentOpensAt:  published.EnrollmentOpensAt,
		EnrollmentClosesAt: &extendedClose,
		EnrollmentCapacity: &grownCapacity,
	})
	if err != nil {
		t.Fatalf("extend published offering: %v", err)
	}
	if updatedOffering.EnrollmentCapacity == nil ||
		*updatedOffering.EnrollmentCapacity != 150 {
		t.Fatalf("unexpected updated offering: %+v", updatedOffering)
	}

	newLecturerID := uuid.New()
	assigned, err := repo.AssignLecturer(ctx, AssignLecturerRecord{
		OfferingID:      offering.ID,
		ActorID:         actorID,
		ExpectedVersion: updatedOffering.Version,
		LecturerUserID:  newLecturerID,
	})
	if err != nil {
		t.Fatalf("assign lecturer: %v", err)
	}
	if assigned.LecturerUserID == nil || *assigned.LecturerUserID != newLecturerID {
		t.Fatalf("unexpected assigned offering: %+v", assigned)
	}

	var auditCount int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM course_audit_events WHERE course_id = $1`,
		course.ID,
	).Scan(&auditCount); err != nil {
		t.Fatalf("count audit events: %v", err)
	}
	if auditCount < 5 {
		t.Fatalf("expected audit events for mutations, got %d", auditCount)
	}
}

func TestRepositoryRejectsPublishedUnlimitedToFiniteCapacity(t *testing.T) {
	repo, pool := newTestRepository(t)
	ctx := context.Background()
	course, actorID := createTestCourse(t, repo, pool)
	offering := createTestOffering(t, repo, course, actorID, true)
	if _, err := pool.Exec(ctx,
		`UPDATE course_offerings SET enrollment_capacity = NULL WHERE id = $1`, offering.ID,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ChangeCourseStatus(ctx, ChangeCourseStatusRecord{
		CourseID: course.ID, ActorID: actorID, ExpectedVersion: 1,
		FromStatus: CourseDraft, ToStatus: CourseActive, Action: "publish",
	}); err != nil {
		t.Fatal(err)
	}
	published, err := repo.ChangeOfferingStatus(ctx, ChangeOfferingStatusRecord{
		OfferingID: offering.ID, ActorID: actorID, ExpectedVersion: 1,
		FromStatus: OfferingDraft, ToStatus: OfferingPublished, Action: "publish",
	})
	if err != nil {
		t.Fatal(err)
	}
	capacity := 10
	_, err = repo.UpdateOffering(ctx, UpdateOfferingRecord{
		OfferingID: offering.ID, ActorID: actorID, ExpectedVersion: published.Version,
		AcademicYear: published.AcademicYear, Semester: published.Semester,
		StartDate: published.StartDate, EndDate: published.EndDate,
		EnrollmentOpensAt:  published.EnrollmentOpensAt,
		EnrollmentClosesAt: published.EnrollmentClosesAt,
		EnrollmentCapacity: &capacity,
	})
	if !errors.Is(err, ErrFieldLocked) {
		t.Fatalf("expected published unlimited-to-finite lock, got %v", err)
	}
}

func TestRepositoryListFiltersAndHiddenError(t *testing.T) {
	repo, pool := newTestRepository(t)
	ctx := context.Background()
	course, actorID := createTestCourse(t, repo, pool)
	offering := createTestOffering(t, repo, course, actorID, false)

	if _, err := repo.GetCourse(ctx, uuid.New()); !errors.Is(err, ErrCourseNotFound) {
		t.Fatalf("expected course not found, got %v", err)
	}
	if _, err := repo.GetOffering(ctx, uuid.New()); !errors.Is(err, ErrOfferingNotFound) {
		t.Fatalf("expected offering not found, got %v", err)
	}

	courses, err := repo.ListCourses(ctx, CourseFilter{
		Search: course.Code,
		Limit:  10,
		Sort:   "code",
		Order:  "asc",
	})
	if err != nil {
		t.Fatalf("list courses: %v", err)
	}
	if courses.Total != 1 || len(courses.Courses) != 1 {
		t.Fatalf("unexpected course list: %+v", courses)
	}

	offerings, err := repo.ListOfferings(ctx, OfferingFilter{
		CourseID: &course.ID,
		Search:   course.Title,
		Limit:    10,
		Sort:     "start_date",
		Order:    "asc",
	})
	if err != nil {
		t.Fatalf("list offerings: %v", err)
	}
	if offerings.Total != 1 || len(offerings.Offerings) != 1 {
		t.Fatalf("unexpected offering list: %+v", offerings)
	}
	if offerings.Offerings[0].ID != offering.ID {
		t.Fatalf("offering id = %s, want %s", offerings.Offerings[0].ID, offering.ID)
	}

	empty, err := repo.ListOfferings(ctx, OfferingFilter{
		CourseID: &course.ID,
		Status:   OfferingPublished,
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("list published offerings: %v", err)
	}
	if empty.Total != 0 || len(empty.Offerings) != 0 {
		t.Fatalf("expected no published offerings, got %+v", empty)
	}
}

func TestRepositoryAssignmentAuditEvent(t *testing.T) {
	repo, pool := newTestRepository(t)
	ctx := context.Background()
	course, actorID := createTestCourse(t, repo, pool)
	offering := createTestOffering(t, repo, course, actorID, true)

	newLecturerID := uuid.New()
	assigned, err := repo.AssignLecturer(ctx, AssignLecturerRecord{
		OfferingID:      offering.ID,
		ActorID:         actorID,
		ExpectedVersion: 1,
		LecturerUserID:  newLecturerID,
	})
	if err != nil {
		t.Fatalf("assign lecturer: %v", err)
	}

	var event string
	if err := pool.QueryRow(ctx, `
		SELECT event
		FROM course_audit_events
		WHERE offering_id = $1
		ORDER BY id DESC
		LIMIT 1
	`, offering.ID).Scan(&event); err != nil {
		t.Fatalf("read assignment audit event: %v", err)
	}
	if event != "offering.lecturer_reassigned" {
		t.Fatalf("event = %q", event)
	}
	if assigned.LecturerAssignedAt == nil || assigned.LecturerAssignedBy == nil {
		t.Fatal("expected assignment metadata")
	}
	if assigned.LecturerAssignedAt.After(time.Now().Add(time.Minute)) {
		t.Fatal("unexpected assignment timestamp")
	}
}
