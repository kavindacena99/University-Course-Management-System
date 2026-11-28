import React, { useMemo, useRef, useState } from "react";
import "./StudentLearning_HubBrowseCatalog.css";

type HubTab = "enrolled" | "catalog" | "assignments";

interface CourseItem {
  id: string;
  code: string;
  title: string;
  instructor: string;
  description: string;
  seatsOccupied: number;
  seatsTotal: number;
  completion: number;
  enrolled: boolean;
}

interface AssignmentRecord {
  id: string;
  courseCode: string;
  title: string;
  feedback?: string;
  grade?: string;
  uploadedFileName?: string;
}

const INITIAL_COURSES: CourseItem[] = [
  {
    id: "course-1",
    code: "CS101",
    title: "Data Structures & Algorithm",
    instructor: "Dr ABC Herath",
    description: "Comprehensive study of stacks, Queues, Trees, and algorithmic complex",
    seatsOccupied: 38,
    seatsTotal: 45,
    completion: 65,
    enrolled: true,
  },
  {
    id: "course-2",
    code: "CS105",
    title: "Linear Algebra",
    instructor: "Dr ABC",
    description: "Vector spaces, Matrix transformation",
    seatsOccupied: 35,
    seatsTotal: 30,
    completion: 55,
    enrolled: true,
  },
  {
    id: "course-3",
    code: "CS101",
    title: "UI/UX Design",
    instructor: "Ms. K. Perera",
    description: "Building responses, Scalable interfaces, component design",
    seatsOccupied: 19,
    seatsTotal: 20,
    completion: 0,
    enrolled: false,
  },
];

const INITIAL_ASSIGNMENTS: AssignmentRecord[] = [
  {
    id: "asg-1",
    courseCode: "CS101",
    title: "Binary Tree Implementation",
    feedback: 'Feedback: "Excellent clean memory handling and comments."',
    grade: "Grade : 95/100",
  },
  {
    id: "asg-2",
    courseCode: "CS102",
    title: "Binary Tree Implementation",
  },
  {
    id: "asg-3",
    courseCode: "MATH201",
    title: "Matrix Proofs Assignment",
  },
];

const ACCEPTED_EXTENSIONS = [".pdf", ".docx", ".zip"];
const MAX_FILE_SIZE_BYTES = 100 * 1024 * 1024; // 100MB

