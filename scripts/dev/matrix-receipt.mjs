// Purpose: record positive outcomes from the actual Node and Go test commands.
// Depends on: Node 24, Go test JSON events, and the current shard-plan.json.
// Used by: test-node.sh, test-local.sh and CI receipt validators.
import { spawnSync } from "node:child_process";
import { readFileSync, readdirSync, mkdirSync, writeFileSync, lstatSync } from "node:fs";
import { createHash } from "node:crypto";
import { basename, dirname, resolve, relative, isAbsolute, posix } from "node:path";
import { fileURLToPath } from "node:url";

export const nodeSuites = [
  "tests/ci/product-editor-matrix-ledger.test.mjs",
  "tests/ci/product-editor-matrix-execution.test.mjs",
  "tests/ci/product-editor-matrix-controls.test.mjs",
];
const nodeInventoryPath = "scripts/dev/matrix-test-inventory.json";
export const nodeCaseInventory = JSON.parse(readFileSync(nodeInventoryPath, "utf8"));
const nodeInventory = nodeCaseInventory;
const receiptSourceFiles = ["scripts/dev/matrix-receipt.mjs", "scripts/dev/matrix-test-reporter.mjs", "scripts/dev/matrix-receipt-verify.mjs",
  nodeInventoryPath, "scripts/dev/test-node.sh", "scripts/dev/test-local.sh", "scripts/dev/shard-plan.json",
  "scripts/dev/ci-plan.mjs", "scripts/dev/pr-modes.mjs", "tests/ci/ci-plan.test.mjs", "tests/ci/pr-modes.test.mjs",
  "tests/ci/matrix-receipt.test.mjs", "tests/foundation/browser_catalog_core_test.go", ...nodeSuites];

function sha(bytes) { return createHash("sha256").update(bytes).digest("hex"); }
export function canonicalEvidenceDrift(sourcePathOverride = null, checkout = process.cwd()) {
  const delivery = readFileSync(resolve(checkout, "output/matrix-ledger-enforce/DELIVERY.md"), "utf8");
  const directory = /\[SOURCE\.json\]\(([A-Za-z0-9._-]+)\/SOURCE\.json\)/.exec(delivery)?.[1];
  if (!directory) throw new Error("canonical matrix delivery does not link SOURCE.json");
  const source = JSON.parse(readFileSync(sourcePathOverride || resolve(checkout, `output/matrix-ledger-enforce/${directory}/SOURCE.json`), "utf8"));
  const gatePath = "tests/foundation/catalog_matrix_ledger_test.go";
  const gateBytes = readFileSync(resolve(checkout, gatePath));
  const goSource = gateBytes.toString("utf8");
  const body = /var productEditorMatrixEvidenceSources = \[\]string\{([\s\S]*?)\n\}/.exec(goSource)?.[1];
  const bound = [...(body || "").matchAll(/"([^"\n]+)"/g)].map((match) => match[1]);
  if (!bound.length) throw new Error("canonical Go evidence source inventory unavailable");
  const drift = [];
  let gateChanged = false;
  for (const [path, expected] of Object.entries(source.source_files_sha256 || {}).sort(([a], [b]) => a.localeCompare(b))) {
    if (!/^[0-9a-f]{64}$/.test(expected || "")) throw new Error(`SOURCE.json has an invalid bound hash for ${path}`);
    // Match the Go evidence policy: only a genuinely removed producer is
    // historical drift; unsafe paths, links and all other I/O errors fail.
    if (typeof path !== "string" || isAbsolute(path) || posix.normalize(path) !== path ||
        path === "." || path === ".." || path.startsWith("../"))
      throw new Error(`canonical source path must stay repository-relative: ${path}`);
    let actual = null;
    try {
      let full = checkout;
      for (const part of path.split("/")) {
        full = resolve(full, part);
        if (lstatSync(full).isSymbolicLink()) throw new Error(`canonical source symlink: ${path}`);
      }
      actual = sha(readFileSync(full));
    } catch (error) {
      if (error.code !== "ENOENT") throw error;
    }
    if (actual !== expected) {
      drift.push(path);
      if (path === gatePath) gateChanged = true;
    }
  }
  for (const path of bound) {
    if (source.source_files_sha256?.[path]) continue;
    if (!gateChanged) throw new Error(`canonical source receipt missing tested producer: ${path}`);
    drift.push(path);
  }
  return drift.sort();
}

