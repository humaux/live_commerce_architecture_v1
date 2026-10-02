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
