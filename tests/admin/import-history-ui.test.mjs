// Purpose: actual history component SSR and controlled navigation callbacks, including scope reset and double-click safety.
// Depends on: React SSR, shared TableFrame/formatters, compiled component, controlled guarded-read fixture and Node loader.
// Used by: W5-U1 local MOCK evidence; browser/390px/click-persistence acceptance remains NOT_RUN.
import assert from "node:assert/strict";
import { test } from "node:test";
import { readFileSync } from "node:fs";
import { runInNewContext } from "node:vm";
import { createRequire } from "node:module";
import ts from "typescript-api";
import { importHistoryCopy } from "../../apps/admin/lib/import-history-copy.ts";
import * as format from "../../packages/format/src/index.ts";
import { registerHistoryTestLoader } from "./import-history-test-loader.mjs";
registerHistoryTestLoader();
const require = createRequire(new URL("../../apps/admin/package.json", import.meta.url));
const React = require("react"); const jsxRuntime = require("react/jsx-runtime"); const { renderToStaticMarkup } = require("react-dom/server");
const { TableFrame } = await import("../../packages/ui/src/Presentation.tsx");
const source = ts.transpileModule(readFileSync("apps/admin/components/CustomerHistoricalOrders.tsx", "utf8"), {
  compilerOptions: { target: ts.ScriptTarget.ES2023, module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX },
}).outputText;
const id = "abcdef11-1111-4111-8111-111111111111";
const row = { order_id: "<SL-SYNTHETIC>", ordered_at: "2026-10-07T01:30:00Z", status: "已完成", total_minor: 128000, currency: "TWD", items_summary: "<img>synthetic", city: null };
const first = { items: [row], next_cursor: "Cursor_A", total: 51 };
const second = { items: [{ ...row, order_id: "SL-SECOND-SYNTHETIC" }], next_cursor: "", total: 51 };
const props = { locale: "en", store: id, customer: id, boundary: "a".repeat(64) };
function compile(react, jsx, guard, reader, frame) {
  const module = { exports: {} };
  runInNewContext(source, { module, exports: module.exports,
    require: (path) => {
      if (path === "react") return react;
      if (path === "react/jsx-runtime") return jsx;
      if (path === "@live-commerce/ui") return { TableFrame: frame };
      if (path === "@live-commerce/format") return format;
      if (path === "../lib/customers-client") return { useGuardedRead: guard };
      if (path === "../lib/import-history-client") return { readHistoricalOrders: reader };
      if (path === "../lib/import-history-copy") return { importHistoryCopy };
      if (path.endsWith(".css")) return {};
      throw Error(`Unexpected fixture import ${path}`);
    },
  });
  return module.exports.CustomerHistoricalOrders;
}
test("SSR renders actual safe rows, shared money/time, total count and read-only three-locale basis", () => {
  for (const locale of ["en", "zh-CN", "zh-TW"]) {
    const Component = compile(React, jsxRuntime, () => ({ status: "ready", data: first, reload: () => {} }), () => {}, TableFrame);
    const html = renderToStaticMarkup(React.createElement(Component, { ...props, locale }));
    assert.ok(html.includes(importHistoryCopy[locale].basis)); assert.match(html, /SHOPLINE/);
    assert.match(html, /&lt;SL-SYNTHETIC&gt;/); assert.match(html, /&lt;img&gt;synthetic/);
    assert.match(html, /NT\$/); assert.match(html, /1,280/); assert.match(html, /09:30/);
    assert.ok(html.includes(importHistoryCopy[locale].count(1, 51)));
    assert.doesNotMatch(html, /<a\b|href=|<img>|refund|ship|edit|delete/i);
    assert.match(html, /import-history-scroll/);
  }
});
test("SSR distinguishes empty archive, 403, session loss and unavailable without stale rows/count", () => {
  for (const [status, data, text] of [["ready", { items: [], next_cursor: "", total: 0 }, "No historical orders"],
    ["ready", { items: [], next_cursor: "", total: 51 }, "No historical orders on this page"],
    ["forbidden", null, "do not have permission"], ["signed-out", null, "session ended"], ["unavailable", null, "temporarily unavailable"], ["loading", null, "Loading historical orders"]]) {
    const Component = compile(React, jsxRuntime, () => ({ status, data, reload: () => {} }), () => {}, TableFrame);
    const html = renderToStaticMarkup(React.createElement(Component, props));
    assert.ok(html.includes(text)); assert.doesNotMatch(html, /SL-SYNTHETIC/);
    if (status !== "ready") assert.doesNotMatch(html, /data-testid="historical-orders-count"/);
  }
});
test("actual pagination callbacks use opaque trail; double click cannot skip; refresh/scope reset page1", async () => {
  let slots = []; let cursor = 0; let scopeKey = ""; let latestRead; let reloads = 0;
  const calls = [];
  const react = {
    useId: () => { const n = cursor++; return slots[n] ??= "synthetic-history-heading"; },
    useState: (initial) => { const n = cursor++; if (!(n in slots)) slots[n] = { value: initial };
      return [slots[n].value, (v) => { slots[n].value = typeof v === "function" ? v(slots[n].value) : v; }]; },
  };
  const jsx = (type, props, key) => ({ type, props, key });
  const Component = compile(react, { jsx, jsxs: jsx }, (scope, load) => {
    latestRead = load; const after = scope.split("|").at(-1);
    return { status: "ready", data: after === "Cursor_A" ? second : first, reload: () => { reloads++; } };
  }, async (...args) => { calls.push(args); return first; }, (p) => p.children);
  const render = (p = props) => { const wrapped = Component(p); if (scopeKey !== wrapped.key) { slots = []; scopeKey = wrapped.key; }
    cursor = 0; return wrapped.type(wrapped.props); };
  const nodes = (tree) => { const all = []; const walk = (n) => { if (Array.isArray(n)) n.forEach(walk); else if (n?.props) { all.push(n); walk(n.props.children); } }; walk(tree); return all; };
  const button = (tree, label) => nodes(tree).find((n) => n.type === "button" && n.props.children === label);
  let tree = render(); assert.equal(button(tree, "Previous").props.disabled, true);
  const next = button(tree, "Next"); next.props.onClick(); next.props.onClick(); tree = render();
  await latestRead(new AbortController().signal); assert.equal(calls.at(-1)[2], "Cursor_A");
  assert.equal(button(tree, "Next").props.disabled, true); assert.equal(button(tree, "Previous").props.disabled, false);
  const previous = button(tree, "Previous"); previous.props.onClick(); previous.props.onClick(); tree = render();
  await latestRead(new AbortController().signal); assert.equal(calls.at(-1)[2], "");
  button(tree, "Next").props.onClick(); tree = render(); button(tree, "Refresh").props.onClick(); tree = render();
  await latestRead(new AbortController().signal); assert.equal(calls.at(-1)[2], ""); assert.equal(reloads, 1);
  button(tree, "Next").props.onClick(); tree = render();
  tree = render({ ...props, customer: "22222222-2222-4222-8222-222222222222", boundary: "b".repeat(64) });
  await latestRead(new AbortController().signal);
  assert.equal(calls.at(-1)[1], "22222222-2222-4222-8222-222222222222"); assert.equal(calls.at(-1)[2], ""); assert.equal(calls.at(-1)[3], "b".repeat(64));
  assert.equal(button(tree, "Previous").props.disabled, true);
});
