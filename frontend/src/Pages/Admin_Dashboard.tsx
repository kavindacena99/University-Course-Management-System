import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { BookOpen, LogOut, Search, TrendingUp, Users } from "lucide-react";
import type {
  AdminAnalyticsSummary,
  AdminUser,
  Course,
} from "../types/api";
import {
  approveUser,
  changeCourseStatus,
  getAdminSummary,
  listAdminCourses,
  listPendingUsers,
} from "../services/admin";
import { logout } from "../services/auth";
import "./Admin_Dashboard.css";

type AdminTab = "overview" | "courses" | "users";

async function loadAdminData() {
  const [summaryResponse, courseResponse, userResponse] = await Promise.all([
    getAdminSummary(),
    listAdminCourses(),
    listPendingUsers(),
  ]);
  return {
    summary: summaryResponse.summary,
    courses: courseResponse.courses,
    users: userResponse.users,
  };
}

export default function Admin_Dashboard() {
  const navigate = useNavigate();
  const [activeTab, setActiveTab] = useState<AdminTab>("overview");
  const [search, setSearch] = useState("");
  const [summary, setSummary] = useState<AdminAnalyticsSummary | null>(null);
  const [courses, setCourses] = useState<Course[]>([]);
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [loading, setLoading] = useState(true);
  const [actionId, setActionId] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    let active = true;
    loadAdminData()
      .then((data) => {
        if (!active) return;
        setSummary(data.summary);
        setCourses(data.courses);
        setUsers(data.users);
      })
      .catch((requestError) => {
        if (active) {
          setError(requestError instanceof Error ? requestError.message : "Unable to load administration data.");
        }
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, []);

  const query = search.trim().toLowerCase();
  const filteredCourses = useMemo(
    () =>
      courses.filter(
        (course) =>
          !query ||
          course.code.toLowerCase().includes(query) ||
          course.title.toLowerCase().includes(query),
      ),
    [courses, query],
  );
  const filteredUsers = useMemo(
    () =>
      users.filter((user) =>
        `${user.first_name} ${user.last_name} ${user.email}`.toLowerCase().includes(query),
      ),
    [users, query],
  );

  async function refresh() {
    const data = await loadAdminData();
    setSummary(data.summary);
    setCourses(data.courses);
    setUsers(data.users);
  }

  async function runAction(id: string, action: () => Promise<unknown>) {
    setActionId(id);
    setError("");
    try {
      await action();
      await refresh();
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : "The action failed.");
    } finally {
      setActionId("");
    }
  }

  async function signOut() {
    try {
      await logout();
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : "Unable to sign out.");
    } finally {
      navigate("/login");
    }
  }

  return (
    <div className="ucms-admin">
      <aside className="ucms-admin__sidebar">
        <a className="ucms-admin__brand" href="/admin" aria-label="University Course Management System">
          <span aria-hidden="true">UC</span>
          <strong>University Course<br />Management System</strong>
        </a>
        <nav aria-label="Administration">
          {([
            ["overview", "Overview", <TrendingUp key="overview" size={18} />],
            ["courses", "Courses", <BookOpen key="courses" size={18} />],
            ["users", "Pending users", <Users key="users" size={18} />],
          ] as const).map(([tab, label, icon]) => (
            <button
              className={activeTab === tab ? "is-active" : ""}
              key={tab}
              onClick={() => setActiveTab(tab)}
              type="button"
            >
              {icon}<span>{label}</span>
              {tab === "users" && users.length > 0 && <small>{users.length}</small>}
            </button>
          ))}
        </nav>
        <button className="ucms-admin__logout" onClick={signOut} type="button">
          <LogOut size={18} /> Sign out
        </button>
      </aside>

      <main className="ucms-admin__main">
        <header className="ucms-admin__header">
          <div>
            <p>Administration</p>
            <h1>{activeTab === "overview" ? "Dashboard" : activeTab === "courses" ? "Courses" : "Pending users"}</h1>
          </div>
          <label className="ucms-admin__search">
            <Search size={17} aria-hidden="true" />
            <input
              aria-label="Search courses and pending users"
              onChange={(event) => setSearch(event.target.value)}
              placeholder="Search courses or users"
              value={search}
            />
          </label>
        </header>

        {error && <p className="ucms-admin__error" role="alert">{error}</p>}
        {loading ? (
          <p className="ucms-admin__state" role="status">Loading administration data...</p>
        ) : (
          <>
            {activeTab === "overview" && (
              <>
                <section className="ucms-admin__stats" aria-label="Academic metrics">
                  <article>
                    <span>Total results</span>
                    <strong>{summary?.total_results.toLocaleString() ?? "—"}</strong>
                  </article>
                  <article>
                    <span>Average result</span>
                    <strong>{summary?.average_percent == null ? "—" : `${summary.average_percent.toFixed(1)}%`}</strong>
                  </article>
                  <article>
                    <span>Courses</span>
                    <strong>{courses.length.toLocaleString()}</strong>
                  </article>
                  <article>
                    <span>Pending approvals</span>
                    <strong>{users.length.toLocaleString()}</strong>
                  </article>
                </section>

                <section className="ucms-admin__panel">
                  <h2>Grade distribution</h2>
                  {summary?.grade_distribution.length ? (
                    <div className="ucms-admin__distribution">
                      {summary.grade_distribution.map((item) => (
                        <article key={item.grade_code}>
                          <strong>{item.grade_code}</strong>
                          <span>{item.count} results</span>
                        </article>
                      ))}
                    </div>
                  ) : <p className="ucms-admin__state">No released results are available yet.</p>}
                </section>
              </>
            )}

            {(activeTab === "overview" || activeTab === "courses") && (
              <section className="ucms-admin__panel">
                <div className="ucms-admin__panel-heading">
                  <div>
                    <h2>Course directory</h2>
                    <p>Publish draft courses or manage active and archived courses.</p>
                  </div>
                </div>
                <div className="ucms-admin__table-wrap">
                  <table>
                    <thead>
                      <tr><th>Code</th><th>Course</th><th>Credits</th><th>Status</th><th>Action</th></tr>
                    </thead>
                    <tbody>
                      {filteredCourses.map((course) => {
                        const action = course.status === "draft" ? "publish" : course.status === "active" ? "archive" : "restore";
                        const label = action === "publish" ? "Publish" : action === "archive" ? "Archive" : "Restore";
                        return (
                          <tr key={course.id}>
                            <td>{course.code}</td>
                            <td>{course.title}</td>
                            <td>{course.credits}</td>
                            <td><span className={`ucms-admin__status ucms-admin__status--${course.status}`}>{course.status}</span></td>
                            <td>
                              <button
                                className="ucms-admin__action"
                                disabled={actionId === course.id}
                                onClick={() => void runAction(course.id, () => changeCourseStatus(course, action))}
                                type="button"
                              >
                                {actionId === course.id ? "Saving..." : label}
                              </button>
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                  {!filteredCourses.length && <p className="ucms-admin__state">No matching courses.</p>}
                </div>
              </section>
            )}

            {(activeTab === "overview" || activeTab === "users") && (
              <section className="ucms-admin__panel">
                <div className="ucms-admin__panel-heading">
                  <div>
                    <h2>Pending registrations</h2>
                    <p>Approve accounts submitted for university access.</p>
                  </div>
                </div>
                <div className="ucms-admin__table-wrap">
                  <table>
                    <thead><tr><th>Name</th><th>Email</th><th>Submitted</th><th>Action</th></tr></thead>
                    <tbody>
                      {filteredUsers.map((user) => (
                        <tr key={user.id}>
                          <td>{user.first_name} {user.last_name}</td>
                          <td>{user.email}</td>
                          <td>{new Date(user.created_at).toLocaleDateString()}</td>
                          <td>
                            <button
                              className="ucms-admin__action"
                              disabled={actionId === user.id}
                              onClick={() => void runAction(user.id, () => approveUser(user.id, user.version))}
                              type="button"
                            >
                              {actionId === user.id ? "Approving..." : "Approve"}
                            </button>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                  {!filteredUsers.length && <p className="ucms-admin__state">No pending registrations.</p>}
                </div>
              </section>
            )}
          </>
        )}
      </main>
    </div>
  );
}
