// Purpose: native-Request proof for bodyless tag/note DELETE, query dispatch and private report Cookie cache variation.
// Depends on: real W6 route/auth/backend modules through w6-real-route-loader; upstream fetch only is fake.
// Used by: focused Node P2 red/green and root test-node gate registration; no PG/Next/browser runtime.
import test from "node:test";
import assert from "node:assert/strict";
import { w6RealBoundary, W6_STORE, W6_TAG } from "./w6-real-route-loader.mjs";

for (const kind of ["tag", "note"]) test(`P1 actual ${kind} DELETE accepts an empty Next-style stream without JSON MIME`, async () => {
  const boundary = await w6RealBoundary();
  const resource = kind === "tag" ? `customers/tags/${W6_TAG}` : `customers/${W6_STORE}/notes/${W6_TAG}`;
  const context = { params: Promise.resolve({ store: W6_STORE, tag: W6_TAG, customer: W6_STORE, note: W6_TAG }) };
  try {
    for (const streamed of [false, true]) {
      boundary.calls.length = 0;
      const request = new Request(boundary.request(resource, { "Idempotency-Key": "synthetic-delete-key" }), {
        method: "DELETE", ...(streamed ? { body: new ReadableStream({ start(c) { c.enqueue(new Uint8Array()); c.close(); } }), duplex: "half" } : {}),
      });
      assert.equal(request.headers.get("content-type"), null);
      assert.equal(request.body !== null, streamed);
      const response = await boundary[kind].DELETE(request, context);
      assert.equal(response.status, 200, "EMPTY-DELETE-MUST-REACH-GO");
      assert.equal((await response.json()).deleted, true);
      assert.equal(boundary.calls.length, 2, "real store authorization then exactly one deletion");
      const forwarded = boundary.calls[1];
      assert.equal(forwarded.init.body, undefined);
      assert.equal(forwarded.headers.get("content-type"), null);
      assert.equal(forwarded.headers.get("idempotency-key"), "synthetic-delete-key");
    }
    for (const type of [null, "text/plain", "application/json"]) {
      boundary.calls.length = 0;
      const headers = { "Idempotency-Key": "synthetic-delete-key", "Content-Length": "0", ...(type ? { "Content-Type": type } : {}) };
      const request = new Request(boundary.request(resource, headers), {
        method: "DELETE", body: new ReadableStream({ start(c) { c.enqueue(new TextEncoder().encode("{}")); c.close(); } }), duplex: "half",
      });
      assert.equal((await boundary[kind].DELETE(request, context)).status, 422, "nonempty DELETE is never forwarded, even with a false zero length");
      assert.equal(boundary.calls.length, 1, "only the authority lookup may run");
    }
  } finally { boundary.restore(); }
});

test("P1 body-carrying tag writes still require JSON MIME through the real BFF", async () => {
  const boundary = await w6RealBoundary();
  try {
    for (const type of [null, "text/plain"]) {
      const headers = { "Idempotency-Key": "synthetic-create-key", ...(type ? { "Content-Type": type } : {}) };
      const request = new Request(boundary.request("customers/tags", headers), { method: "POST", body: '{"name":"Synthetic","color":"blue"}' });
      if (!type) request.headers.delete("content-type");
      assert.equal((await boundary.tags.POST(request, boundary.customerContext)).status, 422);
      assert.equal(boundary.calls.length, 0, "MIME refusal before upstream");
    }
  } finally { boundary.restore(); }
});

test("P2 customer bare/encoded tag keys are invalid422 through the actual leaf before any backend", async () => {
  const boundary = await w6RealBoundary();
  try {
    for (const query of ["?tag", `?%74ag=${W6_TAG}`, `?ta%67=${W6_TAG}`]) {
      boundary.calls.length = 0;
      const response = await boundary.customers.GET(boundary.request(`customers${query}`), boundary.customerContext);
      assert.equal(response.status, 422, `P2-BARE-TAG-422 ${query}`);
      assert.equal((await response.json()).code, "invalid_request");
      assert.equal(boundary.calls.length, 0, "malformed dispatch never calls store authority or data transport");
    }
  } finally { boundary.restore(); }
});

for (const report of ["products", "products.csv"]) test(`P2 report ${report} real session/CSRF response is private and Vary Cookie`, async () => {
  const boundary = await w6RealBoundary();
  try {
    const response = await boundary.reports.GET(boundary.request(`reports/${report}?from=2026-09-01&to=2026-09-30`), boundary.reportContext(report));
    assert.equal(response.status, 200);
    assert.equal(response.headers.get("vary"), "Cookie", `P2-REPORT-VARY-COOKIE ${report}`);
    assert.equal(response.headers.get("cache-control"), "private, no-store");
    assert.equal(response.headers.get("x-content-type-options"), "nosniff");
    assert.equal(response.headers.get("set-cookie"), null, "no upstream cookies reach browser");
    assert.equal(boundary.calls.length, 2, "real authenticatedStores plus selected report");
    for (const call of boundary.calls) {
      assert.equal(call.headers.get("cookie"), null);
      assert.equal(call.headers.get("x-csrf-token"), null);
      assert.equal(call.headers.get("authorization"), `Bearer ${boundary.session}`);
      assert.equal(call.init.cache, "no-store"); assert.equal(call.init.redirect, "error");
    }
    if (report.endsWith(".csv")) { assert.equal(response.headers.get("content-type"), "text/csv; charset=utf-8"); assert.match(response.headers.get("content-disposition"), /^attachment;/); }
    else assert.deepEqual(await response.json(), { from: "2026-09-01", to: "2026-09-30", timezone: "Asia/Taipei", truncated: false, rows: [] });
  } finally { boundary.restore(); }
});

test("P2 report malformed query and wrong CSRF are rejected by actual helpers before upstream", async () => {
  const boundary = await w6RealBoundary();
  try {
    for (const query of ["?from=2026-09-01", "?from=2026-09-01&to=2026-09-30&tenant_id=foreign", "?from=2026-09-01&to=2026-09-30&to=2026-09-30"]) {
      const response = await boundary.reports.GET(boundary.request(`reports/products${query}`), boundary.reportContext("products"));
      assert.equal(response.status, 422); assert.equal(boundary.calls.length, 0);
    }
    const denied = await boundary.reports.GET(boundary.request("reports/products.csv?from=2026-09-01&to=2026-09-30", { "X-CSRF-Token": Buffer.alloc(32, 12).toString("base64url") }), boundary.reportContext("products.csv"));
    assert.equal(denied.status, 403); assert.equal(boundary.calls.length, 0);
  } finally { boundary.restore(); }
});
