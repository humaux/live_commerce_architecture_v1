// Purpose: closed A15/A16 DTOs and transient for-buyer form mapping; no client quote or identity inference.
// Depends on: shared format, manual-order-form, merchant-tools-model and customers/orders validation.
// Used by: CreateOrderDrawer, create-order-client and authenticated tools catchall BFF.
// Invariants: I01/I02/I05/I08/I09; live-console-v1 §5, A15/A16.
import { money as formatMoney } from '../../../packages/format/src/index.ts';
import { object } from "./customers-model.ts";
import { canonicalUUID } from "./orders-model.ts";
import {
  manualBody,
  draftProblem,
  manualPaymentModes,
  optionKeyPattern,
  parseManualResult,
  type ManualDraft,
  type ManualOption,
  type ManualResult,
} from "./merchant-tools-model.ts";
import { emptyManualForm, type ManualFormValues } from "./manual-order-form.ts";

/** Exactly one private target selector for A15. */
export type DrawerTarget = { conversationId?: string; bundleId?: string };
/** A15 claim line; prices are informational unit references, never quote inputs. */
export type PrefillItem = {
  sku_id: string;
  offer_id: string;
  keyword: string;
  name: string;
  variant: string;
  quantity: number;
  live_price_minor: number | null;
  catalog_price_minor: number;
  sellable: boolean;
  live_quantity_remaining: number;
};
/** A15 response; customer authority comes solely from an explicit merchant link. */
export type OrderPrefill = {
  items: PrefillItem[];
  bundles: string[];
  customer: { name: string; phone: string; email: string | null } | null;
  last_delivery:
    | (
        | { option_key: string; cvs: ManualDraft["cvs"] }
        | { option_key: string; home_address: ManualDraft["home"] }
      )
    | null;
  suggested_option_key: string | null;
  live_price_eligible: boolean;
  live_price_reason: string;
};
/** Exact price-free A16 input, with inactive delivery detail explicitly null. */
export type ForBuyerBody = {
  items: { sku_id: string; quantity: number }[];
  customer: { name: string; phone: string; email: string };
  delivery: {
    option_key: string;
    home_address: ManualDraft["home"] | null;
    cvs: ManualDraft["cvs"] | null;
  };
  payment_mode: ManualDraft["mode"];
  locale: ManualDraft["locale"];
  for: { bundle_ids: string[]; conversation_id: string | null };
  send_payment_link: boolean;
};
/** A16 result; replay can redact an otherwise configured buyer capability. */
export type ForBuyerResult = ManualResult & {
  live_price: "applied" | "not_applied";
  live_price_reason: string;
  send:
    | { state: "queued"; operation_id: string }
    | { state: "not_sent"; reason: string };
};
/** Safe creation outcome; uncertain commands can only be retried with their original receipt. */
export type ForBuyerOutcome =
  | { ok: true; value: ForBuyerResult; status: number }
  | {
      ok: false;
      code: string;
      uncertain: boolean;
      status: number;
      order_id?: string | null;
    };

/** Closed server live-price explanations, plus a safe unknown fallback. */
export const livePriceReasons = [
  "",
  "no_bundle",
  "no_conversation",
  "bundle_buyer_unverified",
  "bundle_buyer_mismatch",
  "permission",
  "no_live_line",
  "live_price_unavailable",
  "some_lines_catalog",
  "unknown",
] as const;
/** Closed planner refusals; the UI never reflects provider diagnostics. */
export const forBuyerSendReasons = [
  "not_requested",
  "no_conversation",
  "send_unavailable",
  "link_unavailable",
  "window_closed",
  "capability",
  "takeover_changed",
  "duplicate_recent",
  "rate_limited",
  "unknown",
] as const;
const failure = () => new Error("unavailable");
const id = (v: unknown): v is string =>
  typeof v === "string" && canonicalUUID.test(v);
const text = (v: unknown, max: number, min = 0): v is string =>
  typeof v === "string" &&
  Array.from(v).length >= min &&
  Array.from(v).length <= max &&
  !/[\u0000-\u001f\u007f-\u009f]/u.test(v);
