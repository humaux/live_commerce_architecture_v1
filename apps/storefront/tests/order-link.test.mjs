// storefront-v2 section G3 (unit merchant-tools): the manual-order buyer link. Fragment parser (the token lives only in `#o=&t=`), body / answer
// validators, the BFF route (fresh token as bearer, cookie only on a 200 that names the SAME order, token never echoed, one identical refusal) and
// the copy parity of the three locales. The Go exchange (single use, 7 days, cross-store) is tests/foundation/merchant_tools_link_smoke_test.go.
import test from "node:test";
import assert from "node:assert/strict";
import { LINK_TOKEN, orderLinkFragment, validLinkBody, validLinkResult } from "../lib/order-link-contract.ts";
import { orderLinkCopy } from "../lib/order-link-copy.ts";
import { handleBuyerRequest } from "../lib/buyer-server.ts";

const bff = Buffer.alloc(32, 1).toString("base64url");
const signing = Buffer.alloc(32, 2).toString("base64url");
const origin = "https://shop.example";
const api = "http://127.0.0.1:3219";
const order = "00000000-0000-4000-8000-000000000007";
const token = Buffer.alloc(32, 9).toString("base64url");
function enabled() {
  process.env.COMMERCE_BUYER_WEB_ENABLED = "1";
  process.env.COMMERCE_BUYER_API_ORIGIN = api;
  process.env.COMMERCE_BUYER_BFF_KEY = bff;
  process.env.COMMERCE_BUYER_COOKIE_KEY = signing;
  process.env.COMMERCE_BUYER_SESSION_TTL = "3600";
}
function link(body, { method = "POST", extra = {} } = {}) {
  const headers = { Host: "shop.example", Origin: origin, "Content-Type": "application/json", ...extra };
  return new Request(`${origin}/api/buyer/orders/link`, { method, headers, body: method === "GET" ? undefined : body });
}
const good = JSON.stringify({ order_id: order, token });

test("fragment: exactly #o=<order id>&t=<43-char token>; anything else is null", () => {
  assert.deepEqual(orderLinkFragment(`#o=${order}&t=${token}`), { orderID: order, token });
  for (const bad of ["", "#", `#t=${token}&o=${order}`, `#o=${order}`, `#o=${order}&t=${token}&x=1`, `#o=${order}&t=${token.slice(1)}`, `#o=${"0a0b0c0d-0000-4000-8000-00000000000a".toUpperCase()}&t=${token}`,
    `?o=${order}&t=${token}`, `#o=not-a-uuid&t=${token}`, `#o=${order}&t=${token}+`, ` #o=${order}&t=${token}`])
    assert.equal(orderLinkFragment(bad), null, bad);
  assert.equal(LINK_TOKEN.test(token), true);
});

test("body is exactly {order_id, token}; the answer is exactly the order the link named", () => {
  assert.equal(validLinkBody({ order_id: order, token }), true);
  for (const bad of [{}, { order_id: order }, { order_id: order, token, store_id: "x" }, { order_id: "x", token }, { order_id: order, token: "short" }, [], null, "x"])
    assert.equal(validLinkBody(bad), false);
  assert.equal(validLinkResult({ order_id: order }, order), true);
  for (const bad of [{}, { order_id: order, extra: 1 }, { order_id: "00000000-0000-4000-8000-000000000008" }, null, []]) assert.equal(validLinkResult(bad, order), false);
});

