import { useMemo, useState } from "react";

const assets = "/assets";

type Course = {
  id: number;
  title: string;
  category: "HND" | "Top Up";
  gradient: string;
};

const courses: Course[] = [
  {
    id: 1,
    title: "HND in Computing - UOK001",
    category: "HND",
    gradient: "linear-gradient(90deg, #a0e1ef, #22577a)",
  },
  {
    id: 2,
    title: "HND in Computing - UOK004",
    category: "HND",
    gradient: "linear-gradient(90deg, #4abb84, #57cc99)",
  },
  {
    id: 3,
    title: "HND in Computing - UOK008",
    category: "HND",
    gradient: "linear-gradient(90deg, #b8ecd5, #57cc99)",
  },
  {
    id: 4,
    title: "HND in Computing - UOK010",
    category: "HND",
    gradient: "linear-gradient(90deg, #988152, #e9d8a6 80%)",
  },
  {
    id: 5,
    title: "HND in Computing - UOK012",
    category: "HND",
    gradient: "linear-gradient(90deg, #c6b49c, #ee9b00)",
  },
  {
    id: 6,
    title: "Top Up (Software) - UOK20",
    category: "Top Up",
    gradient: "linear-gradient(90deg, #cca0a0, #ca6702)",
  },
  {
    id: 7,
    title: "Top Up (Software) - UOK15",
    category: "Top Up",
    gradient: "linear-gradient(90deg, #f1c7c7, #bb3e03)",
  },
  {
    id: 8,
    title: "Top Up (Software) - UOK15",
    category: "Top Up",
    gradient: "linear-gradient(90deg, #c09393, #ae2012)",
  },
];

const navItems = [
  ["Dashboard", "57bc3.png"],
  ["My Courses", "45c82.png"],
  ["Assignments", "2c8cd.png"],
  ["Results and Grades", "1cad0.png"],
  ["Class Analysis", "4ebe4.png"],
  ["Academic Calender", "2e776.png"],
  ["Notice Board", "4c71a.png"],
];

function App() {
  const [query, setQuery] = useState("");
  const [globalQuery, setGlobalQuery] = useState("");
  const [category, setCategory] = useState<"All" | Course["category"]>("All");
  const [sort, setSort] = useState("default");
  const [activeNav, setActiveNav] = useState("My Courses");
  const [menuCourse, setMenuCourse] = useState<number | null>(null);
  const [profileOpen, setProfileOpen] = useState(false);
  const [mobileNavOpen, setMobileNavOpen] = useState(false);
  const [showAdd, setShowAdd] = useState(false);

  const visibleCourses = useMemo(() => {
    const needle = `${query} ${globalQuery}`.trim().toLowerCase();
    const filtered = courses.filter(
      (course) =>
        (category === "All" || course.category === category) &&
        (!needle || course.title.toLowerCase().includes(needle)),
    );
    return sort === "name"
      ? [...filtered].sort((a, b) => a.title.localeCompare(b.title))
      : filtered;
  }, [category, globalQuery, query, sort]);

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
          onClick={() => window.alert("You have been logged out.")}
        >
          <img src={`${assets}/78101.svg`} alt="" />
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
            <img src={`${assets}/bb404.svg`} alt="" />
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
            <img className="heading-rule" src={`${assets}/15af6.svg`} alt="" />

            <div className="filters">
              <label className="select-control select-control--category">
                <span className="sr-only">Course category</span>
                <select
                  value={category}
                  onChange={(event) => setCategory(event.target.value as typeof category)}
                >
                  <option>All</option>
                  <option>HND</option>
                  <option>Top Up</option>
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
              {visibleCourses.map((course) => (
                <article className="course-card" key={course.id}>
                  <button
                    className="card-main"
                    onClick={() => window.alert(`Opening ${course.title}`)}
                    type="button"
                  >
                    <span className="course-banner" style={{ background: course.gradient }} />
                    <strong>{course.title}</strong>
                  </button>
                  <button
                    className="course-menu-button"
                    aria-label={`More options for ${course.title}`}
                    aria-expanded={menuCourse === course.id}
                    onClick={() => setMenuCourse(menuCourse === course.id ? null : course.id)}
                    type="button"
                  >
                    <img src={`${assets}/1779c.png`} alt="" />
                  </button>
                  {menuCourse === course.id && (
                    <div className="course-menu">
                      <button type="button">View course</button>
                      <button type="button">Edit details</button>
                    </div>
                  )}
                </article>
              ))}
              {visibleCourses.length === 0 && (
                <p className="empty-state">No courses match your search.</p>
              )}
            </div>

            <button
              className="add-course"
              onClick={() => setShowAdd(true)}
              aria-label="Add a course"
              type="button"
            >
              <img src={`${assets}/463d6.png`} alt="" />
            </button>
          </section>
        ) : (
          <section className="placeholder-panel">
            <h2>{activeNav}</h2>
            <p>Select “My Courses” from the navigation to return to the course overview.</p>
          </section>
        )}
      </main>

      {showAdd && (
        <div className="modal-backdrop" role="presentation" onMouseDown={() => setShowAdd(false)}>
          <div
            className="modal"
            role="dialog"
            aria-modal="true"
            aria-labelledby="add-title"
            onMouseDown={(event) => event.stopPropagation()}
          >
            <button
              className="modal-close"
              aria-label="Close"
              type="button"
              onClick={() => setShowAdd(false)}
            >
              ×
            </button>
            <h2 id="add-title">Add a new course</h2>
            <p>Enter the course details to add it to your overview.</p>
            <label>
              Course name
              <input placeholder="e.g. HND in Computing" />
            </label>
            <label>
              Course code
              <input placeholder="e.g. UOK021" />
            </label>
            <button className="primary-button" type="button" onClick={() => setShowAdd(false)}>
              Add course
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

export default App;
