// storefront-v2 §C (unit checkout-offline): bank-transfer wire contract, the BFF routes, the options/checkout validators and the copy parity.
import test from "node:test";
import assert from "node:assert/strict";
import {
  minorFromText,
  transferCountdown,
  validBuyerEmail,
  validProofBody,
  validProofResult,
  validTransferView,
  TRANSFER_ERROR_CODES,
} from "../lib/bank-transfer-contract.ts";
import { bankTransferCopy } from "../lib/bank-transfer-copy.ts";
import { checkoutInput, validOption, validOrder } from "../lib/purchase.ts";
import { handleBuyerRequest } from "../lib/buyer-server.ts";

const cu = (n) => `00000000-0000-0000-0000-${String(n).padStart(12, "0")}`;
const order = cu(7);
const t0 = "2026-10-01T04:00:00.000000Z";
const view = (over = {}) => ({
  order_id: order, state: "AWAITING", window_hours: 72, deadline_at: t0, currency: "TWD", amount_minor: 90000,
  bank: { bank_name: "Taiwan Bank", branch: "Taipei", account_name: "Shop Ltd", account_number: "123-456-7890" },
  proof: null, reject_reason: null, confirmed_at: null, refunded_at: null, ...over,
});
const proof = { last5: "12345", amount_minor: 90000, paid_at: t0, submitted_at: t0, count: 1 };

test("transfer view: closed shape and state rules mirror Go validTransferView", () => {
  assert.equal(validTransferView(view(), order), true);
  assert.equal(validTransferView(view({ state: "SUBMITTED", proof }), order), true);
  assert.equal(validTransferView(view({ state: "REJECTED", proof, reject_reason: "amount differs" }), order), true);
  assert.equal(validTransferView(view({ state: "CONFIRMED", confirmed_at: t0 }), order), true);
  assert.equal(validTransferView(view({ state: "REFUNDED_OFFLINE", confirmed_at: t0, refunded_at: t0 }), order), true);
  assert.equal(validTransferView(view({ state: "EXPIRED", bank: null }), order), true);
  for (const [name, bad] of [
    ["another order", view({ order_id: cu(8) })],
    ["unknown state", view({ state: "PAID" })],
    ["window too short", view({ window_hours: 5 })],
    ["window too long", view({ window_hours: 169 })],
    ["bad currency", view({ currency: "twd" })],
    ["fractional amount", view({ amount_minor: 1.5 })],
    ["expired keeps the bank", view({ state: "EXPIRED" })],
    ["awaiting hides the bank", view({ bank: null })],
    ["submitted without proof", view({ state: "SUBMITTED" })],
    ["rejected without reason", view({ state: "REJECTED", proof })],
    ["reason while awaiting", view({ reject_reason: "x" })],
    ["confirmed without time", view({ state: "CONFIRMED" })],
    ["time while awaiting", view({ confirmed_at: t0 })],
    ["refund time without refund", view({ state: "CONFIRMED", confirmed_at: t0, refunded_at: t0 })],
    ["proof with letters", view({ state: "SUBMITTED", proof: { ...proof, last5: "12a45" } })],
    ["extra key", { ...view(), account_hint: "x" }],
    ["missing key", (({ proof, ...r }) => r)(view())],
    ["bank extra key", view({ bank: { ...view().bank, iban: "x" } })],
  ])
    assert.equal(validTransferView(bad, order), false, name);
  assert.equal(validTransferView(view(), "not-a-uuid"), false);
  assert.equal(validTransferView(null, order), false);
});

