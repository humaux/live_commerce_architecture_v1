// Purpose: prove actual matrix receipts require per-file Node passes, every Go family pass and a same-run browser backstop for a historical skip.
// Depends on: the receipt parser, current shard plan, and isolated output-directory child probes.
// Used by: test-node.sh and product-editor matrix gate maintenance.
import test from "node:test";
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { readFileSync, mkdirSync, writeFileSync, existsSync, rmSync, symlinkSync, mkdtempSync, readdirSync, copyFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { tmpdir } from "node:os";
import { createHash } from "node:crypto";
import { runInNewContext } from "node:vm";
import { nodeSuites, nodeCaseInventory, nodeOutcome, goOutcome, goFamily, validateHistoricalSkip, browserOutcome, sourceHashes, canonicalEvidenceDrift } from "../../scripts/dev/matrix-receipt.mjs";
import { foundationBrowserRequired, verifyReceipt } from "../../scripts/dev/matrix-receipt-verify.mjs";

const probeRoot = mkdtempSync(join(tmpdir(), "matrix-receipt-probes-"));
const probePath = (...parts) => join(probeRoot, ...parts);
test.after(() => rmSync(probeRoot, { recursive: true, force: true }));

test("the independent receipt action refuses failed job outcomes even with valid receipts and a commented shell check", () => {
  const workflow = readFileSync(".github/workflows/gates.yml", "utf8")
    .replace('[[ "$GATE" == success ]] || rc=1', '# [[ "$GATE" == success ]] || rc=1');
  const required = workflow.slice(workflow.indexOf("\n  required:"));
  const body = required.slice(required.indexOf("          script: |\n") + "          script: |\n".length)
    .split("\n").filter((line) => line.startsWith("            ")).map((line) => line.slice(12)).join("\n");
  const execute = (changes) => {
    const failures = [];
    runInNewContext(body, { require: () => ({ spawnSync: () => ({ status: 0 }) }),
      core: { setFailed: (message) => failures.push(message) }, process: { execPath: process.execPath,
        env: { PLAN: "success", GATE: "success", AGG: "skipped", DEPLOY: "skipped", AGG_EXPECTED: "false", DEPLOY_EXPECTED: "false", ...changes } } });
    return failures;
  };
  assert.deepEqual(execute({}), []);
  for (const job of ["PLAN", "GATE", "AGG", "DEPLOY"]) {
    assert.ok(execute({ [job]: "failure" }).some((message) => message.startsWith(job)), `${job} cannot be donated by passing receipts`);
  }
  assert.ok(execute({ AGG_EXPECTED: "true" }).some((message) => message.startsWith("AGG")));
});

function nodeEvents({ skipped = false, only = false, omit = null, removeCase = null } = {}) {
  const events = [];
  let total = 0;
  for (const file of nodeSuites.filter((path) => path !== omit)) {
    const names = nodeCaseInventory.suites[file].tests.filter((name) => `${file}\0${name}` !== removeCase);
    total += names.length;
    events.push(...names.map((name) => ({ type: "test:complete", data: { file, name, nesting: 0,
      details: { type: "test", passed: !skipped }, ...(skipped ? { skip: "filtered" } : {}), ...(only ? { only: true } : {}) } })));
    events.push({ type: "test:summary", data: { file, counts: { tests: names.length, passed: skipped ? 0 : names.length,
      failed: 0, cancelled: 0, skipped: skipped ? names.length : 0, todo: 0 } } });
  }
  events.push({ type: "test:plan", data: { nesting: 0, count: total } });
  return events;
}
function persistReceipt(directory, kind, mode, runId, events, command = [], job = "gate") {
  const receiptPath = resolve(directory, `${kind}.json`);
  mkdirSync(dirname(receiptPath), { recursive: true });
  const raw = events.map((event) => JSON.stringify(event)).join("\n") + "\n";
  writeFileSync(`${receiptPath}.events`, raw);
  const identity = spawnSync("git", ["rev-parse", "HEAD"], { encoding: "utf8" }).stdout.trim();
  const hashes = sourceHashes();
  let outcome;
  if (kind === "node") outcome = nodeOutcome(events, nodeSuites);
  else if (kind === "go") outcome = goOutcome(events, goFamily().family);
  else outcome = browserOutcome(events);
  const actualCommand = command.length ? command : kind === "go"
    ? ["GOTOOLCHAIN=go1.27.2", "go", "test", "-race", "-count=1", "./tests/foundation"]
    : kind === "browser"
      ? ["PRODUCT_EDITOR_ACCEPTANCE=1", "LC_BROWSER_CATALOG_CORE_ACCEPTANCE=1", "GOTOOLCHAIN=go1.27.2", "go", "test", "-race", "-tags", "browser", "-count=1", "-run", "^TestBrowserCatalogCore$", "./tests/foundation"]
      : command;
  const receipt = { schema: "matrix-execution-receipt/v1", kind, mode, run_id: runId, job, command: actualCommand,
    source_before: { commit: identity, hashes }, source_after: { commit: identity, hashes }, exit_code: 0,
    event_file: `${kind}.json.events`, event_sha256: createHash("sha256").update(raw).digest("hex"), outcome, violations: [] };
  if (kind === "go") receipt.shard = goFamily().shard;
  writeFileSync(receiptPath, `${JSON.stringify(receipt, null, 2)}\n`);
  return receiptPath;
}
function persistGateStepResult(directory, mode, runId, outcome = "success", job = "gate") {
  const receiptPath = resolve(directory, `${mode.replace(/[^A-Za-z0-9_.-]/g, "_")}.gate.json`);
  mkdirSync(dirname(receiptPath), { recursive: true });
  const sourceCommit = spawnSync("git", ["rev-parse", "HEAD"], { encoding: "utf8" }).stdout.trim();
  writeFileSync(receiptPath, `${JSON.stringify({ schema: "matrix-gate-step-receipt/v1", mode,
    run_id: runId, job, step_name: "Gate", outcome, source_commit: sourceCommit }, null, 2)}\n`);
  return receiptPath;
}
function workflowArtifactPaths(workflow, visualLint) {
  const lines = workflow.split(/\r?\n/);
  const steps = [];
  for (let i = 0; i < lines.length; i++) {
    if (!/^      - /.test(lines[i])) continue;
    let end = i + 1;
    while (end < lines.length && !/^      - /.test(lines[end])) end++;
    steps.push(lines.slice(i, end));
    i = end - 1;
  }
  const condition = visualLint ? /startsWith\(matrix\.mode, '--browser-visual-lint'\)/ : /!startsWith\(matrix\.mode, '--browser-visual-lint'\)/;
  const step = steps.find((block) => block.some((line) => line.includes("actions/upload-artifact@") && condition.test(line)));
  assert.ok(step, `workflow has the ${visualLint ? "visual" : "ordinary"} artifact upload`);
  const pathIndex = step.findIndex((line) => /^\s+path:\s*\|\s*$/.test(line));
  assert.notEqual(pathIndex, -1, "upload step declares a path list");
  const indent = step[pathIndex].match(/^\s*/)[0].length;
  const paths = [];
  for (const line of step.slice(pathIndex + 1)) {
    const lineIndent = line.match(/^\s*/)[0].length;
    if (line.trim() && lineIndent <= indent) break;
    if (line.trim()) paths.push(line.trim());
  }
  return paths;
}
function artifactPathMatches(pattern, relativePath, runId) {
  const expanded = pattern.replaceAll("${{ github.run_id }}", runId).replaceAll("${{ github.job }}", "gate");
  if (expanded.endsWith("/")) return relativePath.startsWith(expanded);
  const source = expanded.split(/(\*\*|\*)/).map((part) => part === "**" ? ".*" : part === "*" ? "[^/]*" : part.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")).join("");
  return new RegExp(`^${source}$`).test(relativePath);
}
function filesUnder(root) {
  const files = [];
  if (!existsSync(root)) return files;
  for (const entry of readdirSync(root, { withFileTypes: true })) {
    const path = join(root, entry.name);
    if (entry.isDirectory()) files.push(...filesUnder(path));
    else if (entry.isFile()) files.push(path);
  }
  return files;
}
function assembleUploadedArtifacts(workflow, producers, downloadRoot, runId, omittedMode = null) {
  for (const { mode, root: producerRoot } of producers) {
    if (mode === omittedMode) continue;
    const paths = workflowArtifactPaths(workflow, mode.startsWith("--browser-visual-lint"));
    const artifactRoot = resolve(downloadRoot, mode.replace(/[^A-Za-z0-9_.-]/g, "_"));
    for (const file of filesUnder(producerRoot)) {
      const relativePath = file.slice(producerRoot.length + 1).split("\\").join("/");
      if (!paths.some((pattern) => artifactPathMatches(pattern, relativePath, runId))) continue;
      const output = resolve(artifactRoot, relativePath);
      mkdirSync(dirname(output), { recursive: true });
      copyFileSync(file, output);
    }
  }
}
function historicalGoEvents({ reason } = {}) {
  const packageName = "livecommerce/tests/foundation";
  const { family } = goFamily();
  const drift = canonicalEvidenceDrift();
  const skipReason = reason ?? `canonical evidence is historical (source drift: ${drift.join(", ")})`;
  return [{ Package: packageName, Action: "start" }, ...family.flatMap((Test) => Test === "TestProductEditorMatrixLedgerEvidence"
    ? [{ Package: packageName, Test, Action: "output", Output: `    catalog_matrix_ledger_test.go:123: ${skipReason}\n` }, { Package: packageName, Test, Action: "skip" }]
    : [{ Package: packageName, Test, Action: "pass" }])];
}

test("independent historical drift accepts a removed bound producer but refuses links and unsafe source paths", () => {
  const root = probePath("removed-producer");
  rmSync(root, { recursive: true, force: true });
  const gate = "tests/foundation/catalog_matrix_ledger_test.go";
  const producer = "tests/admin/old-matrix-producer.ts";
  const gateBytes = Buffer.from(`var productEditorMatrixEvidenceSources = []string{\n"${gate}",\n"${producer}",\n}\n`);
  const hash = (bytes) => createHash("sha256").update(bytes).digest("hex");
  const put = (path, bytes) => { mkdirSync(dirname(resolve(root, path)), { recursive: true }); writeFileSync(resolve(root, path), bytes); };
  put(gate, gateBytes);
  put(producer, "tested producer\n");
  put("output/matrix-ledger-enforce/DELIVERY.md", "[SOURCE.json](fixture/SOURCE.json)\n");
  const sources = { [gate]: hash(gateBytes), [producer]: hash(Buffer.from("tested producer\n")) };
  const sourcePath = "output/matrix-ledger-enforce/fixture/SOURCE.json";
  const bind = (bound = sources) => put(sourcePath, JSON.stringify({ source_files_sha256: bound }));
  try {
    bind();
    assert.deepEqual(canonicalEvidenceDrift(null, root), []);
    rmSync(resolve(root, producer));
    assert.deepEqual(canonicalEvidenceDrift(null, root), [producer], "removed SOURCE-bound producer is exact historical drift");
    symlinkSync("missing-producer.ts", resolve(root, producer));
    assert.throws(() => canonicalEvidenceDrift(null, root), /canonical source symlink/, "dangling links are not removed producers");
    rmSync(resolve(root, producer));
    mkdirSync(resolve(root, producer));
    assert.throws(() => canonicalEvidenceDrift(null, root), /EISDIR/, "a source directory remains an I/O failure");
    rmSync(resolve(root, producer), { recursive: true });
    bind({ ...sources, "../escape.ts": hash(Buffer.from("escape")) });
    assert.throws(() => canonicalEvidenceDrift(null, root), /repository-relative/);
    bind({ ...sources, [producer]: "invalid" });
    assert.throws(() => canonicalEvidenceDrift(null, root), /invalid bound hash/);
  } finally { rmSync(root, { recursive: true, force: true }); }
});

test("Node receipt accepts exact required suites with individual passing case identities", () => {
  const result = nodeOutcome(nodeEvents(), nodeSuites);
  assert.equal(result.ok, true, result.violations.join("; "));
  assert.equal(result.cases.length, Object.values(nodeCaseInventory.suites).reduce((sum, suite) => sum + suite.tests.length, 0));
});
test("Node receipt rejects selected-out cases, only cases, a missing case and a missing suite file", () => {
  assert.equal(nodeOutcome(nodeEvents({ skipped: true }), nodeSuites).ok, false);
  assert.equal(nodeOutcome(nodeEvents({ only: true }), nodeSuites).ok, false);
  assert.equal(nodeOutcome(nodeEvents({ removeCase: `${nodeSuites[0]}\0${nodeCaseInventory.suites[nodeSuites[0]].tests[0]}` }), nodeSuites).ok, false,
    "a filtered case cannot hide behind matching per-file summary counts");
  assert.equal(nodeOutcome(nodeEvents({ omit: nodeSuites[0] }), nodeSuites).ok, false);
});
test("Node_OPTIONS name filtering cannot create a positive actual receipt", () => {
  const path = probePath("node-options-preload.json");
  mkdirSync(dirname(path), { recursive: true });
  const preload = probePath("exit-zero.cjs");
  writeFileSync(preload, "process.exit(0);\n");
  const env = { ...process.env, NODE_OPTIONS: `--require=${preload}`, LC_MATRIX_RECEIPT_PATH: path,
    LC_MATRIX_RECEIPT_MODE: "probe-node-filter", GITHUB_RUN_ID: "probe", GITHUB_JOB: "probe",
  };
  const result = spawnSync(process.execPath, ["scripts/dev/matrix-receipt.mjs", "node"], { encoding: "utf8", env, timeout: 120_000 });
  assert.equal(result.error, undefined);
  assert.equal(result.status, 0, `${result.stdout}\n${result.stderr}`);
  assert.equal(existsSync(path), false, "preload exited before receipt execution");
  const plan = JSON.parse(readFileSync("scripts/dev/shard-plan.json", "utf8"));
  const matrix = JSON.stringify(["shard:unit", `shard:${plan.groups.find((group) => group.catch_all).id}`].map((mode) => ({ mode })));
  const head = spawnSync("git", ["rev-parse", "HEAD"], { encoding: "utf8" }).stdout.trim();
  const empty = probePath("empty");
  mkdirSync(empty, { recursive: true });
  for (const { mode } of JSON.parse(matrix)) persistGateStepResult(resolve(empty, "gate-results"), mode, "probe");
  const verify = spawnSync(process.execPath, ["scripts/dev/matrix-receipt-verify.mjs", "required",
    empty, "probe", head, matrix, "workflow_dispatch"], { encoding: "utf8",
  env: { ...process.env, NODE_OPTIONS: "" } });
  assert.equal(verify.status, 1, "missing actual receipts fail the required aggregator");
  assert.match(verify.stderr, /expected exactly one node receipt/);
});

test("Go receipt derives the catch-all family and requires every actual pass event", () => {
  const { family, shard } = goFamily();
  assert.equal(shard, JSON.parse(readFileSync("scripts/dev/shard-plan.json", "utf8")).groups.find((group) => group.catch_all).id);
  const packageName = "livecommerce/tests/foundation";
  const events = [{ Package: packageName, Action: "start" }, ...family.map((Test) => ({ Package: packageName, Test, Action: "pass" }))];
  assert.equal(goOutcome(events, family).ok, true);
  assert.equal(goOutcome(events.filter((event) => event.Test !== family[0]), family).ok, false);
  assert.equal(goOutcome(events.map((event) => event.Test === family[0] ? { ...event, Action: "skip" } : event), family).ok, false);
  assert.equal(goOutcome([...events, { Package: packageName, Test: family[0], Action: "pass" }], family).ok, false);
});

test("Go family cannot remove or rename the canonical historical-evidence test", () => {
  const directory = probePath("required-go-identity");
  const { family } = goFamily();
  const required = "TestProductEditorMatrixLedgerEvidence";
  mkdirSync(directory, { recursive: true });
  const put = (names) => writeFileSync(resolve(directory, "fixture_test.go"),
    names.map((name) => `func ${name}(t *testing.T) {}`).join("\n"));
  try {
    put(family);
    assert.deepEqual(goFamily(directory).family, family);
    put(family.filter((name) => name !== required));
    assert.throws(() => goFamily(directory), /required canonical evidence test/, "removed test cannot erase the browser backstop");
    put(family.map((name) => name === required ? `${required}Renamed` : name));
    assert.throws(() => goFamily(directory), /required canonical evidence test/, "renamed test cannot erase the browser backstop");
  } finally { rmSync(directory, { recursive: true, force: true }); }
});

test("Go family discovery is independent of testing parameter names and formatting", () => {
  const directory = probePath("go-signature-forms");
  const { family } = goFamily();
  mkdirSync(directory, { recursive: true });
  const put = (source) => writeFileSync(resolve(directory, "fixture_test.go"), source);
  try {
    const declarations = family.map((name, index) => {
      if (index % 3 === 0) return `func ${name}(tb *testing.T) {}`;
      if (index % 3 === 1) return `func ${name}(*testing.T) {}`;
      return `func ${name}(\n\ttb *testing.T,\n) {}`;
    });
    put(`package foundation\n\n${declarations.join("\n\n")}\n`);
    assert.deepEqual(goFamily(directory).family, family);
  } finally { rmSync(directory, { recursive: true, force: true }); }
});

test("Go outcomes reject a reduced family even when every remaining test passed", () => {
  const family = goFamily().family.filter((name) => name !== "TestProductEditorMatrixLedgerEvidence");
  const events = [{ Package: "livecommerce/tests/foundation", Action: "start" },
    ...family.map((Test) => ({ Package: "livecommerce/tests/foundation", Test, Action: "pass" }))];
  const reduced = goOutcome(events, family);
  assert.equal(reduced.ok, false, "all remaining passes do not authorize dropping the mandatory evidence test");
  assert.ok(reduced.violations.some((message) => /required canonical evidence test/.test(message)));
});

test("historical canonical skip needs exact real drift and cannot be a matching-source or wrong-reason skip", () => {
  const event = [{ Test: "TestProductEditorMatrixLedgerEvidence", Action: "skip" }];
  const drift = ["tests/admin/product-editor.acceptance.ts"];
  assert.equal(validateHistoricalSkip(event, "canonical evidence is historical (source drift: tests/admin/product-editor.acceptance.ts)", drift).ok, true);
  assert.equal(validateHistoricalSkip(event, "canonical evidence is historical (source drift: tests/admin/product-editor.acceptance.ts)", []).ok, false);
  assert.equal(validateHistoricalSkip(event, "skip: changed file", drift).ok, false);
  assert.equal(validateHistoricalSkip([...event, { Test: "TestProductEditorMatrixLedger", Action: "skip" }],
    "canonical evidence is historical (source drift: tests/admin/product-editor.acceptance.ts)", drift).ok, false);
});

test("fresh live browser backstop requires the actual product editor browser test pass", () => {
  assert.equal(browserOutcome([]).ok, false);
  const packageName = "livecommerce/tests/foundation";
  const allPass = ["TestBrowserCatalogCore", "TestBrowserCatalogCore/PE12-17-TWD", "TestBrowserCatalogCore/CC12-frozen"]
    .map((Test) => ({ Package: packageName, Test, Action: "pass" }));
  assert.equal(browserOutcome([{ Package: packageName, Test: "TestBrowserCatalogCore", Action: "pass" },
    { Package: packageName, Test: "TestBrowserCatalogCore/CC12-frozen", Action: "pass" }]).ok, false,
  "parent and CC12 do not prove PRODUCT_EDITOR_ACCEPTANCE executed");
  assert.equal(browserOutcome([{ ...allPass[1], Action: "skip" }, ...allPass.slice(0, 1), ...allPass.slice(2)]).ok, false);
  assert.equal(browserOutcome(allPass).ok, true);
});

test("required verifier recomputes event outcomes and rejects a rehashed outcome forgery", () => {
  const events = nodeEvents();
  const path = probePath("raw", "node.json");
  mkdirSync(dirname(path), { recursive: true });
  const raw = events.map((event) => JSON.stringify(event)).join("\n") + "\n";
  const eventFile = "node.json.events";
  writeFileSync(resolve(dirname(path), eventFile), raw);
  const identity = spawnSync("git", ["rev-parse", "HEAD"], { encoding: "utf8" }).stdout.trim();
  const hashes = sourceHashes();
  const outcome = nodeOutcome(events, nodeSuites);
  const receipt = { schema: "matrix-execution-receipt/v1", kind: "node", mode: "shard:unit", run_id: "probe", job: "gate",
    source_before: { commit: identity, hashes }, source_after: { commit: identity, hashes }, exit_code: 0,
    event_file: eventFile, event_sha256: createHash("sha256").update(raw).digest("hex"), outcome, violations: [] };
  assert.deepEqual(verifyReceipt(receipt, { receiptPath: resolve(path), kind: "node", mode: "shard:unit", runId: "probe", job: "gate", sourceCommit: identity }), []);
  const forged = events.map((event) => event.type === "test:complete"
    ? { ...event, data: { ...event.data, skip: "selected out", details: { type: "test", passed: false } } } : event);
  const forgedRaw = forged.map((event) => JSON.stringify(event)).join("\n") + "\n";
  writeFileSync(resolve(dirname(path), eventFile), forgedRaw);
  receipt.event_sha256 = createHash("sha256").update(forgedRaw).digest("hex");
  const forgedErrors = verifyReceipt(receipt, { receiptPath: resolve(path), kind: "node", mode: "shard:unit", runId: "probe", job: "gate", sourceCommit: identity });
  assert.ok(forgedErrors.some((issue) => /independently parsed raw events/.test(issue)), forgedErrors.join("; "));
});

test("required verifier accepts a source-drift skip only with a verified same-run browser receipt", () => {
  const root = probePath("historical-required");
  rmSync(root, { recursive: true, force: true });
  const runId = "historical-probe";
  const head = spawnSync("git", ["rev-parse", "HEAD"], { encoding: "utf8" }).stdout.trim();
  const plan = JSON.stringify(["shard:unit", `shard:${goFamily().shard}`, "--browser-product-editor"].map((mode) => ({ mode })));
  const nodeDir = resolve(root, "node");
  const goDir = resolve(root, "go");
  persistReceipt(nodeDir, "node", "shard:unit", runId, nodeEvents());
  const goEvents = historicalGoEvents();
  const goReceipt = persistReceipt(goDir, "go", `shard:${goFamily().shard}`, runId, goEvents);
  assert.equal(JSON.parse(readFileSync(goReceipt, "utf8")).outcome.ok, true, "real checkout drift and exact source list validate at Go receipt creation");
  for (const { mode } of JSON.parse(plan)) persistGateStepResult(resolve(root, "gate-results"), mode, runId);

  const verify = (id) => spawnSync(process.execPath, ["scripts/dev/matrix-receipt-verify.mjs", "required", root, id, head, plan, "workflow_dispatch"],
    { encoding: "utf8", env: { ...process.env, NODE_OPTIONS: "" } });
  const missingBrowser = verify(runId);
  writeFileSync(resolve(root, "red-missing-browser.log"), `${missingBrowser.stdout}${missingBrowser.stderr}`);
  assert.equal(missingBrowser.status, 1);
  assert.match(missingBrowser.stderr, /same-run browser receipt/);

  const browserEvents = ["TestBrowserCatalogCore", "TestBrowserCatalogCore/PE12-17-TWD", "TestBrowserCatalogCore/CC12-frozen"]
    .map((Test) => ({ Package: "livecommerce/tests/foundation", Test, Action: "pass" }));
  persistReceipt(resolve(root, "browser-wrong-run"), "browser", "--browser-product-editor", `${runId}-old`, browserEvents,
    ["PRODUCT_EDITOR_ACCEPTANCE=1", "LC_BROWSER_CATALOG_CORE_ACCEPTANCE=1", "GOTOOLCHAIN=go1.27.2", "go", "test", "-race", "-tags", "browser", "-count=1", "-run", "^TestBrowserCatalogCore$", "./tests/foundation"]);
  const wrongRun = verify(runId);
  writeFileSync(resolve(root, "red-wrong-run-browser.log"), `${wrongRun.stdout}${wrongRun.stderr}`);
  assert.equal(wrongRun.status, 1);
  assert.match(wrongRun.stderr, /same-run browser receipt/);

  persistReceipt(resolve(root, "browser"), "browser", "--browser-product-editor", runId, browserEvents,
    ["PRODUCT_EDITOR_ACCEPTANCE=1", "LC_BROWSER_CATALOG_CORE_ACCEPTANCE=1", "GOTOOLCHAIN=go1.27.2", "go", "test", "-race", "-tags", "browser", "-count=1", "-run", "^TestBrowserCatalogCore$", "./tests/foundation"]);
  writeFileSync(resolve(root, "unrelated-producer.json"), "");
  writeFileSync(resolve(root, "unrelated-ledger.json"), "not a receipt");
  const accepted = verify(runId);
  assert.equal(accepted.status, 0, `${accepted.stdout}\n${accepted.stderr}`);
});

test("required verifier requires one successful named Gate step receipt for every planned mode", () => {
  const root = probePath("required-gate-mode-results");
  const runId = "planned-gate-mode-probe";
  const head = spawnSync("git", ["rev-parse", "HEAD"], { encoding: "utf8" }).stdout.trim();
  const modes = ["shard:unit", `shard:${goFamily().shard}`, "--browser-catalog-core"];
  const plan = JSON.stringify(modes.map((mode) => ({ mode })));
  persistReceipt(resolve(root, "node"), "node", modes[0], runId, nodeEvents());
  const family = goFamily().family;
  const goEvents = [{ Package: "livecommerce/tests/foundation", Action: "start" },
    ...family.map((Test) => ({ Package: "livecommerce/tests/foundation", Test, Action: "pass" }))];
  persistReceipt(resolve(root, "go"), "go", modes[1], runId, goEvents);
  const verify = () => spawnSync(process.execPath,
    ["scripts/dev/matrix-receipt-verify.mjs", "required", root, runId, head, plan, "workflow_dispatch"],
    { encoding: "utf8", env: { ...process.env, NODE_OPTIONS: "" } });

  const missing = verify();
  assert.equal(missing.status, 1, `${missing.stdout}\n${missing.stderr}`);
  assert.match(missing.stderr, /Gate step.*receipt/i);

  for (const mode of modes) persistGateStepResult(resolve(root, "gate-results"), mode, runId);
  const complete = verify();
  assert.equal(complete.status, 0, `${complete.stdout}\n${complete.stderr}`);

  const gatePath = resolve(root, "gate-results", `${modes[2].replace(/[^A-Za-z0-9_.-]/g, "_")}.gate.json`);
  const valid = JSON.parse(readFileSync(gatePath, "utf8"));
  writeFileSync(gatePath, JSON.stringify({ ...valid, source_commit: `${head}0` }));
  const wrongSource = verify();
  assert.equal(wrongSource.status, 1, `${wrongSource.stdout}\n${wrongSource.stderr}`);
  assert.match(wrongSource.stderr, /Gate step.*source/i);

  writeFileSync(gatePath, JSON.stringify({ ...valid, run_id: `${runId}-old` }));
  const wrongRun = verify();
  assert.equal(wrongRun.status, 1, `${wrongRun.stdout}\n${wrongRun.stderr}`);
  assert.match(wrongRun.stderr, /Gate step.*same-run/i);

  persistGateStepResult(resolve(root, "gate-results"), modes[2], runId, "skipped");
  const skipped = verify();
  assert.equal(skipped.status, 1, `${skipped.stdout}\n${skipped.stderr}`);
  assert.match(skipped.stderr, /Gate step.*(skipped|outcome|receipt)/i);
});

test("visual-lint and ordinary artifact uploads both deliver every planned Gate receipt", () => {
  const workflow = readFileSync(".github/workflows/gates.yml", "utf8");
  const runId = "gate-artifact-upload-probe";
  const shard = goFamily().shard;
  const modes = ["shard:unit", `shard:${shard}`, "--browser-catalog-core", "--browser-visual-lint@0/1"];
  const plan = JSON.stringify(modes.map((mode) => ({ mode })));
  const head = spawnSync("git", ["rev-parse", "HEAD"], { encoding: "utf8" }).stdout.trim();
  const family = goFamily().family;
  const producers = modes.map((mode) => {
    const root = probePath("gate-artifact-producers", mode.replace(/[^A-Za-z0-9_.-]/g, "_"));
    const jobRoot = resolve(root, "output", "matrix-coverage-v2-execution", runId, "gate");
    persistGateStepResult(resolve(jobRoot, "gate-results"), mode, runId);
    if (mode === "shard:unit") persistReceipt(jobRoot, "node", mode, runId, nodeEvents());
    if (mode === `shard:${shard}`) {
      const events = [{ Package: "livecommerce/tests/foundation", Action: "start" },
        ...family.map((Test) => ({ Package: "livecommerce/tests/foundation", Test, Action: "pass" }))];
      persistReceipt(jobRoot, "go", mode, runId, events);
    }
    return { mode, root };
  });
  const visualGatePath = `output/matrix-coverage-v2-execution/${runId}/gate/gate-results/${modes[3].replace(/[^A-Za-z0-9_.-]/g, "_")}.gate.json`;
  assert.ok(workflowArtifactPaths(workflow, true).some((pattern) => artifactPathMatches(pattern, visualGatePath, runId)),
    "the dedicated visual-lint upload includes the actual Gate receipt path");

  const verifyArtifacts = (omittedMode = null) => {
    const downloadRoot = probePath("gate-artifact-downloads", omittedMode?.replace(/[^A-Za-z0-9_.-]/g, "_") || "complete");
    assembleUploadedArtifacts(workflow, producers, downloadRoot, runId, omittedMode);
    return spawnSync(process.execPath,
      ["scripts/dev/matrix-receipt-verify.mjs", "required", downloadRoot, runId, head, plan, "pull_request"],
      { encoding: "utf8", env: { ...process.env, NODE_OPTIONS: "" } });
  };
  const complete = verifyArtifacts();
  assert.equal(complete.status, 0, `${complete.stdout}\n${complete.stderr}`);
  for (const mode of modes) {
    const missing = verifyArtifacts(mode);
    assert.equal(missing.status, 1, `omitting ${mode} must fail required: ${missing.stdout}\n${missing.stderr}`);
    assert.match(missing.stderr, /Gate step.*receipt/i);
  }
});

test("required verifier reports a missing Go receipt after a valid Node receipt", () => {
  const root = probePath("required-missing-go");
  const runId = "missing-go-probe";
  const head = spawnSync("git", ["rev-parse", "HEAD"], { encoding: "utf8" }).stdout.trim();
  const modes = ["shard:unit", `shard:${goFamily().shard}`];
  const plan = JSON.stringify(modes.map((mode) => ({ mode })));
  persistReceipt(resolve(root, "node"), "node", modes[0], runId, nodeEvents());
  for (const mode of modes) persistGateStepResult(resolve(root, "gate-results"), mode, runId);
  const missingGo = spawnSync(process.execPath,
    ["scripts/dev/matrix-receipt-verify.mjs", "required", root, runId, head, plan, "workflow_dispatch"],
    { encoding: "utf8", env: { ...process.env, NODE_OPTIONS: "" } });
  assert.equal(missingGo.status, 1, `${missingGo.stdout}\n${missingGo.stderr}`);
  assert.match(missingGo.stderr, /expected exactly one go receipt/);
});

test("Gates captures the actual named Gate step outcome into the required receipt path", () => {
  const workflow = readFileSync(".github/workflows/gates.yml", "utf8");
  assert.match(workflow, /- name: Product-editor matrix Node suites\n\s+id: node-matrix\n\s+if: matrix\.mode == 'shard:unit' \|\| matrix\.mode == 'foundation'[\s\S]*?LC_MATRIX_RECEIPT_MODE: \$\{\{ matrix\.mode \}\}/);
  assert.match(workflow, /- name: Gate\n\s+id: gate\n/);
  assert.match(workflow, /- name: Capture named Gate step execution receipt\n\s+if: always\(\)[\s\S]*?LC_MATRIX_GATE_MODE: \$\{\{ matrix\.mode \}\}[\s\S]*?LC_MATRIX_GATE_OUTCOME: \$\{\{ steps\.gate\.outcome \}\}[\s\S]*?matrix-receipt\.mjs gate-step/);
});

test("Go execution receipts bind the actual race-enabled command", () => {
  const root = probePath("go-race-command");
  const runId = "go-race-command-probe";
  const head = spawnSync("git", ["rev-parse", "HEAD"], { encoding: "utf8" }).stdout.trim();
  const { family, shard } = goFamily();
  const events = [{ Package: "livecommerce/tests/foundation", Action: "start" },
    ...family.map((Test) => ({ Package: "livecommerce/tests/foundation", Test, Action: "pass" }))];
  const command = ["GOTOOLCHAIN=go1.27.2", "go", "test", "-p", "1", "-race", "-count=1", "./tests/foundation"];
  const receiptPath = persistReceipt(root, "go", `shard:${shard}`, runId, events, command);
  const receipt = JSON.parse(readFileSync(receiptPath, "utf8"));
  const context = { receiptPath, kind: "go", mode: `shard:${shard}`, runId, job: "gate", sourceCommit: head };
  assert.deepEqual(verifyReceipt(receipt, context), []);

  receipt.command = receipt.command.filter((argument) => argument !== "-race");
  const errors = verifyReceipt(receipt, context);
  assert.ok(errors.some((issue) => /-race/.test(issue)), errors.join("; "));
  receipt.command = [...command, "-race=false"];
  const disabledErrors = verifyReceipt(receipt, context);
  assert.ok(disabledErrors.some((issue) => /disables -race/.test(issue)), disabledErrors.join("; "));
});

test("manual full foundation execution cannot bypass historical-skip browser verification", () => {
  const root = probePath("manual-foundation-required");
  const runId = "manual-foundation-probe";
  const head = spawnSync("git", ["rev-parse", "HEAD"], { encoding: "utf8" }).stdout.trim();
  const modes = ["foundation", "--browser-product-editor"];
  const plan = JSON.stringify(modes.map((mode) => ({ mode })));
  persistReceipt(resolve(root, "node"), "node", "foundation", runId, nodeEvents(), [], "gate");
  const goPath = persistReceipt(resolve(root, "go"), "go", "foundation", runId, historicalGoEvents());
  assert.equal(JSON.parse(readFileSync(goPath, "utf8")).outcome.historical_skip?.ok, true);
  for (const mode of modes) persistGateStepResult(resolve(root, "gate-results"), mode, runId);
  const verify = () => spawnSync(process.execPath,
    ["scripts/dev/matrix-receipt-verify.mjs", "required", root, runId, head, plan, "workflow_dispatch"],
    { encoding: "utf8", env: { ...process.env, NODE_OPTIONS: "" } });

  const missingBrowser = verify();
  assert.equal(missingBrowser.status, 1, `${missingBrowser.stdout}\n${missingBrowser.stderr}`);
  assert.match(missingBrowser.stderr, /same-run browser receipt/);

  const browserEvents = ["TestBrowserCatalogCore", "TestBrowserCatalogCore/PE12-17-TWD", "TestBrowserCatalogCore/CC12-frozen"]
    .map((Test) => ({ Package: "livecommerce/tests/foundation", Test, Action: "pass" }));
  persistReceipt(resolve(root, "browser"), "browser", "--browser-product-editor", runId, browserEvents,
    ["PRODUCT_EDITOR_ACCEPTANCE=1", "LC_BROWSER_CATALOG_CORE_ACCEPTANCE=1", "GOTOOLCHAIN=go1.27.2", "go", "test", "-race", "-tags", "browser", "-count=1", "-run", "^TestBrowserCatalogCore$", "./tests/foundation"]);
  const withBrowser = verify();
  assert.equal(withBrowser.status, 0, `${withBrowser.stdout}\n${withBrowser.stderr}`);
});

test("Foundation verifier requires fresh same-run browser proof only for a historical Go skip", () => {
  const root = probePath("foundation-historical");
  rmSync(root, { recursive: true, force: true });
  const runId = "foundation-historical-probe";
  const head = spawnSync("git", ["rev-parse", "HEAD"], { encoding: "utf8" }).stdout.trim();
  const browserCommand = ["PRODUCT_EDITOR_ACCEPTANCE=1", "LC_BROWSER_CATALOG_CORE_ACCEPTANCE=1", "GOTOOLCHAIN=go1.27.2", "go", "test", "-race", "-tags", "browser", "-count=1", "-run", "^TestBrowserCatalogCore$", "./tests/foundation"];
  const runVerifier = (goEvents, browserOutcome, browserRunId = null) => {
    rmSync(root, { recursive: true, force: true });
    persistReceipt(root, "node", "foundation", runId, nodeEvents(), [], "foundation");
    persistReceipt(root, "go", "foundation", runId, goEvents, [], "foundation");
    if (browserRunId !== null) {
      const browserEvents = ["TestBrowserCatalogCore", "TestBrowserCatalogCore/PE12-17-TWD", "TestBrowserCatalogCore/CC12-frozen"]
        .map((Test) => ({ Package: "livecommerce/tests/foundation", Test, Action: "pass" }));
      persistReceipt(root, "browser", "--browser-product-editor", browserRunId, browserEvents, browserCommand, "foundation");
    }
    return spawnSync(process.execPath, ["scripts/dev/matrix-receipt-verify.mjs", "foundation", root, "success", "success",
      browserOutcome, "true", runId, head], { encoding: "utf8", env: { ...process.env, GITHUB_JOB: "foundation", NODE_OPTIONS: "" } });
  };
  try {
    const { family } = goFamily();
    const passingGoEvents = [{ Package: "livecommerce/tests/foundation", Action: "start" },
      ...family.map((Test) => ({ Package: "livecommerce/tests/foundation", Test, Action: "pass" }))];
    const detectorDir = probePath("foundation-detector");
    persistReceipt(detectorDir, "go", "foundation", runId, passingGoEvents, [], "foundation");
    assert.equal(foundationBrowserRequired(detectorDir, "success", runId, head), false,
      "a current all-PASS Go receipt does not request browser work");
    persistReceipt(detectorDir, "go", "foundation", runId, historicalGoEvents(), [], "foundation");
    assert.equal(foundationBrowserRequired(detectorDir, "success", runId, head), true,
      "only an independently verified historical skip schedules browser work");
    assert.throws(() => foundationBrowserRequired(detectorDir, "failure", runId, head), /Go runner outcome/);

    const allPass = runVerifier(passingGoEvents, "skipped");
    assert.equal(allPass.status, 0, `${allPass.stdout}\n${allPass.stderr}`);

    const historical = historicalGoEvents();
    const missing = runVerifier(historical, "skipped");
    assert.equal(missing.status, 1);
    assert.match(missing.stderr, /same-run browser receipt/);

    const wrongRun = runVerifier(historical, "success", `${runId}-old`);
    assert.equal(wrongRun.status, 1);
    assert.match(wrongRun.stderr, /same-run browser receipt/);

    const failedBrowser = runVerifier(historical, "failure", runId);
    assert.equal(failedBrowser.status, 1);
    assert.match(failedBrowser.stderr, /browser.*outcome|browser.*receipt/i);

    const accepted = runVerifier(historical, "success", runId);
    assert.equal(accepted.status, 0, `${accepted.stdout}\n${accepted.stderr}`);
  } finally { rmSync(root, { recursive: true, force: true }); }
});

test("Go historical skip rejects a matching-source skip and a wrong reason at the verifier's parser boundary", () => {
  const event = [{ Test: "TestProductEditorMatrixLedgerEvidence", Action: "skip" }];
  const gate = readFileSync("tests/foundation/catalog_matrix_ledger_test.go", "utf8");
  const body = /var productEditorMatrixEvidenceSources = \[\]string\{([\s\S]*?)\n\}/.exec(gate)[1];
  const bound = [...body.matchAll(/"([^"\n]+)"/g)].map((match) => match[1]);
  const source = { source_files_sha256: Object.fromEntries(bound.map((path) => [path,
    createHash("sha256").update(readFileSync(path)).digest("hex")])) };
  const directory = probePath("source-facts");
  mkdirSync(directory, { recursive: true });
  const matchingPath = resolve(directory, "matching-SOURCE.json");
  writeFileSync(matchingPath, JSON.stringify(source));
  const matchingDrift = canonicalEvidenceDrift(matchingPath);
  assert.deepEqual(matchingDrift, [], "independent hashing of current bound files produces no drift");
  const matchingReason = "canonical evidence is historical (source drift: )";
  assert.equal(validateHistoricalSkip(event, matchingReason, matchingDrift).ok, false,
    "the same reason with matching source hashes is RED");

  const changedSource = structuredClone(source);
  changedSource.source_files_sha256[bound[0]] = "0".repeat(64);
  const changedPath = resolve(directory, "changed-SOURCE.json");
  writeFileSync(changedPath, JSON.stringify(changedSource));
  const drift = canonicalEvidenceDrift(changedPath);
  assert.deepEqual(drift, [bound[0]], "the verifier recomputes a real current-checkout hash mismatch");
  const exact = `canonical evidence is historical (source drift: ${drift.join(", ")})`;
  assert.equal(validateHistoricalSkip(event, exact, drift).ok, true);
  assert.equal(validateHistoricalSkip(event, "operator requested skip", drift).ok, false);
});
