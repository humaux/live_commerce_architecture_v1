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
//   its own evidence executions is that execution, not another one. Equal summary counts alone never merge two executions
//   (#190 review P2): a summary-only tee is accepted only when the evidence log's whole text, including a non-summary runner
//   identity line, is one contiguous block of the gate's browser-run phase. A browser mode's lc_prepare runs unit/model node suites
//   first, so a node summary is evidence only when the suite marks its own browser execution and its summary sits after the
//   gate's browser-run marker (review 184 P1a); a node browser runner may instead print PASS/FAIL case lines to the gate log
//   after that marker. Original and durable copies of one evidence tree share a per-run directory identity and count once (P2).
//   status is FAIL when the gate step failed OR a parsed failure (Go `--- FAIL:` / JSON fail / bare `FAIL<TAB>`, node summary
//   fail>0, Playwright `N failed` / spec ✘) was seen, even if the gate exited 0 (#147 CI-6); an explicit `0 failed` is not
//   a failure. Historical evidence never feeds this.
// Depends on: scripts/dev/ci-plan.mjs (artifactName), scripts/dev/pr-modes.mjs (modeEntries, read-only mode registry);
//   env MODE, GATE_OUTCOME, GITHUB_RUN_ID, GITHUB_RUN_ATTEMPT, GITHUB_SHA, RECEIPT_SPEC_ROOT (write); env PLANNED (the plan
//   job's `matrix` output), GITHUB_RUN_ID, GITHUB_SHA (verify). Ordinary paths have no network.
//   canonical-* additionally uses trusted workflow context, Git/Go inventory and read-only GitHub artifact transport; stdlib only.
// Used by: .github/workflows/gates.yml (gate job "Run receipt" step, `receipts` job feeding `required`), tests/ci/matrix-receipts.test.mjs.
// Usage: node scripts/dev/matrix-receipts.mjs write <out-dir> <gate-log | go-test-json-log>...   |   node scripts/dev/matrix-receipts.mjs verify <receipts-dir>
import { mkdirSync, readFileSync, readdirSync, writeFileSync, existsSync } from "node:fs";
import path from "node:path";
import { isDeepStrictEqual } from "node:util";
import { createHash } from "node:crypto";
import { execFileSync } from "node:child_process";
import { check as checkShards, selectedBy } from "./shard-plan.mjs";
import { expand } from "./ci-plan.mjs";
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
// The lines a summary-only reporter contributes; every other non-empty line is a runner identity/marker (#190 review P2).
const SUMMARY_COUNT_LINE = /^\s*\d+ (?:passed|failed)\b/;

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

