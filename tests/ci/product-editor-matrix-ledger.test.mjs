// Purpose: Fail when mobile product-editor controls bypass the canonical action ledger.
// Depends on: Node test/assert/fs/vm and installed typescript-api; reads the actual Playwright spec and shared cases.
// Used by: test-node / product-editor matrix coverage gate; no browser, network or PostgreSQL is started.
import test from "node:test";
import assert from "node:assert/strict";
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import path from "node:path";
import { runInNewContext } from "node:vm";
import ts from "typescript-api";

const specPath = "tests/admin/product-editor.acceptance.ts";
let spec = readFileSync(specPath, "utf8");
const startMarker = "// Keep additional products after the frozen list-count checks.";
const endMarker = "// Controlled read-only ambiguity:";
const actions = new Set(["click", "fill", "press", "check", "uncheck", "selectOption", "accept"]);

test("real Playwright ESM consumer collects the product-editor cases", () => {
  // Match brfPlaywright's generated-config seam, without starting any server or browser.
  const unit = "output/matrix-ledger-enforce";
  mkdirSync(unit, { recursive: true });
  const dir = mkdtempSync(path.join(unit, "collection-"));
  const config = path.join(dir, "playwright.config.ts");
  writeFileSync(config, `import { defineConfig } from "@playwright/test"; export default defineConfig({ testDir: ${JSON.stringify(path.resolve("tests/admin"))}, testMatch: ["catalog-core.spec.ts"], reporter: "list" });`);
  try {
    const result = spawnSync("pnpm", ["exec", "playwright", "test", "--list", "--config", config], {
      cwd: process.cwd(), encoding: "utf8", timeout: 60000,
      env: { ...process.env, PRODUCT_EDITOR_ACCEPTANCE: "1", LC_BROWSER_CATALOG_CORE_ACCEPTANCE: "1",
        LC_BROWSER_PUBLIC_ORIGIN: "http://127.0.0.1:1", LC_BROWSER_EVIDENCE: dir,
        LC_BROWSER_STORE: "11111111-1111-4111-8111-111111111111", LC_BROWSER_TAG: "MOCK-cli-collection",
        LC_BROWSER_CONTROL: "http://127.0.0.1:1", LC_BROWSER_CONTROL_KEY: "MOCK-cli-collection-only" },
    });
    assert.ifError(result.error);
    assert.equal(result.status, 0, result.stdout + result.stderr);
    assert.match(result.stdout, /PE12-17/);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});
// Explicit source mutations let the same real-source gate demonstrate RED in a saved command log.
if (process.env.LC_MATRIX_ACTION_MUTATION === "bare-click")
  spec = spec.replace(startMarker, `${startMarker}\nawait page.getByTestId("undeclared-control").click();\n//`);
if (process.env.LC_MATRIX_ACTION_MUTATION === "removed-wrapper")
  spec = spec.replace("await matrixStep(", "await removedMatrixStep(");
if (process.env.LC_MATRIX_ACTION_MUTATION === "extra-control")
  spec = extraControl(spec);
if (process.env.LC_MATRIX_ACTION_MUTATION === "direct-push")
  spec = spec.replace(startMarker, `${startMarker}\nledger.push({ actual: "PASS" });\n//`);
if (process.env.LC_MATRIX_ACTION_MUTATION === "swap-locator") spec = swapLocator(spec);
if (process.env.LC_MATRIX_ACTION_MUTATION === "swap-alias") spec = swapAlias(spec);
if (process.env.LC_MATRIX_ACTION_MUTATION === "swap-field") spec = swapField(spec);
if (process.env.LC_MATRIX_ACTION_MUTATION === "swap-row") spec = swapRow(spec);
if (process.env.LC_MATRIX_ACTION_MUTATION === "swap-role") spec = swapRole(spec);
if (process.env.LC_MATRIX_ACTION_MUTATION === "swap-scope") spec = swapScope(spec);
if (process.env.LC_MATRIX_ACTION_MUTATION === "unknown-locator") spec = unknownLocator(spec);
if (process.env.LC_MATRIX_ACTION_MUTATION === "swap-native-locator") spec = swapNative(spec, 'page.getByTestId("product-save").accept()');
if (process.env.LC_MATRIX_ACTION_MUTATION === "swap-native-method") spec = swapNative(spec, "dialog.check()");
if (process.env.LC_MATRIX_ACTION_MUTATION === "swap-press-key") spec = swapPressKey(spec);

function swapLocator(text) {
  // Change both the operation and its readback, leaving the claimed identity intact.
  const start = text.indexOf(startMarker), end = text.indexOf("const axesStart", start);
  const callback = text.slice(start, end);
  assert.equal(callback.match(/getByTestId\("product-name"\)/g)?.length, 2);
  return text.slice(0, start) + callback.replaceAll('getByTestId("product-name")', 'getByTestId("product-slug")') + text.slice(end);
}
function swapAlias(text) {
  assert.ok(text.includes('const selected = row.getByTestId(`matrix-select-${i}`)'));
  return text.replace('const selected = row.getByTestId(`matrix-select-${i}`)', 'const selected = row.getByTestId(`matrix-active-${i}`)');
}
function swapField(text) {
  assert.ok(text.includes('control: row.getByTestId(`new-price-${i}`),'));
  return text.replace('control: row.getByTestId(`new-price-${i}`),', 'control: row.getByTestId(`matrix-compare-${i}`),');
}
function swapRow(text) {
  assert.ok(text.includes('const selected = row.getByTestId(`matrix-select-${i}`)'));
  return text.replace('const selected = row.getByTestId(`matrix-select-${i}`)', 'const selected = page.getByTestId(`matrix-row-${1 - i}`).getByTestId(`matrix-select-${i}`)');
}
function swapRole(text) {
  const start = text.indexOf('const tracking = row.getByRole("checkbox",', text.indexOf(startMarker));
  assert.ok(start >= 0);
  return text.slice(0, start) + text.slice(start).replace("name: c.untracked", "name: c.active");
}
function swapScope(text) {
  assert.ok(text.includes('const removeColor = page.locator(".product-axis").nth(0).locator(".pe-axis-remove")'));
  return text.replace('const removeColor = page.locator(".product-axis").nth(0).locator(".pe-axis-remove")', 'const removeColor = page.locator(".product-axis").nth(1).locator(".pe-axis-remove")');
}
function unknownLocator(text) {
  return text.replace('const selected = row.getByTestId(`matrix-select-${i}`)', 'const selected = unknownLocatorFactory(row, i)');
}
function swapNative(text, receiver) {
  const start = text.indexOf(startMarker), mobile = text.slice(start);
  assert.equal(mobile.match(/await dialog\.accept\(\);/g)?.length, 1);
  return text.slice(0, start) + mobile.replace("await dialog.accept();", `await ${receiver};`);
}
function swapPressKey(text) {
  const start = text.indexOf(startMarker), mobile = text.slice(start);
  assert.equal(mobile.match(/press\("Escape"\)/g)?.length, 1);
  return text.slice(0, start) + mobile.replace('press("Escape")', 'press("Enter")');
}

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
  try { failures.push(...locatorIdentities(source, start, end)); }
  catch (error) { failures.push(`identity mismatch: unsupported source binding: ${error.message}`); }
  return failures;
}