test("proof body and result: five digits, positive minor amount, ISO instant", () => {
  const body = { last5: "01234", amount_minor: 90000, paid_at: "2026-10-01T03:00:00.000Z" };
  assert.equal(validProofBody(body), true);
  assert.equal(validProofBody({ ...body, paid_at: "2026-10-01T11:00:00+08:00" }), true);
  for (const [name, bad] of [
    ["short digits", { ...body, last5: "1234" }],
    ["six digits", { ...body, last5: "123456" }],
    ["letters", { ...body, last5: "12a45" }],
    ["zero amount", { ...body, amount_minor: 0 }],
    ["float amount", { ...body, amount_minor: 10.5 }],
    ["string amount", { ...body, amount_minor: "900" }],
    ["date only", { ...body, paid_at: "2026-10-01" }],
    ["not a date", { ...body, paid_at: "yesterday" }],
    ["extra key", { ...body, note: "x" }],
  ])
    assert.equal(validProofBody(bad), false, name);
  const result = { order_id: order, state: "SUBMITTED", proof_count: 2, submitted_at: t0 };
  assert.equal(validProofResult(result, order), true);
  assert.equal(validProofResult({ ...result, state: "CONFIRMED" }, order), false, "a proof never confirms an order");
  assert.equal(validProofResult({ ...result, order_id: cu(9) }, order), false);
  assert.equal(validProofResult({ ...result, proof_count: 0 }, order), false);
});

test("minorFromText: major units with at most two decimals, never a float surprise", () => {
  assert.equal(minorFromText("900"), 90000);
  assert.equal(minorFromText("900.5"), 90050);
  assert.equal(minorFromText(" 1500.25 "), 150025);
  assert.equal(minorFromText("0.01"), 1);
  for (const bad of ["", "0", "0.00", "-5", "1,500", "12.345", "1e3", "abc", "9999999999999"]) assert.equal(minorFromText(bad), null, bad);
});

test("buyer email mirror: one plain address, 254 characters at most", () => {
  for (const ok of ["a@b.co", "buyer+tag@example.com", "x.y@sub.example.org"]) assert.equal(validBuyerEmail(ok), true, ok);
  for (const bad of ["", "plain", "a@", "@b.co", "a b@c.co", "Name <a@b.co>", "a@b.co,c@d.co", "a@@b.co", ".a@b.co", "a..b@c.co", "a@b..co", "(c)a@b.co",
    `"q"@b.co`, `${"a".repeat(250)}@b.co`, 5, null, undefined])
    assert.equal(validBuyerEmail(bad), false, String(bad));
});

test("countdown: whole hours and minutes, never negative", () => {
  const deadline = "2026-10-01T12:00:00Z";
  const at = (iso) => Date.parse(iso);
  assert.deepEqual(transferCountdown(deadline, at("2026-10-01T09:30:30Z")), { hours: 2, minutes: 29, expired: false });
  assert.deepEqual(transferCountdown(deadline, at("2026-10-01T11:59:30Z")), { hours: 0, minutes: 0, expired: false });
  assert.deepEqual(transferCountdown(deadline, at("2026-10-01T12:00:00Z")), { hours: 0, minutes: 0, expired: true });
  assert.deepEqual(transferCountdown(deadline, at("2026-10-02T00:00:00Z")), { hours: 0, minutes: 0, expired: true });
  assert.equal(transferCountdown("nope", 0).expired, true);
});

test("copy: three locales carry every key and every refusal code", () => {
  const keys = Object.keys(bankTransferCopy.en).sort().join();
  for (const locale of ["zh-CN", "zh-TW"]) assert.equal(Object.keys(bankTransferCopy[locale]).sort().join(), keys, locale);
  for (const [locale, copy] of Object.entries(bankTransferCopy)) {
    for (const code of TRANSFER_ERROR_CODES) assert.ok(copy.errors[code]?.length > 0, `${locale} ${code}`);
    for (const state of ["AWAITING", "SUBMITTED", "REJECTED", "CONFIRMED", "EXPIRED", "REFUNDED_OFFLINE"]) assert.ok(copy.states[state], `${locale} ${state}`);
    assert.match(copy.bankNote(72), /72/);
    assert.match(copy.timeLeft(3, 5), /3/);
  }
  // The shop, not the site, confirms: no locale may claim the buyer's own submission paid the order.
  assert.doesNotMatch(bankTransferCopy.en.sent, /\bpaid\b|confirmed/i);
});

