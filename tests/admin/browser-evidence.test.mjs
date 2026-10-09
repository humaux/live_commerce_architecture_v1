// Purpose: prove local browser evidence is isolated and harness roots retain ownership.
// Depends on: Node test/fs/child_process and ../browser-evidence.mjs; no product/browser/PG runtime.
// Used by: test-node; any failed filesystem fixture is retained for review.
import test from "node:test";
import assert from "node:assert/strict";
import { execFileSync, spawnSync } from "node:child_process";
import { existsSync, globSync, symlinkSync, mkdirSync, mkdtempSync, readFileSync, realpathSync, rmSync, statSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";

const helper = new URL("../browser-evidence.mjs", import.meta.url).href;
test("browser evidence uses unique local runs or the selected harness parent without rewriting history", () => {
  const fixture = realpathSync(mkdtempSync(path.join(tmpdir(), "lc-browser-evidence-test-")));
  let passed = false;
  try {
    const historical = path.join(fixture, "historical.json");
    writeFileSync(historical, "immutable historical evidence\n");
    const run = (overrides = {}) => {
      const env = { ...process.env };
      delete env.LC_BROWSER_EVIDENCE;
      delete env.LC_BROWSER_EVIDENCE_ROOT;
      const result = spawnSync(process.execPath, ["--input-type=module", "-e",
        `import { browserEvidenceDirectory } from ${JSON.stringify(helper)}; console.log(JSON.stringify([browserEvidenceDirectory("screens"), browserEvidenceDirectory("screens")]));`],
      { cwd: fixture, env: { ...env, ...overrides }, encoding: "utf8", timeout: 10_000 });
      assert.equal(result.error, undefined);
      assert.equal(result.status, 0, result.stderr);
      return JSON.parse(result.stdout);
    };
    const local = run();
    assert.notEqual(local[0], local[1], "two standalone runs must not overwrite one another");
    for (const directory of local) {
      assert.equal(path.dirname(directory), path.join(fixture, "output/playwright"));
      writeFileSync(path.join(directory, "shot.png"), "synthetic evidence");
    }
    const root = path.join(fixture, "run-root"), suite = path.join(root, "suite");
    assert.deepEqual(run({ LC_BROWSER_EVIDENCE_ROOT: root }), [path.join(root, "screens"), path.join(root, "screens")]);
    assert.deepEqual(run({ LC_BROWSER_EVIDENCE_ROOT: root, LC_BROWSER_EVIDENCE: suite }), [path.join(suite, "screens"), path.join(suite, "screens")]);
    assert.equal(readFileSync(historical, "utf8"), "immutable historical evidence\n");
    passed = true;
  } finally {
    if (passed) rmSync(fixture, { recursive: true });
    else console.error(`Retained failed browser evidence fixture: ${fixture}`);
  }
});


test("browser evidence rejects labels that escape the harness directory before creating files", () => {
  const fixture = realpathSync(mkdtempSync(path.join(tmpdir(), "lc-browser-evidence-label-")));
  let passed = false;
  try {
    const result = spawnSync(process.execPath, ["--input-type=module", "-e",
      `import assert from 'node:assert/strict'; import { browserEvidenceDirectory } from ${JSON.stringify(helper)};
       for (const label of ['..', '../outside', '/tmp/outside', 'a/b', 'a\\b', '', null])
         assert.throws(() => browserEvidenceDirectory(label), /evidence label/, String(label));`],
      { cwd: fixture, env: { ...process.env, LC_BROWSER_EVIDENCE: path.join(fixture, "run") }, encoding: "utf8", timeout: 10_000 });
    assert.equal(result.error, undefined);
    assert.equal(result.status, 0, result.stderr);
    passed = true;
  } finally {
    if (passed) rmSync(fixture, { recursive: true });
    else console.error(`Retained failed label fixture: ${fixture}`);
  }
});


test("shell isolates visual fixtures from a click ledger when caller reuses its evidence root", () => {
  const source = readFileSync(new URL("../../scripts/dev/test-local.sh", import.meta.url), "utf8");
  const start = source.indexOf('# Allocate once before any browser build/log write.');
  const finish = source.indexOf('\nesac', start);
  assert.ok(start >= 0 && finish > start, "real shell allocation entry must exist");
  const allocation = source.slice(start, finish + "\nesac".length);
  const fixture = realpathSync(mkdtempSync(path.join(tmpdir(), "lc-shell-evidence-")));
  let passed = false;
  try {
    const allocate = mode => {
      const result = spawnSync("bash", ["-c", `set -euo pipefail; test_mode="$1"; ${allocation}; printf '%s' "$LC_SWEEP_OUT"`, "evidence-test", mode],
        { cwd: fixture, env: { ...process.env, LC_BROWSER_EVIDENCE_ROOT: fixture }, encoding: "utf8", timeout: 10_000 });
      assert.equal(result.status, 0, result.stderr);
      return result.stdout;
    };
    const click = allocate("--browser-click-sweep"), visual = allocate("--browser-visual-lint");
    assert.notEqual(click, visual, "visual empty journeys must not share click journey evidence");
    const clickFile = path.join(click, "journeys.json");
    mkdirSync(click, { recursive: true }); mkdirSync(visual, { recursive: true });
    writeFileSync(clickFile, '{"orders":7}\n');
    writeFileSync(path.join(visual, "journeys.json"), '{}\n');
    assert.equal(readFileSync(clickFile, "utf8"), '{"orders":7}\n');
    passed = true;
  } finally {
    if (passed) rmSync(fixture, { recursive: true });
    else console.error(`Retained failed shell fixture: ${fixture}`);
  }
});


test("ops disk guard allocation isolates its log and child evidence from committed history", () => {
  const source = readFileSync(new URL("../../scripts/dev/test-local.sh", import.meta.url), "utf8");
  const start = source.indexOf('# Allocate once before any browser build/log write.');
  const finish = source.indexOf('\nesac', start);
  assert.ok(start >= 0 && finish > start);
  const fixture = realpathSync(mkdtempSync(path.join(tmpdir(), "lc-od-evidence-")));
  let passed = false;
  try {
    const env = { ...process.env, LC_BROWSER_EVIDENCE_ROOT: fixture };
    delete env.LC_OD_EVIDENCE;
    const result = spawnSync("bash", ["-c", `set -euo pipefail; test_mode=--ops-disk-guard; ${source.slice(start, finish + "\nesac".length)}; printf '%s' "$LC_OD_EVIDENCE"`],
      { cwd: fixture, env, encoding: "utf8", timeout: 10_000 });
    assert.equal(result.status, 0, result.stderr);
    assert.equal(result.stdout, path.join(fixture, "ops-disk-guard"));
    passed = true;
  } finally {
    if (passed) rmSync(fixture, { recursive: true });
    else console.error(`Retained failed ops fixture: ${fixture}`);
  }
});


test("visual artifact upload selects lint/index/shots and excludes raw unrelated evidence", () => {
  const workflow = readFileSync(new URL("../../.github/workflows/gates.yml", import.meta.url), "utf8");
  const marker = workflow.includes('      - name: Upload visual lint evidence') ? '      - name: Upload visual lint evidence' : '      - uses: actions/upload-artifact';
  const section = workflow.slice(workflow.indexOf(marker, workflow.indexOf('      - name: Failure summary')));
  const match = section.match(/          path: \|\n((?:            .+\n)+)/);
  assert.ok(match, "real visual upload path block must exist");
  const patterns = match[1].trim().split('\n').map(s => s.trim());
  const fixture = realpathSync(mkdtempSync(path.join(tmpdir(), "lc-upload-layout-")));
  let passed = false;
  try {
    const prefix = 'output/playwright/run.local/';
    const audit = prefix + 'ui-visual-audit/20261009T010203Z/';
    const selected = [audit+'lint.json',audit+'lint.md',audit+'index.json',audit+'shots/admin/home/en-390x844.png',prefix+'ci-gates/visual.log'];
    const excluded = [audit+'crops/control.png',prefix+'trace.zip',prefix+'facts.json',prefix+'catalog-core/result.json'];
    for (const name of [...selected, ...excluded]) { const p = path.join(fixture,name); mkdirSync(path.dirname(p),{recursive:true}); writeFileSync(p,name); }
    const files = [...new Set(patterns.flatMap(p => globSync(p.endsWith('/') ? p+'**/*' : p,{cwd:fixture})).filter(p => statSync(path.join(fixture,p)).isFile()))].sort();
    assert.deepEqual(files,selected.sort());
    passed = true;
  } finally { if(passed)rmSync(fixture,{recursive:true}); else console.error(`Retained failed upload fixture: ${fixture}`); }
});

test("visual reviewer discovers nested per-run artifact shots through its real copy loop", () => {
  const source = readFileSync(new URL("../../output/integrator/tools/visual_review.sh", import.meta.url), "utf8");
  const start = source.indexOf('mkdir -p "$WT/visual-shots"');
  const finish = source.indexOf('\nn=$(find ',start);
  assert.ok(start>=0&&finish>start,"real review copy entry must exist");
  const fixture = realpathSync(mkdtempSync(path.join(tmpdir(), "lc-visual-review-layout-")));
  let passed = false;
  try {
    const out=path.join(fixture,'out'),wt=path.join(fixture,'wt');
    const d=path.join(out,'dl','artifact-1','run.local','ui-visual-audit','20261009T010203Z');
    mkdirSync(path.join(d,'shots','admin'),{recursive:true});
    writeFileSync(path.join(d,'shots','admin','en-390.png'),'synthetic screenshot bytes');
    writeFileSync(path.join(d,'lint.md'),'lint report');writeFileSync(path.join(d,'index.json'),'[]');
    const partial=path.join(out,'dl','artifact-2','playwright','run.partial','ui-visual-audit','20261009T010204Z','shots','admin');
    mkdirSync(partial,{recursive:true});writeFileSync(path.join(partial,'partial.png'),'partial run screenshot');
    const same=path.join(out,'dl','artifact-3','run.same','ui-visual-audit','20261009T010203Z');
    mkdirSync(path.join(same,'shots'),{recursive:true});writeFileSync(path.join(same,'index.json'),'[{"page":"other-shard"}]');
    const result=spawnSync('bash',['-c','set -euo pipefail; '+source.slice(start,finish)],{env:{...process.env,OUT:out,WT:wt},encoding:'utf8',timeout:10_000});
    assert.equal(result.status,0,result.stderr);
    assert.equal(readFileSync(path.join(wt,'visual-shots','admin','en-390.png'),'utf8'),'synthetic screenshot bytes');
    assert.equal(readFileSync(path.join(wt,'visual-shots','admin','partial.png'),'utf8'),'partial run screenshot');
    assert.equal(readFileSync(path.join(wt,'visual-shots','lint.md'),'utf8'),'lint report');
    // One merged index keyed by artifact path: shards finishing in the same second must not overwrite each other (PR #23 review).
    const merged=JSON.parse(readFileSync(path.join(wt,'visual-shots','index.json'),'utf8')).shards;
    assert.deepEqual(merged.map(x=>x.shard),['artifact-1/run.local/ui-visual-audit/20261009T010203Z','artifact-3/run.same/ui-visual-audit/20261009T010203Z']);
    assert.deepEqual(merged.map(x=>x.index),[[],[{page:'other-shard'}]]);
    passed = true;
  } finally { if(passed)rmSync(fixture,{recursive:true}); else console.error(`Retained failed reviewer fixture: ${fixture}`); }
});

test("explicit evidence roots inside a repository must be ignored and untracked (PR #23 review)", () => {
  const source = readFileSync(new URL("../../scripts/dev/test-local.sh", import.meta.url), "utf8");
  const start = source.indexOf("      lc_root_existed=0;");
  const finish = source.indexOf("\n      done", start);
  assert.ok(start >= 0 && finish > start, "real explicit-root guard must exist");
  const guard = source.slice(start, finish + "\n      done".length);
  const repo = realpathSync(mkdtempSync(path.join(tmpdir(), "lc-evidence-root-")));
  try {
    const git = (...args) => execFileSync("git", args, { cwd: repo, stdio: "ignore" });
    git("init", "-q"); writeFileSync(path.join(repo, ".gitignore"), "ignored/\n");
    mkdirSync(path.join(repo, "tracked")); writeFileSync(path.join(repo, "tracked", "x"), "x"); git("add", "-A");
    mkdirSync(path.join(repo, "plain"));
    const run = (root) => spawnSync("bash", ["-c", `set -euo pipefail; LC_BROWSER_EVIDENCE_ROOT="$1"; ${guard}; echo ok`, "guard", root],
      { cwd: repo, encoding: "utf8", timeout: 10_000 });
    for (const root of ["tracked", "plain", "."]) assert.equal(run(root).status, 2, `${root} must be refused`);
    assert.equal(run("fresh-refused").status, 2); assert.equal(existsSync(path.join(repo, "fresh-refused")), false, "a refused root it created is removed");
    assert.equal(run("ignored/run.1").status, 0, "an ignored untracked root is accepted");
    symlinkSync(path.join(repo, "tracked"), path.join(repo, "ignored", "run.1", "ui-click-sweep"));
    assert.equal(run("ignored/run.1").status, 2, "a symlinked fixed child inside a reused root is refused");
    assert.equal(run(realpathSync(tmpdir())).status, 0, "a root outside the repository is accepted");
  } finally { rmSync(repo, { recursive: true, force: true }); }
});

test("visual review refuses an incomplete report: every captured page needs exactly one PASS|FIX section (PR #23 review)", () => {
  const dir = realpathSync(mkdtempSync(path.join(tmpdir(), "lc-visual-check-")));
  try {
    const index = path.join(dir, "index.json"), report = path.join(dir, "findings.md");
    writeFileSync(index, JSON.stringify({ shards: [
      // Real tests/ui/visual-audit.mjs shape: page objects under "shots", counts under captured/expected, "missing" list.
      { shard: "a/ui-visual-audit/1", index: { expected: 2, captured: 2, missing: [], shots: [{ app: "admin", id: "orders", locale: "en" }, { app: "admin", id: "orders", locale: "zh-TW" }] } },
      { shard: "b/ui-visual-audit/1", index: { expected: 1, captured: 1, missing: [], shots: [{ app: "storefront", id: "cart", locale: "en" }] } }] }));
    const check = (text) => { writeFileSync(report, text); return spawnSync("python3", [new URL("../../output/integrator/tools/visual_review_check.py", import.meta.url).pathname, index, report], { encoding: "utf8" }).status; };
    assert.equal(check("VERDICT: PASS\n## admin orders: PASS\n## storefront cart: FIX\n- [P2] en-390: x\n"), 0, "complete report accepted");
    assert.notEqual(check("VERDICT: PASS\n"), 0, "verdict-only report refused");
    assert.notEqual(check("VERDICT: PASS\n## admin orders: PASS\n"), 0, "missing page refused");
    assert.notEqual(check("VERDICT: PASS\n## admin orders: PASS\n## admin orders: FIX\n## storefront cart: PASS\n"), 0, "duplicate section refused");
    assert.notEqual(check("## admin orders: PASS\n## storefront cart: PASS\n"), 0, "missing VERDICT refused");
    writeFileSync(index, JSON.stringify({ shards: [{ shard: "c", index: { expected: 2, captured: 1, missing: ["storefront/cart/en"], shots: [{ app: "admin", id: "orders" }] } }] }));
    assert.notEqual(check("VERDICT: PASS\n## admin orders: PASS\n"), 0, "a shard with missing shots is refused");
  } finally { rmSync(dir, { recursive: true, force: true }); }
});
