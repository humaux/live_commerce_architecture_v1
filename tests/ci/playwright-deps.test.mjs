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

function installer(failures, browsers = ['chromium']) {
  mkdirSync('output/playwright', { recursive: true }); // untracked: absent on a fresh checkout (PR #27 review)
  const dir = mkdtempSync('output/playwright/ci-playwright-deps-test.');
  const bin = path.resolve(dir, 'bin'), trace = path.resolve(dir, 'trace.jsonl'); mkdirSync(bin);
  const fake = path.resolve(dir, 'edge.cjs');
  writeFileSync(fake, `const fs=require('fs'); const tool=require('path').basename(process.argv[1]); const args=process.argv.slice(2); fs.appendFileSync(process.env.EDGE_TRACE,JSON.stringify({tool,args})+'\\n');
if(tool==='uname'){console.log('Linux');process.exit(0)}
if(tool==='sudo'){const r=require('child_process').spawnSync(args[0],args.slice(1),{stdio:'inherit'});process.exit(r.status??1)}
if(tool==='timeout'){const i=args.findIndex(x=>/^\\d+s$/.test(x));const r=require('child_process').spawnSync(args[i+1],args.slice(i+2),{stdio:'inherit'});process.exit(r.status??1)}
if(tool==='sleep')process.exit(0);
if(tool==='node'&&args[0]==='-p'){console.log(process.env.FAKE_CLI);process.exit(0)}
if(tool==='node'){const p=process.env.EDGE_TRACE+'.attempt';let n=fs.existsSync(p)?Number(fs.readFileSync(p)):0;fs.writeFileSync(p,String(++n));process.exit(n<=Number(process.env.EDGE_FAILURES)?17:0)}
`);
  for (const name of ['uname','sudo','timeout','sleep','node','pnpm']) {
    const wrapper = path.join(bin, name); writeFileSync(wrapper, `#!/bin/sh\nexec '${process.execPath}' '${fake}' "$@"\n`, {mode:0o755});
    // Each edge identifies itself through argv[1].
    const body=readFileSync(fake,'utf8').replace("const tool=require('path').basename(process.argv[1]);", `const tool=${JSON.stringify(name)};`);
    const own=path.join(bin,name+'.cjs');writeFileSync(own,body);writeFileSync(wrapper,`#!/bin/sh\nexec '${process.execPath}' '${own}' "$@"\n`,{mode:0o755});
  }
  const result=spawnSync('bash',['scripts/dev/ci-playwright-install.sh',...browsers], {encoding:'utf8',timeout:10000,env:{...process.env,PATH:bin+':'+process.env.PATH,EDGE_TRACE:trace,EDGE_FAILURES:String(failures),FAKE_CLI:'/fixture/playwright-cli.js'}});
  const events=readFileSync(trace,'utf8').trim().split('\n').map(JSON.parse);
  rmSync(dir,{recursive:true}); return {result,events};
}
test('installer retries transient dependency failure at most three times under root timeouts', () => {
  const {result,events}=installer(2);assert.equal(result.status,0,result.stderr);
  const timed=events.filter(x=>x.tool==='timeout');assert.equal(timed.length,3);
  for(const call of timed){assert.ok(call.args.includes('--kill-after=15s'));assert.ok(call.args.includes('240s'));assert.ok(!call.args.includes('--foreground'));}
  assert.equal(events.filter(x=>x.tool==='sudo').length,3);
  assert.deepEqual(events.filter(x=>x.tool==='sleep').map(x=>x.args),[['10'],['10']]);
  const downloads=events.filter(x=>x.tool==='pnpm');assert.deepEqual(downloads.map(x=>x.args),[['exec','playwright','install','chromium']]);
});
test('persistent dependency failure stops after three attempts and never downloads browsers', () => {
  const {result,events}=installer(100);assert.notEqual(result.status,0);assert.equal(events.filter(x=>x.tool==='timeout').length,3);assert.equal(events.filter(x=>x.tool==='pnpm').length,0);
});
test('mixed engine installer preserves both dependencies and both binaries; first success needs no retry', () => {
  const {result,events}=installer(0,['chromium','webkit']);assert.equal(result.status,0,result.stderr);
  assert.equal(events.filter(x=>x.tool==='timeout').length,1);assert.equal(events.filter(x=>x.tool==='sleep').length,0);
  assert.deepEqual(events.filter(x=>x.tool==='node'&&x.args[0]!=='-p').map(x=>x.args.slice(1)),[['install-deps','chromium','webkit']]);
  assert.deepEqual(events.filter(x=>x.tool==='pnpm').map(x=>x.args),[['exec','playwright','install','chromium','webkit']]);
});
test('invalid browser set cannot invoke dependency installer or browser downloads', () => {
  const {result,events}=installer(0,['firefox']);assert.equal(result.status,2);
  assert.equal(events.filter(x=>['sudo','timeout','node','pnpm'].includes(x.tool)).length,0);
});
test('dependency CLI resolves from the directly declared package in the frozen pnpm installation', () => {
  const cli=execFileSync(process.execPath,['-p','require.resolve("@playwright/test/cli")'],{encoding:'utf8'}).trim();
  assert.ok(readFileSync(cli,'utf8').includes('playwright/lib/program'));
  const {events}=installer(0);assert.ok(events.find(x=>x.tool==='node'&&x.args[0]==='-p').args[1].includes('@playwright/test/cli'));
});
test('workflow keeps the overall fifteen-minute bound and partitions cache by planned engine set', () => {
  const workflow=readFileSync('.github/workflows/gates.yml','utf8');
  assert.match(workflow,/timeout-minutes: 15/);
  assert.match(workflow,/node scripts\/dev\/ci-playwright-browsers\.mjs/);
  assert.match(workflow,/bash scripts\/dev\/ci-playwright-install\.sh/);
  assert.ok(!workflow.includes('install --with-deps chromium webkit'));
  assert.match(workflow,/steps\.browsers\.outputs\.cache/);
});
