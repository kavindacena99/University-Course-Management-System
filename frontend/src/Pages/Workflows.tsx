import { useEffect, useState, type FormEvent, type ReactNode } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { useSession, hasPermission, hasRole } from "../session";
import * as api from "../services/workflows";
import type { Course } from "../types/api";
import "./Workflows.css";

function message(error: unknown) {
  return error instanceof Error ? error.message : "The request failed.";
}
function useLoad<T>(
  load: () => Promise<T>,
  keys: unknown[],
): { data: T | null; error: string; loading: boolean; reload: () => void } {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [revision, reload] = useState(0);
  // Route IDs and offsets are the only values used by callers.
  useEffect(() => {
    let active = true;
    setLoading(true);
    setError("");
    load()
      .then((value) => {
        if (active) setData(value);
      })
      .catch((cause) => {
        if (active) setError(message(cause));
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [...keys, revision]);
  return { data, error, loading, reload: () => reload((value) => value + 1) };
}
function Shell({ title, children }: { title: string; children: ReactNode }) {
  const { user, signOut } = useSession();
  const navigate = useNavigate();
  const [logoutError, setLogoutError] = useState("");
  return (
    <div className="workflow-shell">
      <header>
        <Link to="/courses" className="workflow-brand">
          University Course Management
        </Link>
        <nav aria-label="Primary navigation">
          <Link to="/courses">Courses</Link>
          <Link to="/offerings">Offerings</Link>
          {user && hasRole(user, "STUDENT") && (
            <>
              <Link to="/student/enrollments">My enrolments</Link>
              <Link to="/student/report">Academic report</Link>
            </>
          )}
          {user && hasRole(user, "LECTURER") && (
            <Link to="/lecturer/offerings">Assigned offerings</Link>
          )}
          {user && hasRole(user, "ADMIN") && <Link to="/admin/dashboard">Administration</Link>}
        </nav>
        <span>
          {user?.first_name} {user?.last_name}
        </span>
        <button
          onClick={async () => {
            try {
              await signOut();
              navigate("/login", { replace: true });
            } catch (cause) {
              setLogoutError(message(cause));
            }
          }}
        >
          Log out
        </button>
      </header>
      <main>
        {logoutError && <p role="alert">Log out failed: {logoutError}</p>}
        <h1>{title}</h1>
        {children}
      </main>
    </div>
  );
}
function Status({ loading, error }: { loading: boolean; error: string }) {
  return (
    <>
      {loading && <p role="status">Loading…</p>}
      {error && <p role="alert">{error}</p>}
    </>
  );
}
function Pager({
  offset,
  total,
  setOffset,
}: {
  offset: number;
  total: number;
  setOffset: (value: number) => void;
}) {
  return (
    <div className="workflow-pager">
      <button disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - 20))}>
        Previous
      </button>
      <span>
        {total === 0 ? "0 results" : `${offset + 1}–${Math.min(total, offset + 20)} of ${total}`}
      </span>
      <button disabled={offset + 20 >= total} onClick={() => setOffset(offset + 20)}>
        Next
      </button>
    </div>
  );
}

