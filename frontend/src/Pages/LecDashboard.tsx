import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import type { CourseOffering, UserSummary } from "../types/api";
import { listAssignedOfferings } from "../services/assessments";
import { getCurrentUser, logout } from "../services/auth";
import "./LecDashboard.css";

export default function LecDashboard() {
  const navigate = useNavigate();
  const [offerings, setOfferings] = useState<CourseOffering[]>([]);
  const [user, setUser] = useState<UserSummary | null>(null);
  const [query, setQuery] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    let active = true;
    Promise.all([listAssignedOfferings(), getCurrentUser()])
      .then(([offeringResponse, currentUser]) => {
        if (!active) return;
        setOfferings(offeringResponse.offerings);
        setUser(currentUser);
      })
      .catch((requestError) => {
        if (active) {
          setError(requestError instanceof Error ? requestError.message : "Unable to load your dashboard.");
        }
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, []);

  const filteredOfferings = useMemo(() => {
    const needle = query.trim().toLowerCase();
    return offerings.filter((offering) =>
      `${offering.course_code} ${offering.course_title} ${offering.academic_year} ${offering.semester}`
        .toLowerCase()
        .includes(needle),
    );
  }, [offerings, query]);

  async function signOut() {
    try {
      await logout();
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : "Unable to sign out.");
    } finally {
      navigate("/login");
    }
  }

  const displayName = user ? `${user.first_name} ${user.last_name}`.trim() : "Lecturer";

  return (
    <div className="lecturer-home">
      <aside className="lecturer-home__sidebar">
        <Link className="lecturer-home__brand" to="/lecturer/dashboard">
          <span aria-hidden="true">UC</span>
          <strong>University Course<br />Management System</strong>
        </Link>
        <nav aria-label="Lecturer navigation">
          <Link className="is-active" to="/lecturer/dashboard">Dashboard</Link>
          <Link to="/lecturer/courses">My courses</Link>
          <Link to="/lecturer/assignments">Assignments</Link>
        </nav>
        <button className="lecturer-home__logout" onClick={signOut} type="button">Sign out</button>
      </aside>

      <main className="lecturer-home__main">
        <header className="lecturer-home__header">
          <div>
            <p>{new Date().toLocaleDateString(undefined, { weekday: "long", month: "long", day: "numeric" })}</p>
            <h1>Welcome, {displayName}</h1>
          </div>
          <label>
            <span className="sr-only">Search assigned courses</span>
            <input
              onChange={(event) => setQuery(event.target.value)}
              placeholder="Search assigned courses"
              type="search"
              value={query}
            />
          </label>
        </header>

        {error && <p className="lecturer-home__error" role="alert">{error}</p>}
        <section className="lecturer-home__summary" aria-label="Teaching overview">
          <article>
            <span>Assigned course offerings</span>
            <strong>{loading ? "…" : offerings.length}</strong>
          </article>
          <article>
            <span>Published offerings</span>
            <strong>{loading ? "…" : offerings.filter((item) => item.status === "published").length}</strong>
          </article>
        </section>

        <section className="lecturer-home__courses" aria-labelledby="offerings-title">
          <div className="lecturer-home__section-heading">
            <div>
              <h2 id="offerings-title">My course offerings</h2>
              <p>Open an offering to view and manage its assessments.</p>
            </div>
            <Link to="/lecturer/assignments">View assessments</Link>
          </div>
          {loading ? (
            <p role="status">Loading assigned offerings...</p>
          ) : filteredOfferings.length ? (
            <div className="lecturer-home__grid">
              {filteredOfferings.map((offering) => (
                <article key={offering.id}>
                  <div className="lecturer-home__course-top">
                    <span>{offering.course_code}</span>
                    <span className={`lecturer-home__status lecturer-home__status--${offering.status}`}>
                      {offering.status}
                    </span>
                  </div>
                  <h3>{offering.course_title}</h3>
                  <p>{offering.academic_year} · {offering.semester}</p>
                  <Link to={`/lecturer/assignments?offeringId=${encodeURIComponent(offering.id)}`}>
                    Manage assessments
                  </Link>
                </article>
              ))}
            </div>
          ) : (
            <p className="lecturer-home__empty">
              {offerings.length ? "No offerings match your search." : "No course offerings are assigned to your account."}
            </p>
          )}
        </section>
      </main>
    </div>
  );
}
