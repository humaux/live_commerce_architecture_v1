// Purpose: exercise aggregate timeout admission through the real runner's inert metadata path.
// Depends on: Bash and test-local.sh --dry-run; no Docker, Go, builds or product tests.
// Used by: DB-free candidate validation (explicit Node invocation). Invariant: invalid configuration fails before external work.
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import test from 'node:test';

const runner = 'scripts/dev/test-local.sh';
function invoke(value, args = ['--dry-run']) {
  const env = { PATH: '/usr/bin:/bin' };
  if (value !== undefined) env.LC_FOUNDATION_TIMEOUT_SECONDS = value;
  return spawnSync('/bin/bash', [runner, ...args], { env, encoding: 'utf8', timeout: 5000 });
}

test('unset keeps 4500 seconds; inclusive bounds and interior values use actual admission', () => {
  for (const value of [undefined, '4500', '4501', '6000', '7199', '7200']) {
    const r = invoke(value);
    assert.equal(r.status, 0, r.stderr);
    assert.match(r.stdout, new RegExp(`^# foundation_aggregate_timeout_seconds=${value ?? '4500'}$`, 'm'));
    assert.match(r.stdout, /-p 1 -race -count=1 -timeout="?\$\{LC_FOUNDATION_TIMEOUT_SECONDS\}s"? -v/);
    assert.match(r.stdout, /go vet \.\/\.\.\./);
  }
});

test('malformed, insufficient and unbounded budgets fail before external tool lookup', () => {
  for (const value of ['', '0', '-1', '4499', '7201', '04500', '+4500', '4500s', '4500.0',
    ' 4500', '4500 ', '4500\n', '999999999999999999999999999999', '1e4', '4500;exit 0',
    '$(exit 0)', '`exit 0`']) {
    // No --dry-run: prove real entry rejection happens before even PATH tool lookups.
    const r = spawnSync('/bin/bash', [runner], { encoding: 'utf8', timeout: 5000,
      env: { PATH: '/nonexistent', LC_FOUNDATION_TIMEOUT_SECONDS: value } });
    assert.equal(r.status, 2, `${JSON.stringify(value)}: ${r.stderr}`);
    assert.match(r.stderr, /LC_FOUNDATION_TIMEOUT_SECONDS must be canonical integer seconds in 4500\.\.7200/);
    assert.equal(r.stdout, '');
    assert.doesNotMatch(r.stderr, /command not found/);
  }
});
