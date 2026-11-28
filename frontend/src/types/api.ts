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
  status?: string;
  roles?: string[];
  permissions?: string[];
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
  status: "draft" | "published" | "archived";
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
