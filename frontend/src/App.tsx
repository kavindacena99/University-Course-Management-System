import { BrowserRouter, Navigate, Route, Routes, Link } from "react-router-dom";
import { SessionProvider, Protected } from "./session";
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
import {
  AssessmentPage,
  CoursePage,
  CoursesPage,
  CreateCoursePage,
  LecturerAnalyticsPage,
  LecturerOfferingsPage,
  OfferingPage,
  OfferingsPage,
  StudentSubmissionPage,
  SubmissionPage,
} from "./Pages/Workflows";

function NotFound() {
  return (
    <main className="route-state">
      <h1>Page not found</h1>
      <p>The requested page does not exist.</p>
      <Link to="/courses">Return to courses</Link>
    </main>
  );
}

export default function App() {
  return (
    <BrowserRouter>
      <SessionProvider>
        <Routes>
          <Route path="/" element={<Navigate to="/login" replace />} />
          <Route path="/login" element={<Login />} />
          <Route path="/register" element={<RegisterPage />} />
          <Route path="/forgot-password" element={<ForgotPasswordPage />} />
          <Route path="/reset-password" element={<ResetPasswordPage />} />
          <Route path="/activate" element={<ActivateAccountPage />} />

          <Route
            path="/courses"
            element={
              <Protected>
                <CoursesPage />
              </Protected>
            }
          />
          <Route
            path="/courses/:id"
            element={
              <Protected>
                <CoursePage />
              </Protected>
            }
          />
          <Route
            path="/catalog"
            element={
              <Protected>
                <Courses />
              </Protected>
            }
          />
          <Route
            path="/lecturer/courses"
            element={
              <Protected>
                <Courses />
              </Protected>
            }
          />
          <Route
            path="/lecturer/courses/create"
            element={
              <Protected roles={["ADMIN", "STAFF"]}>
                <LecCreateCourse />
              </Protected>
            }
          />
          <Route
            path="/lecturer/courses/edit"
            element={
              <Protected roles={["ADMIN", "STAFF"]}>
                <LecCreateCourse />
              </Protected>
            }
          />
          <Route
            path="/lecturer/courses/view"
            element={
              <Protected>
                <LecViewCourse />
              </Protected>
            }
          />

          <Route
            path="/offerings"
            element={
              <Protected>
                <OfferingsPage />
              </Protected>
            }
          />
          <Route
            path="/offerings/:id"
            element={
              <Protected>
                <OfferingPage />
              </Protected>
            }
          />
          <Route
            path="/lecturer/offerings"
            element={
              <Protected role="LECTURER">
                <LecturerOfferingsPage />
              </Protected>
            }
          />
          <Route
            path="/lecturer/offerings/:id/analytics"
            element={
              <Protected role="LECTURER" permission="analytics:lecturer:read">
                <LecturerAnalyticsPage />
              </Protected>
            }
          />
          <Route
            path="/lecturer/offerings/:id"
            element={
              <Protected role="LECTURER">
                <OfferingPage lecturer />
              </Protected>
            }
          />

          <Route
            path="/student/enrollments"
            element={
              <Protected role="STUDENT">
                <StudentEnrollments />
              </Protected>
            }
          />
          <Route
            path="/student/enrolments"
            element={
              <Protected role="STUDENT">
                <StudentEnrollments />
              </Protected>
            }
          />
          <Route
            path="/student/assignments"
            element={
              <Protected role="STUDENT" permission="assessments:submit">
                <StudentAssignments />
              </Protected>
            }
          />
          <Route
            path="/student/results"
            element={
              <Protected role="STUDENT" permission="analytics:student:read">
                <StudentResults />
              </Protected>
            }
          />
          <Route
            path="/student/assessments/:id"
            element={
              <Protected role="STUDENT" permission="assessments:submit">
                <AssessmentPage />
              </Protected>
            }
          />
          <Route
            path="/student/submissions/:id"
            element={
              <Protected role="STUDENT">
                <StudentSubmissionPage />
              </Protected>
            }
          />
          <Route path="/student/report" element={<Navigate to="/student/results" replace />} />

          <Route
            path="/lecturer/dashboard"
            element={
              <Protected roles={["LECTURER", "ADMIN"]}>
                <LecDashboard />
              </Protected>
            }
          />
          <Route
            path="/lecturer/assignments"
            element={
              <Protected roles={["LECTURER", "ADMIN"]} permission="assessments:read">
                <LecModuleAssignment />
              </Protected>
            }
          />
          <Route
            path="/lecturer/assessments/create"
            element={
              <Protected roles={["LECTURER", "ADMIN"]} permission="assessments:manage">
                <LecAssignmentSubmissionSettings />
              </Protected>
            }
          />
          <Route
            path="/lecturer/assessments/:id"
            element={
              <Protected roles={["LECTURER", "ADMIN"]} permission="assessments:read">
                <AssessmentPage lecturer />
              </Protected>
            }
          />
          <Route
            path="/lecturer/grading"
            element={
              <Protected roles={["LECTURER", "ADMIN"]} permission="assessments:grade">
                <LecAssignmentGrading />
              </Protected>
            }
          />
          <Route
            path="/lecturer/submissions/:id"
            element={
              <Protected roles={["LECTURER", "ADMIN"]} permission="assessments:grade">
                <SubmissionPage />
              </Protected>
            }
          />
          <Route
            path="/lecturer/submissions"
            element={
              <Protected roles={["LECTURER", "ADMIN"]}>
                <Navigate to="/lecturer/assignments" replace />
              </Protected>
            }
          />

          <Route
            path="/admin"
            element={<Navigate to="/admin/dashboard" replace />}
          />
          <Route
            path="/admin/dashboard"
            element={
              <Protected role="ADMIN">
                <Admin_Dashboard />
              </Protected>
            }
          />
          <Route
            path="/admin/courses/create"
            element={
              <Protected permission="courses:manage">
                <CreateCoursePage />
              </Protected>
            }
          />
          <Route
            path="/admin/final-results"
            element={
              <Protected
                role="ADMIN"
                permissions={["assessments:student:read", "assessments:results:import"]}
              >
                <AdminFinalResults />
              </Protected>
            }
          />
          <Route
            path="/account"
            element={
              <Protected>
                <AccountSettings />
              </Protected>
            }
          />

          <Route
            path="/LecModuleAssignment"
            element={
              <Protected roles={["LECTURER", "ADMIN"]}>
                <LecModuleAssignment />
              </Protected>
            }
          />
          <Route
            path="/LecAssignmentSubmission"
            element={<Navigate to="/lecturer/assignments" replace />}
          />
          <Route path="/LecViewCourse" element={<Protected><LecViewCourse /></Protected>} />
          <Route
            path="/LecCreateCourse"
            element={
              <Protected roles={["ADMIN", "STAFF"]}>
                <LecCreateCourse />
              </Protected>
            }
          />
          <Route path="/LecMyCourses" element={<Protected><Courses /></Protected>} />
          <Route
            path="/AssignmentSubmissionSetting"
            element={
              <Protected roles={["LECTURER", "ADMIN"]} permission="assessments:manage">
                <LecAssignmentSubmissionSettings />
              </Protected>
            }
          />
          <Route
            path="/LecAssignmentGrading"
            element={
              <Protected roles={["LECTURER", "ADMIN"]} permission="assessments:grade">
                <LecAssignmentGrading />
              </Protected>
            }
          />
          <Route
            path="/LecAssignmentSubmissionSettings"
            element={
              <Protected roles={["LECTURER", "ADMIN"]} permission="assessments:manage">
                <LecAssignmentSubmissionSettings />
              </Protected>
            }
          />
          <Route
            path="/LecDashboard"
            element={
              <Protected roles={["LECTURER", "ADMIN"]}>
                <LecDashboard />
              </Protected>
            }
          />
          <Route path="/Admin_Dashboard" element={<Navigate to="/admin/dashboard" replace />} />
          <Route path="*" element={<NotFound />} />
        </Routes>
      </SessionProvider>
    </BrowserRouter>
  );
}