const integer = (v: unknown, min: number, max: number): v is number =>
  Number.isSafeInteger(v) && (v as number) >= min && (v as number) <= max;
const money = (v: unknown) => integer(v, 0, 1_000_000_000_000);
const reason = (v: unknown, allowed: readonly string[]) => {
  if (!text(v, 128)) throw failure();
  return allowed.includes(v) ? v : "unknown";
};
const ids = (v: unknown, max: number): v is string[] =>
  Array.isArray(v) &&
  v.length <= max &&
  v.every(id) &&
  new Set(v).size === v.length;
function home(value: unknown): ManualDraft["home"] {
  const v = object(value, ["region", "city", "postal_code", "line1", "line2"]);
  if (
    !text(v.region, 100) ||
    !text(v.city, 100, 1) ||
    !text(v.postal_code, 20) ||
    !text(v.line1, 200, 1) ||
    !text(v.line2, 200)
  )
    throw failure();
  return v as ManualDraft["home"];
}
function cvs(value: unknown): ManualDraft["cvs"] {
  const v = object(value, ["store_code", "store_name", "store_address"]);
  if (
    !text(v.store_code, 32, 1) ||
    !text(v.store_name, 40, 1) ||
    !text(v.store_address, 120, 5)
  )
    throw failure();
  return v as ManualDraft["cvs"];
}
function customer(value: unknown, nullableEmail = false) {
  const v = object(value, ["name", "phone", "email"]);
  if (
    !text(v.name, 120, 1) ||
    typeof v.phone !== "string" ||
    !/^[0-9+() -]{6,32}$/.test(v.phone) ||
    v.phone.replace(/\D/g, "").length < 6 ||
    v.phone.replace(/\D/g, "").length > 20 ||
    !(
      (nullableEmail && v.email === null) ||
      (text(v.email, 254) &&
        (v.email === "" || /^[^\s@]+@[^\s@]+$/.test(v.email)))
    )
  )
    throw failure();
  return v as { name: string; phone: string; email: string | null };
}
/** Refuse selector drift before building a URL; no PII is a permitted query field. */
export function targetQuery(target: DrawerTarget): string {
  const keys = Object.keys(target);
  if (keys.length !== 1 || !["bundleId", "conversationId"].includes(keys[0]))
    throw failure();
  const value = target[keys[0] as keyof DrawerTarget];
  if (!id(value)) throw failure();
  return `?${keys[0] === "bundleId" ? "bundle_id" : "conversation_id"}=${value}`;
}
/** Exact single-key A15 grammar, rejecting duplicate keys and empty trailing parameters. */
export function validPrefillQuery(search: string): boolean {
  return /^\?(?:bundle_id|conversation_id)=[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(
    search,
  );
}
/** Parse only the frozen A15 shape; all inactive delivery members must be omitted. */
export function parseOrderPrefill(value: unknown): OrderPrefill {
  const v = object(value, [
    "items",
    "bundles",
    "customer",
    "last_delivery",
    "suggested_option_key",
    "live_price_eligible",
    "live_price_reason",
  ]);
  if (
    !Array.isArray(v.items) ||
    v.items.length > 50 ||
    !ids(v.bundles, 50) ||
    typeof v.live_price_eligible !== "boolean" ||
    !(
      v.suggested_option_key === null ||
      (typeof v.suggested_option_key === "string" &&
        optionKeyPattern.test(v.suggested_option_key))
    )
  )
    throw failure();
  const items = v.items.map((raw): PrefillItem => {
    const x = object(raw, [
      "sku_id",
      "offer_id",
      "keyword",
      "name",
      "variant",
      "quantity",
      "live_price_minor",
      "catalog_price_minor",
      "sellable",
      "live_quantity_remaining",
    ]);
    if (
      !id(x.sku_id) ||
      !id(x.offer_id) ||
      !text(x.keyword, 120) ||
      !text(x.name, 240) ||
      !text(x.variant, 120) ||
      !integer(x.quantity, 1, 1000) ||
      !(x.live_price_minor === null || money(x.live_price_minor)) ||
      !money(x.catalog_price_minor) ||
      typeof x.sellable !== "boolean" ||
      !integer(x.live_quantity_remaining, 0, 1000) ||
      x.live_quantity_remaining > x.quantity
    )
      throw failure();
    return x as PrefillItem;
  });
  let delivery: OrderPrefill["last_delivery"] = null;
  if (v.last_delivery !== null) {
    const raw = v.last_delivery as Record<string, unknown>;
    const x = object(raw, [
      "option_key",
      raw && Object.hasOwn(raw, "cvs") ? "cvs" : "home_address",
    ]);
    if (
      typeof x.option_key !== "string" ||
      !optionKeyPattern.test(x.option_key)
    )
      throw failure();
    delivery = Object.hasOwn(x, "cvs")
      ? { option_key: x.option_key, cvs: cvs(x.cvs) }
      : { option_key: x.option_key, home_address: home(x.home_address) };
  }
  const liveReason = reason(v.live_price_reason, livePriceReasons);
  if (v.live_price_eligible !== (liveReason === "")) throw failure();
  return {
    items,
    bundles: v.bundles,
    customer: v.customer === null ? null : customer(v.customer, true),
    last_delivery: delivery,
    suggested_option_key: v.suggested_option_key as string | null,
    live_price_eligible: v.live_price_eligible,
    live_price_reason: liveReason,
  };
}
/** Parse A16 success and replay, opting in to its contracted redacted link only. */
export function parseForBuyerResult(value: unknown): ForBuyerResult {
  const v = object(value, [
    "order_id",
    "commercial_state",
    "payment_mode",
    "total_minor",
    "currency",
    "expires_at",
    "buyer_link",
    "link_state",
    "source",
    "live_price",
    "live_price_reason",
    "send",
  ]);
  const { live_price, live_price_reason, send, ...base } = v;
  const manual = parseManualResult(base, { allowRedactedLink: true });
  if (live_price !== "applied" && live_price !== "not_applied") throw failure();
  const liveReason = reason(live_price_reason, livePriceReasons);
  let planned: ForBuyerResult["send"];
  if (
    send &&
    typeof send === "object" &&
    (send as Record<string, unknown>).state === "queued"
  ) {
    const s = object(send, ["state", "operation_id"]);
    if (!id(s.operation_id)) throw failure();
    planned = { state: "queued", operation_id: s.operation_id };
  } else {
    const s = object(send, ["state", "reason"]);
    if (s.state !== "not_sent") throw failure();
    planned = {
      state: "not_sent",
      reason: reason(s.reason, forBuyerSendReasons),
    };
  }
  return {
    ...manual,
    live_price,
    live_price_reason: liveReason,
    send: planned,
  };
}
/** Validate all A16 fields at the BFF boundary; server options/quote still decide availability. */
export function validForBuyerBody(value: unknown): value is ForBuyerBody {
  try {
    const v = object(value, [
      "items",
      "customer",
      "delivery",
      "payment_mode",
      "locale",
      "for",
      "send_payment_link",
    ]);
    if (!Array.isArray(v.items) || v.items.length < 1 || v.items.length > 50)
      return false;
    const skuIDs = v.items.map((raw) => {
      const x = object(raw, ["sku_id", "quantity"]);
      if (!id(x.sku_id) || !integer(x.quantity, 1, 1000)) throw failure();
      return x.sku_id;
    });
    if (new Set(skuIDs).size !== skuIDs.length) return false;
    customer(v.customer);
    const d = object(v.delivery, ["option_key", "home_address", "cvs"]);
    if (
      typeof d.option_key !== "string" ||
      !optionKeyPattern.test(d.option_key) ||
      (d.home_address === null) === (d.cvs === null)
    )
      return false;
    if (d.home_address !== null) home(d.home_address);
    else cvs(d.cvs);
    const f = object(v.for, ["bundle_ids", "conversation_id"]);
    return (
      ids(f.bundle_ids, 5) &&
      (f.conversation_id === null || id(f.conversation_id)) &&
      typeof v.send_payment_link === "boolean" &&
      (manualPaymentModes as readonly unknown[]).includes(v.payment_mode) &&
      ["zh-CN", "zh-TW", "en"].includes(String(v.locale))
    );
  } catch {
    return false;
  }
}
/** Map claim quantities and explicitly linked contact into a fresh transient form, without summing prices. */
export function prefillForm(
  prefill: OrderPrefill,
  locale: ManualDraft["locale"],
  currency: string,
): ManualFormValues {
  const form = emptyManualForm(locale);
  for (const item of prefill.items) {
    const prior = form.lines.find((l) => l.sku_id === item.sku_id);
    if (prior) {
      prior.quantity += item.quantity;
      continue;
    }
    form.lines.push({
      sku_id: item.sku_id,
      quantity: item.quantity,
      label: item.name,
      code: item.variant,
      price: formatMoney(locale, currency, item.catalog_price_minor),
    });
  }
  if (prefill.customer !== null) {
    Object.assign(form, {
      name: prefill.customer.name,
      phone: prefill.customer.phone,
      email: prefill.customer.email ?? "",
    });
    const d = prefill.last_delivery;
    if (d) {
      form.optionKey = prefill.suggested_option_key ?? d.option_key;
      if ("cvs" in d) form.cvs = { ...d.cvs };
      else form.home = { ...d.home_address };
    }
  }
  return form;
}
/** Build the exact A16 body once per attempt; too many bundles are refused, never silently omitted. */
export function makeForBuyerBody(
  values: ManualFormValues,
  available: ManualOption[],
  prefill: OrderPrefill,
  target: DrawerTarget,
  send: boolean,
): ForBuyerBody {
  targetQuery(target);
  const option =
    available.find((o) => o.option_key === values.optionKey) ?? null;
  const draft: ManualDraft = {
    lines: values.lines,
    name: values.name,
    phone: values.phone,
    email: values.email,
    option,
    mode: values.mode,
    home: values.home,
    cvs: values.cvs,
    locale: values.buyerLocale,
  };
  if (draftProblem(draft) !== null) throw failure();
  const body = {
    ...manualBody(draft),
    for: {
      bundle_ids: [...prefill.bundles],
      conversation_id: target.conversationId ?? null,
    },
    send_payment_link: send,
  };
  if (!validForBuyerBody(body)) throw failure();
  return body;
}
/** Closed status/code vocabulary from ManualOrders and A16; unknown replies become uncertain. */
export const forBuyerErrors: Readonly<Record<number, readonly string[]>> = {
  400: ["invalid_json", "invalid_request"],
  401: ["unauthorized"],
  403: ["forbidden"],
  404: ["not_found"],
  409: [
    "bundle_already_ordered",
    "bundle_buyer_mismatch",
    "capability",
    "insufficient_inventory",
    "conflict",
    "idempotency_conflict",
    "version_changed",
    "cod_surcharge_changed",
  ],
  415: ["json_required"],
  422: [
    "invalid_request",
    "bank_transfer_unavailable",
    "pay_at_pickup_unavailable",
    "cash_on_delivery_unavailable",
    "cvs_entry_unavailable",
    "cvs_amount_exceeds",
    "cvs_recipient_rejected",
    "cvs_environment_mismatch",
    "cvs_source_mismatch",
    "pay_at_pickup_amount_exceeds",
    "cash_on_delivery_amount_exceeds",
    "max_per_order_exceeded",
    "service_unavailable",
    "bad_store_code",
    "bad_store_name",
    "bad_store_address",
  ],
  429: [
    "rate_limited",
    "pay_at_pickup_limit",
    "cash_on_delivery_limit",
    "too_many_stores_entered",
    "throttled",
  ],
  503: ["retry_later", "manual_order_unavailable"],
};
/** Read only a recognized code and the one explicitly permitted order pointer. */
export function forBuyerError(
  status: number,
  value: unknown,
): { code: string; order_id?: string | null } | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) return null;
  const v = value as Record<string, unknown>;
  if (typeof v.code !== "string" || !forBuyerErrors[status]?.includes(v.code))
    return null;
  if (v.code === "bundle_already_ordered") {
    if (!v.details || typeof v.details !== "object" || Array.isArray(v.details))
      return null;
    const details = v.details as Record<string, unknown>;
    if (!(details.order_id === null || id(details.order_id))) return null;
    return { code: v.code, order_id: details.order_id as string | null };
  }
  return { code: v.code };
}
