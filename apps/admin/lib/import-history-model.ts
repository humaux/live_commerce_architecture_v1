// Purpose: frozen W5-03B historical-order projection and exact read request grammar.
// Depends on: customers-model closed-object/time checks, orders-model UUID/cursor fences, twcity Go/0156 allowlist.
// Used by: import-history-client, historical-orders BFF leaf and CustomerHistoricalOrders; no trade/revenue authority.
import { isInstant, object } from "./customers-model.ts";
import { canonicalCursor, canonicalUUID } from "./orders-model.ts";

/** One SHOPLINE archive row; order_id is external text, never a live order UUID/link. */
export type HistoricalOrder = {
  order_id: string; ordered_at: string; status: string; total_minor: number; currency: "TWD"; items_summary: string; city: string | null;
};
/** Server keyset page; total counts archive rows and is never a financial total. */
export type HistoricalOrdersPage = { items: HistoricalOrder[]; next_cursor: string; total: number };

// Matches internal/twcity.Cities and migration 0156; no city normalization on reads (server facts stay exact).
const cities = new Set(["臺北市", "新北市", "桃園市", "臺中市", "臺南市", "高雄市", "基隆市", "新竹市", "嘉義市",
  "新竹縣", "苗栗縣", "彰化縣", "南投縣", "雲林縣", "嘉義縣", "屏東縣", "宜蘭縣", "花蓮縣", "臺東縣", "澎湖縣", "金門縣", "連江縣"]);
const boundedText = (v: unknown, min: number, max: number): v is string =>
  typeof v === "string" && Array.from(v).length >= min && Array.from(v).length <= max && !/[\p{Cc}\p{Cs}]/u.test(v);
const archiveTime = (v: unknown): v is string => isInstant(v) && new Date(v).toISOString().slice(0, 19) === v.slice(0, 19);

/** Strictly decode the public safe archive fields; rejects unknown keys, unsafe city data and malformed bounds. */
export function parseHistoricalOrdersPage(value: unknown, limit = 100): HistoricalOrdersPage {
  const v = object(value, ["items", "next_cursor", "total"]);
  if (!Number.isInteger(limit) || limit < 1 || limit > 100 || !Array.isArray(v.items) || v.items.length > limit ||
    !Number.isSafeInteger(v.total) || (v.total as number) < v.items.length || (v.total as number) > 2000 ||
    typeof v.next_cursor !== "string" || (v.next_cursor !== "" && !canonicalCursor.test(v.next_cursor)) ||
    (v.next_cursor !== "" && v.items.length === 0)) throw new Error("unavailable");
  const items = v.items.map((value): HistoricalOrder => {
    const r = object(value, ["order_id", "ordered_at", "status", "total_minor", "currency", "items_summary", "city"]);
    if (!boundedText(r.order_id, 1, 64) || !archiveTime(r.ordered_at) || !boundedText(r.status, 1, 40) ||
      !Number.isSafeInteger(r.total_minor) || (r.total_minor as number) < 0 || (r.total_minor as number) > 1_000_000_000_000 ||
      r.currency !== "TWD" || !boundedText(r.items_summary, 0, 500) || !(r.city === null || cities.has(r.city as string)))
      throw new Error("unavailable");
    return r as HistoricalOrder;
  });
  if (new Set(items.map((r) => r.order_id)).size !== items.length) throw new Error("unavailable");
  return { items, next_cursor: v.next_cursor, total: v.total as number };
}

/** Exact read-only limit/after grammar of customer_historical.go; returns the requested page cap, or null. */
export function historicalOrdersQuery(rawURL: string): number | null {
  const at = rawURL.indexOf("?"); if (at < 0) return 50;
  const raw = rawURL.slice(at + 1); if (!raw || raw.length > 4096) return null;
  const seen = new Map<string, string>();
  for (const part of raw.split("&")) {
    const match = /^(limit|after)=([^=#+]*)$/.exec(part);
    if (!match || seen.has(match[1]) || match[2] === "") return null;
    seen.set(match[1], match[2]);
  }
  const limit = seen.get("limit"); const after = seen.get("after");
  if (limit !== undefined && !/^(?:[1-9]|[1-9][0-9]|100)$/.test(limit)) return null;
  if (after !== undefined && !canonicalCursor.test(after)) return null;
  return limit === undefined ? 50 : Number(limit);
}

/** Refuse non-GET, body, key and malformed scope/query before any authenticated backend read. */
export function validHistoricalOrdersRequest(request: Request, store: string, customer: string): boolean {
  const length = request.headers.get("content-length");
  return request.method === "GET" && canonicalUUID.test(store) && canonicalUUID.test(customer) && request.body === null &&
    !request.headers.has("idempotency-key") && !request.headers.has("transfer-encoding") && (length === null || length === "0") &&
    historicalOrdersQuery(request.url) !== null;
}

/** Fixed 50-item customer archive read URL; validates identifiers/cursor, carries no client tenant or money. */
export function historicalOrdersURL(store: string, customer: string, after = ""): string {
  if (!canonicalUUID.test(store) || !canonicalUUID.test(customer) || (after !== "" && !canonicalCursor.test(after))) throw new Error("invalid_request");
  return `/api/stores/${store}/customers/${customer}/historical-orders?limit=50${after ? `&after=${after}` : ""}`;
}
