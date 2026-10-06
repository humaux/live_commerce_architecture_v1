// Purpose: adversarial contracts for pick selection, wire projection and exact BFF grammar.
// Depends on: node:test and the picklist pure model; no database or browser.
// Used by: test-node.sh and W3-U1b red-to-green acceptance.
import test from "node:test";
import assert from "node:assert/strict";
import {
  cvsRecoveryResolved,
  validSelection,
  selectOrders,
  parsePickList,
  parseCvsBatch,
  validPickRequest,
  carrierFilename,
} from "../../apps/admin/lib/picklist-model.ts";
const id = (n: number) =>
  `11111111-1111-4111-8111-${String(n).padStart(12, "0")}`;
const line = {
  sku_id: id(9),
  sku_code: "SKU-9",
  title: "合成商品",
  option_label: null,
  qty: 2,
};
const pick = {
  generated_at: "2026-10-06T06:00:00Z",
  orders: [
    {
      order_id: id(1),
      order_number: `LC-${id(1).replaceAll("-", "").toUpperCase()}`,
      lines: [line],
    },
  ],
  totals: [line],
  skipped: [],
};
test("selection has XOR bodies, canonical unique IDs and separate 500/100 limits", () => {
  assert.equal(
    validSelection({ order_ids: Array.from({ length: 500 }, (_, i) => id(i)) }),
    true,
  );
  for (const b of [
    {},
    { order_ids: [] },
    { order_ids: [id(1), id(1)] },
    { order_ids: [id(1)], session_id: id(2) },
    { session_id: id(1), tenant_id: id(2) },
    { order_ids: ["bad"] },
    { order_ids: Array.from({ length: 501 }, (_, i) => id(i)) },
  ])
    assert.equal(validSelection(b), false);
  assert.equal(validSelection({ session_id: id(1) }), true);
  assert.equal(validSelection({ session_id: id(1) }, 100), false);
  assert.equal(
    validSelection(
      { order_ids: Array.from({ length: 101 }, (_, i) => id(i)) },
      100,
    ),
    false,
  );
});
test("cross-page selection retains existing rows, deduplicates and refuses overflow atomically", () => {
  const a = selectOrders(
    [],
    Array.from({ length: 490 }, (_, i) => id(i)),
    true,
  );
  const b = selectOrders(
    a,
    Array.from({ length: 10 }, (_, i) => id(i + 490)),
    true,
  );
  assert.equal(b.length, 500);
  assert.throws(() => selectOrders(b, [id(600)], true));
  assert.deepEqual(selectOrders(b, [id(0)], false), b.slice(1));
  assert.equal(selectOrders(b, [id(1)], true).length, 500);
});
test("pick projection rejects PII/extra keys, invalid quantities and unmatched order identities", () => {
  assert.deepEqual(parsePickList(pick, { order_ids: [id(1)] }), pick);
  assert.throws(() =>
    parsePickList({ ...pick, recipient: "secret" }, { order_ids: [id(1)] }),
  );
  assert.throws(() =>
    parsePickList(
      { ...pick, totals: [{ ...line, qty: 0 }] },
      { order_ids: [id(1)] },
    ),
  );
  assert.throws(() => parsePickList(pick, { order_ids: [id(2)] }));
  assert.throws(() =>
    parsePickList(
      { ...pick, orders: [{ ...pick.orders[0], order_number: "LC-WRONG" }] },
      { order_ids: [id(1)] },
    ),
  );
  assert.deepEqual(
    parsePickList(
      {
        ...pick,
        orders: [],
        totals: [],
        skipped: [{ order_id: id(1), code: "not_pickable" }],
      },
      { order_ids: [id(1)] },
    ).orders,
    [],
  );
});
test("batch replies bind one-for-one to confirmed orders and preserve uncertainty code", () => {
  const value = {
    results: [
      { order_id: id(1), outcome: "queued" },
      { order_id: id(2), outcome: "failed", code: "retry" },
    ],
  };
  assert.deepEqual(parseCvsBatch(value, [id(1), id(2)]), value);
  for (const v of [
    { results: [] },
    { results: [value.results[0], value.results[0]] },
    { results: [{ order_id: id(1), outcome: "unknown" }] },
    { results: [{ order_id: id(1), outcome: "queued", phone: "secret" }] },
  ])
    assert.throws(() => parseCvsBatch(v, [id(1), id(2)]));
});
test("pick/export POSTs reject keys and all query variants except one exact template", () => {
  const req = (path: string, key?: string) =>
    new Request(`https://admin.example.invalid/${path}`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...(key ? { "Idempotency-Key": key } : {}),
      },
      body: JSON.stringify({ order_ids: [id(1)] }),
    });
  assert.equal(validPickRequest("pick-list", req("orders/pick-list")), true);
  assert.equal(
    validPickRequest("pick-list", req("orders/pick-list", "abcd1234")),
    false,
  );
  for (const query of [
    "",
    "?",
    "?template=evil",
    "?template=generic&x=1",
    "?template=generic&template=generic",
  ])
    assert.equal(
      validPickRequest("export", req("orders/export" + query)),
      false,
    );
  assert.equal(
    validPickRequest("export", req("orders/export?template=generic")),
    true,
  );
  assert.equal(
    validPickRequest("cvs-batch", req("shipments/cvs-batch", "abcd1234")),
    true,
  );
  assert.equal(
    validPickRequest("cvs-batch", req("shipments/cvs-batch")),
    false,
  );
});
test("CSV filename binds to selected template and store; no path/header injection", () => {
  const h = new Headers({
    "Content-Type": "text/csv; charset=utf-8",
    "Cache-Control": "no-store, private",
    "Content-Disposition":
      'attachment; filename="generic-11111111-202610060600.csv"',
  });
  assert.equal(
    carrierFilename(h, "generic", id(1)),
    "generic-11111111-202610060600.csv",
  );
  assert.throws(() => carrierFilename(h, "black_cat", id(1)));
  h.set("Content-Disposition", 'attachment; filename="../orders.csv"');
  assert.throws(() => carrierFilename(h, "generic", id(1)));
});

