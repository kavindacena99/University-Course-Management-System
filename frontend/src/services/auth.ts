import type { AuthResponse, UserSummary } from "../types/api";
import { apiRequest, endSession, hasCsrfToken, restoreCsrf } from "./api";
export function login(email: string, password: string): Promise<AuthResponse> {
  return apiRequest<AuthResponse>("/api/v1/auth/login", {
    method: "POST",
    body: JSON.stringify({ email, password }),
  });
}
export async function getCurrentUser(): Promise<UserSummary> {
  const response = await apiRequest<{ user: UserSummary }>("/api/v1/auth/me");
  if (!hasCsrfToken() && !(await restoreCsrf())) throw new Error("Session could not be restored.");
  return response.user;
}
export async function logout(): Promise<void> {
  await apiRequest<void>("/api/v1/auth/logout", { method: "POST" });
  endSession();
}

export function registerUser(input: { email: string; first_name: string; last_name: string }) {
  return apiRequest<{ user: UserSummary }>("/api/v1/users", {
    method: "POST",
    body: JSON.stringify(input),
  });
}
export function requestPasswordReset(email: string): Promise<{ status: string }> {
  return apiRequest("/api/v1/auth/password/forgot", {
    method: "POST",
    body: JSON.stringify({ email }),
  });
}
export function resetPassword(token: string, newPassword: string) {
  return apiRequest<{ user: UserSummary }>("/api/v1/auth/password/reset", {
    method: "POST",
    body: JSON.stringify({ token, new_password: newPassword }),
  });
}
export function activateAccount(token: string, password: string) {
  return apiRequest<{ user: UserSummary }>("/api/v1/auth/activate", {
    method: "POST",
    body: JSON.stringify({ token, password }),
  });
}
