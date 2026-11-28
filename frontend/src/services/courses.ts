import type {
  Course,
  CourseList,
  CourseOffering,
  CourseOfferingList,
  CreateCourseInput,
  CreateOfferingInput,
  UpdateCourseInput,
  UpdateOfferingInput,
} from "../types/api";
import { apiRequest } from "./api";

export function listCourses(search = "", limit = 50, offset = 0): Promise<CourseList> {
  const params = new URLSearchParams({ limit: String(limit), offset: String(offset) });
  if (search.trim()) params.set("search", search.trim());
  return apiRequest<CourseList>(`/api/v1/courses?${params.toString()}`);
}

export function listPublishedOfferings(): Promise<CourseOfferingList> {
  const params = new URLSearchParams({
    status: "published",
    limit: "100",
    offset: "0",
  });
  return apiRequest(`/api/v1/offerings?${params.toString()}`);
}

export function createCourse(input: CreateCourseInput): Promise<{ course: Course }> {
  return apiRequest<{ course: Course }>("/api/v1/admin/courses", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function updateCourse(
  courseId: string,
  input: UpdateCourseInput,
): Promise<{ course: Course }> {
  return apiRequest<{ course: Course }>(
    `/api/v1/admin/courses/${encodeURIComponent(courseId)}`,
    {
      method: "PUT",
      body: JSON.stringify(input),
    },
  );
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

export function createOffering(
  input: CreateOfferingInput,
): Promise<{ offering: CourseOffering }> {
  return apiRequest("/api/v1/admin/offerings", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function updateOffering(
  offeringId: string,
  input: UpdateOfferingInput,
): Promise<{ offering: CourseOffering }> {
  return apiRequest(`/api/v1/offerings/${encodeURIComponent(offeringId)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export function assignOfferingLecturer(
  offeringId: string,
  version: number,
  lecturerUserId: string,
): Promise<{ offering: CourseOffering }> {
  return apiRequest(
    `/api/v1/admin/offerings/${encodeURIComponent(offeringId)}/lecturer`,
    {
      method: "PUT",
      body: JSON.stringify({ version, lecturer_user_id: lecturerUserId }),
    },
  );
}

export function changeOfferingStatus(
  offering: CourseOffering,
  action: "publish" | "close" | "archive" | "restore",
): Promise<{ offering: CourseOffering }> {
  const path =
    action === "restore"
      ? `/api/v1/admin/offerings/${encodeURIComponent(offering.id)}/restore`
      : `/api/v1/offerings/${encodeURIComponent(offering.id)}/${action}`;
  return apiRequest(path, {
    method: "POST",
    body: JSON.stringify({ version: offering.version }),
  });
}
