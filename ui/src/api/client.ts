// Thin fetch wrapper: adds the Authorization header, resolves the base
// URL, and normalizes errors. There is no generated API client here -- the
// surface is small enough (a dozen endpoints) that a hand-written wrapper
// stays easier to read than introducing a codegen step.

const CREDENTIAL_STORAGE_KEY = "opamp-fleet-ui:credential";

// What's stored is the full Authorization header VALUE, not a bare token --
// the server accepts either a bearer API token or HTTP Basic Auth (see
// internal/api's withAuth), and the login screen lets the operator pick
// either. Storing the ready-to-send header value means request() never
// needs to know which one was used.
//
// sessionStorage, not localStorage: the credential should not outlive the
// browser tab by default, since there is no per-user login/logout flow on
// the server side to invalidate it remotely (see docs/RBAC.md -- this is a
// shared credential, not a personal session).
export function getCredential(): string | null {
  return sessionStorage.getItem(CREDENTIAL_STORAGE_KEY);
}

export function setBearerToken(token: string): void {
  sessionStorage.setItem(CREDENTIAL_STORAGE_KEY, `Bearer ${token}`);
}

export function setBasicAuth(username: string, password: string): void {
  sessionStorage.setItem(CREDENTIAL_STORAGE_KEY, `Basic ${btoa(`${username}:${password}`)}`);
}

export function clearCredential(): void {
  sessionStorage.removeItem(CREDENTIAL_STORAGE_KEY);
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
  const credential = getCredential();
  const headers: Record<string, string> = { Accept: "application/json" };
  if (credential) headers.Authorization = credential;
  if (body !== undefined) headers["Content-Type"] = "application/json";

  const res = await fetch(`${baseUrl()}${path}`, {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });

  if (res.status === 401) {
    clearCredential();
    throw new ApiError(401, "Session expirée ou identifiants invalides.");
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
