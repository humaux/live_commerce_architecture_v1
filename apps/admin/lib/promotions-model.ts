// Admin discount-code model (contracts/storefront-v2.md §F): strict parser for the DTO of Go internal/httpapi/promotions.go (BFF
// `/api/stores/{store}/promotions[/{id}]`) and the pure form -> request-body builder that puts exactly the frozen keys on the wire.
// It never decides a rule: SQL (migration 0091 check_fields / refusal_for) is the authority for every limit, window and amount; a parser only
// refuses a malformed read and the builder only stops an obviously invalid body before it is sent. Times: the merchant types Asia/Taipei wall
// time; the wire carries RFC 3339 with +08:00 (Taiwan has no DST), the page shows the instant back in Asia/Taipei.
import { amountToMinor, minorToInput } from "./orders-model.ts";

export type PromoKind = "percent" | "fixed";
export type PromoStatus = "active" | "paused";
export type Promotion = {
  id: string;
  code: string;
  kind: PromoKind;
  percent: number | null;
  fixed_minor: number | null;
  min_subtotal_minor: number;
  starts_at: string | null;
  ends_at: string | null;
  total_limit: number | null;
  per_buyer_limit: number | null;
  status: PromoStatus;
  version: number;
  used: number;
  created_at: string;
};

const KEYS = ["id", "code", "kind", "percent", "fixed_minor", "min_subtotal_minor", "starts_at", "ends_at", "total_limit", "per_buyer_limit", "status", "version", "used", "created_at"];
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
export const CODE_PATTERN = /^[A-Z0-9-]{3,24}$/;
const MAX_MONEY = 1_000_000_000_000;

const isInt = (v: unknown, min: number, max: number): v is number => typeof v === "number" && Number.isSafeInteger(v) && v >= min && v <= max;
const isTime = (v: unknown): v is string => typeof v === "string" && Number.isFinite(Date.parse(v));

export function parsePromotion(value: unknown): Promotion {
  if (!value || typeof value !== "object" || Array.isArray(value)) throw new Error("unavailable");
  const v = value as Record<string, unknown>;
  if (Object.keys(v).sort().join(",") !== [...KEYS].sort().join(",")) throw new Error("unavailable");
  const optInt = (x: unknown, min: number, max: number) => x === null || isInt(x, min, max);
  const optTime = (x: unknown) => x === null || isTime(x);
  if (
    typeof v.id !== "string" || !UUID.test(v.id) || typeof v.code !== "string" || !CODE_PATTERN.test(v.code) ||
    (v.kind !== "percent" && v.kind !== "fixed") || !optInt(v.percent, 1, 90) || !optInt(v.fixed_minor, 1, MAX_MONEY) ||
    (v.kind === "percent") !== (v.percent !== null) || (v.kind === "fixed") !== (v.fixed_minor !== null) ||
    !isInt(v.min_subtotal_minor, 0, MAX_MONEY) || !optTime(v.starts_at) || !optTime(v.ends_at) ||
    !optInt(v.total_limit, 1, 1_000_000_000) || !optInt(v.per_buyer_limit, 1, 1_000_000) ||
    (v.status !== "active" && v.status !== "paused") || !isInt(v.version, 1, Number.MAX_SAFE_INTEGER) ||
    !isInt(v.used, 0, Number.MAX_SAFE_INTEGER) || !isTime(v.created_at)
  )
    throw new Error("unavailable");
  return v as Promotion;
}

// The list response is `{promotions: [...]}`; an unknown key or a duplicate code is a malformed read.
export function parsePromotions(value: unknown): Promotion[] {
  if (!value || typeof value !== "object" || Array.isArray(value) || Object.keys(value).join() !== "promotions") throw new Error("unavailable");
  const list = (value as { promotions: unknown }).promotions;
  if (!Array.isArray(list) || list.length > 500) throw new Error("unavailable");
  const rows = list.map(parsePromotion);
  if (new Set(rows.map((r) => r.code)).size !== rows.length) throw new Error("unavailable");
  return rows;
}

// ---- Asia/Taipei wall time <-> instant ----------------------------------------------------------------------------------------
const WALL = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/;
// "2026-10-01T09:30" (datetime-local, Taipei) -> RFC 3339 instant; null when empty or not a real date.
export function taipeiToInstant(wall: string): string | null | undefined {
  const text = wall.trim();
  if (text === "") return null;
  if (!WALL.test(text)) return undefined;
  const at = Date.parse(`${text}:00+08:00`);
  return Number.isFinite(at) ? `${text}:00+08:00` : undefined;
}
// Instant -> "YYYY-MM-DDTHH:mm" in Asia/Taipei, the value a datetime-local input takes back.
export function instantToTaipei(iso: string | null): string {
  if (iso === null) return "";
  const parts = new Intl.DateTimeFormat("en-CA", {
    timeZone: "Asia/Taipei", year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hourCycle: "h23",
  }).formatToParts(new Date(iso));
  const get = (type: string) => parts.find((p) => p.type === type)?.value ?? "";
  return `${get("year")}-${get("month")}-${get("day")}T${get("hour")}:${get("minute")}`;
}