test("MOCK browser fixture list matches the incumbent closed order parser", async () => {
  const { pickFixture, storeID } = await import("./picklist-fixture.mjs");
  const { parseOrderListV2 } =
    await import("../../apps/admin/lib/orders-v2.ts");
  const f = await pickFixture();
  try {
    const response = await fetch(
      `${f.origin}/v1/admin/stores/${storeID}/orders?view=v2`,
      { headers: { Authorization: `Bearer ${f.token}` } },
    );
    assert.equal(parseOrderListV2(await response.json()).items.length, 10);
  } finally {
    await f.close();
  }
});

test("recovery never clears an unknown or missing current attempt", () => {
  const state = (value: string) => ({
    current_attempt: 1,
    attempts: [{ attempt: 1, state: value }],
  });
  assert.equal(cvsRecoveryResolved([]), false);
  assert.equal(cvsRecoveryResolved([null]), false);
  assert.equal(
    cvsRecoveryResolved([state("CREATED"), state("UNKNOWN")]),
    false,
  );
  assert.equal(
    cvsRecoveryResolved([
      { current_attempt: 2, attempts: [{ attempt: 1, state: "CREATED" }] },
    ]),
    false,
  );
  assert.equal(
    cvsRecoveryResolved([state("REQUESTED"), state("CREATED")]),
    true,
  );
});

test("recovery and order-detail MOCK fixtures satisfy real read parsers", async () => {
  const { pickFixture, storeID, orderID } =
    await import("./picklist-fixture.mjs");
  const { parseCvsShipment } =
    await import("../../apps/admin/lib/logistics-model.ts");
  const { parseOrderDetail } =
    await import("../../apps/admin/lib/orders-model.ts");
  const f = await pickFixture();
  const headers = { Authorization: `Bearer ${f.token}` };
  try {
    for (const state of ["unknown", "created"]) {
      f.state.recovery = state;
      const r = await fetch(
        `${f.origin}/v1/admin/stores/${storeID}/orders/${orderID(501)}/cvs-shipment`,
        { headers },
      );
      const value = parseCvsShipment(await r.json());
      assert.equal(cvsRecoveryResolved([value]), state === "created");
    }
    const r = await fetch(
      `${f.origin}/v1/admin/stores/${storeID}/orders/${orderID(501)}`,
      { headers },
    );
    assert.equal(
      parseOrderDetail(await r.json(), orderID(501)).items[0].name,
      "Synthetic frozen item",
    );
  } finally {
    await f.close();
  }
});
