// Purpose: actual history client -> native Request -> real GET/auth/cookies/localError -> synthetic upstream fetch.
// Depends on: real route/client/settings/auth, frozen real-route loader; only upstream network is faked.
// Used by: W5-U1 real-seam regression; browser bridge is in-process routing, never a fake BFF response/helper.
import assert from "node:assert/strict";
import { after, test } from "node:test";
import { randomBytes } from "node:crypto";
import { registerImportWireLoader } from "./import-wire-real-loader.mjs";
const publicOrigin = "https://admin.example.invalid";
const apiOrigin = "https://backend.example.invalid";
const env = { COMMERCE_IDENTITY_ENABLED: "1", COMMERCE_FIXTURE_ENABLED: "0", COMMERCE_PASSWORD_LOGIN_ENABLED: "1",
  COMMERCE_IDENTITY_ALLOW_LOOPBACK_TESTS: "0", COMMERCE_BFF_KEY: randomBytes(32).toString("base64url"),
  COMMERCE_PUBLIC_ORIGIN: publicOrigin, COMMERCE_API_ORIGIN: apiOrigin, COMMERCE_OIDC_ISSUER: "" };
const oldEnv = Object.fromEntries(Object.keys(env).map((name) => [name, process.env[name]])); Object.assign(process.env, env);
const loader = registerImportWireLoader();
const auth = await import("../../apps/admin/lib/auth.ts");
const { GET } = await import("../../apps/admin/app/api/stores/[store]/customers/[customer]/historical-orders/route.ts");
const { readHistoricalOrders } = await import("../../apps/admin/lib/import-history-client.ts");
const { sessionBoundary } = await import("../../apps/admin/lib/settings-client.ts");
const realFetch = globalThis.fetch; const docDescriptor = Object.getOwnPropertyDescriptor(globalThis, "document");
after(() => { globalThis.fetch = realFetch; loader.deregister();
  if (docDescriptor) Object.defineProperty(globalThis, "document", docDescriptor); else delete globalThis.document;
  for (const name of Object.keys(env)) { if (oldEnv[name] === undefined) delete process.env[name]; else process.env[name] = oldEnv[name]; }
});
const id = "abcdef11-1111-4111-8111-111111111111";
const page = { items: [], next_cursor: "", total: 0 };
async function fixture(run, options = {}) {
  const token = randomBytes(32).toString("base64url"); const headers = new Headers();
  assert.equal(auth.setSessionCookies(headers, token, new Date(Date.now() + 3600000).toISOString()), true);
  const pairs = headers.getSetCookie().map((v) => v.split(";", 1)[0]); const cookieHeader = pairs.join("; ");
  const csrfPair = pairs.find((p) => p.startsWith(auth.CSRF_COOKIE + "="));
  const doc = { cookie: csrfPair }; Object.defineProperty(globalThis, "document", { configurable: true, value: doc });
  const requests = []; const upstream = []; const responses = [];
  globalThis.fetch = async (input, init = {}) => {
    const url = new URL(typeof input === "string" ? input : input.url, publicOrigin);
    if (url.origin === publicOrigin) {
      // Route a browser-relative fetch in-process, adding the real helper-issued browser cookie jar.
      const req = new Request(url, { ...init, headers: { ...Object.fromEntries(new Headers(init.headers)), Cookie: cookieHeader } });
      requests.push(req); const parts = /^\/api\/stores\/([^/]+)\/customers\/([^/]+)\/historical-orders$/.exec(url.pathname);
      assert.ok(parts, "unexpected BFF resource");
      const response = await GET(req, { params: Promise.resolve({ store: parts[1], customer: parts[2] }) });
      responses.push(response.clone());
      if (response.headers.getSetCookie().some((v) => v.startsWith(auth.CSRF_COOKIE + "=;") && v.includes("Max-Age=0"))) doc.cookie = "";
      return response;
    }
    assert.equal(url.origin, apiOrigin, "only upstream fetch is faked");
    const req = new Request(input, init); upstream.push(req);
    if (url.pathname === "/v1/admin/stores") return Response.json({ items: [{ id, name: "Synthetic store", currency: "TWD", permissions: ["customers:read"] }] });
    assert.equal(url.pathname, `/v1/admin/stores/${id}/customers/${id}/historical-orders`);
    // Network-only mutation proves the client observes the actual route's rejected unsafe projection.
    if (process.env.HISTORY_SEAM_INJECT_CITY === "1" && !options.history)
      return Response.json({ ...page, address: "synthetic injected private cell" });
    return (options.history ?? (() => Response.json(page)))();
  };
  try { await run({ doc, boundary: await sessionBoundary(), requests, upstream, responses }); }
  finally { globalThis.fetch = realFetch; if (docDescriptor) Object.defineProperty(globalThis, "document", docDescriptor); else delete globalThis.document; }
}
test("actual client issues GET50/after through real route/private successful projection", async () => fixture(async ({ boundary, requests, upstream, responses }) => {
  assert.deepEqual(await readHistoricalOrders(id, id, "Abc-_1", boundary, new AbortController().signal), page);
  assert.equal(requests.length, 1); assert.equal(requests[0].method, "GET"); assert.equal(requests[0].body, null);
  assert.equal(new URL(requests[0].url).search, "?limit=50&after=Abc-_1");
  assert.ok(auth.sessionToken(requests[0])); assert.equal(upstream.length, 2);
  assert.equal(responses[0].headers.get("cache-control"), "private, no-store"); assert.equal(responses[0].headers.get("vary"), "Cookie");
}));
test("actual helper-issued browser fence rejects changed parent before dispatch", async () => fixture(async ({ doc, boundary, requests, upstream }) => {
  doc.cookie = `${auth.CSRF_COOKIE}=${randomBytes(32).toString("base64url")}`;
  await assert.rejects(readHistoricalOrders(id, id, "", boundary, new AbortController().signal), (e) => e.code === "signed-out");
  assert.equal(requests.length, 0); assert.equal(upstream.length, 0);
}));
test("private success from real route is discarded when the browser session changes during upstream read", { timeout: 5000 }, async () => {
  let finish; let started;
  const waiting = new Promise((resolve) => { started = resolve; });
  await fixture(async ({ doc, boundary, requests, upstream }) => {
    const read = readHistoricalOrders(id, id, "", boundary, new AbortController().signal);
    await waiting; doc.cookie = `${auth.CSRF_COOKIE}=${randomBytes(32).toString("base64url")}`; finish(Response.json(page));
    await assert.rejects(read, (e) => e.code === "signed-out"); assert.equal(requests.length, 1); assert.equal(upstream.length, 2);
  }, { history: () => new Promise((resolve) => { finish = resolve; started(); }) });
});
test("real localError no-store 401/403/404/503 preserves correct read category with no retry", async () => {
  for (const [status, code, category] of [[401, "unauthorized", "signed-out"], [403, "forbidden", "forbidden"], [404, "not_found", "not-found"], [503, "retry_later", "unavailable"]]) {
    await fixture(async ({ boundary, requests, upstream, responses }) => {
      await assert.rejects(readHistoricalOrders(id, id, "", boundary, new AbortController().signal), (e) => e.code === category);
      assert.equal(requests.length, 1); assert.equal(upstream.length, 2); assert.equal(responses[0].headers.get("cache-control"), "no-store");
      const body = await responses[0].json(); assert.equal(body.code, code); assert.match(body.request_id, /^[0-9a-f]{32}$/);
      if (status === 401) assert.equal(responses[0].headers.getSetCookie().length, 2);
    }, { history: () => Response.json({ code, message: "synthetic private diagnostic" }, { status }) });
  }
});
test("malformed sensitive backend projection becomes real unavailable, never a fake BFF success", async () => fixture(async ({ boundary, requests, responses }) => {
  await assert.rejects(readHistoricalOrders(id, id, "", boundary, new AbortController().signal), (e) => e.code === "unavailable");
  assert.equal(requests.length, 1); assert.equal(responses[0].status, 503);
  assert.equal(responses[0].headers.get("cache-control"), "no-store"); assert.equal((await responses[0].text()).includes("synthetic private diagnostic"), false);
}, { history: () => Response.json({ ...page, address: "synthetic private diagnostic" }) }));
