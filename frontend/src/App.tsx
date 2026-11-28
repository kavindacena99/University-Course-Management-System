// import { FormEvent, useState } from "react";

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

export default function App() {
  return (
    <Router>
      <Routes>
        <Route path="/" element={<Login />} />
        <Route path="/login" element={<Login />} />

        <Route path="/courses" element={<Courses />} />
        <Route path="/lecturer/courses" element={<Courses />} />
        <Route path="/lecturer/courses/create" element={<LecCreateCourse />} />
        <Route path="/lecturer/courses/view" element={<LecViewCourse />} />
        <Route path="/lecturer/assignments" element={<LecModuleAssignment />} />
        <Route path="/lecturer/submissions" element={<LecAssignmentSubmission />} />

        <Route path="/LecModuleAssignment" element={<LecModuleAssignment />} />
        <Route path="/LecAssignmentSubmission" element={<LecAssignmentSubmission />} />
        <Route path="/LecViewCourse" element={<LecViewCourse />} />
        <Route path="/LecCreateCourse" element={<LecCreateCourse />} />
        <Route path="/LecMyCourses" element={<Courses />} />

        {/* Lecturer My Courses Page */}
        <Route path="/LecMyCourses" element={<LecMyCourses />} />

        { /* AssignmentSubmissionSetting */}
        <Route path="/AssignmentSubmissionSetting" element={<AssignmentSubmissionSetting /> } />

      { /* LecAssignmentGrading */}
        <Route path="/LecAssignmentGrading" element={<LecAssignmentGrading /> } />
        
      { /* LecAssignmentSubmissionSettings */}
        <Route path="/LecAssignmentSubmissionSettings" element={<LecAssignmentSubmissionSettings /> } />                

      { /* LecDashboard */}
        <Route path="/LecDashboard" element={<LecDashboard /> } />       

        { /* AdminStudentLogin */}
        <Route path="/AdminStudentLogin" element={<AdminStudentLogin /> } />

        { /* Admin_Dashboard */}
        <Route path="/Admin_Dashboard" element={<Admin_Dashboard /> } />        
      </Routes>
    </Router>
  );
}
