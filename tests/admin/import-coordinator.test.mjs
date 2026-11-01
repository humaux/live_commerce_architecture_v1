// Purpose: exercise actual import form callbacks/effects for immutable replay, original request mapping and lifecycle uncertainty.
// Depends on: TypeScript AST, synthetic React hooks, actual import copy/field definitions and Node File/AbortController.
// Used by: W5-U1 deterministic MOCK regressions; does not replace genuine browser clicks or Go persistence checks.
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { runInNewContext } from "node:vm";
import ts from "typescript-api";
import { dataOpsCopy, dataOpsText } from "../../packages/i18n/src/data-ops.ts";
import { importCopy } from "../../apps/admin/lib/import-copy.ts";
import { importViewCopy } from "../../apps/admin/lib/import-view-copy.ts";
import { importFields, requiredImportFields } from "../../apps/admin/lib/import-model.ts";
import { readImportHeader } from "../../apps/admin/lib/import-header.ts";

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

function harness(commit, headerReader = async () => ["ref", "label"], locale = "en", options = {}) {
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
  runInNewContext(compiled, { module, exports: module.exports, ...hooks, AbortController, File, Set, Object, Error,
    dataOpsCopy, dataOpsText, importCopy, importViewCopy, importFields, requiredImportFields, pageHidden: () => false,
    document: { visibilityState: "visible" },
    ImportColumnMapping: "mapping", ImportPreviewCounts: "counts", ImportVerdictTable: "verdicts",
    readImportHeader: headerReader,
    guessImportMapping: kind => kind === "orders" ? (options.orderMapping ?? { order_id: "order", customer_id: "customer", ordered_at: "when", status: "state", total: "total" })
      : (options.requestedMapping ?? { external_id: "ref", name: "label", phone: "" }),
    sendImport: async args => { calls.push(args); return args.action === "preview" ? { kind: "preview", value: options.preview ?? preview } : commit(args); },
    require: name => { if (name === "react/jsx-runtime") return { jsx, jsxs: jsx }; throw Error("unexpected import " + name); },
  });
  const props = { locale, store: "store", boundary: "scope", onSignedOut: value => auth.push(value), onUnconfirmed: () => { unconfirmed++; } };
  let tree;
  function render() { cursor = 0; tree = module.exports.ImportForm(props); for (const effect of effects.splice(0)) effect(); return tree; }
  function nodes() { const all = []; function visit(node) { if (!node || typeof node !== "object") return; all.push(node); for (const child of [node.props?.children].flat(Infinity)) visit(child); } visit(tree); return all; }
  const control = id => { const node = nodes().find(node => node.props?.["data-testid"] === id); assert.ok(node, id); return node; };
  async function click(id) { control(id).props.onClick(); await tick(); render(); }
  async function ready(name = "synthetic.csv", contents = "\uFEFFref,label\r\nsynthetic-id,DO_NOT_SHOW_CELL\r\n") {
    render();
    const file = new File([contents], name, { type: "text/csv" });
    control("import-file").props.onChange({ target: { files: [file] } }); await tick(); render();
    await click("import-preview"); return file;
  }
  function unmount() { for (const slot of slots) slot?.cleanup?.(); }
  return { render, control, nodes, has: id => nodes().some(node => node.props?.["data-testid"] === id), click, ready, calls, auth, unmount, unconfirmed: () => unconfirmed };
}

test("oversize file uses the actual header guard and specific size guidance in all locales without dispatch", async () => {
  for (const locale of ["en", "zh-TW", "zh-CN"]) {
    const h = harness(() => { throw Error("must not dispatch"); }, readImportHeader, locale);
    h.render();
    h.control("import-file").props.onChange({ target: { files: [new File([new Uint8Array((2 << 20) + 1)], "too-large.csv")] } });
    await tick(); h.render();
    assert.equal(h.nodes().find(n => n.props?.role === "alert")?.props.children, importCopy[locale].fileTooLarge, "MIUI-SIZE-GUIDANCE");
    assert.equal(h.calls.length, 0); assert.equal(h.control("import-preview").props.disabled, true); assert.equal(h.has("import-retry-same"), false);
  }
});

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
  assert.equal(h.has("import-type-orders"), false);
  assert.equal(h.calls.length, 3);
  assert.equal(h.calls[2].file, file); assert.equal(h.calls[2].mapping, h.calls[1].mapping);
  assert.equal(h.calls[2].expectedApplyRows, 1);
  h.unmount(); assert.equal(h.unconfirmed(), 0);
});

