import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { createSocket } from 'node:dgram';
import { existsSync } from 'node:fs';
import { chmod, mkdir, mkdtemp, readFile, rm, stat, writeFile } from 'node:fs/promises';
import { createServer } from 'node:net';
import { tmpdir } from 'node:os';
import { join, resolve, sep } from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

const runner = fileURLToPath(new URL('../../scripts/dev/r04-local-input.mjs', import.meta.url));
const pinnedBinary = process.env.COMMERCE_R04_LIVEKIT_BINARY;
const evidenceRoot = fileURLToPath(new URL('../../output/playwright/', import.meta.url));

async function invoke(env, timeoutMs = 120_000) {
  assert.ok(existsSync(runner), 'frozen R04 probe source must be present');
  const child = spawn(process.execPath, [runner], {
    env: { ...process.env, COMMERCE_R04_FAULT: '', ...env },
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  const chunks = { stdout: '', stderr: '' };
  let overflow = false;
  for (const stream of ['stdout', 'stderr']) {
    child[stream].on('data', (chunk) => {
      chunks[stream] += chunk.toString('utf8');
      if (chunks[stream].length > 64_000) {
        overflow = true;
        child.kill('SIGTERM');
      }
    });
  }
  let timedOut = false;
  const timer = setTimeout(() => {
    timedOut = true;
    child.kill('SIGTERM');
  }, timeoutMs);
  try {
    const result = await new Promise((resolve, reject) => {
      child.once('error', reject);
      child.once('close', (code, signal) => resolve({ code, signal }));
    });
    assert.equal(timedOut, false, 'R04 probe exceeded its independent timeout');
    assert.equal(overflow, false, 'R04 probe exceeded bounded output');
    return { ...result, ...chunks };
  } finally {
    clearTimeout(timer);
  }
}

async function runWithEvidence(fault) {
  assert.ok(pinnedBinary, 'set COMMERCE_R04_LIVEKIT_BINARY to the verified native binary');
  await mkdir(evidenceRoot, { recursive: true });
  const dir = await mkdtemp(join(evidenceRoot, 'r04-independent-'));
  const result = await invoke({
    COMMERCE_R04_LIVEKIT_BINARY: pinnedBinary,
    COMMERCE_R04_FAULT: fault,
    COMMERCE_R04_EVIDENCE_DIR: dir,
  });
  const match = result.stdout.match(/^evidence_path=(.+)$/m);
  assert.ok(match, `R04 must print one sanitized evidence path (exit ${result.code})`);
  const file = resolve(match[1]);
  assert.ok(file.startsWith(resolve(dir) + sep), 'evidence must stay in the task-owned directory');
  const raw = await readFile(file, 'utf8');
  const mode = (await stat(file)).mode & 0o777;
  assert.equal(mode & 0o077, 0, 'evidence must not be group/world readable');
  assert.doesNotMatch(raw, /"(?:api_key|secret|jwt|token)"\s*:/i, 'evidence must not contain credential fields');
  return { result, evidence: JSON.parse(raw), file };
}

function assertCleanup(evidence, file) {
  for (const key of ['publisher_context', 'observer_context', 'browser', 'fixture_listener', 'server', 'config']) {
    assert.equal(evidence.cleanup?.[key], true, `${key} cleanup failed; ${file}`);
  }
}

async function canRebindTCP(port) {
  const listener = createServer();
  return new Promise((resolve) => {
    listener.once('error', () => resolve(false));
    listener.listen(port, '127.0.0.1', () => listener.close(() => resolve(true)));
  });
}

async function canRebindUDP(port) {
  const socket = createSocket('udp4');
  return new Promise((resolve) => {
    socket.once('error', () => { socket.close(); resolve(false); });
    socket.bind(port, '127.0.0.1', () => socket.close(() => resolve(true)));
  });
}

async function assertOwnedResourcesGone(evidence, file) {
  const r = evidence.resources;
  assert.ok(r, `safe resource ownership receipt missing; ${file}`);
  assert.ok(Number.isInteger(r.server_pid) && r.server_pid > 0, `owned server PID missing; ${file}`);
  let pidGone = false;
  try { process.kill(r.server_pid, 0); }
  catch (error) { if (error.code === 'ESRCH') pidGone = true; else throw error; }
  assert.equal(pidGone, true, `owned server PID still exists; ${file}`);
  for (const key of ['signal_port', 'rtc_tcp_port', 'fixture_port']) {
    assert.ok(Number.isInteger(r[key]) && r[key] > 0 && r[key] < 65536, `${key} missing; ${file}`);
    assert.equal(await canRebindTCP(r[key]), true, `${key} TCP listener remains open; ${file}`);
  }
  assert.ok(Number.isInteger(r.rtc_udp_port) && r.rtc_udp_port > 0 && r.rtc_udp_port < 65536,
    `rtc_udp_port missing; ${file}`);
  assert.equal(await canRebindUDP(r.rtc_udp_port), true, `RTC UDP listener remains open; ${file}`);
  assert.ok(typeof r.config_dir === 'string' && r.config_dir.startsWith(tmpdir() + sep),
    `private config directory path missing or outside task temp; ${file}`);
  assert.equal(existsSync(r.config_dir), false, `private config directory remains; ${file}`);
}

function assertRemoteMedia(evidence, file) {
  assert.equal(evidence.gates?.RLI02_remote_tracks, true, `remote A/V tracks absent; ${file}`);
  const c = evidence.counters || {};
  assert.ok(c.frames_after > c.frames_before && c.width > 0 && c.height > 0,
    `remote decoded video frames must advance with dimensions; ${file}`);
  assert.ok(c.audio_packets_delta > 0 && c.audio_bytes_delta > 0,
    `remote audio RTP packets and bytes must advance; ${file}`);
  assert.ok(c.audio_energy_delta > 0 || c.audio_rms_energy > 0,
    `remote decoded audio energy must advance; ${file}`);
}

test('R04 rejects a checksum-invalid executable before invoking it', async () => {
  const dir = await mkdtemp(join(tmpdir(), 'r04-invalid-binary-'));
  const fakeBinary = join(dir, 'livekit-server');
  const marker = join(dir, 'invoked');
  try {
    await writeFile(fakeBinary, `#!/usr/bin/env node\nrequire('node:fs').writeFileSync(${JSON.stringify(marker)}, 'invoked');\n`);
    await chmod(fakeBinary, 0o700);
    const result = await invoke({ COMMERCE_R04_LIVEKIT_BINARY: fakeBinary, COMMERCE_R04_EVIDENCE_DIR: dir }, 15_000);
    assert.notEqual(result.code, 0, 'checksum-invalid binary must fail');
    assert.equal(existsSync(marker), false, 'invalid binary must not be executed even for version probing');
  } finally {
    await rm(dir, { recursive: true, force: true });
  }
});

test('R04 rejects an unknown fault before invoking any child process', async () => {
  assert.ok(pinnedBinary, 'set COMMERCE_R04_LIVEKIT_BINARY to the verified native binary');
  const dir = await mkdtemp(join(tmpdir(), 'r04-invalid-fault-'));
  const hook = join(dir, 'watch-children.cjs');
  const marker = join(dir, 'child-invoked');
  try {
    await writeFile(hook, `const cp = require('node:child_process');
const fs = require('node:fs');
for (const name of ['spawn', 'execFile', 'execFileSync', 'spawnSync']) {
  const original = cp[name];
  cp[name] = (...args) => { fs.appendFileSync(${JSON.stringify(marker)}, name + '\\n'); return original(...args); };
}
require('node:module').syncBuiltinESMExports();
`);
    const result = await invoke({
      COMMERCE_R04_LIVEKIT_BINARY: pinnedBinary,
      COMMERCE_R04_FAULT: 'unknown-fault',
      COMMERCE_R04_EVIDENCE_DIR: dir,
      NODE_OPTIONS: `${process.env.NODE_OPTIONS || ''} --require=${hook}`.trim(),
    }, 15_000);
    assert.notEqual(result.code, 0, 'unknown fault must fail');
    assert.equal(existsSync(marker), false, 'fault validation must precede all child-process calls');
    const match = result.stdout.match(/^evidence_path=(.+)$/m);
    assert.ok(match, 'preflight failure must retain a sanitized evidence receipt');
    const evidence = JSON.parse(await readFile(match[1], 'utf8'));
    assert.equal(evidence.failure, 'invalid_fault_value');
  } finally {
    await rm(dir, { recursive: true, force: true });
  }
});

test('R04 delivers decoded remote audio and video through the actual local server', async () => {
  const { result, evidence, file } = await runWithEvidence('');
  assertCleanup(evidence, file);
  await assertOwnedResourcesGone(evidence, file);
  assertRemoteMedia(evidence, file);
  assert.equal(evidence.capture_mode, 'chromium_fake_get_user_media_wav', `capture must use fake getUserMedia; ${file}`);
  assert.equal(result.code, 0, `R04 source exited nonzero; ${file}`);
  assert.equal(evidence.status, 'PASS', `R04 receipt not PASS; ${file}`);
  for (const key of ['RLI01_pins', 'RLI01_loopback_ready', 'RLI03_decoded_media',
    'RLI04_expired_rejected', 'RLI04_tampered_rejected', 'RLI04_observer_publish_denied',
    'RLI05_subscriber_removal', 'RLI05_room_empty']) {
    assert.equal(evidence.gates?.[key], true, `${key} must pass; ${file}`);
  }
  for (const key of ['expired', 'tampered']) {
    assert.deepEqual(evidence.auth?.[key], { reason: 'NotAllowed', status: 401 },
      `${key} must be server authentication rejection; ${file}`);
  }
  assert.deepEqual(evidence.observer_denial, {
    server_can_publish: false, server_track_count: 0, client_error: 'PublishTrackError',
    client_status: 403, denial_layer: 'client_permissions',
  }, `observer publish denial must be classified exactly; ${file}`);
});

test('R04 injected post-publish failure retains media proof and cleans owned resources', async () => {
  const { result, evidence, file } = await runWithEvidence('after-publish');
  assert.notEqual(result.code, 0, `injected fault must exit nonzero; ${file}`);
  assert.equal(evidence.status, 'FAIL', `injected fault must not be PASS; ${file}`);
  assert.equal(evidence.failure, 'injected_after_publish', `fault must occur after publish; ${file}`);
  assertCleanup(evidence, file);
  await assertOwnedResourcesGone(evidence, file);
  assertRemoteMedia(evidence, file);
  assert.equal(evidence.capture_mode, 'chromium_fake_get_user_media_wav', `capture must use fake getUserMedia; ${file}`);
  assert.notEqual(evidence.gates?.RLI05_room_empty, true,
    `fault must not claim an authoritative room-empty query it did not perform; ${file}`);
});

test('R04 injected operation timeout cannot continue acquiring resources after cleanup', async () => {
  const { result, evidence, file } = await runWithEvidence('timeout-after-publish');
  assert.notEqual(result.code, 0, `injected timeout must exit nonzero; ${file}`);
  assert.equal(evidence.status, 'FAIL', `injected timeout must not be PASS; ${file}`);
  assert.equal(evidence.failure, 'injected_operation_timeout', `wrong timeout failure class; ${file}`);
  assertCleanup(evidence, file);
  await assertOwnedResourcesGone(evidence, file);
  await new Promise(resolve => setTimeout(resolve, 500));
  await assertOwnedResourcesGone(evidence, file);
  assertRemoteMedia(evidence, file);
  assert.equal(evidence.capture_mode, 'chromium_fake_get_user_media_wav', `capture must use fake getUserMedia; ${file}`);
  assert.notEqual(evidence.gates?.RLI05_room_empty, true,
    `fault must not claim an authoritative room-empty query it did not perform; ${file}`);
});