function locatorIdentities(source, start, end) {
  // Slice only identities, locator expressions and their lexical dependencies from the
  // real spec. Execute its actual loops/object factories against inert locators: no UI,
  // assertions, HTTP, storage or provider actions execute in this source gate.
  const host = ts.createCompilerHost({ noResolve: true });
  host.getSourceFile = (name) => name === specPath ? source : undefined;
  const checker = ts.createProgram([specPath], { noResolve: true }, host).getTypeChecker();
  const needed = new Set(), steps = new Map(), reads = new Map();
  const inRegion = (node) => node.getStart(source) >= start && node.getEnd() <= end;
  const call = (node, name) => ts.isCallExpression(node) && ts.isIdentifier(node.expression) && node.expression.text === name;
  const unwrap = (node) => ts.isAwaitExpression(node) || ts.isParenthesizedExpression(node) ? unwrap(node.expression) : node;
  const walk = (node, visit) => { visit(node); ts.forEachChild(node, (child) => walk(child, visit)); };
  const text = (node) => node.getText(source);
  function pure(node) {
    // This VM is only a bounded evaluator, never an authority/security boundary.
    // Whitelist the current pure locator/data grammar before compiling any slice.
    const methods = new Set(["getByTestId", "getByRole", "locator", "nth", "map", "entries", "now", "toString", "slice"]);
    walk(node, (child) => {
      if (ts.isNewExpression(child) || ts.isAwaitExpression(child) || ts.isSpreadAssignment(child) ||
          ts.isThrowStatement(child) || ts.isTryStatement(child) || ts.isExpressionStatement(child))
        throw new Error(`non-pure locator dependency: ${text(child)}`);
      if (ts.isPropertyAccessExpression(child) && ["constructor", "prototype", "__proto__"].includes(child.name.text))
        throw new Error("unsafe property in locator dependency");
      if (ts.isBinaryExpression(child) && child.operatorToken.kind >= ts.SyntaxKind.FirstAssignment &&
          child.operatorToken.kind <= ts.SyntaxKind.LastAssignment)
        throw new Error("mutable locator dependency is not supported");
      if (ts.isDeleteExpression(child) || ts.isPostfixUnaryExpression(child))
        throw new Error("mutating locator dependency is not supported");
      if (ts.isGetAccessor(child) || ts.isSetAccessor(child) || ts.isMethodDeclaration(child))
        throw new Error("dynamic object behavior is not supported");
      if (ts.isElementAccessExpression(child))
        assert.ok(ts.isIdentifier(child.argumentExpression) || ts.isNumericLiteral(child.argumentExpression), "dynamic property key is not supported");
      if (!ts.isCallExpression(child)) return;
      if (ts.isPropertyAccessExpression(child.expression)) {
        assert.ok(methods.has(child.expression.name.text), `unknown locator/data method: ${text(child.expression)}`);
      } else if (ts.isIdentifier(child.expression) && child.expression.text !== "String") {
        const declarations = checker.getSymbolAtLocation(child.expression)?.declarations ?? [];
        assert.ok(declarations.length === 1 && ts.isVariableDeclaration(declarations[0]) &&
          declarations[0].initializer && ts.isArrowFunction(declarations[0].initializer), `unknown locator factory: ${text(child.expression)}`);
      } else assert.ok(ts.isIdentifier(child.expression), "dynamic locator call is not supported");
    });
  }
  function need(node) {
    pure(node);
    walk(node, (child) => {
      if (!ts.isIdentifier(child)) return;
      for (const declaration of checker.getSymbolAtLocation(child)?.declarations ?? []) {
        let variable = declaration;
        while (ts.isBindingElement(variable) || ts.isArrayBindingPattern(variable) || ts.isObjectBindingPattern(variable)) variable = variable.parent;
        if (ts.isVariableDeclaration(variable)) {
          assert.ok((variable.parent.flags & ts.NodeFlags.Const) || ts.isForStatement(variable.parent.parent), "mutable source locator/data binding is not supported");
        }
        if (!ts.isVariableDeclaration(declaration) || !declaration.initializer || needed.has(declaration)) continue;
        needed.add(declaration);
        need(declaration.initializer);
      }
    });
  }
  function parentStep(node) {
    for (let p = node.parent; p; p = p.parent)
      if (steps.has(p)) return steps.get(p);
  }
  walk(source, (node) => {
    if (inRegion(node) && call(node, "matrixStep")) {
      const callback = node.arguments[1];
      assert.ok(ts.isArrowFunction(callback) || ts.isFunctionExpression(callback), "matrixStep needs an inspectable callback");
      steps.set(callback, { node, operations: [] });
      need(node.arguments[0]);
    }
  });
  walk(source, (node) => {
    if (!inRegion(node) || !ts.isBinaryExpression(node) ||
        node.operatorToken.kind < ts.SyntaxKind.FirstAssignment || node.operatorToken.kind > ts.SyntaxKind.LastAssignment) return;
    const left = text(node.left);
    assert.ok((left === "matrixNativeDialog" && (ts.isArrowFunction(node.right) || text(node.right) === "undefined")) ||
      (left === "acceptedConfirmation" && call(node.right, "matrixStep")), "dynamic assignment may change source locator identity");
  });
  walk(source, (node) => {
    if (!inRegion(node) || !ts.isCallExpression(node)) return;
    const step = parentStep(node), callee = node.expression;
    if (!step) return;
    if (ts.isPropertyAccessExpression(callee) && actions.has(callee.name.text)) {
      step.operations.push(node);
      need(callee.expression);
      if (callee.name.text === "press") {
        assert.equal(node.arguments.length, 1, "press key must be explicit and unambiguous");
        need(node.arguments[0]);
      }
    }
  });
  walk(source, (node) => {
    if (!inRegion(node) || !call(node, "expect")) return;
    const step = parentStep(node), receiver = node.arguments[0];
    if (step && receiver && (!step.operations.length || text(receiver) === "field.control")) {
      // Read-only declarations have no operated locator; bind their actual DOM reads.
      reads.set(node, receiver);
      need(receiver);
    }
  });
  // Conditions and iteration sources are dependencies too. The original lexical scopes,
  // row indices, locale data and field.label/control pairs survive the slice unchanged.
  for (const entry of [...needed, ...[...steps.values()].map((entry) => entry.node), ...reads.keys()]) {
    for (let p = entry.parent; p; p = p.parent) {
      if (ts.isForOfStatement(p) || ts.isForStatement(p) || ts.isIfStatement(p)) {
        if (ts.isForOfStatement(p)) need(p.expression);
        if (ts.isForStatement(p)) { if (p.condition) need(p.condition); if (p.initializer) need(p.initializer); }
        if (ts.isIfStatement(p)) need(p.expression);
      }
    }
  }
  const emitted = new Set();
  function emit(node) {
    if (ts.isBlock(node) || ts.isSourceFile(node)) return node.statements.map(emit).filter(Boolean).join("\n");
    if (ts.isVariableStatement(node)) {
      const declarations = node.declarationList.declarations.filter((d) => needed.has(d));
      return declarations.length ? `const ${declarations.map(text).join(", ")};` : "";
    }
    if (ts.isForOfStatement(node)) {
      const body = emit(node.statement);
      return body ? `for (${text(node.initializer)} of ${text(node.expression)}) {${body}}` : "";
    }
    if (ts.isForStatement(node)) {
      const body = emit(node.statement);
      return body ? `for (${text(node.initializer)}; ${text(node.condition)}; ${text(node.incrementor)}) {${body}}` : "";
    }
    if (ts.isIfStatement(node)) {
      const yes = emit(node.thenStatement), no = node.elseStatement ? emit(node.elseStatement) : "";
      return yes || no ? `if (${text(node.expression)}) {${yes}} else {${no}}` : "";
    }
    if (!ts.isExpressionStatement(node)) return "";
    const expression = unwrap(node.expression);
    if (call(expression, "matrixStep")) return emitStep(expression);
    if (ts.isCallExpression(expression) && ts.isPropertyAccessExpression(expression.expression) && actions.has(expression.expression.name.text)) {
      emitted.add(expression);
      const method = expression.expression.name.text;
      return `__action(${text(expression.expression.expression)}, ${JSON.stringify(method)}${method === "press" ? `, ${text(expression.arguments[0])}` : ""});`;
    }
    const found = [];
    walk(expression, (child) => { if (reads.has(child)) found.push(`__read(${text(reads.get(child))});`); });
    // The real native-dialog handler is assigned rather than invoked in the spec.
    // Trace its body with an inert Dialog, retaining the actual parameter binding.
    if (ts.isBinaryExpression(expression) && ts.isArrowFunction(expression.right)) {
      const body = emit(expression.right.body);
      if (body) return `{ const ${text(expression.right.parameters[0].name)} = __dialog; ${body} }`;
    }
    if (ts.isBinaryExpression(expression) && call(expression.right, "matrixStep")) return emitStep(expression.right);
    return found.join("\n");
  }
  function emitStep(node) {
    emitted.add(node);
    return `__step(${text(node.arguments[0])}, locale, () => {${emit(node.arguments[1].body)}});`;
  }
  // Do not emit setup outside the bounded mobile region.
  const region = source.statements.flatMap(function find(node) {
    if (inRegion(node)) return [node];
    const children = [];
    ts.forEachChild(node, (child) => { children.push(...find(child)); });
    return children;
  });
  const code = ts.transpileModule(region.map(emit).join("\n"), {
    compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS },
  }).outputText;
  for (const { node, operations } of steps.values()) {
    assert.ok(emitted.has(node), "unsupported syntax hides a matrix callback");
    for (const operation of operations) assert.ok(emitted.has(operation), "unsupported syntax hides an operated locator");
  }
  const failures = [], records = [], stack = [];
  const locator = (segments = []) => Object.freeze({
    segments,
    getByTestId: (id) => locator([...segments, ["testId", id]]),
    getByRole: (role, options) => locator([...segments, ["role", role, options]]),
    locator: (css) => locator([...segments, ["css", css]]),
    nth: (i) => locator([...segments, ["nth", i]]),
  });
  const record = (receiver, method, key) => {
    assert.ok(stack.length, "source action must belong to a matrix step");
    assert.ok(receiver && Array.isArray(receiver.segments), "unknown/dynamic source locator");
    stack.at(-1)[method ? "actions" : "reads"].push(receiver.segments);
    if (method) { stack.at(-1).methods.push(method); stack.at(-1).keys.push(key); }
  };
  try {
    runInNewContext(code, {
      page: locator(), productEditorCopy, tag: "MOCK-source-gate",
      __dialog: locator([["nativeDialog"]]),
      __action: record, __read: (receiver) => record(receiver),
      __step: (identity, locale, body) => {
        const entry = { identity, locale, actions: [], reads: [], methods: [], keys: [] };
        stack.push(entry); body(); stack.pop(); records.push(entry);
      },
    }, { timeout: 1000 });
  } catch (error) {
    failures.push(`identity mismatch: cannot resolve actual source locator: ${error.message}`);
    return failures;
  }
  for (const entry of records) {
    try { assertLocatorIdentity(entry); }
    catch (error) { failures.push(`identity mismatch: ${JSON.stringify(entry.identity)} (${entry.locale}): ${error.message}`); }
  }
  for (const locale of Object.keys(productEditorCopy)) {
    const keys = records.filter((record) => record.locale === locale).map(({ identity }) => identityKey(identity));
    const expected = cases.map((entry) => identityKey({ ...entry, control: expand(entry.control, productEditorCopy[locale]) }));
    if (JSON.stringify([...keys].sort()) !== JSON.stringify([...expected].sort()))
      failures.push(`identity mismatch: source declarations do not cover the shared table exactly (${locale}): ${keys.length}/${expected.length}; missing ${expected.filter((key) => !keys.includes(key)).slice(0, 3).join(", ")}`);
  }
  return failures;
}

