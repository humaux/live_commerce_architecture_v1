// Purpose: prove actual matrix receipts require per-file Node passes, every Go family pass and a same-run browser backstop for a historical skip.
// Depends on: the receipt parser, current shard plan, and isolated output-directory child probes.
// Used by: test-node.sh and product-editor matrix gate maintenance.
import test from "node:test";
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { readFileSync, mkdirSync, writeFileSync, existsSync, rmSync, symlinkSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { createHash } from "node:crypto";
import { runInNewContext } from "node:vm";
import { nodeSuites, nodeCaseInventory, nodeOutcome, goOutcome, goFamily, validateHistoricalSkip, browserOutcome, sourceHashes, canonicalEvidenceDrift } from "../../scripts/dev/matrix-receipt.mjs";
import { verifyReceipt } from "../../scripts/dev/matrix-receipt-verify.mjs";

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
function persistReceipt(directory, kind, mode, runId, events, command = []) {
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
  const receipt = { schema: "matrix-execution-receipt/v1", kind, mode, run_id: runId, job: "gate", command,
    source_before: { commit: identity, hashes }, source_after: { commit: identity, hashes }, exit_code: 0,
    event_file: `${kind}.json.events`, event_sha256: createHash("sha256").update(raw).digest("hex"), outcome, violations: [] };
  if (kind === "go") receipt.shard = goFamily().shard;
  writeFileSync(receiptPath, `${JSON.stringify(receipt, null, 2)}\n`);
  return receiptPath;
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
  const root = resolve("output/matrix-coverage-v2-execution/probes/removed-producer");
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
  const path = "output/matrix-coverage-v2-execution/probes/node-options-preload.json";
  mkdirSync(dirname(path), { recursive: true });
  const preload = resolve("output/matrix-coverage-v2-execution/probes/exit-zero.cjs");
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
  mkdirSync("output/matrix-coverage-v2-execution/probes/empty", { recursive: true });
  const verify = spawnSync(process.execPath, ["scripts/dev/matrix-receipt-verify.mjs", "required",
    "output/matrix-coverage-v2-execution/probes/empty", "probe", head, matrix, "workflow_dispatch"], { encoding: "utf8",
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
  const directory = resolve("output/matrix-coverage-v2-execution/probes/required-go-identity");
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
  const path = "output/matrix-coverage-v2-execution/probes/raw/node.json";
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
  const root = "output/matrix-coverage-v2-execution/probes/historical-required";
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

  const verify = (id) => spawnSync(process.execPath, ["scripts/dev/matrix-receipt-verify.mjs", "required", root, id, head, plan, "workflow_dispatch"],
    { encoding: "utf8", env: { ...process.env, NODE_OPTIONS: "" } });
  const missingBrowser = verify(runId);
  writeFileSync(resolve(root, "red-missing-browser.log"), `${missingBrowser.stdout}${missingBrowser.stderr}`);
  assert.equal(missingBrowser.status, 1);
  assert.match(missingBrowser.stderr, /same-run browser receipt/);

  const browserEvents = ["TestBrowserCatalogCore", "TestBrowserCatalogCore/PE12-17-TWD", "TestBrowserCatalogCore/CC12-frozen"]
    .map((Test) => ({ Package: "livecommerce/tests/foundation", Test, Action: "pass" }));
  persistReceipt(resolve(root, "browser-wrong-run"), "browser", "--browser-product-editor", `${runId}-old`, browserEvents,
    ["PRODUCT_EDITOR_ACCEPTANCE=1", "LC_BROWSER_CATALOG_CORE_ACCEPTANCE=1", "-tags", "browser", "-run", "^TestBrowserCatalogCore$", "./tests/foundation"]);
  const wrongRun = verify(runId);
  writeFileSync(resolve(root, "red-wrong-run-browser.log"), `${wrongRun.stdout}${wrongRun.stderr}`);
  assert.equal(wrongRun.status, 1);
  assert.match(wrongRun.stderr, /same-run browser receipt/);

  persistReceipt(resolve(root, "browser"), "browser", "--browser-product-editor", runId, browserEvents,
    ["PRODUCT_EDITOR_ACCEPTANCE=1", "LC_BROWSER_CATALOG_CORE_ACCEPTANCE=1", "-tags", "browser", "-run", "^TestBrowserCatalogCore$", "./tests/foundation"]);
  writeFileSync(resolve(root, "unrelated-producer.json"), "");
  writeFileSync(resolve(root, "unrelated-ledger.json"), "not a receipt");
  const accepted = verify(runId);
  assert.equal(accepted.status, 0, `${accepted.stdout}\n${accepted.stderr}`);
});

test("Go historical skip rejects a matching-source skip and a wrong reason at the verifier's parser boundary", () => {
  const event = [{ Test: "TestProductEditorMatrixLedgerEvidence", Action: "skip" }];
  const gate = readFileSync("tests/foundation/catalog_matrix_ledger_test.go", "utf8");
  const body = /var productEditorMatrixEvidenceSources = \[\]string\{([\s\S]*?)\n\}/.exec(gate)[1];
  const bound = [...body.matchAll(/"([^"\n]+)"/g)].map((match) => match[1]);
  const source = { source_files_sha256: Object.fromEntries(bound.map((path) => [path,
    createHash("sha256").update(readFileSync(path)).digest("hex")])) };
  const directory = "output/matrix-coverage-v2-execution/probes/source-facts";
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
