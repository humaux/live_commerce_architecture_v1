// Purpose: drive the actual history GET route with native Requests and real auth/backend/cookie/CSRF helpers.
// Depends on: actual route/auth/model, frozen real-route loader and synthetic upstream fetch only.
// Used by: W5-U1 PROCESS real-seam gate; no shared helper or localError stand-ins, no browser/PG.
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
const originalFetch = globalThis.fetch;
after(() => { globalThis.fetch = originalFetch; loader.deregister();
  for (const name of Object.keys(env)) { if (oldEnv[name] === undefined) delete process.env[name]; else process.env[name] = oldEnv[name]; }
});
const store = "abcdef11-1111-4111-8111-111111111111";
const customer = "22222222-2222-4222-8222-222222222222";
const token = randomBytes(32).toString("base64url");
const cookies = new Headers(); assert.equal(auth.setSessionCookies(cookies, token, new Date(Date.now() + 3600000).toISOString()), true);
const pairs = cookies.getSetCookie().map((v) => v.split(";", 1)[0]); const cookieHeader = pairs.join("; ");
const csrf = pairs.find((p) => p.startsWith(auth.CSRF_COOKIE + "=")).slice(auth.CSRF_COOKIE.length + 1);
const url = `${publicOrigin}/api/stores/${store}/customers/${customer}/historical-orders`;
const page = { items: [{ order_id: "SL-SYNTHETIC", ordered_at: "2026-10-07T00:00:00Z", status: "done", total_minor: 100,
  currency: "TWD", items_summary: "synthetic", city: null }], next_cursor: "", total: 1 };
