import { useCallback, useEffect, useMemo, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import type { Assessment, CourseOffering } from "../types/api";
import {
  changeAssessmentStatus,
  listAssessments,
  listAssignedOfferings,
} from "../services/assessments";
import { logout } from "../services/auth";
import "./LecModuleAsssignment.css";

export default function LecModuleAssignment() {
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const requestedOfferingId = searchParams.get("offeringId");
  const [offerings, setOfferings] = useState<CourseOffering[]>([]);
  const [offeringId, setOfferingId] = useState(requestedOfferingId ?? "");
  const [assessments, setAssessments] = useState<Assessment[]>([]);
  const [query, setQuery] = useState("");
  const [loading, setLoading] = useState(true);
  const [actionId, setActionId] = useState("");
  const [error, setError] = useState("");

  const loadAssessments = useCallback(async (id: string) => {
    if (!id) {
      setAssessments([]);
      return;
    }
    const response = await listAssessments(id);
    setAssessments(response.assessments);
  }, []);

  useEffect(() => {
    let active = true;
    listAssignedOfferings()
      .then(async (response) => {
        if (!active) return;
        const selected =
          response.offerings.find((item) => item.id === requestedOfferingId) ??
          response.offerings[0];
        setOfferings(response.offerings);
        setOfferingId(selected?.id ?? "");
        if (selected) {
          if (requestedOfferingId !== selected.id) {
            setSearchParams({ offeringId: selected.id }, { replace: true });
          }
          const result = await listAssessments(selected.id);
          if (active) setAssessments(result.assessments);
        }
      })
      .catch((requestError) => {
        if (active) {
          setError(
            requestError instanceof Error ? requestError.message : "Unable to load assessments.",
          );
        }
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [requestedOfferingId, setSearchParams]);

  const visibleAssessments = useMemo(() => {
    const needle = query.trim().toLowerCase();
    return assessments.filter((item) =>
      `${item.title} ${item.description} ${item.component_kind} ${item.status}`
        .toLowerCase()
        .includes(needle),
    );
  }, [assessments, query]);

  async function selectOffering(id: string) {
    setOfferingId(id);
    setSearchParams(id ? { offeringId: id } : {});
    setLoading(true);
    setError("");
    try {
      await loadAssessments(id);
    } catch (requestError) {
      setError(
        requestError instanceof Error ? requestError.message : "Unable to load assessments.",
      );
    } finally {
      setLoading(false);
    }
  }

  async function updateStatus(assessment: Assessment, action: "publish" | "close") {
    setActionId(assessment.id);
    setError("");
    try {
      await changeAssessmentStatus(assessment, action);
      await loadAssessments(offeringId);
    } catch (requestError) {
      setError(
        requestError instanceof Error ? requestError.message : "Unable to update assessment.",
      );
    } finally {
      setActionId("");
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

  const selectedOffering = offerings.find((item) => item.id === offeringId);

  return (
    <div className="lecturer-assessments">
      <aside className="lecturer-assessments__sidebar">
        <Link className="lecturer-assessments__brand" to="/lecturer/dashboard">
          <span aria-hidden="true">UC</span>
          <strong>
            University Course
            <br />
            Management System
          </strong>
        </Link>
        <nav aria-label="Lecturer navigation">
          <Link to="/lecturer/dashboard">Dashboard</Link>
          <Link to="/lecturer/courses">My courses</Link>
          <Link className="is-active" to="/lecturer/assignments">
            Assignments
          </Link>
        </nav>
        <button className="lecturer-assessments__logout" onClick={signOut} type="button">
          Sign out
        </button>
      </aside>

      <main className="lecturer-assessments__main">
        <header className="lecturer-assessments__header">
          <div>
            <p>Teaching</p>
            <h1>Assessments</h1>
          </div>
          <button
            className="lecturer-assessments__primary"
            disabled={!offeringId}
            onClick={() => navigate(`/lecturer/assessments/create?offeringId=${encodeURIComponent(offeringId)}`)}
            type="button"
          >
            Create assessment
          </button>
        </header>

        <section className="lecturer-assessments__panel">
          {error && (
            <p className="lecturer-assessments__error" role="alert">
              {error}
            </p>
          )}
          <div className="lecturer-assessments__filters">
            <label>
              <span>Course offering</span>
              <select
                disabled={!offerings.length}
                onChange={(event) => void selectOffering(event.target.value)}
                value={offeringId}
              >
                {offerings.map((item) => (
                  <option key={item.id} value={item.id}>
                    {item.course_code} - {item.course_title} ({item.academic_year} {item.semester})
                  </option>
                ))}
              </select>
            </label>
            <label>
              <span>Search assessments</span>
              <input onChange={(event) => setQuery(event.target.value)} value={query} />
            </label>
          </div>

          {selectedOffering && (
            <p className="lecturer-assessments__context">
              Showing assessments for{" "}
              <strong>
                {selectedOffering.course_code} - {selectedOffering.course_title}
              </strong>
            </p>
          )}

          {loading ? (
            <p className="lecturer-assessments__state" role="status">
              Loading assessments...
            </p>
          ) : !offerings.length ? (
            <p className="lecturer-assessments__state">
              No course offerings are assigned to your account.
            </p>
          ) : visibleAssessments.length ? (
            <div className="lecturer-assessments__table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>Assessment</th>
                    <th>Component</th>
                    <th>Due</th>
                    <th>Weight</th>
                    <th>Status</th>
                    <th>Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {visibleAssessments.map((assessment) => (
                    <tr key={assessment.id}>
                      <td>
                        <strong>{assessment.title}</strong>
                        <small>{assessment.description}</small>
                      </td>
                      <td>{assessment.component_kind.replace("_", " ")}</td>
                      <td>{new Date(assessment.due_at).toLocaleString()}</td>
                      <td>{assessment.weight_percent}%</td>
                      <td>
                        <span
                          className={`lecturer-assessments__status lecturer-assessments__status--${assessment.status}`}
                        >
                          {assessment.status}
                        </span>
                      </td>
                      <td className="lecturer-assessments__actions">
                        {assessment.status === "draft" && (
                          <button
                            disabled={actionId === assessment.id}
                            onClick={() => void updateStatus(assessment, "publish")}
                            type="button"
                          >
                            Publish
                          </button>
                        )}
                        {assessment.status === "published" && (
                          <button
                            disabled={actionId === assessment.id}
                            onClick={() => void updateStatus(assessment, "close")}
                            type="button"
                          >
                            Close
                          </button>
                        )}
                        <Link
                          to={`/lecturer/grading?assessmentId=${encodeURIComponent(assessment.id)}`}
                        >
                          Submissions
                        </Link>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            <p className="lecturer-assessments__state">
              No assessments found for this course offering.
            </p>
          )}
        </section>
      </main>
    </div>
  );
}
