// Purpose: scoped, explicitly retried tracking CSV transport; no automatic shipping writes.
// Depends on: sessionBoundary/CSRF fence, tools BFF, immutable CSV Blob and closed tracking DTOs.
// Used by: TrackingImport. Calls Go tracking-import via /api/stores/{store}/tools/shipments/tracking-import.
import { csrfCookie, sessionBoundary, safeError } from "./settings-client";
import { parseTrackingCommit, parseTrackingPreview, trackingUUID, type TrackingCommit, type TrackingPreview } from "./tracking-import-model";
export type TrackingAnswer = { kind: "preview" | "stale"; value: TrackingPreview } | { kind: "committed"; value: TrackingCommit } | { kind: "error"; code: string; uncertain: boolean };

/** Send exactly one request; an unknown commit can only be recovered by an explicit identical call. */
export async function sendTracking(store: string, mode: "preview" | "commit", file: Blob, boundary: string, expected?: number): Promise<TrackingAnswer> {
  if (!trackingUUID.test(store) || (mode === "commit" && (!Number.isInteger(expected) || expected! < 0 || expected! > 500)))
    return { kind: "error", code: "invalid_request", uncertain: false };
  const csrf = csrfCookie();
  try { if (!csrf || await sessionBoundary(csrf) !== boundary) return { kind: "error", code: "unauthorized", uncertain: false }; }
  catch { return { kind: "error", code: "unauthorized", uncertain: false }; }
  try {
    const url = `/api/stores/${store}/tools/shipments/tracking-import/${mode}${mode === "commit" ? `?expected_apply_rows=${expected}` : ""}`;
    const response = await fetch(url, { method: "POST", credentials: "same-origin", cache: "no-store", redirect: "error", headers: { "Content-Type": "text/csv; charset=utf-8", "X-CSRF-Token": csrf }, body: file, signal: AbortSignal.timeout(80000) });
    const value: unknown = await response.json().catch(() => null);
    // A reply for a previous session must never appear under the new actor.
    if (csrfCookie() !== csrf || await sessionBoundary(csrf) !== boundary)
      return { kind: "error", code: "unauthorized", uncertain: mode === "commit" };
    if (response.status === 200) return mode === "preview" ? { kind: "preview", value: parseTrackingPreview(value) } : { kind: "committed", value: parseTrackingCommit(value, expected) };
    if (mode === "commit" && response.status === 409) {
      try { return { kind: "stale", value: parseTrackingPreview(value) }; } catch { /* A coded conflict is not a new preview. */ }
    }
    return { kind: "error", code: safeError(value).code, uncertain: mode === "commit" && (response.status >= 500 || response.status === 408) };
  } catch { return { kind: "error", code: "retry_later", uncertain: mode === "commit" }; }
}

/** Private failure report URL: identifiers are server-validated; no buyer data is placed in query strings. */
export function trackingResultHref(store: string, batch: string): string {
  if (!trackingUUID.test(store) || !trackingUUID.test(batch)) throw new Error("invalid_response");
  return `/api/stores/${store}/tools/shipments/tracking-import/${batch}/result.csv?only=failed`;
}
