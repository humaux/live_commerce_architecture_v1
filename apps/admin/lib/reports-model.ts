// Purpose: Strict decoders for the four frozen W6-02B report DTOs (contracts/reporting-v2.md, internal/reporting):
//   product rows sorted by net desc with declared truncation, channel buckets in the closed vocabulary order with
//   per-environment money, nested funnel counts, and manual-order buckets with nullable principal/session ids.
//   Every invariant Go re-verifies (I05 net = captured - refunded, ascending environments, 0..91-day echo) is checked
//   again here; a report that disagrees with itself is refused ("unavailable"), never shown.
// Depends on: ./orders-model.ts (canonicalUUID), ./customers-model.ts (object).
// Used by: apps/admin/lib/reports-client.ts, apps/admin/components/Reports.tsx, tests/admin/reports-bff.test.ts
import { canonicalUUID } from "./orders-model.ts";
import { object } from "./customers-model.ts";

export type ReportEnvironment = "LIVE" | "SANDBOX";
export type ReportMoney = {
  environment: ReportEnvironment;
  captured_count: number;
  captured_minor: number;
  refunded_minor: number;
  net_minor: number;
  offline_count: number;
  offline_minor: number;
};
export type ProductRow = {
  sku_id: string;
  product_id: string;
  code: string;
  name: string;
  currency: string;
  environment: ReportEnvironment;
  units: number;
  captured_minor: number;
  refunded_minor: number;
  net_minor: number;
  offline_units: number;
  offline_minor: number;
};
export type ProductReport = { from: string; to: string; timezone: string; truncated: boolean; rows: ProductRow[] };
export type ChannelRow = { channel: string; currency: string; orders: number; cancelled_orders: number; money: ReportMoney[] };
export type ChannelReport = { from: string; to: string; timezone: string; rows: ChannelRow[] };
export type FunnelReport = {
  from: string;
  to: string;
  timezone: string;
  session_id: string | null;
  claimed: number;
  link_sent: number;
  ordered: number;
  paid: number;
  ordered_without_link: number;
};
export type ManualRow = {
  principal_id: string | null;
  session_id: string | null;
  currency: string;
  orders: number;
  cancelled_orders: number;
  money: ReportMoney[];
};
export type ManualReport = { from: string; to: string; timezone: string; rows: ManualRow[] };

export const reportTimezone = "Asia/Taipei";
export const maxProductRows = 1000; // reporting.MaxProductRows; truncation must declare exactly this many
export const reportChannels = ["facebook_live", "instagram_live", "storefront", "manual"] as const;
// Channel/manual row counts have no backend cap; never invent a 500-bucket contract.

const currencyPattern = /^[A-Z]{3}$/;
function int(value: unknown): value is number {
  return Number.isSafeInteger(value) && (value as number) >= 0;
}
function isEnvironment(value: unknown): value is ReportEnvironment {
  return value === "LIVE" || value === "SANDBOX";
}
function head(value: unknown, keys: string[], from: string, to: string): Record<string, unknown> {
  const v = object(value, keys);
  if (v.from !== from || v.to !== to || v.timezone !== reportTimezone) throw new Error("unavailable");
  return v;
}
function parseMoney(value: unknown): ReportMoney {
  const v = object(value, ["environment", "captured_count", "captured_minor", "refunded_minor", "net_minor", "offline_count", "offline_minor"]);
  if (!isEnvironment(v.environment) || !int(v.captured_count) || !int(v.captured_minor) || !int(v.refunded_minor) ||
    !int(v.offline_count) || !int(v.offline_minor) || !Number.isSafeInteger(v.net_minor) ||
    // I05: net is exactly captured minus refunded; offline money is never part of captured or net.
    v.net_minor !== (v.captured_minor as number) - (v.refunded_minor as number) ||
    (v.captured_count === 0 && v.captured_minor !== 0) || (v.offline_count === 0 && v.offline_minor !== 0))
    throw new Error("unavailable");
  return v as ReportMoney;
}
// Environments ascend strictly ("LIVE" < "SANDBOX"), so a bucket carries at most the two deployments.
function parseMoneyList(value: unknown): ReportMoney[] {
  if (!Array.isArray(value) || value.length > 2) throw new Error("unavailable");
  const money = value.map(parseMoney);
  if (money.length === 2 && money[0].environment >= money[1].environment) throw new Error("unavailable");
  return money;
}

