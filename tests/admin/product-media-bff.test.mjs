// Purpose: guard PR394 cover_image_url through the actual admin BFF exports, including strict width and scope fences.
// Depends on: existing W6 real-route loader, real auth/backend helpers and native Request; only upstream fetch is MOCK.
// Used by: CI test-node; synthetic transport bytes prove no Go decoding, rendition dimensions, PG or LIVE behavior.
// Authority: contracts/storefront-v2.md amendment 2026-11-01 and contracts/media-sizes-v1.md; I01/I11/I15 remain enforced.
import test from "node:test";
import assert from "node:assert/strict";
import { loadW6RealRoutes, W6_STORE, W6_ORIGIN } from "./w6-real-route-loader.mjs";

const PRODUCT = "33333333-3333-4333-8333-333333333333";
const IMAGE = "44444444-4444-4444-8444-444444444444";
const FOREIGN_STORE = "55555555-5555-4555-8555-555555555555";
const ROOT = `products/${PRODUCT}/images`;
const ITEM = `${ROOT}/${IMAGE}`;
const COVER_URL = `/api/stores/${W6_STORE}/${ITEM}?w=360`;
const BYTES = new TextEncoder().encode("synthetic-image-transport-only");

/** Reuse the installed loader without replacing the route, cookies, CSRF, membership or backend functions. */
async function mediaBoundary({ cacheControl = "private, max-age=300" } = {}) {
  const { auth } = await loadW6RealRoutes();
  const route = await import("../../apps/admin/app/api/stores/[store]/[...resource]/route.ts");
  assert.ok(auth.authConfig, "real identity configuration must be active, never fixtureSession");
  const originalFetch = globalThis.fetch;
  const session = Buffer.alloc(32, 7).toString("base64url");
  const csrf = Buffer.alloc(32, 11).toString("base64url");
  const calls = [];
  globalThis.fetch = async (input, init = {}) => {
    const url = new URL(String(input));
    calls.push({ url, init, headers: new Headers(init.headers) });
    assert.equal(url.origin, auth.authConfig.apiOrigin);
    if (url.pathname === "/v1/admin/stores") return Response.json({ items: [{
      id: W6_STORE, name: "Synthetic media store", currency: "TWD", permissions: ["catalog:read", "catalog:write"],
    }] });
    if (url.pathname === `/v1/admin/stores/${W6_STORE}/catalog-products`) return Response.json({
      items: [{ id: PRODUCT, cover_image_url: COVER_URL }], next_cursor: "",
    });
    if (url.pathname === `/v1/admin/stores/${W6_STORE}/${ITEM}` && init.method === "GET") return new Response(BYTES.slice(), {
      headers: { "Content-Type": "image/png", "Cache-Control": cacheControl, "Set-Cookie": "untrusted-upstream=1" },
    });
    if (url.pathname === `/v1/admin/stores/${W6_STORE}/${ROOT}` && init.method === "POST") return Response.json({ image_id: IMAGE }, { status: 201 });
    throw new Error("unexpected MOCK media upstream");
  };
  return { calls, session, restore() { globalThis.fetch = originalFetch; },
    async invoke(resource, { method = "GET", store = W6_STORE, authenticated = true, headers = {}, body } = {}) {
      const address = resource.startsWith("/api/") ? resource : `/api/stores/${store}/${resource}`;
      const url = new URL(address, W6_ORIGIN);
      const segments = url.pathname.split("/");
      const request = new Request(url, { method, headers: {
        ...(authenticated ? { Cookie: `${auth.SESSION_COOKIE}=${session}; ${auth.CSRF_COOKIE}=${csrf}` } : {}),
        Origin: W6_ORIGIN, "Sec-Fetch-Site": "same-origin", "X-CSRF-Token": csrf, ...headers,
      }, ...(body !== undefined ? { body } : {}) });
      return route[method](request, { params: Promise.resolve({ store: segments[3], resource: segments.slice(4) }) });
    },
  };
}

function assertPrivateTransport(boundary) {
  for (const call of boundary.calls) {
    assert.equal(call.headers.get("authorization"), `Bearer ${boundary.session}`);
    for (const header of ["cookie", "x-csrf-token", "x-commerce-bff-key", "x-tenant-id", "x-store-id"]) assert.equal(call.headers.get(header), null, header);
    assert.equal(call.init.cache, "no-store");
    assert.equal(call.init.redirect, "error");
  }
}

