import assert from "node:assert/strict";
import test from "node:test";
import { proxyImage } from "../lib/media-proxy.ts";
import { productImageSet } from "../lib/routes.ts";

test("MS4 srcset descriptors use actual decoded widths and deduplicate small renditions", () => {
  assert.equal(productImageSet("p", "i", []), undefined, "historical image keeps original until backfill");
  assert.equal(productImageSet("p", "i", [360,720,1080].map(width => ({width, pixel_width: 3}))), "/media/p/p/i?w=360 3w");
  assert.equal(productImageSet("p", "i", [360,720,1080].map(width => ({width, pixel_width: width}))), "/media/p/p/i?w=360 360w, /media/p/p/i?w=720 720w, /media/p/p/i?w=1080 1080w");
});

const product = "11111111-1111-4111-8111-111111111111";
const image = "22222222-2222-4222-8222-222222222222";
const route = `/v1/buyer/media/p/${product}/${image}`;
const request = (query, host = "shop.example") => new Request(`https://${host}/media/p/${product}/${image}${query}`, {
  headers: { host, "x-forwarded-host": "foreign.example", cookie: "private=1" },
});

test("MS1 product rendition allowlist preserves trusted Host and never forwards buyer cookies", async (t) => {
  t.mock.method(globalThis, "fetch", async (url, init) => {
    assert.match(url, /\?w=(360|720|1080)$/);
    assert.equal(init.headers["X-Commerce-Storefront-Origin"], "https://shop.example");
    assert.equal(init.headers.cookie, undefined);
    assert.equal(init.cache, "no-store");
    assert.equal(init.redirect, "error");
    return new Response(new Uint8Array([255, 216, 255]), { headers: { "content-type": "image/jpeg" } });
  });
  const old = { ...process.env };
  Object.assign(process.env, { COMMERCE_BUYER_WEB_ENABLED: "1", COMMERCE_BUYER_API_ORIGIN: "http://127.0.0.1:19000", COMMERCE_BUYER_BFF_KEY: "A".repeat(43) });
  t.after(() => { process.env = old; });
  for (const width of [360, 720, 1080]) assert.equal((await proxyImage(request(`?w=${width}`), route)).status, 200);
});

test("MS2 invalid, duplicate, extra, encoded widths and non-product variants fail before fetch", async (t) => {
  t.mock.method(globalThis, "fetch", () => { assert.fail("invalid width must not fetch"); });
  for (const q of ["?w=", "?w=0", "?w=640", "?w=0360", "?w=360.0", "?w=-360", "?w=360&w=720", "?w=360&store_id=x", "?%77=360", "?w=%33%36%30"]) {
    assert.equal((await proxyImage(request(q), route)).status, 404, q);
  }
  assert.equal((await proxyImage(request("?w=360"), `/v1/buyer/media/s/${image}`)).status, 404);
});

test("MS3 old-image fallback is never promoted to immutable; committed variants are cacheable", async (t) => {
  const old = { ...process.env };
  Object.assign(process.env, { COMMERCE_BUYER_WEB_ENABLED: "1", COMMERCE_BUYER_API_ORIGIN: "http://127.0.0.1:19000", COMMERCE_BUYER_BFF_KEY: "A".repeat(43) });
  t.after(() => { process.env = old; });
  for (const exists of [false, true]) {
    const mock = t.mock.method(globalThis, "fetch", async () => new Response(new Uint8Array([255,216,255]), { headers: {
      "content-type": "image/jpeg", "X-Commerce-Image-Rendition": exists ? "1" : "0", "cache-control": exists ? "public, max-age=86400, immutable" : "no-store",
    } }));
    const response = await proxyImage(request("?w=360"), route);
    assert.equal(response.headers.get("cache-control"), exists ? "public, max-age=86400, immutable" : "no-store");
    assert.equal(response.headers.get("X-Commerce-Image-Rendition"), null, "private producer marker is not public API");
    mock.mock.restore();
  }
});
