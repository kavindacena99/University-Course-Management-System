import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import Login from "./Pages/Login";
import StudentAssignments from "./Pages/StudentAssignments";
import StudentResults from "./Pages/StudentResults";
import LecturerDashboard from "./Pages/LecDashboard";
import LecturerAssignments from "./Pages/LecModuleAssignment";
import LecturerAssessmentCreate from "./Pages/LecAssignmentSubmissionSettings";
import LecturerGrading from "./Pages/LecAssignmentGrading";
import AdministratorDashboard from "./Pages/Admin_Dashboard";
import {
  ActivateAccountPage,
  ForgotPasswordPage,
  RegisterPage,
  ResetPasswordPage,
} from "./Pages/AccountFlows";
import { SessionProvider, Protected } from "./session";
import {
  AssessmentPage,
  CoursePage,
  CoursesPage,
  CreateCoursePage,
  LecturerAnalyticsPage,
  LecturerOfferingsPage,
  OfferingPage,
  OfferingsPage,
  StudentEnrollmentsPage,
  StudentSubmissionPage,
  SubmissionPage,
} from "./Pages/Workflows";

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
          <Route path="/lecturer/courses" element={<Navigate to="/lecturer/offerings" replace />} />
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
            path="/student/enrollments"
            element={
              <Protected role="STUDENT">
                <StudentEnrollmentsPage />
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
            path="/lecturer/offerings"
            element={
              <Protected role="LECTURER">
                <LecturerOfferingsPage />
              </Protected>
            }
          />
          <Route
            path="/lecturer/dashboard"
            element={
              <Protected role="LECTURER">
                <LecturerDashboard />
              </Protected>
            }
          />
          <Route
            path="/lecturer/assignments"
            element={
              <Protected role="LECTURER" permission="assessments:read">
                <LecturerAssignments />
              </Protected>
            }
          />
          <Route
            path="/lecturer/assessments/create"
            element={
              <Protected role="LECTURER" permission="assessments:manage">
                <LecturerAssessmentCreate />
              </Protected>
            }
          />
          <Route
            path="/lecturer/grading"
            element={
              <Protected role="LECTURER" permission="assessments:grade">
                <LecturerGrading />
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
            path="/lecturer/offerings/:id/analytics"
            element={
              <Protected role="LECTURER" permission="analytics:lecturer:read">
                <LecturerAnalyticsPage />
              </Protected>
            }
          />
          <Route
            path="/lecturer/offerings/:id/assessments/create"
            element={<Navigate to="/lecturer/assessments/create" replace />}
          />
          <Route
            path="/lecturer/assessments/:id"
            element={
              <Protected role="LECTURER" permission="assessments:read">
                <AssessmentPage lecturer />
              </Protected>
            }
          />
          <Route
            path="/lecturer/submissions/:id"
            element={
              <Protected role="LECTURER" permission="assessments:grade">
                <SubmissionPage />
              </Protected>
            }
          />
          <Route
            path="/admin/dashboard"
            element={
              <Protected role="ADMIN" permission="analytics:admin:read">
                <AdministratorDashboard />
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
          <Route path="*" element={<Navigate to="/courses" replace />} />
        </Routes>
      </SessionProvider>
    </BrowserRouter>
  );
}
