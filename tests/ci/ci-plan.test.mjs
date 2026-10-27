// Unit tests of the gates.yml matrix planner (scripts/dev/ci-plan.mjs): sweep / visual-lint / foundation-shards fan out completely, plain modes pass through, an explicit single
// shard re-run does NOT ask for the whole-run aggregate, and artifact names are unique; CI-SPEED: build-once consumers and the docs-only empty plan. Run by scripts/dev/test-node.sh.
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdtempSync, readFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import { SWEEP_SHARDS, VISUAL_SHARDS, artifactName, expand, main, prebuiltModes } from "../../scripts/dev/ci-plan.mjs";

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
  assert.match(text, /\nsweep_n=10\nvisual_n=0\nprebuilt_modes=\["--browser-click-sweep@1\/10",.*"--browser-click-sweep@10\/10"\]\nprebuild=true\n$/);
});

// PR #39 r7 (Codex P2): the nightly schedule shares the ref and mode names with post-merge push runs and has
// cancel-in-progress on; without the event in the group it would cancel push verification jobs.
test("gate job concurrency group separates run classes by event", () => {
  const wf = readFileSync(new URL("../../.github/workflows/gates.yml", import.meta.url), "utf8");
  const group = /^\s+group: gates-.*matrix\.mode.*$/m.exec(wf)?.[0] ?? "";
  assert.match(group, /\$\{\{ github\.event_name \}\}/, "gate job concurrency group must include github.event_name");
});

// CI-SPEED build-once + docs-only: shape of the plan outputs and of the gates.yml jobs that consume them.
const registry = readFileSync(new URL("../../scripts/dev/test-local.sh", import.meta.url), "utf8");
const wf = readFileSync(new URL("../../.github/workflows/gates.yml", import.meta.url), "utf8");
const job = (name) => wf.split(new RegExp(`^  ${name}:\\s*$`, "m"))[1]?.split(/^  [\w-]+:\s*$/m)[0] ?? "";

test("CI-SPEED: an empty plan is refused unless the PR was classified docs-only", () => {
  assert.throws(() => expand([], { groupIds }), /modes must be/);
  assert.deepEqual(expand([], { groupIds, allowEmpty: true }), { matrix: [], sweepN: 0, visualN: 0 });
  const out = path.join(mkdtempSync(path.join(tmpdir(), "ci-plan-")), "out");
  assert.throws(() => main({ MODES: "[]", GITHUB_OUTPUT: out, ALLOW_EMPTY: "false" }), /modes must be/);
  main({ MODES: "[]", GITHUB_OUTPUT: out, ALLOW_EMPTY: "true" });
  assert.equal(readFileSync(out, "utf8"), "matrix=[]\nsweep_n=0\nvisual_n=0\nprebuilt_modes=[]\nprebuild=false\n");
});

test("CI-SPEED: build-once consumers are exactly the registry arms that build a Next app or compile ./tests/foundation", () => {
  const matrix = expand(["foundation-shards", "--browser-click-sweep", "--browser-cvs", "--browser-ui-gallery", "--browser-picklist", "focused:^TestX$"], { groupIds }).matrix;
  const got = prebuiltModes(matrix, registry);
  assert.ok(got.includes("--browser-click-sweep@1/10") && got.includes("--browser-click-sweep@10/10") && got.includes("--browser-cvs"));
  for (const m of ["shard:unit", "shard:g01", "focused:^TestX$", "--browser-ui-gallery", "--browser-picklist"]) assert.ok(!got.includes(m), `${m} builds nothing shared (or its own flagged build)`);
});

