// Unit tests of the gates.yml matrix planner (scripts/dev/ci-plan.mjs): sweep / visual-lint / foundation-shards fan out completely, plain modes pass through, an explicit single
// shard re-run does NOT ask for the whole-run aggregate, and artifact names are unique. Run by scripts/dev/test-node.sh.
import assert from "node:assert/strict";
import { mkdtempSync, readFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import { SWEEP_SHARDS, VISUAL_SHARDS, artifactName, expand, main } from "../../scripts/dev/ci-plan.mjs";
import { canonicalEvidenceDrift } from "../../scripts/dev/matrix-receipt.mjs";

const groupIds = ["g01", "g02", "g03"];
const modes = (r) => r.matrix.map((e) => e.mode);

test("the full sweep and the full visual lint fan out into every shard and ask for the aggregate", () => {
  const r = expand(["--browser-click-sweep", "--browser-visual-lint", "--browser-cvs"], { groupIds });
  assert.equal(r.sweepN, SWEEP_SHARDS);
  assert.equal(r.visualN, VISUAL_SHARDS);
  assert.deepEqual(modes(r).filter((m) => m.startsWith("--browser-click-sweep@")), Array.from({ length: SWEEP_SHARDS }, (_, i) => `--browser-click-sweep@${i + 1}/${SWEEP_SHARDS}`));
  assert.equal(modes(r).filter((m) => m.startsWith("--browser-visual-lint@")).length, VISUAL_SHARDS);
  assert.ok(modes(r).includes("--browser-cvs"), "plain browser modes pass through unchanged");
});

test("foundation-shards = the unit shard + every plan group; the serial `foundation` mode is untouched", () => {
  assert.deepEqual(modes(expand(["foundation-shards"], { groupIds })), ["shard:unit", "shard:g01", "shard:g02", "shard:g03"]);
  assert.deepEqual(modes(expand(["foundation", "focused:^TestX$", "shard:^TestL"], { groupIds })), ["foundation", "focused:^TestX$", "shard:^TestL"]);
  const real = JSON.parse(readFileSync(new URL("../../scripts/dev/shard-plan.json", import.meta.url), "utf8")).groups.map((g) => g.id);
  assert.equal(expand(["foundation-shards"], { groupIds: real }).matrix.length, real.length + 1);
});

test("actual canonical producer drift adds the browser backstop only when the complete matrix family is planned", () => {
  const base = { groupIds, catchAllId: "g03" };
  const clean = expand(["foundation-shards"], { ...base, matrixDrift: false });
  const drifted = expand(["foundation-shards"], { ...base, matrixDrift: true });
  const alreadySelected = expand(["foundation-shards", "--browser-product-editor"], { ...base, matrixDrift: true });
  assert.equal(modes(clean).includes("--browser-product-editor"), false);
  assert.equal(modes(drifted).filter((mode) => mode === "--browser-product-editor").length, 1);
  assert.equal(modes(alreadySelected).filter((mode) => mode === "--browser-product-editor").length, 1);
  assert.equal(modes(expand(["shard:unit"], { ...base, matrixDrift: true })).includes("--browser-product-editor"), false);
  assert.equal(modes(expand(["shard:g03"], { ...base, matrixDrift: true })).includes("--browser-product-editor"), false);
  const real = JSON.parse(readFileSync(new URL("../../scripts/dev/shard-plan.json", import.meta.url), "utf8"));
  const realCatchAll = real.groups.find((group) => group.catch_all).id;
  const needsBackstop = canonicalEvidenceDrift().length > 0;
  const output = path.join(mkdtempSync(path.join(tmpdir(), "ci-plan-matrix-drift-")), "out");
  main({ MODES: JSON.stringify(["foundation-shards"]), GITHUB_OUTPUT: output });
  const actual = JSON.parse(/^matrix=(.*)$/m.exec(readFileSync(output, "utf8"))[1]).map((entry) => entry.mode);
  assert.equal(actual.includes("--browser-product-editor"), needsBackstop);
  assert.ok(actual.includes(`shard:${realCatchAll}`));
});

test("manual foundation mode schedules the drift-aware browser backstop", () => {
  const output = path.join(mkdtempSync(path.join(tmpdir(), "ci-plan-manual-foundation-")), "out");
  main({ MODES: JSON.stringify(["foundation"]), GITHUB_OUTPUT: output });
  const actual = JSON.parse(/^matrix=(.*)$/m.exec(readFileSync(output, "utf8"))[1]).map((entry) => entry.mode);
  assert.ok(actual.includes("foundation"), "manual foundation execution remains available");
  assert.equal(actual.includes("--browser-product-editor"), canonicalEvidenceDrift().length > 0,
    "the documented manual full-Go mode must request same-run browser proof for canonical source drift");
});

test("an explicit shard re-run (or a partial list) never claims the whole-run aggregate", () => {
  assert.equal(expand(["--browser-click-sweep@3/10"], { groupIds }).sweepN, 0);
  const partial = expand(["--browser-click-sweep@1/3", "--browser-click-sweep@2/3"], { groupIds });
  assert.equal(partial.sweepN, 0);
  const all = expand(["--browser-click-sweep@1/3", "--browser-click-sweep@2/3", "--browser-click-sweep@3/3"], { groupIds });
  assert.equal(all.sweepN, 3, "listing every shard yourself is the same as the short form");
  assert.equal(expand(["--browser-click-sweep@1/3", "--browser-click-sweep@2/4"], { groupIds }).sweepN, 0, "mixed N is not a whole run");
});

test("duplicates collapse, artifact names are safe and unique, bad input is refused", () => {
  assert.equal(expand(["--browser-cvs", "--browser-cvs"], { groupIds }).matrix.length, 1);
  assert.equal(artifactName("--browser-click-sweep@3/10"), "gate-browser-click-sweep_3_10");
  assert.equal(artifactName("shard:^Test(A|[N-R])"), "gate-shard_Test_A_N-R");
  for (const e of expand(["foundation-shards", "--browser-click-sweep", "focused:^TestA$|^TestB$"], { groupIds }).matrix) assert.match(e.art, /^[A-Za-z0-9_.-]+$/, e.art);
  assert.throws(() => expand(["shard:a/b", "shard:a_b"], { groupIds }), /same artifact name/);
  for (const bad of [[], "x", [1], ["bad\nmode"]]) assert.throws(() => expand(bad, { groupIds }), /modes must be/);
});

test("main writes matrix / sweep_n / visual_n to GITHUB_OUTPUT", () => {
  const out = path.join(mkdtempSync(path.join(tmpdir(), "ci-plan-")), "out");
  main({ MODES: JSON.stringify(["--browser-click-sweep"]), GITHUB_OUTPUT: out });
  const text = readFileSync(out, "utf8");
  assert.match(text, /^matrix=\[\{"mode":"--browser-click-sweep@1\/10","art":"gate-browser-click-sweep_1_10"\}/);
  assert.match(text, /\nsweep_n=10\nvisual_n=0\n$/);
});

// PR #39 r7 (Codex P2): the nightly schedule shares the ref and mode names with post-merge push runs and has
// cancel-in-progress on; without the event in the group it would cancel push verification jobs.
test("gate job concurrency group separates run classes by event", () => {
  const wf = readFileSync(new URL("../../.github/workflows/gates.yml", import.meta.url), "utf8");
  const group = /^\s+group: gates-.*matrix\.mode.*$/m.exec(wf)?.[0] ?? "";
  assert.match(group, /\$\{\{ github\.event_name \}\}/, "gate job concurrency group must include github.event_name");
});
