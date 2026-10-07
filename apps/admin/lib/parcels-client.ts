// Purpose: browser client for the W3-07B parcel-group routes: BFF `/api/stores/{store}/orders/merge-suggestions` and
//   `parcel-groups*` -> Go internal/httpapi/parcels.go. Reads parse the frozen DTOs; the keyed writes reuse writeSettings
//   (cookie CSRF + session fence + Idempotency-Key) and keep the parsed response (the create/ship answers carry the group
//   the UI then tracks; no group list route exists). Dissolve is a keyless, bodyless DELETE guarded by expected_version.
// Depends on: ./settings-client (csrfCookie, sessionBoundary, safeError, writeSettings), ./orders-client (read,
//   OrderReadError), ./parcels-model.ts (parsers).
// Used by: apps/admin/components/ParcelGroup.tsx.
// Invariants: one Idempotency-Key per logical write (retry reuses the caller's key); a network/5xx answer is `uncertain`
//   and never retried blindly; every answer is parsed before it changes UI state.
import { csrfCookie, safeError, sessionBoundary, writeSettings } from "./settings-client";
import { read, OrderReadError } from "./orders-client";
import {
  parseMergeSuggestions,
  parseParcelGroupCreated,
  parseParcelGroupDissolved,
  parseParcelGroupShipped,
  type MergeSuggestion,
  type ParcelGroupDissolved,
  type ParcelGroupShipment,
} from "./parcels-model.ts";

/** Read the merge suggestions (Go GET orders/merge-suggestions, orders:read). Throws OrderReadError like the order reads. */
export async function readMergeSuggestions(store: string, signal: AbortSignal): Promise<MergeSuggestion[]> {
  try {
    return parseMergeSuggestions(await read(`/api/stores/${store}/orders/merge-suggestions`, signal));
  } catch (error) {
    if (error instanceof OrderReadError) throw error;
    throw new OrderReadError("unavailable");
  }
}

export type ParcelWrite<T> = { ok: true; value: T } | { ok: false; code: string; uncertain: boolean };

// Keyed JSON command (POST create / PUT shipment). The parsed answer is the UI's only source of group state.
async function command<T>(
  store: string,
  method: "POST" | "PUT",
  resource: string,
  key: string,
  body: string,
  boundary: string,
  parse: (value: unknown) => T,
): Promise<ParcelWrite<T>> {
  let result;
  try {
    result = await writeSettings<T>(store, { key, method, resource }, body, boundary);
  } catch {
    // session_changed: nothing was sent.
    return { ok: false, code: "unauthorized", uncertain: false };
  }
  if (result.error === null && !result.uncertain && result.value !== null) {
    try {
      return { ok: true, value: parse(result.value) };
    } catch {
      // A committed write whose answer we cannot parse: the write may be in. Never retry blindly.
      return { ok: false, code: "retry_later", uncertain: true };
    }
  }
  return { ok: false, code: result.error?.code ?? "retry_later", uncertain: result.uncertain };
}

/** Go POST parcel-groups (fulfillment:write): {order_ids} -> 201 {id, state:"OPEN", version, order_ids}. */
export const createParcelGroup = (store: string, key: string, orderIDs: string[], boundary: string) =>
  command(store, "POST", "parcel-groups", key, JSON.stringify({ order_ids: orderIDs }), boundary, parseParcelGroupCreated);

/** Go PUT parcel-groups/{id}/shipment (fulfillment:write): the 8-key shipment body, expected_version 0, status SHIPPED. */
export const shipParcelGroup = (store: string, id: string, key: string, body: string, boundary: string) =>
  command(store, "PUT", `parcel-groups/${id}/shipment`, key, body, boundary, parseParcelGroupShipped);

/** Go DELETE parcel-groups/{id}?expected_version=N (fulfillment:write): keyless and bodyless by contract; CSRF-fenced. */
export async function dissolveParcelGroup(
  store: string,
  id: string,
  expectedVersion: number,
  boundary: string,
): Promise<ParcelWrite<ParcelGroupDissolved>> {
  const csrf = csrfCookie();
  try {
    if (!csrf || (await sessionBoundary(csrf)) !== boundary) return { ok: false, code: "unauthorized", uncertain: false };
  } catch {
    return { ok: false, code: "unauthorized", uncertain: false };
  }
  let response: Response;
  try {
    response = await fetch(`/api/stores/${store}/parcel-groups/${id}?expected_version=${expectedVersion}`, {
      method: "DELETE",
      credentials: "same-origin",
      headers: { "X-CSRF-Token": csrf },
      signal: AbortSignal.timeout(8000),
    });
  } catch {
    // UNKNOWN: the dissolve may be in; the CAS version makes a same-version retry safe (group_not_open/version_changed after success).
    return { ok: false, code: "retry_later", uncertain: true };
  }
  const value: unknown = await response.json().catch(() => null);
  if (response.ok) {
    try {
      return { ok: true, value: parseParcelGroupDissolved(value) };
    } catch {
      return { ok: false, code: "retry_later", uncertain: true };
    }
  }
  return { ok: false, code: safeError(value).code, uncertain: response.status >= 500 };
}
