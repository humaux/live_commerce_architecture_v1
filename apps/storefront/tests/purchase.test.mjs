import test from "node:test";
import assert from "node:assert/strict";
import { buyerDemoLabel } from "../lib/demo-label.ts";
import {
  cartSelection,
  lineSubtotal,
  parsePending,
  validCart,
  writePurchase,
  pendingPurchase,
  optionKey,
} from "../lib/purchase.ts";

const sku = "00000000-0000-0000-0000-000000000001";
const other = "00000000-0000-0000-0000-000000000002";
const context = "a".repeat(43);
const key = "00000000-0000-0000-0000-000000000003";
const cart = {
  id: key,
  currency: "TWD",
  version: 8,
  items: [
    { sku_id: other, quantity: 4 },
    { sku_id: sku, quantity: 1 },
  ],
};
test("demonstration disclosure is explicit and local-only", () => {
  assert.equal(
    buyerDemoLabel({
      COMMERCE_BUYER_DEMO_LABEL: "1",
      COMMERCE_BUYER_API_ORIGIN: "http://127.0.0.1:4321",
    }),
    true,
  );
  for (const env of [
    {},
    { COMMERCE_BUYER_API_ORIGIN: "http://127.0.0.1:4321" },
    {
      COMMERCE_BUYER_DEMO_LABEL: "1",
      COMMERCE_BUYER_API_ORIGIN: "https://api.example.com",
    },
  ])
    assert.equal(buyerDemoLabel(env), false);
});
test("cart selection preserves unrelated SKUs and the observed CAS version", () => {
  const next = cartSelection(cart, sku, 3);
  assert.equal(next.expected_version, 8);
  assert.deepEqual(next.items, [
    { sku_id: sku, quantity: 3 },
    { sku_id: other, quantity: 4 },
  ]);
  assert.equal(cart.items[1].quantity, 1);
  assert.equal(
    validCart({ ...cart, items: [...cart.items, cart.items[0]] }),
    false,
  );
  assert.throws(() => cartSelection(cart, sku, 0));
});
test("minor amounts are bounded before multiplication, including zero", () => {
  assert.equal(lineSubtotal(39000, 2), 78000);
  assert.equal(lineSubtotal(0, 1_000_000_000), 0);
  for (const [p, q] of [
    [1e12, 2],
    [1, 1.5],
    [-1, 1],
    [NaN, 1],
    [1, Infinity],
  ])
    assert.equal(lineSubtotal(p, q), null);
});
test("delivery identity includes country for one market and method", () => {
  assert.notEqual(
    optionKey({ market_id: key, country: "TW", method: "delivery:home" }),
    optionKey({ market_id: key, country: "HK", method: "delivery:home" }),
  );
});
test("journal is bound to context and rejects PII/unknown fields", () => {
  const value = {
    v: 1,
    kind: "cart",
    context,
    key,
    body: cartSelection(cart, sku, 3),
  };
  assert.deepEqual(parsePending(JSON.stringify(value), context), value);
  for (const bad of [
    { ...value, context: "b".repeat(43) },
    { ...value, body: { ...value.body, phone: "123456" } },
    { ...value, kind: "destination" },
    { ...value, token: "secret" },
    { ...value, key: "bad" },
  ])
    assert.throws(() => parsePending(JSON.stringify(bad), context));
});
test("lost response replays exact key/body, then clears only after parsed receipt", async () => {
  const oldFetch = globalThis.fetch;
  const makeStorage = () => {
    const data = new Map();
    return {
      getItem: (k) => data.get(k) ?? null,
      setItem: (k, v) => data.set(k, v),
      removeItem: (k) => data.delete(k),
    };
  };
  globalThis.localStorage = makeStorage();
  globalThis.sessionStorage = makeStorage();
  globalThis.window = { localStorage };
  const oldLocks = Object.getOwnPropertyDescriptor(navigator, "locks");
  Object.defineProperty(navigator, "locks", {
    configurable: true,
    value: { request: (...args) => args.at(-1)() },
  });
  const calls = [];
  globalThis.fetch = async (_url, init) => {
    if (_url.endsWith("/session"))
      return Response.json({
        state: "active",
        context,
        expires_at: new Date(Date.now() + 100000).toISOString(),
      });
    if (init.method === "GET") return Response.json({ ...cart, version: 10 });
    calls.push({
      body: init.body,
      key: new Headers(init.headers).get("Idempotency-Key"),
    });
    if (calls.length === 1) throw new Error("synthetic connection loss");
    return Response.json({ ...cart, version: 9 });
  };
  try {
    const input = { kind: "cart", body: cartSelection(cart, sku, 3) };
    await assert.rejects(writePurchase(context, input));
    assert.equal(pendingPurchase(context).kind, "cart");
    await assert.rejects(
      writePurchase(context, {
        kind: "cart",
        body: cartSelection(cart, sku, 7),
      }),
    );
    assert.equal(calls.length, 1);
    const result = await writePurchase(context);
    assert.equal(result.kind, "cart");
    assert.equal(result.value.version, 10);
    assert.deepEqual(calls[1], calls[0]);
    assert.equal(pendingPurchase(context), null);
  } finally {
    globalThis.fetch = oldFetch;
    if (oldLocks) Object.defineProperty(navigator, "locks", oldLocks);
    else delete navigator.locks;
    delete globalThis.window;
    delete globalThis.localStorage;
    delete globalThis.sessionStorage;
  }
});

