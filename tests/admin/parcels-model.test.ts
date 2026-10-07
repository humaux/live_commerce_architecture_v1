// Purpose: node unit tests for the W3-07B parcel-group DTO parsers and the three-locale copy parity.
// Depends on: apps/admin/lib/parcels-model.ts (frozen shapes of internal/httpapi/parcels.go) and parcels-copy.ts.
// Used by: scripts/dev/test-local.sh --browser-merchant-orders-ui (pure model gate, no browser/PG).
import assert from "node:assert/strict";
import { test } from "node:test";
import {
  parseMergeSuggestions,
  parseParcelGroupCreated,
  parseParcelGroupDissolved,
  parseParcelGroupShipped,
  parcelRefusalCodes,
} from "../../apps/admin/lib/parcels-model.ts";
import { parcelCopy } from "../../apps/admin/lib/parcels-copy.ts";

const a = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const b = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
const group = "dddddddd-dddd-4ddd-8ddd-dddddddddddd";
const principal = "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee";

const shipment = {
  version: 1,
  status: "SHIPPED",
  carrier_code: "black_cat",
  carrier_name: null,
  tracking_number: "PARCEL-TRACK-001",
  tracking_url: null,
  recorded_at: "2026-10-07T01:02:03.000000Z",
  note: null,
  void_reason: null,
  principal_id: principal,
};

test("merge suggestions accept the frozen shape and reject drift", () => {
  const ok = { items: [{ recipient_name: "Synthetic Buyer", order_ids: [a, b] }] };
  assert.deepEqual(parseMergeSuggestions(ok), [{ recipient_name: "Synthetic Buyer", order_ids: [a, b] }]);
  assert.deepEqual(parseMergeSuggestions({ items: [] }), []);
  const bad = [
    {},
    { items: {}, },
    { items: [{ recipient_name: "x", order_ids: [a, b], extra: 1 }] },
    { items: [{ recipient_name: "", order_ids: [a, b] }] },
    { items: [{ recipient_name: 1, order_ids: [a, b] }] },
    { items: [{ recipient_name: "x", order_ids: [a] }] }, // one order is never a group
    { items: [{ recipient_name: "x", order_ids: [a, a] }] }, // duplicates
    { items: [{ recipient_name: "x", order_ids: [a, "not-a-uuid"] }] },
  ];
  const overflow = Array.from({ length: 21 }, (_, i) => `${String(i).padStart(8, "0")}-0000-4000-8000-000000000000`);
  bad.push({ items: [{ recipient_name: "x", order_ids: overflow }] });
  for (const value of bad) assert.throws(() => parseMergeSuggestions(value), /unavailable/);
  const over100 = { items: Array.from({ length: 101 }, () => ({ recipient_name: "x", order_ids: [a, b] })) };
  assert.throws(() => parseMergeSuggestions(over100), /unavailable/);
});

test("created group requires the OPEN state and its members", () => {
  const ok = { id: group, state: "OPEN", version: 1, order_ids: [a, b] };
  assert.deepEqual(parseParcelGroupCreated(ok), ok);
  for (const value of [
    { id: group, state: "OPEN", version: 1 }, // members required on create
    { id: group, state: "SHIPPED", version: 1, order_ids: [a, b] },
    { id: group, state: "OPEN", version: 0, order_ids: [a, b] },
    { id: group, state: "OPEN", version: 1, order_ids: [a, b], extra: 1 },
    { id: "nope", state: "OPEN", version: 1, order_ids: [a, b] },
  ])
    assert.throws(() => parseParcelGroupCreated(value), /unavailable/);
});

test("dissolved group is exactly id/state/version", () => {
  const ok = { id: group, state: "DISSOLVED", version: 2 };
  assert.deepEqual(parseParcelGroupDissolved(ok), ok);
  for (const value of [
    { id: group, state: "DISSOLVED", version: 2, order_ids: [a, b] },
    { id: group, state: "OPEN", version: 2 },
    { id: group, state: "DISSOLVED", version: 0 },
  ])
    assert.throws(() => parseParcelGroupDissolved(value), /unavailable/);
});

test("group shipment carries one SHIPPED version per member", () => {
  const ok = {
    id: group,
    state: "SHIPPED",
    version: 2,
    shipments: [
      { order_id: a, shipment },
      { order_id: b, shipment: { ...shipment, note: "n" } },
    ],
  };
  const parsed = parseParcelGroupShipped(ok);
  assert.equal(parsed.shipments.length, 2);
  assert.equal(parsed.shipments[0].shipment.tracking_number, "PARCEL-TRACK-001");
  for (const value of [
    { id: group, state: "OPEN", version: 2, shipments: ok.shipments },
    { id: group, state: "SHIPPED", version: 2, shipments: [ok.shipments[0]] }, // a group never has one member
    { id: group, state: "SHIPPED", version: 2, shipments: [{ order_id: a, shipment: { ...shipment, status: "VOIDED", void_reason: "other" } }, ok.shipments[1]] },
    { id: group, state: "SHIPPED", version: 2, shipments: [ok.shipments[0], ok.shipments[0]] }, // duplicate member
    { id: group, state: "SHIPPED", version: 2, shipments: [{ order_id: a, shipment, extra: 1 }, ok.shipments[1]] },
  ])
    assert.throws(() => parseParcelGroupShipped(value), /unavailable/);
});

test("three locales cover the same keys and every refusal code", () => {
  const flatten = (value: unknown, prefix = ""): string[] =>
    Object.entries(value as Record<string, unknown>).flatMap(([key, item]) =>
      item && typeof item === "object" ? flatten(item, `${prefix}${key}.`) : [`${prefix}${key}`],
    );
  const en = flatten(parcelCopy.en).sort();
  for (const locale of ["zh-TW", "zh-CN"] as const)
    assert.deepEqual(flatten(parcelCopy[locale]).sort(), en, locale);
  for (const code of parcelRefusalCodes) {
    assert.ok(parcelCopy.en.errors[code], `en missing ${code}`);
    assert.ok(parcelCopy["zh-TW"].errors[code], `zh-TW missing ${code}`);
    assert.ok(parcelCopy["zh-CN"].errors[code], `zh-CN missing ${code}`);
  }
  // Owner ruling (2026-10-06): the banner states that only parcels merge, never payments or amounts.
  for (const locale of ["en", "zh-TW", "zh-CN"] as const) {
    assert.ok(parcelCopy[locale].rule.length > 10);
    assert.equal(typeof parcelCopy[locale].banner(2), "string");
    assert.equal(typeof parcelCopy[locale].groupTitle("abcd1234"), "string");
  }
});
