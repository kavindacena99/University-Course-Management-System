import { useState } from "react";
import "./LecAssignmentSubmission.css";

const assets = "/assets";

const navigation = [
  { label: "Dashboard", icon: "57bc3.png" },
  { label: "My Courses", icon: "45c82.png" },
  { label: "Assignments", icon: "2c8cd.png" },
  { label: "Results and Grades", icon: "1cad0.png" },
  { label: "Class Analysis", icon: "4ebe4.png" },
  { label: "Academic Calender", icon: "2e776.png" },
  { label: "Notice Board", icon: "4c71a.png" },
];

const initialSections = [
  { id: 1, title: "General", items: [] },
  { id: 2, title: "Attendance", items: [] },
  { id: 3, title: "First Semester Course Content", items: [] },
  {
    id: 4,
    title: "First Semester Assignments",
    items: [
      "Programming Assignment",
      "Networking Assignment",
      "Database Design and Development Assignment",
      "Professional Practise Assignment",
    ],
  },
];

export default function LecAssignmentSubmission() {

  const [activeNav, setActiveNav] = useState("My Courses");
  const [activeTab, setActiveTab] = useState("Course");
  const [editMode, setEditMode] = useState(true);
  const [openSections, setOpenSections] = useState(() => new Set([4]));
  const [sections, setSections] = useState(initialSections);
  const [selectedAssignment, setSelectedAssignment] = useState<string | null>(null);
  const [menuOpen, setMenuOpen] = useState(false);

  const toggleSection = (id: number) => {
    setOpenSections((current) => {
      const next = new Set(current);
      next.has(id) ? next.delete(id) : next.add(id);
      return next;
    });
  };

  const addSection = () => {
    if (!editMode) return;
    const id = Math.max(...sections.map((section) => section.id)) + 1;
    setSections((current) => [
      ...current,
      { id, title: `New Course Section ${id - 3}`, items: [] },
    ]);
    setOpenSections((current) => new Set(current).add(id));
  };

  return (
    <div className="app-shell">
      <button
        className="mobile-menu"
        aria-label={menuOpen ? "Close navigation" : "Open navigation"}
        aria-expanded={menuOpen}
        onClick={() => setMenuOpen((open) => !open)}
      >
        <span />
        <span />
        <span />
      </button>

      <aside className={`sidebar ${menuOpen ? "sidebar-open" : ""}`}>
        <div className="brand">
          <img src={`${assets}/580ec.png`} alt="" />
          <p>
            University Course
            <span>Management System</span>
          </p>
        </div>

        <nav aria-label="Main navigation">
          {navigation.map((item) => (
            <button
              key={item.label}
              className={activeNav === item.label ? "nav-active" : ""}
              onClick={() => {
                setActiveNav(item.label);
                setMenuOpen(false);
              }}
            >
              <img src={`${assets}/${item.icon}`} alt="" />
              <span>{item.label}</span>
            </button>
          ))}
        </nav>

        <button className="logout" onClick={() => setActiveNav("")}>
          <img src={`${assets}/c8ef2.svg`} alt="" />
          <span>LOGOUT</span>
        </button>
      </aside>

      {menuOpen && (
        <button
          className="scrim"
          aria-label="Close navigation"
          onClick={() => setMenuOpen(false)}
        />
      )}

      <main>
        <section className="course-card" aria-labelledby="course-title">
          <header className="course-header">
            <div>
              <p className="breadcrumb">School of Computing / HND in Computing/ Cohorts</p>
              <h1 id="course-title">HND in Computing - UOK001</h1>
            </div>
            <button
              className={`edit-toggle ${editMode ? "is-on" : ""}`}
              role="switch"
              aria-checked={editMode}
              onClick={() => setEditMode((mode) => !mode)}
            >
              <span>Edit Mode</span>
              <img src={`${assets}/01f55.png`} alt="" />
            </button>
          </header>

          <div className="tabs" role="tablist" aria-label="Course sections">
            {["Course", "Participants", "Grades", "Reports"].map((tab) => (
              <button
                key={tab}
                role="tab"
                aria-selected={activeTab === tab}
                className={activeTab === tab ? "tab-active" : ""}
                onClick={() => setActiveTab(tab)}
              >
                {tab}
              </button>
            ))}
          </div>

          <div className="section-list">
            {sections.map((section) => {
              const isOpen = openSections.has(section.id);
              return (
                <article
                  className={`course-section ${isOpen ? "section-open" : ""}`}
                  key={section.id}
                >
                  <button
                    className="section-heading"
                    aria-expanded={isOpen}
                    aria-controls={`section-${section.id}`}
                    onClick={() => toggleSection(section.id)}
                  >
                    <img src={`${assets}/2cb9f.png`} alt="" />
                    <span>{section.title}</span>
                  </button>

                  {isOpen && (
                    <div className="section-content" id={`section-${section.id}`}>
                      {section.items.length > 0 ? (
                        section.items.map((item) => (
                          <button
                            key={item}
                            className={selectedAssignment === item ? "assignment-selected" : ""}
                            onClick={() => setSelectedAssignment(item)}
                          >
                            <img src={`${assets}/e34bb.png`} alt="" />
                            <span>{item}</span>
                          </button>
                        ))
                      ) : (
                        <p>
                          {editMode
                            ? "This section is ready for course content."
                            : "No content available."}
                        </p>
                      )}
                    </div>
                  )}
                </article>
              );
            })}
          </div>

          <button className="add-section" disabled={!editMode} onClick={addSection}>
            + Add New Section
          </button>
        </section>
      </main>
    </div>
  );
}
