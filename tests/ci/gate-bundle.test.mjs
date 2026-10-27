// Purpose: CI-SPEED-2 gate bundles. (1) scripts/dev/ci-plan.mjs bundleJobs: every planned mode in exactly one job, only measured short legs bundled, each
//   member keeps its own mode/art, explicit dispatches never bundled. (2) gates.yml contract: the gate matrix is `jobs`, receipts still verify `matrix`,
//   BUNDLE_MAX unrolled per-member upload slots under the solo artifact names. (3) The REAL Gate step shell is EXECUTED with stubbed browsers: a red member
//   must leave its own FAIL receipt, must not stop the members after it, and must turn the step (and so `required`) red.
// Depends on: scripts/dev/ci-plan.mjs, scripts/dev/matrix-receipts.mjs (verify), scripts/dev/pr-modes.mjs (registry), .github/workflows/gates.yml; bash, jq, GNU timeout (ubuntu runner).
// Used by: scripts/dev/test-node.sh (Foundation checks).
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { chmodSync, existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, symlinkSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { BUNDLE_LIMIT_S, BUNDLE_MAX, GATE_SECONDS, artifactName, bundleJobs, expand } from "../../scripts/dev/ci-plan.mjs";
import { receiptFile, verify } from "../../scripts/dev/matrix-receipts.mjs";
import { modeEntries } from "../../scripts/dev/pr-modes.mjs";

const repo = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const wf = readFileSync(path.join(repo, ".github/workflows/gates.yml"), "utf8");
const job = (name) => wf.split(new RegExp(`^  ${name}:\\s*$`, "m"))[1]?.split(/^  [\w-]+:\s*$/m)[0] ?? "";
const registryNames = new Set(modeEntries(readFileSync(path.join(repo, "scripts/dev/test-local.sh"), "utf8")).map((e) => e.name));
// A UI-scale plan: shards, sweep/visual slices, every measured leg, the two long legs and one unmeasured mode.
const uiMatrix = expand(["foundation-shards", "--browser-click-sweep", "--browser-visual-lint", ...Object.keys(GATE_SECONDS), "--browser-live-console", "--browser-webkit", "--browser-new-unmeasured"], { groupIds: ["g01", "g02"] }).matrix;

test("CI-SPEED-2: every planned mode runs in exactly one job; only measured short legs are bundled, each keeping its own mode and artifact name", () => {
  const jobs = bundleJobs(uiMatrix);
  const members = jobs.flatMap((j) => j.members.map((m) => m.mode));
  assert.deepEqual([...members].sort(), uiMatrix.map((e) => e.mode).sort(), "the job members are exactly the planned modes, once each");
  const bundles = jobs.filter((j) => j.bundle);
  assert.ok(bundles.length >= 5 && bundles.length <= 12, `~10 bundles for a UI PR, got ${bundles.length}`);
  assert.ok(jobs.length < uiMatrix.length / 2, `${uiMatrix.length} modes -> ${jobs.length} jobs`);
  for (const b of bundles) {
    assert.match(b.mode, /^bundle:\d\d$/);
    assert.equal(b.timeout, 360);
    assert.ok(b.members.length >= 2 && b.members.length <= BUNDLE_MAX, `${b.mode}: ${b.members.length} members`);
    for (const m of b.members) {
      assert.ok(GATE_SECONDS[m.mode] < BUNDLE_LIMIT_S, `${m.mode} is not a measured short leg`);
      assert.equal(m.art, artifactName(m.mode), "a member keeps the artifact name its solo job had");
    }
  }
  for (const j of jobs.filter((x) => !x.bundle)) {
    assert.deepEqual(j, { mode: j.mode, art: artifactName(j.mode), bundle: false, timeout: 150, members: [{ mode: j.mode, art: artifactName(j.mode) }] });
  }
  const solo = new Set(jobs.filter((j) => !j.bundle).map((j) => j.mode));
  for (const m of ["shard:unit", "shard:g01", "--browser-click-sweep@1/10", "--browser-visual-lint@4/4", "--browser-live-console", "--browser-webkit", "--browser-new-unmeasured"]) assert.ok(solo.has(m), `${m} keeps its own job`);
  assert.deepEqual(bundleJobs(uiMatrix), jobs, "deterministic");
});

test("CI-SPEED-2: bundling off (explicit dispatch) or fewer than two short legs = one job per mode, exactly as before", () => {
  const off = bundleJobs(uiMatrix, { enabled: false });
  assert.equal(off.length, uiMatrix.length);
  assert.ok(off.every((j) => !j.bundle && j.timeout === 150));
  assert.deepEqual(bundleJobs(expand(["--browser-click-sweep@3/10"], { groupIds: [] }).matrix).map((j) => j.mode), ["--browser-click-sweep@3/10"]);
  assert.equal(bundleJobs(expand(["--browser-buyer", "shard:unit"], { groupIds: [] }).matrix).filter((j) => j.bundle).length, 0);
});

test("CI-SPEED-2: the measured table names only real registry modes, all under the limit except none", () => {
  for (const [mode, s] of Object.entries(GATE_SECONDS)) {
    assert.ok(registryNames.has(mode), `${mode} is not a test-local.sh registry mode (stale GATE_SECONDS entry)`);
    assert.ok(Number.isInteger(s) && s > 0 && s < BUNDLE_LIMIT_S, `${mode}: ${s}s`);
  }
});

test("CI-SPEED-2: gates.yml runs `jobs`, verifies receipts against `matrix`, and unrolls exactly BUNDLE_MAX per-member upload slots", () => {
  const plan = job("plan"), gate = job("gate"), receipts = job("receipts");
  assert.match(plan, /jobs: \$\{\{ steps\.plan\.outputs\.jobs \}\}/);
  assert.match(plan, /BUNDLE: \$\{\{ github\.event_name != 'workflow_dispatch' \|\| \(inputs\.full_browser_universe == 'true' && inputs\.extra_env == ''\) \}\}/);
  assert.match(gate, /include: \$\{\{ fromJSON\(needs\.plan\.outputs\.jobs\) \}\}/);
  assert.match(gate, /timeout-minutes: \$\{\{ matrix\.timeout \}\}/);
  assert.match(receipts, /PLANNED: \$\{\{ needs\.plan\.outputs\.matrix \}\}/, "receipts verify the planned MODES, never the job list");
  assert.match(gate, /timeout --kill-after=60s 150m bash -c 'set -eo pipefail; gate_one "\$1"' _ "\$m"/, "each member keeps the 150 min solo bound");
  assert.match(gate, /node scripts\/dev\/ci-playwright-browsers\.mjs \$MEMBERS/);
  for (let i = 0; i < BUNDLE_MAX; i++) {
    assert.ok(gate.includes(`        if: always() && matrix.bundle && matrix.members[${i}].art\n        uses: actions/upload-artifact@b7c566a772e6b6bfb58ed0dc250532a479d7789f # v6.0.0\n        with:\n          name: receipt-\${{ matrix.members[${i}].art }}\n          path: output/members/\${{ matrix.members[${i}].art }}/receipt/`), `receipt slot ${i}`);
    assert.ok(gate.includes(`          name: \${{ matrix.members[${i}].art }}\n          path: |\n            output/members/\${{ matrix.members[${i}].art }}/ci-gates/\n            output/members/\${{ matrix.members[${i}].art }}/playwright/`), `evidence slot ${i}`);
  }
  assert.ok(!gate.includes(`matrix.members[${BUNDLE_MAX}]`), "no slot beyond BUNDLE_MAX");
  // The solo receipt + evidence steps never run in a bundle job (a receipt for mode "bundle:NN" would be unplanned = red).
  assert.match(gate, /- name: Run receipt\n        if: always\(\) && !matrix\.bundle\n/);
  assert.match(gate, /if: always\(\) && !matrix\.bundle\n        with:\n          name: receipt-\$\{\{ matrix\.art \}\}/);
  assert.match(gate, /if: always\(\) && !matrix\.bundle && !startsWith\(matrix\.mode, '--browser-visual-lint'\)/);
  // One receipt command for solo and bundle members: same writer, same log globs.
  const write = "node scripts/dev/matrix-receipts.mjs write output/receipt output/ci-gates/*.log output/playwright/*/ci-gates/*.log output/ci-gates/**/*.jsonl output/playwright/**/*.jsonl";
  assert.equal(gate.split(write).length - 1, 2, "the solo step and the bundle loop write receipts with the identical command");
});

// ---- the real Gate step, executed ----
const gateScript = () => {
  const block = /\n      - name: Gate\n        id: gate\n        run: \|\n((?:          .*\n|\s*\n)+)/.exec(job("gate"))?.[1];
  assert.ok(block, "Gate step run block not found");
  return block.split("\n").map((l) => l.slice(10)).join("\n");
};

/** A throwaway checkout: the real scripts/ (registry + receipt writer), a stub xvfb-run that plays one browser gate, and the real Gate step. */
function runGate({ bundle, mode, members = [], fail = [], stdinEater = [] }) {
  const dir = mkdtempSync(path.join(tmpdir(), "gate-bundle-"));
  symlinkSync(path.join(repo, "scripts"), path.join(dir, "scripts"));
  const bin = path.join(dir, ".bin");
  mkdirSync(bin);
  writeFileSync(path.join(bin, "xvfb-run"), `#!/usr/bin/env bash
mode="\${!#}"
case " $STUB_STDIN " in *" $mode "*) cat >/dev/null ;; esac
mkdir -p "$LC_BROWSER_EVIDENCE_ROOT/stub"
printf '  \\u2713  1 [chromium] stub spec for %s (1ms)\\n' "$mode" > "$LC_BROWSER_EVIDENCE_ROOT/stub/spec.log"
echo "ran $mode" >> "$STUB_TRACE"
case " $STUB_FAIL " in *" $mode "*) echo "--- FAIL: TestBundleStub (0.00s)"; exit 1 ;; esac
echo "--- PASS: TestBundleStub (0.00s)"
`);
  chmodSync(path.join(bin, "xvfb-run"), 0o755);
  writeFileSync(path.join(dir, "gate.sh"), gateScript());
  const env = {
    ...process.env, PATH: `${bin}:${process.env.PATH}`, MODE: mode, BUNDLE: String(bundle), EXTRA_ENV: "",
    MEMBERS_JSON: JSON.stringify(members.map((m) => ({ mode: m, art: artifactName(m) }))),
    GITHUB_RUN_ID: "77", GITHUB_RUN_ATTEMPT: "1", GITHUB_SHA: "abc123", GITHUB_EVENT_NAME: "pull_request",
    STUB_FAIL: fail.join(" "), STUB_STDIN: stdinEater.join(" "), STUB_TRACE: path.join(dir, "trace.txt"),
  };
  delete env.LC_BROWSER_EVIDENCE_ROOT; delete env.RECEIPT_SPEC_ROOT;
  // GitHub runs an unspecified-shell step as `bash -e {0}`.
  const r = spawnSync("bash", ["-e", "gate.sh"], { cwd: dir, env, encoding: "utf8", input: "" });
  const receipts = [];
  const membersDir = path.join(dir, "output/members");
  if (existsSync(membersDir)) for (const art of readdirSync(membersDir)) {
    const rdir = path.join(membersDir, art, "receipt");
    if (existsSync(rdir)) for (const f of readdirSync(rdir)) receipts.push({ art, file: f, ...JSON.parse(readFileSync(path.join(rdir, f), "utf8")) });
  }
  const trace = existsSync(env.STUB_TRACE) ? readFileSync(env.STUB_TRACE, "utf8").trim().split("\n") : [];
  return { r, dir, receipts, trace };
}

const three = ["--browser-buyer", "--browser-storefront", "--browser-cvs"];
const planned = three.map((mode) => ({ mode, art: artifactName(mode) }));

test("CI-SPEED-2 executed: a green bundle writes one PASS receipt per member, in that member's own tree, and the receipts verifier is green", () => {
  const { r, receipts, trace, dir } = runGate({ bundle: true, mode: "bundle:01", members: three });
  assert.equal(r.status, 0, r.stdout + r.stderr);
  assert.deepEqual(trace, three.map((m) => `ran ${m}`));
  assert.equal(receipts.length, 3);
  for (const rc of receipts) {
    assert.equal(rc.file, receiptFile(rc.mode));
    assert.equal(rc.art, artifactName(rc.mode), "receipt sits in its own member tree");
    assert.equal(rc.status, "PASS");
    assert.equal(rc.spec_count, 1, `${rc.mode}: counts only its own spec log, not an earlier member's`);
    assert.ok(existsSync(path.join(dir, "output/members", rc.art, "playwright")), "evidence moved for the member's own artifact");
  }
  assert.deepEqual(verify(planned, receipts, { runId: "77", sha: "abc123" }), []);
});

test("CI-SPEED-2 executed: a red member gets its own FAIL receipt, every later member still runs, and the step goes red", () => {
  const { r, receipts, trace } = runGate({ bundle: true, mode: "bundle:01", members: three, fail: ["--browser-buyer"] });
  assert.notEqual(r.status, 0, "a bundle with a red member must fail the step (else `required` stays green)");
  assert.deepEqual(trace, three.map((m) => `ran ${m}`), "never stop at the first failure");
  const byMode = Object.fromEntries(receipts.map((x) => [x.mode, x.status]));
  assert.deepEqual(byMode, { "--browser-buyer": "FAIL", "--browser-storefront": "PASS", "--browser-cvs": "PASS" });
  assert.deepEqual(verify(planned, receipts, { runId: "77", sha: "abc123" }), ["--browser-buyer: status FAIL"]);
});

test("CI-SPEED-2 executed: a gate that reads stdin cannot swallow the remaining members", () => {
  const { r, trace } = runGate({ bundle: true, mode: "bundle:01", members: three, stdinEater: ["--browser-buyer"] });
  assert.equal(r.status, 0, r.stdout + r.stderr);
  assert.deepEqual(trace, three.map((m) => `ran ${m}`));
});

test("CI-SPEED-2 executed: a solo job runs its one mode in place (no member trees), and a red solo gate is red", () => {
  const green = runGate({ bundle: false, mode: "--browser-buyer" });
  assert.equal(green.r.status, 0, green.r.stdout + green.r.stderr);
  assert.deepEqual(green.trace, ["ran --browser-buyer"]);
  assert.ok(!existsSync(path.join(green.dir, "output/members")));
  assert.ok(readdirSync(path.join(green.dir, "output/playwright")).some((d) => d.startsWith("run.")), "solo evidence layout unchanged");
  const red = runGate({ bundle: false, mode: "--browser-buyer", fail: ["--browser-buyer"] });
  assert.notEqual(red.r.status, 0);
});