export function sourceHashes() {
  const goFiles = readdirSync("tests/foundation").filter((name) => name.endsWith("_test.go") &&
    /^func TestProductEditorMatrix[A-Za-z0-9_]*/m.test(readFileSync(resolve("tests/foundation", name), "utf8")))
    .map((name) => `tests/foundation/${name}`);
  const delivery = readFileSync("output/matrix-ledger-enforce/DELIVERY.md", "utf8");
  const canonical = /\[SOURCE\.json\]\(([A-Za-z0-9._-]+)\/SOURCE\.json\)/.exec(delivery)?.[1];
  if (!canonical) throw new Error("canonical matrix delivery does not link SOURCE.json");
  const evidenceFiles = ["output/matrix-ledger-enforce/DELIVERY.md", "output/matrix-ledger-enforce/" + canonical + "/SOURCE.json"];
  return Object.fromEntries([...new Set([...receiptSourceFiles, ...goFiles, ...evidenceFiles])].sort().map((path) => [path, sha(readFileSync(path))]));
}
function sourceIdentity() { return { commit: execGitHead(), hashes: sourceHashes() }; }
function execGitHead() {
  const result = spawnSync("git", ["rev-parse", "HEAD"], { encoding: "utf8" });
  if (result.status !== 0) throw new Error(`git rev-parse HEAD failed: ${result.stderr}`);
  return result.stdout.trim();
}
function outputPath(kind) {
  return process.env.LC_MATRIX_RECEIPT_PATH || resolve("output/matrix-coverage-v2-execution",
    process.env.GITHUB_RUN_ID || "local", process.env.GITHUB_JOB || "local", `${kind}.json`);
}
function baseReceipt(kind, command) {
  return { schema: "matrix-execution-receipt/v1", kind, command, run_id: process.env.GITHUB_RUN_ID || "local",
    job: process.env.GITHUB_JOB || "local", mode: process.env.LC_MATRIX_RECEIPT_MODE || "local",
    source_before: sourceIdentity(), source_after: null, started_at: new Date().toISOString(), exit_code: null };
}
function finish(receipt, kind, status, events, raw) {
  receipt.source_after = sourceIdentity();
  receipt.ended_at = new Date().toISOString();
  receipt.exit_code = status;
  receipt.events = events;
  const path = outputPath(kind);
  if (raw !== undefined) {
    receipt.event_file = `${basename(path)}.events`;
    receipt.event_sha256 = sha(raw);
  }
  mkdirSync(dirname(path), { recursive: true });
  writeFileSync(path, `${JSON.stringify(receipt, null, 2)}\n`);
  if (raw !== undefined) writeFileSync(`${path}.events`, raw);
  console.error(`matrix-receipt: ${kind} wrote ${relative(process.cwd(), path)}`);
}
function child(command, args, env = process.env) {
  const result = spawnSync(command, args, { encoding: "utf8", env, maxBuffer: 64 * 1024 * 1024 });
  if (result.error) throw result.error;
  return result;
}
function parseNodeEvents(raw) {
  return raw.split(/\r?\n/).filter(Boolean).map((line, i) => {
    try { return JSON.parse(line); } catch { throw new Error(`invalid Node reporter event at line ${i + 1}`); }
  });
}
export function nodeOutcome(events, actualFiles) {
  const expected = new Set(nodeSuites.map((path) => resolve(path)));
  const fileEvents = events.filter((event) => event.type === "test:summary" && event.data?.file && expected.has(resolve(event.data.file)));
  const actual = new Set(fileEvents.map((event) => resolve(event.data?.file || "")));
  const results = events.filter((event) => event.type === "test:complete" && event.data?.nesting === 0 && event.data?.details?.type === "test" &&
    !(event.data.name === event.data.file || (event.data.file && resolve(event.data.name) === resolve(event.data.file))));
  const cases = results.map(({ data }) => ({ file: relative(process.cwd(), resolve(data.file || "")), name: data.name,
    passed: data.details?.passed === true, skipped: Boolean(data.skip), todo: Boolean(data.todo), only: Boolean(data.only) }));
  const violations = [];
  const identities = new Set();
  for (const item of cases) {
    const key = `${item.file}\0${item.name}`;
    if (identities.has(key)) violations.push(`duplicate Node case identity: ${item.file} / ${item.name}`);
    identities.add(key);
  }
  if (expected.size !== actual.size || [...expected].some((path) => !actual.has(path))) violations.push("required Node suite file set differs");
  for (const path of expected) {
    const name = relative(process.cwd(), path);
    const expectedNames = nodeInventory.suites?.[name]?.tests;
    const sourceHash = nodeInventory.suites?.[name]?.source_sha256;
    if (!Array.isArray(expectedNames) || !expectedNames.length || sha(readFileSync(path)) !== sourceHash)
      violations.push(`Node case inventory is missing or stale for ${name}`);
    const actualNames = cases.filter((item) => resolve(item.file) === path).map((item) => item.name).sort();
    if (JSON.stringify(actualNames) !== JSON.stringify([...(expectedNames || [])].sort()))
      violations.push(`Node case identities differ from source-bound inventory for ${name}`);
  }
  if (cases.some((item) => !item.passed || item.skipped || item.todo || item.only)) violations.push("Node event contains failed, skipped, todo, or only case");
  const summaries = fileEvents.map((event) => ({ file: resolve(event.data.file), counts: event.data?.counts || {} }));
  for (const { file, counts } of summaries) {
    const casesInFile = cases.filter((item) => resolve(item.file) === file);
    if (counts.tests !== casesInFile.length || counts.passed !== casesInFile.filter((item) => item.passed).length ||
        counts.failed || counts.cancelled || counts.skipped || counts.todo)
      violations.push(`Node per-file summary disagrees with actual cases for ${relative(process.cwd(), file)}`);
  }
  const aggregate = events.filter((event) => event.type === "test:plan" && event.data?.nesting === 0);
  const expectedCount = Object.values(nodeInventory.suites).reduce((sum, suite) => sum + suite.tests.length, 0);
  if (summaries.length !== nodeSuites.length || aggregate.length !== 1 || aggregate[0].data.count !== expectedCount)
    violations.push("Node per-file summary contains failed, cancelled, skipped, todo, or missing counts");
  if (actualFiles && actualFiles.some((path) => !expected.has(resolve(path)))) violations.push("unexpected Node suite argument");
  return { ok: violations.length === 0, violations, files: [...actual].map((path) => relative(process.cwd(), path)).sort(), cases };
}
export function goFamily(foundationDirectory = "tests/foundation") {
  const files = readdirSync(foundationDirectory).filter((name) => name.endsWith("_test.go"));
  const names = new Set();
  for (const name of files) {
    const text = readFileSync(resolve(foundationDirectory, name), "utf8");
    for (const match of text.matchAll(/^func\s+(TestProductEditorMatrix[A-Za-z0-9_]*)\s*\(\s*(?:[A-Za-z_][A-Za-z0-9_]*\s+)?\*testing\.T\s*,?\s*\)/gm)) names.add(match[1]);
  }
  const plan = JSON.parse(readFileSync("scripts/dev/shard-plan.json", "utf8"));
  const catchAll = plan.groups.filter((group) => group.catch_all === true);
  if (catchAll.length !== 1) throw new Error("shard plan must have exactly one catch-all group");
  const otherNames = new Set(plan.groups.filter((group) => !group.catch_all).flatMap((group) => group.tests));
  const family = [...names].sort();
  if (!names.has("TestProductEditorMatrixLedgerEvidence"))
    throw new Error("Go matrix family is missing the required canonical evidence test");
  if (!family.length || family.some((name) => otherNames.has(name)))
    throw new Error("Go matrix family must not be assigned outside its catch-all group");
  return { family, shard: catchAll[0].id };
}
export function goOutcome(events, family) {
  const outcomes = new Map();
  const eventCounts = new Map();
  const testOutput = new Map();
  const packages = new Set();
  const malformed = [];
  for (const event of events) {
    if (event.Package) packages.add(event.Package);
    if (event.Test && family.includes(event.Test) && event.Action === "output")
      testOutput.set(event.Test, `${testOutput.get(event.Test) || ""}${event.Output || ""}`);
    if (event.Test && family.includes(event.Test) && ["pass", "skip", "fail"].includes(event.Action)) {
      const key = `${event.Package}:${event.Test}`;
      eventCounts.set(key, (eventCounts.get(key) || 0) + 1);
      if (outcomes.has(key) && outcomes.get(key) !== event.Action) malformed.push(`conflicting Go outcomes for ${key}`);
      outcomes.set(key, event.Action);
    }
  }
  const expectedPackage = "livecommerce/tests/foundation";
  const violations = [...malformed];
  if (!family.includes("TestProductEditorMatrixLedgerEvidence"))
    violations.push("Go matrix family is missing the required canonical evidence test");
  if (!packages.has(expectedPackage)) violations.push("Go test JSON did not execute tests/foundation");
  for (const event of events) if (event.Test?.startsWith("TestProductEditorMatrix") && ["skip", "fail"].includes(event.Action) &&
      !(event.Test === "TestProductEditorMatrixLedgerEvidence" && event.Action === "skip"))
    violations.push(`Go matrix event is ${event.Action}: ${event.Test}`);
  const historical = family.includes("TestProductEditorMatrixLedgerEvidence")
    ? historicalSkip(events, outcomes.get(`${expectedPackage}:TestProductEditorMatrixLedgerEvidence`), testOutput.get("TestProductEditorMatrixLedgerEvidence")) : null;
  for (const name of family) {
    const key = `${expectedPackage}:${name}`;
    const action = outcomes.get(key);
    if (eventCounts.get(key) !== 1) violations.push(`Go matrix test requires one actual outcome event: ${name}`);
    if (action !== "pass" && !(name === "TestProductEditorMatrixLedgerEvidence" && action === "skip" && historical?.ok))
      violations.push(`Go matrix test missing pass outcome: ${name}`);
  }
  if (historical && !historical.ok) violations.push(...historical.violations);
  const actual = [...outcomes].map(([key, action]) => ({ package: key.slice(0, key.lastIndexOf(":")), name: key.slice(key.lastIndexOf(":") + 1), action }));
  return { ok: violations.length === 0, violations, packages: [...packages].sort(), family, outcomes: actual,
    historical_skip: historical?.ok ? historical : null };
}

