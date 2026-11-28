import type { Course, CourseList, CreateCourseInput } from "../types/api";
import { apiRequest } from "./api";

export function listCourses(search = "", limit = 50, offset = 0): Promise<CourseList> {
  const params = new URLSearchParams({ limit: String(limit), offset: String(offset) });
  if (search.trim()) params.set("search", search.trim());
  return apiRequest<CourseList>(`/api/v1/courses?${params.toString()}`);
}

export function createCourse(input: CreateCourseInput): Promise<{ course: Course }> {
  return apiRequest<{ course: Course }>("/api/v1/admin/courses", {
    method: "POST",
    body: JSON.stringify(input),
  });
}
