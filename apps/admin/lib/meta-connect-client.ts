// Admin Page/Instagram connect client: browser -> BFF `/api/stores/{store}/meta-connect/{status,states/{id},pick,disconnect}`
// -> Go `/v1/admin/stores/{store_id}/meta-connect/*` (internal/httpapi/meta_connect.go), and browser -> BFF POST `/api/meta/connect`
// -> Go POST meta-connect/start. Reads parse the closed shapes (meta-connect-model.ts); writes reuse writeSettings (cookie CSRF +
// session fence + Idempotency-Key) and never trust their own response for state: the card re-GETs.
// Non-goals: no token ever reaches this file (the Page token stays in Go/PG, sealed); no retry policy.
import { read } from "./orders-client";
import { csrfCookie, sessionBoundary, writeSettings } from "./settings-client";
import { parsePickState, parseStatus, type ConnectStatus, type PickState } from "./meta-connect-model";
import { safeDialogURL } from "./meta-connect-request";

export type { ConnectStatus, PickState } from "./meta-connect-model";
export type WriteResult<T = null> = { ok: true; value: T } | { ok: false; code: string; uncertain: boolean };

// Go GET meta-connect/status (integration:read); 403 hides the card.
export async function readStatus(store: string, signal: AbortSignal): Promise<ConnectStatus> {
  return parseStatus(await read(`/api/stores/${store}/meta-connect/status`, signal));
}
// Go GET meta-connect/states/{id} (integration:manage; principal + store match, unexpired, unfinished).
export async function readPickState(store: string, id: string, signal: AbortSignal): Promise<PickState> {
  return parsePickState(await read(`/api/stores/${store}/meta-connect/states/${id}`, signal), id);
}

async function send(store: string, resource: string, key: string, body: string, boundary: string): Promise<WriteResult> {
  try {
    const result = await writeSettings<unknown>(store, { key, method: "POST", resource }, body, boundary);
    if (result.error === null && !result.uncertain) return { ok: true, value: null };
    return { ok: false, code: result.error?.code ?? "retry_later", uncertain: result.uncertain };
  } catch {
    return { ok: false, code: "unauthorized", uncertain: false }; // session_changed: nothing was sent
  }
}
// Go POST meta-connect/pick {state_id, page_id, include_instagram}
export const postPick = (store: string, key: string, stateID: string, pageID: string, includeInstagram: boolean, boundary: string) =>
  send(store, "meta-connect/pick", key, JSON.stringify({ state_id: stateID, page_id: pageID, include_instagram: includeInstagram }), boundary);
// Go POST meta-connect/disconnect {}
export const postDisconnect = (store: string, key: string, boundary: string) => send(store, "meta-connect/disconnect", key, "{}", boundary);

// BFF POST /api/meta/connect -> Go POST meta-connect/start; answers {dialog_url} (checked to be exactly www.facebook.com).
export async function postConnect(store: string, key: string, boundary: string): Promise<WriteResult<string>> {
  const csrf = csrfCookie();
  try {
    if (!csrf || (await sessionBoundary(csrf)) !== boundary || csrfCookie() !== csrf) throw new Error("session_changed");
  } catch {
    return { ok: false, code: "unauthorized", uncertain: false };
  }
  let response: Response;
  try {
    response = await fetch("/api/meta/connect", {
      method: "POST", credentials: "same-origin", signal: AbortSignal.timeout(8000),
      headers: { "Content-Type": "application/json", "Idempotency-Key": key, "X-CSRF-Token": csrf }, body: JSON.stringify({ store }),
    });
  } catch {
    return { ok: false, code: "retry_later", uncertain: true };
  }
  const value: unknown = await response.json().catch(() => null);
  if (!response.ok) {
    const code = value && typeof value === "object" && typeof (value as Record<string, unknown>).code === "string" ? ((value as Record<string, unknown>).code as string) : "retry_later";
    return { ok: false, code, uncertain: response.status >= 500 };
  }
  const url = safeDialogURL(value && typeof value === "object" ? (value as Record<string, unknown>).dialog_url : null);
  return url ? { ok: true, value: url } : { ok: false, code: "retry_later", uncertain: false };
}

export const newKey = (prefix: string) => `${prefix}-${crypto.randomUUID()}`;
