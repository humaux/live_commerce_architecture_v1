import assert from "node:assert/strict";
import { Readable } from "node:stream";
import { test } from "node:test";
import { orderActionRoute, validCSVHeaders, validKeylessRequest, validOrdersQuery } from "../../apps/admin/lib/orders-request.ts";

const collection =
  "http://127.0.0.1:3100/api/stores/11111111-1111-4111-8111-111111111111/orders";

test("raw merchant order query accepts only canonical collection fields", () => {
  for (const query of ["", "?limit=1", "?limit=100&state=all&cursor=A_-", "?state=shipped", "?state=unshipped"]) {
    assert.equal(validOrdersQuery(collection + query, false), true, query);
  }
  for (const query of [
    "?",
    "?limit=1&",
    "?limit=1&&state=all",
    "?limit=%31",
    "?%6cimit=1",
    "?limit=01",
    "?limit=101",
    "?limit=1&limit=2",
    "?cursor=A+B",
    "?cursor=%2541",
    "?state=draft",
    "?__next=1",
  ]) {
    assert.equal(validOrdersQuery(collection + query, false), false, query);
  }
});

test("merchant order detail rejects every query, including bare question mark", () => {
  const detail = `${collection}/22222222-2222-4222-8222-222222222222`;
  assert.equal(validOrdersQuery(detail, true), true);
  assert.equal(validOrdersQuery(detail + "?", true), false);
  assert.equal(validOrdersQuery(detail + "?limit=1", true), false);
});

const O = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const R = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
test("BFF grammar adds exactly the refund, shipment, export and permission resources", () => {
  const accepted: [string, string, string][] = [
    ["GET", `orders/${O}/refunds`, "get"],
    ["GET", `orders/${O}/shipment/history`, "get"],
    ["GET", "order-actions", "get"],
    ["GET", "orders/unshipped.csv", "csv"],
    ["POST", `orders/${O}/refunds`, "command"],
    ["POST", `orders/${O}/refunds/${R}/refresh`, "refresh"],
    ["PUT", `orders/${O}/shipment`, "command"],
  ];
  for (const [method, path, kind] of accepted) assert.equal(orderActionRoute(method, path), kind, `${method} ${path}`);
  for (const [method, path] of [
    ["PUT", `orders/${O}/refunds`], ["POST", `orders/${O}/shipment`], ["GET", `orders/${O}/shipment`],
    ["DELETE", `orders/${O}/refunds`], ["PATCH", `orders/${O}/shipment`], ["POST", "orders/unshipped.csv"],
    ["GET", "orders/unshipped.csv/"], ["GET", "orders/UNSHIPPED.csv"], ["GET", `orders/${O}/refunds/${R}`],
    ["POST", `orders/${O}/refunds/${R}/refresh/x`], ["POST", `orders/${O}/refunds/not-a-uuid/refresh`],
    ["GET", `orders/${O.toUpperCase()}/refunds`], ["GET", `orders/${O}/refunds/`], ["GET", "order-actions/x"],
    ["GET", `orders/${O}/../refunds`], ["GET", "orders"], ["GET", `orders/${O}`],
  ])
    assert.equal(orderActionRoute(method, path), null, `${method} ${path}`);
});

test("CSV pass-through requires the exact attachment headers", () => {
  const good = {
    "content-type": "text/csv; charset=utf-8",
    "content-disposition": 'attachment; filename="unshipped-1a2b3c4d-202609290102.csv"',
    "cache-control": "no-store, private",
    "x-export-truncated": "false",
  };
  assert.equal(validCSVHeaders(new Headers(good)), true);
  for (const change of [
    { "content-type": "application/json" },
    { "content-type": "text/csv" },
    { "content-disposition": "inline" },
    { "content-disposition": 'attachment; filename="../etc/passwd"' },
    { "content-disposition": 'attachment; filename="unshipped-1a2b3c4d-202609290102.csv"; x=1' },
    { "cache-control": "public, max-age=60" },
    { "x-export-truncated": "maybe" },
  ])
    assert.equal(validCSVHeaders(new Headers({ ...good, ...change })), false, JSON.stringify(change));
  const missing = new Headers(good);
  missing.delete("x-export-truncated");
  assert.equal(validCSVHeaders(missing), false);
});

// Next 16.3.5 (NextRequestAdapter.fromNodeNextRequest) passes the Node IncomingMessage as the body of every
// non-GET/HEAD request, so an empty POST still has a non-null `body` stream. Model exactly that here.
const target = "http://127.0.0.1:3100/api/stores/s/x";
const nextShaped = (method: string, headers: Record<string, string> = {}) =>
  new Request(target, { method, headers, ...(method === "GET" ? {} : { body: Readable.from([]) as never, duplex: "half" }) });

test("keyless refresh POST is accepted with Next's always-present empty body stream", () => {
  const empty = nextShaped("POST", { "content-length": "0" });
  assert.notEqual(empty.body, null, "premise: Next-shaped POST has a body stream");
  assert.equal(validKeylessRequest("refresh", empty), true);
  assert.equal(validKeylessRequest("refresh", nextShaped("POST")), true);
});

test("keyless requests still reject payload, chunking and keys", () => {
  for (const headers of [
    { "content-length": "2" },
    { "transfer-encoding": "chunked" },
    { "idempotency-key": "refund-12345678" },
    { "content-length": "0", "idempotency-key": "refund-12345678" },
  ])
    assert.equal(validKeylessRequest("refresh", nextShaped("POST", headers)), false, JSON.stringify(headers));
  assert.equal(validKeylessRequest("get", nextShaped("GET")), true);
  assert.equal(validKeylessRequest("csv", nextShaped("GET", { "content-length": "0" })), true);
  assert.equal(validKeylessRequest("get", nextShaped("GET", { "content-length": "1" })), false);
  assert.equal(validKeylessRequest("get", nextShaped("GET", { "idempotency-key": "k-12345678" })), false);
  // Only refresh may carry the adapter's stream; a GET-kind request with a body stream stays rejected.
  assert.equal(validKeylessRequest("get", nextShaped("POST", { "content-length": "0" })), false);
});