const storeReply = () => Response.json({ items: [{ id: store, name: "Synthetic store", currency: "TWD", permissions: ["customers:read"] }] });
const request = (search = "", init = {}) => new Request(url + search, { ...init, headers: { Cookie: cookieHeader, ...init.headers } });
async function fixture(run, options = {}) {
  const calls = [];
  globalThis.fetch = async (input, init = {}) => {
    const req = new Request(input, init); calls.push(req);
    assert.equal(new URL(req.url).origin, apiOrigin, "only upstream fetch is faked");
    if (new URL(req.url).pathname === "/v1/admin/stores") return (options.stores ?? storeReply)();
    assert.equal(new URL(req.url).pathname, `/v1/admin/stores/${store}/customers/${customer}/historical-orders`);
    // Test-only network mutation: actual projection must refuse a street in the otherwise successful upstream row.
    if (process.env.HISTORY_SEAM_INJECT_CITY === "1" && !options.history)
      return Response.json({ ...page, items: [{ ...page.items[0], city: "臺北市 合成街" }] });
    return (options.history ?? (() => Response.json(page)))();
  };
  try { await run(calls); } finally { globalThis.fetch = originalFetch; }
}
const call = (req = request(), selected = store, buyer = customer) => GET(req, { params: Promise.resolve({ store: selected, customer: buyer }) });
async function assertLocalError(response, status, code) {
  assert.equal(response.status, status); assert.equal(response.headers.get("cache-control"), "no-store");
  const body = await response.json(); assert.equal(body.code, code); assert.equal(typeof body.message, "string");
  assert.match(body.request_id, /^[0-9a-f]{32}$/); assert.equal(body.request_id, response.headers.get("x-request-id"));
  assert.equal(body.retryable, status >= 500 || status === 429); assert.deepEqual(body.details, {});
  return body;
}
test("real cookie and CSRF helpers generate and validate the actual host-cookie pair", () => {
  assert.equal(auth.sessionToken(request()), token);
  const valid = new Request(url, { method: "POST", headers: { Cookie: cookieHeader, Origin: publicOrigin, "X-CSRF-Token": csrf } });
  assert.equal(auth.requireOrigin(valid), true); assert.equal(auth.requireCSRF(valid), true);
  assert.equal(auth.requireCSRF(new Request(url, { method: "POST", headers: { Cookie: cookieHeader, "X-CSRF-Token": randomBytes(32).toString("base64url") } })), false);
  assert.ok(cookies.getSetCookie().every((v) => /Path=\/;.*Secure;.*SameSite=Lax/.test(v)));
  assert.ok(cookies.getSetCookie().find((v) => v.startsWith(auth.SESSION_COOKIE + "=")).includes("HttpOnly"));
});
test("exact route rejects methods/keys/body/query/scope before any upstream request using real localError", async () => fixture(async (calls) => {
  await assertLocalError(await call(request("", { method: "POST", body: "{}" })), 405, "method_not_allowed");
  for (const req of [request("?cursor=x"), request("?tenant_id=x"), request("?limit=050"), request("?after=a=b"),
    request("", { headers: { "Idempotency-Key": "synthetic-key" } }), request("", { headers: { "transfer-encoding": "chunked" } })])
    await assertLocalError(await call(req), 422, "invalid_request");
  await assertLocalError(await call(request(), store, "wrong"), 422, "invalid_request"); assert.equal(calls.length, 0);
}));
test("missing/duplicate session cookies reject and real clearAuthCookies expires session and CSRF", async () => fixture(async (calls) => {
  for (const Cookie of ["", `${cookieHeader}; ${auth.SESSION_COOKIE}=${token}`]) {
    const response = await call(request("", { headers: { Cookie } }));
    const expired = response.headers.getSetCookie(); assert.equal(expired.length, 2);
    assert.ok(expired.some((v) => v.startsWith(auth.SESSION_COOKIE + "=;") && v.includes("Max-Age=0")));
    assert.ok(expired.some((v) => v.startsWith(auth.CSRF_COOKIE + "=;") && v.includes("Max-Age=0")));
    await assertLocalError(response, 401, "unauthorized");
  }
  assert.equal(calls.length, 0);
}));
test("actual authenticatedStores denies a store absent from the server projection", async () => fixture(async (calls) => {
  await assertLocalError(await call(), 404, "not_found"); assert.equal(calls.length, 1);
}, { stores: () => Response.json({ items: [{ id: customer, name: "Other synthetic store", currency: "TWD" }] }) }));
test("private success routes server cookie token and exact canonical after, never caller bearer/cookie to Go", async () => fixture(async (calls) => {
  const response = await call(request("?limit=50&after=Abc-_1", { headers: { Authorization: "Bearer caller-controlled" } }));
  assert.deepEqual(await response.json(), page); assert.equal(response.status, 200);
  assert.equal(response.headers.get("cache-control"), "private, no-store"); assert.equal(response.headers.get("vary"), "Cookie");
  assert.equal(calls.length, 2); assert.equal(new URL(calls[1].url).search, "?limit=50&after=Abc-_1");
  for (const req of calls) { assert.equal(req.method, "GET"); assert.equal(req.headers.get("authorization"), `Bearer ${token}`);
    assert.equal(req.headers.has("cookie"), false); assert.equal(req.headers.has("x-commerce-bff-key"), false); assert.equal(req.body, null); }
}));
test("actual safeError/localError preserve coded status with no-store and redact upstream diagnostics", async () => {
  for (const [status, code] of [[401, "unauthorized"], [403, "forbidden"], [404, "not_found"], [503, "retry_later"]]) {
    await fixture(async (calls) => { const response = await call();
      if (status === 401) assert.equal(response.headers.getSetCookie().length, 2);
      const body = await assertLocalError(response, status, code); assert.equal(JSON.stringify(body).includes("synthetic private diagnostic"), false);
      assert.equal(calls.length, 2);
    }, { history: () => Response.json({ code, message: "synthetic private diagnostic", details: { cell: "synthetic private diagnostic" } }, { status }) });
  }
});
test("actual readBody and strict projection refuse MIME/oversize/extra private fields/city; network loss stays private", async () => {
  for (const history of [() => Response.json({ ...page, email: "synthetic@example.invalid" }),
    () => Response.json({ ...page, items: [{ ...page.items[0], city: "臺北市 合成街" }] }),
    () => new Response("synthetic", { headers: { "Content-Type": "text/html" } }),
    () => new Response(" ".repeat((512 << 10) + 1), { headers: { "Content-Type": "application/json" } }),
    () => { throw Error("synthetic upstream unavailable"); }]) {
    await fixture(async (calls) => { await assertLocalError(await call(), 503, "retry_later"); assert.equal(calls.length, 2); }, { history });
  }
});
