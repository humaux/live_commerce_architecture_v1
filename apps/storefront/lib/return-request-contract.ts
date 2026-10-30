// Purpose: closed buyer after-sales request grammar and safe persisted projection at the BFF boundary.
// Depends on: returns-v1 amendment 0210 and server-issued UUIDs/timestamps; no runtime dependencies.
// Used by: buyer-server.ts only; UI implementation follows PARITY-NEWFEAT.
// Invariants: no private authority/PII fields or unknown states, bounded notes, no client prices or photo URLs.
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
const statuses = ["requested", "approved", "rejected", "withdrawn"];
export const RETURN_REQUEST_CODES = ["window_closed", "request_open", "version_changed", "not_shipped", "not_returnable", "invalid_state", "unknown_line", "ambiguous_line", "invalid_quantities", "exceeds_shipped"];
function object(v: unknown): v is Record<string, unknown> { return !!v && typeof v === "object" && !Array.isArray(v); }
function keys(v: Record<string, unknown>, required: string[], optional: string[] = []) {
  return required.every(k => Object.hasOwn(v,k)) && Object.keys(v).every(k=>required.includes(k)||optional.includes(k));
}
function text(v: unknown,n=1000): v is string {return typeof v==="string" && Array.from(v).length<=n && !/[\u0000-\u001f\u007f-\u009f]/u.test(v);}
function id(v: unknown): v is string {return typeof v==="string" && uuid.test(v);}
function integer(v: unknown): v is number {return typeof v==="number" && Number.isSafeInteger(v)&&v>0;}
function date(v: unknown): v is string {return typeof v==="string" && /^\d{4}-\d\d-\d\dT/.test(v) && Number.isFinite(Date.parse(v));}
function lines(v: unknown, resolved: boolean) {
  if (!Array.isArray(v)||v.length<1||v.length>50) return false;
  const seen=new Set<string>();
  return v.every(l=>{
    if (!object(l)||!keys(l,["sku_id","quantity"], ["warehouse_id"])||!id(l.sku_id)||!integer(l.quantity)||l.quantity>999999999||
      (resolved ? !id(l.warehouse_id) : l.warehouse_id!==undefined && l.warehouse_id!==null && !id(l.warehouse_id))) return false;
    const k=l.sku_id+":"+(l.warehouse_id??"");if (seen.has(k))return false;seen.add(k);return true;
  });
}
export function validReturnRequestInput(v: unknown) {
  return object(v)&&keys(v,["reason","lines"],["note"])&&v.reason==="damaged"&&(v.note===undefined||text(v.note))&&lines(v.lines,false);
}
export function validReturnRequestWithdraw(v: unknown) {return object(v)&&keys(v,["expected_version"])&&integer(v.expected_version);}
export function validReturnRequest(v: unknown, expectedID?: string, orderID?: string) {
  if (!object(v)||!keys(v,["id","number","order_id","status","version","reason","note","rejection_reason","return_id","return_state","created_at","updated_at","lines","timeline"])||
    !id(v.id)||expectedID!==undefined&&v.id!==expectedID||!id(v.order_id)||orderID!==undefined&&v.order_id!==orderID||
    typeof v.number!=="string"||!/^R-[A-Za-z0-9-]{1,64}$/.test(v.number)||!statuses.includes(String(v.status))||!integer(v.version)||
    v.version!==(v.status==="requested"?1:2)||v.reason!=="damaged"||!text(v.note)||!date(v.created_at)||!date(v.updated_at)||Date.parse(v.updated_at)<Date.parse(v.created_at)||
    !lines(v.lines,true)||!Array.isArray(v.timeline)) return false;
  if (v.status==="approved" ? !id(v.return_id)||!["REGISTERED","RECEIVED","INSPECTED","CLOSED","CANCELLED"].includes(String(v.return_state)) : v.return_id!==null||v.return_state!==null) return false;
  if (v.status==="rejected" ? !text(v.rejection_reason)||v.rejection_reason.trim()==="" : v.rejection_reason!==null) return false;
  if (v.timeline.length!==(v.status==="requested"?1:2)) return false;
  return v.timeline.every((e,i)=>object(e)&&keys(e,["status","at"])&&e.status===(i===0?"requested":v.status)&&date(e.at)&&Date.parse(e.at)===Date.parse(i===0?v.created_at as string:v.updated_at as string));
}
export function validReturnRequestList(v: unknown) {return object(v)&&keys(v,["items"])&&Array.isArray(v.items)&&v.items.every(x=>validReturnRequest(x));}
