import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import type { StudentReport } from "../types/api";
import { getStudentReport } from "../services/assessments";
import "./StudentResults.css";

export default function StudentResults() {
  const [report, setReport] = useState<StudentReport | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    let active = true;
    getStudentReport()
      .then((response) => {
        if (active) setReport(response.report);
      })
      .catch((requestError) => {
        if (active) {
          setError(requestError instanceof Error ? requestError.message : "Unable to load your results.");
        }
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, []);

  return (
    <main className="student-results">
      <header className="student-results__header">
        <div>
          <p>Academic record</p>
          <h1>Results and grades</h1>
        </div>
        <Link to="/courses">Back to courses</Link>
      </header>

      {error && <p className="student-results__error" role="alert">{error}</p>}
      {loading ? (
        <p role="status">Loading your academic record...</p>
      ) : report ? (
        <>
          <section className="student-results__summary" aria-label="Academic summary">
            <article><span>Cumulative GPA</span><strong>{report.cumulative_gpa == null ? "—" : report.cumulative_gpa.toFixed(2)}</strong></article>
            <article><span>Programme</span><strong>{report.programme_code || "Not assigned"}</strong></article>
            <article><span>Record status</span><strong>{report.gpa_status.replace(/_/g, " ")}</strong></article>
          </section>
          <section className="student-results__panel">
            <h2>Final results</h2>
            {report.results.length ? (
              <div className="student-results__table-wrap">
                <table>
                  <thead><tr><th>Offering</th><th>Academic year</th><th>Semester</th><th>Mark</th><th>Percentage</th><th>Grade</th></tr></thead>
                  <tbody>
                    {report.results.map((result) => (
                      <tr key={result.offering_id}>
                        <td title={result.offering_id}>{result.offering_id.slice(0, 8)}…</td>
                        <td>{result.academic_year}</td>
                        <td>{result.semester}</td>
                        <td>{result.final_mark} / {result.max_mark}</td>
                        <td>{result.percent.toFixed(1)}%</td>
                        <td>{result.grade_code || "—"}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : <p className="student-results__empty">No final results are available yet.</p>}
          </section>
          <section className="student-results__panel">
            <h2>Released assessment grades</h2>
            {report.component_grades.length ? (
              <div className="student-results__table-wrap">
                <table>
                  <thead><tr><th>Assessment</th><th>Attempt</th><th>Mark</th><th>Weight</th><th>Released</th></tr></thead>
                  <tbody>
                    {report.component_grades.map((grade) => (
                      <tr key={grade.grade_id}>
                        <td title={grade.assessment_id}>{grade.assessment_id.slice(0, 8)}…</td>
                        <td>{grade.attempt_no}</td>
                        <td>{grade.mark} / {grade.max_mark}</td>
                        <td>{grade.weight_percent}%</td>
                        <td>{new Date(grade.released_at).toLocaleDateString()}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : <p className="student-results__empty">No released assessment grades are available yet.</p>}
          </section>
        </>
      ) : null}
    </main>
  );
}
