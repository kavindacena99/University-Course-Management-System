import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import "./Courses.css";
import type { Course } from "../types/api";
import { listCourses } from "../services/courses";
import { logout } from "../services/auth";

const assets = "/assets";

const navItems = [
  ["Dashboard", "57bc3.png"],
  ["My Courses", "45c82.png"],
  ["Assignments", "2c8cd.png"],
  ["Results and Grades", "1cad0.png"],
  ["Class Analysis", "4ebe4.png"],
  ["Academic Calender", "2e776.png"],
  ["Notice Board", "4c71a.png"],
];

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
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    listCourses()
      .then((response) => {
        if (!cancelled) setCourses(response.courses);
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
    setActiveNav(label);
    setMobileNavOpen(false);
  }

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
            <span className="avatar">DS</span>
            <span className="profile-copy">
              <strong>Ms Dileeshi Sathsarani</strong>
              <small>Lecturer in IT</small>
            </span>
            <img src={`${assets}/0d534.svg`} alt="" />
          </button>
          {profileOpen && (
            <div className="profile-menu">
              <button type="button">View profile</button>
              <button type="button">Account settings</button>
            </div>
          )}
        </div>
      </header>

      <main>
        <h1>{activeNav}</h1>
        {activeNav === "My Courses" ? (
          <section className="course-panel" aria-labelledby="overview-title">
            <div className="panel-heading">
              <h2 id="overview-title">Hii Dileeshi !</h2>
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
                  <option value="published">Published</option>
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
                    onClick={() => navigate("/lecturer/courses/view")}
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

            <button
              className="add-course"
              onClick={() => navigate("/lecturer/courses/create")}
              aria-label="Add a course"
              type="button"
            >
              <span aria-hidden="true">+</span>
            </button>
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