import type { AuthResponse, UserSummary } from "../types/api";
import { apiRequest, clearAuthState, hasCsrfToken, restoreCsrf } from "./api";
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
  clearAuthState();
}
