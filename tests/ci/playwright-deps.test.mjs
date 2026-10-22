// Purpose: pin registry-derived browser coverage and bounded dependency installation.
// Depends on: actual planner/installer/workflow, Node process/fs and fake external CLI edges.
// Used by: test-node.sh; no apt, sudo, browser download or product fixture is executed.
import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, readdirSync, mkdtempSync, mkdirSync, writeFileSync, rmSync } from 'node:fs';
import { spawnSync, execFileSync } from 'node:child_process';
import path from 'node:path';
import { modeEntries } from '../../scripts/dev/pr-modes.mjs';
import { browserRequirements, assertWebKitCoverage } from '../../scripts/dev/ci-playwright-browsers.mjs';
const registry = readFileSync('scripts/dev/test-local.sh', 'utf8');
const goSources = readdirSync('tests/foundation').filter(n => n.endsWith('_test.go')).map(n => readFileSync(`tests/foundation/${n}`, 'utf8'));

test('ordinary and sharded browser jobs require only Chromium; default mixed modes keep WebKit', () => {
  for (const mode of ['--browser-returns-ui', '--browser-store-domains', '--browser-live-console', '--browser-visual-lint@4/4'])
    assert.deepEqual(browserRequirements(mode, registry), ['chromium']);
  for (const mode of ['--browser-webkit', '--browser-cvs'])
    assert.deepEqual(browserRequirements(mode, registry), ['chromium', 'webkit']);
  assert.deepEqual(browserRequirements('shard:unit', registry), []);
});
test('explicit WebKit calibration takes the union without dropping Chromium', () => {
  assert.deepEqual(browserRequirements('--browser-order', registry, 'LC_BROWSER_ENGINE=webkit'), ['chromium', 'webkit']);
  assert.deepEqual(browserRequirements('--browser-cvs', registry, 'LC_BROWSER_ENGINE=chromium'), ['chromium', 'webkit']);
  assert.throws(() => browserRequirements('--browser-order', registry, 'LC_BROWSER_ENGINE=firefox'), /engine|browser/i);
  assert.throws(() => browserRequirements('--browser-order', registry, 'PATH=bad'), /extra_env/i);
});
test('every current browser arm declares requirements; unknown or missing declarations fail closed', () => {
  for (const entry of modeEntries(registry).filter(e => /^--(?:browser|stripe-browser)/.test(e.name)))
    assert.ok(browserRequirements(entry.name, registry).includes('chromium'), entry.name);
  assert.throws(() => browserRequirements('--browser-unknown', registry), /mode|registry/i);
  const missing = registry.replace(/^[ \t]*lc_browsers=.*\n/gm, '');
  assert.throws(() => browserRequirements('--browser-order', missing), /declar|browser/i);
});
test('coverage assertion derives mixed Go roots and catches a new mode reusing them', () => {
  assertWebKitCoverage(registry, goSources);
  const mixed = `  --browser-new-mixed)\n    lc_build=storefront\n    lc_fixture=pg\n    lc_browsers=chromium\n    lc_prepare() {\n  :\n    }\n    lc_run() {\n  go test -tags browser -run '^TestBrowserTaiwanCvs$' ./tests/foundation\n    }\n    ;;\n`;
  assert.throws(() => assertWebKitCoverage(registry.replace('  # APPEND MODES HERE', () => mixed + '  # APPEND MODES HERE'), goSources), /webkit/i);
});

