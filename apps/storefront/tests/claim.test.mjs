// Purpose: claim wire validation and purchase-lock/token-leak regression tests.
// Depends on: buyer BFF, claim-contract and purchase helpers with synthetic transport.
// Used by: scripts/dev/test-node.sh and CDC03.
// Claim-link BFF and contract (live-keyword-claims-v1 §7.2, §11.1): the token travels only
// as X-Commerce-Claim-Token on B1/B2, is never echoed, and only the frozen projections
// are forwarded. Private Go transport is stubbed; real chains are KC14/KC16.
import test from "node:test";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { runInNewContext } from "node:vm";
import * as claimContract from "../lib/claim-contract.ts";
import { claimCopy } from "../lib/claim-copy.ts";
import assert from "node:assert/strict";
import { handleBuyerRequest } from "../lib/buyer-server.ts";
import { cartSelection, cartWithQuantity } from "../lib/purchase.ts";
import {
  claimFragment,
  validClaimPreview,
  validClaimRedeemed,
} from "../lib/claim-contract.ts";

const bff = Buffer.alloc(32, 1).toString("base64url");
const signing = Buffer.alloc(32, 2).toString("base64url");
const origin = "https://shop.example";
const api = "http://127.0.0.1:3219";
const token = Buffer.alloc(32, 7).toString("base64url");
const sku = "11111111-1111-4111-8111-111111111111";
const preview = {
  bundle_version: 2, bound: false, expires_at: "2026-10-01T00:00:00Z",
  lines: [{ keyword: "A1", sku_id: sku, sku_code: "A-RED", product_name: "Red", currency: "TWD",
    unit_price_minor: 1200, quantity: 2, pending: true, available: true, sold_out: false }],
};
const redeemed = {
  bundle_version: 2,
  cart: { id: "22222222-2222-4222-8222-222222222222", currency: "TWD", version: 3, items: [{ sku_id: sku, quantity: 2 }] },
  applied: [{ sku_id: sku, quantity: 2 }], skipped: [],
};

function enabled() {
  process.env.COMMERCE_BUYER_WEB_ENABLED = "1";
  process.env.COMMERCE_BUYER_API_ORIGIN = api;
  process.env.COMMERCE_BUYER_BFF_KEY = bff;
  process.env.COMMERCE_BUYER_COOKIE_KEY = signing;
  process.env.COMMERCE_BUYER_SESSION_TTL = "3600";
}

function req(method, suffix, { cookie, context, body, extra = {} } = {}) {
  const headers = { Host: "shop.example", ...extra };
  if (method !== "GET") headers.Origin = origin;
  if (cookie) headers.Cookie = cookie;
  if (context) headers["X-Buyer-Context"] = context;
  if (body !== undefined) headers["Content-Type"] = "application/json";
  return new Request(`${origin}/api/buyer/${suffix}`, { method, headers, body });
}

async function session() {
  const response = await handleBuyerRequest(req("POST", "session/prepare", { body: "{}" }));
  assert.equal(response.status, 200);
  return { cookie: response.headers.get("set-cookie").split(";", 1)[0], context: (await response.json()).context };
}

test("fragment grammar accepts only one canonical token", () => {
  assert.equal(claimFragment(`#t=${token}`), token);
  for (const hash of ["", "#", `#t=${token}&x=1`, `#x=${token}`, `#t=${token.slice(0, 42)}B`, `#t=${token}=`, `#T=${token}`])
    assert.equal(claimFragment(hash), null, hash);
});

test("claim projections are closed", () => {
  assert.equal(validClaimPreview(preview), true);
  assert.equal(validClaimPreview({ ...preview, label: "amy" }), false);
  assert.equal(validClaimPreview({ ...preview, lines: [] }), false);
  assert.equal(validClaimPreview({ ...preview, lines: [{ ...preview.lines[0], actor_key: "x" }] }), false);
  assert.equal(validClaimRedeemed(redeemed), true);
  assert.equal(validClaimRedeemed({ ...redeemed, skipped: [{ sku_id: sku, reason: "gone" }] }), false);
  assert.equal(validClaimRedeemed({ ...redeemed, cart: { ...redeemed.cart, id: "" , version: 0, items: [] } }), true);
});

