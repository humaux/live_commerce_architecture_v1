import {
  buyerRequest,
  BuyerClientError,
  readBuyerSession,
  definiteError,
} from "./buyer-client.ts";

export type Item = { sku_id: string; quantity: number };
export type Cart = {
  id: string;
  currency: string;
  version: number;
  items: Item[];
};
export type Product = {
  product_id: string;
  sku_id: string;
  name: string;
  description: string;
  sku_code: string;
  currency: string;
  price_minor: number;
};
export type Option = {
  market_id: string;
  country: string;
  currency: string;
  method: string;
  delivery_kind: string;
  name_hans: string;
  name_hant: string;
  name_en: string;
  sort_order: number;
};
export const optionKey = (value: Option) =>
  `${value.market_id}:${value.country}:${value.method}`;
export async function assertPurchaseContext(context: string) {
  const current = await readBuyerSession();
  if (current.state !== "active" || current.context !== context)
    throw new BuyerClientError("context_changed");
}
export type Quote = {
  id: string;
  cart_version: number;
  currency: string;
  expires_at: string;
  lines: {
    sku_id: string;
    name: string;
    code: string;
    quantity: number;
    unit_price_minor: number;
  }[];
  amount: {
    subtotal_minor: number;
    discount_minor: number;
    shipping_minor: number;
    shipping_tax_minor: number;
    tax_minor: number;
    total_minor: number;
  };
};
type CartWrite = { expected_version: number; items: Item[] };
type QuoteWrite = {
  cart_version: number;
  market_id: string;
  country: string;
  method: string;
};
type Pending = { v: 1; context: string; key: string } & (
  { kind: "cart"; body: CartWrite } | { kind: "quote"; body: QuoteWrite }
);
const UUID = /^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/;
const CONTEXT = /^[A-Za-z0-9_-]{43}$/;
const MAX_AMOUNT = 1_000_000_000_000;
const record = (v: unknown): v is Record<string, unknown> =>
  !!v && typeof v === "object" && !Array.isArray(v);
const integer = (
  v: unknown,
  min = 0,
  max = Number.MAX_SAFE_INTEGER,
): v is number =>
  typeof v === "number" && Number.isSafeInteger(v) && v >= min && v <= max;
const id = (v: unknown): v is string => typeof v === "string" && UUID.test(v);
const currency = (v: unknown): v is string =>
  typeof v === "string" && /^[A-Z]{3}$/.test(v);
const exact = (v: Record<string, unknown>, keys: string[]) =>
  Object.keys(v).sort().join() === keys.sort().join();
export const validItems = (v: unknown): v is Item[] =>
  Array.isArray(v) &&
  v.length <= 100 &&
  v.every(
    (x) =>
      record(x) &&
      exact(x, ["sku_id", "quantity"]) &&
      id(x.sku_id) &&
      integer(x.quantity, 1, 1_000_000_000),
  ) &&
  new Set(v.map((x) => x.sku_id)).size === v.length;
export const validCart = (v: unknown): v is Cart =>
  record(v) &&
  (v.id === "" || id(v.id)) &&
  currency(v.currency) &&
  integer(v.version) &&
  validItems(v.items);
export const validProduct = (v: unknown): v is Product =>
  record(v) &&
  id(v.product_id) &&
  id(v.sku_id) &&
  [v.name, v.description, v.sku_code].every((x) => typeof x === "string") &&
  currency(v.currency) &&
  integer(v.price_minor, 0, MAX_AMOUNT);
export const validOption = (v: unknown): v is Option =>
  record(v) &&
  id(v.market_id) &&
  currency(v.currency) &&
  typeof v.country === "string" &&
  /^[A-Z]{2}$/.test(v.country) &&
  typeof v.method === "string" &&
  /^delivery:[a-z0-9_-]+$/.test(v.method) &&
  ["home", "cvs_711", "cvs_familymart"].includes(String(v.delivery_kind)) &&
  [v.name_hans, v.name_hant, v.name_en].every((x) => typeof x === "string") &&
  integer(v.sort_order, -2147483648, 2147483647);
