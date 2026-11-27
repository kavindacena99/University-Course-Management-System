import type { AuthResponse } from "../types/api";
import { apiRequest, setCsrfToken } from "./api";

export async function login(email: string, password: string): Promise<AuthResponse> {
  const response = await apiRequest<AuthResponse>("/api/v1/auth/login", {
    method: "POST",
    body: JSON.stringify({ email, password }),
  });

  if (response.csrf_token) {
    setCsrfToken(response.csrf_token);
  }
  if (response.access_token) {
    sessionStorage.setItem("ucm_access_token", response.access_token);
  }
  sessionStorage.setItem("ucm_user", JSON.stringify(response.user));
  return response;
}

export function getStoredUser() {
  const value = sessionStorage.getItem("ucm_user");
  return value ? JSON.parse(value) : null;
}

export async function getCurrentUser(): Promise<AuthResponse["user"]> {
  const response = await apiRequest<{ user: AuthResponse["user"] }>("/api/v1/auth/me");
  return response.user;
}

export async function logout(): Promise<void> {
  await apiRequest<void>("/api/v1/auth/logout", { method: "POST" });
  sessionStorage.clear();
  setCsrfToken("");
}