// ---- form <-> body --------------------------------------------------------------------------------------------------------------
export type PromoForm = {
  code: string;
  kind: PromoKind;
  value: string; // percent: whole number 1..90; fixed: major units of the store currency
  minSubtotal: string; // major units, empty = none
  startsAt: string;
  endsAt: string;
  totalLimit: string; // empty = unlimited
  perBuyerLimit: string;
  status: PromoStatus;
};
export const emptyForm: PromoForm = { code: "", kind: "percent", value: "", minSubtotal: "", startsAt: "", endsAt: "", totalLimit: "", perBuyerLimit: "", status: "active" };

export function formFrom(p: Promotion, currency: string): PromoForm {
  return {
    code: p.code, kind: p.kind, value: p.kind === "percent" ? String(p.percent) : minorToInput(p.fixed_minor as number, currency),
    minSubtotal: p.min_subtotal_minor === 0 ? "" : minorToInput(p.min_subtotal_minor, currency), startsAt: instantToTaipei(p.starts_at),
    endsAt: instantToTaipei(p.ends_at), totalLimit: p.total_limit === null ? "" : String(p.total_limit),
    perBuyerLimit: p.per_buyer_limit === null ? "" : String(p.per_buyer_limit), status: p.status,
  };
}

export type PromoFields = {
  kind: PromoKind; percent: number | null; fixed_minor: number | null; min_subtotal_minor: number; starts_at: string | null; ends_at: string | null;
  total_limit: number | null; per_buyer_limit: number | null; status: PromoStatus;
};
const optionalCount = (text: string, max: number): number | null | undefined => {
  const t = text.trim();
  if (t === "") return null;
  return /^[0-9]{1,10}$/.test(t) && Number(t) >= 1 && Number(t) <= max ? Number(t) : undefined;
};

// The fields shared by create and update; undefined when any field is outside the rules (the page shows its generic "check the form" line).
export function promoFields(f: PromoForm, currency: string): PromoFields | undefined {
  const percent = f.kind === "percent" && /^[0-9]{1,2}$/.test(f.value.trim()) ? Number(f.value.trim()) : null;
  const fixed = f.kind === "fixed" ? amountToMinor(f.value, currency) : null;
  const min = f.minSubtotal.trim() === "" ? 0 : amountToMinor(f.minSubtotal, currency);
  const startsAt = taipeiToInstant(f.startsAt);
  const endsAt = taipeiToInstant(f.endsAt);
  const total = optionalCount(f.totalLimit, 1_000_000_000);
  const buyer = optionalCount(f.perBuyerLimit, 1_000_000);
  if (
    (f.kind === "percent" && (percent === null || percent < 1 || percent > 90)) || (f.kind === "fixed" && (fixed === null || fixed < 1)) ||
    min === null || startsAt === undefined || endsAt === undefined || total === undefined || buyer === undefined ||
    (startsAt !== null && endsAt !== null && Date.parse(endsAt) <= Date.parse(startsAt))
  )
    return undefined;
  return { kind: f.kind, percent, fixed_minor: fixed, min_subtotal_minor: min, starts_at: startsAt, ends_at: endsAt, total_limit: total, per_buyer_limit: buyer, status: f.status };
}

export type CreateBody = PromoFields & { code: string };
export type UpdateBody = PromoFields & { expected_version: number };
export function createBody(f: PromoForm, currency: string): CreateBody | undefined {
  const code = f.code.trim().toUpperCase();
  const fields = promoFields(f, currency);
  return fields && CODE_PATTERN.test(code) ? { code, ...fields } : undefined;
}
export function updateBody(f: PromoForm, currency: string, expectedVersion: number): UpdateBody | undefined {
  const fields = promoFields(f, currency);
  return fields && isInt(expectedVersion, 1, Number.MAX_SAFE_INTEGER - 1) ? { expected_version: expectedVersion, ...fields } : undefined;
}
// Pause / resume keeps every other field exactly as read (the server replaces the whole editable set).
export function toggleBody(p: Promotion): UpdateBody {
  return {
    expected_version: p.version, kind: p.kind, percent: p.percent, fixed_minor: p.fixed_minor, min_subtotal_minor: p.min_subtotal_minor,
    starts_at: p.starts_at, ends_at: p.ends_at, total_limit: p.total_limit, per_buyer_limit: p.per_buyer_limit, status: p.status === "active" ? "paused" : "active",
  };
}
