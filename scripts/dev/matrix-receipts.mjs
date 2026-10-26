#!/usr/bin/env node
// Purpose: MATRIX-RECEIPT (#128). `write` turns one gates.yml matrix job's own gate log into `receipt-<mode>.json`
//   {mode, run_id, run_attempt, sha, status, test_count, spec_count}; `verify` proves every mode the plan job emitted has exactly one receipt
//   from THIS run and sha with status PASS and test_count > 0. Missing, duplicate, stale, unplanned, failed or zero-count = red.
//   test_count is read from the runners' own output in the log, never from a static list: top-level `--- PASS:` / `--- FAIL:` lines of
//   `go test -v` (the lines `go tool test2json` turns into pass/fail events; test-focused.sh counts the same), top-level pass/fail events of
//   `go test -json` logs (--browser-webkit / --stripe-browser write one .jsonl per step instead of -v text), plus node --test summaries
//   (`ℹ` spec / `#` TAP): only the LAST COMPLETE summary of one marked `node --test` invocation counts, and only when it has all
//   eight Node v24 diagnostics with one prefix (#147 CI-9 P1). The invocation boundary is the marker pair printed by the gates'
//   own `node` wrapper (scripts/dev/test-local.sh, scripts/dev/test-node.sh); an excerpt a test printed before the real summary,
//   a truncated fragment, or a summary outside any invocation can never inflate test_count.
//   Browser specs run inside go tests, so a browser mode's Go harness PASS alone does not prove a spec ran (#147 CI-7): the
//   Playwright list-reporter result lines are counted from the job's own `*.log` evidence under RECEIPT_SPEC_ROOT
//   (default output/playwright), and a browser mode (registry arm with lc_browsers) with no parsed spec/node result is FAIL.
//   Reporter precedence is per execution, not per job (#184 review P2): each evidence log and each lc_run gate-log segment
//   picks glyphs, else custom case lines, else its summary, and distinct executions are summed; a gate log that tees one of
//   its own evidence executions is that execution, not another one. A browser mode's lc_prepare runs unit/model node suites
//   first, so a node summary is evidence only when the suite marks its own browser execution and its summary sits after the
//   gate's browser-run marker (review 184 P1a); a node browser runner may instead print PASS/FAIL case lines to the gate log
//   after that marker. Original and durable copies of one evidence tree share a per-run directory identity and count once (P2).
//   status is FAIL when the gate step failed OR a parsed failure (Go `--- FAIL:` / JSON fail / bare `FAIL<TAB>`, node summary
//   fail>0, Playwright `N failed` / spec ✘) was seen, even if the gate exited 0 (#147 CI-6); an explicit `0 failed` is not
//   a failure. Historical evidence never feeds this.
// Depends on: scripts/dev/ci-plan.mjs (artifactName), scripts/dev/pr-modes.mjs (modeEntries, read-only mode registry);
//   env MODE, GATE_OUTCOME, GITHUB_RUN_ID, GITHUB_RUN_ATTEMPT, GITHUB_SHA, RECEIPT_SPEC_ROOT (write); env PLANNED (the plan
//   job's `matrix` output), GITHUB_RUN_ID, GITHUB_SHA (verify). No network, no third-party dependencies.
// Used by: .github/workflows/gates.yml (gate job "Run receipt" step, `receipts` job feeding `required`), tests/ci/matrix-receipts.test.mjs.
// Usage: node scripts/dev/matrix-receipts.mjs write <out-dir> <gate-log | go-test-json-log>...   |   node scripts/dev/matrix-receipts.mjs verify <receipts-dir>
import { mkdirSync, readFileSync, readdirSync, writeFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { artifactName } from "./ci-plan.mjs";
import { modeEntries } from "./pr-modes.mjs";

export const receiptFile = (mode) => `receipt-${artifactName(mode).replace(/^gate-/, "")}.json`;

// #147 CI-9 P1: the two marker lines that bracket one `node --test` process in a gate log. tests/ci/matrix-receipts.test.mjs
// asserts both gate scripts still print these exact literals, so parser and shell can never drift apart silently.
export const NODE_TEST_BEGIN = "## matrix-receipts node-test begin";
export const NODE_TEST_END = "## matrix-receipts node-test end";
// #147 round 2 (review 184 P1a): test-local.sh prints this immediately before the mode's lc_run, so node:test summaries
// from lc_prepare prerequisites (returns-model/returns-request, unit BFF suites) are never browser evidence.
export const BROWSER_RUN_BEGIN = "## matrix-receipts browser-run begin";
// A node:test suite that really drives Playwright (tests/admin/picklist.spec.ts) prints this while it runs; only such an
// invocation's summary can satisfy CI-7 for a node browser mode. Unit suites print neither it nor a browser.
export const BROWSER_SPEC_MARK = "## matrix-receipts browser-spec";
// Node v24's summary is EIGHT ordered diagnostics (lib/internal/test_runner/test.js): tests, suites, pass, fail, cancelled,
// skipped, todo, duration_ms; spec uses `ℹ`, TAP uses `# `, and the prefix is constant within one summary. Anything shorter
// (a four-line excerpt), longer/mixed, or differently numbered is not that runner's summary. Post-summary failure details are
// ordinary following lines, so the block is not required to reach EOF.
const NODE_SUMMARY = /(?:^|\n)(ℹ|#) tests (?<tests>\d+)\n\1 suites (?<suites>\d+)\n\1 pass (?<pass>\d+)\n\1 fail (?<fail>\d+)\n\1 cancelled (?<cancelled>\d+)\n\1 skipped (?<skipped>\d+)\n\1 todo (?<todo>\d+)\n\1 duration_ms (?<duration_ms>\d+(?:\.\d+)?)(?=\n|$)/g;

/** The text of each CLOSED begin/end marker pair = one node --test invocation; a log cut short before its end marker yields none. */
function nodeInvocations(lines) {
  const invocations = [];
  let start = -1, beginAt = -1, depth = 0;
  for (let i = 0; i < lines.length; i++) {
    if (lines[i] === NODE_TEST_BEGIN) { if (depth++ === 0) { start = i + 1; beginAt = i; } }
    else if (lines[i] === NODE_TEST_END && depth > 0) { if (--depth === 0) { invocations.push({ text: lines.slice(start, i).join("\n"), at: beginAt }); start = -1; } }
  }
  return invocations;
}

/** The gate log's browser-run phase: text after the marker test-local.sh prints immediately before lc_run. */
function afterBrowserRun(text) {
  const lines = text.split(/\r?\n/), at = lines.indexOf(BROWSER_RUN_BEGIN);
  return at < 0 ? "" : lines.slice(at + 1).join("\n");
}

/** Tests that ran according to the runners' own output in one log text, and whether any top-level one failed (#147 CI-6/CI-9). */
export function scanTests(text) {
  let n = 0, failed = false, go = 0, node = 0, nodeRun = 0;
  const lines = text.split(/\r?\n/);
  const runAt = lines.indexOf(BROWSER_RUN_BEGIN);
  for (const line of lines) {
    if (/^--- (PASS|FAIL): /.test(line)) { n++; go++; if (line.startsWith("--- FAIL")) failed = true; }
    if (line.startsWith("{")) {
      try {
        const e = JSON.parse(line);
        if ((e.Action === "pass" || e.Action === "fail") && e.Test && !e.Test.includes("/")) { n++; go++; if (e.Action === "fail") failed = true; }
      } catch { /* not a go test -json event */ }
    }
    // #147 CI-6: a package-level go test failure line (`FAIL\tpkg`, or `FAIL` alone) is a parsed failure, not only `--- FAIL:`.
    if (/^FAIL(?:\t|$)/.test(line)) failed = true;
    // #147 CI-6: Playwright's list-reporter summary ("  1 failed"); round 2: only a POSITIVE count is a failure, so a
    // reporter that explicitly prints "0 failed" next to "N passed" stays green.
    const summaryFail = /^\s*(\d+) failed\b/.exec(line);
    if (summaryFail && Number(summaryFail[1]) > 0) failed = true;
  }
  // #147 CI-9 P1: one invocation's count is its LAST complete summary, never an earlier excerpt and never a fragment.
  for (const invocation of nodeInvocations(lines)) {
    let last = null;
    for (const summary of invocation.text.matchAll(NODE_SUMMARY)) last = summary;
    if (!last) continue;
    const count = Number(last.groups.pass) + Number(last.groups.fail);
    n += count; node += count;
    if (runAt >= 0 && invocation.at > runAt && invocation.text.includes(BROWSER_SPEC_MARK)) nodeRun += count;
    if (Number(last.groups.fail) > 0) failed = true;
  }
  return { n, failed, go, node, nodeRun };
}
export const countTests = (text) => scanTests(text).n;

/**
 * Playwright list-reporter result lines (#147 CI-7). A browser mode's Go harness is only the driver; the specs it actually ran are
 * the `✓`/`✘` result lines the gate's runners wrote into its own evidence logs, so a harness PASS with zero of them is not a
 * passing test. Some browser runners (storefront gate .mjs, the joint merchant/buyer gate) print one `PASS <case>` / `FAIL <case>`
 * line per case instead; a log with no Playwright glyphs falls back to those case lines (see makeReceipt).
 * A run with a failed spec/case is a parsed failure (#147 CI-6).
 */
export function scanSpecs(text) {
  let glyphs = 0, caseLines = 0, summaryPass = 0, summaryFail = 0, failed = false;
  for (const line of text.split(/\r?\n/)) {
    // Playwright's list reporter uses ✓/✘; node's spec reporter uses ✔/✖, which must not be mistaken for a spec.
    if (/^\s+✓\s+\d+\s/.test(line)) glyphs++;
    else if (/^\s+✘\s+\d+\s/.test(line)) { glyphs++; failed = true; }
    // A harness that withholds the raw reporter output writes only the final `23 passed` / `1 failed` summary.
    else if (/^\s*(\d+) passed\b/.test(line)) summaryPass += Number(line.match(/^\s*(\d+)/)[1]);
    // #147 round 2 (review 184 P1b): a summary that explicitly says "0 failed" is a pass; only a positive count is a failure.
    else if (/^\s*(\d+) failed\b/.test(line)) {
      const failures = Number(line.match(/^\s*(\d+)/)[1]);
      summaryFail += failures;
      if (failures > 0) failed = true;
    }
    // Custom storefront/browser runners print one `PASS <case>` / `FAIL <case>` line per case instead of a reporter.
    else if (/^PASS\s/.test(line)) caseLines++;
    else if (/^FAIL\s/.test(line)) { caseLines++; failed = true; }
  }
  // Per log one reporter decides: glyphs, else case lines, else the summary total (so a banner cannot double-count).
  return { glyphs, cases: caseLines || summaryPass + summaryFail, failed };
}

/** The result lines of one execution, in order: Playwright glyphs or a custom runner's PASS/FAIL case lines. */
function resultLines(text) {
  return text.split(/\r?\n/).filter((line) => /^\s*[✓✘]\s+\d+\s/.test(line) || /^(PASS|FAIL)\s/.test(line));
}

/** True when `lines` is one contiguous block of `other`; a tee copies the execution's own output verbatim. */
function isTee(lines, other) {
  if (!lines.length || lines.length > other.length) return false;
  return other.some((_, start) => start + lines.length <= other.length && lines.every((line, i) => line === other[start + i]));
}

/**
 * #147 round 2 (review 184 P1a): a Go harness may run the real browser suite as `node --test <spec>` with its stdout
 * redirected into the harness's own evidence log (input-delivery stop/revoke, studio-bff browser). That file is the
 * suite's own output, not a gate log mixing unrelated test stdout, and the suite prints BROWSER_SPEC_MARK itself. Its
 * LAST complete runner summary is that execution's spec count (#172 semantics); a log without the marker (next.log,
 * build.log) contributes nothing, so a unit/model suite cannot smuggle results in.
 */
function scanBrowserNodeLog(text) {
  if (!text.includes(BROWSER_SPEC_MARK)) return { n: 0, failed: false };
  let last = null;
  for (const summary of text.matchAll(NODE_SUMMARY)) last = summary;
  if (!last) return { n: 0, failed: false };
  return { n: Number(last.groups.pass) + Number(last.groups.fail), failed: Number(last.groups.fail) > 0 };
}

/**
 * Is this mode's registry arm a browser mode? Read from test-local.sh's MODE REGISTRY (the same data pr-modes.mjs uses), never a
 * second hand-maintained list (#147 CI-7). A plan fan-out suffix (`--browser-click-sweep@3/10`) maps to its bare mode.
 */
export function isBrowserMode(mode) {
  const base = String(mode ?? "").replace(/@\d+\/\d+$/, "");
  if (!base) return false;
  return modeEntries(readFileSync(new URL("./test-local.sh", import.meta.url), "utf8")).some((e) => e.name === base && /lc_browsers=/.test(e.body));
}

/** The gate's own evidence logs under its browser evidence root, excluding the logs already passed on the CLI (#147 CI-7). */
export function specLogFiles(root, alreadyRead = []) {
  const seen = new Set(alreadyRead.map((f) => path.resolve(f)));
  let entries = [];
  try { entries = readdirSync(root, { withFileTypes: true, recursive: true }); } catch { return []; }
  const files = entries.filter((d) => d.isFile() && /\.log$/.test(d.name))
    .map((d) => path.join(d.parentPath, d.name)).filter((f) => !seen.has(path.resolve(f))).sort();
  // #147 round 2 (review 184 P2): meta-health-ui keeps the original evidence tree and os.CopyFS-duplicates it under a
  // durable sibling, so both trees carry the same per-run directory name and file name: that pair is ONE execution.
  // The identity is the layout, never the bytes — independently timestamped runs have different directories and add up,
  // even when their output is identical.
  const canonical = new Map();
  for (const file of files) {
    const identity = `${path.basename(path.dirname(file))}/${path.basename(file)}`;
    if (!canonical.has(identity)) canonical.set(identity, file);
  }
  return [...canonical.values()];
}

// status is the gate's outcome AND the logs: a failed top-level test in a gate that exited 0 is still FAIL (#147 CI-6).
// A browser mode also needs at least one parsed spec/node result: its Go harness alone can pass with zero specs (#147 CI-7).
// run_attempt is recorded but deliberately not compared by verify: after `gh run rerun --failed` the jobs that were not rerun keep their attempt-1 receipts (same run_id and sha).
export function makeReceipt({ mode, outcome, runId, runAttempt, sha, logs, specs = [], browser = false }) {
  const scans = logs.map(scanTests), specScans = specs.map(scanSpecs);
  // #184 review P2: reporter precedence belongs to ONE execution. Each evidence log keeps its own glyph/case/summary
  // order (scanSpecs.cases already prefers case lines over a summary), then the distinct executions are summed; the old
  // job-wide `glyphs || cases` dropped a whole second reporter format (2 glyphs + 3 custom cases counted 2, not 5).
  const nodeScans = specs.map(scanBrowserNodeLog);
  const evidenceLines = specs.map(resultLines);
  const evidenceCounts = specScans.map((scan) => scan.glyphs || scan.cases);
  const evidenceSpecs = specs.reduce((sum, _text, i) => sum + (evidenceCounts[i] || nodeScans[i].n), 0);
  // #147 round 2: a node browser runner (storefront/shell/platform/gallery) prints its PASS/FAIL case lines to the gate log
  // itself, so the mode's lc_run phase is evidence too. A gate log that repeats the result lines (or the exact summary count)
  // of one of its own evidence executions is a tee of that execution, never a second execution; a gate-log execution with
  // different lines or a different count still adds, even when one line happens to be identical.
  const runScans = browser ? logs.map((text) => scanSpecs(afterBrowserRun(text))) : [];
  const runLines = browser ? logs.map((text) => resultLines(afterBrowserRun(text))) : [];
  const runSpecs = browser ? runScans.reduce((sum, scan, i) => {
    const count = scan.glyphs || scan.cases;
    if (!count) return sum;
    const teed = evidenceCounts.some((evidenceCount, j) => evidenceCount === count
      && (runLines[i].length ? isTee(runLines[i], evidenceLines[j]) : evidenceLines[j].length === 0));
    return sum + (teed ? 0 : count);
  }, 0) : 0;
  const specCount = evidenceSpecs + runSpecs;
  const testCount = scans.reduce((s, x) => s + x.n, 0) + specCount;
  // Tests the gate printed that are not this browser mode's Go harness: browser spec/case results, plus a node --test
  // suite that marked its own browser execution inside lc_run. lc_prepare prerequisites and run-phase unit suites never count.
  const nodeRun = scans.reduce((s, x) => s + x.nodeRun, 0);
  const nonHarness = nodeRun + specCount;
  const failed = scans.some((s) => s.failed) || specScans.some((s) => s.failed) || nodeScans.some((s) => s.failed) || runScans.some((s) => s.failed);
  const status = outcome === "success" && !failed && !(browser && nonHarness === 0) ? "PASS" : "FAIL";
  return { mode, run_id: String(runId), run_attempt: String(runAttempt), sha, status, test_count: testCount, spec_count: specCount };
}

/** planned: [{mode}] from ci-plan.mjs; receipts: parsed objects. Returns the list of problems (empty = green). */
export function verify(planned, receipts, { runId, sha }) {
  const problems = [], byMode = new Map();
  for (const r of receipts) byMode.set(r?.mode, [...(byMode.get(r?.mode) ?? []), r]);
  const want = new Set(planned.map((e) => e.mode));
  if (!want.size) problems.push("plan emitted no modes");
  for (const mode of want) {
    const got = byMode.get(mode) ?? [];
    if (got.length !== 1) { problems.push(`${mode}: ${got.length ? `${got.length} receipts (duplicate)` : "no receipt (job wrote none)"}`); continue; }
    const r = got[0];
    if (r.run_id !== String(runId)) problems.push(`${mode}: stale receipt from run ${r.run_id}, this run is ${runId}`);
    if (r.sha !== sha) problems.push(`${mode}: receipt sha ${r.sha} != run sha ${sha}`);
    if (r.status !== "PASS") problems.push(`${mode}: status ${r.status}`);
    if (!Number.isInteger(r.test_count) || r.test_count <= 0) problems.push(`${mode}: test_count ${r.test_count} (no test ran)`);
  }
  for (const [mode] of byMode) if (!want.has(mode)) problems.push(`unplanned receipt for mode ${JSON.stringify(mode)}`);
  return problems;
}

const walk = (dir) => readdirSync(dir, { withFileTypes: true, recursive: true }).filter((d) => d.isFile() && /^receipt-.*\.json$/.test(d.name)).map((d) => path.join(d.parentPath, d.name));

export function main(argv = process.argv.slice(2), env = process.env) {
  const [cmd, dir, ...logs] = argv;
  if (cmd === "write" && dir) {
    // #147 CI-7: browser specs are printed to the gate's own evidence logs, not the gate log itself; count the job's spec logs.
    const browser = isBrowserMode(env.MODE);
    const specLogs = browser ? specLogFiles(env.RECEIPT_SPEC_ROOT ?? "output/playwright", logs) : [];
    const r = makeReceipt({ mode: env.MODE, outcome: env.GATE_OUTCOME, runId: env.GITHUB_RUN_ID, runAttempt: env.GITHUB_RUN_ATTEMPT, sha: env.GITHUB_SHA, logs: logs.map((f) => readFileSync(f, "utf8")), specs: specLogs.map((f) => readFileSync(f, "utf8")), browser });
    if (!r.mode || !r.sha || r.run_id === "undefined") throw new Error("MODE, GITHUB_RUN_ID and GITHUB_SHA are required");
    mkdirSync(dir, { recursive: true });
    writeFileSync(path.join(dir, receiptFile(r.mode)), `${JSON.stringify(r)}\n`);
    console.log(`matrix-receipts: ${JSON.stringify(r)}`);
    return 0;
  }
  if (cmd === "verify" && dir) {
    let files = [];
    try { files = walk(dir); } catch { /* no receipts downloaded at all: every planned mode is reported missing */ }
    const receipts = files.map((f) => { try { return JSON.parse(readFileSync(f, "utf8")); } catch { return { mode: `<unparsable ${f}>` }; } });
    const problems = verify(JSON.parse(env.PLANNED ?? ""), receipts, { runId: env.GITHUB_RUN_ID, sha: env.GITHUB_SHA });
    for (const r of receipts) console.log(`  receipt ${r.mode}: ${r.status} test_count=${r.test_count} run=${r.run_id}/${r.run_attempt} sha=${r.sha}`);
    for (const p of problems) console.error(`FAIL: ${p}`);
    console.log(problems.length ? `matrix-receipts: RED (${problems.length} problem(s))` : `matrix-receipts: PASS (${receipts.length} receipt(s) match the plan)`);
    return problems.length ? 1 : 0;
  }
  console.error("usage: matrix-receipts.mjs write <out-dir> <gate-log>... | verify <receipts-dir>");
  return 2;
}
if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try { process.exit(main()); } catch (e) { console.error(`matrix-receipts: ${e.message}`); process.exit(1); }
}