/** Decode exact product DTO and verify money/truncation; no side effects. */
export function parseProductReport(value: unknown, from: string, to: string): ProductReport {
  const v = head(value, ["from", "to", "timezone", "truncated", "rows"], from, to);
  if (typeof v.truncated !== "boolean" || !Array.isArray(v.rows) || v.rows.length > maxProductRows ||
    (v.truncated && v.rows.length !== maxProductRows)) throw new Error("unavailable");
  let previous: number | null = null;
  const rows = v.rows.map((row) => {
    const r = object(row, ["sku_id", "product_id", "code", "name", "currency", "environment", "units",
      "captured_minor", "refunded_minor", "net_minor", "offline_units", "offline_minor"]);
    if (typeof r.sku_id !== "string" || !canonicalUUID.test(r.sku_id) || typeof r.product_id !== "string" || !canonicalUUID.test(r.product_id) ||
      typeof r.code !== "string" || typeof r.name !== "string" ||
      typeof r.currency !== "string" || !currencyPattern.test(r.currency) || !isEnvironment(r.environment) ||
      !int(r.units) || !int(r.captured_minor) || !int(r.refunded_minor) || !int(r.offline_units) || !int(r.offline_minor) ||
      !Number.isSafeInteger(r.net_minor) || r.net_minor !== (r.captured_minor as number) - (r.refunded_minor as number) ||
      (previous !== null && previous < (r.net_minor as number))) throw new Error("unavailable");
    previous = r.net_minor as number;
    return r as ProductRow;
  });
  return { from, to, timezone: reportTimezone, truncated: v.truncated, rows };
}

/** Decode exact channel DTO and verify per-environment money; no side effects. */
export function parseChannelReport(value: unknown, from: string, to: string): ChannelReport {
  const v = head(value, ["from", "to", "timezone", "rows"], from, to);
  if (!Array.isArray(v.rows)) throw new Error("unavailable");
  let last = -1;
  const rows = v.rows.map((row) => {
    const r = object(row, ["channel", "currency", "orders", "cancelled_orders", "money"]);
    const at = (reportChannels as readonly string[]).indexOf(String(r.channel));
    if (at < 0 || at < last || typeof r.currency !== "string" || !currencyPattern.test(r.currency) ||
      !int(r.orders) || !int(r.cancelled_orders)) throw new Error("unavailable");
    last = at;
    return { ...r, money: parseMoneyList(r.money) } as ChannelRow;
  });
  return { from, to, timezone: reportTimezone, rows };
}

/** Decode a nested cohort funnel with exact session/range echoes; no side effects. */
export function parseFunnelReport(value: unknown, from: string, to: string, sessionId: string): FunnelReport {
  const v = head(value, ["from", "to", "timezone", "session_id", "claimed", "link_sent", "ordered", "paid", "ordered_without_link"], from, to);
  const session = v.session_id;
  if ((sessionId === "") !== (session === null) || (session !== null && session !== sessionId) ||
    (sessionId !== "" && !canonicalUUID.test(sessionId)) ||
    !int(v.claimed) || !int(v.link_sent) || !int(v.ordered) || !int(v.paid) || !int(v.ordered_without_link) ||
    // Stages are nested: paid <= ordered <= link_sent <= claimed, and unlinked orders fit inside the claim total.
    (v.paid as number) > (v.ordered as number) || (v.ordered as number) > (v.link_sent as number) ||
    (v.link_sent as number) > (v.claimed as number) || (v.link_sent as number) + (v.ordered_without_link as number) > (v.claimed as number))
    throw new Error("unavailable");
  return {
    from, to, timezone: reportTimezone, session_id: session as string | null,
    claimed: v.claimed as number, link_sent: v.link_sent as number, ordered: v.ordered as number,
    paid: v.paid as number, ordered_without_link: v.ordered_without_link as number,
  };
}

/** Decode manual-order buckets with nullable identities; no side effects. */
export function parseManualReport(value: unknown, from: string, to: string): ManualReport {
  const v = head(value, ["from", "to", "timezone", "rows"], from, to);
  if (!Array.isArray(v.rows)) throw new Error("unavailable");
  const rows = v.rows.map((row) => {
    const r = object(row, ["principal_id", "session_id", "currency", "orders", "cancelled_orders", "money"]);
    if (!(r.principal_id === null || (typeof r.principal_id === "string" && canonicalUUID.test(r.principal_id))) ||
      !(r.session_id === null || (typeof r.session_id === "string" && canonicalUUID.test(r.session_id))) ||
      typeof r.currency !== "string" || !currencyPattern.test(r.currency) || !int(r.orders) || !int(r.cancelled_orders))
      throw new Error("unavailable");
    return { ...r, money: parseMoneyList(r.money) } as ManualRow;
  });
  return { from, to, timezone: reportTimezone, rows };
}
