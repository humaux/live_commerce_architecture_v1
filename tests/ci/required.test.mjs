// Purpose: CI-SPEED-520 combines receipt verification and the stable required verdict on one runner without relaxing the gate.
// Depends on: gates.yml, the real matrix receipt writer/verifier, Node24 and bash. No network, PG, browser, package install or Git checkout.
// Used by: scripts/dev/test-node.sh. These fixtures exercise real local commands, not GitHub scheduling/cancellation itself.
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { makeReceipt } from "../../scripts/dev/matrix-receipts.mjs";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const wf = readFileSync(path.join(root, ".github/workflows/gates.yml"), "utf8");
const job = (name) => wf.split(new RegExp(`^  ${name}:\\s*$`, "m"))[1]?.split(/^  [\w-]+:\s*$/m)[0] ?? "";
const required = job("required");
const stepBlocks = required.split(/^      - /m).slice(1);
const verifyStep = stepBlocks.find((s) => /^name: Every planned mode has one fresh PASS receipt with tests\n/.test(s)) ?? "";
const verdictStep = stepBlocks.find((s) => /^name: Required verdict\n/.test(s)) ?? "";
const verdict = /        run: \|\n([\s\S]*)$/.exec(verdictStep)?.[1].split("\n").map((s) => s.slice(10)).join("\n");
const verifier = /^        run: (.+)$/m.exec(verifyStep)?.[1];
const identity = { GITHUB_RUN_ID: "520", GITHUB_RUN_ATTEMPT: "2", GITHUB_SHA: "a".repeat(40) };
const defaults = { FOUNDATION: "success", FOUNDATION_EXPECTED: "true", GATE_EXPECTED: "true", RECEIPT_DOWNLOAD: "success", PLAN: "success", BUILD: "skipped", BUILD_EXPECTED: "false", GATE: "success", AGG: "skipped", AGG_EXPECTED: "false", DEPLOY: "skipped", DEPLOY_EXPECTED: "false", RECEIPTS: "success" };
const receipt = (mode, extra = {}) => ({ mode, run_id: identity.GITHUB_RUN_ID, run_attempt: "1", sha: identity.GITHUB_SHA, status: "PASS", test_count: 2, spec_count: 1, ...extra });

function runVerdict(env = {}) {
  assert.ok(verdict, "required verdict shell must exist");
  const r = spawnSync("bash", ["-e", "-c", verdict], { env: { ...process.env, ...defaults, ...env }, encoding: "utf8" });
  assert.equal(r.error, undefined);
  return r;
}

