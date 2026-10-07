// W3-U5 Node half (contracts/returns-v1.md §2/§6): the admin returns model must decode exactly what the
// W3-08B Go/SQL routes emit (returns.rma_json projection, returns.list_cancel_refund_gaps) and build exactly the
// strict command bodies those routes accept. Independent of the Next runtime: imports only the pure model.
// Run: node --test --experimental-strip-types tests/admin/returns-model.test.ts
import assert from "node:assert/strict";
import { test } from "node:test";
import {
  cancelOrderBody,
  cancelReasonText,
  inspectBody,
  parseRmaList,
  parseRefundGaps,
  receiveBody,
  registerBody,
  registerReasonText,
  rmaStates,
  type Rma,
} from "../../apps/admin/lib/returns-model.ts";

const RMA_ID = "11111111-1111-4111-8111-111111111111";
const ORDER = "22222222-2222-4222-8222-222222222222";
const SKU = "33333333-3333-4333-8333-333333333333";
const SKU2 = "44444444-4444-4444-8444-444444444444";
const WH = "55555555-5555-4555-8555-555555555555";
const WH2 = "66666666-6666-4666-8666-666666666666";
const T0 = "2026-10-06T02:03:04.123456+00:00";
const T1 = "2026-10-06T03:03:04.123456+00:00";

const line = (change: Record<string, unknown> = {}) => ({
  warehouse_id: WH,
  sku_id: SKU,
  qty_registered: 2,
  qty_received: null,
  qty_restock: null,
  qty_scrap: null,
  ...change,
});
const rma = (state: string, change: Record<string, unknown> = {}, lines: unknown[] = [line()]) => ({
  id: RMA_ID,
  order_id: ORDER,
  state,
  version: 1,
  reason: "damaged: corner crushed",
  refund_id: null,
  created_at: T0,
  updated_at: T0,
  lines,
  ...change,
});

const parses = (value: unknown): Rma[] => parseRmaList({ items: [value] });

test("RMA list decodes every state with the exact rma_json keys", () => {
  assert.deepEqual(parseRmaList({ items: [] }), []);
  const registered = parses(rma("REGISTERED"))[0];
  assert.equal(registered.state, "REGISTERED");
  assert.equal(registered.lines[0].qty_received, null);
  const received = parses(rma("RECEIVED", { version: 2, updated_at: T1 }, [line({ qty_received: 1 })]))[0];
  assert.equal(received.lines[0].qty_received, 1);
  const inspected = parses(
    rma("INSPECTED", { version: 3 }, [line({ qty_received: 2, qty_restock: 1, qty_scrap: 1 })]),
  )[0];
  assert.equal(inspected.lines[0].qty_restock, 1);
  const closed = parses(
    rma("CLOSED", { version: 4, refund_id: ORDER }, [line({ qty_received: 2, qty_restock: 2, qty_scrap: 0 })]),
  )[0];
  assert.equal(closed.refund_id, ORDER);
  parses(rma("CANCELLED", { version: 2 }));
  // the store list route emits the same shape
  const list = parseRmaList({ items: [rma("REGISTERED"), rma("CLOSED", { id: ORDER, version: 4 }, [line({ qty_received: 1, qty_restock: 0, qty_scrap: 1 })])] });
  assert.equal(list.length, 2);
});

