import { ChangeEvent, useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import type { Assessment, CourseOffering } from "../types/api";
import {
  getOffering,
  listAssessments,
  listStudentEnrollments,
  submitAssignment,
} from "../services/assessments";
import { getCurrentUser } from "../services/auth";
import "./StudentAssignments.css";

type AssignmentItem = {
  assessment: Assessment;
  offering: CourseOffering;
};

export default function StudentAssignments() {
  const [items, setItems] = useState<AssignmentItem[]>([]);
  const [files, setFiles] = useState<Record<string, File | undefined>>({});
  const [messages, setMessages] = useState<Record<string, string>>({});
  const [query, setQuery] = useState("");
  const [uploadingId, setUploadingId] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    let active = true;
    async function load() {
      try {
        const user = await getCurrentUser();
        const enrollments = await listStudentEnrollments(user.id);
        const activeEnrollments = enrollments.enrollments.filter(
          (enrollment) => enrollment.status === "active",
        );
        const assignmentGroups = await Promise.all(
          activeEnrollments.map(async (enrollment) => {
            const [offeringResponse, assessmentResponse] = await Promise.all([
              getOffering(enrollment.offering_id),
              listAssessments(enrollment.offering_id, "published"),
            ]);
            return assessmentResponse.assessments.map((assessment) => ({
              assessment,
              offering: offeringResponse.offering,
            }));
          }),
        );
        if (active) setItems(assignmentGroups.flat());
      } catch (requestError) {
        if (active) {
          setError(requestError instanceof Error ? requestError.message : "Unable to load your assignments.");
        }
      } finally {
        if (active) setLoading(false);
      }
    }
    void load();
    return () => {
      active = false;
    };
  }, []);

  const visibleItems = useMemo(() => {
    const needle = query.trim().toLowerCase();
    return items.filter(({ assessment, offering }) =>
      `${assessment.title} ${assessment.description} ${offering.course_code} ${offering.course_title}`
        .toLowerCase()
        .includes(needle),
    );
  }, [items, query]);

  function chooseFile(assessmentId: string, event: ChangeEvent<HTMLInputElement>) {
    const file = event.currentTarget.files?.[0];
    setFiles((current) => ({ ...current, [assessmentId]: file }));
    setMessages((current) => ({ ...current, [assessmentId]: "" }));
  }

  async function submit(item: AssignmentItem) {
    const { assessment } = item;
    const file = files[assessment.id];
    if (!file) {
      setMessages((current) => ({ ...current, [assessment.id]: "Choose a file to submit." }));
      return;
    }
    const extension = file.name.split(".").pop()?.toLowerCase() ?? "";
    if (!assessment.allowed_file_types.some((fileType) => fileType.extension === extension)) {
      setMessages((current) => ({
        ...current,
        [assessment.id]: "This file extension is not allowed for the assessment.",
      }));
      return;
    }

    setUploadingId(assessment.id);
    setError("");
    setMessages((current) => ({ ...current, [assessment.id]: "" }));
    try {
      const response = await submitAssignment(assessment.id, file);
      setMessages((current) => ({
        ...current,
        [assessment.id]: `Submitted successfully (attempt ${response.submission.attempt_no}).`,
      }));
      setFiles((current) => ({ ...current, [assessment.id]: undefined }));
    } catch (requestError) {
      setMessages((current) => ({
        ...current,
        [assessment.id]: requestError instanceof Error ? requestError.message : "Unable to submit the file.",
      }));
    } finally {
      setUploadingId("");
    }
  }

  return (
    <main className="student-assignments">
      <header className="student-assignments__header">
        <div>
          <p>Learning</p>
          <h1>Assignments</h1>
        </div>
        <nav>
          <Link to="/courses">My courses</Link>
          <Link to="/student/results">Results and grades</Link>
        </nav>
      </header>

      <label className="student-assignments__search">
        <span className="sr-only">Search assignments</span>
        <input onChange={(event) => setQuery(event.target.value)} placeholder="Search assessments or courses" value={query} />
      </label>

      {error && <p className="student-assignments__error" role="alert">{error}</p>}
      {loading ? (
        <p role="status">Loading your assignments...</p>
      ) : visibleItems.length ? (
        <section className="student-assignments__list" aria-label="Published assessments">
          {visibleItems.map(({ assessment, offering }) => (
            <article key={assessment.id}>
              <div className="student-assignments__details">
                <p>{offering.course_code} · {offering.course_title}</p>
                <h2>{assessment.title}</h2>
                {assessment.description && <p>{assessment.description}</p>}
                <dl>
                  <div><dt>Due</dt><dd>{new Date(assessment.due_at).toLocaleString()}</dd></div>
                  <div><dt>Maximum mark</dt><dd>{assessment.max_mark}</dd></div>
                  <div><dt>Attempts</dt><dd>{assessment.max_attempts}</dd></div>
                  <div><dt>Late policy</dt><dd>{assessment.late_policy === "accept" ? "Late submissions accepted" : "Late submissions rejected"}</dd></div>
                </dl>
              </div>
              <div className="student-assignments__upload">
                <label>
                  Submission file
                  <input
                    accept={assessment.allowed_file_types.map((fileType) => `.${fileType.extension}`).join(",")}
                    onChange={(event) => chooseFile(assessment.id, event)}
                    type="file"
                  />
                </label>
                {files[assessment.id] && <small>{files[assessment.id]?.name}</small>}
                <button
                  disabled={uploadingId === assessment.id}
                  onClick={() => void submit({ assessment, offering })}
                  type="button"
                >
                  {uploadingId === assessment.id ? "Submitting..." : "Submit assignment"}
                </button>
                {messages[assessment.id] && (
                  <p className="student-assignments__message" role="status">{messages[assessment.id]}</p>
                )}
              </div>
            </article>
          ))}
        </section>
      ) : (
        <p className="student-assignments__empty">
          {items.length ? "No assignments match your search." : "No published assignments are available for your active enrollments."}
        </p>
      )}
    </main>
  );
}
