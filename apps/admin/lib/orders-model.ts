export const orderStates = [
  "all",
  "DRAFT",
  "AWAITING_PAYMENT",
  "CONFIRMED",
  "CANCELLED",
] as const;
export type OrderFilter = (typeof orderStates)[number];
export type CommercialState = Exclude<OrderFilter, "all">;
export type FulfillmentState =
  | "MANUAL_UNASSIGNED"
  | "CANCELLED"
  | "PAID_ALLOCATION_FAILED";
export type PaymentState =
  | "NOT_STARTED"
  | "PENDING"
  | "AUTHORIZED"
  | "CAPTURED"
  | "REVIEW_REQUIRED";
export type WorkState = "NONE" | "READY" | "REVIEW_REQUIRED";

export type OrderSummary = {
  order_id: string;
  created_at: string;
  updated_at: string;
  currency: string;
  total_minor: number;
  commercial_state: CommercialState;
  fulfillment_state: FulfillmentState;
  payment_state: PaymentState;
  test_mode: boolean;
  work_state: WorkState;
};
export type OrderList = { items: OrderSummary[]; next_cursor: string };
export type LineAmount = {
  subtotal_minor: number;
  discount_minor: number;
  tax_minor: number;
  total_minor: number;
};
export type OrderItem = {
  sku_id: string;
  code: string;
  name: string;
  quantity: number;
  unit_price_minor: number;
  amount: LineAmount;
};
export type OrderTotals = {
  subtotal_minor: number;
  discount_minor: number;
  shipping_minor: number;
  shipping_tax_minor: number;
  tax_minor: number;
  total_minor: number;
};
export type HomeAddress = {
  region: string;
  city: string;
  postal_code: string;
  line1: string;
  line2: string;
};
export type Pickup = {
  kind: "cvs_711" | "cvs_familymart";
  namespace: string;
  code: string;
  name: string;
  address: string;
  verification_kind: "MANUAL_ATTESTED";
};
export type Destination = {
  kind: "home" | "cvs_711" | "cvs_familymart";
  country: string;
  recipient_name: string;
  phone: string;
  home_address: HomeAddress;
  pickup: Pickup | null;
};
export type OrderDetail = OrderSummary & {
  country: string;
  service_code: string;
  items: OrderItem[];
  totals: OrderTotals;
  destination: Destination;
};

export const canonicalUUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
export const canonicalCursor = /^[A-Za-z0-9_-]{1,1024}$/;
const timestamp = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{6}Z$/;
const maxMoney = 1_000_000_000_000;
const summaryKeys = [
  "order_id", "created_at", "updated_at", "currency", "total_minor",
  "commercial_state", "fulfillment_state", "payment_state", "test_mode", "work_state",
];

function object(value: unknown, keys: string[]): Record<string, unknown> {
  if (!value || typeof value !== "object" || Array.isArray(value)) throw new Error("unavailable");
  const result = value as Record<string, unknown>;
  if (Object.keys(result).sort().join(",") !== [...keys].sort().join(",")) throw new Error("unavailable");
  return result;
}
function oneOf<T extends string>(value: unknown, values: readonly T[]): value is T {
  return typeof value === "string" && values.includes(value as T);
}
function pattern(value: unknown, regex: RegExp): value is string {
  return typeof value === "string" && regex.test(value);
}
function money(value: unknown): value is number {
  return Number.isSafeInteger(value) && (value as number) >= 0 && (value as number) <= maxMoney;
}
function text(value: unknown, max: number, required: boolean): value is string {
  return typeof value === "string" && (!required || value.length > 0) &&
    Array.from(value).length <= max && !/[\p{C}\p{Zl}\p{Zp}]/u.test(value) &&
    !/[\ud800-\udbff](?![\udc00-\udfff])|(?<![\ud800-\udbff])[\udc00-\udfff]/.test(value);
}
function date(value: unknown): value is string {
  if (typeof value !== "string" || !timestamp.test(value)) return false;
  const parsed = new Date(value);
  return Number.isFinite(parsed.getTime()) && parsed.toISOString().slice(0, 19) === value.slice(0, 19);
}
function add(a: number, b: number) {
  if (!money(a) || !money(b) || a > maxMoney - b) throw new Error("unavailable");
  return a + b;
}
function phone(value: unknown): value is string {
  if (typeof value !== "string" || value.length < 6 || value.length > 32 || value.trim() !== value ||
    !/^[0-9+() -]+$/.test(value)) return false;
  const digits = value.replace(/\D/g, "").length;
  return digits >= 6 && digits <= 20;
}

