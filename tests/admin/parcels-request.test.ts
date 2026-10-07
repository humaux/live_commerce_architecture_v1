// Purpose: node unit tests for the W3-07B parcel-group BFF request grammar (exact resources, delete query, keyless rules).
// Depends on: apps/admin/lib/parcels-request.ts (mirror of internal/httpapi/parcels.go routes).
// Used by: scripts/dev/test-local.sh --browser-merchant-orders-ui and --browser-merchant-orders-bff (pure grammar gate, no browser/PG).
import assert from "node:assert/strict";
import { test } from "node:test";
import { parcelRoute, validParcelDeleteQuery } from "../../apps/admin/lib/parcels-request.ts";

const g = "dddddddd-dddd-4ddd-8ddd-dddddddddddd";
const base = "http://127.0.0.1:3100/api/stores/11111111-1111-4111-8111-111111111111";

test("exactly the four W3-07B resources route", () => {
  assert.equal(parcelRoute("GET", "orders/merge-suggestions"), "get");
  assert.equal(parcelRoute("POST", "parcel-groups"), "command");
  assert.equal(parcelRoute("DELETE", `parcel-groups/${g}`), "delete");
  assert.equal(parcelRoute("PUT", `parcel-groups/${g}/shipment`), "command");
  for (const [method, path] of [
    ["GET", "parcel-groups"],
    ["GET", `parcel-groups/${g}`],
    ["GET", `parcel-groups/${g}/shipment`],
    ["POST", "orders/merge-suggestions"],
    ["POST", `parcel-groups/${g}`],
    ["PUT", "parcel-groups"],
    ["PUT", `parcel-groups/${g}`],
    ["DELETE", "parcel-groups"],
    ["DELETE", `parcel-groups/${g}/shipment`],
    ["PATCH", `parcel-groups/${g}`],
    ["GET", "orders/merge-suggestions/extra"],
    ["POST", "parcel-groups/"],
    ["DELETE", `parcel-groups/${g.toUpperCase()}`],
    ["DELETE", `parcel-groups/${g}x`],
    ["PUT", `parcel-groups/${g}/shipment/extra`],
  ] as const)
    assert.equal(parcelRoute(method, path), null, `${method} ${path}`);
});

test("dissolve carries exactly ?expected_version=N (N >= 1)", () => {
  const detail = `${base}/parcel-groups/${g}`;
  assert.equal(validParcelDeleteQuery(`${detail}?expected_version=1`), true);
  assert.equal(validParcelDeleteQuery(`${detail}?expected_version=9007199254740991`), true);
  for (const suffix of [
    "",
    "?",
    "?expected_version=",
    "?expected_version=0",
    "?expected_version=-1",
    "?expected_version=01",
    "?expected_version=1.0",
    "?expected_version=1&expected_version=2",
    "?expected_version=1&x=2",
    "?x=2&expected_version=1",
    "?Expected_Version=1",
    "?expected_version=9223372036854775808",
    "?expected_version=1?",
  ])
    assert.equal(validParcelDeleteQuery(detail + suffix), false, suffix || "(no query)");
});
