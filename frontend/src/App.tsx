import { BrowserRouter as Router, Routes, Route } from "react-router-dom";
import Login from "./Pages/Login";
import Courses from "./Pages/Courses";
import LecModuleAssignment from "./Pages/LecModuleAssignment";
import LecAssignmentSubmission from "./Pages/LecAssignmentSubmission";
import LecViewCourse from "./Pages/LecViewCourse";
import LecCreateCourse from "./Pages/LecCreateCourse";

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

        <Route path="*" element={<Login />} />
      </Routes>
    </Router>
  );
}
