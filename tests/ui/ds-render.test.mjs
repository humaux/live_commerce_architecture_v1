// Purpose: DS-1 server-render tests of the real Controls.tsx / Data.tsx JSX: SegmentedSwitch disabled-reason wiring and the selection banner count in page scope.
// Depends on: node:test/assert/fs/vm, typescript-api, React SSR from apps/admin, packages/ui/src/ds/{Controls,Data}.tsx and logic.ts; no browser, no network.
// Used by: scripts/dev/test-node.sh. Effects, focus and menus are not rendered here; they are covered by the click ledger (ui-gallery-clicks.mjs).
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { test } from "node:test";
import { runInNewContext } from "node:vm";
import ts from "typescript-api";
import * as logic from "../../packages/ui/src/ds/logic.ts";

const requireApp = createRequire(new URL("../../apps/admin/package.json", import.meta.url));
const React = requireApp("react");
const { renderToStaticMarkup } = requireApp("react-dom/server");
const css = { __esModule: true, default: new Proxy({}, { get: (_, key) => String(key) }) };

function load(path, resolve) {
  const exports = {};
  const source = readFileSync(path, "utf8");
  runInNewContext(ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX } }).outputText, { exports, require: resolve, process });
  return exports;
}
const icon = load("packages/ui/src/ds/Icon.tsx", requireApp);
const controls = load("packages/ui/src/ds/Controls.tsx", (name) => (name.endsWith(".css") ? css : name === "./logic" ? logic : name === "./Icon" ? icon : requireApp(name)));
const data = load("packages/ui/src/ds/Data.tsx", (name) => (name.endsWith(".css") ? css : name === "./logic" ? logic : name === "./Controls" ? controls : name === "./Icon" ? icon : requireApp(name)));
const html = (node) => renderToStaticMarkup(node);

test("SegmentedSwitch: a disabled switch points aria-describedby at its (hidden until clicked) reason", () => {
  const out = html(React.createElement(controls.SegmentedSwitch, { checked: true, onChange() {}, label: "Auto reply", onLabel: "Yes", offLabel: "No", disabledReason: "Needs the Pro plan" }));
  const describedBy = /aria-describedby="([^"]+)"/.exec(out)?.[1];
  assert.ok(describedBy, "aria-describedby present");
  assert.ok(out.includes(`<span id="${describedBy}" class="sr">Needs the Pro plan</span>`), "reason element has that id and is screen-reader-only before interaction");
  assert.match(out, /aria-disabled="true"/);
  assert.match(out, /title="Needs the Pro plan"/);
});

test("SegmentedSwitch: an enabled switch renders no reason element and no aria-describedby", () => {
  const out = html(React.createElement(controls.SegmentedSwitch, { checked: false, onChange() {}, label: "Auto reply", onLabel: "Yes", offLabel: "No" }));
  assert.doesNotMatch(out, /aria-describedby|aria-disabled/);
});

const labels = {
  selectPage: "Select page",
  selectRow: (n) => `Row ${n}`,
  selected: (n) => `${n} selected`,
  pageOnly: "This page only",
  allResults: (n) => `All ${n}`,
  selectAllResults: (n) => `Select all ${n}`,
  clear: "Clear",
};
const table = (state, rows) =>
  html(
    React.createElement(data.DataTable, {
      caption: "t",
      columns: [{ id: "n", header: "N", cell: (r) => r }],
      rows,
      rowId: (r) => r,
      empty: "empty",
      error: "error",
      loadingLabel: "loading",
      selection: { state, onChange() {}, total: 47, filterKey: "all|", labels },
    }),
  );

test("DataTable banner: page scope counts only ids on the current page", () => {
  const onPageOne = logic.togglePage(logic.emptySelection, ["a", "b", "c"]);
  assert.match(table(onPageOne, ["a", "b", "c"]), /3 selected/);
  assert.doesNotMatch(table(onPageOne, ["d", "e", "f"]), /\d+ selected/, "ids kept from page 1 are not counted (or shown) on page 2");
  const mixed = logic.toggleRow(onPageOne, "e", ["d", "e", "f"]);
  assert.match(table(mixed, ["d", "e", "f"]), /1 selected/);
  // the row attribute data-selected stays on every data row (the banner check above looks for "<n> selected" text only)
  const flags = (html) => [...html.matchAll(/data-selected="(true|false)"/g)].map((m) => m[1]);
  assert.deepEqual(flags(table(onPageOne, ["d", "e", "f"])), ["false", "false", "false"], "page 2 rows are all marked unselected");
  assert.deepEqual(flags(table(mixed, ["d", "e", "f"])), ["false", "true", "false"]);
});

test("DataTable banner: all-results scope counts the total", () => {
  const all = logic.selectAllResults(logic.togglePage(logic.emptySelection, ["a", "b", "c"]), ["a", "b", "c"], 47, "all|");
  assert.match(table(all, ["a", "b", "c"]), /47 selected/);
});