test("BFF link: no cookie needed, fresh token upstream, cookie only on a 200 naming the same order, link token never echoed", async () => {
  enabled();
  const old = globalThis.fetch;
  const calls = [];
  let answer = () => Response.json({ order_id: order });
  globalThis.fetch = async (url, options) => {
    calls.push({ url: String(url), method: options.method, headers: options.headers, body: options.body });
    return answer();
  };
  try {
    let response = await handleBuyerRequest(link(good, { extra: { "X-Forwarded-For": "203.0.113.9", "X-Buyer-Context": "ignored" } }));
    assert.equal(response.status, 200);
    const text = await response.clone().text();
    assert.deepEqual(JSON.parse(text), { order_id: order });
    assert.equal(text.includes(token), false, "the link token is never echoed");
    const call = calls.at(-1);
    assert.equal(call.url, `${api}/v1/buyer/orders/link`);
    assert.equal(call.method, "POST");
    assert.equal(call.body, good);
    assert.equal(call.headers.get("Idempotency-Key"), null);
    const set = response.headers.get("set-cookie");
    assert.match(set, /^__Host-commerce_buyer=.+; Path=\/; Max-Age=3600; Secure; HttpOnly; SameSite=Lax$/);
    const payload = JSON.parse(Buffer.from(set.split("=", 2)[1].split(".")[0], "base64url").toString());
    assert.equal(call.headers.get("Authorization"), `Bearer ${payload.token}`, "the cookie holds the capability token Go registered");
    assert.notEqual(payload.token, token, "the cookie capability is NOT the link token");
    assert.equal(set.includes(token), false);
    await handleBuyerRequest(link(good));
    assert.notEqual(calls.at(-1).headers.get("Authorization"), call.headers.get("Authorization"), "every exchange mints a new capability");

    // every upstream refusal leaves the visitor's cookie alone and carries only Go's code
    answer = () => Response.json({ code: "not_found", message: "x", request_id: "0".repeat(32), retryable: false, details: {} }, { status: 404, headers: { "Cache-Control": "no-store", "X-Content-Type-Options": "nosniff" } });
    response = await handleBuyerRequest(link(good));
    assert.equal(response.status, 404);
    assert.equal(response.headers.get("set-cookie"), null);
    assert.equal((await response.json()).code, "not_found");
    answer = () => Response.json({ code: "rate_limited", message: "x", request_id: "0".repeat(32), retryable: true, details: {} }, { status: 429, headers: { "Retry-After": "77" } });
    response = await handleBuyerRequest(link(good));
    assert.equal(response.status, 429);
    assert.equal(response.headers.get("retry-after"), "77");
    assert.equal(response.headers.get("set-cookie"), null);

    // a 200 for ANOTHER order, an extra key or garbage never sets a cookie
    for (const bad of [{ order_id: "00000000-0000-4000-8000-000000000008" }, { order_id: order, extra: 1 }, { order_id: "x" }, {}]) {
      answer = () => Response.json(bad);
      response = await handleBuyerRequest(link(good));
      assert.equal(response.status, 503);
      assert.equal(response.headers.get("set-cookie"), null);
    }
  } finally {
    globalThis.fetch = old;
  }
});

test("BFF link: strict local denial before any private fetch", async () => {
  enabled();
  const old = globalThis.fetch;
  globalThis.fetch = () => {
    throw new Error("must not fetch");
  };
  try {
    const cases = [
      ["GET", 405, () => handleBuyerRequest(link(undefined, { method: "GET" }))],
      ["bad order id", 400, () => handleBuyerRequest(link(JSON.stringify({ order_id: "nope", token })))],
      ["short token", 400, () => handleBuyerRequest(link(JSON.stringify({ order_id: order, token: "abc" })))],
      ["extra key", 400, () => handleBuyerRequest(link(JSON.stringify({ order_id: order, token, store_id: "x" })))],
      ["not json", 400, () => handleBuyerRequest(link("{"))],
      ["other media type", 415, () => handleBuyerRequest(link(good, { extra: { "Content-Type": "text/plain" } }))],
      ["idempotency key", 422, () => handleBuyerRequest(link(good, { extra: { "Idempotency-Key": "abcdefgh1234" } }))],
      ["wrong origin", 403, () => handleBuyerRequest(link(good, { extra: { Origin: "https://evil.example" } }))],
      ["tenant header", 403, () => handleBuyerRequest(link(good, { extra: { "X-Tenant-ID": "t" } }))],
    ];
    for (const [name, status, run] of cases) {
      const response = await run();
      assert.equal(response.status, status, name);
      assert.equal(response.headers.get("set-cookie"), null, name);
    }
  } finally {
    globalThis.fetch = old;
  }
});

test("copy: three locales, one refusal text, a retry and a lookup path", () => {
  for (const locale of ["zh-CN", "zh-TW", "en"]) {
    const c = orderLinkCopy[locale];
    assert.deepEqual(Object.keys(c).sort(), Object.keys(orderLinkCopy.en).sort(), locale);
    for (const value of Object.values(c)) assert.ok(typeof value === "string" && value.length > 0);
  }
});
