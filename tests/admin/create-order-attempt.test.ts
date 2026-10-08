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
  const guard = new OrderGuard(storage, "store", "opaque-session");
  assert.equal(guard.blocked(), false);
  assert.equal(guard.arm(), true);
  assert.deepEqual([...map.values()], ["1"]);
  assert.equal(new OrderGuard(storage, "store", "opaque-session").arm(), false);
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
    "scope",
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