// ---- Buyer shipment (manual-fulfilment-v1 §5.2/§3.2): mirrors internal/checkout.Get's drift rule.
import { validOrder, validOrderSummary, validTrackingURL } from "../lib/purchase.ts";
const shipOrder = (extra = {}) => ({
  order_id: key, cart_id: other, cart_version: 1, commercial_state: "CONFIRMED", fulfillment_state: "MERCHANT_SHIPPED",
  snapshot: {
    quote: { currency: "TWD", lines: [{ sku_id: sku, name: "Item", code: "S", quantity: 1, unit_price_minor: 2500 }],
      amount: { subtotal_minor: 2500, discount_minor: 0, shipping_minor: 0, shipping_tax_minor: 0, tax_minor: 0, total_minor: 2500 } },
    destination: { kind: "home", country: "TW", recipient_name: "R", phone: "1", home_address: { region: "", city: "C", postal_code: "", line1: "L", line2: "" } },
    service: { code: "home", name_hans: "a", name_hant: "b", name_en: "c", delivery_kind: "home", mode: "MANUAL" },
  },
  shipment: { status: "SHIPPED", carrier_code: "seven_eleven_cvs", carrier_name: null, tracking_number: "0012345678", tracking_url: "https://t.cat.com.tw/q?no=1", recorded_at: "2026-09-29T01:02:03Z" },
  ...extra,
});
const ship = (patch) => ({ ...shipOrder().shipment, ...patch });

test("RUI02 buyer order admits MERCHANT_SHIPPED only with a SHIPPED head; absent shipment ≡ null", () => {
  assert.equal(validOrder(shipOrder()), true);
  assert.equal(validOrder(shipOrder({ shipment: ship({ carrier_code: "other", carrier_name: "Local Courier", tracking_url: null }) })), true);
  assert.equal(validOrder(shipOrder({ fulfillment_state: "MANUAL_UNASSIGNED", shipment: null })), true);
  const { shipment, ...absent } = shipOrder({ fulfillment_state: "MANUAL_UNASSIGNED" });
  assert.equal(validOrder(absent), true);
  assert.equal(validOrder(shipOrder({ shipment: null })), false, "MERCHANT_SHIPPED without head");
  assert.equal(validOrder(shipOrder({ fulfillment_state: "MANUAL_UNASSIGNED" })), false, "head without MERCHANT_SHIPPED");
  assert.equal(validOrder({ ...absent, fulfillment_state: "MERCHANT_SHIPPED" }), false);
});

test("RUI02 shipment shape negatives (incl. unsafe links)", () => {
  for (const [name, patch] of [
    ["status VOIDED", { status: "VOIDED" }],
    ["unknown carrier", { carrier_code: "dhl" }],
    ["other without name", { carrier_code: "other", carrier_name: null }],
    ["control char in name", { carrier_name: "A\nB" }],
    ["name too long", { carrier_name: "x".repeat(81) }],
    ["empty tracking", { tracking_number: "" }],
    ["tracking with symbol", { tracking_number: "12;34" }],
    ["tracking trailing space", { tracking_number: "1234 " }],
    ["tracking 65 chars", { tracking_number: "1".repeat(65) }],
    ["javascript url", { tracking_url: "javascript:alert(1)" }],
    ["http url", { tracking_url: "http://t.cat.com.tw/q" }],
    ["userinfo", { tracking_url: "https://u:p@t.cat.com.tw/q" }],
    ["port", { tracking_url: "https://t.cat.com.tw:8443/q" }],
    ["fragment", { tracking_url: "https://t.cat.com.tw/q#x" }],
    ["dotless host", { tracking_url: "https://localhost/q" }],
    ["whitespace", { tracking_url: "https://t.cat.com.tw/a b" }],
    ["url over 512 bytes", { tracking_url: "https://t.cat.com.tw/" + "a".repeat(500) }],
    ["bad recorded_at", { recorded_at: "yesterday" }],
    ["extra merchant field (note)", { note: "internal" }],
    ["missing recorded_at", { recorded_at: undefined }],
  ]) {
    const s = ship(patch);
    if (patch.recorded_at === undefined && "recorded_at" in patch) delete s.recorded_at;
    assert.equal(validOrder(shipOrder({ shipment: s })), false, name);
  }
  assert.equal(validTrackingURL("https://t.cat.com.tw/q?no=1"), true);
});

test("RUI02 history summary admits MERCHANT_SHIPPED (a shipped order must not break the list)", () => {
  const summary = { order_id: key, cart_id: other, cart_version: 1, commercial_state: "CONFIRMED", fulfillment_state: "MERCHANT_SHIPPED", created_at: "2026-09-29T01:02:03Z", currency: "TWD", total_minor: 2500 };
  assert.equal(validOrderSummary(summary), true);
  assert.equal(validOrderSummary({ ...summary, fulfillment_state: "DELIVERED" }), false);
});