export function parseOrderSummary(value: unknown): OrderSummary {
  const v = object(value, summaryKeys);
  if (!pattern(v.order_id, canonicalUUID) || !date(v.created_at) || !date(v.updated_at) ||
    v.updated_at < v.created_at || !pattern(v.currency, /^[A-Z]{3}$/) || !money(v.total_minor) ||
    !oneOf(v.commercial_state, orderStates.slice(1)) ||
    !oneOf(v.fulfillment_state, ["MANUAL_UNASSIGNED", "CANCELLED", "PAID_ALLOCATION_FAILED"]) ||
    !oneOf(v.payment_state, ["NOT_STARTED", "PENDING", "AUTHORIZED", "CAPTURED", "REVIEW_REQUIRED"]) ||
    !oneOf(v.work_state, ["NONE", "READY", "REVIEW_REQUIRED"]) || typeof v.test_mode !== "boolean")
    throw new Error("unavailable");
  const row = v as OrderSummary;
  if ((row.payment_state === "NOT_STARTED" && (row.test_mode || row.work_state !== "NONE")) ||
    (row.fulfillment_state === "CANCELLED" && row.commercial_state !== "CANCELLED") ||
    (row.fulfillment_state === "PAID_ALLOCATION_FAILED" &&
      (row.payment_state !== "REVIEW_REQUIRED" || row.work_state !== "REVIEW_REQUIRED")) ||
    (row.work_state === "READY" &&
      (row.payment_state !== "CAPTURED" || row.commercial_state !== "CONFIRMED" || row.fulfillment_state !== "MANUAL_UNASSIGNED")) ||
    (row.work_state === "REVIEW_REQUIRED" && row.payment_state !== "REVIEW_REQUIRED") ||
    (row.commercial_state === "CONFIRMED" &&
      (row.work_state === "NONE" || !["CAPTURED", "REVIEW_REQUIRED"].includes(row.payment_state))))
    throw new Error("unavailable");
  return row;
}

export function parseOrderList(value: unknown): OrderList {
  const v = object(value, ["items", "next_cursor"]);
  if (!Array.isArray(v.items) || v.items.length > 10 || typeof v.next_cursor !== "string" ||
    (v.next_cursor !== "" && !canonicalCursor.test(v.next_cursor))) throw new Error("unavailable");
  const items = v.items.map(parseOrderSummary);
  if (new Set(items.map((item) => item.order_id)).size !== items.length) throw new Error("unavailable");
  return { items, next_cursor: v.next_cursor };
}

