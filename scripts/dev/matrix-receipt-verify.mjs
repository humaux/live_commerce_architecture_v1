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
function verifyGoCommand(command) {
  const errors = [];
  if (!Array.isArray(command) || command.some((part) => typeof part !== "string")) return ["actual Go command argv is missing"];
  const argv = [...command];
  while (argv.length && /^[A-Za-z_][A-Za-z0-9_]*=/.test(argv[0])) argv.shift();
  if (argv[0] !== "go" || argv[1] !== "test") errors.push("actual command argv is not go test");
  if (argv.slice(2).includes("-race=false")) errors.push("actual go test command disables -race");
  if (!argv.slice(2).includes("-race")) errors.push("actual go test command is missing -race");
  return errors;
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
      const commandErrors = verifyGoCommand(receipt.command);
      if (commandErrors.length) throw new Error(commandErrors.join("; "));
      actual = goOutcome(events, family);
    } else {
      actual = browserOutcome(events);
      const commandErrors = verifyGoCommand(receipt.command);
      if (commandErrors.length) throw new Error(commandErrors.join("; "));
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
    else if (entry.isFile() && (["node.json", "go.json", "browser.json"].includes(entry.name) || entry.name.endsWith(".gate.json"))) found.push(path);
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
    try {
      const entries = JSON.parse(matrixText);
      if (!Array.isArray(entries) || entries.some((entry) => !entry || typeof entry.mode !== "string")) throw new Error("bad mode entries");
      plannedModes = entries.map((entry) => entry.mode);
      if (new Set(plannedModes).size !== plannedModes.length) throw new Error("duplicate planned modes");
    }
    catch { throw new Error("plan matrix is missing or invalid"); }
    const gateReceipts = receipts.filter(({ receipt }) => receipt.schema === "matrix-gate-step-receipt/v1");
    if (gateReceipts.length !== plannedModes.length)
      throw new Error(`expected exactly ${plannedModes.length} Gate step receipts for the planned modes; found ${gateReceipts.length}`);
    for (const plannedMode of plannedModes) {
      const matches = gateReceipts.filter(({ receipt }) => receipt.mode === plannedMode);
      if (matches.length !== 1) throw new Error(`expected exactly one Gate step receipt for ${plannedMode}; found ${matches.length}`);
      const { receipt } = matches[0];
      if (receipt.run_id !== runId || receipt.job !== "gate" || receipt.step_name !== "Gate" ||
          receipt.outcome !== "success" || receipt.source_commit !== sha)
        throw new Error(`Gate step receipt for ${plannedMode} must prove same-run successful Gate execution on the planned source`);
    }
    const requiredModes = ["shard:unit", `shard:${shard}`];
    const planned = requiredModes.filter((mode) => plannedModes.includes(mode));
    const automatic = ["pull_request", "push", "schedule"].includes(eventName);
    const foundationPlanned = plannedModes.includes("foundation");
    if (planned.length > 0 && planned.length !== requiredModes.length && !foundationPlanned)
      throw new Error(`matrix plan must include both ${requiredModes.join(" and ")}`);
    if (foundationPlanned && planned.length > 0 && planned.length !== requiredModes.length)
      throw new Error(`foundation cannot be combined with a partial matrix plan; include ${requiredModes.join(" and ")}`);
    const verifyOne = (kind, expectedMode) => {
      const matches = receipts.filter(({ receipt }) => receipt.kind === kind && receipt.mode === expectedMode && receipt.job === "gate" && receipt.run_id === runId);
      if (matches.length !== 1) throw new Error(`expected exactly one ${kind} receipt for ${expectedMode}; found ${matches.length}`);
      const errors = verifyReceipt(matches[0].receipt, { receiptPath: matches[0].path, kind, mode: expectedMode, runId, job: "gate", sourceCommit: sha });
      if (errors.length) throw new Error(`${kind} receipt ${matches[0].path}: ${errors.join("; ")}`);
      return matches[0].receipt;
    };
    const verifyBrowser = () => {
      try { return verifyOne("browser", "--browser-product-editor"); }
      catch (error) { throw new Error(`same-run browser receipt: ${error.message}`); }
    };
    if (planned.length === requiredModes.length) {
      verifyOne("node", "shard:unit");
      const matrixReceipt = verifyOne("go", `shard:${shard}`);
      if (matrixReceipt.outcome?.historical_skip) {
        if (!plannedModes.includes("--browser-product-editor")) throw new Error("historical canonical skip requires a planned --browser-product-editor job");
        verifyBrowser();
      }
    } else if (automatic) {
      throw new Error(`automatic required plan must include ${requiredModes.join(" and ")}`);
    }
    if (foundationPlanned) {
      verifyOne("node", "foundation");
      const goReceipt = verifyOne("go", "foundation");
      if (goReceipt.outcome?.historical_skip) {
        if (!plannedModes.includes("--browser-product-editor")) throw new Error("manual foundation historical skip requires a planned --browser-product-editor job");
        verifyBrowser();
      } else if (plannedModes.includes("--browser-product-editor")) {
        verifyBrowser();
      }
    } else if (planned.length === 0) {
      console.log("matrix-receipt-verify: custom manual run has no matrix receipt modes");
    }
    if (plannedModes.includes("--browser-product-editor")) verifyBrowser();
  } else throw new Error("usage: matrix-receipt-verify.mjs foundation-browser-required|foundation|required ...");
  console.log(`matrix-receipt-verify: ${mode} positive execution receipts verified`);
} catch (error) { fail(error.message); }
