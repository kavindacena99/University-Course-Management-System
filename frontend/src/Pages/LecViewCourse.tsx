import { FormEvent, useEffect, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import type { Course, CourseOffering } from "../types/api";
import {
  assignOfferingLecturer,
  changeOfferingStatus,
  createOffering,
  getCourse,
  listCourseOfferings,
  updateOffering,
} from "../services/courses";
import { getCurrentUser } from "../services/auth";
import "./LecViewCourses.css";

type OfferingFormState = {
  academic_year: string;
  semester: string;
  start_date: string;
  end_date: string;
  enrollment_opens_at: string;
  enrollment_closes_at: string;
  enrollment_capacity: string;
  lecturer_user_id: string;
};

function asLocalDateTime(value?: string | null) {
  if (!value) return "";
  const date = new Date(value);
  const pad = (part: number) => String(part).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

function toIsoDateTime(value: string) {
  return value ? new Date(value).toISOString() : null;
}

function OfferingManager({
  offering,
  canManage,
  isAdmin,
  onUpdated,
}: {
  offering: CourseOffering;
  canManage: boolean;
  isAdmin: boolean;
  onUpdated: (offering: CourseOffering) => void;
}) {
  const [open, setOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [form, setForm] = useState<OfferingFormState>({
    academic_year: offering.academic_year,
    semester: offering.semester,
    start_date: offering.start_date,
    end_date: offering.end_date,
    enrollment_opens_at: asLocalDateTime(offering.enrollment_opens_at),
    enrollment_closes_at: asLocalDateTime(offering.enrollment_closes_at),
    enrollment_capacity: offering.enrollment_capacity ? String(offering.enrollment_capacity) : "",
    lecturer_user_id: offering.lecturer_user_id ?? "",
  });

  const setField = (field: keyof OfferingFormState, value: string) => {
    setForm((current) => ({ ...current, [field]: value }));
  };

  async function saveDetails(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    const capacity = form.enrollment_capacity ? Number(form.enrollment_capacity) : null;
    if (
      !form.academic_year.trim() ||
      !form.semester.trim() ||
      !form.start_date ||
      !form.end_date ||
      form.end_date < form.start_date ||
      (capacity !== null && (!Number.isInteger(capacity) || capacity < 1 || capacity > 100000)) ||
      (form.enrollment_opens_at && form.enrollment_closes_at && form.enrollment_closes_at <= form.enrollment_opens_at)
    ) {
      setError("Enter a valid academic period, date range, enrollment window, and capacity.");
      return;
    }

    setSaving(true);
    try {
      const { offering: updated } = await updateOffering(offering.id, {
        version: offering.version,
        academic_year: form.academic_year.trim(),
        semester: form.semester.trim(),
        start_date: form.start_date,
        end_date: form.end_date,
        enrollment_opens_at: toIsoDateTime(form.enrollment_opens_at),
        enrollment_closes_at: toIsoDateTime(form.enrollment_closes_at),
        enrollment_capacity: capacity,
      });
      onUpdated(updated);
      setOpen(false);
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : "Unable to update the offering.");
    } finally {
      setSaving(false);
    }
  }

  async function assignLecturer() {
    setError("");
    if (!form.lecturer_user_id.trim()) {
      setError("Enter the approved lecturer's user ID.");
      return;
    }
    setSaving(true);
    try {
      const { offering: updated } = await assignOfferingLecturer(
        offering.id,
        offering.version,
        form.lecturer_user_id.trim(),
      );
      onUpdated(updated);
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : "Unable to assign the lecturer.");
    } finally {
      setSaving(false);
    }
  }

  async function setStatus(action: "publish" | "close" | "archive" | "restore") {
    setError("");
    setSaving(true);
    try {
      const { offering: updated } = await changeOfferingStatus(offering, action);
      onUpdated(updated);
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : "Unable to update the offering status.");
    } finally {
      setSaving(false);
    }
  }

  if (!canManage) return null;

  return (
    <div className="course-details__offering-management">
      <button type="button" disabled={saving} onClick={() => setOpen((value) => !value)}>
        {open ? "Hide offering settings" : "Manage offering"}
      </button>
      {offering.status === "draft" && (
        <button type="button" disabled={saving} onClick={() => void setStatus("publish")}>Publish</button>
      )}
      {offering.status === "published" && (
        <button type="button" disabled={saving} onClick={() => void setStatus("close")}>Close enrollment</button>
      )}
      {["published", "closed"].includes(offering.status) && (
        <button type="button" disabled={saving} onClick={() => void setStatus("archive")}>Archive</button>
      )}
      {offering.status === "archived" && isAdmin && (
        <button type="button" disabled={saving} onClick={() => void setStatus("restore")}>Restore</button>
      )}
      {open && (
        <form className="course-details__offering-form" onSubmit={saveDetails}>
          <label>
            Academic year
            <input maxLength={20} required value={form.academic_year} onChange={(event) => setField("academic_year", event.target.value)} />
          </label>
          <label>
            Semester
            <input maxLength={20} required value={form.semester} onChange={(event) => setField("semester", event.target.value)} />
          </label>
          <label>
            Start date
            <input type="date" required value={form.start_date} onChange={(event) => setField("start_date", event.target.value)} />
          </label>
          <label>
            End date
            <input type="date" required value={form.end_date} onChange={(event) => setField("end_date", event.target.value)} />
          </label>
          <label>
            Enrollment opens
            <input type="datetime-local" value={form.enrollment_opens_at} onChange={(event) => setField("enrollment_opens_at", event.target.value)} />
          </label>
          <label>
            Enrollment closes
            <input type="datetime-local" value={form.enrollment_closes_at} onChange={(event) => setField("enrollment_closes_at", event.target.value)} />
          </label>
          <label>
            Enrollment capacity
            <input type="number" min="1" max="100000" step="1" value={form.enrollment_capacity} onChange={(event) => setField("enrollment_capacity", event.target.value)} />
          </label>
          <button type="submit" disabled={saving}>{saving ? "Saving..." : "Save offering"}</button>
          {isAdmin && (
            <>
              <label>
                Lecturer user ID
                <input value={form.lecturer_user_id} onChange={(event) => setField("lecturer_user_id", event.target.value)} />
              </label>
              <button type="button" disabled={saving} onClick={() => void assignLecturer()}>
                Assign lecturer
              </button>
            </>
          )}
          {error && <p role="alert">{error}</p>}
        </form>
      )}
      {!open && error && <p role="alert">{error}</p>}
    </div>
  );
}

