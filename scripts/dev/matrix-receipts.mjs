#!/usr/bin/env node
// Purpose: MATRIX-RECEIPT (#128). `write` turns one gates.yml matrix job's own gate log into `receipt-<mode>.json`
//   {mode, run_id, run_attempt, sha, status, test_count}; `verify` proves every mode the plan job emitted has exactly one receipt
//   from THIS run and sha with status PASS and test_count > 0. Missing, duplicate, stale, unplanned, failed or zero-count = red.
//   test_count is read from the runners' own output in the log, never from a static list: top-level `--- PASS:` / `--- FAIL:` lines of
//   `go test -v` (the lines `go tool test2json` turns into pass/fail events; test-focused.sh counts the same), top-level pass/fail events of
//   `go test -json` logs (--browser-webkit / --stripe-browser write one .jsonl per step instead of -v text), plus the node --test summary
//   (`ℹ pass N` / `ℹ fail N` spec reporter, `# pass N` / `# fail N` TAP). Browser specs run inside those go tests.
//   Historical evidence never feeds this check.
// Depends on: scripts/dev/ci-plan.mjs (artifactName); env MODE, GATE_OUTCOME, GITHUB_RUN_ID, GITHUB_RUN_ATTEMPT, GITHUB_SHA (write);
//   env PLANNED (the plan job's `matrix` output), GITHUB_RUN_ID, GITHUB_SHA (verify). No network, no dependencies.
// Used by: .github/workflows/gates.yml (gate job "Run receipt" step, `receipts` job feeding `required`), tests/ci/matrix-receipts.test.mjs.
// Usage: node scripts/dev/matrix-receipts.mjs write <out-dir> <gate-log | go-test-json-log>...   |   node scripts/dev/matrix-receipts.mjs verify <receipts-dir>
import { mkdirSync, readFileSync, readdirSync, writeFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { artifactName } from "./ci-plan.mjs";

export const receiptFile = (mode) => `receipt-${artifactName(mode).replace(/^gate-/, "")}.json`;

/** Tests that ran according to the runners' own output in one log text. */
export function countTests(text) {
  let n = 0;
  for (const line of text.split("\n")) {
    if (/^--- (PASS|FAIL): /.test(line)) n++;
    const m = /^(?:ℹ|#) (?:pass|fail) (\d+)\s*$/.exec(line);
    if (m) n += Number(m[1]);
    if (line.startsWith("{")) {
      try { const e = JSON.parse(line); if ((e.Action === "pass" || e.Action === "fail") && e.Test && !e.Test.includes("/")) n++; } catch { /* not a go test -json event */ }
    }
  }
  return n;
}

export function makeReceipt({ mode, outcome, runId, runAttempt, sha, logs }) {
  return { mode, run_id: String(runId), run_attempt: String(runAttempt), sha, status: outcome === "success" ? "PASS" : "FAIL", test_count: logs.reduce((s, t) => s + countTests(t), 0) };
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
    const r = makeReceipt({ mode: env.MODE, outcome: env.GATE_OUTCOME, runId: env.GITHUB_RUN_ID, runAttempt: env.GITHUB_RUN_ATTEMPT, sha: env.GITHUB_SHA, logs: logs.map((f) => readFileSync(f, "utf8")) });
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
