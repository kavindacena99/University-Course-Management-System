import { useCallback, useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import type { CourseOffering, Enrollment, UserSummary } from "../types/api";
import { getCurrentUser } from "../services/auth";
import { cancelEnrollment, createEnrollment, listStudentEnrollmentHistory } from "../services/enrollments";
import { listPublishedOfferings } from "../services/courses";
import "./StudentEnrollments.css";

export default function StudentEnrollments() {
  const [user, setUser] = useState<UserSummary | null>(null);
  const [offerings, setOfferings] = useState<CourseOffering[]>([]);
  const [enrollments, setEnrollments] = useState<Enrollment[]>([]);
  const [loading, setLoading] = useState(true);
  const [busyOffering, setBusyOffering] = useState("");
  const [busyEnrollment, setBusyEnrollment] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");

  const refresh = useCallback(async (studentId: string) => {
    const [availableResponse, enrollmentResponse] = await Promise.all([
      listPublishedOfferings(),
      listStudentEnrollmentHistory(studentId),
    ]);
    setOfferings(availableResponse.offerings);
    setEnrollments(enrollmentResponse.enrollments);
  }, []);

  useEffect(() => {
    let active = true;
    getCurrentUser()
      .then(async (currentUser) => {
        await refresh(currentUser.id);
        if (active) setUser(currentUser);
      })
      .catch((requestError: unknown) => {
        if (active) {
          setError(requestError instanceof Error ? requestError.message : "Unable to load enrollments.");
        }
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [refresh]);

  const activeOfferings = useMemo(
    () => new Map(offerings.map((offering) => [offering.id, offering])),
    [offerings],
  );
  const activeEnrollmentIds = useMemo(
    () => new Set(enrollments.filter((item) => item.status === "active").map((item) => item.offering_id)),
    [enrollments],
  );
  const availableOfferings = offerings.filter((offering) => !activeEnrollmentIds.has(offering.id));

  async function enroll(offering: CourseOffering) {
    if (!user) return;
    setBusyOffering(offering.id);
    setError("");
    setNotice("");
    try {
      await createEnrollment(offering.id);
      await refresh(user.id);
      setNotice(`Enrolled in ${offering.course_code} successfully.`);
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : "Unable to enroll in this offering.");
    } finally {
      setBusyOffering("");
    }
  }

  async function withdraw(enrollment: Enrollment) {
    if (!user) return;
    setBusyEnrollment(enrollment.id);
    setError("");
    setNotice("");
    try {
      await cancelEnrollment(enrollment);
      await refresh(user.id);
      setNotice("Enrollment cancelled.");
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : "Unable to cancel this enrollment.");
    } finally {
      setBusyEnrollment("");
    }
  }

  return (
    <main className="student-enrollments">
      <header className="student-enrollments__header">
        <div>
          <p>Student portal</p>
          <h1>Course enrollment</h1>
          <span>{user ? `${user.first_name} ${user.last_name}` : ""}</span>
        </div>
        <nav aria-label="Student navigation">
          <Link to="/courses">My courses</Link>
          <Link to="/student/assignments">Assignments</Link>
          <Link to="/student/results">Results</Link>
        </nav>
      </header>

      {error && <p className="student-enrollments__message student-enrollments__message--error" role="alert">{error}</p>}
      {notice && <p className="student-enrollments__message" role="status">{notice}</p>}

      {loading ? (
        <p role="status">Loading course offerings...</p>
      ) : (
        <>
          <section aria-labelledby="enrolled-title">
            <h2 id="enrolled-title">Your enrollments</h2>
            {enrollments.length ? (
              <div className="student-enrollments__list">
                {enrollments.map((enrollment) => {
                  const offering = activeOfferings.get(enrollment.offering_id);
                  return (
                    <article key={enrollment.id}>
                      <div>
                        <strong>{offering ? `${offering.course_code} - ${offering.course_title}` : "Course offering"}</strong>
                        <span>
                          {offering
                            ? `${offering.academic_year} · ${offering.semester}`
                            : enrollment.offering_id}
                        </span>
                      </div>
                      <span className={`student-enrollments__status student-enrollments__status--${enrollment.status}`}>
                        {enrollment.status}
                      </span>
                      {enrollment.status === "active" && (
                        <button
                          type="button"
                          disabled={busyEnrollment === enrollment.id}
                          onClick={() => void withdraw(enrollment)}
                        >
                          {busyEnrollment === enrollment.id ? "Cancelling..." : "Cancel enrollment"}
                        </button>
                      )}
                    </article>
                  );
                })}
              </div>
            ) : <p>No course enrollments yet.</p>}
          </section>

          <section aria-labelledby="available-title">
            <h2 id="available-title">Available offerings</h2>
            {availableOfferings.length ? (
              <div className="student-enrollments__list">
                {availableOfferings.map((offering) => (
                  <article key={offering.id}>
                    <div>
                      <strong>{offering.course_code} - {offering.course_title}</strong>
                      <span>{offering.academic_year} · {offering.semester}</span>
                      <span>{offering.start_date} to {offering.end_date}</span>
                    </div>
                    <button
                      type="button"
                      disabled={busyOffering === offering.id}
                      onClick={() => void enroll(offering)}
                    >
                      {busyOffering === offering.id ? "Enrolling..." : "Enroll"}
                    </button>
                  </article>
                ))}
              </div>
            ) : <p>No published course offerings are currently available.</p>}
          </section>
        </>
      )}
    </main>
  );
}
