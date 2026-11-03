// Scheduling-only regression tests: preserve every planned mode and every
// foundation command while removing the unused browser-build dependency.
import assert from "node:assert/strict";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import { artifactName, bundleJobs, expand, main, partitionJobs } from "../../scripts/dev/ci-plan.mjs";

const wf = readFileSync(new URL("../../.github/workflows/gates.yml", import.meta.url), "utf8");
const job = (name) => wf.split(new RegExp(`^  ${name}:\\s*$`, "m"))[1]?.split(/^  [\w-]+:\s*$/m)[0] ?? "";
const gateRun = (source) => source.split("      - name: Gate\n", 2)[1]?.split("      # MATRIX-RECEIPT", 1)[0];
const source = readFileSync(new URL("../../scripts/dev/test-local.sh", import.meta.url), "utf8");

test("every real foundation shard moves once and every other mode stays in the original lane", () => {
  const plan = JSON.parse(readFileSync(new URL("../../scripts/dev/shard-plan.json", import.meta.url), "utf8"));
  const groupIds = plan.groups.map((g) => g.id);
  for (const enabled of [false, true]) {
    const matrix = expand(["foundation-shards", "--browser-click-sweep", "--browser-cvs", "--browser-buyer", "foundation", "shard:^TestX$", "focused:^TestY$"], { groupIds }).matrix;
    const jobs = bundleJobs(matrix, { enabled });
    const before = structuredClone(jobs);
    const lanes = partitionJobs(jobs);
    assert.deepEqual(lanes.foundation.map((j) => j.mode), ["shard:unit@1/2", "shard:unit@2/2", ...groupIds.map((id) => `shard:${id}`)]);
    assert.deepEqual(jobs, before, "partitioning does not alter mode, members, timeout or artifacts");
    assert.deepEqual([...lanes.foundation, ...lanes.gate].map((j) => j.mode).sort(), jobs.map((j) => j.mode).sort());
    const all = [...lanes.foundation, ...lanes.gate].flatMap((j) => j.members);
    assert.equal(new Set(all.map((m) => m.mode)).size, matrix.length);
    assert.deepEqual(all.map((m) => m.art).sort(), matrix.map((m) => m.art).sort());
  }
});

test("unknown modes, custom regexes, and bundles never enter the early foundation lane", () => {
  const names = ["shard:g01extra", "shard:g001", "shard:g1", "shard:^TestX$", "foundation", "focused:^TestX$", "--browser-cvs"];
  const jobs = names.map((mode) => ({ mode, bundle: false, members: [{ mode, art: artifactName(mode) }] }));
  jobs.push({ mode: "shard:g01", bundle: true, members: [{ mode: "shard:g01" }] });
  jobs.push({ mode: "shard:g01", bundle: false, members: [{ mode: "shard:g01" }, { mode: "--browser-cvs" }] });
  jobs.push({ mode: "shard:g01", bundle: false, members: [{ mode: "--browser-cvs" }] });
  assert.equal(partitionJobs(jobs).foundation.length, 0);
  assert.deepEqual(partitionJobs(jobs).gate, jobs);
});

test("empty lanes are explicit and planner retains the complete matrix and jobs outputs", () => {
  for (const modes of [["foundation-shards"], ["--browser-cvs"], ["foundation-shards", "--browser-cvs"]]) {
    const dir = mkdtempSync(path.join(tmpdir(), "foundation-lane-"));
    try {
      const out = path.join(dir, "out");
      main({ MODES: JSON.stringify(modes), BUNDLE: "true", GITHUB_OUTPUT: out });
      const values = Object.fromEntries(readFileSync(out, "utf8").trim().split("\n").map((line) => {
        const i = line.indexOf("="); return [line.slice(0, i), line.slice(i + 1)];
      }));
      const jobs = JSON.parse(values.jobs), foundation = JSON.parse(values.foundation_jobs), gate = JSON.parse(values.gate_jobs);
      assert.deepEqual([...foundation, ...gate].map((j) => j.mode).sort(), jobs.map((j) => j.mode).sort());
      assert.equal(foundation.length > 0, modes.includes("foundation-shards"));
      assert.equal(gate.length > 0, modes.includes("--browser-cvs"));
      assert.equal(JSON.parse(values.matrix).length, jobs.flatMap((j) => j.members).length);
    } finally {
      rmSync(dir, { recursive: true, force: true });
    }
  }
});

test("early foundation job does not depend on build and the original lane uses its own matrix", () => {
  const early = job("foundation_gate"), gate = job("gate");
  assert.match(early, /needs: plan\n/);
  assert.doesNotMatch(early, /needs\.build|LC_PREBUILT_DIR|download-artifact/);
  assert.match(early, /needs\.plan\.outputs\.foundation_jobs != '\[\]'/);
  assert.match(early, /fromJSON\(needs\.plan\.outputs\.foundation_jobs\)/);
  assert.match(gate, /needs: \[plan, build\]/);
  assert.match(gate, /needs\.plan\.outputs\.gate_jobs != '\[\]'/);
  assert.match(gate, /fromJSON\(needs\.plan\.outputs\.gate_jobs\)/);
  for (const lane of [early, gate]) {
    assert.match(lane, /fail-fast: false/);
    assert.match(lane, /persist-credentials: false/);
    assert.match(lane, /group: gates-\$\{\{ github\.event_name \}\}/);
    assert.match(lane, /EXTRA_ENV: \$\{\{ github\.event_name == 'workflow_dispatch' && inputs\.extra_env \|\| '' \}\}/);
  }
  assert.match(early, /fetch-depth: 0/, 'foundation keeps pinned pre-LMR source');
  assert.match(gate, /fetch-depth: \$\{\{ matrix\.mode == '--browser-webkit' && '1' \|\| '0' \}\}/, 'only the seven-step WebKit mode may use a shallow checkout');
});