export default function LecViewCourse() {
  const [searchParams] = useSearchParams();
  const courseId = searchParams.get("courseId") ?? "";
  const [course, setCourse] = useState<Course | null>(null);
  const [offerings, setOfferings] = useState<CourseOffering[]>([]);
  const [currentUserId, setCurrentUserId] = useState("");
  const [isAdmin, setIsAdmin] = useState(false);
  const [canManageFinalResults, setCanManageFinalResults] = useState(false);
  const [canManageAssessments, setCanManageAssessments] = useState(false);
  const [creatingOffering, setCreatingOffering] = useState(false);
  const [offeringError, setOfferingError] = useState("");
  const [newOffering, setNewOffering] = useState({
    academic_year: "2026/2027",
    semester: "2",
    start_date: "2027-01-01",
    end_date: "2027-06-30",
    enrollment_opens_at: "",
    enrollment_closes_at: "",
    enrollment_capacity: "",
    lecturer_user_id: "",
  });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    let active = true;
    if (!courseId) {
      setError("Select a course from the course directory to view its details.");
      setLoading(false);
      return () => {
        active = false;
      };
    }

    Promise.all([
      getCourse(courseId),
      listCourseOfferings(courseId),
      getCurrentUser(),
    ])
      .then(([courseResponse, offeringResponse, user]) => {
        if (!active) return;
        setCourse(courseResponse.course);
        setOfferings(offeringResponse.offerings);
        setCurrentUserId(user.id);
        setIsAdmin(user.roles.includes("ADMIN"));
        setCanManageFinalResults(
          user.roles.includes("ADMIN") &&
            user.permissions?.includes("assessments:student:read") === true &&
            user.permissions?.includes("assessments:results:import") === true,
        );
        setCanManageAssessments(
          user.roles.includes("LECTURER") || user.roles.includes("ADMIN"),
        );
      })
      .catch((requestError) => {
        if (active) {
          setError(requestError instanceof Error ? requestError.message : "Unable to load course details.");
        }
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [courseId]);

  async function createCourseOffering(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setOfferingError("");
    if (!course || course.status !== "active") {
      setOfferingError("Publish this course before creating an offering.");
      return;
    }
    const capacity = newOffering.enrollment_capacity
      ? Number(newOffering.enrollment_capacity)
      : null;
    if (
      !newOffering.academic_year.trim() ||
      !newOffering.semester.trim() ||
      !newOffering.start_date ||
      !newOffering.end_date ||
      newOffering.end_date < newOffering.start_date ||
      (capacity !== null && (!Number.isInteger(capacity) || capacity < 1 || capacity > 100000)) ||
      (newOffering.enrollment_opens_at &&
        newOffering.enrollment_closes_at &&
        newOffering.enrollment_closes_at <= newOffering.enrollment_opens_at)
    ) {
      setOfferingError("Enter a valid academic period, date range, enrollment window, and capacity.");
      return;
    }
    setCreatingOffering(true);
    try {
      const { offering } = await createOffering({
        course_id: course.id,
        academic_year: newOffering.academic_year.trim(),
        semester: newOffering.semester.trim(),
        start_date: newOffering.start_date,
        end_date: newOffering.end_date,
        enrollment_opens_at: toIsoDateTime(newOffering.enrollment_opens_at),
        enrollment_closes_at: toIsoDateTime(newOffering.enrollment_closes_at),
        enrollment_capacity: capacity,
        lecturer_user_id: newOffering.lecturer_user_id.trim() || null,
      });
      setOfferings((current) => [offering, ...current]);
      setNewOffering((current) => ({ ...current, lecturer_user_id: "" }));
    } catch (requestError) {
      setOfferingError(requestError instanceof Error ? requestError.message : "Unable to create the offering.");
    } finally {
      setCreatingOffering(false);
    }
  }

  return (
    <main className="course-details">
      <header className="course-details__header">
        <div>
          <p>Course directory</p>
          <h1>{course ? `${course.code} - ${course.title}` : "Course details"}</h1>
        </div>
        <Link to="/courses">Back to courses</Link>
      </header>

      {error && <p className="course-details__error" role="alert">{error}</p>}
      {loading ? (
        <p role="status">Loading course details...</p>
      ) : course ? (
        <>
          <section className="course-details__summary">
            <article><span>Status</span><strong>{course.status}</strong></article>
            <article><span>Credits</span><strong>{course.credits}</strong></article>
            <article><span>Last updated</span><strong>{new Date(course.updated_at).toLocaleDateString()}</strong></article>
          </section>
          {course.description && (
            <section className="course-details__panel">
              <h2>About this course</h2>
              <p>{course.description}</p>
            </section>
          )}
          <section className="course-details__panel">
            <h2>Course offerings</h2>
            {offerings.length ? (
              <div className="course-details__offerings">
                {offerings.map((offering) => (
                  <article key={offering.id}>
                    <div>
                      <strong>{offering.academic_year} · {offering.semester}</strong>
                      <span>{offering.start_date} to {offering.end_date}</span>
                    </div>
                    <span className={`course-details__status course-details__status--${offering.status}`}>
                      {offering.status}
                    </span>
                    <OfferingManager
                      offering={offering}
                      canManage={isAdmin || offering.lecturer_user_id === currentUserId}
                      isAdmin={isAdmin}
                      onUpdated={(updated) => {
                        setOfferings((current) =>
                          current.map((item) => item.id === updated.id ? updated : item),
                        );
                      }}
                    />
                    {canManageAssessments && (
                      <Link to={`/lecturer/assignments?offeringId=${encodeURIComponent(offering.id)}`}>
                        Assessments
                      </Link>
                    )}
                    {canManageFinalResults && (
                      <Link to={`/admin/final-results?offeringId=${encodeURIComponent(offering.id)}`}>
                        Final results
                      </Link>
                    )}
                  </article>
                ))}
              </div>
            ) : <p className="course-details__empty">No offerings are available for this course.</p>}
            {isAdmin && (
              <form className="course-details__offering-form" onSubmit={createCourseOffering}>
                <h3>Create offering</h3>
                <label>
                  Academic year
                  <input maxLength={20} required value={newOffering.academic_year} onChange={(event) => setNewOffering((current) => ({ ...current, academic_year: event.target.value }))} />
                </label>
                <label>
                  Semester
                  <input maxLength={20} required value={newOffering.semester} onChange={(event) => setNewOffering((current) => ({ ...current, semester: event.target.value }))} />
                </label>
                <label>
                  Start date
                  <input type="date" required value={newOffering.start_date} onChange={(event) => setNewOffering((current) => ({ ...current, start_date: event.target.value }))} />
                </label>
                <label>
                  End date
                  <input type="date" required value={newOffering.end_date} onChange={(event) => setNewOffering((current) => ({ ...current, end_date: event.target.value }))} />
                </label>
                <label>
                  Enrollment opens
                  <input type="datetime-local" value={newOffering.enrollment_opens_at} onChange={(event) => setNewOffering((current) => ({ ...current, enrollment_opens_at: event.target.value }))} />
                </label>
                <label>
                  Enrollment closes
                  <input type="datetime-local" value={newOffering.enrollment_closes_at} onChange={(event) => setNewOffering((current) => ({ ...current, enrollment_closes_at: event.target.value }))} />
                </label>
                <label>
                  Enrollment capacity
                  <input type="number" min="1" max="100000" step="1" value={newOffering.enrollment_capacity} onChange={(event) => setNewOffering((current) => ({ ...current, enrollment_capacity: event.target.value }))} />
                </label>
                <label>
                  Lecturer user ID (optional)
                  <input value={newOffering.lecturer_user_id} onChange={(event) => setNewOffering((current) => ({ ...current, lecturer_user_id: event.target.value }))} />
                </label>
                <button type="submit" disabled={creatingOffering || course.status !== "active"}>
                  {creatingOffering ? "Creating..." : "Create draft offering"}
                </button>
                {offeringError && <p role="alert">{offeringError}</p>}
              </form>
            )}
          </section>
        </>
      ) : null}
    </main>
  );
}
