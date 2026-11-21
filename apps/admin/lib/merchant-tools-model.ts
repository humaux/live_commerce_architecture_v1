// Merchant-tools model (contracts/storefront-v2.md section G): strict parsers for the three BFF answers (dashboard, product import
// preview/commit, manual-order options and result), the client-side CSV size guard, the manual-order request builder and the BFF request
// grammar of apps/admin/app/api/stores/[store]/tools/[...resource]/route.ts (-> Go internal/httpapi/merchanttools.go).
// Framework-free on purpose (no next/*, no lib/auth.ts) so node --test can import it. The server decides every rule and every
// permission: nothing here gates an action; it refuses malformed answers and keeps junk (a price field, a tenant id) off the wire.
import { count, isInstant, object } from "./customers-model.ts";
import { canonicalUUID, parseSourcedOrderSummary, type OrderSummary } from "./orders-model.ts";

export const MAX_CSV_BYTES = 2 * 1024 * 1024;
export const MAX_CSV_ROWS = 1500; // measured ceiling of one all-or-nothing transaction (see internal/merchanttools/csvfile.go)
const maxMoney = 1_000_000_000_000;
const fail = () => new Error("unavailable");
const money = (v: unknown): v is number => Number.isSafeInteger(v) && (v as number) >= 0 && (v as number) <= maxMoney;

// ---- dashboard (G1) ----------------------------------------------------------------------------------------------------------
export type ModeTotals = { card_minor: number; bank_transfer_minor: number; pay_at_pickup_minor: number; cod_minor: number };
export type Gmv = { currency: string; environment: string; today: ModeTotals; last_7_days: ModeTotals };
export type Dashboard = {
  generated_at: string; timezone: "Asia/Taipei"; orders: { today: number; last_7_days: number }; gmv: Gmv[];
  todos: { awaiting_transfer_confirmation: number; to_ship: number; cvs_awaiting_label: number; low_stock_skus: number; open_refunds: number };
  latest_orders: OrderSummary[];
};
const modeKeys = ["card_minor", "bank_transfer_minor", "pay_at_pickup_minor", "cod_minor"];
function totals(value: unknown): ModeTotals {
  const v = object(value, modeKeys);
  if (!modeKeys.every((key) => money(v[key]))) throw fail();
  return v as ModeTotals;
}

export function parseDashboard(value: unknown): Dashboard {
  const v = object(value, ["generated_at", "timezone", "orders", "gmv", "todos", "latest_orders"]);
  const orders = object(v.orders, ["today", "last_7_days"]);
  const todos = object(v.todos, ["awaiting_transfer_confirmation", "to_ship", "cvs_awaiting_label", "low_stock_skus", "open_refunds"]);
  if (!isInstant(v.generated_at) || v.timezone !== "Asia/Taipei" || !count(orders.today) || !count(orders.last_7_days) ||
    (orders.today as number) > (orders.last_7_days as number) || !Object.values(todos).every(count) ||
    !Array.isArray(v.gmv) || v.gmv.length > 12 || !Array.isArray(v.latest_orders) || v.latest_orders.length > 10) throw fail();
  const gmv = v.gmv.map((item): Gmv => {
    const g = object(item, ["currency", "environment", "today", "last_7_days"]);
    if (typeof g.currency !== "string" || !/^[A-Z]{3}$/.test(g.currency) || !["LIVE", "SANDBOX"].includes(String(g.environment))) throw fail();
    const today = totals(g.today);
    const week = totals(g.last_7_days);
    // The 7-day window contains today: a smaller week than today is a broken projection.
    if (modeKeys.some((key) => today[key as keyof ModeTotals] > week[key as keyof ModeTotals])) throw fail();
    return { currency: g.currency, environment: g.environment as string, today, last_7_days: week };
  });
  if (new Set(gmv.map((g) => `${g.currency}|${g.environment}`)).size !== gmv.length) throw fail();
  return {
    generated_at: v.generated_at, timezone: "Asia/Taipei", orders: orders as Dashboard["orders"], gmv,
    todos: todos as Dashboard["todos"], latest_orders: v.latest_orders.map((item) => parseSourcedOrderSummary(item)),
  };
}