export function parseOrderDetail(value: unknown, requestedID: string): OrderDetail {
  const v = object(value, [...summaryKeys, "country", "service_code", "items", "totals", "destination"]);
  const summary = parseOrderSummary(Object.fromEntries(summaryKeys.map((key) => [key, v[key]])));
  if (summary.order_id !== requestedID || !pattern(v.country, /^[A-Z]{2}$/) ||
    !pattern(v.service_code, /^[a-z][a-z0-9_-]{0,39}$/) ||
    !Array.isArray(v.items) || v.items.length < 1 || v.items.length > 50) throw new Error("unavailable");
  const t = object(v.totals, ["subtotal_minor", "discount_minor", "shipping_minor", "shipping_tax_minor", "tax_minor", "total_minor"]);
  if (![t.subtotal_minor, t.discount_minor, t.shipping_minor, t.shipping_tax_minor, t.tax_minor, t.total_minor].every(money) ||
    t.total_minor !== summary.total_minor) throw new Error("unavailable");
  const d = object(v.destination, ["kind", "country", "recipient_name", "phone", "home_address", "pickup"]);
  const h = object(d.home_address, ["region", "city", "postal_code", "line1", "line2"]);
  if (!oneOf(d.kind, ["home", "cvs_711", "cvs_familymart"]) || d.country !== v.country ||
    !text(d.recipient_name, 120, true) || !phone(d.phone) ||
    !text(h.region, 100, false) || !text(h.city, 100, false) || !text(h.postal_code, 20, false) ||
    !text(h.line1, 200, false) || !text(h.line2, 200, false)) throw new Error("unavailable");
  let pickup: Pickup | null = null;
  if (d.kind === "home") {
    if (d.pickup !== null || h.city === "" || h.line1 === "") throw new Error("unavailable");
  } else {
    const p = object(d.pickup, ["kind", "namespace", "code", "name", "address", "verification_kind"]);
    if (d.country !== "TW" || p.kind !== d.kind || Object.values(h).some((field) => field !== "") ||
      !pattern(p.namespace, /^[a-z][a-z0-9_.:-]{0,63}$/) ||
      !pattern(p.code, /^[A-Za-z0-9_-]{1,32}$/) || !text(p.name, 120, true) ||
      !text(p.address, 400, true) || p.verification_kind !== "MANUAL_ATTESTED") throw new Error("unavailable");
    pickup = p as Pickup;
  }
  let subtotal = 0, discount = 0, tax = 0;
  const items = v.items.map((raw): OrderItem => {
    const item = object(raw, ["sku_id", "code", "name", "quantity", "unit_price_minor", "amount"]);
    const amount = object(item.amount, ["subtotal_minor", "discount_minor", "tax_minor", "total_minor"]);
    if (!pattern(item.sku_id, canonicalUUID) || !pattern(item.code, /^[A-Za-z0-9_.-]{1,64}$/) ||
      !text(item.name, 120, true) || !Number.isSafeInteger(item.quantity) ||
      (item.quantity as number) < 1 || (item.quantity as number) > 1_000_000_000 ||
      !money(item.unit_price_minor) || !Object.values(amount).every(money) ||
      (item.unit_price_minor as number) > maxMoney / (item.quantity as number) ||
      amount.subtotal_minor !== (item.unit_price_minor as number) * (item.quantity as number) ||
      (amount.discount_minor as number) > (amount.subtotal_minor as number)) throw new Error("unavailable");
    subtotal = add(subtotal, amount.subtotal_minor as number);
    discount = add(discount, amount.discount_minor as number);
    tax = add(tax, amount.tax_minor as number);
    return { ...item, amount } as OrderItem;
  });
  tax = add(tax, t.shipping_tax_minor as number);
  if (subtotal !== t.subtotal_minor || discount !== t.discount_minor || tax !== t.tax_minor) throw new Error("unavailable");
  const base = add(subtotal - discount, t.shipping_minor as number);
  const inclusive = items.every((item) => item.amount.total_minor === item.amount.subtotal_minor - item.amount.discount_minor);
  const exclusive = items.every((item) => item.amount.total_minor === item.amount.subtotal_minor - item.amount.discount_minor + item.amount.tax_minor);
  if (!((inclusive && summary.total_minor === base) ||
    (exclusive && tax <= maxMoney - base && summary.total_minor === base + tax))) throw new Error("unavailable");
  return { ...summary, country: v.country as string, service_code: v.service_code as string,
    items, totals: t as OrderTotals, destination: { ...d, home_address: h, pickup } as Destination };
}