for (const width of [360, 720, 1080]) test(`MOCK actual media GET forwards exact w=${width}; 360 follows catalog cover_image_url`, async () => {
  const boundary = await mediaBoundary();
  try {
    let address = `/api/stores/${W6_STORE}/${ITEM}?w=${width}`;
    if (width === 360) {
      const catalog = await boundary.invoke("catalog-products");
      assert.equal(catalog.status, 200);
      address = (await catalog.json()).items[0].cover_image_url;
      assert.equal(address, COVER_URL);
    }
    const response = await boundary.invoke(address);
    assert.equal(response.status, 200, "P1-MD-01 advertised thumbnail must reach the actual BFF image GET");
    assert.deepEqual(new Uint8Array(await response.arrayBuffer()), BYTES);
    assert.equal(response.headers.get("content-type"), "image/png");
    assert.equal(response.headers.get("x-content-type-options"), "nosniff");
    assert.equal(response.headers.get("content-security-policy"), "default-src 'none'; sandbox");
    assert.equal(response.headers.get("set-cookie"), null);
    const reads = boundary.calls.filter(call => call.url.pathname.endsWith(`/${ITEM}`));
    assert.equal(reads.length, 1, "exactly one image read, never retry");
    assert.equal(reads[0].url.pathname, `/v1/admin/stores/${W6_STORE}/${ITEM}`);
    assert.equal(reads[0].url.search, `?w=${width}`, "width must reach Go unchanged");
    assert.equal(reads[0].init.method, "GET");
    assert.equal(reads[0].init.body, undefined);
    assert.equal(boundary.calls.filter(call => call.url.pathname === "/v1/admin/stores").length, width === 360 ? 2 : 1);
    assertPrivateTransport(boundary);
  } finally { boundary.restore(); }
});

test("MOCK actual media original GET retains no query and unchanged upstream bytes", async () => {
  const boundary = await mediaBoundary();
  try {
    const response = await boundary.invoke(ITEM);
    assert.equal(response.status, 200);
    assert.deepEqual(new Uint8Array(await response.arrayBuffer()), BYTES);
    assert.equal(boundary.calls.length, 2, "membership then image");
    assert.equal(boundary.calls[1].url.href, `${boundary.calls[0].url.origin}/v1/admin/stores/${W6_STORE}/${ITEM}`);
    assertPrivateTransport(boundary);
  } finally { boundary.restore(); }
});

test("MOCK historical rendition fallback preserves upstream no-store through actual media GET", async () => {
  const boundary = await mediaBoundary({ cacheControl: "no-store" });
  try {
    const response = await boundary.invoke(`${ITEM}?w=360`);
    assert.equal(response.status, 200);
    assert.deepEqual(new Uint8Array(await response.arrayBuffer()), BYTES);
    assert.match(response.headers.get("cache-control") ?? "", /(?:^|,\s*)no-store(?:,|$)/);
    assert.doesNotMatch(response.headers.get("cache-control") ?? "", /max-age|immutable|public/);
    assert.equal(boundary.calls.length, 2);
    assert.equal(boundary.calls[1].url.search, "?w=360");
  } finally { boundary.restore(); }
});

test("MOCK actual media GET rejects malformed raw width queries before any upstream", async () => {
  const boundary = await mediaBoundary();
  try {
    for (const query of ["?", "?w", "?w=", "?w=361", "?w=0360", "?w=+360", "?w=360.0", "?w=360&w=360", "?w=360&w=720",
      "?w=360&x=1", "?tenant_id=synthetic", "?W=360", "?%77=360", "?w=%33%36%30", "?w=360&", "?w=360=720", "?role=main"]) {
      boundary.calls.length = 0;
      const response = await boundary.invoke(ITEM + query);
      assert.equal(response.status, 422, query);
      assert.equal((await response.json()).code, "invalid_request", query);
      assert.equal(boundary.calls.length, 0, query);
    }
  } finally { boundary.restore(); }
});

