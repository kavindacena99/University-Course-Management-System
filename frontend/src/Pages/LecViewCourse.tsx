import { FormEvent, useState } from "react";
import "./LecViewCourses.css";

const assets = "/assets";

const navItems = [
  { label: "Dashboard", icon: "57bc3.png" },
  { label: "My Courses", icon: "45c82.png" },
  { label: "Assignments", icon: "2c8cd.png" },
  { label: "Results and Grades", icon: "1cad0.png" },
  { label: "Class Analysis", icon: "4ebe4.png" },
  { label: "Academic Calender", icon: "2e776.png" },
  { label: "Notice Board", icon: "4c71a.png" },
];

const initialSections = [
  "General",
  "Attendance",
  "First Semester Course Content",
  "First Semester Assignments",
];

const sectionDetails: Record<string, string> = {
  General: "Course announcements, overview, and learning resources.",
  Attendance: "Review attendance records and participation summaries.",
  "First Semester Course Content":
    "Lectures, reading material, and practical resources for semester one.",
  "First Semester Assignments": "Assignment briefs, due dates, and submission information.",
};

export default function LecViewCourse() {

  const [activeNav, setActiveNav] = useState("My Courses");
  const [activeTab, setActiveTab] = useState("Course");
  const [editMode, setEditMode] = useState(true);
  const [sections, setSections] = useState(initialSections);
  const [openSections, setOpenSections] = useState<string[]>([]);
  const [isAdding, setIsAdding] = useState(false);
  const [newSection, setNewSection] = useState("");
  const [notice, setNotice] = useState("");

  const toggleSection = (section: string) => {
    setOpenSections((current) =>
      current.includes(section)
        ? current.filter((item) => item !== section)
        : [...current, section],
    );
  };

  const addSection = (event: FormEvent) => {
    event.preventDefault();
    const name = newSection.trim();
    if (!name) return;
    setSections((current) => [...current, name]);
    setOpenSections((current) => [...current, name]);
    setNewSection("");
    setIsAdding(false);
  };

  return (
    <div className="app-shell">
      <aside className="sidebar" aria-label="Primary navigation">
        <div className="brand">
          <img className="brand-mark" src={`${assets}/580ec.png`} alt="" width="104" height="104" />
          <div className="brand-name">
            <span>University Course</span>
            <span>Management System</span>
          </div>
        </div>

        <nav className="nav-list">
          {navItems.map((item) => {
            const active = activeNav === item.label;
            return (
              <button
                className={`nav-item${active ? " active" : ""}`}
                key={item.label}
                type="button"
                aria-current={active ? "page" : undefined}
                onClick={() => {
                  setActiveNav(item.label);
                  setNotice(`Viewing ${item.label}`);
                }}
              >
                <img src={`${assets}/${item.icon}`} alt="" width="48" height="48" />
                <span>{item.label}</span>
              </button>
            );
          })}
        </nav>

        <button
          className="logout"
          type="button"
          onClick={() => setNotice("You have been logged out.")}
        >
          <img src={`${assets}/c8ef2.svg`} alt="" />
          <span>LOGOUT</span>
        </button>
      </aside>

      <main className="main-content">
        <section className="course-card" aria-labelledby="course-title">
          <div className="course-heading">
            <div>
              <p className="breadcrumb">School of Computing / HND in Computing/ Cohorts</p>
              <h1 id="course-title">HND in Computing - UOK001</h1>
            </div>
            <label className="edit-toggle">
              <span>Edit Mode</span>
              <input
                type="checkbox"
                checked={editMode}
                onChange={(event) => setEditMode(event.target.checked)}
              />
              <span className={`switch-image${editMode ? " on" : ""}`} aria-hidden="true">
                <img src={`${assets}/01f55.png`} alt="" width="45" height="45" />
              </span>
            </label>
          </div>

          <div className="course-tabs" role="tablist" aria-label="Course pages">
            {["Course", "Participants", "Grades", "Reports"].map((tab) => (
              <button
                key={tab}
                type="button"
                role="tab"
                aria-selected={activeTab === tab}
                className={activeTab === tab ? "selected" : ""}
                onClick={() => setActiveTab(tab)}
              >
                {tab}
              </button>
            ))}
          </div>
          <img className="heading-line" src={`${assets}/1b0db.svg`} alt="" />

          <div className="sections" aria-live="polite">
            {activeTab !== "Course" && (
              <div className="tab-message">
                <strong>{activeTab}</strong>
                <span>This course view is ready for your content.</span>
              </div>
            )}

            {activeTab === "Course" &&
              sections.map((section) => {
                const isOpen = openSections.includes(section);
                return (
                  <div className={`section-row${isOpen ? " expanded" : ""}`} key={section}>
                    <button
                      className="section-button"
                      type="button"
                      aria-expanded={isOpen}
                      onClick={() => toggleSection(section)}
                    >
                      <img
                        className="arrow"
                        src={`${assets}/2cb9f.png`}
                        alt=""
                        width="30"
                        height="30"
                      />
                      <span>{section}</span>
                    </button>
                    {isOpen && (
                      <div className="section-detail">
                        {sectionDetails[section] ?? "New course section. Add your content here."}
                      </div>
                    )}
                  </div>
                );
              })}

            {activeTab === "Course" && isAdding && (
              <form className="add-form" onSubmit={addSection}>
                <label htmlFor="section-name">Section name</label>
                <div>
                  <input
                    id="section-name"
                    autoFocus
                    value={newSection}
                    onChange={(event) => setNewSection(event.target.value)}
                    placeholder="Enter a section name"
                  />
                  <button type="submit">Add</button>
                  <button type="button" onClick={() => setIsAdding(false)}>
                    Cancel
                  </button>
                </div>
              </form>
            )}

            {activeTab === "Course" && !isAdding && (
              <button
                className="add-section"
                type="button"
                disabled={!editMode}
                onClick={() => setIsAdding(true)}
              >
                + Add New Section
              </button>
            )}
          </div>

          {notice && (
            <button className="notice" type="button" onClick={() => setNotice("")}>
              {notice}
              <span aria-hidden="true">×</span>
            </button>
          )}
        </section>
      </main>
    </div>
  );
}
