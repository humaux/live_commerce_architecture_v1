// Frozen D7/D9 projection: GET /api/stores/{store}/ads/attribution -> Go /v1/admin/stores/{store}/ads/attribution.
// I12: only reported aggregates; absent or forged fields fail closed, never become zero or buyer identities.
export type Buyers = {
  counties: { name: string; orders: number; net_minor: number }[];
  new_buyers: number;
  returning_buyers: number;
  average_order_minor: number | null;
  top_products: { product_id: string; name: string; quantity: number }[];
  orders_per_minute: { at: string; orders: number; net_minor: number }[];
};
export type Breakdown = {
  day: string;
  timezone_name: string;
  dimension: "age_gender" | "region" | "placement" | "device" | "hourly";
  bucket: string;
  hour_start: string | null;
  spend_minor: number | null;
  reach: number | null;
  impressions: number | null;
  clicks: number | null;
  engagements: number | null;
  comments: number | null;
  purchases: number | null;
  purchase_value_minor: number | null;
};
export type DraftAttribution = {
  draft_id: string;
  source_ref: string;
  template: "BOOST_POST" | "PRODUCT_TRAFFIC";
  currency: "TWD" | "HKD" | "USD";
  meta_account_timezone: string | null;
  spend_minor: number | null;
  orders: {
    path: "ad_click" | "boosted_post";
    orders: number;
    net_minor: number;
    pending_orders: number;
    pending_minor: number;
  }[];
  meta: { purchases: number | null; purchase_value_minor: number | null };
  roas: number | null;
  provisional: boolean;
  breakdowns: Breakdown[];
  breakdowns_unavailable: { day: string; dimensions: string[] }[];
  buyers: Buyers;
};
export type SessionAttribution = {
  session_id: string;
  title: string;
  starts_at: string;
  ends_at: string | null;
  post_ids: string[];
  draft_ids: string[];
  currency: string;
  spend_minor: number | null;
  orders: number;
  net_minor: number;
  pending_orders: number;
  pending_minor: number;
  ambiguous_orders: number;
  funnel: {
    comments: number;
    claims: number;
    checkout_links: number;
    paid_orders: number;
  };
  buyers: Buyers;
  timeline: {
    at: string;
    spend_minor: number | null;
    orders: number;
    net_minor: number;
    comments: number;
    claims: number;
    viewers: number | null;
  }[];
  live_audience: {
    status: "not_read" | "not_authorized" | "insufficient" | "available";
    views: number | null;
    peak_concurrent: number | null;
    total_view_time_ms: number | null;
    age_gender: { bucket: string; view_time_ms: number }[];
    regions: { bucket: string; view_time_ms: number }[];
  };
};
export type AttributionReport = {
  window: { from: string; to: string };
  order_timezone: "Asia/Taipei";
  truncated: boolean;
  drafts: DraftAttribution[];
  sessions: SessionAttribution[];
};

type Reader<T> = (v: unknown) => T;
const fail = (): never => {
  throw new Error("attribution_shape");
};
const string: Reader<string> = (v) =>
  typeof v === "string" && v.length <= 2048 ? v : fail();
const count: Reader<number> = (v) =>
  typeof v === "number" && Number.isSafeInteger(v) && v >= 0 ? v : fail();
const signed: Reader<number> = (v) =>
  typeof v === "number" && Number.isSafeInteger(v) ? v : fail();
const ratio: Reader<number> = (v) =>
  typeof v === "number" && Number.isFinite(v) ? v : fail();
const bool: Reader<boolean> = (v) => (typeof v === "boolean" ? v : fail());
const nullable =
  <T>(read: Reader<T>): Reader<T | null> =>
  (v) =>
    v === null ? null : read(v);
const array =
  <T>(read: Reader<T>): Reader<T[]> =>
  (v) =>
    Array.isArray(v) && v.length <= 100000 ? v.map(read) : fail();
const choices =
  <T extends string>(values: readonly T[]): Reader<T> =>
  (v) =>
    typeof v === "string" && values.includes(v as T) ? (v as T) : fail();
const object =
  <T>(fields: { [K in keyof T]: Reader<T[K]> }): Reader<T> =>
  (v) => {
    if (!v || typeof v !== "object" || Array.isArray(v)) return fail();
    const input = v as Record<string, unknown>,
      result: Record<string, unknown> = {};
    for (const key of Object.keys(fields))
      result[key] = fields[key as keyof T](input[key]);
    return result as T; // Only whitelisted fields survive, including nested objects.
  };
const uuid: Reader<string> = (v) =>
  /^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/.test(string(v))
    ? (v as string)
    : fail();
