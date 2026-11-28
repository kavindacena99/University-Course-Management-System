import React, { useState, useMemo } from "react";
import "./Admin_Dashboard.css";
import {
  Users,
  Bell,
  BookOpen,
  TrendingUp,
  Award,
  AlertTriangle,
  Check,
  Filter,
  Calendar,
  Mail,
  ChevronRight,
  Search,
  Menu,
  X,
} from "lucide-react";

interface CourseSummary {
  id: string;
  code: string;
  title: string;
  department: string;
  enrolled: number;
  status: "Active" | "Pending Review";
}

interface NoticeEntry {
  id: string;
  title: string;
  date: string;
  audience: string;
}

const INITIAL_COURSES: CourseSummary[] = [
  {
    id: "crs-1",
    code: "CS-301",
    title: "Database Management Systems",
    department: "Computing",
    enrolled: 145,
    status: "Active",
  },
  {
    id: "crs-2",
    code: "CS-304",
    title: "Computer Networks & Security",
    department: "Computing",
    enrolled: 128,
    status: "Active",
  },
  {
    id: "crs-3",
    code: "SE-402",
    title: "Distributed Systems Architecture",
    department: "Software Engineering",
    enrolled: 92,
    status: "Pending Review",
  },
];

const INITIAL_NOTICES: NoticeEntry[] = [
  {
    id: "ntc-1",
    title: "Semester Registration Deadline",
    date: "Dec 05, 2026",
    audience: "All Students",
  },
  {
    id: "ntc-2",
    title: "Board of Examiners Moderation",
    date: "Dec 12, 2026",
    audience: "Faculty & Admin",
  },
];

