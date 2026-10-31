// Purpose: injected-violation (RED) and clean (GREEN) cases for the evidence retention gate, through its real CLI on a temp repo.
// Depends on: scripts/dev/check-evidence-retention.mjs, git, node:test; no network, no application code.
// Used by: scripts/dev/test-node.sh (foundation workflow).
import test from 'node:test';
import assert from 'node:assert/strict';
import {execFileSync, spawnSync} from 'node:child_process';
import {mkdirSync, mkdtempSync, realpathSync, rmSync, writeFileSync} from 'node:fs';
import {tmpdir} from 'node:os';
import path from 'node:path';
import {fileURLToPath} from 'node:url';
import {judge, RAW, MAX_FILE} from '../../scripts/dev/check-evidence-retention.mjs';

const cli = fileURLToPath(new URL('../../scripts/dev/check-evidence-retention.mjs', import.meta.url));

/** Base commit with `baseFiles`, branch commit adding `prFiles` ({path: bytes}); returns the gate's exit code and output. */
function gate(baseFiles, prFiles, args = ['--base', 'base']) {
  const repo = realpathSync(mkdtempSync(path.join(tmpdir(), 'lc-evidence-retention-')));
  try {
    const git = (...a) => execFileSync('git', a, {cwd: repo, encoding: 'utf8'});
    const put = files => { for (const [file, bytes] of Object.entries(files)) { mkdirSync(path.dirname(path.join(repo, file)), {recursive: true}); writeFileSync(path.join(repo, file), Buffer.alloc(bytes, 120)); } };
    git('init', '-q'); git('config', 'user.email', 't@example.invalid'); git('config', 'user.name', 't');
    put({'README.md': 1, ...baseFiles}); git('add', '-A'); git('commit', '-qm', 'base'); git('branch', 'base');
    put(prFiles); git('add', '-A', '-f'); git('commit', '-qm', 'pr', '--allow-empty');
    const r = spawnSync(process.execPath, [cli, ...args], {cwd: repo, encoding: 'utf8', env: {...process.env, GITHUB_BASE_REF: ''}});
    return {status: r.status, out: r.stdout + r.stderr};
  } finally { rmSync(repo, {recursive: true, force: true}); }
}

test('GREEN: a DELIVERY note and a small parity sheet pass', () => {
  const r = gate({}, {'output/unit-x/DELIVERY.md': 4_000, 'output/unit-x/parity/20-443.png': 200_000});
  assert.equal(r.status, 0, r.out); assert.match(r.out, /evidence-retention: PASS \(2 output\/ files/);
});

for (const [name, file] of [
  ['downloaded fix artifact tree', 'output/parity-checkout/fix1-artifacts/gate-browser-e2e/ci-gates/--browser-e2e.log'],
  ['relay artifact tree', 'output/parity-checkout/relay2-artifact/playwright/run.kfwVz1JJ/home-cod/x.log'],
  ['copied Playwright run root', 'output/FEAT-CONTACT-STORE/FIX1-evidence/browser/playwright/run.pZts2ZJb/next.log'],
  ['compressed CI log', 'output/PARITY-LIVE/fix1-relay1/job-filtered.log.gz'],
  ['Playwright trace', 'output/unit-x/results/inbox-ui-INU01/trace.zip'],
]) {
  test(`RED: ${name} is rejected`, () => {
    const r = gate({}, {[file]: 10});
    assert.equal(r.status, 1, r.out); assert.match(r.out, /raw CI artifact copy/);
  });
}

test('RED: a single file over 1 MiB is rejected', () => {
  const r = gate({}, {'output/parity-checkout/fix1-ci-raw.log': MAX_FILE + 1});
  assert.equal(r.status, 1, r.out); assert.match(r.out, /per-file limit/);
});

test('RED: many small files over the 4 MiB per-PR budget are rejected', () => {
  const files = Object.fromEntries(Array.from({length: 5}, (_, i) => [`output/unit-x/figma/frame-${i}.png`, 900_000]));
  const r = gate({}, files);
  assert.equal(r.status, 1, r.out); assert.match(r.out, /per-PR budget/);
});

test('history already on the base is never re-judged', () => {
  const r = gate({'output/old/relay1-artifact/x.log': 2_000_000}, {'output/unit-x/DELIVERY.md': 100});
  assert.equal(r.status, 0, r.out);
});

test('no PR base reports NOT_RUN instead of guessing one', () => {
  const r = gate({}, {'output/unit-x/fix1-artifacts/a.log': 10}, []);
  assert.equal(r.status, 0, r.out); assert.match(r.out, /NOT_RUN/);
});

test('judge keeps signed evidence names and catches raw names (unit)', () => {
  for (const ok of ['output/parity-entry/parity/20-443.png', 'output/ci-select-backend-browser/browser-ownership.json', 'output/parity-checkout/FIX2.md'])
    assert.equal(RAW.test(ok), false, ok);
  assert.deepEqual(judge([{path: 'output/a/FIX1-ARTIFACTS/x.png', size: 1}]).length, 1, 'case-insensitive');
});
