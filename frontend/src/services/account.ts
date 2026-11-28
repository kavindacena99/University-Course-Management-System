import type { AccountProfile, UserSession } from "../types/api";
import { apiRequest } from "./api";

export function getAccountProfile(): Promise<{ user: AccountProfile }> {
  return apiRequest("/api/v1/users/me");
}

export function updateAccountProfile(input: {
  version: number;
  first_name: string;
  last_name: string;
}): Promise<{ user: AccountProfile }> {
  return apiRequest("/api/v1/users/me", {
    method: "PATCH",
    body: JSON.stringify(input),
  });
}

export function changeAccountPassword(currentPassword: string, newPassword: string): Promise<void> {
  return apiRequest("/api/v1/auth/password/change", {
    method: "POST",
    body: JSON.stringify({
      current_password: currentPassword,
      new_password: newPassword,
    }),
  });
}

export function listAccountSessions(): Promise<{ sessions: UserSession[] }> {
  return apiRequest("/api/v1/auth/sessions");
}

export function revokeAccountSession(sessionId: string): Promise<void> {
  return apiRequest(`/api/v1/auth/sessions/${encodeURIComponent(sessionId)}`, {
    method: "DELETE",
  });
}

export function logoutAllSessions(): Promise<void> {
  return apiRequest("/api/v1/auth/logout-all", { method: "POST" });
}