// ---- product import (G2) -----------------------------------------------------------------------------------------------------
export const rowErrorCodes = [
  "required", "too_long", "invalid_value", "duplicate_sku", "conflict_product_field", "slug_taken", "sku_in_other_product", "sku_archived",
  "option_mismatch", "unknown_warehouse", "unknown_collection", "limit", "conflict",
] as const;
export type RowErrorCode = (typeof rowErrorCodes)[number];
export type RowError = { row: number; column: string; code: RowErrorCode };
export type ImportResult = {
  file_sha256: string; rows: number; created_products: number; updated_products: number; created_skus: number; updated_skus: number;
  stock_adjustments: number; unchanged_rows: number; errors: RowError[]; errors_truncated: boolean; committed: boolean; replayed: boolean;
};
const importKeys = [
  "file_sha256", "rows", "created_products", "updated_products", "created_skus", "updated_skus", "stock_adjustments", "unchanged_rows",
  "errors", "errors_truncated", "committed", "replayed",
];
export function parseImportResult(value: unknown): ImportResult {
  const v = object(value, importKeys);
  if (typeof v.file_sha256 !== "string" || !/^[0-9a-f]{64}$/.test(v.file_sha256) ||
    !["rows", "created_products", "updated_products", "created_skus", "updated_skus", "stock_adjustments", "unchanged_rows"].every((key) => count(v[key])) ||
    typeof v.errors_truncated !== "boolean" || typeof v.committed !== "boolean" || typeof v.replayed !== "boolean" ||
    !Array.isArray(v.errors) || v.errors.length > 200) throw fail();
  const errors = v.errors.map((item): RowError => {
    const e = object(item, ["row", "column", "code"]);
    if (!count(e.row) || (e.row as number) > MAX_CSV_ROWS || typeof e.column !== "string" || e.column.length > 120 ||
      !(rowErrorCodes as readonly string[]).includes(String(e.code))) throw fail();
    return { row: e.row as number, column: e.column, code: e.code as RowErrorCode };
  });
  if (v.committed && errors.length > 0) throw fail(); // a commit with errors is never "committed"
  return { ...(v as unknown as ImportResult), errors };
}

/** Client-side guard before any upload: the same bounds as Go (2 MiB, UTF-8 text), so a wrong file never leaves the browser. */
export function csvFileProblem(name: string, size: number): "empty" | "too_large" | "not_csv" | null {
  if (!/\.csv$/i.test(name)) return "not_csv";
  if (size <= 0) return "empty";
  return size > MAX_CSV_BYTES ? "too_large" : null;
}

// ---- manual order (G3) -------------------------------------------------------------------------------------------------------
export const manualPaymentModes = ["bank_transfer", "pay_at_pickup", "cash_on_delivery"] as const;
export type ManualPaymentMode = (typeof manualPaymentModes)[number];
/** The order state begin_hold writes per mode (internal/checkout commercialAtPlacement): a COD order waits for the carrier, never CONFIRMED. */
const placementState = { bank_transfer: "AWAITING_TRANSFER", pay_at_pickup: "CONFIRMED", cash_on_delivery: "AWAITING_COLLECTION" } as const;
export type ManualOption = {
  option_key: string; market_id: string; country: string; delivery_code: string; delivery_kind: string; mode: string;
  name_hans: string; name_hant: string; name_en: string; currency: string; service_version: number; allocation_version: number;
  payment_modes: ManualPaymentMode[]; pickup_selection: "ecpay_map" | "buyer_entered" | null;
  // home-cod R5 (internal/checkout/options.go): only on a home row that lists cash_on_delivery. Whole TWD in minor units; the surcharge is omitted at 0.
  pickup_max_minor?: number; // Absent on older options and when pay at pickup is disabled.
  cod_surcharge_minor?: number; cod_carrier?: "black_cat" | "hsinchu"; cod_max_minor?: number;
};
const optionKeys = ["option_key", "market_id", "country", "delivery_code", "delivery_kind", "mode", "name_hans", "name_hant", "name_en",
  "currency", "service_version", "allocation_version", "payment_modes", "pickup_selection"];
