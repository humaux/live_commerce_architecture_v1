// storefront-v2 §E5 (unit buyer-comms): guest order lookup contract validators, the BFF route (fresh token as bearer, cookie only on a 200, same
// refusal for a mismatch, Retry-After relayed, client IP forwarded) and the copy parity of the three locales.
import test from "node:test";
import assert from "node:assert/strict";
import { validContact, validLookupBody, validLookupResult, validOrderRef } from "../lib/lookup-contract.ts";
import { lookupCopy } from "../lib/lookup-copy.ts";
import { handleBuyerRequest } from "../lib/buyer-server.ts";

const bff = Buffer.alloc(32, 1).toString("base64url");
const signing = Buffer.alloc(32, 2).toString("base64url");
const origin = "https://shop.example";
const api = "http://127.0.0.1:3219";
const order = "00000000-0000-0000-0000-000000000007";
function enabled() {
  process.env.COMMERCE_BUYER_WEB_ENABLED = "1";
  process.env.COMMERCE_BUYER_API_ORIGIN = api;
  process.env.COMMERCE_BUYER_BFF_KEY = bff;
  process.env.COMMERCE_BUYER_COOKIE_KEY = signing;
  process.env.COMMERCE_BUYER_SESSION_TTL = "3600";
}
function lookup(body, { method = "POST", extra = {} } = {}) {
  const headers = { Host: "shop.example", Origin: origin, "Content-Type": "application/json", ...extra };
  return new Request(`${origin}/api/buyer/orders/lookup`, { method, headers, body: method === "GET" ? undefined : body });
}
const good = JSON.stringify({ order_ref: "0123-ABCD-4567", contact: "buyer@example.test" });

test("order ref: 12 hex (dashes or spaces, any case) or a full id", () => {
  for (const ok of ["0123-ABCD-4567", "0123abcd4567", " 0123 abcd 4567 ", "00000000-0000-0000-0000-000000000007"]) assert.equal(validOrderRef(ok), true, ok);
  for (const bad of ["", "0123-ABCD-456", "0123-ABCD-4567-8", "0123-ABCD-456G", "x".repeat(65), 5, null, "0123_ABCD_4567"]) assert.equal(validOrderRef(bad), false, String(bad));
});

test("contact: one email or a phone of 6..20 digits", () => {
  for (const ok of ["a@b.co", " Buyer@Example.test ", "0912-345-678", "+886 912 345 678", "(02) 2345-6789"]) assert.equal(validContact(ok), true, ok);
  for (const bad of ["", "a@", "@b.co", "a b@c.co", "a@b@c", "12345", "123456789012345678901", "abc", "0912-345-67x", null, `${"a".repeat(250)}@b.co`]) assert.equal(validContact(bad), false, String(bad));
});

test("body is exactly {order_ref, contact}; the answer is exactly {order_id}", () => {
  assert.equal(validLookupBody({ order_ref: "0123-ABCD-4567", contact: "a@b.co" }), true);
  for (const bad of [{}, { order_ref: "0123-ABCD-4567" }, { order_ref: "0123-ABCD-4567", contact: "a@b.co", store_id: "x" }, { order_ref: "bad", contact: "a@b.co" }, [], null])
    assert.equal(validLookupBody(bad), false);
  assert.equal(validLookupResult({ order_id: order }), true);
  for (const bad of [{}, { order_id: "x" }, { order_id: order, extra: 1 }, { order_id: "0123ABCD-4567-4DEF-8123-456789ABCDEF" }, null, []]) assert.equal(validLookupResult(bad), false);
});