test("MOCK width cannot authorize another image resource or method", async () => {
  const boundary = await mediaBoundary();
  try {
    const cases = [
      ["GET", ROOT, 422], ["GET", `products/${PRODUCT}`, 422], ["GET", `collections/${PRODUCT}/image`, 422],
      ["POST", ROOT, 422], ["POST", `${ITEM}/delete`, 422], ["POST", `${ITEM}/move`, 422], ["POST", `${ROOT}/order`, 422],
      ["POST", `products/${PRODUCT}/option-images`, 422], ["POST", `products/${PRODUCT}/image-axis`, 422],
      ["POST", ITEM, 404], ["PATCH", ITEM, 404], ["PUT", ITEM, 404], ["DELETE", ITEM, 405], ["HEAD", ITEM, 405],
    ];
    for (const [method, resource, status] of cases) {
      boundary.calls.length = 0;
      assert.equal((await boundary.invoke(`${resource}?w=360`, { method })).status, status, `${method} ${resource}`);
      assert.equal(boundary.calls.length, 0, `${method} ${resource}`);
    }
  } finally { boundary.restore(); }
});

test("MOCK width GET preserves original read framing and idempotency-key refusals", async () => {
  const boundary = await mediaBoundary();
  try {
    for (const headers of [{ "Idempotency-Key": "synthetic-image-read" }, { "Content-Length": "1" }, { "Transfer-Encoding": "chunked" }]) {
      boundary.calls.length = 0;
      const response = await boundary.invoke(`${ITEM}?w=360`, { headers });
      assert.equal(response.status, 422);
      assert.equal((await response.json()).code, "invalid_request");
      assert.equal(boundary.calls.length, 0);
    }
  } finally { boundary.restore(); }
});

test("MOCK actual media route retains legacy upload role grammar and raw multipart body", async () => {
  const boundary = await mediaBoundary();
  try {
    const body = "--synthetic-boundary\r\nContent-Disposition: form-data; name=\"file\"; filename=\"synthetic.png\"\r\nContent-Type: image/png\r\n\r\nsynthetic-only\r\n--synthetic-boundary--\r\n";
    const headers = { "Content-Type": "multipart/form-data; boundary=synthetic-boundary", "Idempotency-Key": "synthetic-upload-key" };
    for (const query of ["", "?role=main", "?role=detail", "?role=sku&option_value=Blue", "?role=sku&option_value=%E8%97%8D"]) {
      boundary.calls.length = 0;
      const response = await boundary.invoke(ROOT + query, { method: "POST", headers, body });
      assert.equal(response.status, 201, query);
      assert.equal(boundary.calls.length, 2);
      assert.equal(boundary.calls[1].url.search, query);
      assert.equal(boundary.calls[1].headers.get("idempotency-key"), "synthetic-upload-key");
      assert.equal(boundary.calls[1].headers.get("content-type"), headers["Content-Type"]);
      assert.equal(await boundary.calls[1].init.body.text(), body);
      assertPrivateTransport(boundary);
    }
    for (const query of ["?role=sku", "?role=main&option_value=Blue", "?role=main&role=detail", "?role=main&w=360", "?%72ole=main"]) {
      boundary.calls.length = 0;
      const response = await boundary.invoke(ROOT + query, { method: "POST", headers, body });
      assert.equal(response.status, 422, query);
      assert.equal(boundary.calls.length, 0, query);
    }
  } finally { boundary.restore(); }
});

test("MOCK actual session parser refuses unauthenticated original and width reads", async () => {
  const boundary = await mediaBoundary();
  try {
    for (const query of ["", "?w=360"]) {
      boundary.calls.length = 0;
      const response = await boundary.invoke(ITEM + query, { authenticated: false });
      assert.equal(response.status, 401, query);
      assert.equal((await response.json()).code, "unauthorized");
      assert.equal(boundary.calls.length, 0, "no session means no upstream lookup or bytes");
    }
  } finally { boundary.restore(); }
});

test("MOCK actual membership check refuses foreign store despite client scope headers", async () => {
  const boundary = await mediaBoundary();
  try {
    for (const query of ["", "?w=360"]) {
      boundary.calls.length = 0;
      const response = await boundary.invoke(ITEM + query, { store: FOREIGN_STORE, headers: {
        "X-Store-ID": W6_STORE, "X-Tenant-ID": "synthetic-tenant", "X-Forwarded-Host": "synthetic.invalid",
      } });
      assert.equal(response.status, 404, query);
      assert.equal((await response.json()).code, "not_found");
      assert.equal(boundary.calls.length, 1, "only authoritative membership lookup; no foreign image fetch");
      assert.equal(boundary.calls[0].url.pathname, "/v1/admin/stores");
      assertPrivateTransport(boundary);
    }
  } finally { boundary.restore(); }
});
