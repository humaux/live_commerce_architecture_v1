// Purpose: Fail when mobile product-editor controls bypass the canonical action ledger.
// Depends on: Node test/assert/fs/vm and installed typescript-api; reads the actual Playwright spec and shared cases.
// Used by: test-node / product-editor matrix coverage gate; no browser, network or PostgreSQL is started.
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import ts from "typescript-api";

const specPath = "tests/admin/product-editor.acceptance.ts";
let spec = readFileSync(specPath, "utf8");
const startMarker = "// Keep additional products after the frozen list-count checks.";
const endMarker = "// Controlled read-only ambiguity:";
const actions = new Set(["click", "fill", "press", "check", "uncheck", "selectOption", "accept"]);
// Explicit source mutations let the same real-source gate demonstrate RED in a saved command log.
if (process.env.LC_MATRIX_ACTION_MUTATION === "bare-click")
  spec = spec.replace(startMarker, `${startMarker}\nawait page.getByTestId("undeclared-control").click();\n//`);
if (process.env.LC_MATRIX_ACTION_MUTATION === "removed-wrapper")
  spec = spec.replace("await matrixStep(", "await removedMatrixStep(");
if (process.env.LC_MATRIX_ACTION_MUTATION === "extra-control")
  spec = extraControl(spec);
if (process.env.LC_MATRIX_ACTION_MUTATION === "direct-push")
  spec = spec.replace(startMarker, `${startMarker}\nledger.push({ actual: "PASS" });\n//`);

function extraControl(text) {
  const source = ts.createSourceFile(specPath, text, ts.ScriptTarget.Latest, true);
  let insertAt;
  function visit(node) {
    if (insertAt === undefined && node.getStart(source) > text.indexOf(startMarker) &&
        ts.isExpressionStatement(node) && node.getText(source).replace(/\s+/g, "").startsWith('awaitpage.getByTestId("product-name").fill('))
      insertAt = node.getEnd();
    ts.forEachChild(node, visit);
  }
  visit(source);
  assert.ok(insertAt !== undefined, "mutation targets the actual mobile product-name fill");
  return text.slice(0, insertAt) + '\nawait page.getByTestId("unrecorded-second-control").click();' + text.slice(insertAt);
}

function uncoveredActions(text) {
  const start = text.indexOf(startMarker), end = text.indexOf(endMarker);
  assert.ok(start >= 0 && end > start, "the real mobile acceptance region must exist");
  const source = ts.createSourceFile(specPath, text, ts.ScriptTarget.Latest, true);
  assert.equal(source.parseDiagnostics.length, 0, "the real spec must parse");
  const failures = [];
  const receivers = new Map();
  function recordingCallback(node) {
    for (let p = node.parent; p; p = p.parent) {
      if ((ts.isArrowFunction(p) || ts.isFunctionExpression(p)) &&
          ts.isCallExpression(p.parent) && ts.isIdentifier(p.parent.expression) &&
          p.parent.expression.text === "matrixStep" && p.parent.arguments[1] === p) return p;
    }
    return undefined;
  }
  function visit(node) {
    if (node.getStart(source) >= start && node.getEnd() <= end && ts.isCallExpression(node)) {
      const callee = node.expression;
      if (ts.isPropertyAccessExpression(callee) && actions.has(callee.name.text)) {
        const callback = recordingCallback(node);
        if (!callback) failures.push(`${source.getLineAndCharacterOfPosition(node.getStart(source)).line + 1}: ${node.getText(source)}`);
        else {
          const controls = receivers.get(callback) ?? new Set();
          controls.add(callee.expression.getText(source).replace(/\s+/g, ""));
          receivers.set(callback, controls);
        }
      }
      if (ts.isPropertyAccessExpression(callee) && callee.expression.getText(source) === "ledger" && callee.name.text === "push")
        failures.push("mobile ledger.push bypasses the canonical recorder");
    }
    ts.forEachChild(node, visit);
  }
  visit(source);
  for (const controls of receivers.values())
    if (controls.size > 1) failures.push(`one recording callback operates distinct controls: ${[...controls].join(", ")}`);
  return failures;
}

test("every actual mobile control operation is enclosed by its recording callback", () => {
  assert.deepEqual(uncoveredActions(spec), []);
});

test("a new bare click is rejected without running a browser", () => {
  const injected = spec.replace(startMarker, `${startMarker}\nawait page.getByTestId("undeclared-control").click();\n//`);
  assert.ok(uncoveredActions(injected).some((line) => line.includes("undeclared-control")));
});

test("a different control cannot borrow an existing action's recording callback", () => {
  const injected = extraControl(spec);
  assert.ok(uncoveredActions(injected).some((line) => line.includes("distinct controls")));
});

test("a direct ledger push cannot bypass canonical recording", () => {
  const injected = spec.replace(startMarker, `${startMarker}\nledger.push({ actual: "PASS" });\n//`);
  assert.ok(uncoveredActions(injected).some((line) => line.includes("ledger.push")));
});

