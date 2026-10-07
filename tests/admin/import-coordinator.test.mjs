// Purpose: exercise actual import form callbacks/effects for immutable replay, original request mapping and lifecycle uncertainty.
// Depends on: TypeScript AST, synthetic React hooks, actual import copy/field definitions and Node File/AbortController.
// Used by: W5-U1 deterministic MOCK regressions; does not replace genuine browser clicks or Go persistence checks.
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { runInNewContext } from "node:vm";
import ts from "typescript-api";
import { importCopy } from "../../apps/admin/lib/import-copy.ts";
import { importViewCopy } from "../../apps/admin/lib/import-view-copy.ts";
import { importFields, requiredImportFields } from "../../apps/admin/lib/import-model.ts";

const source = readFileSync(new URL("../../apps/admin/components/ImportWizard.tsx", import.meta.url), "utf8");
const ast = ts.createSourceFile("ImportWizard.tsx", source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
const form = ast.statements.find(node => ts.isFunctionDeclaration(node) && node.name?.text === "ImportForm");
assert.ok(form);
const compiled = ts.transpileModule("export " + form.getText(ast), { compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX } }).outputText;
const tick = () => new Promise(resolve => setImmediate(resolve));
const preview = { file_sha256: "a".repeat(64), headers: ["ref", "label"], mapping: { external_id: "ref", name: "label" },
  rows_total: 1, new_rows: 1, update_rows: 0, apply_rows: 1, failed_rows: 0, erased_rows: 0, consent_ignored_rows: 0,
  rows: [{ row: 2, outcome: "created" }] };
const receipt = { batch_id: "abcdef11-1111-4111-8111-111111111111", created: 1, updated: 0, failed: 0, replayed: false };

function harness(commit) {
  const slots = [], effects = [], calls = [], auth = [];
  let cursor = 0, unconfirmed = 0;
  const hooks = {
    useState(initial) { const index = cursor++; slots[index] ??= { value: initial }; return [slots[index].value, value => {
      slots[index].value = typeof value === "function" ? value(slots[index].value) : value;
    }]; },
    useRef(initial) { const index = cursor++; return slots[index] ??= { current: initial }; },
    useEffect(effect, dependencies) { const index = cursor++; if (!slots[index]) { slots[index] = { dependencies }; effects.push(() => { slots[index].cleanup = effect(); }); } },
  };
  const module = { exports: {} }, jsx = (type, props) => ({ type, props });
  runInNewContext(compiled, { module, exports: module.exports, ...hooks, AbortController, File, Set, Object,
    importCopy, importViewCopy, importFields, requiredImportFields, pageHidden: () => false,
    document: { visibilityState: "visible" },
    ImportColumnMapping: "mapping", ImportPreviewCounts: "counts", ImportVerdictTable: "verdicts",
    readImportHeader: async () => ["ref", "label"],
    guessImportMapping: () => ({ external_id: "ref", name: "label", phone: "" }),
    sendImport: async args => { calls.push(args); return args.action === "preview" ? { kind: "preview", value: preview } : commit(args); },
    require: name => { if (name === "react/jsx-runtime") return { jsx, jsxs: jsx }; throw Error("unexpected import " + name); },
  });
  const props = { locale: "en", store: "store", boundary: "scope", onSignedOut: value => auth.push(value), onUnconfirmed: () => { unconfirmed++; } };
  let tree;
  function render() { cursor = 0; tree = module.exports.ImportForm(props); for (const effect of effects.splice(0)) effect(); return tree; }
  function nodes() { const all = []; function visit(node) { if (!node || typeof node !== "object") return; all.push(node); for (const child of [node.props?.children].flat(Infinity)) visit(child); } visit(tree); return all; }
  const control = id => { const node = nodes().find(node => node.props?.["data-testid"] === id); assert.ok(node, id); return node; };
  async function click(id) { control(id).props.onClick(); await tick(); render(); }
  async function ready() {
    render(); await click("import-type-customers");
    const file = new File(["\uFEFFref,label\r\nsynthetic-id,DO_NOT_SHOW_CELL\r\n"], "synthetic.csv", { type: "text/csv" });
    control("import-file").props.onChange({ target: { files: [file] } }); await tick(); render();
    await click("import-preview"); await click("import-confirm-next"); return file;
  }
  function unmount() { for (const slot of slots) slot?.cleanup?.(); }
  return { render, control, has: id => nodes().some(node => node.props?.["data-testid"] === id), click, ready, calls, auth, unmount, unconfirmed: () => unconfirmed };
}

