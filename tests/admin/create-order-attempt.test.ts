// Purpose: prove immutable money-path retries and coarse privacy-safe unresolved guards.
// Depends on: Node test/assert and create-order-attempt; no transport or React mocks.
// Used by: LC-U3 Node gate; storage failures must fail closed.
import test from "node:test";
import assert from "node:assert/strict";
import {
  OrderAttempt,
  OrderGuard,
} from "../../apps/admin/lib/create-order-attempt.ts";
test("UNKNOWN preserves the exact original bytes and key despite draft edits", () => {
  let keys = 0;
  const attempt = new OrderAttempt(() => `key-${++keys}`);
  const draft = { customer: { name: "synthetic" }, items: [{ quantity: 1 }] };
  const first = attempt.prepare(draft);
  draft.customer.name = "changed";
  assert.equal(attempt.prepare(draft), first);
  assert.equal(keys, 1);
  assert.equal(JSON.parse(first.body).customer.name, "synthetic");
  assert.ok(Object.isFrozen(first));
  attempt.clear();
  assert.notEqual(attempt.prepare(draft).key, first.key);
});
test("navigation guard stores only an opaque boolean and blocks replacement attempts", () => {
  const map = new Map<string, string>();
  const storage = {
    getItem: (k: string) => map.get(k) ?? null,
    setItem: (k: string, v: string) => {
      map.set(k, v);
    },
    removeItem: (k: string) => {
      map.delete(k);
    },
  };
  const guard = new OrderGuard(storage, "store", "c".repeat(64));
  assert.equal(guard.blocked(), false);
  assert.equal(guard.arm(), true);
  assert.deepEqual([...map.values()], ["1"]);
  assert.equal(new OrderGuard(storage, "store", "c".repeat(64)).arm(), false);
  assert.equal(guard.clear(), true);
  assert.equal(guard.blocked(), false);
});
test("unavailable storage never permits an unguarded money write", () => {
  const fail = () => {
    throw Error("storage denied");
  };
  const guard = new OrderGuard(
    { getItem: fail, setItem: fail, removeItem: fail },
    "store",
    "c".repeat(64),
  );
  assert.equal(guard.blocked(), true);
  assert.equal(guard.arm(), false);
  assert.equal(guard.clear(), false);
});

test("post-dispatch auth refusal can follow commit and must retain the order guard", async () => {
  const { retainOrderAttempt } =
    await import("../../apps/admin/lib/create-order-attempt.ts");
  for (const status of [401, 403, 503])
    assert.equal(
      retainOrderAttempt(
        { status, uncertain: status === 503, code: "forbidden" },
        false,
      ),
      true,
    );
  assert.equal(
    retainOrderAttempt(
      { status: 422, uncertain: false, code: "invalid_request" },
      true,
    ),
    true,
  );
  assert.equal(
    retainOrderAttempt(
      { status: 422, uncertain: false, code: "invalid_request" },
      false,
    ),
    false,
  );
});

test("a definitive response unlocks only the explicit new-attempt action", () => {
  const attempt = new OrderAttempt(() => crypto.randomUUID());
  const first = attempt.prepare({ customer: { name: "original" } });
  assert.equal(attempt.startNew(), false, "UNKNOWN cannot be replaced");
  assert.equal(attempt.prepare({ customer: { name: "edited" } }), first);
  attempt.finish();
  assert.equal(attempt.resolved(), true);
  assert.equal(
    attempt.prepare({ customer: { name: "edited" } }),
    first,
    "definitive response alone never replaces receipt",
  );
  assert.equal(attempt.startNew(), true);
  const next = attempt.prepare({ customer: { name: "edited" } });
  assert.notEqual(next.key, first.key);
  assert.equal(JSON.parse(next.body).customer.name, "edited");
});
test("attempt_closed retains its guard even on the first reply until authoritative recovery", async () => {
  const { retainOrderAttempt } = await import("../../apps/admin/lib/create-order-attempt.ts");
  for (const previouslyUncertain of [false, true])
    assert.equal(retainOrderAttempt({ status: 409, code: "attempt_closed", uncertain: false }, previouslyUncertain), true);
});