export function CoursesPage() {
  const [offset, setOffset] = useState(0);
  const { data, error, loading } = useLoad(() => api.allCourses(offset), [offset]);
  const { user } = useSession();
  return (
    <Shell title="Courses">
      <Status loading={loading} error={error} />
      {user && hasPermission(user, "courses:manage") && (
        <p>
          <Link to="/admin/courses/create">Create course</Link>
        </p>
      )}
      {data && (
        <>
          <ul className="workflow-list">
            {data.courses.map((course: Course) => (
              <li key={course.id}>
                <Link to={`/courses/${course.id}`}>
                  {course.code} · {course.title}
                </Link>
                <span>
                  {course.status} · {course.credits} credits
                </span>
              </li>
            ))}
          </ul>
          {!data.total && <p>No courses are available.</p>}
          <Pager offset={offset} total={data.total} setOffset={setOffset} />
        </>
      )}
    </Shell>
  );
}
export function CoursePage() {
  const { id = "" } = useParams();
  const details = useLoad(() => api.course(id), [id]);
  const [offset, setOffset] = useState(0);
  const offerings = useLoad(() => api.courseOfferings(id, offset), [id, offset]);
  return (
    <Shell title={details.data?.course.title || "Course"}>
      <Status
        loading={details.loading || offerings.loading}
        error={details.error || offerings.error}
      />
      {details.data && (
        <p>
          {details.data.course.code} · {details.data.course.description}
        </p>
      )}
      {offerings.data && (
        <>
          <h2>Offerings</h2>
          <ul className="workflow-list">
            {offerings.data.offerings.map((item) => (
              <li key={item.id}>
                <Link to={`/offerings/${item.id}`}>
                  {item.academic_year} · {item.semester}
                </Link>
                <span>{item.status}</span>
              </li>
            ))}
          </ul>
          {!offerings.data.total && <p>No offerings are available.</p>}
          <Pager offset={offset} total={offerings.data.total} setOffset={setOffset} />
        </>
      )}
    </Shell>
  );
}
export function CreateCoursePage() {
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const navigate = useNavigate();
  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    setBusy(true);
    setError("");
    try {
      const result = await import("../services/courses").then((module) =>
        module.createCourse({
          code: String(form.get("code")).trim(),
          title: String(form.get("title")).trim(),
          description: String(form.get("description")).trim(),
          credits: Number(form.get("credits")),
        }),
      );
      navigate(`/courses/${result.course.id}`);
    } catch (cause) {
      setError(message(cause));
    } finally {
      setBusy(false);
    }
  }
  return (
    <Shell title="Create course">
      <form onSubmit={save} className="workflow-form">
        <label>
          Code
          <input name="code" required minLength={2} maxLength={32} pattern="[A-Za-z0-9_-]+" />
        </label>
        <label>
          Title
          <input name="title" required maxLength={200} />
        </label>
        <label>
          Description
          <textarea name="description" maxLength={5000} />
        </label>
        <label>
          Credits
          <input name="credits" type="number" required min="1" max="30" />
        </label>
        <button disabled={busy}>{busy ? "Saving…" : "Create course"}</button>
        {error && <p role="alert">{error}</p>}
      </form>
    </Shell>
  );
}
export function LecturerOfferingsPage() {
  const [offset, setOffset] = useState(0);
  const { data, error, loading } = useLoad(() => api.offerings(true, offset), [offset]);
  return (
    <Shell title="My assigned offerings">
      <Status loading={loading} error={error} />
      {data && (
        <>
          <ul className="workflow-list">
            {data.offerings.map((item) => (
              <li key={item.id}>
                <Link to={`/lecturer/offerings/${item.id}`}>
                  {item.course_code} · {item.course_title}
                </Link>
                <span>
                  {item.academic_year} · {item.semester}
                </span>
              </li>
            ))}
          </ul>
          {!data.total && <p>No offerings are assigned to you.</p>}
          <Pager offset={offset} total={data.total} setOffset={setOffset} />
        </>
      )}
    </Shell>
  );
}
export function OfferingsPage() {
  const [offset, setOffset] = useState(0);
  const { data, error, loading } = useLoad(() => api.offerings(false, offset), [offset]);
  return (
    <Shell title="Offerings">
      <Status loading={loading} error={error} />
      {data && (
        <>
          <ul className="workflow-list">
            {data.offerings.map((item) => (
              <li key={item.id}>
                <Link to={`/offerings/${item.id}`}>
                  {item.course_code} · {item.course_title}
                </Link>
                <span>
                  {item.academic_year} · {item.semester} · {item.status}
                </span>
              </li>
            ))}
          </ul>
          {!data.total && <p>No offerings are available.</p>}
          <Pager offset={offset} total={data.total} setOffset={setOffset} />
        </>
      )}
    </Shell>
  );
}
function EnrollmentItem({ item }: { item: api.Enrollment }) {
  const { data } = useLoad(() => api.offering(item.offering_id), [item.offering_id]);
  return (
    <li>
      <Link to={`/offerings/${item.offering_id}`}>
        {data?.offering.course_code || item.offering_id} ·{" "}
        {data?.offering.course_title || "Offering"}
      </Link>
      <span>{item.status}</span>
    </li>
  );
}
export function StudentEnrollmentsPage() {
  const { user } = useSession();
  const [offset, setOffset] = useState(0);
  const { data, error, loading } = useLoad(
    () => api.enrollments(user!.id, offset),
    [user?.id, offset],
  );
  return (
    <Shell title="My enrolments">
      <Status loading={loading} error={error} />
      {data && (
        <>
          <ul className="workflow-list">
            {data.enrollments.map((item) => (
              <EnrollmentItem key={item.id} item={item} />
            ))}
          </ul>
          {!data.total && <p>You have no enrolments.</p>}
          <Pager offset={offset} total={data.total} setOffset={setOffset} />
        </>
      )}
    </Shell>
  );
}
export function OfferingPage({ lecturer = false }: { lecturer?: boolean }) {
  const { id = "" } = useParams();
  const { user } = useSession();
  const [notice, setNotice] = useState("");
  const [offset, setOffset] = useState(0);
  const details = useLoad(() => api.offering(id), [id]);
  const assessmentList = useLoad(() => api.assessments(id, offset), [id, offset]);
  const enrolmentList = useLoad(
    () => (lecturer ? api.offeringEnrollments(id) : Promise.resolve(null)),
    [id, lecturer],
  );
  const assigned =
    !lecturer ||
    details.data?.offering.lecturer_user_id === user?.id ||
    (!!user && hasRole(user, "ADMIN"));
  async function enrol() {
    try {
      await api.enroll(id, user!.id);
      setNotice("Enrolment saved.");
      assessmentList.reload();
    } catch (cause) {
      setNotice(message(cause));
    }
  }
  return (
    <Shell title={details.data?.offering.course_title || "Offering"}>
      <Status
        loading={details.loading || assessmentList.loading}
        error={details.error || assessmentList.error}
      />
      {details.data && !assigned && <p role="alert">This offering is not assigned to you.</p>}
      {assigned && details.data && (
        <>
          <p>
            {details.data.offering.course_code} · {details.data.offering.academic_year} ·{" "}
            {details.data.offering.semester} · {details.data.offering.status}
          </p>
          {!lecturer && user && hasRole(user, "STUDENT") && (
            <button onClick={enrol}>Enrol in offering</button>
          )}
          {lecturer && (
            <>
              {user && hasPermission(user, "analytics:lecturer:read") && (
                <Link to={`/lecturer/offerings/${id}/analytics`}>Offering analytics</Link>
              )}
              <h2>Enrolments</h2>
              <Status loading={enrolmentList.loading} error={enrolmentList.error} />
              <p>{enrolmentList.data?.total ?? "—"} enrolled records</p>
            </>
          )}
          {notice && <p role="status">{notice}</p>}
          <h2>Assessments</h2>
          {lecturer && user && hasPermission(user, "assessments:manage") && (
            <p>
              <Link to={`/lecturer/offerings/${id}/assessments/create`}>Create assessment</Link>
            </p>
          )}
          {assessmentList.data && (
            <>
              <ul className="workflow-list">
                {assessmentList.data.assessments.map((item) => (
                  <li key={item.id}>
                    <Link
                      to={
                        lecturer
                          ? `/lecturer/assessments/${item.id}`
                          : `/student/assessments/${item.id}`
                      }
                    >
                      {item.title}
                    </Link>
                    <span>
                      {item.status} · due {new Date(item.due_at).toLocaleDateString()}
                    </span>
                  </li>
                ))}
              </ul>
              {!assessmentList.data.total && <p>No assessments are available.</p>}
              <Pager offset={offset} total={assessmentList.data.total} setOffset={setOffset} />
            </>
          )}
        </>
      )}
    </Shell>
  );
}
export function CreateAssessmentPage() {
  const { id = "" } = useParams();
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const navigate = useNavigate();
  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    setBusy(true);
    setError("");
    try {
      const result = await api.createAssessment(id, {
        title: String(form.get("title")).trim(),
        description: String(form.get("description")).trim(),
        component_kind: String(form.get("component_kind")),
        due_at: new Date(String(form.get("due_at"))).toISOString(),
        max_mark: Number(form.get("max_mark")),
        weight_percent: Number(form.get("weight_percent")),
        max_attempts: Number(form.get("max_attempts")),
        late_policy: String(form.get("late_policy")),
        allowed_file_types: [{ extension: "pdf", media_type: "application/pdf" }],
      });
      navigate(`/lecturer/assessments/${result.assessment.id}`);
    } catch (cause) {
      setError(message(cause));
    } finally {
      setBusy(false);
    }
  }
  return (
    <Shell title="Create assessment">
      <form onSubmit={save} className="workflow-form">
        <label>
          Title
          <input name="title" required maxLength={200} />
        </label>
        <label>
          Description
          <textarea name="description" maxLength={10000} />
        </label>
        <label>
          Component kind
          <select name="component_kind">
            <option value="ca">Continuous assessment</option>
            <option value="final_exam">Final exam</option>
            <option value="other">Other</option>
          </select>
        </label>
        <label>
          Due date
          <input name="due_at" type="datetime-local" required />
        </label>
        <label>
          Maximum mark
          <input name="max_mark" type="number" min="0.01" step="0.01" required />
        </label>
        <label>
          Weight percent
          <input name="weight_percent" type="number" min="0.01" max="100" step="0.01" required />
        </label>
        <label>
          Maximum attempts
          <input name="max_attempts" type="number" min="1" max="100" defaultValue="1" required />
        </label>
        <label>
          Late policy
          <select name="late_policy">
            <option value="reject">Reject late submissions</option>
            <option value="accept">Accept late submissions</option>
          </select>
        </label>
        <p>Accepted file type: PDF</p>
        <button disabled={busy}>{busy ? "Saving…" : "Create draft"}</button>
        {error && <p role="alert">{error}</p>}
      </form>
    </Shell>
  );
}
export function AssessmentPage({ lecturer = false }: { lecturer?: boolean }) {
  const { id = "" } = useParams();
  const { user } = useSession();
  const navigate = useNavigate();
  const [notice, setNotice] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [offset, setOffset] = useState(0);
  const details = useLoad(() => api.assessment(id), [id]);
  const list = useLoad(
    () => (lecturer ? api.submissions(id, offset) : Promise.resolve(null)),
    [id, offset, lecturer],
  );
  async function publish() {
    try {
      await api.publishAssessment(id, details.data!.assessment.version);
      setNotice("Assessment published.");
      details.reload();
    } catch (cause) {
      setNotice(message(cause));
    }
  }
  async function upload() {
    if (!file) return;
    if (file.size < 1 || file.size > 20 * 1024 * 1024) {
      setNotice("Choose a nonempty file no larger than 20 MiB.");
      return;
    }
    const allowed = details.data?.assessment.allowed_file_types || [];
    if (
      allowed.length &&
      !allowed.some((item) => file.name.toLowerCase().endsWith(`.${item.extension}`))
    ) {
      setNotice("This file extension is not allowed for the assessment.");
      return;
    }
    try {
      const result = await api.submit(id, file);
      navigate(`/student/submissions/${result.submission.id}`);
    } catch (cause) {
      setNotice(message(cause));
    }
  }
  return (
    <Shell title={details.data?.assessment.title || "Assessment"}>
      <Status loading={details.loading || list.loading} error={details.error || list.error} />
      {details.data && (
        <>
          <p>{details.data.assessment.description}</p>
          <p>
            Status: {details.data.assessment.status} · due{" "}
            {new Date(details.data.assessment.due_at).toLocaleString()} · maximum mark{" "}
            {details.data.assessment.max_mark}
          </p>
          {lecturer &&
            user &&
            hasPermission(user, "assessments:manage") &&
            details.data.assessment.status === "draft" && (
              <button onClick={publish}>Publish assessment</button>
            )}
          {!lecturer && details.data.assessment.status === "published" && (
            <div className="workflow-form">
              <label>
                Submission file
                <input
                  type="file"
                  accept={details.data.assessment.allowed_file_types
                    .map((item) => `.${item.extension}`)
                    .join(",")}
                  onChange={(event) => setFile(event.target.files?.[0] || null)}
                />
              </label>
              <button disabled={!file} onClick={upload}>
                Upload submission
              </button>
            </div>
          )}
          {lecturer && list.data && (
            <>
              <h2>Submissions</h2>
              <ul className="workflow-list">
                {list.data.submissions.map((item) => (
                  <li key={item.id}>
                    <Link to={`/lecturer/submissions/${item.id}`}>{item.original_filename}</Link>
                    <span>
                      Student {item.student_user_id} · {item.status}
                    </span>
                  </li>
                ))}
              </ul>
              {!list.data.total && <p>No submissions yet.</p>}
              <Pager offset={offset} total={list.data.total} setOffset={setOffset} />
            </>
          )}
        </>
      )}
      {notice && <p role="status">{notice}</p>}
      {user && !lecturer && (
        <p>Only your own released grade is visible through the submission record.</p>
      )}
    </Shell>
  );
}
export function SubmissionPage() {
  const { id = "" } = useParams();
  const { user } = useSession();
  const details = useLoad(() => api.submission(id), [id]);
  const [mark, setMark] = useState("");
  const [feedback, setFeedback] = useState("");
  const [notice, setNotice] = useState("");
  async function download() {
    try {
      const blob = await api.download(id);
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = details.data?.submission.original_filename || "submission";
      link.click();
      URL.revokeObjectURL(url);
    } catch (cause) {
      setNotice(message(cause));
    }
  }
  async function grade() {
    try {
      await api.saveGrade(id, details.data?.grade?.version || 0, Number(mark), feedback);
      setNotice("Grade saved.");
      details.reload();
    } catch (cause) {
      setNotice(message(cause));
    }
  }
  async function release() {
    try {
      await api.releaseGrade(id, details.data!.grade!.version);
      setNotice("Grade released.");
      details.reload();
    } catch (cause) {
      setNotice(message(cause));
    }
  }
  return (
    <Shell title="Submission">
      <Status loading={details.loading} error={details.error} />
      {details.data && (
        <>
          <p>
            {details.data.submission.original_filename} · {details.data.submission.size_bytes} bytes
          </p>
          <button onClick={download}>Download protected file</button>
          <form
            onSubmit={(event) => {
              event.preventDefault();
              grade();
            }}
            className="workflow-form"
          >
            <label>
              Mark
              <input
                type="number"
                min="0"
                step="0.01"
                value={mark}
                onChange={(event) => setMark(event.target.value)}
                required
              />
            </label>
            <label>
              Feedback
              <textarea value={feedback} onChange={(event) => setFeedback(event.target.value)} />
            </label>
            <button>Save grade</button>
          </form>
          {details.data.grade && (
            <>
              <p>
                Saved mark: {details.data.grade.mark} · {details.data.grade.status}
              </p>
              {details.data.grade.status !== "released" &&
                user &&
                hasPermission(user, "assessments:release") && (
                  <button onClick={release}>Release grade</button>
                )}
            </>
          )}
        </>
      )}
      {notice && <p role="status">{notice}</p>}
    </Shell>
  );
}
export function StudentSubmissionPage() {
  const { id = "" } = useParams();
  const details = useLoad(() => api.submission(id), [id]);
  return (
    <Shell title="My submission">
      <Status loading={details.loading} error={details.error} />
      {details.data && (
        <>
          <p>
            {details.data.submission.original_filename} · {details.data.submission.status}
          </p>
          <p>
            Grade:{" "}
            {details.data.grade
              ? `${details.data.grade.mark} · ${details.data.grade.feedback}`
              : "Not released"}
          </p>
          <Link to={`/student/assessments/${details.data.submission.assessment_id}`}>
            Back to assessment
          </Link>
        </>
      )}
    </Shell>
  );
}
export function StudentReportPage() {
  const { data, error, loading } = useLoad(api.studentReport, []);
  return (
    <Shell title="Academic report">
      <Status loading={loading} error={error} />
      {data && (
        <>
          <p>GPA status: {data.report.gpa_status}</p>
          <p>Cumulative GPA: {data.report.cumulative_gpa ?? "Not available"}</p>
          <h2>Results</h2>
          <ul className="workflow-list">
            {data.report.results.map((item) => (
              <li key={item.offering_id}>
                {item.offering_id} · {item.percent}% · {item.grade_code || "Unclassified"}
              </li>
            ))}
          </ul>
          {!data.report.results.length && <p>No final results are available.</p>}
        </>
      )}
    </Shell>
  );
}
export function LecturerAnalyticsPage() {
  const { id = "" } = useParams();
  const { data, error, loading } = useLoad(() => api.lecturerReport(id), [id]);
  return (
    <Shell title="Offering analytics">
      <Status loading={loading} error={error} />
      {data && (
        <>
          <p>{data.report.total_results} final results</p>
          <ul className="workflow-list">
            {data.report.results.map((item) => (
              <li key={item.student_user_id}>
                {item.student_user_id} · {item.percent}% · {item.grade_code || "Unclassified"}
              </li>
            ))}
          </ul>
          {!data.report.total_results && <p>No projected results are available.</p>}
        </>
      )}
    </Shell>
  );
}
export function AdminPage() {
  const { user } = useSession();
  const { data, error, loading } = useLoad(api.adminSummary, []);
  return (
    <Shell title="Administration">
      <Status loading={loading} error={error} />
      {data && (
        <>
          <p>{data.summary.total_results} final results</p>
          <p>
            Average:{" "}
            {data.summary.average_percent == null
              ? "Not available"
              : `${data.summary.average_percent}%`}
          </p>
          <ul className="workflow-list">
            {data.summary.grade_distribution.map((item) => (
              <li key={item.grade_code}>
                {item.grade_code}: {item.count}
              </li>
            ))}
          </ul>
          {user && hasPermission(user, "courses:manage") && (
            <Link to="/admin/courses/create">Create course</Link>
          )}
        </>
      )}
    </Shell>
  );
}