export const StudentLearning_HubBrowseCatalog: React.FC = () => {
  const [activeNav, setActiveNav] = useState<"dashboard" | "catalog" | "assignment">("dashboard");
  const [activeTab, setActiveTab] = useState<HubTab>("enrolled");
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [profileMenuOpen, setProfileMenuOpen] = useState(false);

  const [globalSearch, setGlobalSearch] = useState("");
  const [courses, setCourses] = useState<CourseItem[]>(INITIAL_COURSES);
  const [assignments, setAssignments] = useState<AssignmentRecord[]>(INITIAL_ASSIGNMENTS);

  // Upload Assignment Modal State
  const [uploadTarget, setUploadTarget] = useState<AssignmentRecord | null>(null);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [isDragging, setIsDragging] = useState(false);
  const fileInputRef = useRef<HTMLInputElement | null>(null);

  const handleNavClick = (nav: "dashboard" | "catalog" | "assignment") => {
    setActiveNav(nav);
    setSidebarOpen(false);
    if (nav === "dashboard") setActiveTab("enrolled");
    if (nav === "catalog") setActiveTab("catalog");
    if (nav === "assignment") setActiveTab("assignments");
  };

  const handleToggleEnrollment = (courseId: string) => {
    setCourses((prev) =>
      prev.map((course) => {
        if (course.id !== courseId) return course;
        const nextEnrolled = !course.enrolled;
        return {
          ...course,
          enrolled: nextEnrolled,
          seatsOccupied: nextEnrolled
            ? course.seatsOccupied + 1
            : Math.max(0, course.seatsOccupied - 1),
          completion: nextEnrolled ? course.completion : 0,
        };
      })
    );
  };

  const openUploadModal = (assignment: AssignmentRecord) => {
    setUploadTarget(assignment);
    setSelectedFile(null);
    setUploadError(null);
    setIsDragging(false);
  };

  const closeUploadModal = () => {
    setUploadTarget(null);
    setSelectedFile(null);
    setUploadError(null);
    setIsDragging(false);
  };

  const validateAndSetFile = (file: File | undefined) => {
    if (!file) return;
    const lowerName = file.name.toLowerCase();
    const hasValidExt = ACCEPTED_EXTENSIONS.some((ext) => lowerName.endsWith(ext));

    if (!hasValidExt) {
      setUploadError("Only PDF, DOCX, or ZIP files are supported.");
      return;
    }
    if (file.size > MAX_FILE_SIZE_BYTES) {
      setUploadError("File size must be 100MB or less.");
      return;
    }

    setUploadError(null);
    setSelectedFile(file);
  };

  const handleUploadAndSend = () => {
    if (!uploadTarget) return;
    if (!selectedFile) {
      setUploadError("Please select or drag a file before sending.");
      return;
    }

    setAssignments((prev) =>
      prev.map((item) =>
        item.id === uploadTarget.id
          ? { ...item, uploadedFileName: selectedFile.name }
          : item
      )
    );
    closeUploadModal();
  };

  const enrolledCount = useMemo(
    () => courses.filter((c) => c.enrolled).length,
    [courses]
  );

  const normalizedQuery = globalSearch.trim().toLowerCase();

  const filteredEnrolled = useMemo(() => {
    return courses.filter((c) => {
      if (!c.enrolled) return false;
      if (!normalizedQuery) return true;
      return (
        c.code.toLowerCase().includes(normalizedQuery) ||
        c.title.toLowerCase().includes(normalizedQuery) ||
        c.instructor.toLowerCase().includes(normalizedQuery)
      );
    });
  }, [courses, normalizedQuery]);

  const filteredCatalog = useMemo(() => {
    if (!normalizedQuery) return courses;
    return courses.filter(
      (c) =>
        c.code.toLowerCase().includes(normalizedQuery) ||
        c.title.toLowerCase().includes(normalizedQuery) ||
        c.description.toLowerCase().includes(normalizedQuery)
    );
  }, [courses, normalizedQuery]);

  const filteredAssignments = useMemo(() => {
    if (!normalizedQuery) return assignments;
    return assignments.filter(
      (a) =>
        a.courseCode.toLowerCase().includes(normalizedQuery) ||
        a.title.toLowerCase().includes(normalizedQuery) ||
        (a.feedback && a.feedback.toLowerCase().includes(normalizedQuery))
    );
  }, [assignments, normalizedQuery]);

  return (
    <div className="app-shell student-learning-hub-browse-catalog-page">
      {sidebarOpen && (
        <div
          className="nav-scrim"
          onClick={() => setSidebarOpen(false)}
          aria-hidden="true"
        />
      )}

      {/* Sidebar */}
      <aside className={`sidebar ${sidebarOpen ? "sidebar--open" : ""}`}>
        <div className="logo-card">
          <div className="brand-badge" aria-hidden="true">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.9">
              <path d="M22 10L12 5 2 10l10 5 10-5z" />
              <path d="M6 12v5c0 1.66 2.69 3 6 3s6-1.34 6-3v-5" />
              <path d="M20 11v5" />
            </svg>
          </div>
          <p>
            University Course
            <br />
            Management System
          </p>
        </div>

        <nav aria-label="Primary Navigation">
          <button
            type="button"
            className={`nav-item ${activeNav === "dashboard" ? "nav-item--active" : ""}`}
            onClick={() => handleNavClick("dashboard")}
          >
            <span className="nav-icon" aria-hidden="true">
              <svg viewBox="0 0 24 24" fill="currentColor">
                <path d="M12 3.2 2.5 11h2.8v9.5h5.2v-6h3v6h5.2V11h2.8L12 3.2Z" />
              </svg>
            </span>
            <span>Dashboard</span>
          </button>

          <button
            type="button"
            className={`nav-item ${activeNav === "catalog" ? "nav-item--active" : ""}`}
            onClick={() => handleNavClick("catalog")}
          >
            <span className="nav-icon" aria-hidden="true">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.9">
                <circle cx="10.5" cy="10.5" r="7.5" />
                <path d="M3 10.5h15M10.5 3a11 11 0 0 1 0 15M10.5 3a11 11 0 0 0 0 15" />
                <circle cx="17.5" cy="17.5" r="3" fill="#ffffff" />
                <path d="m19.8 19.8 2.2 2.2" />
              </svg>
            </span>
            <span>Browse Catalog</span>
          </button>

          <button
            type="button"
            className={`nav-item ${activeNav === "assignment" ? "nav-item--active" : ""}`}
            onClick={() => handleNavClick("assignment")}
          >
            <span className="nav-icon" aria-hidden="true">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8">
                <rect x="4" y="3" width="12" height="16" rx="2" />
                <path d="M7.5 7.5h5M7.5 11h5M7.5 14.5h3" />
                <circle cx="17.5" cy="16.5" r="2.2" fill="#ffffff" />
                <path d="M14 21c.6-1.7 2-2.5 3.5-2.5s2.9.8 3.5 2.5" />
              </svg>
            </span>
            <span>Assignment</span>
          </button>
        </nav>

        <button
          type="button"
          className="logout"
          onClick={() => alert("Logged out")}
        >
          <span className="logout-icon" aria-hidden="true">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.4">
              <circle cx="12" cy="12" r="9" />
              <path d="M9 12h6m0 0-2.5-2.5M15 12l-2.5 2.5" />
            </svg>
          </span>
          <span>LOGOUT</span>
        </button>
      </aside>

      {/* Topbar */}
      <header className="topbar">
        <button
          type="button"
          className="menu-button"
          aria-label="Toggle Menu"
          onClick={() => setSidebarOpen((prev) => !prev)}
        >
          <span />
          <span />
          <span />
        </button>

        <label className="global-search">
          <span className="search-icon" aria-hidden="true">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <circle cx="11" cy="11" r="7" />
              <path d="m21 21-4.35-4.35" />
            </svg>
          </span>
          <input
            type="text"
            placeholder="Search Courses, Assignments, Results and Grades etc..."
            value={globalSearch}
            onChange={(e) => setGlobalSearch(e.target.value)}
          />
        </label>

        <div className="profile-wrap">
          <button
            type="button"
            className="profile"
            onClick={() => setProfileMenuOpen((prev) => !prev)}
          >
            <div className="avatar">DS</div>
            <div className="profile-copy">
              <strong>Dileeshi Sathsarani</strong>
              <small>Student ID: 3232</small>
            </div>
          </button>

          {profileMenuOpen && (
            <div className="profile-menu" role="menu">
              <button
                type="button"
                role="menuitem"
                onClick={() => {
                  setProfileMenuOpen(false);
                  handleNavClick("dashboard");
                }}
              >
                Enrolled Courses ({enrolledCount})
              </button>
              <button
                type="button"
                role="menuitem"
                onClick={() => {
                  setProfileMenuOpen(false);
                  handleNavClick("catalog");
                }}
              >
                Browse Catalog
              </button>
              <button
                type="button"
                role="menuitem"
                onClick={() => {
                  setProfileMenuOpen(false);
                  handleNavClick("assignment");
                }}
              >
                Assignments
              </button>
            </div>
          )}
        </div>
      </header>

      {/* Main Content */}
      <main
        className={`student-learning-hub-browse-catalog-main ${
          uploadTarget ? "is-modal-dimmed" : ""
        }`}
      >
        <section
          className="student-learning-hub-browse-catalog-card"
          aria-labelledby="hub-title"
        >
          <h1 id="hub-title" className="hub-title">
            Student Learning Hub
          </h1>

          {/* Tabs */}
          <div className="hub-tabs" role="tablist" aria-label="Student Learning Hub Sections">
            <button
              type="button"
              role="tab"
              aria-selected={activeTab === "enrolled"}
              className={`hub-tab ${activeTab === "enrolled" ? "hub-tab--active" : ""}`}
              onClick={() => {
                setActiveTab("enrolled");
                setActiveNav("dashboard");
              }}
            >
              Enrolled Courses({enrolledCount})
            </button>

            <button
              type="button"
              role="tab"
              aria-selected={activeTab === "catalog"}
              className={`hub-tab ${activeTab === "catalog" ? "hub-tab--active" : ""}`}
              onClick={() => {
                setActiveTab("catalog");
                setActiveNav("catalog");
              }}
            >
              Browse Catalog
            </button>

            <button
              type="button"
              role="tab"
              aria-selected={activeTab === "assignments"}
              className={`hub-tab ${activeTab === "assignments" ? "hub-tab--active" : ""}`}
              onClick={() => {
                setActiveTab("assignments");
                setActiveNav("assignment");
              }}
            >
              Assignments
            </button>
          </div>

          <hr className="hub-divider" />

          {/* Screen 1: Enrolled Courses */}
          {activeTab === "enrolled" && (
            <div className="enrolled-grid">
              {filteredEnrolled.length > 0 ? (
                filteredEnrolled.map((course) => (
                  <article key={course.id} className="enrolled-course-card">
                    <div className="enrolled-card-top">
                      <span className="course-code-badge">{course.code}</span>
                      <button
                        type="button"
                        className="drop-course-link"
                        onClick={() => handleToggleEnrollment(course.id)}
                      >
                        Drop Course
                      </button>
                    </div>

                    <h2 className="course-card-title">{course.title}</h2>
                    <p className="enrolled-course-instructor">
                      Instructor: {course.instructor}
                    </p>

                    <div className="completion-block">
                      <div className="completion-header">
                        <span>Course Completion</span>
                        <span className="completion-value">{course.completion}%</span>
                      </div>
                      <div
                        className="progress-track"
                        role="progressbar"
                        aria-valuenow={course.completion}
                        aria-valuemin={0}
                        aria-valuemax={100}
                      >
                        <div
                          className="progress-fill"
                          style={{ width: `${course.completion}%` }}
                        />
                      </div>
                    </div>
                  </article>
                ))
              ) : (
                <div className="hub-empty">No enrolled courses match your search.</div>
              )}
            </div>
          )}

          {/* Screen 2: Browse Catalog */}
          {activeTab === "catalog" && (
            <div className="catalog-grid">
              {filteredCatalog.length > 0 ? (
                filteredCatalog.map((course) => (
                  <article key={course.id} className="catalog-course-card">
                    <div className="catalog-card-top">
                      <span className="course-code-badge">{course.code}</span>
                      <span className="seats-label">
                        {course.seatsOccupied}/{course.seatsTotal} Seats
                      </span>
                    </div>

                    <h2 className="course-card-title">{course.title}</h2>
                    <p className="catalog-course-desc">{course.description}</p>

                    <button
                      type="button"
                      className={`catalog-action-btn ${
                        course.enrolled
                          ? "catalog-action-btn--drop"
                          : "catalog-action-btn--enroll"
                      }`}
                      onClick={() => handleToggleEnrollment(course.id)}
                    >
                      {course.enrolled ? "Drop Course" : "Enroll Now"}
                    </button>
                  </article>
                ))
              ) : (
                <div className="hub-empty">No catalog courses match your search.</div>
              )}
            </div>
          )}

          {/* Screen 3: Pending and Upcoming Assignments */}
          {activeTab === "assignments" && (
            <div className="upcoming-assignments-panel">
              <h2 className="upcoming-assignments-heading">
                Pending and Upcoming Assignments
              </h2>

              <div className="upcoming-assignments-list">
                {filteredAssignments.length > 0 ? (
                  filteredAssignments.map((item) => (
                    <article key={item.id} className="upcoming-assignment-card">
                      <div className="upcoming-assignment-left">
                        <span className="upcoming-course-code">{item.courseCode}</span>
                        <h3 className="upcoming-assignment-title">{item.title}</h3>
                        {item.feedback && (
                          <p className="upcoming-assignment-feedback">{item.feedback}</p>
                        )}
                        {item.uploadedFileName && !item.grade && (
                          <p className="upcoming-assignment-submitted">
                            Submitted: {item.uploadedFileName}
                          </p>
                        )}
                      </div>

                      <div className="upcoming-assignment-right">
                        {item.grade ? (
                          <span className="grade-pill">{item.grade}</span>
                        ) : (
                          <button
                            type="button"
                            className="upload-file-pill-btn"
                            onClick={() => openUploadModal(item)}
                          >
                            {item.uploadedFileName ? "Re-upload File" : "Upload File"}
                          </button>
                        )}
                      </div>
                    </article>
                  ))
                ) : (
                  <div className="hub-empty">No assignments match your search.</div>
                )}
              </div>
            </div>
          )}
        </section>
      </main>

      {/* Screen 4: Upload Assignment Modal */}
      {uploadTarget && (
        <div
          className="assignment-modal-backdrop"
          role="dialog"
          aria-modal="true"
          aria-labelledby="upload-modal-title"
          onClick={closeUploadModal}
        >
          <div className="assignment-modal-card" onClick={(e) => e.stopPropagation()}>
            <button
              type="button"
              className="assignment-modal-close"
              aria-label="Close modal"
              onClick={closeUploadModal}
            >
              ×
            </button>

            <h2 id="upload-modal-title" className="assignment-modal-title">
              Upload Assignment
            </h2>
            <p className="assignment-modal-subtitle">
              Submitting Coursework for {uploadTarget.courseCode}
            </p>

            <input
              ref={fileInputRef}
              type="file"
              accept=".pdf,.docx,.zip"
              className="sr-only"
              onChange={(e) => validateAndSetFile(e.target.files?.[0])}
            />

            <div
              role="button"
              tabIndex={0}
              className={`dropzone-box ${isDragging ? "dropzone-box--active" : ""}`}
              onClick={() => fileInputRef.current?.click()}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") {
                  e.preventDefault();
                  fileInputRef.current?.click();
                }
              }}
              onDragOver={(e) => {
                e.preventDefault();
                setIsDragging(true);
              }}
              onDragLeave={() => setIsDragging(false)}
              onDrop={(e) => {
                e.preventDefault();
                setIsDragging(false);
                validateAndSetFile(e.dataTransfer.files?.[0]);
              }}
            >
              <span className="dropzone-cloud" aria-hidden="true">
                <svg viewBox="0 0 24 24" fill="currentColor">
                  <path d="M18.5 19H7a4.5 4.5 0 0 1-.4-8.98A5.5 5.5 0 0 1 17.3 8.5 4 4 0 0 1 18.5 19ZM12 10.5l-3.5 3.5h2.3V17h2.4v-3h2.3L12 10.5Z" />
                </svg>
              </span>

              <span className="dropzone-primary">
                {selectedFile ? selectedFile.name : "Click to Browse or Drag File here"}
              </span>
              <span className="dropzone-secondary">PDF, DOCX or ZIP up to 100MB</span>
            </div>

            {uploadError && <p className="upload-error-text">{uploadError}</p>}

            <hr className="modal-footer-rule" />

            <div className="assignment-modal-actions">
              <button
                type="button"
                className="modal-cancel-btn"
                onClick={closeUploadModal}
              >
                Cancel
              </button>
              <button
                type="button"
                className="modal-submit-btn"
                onClick={handleUploadAndSend}
              >
                Upload and Send
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default StudentLearning_HubBrowseCatalog;