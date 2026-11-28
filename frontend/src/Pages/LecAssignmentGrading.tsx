import { useMemo, useState } from "react";
import "./LecAssignmentGrading.css";

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

type Submission = {
  id: number;
  name: string;
  status: "Submitted" | "Late Submitted" | "Not submitted";
  selected: boolean;
  grade: boolean;
  modified: string;
  file: string;
  feedback: boolean;
};

const initialSubmissions: Submission[] = [
  {
    id: 1,
    name: "Mr. XYZ",
    status: "Submitted",
    selected: false,
    grade: false,
    modified: "",
    file: "",
    feedback: false,
  },
  {
    id: 2,
    name: "Mr. XYZ",
    status: "Late Submitted",
    selected: false,
    grade: false,
    modified: "",
    file: "",
    feedback: false,
  },
];

export default function LecAssignmentGrading() {
  const [activeNav, setActiveNav] = useState("Results and Grades");
  const [activeTab, setActiveTab] = useState("Submissions");
  const [statusFilter, setStatusFilter] = useState("All statuses");
  const [submissions, setSubmissions] = useState(initialSubmissions);
  const [message, setMessage] = useState("");

  const visibleSubmissions = useMemo(
    () =>
      statusFilter === "All statuses"
        ? submissions
        : submissions.filter((submission) => submission.status === statusFilter),
    [statusFilter, submissions],
  );

  const updateSubmission = <K extends keyof Submission>(
    id: number,
    key: K,
    value: Submission[K],
  ) => {
    setSubmissions((current) =>
      current.map((submission) =>
        submission.id === id ? { ...submission, [key]: value } : submission,
      ),
    );
  };

  const clearAll = () => {
    setSubmissions(
      initialSubmissions.map((submission) => ({
        ...submission,
        selected: false,
        grade: false,
        modified: "",
        file: "",
        feedback: false,
      })),
    );
    setStatusFilter("All statuses");
    setMessage("Submission fields have been cleared.");
  };

  return (
    <main className="app-shell">
      <aside className="sidebar" aria-label="Main navigation">
        <div className="brand">
          <img src={`${assets}/580ec.png`} alt="" className="brand-mark" />
          <p>
            University Course
            <span>Management System</span>
          </p>
        </div>

        <nav className="side-navigation">
          {navigation.map((item) => {
            const active = item.label === activeNav;
            return (
              <button
                className={`nav-item ${active ? "active" : ""}`}
                type="button"
                key={item.label}
                onClick={() => {
                  setActiveNav(item.label);
                  setMessage(`${item.label} selected.`);
                }}
              >
                <img src={`${assets}/${item.icon}`} alt="" />
                <span>{item.label}</span>
              </button>
            );
          })}
        </nav>

        <button
          type="button"
          className="logout"
          onClick={() => setMessage("You have been logged out.")}
        >
          <img src={`${assets}/c8ef2.svg`} alt="" />
          <span>LOGOUT</span>
        </button>
      </aside>

      <section className="workspace">
        <article className="assignment-card">
          <header>
            <p className="breadcrumb">
              School of Computing / HND in Computing/ Cohorts / HND in Computing - UOK 001 / First
              Semester Assignments/ Programming Assignment
            </p>
            <h1>
              <img src={`${assets}/11c57.png`} alt="" />
              Unit 01 : Programming Assignment
            </h1>
            <nav className="tabs" aria-label="Assignment sections">
              {["Assignments", "Settings", "Submissions"].map((tab) => (
                <button
                  type="button"
                  className={activeTab === tab ? "active" : ""}
                  key={tab}
                  onClick={() => {
                    setActiveTab(tab);
                    setMessage(`${tab} tab selected.`);
                  }}
                >
                  {tab}
                </button>
              ))}
            </nav>
          </header>

          <div className="submissions-heading">
            <h2>Submissions</h2>
            <label className="status-filter">
              <span>Status</span>
              <span className="select-wrap">
                <select
                  value={statusFilter}
                  onChange={(event) => setStatusFilter(event.target.value)}
                >
                  <option>All statuses</option>
                  <option>Submitted</option>
                  <option>Late Submitted</option>
                  <option>Not submitted</option>
                </select>
                <img src={`${assets}/0d534.svg`} alt="" />
              </span>
            </label>
            <button type="button" className="clear-button" onClick={clearAll}>
              Clear All
            </button>
          </div>

          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Select</th>
                  <th>Name</th>
                  <th>Status</th>
                  <th>Grade</th>
                  <th>Last Modified Date</th>
                  <th>File Submissions</th>
                  <th>Feedback</th>
                </tr>
              </thead>
              <tbody>
                {visibleSubmissions.map((submission) => (
                  <tr key={submission.id}>
                    <td>
                      <button
                        type="button"
                        className={`image-checkbox ${submission.selected ? "checked" : ""}`}
                        aria-label={`Select ${submission.name}`}
                        aria-pressed={submission.selected}
                        onClick={() =>
                          updateSubmission(submission.id, "selected", !submission.selected)
                        }
                      >
                        <img src={`${assets}/110ec.png`} alt="" />
                      </button>
                    </td>
                    <td>{submission.name}</td>
                    <td>
                      <select
                        className="status-cell"
                        aria-label={`Status for ${submission.name}`}
                        value={submission.status}
                        onChange={(event) =>
                          updateSubmission(
                            submission.id,
                            "status",
                            event.target.value as Submission["status"],
                          )
                        }
                      >
                        <option>Submitted</option>
                        <option>Late Submitted</option>
                        <option>Not submitted</option>
                      </select>
                    </td>
                    <td>
                      <button
                        type="button"
                        className="grade-toggle"
                        aria-pressed={submission.grade}
                        onClick={() => updateSubmission(submission.id, "grade", !submission.grade)}
                      >
                        A <span className={submission.grade ? "checked" : ""} />
                      </button>
                    </td>
                    <td>
                      <input
                        aria-label={`Last modified date for ${submission.name}`}
                        value={submission.modified}
                        onChange={(event) =>
                          updateSubmission(submission.id, "modified", event.target.value)
                        }
                      />
                    </td>
                    <td>
                      <input
                        aria-label={`File submission for ${submission.name}`}
                        value={submission.file}
                        onChange={(event) =>
                          updateSubmission(submission.id, "file", event.target.value)
                        }
                      />
                    </td>
                    <td>
                      <button
                        type="button"
                        className={`feedback-toggle ${submission.feedback ? "checked" : ""}`}
                        aria-label={`Feedback for ${submission.name}`}
                        aria-pressed={submission.feedback}
                        onClick={() =>
                          updateSubmission(submission.id, "feedback", !submission.feedback)
                        }
                      />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            {visibleSubmissions.length === 0 && (
              <p className="empty-state">No submissions match this status.</p>
            )}
          </div>
          <p className="sr-only" aria-live="polite">
            {message}
          </p>
        </article>
      </section>
    </main>
  );
}
