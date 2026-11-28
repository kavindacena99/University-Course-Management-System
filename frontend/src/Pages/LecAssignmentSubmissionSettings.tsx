import { FormEvent, useEffect, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import type { CourseOffering } from "../types/api";
import { createAssessment, listAssignedOfferings } from "../services/assessments";
import "./LecAssignmentSubmissionSettings.css";

export default function LecAssignmentSubmissionSettings() {
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const [offerings, setOfferings] = useState<CourseOffering[]>([]);
  const [offeringId, setOfferingId] = useState(searchParams.get("offeringId") ?? "");
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [componentKind, setComponentKind] = useState<"ca" | "final_exam" | "other">("ca");
  const [dueAt, setDueAt] = useState("");
  const [maxMark, setMaxMark] = useState("100");
  const [weightPercent, setWeightPercent] = useState("100");
  const [maxAttempts, setMaxAttempts] = useState("1");
  const [latePolicy, setLatePolicy] = useState<"accept" | "reject">("reject");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    let active = true;
    listAssignedOfferings()
      .then((response) => {
        if (!active) return;
        setOfferings(response.offerings);
        const selected = response.offerings.find((item) => item.id === searchParams.get("offeringId"))
          ?? response.offerings[0];
        setOfferingId(selected?.id ?? "");
        if (selected && searchParams.get("offeringId") !== selected.id) {
          setSearchParams({ offeringId: selected.id }, { replace: true });
        }
      })
      .catch((requestError) => {
        if (active) {
          setError(requestError instanceof Error ? requestError.message : "Unable to load course offerings.");
        }
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [searchParams, setSearchParams]);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    if (!offeringId) {
      setError("Select an assigned course offering.");
      return;
    }

    setSaving(true);
    try {
      await createAssessment(offeringId, {
        title: title.trim(),
        description: description.trim(),
        component_kind: componentKind,
        due_at: new Date(dueAt).toISOString(),
        max_mark: Number(maxMark),
        weight_percent: Number(weightPercent),
        max_attempts: Number(maxAttempts),
        late_policy: latePolicy,
        allowed_file_types: [],
      });
      navigate(`/lecturer/assignments?offeringId=${encodeURIComponent(offeringId)}`);
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : "Unable to create assessment.");
    } finally {
      setSaving(false);
    }
  }

  const selectedOffering = offerings.find((item) => item.id === offeringId);

  return (
    <div className="assessment-create">
      <aside className="assessment-create__sidebar">
        <Link className="assessment-create__brand" to="/lecturer/dashboard">
          <span aria-hidden="true">UC</span>
          <strong>University Course<br />Management System</strong>
        </Link>
        <nav aria-label="Lecturer navigation">
          <Link to="/lecturer/dashboard">Dashboard</Link>
          <Link to="/lecturer/courses">My courses</Link>
          <Link className="is-active" to="/lecturer/assignments">Assignments</Link>
        </nav>
      </aside>

      <main className="assessment-create__main">
        <header>
          <p>Assignments</p>
          <h1>Create assessment</h1>
          <p>Set the assessment details supported by the current assessment service.</p>
        </header>

        <form className="assessment-create__form" onSubmit={handleSubmit}>
          {error && <p className="assessment-create__error" role="alert">{error}</p>}
          {loading ? (
            <p role="status">Loading assigned offerings...</p>
          ) : !offerings.length ? (
            <p className="assessment-create__empty">No course offerings are assigned to your account.</p>
          ) : (
            <>
              <label>
                Course offering
                <select
                  onChange={(event) => {
                    setOfferingId(event.target.value);
                    setSearchParams(event.target.value ? { offeringId: event.target.value } : {});
                  }}
                  required
                  value={offeringId}
                >
                  {offerings.map((item) => (
                    <option key={item.id} value={item.id}>
                      {item.course_code} - {item.course_title} ({item.academic_year} {item.semester})
                    </option>
                  ))}
                </select>
              </label>

              {selectedOffering && (
                <p className="assessment-create__context">
                  New assessment for {selectedOffering.course_code} - {selectedOffering.course_title}
                </p>
              )}

              <label>
                Assessment title
                <input maxLength={200} onChange={(event) => setTitle(event.target.value)} required value={title} />
              </label>
              <label>
                Description and instructions
                <textarea maxLength={10000} onChange={(event) => setDescription(event.target.value)} rows={5} value={description} />
              </label>

              <div className="assessment-create__grid">
                <label>
                  Component
                  <select onChange={(event) => setComponentKind(event.target.value as typeof componentKind)} value={componentKind}>
                    <option value="ca">Continuous assessment</option>
                    <option value="final_exam">Final exam</option>
                    <option value="other">Other</option>
                  </select>
                </label>
                <label>
                  Due date and time
                  <input onChange={(event) => setDueAt(event.target.value)} required type="datetime-local" value={dueAt} />
                </label>
                <label>
                  Maximum mark
                  <input max="999999.99" min="0.01" onChange={(event) => setMaxMark(event.target.value)} required step="0.01" type="number" value={maxMark} />
                </label>
                <label>
                  Weight (%)
                  <input max="100" min="0.01" onChange={(event) => setWeightPercent(event.target.value)} required step="0.01" type="number" value={weightPercent} />
                </label>
                <label>
                  Maximum attempts
                  <input max="100" min="1" onChange={(event) => setMaxAttempts(event.target.value)} required step="1" type="number" value={maxAttempts} />
                </label>
                <label>
                  Late submission policy
                  <select onChange={(event) => setLatePolicy(event.target.value as typeof latePolicy)} value={latePolicy}>
                    <option value="reject">Reject late submissions</option>
                    <option value="accept">Accept late submissions</option>
                  </select>
                </label>
              </div>

              <p className="assessment-create__hint">
                New assessments start as drafts. Allowed file types use the service configuration. The backend does not currently support editing an existing assessment.
              </p>
              <div className="assessment-create__actions">
                <Link to={`/lecturer/assignments?offeringId=${encodeURIComponent(offeringId)}`}>Cancel</Link>
                <button disabled={saving} type="submit">{saving ? "Creating..." : "Create draft"}</button>
              </div>
            </>
          )}
        </form>
      </main>
    </div>
  );
}
