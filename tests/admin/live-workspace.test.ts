// Purpose: Pin pure live-workspace phase, stock and public embed boundaries before UI implementation.
// Depends on: node:test/assert and features/live/workspace-model.ts.
// Used by: test-node and LC-U1 acceptance.
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { primaryAction, stockDelta, facebookPostLink, settleLiveCommand } from "../../apps/admin/src/features/live/workspace-model.ts";
import * as workspace from "../../apps/admin/src/features/live/workspace-model.ts";
test("Live settings navigation accepts a scene picker entry but never a malformed or unscoped deep link", () => {
  const store = "11111111-1111-4111-8111-111111111111", scene = "22222222-2222-4222-8222-222222222222";
  assert.deepEqual(workspace.liveSettingsSelection({}), { store: "", scene: "" });
  assert.deepEqual(workspace.liveSettingsSelection({ store }), { store, scene: "" });
  assert.deepEqual(workspace.liveSettingsSelection({ store, scene }), { store, scene });
  for (const query of [{ scene }, { store: [store] }, { store, scene: [scene] }, { store: "" }, { store, scene: "" },
    { store: "foreign" }, { store, scene: "foreign" }, { store, tenant_id: scene }])
    assert.equal(workspace.liveSettingsSelection(query), null);
});
test("LC-B7 stock permission admits live_adjust without inventory:write, not unrelated grants", () => {
  for (const permissions of [["inventory:live_adjust"], ["inventory:write"], ["live:read", "inventory:live_adjust"]])
    assert.equal(workspace.liveStockAllowed({ role: "live_operator", permissions }), true);
  assert.equal(workspace.liveStockAllowed({ role: "owner" }), true);
  for (const store of [{}, { role: "admin" }, { permissions: ["live:read", "live:manage"] }])
    assert.equal(workspace.liveStockAllowed(store), false);
});
test("Instagram comment total stays unavailable; Facebook observed count is not a platform total", () => {
  for (const comments of [{ total: 7, source: "stream_seen" }, { total: 7, source: "graph_summary" }, { total: null, source: "unavailable" }] as const)
    assert.deepEqual(workspace.consoleCommentStat("instagram", comments), { total: null, source: "unavailable" });
  for (const comments of [{ total: 7, source: "stream_seen" }, { total: 0, source: "graph_summary" }, { total: null, source: "unavailable" }] as const)
    assert.deepEqual(workspace.consoleCommentStat("facebook", comments), comments);
});
test("LC-U1 one primary action follows lifecycle, never planning or transport state", () => {
  assert.equal(primaryAction("draft"), "start");
  assert.equal(primaryAction("live"), "end");
  assert.equal(primaryAction("ended"), "copy");
  assert.equal(primaryAction("archived"), null);
  assert.equal(primaryAction("READY"), null);
});
test("LC-U1 stock target uses sellable delta and rejects fractions, negative and over-limit changes", () => {
  assert.equal(stockDelta("0", 5), -5);
  assert.equal(stockDelta("8", 5), 3);
  for (const target of ["-1", "1.5", "1e3", "1006", ""]) assert.equal(stockDelta(target, 5), null);
});
test("LC-U1 links only explicit FB post identities, never arbitrary URLs or guessed videos", () => {
  assert.equal(facebookPostLink("facebook", "123_456"), "https://www.facebook.com/123/posts/456");
  assert.equal(facebookPostLink("instagram", "123_456"), null);
  assert.equal(facebookPostLink("facebook", "https://attacker.invalid"), null);
  assert.equal(facebookPostLink("facebook", "456"), null);
});
test("LC-U1 Facebook preview keeps the original no-frame CSP and uses a safe external link", () => {
  const config = readFileSync("apps/admin/next.config.ts", "utf8");
  assert.doesNotMatch(config, /frame-src/);
  const component = readFileSync("apps/admin/components/LiveConsole.tsx", "utf8");
  assert.doesNotMatch(component, /<iframe\b/);
  assert.match(component, /target="_blank" rel="noopener noreferrer"/);
});
test("LC-U1 delayed copy completion cannot navigate after store switch or unmount", async () => {
  for (const reason of ["store-switch", "unmount"]) {
    let resolve!: (id: string) => void, current = true;
    const navigations: string[] = [];
    const response = new Promise<string>((done) => { resolve = done; });
    const work = settleLiveCommand(() => response, () => current, (id) => { navigations.push(id); });
    current = false; resolve(reason);
    assert.equal(await work, false);
    assert.deepEqual(navigations, []);
  }
  let completed = "";
  assert.equal(await settleLiveCommand(async () => "new-scene", () => true, (id) => { completed = id; }), true);
  assert.equal(completed, "new-scene");
});
test("LC-U1 receipt fence survives cookie rotation; pending replay still has a separate session boundary", () => {
  const hook = readFileSync("apps/admin/src/features/live/use-live-workspace.ts", "utf8");
  assert.match(hook, /const fenceKey = `live-workspace-command:\$\{scope\}`;/);
  assert.match(hook, /identity\.current === atStart/);
  assert.match(hook, /\[fenceKey, boundary\]/);
});