// ---- options and checkout validators -------------------------------------------------------------------------------------------------
const home = {
  market_id: cu(3), country: "TW", currency: "TWD", method: "delivery:home", delivery_kind: "home", mode: "MANUAL",
  service_version: 4, allocation_version: 5, name_hans: "a", name_hant: "b", name_en: "c", sort_order: 1,
};
test("options: a home row lists modes only with bank_transfer and then needs its window", () => {
  assert.equal(validOption(home), true);
  assert.equal(validOption({ ...home, payment_modes: ["card", "bank_transfer"], transfer_window_hours: 72 }), true);
  assert.equal(validOption({ ...home, payment_modes: ["bank_transfer"], transfer_window_hours: 6 }), true);
  for (const [name, bad] of [
    ["window without bank_transfer", { ...home, transfer_window_hours: 72 }],
    ["bank_transfer without window", { ...home, payment_modes: ["card", "bank_transfer"] }],
    ["window out of range", { ...home, payment_modes: ["card", "bank_transfer"], transfer_window_hours: 200 }],
    ["card-only list on home", { ...home, payment_modes: ["card"] }],
    ["pay_at_pickup on home", { ...home, payment_modes: ["card", "pay_at_pickup", "bank_transfer"], transfer_window_hours: 72 }],
    ["duplicate", { ...home, payment_modes: ["bank_transfer", "bank_transfer"], transfer_window_hours: 72 }],
  ])
    assert.equal(validOption(bad), false, name);
  const cvs = { ...home, method: "delivery:cvs-711", delivery_kind: "cvs_711", mode: "API", pickup_selection: "ecpay_map" };
  assert.equal(validOption({ ...cvs, payment_modes: ["card", "pay_at_pickup", "bank_transfer"], transfer_window_hours: 48 }), true);
  assert.equal(validOption({ ...cvs, payment_modes: ["card", "pay_at_pickup", "bank_transfer"] }), false, "CVS bank_transfer needs its window too");
});

const soon = new Date(Date.now() + 3_600_000).toISOString();
const quote = {
  id: cu(4), cart_id: cu(1), cart_version: 2, market_id: cu(3), country: "TW", method: home.method, currency: "TWD", expires_at: soon,
  lines: [{ sku_id: cu(2), name: "N", code: "C", quantity: 1, unit_price_minor: 50000 }],
  amount: { subtotal_minor: 50000, discount_minor: 0, shipping_minor: 0, shipping_tax_minor: 0, tax_minor: 0, total_minor: 50000 },
};
const cart = { id: cu(1), version: 2, currency: "TWD", items: [{ sku_id: cu(2), quantity: 1 }] };
const homeHead = {
  id: cu(60), version: 1, cart_id: cu(1), cart_version: 2, kind: "home", country: "TW", recipient_name: "A", phone: "0912345678",
  home_address: { region: "", city: "C", postal_code: "", line1: "L", line2: "" }, selected_at: new Date().toISOString(), expires_at: soon,
};
test("checkoutInput: a home bank_transfer body, the optional trimmed email, and every refusal before any request", () => {
  const option = { ...home, payment_modes: ["card", "bank_transfer"], transfer_window_hours: 72 };
  assert.deepEqual(checkoutInput(quote, home, cart, homeHead), {
    quote_id: quote.id, destination_id: homeHead.id, cart_version: 2, service_version: 4, allocation_version: 5,
  }, "a card-only home body is unchanged");
  const body = checkoutInput(quote, option, cart, homeHead, Date.now(), "bank_transfer", "  buyer@example.com ");
  assert.equal(body.payment_mode, "bank_transfer");
  assert.equal(body.buyer_email, "buyer@example.com");
  assert.equal(checkoutInput(quote, option, cart, homeHead, Date.now(), "bank_transfer", "   ").buyer_email, undefined, "blank email is not sent");
  assert.equal(checkoutInput(quote, option, cart, homeHead).payment_mode, "card", "an option that names modes always names the chosen one");
  assert.throws(() => checkoutInput(quote, home, cart, homeHead, Date.now(), "bank_transfer"), "mode the row does not offer");
  assert.throws(() => checkoutInput(quote, option, cart, homeHead, Date.now(), "pay_at_pickup"), "pay_at_pickup is not offered");
  assert.throws(() => checkoutInput(quote, option, cart, homeHead, Date.now(), "bank_transfer", "not an email"), "invalid email");
});

