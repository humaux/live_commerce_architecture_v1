// Purpose: #147 SF-4 — every public storefront route that answers a retryable 503 takes its Retry-After from the
//   single shared RETRY_AFTER constant in lib/public-upstream.ts. These cases observe the values the routes emit; an
//   equal-valued route-local literal ("60") would still pass, so the gate's witness is a divergent one (feed "61" while
//   RETRY_AFTER.down is "60", which fails the feed case below).
// Depends on: lib/public-upstream.ts (RETRY_AFTER), app/feeds/meta.csv/route.ts, lib/media-proxy.ts,
//   app/sitemap.xml/route.ts, lib/primary-origin.ts; "next/headers" and "react" stubbed below; fetch is mocked.
// Used by: scripts/dev/test-node.sh (apps/storefront/tests/*.test.mjs glob, Foundation "Node unit suites").
import assert from "node:assert/strict";
import { registerHooks } from "node:module";
import test from "node:test";

registerHooks({
  resolve(specifier, context, next) {
    if (specifier === "next/headers")
      return { url: "data:text/javascript,export async function headers(){return new Headers({host:'shop.example'})}", shortCircuit: true };
    if (specifier === "react") return { url: "data:text/javascript,export const cache=(f)=>f", shortCircuit: true };
    return next(specifier, context);
  },
});

const { RETRY_AFTER } = await import("../lib/public-upstream.ts");
const { GET: metaFeed } = await import("../app/feeds/meta.csv/route.ts");
const { proxyImage } = await import("../lib/media-proxy.ts");
const { GET: sitemap } = await import("../app/sitemap.xml/route.ts");
const { canonicalRedirect } = await import("../lib/primary-origin.ts");

const env = { COMMERCE_BUYER_WEB_ENABLED: "1", COMMERCE_BUYER_API_ORIGIN: "http://127.0.0.1:19000", COMMERCE_BUYER_BFF_KEY: "A".repeat(43) };
const image = "22222222-2222-4222-8222-222222222222";

function withEnv(t) {
  const old = { ...process.env };
  Object.assign(process.env, env);
  t.after(() => { process.env = old; });
}

// Upstream says 503 (or any non-200/404): the route must answer its own retryable 503 with the shared hint.
function upstreamDown(t, type) {
  return t.mock.method(globalThis, "fetch", async () => new Response("upstream down", { status: 500, headers: { "content-type": type } }));
}

test("RETRY_AFTER is the one shared source of the public storefront retry hints", () => {
  // RFC 9110 §10.2.3 seconds; 60 for a full data read, 5 for the cheaper canonical-origin resolver.
  assert.deepEqual({ ...RETRY_AFTER }, { down: "60", resolver: "5" });
});

test("meta feed 503 carries the shared down hint", async (t) => {
  withEnv(t);
  upstreamDown(t, "text/csv");
  const res = await metaFeed(new Request("https://shop.example/feeds/meta.csv", { headers: { host: "shop.example" } }));
  assert.equal(res.status, 503);
  assert.equal(res.headers.get("retry-after"), RETRY_AFTER.down);
  assert.equal(res.headers.get("cache-control"), "no-store");
});

test("media proxy 503 carries the shared down hint", async (t) => {
  withEnv(t);
  upstreamDown(t, "image/png");
  const res = await proxyImage(new Request(`https://shop.example/media/s/${image}`, { headers: { host: "shop.example" } }), `/v1/buyer/media/s/${image}`);
  assert.equal(res.status, 503);
  assert.equal(res.headers.get("retry-after"), RETRY_AFTER.down);
  assert.equal(res.headers.get("cache-control"), "no-store");
});

test("sitemap 503 carries the shared down hint", async (t) => {
  withEnv(t);
  upstreamDown(t, "application/json");
  const res = await sitemap();
  assert.equal(res.status, 503);
  assert.equal(res.headers.get("retry-after"), RETRY_AFTER.down);
  assert.equal(res.headers.get("cache-control"), "no-store");
});

// Published design loads, then one catalog list fails: the sitemap answers from its second 503 branch (route.ts:19),
// not the loadShop down branch (route.ts:17), and must use the same shared hint.
function designThenCatalog(t, { products, collections }) {
  return t.mock.method(globalThis, "fetch", async (url) => {
    const path = String(url);
    if (path.endsWith("/v1/buyer/design/published")) return Response.json({ version: 1, document: {} });
    if (path.includes("/v1/buyer/catalog/v2/products")) return products;
    if (path.includes("/v1/buyer/catalog/v2/collections")) return collections;
    throw new Error(`unexpected fetch ${path}`);
  });
}

const PRODUCTS_OK = { store: { name: "Shop", currency: "TWD" }, products: [], next: null };

test("sitemap 503 after a published design loads: product list down carries the shared down hint", async (t) => {
  withEnv(t);
  designThenCatalog(t, { products: new Response("upstream down", { status: 500 }), collections: Response.json({ collections: [] }) });
  const res = await sitemap();
  assert.equal(res.status, 503);
  assert.equal(res.headers.get("retry-after"), RETRY_AFTER.down);
  assert.equal(res.headers.get("cache-control"), "no-store");
});

test("sitemap 503 after a published design loads: collection list down carries the shared down hint", async (t) => {
  withEnv(t);
  designThenCatalog(t, { products: Response.json(PRODUCTS_OK), collections: new Response("upstream down", { status: 500 }) });
  const res = await sitemap();
  assert.equal(res.status, 503);
  assert.equal(res.headers.get("retry-after"), RETRY_AFTER.down);
  assert.equal(res.headers.get("cache-control"), "no-store");
});

test("primary-origin resolver 503 carries the shared resolver hint", async () => {
  const res = await canonicalRedirect(
    new Request("https://shop.example/en", { headers: { host: "shop.example" } }),
    env,
    async () => new Response(null, { status: 500 }),
  );
  assert.equal(res.status, 503);
  assert.equal(res.headers.get("retry-after"), RETRY_AFTER.resolver);
  assert.equal(res.headers.get("cache-control"), "no-store");
});
