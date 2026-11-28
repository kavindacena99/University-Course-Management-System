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

// const assetPathPrefix = "/assets";

// type Notice = {
//   tone: "success" | "info" | "error";
//   text: string;
// } | null;

// export default function App() {
//   const [notice, setNotice] = useState<Notice>(null);

//   function handleSubmit(event: FormEvent<HTMLFormElement>) {
//     event.preventDefault();
//     const form = new FormData(event.currentTarget);
//     const username = String(form.get("username") ?? "").trim();
//     const password = String(form.get("password") ?? "");

//     if (!username || !password) {
//       setNotice({ tone: "error", text: "Enter your user name and password." });
//       return;
//     }

//     setNotice({
//       tone: "success",
//       text: `Welcome back, ${username}. Your sign-in details are ready.`,
//     });
//   }

//   return (
//     <main className="login-page">
//       <section className="login-card" aria-labelledby="page-title">
//         <header className="brand">
//           <h1 id="page-title">University Course Management System</h1>
//           <p>Connecting Minds, Shaping Tomorrow</p>
//         </header>

//         <div className="illustration-panel" aria-hidden="true">
//           <img className="illustration-backdrop" src={`${assetPathPrefix}/804da.svg`} alt="" />
//           <img className="illustration" src={`${assetPathPrefix}/23ae0.png`} alt="" />
//         </div>

//         <div className="form-panel">
//           <div className="welcome-copy">
//             <h2>WELCOME</h2>
//             <p>Enter your User Name and Password to access your account</p>
//           </div>

//           <form onSubmit={handleSubmit} noValidate>
//             <div className="field-group">
//               <label htmlFor="username">User Name</label>
//               <div className="field-row">
//                 <img src={`${assetPathPrefix}/eed30.png`} alt="" />
//                 <input
//                   id="username"
//                   name="username"
//                   type="text"
//                   autoComplete="username"
//                   placeholder="Enter your user name"
//                   aria-describedby="form-notice"
//                 />
//               </div>
//             </div>

//             <div className="field-group">
//               <label htmlFor="password">Password</label>
//               <div className="field-row">
//                 <img src={`${assetPathPrefix}/78c8c.png`} alt="" />
//                 <input
//                   id="password"
//                   name="password"
//                   type="password"
//                   autoComplete="current-password"
//                   placeholder="Enter your password"
//                   aria-describedby="form-notice"
//                 />
//               </div>
//             </div>

//             <div className="form-options">
//               <label className="remember">
//                 <input name="remember" type="checkbox" />
//                 <span>Remember me</span>
//               </label>
//               <button
//                 className="text-button forgot"
//                 type="button"
//                 onClick={() =>
//                   setNotice({
//                     tone: "info",
//                     text: "Password recovery instructions will be sent to your university email.",
//                   })
//                 }
//               >
//                 Forgot Password
//               </button>
//             </div>

//             <button className="primary-button" type="submit">
//               Sign In
//             </button>

//             <button
//               className="google-button"
//               type="button"
//               onClick={() =>
//                 setNotice({
//                   tone: "info",
//                   text: "Google sign-in is ready to connect to your university account.",
//                 })
//               }
//             >
//               <img src={`${assetPathPrefix}/027c9.png`} alt="" />
//               <span>Sign In with Google</span>
//             </button>

//             <div
//               id="form-notice"
//               className={`notice${notice ? ` notice-${notice.tone}` : ""}`}
//               role="status"
//               aria-live="polite"
//             >
//               {notice?.text ?? ""}
//             </div>
//           </form>

//           <p className="signup-copy">
//             <span>Don’t have an account?</span>{" "}
//             <button
//               className="text-button"
//               type="button"
//               onClick={() =>
//                 setNotice({
//                   tone: "info",
//                   text: "Account registration is ready for new students and faculty.",
//                 })
//               }
//             >
//               Sign up
//             </button>
//           </p>
//         </div>
//       </section>
//     </main>
//   );
// }

import { BrowserRouter as Router, Routes, Route } from 'react-router-dom';
import Login from './Pages/Login';
import Courses from './Pages/Courses';
import LecModuleAssignment from './Pages/LecModuleAssignment';
import LecAssignmentSubmission from './Pages/LecAssignmentSubmission';
import LecViewCourse from './Pages/LecViewCourse';
import LecCreateCourse from './Pages/LecCreateCourse';
import LecMyCourses from './Pages/LecMyCourses';
import AssignmentSubmissionSetting from './Pages/AssignmentSubmissionSetting';
import LecAssignmentGrading from './Pages/LecAssignmentGrading';
import LecAssignmentSubmissionSettings from './Pages/LecAssignmentSubmissionSettings';
import LecDashboard from './Pages/LecDashboard';
import AdminStudentLogin from './Pages/AdminStudentLogin';
import Admin_Dashboard from './Pages/Admin_Dashboard';
import StudentLearning_HubBrowseCatalog from './Pages/StudentLearning_HubBrowseCatalog';

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

        { /* Admin_Dashboard */}
        <Route path="/Admin_Dashboard" element={<Admin_Dashboard /> } />        

        { /* StudentLearning_HubBrowseCatalog */}
        <Route path="/StudentLearning_HubBrowseCatalog" element={<StudentLearning_HubBrowseCatalog /> } />  

      </Routes>
    </Router>
  );
}
