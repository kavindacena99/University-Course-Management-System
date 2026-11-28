import { apiFile, apiRequest } from "./api";
import type { Course, CourseList } from "../types/api";
export type Page<T> = { total: number; limit: number; offset: number } & T;
export type Offering = {
  id: string;
  course_id: string;
  course_code: string;
  course_title: string;
  academic_year: string;
  semester: string;
  lecturer_user_id?: string;
  status: string;
  version: number;
  enrollment_capacity?: number;
};
export type Enrollment = {
  id: string;
  offering_id: string;
  student_user_id: string;
  status: string;
  version: number;
};
export type AllowedFile = { extension: string; media_type: string };
export type Assessment = {
  id: string;
  offering_id: string;
  title: string;
  description: string;
  component_kind: string;
  due_at: string;
  max_mark: number;
  weight_percent: number;
  max_attempts: number;
  late_policy: string;
  allowed_file_types: AllowedFile[];
  status: string;
  version: number;
};
export type Submission = {
  id: string;
  assessment_id: string;
  student_user_id: string;
  original_filename: string;
  size_bytes: number;
  status: string;
  submitted_at: string;
};
export type Grade = {
  id: string;
  submission_id: string;
  mark: number;
  feedback: string;
  status: string;
  version: number;
};
export const course = (id: string) => apiRequest<{ course: Course }>(`/api/v1/courses/${id}`);
export const courseOfferings = (id: string, offset = 0) =>
  apiRequest<Page<{ offerings: Offering[] }>>(
    `/api/v1/courses/${id}/offerings?limit=20&offset=${offset}`,
  );
export const offerings = (assigned = false, offset = 0) =>
  apiRequest<Page<{ offerings: Offering[] }>>(
    `/api/v1/offerings${assigned ? "/assigned" : ""}?limit=20&offset=${offset}`,
  );
export const offering = (id: string) =>
  apiRequest<{ offering: Offering }>(`/api/v1/offerings/${id}`);
export const enrollments = (studentId: string, offset = 0) =>
  apiRequest<Page<{ enrollments: Enrollment[] }>>(
    `/api/v1/students/${studentId}/enrollments?limit=20&offset=${offset}`,
  );
export const offeringEnrollments = (id: string, offset = 0) =>
  apiRequest<Page<{ enrollments: Enrollment[] }>>(
    `/api/v1/offerings/${id}/enrollments?limit=20&offset=${offset}`,
  );
export const enroll = (offeringId: string) =>
  apiRequest<{ enrollment: Enrollment }>("/api/v1/enrollments", {
    method: "POST",
    body: JSON.stringify({ offering_id: offeringId }),
  });
export const assessments = (id: string, offset = 0) =>
  apiRequest<Page<{ assessments: Assessment[] }>>(
    `/api/v1/offerings/${id}/assessments?limit=20&offset=${offset}`,
  );
export const assessment = (id: string) =>
  apiRequest<{ assessment: Assessment }>(`/api/v1/assessments/${id}`);
export type AssessmentInput = Pick<
  Assessment,
  | "title"
  | "description"
  | "component_kind"
  | "due_at"
  | "max_mark"
  | "weight_percent"
  | "max_attempts"
  | "late_policy"
  | "allowed_file_types"
>;
export const createAssessment = (id: string, input: AssessmentInput) =>
  apiRequest<{ assessment: Assessment }>(`/api/v1/offerings/${id}/assessments`, {
    method: "POST",
    body: JSON.stringify(input),
  });
export const publishAssessment = (id: string, version: number) =>
  apiRequest<{ assessment: Assessment }>(`/api/v1/assessments/${id}/publish`, {
    method: "POST",
    body: JSON.stringify({ version }),
  });
export const closeAssessment = (id: string, version: number) =>
  apiRequest<{ assessment: Assessment }>(`/api/v1/assessments/${id}/close`, {
    method: "POST",
    body: JSON.stringify({ version }),
  });
export const submissions = (id: string, offset = 0) =>
  apiRequest<Page<{ submissions: Submission[] }>>(
    `/api/v1/assessments/${id}/submissions?limit=20&offset=${offset}`,
  );
export const submission = (id: string) =>
  apiRequest<{ submission: Submission; grade?: Grade }>(`/api/v1/submissions/${id}`);
export async function submit(id: string, file: File) {
  const body = new FormData();
  body.set("file", file);
  return apiRequest<{ submission: Submission }>(`/api/v1/assessments/${id}/submissions`, {
    method: "POST",
    headers: { "Idempotency-Key": crypto.randomUUID() },
    body,
  });
}
export const download = (id: string) => apiFile(`/api/v1/submissions/${id}/file`);
export const saveGrade = (id: string, version: number, mark: number, feedback: string) =>
  apiRequest<{ grade: Grade }>(`/api/v1/submissions/${id}/grade`, {
    method: "PUT",
    body: JSON.stringify({ version, mark, feedback }),
  });
export const releaseGrade = (id: string, version: number) =>
  apiRequest<{ grade: Grade }>(`/api/v1/submissions/${id}/release`, {
    method: "POST",
    body: JSON.stringify({ version }),
  });
export type StudentReport = {
  gpa_status: string;
  cumulative_gpa?: number;
  terms: { academic_year: string; semester: string; gpa?: number }[];
  results: { offering_id: string; percent: number; grade_code?: string }[];
  component_grades: { assessment_id: string; mark: number }[];
};
export type LecturerReport = {
  total_results: number;
  results: { student_user_id: string; percent: number; grade_code?: string }[];
  component_grades: unknown[];
};
export type AdminSummary = {
  total_results: number;
  average_percent?: number;
  grade_distribution: { grade_code: string; count: number }[];
};
export const studentReport = () =>
  apiRequest<{ report: StudentReport }>("/api/v1/analytics/students/me");
export const lecturerReport = (id: string) =>
  apiRequest<{ report: LecturerReport }>(`/api/v1/analytics/lecturer/offerings/${id}`);
export const adminSummary = () =>
  apiRequest<{ summary: AdminSummary }>("/api/v1/analytics/admin/summary");
export const allCourses = (offset = 0) =>
  apiRequest<CourseList>(`/api/v1/courses?limit=20&offset=${offset}`);
