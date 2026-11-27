import type { ApiError } from "../types/api";

const baseUrl = (import.meta.env.VITE_API_BASE_URL || "http://localhost:8080").replace(/\/$/, "");

let csrfToken = "";

export function setCsrfToken(token: string) {
  csrfToken = token;
}

export function getCsrfToken() {
  return csrfToken;
}

export async function apiRequest<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const method = (options.method || "GET").toUpperCase();
  const headers = new Headers(options.headers);
  headers.set("Accept", "application/json");

  if (options.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  if (csrfToken && ["POST", "PUT", "PATCH", "DELETE"].includes(method)) {
    headers.set("X-CSRF-Token", csrfToken);
  }

  const response = await fetch(`${baseUrl}${path}`, {
    ...options,
    method,
    headers,
    credentials: "include",
  });

  if (response.status === 204) {
    return undefined as T;
  }

  const payload = await response.json().catch(() => null);
  if (!response.ok) {
    const apiError = payload?.error as ApiError | undefined;
    const error = new Error(apiError?.message || `Request failed (${response.status})`) as Error & {
      status: number;
      code?: string;
      requestId?: string;
    };
    error.status = response.status;
    error.code = apiError?.code;
    error.requestId = apiError?.request_id;
    throw error;
  }

  return payload as T;
}