test("RMA parser refuses drifted keys, states, quantities and timestamps", () => {
  const bad: unknown[] = [
    rma("REGISTERED", { extra: 1 }),
    { ...rma("REGISTERED"), lines: undefined },
    (() => { const x = rma("REGISTERED") as Record<string, unknown>; delete x.refund_id; return x; })(),
    rma("OPEN"),
    rma("REGISTERED", { version: 0 }),
    rma("REGISTERED", { version: 1.5 }),
    rma("REGISTERED", { reason: "" }),
    rma("REGISTERED", { reason: "x".repeat(241) }),
    rma("REGISTERED", { created_at: "2026-10-06" }),
    rma("REGISTERED", { created_at: "not-a-date" }),
    rma("REGISTERED", { updated_at: "2026-10-05T00:00:00.000000Z" }), // updated before created
    rma("REGISTERED", { order_id: "2222" }),
    // quantities must follow the state machine (§2 CHECKs)
    rma("REGISTERED", {}, [line({ qty_received: 1 })]),
    rma("REGISTERED", {}, [line({ qty_registered: 0 })]),
    rma("RECEIVED", { version: 2 }, [line({ qty_received: 3 })]), // > qty_registered
    rma("RECEIVED", { version: 2 }, [line({})]), // qty_received still null
    rma("RECEIVED", { version: 2 }, [line({ qty_received: 1, qty_restock: 0 })]),
    rma("INSPECTED", { version: 3 }, [line({ qty_received: 2, qty_restock: 1, qty_scrap: 0 })]), // 1+0 != 2
    rma("INSPECTED", { version: 3 }, [line({ qty_received: 2, qty_restock: 3, qty_scrap: -1 })]),
    rma("CLOSED", { version: 4 }, [line({ qty_received: 3, qty_restock: 3, qty_scrap: 0 })]), // received > registered
    rma("CANCELLED", { version: 2 }, [line({ qty_received: 0 })]),
    rma("CLOSED", { version: 4, refund_id: "not-a-uuid" }, [line({ qty_received: 1, qty_restock: 1, qty_scrap: 0 })]),
    rma("REGISTERED", { refund_id: ORDER }), // a refund link only exists after close
    rma("REGISTERED", {}, []), // no lines
    rma("REGISTERED", {}, [line(), line()]), // duplicate (warehouse, sku)
    rma("REGISTERED", {}, [line({ qty_registered: 1e9 + 1 })]), // absurd quantity
  ];
  for (const value of bad) assert.throws(() => parses(value), /unavailable/, JSON.stringify(value).slice(0, 120));
  // the same SKU in two warehouses is a legitimate multi-warehouse line
  assert.equal(parses(rma("REGISTERED", {}, [line(), line({ warehouse_id: WH2 })])).length, 1);
  assert.throws(() => parseRmaList({ items: [rma("REGISTERED")], next_cursor: "" }), /unavailable/);
  assert.throws(() => parseRmaList({ items: rma("REGISTERED") }), /unavailable/);
});

test("refund-gap list decodes the closed cancel_refund_failed shape only", () => {
  const gap = {
    order_id: ORDER,
    captured_minor: 2500,
    refunded_minor: 1000,
    gap_minor: 1500,
    cancelled_at: T0,
    reason: "cancel_refund_failed",
  };
  assert.deepEqual(parseRefundGaps({ items: [gap] }), [{ ...gap, cancelled_at: T0 }]);
  assert.deepEqual(parseRefundGaps({ items: [] }), []);
  for (const value of [
    { ...gap, extra: 1 },
    (() => { const x = { ...gap } as Record<string, unknown>; delete x.reason; return x; })(),
    { ...gap, reason: "other" },
    { ...gap, gap_minor: 1499 },
    { ...gap, refunded_minor: 2500, gap_minor: 0 }, // no gap left: must not be listed
    { ...gap, captured_minor: -1 },
    { ...gap, cancelled_at: "yesterday" },
    { ...gap, order_id: "nope" },
  ])
    assert.throws(() => parseRefundGaps({ items: [value] }), /unavailable/, JSON.stringify(value));
});

test("register body is exactly {reason, lines:[{sku_id, quantity}]} with a composed enum reason", () => {
  assert.equal(registerReasonText("damaged", " corner crushed "), "damaged: corner crushed");
  assert.equal(registerReasonText("buyer_request", ""), "buyer_request");
  assert.equal(registerReasonText("other", "x".repeat(200)), null); // 240-byte server cap, CJK-safe
  const body = registerBody("damaged: crushed", [
    { sku_id: SKU, quantity: 1 },
    { sku_id: SKU2, quantity: 2 },
  ]);
  assert.ok(body);
  assert.deepEqual(JSON.parse(body), {
    reason: "damaged: crushed",
    lines: [
      { sku_id: SKU, quantity: 1 },
      { sku_id: SKU2, quantity: 2 },
    ],
  });
  for (const bad of [
    registerBody("", [{ sku_id: SKU, quantity: 1 }]),
    registerBody("ok", []),
    registerBody("ok", [{ sku_id: SKU, quantity: 0 }]),
    registerBody("ok", [{ sku_id: SKU, quantity: -1 }]),
    registerBody("ok", [{ sku_id: SKU, quantity: 1.5 }]),
    registerBody("ok", [{ sku_id: "nope", quantity: 1 }]),
    registerBody("ok", [
      { sku_id: SKU, quantity: 1 },
      { sku_id: SKU, quantity: 1 },
    ]),
  ])
    assert.equal(bad, null);
});