test('TH-7: a variable-driven LC_BROWSER_ENGINE launch must declare itself, so it cannot slip past the WebKit coverage scan', () => {
  const driven = 'func TestEngineLoop(t *testing.T) {\n\tfor _, engine := range []string{"chromium", "webkit"} {\n\t\tt.Setenv("LC_BROWSER_ENGINE", engine)\n\t}\n}\n';
  assert.throws(() => assertWebKitCoverage(registry, [...goSources, driven]), /marker|variable/i);
  assertWebKitCoverage(registry, [...goSources, '// lc:webkit=front-only\n' + driven]); // TLS front without a browser binary: declared, accepted
  const browser = '// lc:webkit=browser\n' + driven.replace('TestEngineLoop', 'TestEngineBrowser');
  const mode = `  --browser-new-driven)\n    lc_build=storefront\n    lc_fixture=pg\n    lc_browsers=chromium\n    lc_prepare() {\n  :\n    }\n    lc_run() {\n  go test -tags browser -run '^TestEngineBrowser$' ./tests/foundation\n    }\n    ;;\n`;
  assert.throws(() => assertWebKitCoverage(registry.replace('  # APPEND MODES HERE', () => mode + '  # APPEND MODES HERE'), [...goSources, browser]), /webkit/i);
});

function installer({ failures = 0, browsers = ['chromium'], first = 'missing', after = 'none', debs = false, offlineFails = false } = {}) {
  mkdirSync('output/playwright', { recursive: true }); // untracked: absent on a fresh checkout (PR #27 review)
  const dir = mkdtempSync('output/playwright/ci-playwright-deps-test.');
  const bin = path.resolve(dir, 'bin'), trace = path.resolve(dir, 'trace.jsonl'), debDir = path.resolve(dir, 'debs'); mkdirSync(bin);
  if (debs) { mkdirSync(debDir); writeFileSync(path.join(debDir, 'libfoo_1_amd64.deb'), 'x'); }
  const fake = `const fs=require('fs'); const tool=__TOOL__; const args=process.argv.slice(2); const e=process.env; fs.appendFileSync(e.EDGE_TRACE,JSON.stringify({tool,args})+'\\n');
const count=(k)=>{const p=e.EDGE_TRACE+'.'+k;const n=(fs.existsSync(p)?Number(fs.readFileSync(p)):0)+1;fs.writeFileSync(p,String(n));return n};
if(tool==='uname'){console.log('Linux');process.exit(0)}
if(tool==='sudo'){const r=require('child_process').spawnSync(args[0],args.slice(1),{stdio:'inherit'});process.exit(r.status??1)}
if(tool==='timeout'){const i=args.findIndex(x=>/^\\d+s$/.test(x));const r=require('child_process').spawnSync(args[i+1],args.slice(i+2),{stdio:'inherit'});process.exit(r.status??1)}
if(tool==='sleep')process.exit(0);
if(tool==='apt-get'){
  if(args.includes('update'))process.exit(0);
  if(args.includes('--no-download'))process.exit(e.OFFLINE_FAILS==='1'?100:0);
  process.exit(count('apt')<=Number(e.EDGE_FAILURES)?17:0)}
if(tool==='node'&&args[0]==='-p'){console.log(e.FAKE_CLI);process.exit(0)}
if(tool==='node'&&args.includes('--dry-run')){const mode=count('dry')===1?e.DRY_FIRST:e.DRY_AFTER;
  if(mode==='none'){console.log('All system dependencies are installed.');process.exit(0)}
  if(mode==='missing'){console.log('Missing system dependencies (2):\\n  libfoo\\n  libbar');process.exit(1)}
  console.log('boom');process.exit(1)}
if(tool==='node'){process.exit(count('cli')<=Number(e.EDGE_FAILURES)?17:0)}
`;
  for (const name of ['uname', 'sudo', 'timeout', 'sleep', 'node', 'pnpm', 'apt-get']) {
    const own = path.join(bin, name + '.cjs'); writeFileSync(own, fake.replace('__TOOL__', JSON.stringify(name)));
    writeFileSync(path.join(bin, name), `#!/bin/sh\nexec '${process.execPath}' '${own}' "$@"\n`, { mode: 0o755 });
  }
  const result = spawnSync('bash', ['scripts/dev/ci-playwright-install.sh', ...browsers], { encoding: 'utf8', timeout: 10000, env: { ...process.env, PATH: bin + ':' + process.env.PATH, EDGE_TRACE: trace, EDGE_FAILURES: String(failures), DRY_FIRST: first, DRY_AFTER: after, OFFLINE_FAILS: offlineFails ? '1' : '0', LC_PLAYWRIGHT_DEBS_DIR: debDir, GITHUB_OUTPUT: path.resolve(dir, 'out'), FAKE_CLI: '/fixture/playwright-cli.js' } });
  const events = readFileSync(trace, 'utf8').trim().split('\n').map(JSON.parse);
  let out = ''; try { out = readFileSync(path.resolve(dir, 'out'), 'utf8'); } catch {}
  rmSync(dir, { recursive: true }); return { result, events, out };
}
const dryRuns = events => events.filter(x => x.tool === 'node' && x.args.includes('--dry-run'));
const aptCalls = events => events.filter(x => x.tool === 'apt-get');
test('#162: a runner image that already has every dependency makes no apt call at all', () => {
  const { result, events } = installer({ first: 'none' }); assert.equal(result.status, 0, result.stderr);
  assert.equal(aptCalls(events).length + events.filter(x => ['sudo', 'timeout', 'sleep'].includes(x.tool)).length, 0);
  assert.equal(dryRuns(events).length, 1);
  assert.deepEqual(events.filter(x => x.tool === 'pnpm').map(x => x.args), [['exec', 'playwright', 'install', 'chromium']]);
});
test('#162 cache HIT: missing deps install offline from the cached .debs, with no index refresh, download, timeout or retry', () => {
  const { result, events, out } = installer({ debs: true }); assert.equal(result.status, 0, result.stderr);
  const apt = aptCalls(events); assert.equal(apt.length, 1);
  assert.ok(apt[0].args.includes('--no-download')); assert.ok(apt[0].args.some(a => a.startsWith('Dir::Cache::archives=')));
  assert.deepEqual(apt[0].args.slice(-2), ['libfoo', 'libbar']);
  assert.equal(events.filter(x => ['timeout', 'sleep'].includes(x.tool)).length, 0);
  assert.equal(dryRuns(events).length, 2, 'the dependencies are verified after the install');
  assert.ok(!out.includes('debs=downloaded'), 'a hit must not re-save the cache');
  assert.equal(events.filter(x => x.tool === 'pnpm').length, 1);
});
test('#162 cache MISS: only the missing packages are downloaded into the cache dir, then reported for saving; transient failure retries at most three times under root timeouts', () => {
  const { result, events, out } = installer({ failures: 2 }); assert.equal(result.status, 0, result.stderr);
  const timed = events.filter(x => x.tool === 'timeout'); assert.equal(timed.length, 6, 'update + install per attempt');
  for (const call of timed) { assert.ok(call.args.includes('--kill-after=15s')); assert.ok(call.args.includes('240s')); assert.ok(!call.args.includes('--foreground')); }
  const installs = aptCalls(events).filter(x => x.args.includes('install'));
  assert.equal(installs.length, 3);
  for (const call of installs) { assert.ok(!call.args.includes('--no-download')); assert.ok(call.args.some(a => a.startsWith('Dir::Cache::archives='))); assert.deepEqual(call.args.slice(-2), ['libfoo', 'libbar']); }
  assert.deepEqual(events.filter(x => x.tool === 'sleep').map(x => x.args), [['10'], ['10']]);
  assert.ok(out.includes('debs=downloaded'));
  assert.deepEqual(events.filter(x => x.tool === 'pnpm').map(x => x.args), [['exec', 'playwright', 'install', 'chromium']]);
});
test('#162: a cached set that cannot install offline falls back to the network instead of failing or skipping', () => {
  const { result, events } = installer({ debs: true, offlineFails: true }); assert.equal(result.status, 0, result.stderr);
  assert.ok(aptCalls(events).some(x => x.args.includes('--no-download')));
  assert.ok(aptCalls(events).some(x => x.args.includes('install') && !x.args.includes('--no-download')));
});
test('persistent dependency failure stops after three attempts and never downloads browsers', () => {
  const { result, events } = installer({ failures: 100 }); assert.notEqual(result.status, 0);
  assert.equal(aptCalls(events).filter(x => x.args.includes('install')).length, 3); assert.equal(events.filter(x => x.tool === 'pnpm').length, 0);
});
test('#162 RED path: dependencies still missing after an install that claimed success keep the job red', () => {
  const { result, events } = installer({ after: 'missing' }); assert.notEqual(result.status, 0);
  assert.match(result.stderr, /still missing/); assert.equal(events.filter(x => x.tool === 'pnpm').length, 0);
});
test('an unlistable delta (simulation failed) installs the full Playwright set as before, then verifies', () => {
  const { result, events } = installer({ first: 'unknown', failures: 2 }); assert.equal(result.status, 0, result.stderr);
  assert.equal(events.filter(x => x.tool === 'timeout').length, 3); assert.equal(aptCalls(events).length, 0);
  assert.deepEqual(events.filter(x => x.tool === 'node' && x.args.includes('install-deps') && !x.args.includes('--dry-run')).map(x => x.args.slice(1)), [['install-deps', 'chromium'], ['install-deps', 'chromium'], ['install-deps', 'chromium']]);
  assert.equal(dryRuns(events).length, 2);
});
test('mixed engine installer preserves both dependencies and both binaries', () => {
  const { result, events } = installer({ browsers: ['chromium', 'webkit'] }); assert.equal(result.status, 0, result.stderr);
  assert.ok(dryRuns(events).every(x => x.args.slice(1).join(' ') === 'install-deps --dry-run chromium webkit'));
  assert.deepEqual(events.filter(x => x.tool === 'pnpm').map(x => x.args), [['exec', 'playwright', 'install', 'chromium', 'webkit']]);
});
test('invalid browser set cannot invoke dependency installer or browser downloads', () => {
  const { result, events } = installer({ browsers: ['firefox'] }); assert.equal(result.status, 2);
  assert.equal(events.filter(x => ['sudo', 'timeout', 'pnpm', 'apt-get'].includes(x.tool) || (x.tool === 'node' && x.args[0] !== '-p')).length, 0);
});
test('dependency CLI resolves from the directly declared package in the frozen pnpm installation', () => {
  const cli = execFileSync(process.execPath, ['-p', 'require.resolve("@playwright/test/cli")'], { encoding: 'utf8' }).trim();
  assert.ok(readFileSync(cli, 'utf8').includes('playwright/lib/program'));
  const { events } = installer({ first: 'none' }); assert.ok(events.find(x => x.tool === 'node' && x.args[0] === '-p').args[1].includes('@playwright/test/cli'));
});
test('workflow keeps the fifteen-minute bound, partitions both caches by image, Playwright version and engine set, and saves .debs only when downloaded', () => {
  const workflow = readFileSync('.github/workflows/gates.yml', 'utf8');
  assert.match(workflow, /timeout-minutes: 15/);
  assert.match(workflow, /node scripts\/dev\/ci-playwright-browsers\.mjs/);
  assert.match(workflow, /bash scripts\/dev\/ci-playwright-install\.sh/);
  assert.ok(!workflow.includes('install --with-deps chromium webkit'));
  assert.match(workflow, /steps\.browsers\.outputs\.cache/);
  const key = /key: (playwright-debs-[^\n]+)/.exec(workflow)?.[1] ?? '';
  for (const part of ['runner.os', 'runner.arch', 'steps.pw.outputs.image', 'steps.pw.outputs.version', 'steps.browsers.outputs.cache']) assert.ok(key.includes(part), `.deb cache key lacks ${part}`);
  assert.match(workflow, /steps\.pwinstall\.outputs\.debs == 'downloaded'/);
  assert.match(workflow, /LC_PLAYWRIGHT_DEBS_DIR/);
});
