// Purpose: unit tests of the MATRIX-RECEIPT verifier (scripts/dev/matrix-receipts.mjs, #128): test_count comes from runner output only, and the verifier is red for a
//   missing receipt, a wrong sha, a stale run id, test_count 0, a duplicate, a FAIL status and an unplanned mode; green only for one matching PASS receipt per planned mode.
// Depends on: scripts/dev/matrix-receipts.mjs, node:test. Used by: scripts/dev/test-node.sh (CI Foundation checks).
import assert from "node:assert/strict";
import { mkdtempSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import { countTests, main, makeReceipt, receiptFile, verify } from "../../scripts/dev/matrix-receipts.mjs";

const RUN = { runId: "4242", sha: "a".repeat(40) };
const planned = [{ mode: "shard:unit" }, { mode: "shard:g01" }];
const ok = (mode, extra = {}) => ({ mode, run_id: RUN.runId, run_attempt: "1", sha: RUN.sha, status: "PASS", test_count: 3, ...extra });

test("test_count is read from go test -v and node --test output, not from a static list", () => {
  const log = ["=== RUN   TestA", "--- PASS: TestA (0.01s)", "    --- PASS: TestA/sub (0.00s)", "--- FAIL: TestB (0.02s)", "--- SKIP: TestC (0.00s)", "ok  pkg 1s",
    "ℹ tests 4", "ℹ suites 0", "ℹ pass 4", "ℹ fail 0", "# tests 3", "# suites 0", "# pass 2", "# fail 1", "    --- PASS: indented child output is not top-level"].join("\n");
  assert.equal(countTests(log), 2 + 4 + 3);
  assert.equal(countTests("=== RUN   TestA\n--- SKIP: TestA (0.00s)\nPASS\n"), 0, "a run where nothing executed counts 0");
  // go test -json (--browser-webkit writes one .jsonl per step): top-level pass/fail events count; subtests, skips, output and package events do not.
  const json = [{ Action: "run", Test: "TestW" }, { Action: "output", Test: "TestW", Output: "--- PASS: TestW (1s)\n" }, { Action: "pass", Test: "TestW/SU07" }, { Action: "pass", Test: "TestW" },
    { Action: "fail", Test: "TestV" }, { Action: "skip", Test: "TestS" }, { Action: "pass", Package: "p" }].map((e) => JSON.stringify(e)).join("\n");
  assert.equal(countTests(json), 2);
  assert.equal(countTests("{not json\n"), 0);
  assert.equal(makeReceipt({ mode: "shard:unit", outcome: "success", runId: 1, runAttempt: 2, sha: "s", logs: [log, "--- PASS: TestZ (0s)"] }).test_count, 10);
  assert.equal(makeReceipt({ mode: "m", outcome: "failure", runId: 1, runAttempt: 1, sha: "s", logs: [] }).status, "FAIL");
});

// #147 CI-9: a pass/fail line counts only inside the runner's own summary block (tests, suites, pass, fail), not anywhere in a log.
test("a pass line outside a node --test summary block does not count", () => {
  assert.equal(countTests("tool output\nℹ pass 99\n# pass 5\nℹ fail 7\n"), 0);
  assert.equal(countTests("ℹ tests 3\nℹ suites 0\nℹ pass 3\nℹ fail 0\nℹ pass 50\n"), 3, "only the block's own pass line counts");
});

// #147 CI-9: a summary is one complete, same-prefix block at line boundaries; a fragment pasted into tool output cannot add tests.
test("a truncated or mixed-prefix node --test summary fragment does not count", () => {
  assert.equal(countTests("tool output\nℹ suites 0\n# pass 5\n"), 0, "mixed reporter prefixes are not one summary");
  assert.equal(countTests("ℹ suites 0\nℹ pass 5\nℹ fail 0\n"), 0, "a fragment without the tests header is not a summary");
  assert.equal(countTests("ℹ tests 3\nℹ suites 0\nℹ pass 3\n"), 0, "a summary without its fail tail is incomplete");
  assert.equal(countTests("ℹ tests 2\nℹ suites 0\n# pass 2\nℹ fail 0\n"), 0, "the prefix must be consistent across the block");
  assert.equal(countTests("ℹ tests 3\nℹ suites 0\nℹ pass 3\nℹ fail 0\n"), 3, "a complete summary still counts");
});

// #147 CI-6: a parsed failure forces FAIL even when the gate step itself exited 0.
test("status is FAIL when the logs show a failed test, whatever the gate outcome", () => {
  const rec = (log) => makeReceipt({ mode: "m", outcome: "success", runId: 1, runAttempt: 1, sha: "s", logs: [log] }).status;
  assert.equal(rec("--- PASS: TestA (0s)\n"), "PASS");
  assert.equal(rec("--- PASS: TestA (0s)\n--- FAIL: TestB (0s)\n"), "FAIL");
  assert.equal(rec(JSON.stringify({ Action: "fail", Test: "TestV" })), "FAIL");
  assert.equal(rec("ℹ tests 2\nℹ suites 0\nℹ pass 1\nℹ fail 1\n"), "FAIL");
  assert.equal(rec("    --- FAIL: TestA/sub (0s)\n--- PASS: TestA (0s)\n"), "PASS", "only top-level failures decide; a subtest failure fails its parent");
});

test("green only when every planned mode has exactly one PASS receipt from this run and sha with tests", () => {
  assert.deepEqual(verify(planned, [ok("shard:unit"), ok("shard:g01")], RUN), []);
});

test("red: a planned mode whose job wrote no receipt", () => {
  assert.match(verify(planned, [ok("shard:unit")], RUN).join("\n"), /shard:g01: no receipt/);
  assert.equal(verify(planned, [], RUN).length, 2);
});

test("red: a receipt with the wrong sha, or stale from another run", () => {
  assert.match(verify(planned, [ok("shard:unit"), ok("shard:g01", { sha: "b".repeat(40) })], RUN).join("\n"), /shard:g01: receipt sha b+ != run sha/);
  assert.match(verify(planned, [ok("shard:unit", { run_id: "4241" }), ok("shard:g01")], RUN).join("\n"), /shard:unit: stale receipt from run 4241/);
});

test("red: test_count 0 (for example a filter that selects nothing)", () => {
  assert.match(verify(planned, [ok("shard:unit"), ok("shard:g01", { test_count: 0 })], RUN).join("\n"), /shard:g01: test_count 0/);
  assert.equal(verify(planned, [ok("shard:unit"), ok("shard:g01", { test_count: "3" })], RUN).length, 1, "a non-integer count is not trusted");
});

test("red: a duplicate receipt, a FAIL status, an unplanned mode, an empty plan", () => {
  assert.match(verify(planned, [ok("shard:unit"), ok("shard:g01"), ok("shard:g01")], RUN).join("\n"), /shard:g01: 2 receipts \(duplicate\)/);
  assert.match(verify(planned, [ok("shard:unit"), ok("shard:g01", { status: "FAIL" })], RUN).join("\n"), /status FAIL/);
  assert.match(verify(planned, [ok("shard:unit"), ok("shard:g01"), ok("shard:g99")], RUN).join("\n"), /unplanned receipt/);
  assert.deepEqual(verify([], [], RUN), ["plan emitted no modes"]);
});

test("CLI: write a receipt from a gate log, then verify the downloaded artifact tree against the plan", () => {
  const dir = mkdtempSync(path.join(tmpdir(), "receipts-")), log = path.join(dir, "gate.log");
  writeFileSync(log, "--- PASS: TestA (0s)\n");
  const env = { MODE: "shard:unit", GATE_OUTCOME: "success", GITHUB_RUN_ID: RUN.runId, GITHUB_RUN_ATTEMPT: "1", GITHUB_SHA: RUN.sha };
  const out = path.join(dir, "receipt-gate-shard_unit");
  assert.equal(main(["write", out, log], env), 0);
  assert.equal(JSON.parse(readFileSync(path.join(out, receiptFile("shard:unit")), "utf8")).test_count, 1);
  assert.equal(receiptFile("--browser-click-sweep@3/10"), "receipt-browser-click-sweep_3_10.json");
  const venv = { PLANNED: JSON.stringify([{ mode: "shard:unit", art: "gate-shard_unit" }]), GITHUB_RUN_ID: RUN.runId, GITHUB_SHA: RUN.sha };
  assert.equal(main(["verify", dir], venv), 0);
  assert.equal(main(["verify", dir], { ...venv, GITHUB_SHA: "c".repeat(40) }), 1);
  assert.equal(main(["verify", path.join(dir, "absent")], venv), 1, "no artifacts downloaded at all is red");
});
