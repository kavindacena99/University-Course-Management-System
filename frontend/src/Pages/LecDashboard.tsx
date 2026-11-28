import React, { useEffect, useRef, useState } from "react";
import "./LecDashboard.css";

interface ActivityItem {
  id: string;
  icon: string;
  label: string;
}

interface NoticeItem {
  id: string;
  icon: string;
  title: string;
  description: string;
}

interface SessionItem {
  id: string;
  timeLines: string[];
  detailLines: string[];
}

const ACTIVITIES: ActivityItem[] = [
  { id: "act-1", icon: "📝", label: "Networking Quiz" },
  { id: "act-2", icon: "🧑‍💻", label: "WDD Practicals - Lab Session" },
  { id: "act-3", icon: "🌐", label: "IoT Exhibition" },
];

const NOTICES: NoticeItem[] = [
  {
    id: "not-1",
    icon: "♧",
    title: "BOE Meeting",
    description: "BOE Meeting will be held on 07th of October 2026",
  },
  {
    id: "not-2",
    icon: "♧",
    title: "All Staff Meeting",
    description:
      "Weekly staff meeting will be held on 06th of October 2026 04.00pm onwards",
  },
];

const SESSIONS: SessionItem[] = [
  {
    id: "ses-1",
    timeLines: ["09.00 am -", "12.00pm"],
    detailLines: ["HNDIT 079 - Networking", "Lecture hall 103"],
  },
  {
    id: "ses-2",
    timeLines: ["01.00 pm -", "04.00 pm"],
    detailLines: ["Top-Up 10 - Full stack Development", "Lecture Hall 109"],
  },
];

const WEEKDAYS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];

function sameDate(first: Date, second: Date): boolean {
  return (
    first.getFullYear() === second.getFullYear() &&
    first.getMonth() === second.getMonth() &&
    first.getDate() === second.getDate()
  );
}

