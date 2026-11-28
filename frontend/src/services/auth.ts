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
  try {
    await apiRequest<void>("/api/v1/auth/logout", { method: "POST" });
  } finally {
    sessionStorage.clear();
    setCsrfToken("");
  }
}

export function registerUser(input: {
  email: string;
  first_name: string;
  last_name: string;
}) {
  return apiRequest<{ user: AuthResponse["user"] }>("/api/v1/users", {
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
  return apiRequest<{ user: AuthResponse["user"] }>("/api/v1/auth/password/reset", {
    method: "POST",
    body: JSON.stringify({ token, new_password: newPassword }),
  });
}

export function activateAccount(token: string, password: string) {
  return apiRequest<{ user: AuthResponse["user"] }>("/api/v1/auth/activate", {
    method: "POST",
    body: JSON.stringify({ token, password }),
  });
}
