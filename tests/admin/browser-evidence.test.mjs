// Purpose: prove local browser evidence is isolated and harness roots retain ownership.
// Depends on: Node test/fs/child_process and ../browser-evidence.mjs; no product/browser/PG runtime.
// Used by: test-node; any failed filesystem fixture is retained for review.
import test from "node:test";
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { globSync, mkdirSync, mkdtempSync, readFileSync, realpathSync, rmSync, statSync, writeFileSync } from "node:fs";
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
    const result=spawnSync('bash',['-c','set -euo pipefail; '+source.slice(start,finish)],{env:{...process.env,OUT:out,WT:wt},encoding:'utf8',timeout:10_000});
    assert.equal(result.status,0,result.stderr);
    assert.equal(readFileSync(path.join(wt,'visual-shots','admin','en-390.png'),'utf8'),'synthetic screenshot bytes');
    assert.equal(readFileSync(path.join(wt,'visual-shots','admin','partial.png'),'utf8'),'partial run screenshot');
    assert.equal(readFileSync(path.join(wt,'visual-shots','lint.md'),'utf8'),'lint report');
    assert.equal(readFileSync(path.join(wt,'visual-shots','index-20261009T010203Z.json'),'utf8'),'[]');
    passed = true;
  } finally { if(passed)rmSync(fixture,{recursive:true}); else console.error(`Retained failed reviewer fixture: ${fixture}`); }
});
