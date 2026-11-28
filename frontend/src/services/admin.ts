import type {
  AdminAnalyticsSummary,
  AdminUser,
  AdminUserStatusHistory,
  AdminUserList,
  Course,
} from "../types/api";
import { apiRequest } from "./api";
import { listCourses } from "./courses";

export function getAdminSummary(): Promise<{ summary: AdminAnalyticsSummary }> {
  return apiRequest("/api/v1/analytics/admin/summary");
}

export function listAdminCourses() {
  return listCourses("", 100, 0);
}

export function changeCourseStatus(
  course: Course,
  action: "publish" | "archive" | "restore",
): Promise<{ course: Course }> {
  return apiRequest(`/api/v1/admin/courses/${encodeURIComponent(course.id)}/${action}`, {
    method: "POST",
    body: JSON.stringify({ version: course.version }),
  });
}

export function listPendingUsers(search = ""): Promise<AdminUserList> {
  return listAdminUsers("pending", search);
}

export function listAdminUsers(
  status: AdminUser["registration_status"],
  search = "",
): Promise<AdminUserList> {
  const params = new URLSearchParams({ status, limit: "100", offset: "0" });
  if (search.trim()) params.set("search", search.trim());
  return apiRequest(`/api/v1/admin/users?${params.toString()}`);
}

export type AdminUserAction = "approve" | "reject" | "suspend" | "reinstate";

export function changeUserStatus(
  user: AdminUser,
  action: AdminUserAction,
) {
  return apiRequest<{ user: AdminUserList["users"][number] }>(
    `/api/v1/admin/users/${encodeURIComponent(user.id)}/${action}`,
    {
      method: "POST",
      body: JSON.stringify({ version: user.version }),
    },
  );
}

export function getAdminUserHistory(userId: string): Promise<{
  history: AdminUserStatusHistory[];
  total: number;
  limit: number;
  offset: number;
}> {
  return apiRequest(
    `/api/v1/admin/users/${encodeURIComponent(userId)}/history?limit=100&offset=0`,
  );
}
