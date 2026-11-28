import type { ApiError } from "../types/api";

let refreshing: Promise<boolean> | null = null;
let sessionExpired: (() => void) | null = null;
let csrfValue = "";
export function onSessionExpired(handler: (() => void) | null) {
  sessionExpired = handler;
}
export function clearAuthState() {
  csrfValue = "";
}
export function endSession() {
  clearAuthState();
  sessionExpired?.();
}
export function hasCsrfToken() {
  return !!csrfValue;
}
async function refresh(): Promise<boolean> {
  if (!refreshing)
    refreshing = fetch("/api/v1/auth/refresh", {
      method: "POST",
      credentials: "include",
      headers: csrfValue ? { "X-CSRF-Token": csrfValue } : {},
    })
      .then(async (response) => {
        if (!response.ok) return false;
        const payload = (await response.json()) as { csrf_token?: string };
        csrfValue = payload.csrf_token || "";
        return !!csrfValue;
      })
      .catch(() => false)
      .finally(() => {
        refreshing = null;
      });
  return refreshing;
}
export async function restoreCsrf() {
  return refresh();
}
export class RequestError extends Error {
  constructor(
    public status: number,
    public code: string | undefined,
    message: string,
  ) {
    super(message);
  }
}
export async function apiRequest<T>(
  path: string,
  options: RequestInit = {},
  retry = true,
): Promise<T> {
  const headers = new Headers(options.headers);
  headers.set("Accept", "application/json");
  if (options.body && !(options.body instanceof FormData) && !headers.has("Content-Type"))
    headers.set("Content-Type", "application/json");
  const method = (options.method || "GET").toUpperCase();
  if (!["GET", "HEAD", "OPTIONS"].includes(method)) {
    if (csrfValue) headers.set("X-CSRF-Token", csrfValue);
  }
  const response = await fetch(path, { ...options, method, headers, credentials: "include" });
  if (
    response.status === 401 &&
    retry &&
    !["/api/v1/auth/login", "/api/v1/auth/refresh", "/api/v1/auth/logout"].includes(path)
  ) {
    if (await refresh()) return apiRequest<T>(path, options, false);
    endSession();
  }
  if (response.status === 204) return undefined as T;
  if (!response.ok) {
    const payload = (await response.json().catch(() => null)) as { error?: ApiError } | null;
    throw new RequestError(
      response.status,
      payload?.error?.code,
      payload?.error?.message || `Request failed (${response.status})`,
    );
  }
  const payload = (await response.json()) as T & { csrf_token?: string };
  if (path === "/api/v1/auth/login" && payload.csrf_token) csrfValue = payload.csrf_token;
  return payload;
}
export async function apiFile(path: string): Promise<Blob> {
  let response = await fetch(path, { credentials: "include" });
  if (response.status === 401) {
    if (await refresh()) response = await fetch(path, { credentials: "include" });
    else {
      endSession();
    }
  }
  if (response.status === 401) {
    endSession();
  }
  if (!response.ok)
    throw new RequestError(response.status, undefined, `Download failed (${response.status})`);
  return response.blob();
}
