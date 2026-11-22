import { useState } from "react";
import "./LecModuleAsssignment.css";

const assetPath = "/assets/course-management";

const navigation = [
  { label: "Dashboard", icon: "57bc3.png" },
  { label: "My Courses", icon: "45c82.png" },
  { label: "Assignments", icon: "2c8cd.png" },
  { label: "Results and Grades", icon: "1cad0.png" },
  { label: "Class Analysis", icon: "4ebe4.png" },
  { label: "Academic Calender", icon: "2e776.png" },
  { label: "Notice Board", icon: "4c71a.png" },
];

const tabs = ["Assignments", "Settings", "Submissions"] as const;
type Tab = (typeof tabs)[number];

function AssignmentOverview() {
  return (
    <>
      <section className="date-card" aria-label="Assignment dates">
        <p>
          <strong>Released Date: </strong>
          <em>Monday , 28th of September 2026 , 11.59 pm</em>
        </p>
        <p>
          <strong>Due Date: </strong>
          <em>Tuesday, 20th of November 2026 , 11.5 pm</em>
        </p>
      </section>

      <section className="grading-summary" aria-labelledby="grading-heading">
        <h2 id="grading-heading">Grading Summary</h2>
        <div className="summary-table" role="table" aria-label="Grading summary">
          <div role="row">
            <span role="cell">Total Number of Students</span>
            <span role="cell">50</span>
          </div>
          <div role="row">
            <span role="cell">Submitted Assignments</span>
            <span role="cell">39</span>
          </div>
          <div role="row">
            <span role="cell">Need to be Grade</span>
            <span role="cell">20</span>
          </div>
        </div>
      </section>
    </>
  );
}

export default function LecModuleAssignment() {
  const [activeNav, setActiveNav] = useState("Assignments");
  const [activeTab, setActiveTab] = useState<Tab>("Assignments");
  const [editMode, setEditMode] = useState(true);
  const [notice, setNotice] = useState("");

  function showNotice(message: string) {
    setNotice(message);
    window.setTimeout(() => setNotice(""), 2200);
  }

  return (
    <div className="app-shell">
      <aside className="sidebar" aria-label="Course management navigation">
        <div className="brand">
          <img src={`${assetPath}/580ec.png`} alt="" />
          <p>
            <span>University Course </span>
            <span>Management System</span>
          </p>
        </div>

        <nav className="main-navigation">
          {navigation.map((item) => {
            const isActive = activeNav === item.label;
            return (
              <button
                className={isActive ? "nav-item active" : "nav-item"}
                key={item.label}
                onClick={() => {
                  setActiveNav(item.label);
                  if (item.label !== "Assignments") {
                    showNotice(`${item.label} selected`);
                  }
                }}
                type="button"
              >
                <img src={`${assetPath}/${item.icon}`} alt="" />
                <span>{item.label}</span>
              </button>
            );
          })}
        </nav>

        <button
          className="logout-button"
          onClick={() => showNotice("You have been logged out")}
          type="button"
        >
          <img src={`${assetPath}/c8ef2.svg`} alt="" />
          <span>LOGOUT</span>
        </button>
      </aside>

      <main className="workspace">
        <article className="assignment-card">
          <div className="edit-control">
            <span>Edit Mode</span>
            <button
              aria-checked={editMode}
              aria-label="Toggle edit mode"
              className={editMode ? "switch on" : "switch"}
              onClick={() => setEditMode((value) => !value)}
              role="switch"
              type="button"
            >
              <img src={`${assetPath}/01f55.png`} alt="" />
            </button>
          </div>

          <header className="assignment-header">
            <p className="breadcrumb">
              School of Computing / HND in Computing/ Cohorts / HND in Computing - UOK 001 / First
              Semester Assignments/ Programming Assignment
            </p>
            <h1>
              <img src={`${assetPath}/11c57.png`} alt="" />
              <span>Unit 01 : Programming Assignment</span>
            </h1>
          </header>

          <div className="tabs" role="tablist" aria-label="Assignment sections">
            {tabs.map((tab) => (
              <button
                aria-selected={activeTab === tab}
                className={activeTab === tab ? "active" : ""}
                key={tab}
                onClick={() => setActiveTab(tab)}
                role="tab"
                type="button"
              >
                {tab}
              </button>
            ))}
            <img aria-hidden="true" className="tab-divider" src={`${assetPath}/75552.svg`} />
          </div>

          <div className="tab-content" role="tabpanel">
            {activeTab === "Assignments" ? (
              <AssignmentOverview />
            ) : (
              <section className="empty-panel">
                <h2>{activeTab}</h2>
                <p>
                  {activeTab === "Settings"
                    ? `Editing is currently ${editMode ? "enabled" : "disabled"} for this assignment.`
                    : "Student submissions will appear here."}
                </p>
              </section>
            )}
          </div>
        </article>
      </main>

      <div aria-live="polite" className={notice ? "toast visible" : "toast"}>
        {notice}
      </div>
    </div>
  );
}
