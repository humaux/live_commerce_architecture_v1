// Purpose: LC-U1 browser requests through /api/stores/{store}/live-sessions → Go A1/A7/A6 and A5 flow endpoints.
// Depends on: existing studio-client.ts authenticated read/write; catalog-v2-client.ts inventory transport; closed DTO parsers.
// Used by: LiveWorkspace and LiveConsole; each user submit owns its supplied idempotency key.
// Invariants: I01/I02/I03/I05/I06/I11/I14; no automatic retries, secret storage or client inventory truth. A1 backend is currently absent.
import { read, write, StudioError } from "../../../lib/studio-client";
import { parseOffer, type Offer } from "../../../lib/claims-model.ts";
import { send } from "../../../lib/catalog-v2-client";
import { parseBalance } from "../../../lib/catalog-v2-model.ts";
import { sessionBoundary } from "../../../lib/settings-client";
import { parseConsole, parseCopyResult, parseLifecycleResult, parseRecommendResult, parseSessionResults } from "./console-model.ts";
import { validConsoleID, validConsoleBody } from "./console-request.ts";

function base(store: string, session?: string): string {
  if (!validConsoleID(store) || (session !== undefined && !validConsoleID(session))) throw new StudioError("invalid");
  return `/api/stores/${store}/live-sessions${session === undefined ? "" : `/${session}`}`;
}
async function parsed<T>(work: () => Promise<unknown>, parse: (v: unknown) => T, uncertain: boolean): Promise<T> {
  try { return parse(await work()); }
  // I06: a malformed/lost write answer may have committed; preserve the same key and never auto-retry.
  catch (error) { throw error instanceof StudioError ? error : new StudioError(uncertain ? "uncertain" : "unavailable"); }
}
/** Read A1; an unmounted backend reports not-found/unavailable, never a fabricated console. */
export function readConsole(store: string, sessionID: string, signal: AbortSignal) {
  return parsed(() => read(`${base(store, sessionID)}/console`, signal), (v) => parseConsole(v, sessionID), false);
}
/** Read authoritative M4 limits/versions from the claims board; unrelated window/stats are not consumed here. */
export function readOfferControls(store: string, sessionID: string, signal: AbortSignal): Promise<Offer[]> {
  return parsed(() => read(`${base(store, sessionID)}/claims`, signal), (value) => {
    if (!value || typeof value !== "object" || Array.isArray(value)) throw new Error("invalid_offer_controls");
    const offers = (value as Record<string, unknown>).offers;
    if (!Array.isArray(offers) || offers.length > 100) throw new Error("invalid_offer_controls");
    const result = offers.map((offer) => parseOffer(offer, sessionID));
    if (new Set(result.map((offer) => offer.offer_id)).size !== result.length) throw new Error("invalid_offer_controls");
    return result;
  }, false);
}
/** Read A5 results once for 1..50 distinct sessions, retaining their request order. */
export function readSessionResults(store: string, ids: readonly string[], signal: AbortSignal) {
  if (ids.length < 1 || ids.length > 50 || ids.some((id) => !validConsoleID(id)) || new Set(ids).size !== ids.length) return Promise.reject(new StudioError("invalid"));
  return parsed(() => read(`${base(store)}/results?${ids.map((id) => `session_id=${id}`).join("&")}`, signal), (v) => parseSessionResults(v, ids), false);
}
/** Copy A5's source draft and offers; Go performs the transaction, CAS and pricing rules. */
export function copySession(store: string, id: string, body: { title: string; scheduled_at: string | null; expected_version: number }, key: string, boundary: string) {
  return parsed(() => {
    const path = `${base(store, id)}/copy`;
    if (!validConsoleBody(path, JSON.stringify(body))) throw new StudioError("invalid");
    return write(path, "POST", body, key, boundary);
  }, (v) => {
    const result = parseCopyResult(v);
    if (result.source_version !== body.expected_version || result.session.session_id === id) throw new Error("invalid_copy_receipt");
    return result;
  }, true);
}
/** Change A7 lifecycle with its own CAS version, using the existing authenticated write boundary. */
export function changeLifecycle(store: string, id: string, action: "start" | "end" | "archive", version: number, key: string, boundary: string) {
  return parsed(() => {
    const path = `${base(store, id)}/lifecycle`;
    const body = { action, expected_version: version };
    if (!validConsoleBody(path, JSON.stringify(body))) throw new StudioError("invalid");
    return write(path, "POST", body, key, boundary);
  }, (v) => parseLifecycleResult(v, id), true);
}
/** Recommend locally in A6; optional external comment is deliberately false for LC-U1. */
export function recommendOffer(store: string, id: string, offerID: string, version: number, key: string, boundary: string) {
  return parsed(() => {
    if (!validConsoleID(offerID)) throw new StudioError("invalid");
    const path = `${base(store, id)}/claims/offers/${offerID}/recommend`;
    const body = { expected_version: version, post_comment: false };
    if (!validConsoleBody(path, JSON.stringify(body))) throw new StudioError("invalid");
    return write(path, "POST", body, key, boundary);
  }, parseRecommendResult, true);
}
/** Toggle an M4 offer while preserving its authoritative quantity limit, required by the existing Go decoder. */
export function toggleOffer(store: string, id: string, offerID: string, version: number, active: boolean, maxQuantityPerClaim: number, key: string, boundary: string) {
  return parsed(() => {
    if (!validConsoleID(offerID) || !Number.isSafeInteger(version) || version < 1 || typeof active !== "boolean" ||
      !Number.isSafeInteger(maxQuantityPerClaim) || maxQuantityPerClaim < 1 || maxQuantityPerClaim > 999) throw new StudioError("invalid");
    return write(`${base(store, id)}/claims/offers/${offerID}`, "PATCH", { expected_version: version, active, max_quantity_per_claim: maxQuantityPerClaim }, key, boundary);
  }, (v) => parseOffer(v, id), true);
}
/** Inventory adjustment through the existing catalog boundary. Go still requires inventory:write until LC-B7 arrives. */
export async function adjustLiveStock(store: string, body: { warehouse_id: string; sku_id: string; delta: number; expected_version: number; reason: "live_console_edit" }, key: string, boundary: string): Promise<{ sku_id: string }> {
  if (!validConsoleID(store) || !validConsoleID(body.warehouse_id) || !validConsoleID(body.sku_id) || !Number.isSafeInteger(body.delta) ||
    body.delta === 0 || Math.abs(body.delta) > 1000 || !Number.isSafeInteger(body.expected_version) || body.expected_version < 0 || body.reason !== "live_console_edit")
    throw new StudioError("invalid", "invalid_request");
  const result = await send(store, { key, method: "POST", resource: "inventory/adjustments", body: JSON.stringify(body) }, boundary, parseBalance);
  if (!result.ok) {
    const api = /^[a-z0-9_]{1,64}$/.test(result.code) ? result.code : "";
    throw new StudioError(result.uncertain ? "uncertain" : result.code === "forbidden" ? "forbidden" :
      result.reconcile ? "signed-out" : ["version_conflict", "version_changed", "conflict", "idempotency_conflict"].includes(result.code) ? "conflict" :
      ["invalid_request", "insufficient_inventory", "below_reserved"].includes(result.code) ? "invalid" : "unavailable", result.uncertain ? "" : api);
  }
  try { if ((await sessionBoundary()) !== boundary) throw new Error("session_changed"); }
  catch { throw new StudioError("signed-out"); }
  // The catalog parser establishes the existing identity receipt; never update a displayed balance from it.
  if (result.value.sku_id !== body.sku_id) throw new StudioError("uncertain");
  return result.value;
}
