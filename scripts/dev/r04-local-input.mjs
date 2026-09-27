#!/usr/bin/env node
// Test-only LOCAL_REAL transport probe. Never reuse these fixture grants for product admission.
import { createHmac, randomBytes, createHash } from 'node:crypto';
import { createServer as createHTTPServer } from 'node:http';
import { createServer as createTCPServer } from 'node:net';
import { createSocket } from 'node:dgram';
import { spawn, execFileSync } from 'node:child_process';
import { mkdtemp, readFile, writeFile, mkdir, unlink, rmdir } from 'node:fs/promises';
import { tmpdir, platform, arch } from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { chromium } from '@playwright/test';

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
const BINARY_SHA = '2b06c267be38bae34e2314ea648826c39220f93bd9ed25286d6fc17585e28f91';
const SDK_VERSION = '2.22.3';
const MAX_MS = 75_000;
const fault = process.env.COMMERCE_R04_FAULT || '';
const binary = process.env.COMMERCE_R04_LIVEKIT_BINARY;
const evidenceDir = process.env.COMMERCE_R04_EVIDENCE_DIR || path.join(ROOT, 'output', 'playwright');
const evidence = {
  probe: 'R04_LOCAL_REAL_WEBRTC', server_version: '1.13.7', sdk_version: SDK_VERSION,
  status: 'FAIL', gates: {}, counters: {}, cleanup: {}, failure: null,
  resources: { server_pid: null, signal_port: null, rtc_tcp_port: null, rtc_udp_port: null, fixture_port: null, config_dir: null },
};
let server, httpServer, browser, publisherContext, observerContext, configDir, configPath, wavPath;
const started = Date.now();
const deadline = async (label, promise, ms = 12_000) => {
  let timer;
  try { return await Promise.race([promise, new Promise((_, reject) => { timer = setTimeout(() => reject(new Error(`${label}_timeout`)), ms); })]); }
  finally { clearTimeout(timer); }
};
const remaining = (ms = 12_000) => Math.max(1, Math.min(ms, MAX_MS - (Date.now() - started)));
const budgeted = (label, promise, ms) => deadline(label, promise, remaining(ms));
const pause = ms => new Promise(resolve => setTimeout(resolve, ms));
const assert = (condition, label) => { if (!condition) throw new Error(label); };
const assertBudget = () => assert(Date.now() - started < MAX_MS, 'runtime_budget_exceeded');
const b64 = value => Buffer.from(JSON.stringify(value)).toString('base64url');
const sign = (key, secret, payload) => {
  const body = `${b64({ alg: 'HS256', typ: 'JWT' })}.${b64(payload)}`;
  return `${body}.${createHmac('sha256', secret).update(body).digest('base64url')}`;
};
function grant(room, publish, subscribe) {
  return { room, roomJoin: true, canPublish: publish, canPublishSources: publish ? ['camera', 'microphone'] : [],
    canSubscribe: subscribe, canPublishData: false, canUpdateOwnMetadata: false,
    roomAdmin: false, roomCreate: false, roomList: false, roomRecord: false, ingressAdmin: false };
}
const token = (key, secret, identity, video, expiry = 90, issuedAgo = 0) => {
  const now = Math.floor(Date.now() / 1000);
  return sign(key, secret, { iss: key, sub: identity, iat: now - issuedAgo,
    nbf: now - issuedAgo - 1, exp: now + expiry, video });
};
function fakeMicrophoneWAV() {
  const samples = 48_000;
  const wav = Buffer.alloc(44 + samples * 2);
  wav.write('RIFF', 0); wav.writeUInt32LE(wav.length - 8, 4); wav.write('WAVEfmt ', 8);
  wav.writeUInt32LE(16, 16); wav.writeUInt16LE(1, 20); wav.writeUInt16LE(1, 22);
  wav.writeUInt32LE(samples, 24); wav.writeUInt32LE(samples * 2, 28);
  wav.writeUInt16LE(2, 32); wav.writeUInt16LE(16, 34);
  wav.write('data', 36); wav.writeUInt32LE(samples * 2, 40);
  for (let i = 0; i < samples; i++) wav.writeInt16LE(Math.round(Math.sin(i * 2 * Math.PI * 440 / samples) * 16_000), 44 + i * 2);
  return wav;
}
async function tcpPort() {
  const listener = createTCPServer();
  await new Promise((resolve, reject) => listener.once('error', reject).listen(0, '127.0.0.1', resolve));
  const port = listener.address().port;
  await new Promise(resolve => listener.close(resolve));
  return port;
}
async function udpPort() {
  const socket = createSocket('udp4');
  await new Promise((resolve, reject) => socket.once('error', reject).bind(0, '127.0.0.1', resolve));
  const port = socket.address().port;
  await new Promise(resolve => socket.close(resolve));
  return port;
}
async function waitReady(url) {
  for (let n = 0; n < 100; n++) {
    assertBudget();
    if (server.exitCode !== null) throw new Error('server_exited_before_ready');
    try { const r = await fetch(`${url}/`, { signal: AbortSignal.timeout(remaining(500)) }); if (r.status < 500) return; } catch {}
    await pause(100);
  }
  throw new Error('server_not_ready');
}
const pageHTML = `<!doctype html><meta charset="utf-8"><title>R04 synthetic media fixture</title><video id="remote" autoplay playsinline muted></video><script src="/sdk.js"></script>`;
async function pageSetup(page) {
  await page.goto(page.fixtureURL, { timeout: remaining(10_000) });
  await page.waitForFunction(() => !!window.LivekitClient, null, { timeout: remaining(10_000) });
}
async function connect(page, wsURL, jwt, role) {
  return budgeted(`${role}_connect`, page.evaluate(async ({ wsURL, jwt, role }) => {
    const lk = window.LivekitClient;
    const room = new lk.Room({ adaptiveStream: false, dynacast: false, autoSubscribe: role === 'observer' });
    window.probeRoom = room;
    window.probeTracks = {};
    window.probeRemoved = 0;
    room.on(lk.RoomEvent.TrackSubscribed, (track, publication, participant) => {
      if (role === 'observer' && participant.identity !== window.expectedPublisher) return;
      window.probeTracks[track.kind] = track;
      if (track.kind === 'video') track.attach(document.querySelector('#remote'));
      if (track.kind === 'audio') {
        window.probeAudioElement = track.attach();
        document.body.appendChild(window.probeAudioElement);
      }
    });
    room.on(lk.RoomEvent.TrackUnsubscribed, track => {
      if (window.probeTracks[track.kind]) delete window.probeTracks[track.kind];
      window.probeRemoved++;
    });
    await room.connect(wsURL, jwt, { autoSubscribe: role === 'observer' });
    return room.state;
  }, { wsURL, jwt, role }), 15_000);
}
async function waitTracks(page, publisherID) {
  await budgeted('set_expected_publisher', page.evaluate(id => { window.expectedPublisher = id; }, publisherID));
  await page.waitForFunction(() => !!window.probeTracks?.video && !!window.probeTracks?.audio, null, { timeout: remaining(18_000) });
}
async function startPlayback(page) {
  return page.evaluate(async () => {
    const element = window.probeAudioElement;
    let startAudio = 'resolved', play = 'resolved';
    try { await window.probeRoom.startAudio(); } catch { startAudio = 'rejected'; }
    try { await element.play(); } catch { play = 'rejected'; }
    return { attached: !!element, start_audio: startAudio, play,
      paused: element.paused, muted: element.muted, ready_state: element.readyState,
      can_playback_audio: window.probeRoom.canPlaybackAudio };
  });
}
async function sample(page) {
  return page.evaluate(async () => {
    const video = document.querySelector('#remote');
    const audio = window.probeTracks.audio;
    const audioContext = new AudioContext();
    await audioContext.resume();
    const source = audioContext.createMediaStreamSource(new MediaStream([audio.mediaStreamTrack]));
    const analyser = audioContext.createAnalyser();
    source.connect(analyser);
    const waveform = new Float32Array(analyser.fftSize);
    let peakEnergy = 0;
    const audioContextState = audioContext.state;
    const audioContextTimeBefore = audioContext.currentTime;
    const remoteTrackState = { ready_state: audio.mediaStreamTrack.readyState,
      muted: audio.mediaStreamTrack.muted, enabled: audio.mediaStreamTrack.enabled };
    const stats = async () => {
      const report = await audio.getRTCStatsReport();
      let packets = 0, bytes = 0, energy = 0;
      report?.forEach(row => {
        if (row.type === 'inbound-rtp' && row.kind === 'audio') {
          packets += row.packetsReceived || 0;
          bytes += row.bytesReceived || 0;
          energy += row.totalAudioEnergy || 0;
        }
      });
      return { packets, bytes, energy };
    };
    const before = await stats();
    const framesBefore = video.getVideoPlaybackQuality().totalVideoFrames;
    for (let i = 0; i < 25; i++) {
      analyser.getFloatTimeDomainData(waveform);
      let energy = 0;
      for (const value of waveform) energy += value * value;
      peakEnergy = Math.max(peakEnergy, energy / waveform.length);
      await new Promise(resolve => setTimeout(resolve, 100));
    }
    const after = await stats();
    const audioContextTimeDelta = audioContext.currentTime - audioContextTimeBefore;
    await audioContext.close();
    return { frames_before: framesBefore, frames_after: video.getVideoPlaybackQuality().totalVideoFrames,
      width: video.videoWidth, height: video.videoHeight,
      audio_packets_delta: after.packets - before.packets,
      audio_bytes_delta: after.bytes - before.bytes,
      audio_energy_delta: after.energy - before.energy, audio_rms_energy: peakEnergy,
      observer_audio_context_state: audioContextState, observer_audio_context_time_delta: audioContextTimeDelta,
      remote_track: remoteTrackState };
  });
}
async function samplePublisher(page) {
  return page.evaluate(async () => {
    const context = new AudioContext();
    await context.resume();
    const track = window.probeMicTrack;
    const source = context.createMediaStreamSource(new MediaStream([track]));
    const analyser = context.createAnalyser();
    source.connect(analyser);
    const waveform = new Float32Array(analyser.fftSize);
    const outbound = async () => {
      const report = await window.probeRoom.engine.pcManager?.publisher.getStats();
      let packets = 0, bytes = 0, energy = 0, rows = 0;
      report?.forEach(row => {
        if (row.type === 'outbound-rtp' && row.kind === 'audio') {
          rows++;
          packets += row.packetsSent || 0;
          bytes += row.bytesSent || 0;
          energy += row.totalAudioEnergy || 0;
        }
      });
      return { packets, bytes, energy, rows };
    };
    const before = await outbound();
    const contextState = context.state, timeBefore = context.currentTime;
    const trackState = { ready_state: track.readyState, muted: track.muted, enabled: track.enabled };
    let peakEnergy = 0;
    for (let i = 0; i < 10; i++) {
      analyser.getFloatTimeDomainData(waveform);
      let energy = 0;
      for (const value of waveform) energy += value * value;
      peakEnergy = Math.max(peakEnergy, energy / waveform.length);
      await new Promise(resolve => setTimeout(resolve, 100));
    }
    const after = await outbound();
    await context.close();
    return { publisher_audio_context_state: contextState,
      publisher_audio_context_time_delta: context.currentTime - timeBefore,
      destination_track: trackState, destination_pcm_rms_energy: peakEnergy,
      outbound_audio_rows: after.rows, outbound_audio_packets_delta: after.packets - before.packets,
      outbound_audio_bytes_delta: after.bytes - before.bytes,
      outbound_audio_energy_delta: after.energy - before.energy };
  });
}
async function rejectedJWT(page, wsURL, jwt) {
  return page.evaluate(async ({ wsURL, jwt }) => {
    const room = new window.LivekitClient.Room();
    try { await room.connect(wsURL, jwt); return { reason: 'accepted', status: 0 }; }
    catch (error) {
      const lk = window.LivekitClient;
      return { reason: error instanceof lk.ConnectionError && error.reason === lk.ConnectionErrorReason.NotAllowed ? 'NotAllowed' : 'other',
        status: error instanceof lk.ConnectionError && error.status === 401 ? 401 : 0 };
    }
    finally { await room.disconnect(); }
  }, { wsURL, jwt });
}
async function participants(url, jwt, room) {
  assertBudget();
  const r = await fetch(`${url}/twirp/livekit.RoomService/ListParticipants`, {
    method: 'POST', headers: { Authorization: `Bearer ${jwt}`, 'Content-Type': 'application/json' },
    body: JSON.stringify({ room }), signal: AbortSignal.timeout(remaining(3000)),
  });
  assert(r.ok, 'room_query_failed');
  const body = await r.json();
  assert(body.participants === undefined || Array.isArray(body.participants), 'room_query_malformed');
  return body.participants || [];
}
async function cleanup() {
  for (const [name, resource] of [['publisher_context', publisherContext], ['observer_context', observerContext], ['browser', browser]]) {
    try { if (resource) { await deadline(name, resource.close(), 3000); evidence.cleanup[name] = true; } }
    catch { evidence.cleanup[name] = false; }
  }
  try { if (httpServer) { await deadline('fixture_close', new Promise(resolve => httpServer.close(resolve)), 3000); evidence.cleanup.fixture_listener = true; } }
  catch { evidence.cleanup.fixture_listener = false; }
  try {
    if (server && server.exitCode === null && server.signalCode === null) {
      const exited = new Promise(resolve => server.once('exit', resolve));
      server.kill('SIGTERM');
      await deadline('server_stop', exited, 3000).catch(async () => {
        server.kill('SIGKILL');
        await deadline('server_kill', exited, 3000).catch(() => {});
      });
    }
    if (server) evidence.cleanup.server = server.exitCode !== null || server.signalCode !== null;
  } catch { evidence.cleanup.server = false; }
  let configClean = true;
  for (const file of [configPath, wavPath]) {
    if (file) try { await unlink(file); } catch (error) { if (error?.code !== 'ENOENT') configClean = false; }
  }
  if (configDir) try { await rmdir(configDir); } catch { configClean = false; }
  evidence.cleanup.config = configClean;
}
async function run() {
  assert(fault === '' || fault === 'after-publish' || fault === 'timeout-after-publish', 'invalid_fault_value');
  assert(platform() === 'darwin' && arch() === 'arm64', 'unsupported_platform_darwin_arm64_required');
  assert(binary && path.isAbsolute(binary), 'set_COMMERCE_R04_LIVEKIT_BINARY_to_pinned_binary');
  const bytes = await readFile(binary).catch(() => { throw new Error('pinned_binary_missing'); });
  assert(createHash('sha256').update(bytes).digest('hex') === BINARY_SHA, 'binary_checksum_mismatch');
  const version = execFileSync(binary, ['--version'], { encoding: 'utf8', timeout: 3000, stdio: ['ignore', 'pipe', 'ignore'] });
  assert(version.includes('1.13.7'), 'binary_version_mismatch');
  const sdkPackage = JSON.parse(await readFile(path.join(ROOT, 'node_modules/livekit-client/package.json'), 'utf8'));
  assert(sdkPackage.version === SDK_VERSION, 'sdk_version_mismatch');
  const lock = await readFile(path.join(ROOT, 'pnpm-lock.yaml'), 'utf8');
  assert(lock.includes('livekit-client@2.22.3:') && /livekit-client@2\.22\.3:[\s\S]*?integrity: sha512-/.test(lock), 'sdk_lock_integrity_missing');
  evidence.gates.RLI01_pins = true;

  const signalPort = await tcpPort(), rtcTCPPort = await tcpPort(), rtcUDPPort = await udpPort(), fixturePort = await tcpPort();
  assert(new Set([signalPort, rtcTCPPort, rtcUDPPort, fixturePort]).size === 4, 'port_collision');
  Object.assign(evidence.resources, { signal_port: signalPort, rtc_tcp_port: rtcTCPPort, rtc_udp_port: rtcUDPPort, fixture_port: fixturePort });
  const apiKey = `r04${randomBytes(8).toString('hex')}`;
  const secret = randomBytes(32).toString('base64url');
  const room = `r04_probe_${randomBytes(8).toString('hex')}`;
  const pubID = `pub_${randomBytes(8).toString('hex')}`;
  const obsID = `obs_${randomBytes(8).toString('hex')}`;
  const config = `port: ${signalPort}\nbind_addresses:\n  - 127.0.0.1\nrtc:\n  tcp_port: ${rtcTCPPort}\n  udp_port: ${rtcUDPPort}\n  use_external_ip: false\n  node_ip: 127.0.0.1\nkeys:\n  ${apiKey}: ${secret}\n`;
  configDir = await mkdtemp(path.join(tmpdir(), 'r04-input-'));
  evidence.resources.config_dir = configDir;
  configPath = path.join(configDir, 'server.yaml');
  wavPath = path.join(configDir, 'fake-mic.wav');
  await writeFile(configPath, config, { mode: 0o600, flag: 'wx' });
  await writeFile(wavPath, fakeMicrophoneWAV(), { mode: 0o600, flag: 'wx' });
  evidence.capture_mode = 'chromium_fake_get_user_media_wav';
  evidence.capture_path = 'getUserMedia_audio';
  server = spawn(binary, ['--config', configPath], { stdio: 'ignore', env: { ...process.env, LIVEKIT_CONFIG: '' } });
  evidence.resources.server_pid = server.pid;
  const signalURL = `http://127.0.0.1:${signalPort}`;
  const wsURL = `ws://127.0.0.1:${signalPort}`;
  await waitReady(signalURL);
  evidence.gates.RLI01_loopback_ready = true;

  const sdk = await readFile(path.join(ROOT, 'node_modules/livekit-client/dist/livekit-client.umd.js'));
  httpServer = createHTTPServer((req, res) => {
    if (req.url === '/fixture') { res.writeHead(200, { 'content-type': 'text/html; charset=utf-8', 'cache-control': 'no-store' }); res.end(pageHTML); }
    else if (req.url === '/sdk.js') { res.writeHead(200, { 'content-type': 'text/javascript; charset=utf-8', 'cache-control': 'no-store' }); res.end(sdk); }
    else { res.writeHead(404, { 'cache-control': 'no-store' }); res.end(); }
  });
  await budgeted('fixture_listen', new Promise((resolve, reject) => httpServer.once('error', reject).listen(fixturePort, '127.0.0.1', resolve)));
  browser = await chromium.launch({ headless: true, timeout: remaining(15_000), args: ['--use-fake-device-for-media-stream', `--use-file-for-fake-audio-capture=${wavPath}`, '--use-fake-ui-for-media-stream', '--autoplay-policy=no-user-gesture-required'] });
  publisherContext = await budgeted('publisher_context_create', browser.newContext({ permissions: ['camera', 'microphone'] }));
  observerContext = await budgeted('observer_context_create', browser.newContext());
  publisherContext.setDefaultTimeout(remaining(12_000));
  observerContext.setDefaultTimeout(remaining(12_000));
  let unexpected = 0;
  for (const context of [publisherContext, observerContext]) {
    await budgeted('route_install', context.route('**/*', route => {
      const u = new URL(route.request().url());
      if (u.hostname === '127.0.0.1' && (u.port === String(fixturePort) || u.port === String(signalPort))) return route.continue();
      unexpected++;
      return route.abort();
    }));
  }
  const publisher = await budgeted('publisher_page_create', publisherContext.newPage());
  const observer = await budgeted('observer_page_create', observerContext.newPage());
  publisher.fixtureURL = observer.fixtureURL = `http://127.0.0.1:${fixturePort}/fixture`;
  await budgeted('fixture_pages', Promise.all([pageSetup(publisher), pageSetup(observer)]));
  const pubToken = token(apiKey, secret, pubID, grant(room, true, false));
  const obsToken = token(apiKey, secret, obsID, grant(room, false, true));
  await connect(observer, wsURL, obsToken, 'observer');
  await budgeted('set_observer_expected_publisher', observer.evaluate(id => { window.expectedPublisher = id; }, pubID));
  await connect(publisher, wsURL, pubToken, 'publisher');
  await budgeted('publish_tracks', publisher.evaluate(async () => {
    await window.probeRoom.localParticipant.setCameraEnabled(true);
    await window.probeRoom.localParticipant.setMicrophoneEnabled(true, {
      echoCancellation: false, noiseSuppression: false, autoGainControl: false,
    });
    window.probeMicTrack = [...window.probeRoom.localParticipant.trackPublications.values()]
      .find(publication => publication.kind === 'audio')?.track?.mediaStreamTrack;
    if (!window.probeMicTrack) throw new Error('fake_mic_track_missing');
  }), 18_000);
  await waitTracks(observer, pubID);
  assertBudget();
  evidence.gates.RLI02_remote_tracks = true;
  evidence.playback = await budgeted('start_playback', startPlayback(observer));
  const [receiver, publisherAudio] = await budgeted('media_sample', Promise.all([sample(observer), samplePublisher(publisher)]), 10_000);
  evidence.counters = { ...receiver, ...publisherAudio };
  const c = evidence.counters;
  assert(c.frames_after > c.frames_before && c.width > 0 && c.height > 0, 'video_frames_not_advancing');
  assert(c.audio_packets_delta > 0 && c.audio_bytes_delta > 0 && (c.audio_energy_delta > 0 || c.audio_rms_energy > 0), 'audio_media_not_advancing');
  evidence.gates.RLI03_decoded_media = true;
  assertBudget();
  if (fault === 'after-publish') throw new Error('injected_after_publish');
  if (fault === 'timeout-after-publish') {
    await deadline('injected_operation', new Promise(() => {}), 100);
    throw new Error('injected_timeout_not_triggered');
  }
  const screenshotPath = path.join(evidenceDir, `r04-input-${started}-observer.png`);
  await mkdir(evidenceDir, { recursive: true, mode: 0o700 });
  await observer.screenshot({ path: screenshotPath, timeout: remaining(5000) });
  evidence.screenshot_path = screenshotPath;

  const expired = token(apiKey, secret, `exp_${randomBytes(6).toString('hex')}`, grant(room, false, true), -60, 120);
  const valid = token(apiKey, secret, `bad_${randomBytes(6).toString('hex')}`, grant(room, false, true));
  const parts = valid.split('.');
  parts[2] = `${parts[2][0] === 'a' ? 'b' : 'a'}${parts[2].slice(1)}`;
  const tampered = parts.join('.');
  evidence.auth = {
    expired: await budgeted('expired_rejection', rejectedJWT(observer, wsURL, expired), 10_000),
    tampered: await budgeted('tampered_rejection', rejectedJWT(observer, wsURL, tampered), 10_000),
  };
  evidence.gates.RLI04_expired_rejected = evidence.auth.expired.reason === 'NotAllowed' && evidence.auth.expired.status === 401;
  evidence.gates.RLI04_tampered_rejected = evidence.auth.tampered.reason === 'NotAllowed' && evidence.auth.tampered.status === 401;
  assert(evidence.gates.RLI04_expired_rejected && evidence.gates.RLI04_tampered_rejected, 'server_accepted_invalid_jwt');
  const adminJWT = token(apiKey, secret, `admin_${randomBytes(6).toString('hex')}`, { room, roomAdmin: true });
  const serverObserver = (await participants(signalURL, adminJWT, room)).find(participant => participant.identity === obsID);
  assert(serverObserver && serverObserver.permission &&
    (serverObserver.permission.canPublish === undefined || serverObserver.permission.canPublish === false) &&
    (serverObserver.tracks === undefined || Array.isArray(serverObserver.tracks)), 'observer_server_grant_invalid');
  evidence.observer_denial = {
    server_can_publish: serverObserver.permission.canPublish ?? false,
    server_track_count: (serverObserver.tracks || []).length,
    ...(await budgeted('observer_denial', observer.evaluate(async () => {
      const canvas = document.createElement('canvas');
      canvas.width = canvas.height = 2;
      canvas.getContext('2d').fillRect(0, 0, 2, 2);
      const track = canvas.captureStream(5).getVideoTracks()[0];
      try {
        await window.probeRoom.localParticipant.publishTrack(track, { source: window.LivekitClient.Track.Source.Camera });
        return { client_error: 'none', client_status: 0 };
      } catch (error) {
        return { client_error: error instanceof window.LivekitClient.PublishTrackError ? 'PublishTrackError' : 'other',
          client_status: error instanceof window.LivekitClient.PublishTrackError && error.status === 403 ? 403 : 0 };
      } finally { track.stop(); }
    }))),
    denial_layer: 'client_permissions',
  };
  const afterDenial = (await participants(signalURL, adminJWT, room)).find(participant => participant.identity === obsID);
  assert(afterDenial && (afterDenial.tracks || []).length === 0, 'observer_server_track_present');
  evidence.gates.RLI04_observer_publish_denied = evidence.observer_denial.server_can_publish === false &&
    evidence.observer_denial.server_track_count === 0 && evidence.observer_denial.client_error === 'PublishTrackError' &&
    evidence.observer_denial.client_status === 403;
  evidence.gates.RLI04_observer_denial_source = 'client_permissions';
  assert(evidence.gates.RLI04_observer_publish_denied, 'observer_publish_not_denied');
  assert(unexpected === 0, 'unexpected_browser_external_request');
  evidence.gates.RLI04 = true;

  await budgeted('publisher_disconnect', publisher.evaluate(async () => {
    for (const pub of window.probeRoom.localParticipant.trackPublications.values()) pub.track?.stop();
    await window.probeRoom.disconnect();
  }));
  await observer.waitForFunction(() => window.probeRemoved >= 2, null, { timeout: remaining(10_000) });
  evidence.gates.RLI05_subscriber_removal = true;
  await budgeted('observer_disconnect', observer.evaluate(() => window.probeRoom.disconnect()));
  let count = -1;
  for (let n = 0; n < 20; n++) { assertBudget(); count = (await participants(signalURL, adminJWT, room)).length; if (count === 0) break; await pause(100); }
  evidence.gates.RLI05_room_empty = count === 0;
  assert(count === 0, 'room_not_empty');
  evidence.status = 'PASS';
}
try { await run(); }
catch (error) {
  const known = new Set(['invalid_fault_value', 'unsupported_platform_darwin_arm64_required',
    'set_COMMERCE_R04_LIVEKIT_BINARY_to_pinned_binary', 'pinned_binary_missing',
    'binary_checksum_mismatch', 'binary_version_mismatch', 'sdk_version_mismatch',
    'sdk_lock_integrity_missing', 'port_collision', 'server_exited_before_ready',
    'server_not_ready', 'fake_mic_track_missing', 'video_frames_not_advancing',
    'audio_media_not_advancing', 'injected_after_publish', 'injected_operation_timeout',
    'injected_timeout_not_triggered', 'runtime_budget_exceeded', 'server_accepted_invalid_jwt',
    'observer_server_grant_invalid', 'observer_server_track_present', 'observer_publish_not_denied',
    'unexpected_browser_external_request', 'room_not_empty', 'room_query_failed', 'room_query_malformed']);
  evidence.failure = error instanceof Error && known.has(error.message) ? error.message : 'operation_failed';
}
finally {
  await cleanup();
  evidence.duration_ms = Date.now() - started;
  if (Object.values(evidence.cleanup).some(value => value === false)) { evidence.status = 'FAIL'; evidence.failure ||= 'cleanup_failed'; }
  if (evidence.status === 'PASS' && !evidence.gates.RLI05_room_empty) { evidence.status = 'FAIL'; evidence.failure = 'room_not_empty'; }
  const evidencePath = path.join(evidenceDir, `r04-input-${started}-${randomBytes(4).toString('hex')}.json`);
  try { await mkdir(evidenceDir, { recursive: true, mode: 0o700 }); await writeFile(evidencePath, `${JSON.stringify(evidence, null, 2)}\n`, { mode: 0o600, flag: 'wx' }); }
  catch { process.stderr.write('evidence_write_failed\n'); process.exitCode = 1; }
  process.stdout.write(`evidence_path=${evidencePath}\nstatus=${evidence.status}\n`);
  if (evidence.status !== 'PASS') process.exitCode = 1;
}