export default function LecDashboard() {
  const [displayedMonth, setDisplayedMonth] = useState<Date>(() => new Date(2021, 8, 1));
  const [selectedDate, setSelectedDate] = useState<Date>(() => new Date(2021, 8, 19));
  const [searchQuery, setSearchQuery] = useState<string>("");
  const [activeNav, setActiveNav] = useState<string>("dashboard");

  const profileDialogRef = useRef<HTMLDialogElement | null>(null);
  const logoutDialogRef = useRef<HTMLDialogElement | null>(null);
  const dayButtonRefs = useRef<Array<HTMLButtonElement | null>>([]);

  const year = displayedMonth.getFullYear();
  const month = displayedMonth.getMonth();
  const firstWeekday = new Date(year, month, 1).getDay();
  const daysInMonth = new Date(year, month + 1, 0).getDate();

  const calendarTitle = displayedMonth.toLocaleDateString("en-US", {
    month: "long",
    year: "numeric",
  });

  const normalizedQuery = searchQuery.trim().toLowerCase();

  const filteredActivities = ACTIVITIES.filter((item) =>
    item.label.toLowerCase().includes(normalizedQuery)
  );
  const filteredNotices = NOTICES.filter((item) =>
    `${item.title} ${item.description}`.toLowerCase().includes(normalizedQuery)
  );
  const filteredSessions = SESSIONS.filter((item) =>
    `${item.timeLines.join(" ")} ${item.detailLines.join(" ")}`
      .toLowerCase()
      .includes(normalizedQuery)
  );

  const totalMatches =
    filteredActivities.length + filteredNotices.length + filteredSessions.length;

  useEffect(() => {
    dayButtonRefs.current = dayButtonRefs.current.slice(0, daysInMonth);
  }, [daysInMonth]);

  const handleSelectDay = (date: Date, dayIndex: number) => {
    setSelectedDate(date);
    requestAnimationFrame(() => {
      dayButtonRefs.current[dayIndex]?.focus();
    });
  };

  const handlePreviousMonth = () => {
    setDisplayedMonth(
      (prev) => new Date(prev.getFullYear(), prev.getMonth() - 1, 1)
    );
  };

  const handleNextMonth = () => {
    setDisplayedMonth(
      (prev) => new Date(prev.getFullYear(), prev.getMonth() + 1, 1)
    );
  };

  const handleDialogBackdropClick = (
    event: React.MouseEvent<HTMLDialogElement>,
    dialog: HTMLDialogElement | null
  ) => {
    if (!dialog || event.target !== dialog) return;
    const bounds = dialog.getBoundingClientRect();
    const outside =
      event.clientX < bounds.left ||
      event.clientX > bounds.right ||
      event.clientY < bounds.top ||
      event.clientY > bounds.bottom;
    if (outside) dialog.close();
  };

  return (
    <>
      <div className="dashboard">
        <aside className="sidebar" aria-label="Main sidebar">
          <a
            className="brand"
            href="#dashboard"
            aria-label="University home"
            onClick={() => setActiveNav("dashboard")}
          >
            <span className="brand-mark" aria-hidden="true">
              <svg viewBox="0 0 100 100" fill="none">
                <path
                  d="M8 38 50 15l42 23-42 23L8 38Z"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinejoin="round"
                />
                <path
                  d="M24 48v24c17 15 35 15 52 0V48M89 40v25"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinejoin="round"
                />
                <circle cx="89" cy="68" r="3" fill="currentColor" />
              </svg>
            </span>
            <span className="brand-name">
              University Course
              <br />
              Management System
            </span>
          </a>

          <nav className="navigation" aria-label="Main navigation">
            <a
              className={`nav-link${activeNav === "dashboard" ? " active" : ""}`}
              href="#dashboard"
              aria-current={activeNav === "dashboard" ? "page" : undefined}
              onClick={() => setActiveNav("dashboard")}
            >
              <span className="nav-icon" aria-hidden="true">
                ⌂
              </span>
              Dashboard
            </a>
            <a
              className={`nav-link${activeNav === "courses" ? " active" : ""}`}
              href="#sessions"
              aria-current={activeNav === "courses" ? "page" : undefined}
              onClick={() => setActiveNav("courses")}
            >
              <span className="nav-icon" aria-hidden="true">
                ▤
              </span>
              My Courses
            </a>
            <a
              className={`nav-link${activeNav === "assignments" ? " active" : ""}`}
              href="#assignments"
              aria-current={activeNav === "assignments" ? "page" : undefined}
              onClick={() => setActiveNav("assignments")}
            >
              <span className="nav-icon" aria-hidden="true">
                📝
              </span>
              Assignments
            </a>
            <a
              className={`nav-link${activeNav === "results" ? " active" : ""}`}
              href="#assignments"
              aria-current={activeNav === "results" ? "page" : undefined}
              onClick={() => setActiveNav("results")}
            >
              <span className="nav-icon" aria-hidden="true">
                🏅
              </span>
              Results and Grades
            </a>
            <a
              className={`nav-link${activeNav === "analysis" ? " active" : ""}`}
              href="#student-count"
              aria-current={activeNav === "analysis" ? "page" : undefined}
              onClick={() => setActiveNav("analysis")}
            >
              <span className="nav-icon" aria-hidden="true">
                ▥
              </span>
              Class Analysis
            </a>
            <a
              className={`nav-link${activeNav === "calendar" ? " active" : ""}`}
              href="#calendar"
              aria-current={activeNav === "calendar" ? "page" : undefined}
              onClick={() => setActiveNav("calendar")}
            >
              <span className="nav-icon" aria-hidden="true">
                ▦
              </span>
              Academic Calender
            </a>
            <a
              className={`nav-link${activeNav === "notices" ? " active" : ""}`}
              href="#notices"
              aria-current={activeNav === "notices" ? "page" : undefined}
              onClick={() => setActiveNav("notices")}
            >
              <span className="nav-icon" aria-hidden="true">
                ▤
              </span>
              Notice Board
            </a>
          </nav>

          <button
            className="logout"
            id="logout-button"
            type="button"
            onClick={() => logoutDialogRef.current?.showModal()}
          >
            <span className="logout-icon" aria-hidden="true">
              ➜
            </span>
            LOGOUT
          </button>
        </aside>

        <main className="main" id="dashboard">
          <label className="search">
            <svg
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.8"
              aria-hidden="true"
            >
              <circle cx="10.5" cy="10.5" r="6.5" />
              <path d="m16 16 4.5 4.5" />
            </svg>
            <input
              id="search-input"
              type="search"
              aria-label="Search dashboard"
              placeholder="Search Courses, Assignments, Results and Grades etc..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
            />
          </label>

          <h1>Lecturer Dashboard</h1>

          <section className="welcome card" aria-labelledby="welcome-title">
            <img
              className="welcome-image"
              src="lecturer.png"
              alt="Illustration of a lecturer holding a clipboard"
            />
            <div className="welcome-content">
              <h2 id="welcome-title">Good Morning, Ms.Dileeshi!</h2>
              <p>
                Empower Minds, Inspire Futures.
                <br />
                You have 2 Sessions today.
              </p>
              <a className="primary-button" href="#sessions">
                View My classes
                <span className="arrow-circle" aria-hidden="true">
                  ›
                </span>
              </a>
            </div>
          </section>

          <div className="information-grid">
            <section className="card information-card">
              <h2>Upcoming Activities</h2>
              <ul className="activity-list">
                {filteredActivities.map((item) => (
                  <li key={item.id}>
                    <span className="activity-icon" aria-hidden="true">
                      {item.icon}
                    </span>
                    <span>{item.label}</span>
                  </li>
                ))}
              </ul>
            </section>

            <section className="card information-card" id="notices">
              <h2>Notice Board</h2>
              <ul className="notice-list">
                {filteredNotices.map((notice) => (
                  <li className="notice" key={notice.id}>
                    <span className="notice-icon" aria-hidden="true">
                      {notice.icon}
                    </span>
                    <div>
                      <h3>{notice.title}</h3>
                      <p>{notice.description}</p>
                    </div>
                  </li>
                ))}
              </ul>
            </section>
          </div>

          <section className="stats" aria-label="Dashboard statistics">
            <article className="card stat">
              <h2>Today’s Sessions</h2>
              <div className="stat-value">
                <strong>2</strong>
                <span aria-hidden="true">🧑‍🏫</span>
              </div>
              <p>HND in IT &amp; Top-UP SW</p>
            </article>

            <article className="card stat" id="student-count">
              <h2>Student Count</h2>
              <div className="stat-value">
                <strong>700</strong>
                <span aria-hidden="true">👥</span>
              </div>
              <p>Department of Computing</p>
            </article>

            <article className="card stat" id="assignments">
              <h2>Assignments to be Graded</h2>
              <div className="stat-value">
                <strong>50</strong>
                <span aria-hidden="true">📋</span>
              </div>
              <p>Total Number of Assignments</p>
            </article>
          </section>

          {normalizedQuery && (
            <p className="empty-message" id="search-status" role="status">
              {totalMatches
                ? `${totalMatches} matching item${totalMatches === 1 ? "" : "s"} found.`
                : "No matching activities, notices, or sessions found."}
            </p>
          )}
        </main>

        <aside className="right-panel" aria-label="Profile and schedule">
          <section className="profile">
            <div className="avatar" role="img" aria-label="Profile placeholder" />
            <h2>Ms Dileeshi Sathsarani</h2>
            <p>Lecturer in IT</p>
            <button
              className="primary-button"
              id="profile-button"
              type="button"
              aria-haspopup="dialog"
              onClick={() => profileDialogRef.current?.showModal()}
            >
              Profile
            </button>
          </section>

          <section className="calendar" id="calendar" aria-label="Calendar">
            <div className="calendar-header">
              <button
                id="previous-month"
                type="button"
                aria-label="Previous month"
                onClick={handlePreviousMonth}
              >
                ‹
              </button>
              <h2 id="calendar-title" aria-live="polite">
                {calendarTitle}
              </h2>
              <button
                id="next-month"
                type="button"
                aria-label="Next month"
                onClick={handleNextMonth}
              >
                ›
              </button>
            </div>
            <div className="calendar-grid" id="calendar-grid">
              {WEEKDAYS.map((name) => (
                <span className="weekday" key={name}>
                  {name}
                </span>
              ))}

              {Array.from({ length: firstWeekday }, (_, index) => (
                <span key={`spacer-${index}`} aria-hidden="true" />
              ))}

              {Array.from({ length: daysInMonth }, (_, index) => {
                const day = index + 1;
                const date = new Date(year, month, day);
                const isSelected = sameDate(date, selectedDate);
                const ariaLabel = date.toLocaleDateString("en-US", {
                  weekday: "long",
                  year: "numeric",
                  month: "long",
                  day: "numeric",
                });

                return (
                  <button
                    key={day}
                    ref={(el) => {
                      dayButtonRefs.current[index] = el;
                    }}
                    type="button"
                    className={`calendar-day${isSelected ? " selected" : ""}`}
                    // aria-pressed={String(isSelected)}
                    aria-label={ariaLabel}
                    onClick={() => handleSelectDay(date, index)}
                  >
                    {day}
                  </button>
                );
              })}
            </div>
          </section>

          <section className="sessions" id="sessions">
            <h2>Today’s Sessions</h2>

            {filteredSessions.map((session) => (
              <article className="session" key={session.id}>
                <time>
                  {session.timeLines[0]}
                  <br />
                  {session.timeLines[1]}
                </time>
                <p>
                  {session.detailLines[0]}
                  <br />
                  {session.detailLines[1]}
                </p>
              </article>
            ))}
          </section>
        </aside>
      </div>

      <dialog
        ref={profileDialogRef}
        id="profile-dialog"
        aria-labelledby="profile-dialog-title"
        onClick={(e) => handleDialogBackdropClick(e, profileDialogRef.current)}
      >
        <h2 id="profile-dialog-title">Lecturer Profile</h2>
        <p>
          <strong>Ms Dileeshi Sathsarani</strong>
        </p>
        <p>Lecturer in IT</p>
        <p>Department of Computing</p>
        <form method="dialog">
          <button className="primary-button">Close</button>
        </form>
      </dialog>

      <dialog
        ref={logoutDialogRef}
        id="logout-dialog"
        aria-labelledby="logout-dialog-title"
        onClick={(e) => handleDialogBackdropClick(e, logoutDialogRef.current)}
      >
        <h2 id="logout-dialog-title">Demo dashboard</h2>
        <p>
          Logout is not connected to an authentication service in this front-end
          demo.
        </p>
        <form method="dialog">
          <button className="primary-button">Close</button>
        </form>
      </dialog>
    </>
  );
}