test("order: AWAITING_TRANSFER is a valid commercial state without a hold time", () => {
  const base = {
    order_id: order, cart_id: cu(1), cart_version: 2, commercial_state: "AWAITING_TRANSFER", fulfillment_state: "MANUAL_UNASSIGNED",
    payment_mode: "bank_transfer", collection_state: null, cvs_shipment: null, shipment: null,
    snapshot: {
      quote: { currency: "TWD", lines: quote.lines, amount: quote.amount },
      destination: homeHead,
      service: { code: "home", name_hans: "a", name_hant: "b", name_en: "c", delivery_kind: "home", mode: "MANUAL" },
    },
  };
  assert.equal(validOrder(base), true);
  assert.equal(validOrder({ ...base, collection_state: "PENDING" }), false, "only pay_at_pickup carries a collection state");
  assert.equal(validOrder({ ...base, commercial_state: "AWAITING_REFUND" }), false);
});

// ---- BFF routes ------------------------------------------------------------------------------------------------------------------------
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
function req(method, suffix, { cookie, context, body, extra = {} } = {}) {
  const headers = { Host: "shop.example", ...extra };
  if (method !== "GET") headers.Origin = origin;
  if (cookie) headers.Cookie = cookie;
  if (context) headers["X-Buyer-Context"] = context;
  if (body !== undefined) headers["Content-Type"] = "application/json";
  return new Request(`${origin}/api/buyer/${suffix}`, { method, headers, body });
}
async function session() {
  const prepared = await handleBuyerRequest(req("POST", "session/prepare", { body: "{}" }));
  const set = prepared.headers.get("set-cookie");
  return { ck: set.split(";", 1)[0], context: (await prepared.json()).context };
}
const call = (s, method, suffix, { body, key } = {}) =>
  handleBuyerRequest(req(method, suffix, { cookie: s.ck, context: s.context, body, extra: key ? { "Idempotency-Key": key } : {} }));

test("BFF: exact transfer routes, keyed proof PUT, strict body, validated answers, no generic proxy", async () => {
  enabled();
  const old = globalThis.fetch;
  const calls = [];
  let answer = () => Response.json({}, { status: 500 });
  globalThis.fetch = async (url, options) => {
    calls.push({ url: String(url), method: options.method, key: options.headers.get("Idempotency-Key"), body: options.body });
    return answer(String(url), options);
  };
  try {
    const s = await session();
    answer = () => Response.json(view());
    let response = await call(s, "GET", `orders/${order}/bank-transfer`);
    assert.equal(response.status, 200);
    assert.equal((await response.json()).bank.account_number, "123-456-7890");
    assert.equal(calls.at(-1).url, `${api}/v1/buyer/orders/${order}/bank-transfer`);
    assert.equal(calls.at(-1).method, "GET");
    assert.equal(calls.at(-1).key, null);

    // an upstream answer about another order, or a malformed one, never reaches the browser
    answer = () => Response.json(view({ order_id: cu(8) }));
    assert.equal((await call(s, "GET", `orders/${order}/bank-transfer`)).status, 503);
    answer = () => Response.json({ ...view(), extra: 1 });
    assert.equal((await call(s, "GET", `orders/${order}/bank-transfer`)).status, 503);

    const body = { last5: "12345", amount_minor: 90000, paid_at: "2026-10-01T03:00:00.000Z" };
    answer = () => Response.json({ order_id: order, state: "SUBMITTED", proof_count: 1, submitted_at: t0 });
    response = await call(s, "PUT", `orders/${order}/bank-transfer/proof`, { body: JSON.stringify(body), key: "proof-key-0001" });
    assert.equal(response.status, 200);
    assert.equal(calls.at(-1).url, `${api}/v1/buyer/orders/${order}/bank-transfer/proof`);
    assert.equal(calls.at(-1).method, "PUT");
    assert.equal(calls.at(-1).key, "proof-key-0001");
    assert.equal(calls.at(-1).body, JSON.stringify(body));

    const before = calls.length;
    for (const [name, status, run] of [
      ["no key", 422, () => call(s, "PUT", `orders/${order}/bank-transfer/proof`, { body: JSON.stringify(body) })],
      ["short digits", 400, () => call(s, "PUT", `orders/${order}/bank-transfer/proof`, { body: JSON.stringify({ ...body, last5: "123" }), key: "proof-key-0002" })],
      ["zero amount", 400, () => call(s, "PUT", `orders/${order}/bank-transfer/proof`, { body: JSON.stringify({ ...body, amount_minor: 0 }), key: "proof-key-0003" })],
      ["extra field", 400, () => call(s, "PUT", `orders/${order}/bank-transfer/proof`, { body: JSON.stringify({ ...body, order_id: order }), key: "proof-key-0004" })],
      ["missing field", 400, () => call(s, "PUT", `orders/${order}/bank-transfer/proof`, { body: JSON.stringify((({ paid_at, ...r }) => r)(body)), key: "proof-key-0005" })],
      ["POST not PUT", 405, () => call(s, "POST", `orders/${order}/bank-transfer/proof`, { body: JSON.stringify(body), key: "proof-key-0006" })],
      ["GET the proof", 405, () => call(s, "GET", `orders/${order}/bank-transfer/proof`)],
      ["PUT the view", 405, () => call(s, "PUT", `orders/${order}/bank-transfer`, { body: JSON.stringify(body), key: "proof-key-0007" })],
      ["bad order id", 422, () => call(s, "GET", "orders/not-a-uuid/bank-transfer")],
      ["extra segment", 404, () => call(s, "GET", `orders/${order}/bank-transfer/proof/x`)],
      ["query string", 422, () => call(s, "GET", `orders/${order}/bank-transfer?x=1`)],
      ["key on GET", 422, () => call(s, "GET", `orders/${order}/bank-transfer`, { key: "proof-key-0008" })],
    ])
      assert.equal((await run()).status, status, name);
    assert.equal(calls.length, before, "no rejected request reached the private API");

    // a proof answer that claims the order is confirmed is drift
    answer = () => Response.json({ order_id: order, state: "CONFIRMED", proof_count: 1, submitted_at: t0 });
    assert.equal((await call(s, "PUT", `orders/${order}/bank-transfer/proof`, { body: JSON.stringify(body), key: "proof-key-0009" })).status, 503);
  } finally {
    globalThis.fetch = old;
  }
});

