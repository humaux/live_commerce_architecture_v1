// Browser BFF only: GET /api/stores -> Go /v1/admin/stores; POST /api/auth/logout -> Go /v1/identity/logout.
import { csrfCookie, sessionBoundary } from "../../lib/settings-client";
import { signalLogout } from "../../lib/session-events";
import type { Store } from "../../lib/model";
export async function readWorkspace(signal: AbortSignal): Promise<Store[]> {
  const response = await fetch("/api/stores", {
    credentials: "same-origin",
    cache: "no-store",
    signal,
  });
  if (response.status === 401) throw new Error("workspace_session_expired");
  if (!response.ok) throw new Error("workspace_unavailable");
  const body: unknown = await response.json();
  if (
    !body ||
    typeof body !== "object" ||
    !("items" in body) ||
    !Array.isArray(body.items)
  )
    throw new Error("workspace_invalid");
  return body.items as Store[]; // Server BFF authenticatedStores validates every entry.
}
export async function logoutWorkspace() {
  signalLogout();
  const csrf = csrfCookie();
  if (!csrf || !(await sessionBoundary(csrf)) || csrfCookie() !== csrf)
    throw new Error("session_changed");
  const response = await fetch("/api/auth/logout", {
    method: "POST",
    credentials: "same-origin",
    headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf },
    body: "{}",
  });
  if (response.status !== 204 && response.status !== 401)
    throw new Error("logout_failed");
  signalLogout();
}
