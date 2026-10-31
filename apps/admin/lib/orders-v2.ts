// Purpose: Validate orders-v2 scoped list/search projections and their immutable safe merchandise.
// Depends on: ./orders-model.ts; merchant-orders-v2 wire contract.
// Used by: orders-client and merchant order browser views.
// Invariants: Counts are server-owned; private search is memory-only; merchandise never admits PII fields.
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
export type ItemSummary = { items: Array<{product_name: string; variant_label: string | null; quantity: number}>; total_item_count: number };
export type OrderSummaryV2 = OrderSummary & { order_number: string; recipient_masked: string; delivery_kind: typeof deliveries[number] | "unknown"; live_sessions: OrderSession[]; item_summary: ItemSummary };
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
    const { order_number, recipient_masked, delivery_kind, live_sessions, item_summary, ...base } = value;
    const summary = parseSourcedOrderSummary(base);
    if (order_number !== `LC-${summary.order_id.replaceAll("-", "").toUpperCase()}` || typeof recipient_masked !== "string" ||
        (recipient_masked !== "—" && ([...recipient_masked].length !== 4 || !recipient_masked.endsWith("***"))) ||
        (delivery_kind !== "unknown" && !member(delivery_kind, deliveries))) throw new Error("unavailable");
    return { ...summary, order_number: order_number as string, recipient_masked, delivery_kind, live_sessions: sessions(live_sessions), item_summary: parseItemSummary(item_summary) } as OrderSummaryV2;
  });
  if (new Set(items.map(i => i.order_id)).size !== items.length) throw new Error("unavailable");
  return { items, next_cursor: v.next_cursor, total: v.total, counts: counts as Record<Bucket, number>, sessions: sessions(v.sessions) };
}

function parseItemSummary(value: unknown): ItemSummary {
  const summary = object(value, ["items", "total_item_count"]);
  if (!Array.isArray(summary.items) || summary.items.length<1 || summary.items.length>3 ||
      !Number.isSafeInteger(summary.total_item_count) || (summary.total_item_count as number)<1) throw new Error("unavailable");
  const text = (v: unknown): v is string => typeof v === "string" && v.length>0 && [...v].length<=200 && !/[\p{Cc}\p{Cf}\p{Cs}]/u.test(v);
  const items = summary.items.map(value => {
    const item = object(value, ["product_name", "variant_label", "quantity"]);
    if (!text(item.product_name) || (item.variant_label!==null && !text(item.variant_label)) ||
        !Number.isSafeInteger(item.quantity) || (item.quantity as number)<1) throw new Error("unavailable");
    return item as ItemSummary["items"][number];
  });
  const shown = items.reduce((sum,item)=>sum+item.quantity,0);
  if (!Number.isSafeInteger(shown) || shown>(summary.total_item_count as number)) throw new Error("unavailable");
  return {items,total_item_count:summary.total_item_count as number};
}
