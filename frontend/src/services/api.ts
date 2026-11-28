import type { ApiError } from "../types/api";

// Published images use the same-origin Nginx /api proxy so one image works
// unchanged in QA and PROD-like. Local development may explicitly override it.
const baseUrl = (import.meta.env.VITE_API_BASE_URL || "").replace(/\/$/, "");

const csrfStorageKey = "ucm_csrf_token";
let csrfToken = sessionStorage.getItem(csrfStorageKey) || "";

export function setCsrfToken(token: string) {
  csrfToken = token;
  if (token) sessionStorage.setItem(csrfStorageKey, token);
  else sessionStorage.removeItem(csrfStorageKey);
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

  const isFormData =
    typeof FormData !== "undefined" && options.body instanceof FormData;
  if (options.body && !headers.has("Content-Type") && !isFormData) {
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