test("commit uses the original requested mapping, including an empty optional field, not resolved preview metadata", async () => {
  const h = harness(async () => ({ kind: "receipt", value: receipt }));
  const file = await h.ready(); await h.click("import-confirm");
  assert.equal(h.calls.length, 2); const request = h.calls[1];
  assert.equal(request.file, file); assert.equal(request.expectedApplyRows, 1);
  assert.equal(JSON.stringify(request.mapping), JSON.stringify({ external_id: "ref", name: "label", phone: "" }));
  assert.equal(Object.isFrozen(request.mapping), true);
});
test("an UNKNOWN commit remains locked through a later forbidden retry and only explicit same-file success unlocks", async () => {
  let attempts = 0;
  const h = harness(async () => ++attempts === 1 ? { kind: "error", code: "retry_later", uncertain: true }
    : attempts === 2 ? { kind: "error", code: "forbidden", uncertain: false } : { kind: "receipt", value: receipt });
  const file = await h.ready(); await h.click("import-confirm");
  assert.equal(h.control("import-restart").props.disabled, true); assert.equal(h.calls.length, 2);
  await h.click("import-retry-same");
  assert.equal(h.control("import-restart").props.disabled, true); assert.equal(h.calls.length, 3);
  await h.click("import-retry-same");
  assert.equal(h.control("import-restart").props.disabled, false); assert.equal(h.calls.length, 4);
  for (const request of h.calls.slice(1)) {
    assert.equal(request.file, file); assert.equal(request.mapping, h.calls[1].mapping); assert.equal(request.expectedApplyRows, 1);
  }
});
test("leaving during a dispatched commit drops its payload and keeps an unconfirmed boolean without replay", async () => {
  let resolve;
  const h = harness(() => new Promise(done => { resolve = done; }));
  await h.ready(); h.control("import-confirm").props.onClick(); await tick(); h.render();
  assert.equal(h.calls.length, 2);
  h.unmount(); assert.equal(h.unconfirmed(), 1);
  assert.equal(h.calls[1].signal.aborted, true);
  resolve({ kind: "receipt", value: receipt }); await tick();
  assert.equal(h.calls.length, 2); assert.equal(h.unconfirmed(), 1);
});
test("a definite first preflight refusal clears pending before sign-out and does not invent an unconfirmed import", async () => {
  const h = harness(async () => ({ kind: "error", code: "unauthorized", uncertain: false }));
  await h.ready(); await h.click("import-confirm");
  assert.deepEqual(h.auth, [false]); h.unmount();
  assert.equal(h.unconfirmed(), 0);
});

test("a replayed mismatched receipt ends UNKNOWN without offering another retry or unlocking historical orders", async () => {
  let attempts = 0;
  const confirmed = { ...receipt, created: 2, replayed: true };
  const h = harness(async () => ++attempts === 1 ? { kind: "error", code: "retry_later", uncertain: true }
    : { kind: "terminal", code: "receipt_mismatch", value: confirmed });
  const file = await h.ready(); await h.click("import-confirm");
  assert.equal(h.control("import-restart").props.disabled, true);
  await h.click("import-retry-same");
  assert.equal(h.control("import-terminal-mismatch").props.children, importViewCopy.en.receiptMismatch);
  assert.equal(h.has("import-retry-same"), false, "P2-REPLAY-MISMATCH-TERMINAL: no endless same-receipt retry");
  assert.equal(h.control("import-restart").props.disabled, false);
  assert.equal(h.control("import-type-orders").props.disabled, true);
  assert.equal(h.calls.length, 3);
  assert.equal(h.calls[2].file, file); assert.equal(h.calls[2].mapping, h.calls[1].mapping);
  assert.equal(h.calls[2].expectedApplyRows, 1);
  h.unmount(); assert.equal(h.unconfirmed(), 0);
});
