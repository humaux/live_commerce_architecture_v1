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
