// Purpose: #543 C — the inventory next-page control and availability chips say what they do: the chips filter only loaded rows.
// Depends on: TypeScript transpile of the real InventoryView source, a synthetic useState and the data-ops copy namespace.
// Used by: scripts/dev/test-node.sh (no browser/PG). The fixture store has one page, so browser specs never render the next-page button.
// Invariants: contracts/admin-ledger-v1.md has no availability parameter, so filtering stays client-side and must not read as "show all".
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { runInNewContext } from "node:vm";
import ts from "typescript-api";
import { dataOpsCopy } from "../../packages/i18n/src/data-ops.ts";

const source = readFileSync(new URL("../../apps/admin/components/inventory/InventoryView.tsx", import.meta.url), "utf8");
const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX } }).outputText;

function render(locale, nextCursor) {
  const module = { exports: {} }, jsx = (type, props) => ({ type, props }), navigations = [];
  const modules = { react: { useState: initial => [initial, () => {}] }, "react/jsx-runtime": { jsx, jsxs: jsx, Fragment: "fragment" }, "../../../../packages/i18n/src/data-ops": { dataOpsCopy } };
  runInNewContext(compiled, { module, exports: module.exports, require: name => { if (name in modules) return modules[name]; throw Error("unexpected import " + name); } });
  const row = { sku_id: "sku-1", code: "A12", product_name: "Synthetic bag", status: "active", warehouse_id: "w1", on_hand: 1, reserved: 0, allocated: 0, unavailable: 0, available: 1 };
  const noop = () => {};
  const tree = module.exports.InventoryView({ locale, initial: { storeID: "s1", warehouseID: "w1", rows: { items: [row], next_cursor: nextCursor } }, state: "list", setState: noop,
    select: noop, delta: "", setDelta: noop, reason: "", setReason: noop, busy: false, locked: false, query: "", setQuery: noop, navigate: values => navigations.push(values),
    submit: noop, retry: noop, reload: noop, error: "", warehouses: [{ id: "w1", name: "Main" }], warehouseLoading: false, status: "all", refresh: noop });
  const nodes = [];
  (function visit(node) { if (!node || typeof node !== "object") return; nodes.push(node); for (const child of [node.props?.children].flat(Infinity)) visit(child); })(tree);
  return { nodes, navigations };
}

const words = {
  "zh-TW": { more: "載入更多庫存", note: "篩選只套用到目前已載入的庫存列。", all: "全部庫存" },
  "zh-CN": { more: "加载更多库存", note: "筛选仅适用于当前已加载的库存行。", all: "全部库存" },
  en: { more: "Load more inventory", note: "Filters apply only to the inventory rows loaded so far.", all: "All inventory" },
};

test("#543 C: the next-page button reads as loading more, and a note says the chips filter only loaded rows", () => {
  for (const [locale, word] of Object.entries(words)) {
    const { nodes, navigations } = render(locale, "cursor-2");
    const buttons = nodes.filter(node => node.type === "button");
    const more = buttons.find(node => node.props.children === word.more);
    assert.ok(more, `${locale} next-page button is labelled ${word.more}`);
    assert.equal(buttons.some(node => node.props.children === word.all), false, `${locale} no button reads as show-all`);
    assert.ok(nodes.some(node => node.props?.children === word.note), `${locale} loaded-rows filter note`);
    more.props.onClick(); assert.deepEqual(navigations, [{ cursor: "cursor-2" }]);
    const single = render(locale, undefined).nodes;
    assert.equal(single.some(node => node.props?.children === word.note || node.props?.children === word.more), false, `${locale} one page: no note, no button`);
  }
});
