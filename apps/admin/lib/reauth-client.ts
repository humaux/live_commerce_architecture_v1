// Purpose: browser client of the re-auth BFF (contracts/merchant-password-auth-v1.md §16.1, card OC-04): which route to offer and
//   the start / complete / oidc steps. Same-origin POSTs carry the double-submit CSRF header; nothing is retried or stored.
// Depends on: /api/auth/password/reauth/{method,start,complete,oidc} (app/api/auth/password/reauth/[step]/route.ts).
// Used by: components/ReauthDialog.tsx. These calls are under /api/auth/, so the dialog's fetch wrapper never intercepts them.
export type ReauthMethod = { password: boolean; oidc: boolean; account: "password" | "oidc" };
export type ReauthAnswer = { status: number; body: Record<string, unknown> };

function csrf() {
  const values = document.cookie.split(";").map((p) => p.trim()).filter((p) => p.startsWith("__Host-commerce_csrf=")).map((p) => p.slice(21));
  return values.length === 1 ? values[0] : "";
}

/** GET method: the route the dialog offers this session; throws when the BFF cannot answer. */
export async function reauthMethod(): Promise<ReauthMethod> {
  const response = await fetch("/api/auth/password/reauth/method", { credentials: "same-origin", cache: "no-store", signal: AbortSignal.timeout(8000) });
  if (!response.ok) throw new Error(String(response.status));
  return (await response.json()) as ReauthMethod;
}

/** POST one step; a network failure is a 503 retry_later answer, never thrown. */
export async function reauthStep(step: "start" | "complete" | "oidc", body: unknown): Promise<ReauthAnswer> {
  try {
    const response = await fetch(`/api/auth/password/reauth/${step}`, {
      method: "POST", credentials: "same-origin", cache: "no-store",
      headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf() },
      body: JSON.stringify(body), signal: AbortSignal.timeout(16000),
    });
    return { status: response.status, body: await response.json().catch(() => ({})) };
  } catch {
    return { status: 503, body: { code: "retry_later" } };
  }
}