const identityKey = (entry) => JSON.stringify([entry.page, entry.row ?? "", entry.control, entry.action]);
function assertLocatorIdentity({ identity, locale, actions: operated, reads, methods, keys }) {
  const copy = productEditorCopy[locale], { control, row = "" } = identity;
  const declaration = cases.find((entry) => identityKey({ ...entry, control: expand(entry.control, copy) }) === identityKey(identity));
  assert.ok(declaration, "undeclared identity");
  // Reuse canonical action vocabulary rather than maintaining a per-case method list.
  if (operated.length) {
    const grammar = { fill: ["fill"], "fill Enter": ["fill", "press"], Escape: ["press"],
      "check uncheck": ["check", "uncheck"], "uncheck check": ["uncheck", "check"], uncheck: ["uncheck"], accept: ["accept"] };
    let expectedMethods = grammar[declaration.action];
    if (declaration.action.includes("click")) expectedMethods = ["click"];
    if (declaration.action === "check uncheck; choose mode") {
      expectedMethods = ["check", "uncheck"];
      if (methods.length === 3) expectedMethods.push("check");
    }
    assert.ok(expectedMethods, `unknown canonical action ${declaration.action}`);
    assert.deepEqual(methods, expectedMethods, "operated methods must match the canonical action");
    if (declaration.action === "Escape") assert.equal(keys[0], "Escape", "Escape must press Escape");
    if (declaration.action === "fill Enter") assert.equal(keys[1], "Enter", "committing axis values must press Enter");
  }
  assert.ok(!row || rowIndices.has(row), `unknown matrix row ${row}`);
  const index = rowIndices.get(row) ?? 0;
  const rowScope = [["testId", `matrix-row-${index}`]];
  let expected;
  if (control === "native archive confirmation") {
    expected = [["nativeDialog"]];
  }
  else if (control === "product-save-result/link") expected = [["testId", "product-save-result"], ["role", "link", { name: copy.save, exact: true }]];
  else if (control === ".pe-axis-remove (Color axis 1)") expected = [["css", ".product-axis"], ["nth", 0], ["css", ".pe-axis-remove"]];
  else if (control === copy.untracked) expected = [...rowScope, ["role", "checkbox", { name: copy.untracked, exact: true }]];
  else if (control.startsWith("saved axis-")) expected = [["testId", control.replace("saved axis-", "axis-name-")]];
  else if (control === "saved row readback") expected = [...rowScope, ["css", "strong"]];
  else if (declaration.control.startsWith("@")) {
    const token = declaration.control.slice(1);
    if (token === "keyword") expected = [...rowScope, ["role", "textbox", { name: `${copy.keyword} ${index + 1}`, exact: true }]];
    else {
      const ids = { price: "new-price", compare: "matrix-compare", quantity: "matrix-quantity", max: "matrix-quantity", targetQty: "matrix-quantity", code: "matrix-code" };
      assert.ok(ids[token], `unknown field identity ${token}`);
      expected = [...rowScope, ["testId", `${ids[token]}-${index}`]];
    }
  } else if (/^(matrix-|new-price-)/.test(control)) expected = [...rowScope, ["testId", control]];
  else expected = [["testId", /^axis-add-\d+$/.test(control) ? "axis-add" : control]];
  if (operated.length) {
    for (const receiver of operated) assert.equal(JSON.stringify(receiver), JSON.stringify(expected), `operated ${JSON.stringify(receiver)}; expected ${JSON.stringify(expected)}`);
  } else {
    assert.ok(control.startsWith("saved ") || identity.action === "verify existing SKU code disabled", "declared operation is missing");
    assert.ok(reads.some((receiver) => JSON.stringify(receiver) === JSON.stringify(expected)), `missing declared DOM read ${JSON.stringify(expected)}`);
  }
}

