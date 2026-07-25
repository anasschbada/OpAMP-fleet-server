// Thin fetch wrapper: adds the bearer token, resolves the base URL, and
// normalizes errors. There is no generated API client here -- the surface
// is small enough (a dozen endpoints) that a hand-written wrapper stays
// easier to read than introducing a codegen step.

const TOKEN_STORAGE_KEY = "opamp-fleet-ui:token";

// sessionStorage, not localStorage: the token should not outlive the
// browser tab by default, since there is no per-user login/logout flow on
// the server side to invalidate it remotely (see docs/RBAC.md -- this is a
// shared bearer token, not a session).
export function getToken(): string | null {
  return sessionStorage.getItem(TOKEN_STORAGE_KEY);
}

export function setToken(token: string): void {
  sessionStorage.setItem(TOKEN_STORAGE_KEY, token);
}

export function clearToken(): void {
  sessionStorage.removeItem(TOKEN_STORAGE_KEY);
}

// Set by an optional runtime config script (see public/config.js /
// Dockerfile) when the UI is served from a different origin than the API.
// Defaults to same-origin relative paths, which is the common case: the UI
// and API are typically exposed through the same ingress with path-based
// routing.
declare global {
  interface Window {
    __OPAMP_CONFIG__?: { apiBaseUrl?: string };
  }
}

function baseUrl(): string {
  return window.__OPAMP_CONFIG__?.apiBaseUrl ?? "";
}

// Exposed for display purposes (e.g. the sidebar's account popover) --
// shows same-origin explicitly rather than leaving it blank.
export function apiBaseUrl(): string {
  return baseUrl() || window.location.origin;
}

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const token = getToken();
  const headers: Record<string, string> = { Accept: "application/json" };
  if (token) headers.Authorization = `Bearer ${token}`;
  if (body !== undefined) headers["Content-Type"] = "application/json";

  const res = await fetch(`${baseUrl()}${path}`, {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });

  if (res.status === 401) {
    clearToken();
    throw new ApiError(401, "Session expirée ou token invalide.");
  }

  if (!res.ok) {
    let message = `Erreur ${res.status}`;
    try {
      const data = (await res.json()) as { error?: string };
      if (data.error) message = data.error;
    } catch {
      // Non-JSON error body: keep the generic message above.
    }
    throw new ApiError(res.status, message);
  }

  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export const api = {
  get: <T>(path: string) => request<T>("GET", path),
  post: <T>(path: string, body?: unknown) => request<T>("POST", path, body),
};
