// Merchant-tools client: browser -> BFF `/api/stores/{store}/tools/*` (apps/admin/app/api/stores/[store]/tools/[...resource]/route.ts) -> Go
// internal/httpapi/merchanttools.go. Reads go through lib/customers-client `get` (strict parse, no-store check, session fence by
// useGuardedRead); writes carry the cookie CSRF pair and refuse to send under a changed session (the same fence as catalog-v2-client `send`).
// An import or an order is never retried here: an uncertain answer is shown and the caller re-sends the SAME key (manual order) or the same
// file (import: idempotent by file hash), so a repeat can only replay.
import { csrfCookie, safeError, sessionBoundary } from "./settings-client";
import { get } from "./customers-client";
import { parseDashboard, parseImportResult, parseManualOptions, parseManualResult, parseRegenerateResult, type Dashboard, type ImportResult, type ManualOption, type ManualResult, type RegenerateResult } from "./merchant-tools-model";

export type Outcome<T> = { ok: true; value: T; status: number } | { ok: false; code: string; uncertain: boolean; status: number };
const base = (store: string) => `/api/stores/${store}/tools`;

export const readDashboard = (store: string, signal: AbortSignal): Promise<Dashboard> => get(`${base(store)}/dashboard`, parseDashboard, signal);
export const readManualOptions = (store: string, signal: AbortSignal): Promise<ManualOption[]> =>
  get(`${base(store)}/orders/manual/options`, parseManualOptions, signal);

async function fenced(boundary: string): Promise<string | null> {
  const csrf = csrfCookie();
  try {
    if (!csrf || (await sessionBoundary(csrf)) !== boundary || csrfCookie() !== csrf) return null;
  } catch {
    return null;
  }
  return csrf;
}

async function write<T>(url: string, init: { headers: Record<string, string>; body: string | Blob }, boundary: string, parse: (v: unknown, status: number) => T, timeoutMs: number,
  accept: (status: number) => boolean): Promise<Outcome<T>> {
  const csrf = await fenced(boundary);
  if (!csrf) return { ok: false, code: "unauthorized", uncertain: false, status: 401 };
  try {
    const response = await fetch(url, {
      method: "POST", credentials: "same-origin", cache: "no-store", headers: { ...init.headers, "X-CSRF-Token": csrf }, body: init.body,
      signal: AbortSignal.timeout(timeoutMs),
    });
    const value: unknown = await response.json().catch(() => null);
    if (!accept(response.status)) return { ok: false, code: safeError(value).code, uncertain: response.status >= 500, status: response.status };
    try {
      return { ok: true, value: parse(value, response.status), status: response.status };
    } catch {
      return { ok: false, code: "retry_later", uncertain: true, status: 503 };
    }
  } catch {
    return { ok: false, code: "retry_later", uncertain: true, status: 503 };
  }
}

/** POST products/import/preview|commit with the raw CSV. A commit refused for row errors is ok:true with committed=false (status 422). */
export const sendImport = (store: string, mode: "preview" | "commit", file: Blob, boundary: string): Promise<Outcome<ImportResult>> =>
  write(`${base(store)}/products/import/${mode}`, { headers: { "Content-Type": "text/csv; charset=utf-8" }, body: file }, boundary, parseImportResult, 80000,
    (status) => status === 200 || (mode === "commit" && status === 422));

/** POST orders/manual. The Idempotency-Key is chosen by the caller and reused for a retry of the same attempt (replay, never a second order). */
export const placeManualOrder = (store: string, key: string, body: unknown, boundary: string): Promise<Outcome<ManualResult>> =>
  write(`${base(store)}/orders/manual`, { headers: { "Content-Type": "application/json", "Idempotency-Key": key }, body: JSON.stringify(body) }, boundary,
    (value, status) => parseManualResult(value, { allowRedactedLink: status === 200 }), 18000, (status) => status === 200 || status === 201);

/** POST orders/manual/regenerate-link (K3 F2). The Idempotency-Key is chosen by the caller and reused for a retry of the SAME attempt: the new
 *  link token is derived from it, so a replay re-delivers the same link and a new key is a new link (the previous one invalidated in SQL). */
export const regenerateManualLink = (store: string, key: string, body: { order_id: string; locale: string }, boundary: string): Promise<Outcome<RegenerateResult>> =>
  write(`${base(store)}/orders/manual/regenerate-link`, { headers: { "Content-Type": "application/json", "Idempotency-Key": key }, body: JSON.stringify(body) }, boundary,
    parseRegenerateResult, 18000, (status) => status === 200 || status === 201);

/** GET products/export.csv: returns the file bytes for a browser download (the BFF streams only the exact attachment shape). */
export async function fetchExport(store: string): Promise<{ ok: true; blob: Blob; name: string } | { ok: false; code: string }> {
  try {
    const response = await fetch(`${base(store)}/products/export.csv`, { method: "GET", credentials: "same-origin", cache: "no-store", signal: AbortSignal.timeout(25000) });
    if (!response.ok) return { ok: false, code: safeError(await response.json().catch(() => null)).code };
    const name = /filename="(products-[0-9-]{10}\.csv)"/.exec(response.headers.get("content-disposition") ?? "")?.[1];
    if (!name) return { ok: false, code: "retry_later" };
    return { ok: true, blob: await response.blob(), name };
  } catch {
    return { ok: false, code: "retry_later" };
  }
}
