import { FormEvent, useEffect, useMemo, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import type { Assessment, Submission, SubmissionDetail } from "../types/api";
import {
  downloadSubmission,
  getAssessment,
  getSubmission,
  listSubmissions,
  releaseGrade,
  saveGrade,
} from "../services/assessments";
import { logout } from "../services/auth";
import "./LecAssignmentGrading.css";

const pageSize = 100;

export default function LecAssignmentGrading() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const assessmentId = searchParams.get("assessmentId") ?? "";
  const [assessment, setAssessment] = useState<Assessment | null>(null);
  const [submissions, setSubmissions] = useState<Submission[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [statusFilter, setStatusFilter] = useState<"all" | Submission["status"]>("all");
  const [selected, setSelected] = useState<SubmissionDetail | null>(null);
  const [mark, setMark] = useState("");
  const [feedback, setFeedback] = useState("");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");

  useEffect(() => {
    let active = true;
    if (!assessmentId) {
      setError("Choose an assessment from the assessment list before opening grading.");
      setLoading(false);
      return () => {
        active = false;
      };
    }

    Promise.all([getAssessment(assessmentId), listSubmissions(assessmentId, offset)])
      .then(([assessmentResponse, submissionResponse]) => {
        if (!active) return;
        setAssessment(assessmentResponse.assessment);
        setSubmissions(submissionResponse.submissions);
        setTotal(submissionResponse.total);
      })
      .catch((requestError) => {
        if (active) {
          setError(requestError instanceof Error ? requestError.message : "Unable to load submissions.");
        }
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [assessmentId, offset]);

  const visibleSubmissions = useMemo(
    () => statusFilter === "all"
      ? submissions
      : submissions.filter((submission) => submission.status === statusFilter),
    [statusFilter, submissions],
  );

  async function openSubmission(submissionId: string) {
    setError("");
    try {
      const response = await getSubmission(submissionId);
      setSelected(response);
      setMark(response.grade ? String(response.grade.mark) : "");
      setFeedback(response.grade?.feedback ?? "");
      setMessage("");
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : "Unable to load this submission.");
    }
  }

  async function handleSave(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selected || !assessment) return;
    setError("");
    setSaving(true);
    try {
      const response = await saveGrade(selected.submission.id, {
        version: selected.grade?.version ?? 0,
        mark: Number(mark),
        feedback: feedback.trim(),
      });
      setSelected({ ...selected, grade: response.grade });
      setMessage("Grade saved as a draft.");
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : "Unable to save the grade.");
    } finally {
      setSaving(false);
    }
  }

  async function handleRelease() {
    if (!selected?.grade) return;
    setError("");
    setSaving(true);
    try {
      const response = await releaseGrade(selected.submission.id, selected.grade.version);
      setSelected({ ...selected, grade: response.grade });
      setMessage("Grade released to the student.");
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : "Unable to release the grade.");
    } finally {
      setSaving(false);
    }
  }

  async function downloadFile(submission: Submission) {
    setError("");
    try {
      const blob = await downloadSubmission(submission.id);
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement("a");
      anchor.href = url;
      anchor.download = submission.original_filename || "submission";
      document.body.appendChild(anchor);
      anchor.click();
      anchor.remove();
      window.setTimeout(() => URL.revokeObjectURL(url), 1000);
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : "Unable to download submission.");
    }
  }

  async function signOut() {
    try {
      await logout();
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : "Unable to sign out.");
    } finally {
      navigate("/login");
    }
  }

  return (
    <div className="grading-page">
      <aside className="grading-page__sidebar">
        <Link className="grading-page__brand" to="/lecturer/dashboard">
          <span aria-hidden="true">UC</span>
          <strong>University Course<br />Management System</strong>
        </Link>
        <nav aria-label="Lecturer navigation">
          <Link to="/lecturer/dashboard">Dashboard</Link>
          <Link to="/lecturer/courses">My courses</Link>
          <Link className="is-active" to="/lecturer/assignments">Assignments</Link>
        </nav>
        <button className="grading-page__logout" onClick={signOut} type="button">Sign out</button>
      </aside>

      <main className="grading-page__main">
        <header className="grading-page__header">
          <div>
            <p>Assessment submissions</p>
            <h1>{assessment?.title ?? "Grade submissions"}</h1>
          </div>
          <Link to="/lecturer/assignments">Back to assessments</Link>
        </header>

        {error && <p className="grading-page__error" role="alert">{error}</p>}
        {loading ? (
          <p role="status">Loading submissions...</p>
        ) : assessment ? (
          <section className="grading-page__workspace">
            <div className="grading-page__toolbar">
              <p>
                {assessment.max_mark} marks · Due {new Date(assessment.due_at).toLocaleString()}
              </p>
              <label>
                Status
                <select
                  onChange={(event) => setStatusFilter(event.target.value as typeof statusFilter)}
                  value={statusFilter}
                >
                  <option value="all">All submitted</option>
                  <option value="submitted">On time</option>
                  <option value="late">Late</option>
                </select>
              </label>
            </div>

            <div className="grading-page__table-wrap">
              <table>
                <thead>
                  <tr><th>Student ID</th><th>Attempt</th><th>Status</th><th>File</th><th>Submitted</th><th>Grade</th><th>Actions</th></tr>
                </thead>
                <tbody>
                  {visibleSubmissions.map((submission) => (
                    <tr key={submission.id}>
                      <td title={submission.student_user_id}>{submission.student_user_id.slice(0, 8)}…</td>
                      <td>{submission.attempt_no}</td>
                      <td><span className={`grading-page__status grading-page__status--${submission.status}`}>{submission.status}</span></td>
                      <td>{submission.original_filename}</td>
                      <td>{new Date(submission.submitted_at).toLocaleString()}</td>
                      <td>
                        {selected?.submission.id === submission.id
                          ? selected.grade ? `${selected.grade.mark} / ${assessment.max_mark}` : "Not graded"
                          : "Select to view"}
                      </td>
                      <td className="grading-page__row-actions">
                        <button onClick={() => void openSubmission(submission.id)} type="button">Grade</button>
                        <button className="grading-page__secondary" onClick={() => void downloadFile(submission)} type="button">Download</button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
              {!visibleSubmissions.length && <p className="grading-page__empty">No submissions match this filter.</p>}
            </div>

            <div className="grading-page__pagination">
              <span>Showing {total ? offset + 1 : 0}–{Math.min(offset + pageSize, total)} of {total}</span>
              <div>
                <button disabled={offset === 0 || loading} onClick={() => setOffset(Math.max(0, offset - pageSize))} type="button">Previous</button>
                <button disabled={offset + pageSize >= total || loading} onClick={() => setOffset(offset + pageSize)} type="button">Next</button>
              </div>
            </div>

            {selected && (
              <section className="grading-page__editor" aria-labelledby="grade-editor-title">
                <div className="grading-page__editor-heading">
                  <div>
                    <h2 id="grade-editor-title">Grade submission</h2>
                    <p>Student {selected.submission.student_user_id} · {selected.submission.original_filename}</p>
                  </div>
                  <button type="button" onClick={() => setSelected(null)}>Close</button>
                </div>
                {message && <p className="grading-page__message" role="status">{message}</p>}
                <form onSubmit={handleSave}>
                  <label>
                    Mark (maximum {assessment.max_mark})
                    <input
                      max={assessment.max_mark}
                      min="0"
                      onChange={(event) => setMark(event.target.value)}
                      required
                      step="0.01"
                      type="number"
                      value={mark}
                      disabled={selected.grade?.status === "released"}
                    />
                  </label>
                  <label>
                    Feedback
                    <textarea
                      maxLength={10000}
                      onChange={(event) => setFeedback(event.target.value)}
                      rows={4}
                      value={feedback}
                      disabled={selected.grade?.status === "released"}
                    />
                  </label>
                  {selected.grade?.status === "released" ? (
                    <p className="grading-page__message">This grade has already been released.</p>
                  ) : (
                    <div className="grading-page__editor-actions">
                      <button disabled={saving} type="submit">{saving ? "Saving..." : "Save draft grade"}</button>
                      <button
                        disabled={saving || !selected.grade || selected.grade.status !== "draft"}
                        onClick={() => void handleRelease()}
                        type="button"
                      >
                        Release grade
                      </button>
                    </div>
                  )}
                </form>
              </section>
            )}
          </section>
        ) : null}
      </main>
    </div>
  );
}