export function historicalSkip(events, action, output) {
  if (action !== "skip") return null;
  const violations = [];
  const reasonMatch = (output || "").split(/\r?\n/).map((line) =>
    /^\s*(?:[A-Za-z0-9_./-]+\.go:\d+: )?(canonical evidence is historical \(source drift: [^)]+\))\s*$/.exec(line)?.[1]).find(Boolean);
  const drift = canonicalEvidenceDrift();
  const checked = validateHistoricalSkip(events, reasonMatch || output, drift);
  violations.push(...checked.violations);
  if (!reasonMatch) violations.push("canonical evidence skip reason must begin with the fixed source-drift prefix");
  return { ok: violations.length === 0, violations, name: "TestProductEditorMatrixLedgerEvidence",
    reason: reasonMatch || "", drift: [...drift].sort() };
}

export function validateHistoricalSkip(events, output, drift) {
  const violations = [];
  const reasonMatch = /^canonical evidence is historical \(source drift: ([^)]+)\)$/.exec((output || "").trim());
  if (!reasonMatch) violations.push("canonical evidence skip reason is malformed");
  const named = reasonMatch ? reasonMatch[1].split(", ").sort() : [];
  if (!drift.length) violations.push("canonical evidence SKIP has no real bound-source drift");
  if (JSON.stringify(named) !== JSON.stringify([...drift].sort())) violations.push("canonical evidence SKIP does not list exact drifted bound files");
  if (events.filter((event) => event.Test && event.Action === "skip" && event.Test.startsWith("TestProductEditorMatrix")).length !== 1)
    violations.push("canonical evidence SKIP must be the only skipped matrix family test");
  return { ok: violations.length === 0, violations, drift: [...drift].sort() };
}