test("BFF lookup: no cookie needed, fresh token goes upstream, cookie only on 200 and it carries that same token", async () => {
  enabled();
  const old = globalThis.fetch;
  const calls = [];
  let answer = () => Response.json({ order_id: order });
  globalThis.fetch = async (url, options) => {
    calls.push({ url: String(url), method: options.method, headers: options.headers, body: options.body });
    return answer();
  };
  try {
    let response = await handleBuyerRequest(lookup(good, { extra: { "X-Forwarded-For": "203.0.113.9", "X-Buyer-Context": "ignored" } }));
    assert.equal(response.status, 200);
    assert.deepEqual(await response.json(), { order_id: order });
    const call = calls.at(-1);
    assert.equal(call.url, `${api}/v1/buyer/orders/lookup`);
    assert.equal(call.method, "POST");
    assert.equal(call.body, good);
    assert.equal(call.headers.get("X-Commerce-Client-IP"), "203.0.113.9");
    assert.equal(call.headers.get("Idempotency-Key"), null);
    const set = response.headers.get("set-cookie");
    assert.match(set, /^__Host-commerce_buyer=.+; Path=\/; Max-Age=3600; Secure; HttpOnly; SameSite=Lax$/);
    const payload = JSON.parse(Buffer.from(set.split("=", 2)[1].split(".")[0], "base64url").toString());
    assert.equal(call.headers.get("Authorization"), `Bearer ${payload.token}`, "the cookie holds the token Go registered");
    assert.equal(payload.origin, origin);
    // a second lookup mints a different token
    await handleBuyerRequest(lookup(good));
    assert.notEqual(calls.at(-1).headers.get("Authorization"), call.headers.get("Authorization"));
    assert.equal(calls.at(-1).headers.get("X-Commerce-Client-IP"), null, "no X-Forwarded-For, no header");

    // a list / port / zone is not forwarded as an IP
    for (const bad of ["1.1.1.1, 2.2.2.2", "1.1.1.1:80", "fe80::1%eth0", "nope"]) {
      await handleBuyerRequest(lookup(good, { extra: { "X-Forwarded-For": bad } }));
      assert.equal(calls.at(-1).headers.get("X-Commerce-Client-IP"), null, bad);
    }

    // every upstream refusal leaves the visitor's cookie alone
    answer = () => Response.json({ code: "not_found", message: "x", request_id: "0".repeat(32), retryable: false, details: {} }, { status: 404, headers: { "Cache-Control": "no-store", "X-Content-Type-Options": "nosniff" } });
    response = await handleBuyerRequest(lookup(good));
    assert.equal(response.status, 404);
    assert.equal(response.headers.get("set-cookie"), null);
    assert.equal((await response.json()).code, "not_found");
    answer = () => Response.json({ code: "rate_limited", message: "x", request_id: "0".repeat(32), retryable: true, details: {} }, { status: 429, headers: { "Retry-After": "123" } });
    response = await handleBuyerRequest(lookup(good));
    assert.equal(response.status, 429);
    assert.equal(response.headers.get("retry-after"), "123");
    assert.equal(response.headers.get("set-cookie"), null);

    // a malformed upstream 200 never sets a cookie
    for (const bad of [{ order_id: order, extra: 1 }, { order_id: "x" }, {}]) {
      answer = () => Response.json(bad);
      response = await handleBuyerRequest(lookup(good));
      assert.equal(response.status, 503);
      assert.equal(response.headers.get("set-cookie"), null);
    }
  } finally {
    globalThis.fetch = old;
  }
});

test("BFF lookup: strict local denial before any private fetch", async () => {
  enabled();
  const old = globalThis.fetch;
  globalThis.fetch = () => {
    throw new Error("must not fetch");
  };
  try {
    const cases = [
      ["GET", 405, () => handleBuyerRequest(lookup(undefined, { method: "GET" }))],
      ["bad ref", 400, () => handleBuyerRequest(lookup(JSON.stringify({ order_ref: "nope", contact: "a@b.co" })))],
      ["bad contact", 400, () => handleBuyerRequest(lookup(JSON.stringify({ order_ref: "0123-ABCD-4567", contact: "" })))],
      ["extra key", 400, () => handleBuyerRequest(lookup(JSON.stringify({ order_ref: "0123-ABCD-4567", contact: "a@b.co", store_id: "x" })))],
      ["not json", 400, () => handleBuyerRequest(lookup("{"))],
      ["other media type", 415, () => handleBuyerRequest(lookup(good, { extra: { "Content-Type": "text/plain" } }))],
      ["idempotency key", 422, () => handleBuyerRequest(lookup(good, { extra: { "Idempotency-Key": "abcdefgh1234" } }))],
      ["wrong origin", 403, () => handleBuyerRequest(lookup(good, { extra: { Origin: "https://evil.example" } }))],
      ["tenant header", 403, () => handleBuyerRequest(lookup(good, { extra: { "X-Tenant-ID": "t" } }))],
    ];
    for (const [name, status, run] of cases) assert.equal((await run()).status, status, name);
  } finally {
    globalThis.fetch = old;
  }
});

test("copy: three locales carry every key", () => {
  const keys = Object.keys(lookupCopy.en).sort().join();
  for (const locale of ["zh-CN", "zh-TW"]) assert.equal(Object.keys(lookupCopy[locale]).sort().join(), keys, locale);
  for (const [locale, copy] of Object.entries(lookupCopy)) for (const [k, v] of Object.entries(copy)) assert.ok(v.length > 0, `${locale}.${k}`);
  // one refusal for every mismatch: the text must not name which half was wrong
  for (const copy of Object.values(lookupCopy)) assert.doesNotMatch(copy.noMatch, /only|\bwrong (email|number)\b/i);
});
