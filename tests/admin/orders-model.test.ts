import assert from "node:assert/strict";
import { test } from "node:test";
import {
  parseOrderSummary,
  parseOrderList,
  parseOrderDetail,
} from "../../apps/admin/lib/orders-model.ts";

const id = "11111111-1111-4111-8111-111111111111";
const sku = "22222222-2222-4222-8222-222222222222";
const summary = {
  order_id: id,
  created_at: "2026-09-27T00:00:00.000000Z",
  updated_at: "2026-09-27T00:00:00.000000Z",
  currency: "TWD",
  total_minor: 1250,
  commercial_state: "DRAFT",
  fulfillment_state: "MANUAL_UNASSIGNED",
  payment_state: "NOT_STARTED",
  test_mode: false,
  work_state: "NONE",
};
const detail = {
  ...summary,
  country: "TW",
  service_code: "home",
  items: [
    {
      sku_id: sku,
      code: "FROZEN-CODE",
      name: "Frozen item",
      quantity: 1,
      unit_price_minor: 1250,
      amount: {
        subtotal_minor: 1250,
        discount_minor: 0,
        tax_minor: 0,
        total_minor: 1250,
      },
    },
  ],
  totals: {
    subtotal_minor: 1250,
    discount_minor: 0,
    shipping_minor: 0,
    shipping_tax_minor: 0,
    tax_minor: 0,
    total_minor: 1250,
  },
  destination: {
    kind: "home",
    country: "TW",
    recipient_name: "Synthetic Buyer",
    phone: "+886900000001",
    home_address: {
      region: "",
      city: "Synthetic city",
      postal_code: "",
      line1: "Synthetic home address",
      line2: "",
    },
    pickup: null,
  },
};

test("MOU parser accepts exact frozen DTO and rejects malformed identity, money and state", () => {
  assert.equal(parseOrderSummary(summary).order_id, id);
  assert.equal(
    parseOrderList({ items: [summary], next_cursor: "" }).items.length,
    1,
  );
  assert.equal(
    parseOrderDetail(detail, id).destination.recipient_name,
    "Synthetic Buyer",
  );
  const bad = [
    { ...summary, order_id: [id] }, // String([id]) must not become authority.
    { ...summary, currency: ["TWD"] },
    { ...summary, currency: 123 }, // String(123) matches three letters only if coercion is loose.
    { ...summary, total_minor: Number.MAX_SAFE_INTEGER + 1 },
    { ...summary, work_state: "READY", payment_state: "PENDING" },
    { ...summary, test_mode: "false" },
    { ...summary, created_at: "2026-02-30T00:00:00.000000Z" },
  ];
  for (const row of bad)
    assert.throws(
      () => parseOrderSummary(row),
      /unavailable/,
      JSON.stringify(row),
    );
  assert.throws(
    () => parseOrderList({ items: [summary, summary], next_cursor: "" }),
    /unavailable/,
  );
  assert.throws(
    () => parseOrderList({ items: [summary], next_cursor: "x=" }),
    /unavailable/,
  );
});

test("MOU detail parser rejects spoofed recipient, pickup and mismatched money", () => {
  assert.throws(() => parseOrderDetail(detail, sku), /unavailable/);
  for (const change of [
    { country: ["TW"] },
    { service_code: ["home"] },
    { items: [{ ...detail.items[0], sku_id: [sku] }] },
    { items: [{ ...detail.items[0], code: ["FROZEN-CODE"] }] },
    { totals: { ...detail.totals, total_minor: 0 } },
    { destination: { ...detail.destination, phone: "javascript:alert(1)" } },
  ])
    assert.throws(
      () => parseOrderDetail({ ...detail, ...change }, id),
      /unavailable/,
    );
  const pickup = {
    ...detail,
    destination: {
      kind: "cvs_familymart",
      country: "TW",
      recipient_name: "Synthetic Recipient",
      phone: "+886900000002",
      home_address: {
        region: "",
        city: "",
        postal_code: "",
        line1: "",
        line2: "",
      },
      pickup: {
        kind: "cvs_familymart",
        namespace: "fixture.case",
        code: "017888",
        name: "Synthetic pickup",
        address: "Synthetic address",
        verification_kind: "MANUAL_ATTESTED",
      },
    },
  };
  assert.equal(parseOrderDetail(pickup, id).destination.pickup?.code, "017888");
  assert.throws(
    () =>
      parseOrderDetail(
        {
          ...pickup,
          destination: {
            ...pickup.destination,
            pickup: { ...pickup.destination.pickup, code: ["017888"] },
          },
        },
        id,
      ),
    /unavailable/,
  );
});