export function browserOutcome(events) {
  const expected = "TestBrowserCatalogCore";
  const required = [expected, `${expected}/PE12-17-TWD`, `${expected}/CC12-frozen`];
  const outcomes = events.filter((event) => event.Package === "livecommerce/tests/foundation" && event.Test && required.includes(event.Test) && ["pass", "skip", "fail"].includes(event.Action));
  const violations = [];
  for (const name of required) {
    const matches = outcomes.filter((event) => event.Test === name);
    if (matches.length !== 1 || matches[0]?.Action !== "pass") violations.push(`fresh browser receipt requires one passed ${name}`);
  }
  for (const event of events) if (event.Package === "livecommerce/tests/foundation" && event.Test?.startsWith(expected) &&
      ["skip", "fail"].includes(event.Action)) violations.push(`fresh browser family event is ${event.Action}: ${event.Test}`);
  const ok = violations.length === 0;
  return { ok, violations,
    tests: outcomes.map(({ Test, Action }) => ({ name: Test, action: Action })) };
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
const [kind, ...rawArgs] = process.argv.slice(2);
if (kind === "gate-step") {
  const { LC_MATRIX_GATE_MODE: mode, LC_MATRIX_GATE_OUTCOME: outcome, GITHUB_RUN_ID: runId,
    GITHUB_JOB: job, GITHUB_SHA: sourceCommit } = process.env;
  if (!mode || !/^[\x20-\x7e]+$/.test(mode)) throw new Error("Gate step receipt requires a printable matrix mode");
  if (!new Set(["success", "failure", "cancelled", "skipped"]).has(outcome)) throw new Error("Gate step receipt requires the actual GitHub step outcome");
  if (!runId || !job || !/^[0-9a-f]{40,64}$/i.test(sourceCommit || "")) throw new Error("Gate step receipt requires GitHub run, job and source identities");
  const safeMode = mode.replace(/[^A-Za-z0-9_.-]/g, "_");
  const path = resolve("output/matrix-coverage-v2-execution", runId, job, "gate-results", `${safeMode}.gate.json`);
  const receipt = { schema: "matrix-gate-step-receipt/v1", mode, run_id: runId, job, step_name: "Gate", outcome, source_commit: sourceCommit };
  mkdirSync(dirname(path), { recursive: true });
  writeFileSync(path, `${JSON.stringify(receipt, null, 2)}\n`);
  console.error(`matrix-receipt: Gate step wrote ${relative(process.cwd(), path)}`);
} else {
if (!["node", "go", "browser"].includes(kind)) throw new Error("usage: matrix-receipt.mjs node|go|browser <command and args>");
const receipt = baseReceipt(kind, rawArgs);
let result;
let outcome;
let raw;
if (kind === "node") {
  const suites = rawArgs.length ? rawArgs : nodeSuites;
  const args = ["--test", `--test-reporter=${resolve("scripts/dev/matrix-test-reporter.mjs")}`, ...suites];
  result = child(process.execPath, args, { ...process.env, NODE_OPTIONS: "" });
  raw = result.stdout;
  const events = parseNodeEvents(raw);
  outcome = nodeOutcome(events, suites.filter((arg) => arg.startsWith("tests/")).map((arg) => arg.split("=")[0]));
  for (const event of events.filter((event) => event.type === "test:complete" && event.data?.details?.passed === false)) {
    console.error(`matrix-receipt: failed ${event.data.file} / ${event.data.name}: ${event.data.details.error?.stack || event.data.details.error?.message || "see raw events"}`);
  }
  console.error(`matrix-receipt: Node actual ${outcome.cases.filter((item) => item.passed).length} passed; ${outcome.files.length} suite files`);
} else {
  let command = rawArgs;
  const env = { ...process.env };
  while (command.length && /^[A-Za-z_][A-Za-z0-9_]*=/.test(command[0])) {
    const i = command[0].indexOf("="); env[command[0].slice(0, i)] = command[0].slice(i + 1); command = command.slice(1);
  }
  if (command[0] !== "go" || command[1] !== "test") throw new Error("Go receipt must wrap the actual go test command");
  const args = ["test", "-json", ...command.slice(2)];
  result = child(command[0], args, env);
  raw = result.stdout;
  const events = raw.split(/\r?\n/).filter(Boolean).map((line, i) => {
    try { return JSON.parse(line); } catch { throw new Error(`invalid Go test JSON event at line ${i + 1}`); }
  });
  if (kind === "go") {
    const { family, shard } = goFamily();
    receipt.shard = shard;
    outcome = goOutcome(events, family);
  } else outcome = browserOutcome(events);
}
receipt.outcome = outcome;
receipt.violations = outcome.violations;
finish(receipt, kind, result.status ?? 1, outcome, raw);
for (const violation of outcome.violations) console.error(`matrix-receipt: ${violation}`);
if (result.stderr) process.stderr.write(result.stderr);
if (result.stdout && kind === "go") {
  for (const line of result.stdout.split(/\r?\n/)) {
    try { const event = JSON.parse(line); if (event.Action === "output" && event.Output) process.stdout.write(event.Output); }
    catch { /* parsing above has already failed closed */ }
  }
}
if (result.error) throw result.error;
if (result.status !== 0 || !outcome.ok) process.exit(result.status || 1);
}
}