test("removing the recording wrapper rejects the enclosed real action", () => {
  assert.ok(spec.includes("await matrixStep("), "mutation targets a real action wrapper");
  const injected = spec.replace("await matrixStep(", "await removedMatrixStep(");
  assert.ok(uncoveredActions(injected).length > 0);
});

// Execute the actual TS recorder and actual locale copy data; only UI callbacks are test inputs.
function loadTS(file, require) {
  const exports = {};
  const code = ts.transpileModule(readFileSync(file, "utf8"), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022, esModuleInterop: true },
  }).outputText;
  new Function("require", "exports", code)(require, exports);
  return exports;
}
const cases = JSON.parse(readFileSync("tests/foundation/testdata/product_editor_matrix_cases.json", "utf8"));
const { productEditorCopy } = loadTS("apps/admin/lib/product-editor-copy.ts", () => { throw new Error("unexpected copy dependency"); });
const { createProductEditorMatrixLedger } = loadTS("tests/admin/product-editor-matrix-ledger.ts", (name) => {
  if (name.endsWith("product_editor_matrix_cases.json")) return cases;
  if (name.endsWith("product-editor-copy")) return { productEditorCopy };
  throw new Error(`unexpected recorder dependency ${name}`);
});
const expand = (text, copy) => text.replace(/@([A-Za-z]+)/g, (_, token) => copy[token]);

test("shared table has only complete meaningful unique declarations", () => {
  const identities = new Set();
  for (const entry of cases) {
    assert.deepEqual(Object.keys(entry).sort(), ["action", "control", "expected", "page", "row"]);
    for (const [field, value] of Object.entries(entry)) {
      assert.equal(typeof value, "string");
      if (field !== "row") assert.ok(value.trim().length > 0);
    }
    assert.ok(entry.expected.length > 25, "canonical expectations describe checked behavior");
    const identity = JSON.stringify([entry.page, entry.row, entry.control, entry.action]);
    assert.ok(!identities.has(identity), `duplicate declaration ${identity}`);
    identities.add(identity);
  }
});

for (const locale of ["zh-TW", "zh-CN", "en"]) {
  test(`actual recorder expands every canonical expectation and preserves detailed observations (${locale})`, async () => {
    const ledger = [], recorder = createProductEditorMatrixLedger(ledger, locale), copy = productEditorCopy[locale];
    assert.throws(() => recorder.assertComplete(), /Missing matrix actions/);
    for (const entry of cases) {
      const identity = { ...entry, control: expand(entry.control, copy) };
      const observed = { value: "test input", nested: { retained: true }, original: entry.expected };
      const before = ledger.length;
      await recorder.step(identity, async () => {
        assert.equal(ledger.length, before, "PASS must follow the action and assertion callback");
        return { observed, persistence: "preserved", expected: "cannot override canonical", actual: "FAIL", width: 1 };
      });
      assert.strictEqual(ledger.at(-1).observed, observed);
      assert.equal(ledger.at(-1).expected, expand(entry.expected, copy));
      assert.equal(ledger.at(-1).actual, "PASS");
      assert.equal(ledger.at(-1).width, 390);
      assert.equal(ledger.at(-1).persistence, "preserved");
    }
    assert.equal(ledger.length, cases.length);
    recorder.assertComplete();
    await assert.rejects(recorder.step({ ...cases[0], control: expand(cases[0].control, copy) }, async () => ({ observed: {} })), /Duplicate/);
  });
}

test("failed assertion and missing observation cannot emit a PASS or satisfy completeness", async () => {
  const ledger = [], recorder = createProductEditorMatrixLedger(ledger, "en"), entry = cases.find((entry) => entry.control === "product-name");
  await assert.rejects(recorder.step(entry, async () => { throw new Error("real assertion failed"); }), /real assertion failed/);
  await assert.rejects(recorder.step(entry, async () => ({})), /Missing matrix observation/);
  assert.equal(ledger.length, 0);
  assert.throws(() => recorder.assertComplete(), /Missing matrix actions/);
  let called = false;
  await assert.rejects(recorder.step({ ...entry, control: "undeclared" }, async () => { called = true; return { observed: {} }; }), /Undeclared/);
  assert.equal(called, false, "unexpected actions must fail before operating a control");
  await recorder.step(entry, async () => ({ observed: { recovered: true } }));
  assert.equal(ledger.length, 1, "a failed assertion does not consume the declaration");
});

test("in-flight action cannot masquerade as complete or admit a concurrent duplicate", async () => {
  const ledger = [], recorder = createProductEditorMatrixLedger(ledger, "en"), entry = cases.find((entry) => entry.control === "product-name");
  for (const other of cases) {
    if (other === entry) continue;
    await recorder.step({ ...other, control: expand(other.control, productEditorCopy.en) }, async () => ({ observed: {} }));
  }
  let finish;
  const running = recorder.step(entry, () => new Promise((resolve) => { finish = resolve; }));
  assert.throws(() => recorder.assertComplete(), /Missing matrix actions/);
  await assert.rejects(recorder.step(entry, async () => ({ observed: {} })), /Duplicate/);
  finish({ observed: { completed: true } });
  await running;
  assert.equal(ledger.length, cases.length);
  recorder.assertComplete();
});
