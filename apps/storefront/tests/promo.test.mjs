// storefront-v2 §F (unit promotions): discount-code wire contract, quote/order validators with the optional `promotion`, the journal grammar,
// the BFF quote route (promo_code accepted only in canonical shape; refusal codes pass through as definite non-retryable 422s) and copy parity.
import test from "node:test";
import assert from "node:assert/strict";
import { PROMO_ERROR_CODES, isPromoErrorCode, normalizePromoCode, validPromotion } from "../lib/promo-contract.ts";
import { promoCopy } from "../lib/promo-copy.ts";
import { parsePending, validQuote } from "../lib/purchase.ts";
import { handleBuyerRequest } from "../lib/buyer-server.ts";

const cu = (n) => `00000000-0000-0000-0000-${String(n).padStart(12, "0")}`;
const quote = (over = {}) => ({
  id: cu(1), cart_id: cu(2), cart_version: 1, market_id: cu(3), country: "TW", method: "delivery:home", currency: "TWD",
  expires_at: "2026-10-01T04:00:00.000Z",
  lines: [{ sku_id: cu(4), name: "Tea", code: "TEA", quantity: 2, unit_price_minor: 45000 }],
  amount: { subtotal_minor: 90000, discount_minor: 9000, shipping_minor: 6000, shipping_tax_minor: 0, tax_minor: 0, total_minor: 87000 }, ...over,
});
const promotion = { code: "SAVE10", kind: "percent", percent: 10, fixed_minor: 0 };

test("PRO-S1 code mirror: any case and spaces normalise; impossible codes never become a request", () => {
  assert.equal(normalizePromoCode("  save-10 "), "SAVE-10");
  for (const bad of ["", "ab", "has space", "a".repeat(25), "emoji😀", "semi;colon", "ＡＢＣ"]) assert.equal(normalizePromoCode(bad), null, JSON.stringify(bad));
});

test("PRO-S2 promotion object: closed keys, kind/value pairing", () => {
  assert.equal(validPromotion(promotion), true);
  assert.equal(validPromotion({ code: "TEN-OFF", kind: "fixed", percent: 0, fixed_minor: 10000 }), true);
  for (const [name, bad] of Object.entries({
    "extra key": { ...promotion, id: cu(9) },
    "missing key": { code: "SAVE10", kind: "percent", percent: 10 },
    "lower-case code": { ...promotion, code: "save10" },
    "percent 91": { ...promotion, percent: 91 },
    "percent zero": { ...promotion, percent: 0 },
    "fixed with a percent": { code: "TEN-OFF", kind: "fixed", percent: 10, fixed_minor: 5 },
    "percent with an amount": { ...promotion, fixed_minor: 5 },
    "unknown kind": { ...promotion, kind: "bogo" },
    "null": null,
  })) assert.equal(validPromotion(bad), false, name);
});

test("PRO-S3 quote validator: promotion is optional but never malformed; the discount stays the server's own number", () => {
  assert.equal(validQuote(quote()), true);
  assert.equal(validQuote(quote({ promotion })), true);
  assert.equal(validQuote(quote({ promotion: { ...promotion, percent: 0 } })), false);
  assert.equal(validQuote(quote({ promotion: "SAVE10" })), false);
});

test("PRO-S4 journal grammar: a quote write may carry one canonical promo_code, nothing else", () => {
  const body = { cart_version: 1, market_id: cu(3), country: "TW", method: "delivery:home" };
  const raw = (b) => JSON.stringify({ v: 1, context: "x".repeat(43), key: cu(5), kind: "quote", body: b });
  const ctx = "x".repeat(43);
  assert.equal(parsePending(raw(body), ctx).kind, "quote");
  assert.equal(parsePending(raw({ ...body, promo_code: "SAVE10" }), ctx).body.promo_code, "SAVE10");
  for (const bad of ["save10", "AB", "SAVE 10", 5, "A".repeat(25)])
    assert.throws(() => parsePending(raw({ ...body, promo_code: bad }), ctx), /uncertain/, String(bad));
  assert.throws(() => parsePending(raw({ ...body, promo_code: "SAVE10", extra: 1 }), ctx), /uncertain/);
});

test("PRO-S5 copy: three locales cover every refusal code and the same keys", () => {
  for (const locale of ["en", "zh-CN", "zh-TW"]) {
    const c = promoCopy[locale];
    assert.deepEqual(Object.keys(c.errors).sort(), [...PROMO_ERROR_CODES].sort(), locale);
    assert.deepEqual(Object.keys(c).sort(), Object.keys(promoCopy.en).sort(), locale);
    for (const code of PROMO_ERROR_CODES) assert.ok(c.errors[code].length > 5, `${locale} ${code}`);
    assert.match(c.applied("SAVE10", "NT$90"), /SAVE10/);
  }
  assert.equal(isPromoErrorCode("promo_expired"), true);
  assert.equal(isPromoErrorCode("cvs_amount_exceeds"), false);
});

