// Purpose: Pin pure live-workspace phase, stock and public embed boundaries before UI implementation.
// Depends on: node:test/assert and features/live/workspace-model.ts.
// Used by: test-node and LC-U1 acceptance.
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { primaryAction, stockDelta, facebookEmbed } from "../../apps/admin/src/features/live/workspace-model.ts";
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
