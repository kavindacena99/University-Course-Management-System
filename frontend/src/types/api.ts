export type ApiError = {
  code: string;
  message: string;
  request_id?: string;
};

export type UserSummary = {
  id: string;
  email: string;
  first_name: string;
  last_name: string;
  registration_status?: "pending" | "approved" | "rejected" | "suspended";
  roles: string[];
  permissions?: string[];
};

export type AccountProfile = UserSummary & {
  version: number;
  status_reason?: string;
  created_at?: string;
  updated_at?: string;
};

export type UserSession = {
  id: string;
  user_id: string;
  created_at: string;
  last_used_at: string;
  absolute_expires_at: string;
  revoked_at?: string | null;
  user_agent?: string;
  ip_address?: string;
  current: boolean;
};

export type AuthResponse = {
  user: UserSummary;
  access_token?: string;
  refresh_token?: string;
  token_type: "Bearer";
  expires_in: number;
  refresh_expires_in: number;
  csrf_token?: string;
};

export type Course = {
  id: string;
  code: string;
  title: string;
  description: string;
  credits: number;
  status: "draft" | "active" | "archived";
  version: number;
  created_by: string;
  updated_by: string;
  created_at: string;
  updated_at: string;
  archived_at?: string | null;
};

export type CourseList = {
  courses: Course[];
  total: number;
  limit: number;
  offset: number;
};

export type CreateCourseInput = {
  code: string;
  title: string;
  description: string;
  credits: number;
};

export type UpdateCourseInput = CreateCourseInput & {
  version: number;
};

export type CourseOffering = {
  id: string;
  course_id: string;
  course_code: string;
  course_title: string;
  academic_year: string;
  semester: string;
  start_date: string;
  end_date: string;
  enrollment_opens_at?: string | null;
  enrollment_closes_at?: string | null;
  enrollment_capacity?: number | null;
  lecturer_user_id?: string | null;
  status: "draft" | "published" | "closed" | "archived";
  version: number;
};

export type CreateOfferingInput = {
  course_id: string;
  academic_year: string;
  semester: string;
  start_date: string;
  end_date: string;
  enrollment_opens_at?: string | null;
  enrollment_closes_at?: string | null;
  enrollment_capacity?: number | null;
  lecturer_user_id?: string | null;
};

export type UpdateOfferingInput = Omit<
  CreateOfferingInput,
  "course_id" | "lecturer_user_id"
> & {
  version: number;
};

export type CourseOfferingList = {
  offerings: CourseOffering[];
  total: number;
  limit: number;
  offset: number;
};

export type Enrollment = {
  id: string;
  student_user_id: string;
  offering_id: string;
  status: "active" | "cancelled";
  version: number;
  enrolled_at: string;
};

export type EnrollmentList = {
  enrollments: Enrollment[];
  total: number;
  limit: number;
  offset: number;
};

export type AssessmentStatus = "draft" | "published" | "closed";

export type Assessment = {
  id: string;
  offering_id: string;
  title: string;
  description: string;
  component_kind: "ca" | "final_exam" | "other";
  due_at: string;
  max_mark: number;
  weight_percent: number;
  max_attempts: number;
  late_policy: "reject" | "accept";
  allowed_file_types: Array<{ extension: string; media_type: string }>;
  status: AssessmentStatus;
  version: number;
};

export type AssessmentList = {
  assessments: Assessment[];
  total: number;
  limit: number;
  offset: number;
};

export type Submission = {
  id: string;
  assessment_id: string;
  student_user_id: string;
  attempt_no: number;
  original_filename: string;
  media_type: string;
  size_bytes: number;
  status: "submitted" | "late";
  submitted_at: string;
};

export type SubmissionList = {
  submissions: Submission[];
  total: number;
  limit: number;
  offset: number;
};

export type Grade = {
  id: string;
  submission_id: string;
  graded_by_user_id: string;
  mark: number;
  feedback: string;
  status: "draft" | "released";
  version: number;
  graded_at: string;
  released_at?: string;
};

export type FinalResult = {
  id: string;
  offering_id: string;
  student_user_id: string;
  final_mark: number;
  max_mark: number;
  version: number;
  import_batch_id: string;
  imported_by_user_id: string;
  imported_at: string;
};

export type SubmissionDetail = {
  submission: Submission;
  grade: Grade | null;
};

export type AdminAnalyticsSummary = {
  total_results: number;
  average_percent?: number | null;
  grade_distribution: Array<{ grade_code: string; count: number }>;
  programme_breakdown: Array<{
    programme_code: string;
    programme_name: string;
    category: string;
    total_results: number;
    average_percent: number;
  }>;
};

export type StudentReport = {
  student_user_id: string;
  programme_code?: string;
  gpa_status: string;
  cumulative_gpa?: number | null;
  terms: Array<{
    academic_year: string;
    semester: string;
    gpa?: number | null;
    course_count: number;
  }>;
  results: Array<{
    offering_id: string;
    course_id: string;
    academic_year: string;
    semester: string;
    credits: number;
    final_mark: number;
    max_mark: number;
    percent: number;
    grade_code?: string;
    grade_points?: number | null;
    imported_at: string;
  }>;
  component_grades: Array<{
    grade_id: string;
    submission_id: string;
    assessment_id: string;
    offering_id: string;
    student_user_id: string;
    attempt_no: number;
    component_kind: string;
    mark: number;
    max_mark: number;
    weight_percent: number;
    released_at: string;
  }>;
};

export type AdminUser = {
  id: string;
  email: string;
  first_name: string;
  last_name: string;
  registration_status: "pending" | "approved" | "rejected" | "suspended";
  version: number;
  created_at: string;
};

export type AdminUserStatusHistory = {
  id: number;
  user_id: string;
  from_status?: AdminUser["registration_status"] | null;
  to_status: AdminUser["registration_status"];
  reason?: string;
  changed_by: string;
  version: number;
  changed_at: string;
};

export type AdminUserList = {
  users: AdminUser[];
  total: number;
  limit: number;
  offset: number;
};
