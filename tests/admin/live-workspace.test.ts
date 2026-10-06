// Purpose: Pin pure live-workspace phase, stock and public embed boundaries before UI implementation.
// Depends on: node:test/assert and features/live/workspace-model.ts.
// Used by: test-node and LC-U1 acceptance.
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { primaryAction, stockDelta, facebookEmbed, settleLiveCommand } from "../../apps/admin/src/features/live/workspace-model.ts";
import * as workspace from "../../apps/admin/src/features/live/workspace-model.ts";
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
test("LC-U1 embeds only explicit public FB post identities, never arbitrary URLs or guessed videos", () => {
  const href = facebookEmbed("facebook", "123_456");
  assert.ok(href?.startsWith("https://www.facebook.com/plugins/post.php?"));
  assert.ok(href?.includes(encodeURIComponent("https://www.facebook.com/123/posts/456")));
  assert.equal(facebookEmbed("instagram", "123_456"), null);
  assert.equal(facebookEmbed("facebook", "https://attacker.invalid"), null);
  assert.equal(facebookEmbed("facebook", "456"), null);
});
test("LC-U1 public Facebook framing is allowed only on the console route", () => {
  const config = readFileSync("apps/admin/next.config.ts", "utf8");
  assert.match(config, /source: "\/:locale\/studio\/console"/);
  assert.match(config, /frame-src https:\/\/www\.facebook\.com/);
  assert.equal(config.match(/frame-src/g)?.length, 1);
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
