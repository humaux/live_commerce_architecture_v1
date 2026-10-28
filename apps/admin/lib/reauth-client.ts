// Purpose: browser client of the re-auth BFF (contracts/merchant-password-auth-v1.md §16.1, card OC-04): which route to offer and
//   the start / complete / oidc steps and guarded orders/finance CSV downloads. Same-origin POSTs carry the double-submit
//   CSRF header; only the shared re-auth wrapper may replay a refused export once. Bytes live in a short-lived Blob URL.
// Depends on: /api/auth/password/reauth/{method,start,complete,oidc} (app/api/auth/password/reauth/[step]/route.ts).
// Used by: components/ReauthDialog.tsx. Auth calls bypass its fetch wrapper; the two CSV downloads use it.
// Invariants: export errors/cancellation never become files; only validated private CSV attachments are downloaded.
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

/** Fetch through the installed re-auth wrapper, then expose the successful attachment without a second HTTP GET. */
export async function reauthDownload(url: URL, signal: AbortSignal): Promise<string | null> {
  try {
    const response = await fetch(url.href, { credentials: "same-origin", cache: "no-store",
      signal: AbortSignal.any([signal, AbortSignal.timeout(30000)]) });
    if (!response.ok) {
      const body = await response.json().catch(() => ({})) as { code?: string };
      return body.code === "reauth_required" ? null : body.code ?? "retry_later";
    }
    const name = /^attachment; filename="([a-z0-9.-]+)"$/.exec(response.headers.get("content-disposition") ?? "")?.[1];
    const validName = url.pathname.endsWith("/orders/unshipped.csv")
      ? /^unshipped-[0-9a-f]{8}-[0-9]{12}\.csv$/
      : /^finance-\d{4}-\d{2}-\d{2}-\d{4}-\d{2}-\d{2}\.csv$/;
    const cache = response.headers.get("cache-control")?.toLowerCase().split(",").map((part) => part.trim());
    if (response.status !== 200 || !name || !validName.test(name) ||
      response.headers.get("content-type")?.toLowerCase() !== "text/csv; charset=utf-8" ||
      !cache?.includes("private") || !cache.includes("no-store")) return "retry_later";
    const blob = await response.blob();
    if (signal.aborted) return null;
    const href = URL.createObjectURL(blob);
    try {
      const link = document.createElement("a");
      link.href = href; link.download = name; link.rel = "noopener";
      document.body.append(link);
      try { link.click(); } finally { link.remove(); }
    } finally { setTimeout(() => URL.revokeObjectURL(href), 1000); }
    return null;
  } catch { return signal.aborted ? null : "retry_later"; }
}
