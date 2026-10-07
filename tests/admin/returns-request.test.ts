// W3-U5 BFF grammar half (contracts/returns-v1.md §6): the admin BFF must expose exactly the nine W3-08B
// returns/cancel resources — keyed command POSTs, bare JSON reads, and the single `?state=` query on GET returns —
// and nothing else. Independent of the Next runtime: imports only the grammar functions.
// Run: node --test --experimental-strip-types tests/admin/returns-request.test.ts
import assert from "node:assert/strict";
import { test } from "node:test";
import { orderActionRoute, validReturnsQuery } from "../../apps/admin/lib/orders-request.ts";

const O = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const R = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
const base = "http://127.0.0.1:3100/api/stores/11111111-1111-4111-8111-111111111111/returns";

test("BFF grammar adds exactly the returns and merchant-cancel resources", () => {
  const accepted: [string, string, string][] = [
    ["POST", `orders/${O}/cancel`, "command"],
    ["POST", `orders/${O}/returns`, "command"],
    ["GET", `orders/${O}/returns`, "get"],
    ["GET", "orders/cancel-refund-gaps", "get"],
    ["GET", "returns", "get"],
    ["POST", `returns/${R}/receive`, "command"],
    ["POST", `returns/${R}/inspect`, "command"],
    ["POST", `returns/${R}/close`, "command"],
    ["POST", `returns/${R}/cancel`, "command"],
  ];
  for (const [method, path, kind] of accepted) assert.equal(orderActionRoute(method, path), kind, `${method} ${path}`);
  for (const [method, path] of [
    ["GET", `orders/${O}/cancel`],
    ["POST", "orders/cancel-refund-gaps"],
    ["POST", "returns"],
    ["DELETE", `returns/${R}/cancel`],
    ["POST", `returns/${R}/receive/x`],
    ["POST", `returns/${R}/close/`],
    ["POST", `returns/not-a-uuid/receive`],
    ["POST", `returns/${R.toUpperCase()}/inspect`],
    ["GET", `orders/${O}/returns/${R}`],
    ["PUT", `orders/${O}/returns`],
    ["GET", "returns/"],
    ["GET", "returns/x"],
    ["GET", `orders/cancel-refund-gaps/${R}`],
    ["POST", `orders/${O}/cancel/refund`],
  ])
    assert.equal(orderActionRoute(method, path), null, `${method} ${path}`);
});

test("GET returns accepts no query or exactly one RMA state", () => {
  assert.equal(validReturnsQuery(base), true);
  for (const state of ["REGISTERED", "RECEIVED", "INSPECTED", "CLOSED", "CANCELLED"])
    assert.equal(validReturnsQuery(`${base}?state=${state}`), true, state);
  for (const query of [
    "?",
    "?state=",
    "?state=registered",
    "?state=OPEN",
    "?state=REGISTERED&state=CLOSED",
    "?state=REGISTERED&limit=10",
    "?limit=10",
    "?state=REGISTERED&",
    "?state=%52EGISTERED",
    "?__next=1",
  ])
    assert.equal(validReturnsQuery(base + query), false, query);
  // the order-scoped reads stay query-free
  assert.equal(validReturnsQuery(`${base}?state=CLOSED`), true);
});