export const validQuote = (v: unknown): v is Quote =>
  record(v) &&
  id(v.id) &&
  integer(v.cart_version, 1) &&
  currency(v.currency) &&
  typeof v.expires_at === "string" &&
  Number.isFinite(Date.parse(v.expires_at)) &&
  Array.isArray(v.lines) &&
  v.lines.length > 0 &&
  v.lines.every(
    (x) =>
      record(x) &&
      id(x.sku_id) &&
      typeof x.name === "string" &&
      typeof x.code === "string" &&
      integer(x.quantity, 1, 1_000_000_000) &&
      integer(x.unit_price_minor, 0, MAX_AMOUNT),
  ) &&
  record(v.amount) &&
  [
    "subtotal_minor",
    "discount_minor",
    "shipping_minor",
    "shipping_tax_minor",
    "tax_minor",
    "total_minor",
  ].every((k) =>
    integer(
      v.amount && (v.amount as Record<string, unknown>)[k],
      0,
      MAX_AMOUNT,
    ),
  );

export function lineSubtotal(price: number, quantity: number): number | null {
  if (
    !integer(price, 0, MAX_AMOUNT) ||
    !integer(quantity, 1, 1_000_000_000) ||
    (price > 0 && quantity > Math.floor(MAX_AMOUNT / price))
  )
    return null;
  return price * quantity;
}

// SetCart replaces the complete cart. Preserve all unrelated SKUs, use the
// version actually shown to the buyer, and let the server reject a stale CAS.
export function cartSelection(
  cart: Cart,
  sku: string,
  quantity: number,
): CartWrite {
  if (!validCart(cart) || !id(sku) || !integer(quantity, 1, 1_000_000_000))
    throw new BuyerClientError("invalid_response");
  const items = [
    ...cart.items.filter((x) => x.sku_id !== sku),
    { sku_id: sku, quantity },
  ].sort((a, b) => a.sku_id.localeCompare(b.sku_id));
  if (!validItems(items)) throw new BuyerClientError("request_failed");
  return { expected_version: cart.version, items };
}

export async function readPurchase<T>(
  suffix: string,
  context: string,
  valid: (value: unknown) => value is T,
): Promise<T> {
  const response = await buyerRequest("GET", suffix, context);
  if (!response.ok)
    throw new BuyerClientError("request_failed", response.status);
  const value: unknown = await response.json();
  if (!valid(value)) throw new BuyerClientError("invalid_response");
  await assertPurchaseContext(context);
  return value;
}

export async function purchasePage<T>(
  suffix: string,
  context: string,
  valid: (value: unknown) => value is T,
): Promise<{ items: T[]; next_cursor: string }> {
  return readPurchase(
    suffix,
    context,
    (v): v is { items: T[]; next_cursor: string } =>
      record(v) &&
      Array.isArray(v.items) &&
      v.items.length <= 100 &&
      v.items.every(valid) &&
      typeof v.next_cursor === "string" &&
      v.next_cursor.length <= 2048,
  );
}

function storageKey(context: string) {
  if (!CONTEXT.test(context)) throw new BuyerClientError("context_changed");
  return `commerce-purchase-pending-v1:${context}`;
}

export function parsePending(raw: string, context: string): Pending {
  try {
    if (raw.length > 24000) throw new Error();
    const v: unknown = JSON.parse(raw);
    if (
      !record(v) ||
      !exact(v, ["v", "context", "key", "kind", "body"]) ||
      v.v !== 1 ||
      v.context !== context ||
      !id(v.key) ||
      !record(v.body)
    )
      throw new Error();
    const b = v.body;
    if (
      v.kind === "cart" &&
      exact(b, ["expected_version", "items"]) &&
      integer(b.expected_version) &&
      validItems(b.items)
    )
      return v as Pending;
    if (
      v.kind === "quote" &&
      exact(b, ["cart_version", "market_id", "country", "method"]) &&
      integer(b.cart_version, 1) &&
      id(b.market_id) &&
      typeof b.country === "string" &&
      /^[A-Z]{2}$/.test(b.country) &&
      typeof b.method === "string" &&
      /^delivery:[a-z0-9_-]+$/.test(b.method)
    )
      return v as Pending;
    throw new Error();
  } catch {
    throw new BuyerClientError("uncertain");
  }
}