// ---- BFF ---------------------------------------------------------------------------------------------------------------------------------
const bff = Buffer.alloc(32, 1).toString("base64url");
const signing = Buffer.alloc(32, 2).toString("base64url");
const origin = "https://shop.example";
const api = "http://127.0.0.1:3219";
function enabled() {
  process.env.COMMERCE_BUYER_WEB_ENABLED = "1";
  process.env.COMMERCE_BUYER_API_ORIGIN = api;
  process.env.COMMERCE_BUYER_BFF_KEY = bff;
  process.env.COMMERCE_BUYER_COOKIE_KEY = signing;
  process.env.COMMERCE_BUYER_SESSION_TTL = "3600";
}
const req = (method, suffix, { cookie, context, body, extra = {} } = {}) => {
  const headers = { Host: "shop.example", ...extra };
  if (method !== "GET") headers.Origin = origin;
  if (cookie) headers.Cookie = cookie;
  if (context) headers["X-Buyer-Context"] = context;
  if (body !== undefined) headers["Content-Type"] = "application/json";
  return new Request(`${origin}/api/buyer/${suffix}`, { method, headers, body });
};
async function session() {
  const prepared = await handleBuyerRequest(req("POST", "session/prepare", { body: "{}" }));
  return { ck: prepared.headers.get("set-cookie").split(";", 1)[0], context: (await prepared.json()).context };
}
const post = (s, body, key = "quote-key-0001") =>
  handleBuyerRequest(req("POST", "quotes", { cookie: s.ck, context: s.context, body: JSON.stringify(body), extra: { "Idempotency-Key": key } }));

test("PRO-S6 BFF quote route: promo_code relayed untouched when canonical, refused before the private API otherwise", async () => {
  enabled();
  const old = globalThis.fetch;
  const calls = [];
  globalThis.fetch = async (url, options) => {
    calls.push({ url: String(url), body: options.body });
    return Response.json(quote({ promotion }));
  };
  try {
    const s = await session();
    const body = { cart_version: 1, market_id: cu(3), country: "TW", method: "delivery:home" };
    let response = await post(s, { ...body, promo_code: "SAVE10" });
    assert.equal(response.status, 200);
    assert.equal(calls.at(-1).url, `${api}/v1/buyer/quotes`);
    assert.equal(calls.at(-1).body, JSON.stringify({ ...body, promo_code: "SAVE10" }));
    assert.equal((await post(s, body, "quote-key-0002")).status, 200, "a quote without a code is unchanged");
    const before = calls.length;
    for (const [name, bad] of Object.entries({
      "too short": { ...body, promo_code: "AB" },
      "spaces": { ...body, promo_code: "SAVE 10" },
      "number": { ...body, promo_code: 10 },
      "unknown extra key": { ...body, promo_code: "SAVE10", discount_minor: 99999 },
    })) {
      assert.equal((await post(s, bad, "quote-key-0003")).status, 400, name);
    }
    assert.equal(calls.length, before, "no rejected body reached the private API");
  } finally {
    globalThis.fetch = old;
  }
});

test("PRO-S7 BFF: every promo_* refusal is a definite, non-retryable 422 with its own code", async () => {
  enabled();
  const old = globalThis.fetch;
  let refusal = "promo_invalid";
  globalThis.fetch = async () =>
    Response.json({ code: refusal, message: "x", request_id: "0".repeat(32), retryable: false, details: {} }, { status: 422 });
  try {
    const s = await session();
    const body = { cart_version: 1, market_id: cu(3), country: "TW", method: "delivery:home", promo_code: "SAVE10" };
    for (const code of PROMO_ERROR_CODES) {
      refusal = code;
      const response = await post(s, body, "quote-key-0004");
      const answer = await response.json();
      assert.equal(response.status, 422, code);
      assert.equal(answer.code, code);
      assert.equal(answer.retryable, false, `${code} committed nothing`);
    }
    // checkout relays the same codes (promo_changed ...) from BeginCheckout
    refusal = "promo_changed";
    const checkout = await handleBuyerRequest(req("POST", "checkout", { cookie: s.ck, context: s.context,
      body: JSON.stringify({ quote_id: cu(1), destination_id: cu(6), cart_version: 1, service_version: 1, allocation_version: 1 }), extra: { "Idempotency-Key": "checkout-key-0001" } }));
    assert.equal(checkout.status, 422);
    assert.equal((await checkout.json()).code, "promo_changed");
  } finally {
    globalThis.fetch = old;
  }
});