test("claim token rides only its header on B1/B2 and is never echoed", async () => {
  enabled();
  const old = globalThis.fetch;
  const calls = [];
  globalThis.fetch = async (url, options) => {
    calls.push({ url: String(url), options });
    if (String(url).endsWith("/session/bootstrap"))
      return Response.json({ authenticated: true, expires_at: new Date(Date.now() + 3500_000).toISOString() });
    if (String(url).endsWith("/claim-link")) return Response.json(preview);
    if (String(url).endsWith("/claim-link/redeem")) return Response.json(redeemed);
    return Response.json({ id: "", currency: "TWD", version: 0, items: [] });
  };
  try {
    const { cookie, context } = await session();
    const activated = await handleBuyerRequest(req("POST", "session/activate", { cookie, context, body: "{}" }));
    assert.equal(activated.status, 200);
    const claim = { "X-Commerce-Claim-Token": token };

    let response = await handleBuyerRequest(req("GET", "claim-link", { cookie, context, extra: claim }));
    assert.equal(response.status, 200);
    assert.deepEqual(await response.json(), preview);
    let call = calls.at(-1);
    assert.equal(call.url, `${api}/v1/buyer/claim-link`);
    assert.equal(call.options.headers.get("X-Commerce-Claim-Token"), token);
    assert.equal(call.options.headers.get("Idempotency-Key"), null);

    response = await handleBuyerRequest(req("POST", "claim-link/redeem", {
      cookie, context, body: '{"expected_bundle_version":2}', extra: { ...claim, "Idempotency-Key": "redeem-key-1" } }));
    assert.equal(response.status, 200);
    assert.deepEqual(await response.json(), redeemed);
    call = calls.at(-1);
    assert.equal(call.url, `${api}/v1/buyer/claim-link/redeem`);
    assert.equal(call.options.headers.get("X-Commerce-Claim-Token"), token);
    assert.equal(call.options.headers.get("Idempotency-Key"), "redeem-key-1");

    const before = calls.length;
    for (const [method, suffix, extra, body, status] of [
      ["GET", "claim-link", {}, undefined, 422],
      ["GET", "claim-link", { "X-Commerce-Claim-Token": token.slice(0, 42) + "B" }, undefined, 422],
      ["GET", "claim-link", { ...claim, "Idempotency-Key": "read-key-1" }, undefined, 422],
      ["POST", "claim-link/redeem", claim, '{"expected_bundle_version":2}', 422],
      ["POST", "claim-link/redeem", { ...claim, "Idempotency-Key": "redeem-key-2" }, '{"expected_bundle_version":2,"token":"x"}', 400],
      ["GET", "cart", claim, undefined, 403],
      ["PUT", "cart", { ...claim, "Idempotency-Key": "cart-key-01" }, '{"items":[]}', 403],
      ["GET", "claim-link/other", claim, undefined, 404],
      ["DELETE", "claim-link", claim, undefined, 405],
    ]) {
      response = await handleBuyerRequest(req(method, suffix, { cookie, context, body, extra }));
      assert.equal(response.status, status, `${method} ${suffix}`);
      assert.equal((await response.text()).includes(token), false);
    }
    response = await handleBuyerRequest(new Request(`${origin}/api/buyer/claim-link?t=${token}`, {
      method: "GET", headers: { Host: "shop.example", Cookie: cookie, "X-Buyer-Context": context, ...claim } }));
    assert.equal(response.status, 422);
    assert.equal(calls.length, before, "rejected claim requests must not reach Go");

    globalThis.fetch = async () => Response.json({ ...preview, label: "amy" });
    response = await handleBuyerRequest(req("GET", "claim-link", { cookie, context, extra: claim }));
    assert.equal(response.status, 503, "an unexpected projection field is never forwarded");
    globalThis.fetch = async () => Response.json({ code: "not_found" }, { status: 404 });
    response = await handleBuyerRequest(req("GET", "claim-link", { cookie, context, extra: claim }));
    assert.equal(response.status, 404);
    assert.equal((await response.json()).code, "not_found");
  } finally {
    globalThis.fetch = old;
  }
});

// CDC03: mandatory inventory hint and a closed reason enum fail on the pre-amendment parser.
test("claim-priced cart edits send only SKU and quantity, keeping server price authority", () => {
  const other = "33333333-3333-4333-8333-333333333333";
  const cart = { ...redeemed.cart, items: [{ sku_id: sku, quantity: 2, live_unit_price_minor: 20000 }, { sku_id: other, quantity: 1 }] };
  const expected = [{ sku_id: sku, quantity: 2 }];
  assert.deepEqual(cartWithQuantity(cart, other, 0), { expected_version: cart.version, items: expected });
  assert.deepEqual(cartSelection(cart, other, 3).items, [...expected, { sku_id: other, quantity: 3 }]);
  assert.equal(cart.items[0].live_unit_price_minor, 20000, "read snapshot is not mutated");
});

test("direct-checkout sold-out projection is mandatory, boolean and closed", () => {
  assert.equal(validClaimPreview(preview), true);
  const { sold_out, ...legacy } = preview.lines[0];
  assert.equal(validClaimPreview({ ...preview, lines: [legacy] }), false);
  for (const value of [null, 1, "false"])
    assert.equal(validClaimPreview({ ...preview, lines: [{ ...legacy, sold_out: value }] }), false);
  assert.equal(validClaimRedeemed({ ...redeemed, skipped: [{ sku_id: sku, reason: "sold_out" }] }), true);
});