test("BFF: transfer refusal codes pass through as definite 422; checkout accepts bank_transfer and a valid email only", async () => {
  enabled();
  const old = globalThis.fetch;
  const calls = [];
  let refusal = { status: 422, code: "transfer_window_closed" };
  globalThis.fetch = async (url, options) => {
    calls.push({ url: String(url), body: options.body });
    return Response.json({ code: refusal.code, message: "x", request_id: "0".repeat(32), retryable: false, details: {} }, { status: refusal.status });
  };
  try {
    const s = await session();
    const proofBody = JSON.stringify({ last5: "12345", amount_minor: 90000, paid_at: "2026-10-01T03:00:00.000Z" });
    for (const code of TRANSFER_ERROR_CODES) {
      refusal = { status: 422, code };
      const response = await call(s, "PUT", `orders/${order}/bank-transfer/proof`, { body: proofBody, key: "proof-key-0010" });
      const body = await response.json();
      assert.equal(response.status, 422, code);
      assert.equal(body.code, code);
      assert.equal(body.retryable, false, `${code} committed nothing`);
    }
    const checkout = { quote_id: cu(4), destination_id: cu(5), cart_version: 2, service_version: 1, allocation_version: 1, payment_mode: "bank_transfer", buyer_email: "buyer@example.com" };
    refusal = { status: 422, code: "bank_transfer_unavailable" };
    let response = await call(s, "POST", "checkout", { body: JSON.stringify(checkout), key: "checkout-key-01" });
    assert.equal((await response.json()).code, "bank_transfer_unavailable");
    assert.equal(calls.at(-1).body, JSON.stringify(checkout), "payment_mode and buyer_email are forwarded verbatim");
    const sent = calls.length;
    for (const bad of [{ ...checkout, buyer_email: "not an email" }, { ...checkout, buyer_email: 7 }, { ...checkout, payment_mode: "cash" }, { ...checkout, buyer_phone: "1" }]) {
      response = await call(s, "POST", "checkout", { body: JSON.stringify(bad), key: "checkout-key-02" });
      assert.equal(response.status, 400, JSON.stringify(bad));
    }
    assert.equal(calls.length, sent, "rejected bodies stay local");
  } finally {
    globalThis.fetch = old;
  }
});