const codOptionKeys = ["cod_surcharge_minor", "cod_carrier", "cod_max_minor"];
const wholeTWD = (v: unknown, min: number, max: number) => Number.isSafeInteger(v) && (v as number) >= min && (v as number) <= max && (v as number) % 100 === 0;
export const optionKeyPattern = /^[0-9a-f-]{36}\|[A-Z]{2}\|[a-z][a-z0-9_-]{0,39}$/;
export function parseManualOptions(value: unknown): ManualOption[] {
  const v = object(value, ["options"]);
  if (!Array.isArray(v.options) || v.options.length > 200) throw fail();
  return v.options.map((item): ManualOption => {
    if (!item || typeof item !== "object" || Array.isArray(item)) throw fail();
    const raw = item as Record<string, unknown>;
    // The cod_* keys are optional on the wire; everything else is the exact frozen key set.
    const o = object(Object.fromEntries(Object.entries(raw).filter(([k]) => !codOptionKeys.includes(k) && k !== "pickup_max_minor")), optionKeys);
    if (typeof o.option_key !== "string" || !optionKeyPattern.test(o.option_key) || typeof o.market_id !== "string" || !canonicalUUID.test(o.market_id) ||
      !/^[A-Z]{2}$/.test(String(o.country)) || !/^[a-z][a-z0-9_-]{0,39}$/.test(String(o.delivery_code)) ||
      !(o.delivery_kind === "home" || /^cvs_[a-z0-9]+$/.test(String(o.delivery_kind))) || typeof o.mode !== "string" ||
      ![o.name_hans, o.name_hant, o.name_en].every((n) => typeof n === "string" && n.length <= 120) || !/^[A-Z]{3}$/.test(String(o.currency)) ||
      !count(o.service_version) || !count(o.allocation_version) || !Array.isArray(o.payment_modes) || o.payment_modes.length < 1 ||
      !o.payment_modes.every((m) => (manualPaymentModes as readonly string[]).includes(String(m))) ||
      !(o.pickup_selection === null || o.pickup_selection === "ecpay_map" || o.pickup_selection === "buyer_entered") ||
      o.option_key !== `${o.market_id}|${o.country}|${o.delivery_code}`) throw fail();
    if ("pickup_max_minor" in raw && (o.currency !== "TWD" || o.delivery_kind === "home" || !o.payment_modes.includes("pay_at_pickup") ||
      !wholeTWD(raw.pickup_max_minor, 100, 2_000_000))) throw fail();
    // Cash on delivery: home delivery only, and the offer (carrier, cap in whole NT$ up to 20,000, fee in whole NT$ up to 1,000) travels with the mode.
    if (o.payment_modes.includes("cash_on_delivery")) {
      if (o.delivery_kind !== "home" || !wholeTWD(raw.cod_max_minor, 100, 2_000_000) || !(raw.cod_carrier === "black_cat" || raw.cod_carrier === "hsinchu") ||
        (raw.cod_surcharge_minor !== undefined && !wholeTWD(raw.cod_surcharge_minor, 0, 100_000))) throw fail();
    } else if (codOptionKeys.some((k) => k in raw)) throw fail();
    return raw as unknown as ManualOption;
  });
}

/** Only the scoped quote amounts and pickup cap may cross the manual-order BFF. */
export function parsePickupRefusal(value: unknown): { subtotal_minor: number; shipping_minor: number; total_minor: number; max_minor: number } | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) return null;
  const d = value as Record<string, unknown>;
  if (Object.keys(d).sort().join(",") !== "max_minor,shipping_minor,subtotal_minor,total_minor" ||
      !money(d.subtotal_minor) || !money(d.shipping_minor) || !money(d.total_minor) || !wholeTWD(d.max_minor, 100, 2_000_000)) return null;
  return { subtotal_minor: d.subtotal_minor, shipping_minor: d.shipping_minor, total_minor: d.total_minor, max_minor: Number(d.max_minor) };
}

export type ManualDiscount = { name: string; amount_minor: number };
const validDiscount = (v: unknown): v is ManualDiscount => {
 if (!v || typeof v !== "object" || Array.isArray(v)) return false;
 const d = v as Record<string, unknown>;
 return Object.keys(d).sort().join(",") === "amount_minor,name" && typeof d.name === "string" &&
  Array.from(d.name.trim()).length >= 1 && Array.from(d.name.trim()).length <= 30 && !/[\p{Cc}\p{Cf}\p{Cs}\p{Zl}\p{Zp}]/u.test(d.name) && money(d.amount_minor) && (d.amount_minor as number) > 0;
};
/** LP18: a placed manual order records the award outcome; both states are a successful order (ids, numbers, state only). */
export type ManualMemberAward =
  | { member_id: string; points: number; state: "awarded"; entry_id: string }
  | { member_id: string; points: number; state: "not_awarded"; reason: "member_erased" | "member_unavailable" | "forbidden" | "limit_reached" };