test("receive body names every line once with 0 <= qty, at least one unit", () => {
  const body = receiveBody(1, [
    { sku_id: SKU, qty_received: 2 },
    { sku_id: SKU2, qty_received: 0 },
  ]);
  assert.ok(body);
  assert.deepEqual(JSON.parse(body!), {
    expected_version: 1,
    lines: [
      { sku_id: SKU, qty_received: 2 },
      { sku_id: SKU2, qty_received: 0 },
    ],
  });
  assert.equal(receiveBody(1, [{ sku_id: SKU, qty_received: 0 }]), null); // nothing_received
  assert.equal(receiveBody(0, [{ sku_id: SKU, qty_received: 1 }]), null);
  assert.equal(receiveBody(1, [{ sku_id: SKU, qty_received: -1 }]), null);
  assert.equal(
    receiveBody(1, [
      { sku_id: SKU, qty_received: 1 },
      { sku_id: SKU, qty_received: 1 },
    ]),
    null,
  );
});

test("inspect body enforces qty_restock + qty_scrap = qty_received per line before send", () => {
  const body = inspectBody(2, [
    { sku_id: SKU, qty_received: 2, qty_restock: 1, qty_scrap: 1 },
  ]);
  assert.ok(body);
  assert.deepEqual(JSON.parse(body!), {
    expected_version: 2,
    lines: [{ sku_id: SKU, qty_restock: 1, qty_scrap: 1 }],
  });
  assert.equal(inspectBody(2, [{ sku_id: SKU, qty_received: 2, qty_restock: 2, qty_scrap: 1 }]), null);
  assert.equal(inspectBody(2, [{ sku_id: SKU, qty_received: 2, qty_restock: -1, qty_scrap: 3 }]), null);
  assert.equal(inspectBody(0, [{ sku_id: SKU, qty_received: 0, qty_restock: 0, qty_scrap: 0 }]), null); // versions start at 1
  assert.equal(inspectBody(1.5, [{ sku_id: SKU, qty_received: 0, qty_restock: 0, qty_scrap: 0 }]), null);
  // a fully scrapped zero-received line is legitimate (0 + 0 = 0)
  assert.ok(inspectBody(2, [{ sku_id: SKU, qty_received: 0, qty_restock: 0, qty_scrap: 0 }]));
});

test("merchant cancel body is exactly {expected_state, reason} with the CJK-safe reason cap", () => {
  assert.equal(cancelReasonText("  買家要求取消  "), "買家要求取消");
  assert.equal(cancelReasonText("   "), null);
  assert.equal(cancelReasonText("取".repeat(61)), null); // 61 CJK chars exceed the 240-byte server cap
  assert.equal(cancelReasonText("取".repeat(60)), "取".repeat(60));
  const body = cancelOrderBody("CONFIRMED", "買家要求取消");
  assert.ok(body);
  assert.deepEqual(JSON.parse(body!), { expected_state: "CONFIRMED", reason: "買家要求取消" });
  assert.equal(cancelOrderBody("CANCELLED", "x"), null);
  assert.equal(cancelOrderBody("MERCHANT_SHIPPED", "x"), null);
  assert.equal(cancelOrderBody("CONFIRMED", ""), null);
});

test("rmaStates is the closed §2 machine", () => {
  assert.deepEqual([...rmaStates], ["REGISTERED", "RECEIVED", "INSPECTED", "CLOSED", "CANCELLED"]);
});
