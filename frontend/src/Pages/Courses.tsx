import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import "./Courses.css";
import type { Course, UserSummary } from "../types/api";
import { listCourses } from "../services/courses";
import { getCurrentUser, logout } from "../services/auth";

const assets = "/assets";

export default function Courses() {
  const navigate = useNavigate();
  const [query, setQuery] = useState("");
  const [globalQuery, setGlobalQuery] = useState("");
  const [category, setCategory] = useState<"All" | Course["status"]>("All");
  const [sort, setSort] = useState("default");
  const [activeNav, setActiveNav] = useState("My Courses");
  const [menuCourse, setMenuCourse] = useState<string | null>(null);
  const [profileOpen, setProfileOpen] = useState(false);
  const [mobileNavOpen, setMobileNavOpen] = useState(false);
  const [courses, setCourses] = useState<Course[]>([]);
  const [user, setUser] = useState<UserSummary | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    Promise.all([listCourses(), getCurrentUser()])
      .then(([response, currentUser]) => {
        if (!cancelled) {
          setCourses(response.courses);
          setUser(currentUser);
        }
      })
      .catch((requestError) => {
        if (!cancelled) setError(requestError instanceof Error ? requestError.message : "Unable to load courses.");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const visibleCourses = useMemo(() => {
    const needle = `${query} ${globalQuery}`.trim().toLowerCase();
    const filtered = courses.filter(
      (course) =>
        (category === "All" || course.status === category) &&
        (!needle ||
          course.title.toLowerCase().includes(needle) ||
          course.code.toLowerCase().includes(needle)),
    );
    return sort === "name"
      ? [...filtered].sort((a, b) => a.title.localeCompare(b.title))
      : filtered;
  }, [category, courses, globalQuery, query, sort]);

  function chooseNav(label: string) {
    if (label === "Dashboard") {
      navigate(user?.roles.includes("ADMIN") ? "/admin" : "/lecturer/dashboard");
      return;
    }
    if (label === "Assignments") {
      navigate(user?.roles.includes("STUDENT") ? "/student/assignments" : "/lecturer/assignments");
      return;
    }
    if (label === "Results and Grades") {
      navigate("/student/results");
      return;
    }
    setActiveNav(label);
    setMobileNavOpen(false);
  }

  const navItems = user?.roles.includes("ADMIN")
    ? [["Dashboard", "57bc3.png"], ["My Courses", "45c82.png"]]
    : user?.roles.includes("LECTURER")
      ? [["Dashboard", "57bc3.png"], ["My Courses", "45c82.png"], ["Assignments", "2c8cd.png"]]
      : [["My Courses", "45c82.png"], ["Assignments", "2c8cd.png"], ["Results and Grades", "1cad0.png"]];
  const canCreateCourse = Boolean(user?.permissions?.includes("courses:manage"));

  return (
    <div className="app-shell">
      <aside className={`sidebar ${mobileNavOpen ? "sidebar--open" : ""}`}>
        <div className="logo-card">
          <img src={`${assets}/580ec.png`} alt="" />
          <p>
            University Course
            <br />
            Management System
          </p>
        </div>

        <nav aria-label="Main navigation">
          {navItems.map(([label, icon]) => (
            <button
              className={activeNav === label ? "nav-item nav-item--active" : "nav-item"}
              key={label}
              onClick={() => chooseNav(label)}
              type="button"
            >
              <img src={`${assets}/${icon}`} alt="" />
              <span>{label}</span>
            </button>
          ))}
        </nav>

        <button
          className="logout"
          type="button"
          onClick={async () => {
            try {
              await logout();
            } finally {
              navigate("/login");
            }
          }}
        >
          <img src={`${assets}/394ee.svg`} alt="" />
          <span>LOGOUT</span>
        </button>
      </aside>

      {mobileNavOpen && (
        <button
          className="nav-scrim"
          aria-label="Close navigation"
          onClick={() => setMobileNavOpen(false)}
        />
      )}

      <header className="topbar">
        <button
          className="menu-button"
          aria-label="Open navigation"
          onClick={() => setMobileNavOpen(true)}
          type="button"
        >
          <span />
          <span />
          <span />
        </button>
        <label className="global-search">
          <img src={`${assets}/a02fb.svg`} alt="" />
          <input
            aria-label="Search courses, assignments, results and grades"
            onChange={(event) => setGlobalQuery(event.target.value)}
            placeholder="Search Courses, Assignments, Results and Grades etc..."
            value={globalQuery}
          />
        </label>
        <div className="profile-wrap">
          <button
            className="profile"
            onClick={() => setProfileOpen((open) => !open)}
            type="button"
            aria-expanded={profileOpen}
          >
            <span className="avatar">
              {user ? `${user.first_name[0] ?? ""}${user.last_name[0] ?? ""}`.toUpperCase() : ""}
            </span>
            <span className="profile-copy">
              <strong>{user ? `${user.first_name} ${user.last_name}` : "Account"}</strong>
              <small>{user?.email}</small>
            </span>
            <img src={`${assets}/0d534.svg`} alt="" />
          </button>
          {profileOpen && (
            <div className="profile-menu">
              <button type="button" onClick={() => setProfileOpen(false)}>{user?.email}</button>
            </div>
          )}
        </div>
      </header>

      <main>
        <h1>{activeNav}</h1>
        {activeNav === "My Courses" ? (
          <section className="course-panel" aria-labelledby="overview-title">
            <div className="panel-heading">
              <h2 id="overview-title">Welcome{user?.first_name ? `, ${user.first_name}` : ""}</h2>
              <p>Course Overview</p>
            </div>
            <div className="heading-rule" aria-hidden="true" />

            <div className="filters">
              <label className="select-control select-control--category">
                <span className="sr-only">Course category</span>
                <select
                  value={category}
                  onChange={(event) => setCategory(event.target.value as typeof category)}
                >
                  <option>All</option>
                  <option value="draft">Draft</option>
                  <option value="active">Active</option>
                  <option value="archived">Archived</option>
                </select>
              </label>
              <label className="course-search">
                <img src={`${assets}/a02fb.svg`} alt="" />
                <input
                  aria-label="Search courses"
                  placeholder="Search"
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                />
              </label>
              <label className="select-control sort-control">
                <span className="sr-only">Sort courses</span>
                <select
                  className="font-italic"
                  value={sort}
                  onChange={(event) => setSort(event.target.value)}
                >
                  <option value="default">Sort by course name</option>
                  <option value="name">Course name A–Z</option>
                </select>
              </label>
            </div>

            <div className="course-grid">
              {loading && <p className="empty-state">Loading courses...</p>}
              {error && <p className="empty-state" role="alert">{error}</p>}
              {visibleCourses.map((course) => (
                <article className="course-card" key={course.id}>
                  <button
                    className="card-main"
                    onClick={() => navigate(`/lecturer/courses/view?courseId=${encodeURIComponent(course.id)}`)}
                    type="button"
                  >
                    <span className="course-banner" />
                    <strong>{course.code} - {course.title}</strong>
                  </button>
                  <button
                    className="course-menu-button"
                    aria-label={`More options for ${course.title}`}
                    aria-expanded={menuCourse === course.id}
                    onClick={() => setMenuCourse(menuCourse === course.id ? null : course.id)}
                    type="button"
                  >
                    <span aria-hidden="true">•••</span>
                  </button>
                  {menuCourse === course.id && (
                    <div className="course-menu">
                      <button type="button">View course</button>
                      <button type="button">Edit details</button>
                    </div>
                  )}
                </article>
              ))}
              {!loading && !error && visibleCourses.length === 0 && (
                <p className="empty-state">No courses match your search.</p>
              )}
            </div>

            {canCreateCourse && <button
              className="add-course"
              onClick={() => navigate("/lecturer/courses/create")}
              aria-label="Add a course"
              type="button"
            >
              <span aria-hidden="true">+</span>
            </button>}
          </section>
        ) : (
          <section className="placeholder-panel">
            <h2>{activeNav}</h2>
            <p>Select “My Courses” from the navigation to return to the course overview.</p>
          </section>
        )}
      </main>
    </div>
  );
}