// Purpose: fail closed unless CI has receipts from required live matrix executions.
// Depends on: matrix-receipt.mjs source inventory and GitHub run/job/shard context.
// Used by: foundation always-verifier and the Gates required aggregate.
import { readFileSync, readdirSync, statSync, appendFileSync } from "node:fs";
import { createHash } from "node:crypto";
import { basename, dirname, join, resolve } from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { browserOutcome, goFamily, goOutcome, nodeOutcome, nodeSuites, sourceHashes } from "./matrix-receipt.mjs";

function fail(message) { console.error(`matrix-receipt-verify: ${message}`); process.exitCode = 1; }
function readJson(path) { try { return JSON.parse(readFileSync(path, "utf8")); } catch { throw new Error(`missing or invalid receipt: ${path}`); } }
function head() {
  const result = spawnSync("git", ["rev-parse", "HEAD"], { encoding: "utf8" });
  if (result.status !== 0) throw new Error("cannot resolve current source commit");
  return result.stdout.trim();
}
export function verifyReceipt(receipt, { receiptPath, kind, mode, runId, job, sourceCommit }) {
  const errors = [];
  if (receipt.schema !== "matrix-execution-receipt/v1") errors.push("schema mismatch");
  if (receipt.kind !== kind) errors.push(`kind ${receipt.kind} != ${kind}`);
  if (receipt.mode !== mode) errors.push(`mode ${receipt.mode} != ${mode}`);
  if (receipt.run_id !== runId) errors.push("run id mismatch");
  if (receipt.job !== job) errors.push(`job ${receipt.job} != ${job}`);
  if (receipt.exit_code !== 0) errors.push(`command exit ${receipt.exit_code}`);
  if (receipt.source_before?.commit !== sourceCommit || receipt.source_after?.commit !== sourceCommit) errors.push("source commit mismatch");
  if (JSON.stringify(receipt.source_before?.hashes) !== JSON.stringify(receipt.source_after?.hashes)) errors.push("source changed during command");
  if (JSON.stringify(receipt.source_after?.hashes) !== JSON.stringify(sourceHashes())) errors.push("receipt source hashes are stale");
  if (receipt.outcome?.ok !== true || receipt.violations?.length) errors.push("actual test events are incomplete or failed");
  try {
    const eventPath = join(dirname(receiptPath), receipt.event_file || "");
    if (!receipt.event_file || basename(receipt.event_file) !== receipt.event_file) throw new Error("unsafe event filename");
    const raw = readFileSync(eventPath, "utf8");
    const digest = createHash("sha256").update(raw).digest("hex");
    if (receipt.event_sha256 !== digest) throw new Error("raw event digest mismatch");
    const events = raw.split(/\r?\n/).filter(Boolean).map((line) => JSON.parse(line));
    let actual;
    if (kind === "node") actual = nodeOutcome(events, nodeSuites);
    else if (kind === "go") {
      const { family, shard } = goFamily();
      if (receipt.shard !== shard) throw new Error("Go shard is not the source-derived catch-all");
      actual = goOutcome(events, family);
    } else {
      actual = browserOutcome(events);
      const command = receipt.command || [];
      for (const token of ["PRODUCT_EDITOR_ACCEPTANCE=1", "LC_BROWSER_CATALOG_CORE_ACCEPTANCE=1", "-tags", "browser", "-run", "^TestBrowserCatalogCore$", "./tests/foundation"])
        if (!command.includes(token)) throw new Error(`browser command missing ${token}`);
    }
    if (JSON.stringify(actual) !== JSON.stringify(receipt.outcome)) throw new Error("receipt outcome differs from independently parsed raw events");
  } catch (error) { errors.push(`raw event verification: ${error.message}`); }
  return errors;
}
/** Validate the Foundation Go receipt before deciding whether its historical skip needs a live browser backstop. */
export function foundationBrowserRequired(directory, goStepOutcome, runId, sourceCommit) {
  if (goStepOutcome !== "success") throw new Error(`Go runner outcome is ${goStepOutcome}`);
  const receiptPath = resolve(directory, "go.json");
  const receipt = readJson(receiptPath);
  const errors = verifyReceipt(receipt, { receiptPath, kind: "go", mode: "foundation", runId,
    job: process.env.GITHUB_JOB || "foundation", sourceCommit });
  if (errors.length) throw new Error(`Go receipt: ${errors.join("; ")}`);
  return receipt.outcome?.historical_skip?.ok === true;
}
function collectJsonFiles(root) {
  const found = [];
  if (!statSync(root).isDirectory()) return found;
  for (const entry of readdirSync(root, { withFileTypes: true })) {
    const path = join(root, entry.name);
    if (entry.isDirectory()) found.push(...collectJsonFiles(path));
    // Gate artifacts also contain native ledgers, manifests and historical
    // probes. Only these named execution receipts belong to this verifier.
    else if (entry.isFile() && ["node.json", "go.json", "browser.json"].includes(entry.name)) found.push(path);
  }
  return found;
}
function catchAllShard() {
  const plan = JSON.parse(readFileSync("scripts/dev/shard-plan.json", "utf8"));
  const groups = plan.groups.filter((group) => group.catch_all === true);
  if (groups.length !== 1) throw new Error("shard plan catch-all is missing or ambiguous");
  return groups[0].id;
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) try {
  const [mode, ...args] = process.argv.slice(2);
  if (mode === "foundation-browser-required") {
    const [directory, goStepOutcome, runId, sha] = args;
    const required = foundationBrowserRequired(directory, goStepOutcome, runId, sha || head());
    console.log(`matrix-receipt-verify: foundation browser required=${required}`);
    if (process.env.GITHUB_OUTPUT) appendFileSync(process.env.GITHUB_OUTPUT, `required=${required}\n`);
  } else if (mode === "foundation") {
    const [directory, nodeStepOutcome, goStepOutcome, browserStepOutcome, goExpected, runId, sha] = args;
    const expectedSource = sha || head();
    if (nodeStepOutcome !== "success") throw new Error(`Node runner outcome is ${nodeStepOutcome}`);
    const node = readJson(resolve(directory, "node.json"));
    const errors = verifyReceipt(node, { receiptPath: resolve(directory, "node.json"), kind: "node", mode: `${process.env.LC_MATRIX_NODE_MODE || "foundation"}`, runId,
      job: process.env.GITHUB_JOB || "foundation", sourceCommit: expectedSource });
    if (errors.length) throw new Error(`Node receipt: ${errors.join("; ")}`);
    if (goExpected === "true") {
      if (goStepOutcome !== "success") throw new Error(`Go runner outcome is ${goStepOutcome}`);
      const go = readJson(resolve(directory, "go.json"));
      const goErrors = verifyReceipt(go, { receiptPath: resolve(directory, "go.json"), kind: "go", mode: "foundation", runId, job: process.env.GITHUB_JOB || "foundation", sourceCommit: expectedSource });
      if (goErrors.length) throw new Error(`Go receipt: ${goErrors.join("; ")}`);
      const historicalSkip = go.outcome?.historical_skip?.ok === true;
      const hasBrowserReceipt = historicalSkip || browserStepOutcome === "success";
      if (historicalSkip && browserStepOutcome !== "success")
        throw new Error(`historical canonical skip requires a same-run browser receipt and successful browser step; browser outcome is ${browserStepOutcome}`);
      if (hasBrowserReceipt) {
        const browserPath = resolve(directory, "browser.json");
        const browser = readJson(browserPath);
        const browserErrors = verifyReceipt(browser, { receiptPath: browserPath, kind: "browser", mode: "--browser-product-editor",
          runId, job: process.env.GITHUB_JOB || "foundation", sourceCommit: expectedSource });
        if (browserErrors.length) throw new Error(`same-run browser receipt: ${browserErrors.join("; ")}`);
      } else if (browserStepOutcome !== "skipped") {
        throw new Error(`unexpected Foundation browser runner outcome ${browserStepOutcome}`);
      }
    } else {
      if (goStepOutcome !== "skipped") throw new Error(`Go not expected but step outcome is ${goStepOutcome}`);
      if (browserStepOutcome !== "skipped") throw new Error(`browser backstop must be skipped when Go is not expected; outcome is ${browserStepOutcome}`);
    }
  } else if (mode === "required") {
    const [directory, runId, sha, matrixText, eventName] = args;
    const files = collectJsonFiles(resolve(directory));
    const receipts = files.map((path) => ({ path, receipt: readJson(path) }));
    const shard = catchAllShard();
    let plannedModes;
    try { plannedModes = JSON.parse(matrixText).map((entry) => entry.mode); }
    catch { throw new Error("plan matrix is missing or invalid"); }
    const requiredModes = ["shard:unit", `shard:${shard}`];
    const planned = requiredModes.filter((mode) => plannedModes.includes(mode));
    if (planned.length !== requiredModes.length) {
      if (["pull_request", "push", "schedule"].includes(eventName) || planned.length !== 0)
        throw new Error(`automatic required plan must include ${requiredModes.join(" and ")}`);
      console.log("matrix-receipt-verify: custom manual run has no matrix receipt modes");
      process.exit(0);
    }
    let matrixReceipt;
    for (const [kind, expectedMode] of [["node", "shard:unit"], ["go", `shard:${shard}`]]) {
      const matches = receipts.filter(({ receipt }) => receipt.kind === kind && receipt.mode === expectedMode && receipt.job === "gate" && receipt.run_id === runId);
      if (matches.length !== 1) throw new Error(`expected exactly one ${kind} receipt for ${expectedMode}; found ${matches.length}`);
      const errors = verifyReceipt(matches[0].receipt, { receiptPath: matches[0].path, kind, mode: expectedMode, runId, job: "gate", sourceCommit: sha });
      if (errors.length) throw new Error(`${kind} receipt ${matches[0].path}: ${errors.join("; ")}`);
      if (kind === "go") matrixReceipt = matches[0].receipt;
    }
    if (matrixReceipt?.outcome?.historical_skip) {
      if (!plannedModes.includes("--browser-product-editor")) throw new Error("historical canonical skip requires a planned --browser-product-editor job");
      const matches = receipts.filter(({ receipt }) => receipt.kind === "browser" && receipt.mode === "--browser-product-editor" && receipt.job === "gate" && receipt.run_id === runId);
      if (matches.length !== 1) throw new Error(`historical canonical skip requires one same-run browser receipt; found ${matches.length}`);
      const errors = verifyReceipt(matches[0].receipt, { receiptPath: matches[0].path, kind: "browser", mode: "--browser-product-editor", runId, job: "gate", sourceCommit: sha });
      if (errors.length) throw new Error(`fresh browser receipt ${matches[0].path}: ${errors.join("; ")}`);
    }
  } else throw new Error("usage: matrix-receipt-verify.mjs foundation-browser-required|foundation|required ...");
  console.log(`matrix-receipt-verify: ${mode} positive execution receipts verified`);
} catch (error) { fail(error.message); }
