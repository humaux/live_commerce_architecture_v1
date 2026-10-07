// Purpose: guard uploaded inbox browser evidence and execute its actual delayed route callbacks on failure.
// Depends on: Node test/assert/fs and installed typescript-api; reads the real Go harness and Playwright spec.
// Used by: LC-U2b DB-free Node gate; Go/browser runtime remains a separate CI gate.
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
const ts = createRequire(import.meta.url)("typescript-api");

const harness = readFileSync("tests/foundation/browser_inbox_ui_test.go", "utf8");
const spec = readFileSync("tests/admin/inbox-ui.spec.ts", "utf8");

test("inbox evidence is covered by the CI upload root", () => {
  assert.ok(/filepath\.Join\(root, "output\/playwright\/inbox-ui",/.test(harness), "inbox evidence must use the uploaded directory");
  const workflow = readFileSync(".github/workflows/gates.yml", "utf8");
  assert.ok(/output\/playwright\//.test(workflow), "workflow must upload Playwright evidence");
});

test("failed Playwright closes its log and emits sanitized diagnostics before Fatalf", () => {
  assert.ok(/runErr := browser\.Run\(\)[\s\S]*?log\.Close\(\)[\s\S]*?if runErr != nil/.test(harness), "close saved evidence before reading failure output");
  assert.ok(/inboxPlaywrightFailureSummary\(output\)/.test(harness), "failure path must use the private summary parser");
  assert.ok(/t\.Logf\("Playwright %s", line\)/.test(harness), "sanitized summary must enter the Go test log");
  assert.ok(/t\.Run\("failure_diagnostics"/.test(harness), "the actual Go parser fixture must run on CI");
});

// Read the parser's actual RE2 patterns. These portable patterns are also verified by the Go subtest on CI.
test("failure diagnostics match INU cases/counts without admitting private text", () => {
  const constants = (name: string) => {
    const value = harness.match(new RegExp("const " + name + " = `([^`]+)`"))?.[1];
    assert.ok(value, `${name} must be an explicit bounded pattern`);
    return new RegExp(value.replace(/^\(\?m\)/, ""), "gm");
  };
  const cases = constants("inboxFailureCasePattern");
  const counts = constants("inboxFailureCountPattern");
  const privateText = "SYNTHETIC_PRIVATE_DM_NAME_PSID";
  const log = `  1) tests/admin/inbox-ui.spec.ts:450:1 › INU05 hidden thread ${privateText} INU99 inbox-ui.spec.ts:999:9 › INU98\n` +
    `    Error: ${privateText}\n    1 failed ${privateText}\n    7 passed (2m)\n` +
    `  2) tests/admin/other.spec.ts:8:1 › INU99 ${privateText}\n` +
    `  3) tests/admin/inbox-bundle-ui.spec.ts:32:1 › INU09 ${privateText}\n`;
  assert.deepEqual([...log.matchAll(cases)].map((m) => m.slice(1)), [["450", "1", "INU05"], ["32", "1", "INU09"]]);
  assert.deepEqual([...log.matchAll(counts)].map((m) => m.slice(1)), [["1", "failed"], ["7", "passed"]]);
});

const source = ts.createSourceFile("inbox-ui.spec.ts", spec, ts.ScriptTarget.Latest, true);
const delayedHandlers: Array<{ getText: (source: unknown) => string }> = [];
function visit(node: any): void {
  if (ts.isCallExpression(node) && node.expression.getText(source) === "test" &&
    node.arguments[0]?.getText(source).includes("INU04")) {
    const findRoutes = (child: any): void => {
      if (ts.isCallExpression(child) && child.expression.getText(source) === "page.route") {
        const callback = child.arguments[1];
        assert.ok(callback && ts.isArrowFunction(callback));
        delayedHandlers.push(callback);
      }
      ts.forEachChild(child, findRoutes);
    };
    ts.forEachChild(node, findRoutes);
  } else ts.forEachChild(node, visit);
}
visit(source);
assert.equal(delayedHandlers.length, 2, "both INU04 stale-response route barriers are exercised");

for (const [index, callback] of delayedHandlers.entries()) {
  test(`INU04 delayed handler ${index + 1} signals completion when fulfillment rejects`, async () => {
    let completed = 0;
    const code = ts.transpileModule(`const handler = ${callback.getText(source)}; return handler;`, {
      compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext },
    }).outputText;
    const handler = new Function("held", "seen", "delivered", "switchHold", "arrived", "done",
      code)(Promise.resolve(), () => {}, () => completed++, Promise.resolve(), () => {}, () => completed++);
    const aborted = new Error("MOCK_ABORTED_INTERCEPTED_REQUEST");
    await assert.rejects(handler({ fetch: async () => ({}), fulfill: async () => { throw aborted; } }),
      (error: unknown) => error === aborted, "the original error must still fail the scenario");
    assert.equal(completed, 1, "the waiter must wake even after request cancellation");
  });
}
