// Purpose: Validate LC-U1's scoped, immutable replay descriptor before storing or executing it.
// Depends on: frozen console body/UUID grammar; JSON contains merchant command inputs, never cookies or credentials.
// Used by: useLiveCommand and console-client; legacy/malformed fences fail closed rather than create a new command.
import { validConsoleBody, validConsoleID } from "./console-request";

export type LiveRequest = { method: "POST" | "PATCH"; path: string; body: string };
export type LiveJournal = LiveRequest & { key: string; mayHaveCommitted: boolean };
const object = (v: unknown): v is Record<string, unknown> => !!v && typeof v === "object" && !Array.isArray(v);
/** Accept only this store/scene's five UI commands; browser storage is untrusted input. */
export function validLiveRequest(v: unknown, scope: string): v is LiveRequest {
  if (!object(v) || typeof v.path !== "string" || typeof v.body !== "string" || v.body.length > 4096) return false;
  const [store, scene, extra] = scope.split(":");
  if (extra || !validConsoleID(store) || !validConsoleID(scene)) return false;
  let body: unknown; try { body = JSON.parse(v.body); } catch { return false; }
  if (!object(body) || JSON.stringify(body) !== v.body) return false;
  const base = `/api/stores/${store}/live-sessions/${scene}`;
  if (v.method === "POST" && ["lifecycle", "copy"].some((p) => v.path === `${base}/${p}`)) return validConsoleBody(v.path, v.body) &&
    Object.keys(body).join(",") === (v.path.endsWith("/copy") ? "title,scheduled_at,expected_version" : "action,expected_version");
  const offer = new RegExp(`^${base}/claims/offers/([^/]+)(/recommend)?$`).exec(v.path);
  if (offer && validConsoleID(offer[1])) {
    if (offer[2]) return v.method === "POST" && validConsoleBody(v.path, v.body) && Object.keys(body).join(",") === "expected_version,post_comment";
    return v.method === "PATCH" && Object.keys(body).join(",") === "expected_version,active,max_quantity_per_claim" && Number.isSafeInteger(body.expected_version) && (body.expected_version as number) > 0 &&
      typeof body.active === "boolean" && Number.isSafeInteger(body.max_quantity_per_claim) && (body.max_quantity_per_claim as number) >= 1 && (body.max_quantity_per_claim as number) <= 999;
  }
  return v.method === "POST" && v.path === `/api/stores/${store}/inventory/adjustments` && Object.keys(body).join(",") === "warehouse_id,sku_id,delta,expected_version,reason" &&
    validConsoleID(body.warehouse_id) && validConsoleID(body.sku_id) && Number.isSafeInteger(body.delta) && body.delta !== 0 && Math.abs(body.delta as number) <= 1000 &&
    Number.isSafeInteger(body.expected_version) && (body.expected_version as number) >= 0 && body.reason === "live_console_edit";
}
/** Recover only the original same-scope command/key. Old opaque fences remain non-replayable. */
export function parseLiveJournal(value: string, scope: string): LiveJournal | null {
  try {
    const v: unknown = JSON.parse(value);
    return object(v) && Object.keys(v).length === 5 && validConsoleID(v.key) && typeof v.mayHaveCommitted === "boolean" && validLiveRequest(v, scope) ? v as LiveJournal : null;
  } catch { return null; }
}
/** Build a closed same-origin command descriptor; validation remains mandatory at replay. */
export function liveRequest(store: string, scene: string, suffix: string, method: LiveRequest["method"], body: unknown): LiveRequest {
  const request = { method, path: suffix === "inventory/adjustments" ? `/api/stores/${store}/${suffix}` : `/api/stores/${store}/live-sessions/${scene}/${suffix}`, body: JSON.stringify(body) };
  if (!validLiveRequest(request, `${store}:${scene}`)) throw new Error("invalid_live_command");
  return request;
}