export function Admin_Dashboard() {
  const [activeTab, setActiveTab] = useState<"overview" | "courses" | "notices">("overview");
  const [searchQuery, setSearchQuery] = useState<string>("");
  const [departmentFilter, setDepartmentFilter] = useState<string>("All");
  const [sidebarOpen, setSidebarOpen] = useState<boolean>(false);
  const [courses, setCourses] = useState<CourseSummary[]>(INITIAL_COURSES);
  const [notices] = useState<NoticeEntry[]>(INITIAL_NOTICES);

  const departments = useMemo(() => {
    const unique = Array.from(new Set(courses.map((c) => c.department)));
    return ["All", ...unique];
  }, [courses]);

  const filteredCourses = useMemo(() => {
    const q = searchQuery.trim().toLowerCase();
    return courses.filter((course) => {
      const matchesDept =
        departmentFilter === "All" || course.department === departmentFilter;
      const matchesQuery =
        !q ||
        course.code.toLowerCase().includes(q) ||
        course.title.toLowerCase().includes(q) ||
        course.department.toLowerCase().includes(q);
      return matchesDept && matchesQuery;
    });
  }, [courses, departmentFilter, searchQuery]);

  const handleApproveCourse = (id: string) => {
    setCourses((prev) =>
      prev.map((item) =>
        item.id === id ? { ...item, status: "Active" } : item
      )
    );
  };

  return (
    <div className="admin-dashboard-shell">
      <aside
        className={`sidebar${sidebarOpen ? " sidebar--open" : ""}`}
        aria-label="Admin sidebar"
      >
        <div className="logo-card">
          <p>
            University Course
            <br />
            Management System
          </p>
        </div>

        <nav aria-label="Admin navigation">
          <button
            type="button"
            className={`nav-item${activeTab === "overview" ? " nav-item--active" : ""}`}
            onClick={() => {
              setActiveTab("overview");
              setSidebarOpen(false);
            }}
          >
            <TrendingUp size={17} />
            <span>Overview</span>
          </button>

          <button
            type="button"
            className={`nav-item${activeTab === "courses" ? " nav-item--active" : ""}`}
            onClick={() => {
              setActiveTab("courses");
              setSidebarOpen(false);
            }}
          >
            <BookOpen size={17} />
            <span>Courses</span>
          </button>

          <button
            type="button"
            className={`nav-item${activeTab === "notices" ? " nav-item--active" : ""}`}
            onClick={() => {
              setActiveTab("notices");
              setSidebarOpen(false);
            }}
          >
            <Bell size={17} />
            <span>Notices</span>
          </button>
        </nav>

        <button
          type="button"
          className="logout"
          onClick={() => {
            setActiveTab("overview");
            setSearchQuery("");
            setDepartmentFilter("All");
          }}
        >
          <span>Reset View</span>
        </button>
      </aside>

      <header className="topbar">
        <button
          type="button"
          className="menu-button"
          aria-label="Toggle navigation menu"
          onClick={() => setSidebarOpen((prev) => !prev)}
        >
          <span />
          <span />
          <span />
        </button>

        <label className="global-search">
          <Search size={16} />
          <input
            type="search"
            placeholder="Search courses, departments, or notices..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
          />
          {searchQuery && (
            <button
              type="button"
              aria-label="Clear search"
              onClick={() => setSearchQuery("")}
              style={{ background: "transparent", border: "none" }}
            >
              <X size={14} />
            </button>
          )}
        </label>

        <div className="topbar-actions">
          <div className="avatar" title="Administrator">
            AD
          </div>
        </div>
      </header>

      <main>
        <h1>Admin Dashboard</h1>

        <section className="stats-grid" aria-label="Key metrics">
          <div className="stat-card">
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
              <span style={{ fontSize: "13px", color: "#64748b", fontWeight: 600 }}>
                Total Students
              </span>
              <Users size={18} color="#0284c7" />
            </div>
            <strong style={{ display: "block", fontSize: "28px", marginTop: "10px" }}>
              1,240
            </strong>
          </div>

          <div className="stat-card">
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
              <span style={{ fontSize: "13px", color: "#64748b", fontWeight: 600 }}>
                Active Courses
              </span>
              <BookOpen size={18} color="#0d9488" />
            </div>
            <strong style={{ display: "block", fontSize: "28px", marginTop: "10px" }}>
              {courses.length}
            </strong>
          </div>

          <div className="stat-card">
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
              <span style={{ fontSize: "13px", color: "#64748b", fontWeight: 600 }}>
                Pass Rate
              </span>
              <Award size={18} color="#0284c7" />
            </div>
            <strong style={{ display: "block", fontSize: "28px", marginTop: "10px" }}>
              94.2%
            </strong>
          </div>

          <div className="stat-card">
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
              <span style={{ fontSize: "13px", color: "#64748b", fontWeight: 600 }}>
                Pending Approvals
              </span>
              <AlertTriangle size={18} color="#e11d48" />
            </div>
            <strong style={{ display: "block", fontSize: "28px", marginTop: "10px" }}>
              {courses.filter((c) => c.status === "Pending Review").length}
            </strong>
          </div>
        </section>

        {(activeTab === "overview" || activeTab === "courses") && (
          <section className="panel-card" style={{ marginBottom: "24px" }}>
            <div
              style={{
                display: "flex",
                flexWrap: "wrap",
                justifyContent: "space-between",
                alignItems: "center",
                gap: "12px",
                marginBottom: "16px",
              }}
            >
              <h2 style={{ margin: 0, fontSize: "18px", fontWeight: 700 }}>
                Course Directory
              </h2>

              <label style={{ display: "inline-flex", alignItems: "center", gap: "8px", fontSize: "13px" }}>
                <Filter size={15} />
                <select
                  value={departmentFilter}
                  onChange={(e) => setDepartmentFilter(e.target.value)}
                  style={{
                    height: "36px",
                    padding: "0 10px",
                    borderRadius: "8px",
                    border: "1px solid #cbd5e1",
                    background: "#f8fafc",
                  }}
                >
                  {departments.map((dept) => (
                    <option key={dept} value={dept}>
                      {dept}
                    </option>
                  ))}
                </select>
              </label>
            </div>

            <div className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>Code</th>
                    <th>Course Title</th>
                    <th>Department</th>
                    <th>Enrolled</th>
                    <th>Status</th>
                    <th>Action</th>
                  </tr>
                </thead>
                <tbody>
                  {filteredCourses.map((course) => (
                    <tr key={course.id}>
                      <td>
                        <strong>{course.code}</strong>
                      </td>
                      <td>{course.title}</td>
                      <td>{course.department}</td>
                      <td>{course.enrolled}</td>
                      <td>{course.status}</td>
                      <td>
                        {course.status === "Pending Review" ? (
                          <button
                            type="button"
                            className="primary-button"
                            style={{ height: "32px", padding: "0 12px", fontSize: "12px" }}
                            onClick={() => handleApproveCourse(course.id)}
                          >
                            <Check size={14} />
                            <span>Approve</span>
                          </button>
                        ) : (
                          <span style={{ color: "#0d9488", fontWeight: 600, fontSize: "12.5px" }}>
                            Verified
                          </span>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </section>
        )}

        {(activeTab === "overview" || activeTab === "notices") && (
          <section className="panel-card">
            <h2 style={{ margin: "0 0 16px 0", fontSize: "18px", fontWeight: 700 }}>
              Administrative Notices
            </h2>

            <div style={{ display: "flex", flexDirection: "column", gap: "12px" }}>
              {notices.map((notice) => (
                <div
                  key={notice.id}
                  style={{
                    display: "flex",
                    justifyContent: "space-between",
                    alignItems: "center",
                    gap: "12px",
                    padding: "12px 14px",
                    borderRadius: "10px",
                    background: "#f8fafc",
                    border: "1px solid #e2e8f0",
                  }}
                >
                  <div>
                    <div style={{ fontWeight: 600, fontSize: "14px" }}>{notice.title}</div>
                    <div style={{ fontSize: "12.5px", color: "#64748b", marginTop: "4px" }}>
                      <Calendar size={13} style={{ verticalAlign: "middle", marginRight: "4px" }} />
                      {notice.date} &bull;{" "}
                      <Mail size={13} style={{ verticalAlign: "middle", margin: "0 4px" }} />
                      {notice.audience}
                    </div>
                  </div>
                  <ChevronRight size={18} color="#64748b" />
                </div>
              ))}
            </div>
          </section>
        )}
      </main>
    </div>
  );
}

export default Admin_Dashboard;