/** One log as trimmed non-empty lines; a summary-only tee is proven against the whole evidence log, not its count. */
function logLines(text) {
  return text.split(/\r?\n/).map((line) => line.trim()).filter(Boolean);
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
  // itself, so the mode's lc_run phase is evidence too. A gate log that repeats one of its own evidence executions is that
  // execution, never a second one; a gate-log execution with different lines still adds.
  // #190 review P2: a matching summary COUNT is not an execution identity. Summary-only logs carry no result lines, so the
  // old `evidenceLines[j].length === 0` branch merged two independent equal-sized runs (23 + 23 became 23). Dedup now needs
  // proof of a tee: the evidence log's whole text, including at least one non-summary runner line, is one contiguous block
  // of the gate's browser-run phase. Bare equal counts always add; failure flags are still collected from every log.
  const runTexts = browser ? logs.map((text) => afterBrowserRun(text)) : [];
  const runScans = runTexts.map(scanSpecs);
  const runLines = runTexts.map(resultLines);
  const runTextLines = runTexts.map(logLines);
  const evidenceTextLines = specs.map(logLines);
  const evidenceIdentityLines = evidenceTextLines.map((lines) => lines.filter((line) => !SUMMARY_COUNT_LINE.test(line)));
  const summaryOnly = specScans.map((scan, j) => scan.glyphs === 0 && evidenceLines[j].length === 0);
  const runSpecs = browser ? runScans.reduce((sum, scan, i) => {
    const count = scan.glyphs || scan.cases;
    if (!count) return sum;
    const teed = evidenceCounts.some((evidenceCount, j) => {
      if (evidenceCount !== count) return false;
      if (runLines[i].length || evidenceLines[j].length) return runLines[i].length > 0 && isTee(runLines[i], evidenceLines[j]);
      return summaryOnly[j] && evidenceIdentityLines[j].length > 0
        && isTee(evidenceTextLines[j], runTextLines[i]);
    });
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


// Canonical G07 is opt-in; ordinary receipt/rerun semantics above are unchanged.
export const G07_C1 = "TestOrderAttemptDeadlineRealClaimPeriodicLiveness";
export const G07_MATRIX = "TestProductEditorMatrixLedgerEvidence";
export const G07_NODE = ["ledger", "execution", "controls"];
export const g07Hash = (bytes) => createHash("sha256").update(bytes).digest("hex");
const json = (file) => JSON.parse(readFileSync(file, "utf8"));
const save = (file, value) => writeFileSync(file, `${JSON.stringify(value)}\n`);
const equal = isDeepStrictEqual;
const need = (ok, why) => { if (!ok) throw new Error(`canonical G07: ${why}`); };
const clock = () => Math.floor(Date.now() / 1000);
const git = (...args) => execFileSync("git", args, { encoding: "utf8", timeout: 10000 }).trim();
const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const sourceNames = () => {
  const src = readFileSync(path.join(ROOT, "tests/foundation/catalog_matrix_ledger_test.go"), "utf8");
  const body = /var productEditorMatrixEvidenceSources = \[\]string\{([\s\S]*?)\n\}/.exec(src)?.[1];
  need(body && !body.replace(/"[^"\n]+",?\s*/g, "").trim(), "invalid owner source inventory");
  const names = [...body.matchAll(/"([^"\n]+)"/g)].map((m) => m[1]);
  need(names.length && new Set(names).size === names.length, "empty/duplicate owner inventory");
  return names;
};
export function g07Budget(anchor, deadline, now, reserve = 0) {
  need(Number.isSafeInteger(anchor) && Number.isSafeInteger(deadline) && deadline > anchor && deadline - anchor <= 9000, "invalid shared deadline");
  need(Number.isSafeInteger(now) && now >= anchor && now + reserve < deadline, "attempt deadline expired or clock invalid");
  return deadline - now - reserve;
}
function g07Context(env = process.env) {
  need(env.GITHUB_EVENT_NAME === "workflow_dispatch" && env.LC_G07_CANONICAL === "1", "not canonical dispatch");
  need(!env.EXTRA_ENV && (!env.GOFLAGS || env.GOFLAGS === "-mod=readonly"), "uncontrolled flags/extra_env");
  const identity = { subject_sha: env.GITHUB_SHA, workflow_sha: env.GITHUB_WORKFLOW_SHA, workflow_ref: env.GITHUB_WORKFLOW_REF,
    run_id: env.GITHUB_RUN_ID, run_attempt: env.GITHUB_RUN_ATTEMPT, event: env.GITHUB_EVENT_NAME,
    checkout_sha: git("rev-parse", "HEAD"), checkout_tree: git("rev-parse", "HEAD^{tree}") };
  for (const [k, v] of Object.entries(identity)) need(typeof v === "string" && v && v !== "unknown", `missing ${k}`);
  for (const k of ["subject_sha", "workflow_sha", "checkout_sha", "checkout_tree"]) need(/^[a-f0-9]{40}$/.test(identity[k]), `invalid ${k}`);
  need(/^[1-9][0-9]*$/.test(identity.run_id) && /^[1-9][0-9]*$/.test(identity.run_attempt), "invalid run/attempt");
  // Dispatch checks out its exact subject. A future tree-equivalence policy must be explicit, not inferred here.
  need(identity.checkout_sha === identity.subject_sha, "dispatch checkout differs from subject");
  need(!git("status", "--porcelain", "--untracked-files=normal").split("\n").some((line) => line && !line.startsWith("?? output/")), "source dirty/untracked outside evidence");
  const runners = [".github/workflows/gates.yml", "scripts/dev/release-gate.sh", "scripts/dev/matrix-receipts.mjs", "scripts/dev/test-local.sh", "scripts/dev/test-local-runtime.sh", "scripts/dev/test-node.sh", "scripts/dev/shard-plan.mjs", "scripts/dev/ci-plan.mjs"];
  const pins = Object.fromEntries(runners.map((p) => [p, g07Hash(readFileSync(path.join(ROOT, p)))]));
  return { identity, pins, clean: true, source: g07Hash(git("ls-tree", "-r", "HEAD")), plan_hash: g07Hash(readFileSync(path.join(ROOT, "scripts/dev/shard-plan.json"))) };
}
export function g07Events(text, inventory) {
  const packages = new Map(), tests = new Map(), events = [];
  need(typeof text === "string" && text.endsWith("\n") && text.trim(), "missing/truncated Go JSON");
  for (const line of text.trimEnd().split("\n")) {
    const e = JSON.parse(line); events.push(e);
    need(e && typeof e.Package === "string" && Object.hasOwn(inventory, e.Package), "unplanned/missing package attribution");
    const p = packages.get(e.Package) ?? { start: 0, end: 0, terminal: "" };
    if (!e.Test) {
      if (e.Action === "start") { need(p.start === 0 && p.end === 0, "duplicate/restarted package"); p.start++; }
      if (["pass", "skip", "fail"].includes(e.Action)) { need(p.start === 1 && p.end === 0, "unstarted/duplicate package terminal"); p.end++; p.terminal = e.Action; }
      need(!["fail", "build-fail"].includes(e.Action), `package/build failure ${e.Package}`);
      if (e.Action === "output") need(!/WARNING: DATA RACE|race detected during execution|^FAIL(?:\s|$)/m.test(e.Output ?? ""), "race/package failure output");
    } else {
      need(typeof e.Test === "string" && !/[\r\n\t]/.test(e.Test), "invalid test identity");
      need(p.start === 1 && p.end === 0, "test outside active package");
      const key = `${e.Package}\t${e.Test}`, t = tests.get(key) ?? { package: e.Package, name: e.Test, start: 0, end: 0, terminal: "", output: "" };
      if (e.Action === "run") { need(t.end === 0, `case restarted ${key}`); t.start++; }
      if (e.Action === "output") t.output += e.Output ?? "";
      if (["pass", "skip", "fail"].includes(e.Action)) { need(t.start === 1 && t.end === 0, `unstarted/duplicate terminal ${key}`); t.end++; t.terminal = e.Action; }
      need(e.Action !== "fail", `test/subtest failure ${key}`);
      tests.set(key, t);
    }
    packages.set(e.Package, p);
  }
  for (const [pkg, expected] of Object.entries(inventory)) {
    const p = packages.get(pkg);
    need(p?.start === 1 && p.end === 1 && (p.terminal === "pass" || (!expected.length && p.terminal === "skip")), `missing/truncated package ${pkg}`);
    const actual = [...tests.values()].filter((t) => t.package === pkg && !t.name.includes("/")).map((t) => t.name).sort();
    need(equal(actual, [...expected].sort()), `incomplete/extra/misassigned cases ${pkg}`);
  }
  for (const t of tests.values()) need(t.start === 1 && t.end === 1, `unclosed/duplicate case ${t.name}`);
  return { tests: [...tests.values()], events };
}
export function g07Node(text) {
  need(typeof text === "string" && text.endsWith("\n") && (text.match(/^TAP version 13$/gm) ?? []).length === 1, "missing Node TAP");
  const plans = [...text.matchAll(/^1\.\.(\d+)$/gm)], terminals = [...text.matchAll(/^ok (\d+) - .+$/gm)];
  need(plans.length === 1 && Number(plans[0][1]) > 0 && terminals.length === Number(plans[0][1]) && terminals.every((t, i) => Number(t[1]) === i + 1 && t.index < plans[0].index), "missing/truncated/duplicate Node terminals");
  const summaries = [...text.matchAll(NODE_SUMMARY)];
  need(summaries.length === 1 && summaries[0].index > plans[0].index, "missing/duplicate Node summary");
  const g = summaries[0].groups;
  need(Number(g.pass) > 0 && Number(g.tests) === Number(g.pass) && ["fail", "cancelled", "skipped", "todo"].every((k) => Number(g[k]) === 0), "Node not complete PASS");
  need(!/^\s*not ok\b|# (?:SKIP|TODO)\b|['"]only['"].*--test-only/im.test(text), "Node skip/todo/only/failure");
}
function g07OwnerIdentity(proof, expected, mode) {
  need(proof?.schema === "matrix-owner-proof/v1", "owner proof schema");
  for (const [k, v] of Object.entries(expected.identity)) need(proof.identity?.[k] === v, `owner ${k} mismatch`);
  need(proof.identity.tracked_clean === "true" && proof.identity.job === "gate" && proof.identity.mode === mode, "owner job/mode/clean mismatch");
}
export function g07Fresh(fresh, binding, expected) {
  g07OwnerIdentity(fresh, expected, "--browser-product-editor");
  need(fresh.kind === "fresh-ledger" && fresh.validation === "LEDGER_VALIDATED" && fresh.identity.test === "TestBrowserCatalogCore", "wrong fresh owner");
  need(fresh.identity.evidence === binding.evidence && binding.evidence && binding.evidence !== "unknown", "fresh evidence location");
  need(equal(fresh.files, binding.fresh_files), "fresh ledger/case digest mismatch");
  need(fresh.sources?.length === binding.names.length, "fresh inventory size");
  for (const [i, name] of binding.names.entries()) need(fresh.sources[i].path === name && fresh.sources[i].actual === binding.actual[name] && fresh.sources[i].recorded === "unknown", "fresh source binding");
  need(binding.browser_pass === true, "missing browser terminal PASS");
}
export function g07Matrix(canonical, fresh, binding, expected) {
  g07OwnerIdentity(canonical, expected, "foundation"); g07OwnerIdentity(fresh, expected, "--browser-product-editor");
  need(canonical.kind === "canonical" && canonical.validation === "HISTORICAL_SOURCE_DRIFT" && canonical.identity.test === G07_MATRIX, "wrong canonical owner");
  need(canonical.identity.evidence === "unknown", "canonical evidence kind");
  need(fresh.kind === "fresh-ledger" && fresh.validation === "LEDGER_VALIDATED" && fresh.identity.test === "TestBrowserCatalogCore", "wrong fresh owner");
  need(fresh.identity.evidence === binding.evidence && binding.evidence && binding.evidence !== "unknown", "fresh evidence location");
  need(equal(canonical.files, binding.canonical_files), "canonical artifact digest mismatch");
  need(equal(fresh.files, binding.fresh_files), "fresh ledger/case digest mismatch");
  const drift = [];
  for (const [i, name] of binding.names.entries()) {
    const c = canonical.sources?.[i], f = fresh.sources?.[i];
    need(c?.path === name && f?.path === name && /^[a-f0-9]{64}$/.test(binding.recorded[name] ?? ""), "owner inventory mismatch");
    need(c.recorded === binding.recorded[name] && c.actual === binding.actual[name] && f.actual === binding.actual[name] && f.recorded === "unknown", "owner source hash mismatch");
    if (binding.recorded[name] !== binding.actual[name]) drift.push(name);
  }
  need(canonical.sources.length === binding.names.length && fresh.sources.length === binding.names.length && drift.length > 0, "matching/missing/extra source drift");
  const reason = `canonical evidence is historical (source drift: ${drift.join(", ")})`;
  need(equal(canonical.drift, drift) && canonical.reason === reason && binding.reason === reason, "wrong historical reason/drift");
  need(binding.browser_pass === true, "missing browser terminal PASS");
}
// Pure verifier: trusted is constructed from workflow context/current checkout and official transport,
// never deserialized from the untrusted plan/receipt. Returned raw JSON preserves every original SKIP.
export function verifyG07(plan, shards, trusted) {
  g07Budget(trusted.anchor, trusted.deadline, trusted.now);
  need(plan.schema === "g07-shards/v1" && equal(plan.context, trusted.context) && plan.anchor === trusted.anchor && plan.deadline === trusted.deadline, "plan identity/deadline mismatch");
  need(equal(plan.modes, trusted.modes) && equal(plan.inventory, trusted.inventory), "plan inventory/modes mismatch");
  need(shards.length === trusted.modes.length && new Set(shards.map((s) => s.mode)).size === shards.length, "missing/duplicate shard");
  const foundation = Object.keys(plan.inventory).find((p) => p.endsWith("/tests/foundation"));
  need(foundation, "missing foundation inventory");
  const all = [], events = []; let canonical = null, reason = "", browserPass = false, fresh = null, unit = null, nativeEvidence = "";
  for (const shard of shards) {
    need(trusted.modes.includes(shard.mode), "unplanned shard");
    need(shard.official_pass === true, "official job not successful");
    for (const [phase, witness] of [["start", shard.start], ["end", shard.end]]) {
      need(witness?.phase === phase && witness.mode === shard.mode && equal(witness.context, trusted.context), "pre/post identity/source/plan drift");
      need(witness.anchor === trusted.anchor && witness.deadline === trusted.deadline && Number.isSafeInteger(witness.at) && witness.at >= trusted.anchor && witness.at < trusted.deadline, "invalid witness deadline");
    }
    need(shard.end.at >= shard.start.at && shard.end.exit === 0, "false exit/interrupted shard");
    const assigned = shard.mode === "shard:unit" ? Object.fromEntries(Object.entries(plan.inventory).filter(([p]) => p !== foundation))
      : { [foundation]: shard.mode === "--browser-product-editor" ? ["TestBrowserCatalogCore"] : plan.inventory[foundation].filter((t) => selectedBy(trusted.shardPlan, t).includes(shard.mode.slice(6))) };
    const parsed = g07Events(shard.go, assigned);
    if (shard.mode === "--browser-product-editor") {
      const native = parsed.tests.find((t) => t.name === "TestBrowserCatalogCore");
      browserPass = native?.terminal === "pass" && parsed.tests.every((t) => t.terminal === "pass");
      const locations = [...(native?.output ?? "").matchAll(/^\s*browser_catalog_core_test.go:\d+: PASS mobile matrix click ledger: \d+ declared unique PASS identities; evidence=([^\n]+)\/product-editor-click-ledger.json$/gm)];
      need(locations.length === 1, "missing/duplicate owning native evidence location");
      nativeEvidence = path.posix.basename(locations[0][1]);
      need(/^[A-Za-z0-9_.-]+$/.test(nativeEvidence) && shard.end.evidence === nativeEvidence, "native event/end location mismatch");
      fresh = shard.fresh;
      need(!shard.vet, "vet outside unit");
      continue;
    }
    all.push(...parsed.tests); events.push(...parsed.events);
    if (shard.mode === "shard:unit") {
      unit = shard;
      const vet = shard.vet;
      need(Array.isArray(vet) && vet.length === 2 && vet[0].phase === "start" && vet[1].phase === "end" && vet[1].exit === 0 && vet.every((v) => Number.isSafeInteger(v.at) && v.command === "go vet ./..." && v.at >= shard.start.at && v.at <= shard.end.at) && vet[1].at >= vet[0].at, "missing/duplicate/failed/interrupted whole vet");
      for (const name of G07_NODE) g07Node(shard.node?.[name]);
    } else need(!shard.vet, "multiple whole vet");
    for (const t of parsed.tests) {
      if (t.package === foundation && t.name === G07_MATRIX && t.terminal === "skip") {
        const facts = [...t.output.matchAll(/^\s*catalog_matrix_ledger_test.go:\d+: MATRIX_OWNER_PROOF (\{[^\n]+\})$/gm)];
        need(facts.length === 1, "missing/duplicate owning canonical proof"); canonical = JSON.parse(facts[0][1]);
        reason = /^\s*catalog_matrix_ledger_test.go:\d+: (canonical evidence is historical \(source drift: [^\n]+\))$/m.exec(t.output)?.[1] ?? "";
      } else if (t.name.startsWith("TestProductEditorMatrix")) need(t.terminal === "pass", "other matrix SKIP");
    }
  }
  need(unit && all.some((t) => t.package === foundation && t.name === G07_C1 && t.terminal === "pass"), "missing/wrong native C1");
  if (canonical) g07Matrix(canonical, fresh, { ...trusted.binding, evidence: nativeEvidence, browser_pass: browserPass, reason }, trusted.context);
  // Browser qualification is required even when canonical sources happen to match.
  g07Fresh(fresh, { ...trusted.binding, evidence: nativeEvidence, browser_pass: browserPass }, trusted.context);
  return { events, conditional: canonical ? `${foundation}\t${G07_MATRIX}` : "" };
}


// Inventory is emitted by the actual compiled Go test binaries (-list, never -run).
export function g07Inventory(text, packageText = null) {
  const inventory = {}, terminals = new Map();
  need(text.endsWith("\n"), "truncated inventory");
  for (const line of text.trimEnd().split("\n")) {
    const e = JSON.parse(line); need(typeof e.Package === "string", "inventory package missing");
    inventory[e.Package] ??= [];
    need(!["fail", "build-fail"].includes(e.Action), "inventory compile failure");
    if (!e.Test && ["pass", "skip"].includes(e.Action)) terminals.set(e.Package, (terminals.get(e.Package) ?? 0) + 1);
    if (e.Action === "output" && /^(?:Test|Example|Fuzz)\S*\n$/.test(e.Output ?? "")) inventory[e.Package].push(e.Output.trim());
  }
  need(Object.keys(inventory).length > 0, "empty inventory");
  for (const [pkg, names] of Object.entries(inventory)) {
    need(terminals.get(pkg) === 1 && new Set(names).size === names.length, "inventory incomplete/duplicate"); names.sort();
  }
  if (packageText !== null) {
    need(typeof packageText === "string" && packageText.endsWith("\n"), "missing package inventory");
    const packages = packageText.trimEnd().split("\n");
    need(packages.length && new Set(packages).size === packages.length && equal(packages.sort(), Object.keys(inventory).sort()), "compiled/package inventory mismatch");
  }
  return Object.fromEntries(Object.entries(inventory).sort(([a], [b]) => a.localeCompare(b)));
}
function g07PlanShape(inventory) {
  const shardPlan = json(path.join(ROOT, "scripts/dev/shard-plan.json"));
  const foundation = Object.keys(inventory).find((p) => p.endsWith("/tests/foundation"));
  need(foundation && inventory[foundation].length, "empty compiled foundation inventory");
  // The actual Go build constraints select the universe; existing planner selectors partition it.
  const findings = checkShards(shardPlan, inventory[foundation]);
  need(findings.problems.length === 0, findings.problems.join("; "));
  return { shardPlan, modes: expand(["foundation-shards", "--browser-product-editor"], { groupIds: shardPlan.groups.map((g) => g.id) }).matrix.map((m) => m.mode) };
}
const g07Artifact = (mode, attempt) => `g07-a${attempt}-${mode === "plan" ? "plan" : artifactName(mode)}`;
function g07Binding(freshDir) {
  const names = sourceNames(), actual = Object.fromEntries(names.map((p) => [p, g07Hash(readFileSync(path.join(ROOT, p)))]));
  const deliveryPath = "output/matrix-ledger-enforce/DELIVERY.md", delivery = readFileSync(path.join(ROOT, deliveryPath), "utf8");
  const leaf = /\[SOURCE\.json\]\(([A-Za-z0-9._-]+)\/SOURCE\.json\)/.exec(delivery)?.[1]; need(leaf, "missing canonical location");
  const dir = `output/matrix-ledger-enforce/${leaf}`;
  const paths = [deliveryPath, `${dir}/browser/browser-evidence.json`, `${dir}/ARTIFACTS.json`, `${dir}/SOURCE.json`];
  const canonical_files = Object.fromEntries(paths.map((p) => [p, g07Hash(readFileSync(path.join(ROOT, p)))]));
  const source = json(path.join(ROOT, dir, "SOURCE.json"));
  const casePath = "tests/foundation/testdata/product_editor_matrix_cases.json";
  const fresh_files = { "product-editor-click-ledger.json": g07Hash(readFileSync(path.join(freshDir, "ledger.json"))), [casePath]: actual[casePath] };
  return { names, actual, recorded: source.source_files_sha256, canonical_files, fresh_files };
}
// Same archive checker is exercised by synthetic corruption controls; no network in this helper.
export const G07_ARCHIVE_PY = String.raw`
import hashlib,io,pathlib,stat,zipfile
def g07_unpack(data,target,digest,remaining):
 assert len(data)<=128*1024*1024 and 'sha256:'+hashlib.sha256(data).hexdigest()==digest
 seen=set(); total=0
 with zipfile.ZipFile(io.BytesIO(data)) as archive:
  for info in archive.infolist():
   name=info.filename; p=pathlib.PurePosixPath(name)
   assert str(p)==name.rstrip('/') and str(p) not in ('','.') and name not in seen and not p.is_absolute() and '..' not in p.parts and '\\' not in name and not stat.S_ISLNK(info.external_attr>>16)
   seen.add(name); total+=info.file_size; assert total<=remaining
   if info.is_dir(): continue
   body=archive.read(info) # CRC checked even when the GitHub digest matches malformed ZIP bytes
   dest=target.joinpath(*p.parts); dest.parent.mkdir(parents=True,exist_ok=True); dest.write_bytes(body)
 return total
`;
// REST is read-only. The archive digest comes from GitHub, not a manifest in that archive.
// Python stdlib verifies ZIP CRC and rejects traversal, symlinks, duplicate names and oversized data.
// Credentials are used only on api.github.com; signed archive redirects receive no Authorization header.
function g07Transport(dir, modes, env, budget) {
  need(!existsSync(dir), "transport destination must be new");
  const script = String.raw`
import json,os,sys,urllib.request,urllib.error,urllib.parse,time,datetime
${G07_ARCHIVE_PY}
out=pathlib.Path(sys.argv[1]); modes=json.loads(sys.argv[2]); deadline=int(os.environ['LC_G07_DEADLINE'])
repo=os.environ['GITHUB_REPOSITORY']; run=os.environ['GITHUB_RUN_ID']; attempt=os.environ['GITHUB_RUN_ATTEMPT']
headers={'Authorization':'Bearer '+os.environ['GITHUB_TOKEN'],'Accept':'application/vnd.github+json','X-GitHub-Api-Version':'2026-03-10'}
assert all(c.isalnum() or c in '/_-' or c=='.' for c in repo) and repo.count('/')==1
base='https://api.github.com/repos/'+repo
class NoRedirect(urllib.request.HTTPRedirectHandler):
 def redirect_request(self,*args): return None
opener=urllib.request.build_opener(NoRedirect)
def request(url, auth=True):
 assert time.time()<deadline
 return opener.open(urllib.request.Request(url,headers=headers if auth else {}),timeout=min(30,max(1,deadline-time.time())))
def get(route):
 with request(base+route) as r: return json.load(r)
def pages(route,key):
 result=[]
 for page in range(1,101):
  data=get(route+('?page=' if '?' not in route else '&page=')+str(page)+'&per_page=100')[key]; result+=data
  if len(data)<100:return result
 raise ValueError('pagination exceeds bound')
jobs=pages('/actions/runs/'+run+'/attempts/'+attempt+'/jobs','jobs')
arts=pages('/actions/runs/'+run+'/artifacts','artifacts')
expected=json.loads(sys.argv[3]); wanted={name:mode for name,mode in expected}
selected=[a for a in arts if a['name'].startswith('g07-a'+attempt+'-')]
assert len(selected)==len(wanted) and len({a['name'] for a in selected})==len(selected) and set(a['name'] for a in selected)==set(wanted)
out.mkdir(parents=True); receipt=[]; total=0
for a in selected:
 mode=wanted[a['name']]
 matching=[j for j in jobs if (j['name']=='plan' if mode=='plan' else any(s['name']=='Canonical evidence '+mode and s['conclusion']=='success' for s in j.get('steps',[])))]
 assert len(matching)==1
 job=matching[0]; assert job['status']=='completed' and job['conclusion']=='success' and str(job['run_id'])==run
 # The attempt-specific endpoint is authoritative; job run_attempt is not a documented required field.
 assert 'run_attempt' not in job or str(job['run_attempt'])==attempt
 stamp=lambda value: datetime.datetime.fromisoformat(value.replace('Z','+00:00')).timestamp()
 assert stamp(job['completed_at'])<deadline and stamp(job['completed_at'])>=int(os.environ['LC_G07_ANCHOR'])
 assert mode=='plan' or stamp(job['started_at'])>=int(os.environ['LC_G07_ANCHOR'])
 assert job['head_sha']==os.environ['GITHUB_SHA'] and not a['expired'] and a['workflow_run']['head_sha']==os.environ['GITHUB_SHA'] and str(a['workflow_run']['id'])==run
 assert a.get('digest','').startswith('sha256:')
 try:
  response=request(base+'/actions/artifacts/'+str(a['id'])+'/zip')
 except urllib.error.HTTPError as e:
  assert e.code in (301,302,303,307,308)
  location=e.headers['Location']; u=urllib.parse.urlsplit(location)
  assert u.scheme=='https' and (u.hostname.endswith('.blob.core.windows.net') or u.hostname.endswith('.actions.githubusercontent.com'))
  response=request(location,False)
 with response: data=response.read(128*1024*1024+1)
 target=out/a['name']; target.mkdir()
 total+=g07_unpack(data,target,a['digest'],512*1024*1024-total)
 receipt.append({'name':a['name'],'id':a['id'],'digest':a['digest'],'job_id':job['id'],'mode':mode,'run_id':run,'run_attempt':attempt,'conclusion':job['conclusion']})
(out/'transport.json').write_text(json.dumps(receipt)+'\n')
`;
  need(env.GITHUB_TOKEN, "missing official transport credential");
  try {
    execFileSync("python3", ["-c", script, dir, JSON.stringify(modes), JSON.stringify(["plan", ...modes].map((m) => [g07Artifact(m, env.GITHUB_RUN_ATTEMPT), m]))],
      { env, timeout: Math.min(budget * 1000, 240000), stdio: ["ignore", "pipe", "pipe"], maxBuffer: 1048576 });
  } catch (error) {
    const forbidden = String(error.stderr ?? "").includes("HTTP Error 403");
    throw new Error(forbidden ? "canonical G07: official transport HTTP 403 (Actions read required)" : "canonical G07: official transport failed/expired or archive/job validation rejected; no fallback");
  }
}
function canonicalMain(cmd, dir, args, env) {
  const context = g07Context(env), anchor = Number(env.LC_G07_ANCHOR), deadline = Number(env.LC_G07_DEADLINE);
  const remaining = g07Budget(anchor, deadline, clock(), cmd === "canonical-verify" ? 0 : 300);
  if (cmd === "canonical-plan") {
    need(!env.LC_FOUNDATION_RUN && !env.LC_FOUNDATION_SKIP && !env.LC_FOUNDATION_PKGS, "plan narrowed");
    mkdirSync(dir, { recursive: true });
    const packages = execFileSync("go", ["list", "-race", "./..."], { cwd: ROOT, env, timeout: Math.min(remaining * 1000, 540000), encoding: "utf8", maxBuffer: 64 * 1024 * 1024 });
    writeFileSync(path.join(dir, "packages.txt"), packages);
    const raw = execFileSync("go", ["test", "-race", "-count=1", "-json", "-p", "1", "-list", ".", "./..."], { cwd: ROOT, env, timeout: Math.min(g07Budget(anchor, deadline, clock(), 300) * 1000, 540000), encoding: "utf8", maxBuffer: 64 * 1024 * 1024 });
    writeFileSync(path.join(dir, "inventory.jsonl"), raw);
    const inventory = g07Inventory(raw, packages), { modes } = g07PlanShape(inventory);
    need(equal(context, g07Context(env)), "plan source changed during inventory");
    save(path.join(dir, "plan.json"), { schema: "g07-shards/v1", context, anchor, deadline, modes, inventory });
    return 0;
  }
  if (cmd === "canonical-start" || cmd === "canonical-end") {
    const mode = env.MODE, shardPlan = json(path.join(ROOT, "scripts/dev/shard-plan.json"));
    need(["shard:unit", "--browser-product-editor", ...shardPlan.groups.map((g) => `shard:${g.id}`)].includes(mode), "unplanned canonical mode");
    need(!env.LC_FOUNDATION_RUN && !env.LC_FOUNDATION_SKIP && !env.LC_FOUNDATION_PKGS, "operator package/selector override");
    mkdirSync(dir, { recursive: true });
    const phase = cmd.slice(10), witness = { phase, context, anchor, deadline, at: clock(), mode };
    if (phase === "end") {
      need(/^[0-9]+$/.test(args[0] ?? ""), "missing true exit"); witness.exit = Number(args[0]);
      if (mode === "--browser-product-editor" && witness.exit === 0) {
        const files = readdirSync("output/playwright", { recursive: true }).filter((p) => p.endsWith("/matrix-owner-proof.json"));
        need(files.length === 1, "missing/duplicate fresh proof");
        const f = path.join("output/playwright", files[0]), proof = json(f);
        need(proof.identity.evidence === path.basename(path.dirname(f)), "native evidence location mismatch");
        witness.evidence = path.basename(path.dirname(f));
        writeFileSync(path.join(dir, "fresh.json"), readFileSync(f));
        writeFileSync(path.join(dir, "ledger.json"), readFileSync(path.join(path.dirname(f), "product-editor-click-ledger.json")));
      }
    }
    need(!existsSync(path.join(dir, `${phase}.json`)), "duplicate witness");
    save(path.join(dir, `${phase}.json`), witness); return 0;
  }
  if (cmd === "canonical-verify") {
    const shardPlan = json(path.join(ROOT, "scripts/dev/shard-plan.json"));
    const modes = expand(["foundation-shards", "--browser-product-editor"], { groupIds: shardPlan.groups.map((g) => g.id) }).matrix.map((m) => m.mode);
    g07Transport(dir, modes, env, remaining);
    const folder = (m) => path.join(dir, g07Artifact(m, env.GITHUB_RUN_ATTEMPT));
    const plan = json(path.join(folder("plan"), "plan.json")), inventory = g07Inventory(readFileSync(path.join(folder("plan"), "inventory.jsonl"), "utf8"), readFileSync(path.join(folder("plan"), "packages.txt"), "utf8"));
    need(equal(readdirSync(folder("plan")).sort(), ["inventory.jsonl", "packages.txt", "plan.json"]), "unexpected plan artifact member");
    g07PlanShape(inventory);
    const shards = modes.map((mode) => {
      const d = folder(mode), read = (f) => readFileSync(path.join(d, f), "utf8");
      const expectedFiles = ["start.json", "end.json", "go.jsonl", ...(mode === "shard:unit" ? ["vet.jsonl", ...G07_NODE.map((n) => `node-${n}.tap`)] : []), ...(mode === "--browser-product-editor" ? ["fresh.json", "ledger.json"] : [])];
      need(equal(readdirSync(d).sort(), expectedFiles.sort()), "unexpected/missing canonical artifact member");
      return { mode, official_pass: true, start: JSON.parse(read("start.json")), end: JSON.parse(read("end.json")), go: read("go.jsonl"),
        ...(mode === "shard:unit" ? { vet: read("vet.jsonl").trimEnd().split("\n").map(JSON.parse), node: Object.fromEntries(G07_NODE.map((n) => [n, read(`node-${n}.tap`)])) } : {}),
        ...(mode === "--browser-product-editor" ? { fresh: JSON.parse(read("fresh.json")) } : {}) };
    });
    const result = verifyG07(plan, shards, { context, inventory, modes, shardPlan, anchor, deadline, now: clock(), binding: g07Binding(folder("--browser-product-editor")) });
    need(equal(context, g07Context(env)), "finalizer source drift");
    g07Budget(anchor, deadline, clock());
    mkdirSync(args[0], { recursive: true });
    writeFileSync(path.join(args[0], "G07.jsonl"), result.events.map((e) => JSON.stringify(e)).join("\n") + "\n");
    writeFileSync(path.join(args[0], "G07.conditional"), result.conditional);
    return 0;
  }
  throw new Error("unknown canonical command");
}

const walk = (dir) => readdirSync(dir, { withFileTypes: true, recursive: true }).filter((d) => d.isFile() && /^receipt-.*\.json$/.test(d.name)).map((d) => path.join(d.parentPath, d.name));

export function main(argv = process.argv.slice(2), env = process.env) {
  const [cmd, dir, ...logs] = argv;
  if (cmd?.startsWith("canonical-") && dir) return canonicalMain(cmd, dir, logs, env);
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
  console.error("usage: matrix-receipts.mjs write <out-dir> <gate-log>... | verify <receipts-dir> | canonical-plan/start <dir> | canonical-end <dir> <exit> | canonical-verify <new-dir> <out>");
  return 2;
}
if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try { process.exit(main()); } catch (e) { console.error(`matrix-receipts: ${e.message}`); process.exit(1); }
}
