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
// #147 CI-9 P1: one node --test invocation is the text between these markers, which the gates print around every
// `node --test` process (scripts/dev/test-local.sh, scripts/dev/test-node.sh). Keep the literals here: a parser/shell drift is a red test.
const BEGIN = "## matrix-receipts node-test begin", END = "## matrix-receipts node-test end";
// A COMPLETE Node v24 summary: eight ordered diagnostics (test.js), spec `ℹ` or TAP `# ` prefix throughout.
const summary = (pass, fail, prefix = "ℹ") => [`${prefix} tests ${pass + fail}`, `${prefix} suites 0`, `${prefix} pass ${pass}`, `${prefix} fail ${fail}`,
  `${prefix} cancelled 0`, `${prefix} skipped 0`, `${prefix} todo 0`, `${prefix} duration_ms 12.5`];
const invocation = (...blocks) => [BEGIN, ...blocks.flat(), END].join("\n");

test("test_count is read from go test -v and node --test output, not from a static list", () => {
  const log = ["=== RUN   TestA", "--- PASS: TestA (0.01s)", "    --- PASS: TestA/sub (0.00s)", "--- FAIL: TestB (0.02s)", "--- SKIP: TestC (0.00s)", "ok  pkg 1s",
    invocation(summary(4, 0)), invocation(summary(2, 1, "#")), "    --- PASS: indented child output is not top-level"].join("\n");
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

// #147 CI-9: a pass/fail line counts only inside the runner's own complete summary block, and only inside a marked invocation.
test("a pass line outside a node --test summary block does not count", () => {
  assert.equal(countTests("tool output\nℹ pass 99\n# pass 5\nℹ fail 7\n"), 0);
  // The invocation's count is its summary's pass+fail; a stray pass line inside the same invocation does not add to it.
  assert.equal(countTests(invocation([...summary(3, 0), "ℹ pass 50"])), 3, "only the summary's own pass line counts");
});

// #147 CI-9 P1 (review 172): a summary must be COMPLETE (all eight Node v24 diagnostics, one prefix) and it must be the
// LAST complete summary of a marked node --test invocation. Node's spec reporter forwards test stdout verbatim, so a
// printed excerpt of an earlier run is ordinary log content and must never inflate test_count.
test("an interior excerpt is not a summary: only the invocation's last complete summary counts", () => {
  // Codex's exact log shape: a truncated four-line excerpt printed by a test, then the runner's real final zero summary.
  assert.equal(countTests(invocation(
    "ℹ tests 5", "ℹ suites 0", "ℹ pass 5", "ℹ fail 0",
    "tool: excerpt ends",
    ...summary(0, 0),
  )), 0, "the four-line excerpt before the real zero summary contributes nothing");

  // Even a complete previous-run summary printed by a test is not the verdict; the invocation's final summary is.
  assert.equal(countTests(invocation(
    ...summary(5, 0),
    "tool: complete previous-run excerpt ends",
    ...summary(0, 0),
  )), 0, "the last complete summary of the invocation decides");

  // Two legitimate concatenated invocations each contribute their own final summary.
  assert.equal(countTests([BEGIN, ...summary(3, 0), END, BEGIN, ...summary(2, 0), END].join("\n")), 5, "concatenated invocations sum");
});

test("a truncated or mixed-prefix node --test summary fragment does not count", () => {
  assert.equal(countTests("tool output\nℹ suites 0\n# pass 5\n"), 0, "mixed reporter prefixes are not one summary");
  assert.equal(countTests(invocation("ℹ tests 3", "ℹ suites 0", "ℹ pass 3")), 0, "a fragment without the summary tail is incomplete");
  assert.equal(countTests(invocation("ℹ tests 5", "ℹ suites 0", "ℹ pass 5", "ℹ fail 0")), 0, "four lines are not a complete Node v24 summary");
  assert.equal(countTests(invocation("ℹ tests 2", "ℹ suites 0", "# pass 2", "ℹ fail 0", "ℹ cancelled 0", "ℹ skipped 0", "ℹ todo 0", "ℹ duration_ms 1")), 0,
    "the prefix must be consistent across all eight lines");
  assert.equal(countTests([...summary(3, 0)].join("\n")), 0, "a summary outside a marked invocation belongs to no runner invocation");
  assert.equal(countTests(invocation(summary(3, 0))), 3, "a complete summary inside its invocation still counts");
});

// The marker literals are a contract with the gate scripts; a shell/parser drift must fail here, not on a stranger's log.
test("both node gate scripts print the invocation markers this parser requires", () => {
  for (const script of ["scripts/dev/test-local.sh", "scripts/dev/test-node.sh"]) {
    const source = readFileSync(script, "utf8");
    assert.ok(source.includes(`'${BEGIN}'`), `${script} prints the begin marker`);
    assert.ok(source.includes(`'${END}'`), `${script} prints the end marker`);
  }
});

// #147 CI-6: a parsed failure forces FAIL even when the gate step itself exited 0.
test("status is FAIL when the logs show a failed test, whatever the gate outcome", () => {
  const rec = (log) => makeReceipt({ mode: "m", outcome: "success", runId: 1, runAttempt: 1, sha: "s", logs: [log] }).status;
  assert.equal(rec("--- PASS: TestA (0s)\n"), "PASS");
  assert.equal(rec("--- PASS: TestA (0s)\n--- FAIL: TestB (0s)\n"), "FAIL");
  assert.equal(rec(JSON.stringify({ Action: "fail", Test: "TestV" })), "FAIL");
  assert.equal(rec(invocation(summary(1, 1))), "FAIL");
  assert.equal(rec(invocation(summary(3, 1), ...summary(0, 0))), "PASS", "a failure inside a discarded excerpt is not the runner's verdict");
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