test("real ImportForm freezes customer field-v1 across preview, UNKNOWN, manual refusals and success; orders omit it", async () => {
  let attempts = 0;
  const h = harness(async () => ++attempts === 1 ? { kind: "error", code: "retry_later", uncertain: true }
    : attempts === 2 ? { kind: "error", code: "forbidden", uncertain: false } : { kind: "receipt", value: receipt },
  async file => file.name === "orders.csv" ? ["order", "customer", "when", "state", "total"] : ["ref", "label"]);
  const file = await h.ready();
  assert.equal(h.calls.length, 1); assert.equal(h.calls[0].rowErrors, "field-v1");
  assert.equal(h.calls[0].file, file); assert.equal(Object.hasOwn(h.calls[0], "expectedApplyRows"), false);
  await h.click("import-confirm"); assert.equal(h.calls.length, 2);
  assert.equal(h.control("import-restart").props.disabled, true);
  await h.click("import-retry-same"); assert.equal(h.calls.length, 3);
  assert.equal(h.control("import-restart").props.disabled, true);
  await h.click("import-retry-same"); assert.equal(h.calls.length, 4);
  assert.equal(h.control("import-restart").props.disabled, false);
  for (const request of h.calls.slice(1, 4)) {
    assert.equal(request.rowErrors, "field-v1"); assert.equal(request.file, file);
    assert.equal(request.mapping, h.calls[1].mapping); assert.equal(request.expectedApplyRows, 1);
    assert.equal(Object.isFrozen(request.mapping), true);
    assert.deepEqual(JSON.parse(JSON.stringify(request.mapping)), JSON.parse(JSON.stringify(h.calls[0].mapping)));
  }
  await h.click("import-type-orders");
  await h.ready("orders.csv", "order,customer,when,state,total\r\norder-1,customer-1,2026-01-01,paid,1\r\n");
  await h.click("import-confirm-next"); await h.click("import-confirm");
  assert.equal(h.calls.length, 6);
  for (const request of h.calls.slice(4)) { assert.equal(request.kind, "orders"); assert.equal(Object.hasOwn(request, "rowErrors"), false); }
});

test("real ImportForm error rows use source guidance only for explicit external_id with an actual mapped column", async () => {
  const words = {
    "zh-TW": { source: "第 3 筆 · 缺少來源編號", sourceRepair: "在原檔的 source_id 欄位填入穩定且唯一的來源編號。不要填電話或電子郵件。", generic: "第 3 筆 · 缺少必填內容", genericRepair: "請回到原檔補齊這一列缺少的必填內容，再重新預覽。" },
    "zh-CN": { source: "第 3 行 · 缺少来源编号", sourceRepair: "在原文件 source_id 列填入稳定且唯一的来源编号。不要填写电话或电子邮箱。", generic: "第 3 行 · 缺少必填内容", genericRepair: "请回到原文件补齐这一行缺少的必填内容，再重新预览。" },
    en: { source: "Row 3 · Missing source ID", sourceRepair: "Fill in a stable, unique source_id in the original file. Do not use a phone number or email address.", generic: "Row 3 · Missing required value", genericRepair: "Fill in the missing required value in this row of the original file, then preview again." },
  };
  const cases = [
    ...["zh-TW", "zh-CN", "en"].map(locale => ({ locale, column: "source_id", field: "external_id", source: true })),
    { locale: "en", column: "ref", field: "external_id", source: true },
    { locale: "en", column: "Customer ID", field: "external_id", source: true },
    ...["zh-TW", "zh-CN", "en"].flatMap(locale => [{ locale, column: "source_id", field: "name" }, { locale, column: "source_id" }]),
    { locale: "en", column: "", field: "external_id" },
    { locale: "en", column: "not-in-headers", field: "external_id" },
  ];
  for (const { locale, column, field, source } of cases) {
    const headers = !column || column === "not-in-headers" ? ["ref", "Name"] : [column, "Name"];
    const row = { row: 3, outcome: "failed", code: "required", ...(field ? { error_field: field } : {}) };
    const result = { ...preview, headers, mapping: { external_id: column, name: "Name" }, rows_total: 3, new_rows: 1, update_rows: 1, apply_rows: 2, failed_rows: 1,
      rows: [{ row: 1, outcome: "created" }, { row: 2, outcome: "updated" }, row] };
    const h = harness(async () => ({ kind: "receipt", value: receipt }), async () => headers, locale,
      { preview: result, requestedMapping: { external_id: headers[0], name: "Name" } });
    const badRow = field === "name" ? "source-3," : ",DO_NOT_SHOW_CELL";
    await h.ready("synthetic.csv", `${headers.join(",")}\r\nsource-1,One\r\nsource-2,Two\r\n${badRow}\r\n`);
    const reasons = h.nodes().find(node => node.type === "button" && node.props?.children === dataOpsCopy(locale).viewReasons);
    assert.ok(reasons, `${locale} ${column} reasons control`); reasons.props.onClick(); h.render();
    const failed = h.nodes().find(node => node.props?.className === "ops-row"); assert.ok(failed);
    const [heading, repair] = failed.props.children;
    const expectedHeading = source ? words[locale].source : words[locale].generic;
    const expectedRepair = source ? words[locale].sourceRepair.replace("source_id", column) : words[locale].genericRepair;
    assert.equal(heading.props.children, expectedHeading, `${locale} ${column} heading`);
    assert.equal(repair.props.children, expectedRepair, `${locale} ${column} repair`);
    for (const cell of ["DO_NOT_SHOW_CELL", "source-1", "source-2", "source-3"]) assert.equal(JSON.stringify(failed).includes(cell), false);
    if (!source) assert.equal(JSON.stringify(failed).includes("source_id"), false, "generic row must not claim the source column is missing");
  }
});