const instant: Reader<string> = (v) => {
  const s = string(v);
  return /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/.test(
    s,
  ) && Number.isFinite(Date.parse(s))
    ? s
    : fail();
};
const day: Reader<string> = (v) => {
  const s = string(v);
  return /^\d{4}-\d{2}-\d{2}$/.test(s) &&
    Number.isFinite(Date.parse(s)) &&
    new Date(s).toISOString().slice(0, 10) === s
    ? s
    : fail();
};
const zone: Reader<string> = (v) => {
  const s = string(v);
  // Structural account-zone label only. Go owns IANA resolution and hourly conversion;
  // this page renders absolute timestamps through packages/format, never this string.
  return /^(?:UTC|GMT|[A-Za-z_]+(?:\/[A-Za-z0-9_+-]+)+)$/.test(s) ? s : fail();
};
const currency = choices(["TWD", "HKD", "USD"] as const);
const minute = object<Buyers["orders_per_minute"][number]>({
  at: instant,
  orders: count,
  net_minor: signed,
});
const buyers = object<Buyers>({
  counties: array(object({ name: string, orders: count, net_minor: signed })),
  new_buyers: count,
  returning_buyers: count,
  average_order_minor: nullable(signed),
  top_products: array(
    object({ product_id: uuid, name: string, quantity: count }),
  ),
  orders_per_minute: array(minute),
});
const breakdown = object<Breakdown>({
  day,
  timezone_name: zone,
  dimension: choices(["age_gender", "region", "placement", "device", "hourly"]),
  bucket: string,
  hour_start: nullable(instant),
  spend_minor: nullable(count),
  reach: nullable(count),
  impressions: nullable(count),
  clicks: nullable(count),
  engagements: nullable(count),
  comments: nullable(count),
  purchases: nullable(count),
  purchase_value_minor: nullable(signed),
});
const draft = object<DraftAttribution>({
  draft_id: uuid,
  source_ref: string,
  template: choices(["BOOST_POST", "PRODUCT_TRAFFIC"]),
  currency,
  meta_account_timezone: nullable(zone),
  spend_minor: nullable(count),
  orders: array(
    object({
      path: choices(["ad_click", "boosted_post"]),
      orders: count,
      net_minor: signed,
      pending_orders: count,
      pending_minor: count,
    }),
  ),
  meta: object({
    purchases: nullable(count),
    purchase_value_minor: nullable(signed),
  }),
  roas: nullable(ratio),
  provisional: bool,
  breakdowns: array(breakdown),
  breakdowns_unavailable: array(object({ day, dimensions: array(string) })),
  buyers,
});
const audience = object<SessionAttribution["live_audience"]>({
  status: choices(["not_read", "not_authorized", "insufficient", "available"]),
  views: nullable(count),
  peak_concurrent: nullable(count),
  total_view_time_ms: nullable(count),
  age_gender: array(object({ bucket: string, view_time_ms: count })),
  regions: array(object({ bucket: string, view_time_ms: count })),
});
const session = object<SessionAttribution>({
  session_id: uuid,
  title: string,
  starts_at: instant,
  ends_at: nullable(instant),
  post_ids: array(string),
  draft_ids: array(uuid),
  currency,
  spend_minor: nullable(count),
  orders: count,
  net_minor: signed,
  pending_orders: count,
  pending_minor: count,
  ambiguous_orders: count,
  funnel: object({
    comments: count,
    claims: count,
    checkout_links: count,
    paid_orders: count,
  }),
  buyers,
  timeline: array(
    object({
      at: instant,
      spend_minor: nullable(count),
      orders: count,
      net_minor: signed,
      comments: count,
      claims: count,
      viewers: nullable(count),
    }),
  ),
  live_audience: audience,
});
const report = object<AttributionReport>({
  window: object({ from: day, to: day }),
  order_timezone: choices(["Asia/Taipei"]),
  truncated: bool,
  drafts: array(draft),
  sessions: array(session),
});
export function parseAttributionReport(value: unknown): AttributionReport {
  const r = report(value);
  if (
    r.window.from > r.window.to ||
    new Set(r.drafts.map((d) => d.draft_id)).size !== r.drafts.length ||
    new Set(r.sessions.map((s) => s.session_id)).size !== r.sessions.length
  )
    fail();
  for (const d of r.drafts) {
    if (new Set(d.orders.map((o) => o.path)).size !== d.orders.length) fail();
    for (const b of d.breakdowns)
      if ((b.dimension === "hourly") !== (b.hour_start !== null)) fail();
  }
  return r;
}
