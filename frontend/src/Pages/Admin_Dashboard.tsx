import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { BookOpen, LogOut, Search, TrendingUp, Users } from "lucide-react";
import type {
  AdminAnalyticsSummary,
  AdminUser,
  Course,
} from "../types/api";
import {
  changeCourseStatus,
  changeUserStatus,
  getAdminSummary,
  getAdminUserHistory,
  listAdminUsers,
  listAdminCourses,
  listPendingUsers,
} from "../services/admin";
import { logout } from "../services/auth";
import "./Admin_Dashboard.css";

type AdminTab = "overview" | "courses" | "users";

async function loadAdminData(status: AdminUser["registration_status"]) {
  const [summaryResponse, courseResponse, pendingResponse, userResponse] = await Promise.all([
    getAdminSummary(),
    listAdminCourses(),
    listPendingUsers(),
    listAdminUsers(status),
  ]);
  return {
    summary: summaryResponse.summary,
    courses: courseResponse.courses,
    pendingUsers: pendingResponse.users,
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
  const [pendingUsers, setPendingUsers] = useState<AdminUser[]>([]);
  const [userStatus, setUserStatus] = useState<AdminUser["registration_status"]>("pending");
  const [historyUser, setHistoryUser] = useState<AdminUser | null>(null);
  const [userHistory, setUserHistory] = useState<Awaited<ReturnType<typeof getAdminUserHistory>>["history"]>([]);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [loading, setLoading] = useState(true);
  const [usersLoading, setUsersLoading] = useState(false);
  const [actionId, setActionId] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    let active = true;
    loadAdminData("pending")
      .then((data) => {
        if (!active) return;
        setSummary(data.summary);
        setCourses(data.courses);
        setUsers(data.users);
        setPendingUsers(data.pendingUsers);
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

  useEffect(() => {
    if (activeTab !== "users") return;
    let active = true;
    setUsersLoading(true);
    listAdminUsers(userStatus)
      .then((response) => {
        if (active) setUsers(response.users);
      })
      .catch((requestError: unknown) => {
        if (active) setError(requestError instanceof Error ? requestError.message : "Unable to load users.");
      })
      .finally(() => {
        if (active) setUsersLoading(false);
      });
    return () => {
      active = false;
    };
  }, [activeTab, userStatus]);

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
  const filteredUsers = useMemo(() => {
    const source = activeTab === "users" ? users : pendingUsers;
    return source.filter((user) =>
      `${user.first_name} ${user.last_name} ${user.email}`.toLowerCase().includes(query),
    );
  }, [activeTab, pendingUsers, query, users]);

  async function refresh() {
    const data = await loadAdminData(userStatus);
    setSummary(data.summary);
    setCourses(data.courses);
    setUsers(data.users);
    setPendingUsers(data.pendingUsers);
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

  async function showUserHistory(user: AdminUser) {
    setHistoryUser(user);
    setUserHistory([]);
    setHistoryLoading(true);
    setError("");
    try {
      const response = await getAdminUserHistory(user.id);
      setUserHistory(response.history);
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : "Unable to load user history.");
    } finally {
      setHistoryLoading(false);
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
            ["users", "Users", <Users key="users" size={18} />],
          ] as const).map(([tab, label, icon]) => (
            <button
              className={activeTab === tab ? "is-active" : ""}
              key={tab}
              onClick={() => setActiveTab(tab)}
              type="button"
            >
              {icon}<span>{label}</span>
              {tab === "users" && pendingUsers.length > 0 && <small>{pendingUsers.length}</small>}
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
            <h1>{activeTab === "overview" ? "Dashboard" : activeTab === "courses" ? "Courses" : "Users"}</h1>
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
                    <strong>{pendingUsers.length.toLocaleString()}</strong>
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
                    <h2>{activeTab === "users" ? "User accounts" : "Pending registrations"}</h2>
                    <p>{activeTab === "users" ? "Review account registration status and manage access." : "Approve accounts submitted for university access."}</p>
                  </div>
                  {activeTab === "users" && (
                    <label className="ucms-admin__user-filter">
                      Status
                      <select
                        onChange={(event) => setUserStatus(event.target.value as AdminUser["registration_status"])}
                        value={userStatus}
                      >
                        <option value="pending">Pending</option>
                        <option value="approved">Approved</option>
                        <option value="rejected">Rejected</option>
                        <option value="suspended">Suspended</option>
                      </select>
                    </label>
                  )}
                </div>
                <div className="ucms-admin__table-wrap">
                  <table>
                    <thead><tr><th>Name</th><th>Email</th><th>Status</th><th>Submitted</th><th>Action</th></tr></thead>
                    <tbody>
                      {filteredUsers.map((user) => (
                        <tr key={user.id}>
                          <td>{user.first_name} {user.last_name}</td>
                          <td>{user.email}</td>
                          <td>{user.registration_status}</td>
                          <td>{new Date(user.created_at).toLocaleDateString()}</td>
                          <td>
                            {user.registration_status === "pending" && (
                              <>
                                <button
                                  className="ucms-admin__action"
                                  disabled={actionId === user.id}
                                  onClick={() => void runAction(user.id, () => changeUserStatus(user, "approve"))}
                                  type="button"
                                >
                                  {actionId === user.id ? "Saving..." : "Approve"}
                                </button>
                                <button
                                  className="ucms-admin__action ucms-admin__action--secondary"
                                  disabled={actionId === user.id}
                                  onClick={() => void runAction(user.id, () => changeUserStatus(user, "reject"))}
                                  type="button"
                                >
                                  Reject
                                </button>
                              </>
                            )}
                            {user.registration_status === "approved" && (
                              <button
                                className="ucms-admin__action ucms-admin__action--secondary"
                                disabled={actionId === user.id}
                                onClick={() => void runAction(user.id, () => changeUserStatus(user, "suspend"))}
                                type="button"
                              >
                                Suspend
                              </button>
                            )}
                            {user.registration_status === "suspended" && (
                              <button
                                className="ucms-admin__action"
                                disabled={actionId === user.id}
                                onClick={() => void runAction(user.id, () => changeUserStatus(user, "reinstate"))}
                                type="button"
                              >
                                Reinstate
                              </button>
                            )}
                            <button
                              className="ucms-admin__action ucms-admin__action--secondary"
                              onClick={() => void showUserHistory(user)}
                              type="button"
                            >
                              History
                            </button>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                  {usersLoading ? (
                    <p className="ucms-admin__state" role="status">Loading users...</p>
                  ) : !filteredUsers.length ? (
                    <p className="ucms-admin__state">No matching users.</p>
                  ) : null}
                </div>
                {activeTab === "users" && historyUser && (
                  <section className="ucms-admin__history" aria-label="User status history">
                    <div className="ucms-admin__panel-heading">
                      <div>
                        <h3>Status history: {historyUser.first_name} {historyUser.last_name}</h3>
                        <p>{historyUser.email}</p>
                      </div>
                      <button className="ucms-admin__action ucms-admin__action--secondary" onClick={() => setHistoryUser(null)} type="button">
                        Close
                      </button>
                    </div>
                    {historyLoading ? (
                      <p className="ucms-admin__state" role="status">Loading status history...</p>
                    ) : userHistory.length ? (
                      <ul>
                        {userHistory.map((event) => (
                          <li key={event.id}>
                            <strong>{event.from_status || "Account created"} → {event.to_status}</strong>
                            <span>{new Date(event.changed_at).toLocaleString()}</span>
                            {event.reason && <p>{event.reason}</p>}
                          </li>
                        ))}
                      </ul>
                    ) : <p className="ucms-admin__state">No status changes recorded.</p>}
                  </section>
                )}
              </section>
            )}
          </>
        )}
      </main>
    </div>
  );
}