test("CI-SPEED: gates.yml builds once, restores only same-run prebuilt output, and `required` accepts an empty plan only as docs-only", () => {
  const build = job("build"), gate = job("gate"), required = job("required"), receipts = job("receipts");
  assert.match(build, /if: \$\{\{ needs\.plan\.outputs\.prebuild == 'true' \}\}/);
  assert.match(build, /bash scripts\/dev\/test-local\.sh --prebuild "\$RUNNER_TEMP\/prebuilt"/);
  assert.match(build, /go test -race -count=1 -run '\^\$' \.\/tests\/foundation\n\s+go test -race -tags browser -count=1 -run '\^\$' \.\/tests\/foundation/);
  assert.match(build, /cache: false/, "the build job never writes the shard:gNN setup-go key");
  assert.match(gate, /needs: \[plan, build\]/);
  assert.match(gate, /needs\.plan\.outputs\.matrix != '\[\]'/, "an empty matrix is never instantiated");
  assert.match(gate, /contains\(fromJSON\(needs\.plan\.outputs\.prebuilt_modes\), matrix\.mode\)/);
  assert.match(gate, /env\.EXTRA_ENV == ''/, "fault-injection dispatches build their own");
  assert.match(gate, /node scripts\/dev\/prebuilt\.mjs unpack "\$RUNNER_TEMP\/prebuilt" gocache\n\s+echo "LC_PREBUILT_DIR=\$RUNNER_TEMP\/prebuilt" >> "\$GITHUB_ENV"/);
  assert.match(receipts, /needs\.plan\.outputs\.matrix != '\[\]'/);
  assert.match(required, /needs: \[plan, build, gate, aggregate, deploy, receipts\]/);
  assert.match(required, /DOCS_ONLY: \$\{\{ needs\.plan\.outputs\.docs_only == 'true' && needs\.plan\.outputs\.matrix == '\[\]' \}\}/);
  assert.match(required, /for r in "\$BUILD" "\$GATE" "\$AGG" "\$DEPLOY" "\$RECEIPTS"; do \[\[ "\$r" == skipped \]\] \|\| rc=1; done/);
  assert.match(required, /if \[\[ "\$BUILD_EXPECTED" == true \]\]; then \[\[ "\$BUILD" == success \]\] \|\| rc=1; else \[\[ "\$BUILD" == skipped \]\] \|\| rc=1; fi/);
  assert.match(job("plan"), /ALLOW_EMPTY: \$\{\{ github\.event_name == 'pull_request' && steps\.pr\.outputs\.docs_only == 'true' \}\}/);
});

// #238 review P2: the `required` step's shell (incl. the never-yet-exercised docs-only branch) is EXECUTED here, not only regex-matched:
// every combination of job results / expectations against an independent oracle.
test("CI-SPEED review P2: the real `required` script accepts exactly the right result combinations", () => {
  const block = /^  required:[\s\S]*?\n      - run: \|\n((?:          .*\n|\s*\n)+)/m.exec(wf)?.[1];
  assert.ok(block, "required job run block not found");
  const script = block.split("\n").map((l) => l.slice(10)).join("\n");
  const R = ["success", "skipped", "failure"], B = ["true", "false"], combos = [];
  for (const PLAN of ["success", "failure"]) for (const BUILD of R) for (const GATE of R) for (const AGG of R) for (const DEPLOY of R) for (const RECEIPTS of R)
    for (const DOCS_ONLY of B) for (const BUILD_EXPECTED of B) for (const AGG_EXPECTED of B) for (const DEPLOY_EXPECTED of B)
      combos.push({ PLAN, BUILD, GATE, AGG, DEPLOY, RECEIPTS, DOCS_ONLY, BUILD_EXPECTED, AGG_EXPECTED, DEPLOY_EXPECTED });
  const keys = Object.keys(combos[0]);
  const fn = `req() { ( ${script}\n) >/dev/null; }\nwhile read -r ${keys.join(" ")}; do export ${keys.join(" ")}; req; echo $?; done`;
  const out = execFileSync("bash", ["-c", fn], { input: combos.map((c) => keys.map((k) => c[k]).join(" ")).join("\n") + "\n", encoding: "utf8", maxBuffer: 16 << 20 }).trim().split("\n").map(Number);
  assert.equal(out.length, combos.length);
  const oracle = (c) => c.DOCS_ONLY === "true"
    ? c.PLAN === "success" && [c.BUILD, c.GATE, c.AGG, c.DEPLOY, c.RECEIPTS].every((r) => r === "skipped")
    : c.PLAN === "success" && c.GATE === "success" && c.RECEIPTS === "success" && c.BUILD === (c.BUILD_EXPECTED === "true" ? "success" : "skipped")
      && c.AGG === (c.AGG_EXPECTED === "true" ? "success" : "skipped") && c.DEPLOY === (c.DEPLOY_EXPECTED === "true" ? "success" : "skipped");
  combos.forEach((c, i) => assert.equal(out[i] === 0, oracle(c), JSON.stringify(c)));
  assert.ok(out.filter((x) => x === 0).length >= 2, "both the docs-only and the normal green path are reachable");
});
