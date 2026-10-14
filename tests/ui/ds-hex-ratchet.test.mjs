// Purpose: DS-1 gate that hard-coded hex colours in apps/admin never increase against tests/ui/hex-baseline.json, plus negatives proving the ratchet can fail (growth, a new file, id/entity look-alikes).
// Depends on: node:test/assert/fs, scripts/dev/check-hex-ratchet.mjs and the apps/admin source tree (read only).
// Used by: scripts/dev/test-node.sh and the --browser-ui-gallery mode.
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import { BASELINE, compare, countHex, scan } from "../../scripts/dev/check-hex-ratchet.mjs";

const baseline = JSON.parse(readFileSync(BASELINE, "utf8"));

test("baseline is internally consistent: total equals the sum of its files", () => {
  assert.equal(Object.values(baseline.files).reduce((a, b) => a + b, 0), baseline.total);
  assert.ok(baseline.total > 0);
});

test("countHex counts colours, not ids, entities, comments or longer words", () => {
  assert.equal(countHex("a{color:#fff;background:#1a2b3c;border:1px solid #AABBCCDD;x:#0006}"), 4);
  assert.equal(countHex('<a href="#main">&#x27;</a> <span id="#facebook"/>'), 0, "anchors, entities and #facebook are not colours");
  assert.equal(countHex("/* #fff #000 */\n// #123456\n.a{color:var(--ds-text-1)}"), 0, "comments are not counted");
  assert.equal(countHex("rgb(0 0 0 / 6%)"), 0);
});

test("compare: growth of the total or of any file fails; a new file has an allowance of zero; shrinking passes", () => {
  const base = { total: 3, files: { "a.css": 2, "b.css": 1 } };
  assert.deepEqual(compare({ total: 3, files: { "a.css": 2, "b.css": 1 } }, base), []);
  assert.deepEqual(compare({ total: 2, files: { "a.css": 1, "b.css": 1 } }, base), []);
  assert.match(compare({ total: 4, files: { "a.css": 3, "b.css": 1 } }, base).join("\n"), /a\.css: 3 hex colours > baseline 2/);
  assert.match(compare({ total: 4, files: { "a.css": 2, "b.css": 1, "new.css": 1 } }, base).join("\n"), /new\.css: 1 hex colours > baseline 0/);
  assert.match(compare({ total: 4, files: { "a.css": 1, "b.css": 1, "c.css": 2 } }, base).join("\n"), /total hex colours 4 > baseline 3/);
});

test("the live tree holds the baseline (apps/admin hex colours did not increase)", () => {
  assert.deepEqual(compare(scan(), baseline), []);
});
