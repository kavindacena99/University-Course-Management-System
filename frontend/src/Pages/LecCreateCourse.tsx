import { FormEvent, useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import "./LecCreateCourse.css";
import { createCourse } from "../services/courses";
import { getCurrentUser, logout } from "../services/auth";
import type { UserSummary } from "../types/api";

const asset = (name: string) => `/assets/${name}`;

const navItems = [
  { label: "Dashboard", icon: "57bc3.png" },
  { label: "My Courses", icon: "45c82.png" },
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
  onLogout,
}: {
  activeItem: string;
  onNavigate: (item: string) => void;
  onLogout: () => void;
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

      <button className="logout" type="button" onClick={onLogout}>
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
  user,
}: {
  user: UserSummary | null;
}) {
  const [profileOpen, setProfileOpen] = useState(false);

  return (
    <header className="topbar">
      <div className="profile-wrap">
        <button
          className="profile"
          type="button"
          aria-expanded={profileOpen}
          onClick={() => setProfileOpen((open) => !open)}
        >
          <span className="avatar" aria-hidden="true">
            {user ? `${user.first_name[0] ?? ""}${user.last_name[0] ?? ""}`.toUpperCase() : ""}
          </span>
          <span>
            <strong>{user ? `${user.first_name} ${user.last_name}` : "Account"}</strong>
            <small>{user?.email ?? ""}</small>
          </span>
          <img className="chevron" src={asset("0d534.svg")} alt="" />
        </button>
        {profileOpen && (
          <div className="profile-menu" role="status">
            {user?.email ?? "Loading account details..."}
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
                  type="number"
                  id="credits"
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
  const navigate = useNavigate();
  const [activeItem, setActiveItem] = useState("My Courses");
  const [notice, setNotice] = useState("");
  const [user, setUser] = useState<UserSummary | null>(null);

  useEffect(() => {
    let active = true;
    getCurrentUser()
      .then((currentUser) => {
        if (active) setUser(currentUser);
      })
      .catch((error: unknown) => {
        if (active) {
          setNotice(error instanceof Error ? error.message : "Unable to load account details.");
        }
      });
    return () => {
      active = false;
    };
  }, []);

  const navigateToItem = (item: string) => {
    setActiveItem(item);
    navigate(
      item === "Dashboard" && user?.roles.includes("ADMIN")
        ? "/admin"
        : "/courses",
    );
  };

  const handleLogout = async () => {
    try {
      await logout();
      navigate("/login");
    } catch (error) {
      setNotice(error instanceof Error ? error.message : "Unable to sign out.");
    }
  };

  return (
    <div className="app-shell">
      <Sidebar activeItem={activeItem} onNavigate={navigateToItem} onLogout={handleLogout} />
      <div className="workspace">
        <Header user={user} />
        <CourseForm onSaved={setNotice} />
      </div>
      {notice && (
        <button className="toast" type="button" role="status" onClick={() => setNotice("")}>
          {notice}
        </button>
      )}
    </div>
  );
}
