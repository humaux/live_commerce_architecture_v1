// Purpose: public UI projection controls over frozen buyer return boundaries (synthetic data only).
// Depends on: node:test and existing direct TS-import suite pattern. Called by: remote Node runner.
// Invariants: no test execution locally; these controls do not claim browser/persistence acceptance.
import test from "node:test";
import assert from "node:assert/strict";
import { requestProjection, requestListProjection, inputProjection, canWithdraw, resolvesAttempt } from "../components/order-status/return-request-state.ts";
const id = "11111111-1111-4111-8111-111111111111";
const order = "22222222-2222-4222-8222-222222222222";
const sku = "33333333-3333-4333-8333-333333333333";
const at = "2026-10-30T12:00:00Z";
const requested = {
  id, order_id: order, number: "R-11111111111141118111111111111111", status: "requested", version: 1,
  reason: "damaged", note: "測試", rejection_reason: null, return_id: null, return_state: null,
  created_at: at, updated_at: at, lines: [{ sku_id: sku, warehouse_id: sku, quantity: 1 }],
  timeline: [{ status: "requested", at }],
};
test("detail projection preserves facts and refuses foreign IDs, private additions and malformed versions", () => {
  assert.deepEqual(requestProjection(requested, order, id), requested);
  assert.equal(requestProjection(requested, sku, id), null);
  assert.equal(requestProjection(requested, order, sku), null);
  assert.equal(requestProjection({ ...requested, buyer_email: "synthetic@example.invalid" }, order), null);
  assert.equal(requestProjection({ ...requested, version: 2 }, order), null);
  assert.equal(requestListProjection({ items: [{ ...requested, status: "unexpected" }] }, order), null);
  assert.deepEqual(requestListProjection({ items: [requested] }, sku), []);
});
test("valid terminal records retain all closed states and never expose withdrawal", () => {
  const withdrawn = { ...requested, status: "withdrawn", version: 2, timeline: [...requested.timeline, { status: "withdrawn", at }] };
  assert.ok(requestProjection(withdrawn, order));
  assert.equal(canWithdraw(withdrawn), false);
  const approved = { ...requested, status: "approved", version: 2, return_id: sku, return_state: "REGISTERED", timeline: [...requested.timeline, { status: "approved", at }] };
  assert.ok(requestProjection(approved, order));
  assert.equal(canWithdraw(approved), false);
  const rejected = { ...requested, status: "rejected", version: 2, rejection_reason: "<script>synthetic text</script>".repeat(20), timeline: [...requested.timeline, { status: "rejected", at }] };
  assert.ok(requestProjection(rejected, order));
  assert.equal(canWithdraw(rejected), false);
  for (const return_state of ["REGISTERED", "RECEIVED", "INSPECTED", "CLOSED", "CANCELLED"]) {
    assert.ok(requestProjection({ ...approved, return_state }, order));
  }
  assert.equal(requestProjection({ ...approved, return_state: "REFUNDED" }, order), null);
  assert.equal(canWithdraw(requested), true);
  assert.equal(resolvesAttempt("withdraw", requested), false);
  assert.equal(resolvesAttempt("withdraw", withdrawn), true);
  assert.equal(resolvesAttempt("create", requested), true);
});
test("input delegates closed grammar including Unicode codepoints, no controls and server warehouse resolution", () => {
  const input = { reason: "damaged", note: "😀".repeat(1000), lines: [{ sku_id: sku, quantity: 1 }] };
  assert.deepEqual(inputProjection(input), input);
  assert.equal(inputProjection({ ...input, note: "😀".repeat(1001) }), null);
  assert.equal(inputProjection({ ...input, note: "test\n" }), null);
  assert.equal(inputProjection({ ...input, photos: [] }), null);
  assert.equal(inputProjection({ ...input, lines: [{ sku_id: sku, quantity: 0 }] }), null);
  assert.equal(inputProjection({ ...input, lines: [{ sku_id: sku, quantity: 1.5 }] }), null);
  assert.equal(inputProjection({ ...input, lines: [] }), null);
});
