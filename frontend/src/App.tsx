import { useEffect, useState } from "react";
import type { ReactNode } from "react";
import { BrowserRouter as Router, Link, Navigate, Route, Routes } from "react-router-dom";
import type { UserSummary } from "./types/api";
import { getCurrentUser } from "./services/auth";
import Login from "./Pages/Login";
import Courses from "./Pages/Courses";
import LecModuleAssignment from "./Pages/LecModuleAssignment";
import LecViewCourse from "./Pages/LecViewCourse";
import LecCreateCourse from "./Pages/LecCreateCourse";
import LecAssignmentGrading from "./Pages/LecAssignmentGrading";
import LecAssignmentSubmissionSettings from "./Pages/LecAssignmentSubmissionSettings";
import LecDashboard from "./Pages/LecDashboard";
import Admin_Dashboard from "./Pages/Admin_Dashboard";
import StudentResults from "./Pages/StudentResults";
import StudentAssignments from "./Pages/StudentAssignments";
import StudentEnrollments from "./Pages/StudentEnrollments";
import AccountSettings from "./Pages/AccountSettings";
import AdminFinalResults from "./Pages/AdminFinalResults";
import {
  ActivateAccountPage,
  ForgotPasswordPage,
  RegisterPage,
  ResetPasswordPage,
} from "./Pages/AccountFlows";

function isHttpError(error: unknown): error is Error & { status: number } {
  return error instanceof Error && "status" in error && typeof error.status === "number";
}

function ProtectedPage({
  children,
  roles,
  permissions,
}: {
  children: ReactNode;
  roles?: string[];
  permissions?: string[];
}) {
  const [user, setUser] = useState<UserSummary | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    let active = true;
    getCurrentUser()
      .then((currentUser) => {
        if (active) setUser(currentUser);
      })
      .catch((requestError) => {
        if (active) {
          if (!isHttpError(requestError) || requestError.status !== 401) {
            setError(requestError instanceof Error ? requestError.message : "Unable to verify your session.");
          }
        }
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, []);

  if (loading) return <main className="route-state" role="status">Checking your session...</main>;
  if (error) {
    return (
      <main className="route-state">
        <p role="alert">{error}</p>
        <Link to="/login">Return to sign in</Link>
      </main>
    );
  }
  if (!user) return <Navigate to="/login" replace />;
  if (roles?.length && !roles.some((role) => user.roles.includes(role))) {
    return <Navigate to="/unauthorized" replace />;
  }
  if (permissions?.some((permission) => !user.permissions?.includes(permission))) {
    return <Navigate to="/unauthorized" replace />;
  }
  return children;
}

function NotFound() {
  return (
    <main className="route-state">
      <h1>Page not found</h1>
      <p>The requested page does not exist.</p>
      <Link to="/">Return to sign in</Link>
    </main>
  );
}

function Unauthorized() {
  return (
    <main className="route-state">
      <h1>Access denied</h1>
      <p>Your account does not have access to this page.</p>
      <Link to="/">Return to sign in</Link>
    </main>
  );
}

function protectedElement(element: ReactNode, roles?: string[], permissions?: string[]) {
  return <ProtectedPage roles={roles} permissions={permissions}>{element}</ProtectedPage>;
}

export default function App() {
  return (
    <Router>
      <Routes>
        <Route path="/" element={<Navigate to="/login" replace />} />
        <Route path="/login" element={<Login />} />
        <Route path="/register" element={<RegisterPage />} />
        <Route path="/forgot-password" element={<ForgotPasswordPage />} />
        <Route path="/reset-password" element={<ResetPasswordPage />} />
        <Route path="/activate" element={<ActivateAccountPage />} />

        <Route path="/courses" element={protectedElement(<Courses />)} />
        <Route path="/lecturer/courses" element={protectedElement(<Courses />)} />
        <Route path="/lecturer/courses/create" element={protectedElement(<LecCreateCourse />, ["ADMIN", "STAFF"])} />
        <Route path="/lecturer/courses/edit" element={protectedElement(<LecCreateCourse />, ["ADMIN", "STAFF"])} />
        <Route path="/lecturer/courses/view" element={protectedElement(<LecViewCourse />)} />
        <Route path="/lecturer/assignments" element={protectedElement(<LecModuleAssignment />, ["LECTURER", "ADMIN"])} />
        <Route path="/lecturer/submissions" element={protectedElement(<Navigate to="/lecturer/assignments" replace />, ["LECTURER", "ADMIN"])} />
        <Route path="/lecturer/dashboard" element={protectedElement(<LecDashboard />, ["LECTURER", "ADMIN"])} />
        <Route path="/student/results" element={protectedElement(<StudentResults />, ["STUDENT"])} />
        <Route path="/student/assignments" element={protectedElement(<StudentAssignments />, ["STUDENT"])} />
        <Route path="/student/enrollments" element={protectedElement(<StudentEnrollments />, ["STUDENT"])} />
        <Route path="/account" element={protectedElement(<AccountSettings />)} />
        <Route path="/admin" element={protectedElement(<Admin_Dashboard />, ["ADMIN"])} />
        <Route
          path="/admin/final-results"
          element={protectedElement(
            <AdminFinalResults />,
            ["ADMIN"],
            ["assessments:student:read", "assessments:results:import"],
          )}
        />

        <Route path="/LecModuleAssignment" element={protectedElement(<LecModuleAssignment />, ["LECTURER", "ADMIN"])} />
        <Route path="/LecAssignmentSubmission" element={protectedElement(<Navigate to="/lecturer/assignments" replace />, ["LECTURER", "ADMIN"])} />
        <Route path="/LecViewCourse" element={protectedElement(<LecViewCourse />)} />
        <Route path="/LecCreateCourse" element={protectedElement(<LecCreateCourse />, ["ADMIN", "STAFF"])} />
        <Route path="/LecMyCourses" element={protectedElement(<Courses />)} />
        <Route path="/AssignmentSubmissionSetting" element={protectedElement(<LecAssignmentSubmissionSettings />, ["LECTURER", "ADMIN"])} />
        <Route path="/LecAssignmentGrading" element={protectedElement(<LecAssignmentGrading />, ["LECTURER", "ADMIN"])} />
        <Route path="/LecAssignmentSubmissionSettings" element={protectedElement(<LecAssignmentSubmissionSettings />, ["LECTURER", "ADMIN"])} />
        <Route path="/LecDashboard" element={protectedElement(<LecDashboard />, ["LECTURER", "ADMIN"])} />
        <Route path="/Admin_Dashboard" element={<Navigate to="/admin" replace />} />

        <Route path="/unauthorized" element={<Unauthorized />} />
        <Route path="*" element={<NotFound />} />
      </Routes>
    </Router>
  );
}