export function pendingPurchase(context: string): Pending | null {
  try {
    const raw = localStorage.getItem(storageKey(context));
    return raw === null ? null : parsePending(raw, context);
  } catch (error) {
    if (error instanceof BuyerClientError) throw error;
    throw new BuyerClientError("unavailable");
  }
}

// Only non-PII cart/quote requests belong here. Destination bodies and bearer
// tokens must never be added to this journal. Both tabs and retry use one key.
export async function writePurchase(
  context: string,
  input?:
    { kind: "cart"; body: CartWrite } | { kind: "quote"; body: QuoteWrite },
): Promise<{ kind: "cart"; value: Cart } | { kind: "quote"; value: Quote }> {
  if (!navigator.locks) throw new BuyerClientError("unavailable");
  return navigator.locks.request("commerce-purchase-write-v1", async () => {
    let pending = pendingPurchase(context);
    if (
      pending &&
      input &&
      (pending.kind !== input.kind ||
        JSON.stringify(pending.body) !== JSON.stringify(input.body))
    )
      throw new BuyerClientError("uncertain");
    if (!pending) {
      if (!input) throw new BuyerClientError("request_failed");
      const raw = JSON.stringify({
        ...input,
        v: 1,
        context,
        key: crypto.randomUUID(),
      });
      pending = parsePending(raw, context);
      try {
        localStorage.setItem(storageKey(context), raw);
        if (localStorage.getItem(storageKey(context)) !== raw)
          throw new Error();
      } catch {
        throw new BuyerClientError("unavailable");
      }
    }
    const response = await buyerRequest(
      pending.kind === "cart" ? "PUT" : "POST",
      pending.kind === "cart" ? "cart" : "quotes",
      context,
      pending.body,
      pending.key,
    );
    const conclusive =
      !response.ok &&
      response.status < 500 &&
      (await definiteError(response.clone()));
    let value: unknown;
    try {
      value = await response.json();
    } catch {
      throw new BuyerClientError("uncertain");
    }
    if (!response.ok) {
      // A parsed non-retryable API denial is conclusive; HTML/5xx/network loss
      // keeps the journal and blocks a new mutation until explicit recovery.
      if (
        conclusive &&
        record(value) &&
        value.retryable === false &&
        typeof value.code === "string"
      ) {
        localStorage.removeItem(storageKey(context));
        throw new BuyerClientError("request_failed", response.status);
      }
      throw new BuyerClientError("uncertain", response.status);
    }
    if (
      (pending.kind === "cart" && !validCart(value)) ||
      (pending.kind === "quote" && !validQuote(value))
    )
      throw new BuyerClientError("uncertain");
    await assertPurchaseContext(context);
    // A replay receipt is historical. Read the current cart, not its old version,
    // before allowing a follow-up quote or another CAS mutation.
    if (pending.kind === "cart")
      value = await readPurchase("cart", context, validCart);
    // Persist only a result ID before clearing the request. Reload can then GET
    // the quote, including its expired state, without silently creating another.
    if (pending.kind === "quote")
      sessionStorage.setItem(
        `commerce-purchase-quote-v1:${context}`,
        (value as Quote).id,
      );
    localStorage.removeItem(storageKey(context));
    return pending.kind === "cart"
      ? { kind: "cart", value: value as Cart }
      : { kind: "quote", value: value as Quote };
  });
}
