import { FormEvent, useMemo, useState } from "react";
import "./Courses.css";
import { createCourse } from "../services/courses";

const asset = (name: string) => `/assets/${name}`;

const navItems = [
  { label: "Dashboard", icon: "57bc3.png" },
  { label: "My Courses", icon: "45c82.png" },
  { label: "Assignments", icon: "2c8cd.png" },
  { label: "Results and Grades", icon: "1cad0.png" },
  { label: "Class Analysis", icon: "4ebe4.png" },
  { label: "Academic Calender", icon: "2e776.png" },
  { label: "Notice Board", icon: "4c71a.png" },
];

type FormState = {
  title: string;
  code: string;
  description: string;
  credits: string;
};

const emptyForm: FormState = {
  title: "",
  code: "",
  description: "",
  credits: "3",
};

function Sidebar({
  activeItem,
  onNavigate,
}: {
  activeItem: string;
  onNavigate: (item: string) => void;
}) {
  return (
    <aside className="sidebar">
      <div className="sidebar-accent" />
      <div className="brand">
        <div className="brand-mark">
          <img src={asset("580ec.png")} alt="" />
        </div>
        <p>University Course Management System</p>
      </div>

      <nav className="sidebar-nav" aria-label="Main navigation">
        {navItems.map((item) => (
          <button
            className={`nav-item ${activeItem === item.label ? "active" : ""}`}
            key={item.label}
            type="button"
            onClick={() => onNavigate(item.label)}
          >
            <img src={asset(item.icon)} alt="" />
            <span>{item.label}</span>
          </button>
        ))}
      </nav>

      <button className="logout" type="button" onClick={() => onNavigate("Logged out")}>
        <img className="logout-line" src={asset("00bb1.svg")} alt="" />
        <span className="logout-content">
          <img src={asset("394ee.svg")} alt="" />
          <span>LOGOUT</span>
        </span>
      </button>
    </aside>
  );
}

function Header({
  query,
  onQueryChange,
}: {
  query: string;
  onQueryChange: (value: string) => void;
}) {
  const [profileOpen, setProfileOpen] = useState(false);

  return (
    <header className="topbar">
      <label className="search">
        <img src={asset("a02fb.svg")} alt="" />
        <input
          value={query}
          onChange={(event) => onQueryChange(event.target.value)}
          placeholder="Search Courses, Assignments, Results and Grades etc..."
          aria-label="Search courses, assignments, results and grades"
        />
      </label>

      <div className="profile-wrap">
        <button
          className="profile"
          type="button"
          aria-expanded={profileOpen}
          onClick={() => setProfileOpen((open) => !open)}
        >
          <img className="avatar" src={asset("b5e3e.png")} alt="Ms Dileeshi Sathsarani" />
          <span>
            <strong>Ms Dileeshi Sathsarani</strong>
            <small>Lecturer in IT</small>
          </span>
          <img className="chevron" src={asset("0d534.svg")} alt="" />
        </button>
        {profileOpen && (
          <div className="profile-menu">
            <button type="button">View profile</button>
            <button type="button">Account settings</button>
          </div>
        )}
      </div>
    </header>
  );
}

function CourseForm({ onSaved }: { onSaved: (message: string) => void }) {
  const [form, setForm] = useState<FormState>(emptyForm);
  const [editMode, setEditMode] = useState(true);
  const [submitted, setSubmitted] = useState(false);
  const [saving, setSaving] = useState(false);

  const setField = (field: keyof FormState, value: string) => {
    setForm((current) => ({ ...current, [field]: value }));
  };

  const requiredMissing = useMemo(
    () =>
      submitted &&
      (!form.title || !form.code || !form.credits || Number.isNaN(Number(form.credits))),
    [form.code, form.credits, form.title, submitted],
  );

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setSubmitted(true);
    const credits = Number(form.credits);
    if (!form.title || !form.code || !form.credits || !Number.isFinite(credits) || credits < 0) return;

    setSaving(true);
    try {
      await createCourse({
        code: form.code,
        title: form.title,
        description: form.description,
        credits,
      });
      onSaved(`${form.title} has been created successfully.`);
      setForm(emptyForm);
      setSubmitted(false);
    } catch (error) {
      onSaved(error instanceof Error ? error.message : "Unable to create the course.");
    } finally {
      setSaving(false);
    }
  };

  const cancel = () => {
    setForm(emptyForm);
    setSubmitted(false);
    onSaved("Course form cleared.");
  };

  return (
    <main className="content">
      <section className="course-card" aria-labelledby="course-form-title">
        <div className="card-heading">
          <h1 id="course-form-title">Create New Course</h1>
          <label className="edit-toggle">
            <span>Edit Mode</span>
            <input
              type="checkbox"
              checked={editMode}
              onChange={(event) => setEditMode(event.target.checked)}
            />
            <span className="switch-track">
              <img src={asset("01f55.png")} alt="" />
            </span>
          </label>
        </div>

        <form onSubmit={submit} noValidate>
          <fieldset disabled={!editMode}>
            <div className="field field-full">
              <label htmlFor="title">Course Title</label>
              <input
                id="title"
                value={form.title}
                onChange={(event) => setField("title", event.target.value)}
                aria-invalid={submitted && !form.title}
              />
            </div>

            <div className="form-grid">
              <div className="field">
                <label htmlFor="code">Course Code</label>
                <input
                  id="code"
                  value={form.code}
                  onChange={(event) => setField("code", event.target.value)}
                  aria-invalid={submitted && !form.code}
                />
              </div>
              <div className="field">
                <label htmlFor="credits">Course Credits</label>
                <input
                  id="credits"
                  type="number"
                  min="0"
                  value={form.credits}
                  onChange={(event) => setField("credits", event.target.value)}
                  aria-invalid={submitted && (!form.credits || !Number.isFinite(Number(form.credits)))}
                />
              </div>
            </div>

            <div className="field description">
              <label htmlFor="description">Description</label>
              <textarea
                id="description"
                value={form.description}
                onChange={(event) => setField("description", event.target.value)}
              />
            </div>
          </fieldset>

          {requiredMissing && (
            <p className="form-error" role="alert">
              Please complete the course title, course code and a valid credit value.
            </p>
          )}

          <div className="form-actions">
            <button className="button secondary" type="button" onClick={cancel}>
              Cancel
            </button>
            <button className="button primary" type="submit" disabled={!editMode || saving}>
              {saving ? "Saving..." : "Save Course"}
            </button>
          </div>
        </form>
      </section>
    </main>
  );
}

export default function LecCreateCourse () {
    
  const [activeItem, setActiveItem] = useState("My Courses");
  const [query, setQuery] = useState("");
  const [notice, setNotice] = useState("");

  const navigate = (item: string) => {
    setActiveItem(item);
    if (item !== "My Courses") setNotice(`${item} selected.`);
  };

  return (
    <div className="app-shell">
      <Sidebar activeItem={activeItem} onNavigate={navigate} />
      <div className="workspace">
        <Header query={query} onQueryChange={setQuery} />
        {query && (
          <div className="search-status" role="status">
            Searching for “{query}”
          </div>
        )}
        <CourseForm onSaved={setNotice} />
      </div>
      {notice && (
        <button className="toast" type="button" onClick={() => setNotice("")}>
          {notice}
        </button>
      )}
    </div>
  );
}
