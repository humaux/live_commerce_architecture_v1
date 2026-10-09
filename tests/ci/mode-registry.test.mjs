// Purpose: prove the declarative mode registry drives safe discovery and selected command plans.
// Depends on: the actual Bash runner, frozen mode inventory and Node child_process; no PG or browser runs.
// Used by: test-node.sh CI suite; a future mode must require one registry entry only.
import assert from 'node:assert/strict';
import { execFileSync, spawnSync } from 'node:child_process';
import test from 'node:test';

const runner = 'scripts/dev/test-local.sh';
test('mode discovery has no Docker/build dependency and includes the existing universe', () => {
  const result = spawnSync('/bin/bash', [runner, '--list'], { encoding: 'utf8', env: { PATH: '/usr/bin:/bin' } });
  assert.equal(result.status, 0, result.stderr);
  const names = result.stdout.trim().split('\n');
  assert.equal(names.length, new Set(names).size);
  for (const mode of ['foundation', '--browser-live-console', '--browser-migration-import', '--browser-reports', '--browser-inbox', '--studio-backend']) assert.ok(names.includes(mode), mode);
});
test('dry-run resolves a mode without executing dependencies and keeps its command environment', () => {
  const result = spawnSync('/bin/bash', [runner, '--dry-run', '--browser-live-console'], { encoding: 'utf8', env: { PATH: '/usr/bin:/bin' } });
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /LC_BROWSER_LIVE_CONSOLE_ACCEPTANCE=1 GOTOOLCHAIN=go1\.27\.2 go test/);
  assert.match(result.stdout, /pnpm run build:admin/);
  assert.match(result.stdout, /TestBrowserLiveConsoleRealChain/);
  assert.equal(spawnSync('/bin/bash', [runner, '--dry-run', '--not-a-mode'], { encoding: 'utf8' }).status, 2);
});

test('one appended case is discovered and dispatched; malformed and duplicate registry entries fail closed', async (t) => {
  const { readFileSync, mkdirSync, mkdtempSync, writeFileSync, rmSync } = await import('node:fs');
  const { modeEntries, registryModes } = await import('../../scripts/dev/pr-modes.mjs');
  const source = readFileSync(runner, 'utf8');
  const entry = `  --registry-probe)\n    lc_build=none\n    lc_fixture=none\n    lc_prepare() {\n  :\n    }\n    lc_run() {\n  printf 'registry-probe-executed\\n'\n    }\n    ;;\n`;
  const added = source.replace('  # APPEND MODES HERE', entry + '  # APPEND MODES HERE');
  assert.equal(registryModes(added).length, registryModes(source).length + 1);
  assert.throws(() => modeEntries(added.replace('  # APPEND MODES HERE', entry + '  # APPEND MODES HERE')), /duplicate/);
  assert.throws(() => modeEntries(added.replace('lc_build=none\n    lc_fixture=none', 'lc_build=unrecognised\n    lc_fixture=none')), /invalid/);
  mkdirSync('output/ci-mode-registry', { recursive: true });
  const dir = mkdtempSync('output/ci-mode-registry/one-entry-');
  t.after(() => rmSync(dir, { recursive: true, force: true }));
  mkdirSync(`${dir}/scripts/dev`, { recursive: true }); mkdirSync(`${dir}/bin`);
  writeFileSync(`${dir}/${runner}`, added);
  for (const executable of ['go', 'docker', 'node']) writeFileSync(`${dir}/bin/${executable}`, '#!/bin/sh\nexit 99\n', { mode: 0o755 });
  const { resolve } = await import('node:path');
  const env = { PATH: `${resolve(dir)}/bin:/usr/bin:/bin` };
  const list = execFileSync('/bin/bash', [runner, '--list'], { cwd: dir, encoding: 'utf8', env });
  assert.ok(list.split('\n').includes('--registry-probe'));
  assert.equal(execFileSync('/bin/bash', [runner, '--registry-probe'], { cwd: dir, encoding: 'utf8', env }), 'registry-probe-executed\n');
});

test('registry discovery cannot silently lose indented, aliased or unrecognised Bash case arms', async () => {
  const { readFileSync } = await import('node:fs');
  const { modeEntries, registryModes } = await import('../../scripts/dev/pr-modes.mjs');
  const source = readFileSync(runner, 'utf8');
  const indented = source.replace(/^  --browser-inbox\)/m, '   --browser-inbox)');
  assert.deepEqual(registryModes(indented), registryModes(source));
  for (const arm of ['  --browser-inbox|--alias)', '  unrecognised)', '  --browser-inbox) # trailing comment', '  --browser-inbox) printf inline']) {
    assert.throws(() => modeEntries(source.replace(/^  --browser-inbox\)/m, arm)), /registry|entry|arm/);
  }
});