// CDC03: behavior test, including two serialized clicks and a lost reply, with no token journal.
test("claim redeem shares the purchase lock, guards recovery and never persists the token", async () => {
  const purchase = await import("../lib/purchase.ts");
  assert.equal(typeof purchase.redeemClaimLink, "function", "purchase helper must own B2");
  const oldFetch = globalThis.fetch;
  const oldLocks = Object.getOwnPropertyDescriptor(navigator, "locks");
  const oldWindow = globalThis.window, oldLocal = globalThis.localStorage, oldSession = globalThis.sessionStorage;
  const values = new Map(), writes = [], calls = [];
  const storage = {
    getItem: (key) => values.get(key) ?? null,
    setItem: (key, value) => { writes.push([key, value]); values.set(key, value); },
    removeItem: (key) => values.delete(key),
  };
  globalThis.localStorage = storage; globalThis.sessionStorage = storage;
  globalThis.window = { localStorage: storage };
  const ctx = "a".repeat(43);
  let locked = false;
  Object.defineProperty(navigator, "locks", { configurable: true, value: { request: async (name, ...args) => {
    assert(["commerce-purchase-write-v1", "commerce-buyer-session-v1"].includes(name));
    const previous = locked; if (name === "commerce-purchase-write-v1") locked = true;
    try { return await args.at(-1)(); } finally { locked = previous; }
  } } });
  globalThis.fetch = async (url, init) => {
    assert.equal(locked, true, "session checks and redeem must be inside the lock");
    assert.equal(String(url).includes(token), false);
    if (String(url).endsWith("/session")) return Response.json({ state: "active", context: ctx, expires_at: new Date(Date.now() + 100000).toISOString() });
    assert.equal(String(url), "/api/buyer/claim-link/redeem");
    const headers = new Headers(init.headers);
    assert.equal(headers.get("X-Commerce-Claim-Token"), token);
    assert.deepEqual(JSON.parse(init.body), { expected_bundle_version: 2 });
    calls.push(headers.get("Idempotency-Key"));
    return Response.json(redeemed);
  };
  try {
    assert.deepEqual(await purchase.redeemClaimLink(ctx, token, 2), redeemed);
    await purchase.redeemClaimLink(ctx, token, 2);
    assert.equal(new Set(calls).size, 2, "fresh key for each explicit click");
    assert.equal(writes.length, 0, "no token or redeem journal in storage");
    values.set(`commerce-purchase-order-v1:${ctx}`, JSON.stringify({ v: 1, context: ctx, order_id: sku }));
    await assert.rejects(purchase.redeemClaimLink(ctx, token, 2), { code: "uncertain" });
    assert.equal(calls.length, 2, "known order must prevent any B2");
    values.clear();
    values.set(`commerce-purchase-pending-v1:${ctx}`, JSON.stringify({ v: 1, context: ctx, key: sku, kind: "next-cart", body: { expected_version: 3, items: [] } }));
    await assert.rejects(purchase.redeemClaimLink(ctx, token, 2), { code: "uncertain" });
    assert.equal(calls.length, 2, "pending next-cart must prevent any B2");
    values.clear();
    globalThis.fetch = async (url) => {
      if (String(url).endsWith("/session")) return Response.json({ state: "active", context: ctx, expires_at: new Date(Date.now() + 100000).toISOString() });
      calls.push("lost"); throw new Error("synthetic reply loss");
    };
    await assert.rejects(purchase.redeemClaimLink(ctx, token, 2));
    assert.equal(calls.length, 3, "lost reply is not automatically retried");
    assert.equal(writes.some((entry) => JSON.stringify(entry).includes(token)), false);
    Object.defineProperty(navigator, "locks", { configurable: true, value: undefined });
    await assert.rejects(purchase.redeemClaimLink(ctx, token, 2), { code: "unavailable" });
  } finally {
    globalThis.fetch = oldFetch;
    globalThis.window = oldWindow; globalThis.localStorage = oldLocal; globalThis.sessionStorage = oldSession;
    if (oldLocks) Object.defineProperty(navigator, "locks", oldLocks); else delete navigator.locks;
  }
});

