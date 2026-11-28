import { FormEvent, useEffect, useMemo, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import "./LecCreateCourse.css";
import { createCourse, getCourse, updateCourse } from "../services/courses";
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

function CourseForm({
  courseId,
  onSaved,
  onCancel,
}: {
  courseId: string | null;
  onSaved: (message: string) => void;
  onCancel: () => void;
}) {
  const [form, setForm] = useState<FormState>(emptyForm);
  const [courseVersion, setCourseVersion] = useState<number | null>(null);
  const [editMode, setEditMode] = useState(true);
  const [submitted, setSubmitted] = useState(false);
  const [saving, setSaving] = useState(false);
  const [loading, setLoading] = useState(Boolean(courseId));

  const setField = (field: keyof FormState, value: string) => {
    setForm((current) => ({ ...current, [field]: value }));
  };

  useEffect(() => {
    if (!courseId) {
      setLoading(false);
      setCourseVersion(null);
      setForm(emptyForm);
      return;
    }

    let active = true;
    setLoading(true);
    getCourse(courseId)
      .then(({ course }) => {
        if (!active) return;
        setCourseVersion(course.version);
        setForm({
          title: course.title,
          code: course.code,
          description: course.description,
          credits: String(course.credits),
        });
      })
      .catch((error: unknown) => {
        if (active) {
          onSaved(error instanceof Error ? error.message : "Unable to load the course.");
        }
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [courseId, onSaved]);

  const normalizedCode = form.code.trim().toUpperCase();
  const credits = Number(form.credits);
  const invalidCode = !/^[A-Z0-9_-]{2,32}$/.test(normalizedCode);
  const invalidCredits = !Number.isInteger(credits) || credits < 1 || credits > 30;
  const requiredMissing = useMemo(
    () =>
      submitted &&
      (!form.title.trim() ||
        form.title.length > 200 ||
        invalidCode ||
        invalidCredits ||
        form.description.length > 5000),
    [form.description.length, form.title, invalidCode, invalidCredits, submitted],
  );

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setSubmitted(true);
    const title = form.title.trim();
    if (
      !title ||
      title.length > 200 ||
      invalidCode ||
      invalidCredits ||
      form.description.length > 5000 ||
      (courseId !== null && courseVersion === null)
    ) return;

    setSaving(true);
    try {
      const input = {
        code: normalizedCode,
        title,
        description: form.description.trim(),
        credits,
      };
      if (courseId && courseVersion !== null) {
        const { course } = await updateCourse(courseId, { ...input, version: courseVersion });
        setCourseVersion(course.version);
        onSaved(`${course.title} has been updated successfully.`);
      } else {
        const { course } = await createCourse(input);
        onSaved(`${course.title} has been created successfully.`);
        setForm(emptyForm);
        setSubmitted(false);
      }
    } catch (error) {
      onSaved(error instanceof Error ? error.message : "Unable to save the course.");
    } finally {
      setSaving(false);
    }
  };

  const cancel = () => {
    if (courseId) {
      onCancel();
      return;
    }
    setForm(emptyForm);
    setSubmitted(false);
    onSaved("Course form cleared.");
  };

  return (
    <main className="content">
      <section className="course-card" aria-labelledby="course-form-title">
        <div className="card-heading">
          <h1 id="course-form-title">{courseId ? "Edit Course" : "Create New Course"}</h1>
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

        {loading && <p role="status">Loading course...</p>}
        <form onSubmit={submit} noValidate>
          <fieldset disabled={!editMode || loading || saving}>
            <div className="field field-full">
              <label htmlFor="title">Course Title</label>
              <input
                id="title"
                maxLength={200}
                value={form.title}
                onChange={(event) => setField("title", event.target.value)}
                aria-invalid={submitted && (!form.title.trim() || form.title.length > 200)}
              />
            </div>

            <div className="form-grid">
              <div className="field">
                <label htmlFor="code">Course Code</label>
                <input
                  id="code"
                  minLength={2}
                  maxLength={32}
                  pattern="[A-Za-z0-9_-]{2,32}"
                  value={form.code}
                  onChange={(event) => setField("code", event.target.value)}
                  aria-invalid={submitted && invalidCode}
                />
              </div>
              <div className="field">
                <label htmlFor="credits">Course Credits</label>
                <input
                  type="number"
                  id="credits"
                  min="1"
                  max="30"
                  step="1"
                  value={form.credits}
                  onChange={(event) => setField("credits", event.target.value)}
                  aria-invalid={submitted && invalidCredits}
                />
              </div>
            </div>

            <div className="field description">
              <label htmlFor="description">Description</label>
              <textarea
                id="description"
                maxLength={5000}
                value={form.description}
                onChange={(event) => setField("description", event.target.value)}
              />
            </div>
          </fieldset>

          {requiredMissing && (
            <p className="form-error" role="alert">
              Enter a title (up to 200 characters), a valid course code, 1–30 whole credits, and a description up to 5,000 characters.
            </p>
          )}

          <div className="form-actions">
            <button className="button secondary" type="button" onClick={cancel}>
              Cancel
            </button>
            <button className="button primary" type="submit" disabled={!editMode || loading || saving}>
              {saving ? "Saving..." : courseId ? "Save Changes" : "Save Course"}
            </button>
          </div>
        </form>
      </section>
    </main>
  );
}

export default function LecCreateCourse () {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [activeItem, setActiveItem] = useState("My Courses");
  const [notice, setNotice] = useState("");
  const [user, setUser] = useState<UserSummary | null>(null);
  const courseId = searchParams.get("courseId");

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
        <CourseForm
          courseId={courseId}
          onSaved={setNotice}
          onCancel={() => navigate("/courses")}
        />
      </div>
      {notice && (
        <button className="toast" type="button" role="status" onClick={() => setNotice("")}>
          {notice}
        </button>
      )}
    </div>
  );
}