test("every actual mobile control operation is enclosed by its recording callback", () => {
  assert.deepEqual(uncoveredActions(spec), []);
});

for (const [name, mutate] of [["literal", swapLocator], ["lexical alias", swapAlias], ["field factory", swapField],
  ["row coordinate", swapRow], ["role label", swapRole], ["CSS ancestor", swapScope], ["unknown dynamic", unknownLocator]]) {
  test(`a ${name} locator swap cannot certify the original declared control`, () => {
    assert.ok(uncoveredActions(mutate(spec)).some((line) => line.includes("identity mismatch")));
  });
}

test("lexical shadowing and mutable field reassignment fail closed", () => {
  const callbackAction = "await selected.check();";
  assert.ok(spec.includes(callbackAction));
  const shadowed = spec.replace(callbackAction, 'const selected = row.getByTestId(`matrix-active-${i}`);\n' + callbackAction);
  assert.ok(uncoveredActions(shadowed).some((line) => line.includes("identity mismatch")));
  const reassigned = spec.replace("await field.control.fill(field.value);", "{ field.control = row.getByTestId(`matrix-compare-${i}`); await field.control.fill(field.value); }");
  assert.ok(uncoveredActions(reassigned).some((line) => line.includes("identity mismatch")));
});

for (const [name, expression] of [["receiver", 'page.getByTestId("product-save").accept()'], ["method", "dialog.check()"]]) {
  test(`native confirmation cannot substitute its ${name}`, () => {
    assert.ok(uncoveredActions(swapNative(spec, expression)).some((line) => line.includes("identity mismatch")));
  });
}

test("Escape cannot certify another key on the same receiver", () => {
  assert.ok(uncoveredActions(swapPressKey(spec)).some((line) => line.includes("identity mismatch")));
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
// The canonical table supplies row coordinates; no second row/case inventory lives here.
const rowIndices = new Map();
for (const entry of cases) {
  const index = /^(?:matrix-(?:select|active|compare|quantity|code)|new-price)-(\d+)$/.exec(entry.control)?.[1];
  if (!entry.row || index === undefined) continue;
  assert.ok(!rowIndices.has(entry.row) || rowIndices.get(entry.row) === Number(index), `ambiguous canonical row ${entry.row}`);
  rowIndices.set(entry.row, Number(index));
}

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
