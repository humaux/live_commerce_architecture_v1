// Purpose: DS-1 node tests for the pure design-system rules: selection scope (page vs all results), max-one-primary row actions, disabled-with-reason, pagination/URL state, typed confirm, file limits, status families, nav open state.
// Depends on: node:test/assert and packages/ui/src/ds/logic.ts (no React, no DOM, no browser).
// Used by: scripts/dev/test-node.sh and the --browser-ui-gallery mode (runs it before the browser phase).
import assert from "node:assert/strict";
import { test } from "node:test";
import {
  STATUS_FAMILIES,
  buttonState,
  bulkNeedsConfirm,
  bulkTarget,
  clearSelection,
  countPrimary,
  emptySelection,
  fileProblem,
  initialOpenGroups,
  offerAllResults,
  pageFromSearch,
  pageSelection,
  pageWindow,
  resolveRowActions,
  selectAllResults,
  selectionCount,
  statusTone,
  toggleRow,
  togglePage,
  typedConfirmMatches,
  withPage,
} from "../../packages/ui/src/ds/logic.ts";

const page = ["a", "b", "c", "d", "e"];

test("selection: header checkbox picks the page, a second click clears it", () => {
  let state = togglePage(emptySelection, page);
  assert.equal(pageSelection(state, page), "all");
  assert.equal(state.scope, "page");
  assert.equal(selectionCount(state, 47), 5);
  state = togglePage(state, page);
  assert.equal(pageSelection(state, page), "none");
  assert.equal(selectionCount(state, 47), 0);
});

test("selection: partial pick is 'some'; the all-results offer needs a full page AND more results off-page", () => {
  const state = toggleRow(emptySelection, "b", page);
  assert.equal(pageSelection(state, page), "some");
  assert.equal(offerAllResults(state, page, 47), false);
  const full = togglePage(emptySelection, page);
  assert.equal(offerAllResults(full, page, 47), true);
  assert.equal(offerAllResults(full, page, 5), false, "nothing off-page: no offer");
  assert.equal(selectAllResults(togglePage(emptySelection, page), page, 5).scope, "page", "selectAllResults is a no-op without the offer");
  assert.equal(selectAllResults(state, page, 47).scope, "page", "a partial page cannot jump to all results");
});

test("selection: 'all results' counts the total, is sent as a filter, and needs a confirm", () => {
  const all = selectAllResults(togglePage(emptySelection, page), page, 47);
  assert.equal(all.scope, "all");
  assert.equal(selectionCount(all, 47), 47);
  assert.equal(bulkNeedsConfirm(all), true);
  assert.deepEqual(bulkTarget(all, "published|"), { scope: "filter", filter: "published|" });
  const some = toggleRow(emptySelection, "c", page);
  assert.equal(bulkNeedsConfirm(some), false);
  assert.deepEqual(bulkTarget(some, "x"), { scope: "ids", ids: ["c"] });
});

test("selection: leaving 'all' by unchecking a row falls back to this page minus that row; header click clears", () => {
  const all = selectAllResults(togglePage(emptySelection, page), page, 47);
  const left = toggleRow(all, "c", page);
  assert.equal(left.scope, "page");
  assert.deepEqual([...left.ids].sort(), ["a", "b", "d", "e"]);
  assert.equal(togglePage(all, page).ids.size, 0);
  assert.equal(clearSelection().scope, "page");
  assert.equal(offerAllResults(left, page, 47), false);
});

test("row actions: at most ONE primary per row", () => {
  const ok = [{ id: "edit" }, { id: "publish", variant: "primary" as const }, { id: "x", variant: "danger" as const }];
  assert.equal(countPrimary(ok), 1);
  assert.deepEqual(resolveRowActions(ok, true), ok);
  const two = [{ id: "a", variant: "primary" as const }, { id: "b", variant: "primary" as const }];
  assert.throws(() => resolveRowActions(two, true), /at most one primary/);
  const demoted = resolveRowActions(two, false);
  assert.equal(countPrimary(demoted), 1);
  assert.equal(demoted[0]?.variant, "primary");
  assert.equal(demoted[1]?.variant, "secondary");
  assert.equal(countPrimary(resolveRowActions([], true)), 0);
});

