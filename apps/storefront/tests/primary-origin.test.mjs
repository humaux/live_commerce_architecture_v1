// Purpose: unit tests for canonical storefront origin redirect (primaryLocation validation and canonicalRedirect GET/HEAD/failure/mid-read-error handling, #147 SF-2).
// Depends on: node:assert/strict, node:test, ../lib/primary-origin.ts (canonicalRedirect, primaryLocation).
// Used by: scripts/dev/test-node.sh (apps/storefront/tests/*.test.mjs glob, Foundation "Node unit suites").
import assert from "node:assert/strict";
import test from "node:test";
import { canonicalRedirect, primaryLocation } from "../lib/primary-origin.ts";

const source = "https://store.platform.example";
const primary = "https://shop.merchant.example";
const env = { COMMERCE_BUYER_WEB_ENABLED: "1", COMMERCE_BUYER_API_ORIGIN: "http://127.0.0.1:19000", COMMERCE_BUYER_BFF_KEY: "A".repeat(43) };
test("primary origin: exact path/query, same host, and no URL-controlled destination", () => {
  const url = source + "/zh-TW/p/%E5%95%86%E5%93%81?q=a%2Fb&q=c+d&next=https://evil.example";
  assert.equal(primaryLocation({ primary_origin: primary }, source, url), primary + url.slice(source.length));
  assert.equal(primaryLocation({ primary_origin: source }, source, url), null);
  assert.equal(primaryLocation({ primary_origin: null }, source, url), null);
  assert.equal(primaryLocation({ primary_origin: primary }, source, source + "//evil.example/a"), primary + "//evil.example/a");
  for (const bad of ["http://evil.example", "https://user@evil.example", "https://evil.example/path", "https://evil.example?x=1", "https://evil.example#x", "https://evil.example:443", "https://evil.example\\@other.example", "https://evil.example%0d%0aX:bad", "//evil.example", "https://127.0.0.1", "https://EVIL.example"]) {
    assert.throws(() => primaryLocation({ primary_origin: bad }, source, url));
  }
  for (const bad of [{}, { primary_origin: primary, next: primary }, [], null]) assert.throws(() => primaryLocation(bad, source, url));
});
test("GET and HEAD use trusted Host, private bounded uncached resolver and exact 301", async () => {
  for (const method of ["GET", "HEAD"]) {
    let calls = 0;
    const response = await canonicalRedirect(new Request(source + "/en/products?a=1&a=2", { method, headers: { host: "store.platform.example", cookie: "private", "x-forwarded-host": "evil.example" } }), env, async (url, init) => {
      calls++;
      assert.equal(url, env.COMMERCE_BUYER_API_ORIGIN + "/v1/buyer/storefront/primary-origin");
      assert.deepEqual(init.headers, { Accept: "application/json", "X-Commerce-Buyer-BFF-Key": env.COMMERCE_BUYER_BFF_KEY, "X-Commerce-Storefront-Origin": source });
      assert.equal(init.cache, "no-store"); assert.equal(init.redirect, "error");
      return Response.json({ primary_origin: primary });
    });
    assert.equal(calls, 1); assert.equal(response.status, 301);
    assert.equal(response.headers.get("location"), primary + "/en/products?a=1&a=2");
    assert.equal(response.headers.get("cache-control"), "no-store");
  }
});
test("no writes redirected; resolution failures fail closed; unknown hosts keep native 404", async () => {
  const request = method => new Request(source + "/en", { method, headers: { host: "store.platform.example" } });
  assert.equal(await canonicalRedirect(request("POST"), env, () => { throw Error("must not fetch"); }), null);
  for (const fetcher of [async () => { throw Error("timeout"); }, async () => Response.json({ primary_origin: "https://evil.example/path" }), async () => new Response("x".repeat(2049), { headers: { "content-type": "application/json" } }), async () => new Response(null, { status: 500 })]) {
    assert.equal((await canonicalRedirect(request("GET"), env, fetcher)).status, 503);
  }
  assert.equal(await canonicalRedirect(request("GET"), env, async () => new Response(null, { status: 404 })), null);
});
// #147 SF-2: the resolver answers 200 JSON, then the body fails mid-read (also after partial data): fail closed, uncached, retryable.
test("a body-read error from the resolver is a 503 with no-store and Retry-After", async () => {
  const request = new Request(source + "/en", { headers: { host: "store.platform.example" } });
  const head = new TextEncoder().encode('{"primary_origin":"https://shop.merchant.example"}');
  const pulls = [
    (c) => c.error(new Error("socket reset")),
    (() => { let sent = false; return (c) => (sent ? c.error(new Error("socket reset")) : (sent = true, c.enqueue(head))); })(),
  ];
  for (const pull of pulls) {
    const response = await canonicalRedirect(request, env, async () => new Response(new ReadableStream({ pull }), { status: 200, headers: { "content-type": "application/json" } }));
    assert.equal(response.status, 503);
    assert.equal(response.headers.get("cache-control"), "no-store");
    assert.equal(response.headers.get("retry-after"), "5");
  }
});
