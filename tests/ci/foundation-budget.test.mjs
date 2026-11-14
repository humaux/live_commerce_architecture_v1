// Purpose: falsify canonical G07 shard identity/completeness/owner-policy and unchanged strict skip semantics.
// Depends on: existing matrix receipts and release-gate classifier; synthetic MODEL_ONLY fixtures, no Go/PG/browser.
// Used by: direct scripts/dev/test-node.sh execution; runtime qualification remains separate.
import test from "node:test";
import assert from "node:assert/strict";
import { mkdtempSync, readFileSync, writeFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { G07_C1, G07_MATRIX, G07_NODE, G07_ARCHIVE_PY, verifyG07, g07Events, g07Inventory, g07Budget } from "../../scripts/dev/matrix-receipts.mjs";

const P = "example/tests/foundation", U = "example/unit", EMPTY = "example/no_tests";
const rows = (pkg, names, special = {}) => [
  { Package: pkg, Action: "start" },
  ...names.flatMap((name) => [
    { Package: pkg, Test: name, Action: "run" },
    ...(special[name]?.output ? [{ Package: pkg, Test: name, Action: "output", Output: special[name].output }] : []),
    { Package: pkg, Test: name, Action: special[name]?.terminal ?? "pass" },
  ]), { Package: pkg, Action: names.length ? "pass" : "skip" },
];
const encode = (events) => events.map((e) => JSON.stringify(e)).join("\n") + "\n";
const tap = "TAP version 13\nok 1 - actual suite\n1..1\n# tests 1\n# suites 0\n# pass 1\n# fail 0\n# cancelled 0\n# skipped 0\n# todo 0\n# duration_ms 1\n";
function fixture() {
  const identity = { subject_sha: "a".repeat(40), workflow_sha: "b".repeat(40), workflow_ref: "owner/repo/.github/workflows/gates.yml@refs/heads/test",
    run_id: "4242", run_attempt: "2", event: "workflow_dispatch", checkout_sha: "a".repeat(40), checkout_tree: "c".repeat(40) };
  const context = { identity, clean: true, pins: { "scripts/dev/test-local.sh": "f".repeat(64) }, source: "d".repeat(64), plan_hash: "e".repeat(64) };
  const inventory = { [EMPTY]: [], [P]: [G07_C1, G07_MATRIX, "TestProductEditorMatrixLedger", "TestTail"].sort(), [U]: ["TestUnit"] };
  const shardPlan = { groups: [{ id: "g01", tests: [G07_C1, G07_MATRIX], catch_all: false }, { id: "g02", tests: [], catch_all: true }] };
  const modes = ["shard:unit", "shard:g01", "shard:g02", "--browser-product-editor"];
  const binding = { names: ["owner.go", "cases.json"], recorded: { "owner.go": "1".repeat(64), "cases.json": "2".repeat(64) },
    actual: { "owner.go": "3".repeat(64), "cases.json": "2".repeat(64) }, canonical_files: { "SOURCE.json": "4".repeat(64) },
    fresh_files: { "product-editor-click-ledger.json": "5".repeat(64), "cases.json": "2".repeat(64) }, evidence: "native-suite" };
  const common = { schema: "matrix-owner-proof/v1", identity: { ...identity, tracked_clean: "true", job: "gate" } };
  const reason = "canonical evidence is historical (source drift: owner.go)";
  const canonical = { ...structuredClone(common), kind: "canonical", validation: "HISTORICAL_SOURCE_DRIFT", files: binding.canonical_files,
    sources: binding.names.map((name) => ({ path: name, recorded: binding.recorded[name], actual: binding.actual[name] })), drift: ["owner.go"], reason };
  Object.assign(canonical.identity, { mode: "foundation", test: G07_MATRIX, evidence: "unknown" });
  const fresh = { ...structuredClone(common), kind: "fresh-ledger", validation: "LEDGER_VALIDATED", files: binding.fresh_files,
    sources: binding.names.map((name) => ({ path: name, recorded: "unknown", actual: binding.actual[name] })), drift: [], reason: "" };
  Object.assign(fresh.identity, { mode: "--browser-product-editor", test: "TestBrowserCatalogCore", evidence: "native-suite" });
  const make = (mode, go) => ({ mode, official_pass: true, go,
    start: { phase: "start", context: structuredClone(context), mode, anchor: 1000, deadline: 10000, at: 1010 },
    end: { phase: "end", context: structuredClone(context), mode, anchor: 1000, deadline: 10000, at: 1200, exit: 0 } });
  const unit = make(modes[0], encode([...rows(U, ["TestUnit"]), ...rows(EMPTY, [])]));
  unit.vet = [{ phase: "start", command: "go vet ./...", at: 1100 }, { phase: "end", command: "go vet ./...", at: 1110, exit: 0 }];
  unit.node = Object.fromEntries(G07_NODE.map((n) => [n, tap]));
  const shards = [unit,
    make(modes[1], encode(rows(P, [G07_C1, G07_MATRIX], { [G07_MATRIX]: { terminal: "skip", output: `    catalog_matrix_ledger_test.go:1: MATRIX_OWNER_PROOF ${JSON.stringify(canonical)}\n    catalog_matrix_ledger_test.go:2: ${reason}\n` } }))),
    make(modes[2], encode(rows(P, ["TestProductEditorMatrixLedger", "TestTail"]))),
    { ...make(modes[3], encode(rows(P, ["TestBrowserCatalogCore"], { TestBrowserCatalogCore: { output: "    browser_catalog_core_test.go:1: PASS mobile matrix click ledger: 1 declared unique PASS identities; evidence=/runner/output/playwright/run.model/catalog-core/native-suite/product-editor-click-ledger.json\n" } }))), fresh }];
  shards[3].end.evidence = "native-suite";
  const plan = { schema: "g07-shards/v1", context: structuredClone(context), anchor: 1000, deadline: 10000, modes, inventory };
  return { plan, shards, trusted: { context, anchor: 1000, deadline: 10000, now: 1300, modes, inventory, shardPlan, binding } };
}
const verify = (f) => verifyG07(f.plan, f.shards, f.trusted);
const mutateEvents = (f, index, fn) => { f.shards[index].go = encode(fn(f.shards[index].go.trimEnd().split("\n").map(JSON.parse))); };

const editCanonical = (f, change) => mutateEvents(f, 1, (es) => es.map((e) => e.Test === G07_MATRIX && e.Action === "output"
  ? { ...e, Output: e.Output.replace(/MATRIX_OWNER_PROOF (\{[^\n]+\})/, (_match, raw) => { const proof = JSON.parse(raw); change(proof); return `MATRIX_OWNER_PROOF ${JSON.stringify(proof)}`; }) } : e));

test("MODEL_ONLY complete same-attempt fixture: one whole vet, C1, tail, fresh owner facts, historical SKIP retained", () => {
  const result = verify(fixture());
  assert.equal(result.conditional, `${P}\t${G07_MATRIX}`);
  assert.ok(result.events.some((e) => e.Test === G07_MATRIX && e.Action === "skip"));
  assert.ok(!result.events.some((e) => e.Test === G07_MATRIX && e.Action === "pass"));
});
test("MODEL_ONLY matching canonical PASS also requires and binds fresh native proof", () => {
  const f = fixture();
  f.trusted.binding.actual["owner.go"] = f.trusted.binding.recorded["owner.go"];
  f.shards[3].fresh.sources[0].actual = f.trusted.binding.actual["owner.go"];
  mutateEvents(f, 1, (es) => es.filter((e) => !(e.Test === G07_MATRIX && e.Action === "output")).map((e) => e.Test === G07_MATRIX && e.Action === "skip" ? { ...e, Action: "pass" } : e));
  assert.equal(verify(f).conditional, "");
  f.shards[3].fresh.identity.run_attempt = "1";
  assert.throws(() => verify(f), /run_attempt mismatch/);
});
const negatives = {
  "missing shard": (f) => f.shards.pop(),
  "duplicate shard": (f) => f.shards.push(structuredClone(f.shards[0])),
  "unplanned shard": (f) => { f.shards[1].mode = "shard:g99"; },
  "missing case/false exit zero": (f) => mutateEvents(f, 2, (es) => es.filter((e) => e.Test !== "TestTail")),
  "duplicate case": (f) => mutateEvents(f, 2, (es) => [...es.slice(0, -1), ...rows(P, ["TestTail"]).slice(1, -1), es.at(-1)]),
  "unstarted case": (f) => mutateEvents(f, 2, (es) => es.filter((e) => !(e.Test === "TestTail" && e.Action === "run"))),
  "unclosed case": (f) => mutateEvents(f, 2, (es) => es.filter((e) => !(e.Test === "TestTail" && e.Action === "pass"))),
  "misassigned case": (f) => mutateEvents(f, 2, (es) => es.map((e) => e.Test === "TestTail" ? { ...e, Test: "TestWrongPartition" } : e)),
  "missing log": (f) => { f.shards[1].go = ""; },
  "truncated JSON": (f) => { f.shards[1].go = f.shards[1].go.slice(0, -2); },
  "missing package terminal": (f) => mutateEvents(f, 0, (es) => es.slice(0, -1)),
  "empty package omitted": (f) => mutateEvents(f, 0, (es) => es.filter((e) => e.Package !== EMPTY)),
  "empty package terminal before start": (f) => mutateEvents(f, 0, (es) => [...es.filter((e) => e.Package !== EMPTY), ...es.filter((e) => e.Package === EMPTY).reverse()]),
  "package failure": (f) => mutateEvents(f, 2, (es) => es.map((e) => !e.Test && e.Action === "pass" ? { ...e, Action: "fail" } : e)),
  "build failure": (f) => mutateEvents(f, 2, (es) => [{ Package: P, Action: "build-fail" }, ...es]),
  "race failure": (f) => mutateEvents(f, 2, (es) => [es[0], { Package: P, Action: "output", Output: "WARNING: DATA RACE\n" }, ...es.slice(1)]),
  "subtest failure": (f) => mutateEvents(f, 2, (es) => [es[0], { Package: P, Test: "TestTail/sub", Action: "run" }, { Package: P, Test: "TestTail/sub", Action: "fail" }, ...es.slice(1)]),
  "failed true exit": (f) => { f.shards[2].end.exit = 1; },
  "cancelled official job": (f) => { f.shards[2].official_pass = false; },
  "missing whole vet": (f) => { delete f.shards[0].vet; },
  "duplicate whole vet": (f) => f.shards[0].vet.push(...structuredClone(f.shards[0].vet)),
  "failed vet": (f) => { f.shards[0].vet[1].exit = 2; },
  "interrupted vet": (f) => f.shards[0].vet.pop(),
  "vet in second shard": (f) => { f.shards[1].vet = structuredClone(f.shards[0].vet); },
  "C1 omitted from inventory too": (f) => { f.plan.inventory[P] = f.plan.inventory[P].filter((n) => n !== G07_C1); mutateEvents(f, 1, (es) => es.filter((e) => e.Test !== G07_C1)); },
  "post-run source dirty": (f) => { f.shards[1].end.context.clean = false; },
  "wrong runner pin": (f) => { f.shards[1].end.context.pins["scripts/dev/test-local.sh"] = "0".repeat(64); },
  "wrong C1": (f) => mutateEvents(f, 1, (es) => es.map((e) => e.Test === G07_C1 ? { ...e, Test: "TestC1Model" } : e)),
  "expired whole attempt": (f) => { f.trusted.now = 10000; },
  "overlong deadline": (f) => { f.trusted.deadline = 10001; },
  "matching hashes cannot SKIP": (f) => {
    f.trusted.binding.actual["owner.go"] = f.trusted.binding.recorded["owner.go"];
    f.shards[3].fresh.sources[0].actual = f.trusted.binding.actual["owner.go"];
    editCanonical(f, (p) => { p.sources[0].actual = p.sources[0].recorded; });
  },
  "wrong historical reason": (f) => { f.shards[1].go = f.shards[1].go.replaceAll("source drift:", "waived drift:"); },
  "missing fresh browser proof": (f) => { delete f.shards[3].fresh; },
  "historical skip without passed browser job": (f) => { f.shards[3].official_pass = false; },
  "canonical manifest digest": (f) => editCanonical(f, (p) => { p.files["SOURCE.json"] = "0".repeat(64); }),
  "fresh compiled case stamp": (f) => { f.shards[3].fresh.files = { ...f.shards[3].fresh.files, "cases.json": "0".repeat(64) }; },
  "browser later harness failure": (f) => mutateEvents(f, 3, (es) => es.map((e) => e.Test === "TestBrowserCatalogCore" && e.Action === "pass" ? { ...e, Action: "fail" } : e)),
  "fresh event location mismatch": (f) => { f.shards[3].end.evidence = "wrong-native-suite"; },
  "fresh proof location mismatch": (f) => { f.shards[3].fresh.identity.evidence = "wrong-native-suite"; },
  "fresh proof stale source": (f) => { f.shards[3].fresh.sources[0].actual = "0".repeat(64); },
  "fresh proof wrong ledger": (f) => { f.shards[3].fresh.files = { ...f.shards[3].fresh.files, "product-editor-click-ledger.json": "0".repeat(64) }; },
  "other matrix skip": (f) => mutateEvents(f, 2, (es) => es.map((e) => e.Test === "TestProductEditorMatrixLedger" && e.Action === "pass" ? { ...e, Action: "skip" } : e)),
  "Node skip": (f) => { f.shards[0].node.ledger = tap.replace("# skipped 0", "# skipped 1"); },
  "Node todo": (f) => { f.shards[0].node.execution = tap.replace("# todo 0", "# todo 1"); },
  "Node missing": (f) => { delete f.shards[0].node.controls; },
  "Node counts-only excerpt": (f) => { f.shards[0].node.ledger = tap.replace("ok 1 - actual suite\n", ""); },
  "Node truncated terminal tail": (f) => { f.shards[0].node.ledger = tap.replace("1..1", "1..2"); },
  "Node duplicate terminal": (f) => { f.shards[0].node.ledger = tap.replace("1..1", "ok 1 - duplicate\n1..1"); },
  "Node only warning": (f) => { f.shards[0].node.ledger = tap + "# 'only' requires --test-only\n"; },
  "nested child stdout cannot supply missing C1": (f) => mutateEvents(f, 1, (es) => es.filter((e) => e.Test !== G07_C1).map((e) => e.Test === G07_MATRIX && e.Action === "output" ? { ...e, Output: `--- PASS: ${G07_C1} (0s)\n` + e.Output } : e)),
};
for (const k of ["subject_sha", "workflow_sha", "workflow_ref", "checkout_sha", "checkout_tree", "run_id", "run_attempt"]) negatives[`wrong ${k}`] = (f) => { f.shards[1].start.context.identity[k] = "wrong"; };
for (const k of ["source", "plan_hash"]) negatives[`wrong ${k}`] = (f) => { f.shards[1].end.context[k] = "wrong"; };
for (const k of ["subject_sha", "workflow_sha", "workflow_ref", "checkout_sha", "checkout_tree", "run_id", "run_attempt", "job", "event", "tracked_clean", "mode"]) {
  negatives[`fresh owner missing ${k}`] = (f) => { delete f.shards[3].fresh.identity[k]; };
  negatives[`canonical owner unknown ${k}`] = (f) => editCanonical(f, (p) => { p.identity[k] = "unknown"; });
}
for (const [name, change] of Object.entries(negatives)) test(`canonical rejects ${name}`, () => { const f = fixture(); change(f); assert.throws(() => verify(f)); });

test("canonical proof cannot be borrowed from a complete passing HistoricalPolicy case", () => {
  const f = fixture(), policy = "TestProductEditorMatrixLedgerEvidenceHistoricalPolicy";
  f.plan.inventory[P] = [...f.plan.inventory[P], policy].sort();
  f.trusted.inventory = structuredClone(f.plan.inventory);
  // Catch-all g02 owns this additional compiled case; preserve its one package start/end.
  mutateEvents(f, 2, (es) => [...es.slice(0, -1), ...rows(P, [policy]).slice(1, -1), es.at(-1)]);
  assert.equal(verify(f).conditional, `${P}\t${G07_MATRIX}`);

  let proofLine;
  mutateEvents(f, 1, (es) => es.map((e) => {
    if (e.Test !== G07_MATRIX || e.Action !== "output") return e;
    proofLine = e.Output.match(/^\s*catalog_matrix_ledger_test.go:\d+: MATRIX_OWNER_PROOF \{[^\n]+\}\n/m)?.[0];
    assert.ok(proofLine, "real owning proof-line mutation anchor");
    const output = e.Output.replace(proofLine, "");
    assert.match(output, /canonical evidence is historical \(source drift: owner.go\)/);
    return { ...e, Output: output }; // Keep the original reason and SKIP terminal.
  }));
  mutateEvents(f, 2, (es) => es.flatMap((e) => e.Test === policy && e.Action === "run"
    ? [e, { Package: P, Test: policy, Action: "output", Output: proofLine }] : [e]));
  assert.throws(() => verify(f), /missing\/duplicate owning canonical proof/);
});

test("nested HistoricalPolicy stdout is not terminal/proof, but its own PASS remains real", () => {
  const log = encode(rows(P, ["TestPolicy"], { TestPolicy: { output: `--- FAIL: Child (0s)\n--- SKIP: ${G07_MATRIX} (0s)\nMATRIX_OWNER_PROOF {}\n` } }));
  const result = g07Events(log, { [P]: ["TestPolicy"] });
  assert.equal(result.tests.length, 1); assert.equal(result.tests[0].terminal, "pass");
});

test("compiled list accounts for empty packages and refuses failure/duplicate inventory", () => {
  const events = [{ Package: P, Action: "start" }, { Package: P, Action: "output", Output: "TestNative\n" }, { Package: P, Action: "pass" }, ...rows(EMPTY, [])];
  assert.deepEqual(g07Inventory(encode(events)), { [EMPTY]: [], [P]: ["TestNative"] });
  assert.throws(() => g07Inventory(encode(events), P + "\n"), /package inventory mismatch/);
  assert.throws(() => g07Inventory(encode([...events, events[1]])));
  assert.throws(() => g07Inventory(encode([...events, { Package: P, Action: "fail" }])));
  assert.equal(g07Budget(1000, 10000, 1200, 300), 8500);
});

// Execute the existing judge/catalogue/record/final status code only, without invoking a runner.
// Temp files are synthetic JSON events, not native acceptance evidence.
test("existing strict judge keeps ordinary/subtest SKIP red, unknown NOT_RUN strict-3, accepted external NOT_RUN distinct", () => {
  const script = readFileSync(new URL("../../scripts/dev/release-gate.sh", import.meta.url), "utf8");
  const fn = (name) => script.slice(script.indexOf(`${name}() {`), script.indexOf("\n}\n", script.indexOf(`${name}() {`)) + 3);
  const final = script.slice(script.indexOf("n_open=$((n_notrun - n_accepted))"));
  for (const [name, output, want] of [["TestOrdinary", "", 1], ["TestParent/child", "", 1], ["TestUnknown", "    gate.go:1: NOT_RUN: missing provider\n", 3], ["TestMailPA12SMTPProbe", "    gate.go:1: NOT_RUN: missing provider\n", 0]]) {
    const dir = mkdtempSync(path.join(tmpdir(), "g07-judge-"));
    try {
      writeFileSync(path.join(dir, "raw.jsonl"), encode(rows(P, ["TestPass", name], { TestPass: { output: "    fixture.go:1: NOT_RUN: another test fixture only\n" }, [name]: { terminal: "skip", output } })));
      writeFileSync(path.join(dir, "conditional"), "");
      const body = `set -uo pipefail\nOUT="$1"; n_pass=0; n_fail=0; n_notrun=0; n_accepted=0; rows=""; strict=1; subset=""; sha=model; dirty=0; all_modes=0\n${fn("accepted_notrun")}\n${fn("record")}\n${fn("judge_gotest")}\nread -r p f nr other <<<"$(judge_gotest "$OUT/raw.jsonl" "$OUT/nr.skipped" "$OUT/other" json "$OUT/conditional")"\nif ((other > 0 || f > 0)); then record G07 REAL_PG FAIL skips; else record G07 REAL_PG PASS passed; fi\nif ((nr > 0)); then record G07+ REAL_PG NOT_RUN missing - "$OUT/nr.skipped"; fi\n${final}`;
      const run = spawnSync("bash", ["-c", body, "_", dir], { encoding: "utf8", timeout: 10000 });
      assert.equal(run.status, want, run.stderr + run.stdout);
    } finally { rmSync(dir, { recursive: true, force: true }); }
  }
});

// This source contract guards the root's narrow permission amendment, never grants credentials.
test("canonical official reads are scoped to receipts; no workflow/global write or token fallback", () => {
  const workflow = readFileSync(new URL("../../.github/workflows/gates.yml", import.meta.url), "utf8");
  const top = workflow.split("jobs:\n")[0];
  assert.match(top, /^permissions:\n  contents: read\n/m);
  assert.doesNotMatch(top, /^  actions:/m);
  const jobs = [...workflow.matchAll(/^  ([a-z_]+):\n([\s\S]*?)(?=^  [a-z_]+:\n|$(?![\s\S]))/gm)];
  const receipts = jobs.find((j) => j[1] === "receipts")?.[2] ?? "";
  assert.match(receipts, /^    permissions:\n      contents: read\n      actions: read\n/m);
  for (const [, name, body] of jobs) if (name !== "receipts") assert.doesNotMatch(body, /^      actions:/m);
  assert.doesNotMatch(workflow, /permissions:\s*(?:write-all|read-all)|id-token:\s*write/);
  assert.match(receipts, /GITHUB_TOKEN: \$\{\{ github.token \}\}/);
  assert.equal((workflow.match(/GITHUB_TOKEN:/g) ?? []).length, 1);
  assert.match(receipts, /path: \$\{\{ inputs.canonical_g07 == 'true' && 'output\/g07-ordinary-receipts' \|\| 'receipts' \}\}/);
  assert.match(receipts, /verify output\/g07-ordinary-receipts/);
});


test("actual archive checker rejects digest/CRC/traversal/symlink/duplicate mutations", () => {
  const driver = String.raw`
import tempfile,warnings
warnings.simplefilter('ignore', UserWarning)
def zipped(entries):
 stream=io.BytesIO()
 with zipfile.ZipFile(stream,'w',compression=zipfile.ZIP_STORED) as z:
  for name,body in entries:z.writestr(name,body)
 return stream.getvalue()
def digest(data):return 'sha256:'+hashlib.sha256(data).hexdigest()
valid=zipped([('go.jsonl',b'native-payload')])
link=zipfile.ZipInfo('link'); link.create_system=3; link.external_attr=(stat.S_IFLNK|0o777)<<16
bad=[(valid,'sha256:'+'0'*64),
     (valid.replace(b'native-payload',b'broken-payload'),None),
     (zipped([('../escape',b'x')]),None),
     (zipped([('a//b',b'x')]),None),
     (zipped([(link,b'../escape')]),None),
     (zipped([('same',b'a'),('same',b'b')]),None)]
with tempfile.TemporaryDirectory() as root:
 target=pathlib.Path(root)/'good'; target.mkdir()
 assert g07_unpack(valid,target,digest(valid),1024)==len(b'native-payload')
 assert (target/'go.jsonl').read_bytes()==b'native-payload'
 for i,(data,expected) in enumerate(bad):
  target=pathlib.Path(root)/str(i);target.mkdir()
  try:g07_unpack(data,target,expected or digest(data),1024)
  except (AssertionError,zipfile.BadZipFile):pass
  else:raise AssertionError('corrupt archive accepted '+str(i))
`;
  const run = spawnSync("python3", ["-c", G07_ARCHIVE_PY + driver], { encoding: "utf8", timeout: 10000 });
  assert.equal(run.status, 0, run.stderr + run.stdout);
});
