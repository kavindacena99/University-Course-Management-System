import type {
  Course,
  CourseList,
  CourseOfferingList,
  CreateCourseInput,
} from "../types/api";
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

export function getCourse(courseId: string): Promise<{ course: Course }> {
  return apiRequest(`/api/v1/courses/${encodeURIComponent(courseId)}`);
}

export function listCourseOfferings(courseId: string): Promise<CourseOfferingList> {
  const params = new URLSearchParams({ limit: "100", offset: "0" });
  return apiRequest(
    `/api/v1/courses/${encodeURIComponent(courseId)}/offerings?${params.toString()}`,
  );
}
