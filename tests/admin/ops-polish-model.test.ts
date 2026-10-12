// Purpose: independent OP3/OP4 model, copy and browser-assertion contract gates; no browser or Docker.
// Depends on: Node test/assert/fs, typescript-api, finance model, studio copy and actual ops-polish browser spec.
// Used by: focused Node gates and independent PR31 review.
// Written from the unit brief and the
// BD7 amendment (customers-billing-v1: rows carry pickup_collected_count / pickup_collected_minor, separate from captured and net), not from the code.
import assert from "node:assert/strict";
import { test } from "node:test";
import { readFileSync } from "node:fs";
import ts from "typescript-api";
import { parseFinanceSummary } from "../../apps/admin/lib/customers-model.ts";
import { studioCopy } from "../../apps/admin/lib/studio-copy.ts";

test("OP4 actual browser assertion keeps the exact four-route live inventory", () => {
  // Parse the consuming assertion, not the implementation registry. This is a
  // static contract check; browser visibility and toggling require the CI mode.
  const source = ts.createSourceFile("ops-polish.spec.ts", readFileSync(new URL("./ops-polish.spec.ts", import.meta.url), "utf8"), ts.ScriptTarget.Latest, true);
  const consumer = source.statements.find((node): node is ts.FunctionDeclaration => ts.isFunctionDeclaration(node) && node.name?.text === "assertRegistryNavigation");
  assert.ok(consumer?.body, "actual OP4 navigation consumer missing");
  const arrays: ts.Expression[] = [];
  const loops: ts.Expression[] = [];
  const visit = (node: ts.Node) => {
    if (ts.isCallExpression(node) && ts.isPropertyAccessExpression(node.expression) && node.expression.name.text === "toEqual") {
      assert.equal(node.arguments.length, 1);
      arrays.push(node.arguments[0]);
    }
    if (ts.isForOfStatement(node)) loops.push(node.expression);
    ts.forEachChild(node, visit);
  };
  visit(consumer.body);
  // Fail closed if the browser assertion becomes computed or uses unsupported
  // syntax. Only its literal fixture and liveSelected branch are expanded.
  const expand = (node: ts.Expression, liveSelected: boolean): string[] => {
    if (ts.isParenthesizedExpression(node)) return expand(node.expression, liveSelected);
    if (ts.isStringLiteral(node)) return [node.text];
    if (ts.isArrayLiteralExpression(node)) return node.elements.flatMap((item) => expand(ts.isSpreadElement(item) ? item.expression : item, liveSelected));
    if (ts.isConditionalExpression(node) && ts.isIdentifier(node.condition) && node.condition.text === "liveSelected") return expand(liveSelected ? node.whenTrue : node.whenFalse, liveSelected);
    assert.fail(`OP4 inventory must remain explicit: ${node.getText(source)}`);
  };
  const collapsed = ["nav-group-overview", "nav-group-live", "nav-orders", "nav-group-catalog", "nav-group-marketing", "nav-group-storefront", "nav-group-finance", "nav-group-settings"];
  const liveRoutes = ["studio", "live-console", "claims", "live-settings"];
  assert.equal(arrays.length, 1, "OP4 must keep one exact full-navigation assertion");
  assert.deepEqual(expand(arrays[0], false), collapsed);
  assert.deepEqual(expand(arrays[0], true), ["nav-group-overview", "nav-group-live", ...liveRoutes.map((id) => `nav-${id}`), ...collapsed.slice(2)]);
  assert.ok(loops.length >= 2, "live collapse/expand inventories missing");
  assert.deepEqual(expand(loops[0], true), liveRoutes, "collapsed live group must exclude every authorized live route");
  assert.deepEqual(expand(loops[1], true), liveRoutes, "expanded live group must show every authorized live route");
});

const row = (extra: Record<string, unknown> = {}) => ({
  day: "2026-09-30", currency: "TWD", environment: "LIVE", captured_count: 1, captured_minor: 2500, refunded_minor: 500, net_minor: 2000, ...extra,
});
const total = (extra: Record<string, unknown> = {}) => ({ ...row(extra), day: "" });
const doc = (rows: unknown[], totals: unknown[]) => ({ from: "2026-09-01", to: "2026-09-30", timezone: "Asia/Taipei", rows, totals });

test("finance rows with the two pay-at-pickup keys parse and stay separate from captured and net", () => {
  const parsed = parseFinanceSummary(doc([row({ pickup_collected_count: 2, pickup_collected_minor: 5000 })], [total({ pickup_collected_count: 2, pickup_collected_minor: 5000 })]));
  assert.equal(parsed.rows[0].pickup_collected_count, 2);
  assert.equal(parsed.rows[0].pickup_collected_minor, 5000);
  assert.equal(parsed.rows[0].captured_minor, 2500);
  assert.equal(parsed.rows[0].net_minor, 2000, "net is captured minus refunded only");
  assert.equal(parsed.totals[0].pickup_collected_minor, 5000);
});

test("a pickup-only day (no captured money) is a valid row whose captured and net stay zero", () => {
  const only = row({ captured_count: 0, captured_minor: 0, refunded_minor: 0, net_minor: 0, pickup_collected_count: 1, pickup_collected_minor: 2500 });
  const parsed = parseFinanceSummary(doc([only], [{ ...only, day: "" }]));
  assert.equal(parsed.rows[0].net_minor, 0);
  assert.equal(parsed.rows[0].pickup_collected_minor, 2500);
});

test("the older 7-key row (before the column) still parses with zero pickup money", () => {
  const parsed = parseFinanceSummary(doc([row()], [total()]));
  assert.equal(parsed.rows[0].pickup_collected_count, 0);
  assert.equal(parsed.rows[0].pickup_collected_minor, 0);
});

test("a half pair, a negative or a non-integer pickup value is refused, never shown", () => {
  for (const bad of [
    { pickup_collected_count: 1 },
    { pickup_collected_minor: 100 },
    { pickup_collected_count: -1, pickup_collected_minor: 0 },
    { pickup_collected_count: 1, pickup_collected_minor: -5 },
    { pickup_collected_count: 1.5, pickup_collected_minor: 100 },
    { pickup_collected_count: "1", pickup_collected_minor: 100 },
    { pickup_collected_count: 1, pickup_collected_minor: 100, extra_key: 1 },
  ]) assert.throws(() => parseFinanceSummary(doc([row(bad)], [])), /unavailable/, JSON.stringify(bad));
});

test("Studio subtitle is neutral production wording in all three locales", () => {
  for (const locale of ["en", "zh-CN", "zh-TW"] as const) {
    const subtitle = studioCopy[locale].subtitle;
    assert.ok(subtitle.length > 8, `${locale} subtitle is empty`);
    assert.doesNotMatch(subtitle, /MOCK|rehears|local|演练|演練|模拟|模擬|本地|本機|仅供|僅供|Nothing here publishes/i, `${locale}: ${subtitle}`);
  }
  assert.notEqual(studioCopy.en.subtitle, studioCopy["zh-TW"].subtitle);
  assert.notEqual(studioCopy["zh-CN"].subtitle, studioCopy["zh-TW"].subtitle);
});
