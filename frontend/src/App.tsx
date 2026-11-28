import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import Login from "./Pages/Login";
import { SessionProvider, Protected } from "./session";
import {
  AdminPage,
  AssessmentPage,
  CoursePage,
  CoursesPage,
  CreateAssessmentPage,
  CreateCoursePage,
  LecturerAnalyticsPage,
  LecturerOfferingsPage,
  OfferingPage,
  OfferingsPage,
  StudentEnrollmentsPage,
  StudentReportPage,
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
          <Route
            path="/student/report"
            element={
              <Protected role="STUDENT" permission="analytics:student:read">
                <StudentReportPage />
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
            element={
              <Protected role="LECTURER" permission="assessments:manage">
                <CreateAssessmentPage />
              </Protected>
            }
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
                <AdminPage />
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