test("a coded definitive refusal settles an UNKNOWN replay without relaxing ambiguous replies", async () => {
  const { retainOrderAttempt } = await import("../../apps/admin/lib/create-order-attempt.ts");
  for (const outcome of [
    { status: 409, code: "insufficient_inventory" },
    { status: 409, code: "cod_surcharge_changed" },
    { status: 422, code: "max_per_order_exceeded" },
    { status: 422, code: "pay_at_pickup_amount_exceeds" },
    { status: 422, code: "cash_on_delivery_amount_exceeds" },
    { status: 429, code: "pay_at_pickup_limit" },
    { status: 429, code: "cash_on_delivery_limit" },
  ]) assert.equal(retainOrderAttempt({ ...outcome, uncertain: false }, true), false, outcome.code);
  for (const outcome of [
    { status: 0, code: "unavailable", uncertain: true },
    { status: 400, code: "invalid_json", uncertain: false },
    { status: 415, code: "json_required", uncertain: false },
    { status: 409, code: "insufficient_inventory", uncertain: true },
    { status: 503, code: "insufficient_inventory", uncertain: false },
    { status: 401, code: "unauthorized", uncertain: false },
    { status: 403, code: "forbidden", uncertain: false },
    { status: 409, code: "idempotency_conflict", uncertain: false },
    { status: 409, code: "unknown_code", uncertain: false },
    { status: 422, code: "insufficient_inventory", uncertain: false },
    { status: 422, code: "invalid_request", uncertain: false },
    { status: 422, code: "bank_transfer_unavailable", uncertain: false },
    { status: 409, code: "bundle_buyer_mismatch", uncertain: false },
    { status: 409, code: "capability", uncertain: false },
  ]) assert.equal(retainOrderAttempt(outcome, true), true, `${outcome.status}/${outcome.code}`);
  for (const previouslyUncertain of [false, true]) for (const outcome of [
    { status: 409, code: "conflict" }, { status: 404, code: "not_found" },
  ]) assert.equal(retainOrderAttempt({ ...outcome, uncertain: false }, previouslyUncertain), true, "post-commit generic errors retain even on first response");
});

test("legacy CSRF guard is retained fail-closed across a different stable principal scope", () => {
  const oldKey = "buyer-order-unresolved:store:old-csrf-digest";
  const storage = { [oldKey]: "1", getItem: (key: string) => key === oldKey ? "1" : null, setItem() {}, removeItem() {} };
  const guard = new OrderGuard(storage, "store", "c".repeat(64));
  assert.equal(guard.blocked(), true, "a legacy flag cannot be assigned to a new principal or silently discarded");
  assert.equal(guard.arm(), false);
  assert.equal(guard.clear(), false, "clearing this principal's guard cannot erase an unattributed legacy receipt");
});
test("authoritative recovery clears only this principal and unattributed legacy flags in its store", () => {
  // Match browser Storage's enumerable keys rather than a non-enumerable Map stand-in.
  const own = `buyer-order-unresolved:v2:store:${"c".repeat(64)}`, other = `buyer-order-unresolved:v2:store:${"d".repeat(64)}`;
  const foreign = `buyer-order-unresolved:v2:foreign:${"c".repeat(64)}`, legacy = "buyer-order-unresolved:store:old-csrf-digest";
  const foreignLegacy = "buyer-order-unresolved:foreign:old-csrf-digest";
  const storage: any = {
    [own]: "1", [other]: "1", [foreign]: "1", [legacy]: "1", [foreignLegacy]: "1",
    getItem(key: string) { return Object.hasOwn(this, key) ? this[key] : null; },
    setItem(key: string, value: string) { this[key] = value; }, removeItem(key: string) { delete this[key]; },
  };
  const guard = new OrderGuard(storage, "store", "c".repeat(64));
  assert.equal(guard.clear(), false, "ordinary UX/definitive-response cleanup cannot release a legacy unresolved flag");
  assert.equal(storage.getItem(legacy), "1");
  storage.setItem(own, "1");
  assert.equal(guard.authoritativeRecovered(), true, "only successful server reconciliation releases this store's unattributed flag");
  assert.equal(storage.getItem(own), null); assert.equal(storage.getItem(legacy), null);
  for (const preserved of [other, foreign, foreignLegacy]) assert.equal(storage.getItem(preserved), "1");
});
test("authoritative recovery guard cleanup fails closed if Storage refuses a removal", () => {
  const guard = new OrderGuard({ getItem() { return "1"; }, setItem() {}, removeItem() { throw new Error("denied"); } }, "store", "c".repeat(64));
  assert.equal(guard.authoritativeRecovered(), false);
});
