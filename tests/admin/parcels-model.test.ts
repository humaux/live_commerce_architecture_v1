// Purpose: node unit tests for the W3-07B parcel-group DTO parsers (incl. the W3-U4 OPEN-groups read and its reconcile with the
//   session's panels), the recipient mask, and the three-locale copy parity with the exact owner-ruling sentence.
// Depends on: apps/admin/lib/parcels-model.ts (frozen shapes of internal/httpapi/parcels.go), parcels-copy.ts, orders-copy.ts.
// Used by: scripts/dev/test-local.sh --browser-merchant-orders-ui (pure model gate, no browser/PG).
import assert from "node:assert/strict";
import { test } from "node:test";
import {
  maskRecipient,
  parseMergeSuggestions,
  parseOpenParcelGroups,
  parseParcelGroupCreated,
  parseParcelGroupDissolved,
  parseParcelGroupShipped,
  parcelRefusalCodes,
  reconcileGroups,
  type OpenParcelGroup,
  type ParcelGroupView,
} from "../../apps/admin/lib/parcels-model.ts";
import { parcelCopy } from "../../apps/admin/lib/parcels-copy.ts";
import { ordersCopy } from "../../apps/admin/lib/orders-copy.ts";

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

test("merge suggestions accept the frozen shape, keep only the MASKED recipient and reject drift", () => {
  const ok = { items: [{ recipient_name: "Synthetic Buyer", order_ids: [a, b] }] };
  assert.deepEqual(parseMergeSuggestions(ok), [{ recipient_masked: "S***", order_ids: [a, b] }]);
  assert.ok(!JSON.stringify(parseMergeSuggestions(ok)).includes("Synthetic"), "the full name must not survive parsing");
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

test("recipient mask equals the orders-list rule (first non-blank character + ***, placeholder otherwise)", () => {
  for (const [name, want] of [
    ["王小明", "王***"],
    ["Synthetic Buyer", "S***"],
    ["\u3000\u3000王小明", "王***"], // leading ideographic spaces are skipped, as in migration 0110
    [" \u200b\u00a0Ann", "A***"], // ASCII, zero-width and no-break spaces
    ["😀 smile", "😀***"], // one code point, like SQL left(x,1); still 4 characters long
    ["   ", "—"],
    ["", "—"],
    ["\u0007bell", "—"], // an unprintable initial hides instead of revealing
  ] as const) {
    const masked = maskRecipient(name);
    assert.equal(masked, want, JSON.stringify(name));
    assert.ok(masked === "—" || (Array.from(masked).length === 4 && masked.endsWith("***")));
  }
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

const mask = (n: string) => `${n}***`;
const orderNumber = (id: string) => `LC-${id.replaceAll("-", "").toUpperCase()}`;
const member = (id: string, masked = "王***") => ({ order_id: id, order_number: orderNumber(id), recipient_masked: masked });
const c = "cccccccc-cccc-4ccc-8ccc-cccccccccccc";
const d = "dddddddd-dddd-4ddd-8ddd-dddddddddd01";
const openGroup = (id: string, ids: string[], v = 1) => ({ group_id: id, version: v, created_at: "2026-10-07T10:33:06.307598Z", members: ids.map((i) => member(i)) });

test("open groups (GET parcel-groups) accept the frozen shape and fail closed on drift", () => {
  const ok = { items: [openGroup(group, [a, b], 3), openGroup("ffffffff-ffff-4fff-8fff-ffffffffffff", [c, d])] };
  const parsed = parseOpenParcelGroups(ok);
  assert.equal(parsed.length, 2);
  assert.equal(parsed[0].version, 3);
  assert.deepEqual(parsed[0].members[0], member(a));
  assert.deepEqual(parseOpenParcelGroups({ items: [] }), []);
  assert.deepEqual(parseOpenParcelGroups({ items: [{ ...openGroup(group, [a, b]), created_at: "2026-10-07T18:33:06+08:00" }] }).length, 1);
  assert.equal(parseOpenParcelGroups({ items: [{ ...openGroup(group, [a, b]), members: [member(a, "—"), member(b, mask("A"))] }] }).length, 1);
  const g = openGroup(group, [a, b]);
  const bad: unknown[] = [
    {},
    { items: {} },
    { items: [], extra: 1 },
    { items: [{ ...g, extra: 1 }] },
    { items: [{ group_id: g.group_id, version: 1, members: g.members }] }, // created_at is required
    { items: [{ ...g, version: 0 }] },
    { items: [{ ...g, group_id: "nope" }] },
    { items: [{ ...g, created_at: "yesterday" }] },
    { items: [{ ...g, created_at: "2026-13-45T10:00:00Z" }] },
    { items: [{ ...g, members: [member(a)] }] }, // a group is never one order
    { items: [{ ...g, members: [member(a), member(a)] }] }, // duplicate member
    { items: [{ ...g, members: [member(a), { ...member(b), extra: 1 }] }] },
    { items: [{ ...g, members: [member(a), { ...member(b), order_number: orderNumber(a) }] }] }, // order_number must match the id
    { items: [{ ...g, members: [member(a), member(b, "王小明")] }] }, // a full name is never accepted
    { items: [{ ...g, members: [member(a), member(b, "王小***")] }] },
    { items: [{ ...g, members: [member(a), { ...member(b), recipient_masked: 5 }] }] },
    { items: [g, g] }, // duplicate group
    { items: [g, openGroup(c, [b, d])] }, // an order in two groups
    { items: [{ ...g, members: Array.from({ length: 21 }, (_, i) => member(`${String(i).padStart(8, "0")}-0000-4000-8000-000000000000`)) }] },
  ];
  for (const value of bad) assert.throws(() => parseOpenParcelGroups(value), /unavailable/, JSON.stringify(value).slice(0, 80));
  const many = Array.from({ length: 201 }, (_, i) => openGroup(`${String(i).padStart(8, "0")}-0000-4000-8000-00000000aaaa`, [`${String(i).padStart(8, "0")}-0000-4000-8000-00000000bbbb`, `${String(i).padStart(8, "0")}-0000-4000-8000-00000000cccc`]));
  assert.throws(() => parseOpenParcelGroups({ items: many }), /unavailable/);
  assert.equal(parseOpenParcelGroups({ items: many.slice(0, 200) }).length, 200);
});

test("reconcile: a reload rebuilds OPEN panels, a vanished OPEN group is dropped, history and live versions are kept", () => {
  const view = (id: string, state: ParcelGroupView["state"], version: number, ids: string[]): ParcelGroupView => ({ id, short: id.slice(0, 8), state, version, orderIDs: ids });
  const open: OpenParcelGroup[] = parseOpenParcelGroups({ items: [openGroup(group, [a, b], 4)] });
  // Stranded after reload: nothing local, the server lists one OPEN group -> a usable panel with the LIVE version and members.
  const rebuilt = reconcileGroups([], open);
  assert.deepEqual(rebuilt.map((g) => [g.id, g.state, g.version, g.orderIDs]), [[group, "OPEN", 4, [a, b]]]);
  assert.equal(rebuilt[0].members?.[0].order_number, orderNumber(a));
  // A local OPEN group the server still lists takes the server's version (the dissolve CAS must not use a stale one).
  assert.equal(reconcileGroups([view(group, "OPEN", 1, [a, b])], open)[0].version, 4);
  // Uncertain dissolve that landed: the retry answers group_not_open, the refetch no longer lists the group -> the panel is dropped.
  assert.deepEqual(reconcileGroups([view(group, "OPEN", 4, [a, b])], []), []);
  // Session history (SHIPPED/DISSOLVED) is never touched, listed or not, and is not re-added as OPEN.
  const history = [view(group, "SHIPPED", 2, [a, b]), view(c, "DISSOLVED", 2, [c, d])];
  assert.deepEqual(reconcileGroups(history, open), history);
  assert.deepEqual(reconcileGroups(history, []), history);
  // Known groups keep their place; a group the session did not know is appended in the server's order.
  const other = "ffffffff-ffff-4fff-8fff-ffffffffffff";
  const next = reconcileGroups([view(group, "OPEN", 1, [a, b])], parseOpenParcelGroups({ items: [openGroup(other, [c, d]), openGroup(group, [a, b], 2)] }));
  assert.deepEqual(next.map((g) => g.id), [group, other]);
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
  for (const locale of ["en", "zh-TW", "zh-CN"] as const) {
    assert.equal(typeof parcelCopy[locale].banner(2), "string");
    assert.equal(typeof parcelCopy[locale].groupTitle("abcd1234"), "string");
    assert.ok(parcelCopy[locale].groupsUnavailable.length > 5);
  }
});

// Owner ruling (2026-10-06, brief "Integrator 裁决"): the sentence is pinned verbatim in every locale. A rewording that drops
// "parcels only / no payments or amounts / each order priced and notified on its own" must fail here, not in review.
test("the owner-ruling sentence is exact in all three locales", () => {
  assert.equal(parcelCopy["zh-TW"].rule, "只合併包裹，不合併付款或金額；每張訂單仍各自計價、各自收到出貨通知。");
  assert.equal(parcelCopy["zh-CN"].rule, "只合并包裹，不合并付款或金额；每张订单仍各自计价、各自收到出货通知。");
  assert.equal(
    parcelCopy.en.rule,
    "Only the parcels are merged, never payments or amounts; each order is still priced separately and gets its own shipping notification.",
  );
});

test("the in_parcel_group copy is the same sentence in parcels-copy (blocked + errors) and orders-copy, in every locale", () => {
  for (const locale of ["en", "zh-TW", "zh-CN"] as const) {
    const sentence = ordersCopy[locale].errors.in_parcel_group;
    assert.ok(sentence && sentence.length > 10, `${locale} orders-copy in_parcel_group`);
    assert.equal(parcelCopy[locale].blocked, sentence, `${locale} blocked`);
    assert.equal(parcelCopy[locale].errors.in_parcel_group, sentence, `${locale} errors.in_parcel_group`);
  }
  assert.equal(ordersCopy["zh-TW"].errors.in_parcel_group, "此訂單在合包中，請在合包填寫運單");
});