// Execute the actual component callbacks with deterministic hooks, not a copied decision model.
// Real DOM/navigation persistence is exercised by claim-checkout.mjs.
function claimPage({ lines, items, refreshedLines = lines, view = "ready" }) {
  const require = createRequire(import.meta.url), swc = require("next/dist/build/swc");
  const source = readFileSync(new URL("../components/ClaimLink.tsx", import.meta.url), "utf8");
  const compiled = swc.transformSync(source, { filename: "ClaimLink.tsx", jsc: { parser: { syntax: "typescript", tsx: true }, target: "es2022", transform: { react: { runtime: "automatic" } } }, module: { type: "commonjs" } }).code;
  const cart = { ...redeemed.cart, items }, claimed = { ...preview, lines };
  const states = ["zh-TW", view, "a".repeat(43), claimed, cart, false, null, false];
  let stateIndex = 0, refIndex = 0;
  const calls = { writes: [], navigation: [], reads: [] };
  const modules = {
    react: { useEffect() {}, useState: () => { const i = stateIndex++; return [states[i], value => { states[i] = value; }]; }, useRef: value => ({ current: refIndex++ === 0 ? token : value }) },
    "react/jsx-runtime": { jsx: (type, props) => ({ type, props }), jsxs: (type, props) => ({ type, props }), Fragment: "fragment" },
    "../lib/buyer-client": { BuyerClientError: class extends Error {}, buyerRequest: async (method, suffix) => { calls.reads.push(suffix); return Response.json({ ...claimed, lines: refreshedLines }); } },
    "../lib/purchase": { cartWithQuantity, pendingPurchase: () => null, knownOrderID: () => null,
      redeemClaimLink: async () => ({ ...redeemed, cart, applied: [], skipped: [] }),
      writePurchase: async (ctx, command) => { calls.writes.push(command); return { kind: "cart", value: { ...cart, items: command.body.items } }; } },
    "../lib/routes": { cartPath: locale => `/${locale}/cart`, checkoutPath: locale => `/${locale}/checkout` },
    "../lib/claim-contract": claimContract, "../lib/claim-copy": { claimCopy }, "../lib/money": { formatMoney: () => "NT$200" },
  };
  const exports = {};
  runInNewContext(compiled, { exports, require: name => { assert(name in modules, name); return modules[name]; }, window: { location: { assign: url => calls.navigation.push(url) } } });
  const tree = exports.default({ locale: "zh-TW" });
  function find(node, predicate) {
    if (!node || typeof node !== "object") return null;
    if (predicate(node)) return node;
    for (const child of [node.props?.children].flat(Infinity)) { const result = find(child, predicate); if (result) return result; }
    return null;
  }
  return { calls, states, click: async predicate => { const node = find(tree, predicate); assert(node, "control exists"); const button = node.type === "button" ? node : find(node, child => child.type === "button"); assert(button); button.props.onClick(); await new Promise(resolve => setImmediate(resolve)); } };
}

test("ClaimLink removal projects live-priced survivors on ready and conflict pages", async () => {
  const other = "33333333-3333-4333-8333-333333333333";
  for (const view of ["ready", "conflict"]) {
    const items = [{ sku_id: sku, quantity: 2, live_unit_price_minor: 20000 }, { sku_id: other, quantity: 1 }];
    const page = claimPage({ lines: preview.lines, items, view });
    await page.click(node => node.props?.["data-testid"] === `claim-cart-${other}`);
    assert.deepEqual(JSON.parse(JSON.stringify(page.calls.writes)), [{ kind: "cart", body: { expected_version: redeemed.cart.version, items: [{ sku_id: sku, quantity: 2 }] } }]);
    assert.equal(items[0].live_unit_price_minor, 20000, "read snapshot stays intact");
  }
});

test("ClaimLink revisit checks out applied A while pending B is sold out or unavailable", async () => {
  for (const status of [{ sold_out: true, available: true }, { sold_out: false, available: false }]) {
    const lines = [{ ...preview.lines[0], pending: false }, { ...preview.lines[0], sku_id: "33333333-3333-4333-8333-333333333333", keyword: "B2", ...status }];
    const page = claimPage({ lines, items: redeemed.cart.items });
    await page.click(node => node.props?.["data-testid"] === "claim-add");
    assert.deepEqual(page.calls.navigation, ["/zh-TW/checkout?from=claim"]);
  }
});

test("ClaimLink refreshes a nothing result after a second tab applied the claim", async () => {
  const page = claimPage({ lines: preview.lines, refreshedLines: [{ ...preview.lines[0], pending: false }], items: redeemed.cart.items });
  await page.click(node => node.props?.["data-testid"] === "claim-add");
  assert.deepEqual(page.calls.reads, ["claim-link"]);
  assert.deepEqual(page.calls.navigation, ["/zh-TW/checkout?from=claim"]);
});

test("ClaimLink does not navigate when a buyable claim target is absent or has changed quantity", async () => {
  for (const items of [[], [{ sku_id: sku, quantity: 1 }]]) {
    const page = claimPage({ lines: [{ ...preview.lines[0], pending: false }], items });
    await page.click(node => node.props?.["data-testid"] === "claim-add");
    assert.deepEqual(page.calls.navigation, []);
  }
});
