// Purpose: exercise the actual Settings native passive collector, its privacy boundary and cleanup.
// Depends on: settings-v2.spec.ts, node:module type erasure, node:vm and node:test; no React/browser host.
// Used by: scripts/dev/test-node.sh; TEST ONLY collector plumbing, never browser scheduling proof.
import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { stripTypeScriptTypes } from 'node:module';
import { runInNewContext } from 'node:vm';
import { execFileSync } from 'node:child_process';
const source = readFileSync(new URL('./settings-v2.spec.ts', import.meta.url), 'utf8');
const begin = source.indexOf('// SETTINGS PASSIVE COLLECTOR BEGIN');
const end = source.indexOf('// SETTINGS PASSIVE COLLECTOR END');
const forbidden = 'FORBIDDEN_PRIVATE_PAYLOAD';
function host() {
  assert.ok(begin >= 0 && end > begin, 'actual-source collector must exist');
  const listeners = [], calls = [], sentinel = {}, failure = new Error('original storage error');
  let thrown = false;
  class Storage {
    values = new Map();
    setItem(...args) {
      calls.push({ receiver: this, args });
      if (!(this instanceof Storage) || thrown) throw failure;
      this.values.set(String(args[0]), String(args[1])); return sentinel;
    }
    getItem(key) { return this.values.get(key) ?? null; }
  }
  class Element { closest() { return this.inside ? this : null; } inside = true; }
  class HTMLInputElement extends Element { type = 'checkbox'; }
  const document = {
    querySelector: () => ({ disabled: true }),
    addEventListener(type, fn, options) { listeners.push({ type, fn, ...options }); },
    removeEventListener(type, fn, capture) { const i = listeners.findIndex(l => l.type === type && l.fn === fn && l.capture === capture); if (i >= 0) listeners.splice(i, 1); },
  };
  const window = { sessionStorage: new Storage() };
  const original = Storage.prototype.setItem;
  const js = stripTypeScriptTypes(source.slice(begin, end), { mode: 'strip' });
  const api = runInNewContext(`${js}\n({installSettingsPassiveLedger, projectSettingsPassiveLedger})`, { Storage, window, document, Element, HTMLInputElement });
  api.installSettingsPassiveLedger({ store: 'owned', target: 'target' });
  const key = `commerce-settings-draft:owned:${forbidden}`;
  const value = version => JSON.stringify({ step: 3, draft: { reference: forbidden, policyEnabled: true,
    policyObservation: { target: 'target:TW:home', version, dirty: true } } });
  const emit = (type, trusted = true, inside = true) => {
    const target = new HTMLInputElement(); target.inside = inside;
    // Direct callbacks test only collector plumbing; these are not native browser events.
    for (const l of listeners.filter(l => l.type === type)) l.fn({ target, isTrusted: trusted });
  };
  return { window, Storage, original, listeners, calls, sentinel, failure, api, key, value, emit,
    fail: () => { thrown = true; }, finish: () => window.__settingsPassiveLedger.finish() };
}
function closed(h, raw = h.finish()) {
  const result = h.api.projectSettingsPassiveLedger(raw);
  const json = JSON.stringify(result);
  assert.equal(json.includes(forbidden), false);
  for (const row of result.rows) {
    assert.ok(['storage', 'event'].includes(row.kind));
    const fields = row.kind === 'storage'
      ? ['ordinal','kind','readbackMatches','targetMatches','step','policyVersion','policyDirty','serviceVersion','referenceNonempty','policyEnabled']
      : ['ordinal','kind','type','phase','control','trusted','savePresent','saveDisabled'];
    assert.deepEqual(Object.keys(row).sort(), fields.sort());
    assert.ok(Object.values(row).every(v => v === null || ['number', 'boolean', 'string'].includes(typeof v)));
  }
  return result;
}
test('delegates once with original receiver/args/return and preserves storage/event order', () => {
  const h = host(), s = h.window.sessionStorage;
  assert.equal(s.setItem(h.key, h.value(0)), h.sentinel);
  h.emit('input'); s.setItem(h.key, h.value(-1));
  const r = closed(h);
  assert.equal(h.calls.length, 2); assert.equal(h.calls[0].receiver, s);
  assert.deepEqual(h.calls[0].args, [h.key, h.value(0)]);
  assert.deepEqual(Array.from(r.rows, x => x.ordinal), [1,2,3,4]);
  assert.deepEqual(Array.from(r.rows, x => x.kind), ['storage','event','event','storage']);
  assert.equal(r.rows[0].policyVersion, 0); assert.equal(r.rows[3].policyVersion, -1);
  assert.equal(r.rows[1].trusted, true); assert.equal(r.rows[1].phase, 'capture');
  assert.equal(r.rows[2].phase, 'bubble'); assert.equal(r.rows[3].referenceNonempty, true);
});
test('original throw identity and illegal receiver pass through without double delegation', () => {
  const h = host(), wrapped = h.Storage.prototype.setItem;
  assert.throws(() => wrapped.call({}, h.key, 'x'), e => e === h.failure);
  h.fail(); assert.throws(() => wrapped.call(h.window.sessionStorage, h.key, 'x'), e => e === h.failure);
  assert.equal(h.calls.length, 2); const r = closed(h); assert.equal(r.storageError, true); assert.equal(r.rows.length, 0);
});
test('scope, malformed observation and bounded contiguous cap do not change writes', () => {
  const h = host(), s = h.window.sessionStorage;
  s.setItem('commerce-settings-draft:other:private', forbidden);
  new h.Storage().setItem(h.key, forbidden); h.emit('click', true, false);
  s.setItem(h.key, '{');
  for (let i=0;i<140;i++) s.setItem(h.key, h.value(0));
  const r = closed(h); assert.equal(r.rows.length, 128); assert.equal(r.truncated, true);
  assert.equal(r.observationError, true); assert.equal(h.calls.length, 143);
  assert.deepEqual(Array.from(r.rows, x => x.ordinal), Array.from({length:128}, (_,i)=>i+1));
});
test('finally cleanup restores method/listeners even after original failure; diagnostic retained', () => {
  const h = host(); let r;
  try { h.window.sessionStorage.setItem(h.key, h.value(0)); h.fail(); h.window.sessionStorage.setItem(h.key, 'x'); }
  catch (e) { assert.equal(e, h.failure); }
  finally { r = closed(h); }
  assert.equal(h.Storage.prototype.setItem, h.original); assert.equal(h.listeners.length, 0);
  assert.equal(r.rows.length, 1); assert.equal(r.storageError, true);
  assert.equal(h.window.__settingsPassiveLedger, undefined);
});
test('driver projection drops forbidden fields, rejects arbitrary scalar values and caps rows', () => {
  const h = host(); h.window.sessionStorage.setItem(h.key, h.value(0));
  const raw = h.finish(); raw.payload = forbidden; raw.rows[0].payload = forbidden;
  raw.rows.push({ kind:'event', ordinal:2, type:forbidden, phase:forbidden, control:forbidden, trusted:forbidden });
  const r = closed(h, raw);
  assert.equal(r.rows[1].type, null); assert.equal(r.rows[1].trusted, null);
  assert.equal(r.rows[1].control, null); assert.equal(r.rows[1].phase, null);
  raw.rows = Array.from({length:200}, () => raw.rows[0]);
  const capped = closed(h, raw); assert.equal(capped.rows.length,128); assert.equal(capped.truncated,true);
  assert.equal(Object.hasOwn(r, 'payload'), false); assert.equal(Object.hasOwn(r.rows[0], 'payload'), false);
});
test('accepted original journey is byte equal excluding only passive attach/harvest ranges', () => {
  const base = execFileSync('git', ['show','3d924d9cfbe56922be15b692da0ea45cbc5525f1:tests/admin/settings-v2.spec.ts'], {encoding:'utf8'});
  const restored = source.replace(/\/\/ SETTINGS PASSIVE COLLECTOR BEGIN[\s\S]*?\/\/ SETTINGS PASSIVE COLLECTOR END\n\n/, '')
    .replace(/  \/\/ SETTINGS PASSIVE HISTORY BEGIN[\s\S]*?  \/\/ SETTINGS PASSIVE HISTORY END\n/, '')
    .replace(/    \/\/ SETTINGS PASSIVE ATTACH BEGIN[\s\S]*?    \/\/ SETTINGS PASSIVE ATTACH END\n/, '')
    .replace(/      \/\/ SETTINGS PASSIVE HARVEST BEGIN[\s\S]*?      \/\/ SETTINGS PASSIVE HARVEST END\n/, '');
  assert.equal(restored, base, 'every original network fixture, journey and assertion stays unchanged');
});

test('untrusted events remain untrusted and session/target drift are only booleans', () => {
  const h = host(), s = h.window.sessionStorage;
  s.setItem(h.key, h.value(0)); h.emit('change', false);
  s.setItem(h.key + 'second', JSON.stringify({draft:{policyObservation:{target:forbidden,version:-1}}}));
  const r = closed(h); assert.equal(r.rows[1].trusted,false); assert.equal(r.rows[2].trusted,false);
  assert.equal(r.sessionDrift,true); assert.equal(r.targetDrift,true); assert.equal(r.rows[3].targetMatches,false);
});