// Run the exact workflow verification command against downloaded-artifact-shaped fixture directories,
// then feed its real outcome into the exact final verdict shell. Each test removes only its own fixture.
function runRequired(modes, receipts, env = {}) {
  assert.equal(verifier, "node scripts/dev/matrix-receipts.mjs verify receipts");
  const dir = mkdtempSync(path.join(tmpdir(), "required-520-"));
  try {
    for (const [i, r] of receipts.entries()) {
      const artifact = path.join(dir, "receipts", `receipt-gate-${i}`);
      mkdirSync(artifact, { recursive: true });
      writeFileSync(path.join(artifact, "receipt-fixture.json"), typeof r === "string" ? r : JSON.stringify(r));
    }
    // Only the script path changes to address the repository from the isolated fixture working directory.
    const command = verifier.replace("scripts/dev/matrix-receipts.mjs", JSON.stringify(path.join(root, "scripts/dev/matrix-receipts.mjs")));
    const verify = spawnSync("bash", ["-e", "-c", command], { cwd: dir, env: { ...process.env, ...identity, PLANNED: JSON.stringify(modes.map((mode) => ({ mode }))) }, encoding: "utf8" });
    assert.equal(verify.error, undefined);
    const foundation = modes.some((m) => /^shard:(unit|g[0-9]{2})$/.test(m));
    const gate = modes.some((m) => !/^shard:(unit|g[0-9]{2})$/.test(m));
    const result = runVerdict({ FOUNDATION: foundation ? "success" : "skipped", FOUNDATION_EXPECTED: String(foundation), GATE: gate ? "success" : "skipped", GATE_EXPECTED: String(gate), ...env, RECEIPTS: verify.status === 0 ? "success" : "failure" });
    return { verify, result };
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
}

test("CI-SPEED-520: one always-running stable required job owns receipt verification and the final verdict", () => {
  assert.equal(job("receipts"), "", "no separate receipt runner");
  assert.equal((wf.match(/^    name: required$/gm) ?? []).length, 1);
  assert.match(required, /^    needs: \[plan, build, foundation_gate, gate, aggregate, deploy\]$/m);
  assert.match(required, /^    if: \$\{\{ always\(\) \}\}$/m);
  assert.match(required, /^      FOUNDATION: \$\{\{ needs\.foundation_gate\.result \}\}$/m);
  assert.match(required, /^      FOUNDATION_EXPECTED: \$\{\{ needs\.plan\.outputs\.foundation_jobs != '\[\]' \}\}$/m);
  assert.match(required, /^      GATE: \$\{\{ needs\.gate\.result \}\}$/m);
  assert.match(required, /^      GATE_EXPECTED: \$\{\{ needs\.plan\.outputs\.gate_jobs != '\[\]' \}\}$/m);
  assert.doesNotMatch(required.split("    steps:")[0], /continue-on-error/);
  assert.match(verifyStep, /^        id: receipts$/m);
  assert.match(verifyStep, /^        if: \$\{\{ always\(\) && needs\.plan\.result == 'success' \}\}$/m);
  assert.match(verifyStep, /^          PLANNED: \$\{\{ needs\.plan\.outputs\.matrix \}\}$/m);
  assert.doesNotMatch(verifyStep, /continue-on-error/);
  assert.match(verdictStep, /^        if: \$\{\{ always\(\) \}\}$/m);
  assert.match(verdictStep, /^          RECEIPTS: \$\{\{ steps\.receipts\.outcome \}\}$/m);
  assert.doesNotMatch(verdictStep, /^        continue-on-error:/m);
  assert.doesNotMatch(required, /steps\.receipts\.conclusion|needs\.receipts/);
  const download = stepBlocks.find((s) => /^uses: actions\/download-artifact@/.test(s));
  assert.ok(download);
  assert.match(download, /^        id: receipts_download$/m);
  assert.match(verdictStep, /^          RECEIPT_DOWNLOAD: \$\{\{ steps\.receipts_download\.outcome \}\}$/m);
  assert.match(job("foundation_gate"), /^      fail-fast: false$/m);
  assert.match(download, /^          pattern: receipt-gate-\*$/m);
  assert.match(download, /^          path: receipts$/m);
  assert.doesNotMatch(download, /merge-multiple|run-id|github-token/, "same-run separate artifact directories preserve duplicate detection");
  assert.match(job("gate"), /^      fail-fast: false$/m);
  assert.match(readFileSync(path.join(root, "scripts/dev/test-node.sh"), "utf8"), /tests\/ci\/required\.test\.mjs/);
});

test("CI-SPEED-520: every mandatory result rejects failure, cancellation, skipped and missing outcomes", () => {
  assert.equal(runVerdict().status, 0);
  for (const name of ["PLAN", "RECEIPTS", "RECEIPT_DOWNLOAD"]) {
    for (const value of ["failure", "cancelled", "skipped", ""]) {
      const r = runVerdict({ [name]: value });
      assert.notEqual(r.status, 0, `${name}=${value}: ${r.stdout}${r.stderr}`);
    }
  }
  // For build, both execution lanes, shard aggregation and deployment, expected means success and unexpected means skipped, never the reverse.
  for (const name of ["BUILD", "FOUNDATION", "GATE", "AGG", "DEPLOY"]) {
    for (const expected of ["true", "false"]) for (const value of ["success", "failure", "cancelled", "skipped", ""]) {
      const r = runVerdict({ [name]: value, [`${name}_EXPECTED`]: expected });
      assert.equal(r.status === 0, value === (expected === "true" ? "success" : "skipped"), `${name}=${value}, expected=${expected}`);
    }
  }
});

test("CI-SPEED-520: solo, bundle members, full shard aggregation and deploy plans keep the same green path", () => {
  const plans = [
    { modes: ["shard:unit"], env: {} },
    { modes: ["--browser-buyer", "--browser-storefront", "--browser-cvs"], env: { BUILD: "success", BUILD_EXPECTED: "true" } },
    { modes: ["--browser-click-sweep@1/2", "--browser-click-sweep@2/2", "--browser-visual-lint@1/2", "--browser-visual-lint@2/2"], env: { BUILD: "success", BUILD_EXPECTED: "true", AGG: "success", AGG_EXPECTED: "true" } },
    { modes: ["shard:unit", "shard:g01"], env: { DEPLOY: "success", DEPLOY_EXPECTED: "true" } },
  ];
  for (const { modes, env } of plans) {
    const r = runRequired(modes, modes.map((m) => receipt(m)), env);
    assert.equal(r.verify.status, 0, r.verify.stdout + r.verify.stderr);
    assert.equal(r.result.status, 0, r.result.stdout + r.result.stderr);
  }
});

test("CI-SPEED-520: missing, partial, duplicate, stale, malformed, empty, unplanned and failed receipts all reject", () => {
  const modes = ["shard:unit", "shard:g01"];
  const cases = [[], [receipt(modes[0])], [receipt(modes[0]), receipt(modes[0]), receipt(modes[1])],
    [receipt(modes[0], { run_id: "519" }), receipt(modes[1])], [receipt(modes[0], { sha: "b".repeat(40) }), receipt(modes[1])],
    ...[0, -1, 0.5, "2", null].map((test_count) => [receipt(modes[0], { test_count }), receipt(modes[1])]),
    ...["FAIL", "", "CANCELLED"].map((status) => [receipt(modes[0], { status }), receipt(modes[1])]),
    [receipt(modes[0]), receipt(modes[1]), receipt("bundle:01")], ["{not-json", receipt(modes[1])]];
  for (const receipts of cases) {
    const r = runRequired(modes, receipts);
    assert.notEqual(r.verify.status, 0, JSON.stringify(receipts));
    assert.notEqual(r.result.status, 0, JSON.stringify(receipts));
  }
  assert.notEqual(runRequired([], []).result.status, 0, "an empty plan is never evidence of success");
});

test("CI-SPEED-520: failed-job reruns accept earlier attempts of this run/SHA only", () => {
  const modes = ["--browser-buyer", "--browser-storefront"];
  const receipts = [receipt(modes[0], { run_attempt: "1" }), receipt(modes[1], { run_attempt: "2" })];
  assert.equal(runRequired(modes, receipts).result.status, 0);
  for (const stale of [{ run_id: "519" }, { sha: "b".repeat(40) }]) {
    assert.notEqual(runRequired(modes, [receipt(modes[0], { run_attempt: "1", ...stale }), receipts[1]]).result.status, 0);
  }
});

test("CI-SPEED-520: a failed/cancelled matrix or missing browser execution stays red with otherwise good evidence", () => {
  const mode = "--browser-buyer";
  for (const GATE of ["failure", "cancelled", "skipped", ""]) assert.notEqual(runRequired([mode], [receipt(mode)], { GATE }).result.status, 0);
  const browserOnlyHarness = makeReceipt({ mode, outcome: "success", runId: identity.GITHUB_RUN_ID, runAttempt: "2", sha: identity.GITHUB_SHA,
    logs: ["--- PASS: TestBrowserHarness (0.1s)"], specs: [], browser: true });
  assert.equal(browserOnlyHarness.status, "FAIL");
  assert.notEqual(runRequired([mode], [browserOnlyHarness]).result.status, 0);
});

// The Cartesian product below exhausts both optional lanes, the artifact download,
// verification and planning outcomes. The existing 27,000-case oracle separately
// exhausts build/aggregate/deploy dependencies with both lanes required.
test("CI-SPEED integration: every two-lane and download/verifier result combination is fail-closed", () => {
  const R = ["success", "skipped", "failure", "cancelled", ""], B = ["true", "false"], rows = [];
  for (const FOUNDATION of R) for (const GATE of R) for (const FOUNDATION_EXPECTED of B) for (const GATE_EXPECTED of B)
    for (const RECEIPT_DOWNLOAD of R) for (const RECEIPTS of R) for (const PLAN of R)
      rows.push({ FOUNDATION, GATE, FOUNDATION_EXPECTED, GATE_EXPECTED, RECEIPT_DOWNLOAD, RECEIPTS, PLAN });
  const keys = Object.keys(rows[0]);
  const command = `req() { ( ${verdict}\n) >/dev/null; }\nwhile IFS='|' read -r ${keys.join(" ")}; do export ${keys.join(" ")}; req; echo $?; done`;
  const r = spawnSync("bash", ["-c", command], { env: { ...process.env, ...defaults }, input: rows.map((row) => keys.map((k) => row[k]).join("|")).join("\n") + "\n", encoding: "utf8", maxBuffer: 16 << 20 });
  assert.equal(r.error, undefined);
  assert.equal(r.status, 0, r.stderr);
  const outcomes = r.stdout.trim().split("\n").map(Number);
  assert.equal(outcomes.length, rows.length);
  for (const [i, c] of rows.entries()) {
    const ok = c.PLAN === "success" && c.RECEIPT_DOWNLOAD === "success" && c.RECEIPTS === "success"
      && c.FOUNDATION === (c.FOUNDATION_EXPECTED === "true" ? "success" : "skipped")
      && c.GATE === (c.GATE_EXPECTED === "true" ? "success" : "skipped");
    assert.equal(outcomes[i] === 0, ok, JSON.stringify(c));
  }
  assert.ok(outcomes.includes(0), "the allowed green paths are reachable");
});

test("CI-SPEED integration: both lanes must supply every planned receipt and finish successfully", () => {
  const modes = ["shard:unit", "--browser-cvs"], receipts = modes.map((m) => receipt(m));
  assert.equal(runRequired(modes, receipts).result.status, 0);
  for (const missing of [receipts.slice(0, 1), receipts.slice(1)]) assert.notEqual(runRequired(modes, missing).result.status, 0);
  for (const name of ["FOUNDATION", "GATE", "RECEIPT_DOWNLOAD"]) {
    for (const result of ["failure", "cancelled", "skipped", ""]) {
      assert.notEqual(runRequired(modes, receipts, { [name]: result }).result.status, 0, `${name}=${result}`);
    }
  }
});