test("button: a disabled button always has a reason; loading blocks without one", () => {
  assert.deepEqual(buttonState(undefined, false), { blocked: false, busy: false, reason: undefined });
  assert.deepEqual(buttonState("Add an image first", false), { blocked: true, busy: false, reason: "Add an image first" });
  assert.deepEqual(buttonState(undefined, true), { blocked: true, busy: true, reason: undefined });
  assert.throws(() => buttonState("", false), /must not be empty/);
  assert.throws(() => buttonState("   ", false), /must not be empty/);
});

test("pagination: window with gaps, clamped, and URL state that keeps the filters", () => {
  assert.deepEqual(pageWindow(1, 1), [1]);
  assert.deepEqual(pageWindow(1, 5), [1, 2, "gap", 5]);
  assert.deepEqual(pageWindow(4, 6), [1, 2, 3, 4, 5, 6], "a gap of one page shows the page, not an ellipsis");
  assert.deepEqual(pageWindow(10, 20), [1, "gap", 9, 10, 11, "gap", 20]);
  assert.deepEqual(pageWindow(3, 20), [1, 2, 3, 4, "gap", 20]);
  assert.deepEqual(pageWindow(99, 7), [1, "gap", 6, 7]);
  assert.deepEqual(pageWindow(1, 0), []);
  assert.equal(withPage("?q=a&page=3&state=ready", 4), "?q=a&page=4&state=ready");
  assert.equal(withPage("?q=a&page=3", 1), "?q=a", "page 1 has one canonical URL");
  assert.equal(withPage("", 1), "?");
  assert.equal(withPage("page=2", 5), "?page=5");
  assert.equal(pageFromSearch("?page=7"), 7);
  for (const bad of ["?page=0", "?page=-1", "?page=abc", "?page=1.5", "?page=", "", "?page=99999999"]) assert.equal(pageFromSearch(bad), 1, bad);
});

test("typed confirm: trims, folds width and Latin case, never matches an empty phrase", () => {
  assert.equal(typedConfirmMatches(" delete ", "DELETE"), true);
  assert.equal(typedConfirmMatches("ＤＥＬＥＴＥ", "DELETE"), true);
  assert.equal(typedConfirmMatches("刪除", "刪除"), true);
  assert.equal(typedConfirmMatches("刪", "刪除"), false);
  assert.equal(typedConfirmMatches("", ""), false);
  assert.equal(typedConfirmMatches("deleted", "DELETE"), false);
});

test("dropzone: format and size are checked against the rule shown before upload", () => {
  const rule = { extensions: ["jpg", "png"], maxBytes: 5 * 1024 * 1024 };
  assert.equal(fileProblem({ name: "a.JPG", size: 10 }, rule), undefined);
  assert.equal(fileProblem({ name: "a.gif", size: 10 }, rule), "type");
  assert.equal(fileProblem({ name: "noext", size: 10 }, rule), "type");
  assert.equal(fileProblem({ name: "a.png", size: 5 * 1024 * 1024 + 1 }, rule), "size");
  assert.equal(fileProblem({ name: "a.png", size: 5 * 1024 * 1024 }, rule), undefined);
});

test("status families: closed per-domain vocabularies, UNKNOWN is a warning everywhere, no cross-domain reuse of a pill", () => {
  assert.deepEqual(Object.keys(STATUS_FAMILIES).sort(), ["connection", "fulfilment", "payment", "refund"]);
  for (const domain of Object.keys(STATUS_FAMILIES) as Array<keyof typeof STATUS_FAMILIES>)
    assert.equal(statusTone(domain, "unknown"), "warning", `${domain} unknown`);
  assert.equal(statusTone("payment", "paid"), "success");
  assert.equal(statusTone("refund", "rejected"), "danger");
  assert.throws(() => statusTone("payment", "shipped"), /does not belong to the payment family/);
  assert.throws(() => statusTone("connection", "paid"), /does not belong/);
  assert.throws(() => statusTone("refund", "pending"), /does not belong/, "refund has no bare 'pending'");
});

test("nav: the group holding the current page starts open, others closed", () => {
  const nav = [{ id: "home" }, { id: "orders", items: [{ id: "list" }, { id: "returns" }] }, { id: "catalog", items: [{ id: "products" }] }];
  assert.deepEqual([...initialOpenGroups(nav, "returns")], ["orders"]);
  assert.deepEqual([...initialOpenGroups(nav, "catalog")], ["catalog"]);
  assert.deepEqual([...initialOpenGroups(nav, "home")], ["home"]);
  assert.equal(initialOpenGroups(nav, undefined).size, 0);
  assert.equal(initialOpenGroups(nav, "nope").size, 0);
});
