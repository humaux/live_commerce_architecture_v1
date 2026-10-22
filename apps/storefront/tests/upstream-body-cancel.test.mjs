// Purpose: issue #84 regression — when the storefront BFF rejects an upstream answer without reading it, it must cancel the
// response body (Node 24 bundled Undici: always consume or cancel, never rely on GC) and keep the same safe status.
// Depends on: lib/media-proxy.ts (proxyImage), lib/shop-upstream.ts (listProducts), app/feeds/meta.csv/route.ts (GET, #107); "next/headers" and "react" are stubbed by a
// registerHooks resolver (no Next runtime, no network: global fetch is mocked with a cancel-observable ReadableStream).
// Also covers the shared readBounded (#116, #147): a body that errors mid-read (also after partial data) is a 503 / down with Retry-After, never an escaped 500 or a cached partial 200.
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
const { proxyImage } = await import("../lib/media-proxy.ts");
const { listProducts } = await import("../lib/shop-upstream.ts");
const { readBounded } = await import("../lib/public-upstream.ts");
const { GET: metaFeed } = await import("../app/feeds/meta.csv/route.ts");

const env = { COMMERCE_BUYER_WEB_ENABLED: "1", COMMERCE_BUYER_API_ORIGIN: "http://127.0.0.1:19000", COMMERCE_BUYER_BFF_KEY: "A".repeat(43) };

// An upstream answer whose body never ends (a slow gateway error page); `cancelled` counts body cancellations.
function stalled(status, type) {
  const probe = { cancelled: 0 };
  probe.response = new Response(new ReadableStream({ start() {}, cancel() { probe.cancelled++; } }), {
    status, headers: { "content-type": type },
  });
  return probe;
}

function withEnv(t) {
  const old = { ...process.env };
  Object.assign(process.env, env);
  t.after(() => { process.env = old; });
}

const image = "22222222-2222-4222-8222-222222222222";

test("media proxy cancels every rejected upstream body and keeps the safe status", async (t) => {
  withEnv(t);
  for (const [status, type, want] of [[404, "image/png", 404], [422, "application/json", 404], [500, "image/png", 503], [200, "text/html", 503]]) {
    const probe = stalled(status, type);
    const mock = t.mock.method(globalThis, "fetch", async () => probe.response);
    const res = await proxyImage(new Request(`https://shop.example/media/s/${image}`, { headers: { host: "shop.example" } }), `/v1/buyer/media/s/${image}`);
    mock.mock.restore();
    assert.equal(res.status, want, `${status} ${type}`);
    assert.equal(probe.cancelled, 1, `${status} ${type}: rejected body not cancelled`);
  }
});

// #107 (PR #97 review P2-1): the Meta feed route had the same reject-without-cancel branches (404/422 -> 404, non-200 or non-CSV -> 503).
test("meta feed route cancels every rejected upstream body and keeps the safe status", async (t) => {
  withEnv(t);
  for (const [status, type, want] of [[404, "text/csv", 404], [422, "application/json", 404], [500, "text/csv", 503], [200, "text/html", 503]]) {
    const probe = stalled(status, type);
    const mock = t.mock.method(globalThis, "fetch", async () => probe.response);
    const res = await metaFeed(new Request("https://shop.example/feeds/meta.csv", { headers: { host: "shop.example" } }));
    mock.mock.restore();
    assert.equal(res.status, want, `${status} ${type}`);
    assert.equal(probe.cancelled, 1, `${status} ${type}: rejected body not cancelled`);
  }
});

test("shop upstream cancels every rejected upstream body and keeps the state", async (t) => {
  withEnv(t);
  for (const [status, type, want] of [[404, "application/json", "missing"], [500, "application/json", "down"], [200, "text/html", "down"]]) {
    const probe = stalled(status, type);
    const mock = t.mock.method(globalThis, "fetch", async () => probe.response);
    const got = await listProducts({ limit: 1 });
    mock.mock.restore();
    assert.equal(got.state, want, `${status} ${type}`);
    assert.equal(probe.cancelled, 1, `${status} ${type}: rejected body not cancelled`);
  }
});

// #116 (#110 review P2): a body that fails while being read (reset connection) escaped readBounded as a rejection -> 500.
function broken(type) {
  return new Response(new ReadableStream({ pull(c) { c.error(new Error("socket reset")); } }), { status: 200, headers: { "content-type": type } });
}

test("a body-read error is the route's own retryable 503 / down (per its header comment) on every shared readBounded caller", async (t) => {
  withEnv(t);
  let mock = t.mock.method(globalThis, "fetch", async () => broken("text/csv"));
  const feed = await metaFeed(new Request("https://shop.example/feeds/meta.csv", { headers: { host: "shop.example" } }));
  mock.mock.restore();
  assert.equal(feed.status, 503);
  assert.equal(feed.headers.get("cache-control"), "no-store");
  assert.equal(feed.headers.get("x-content-type-options"), "nosniff");
  assert.equal(feed.headers.get("retry-after"), "60");
  assert.equal((await feed.json()).code, "unavailable");

  mock = t.mock.method(globalThis, "fetch", async () => broken("image/png"));
  const img = await proxyImage(new Request(`https://shop.example/media/s/${image}`, { headers: { host: "shop.example" } }), `/v1/buyer/media/s/${image}`);
  mock.mock.restore();
  assert.equal(img.status, 503);
  assert.equal(img.headers.get("retry-after"), "60");

  mock = t.mock.method(globalThis, "fetch", async () => broken("application/json"));
  const got = await listProducts({ limit: 1 });
  mock.mock.restore();
  assert.equal(got.state, "down");
});

// #147 SF-1: real chunks arrive, then the stream rejects mid-read. A partial body must never become a 200 or be cached.
function partial(type) {
  const chunks = [new TextEncoder().encode("id,title\n1,first\n"), new TextEncoder().encode("2,sec")];
  return new Response(new ReadableStream({
    pull(c) { if (chunks.length) c.enqueue(chunks.shift()); else c.error(new Error("socket reset")); },
  }), { status: 200, headers: { "content-type": type } });
}

test("partial data then a mid-stream error: readBounded is null and every caller answers 503 / down, uncached", async (t) => {
  withEnv(t);
  assert.equal(await readBounded(partial("text/csv").body, 1024), null);

  let mock = t.mock.method(globalThis, "fetch", async () => partial("text/csv"));
  const feed = await metaFeed(new Request("https://shop.example/feeds/meta.csv", { headers: { host: "shop.example" } }));
  mock.mock.restore();
  assert.equal(feed.status, 503);
  assert.equal(feed.headers.get("cache-control"), "no-store");
  assert.equal(feed.headers.get("retry-after"), "60");

  mock = t.mock.method(globalThis, "fetch", async () => partial("image/png"));
  const img = await proxyImage(new Request(`https://shop.example/media/s/${image}`, { headers: { host: "shop.example" } }), `/v1/buyer/media/s/${image}`);
  mock.mock.restore();
  assert.equal(img.status, 503);
  assert.equal(img.headers.get("cache-control"), "no-store");
  assert.equal(img.headers.get("retry-after"), "60");

  mock = t.mock.method(globalThis, "fetch", async () => partial("application/json"));
  const got = await listProducts({ limit: 1 });
  mock.mock.restore();
  assert.equal(got.state, "down");
});
