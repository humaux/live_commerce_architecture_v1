// Read contract for BFF /api/stores/{store}/orders?view=v2 -> merchantorders.ListV2.
// No transaction rules or client-side counts: reject malformed server projections.
import { canonicalCursor, canonicalUUID, orderStates, parseSourcedOrderSummary, type OrderSummary } from "./orders-model.ts";

export const buckets = ["all", "unpaid", "transfer_review", "ready_to_ship", "ready_to_consign", "shipped", "completed", "cancelled"] as const;
export type Bucket = typeof buckets[number];
export const deliveries = ["home", "cvs_711", "cvs_familymart", "cvs_hilife", "cvs_okmart"] as const;
export const modes = ["card", "bank_transfer", "pay_at_pickup", "cash_on_delivery"] as const;
export const filterKeys = ["bucket", "q", "payment_mode", "delivery", "session_id", "from", "to"] as const;
export type OrderFilters = Record<Exclude<typeof filterKeys[number], "bucket">, string> & { bucket: Bucket };
export const emptyFilters: OrderFilters = { bucket: "all", q: "", payment_mode: "", delivery: "", session_id: "", from: "", to: "" };
export type OrderSession = { id: string; name: string };
export type OrderSummaryV2 = OrderSummary & { order_number: string; recipient_masked: string; delivery_kind: typeof deliveries[number]; live_sessions: OrderSession[] };
export type OrderListV2 = { items: OrderSummaryV2[]; next_cursor: string; total: number; counts: Record<Bucket, number>; sessions: OrderSession[] };
const member = (value: string, set: readonly string[]) => set.includes(value);
function day(value: string) {
  return /^(?:20|21)[0-9]{2}-[0-9]{2}-[0-9]{2}$/.test(value) &&
    Number.isFinite(Date.parse(`${value}T00:00:00Z`)) && new Date(`${value}T00:00:00Z`).toISOString().slice(0, 10) === value;
}
export function validOrderFilters(f: OrderFilters) {
  return member(f.bucket, buckets) && [...f.q].length <= 80 && f.q.trim() === f.q && !/[\p{Cc}\p{Cf}\p{Cs}]/u.test(f.q) &&
    (!f.payment_mode || member(f.payment_mode, modes)) && (!f.delivery || member(f.delivery, deliveries)) &&
    (!f.session_id || canonicalUUID.test(f.session_id)) && (!f.from || day(f.from)) && (!f.to || day(f.to)) &&
    (!f.from || !f.to || f.from <= f.to);
}
export function appendOrderFilters(query: URLSearchParams, filters: OrderFilters) {
  for (const key of filterKeys) if (key !== "q" && filters[key] && !(key === "bucket" && filters[key] === "all")) query.set(key, filters[key]);
}
export function validOrdersV2Query(raw: string) {
  try {
    if (raw.length > 4096 || raw.split("&").some(part => !/^[a-z_]+=[^=]+$/.test(part) || /%(?![0-9a-fA-F]{2})/.test(part))) return false;
    const params = new URLSearchParams(raw), seen = new Set<string>();
    for (const [key, value] of params) {
      if (seen.has(key) || !value || key === "q" || !["view", "limit", "cursor", "state", ...filterKeys].includes(key)) return false;
      seen.add(key);
    }
    const f = { ...emptyFilters };
    for (const key of filterKeys) if (params.has(key)) Object.assign(f, { [key]: params.get(key)! });
    return params.get("view") === "v2" && validOrderFilters(f) &&
      (!params.has("state") || member(params.get("state")!, orderStates)) &&
      (!params.has("limit") || /^(?:[1-9]|[1-9][0-9]|100)$/.test(params.get("limit")!)) &&
      (!params.has("cursor") || canonicalCursor.test(params.get("cursor")!));
  } catch { return false; }
}
function object(value: unknown, keys: string[]) {
  if (!value || typeof value !== "object" || Array.isArray(value) || Object.keys(value).length !== keys.length || keys.some(k => !(k in value))) throw new Error("unavailable");
  return value as Record<string, unknown>;
}
function sessions(value: unknown): OrderSession[] {
  if (!Array.isArray(value) || value.length > 100) throw new Error("unavailable");
  const result = value.map(v => {
    const row = object(v, ["id", "name"]);
    if (typeof row.id !== "string" || !canonicalUUID.test(row.id) || typeof row.name !== "string" || !row.name || [...row.name].length > 200 || /[\p{Cc}\p{Cf}\p{Cs}]/u.test(row.name)) throw new Error("unavailable");
    return { id: row.id, name: row.name };
  });
  if (new Set(result.map(s => s.id)).size !== result.length) throw new Error("unavailable");
  return result;
}
export function parseOrderListV2(value: unknown): OrderListV2 {
  const v = object(value, ["items", "next_cursor", "total", "counts", "sessions"]);
  if (!Array.isArray(v.items) || v.items.length > 10 || typeof v.next_cursor !== "string" || (v.next_cursor && !canonicalCursor.test(v.next_cursor))) throw new Error("unavailable");
  const counts = object(v.counts, [...buckets]);
  const count = (n: unknown): n is number => typeof n === "number" && Number.isSafeInteger(n) && n >= 0;
  if (!count(v.total) || Object.values(counts).some(n => !count(n)) || v.items.length > v.total) throw new Error("unavailable");
  const items = v.items.map(value => {
    if (!value || typeof value !== "object" || Array.isArray(value)) throw new Error("unavailable");
    const { order_number, recipient_masked, delivery_kind, live_sessions, ...base } = value;
    const summary = parseSourcedOrderSummary(base);
    if (order_number !== `LC-${summary.order_id.replaceAll("-", "").toUpperCase()}` || typeof recipient_masked !== "string" ||
        [...recipient_masked].length !== 4 || !recipient_masked.endsWith("***") || !member(delivery_kind, deliveries)) throw new Error("unavailable");
    return { ...summary, order_number: order_number as string, recipient_masked, delivery_kind, live_sessions: sessions(live_sessions) } as OrderSummaryV2;
  });
  if (new Set(items.map(i => i.order_id)).size !== items.length) throw new Error("unavailable");
  return { items, next_cursor: v.next_cursor, total: v.total, counts: counts as Record<Bucket, number>, sessions: sessions(v.sessions) };
}
