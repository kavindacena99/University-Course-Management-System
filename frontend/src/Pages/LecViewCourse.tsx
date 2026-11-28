import { useEffect, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import type { Course, CourseOffering } from "../types/api";
import { getCourse, listCourseOfferings } from "../services/courses";
import { getCurrentUser } from "../services/auth";
import "./LecViewCourses.css";

export default function LecViewCourse() {
  const [searchParams] = useSearchParams();
  const courseId = searchParams.get("courseId") ?? "";
  const [course, setCourse] = useState<Course | null>(null);
  const [offerings, setOfferings] = useState<CourseOffering[]>([]);
  const [canManageAssessments, setCanManageAssessments] = useState(false);
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
                    {canManageAssessments && (
                      <Link to={`/lecturer/assignments?offeringId=${encodeURIComponent(offering.id)}`}>
                        Assessments
                      </Link>
                    )}
                  </article>
                ))}
              </div>
            ) : <p className="course-details__empty">No offerings are available for this course.</p>}
          </section>
        </>
      ) : null}
    </main>
  );
}
