// Purpose: reproduce SU09-POLL (#341) when payment closes before commercial cancellation arrives.
// Depends on: the actual OrderPayment.tsx predicates, typescript-api and Node test/assert/fs/vm.
// Used by: scripts/dev/test-node.sh (storefront Node suite) in GitHub CI.
// Invariants: execute production predicates without replacing their logic; preserve polling bounds and terminal cases.
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { runInNewContext } from "node:vm";
import ts from "typescript-api";

const source = readFileSync(new URL("../components/OrderPayment.tsx", import.meta.url), "utf8");
const ast = ts.createSourceFile("OrderPayment.tsx", source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
const names = ["terminalPayment", "activeAttempt", "POLL_STEPS_MS", "POLL_WINDOW_MS"];
const declarations = names.map((name) => {
  const statement = ast.statements.find((node) =>
    ts.isVariableStatement(node) && node.declarationList.declarations.some((declaration) =>
      ts.isIdentifier(declaration.name) && declaration.name.text === name));
  assert.ok(statement, `production declaration ${name} must exist`);
  return statement.getText(ast);
});
// Extract only the real pure declarations: no React/browser stubs or copied predicate implementation.
const { activeAttempt, POLL_STEPS_MS, POLL_WINDOW_MS } = runInNewContext(ts.transpileModule(
  `${declarations.join("\n")}\n;({ ${names.join(", ")} });`,
  { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS } },
).outputText);

const closed = {
  payment_state: "CLOSED_UNPAID",
  commercial_state: "AWAITING_PAYMENT",
  cancel_requested: true,
};

test("SU09-POLL: CLOSED_UNPAID keeps polling until asynchronous commercial cancellation arrives", () => {
  assert.equal(activeAttempt(closed), true,
    "payment closure before commercial cancellation must not stop GET polling");
});

test("SU09-POLL: CLOSED_UNPAID stops polling once the commercial order is terminal", () => {
  for (const commercial_state of ["CANCELLED", "CONFIRMED"]) {
    assert.equal(activeAttempt({ ...closed, commercial_state }), false, commercial_state);
  }
});

test("SU09-POLL: existing payment terminal cases remain inactive", () => {
  for (const [payment_state, commercial_state] of [
    ["CAPTURED", "CONFIRMED"],
    ["PARTIALLY_REFUNDED", "CONFIRMED"],
    ["REFUNDED", "CONFIRMED"],
    ["REVIEW_REQUIRED", "AWAITING_PAYMENT"],
  ]) {
    assert.equal(activeAttempt({ ...closed, payment_state, commercial_state }), false, payment_state);
  }
});

test("SU09-POLL: prepared pending attempts and the Stripe presence fence are unchanged", () => {
  assert.equal(activeAttempt({ ...closed, payment_state: "PENDING", cancel_requested: false }), true);
  assert.equal(activeAttempt({ ...closed, payment_state: "PENDING", cancel_requested: undefined }), false);
  assert.equal(activeAttempt({ ...closed, cancel_requested: undefined }), false);
});

test("SU09-POLL: polling step delays and ten-minute window are unchanged", () => {
  assert.deepEqual(Array.from(POLL_STEPS_MS), [5000, 10000, 20000, 30000]);
  assert.equal(POLL_WINDOW_MS, 10 * 60 * 1000);
});
