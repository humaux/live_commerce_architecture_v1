// Purpose: issue #84 regression — when the storefront BFF rejects an upstream answer without reading it, it must cancel the
// response body (Node 24 bundled Undici: always consume or cancel, never rely on GC) and keep the same safe status.
// Depends on: lib/media-proxy.ts (proxyImage), lib/shop-upstream.ts (listProducts); "next/headers" and "react" are stubbed by a
// registerHooks resolver (no Next runtime, no network: global fetch is mocked with a cancel-observable ReadableStream).
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