const awardReasons = ["member_erased", "member_unavailable", "forbidden", "limit_reached"];
/** The exact wire shape of internal/loyalty.AwardOutcome: awarded carries entry_id, not_awarded carries reason, never both. */
function validAwardOutcome(value: unknown): value is ManualMemberAward {
  if (!value || typeof value !== "object" || Array.isArray(value)) return false;
  const a = value as Record<string, unknown>;
  if (typeof a.member_id !== "string" || !canonicalUUID.test(a.member_id) || !count(a.points) || (a.points as number) < 1 || (a.points as number) > 1_000_000) return false;
  const keys = Object.keys(a).sort().join(",");
  if (a.state === "awarded") return keys === "entry_id,member_id,points,state" && typeof a.entry_id === "string" && canonicalUUID.test(a.entry_id);
  if (a.state === "not_awarded") return keys === "member_id,points,reason,state" && typeof a.reason === "string" && awardReasons.includes(a.reason);
  return false;
}
export type ManualResult = {
  order_id: string; commercial_state: "AWAITING_TRANSFER" | "CONFIRMED" | "AWAITING_COLLECTION"; payment_mode: ManualPaymentMode; total_minor: number; currency: string;
  expires_at: string; buyer_link: string | null; link_state: "configured" | "storefront_unavailable" | "domain_selection_required"; source: "merchant_manual"; discount?: ManualDiscount;
  member_award?: ManualMemberAward;
};
export function parseManualResult(value: unknown): ManualResult {
  if (!value || typeof value !== "object" || Array.isArray(value)) throw fail();
  const raw = value as Record<string, unknown>;
  if ("discount" in raw && (!validDiscount(raw.discount) || raw.discount.name !== raw.discount.name.trim())) throw fail();
  if ("member_award" in raw && !validAwardOutcome(raw.member_award)) throw fail();
  const v = object(raw, ["order_id", "commercial_state", "payment_mode", "total_minor", "currency", "expires_at", "buyer_link", "link_state", "source",
    ...("discount" in raw ? ["discount"] : []), ...("member_award" in raw ? ["member_award"] : [])]);
  if (typeof v.order_id !== "string" || !canonicalUUID.test(v.order_id) ||
    !(manualPaymentModes as readonly string[]).includes(String(v.payment_mode)) || v.commercial_state !== placementState[v.payment_mode as ManualPaymentMode] ||
    !money(v.total_minor) || !/^[A-Z]{3}$/.test(String(v.currency)) ||
    !isInstant(v.expires_at) || !["configured", "storefront_unavailable", "domain_selection_required"].includes(String(v.link_state)) ||
    v.source !== "merchant_manual") throw fail();
  // The link is an https URL whose fragment carries the order id and a 43-character capability; nothing else is accepted.
  if (v.link_state === "configured") {
    if (typeof v.buyer_link !== "string" || v.buyer_link.length > 400) throw fail();
    const url = new URL(v.buyer_link);
    if (url.protocol !== "https:" || url.search !== "" || !/^#o=[0-9a-f-]{36}&t=[A-Za-z0-9_-]{43}$/.test(url.hash) || !url.hash.includes(v.order_id)) throw fail();
  } else if (v.buyer_link !== null) throw fail();
  return v as unknown as ManualResult;
}

export type ManualDraft = {
  lines: { sku_id: string; quantity: number }[];
  name: string; phone: string; email: string;
  option: ManualOption | null; mode: ManualPaymentMode | "";
  home: { region: string; city: string; postal_code: string; line1: string; line2: string };
  cvs: { store_code: string; store_name: string; store_address: string };
  locale: "zh-CN" | "zh-TW" | "en";
};
export type DraftProblem = "lines" | "name" | "phone" | "email" | "delivery" | "mode" | "address" | "store" | "pickupName" | "pickupPhone";
const phoneOK = (s: string) => /^[0-9+() -]{6,32}$/.test(s) && s.replace(/\D/g, "").length >= 6 && s.replace(/\D/g, "").length <= 20;

/** Mirrors fulfillment.ecpay_recipient_ok (0073). Blank fields keep the normal required-field feedback. */
export function pickupRecipientErrors(name: string, phone: string) {
  const recipient = name.trim();
  const width = recipient.length + (recipient.match(/[㐀-䶿一-鿿豈-﫿Ａ-Ｚａ-ｚ]/g)?.length ?? 0);
  const nameValid = width >= 4 && width <= 10 && /^[A-Za-z㐀-䶿一-鿿豈-﫿Ａ-Ｚａ-ｚ]+(?: [A-Za-z㐀-䶿一-鿿豈-﫿Ａ-Ｚａ-ｚ]+)*$/.test(recipient);
  const mobile = phone.trim().replace(/[ ()-]/g, "");
  return { name: recipient !== "" && !nameValid, phone: phone.trim() !== "" && !/^(?:09|\+8869)[0-9]{8}$/.test(mobile) };
}

/** First problem of a draft (null = sendable). The server re-validates everything; this keeps an obviously wrong form from being sent.
 *  `skipRecipient` leaves out the two recipient checks (phone, CVS store) so recipientGaps can tell a recipient-only gap apart. */
export function draftProblem(d: ManualDraft, skipRecipient = false): DraftProblem | null {
  if (d.lines.length < 1 || d.lines.length > 50 || d.lines.some((l) => !canonicalUUID.test(l.sku_id) || !Number.isInteger(l.quantity) || l.quantity < 1 || l.quantity > 1000) ||
    new Set(d.lines.map((l) => l.sku_id)).size !== d.lines.length) return "lines";
  if (d.name.trim().length < 1 || Array.from(d.name.trim()).length > 120) return "name";
  if (!skipRecipient && !phoneOK(d.phone.trim())) return "phone";
  if (d.email.trim() !== "" && !(d.email.trim().length <= 254 && /^[^\s@]+@[^\s@]+$/.test(d.email.trim()))) return "email";
  if (!d.option) return "delivery";
  if (d.mode === "" || !d.option.payment_modes.includes(d.mode)) return "mode";
  if (d.mode === "pay_at_pickup") {
    const errors = pickupRecipientErrors(d.name, d.phone);
    if (errors.name) return "pickupName";
    if (errors.phone) return "pickupPhone";
  }
  if (d.mode === "cash_on_delivery" && d.option.delivery_kind !== "home") return "mode"; // COD is home delivery only
  if (d.option.delivery_kind === "home") {
    if (d.home.city.trim() === "" || d.home.line1.trim() === "") return "address";
  } else if (!skipRecipient && !cvsComplete(d.cvs)) return "store";
  return null;
}
const cvsComplete = (c: ManualDraft["cvs"]) => c.store_code.trim() !== "" && c.store_name.trim() !== "" && Array.from(c.store_address.trim()).length >= 5;

export type RecipientGap = "phone" | "store";
/** The signed 訂單缺少收件資料 pre-submit state: the recipient fields still missing when everything else in the draft is sendable
 *  (empty = no recipient-only gap; the ordinary first-problem hint covers every other case). Same rules as draftProblem, no new field. */
export function recipientGaps(d: ManualDraft): RecipientGap[] {
  if (draftProblem(d, true) !== null) return [];
  const gaps: RecipientGap[] = [];
  if (!phoneOK(d.phone.trim())) gaps.push("phone");
  if (d.option!.delivery_kind !== "home" && !cvsComplete(d.cvs)) gaps.push("store");
  return gaps;
}

/** The exact POST body. It has no price, discount, shipping or tenant field: the quote is the only price and the bearer is the only scope. */
export function manualBody(d: ManualDraft): Record<string, unknown> {
  const option = d.option!;
  return {
    items: d.lines.map((l) => ({ sku_id: l.sku_id, quantity: l.quantity })),
    customer: { name: d.name.trim(), phone: d.phone.trim(), email: d.email.trim() },
    delivery: {
      option_key: option.option_key,
      home_address: option.delivery_kind === "home"
        ? { region: d.home.region.trim(), city: d.home.city.trim(), postal_code: d.home.postal_code.trim(), line1: d.home.line1.trim(), line2: d.home.line2.trim() }
        : null,
      cvs: option.delivery_kind === "home" ? null : { store_code: d.cvs.store_code.trim(), store_name: d.cvs.store_name.trim(), store_address: d.cvs.store_address.trim() },
    },
    payment_mode: d.mode,
    locale: d.locale,
  };
}

// ---- manual link regenerate (G3, K3 F2) ------------------------------------------------------------------------------------
export type RegenerateResult = { order_id: string; buyer_link: string; source: "merchant_manual" };
export function parseRegenerateResult(value: unknown): RegenerateResult {
  const v = object(value, ["order_id", "buyer_link", "source"]);
  if (typeof v.order_id !== "string" || !canonicalUUID.test(v.order_id) || typeof v.buyer_link !== "string" || v.buyer_link.length > 400 ||
    v.source !== "merchant_manual") throw fail();
  const url = new URL(v.buyer_link);
  if (url.protocol !== "https:" || url.search !== "" || !/^#o=[0-9a-f-]{36}&t=[A-Za-z0-9_-]{43}$/.test(url.hash) || !url.hash.includes(v.order_id)) throw fail();
  return v as unknown as RegenerateResult;
}

// ---- BFF request grammar -----------------------------------------------------------------------------------------------------
export type ToolsRoute = "dashboard" | "export" | "import-preview" | "import-commit" | "manual-options" | "manual-place" | "manual-regenerate";
/** The only resources of the tools BFF, per method. Everything else is a 404 there. */
export function toolsRoute(method: string, path: string): ToolsRoute | null {
  const table: Record<string, Record<string, ToolsRoute>> = {
    GET: { dashboard: "dashboard", "products/export.csv": "export", "orders/manual/options": "manual-options" },
    POST: { "products/import/preview": "import-preview", "products/import/commit": "import-commit", "orders/manual": "manual-place", "orders/manual/regenerate-link": "manual-regenerate" },
  };
  return table[method]?.[path] ?? null;
}
/** A manual-order body is JSON with exactly these top-level keys and no price-like field anywhere. */
export function validManualBody(body: unknown): boolean {
  if (!body || typeof body !== "object" || Array.isArray(body)) return false;
  const keys = Object.keys(body).filter((key) => key !== "member_award").sort().join(",");
  const raw = body as Record<string, unknown>;
  if (keys !== "customer,delivery,items,locale,payment_mode" && keys !== "customer,delivery,discount,items,locale,payment_mode") return false;
  if ("discount" in raw && !validDiscount(raw.discount)) return false;
  // LP-1 §2: one optional, closed member award; Go/PG own rights and balance checks.
  if ("member_award" in raw) {
    const award = raw.member_award;
    if (!award || typeof award !== "object" || Array.isArray(award)) return false;
    const a = award as Record<string, unknown>;
    if (Object.keys(a).sort().join(",") !== "member_id,points" || typeof a.member_id !== "string" || !canonicalUUID.test(a.member_id) ||
        typeof a.points !== "number" || !Number.isInteger(a.points) || a.points < 1 || a.points > 1000000) return false;
  }
  const forbidden = /(price|amount|total|discount|shipping|tax|tenant|store_id)/i;
  const walk = (value: unknown): boolean => {
    if (Array.isArray(value)) return value.every(walk);
    if (value && typeof value === "object") return Object.entries(value).every(([k, v]) => !forbidden.test(k) && walk(v));
    return true;
  };
  return walk(Object.fromEntries(Object.entries(raw).filter(([key]) => key !== "discount")));
}
/** A regenerate-link body is JSON with exactly {order_id, locale}; the order id is a canonical UUID, the locale one of the three. */
export function validRegenerateBody(body: unknown): boolean {
  if (!body || typeof body !== "object" || Array.isArray(body)) return false;
  const v = body as Record<string, unknown>;
  return Object.keys(v).sort().join(",") === "locale,order_id" && typeof v.order_id === "string" && canonicalUUID.test(v.order_id) &&
    typeof v.locale === "string" && ["zh-CN", "zh-TW", "en"].includes(v.locale);
}