test("both lanes execute byte-identical gate commands, fault injection and verdict propagation", () => {
  const early = gateRun(job("foundation_gate")), original = gateRun(job("gate"));
  assert.ok(early && original);
  assert.equal(early, original, "update both copies together when changing dispatch commands");
  assert.match(early, /set -o pipefail/);
  assert.match(early, /LC_FOUNDATION_PKGS=.*go list \.\/\.\.\./);
  assert.match(early, /LC_FOUNDATION_PKGS=\.\/tests\/foundation LC_FOUNDATION_RUN=/);
  assert.match(early, /LC_FOUNDATION_SKIP=/);
  assert.match(source, /go test -p 1 -race -count=1 -timeout=4500s -v/);
  assert.match(source, /go vet \.\/\.\.\./);
});

test("same receipt and evidence names, freshness inputs and pinned PG image are retained", () => {
  const early = job("foundation_gate"), gate = job("gate");
  const receipt = (s) => s.split("      - name: Run receipt\n", 2)[1]?.split("      - name: Failure summary", 1)[0];
  assert.equal(receipt(early), receipt(gate));
  assert.match(early, /GATE_OUTCOME: \$\{\{ steps\.gate\.outcome \}\}/);
  assert.match(early, /name: receipt-\$\{\{ matrix\.art \}\}/);
  assert.match(early, /name: \$\{\{ matrix\.art \}\}/);
  assert.equal(early.match(/docker pull postgres@sha256:[a-f0-9]+/)[0], gate.match(/docker pull postgres@sha256:[a-f0-9]+/)[0]);
});

// REAL-CLOCK-LANE (owner 2026-11-04): the three 15-minute order-attempt tests must never floor a pull request, must still run on push/nightly,
// and the two lanes together must still cover every top-level test exactly once. These tests are the mechanical proof of that gate change.
import { LONG_REAL_CLOCK, REAL_CLOCK_GROUP, checkLanes, laneGroupIds, topLevelTests } from "../../scripts/dev/shard-plan.mjs";

const shardPlan = JSON.parse(readFileSync(new URL("../../scripts/dev/shard-plan.json", import.meta.url), "utf8"));

test("lanes cover every top-level test exactly once and the real-clock group holds exactly the three measured tests", () => {
  const names = topLevelTests();
  assert.equal(names.length, shardPlan.groups.flatMap((g) => g.tests).length, "no test is listed twice or dropped");
  assert.deepEqual(checkLanes(shardPlan, names).problems, []);
  const grc = shardPlan.groups.find((g) => g.id === REAL_CLOCK_GROUP);
  assert.deepEqual(grc.tests, [...LONG_REAL_CLOCK].sort());
  for (const name of LONG_REAL_CLOCK) assert.ok(!laneGroupIds(shardPlan, "pr").includes(`shard:${REAL_CLOCK_GROUP}`) && !shardPlan.groups.filter((g) => g.id !== REAL_CLOCK_GROUP).some((g) => g.tests.includes(name)), `${name} must not be in a PR shard`);
  assert.deepEqual(laneGroupIds(shardPlan, "full"), shardPlan.groups.map((g) => g.id));
  assert.ok(!laneGroupIds(shardPlan, "pr").includes(REAL_CLOCK_GROUP));
  assert.throws(() => laneGroupIds(shardPlan, "fast"), /unknown foundation lane/);
});

test("the planner omits the real-clock shard on pull_request and keeps it on push/nightly", () => {
  const run = (lane) => {
    const dir = mkdtempSync(path.join(tmpdir(), "real-clock-lane-"));
    try {
      const out = path.join(dir, "out");
      main({ MODES: '["foundation-shards"]', BUNDLE: "true", FOUNDATION_LANE: lane, GITHUB_OUTPUT: out });
      const values = Object.fromEntries(readFileSync(out, "utf8").trim().split("\n").map((line) => { const i = line.indexOf("="); return [line.slice(0, i), line.slice(i + 1)]; }));
      return { matrix: JSON.parse(values.matrix), jobs: JSON.parse(values.jobs), lane: values.foundation_lane };
    } finally { rmSync(dir, { recursive: true, force: true }); }
  };
  const pr = run("pr"), full = run("full");
  assert.equal(pr.lane, "pr");
  assert.ok(!pr.matrix.some((m) => m.mode === `shard:${REAL_CLOCK_GROUP}`), "pull request matrix must not plan grc");
  assert.ok(full.matrix.some((m) => m.mode === `shard:${REAL_CLOCK_GROUP}`), "full matrix must plan grc");
  assert.equal(full.matrix.length, pr.matrix.length + 1);
  assert.ok(pr.jobs.flatMap((j) => j.members).every((m) => m.mode !== `shard:${REAL_CLOCK_GROUP}`));
});

test("gates.yml selects the PR lane for pull_request and the full lane otherwise", () => {
  assert.match(wf, /FOUNDATION_LANE: \$\{\{ github\.event_name == 'pull_request' && 'pr' \|\| 'full' \}\}/);
  assert.match(job("plan"), /FOUNDATION_LANE/);
});
