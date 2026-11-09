// Purpose: execute the real SU07 diagnostic's native-event recording and serialization offline.
// Depends on: stripe-browser.mjs collector source and Node vm/test/assert; no browser or product hooks.
// Used by: scripts/dev/test-node.sh explicit registration.
// Invariants: mock events test collector plumbing only, never native dispatch or payment acceptance.
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createContext, runInContext, runInNewContext } from "node:vm";
import { EventEmitter } from "node:events";

const source = readFileSync(new URL("./stripe-browser.mjs", import.meta.url), "utf8");
const begin = source.indexOf("const su07DiagnosticPages=new WeakMap();");
const end = source.indexOf("// ---- store-tab helpers ----", begin);
assert.ok(begin >= 0 && end > begin, "load the actual bounded collector, not a copy");
const selector = '[data-testid="continue-shopping"]';
const forbidden = "SU07_FORBIDDEN_FREEFORM";
function button() {
  return { disabled: false, inheritedDisabled: false, ariaDisabled: false,
    matches(s) { return s === selector || (s === ":disabled" && (this.disabled || this.inheritedDisabled)); },
    closest(s) { return s === selector ? this : s === '[aria-disabled="true"]' && this.ariaDisabled ? this : null; },
    getBoundingClientRect: () => ({ left: 10, right: 40, top: 10, bottom: 30 }) };
}
function host({ failFinish = false, inject = false } = {}) {
  const listeners = [], removed = [], writes = [], returns = [];
  const state = { button: button(), failQuery: false };
  const window = {};
  const document = {
    readyState: "complete",
    querySelector(s) {
      if (state.failQuery) throw new Error("mock read unavailable");
      return s === selector ? state.button : s === '[data-testid="order-id"]' ? { textContent: "" } : null;
    },
    addEventListener(type, fn, opts) { listeners.push({ type, fn, capture: opts.capture === true, passive: opts.passive }); },
    removeEventListener(type, fn, capture = false) {
      const i = listeners.findIndex(x => x.type === type && x.fn === fn && x.capture === capture);
      if (i >= 0) removed.push(...listeners.splice(i, 1));
    },
  };
  const dom = createContext({ window, document, localStorage: { length: 0 } });
  const page = new EventEmitter();
  page.mainFrame = () => page;
  page.evaluate = async (fn, arg) => {
    if (failFinish && arg?.name === "finish") throw new Error("mock final sample failure");
    dom.arg = arg;
    const value = runInContext(`(${fn.toString()})(arg)`, dom);
    // Fault at the page-to-driver boundary: the actual outer projection must close it.
    if (inject && value?.clickEvents?.length) {
      value.payload = forbidden;
      value.clickEvents[0].payload = forbidden;
      value.clickEvents.push({ ...value.clickEvents[0], type: forbidden });
    }
    returns.push(value);
    return value;
  };
  const loaded = runInNewContext(`${source.slice(begin, end)}\n({startSU07Diagnostic,su07DiagnosticPages})`, {
    origin: "https://example.invalid", within: async p => await p,
    path: { join: (...parts) => parts.join("/") }, evidence: "mock-evidence",
    writeFile: async (name, raw) => { writes.push({ name, raw }); }, hits: () => [], say: () => {}, URL,
  });
  const diagnostic = loaded.startSU07Diagnostic(page);
  function emit(type, overrides = {}) {
    const target = state.button;
    const event = { type, target, isTrusted: true, clientX: 20, clientY: 20, composedPath: () => [target], ...overrides };
    // Direct callback invocation is a MODEL_ONLY host input, not browser dispatch proof.
    for (const l of [...listeners].filter(l => l.type === type).sort((a, b) => Number(b.capture) - Number(a.capture))) l.fn(event);
  }
  return { diagnostic, page, state, window, listeners, removed, writes, returns, emit, loaded,
    result: () => { assert.equal(writes.length, 1); return JSON.parse(writes[0].raw); } };
}
const fields = ["sequence", "phase", "documentOrdinal", "kind", "type", "targetMatches", "closestMatches", "pathIncludes",
  "buttonPresent", "effectivelyDisabled", "sameButtonAsBefore", "pointerInsideButton", "orderPresent", "trusted"].sort();
function closed(h) {
  const result = h.result();
  assert.equal(h.writes[0].raw.includes(forbidden), false, "no injected free-form field/value may be serialized");
  for (const s of result.snapshots) for (const e of s.clickEvents) {
    assert.deepEqual(Object.keys(e).sort(), fields);
    assert.ok(["mousedown", "mouseup", "click"].includes(e.type));
    assert.ok(["capture", "bubble"].includes(e.kind));
    for (const k of fields.filter(k => !["sequence", "phase", "documentOrdinal", "kind", "type"].includes(k))) assert.equal(typeof e[k], "boolean");
  }
  return result;
}
function cleaned(h, old) {
  assert.equal(h.listeners.length, 0); assert.equal(h.removed.length, 6);
  assert.equal(h.window.__su07ContinuationDiagnostic, undefined);
  assert.equal(old.beforeButton, null); assert.equal(old.capture, null); assert.equal(old.bubble, null);
  assert.equal(h.loaded.su07DiagnosticPages.has(h.page), false);
  for (const type of ["framenavigated", "request", "response", "requestfailed"]) assert.equal(h.page.listenerCount(type), 0);
}

test("actual collector delegates three native types only inside the click interval and cleans up", async () => {
  const h = host(); h.page.emit("framenavigated", h.page);
  await h.diagnostic.sample("existing-restore-UI-ready");
  assert.equal(h.listeners.length, 6); assert.ok(h.listeners.every(l => l.passive));
  h.emit("click");
  await h.diagnostic.sample("click-before");
  for (const type of ["mousedown", "mouseup", "click"]) h.emit(type);
  await h.diagnostic.sample("click-after"); h.emit("click");
  const old = h.window.__su07ContinuationDiagnostic;
  await h.diagnostic.finish();
  const r = closed(h), after = r.snapshots.find(s => s.name === "click-after");
  assert.equal(r.schema, 2); assert.equal(after.navigationOrdinal, 1);
  assert.equal(after.clickEvents.length, 6);
  assert.deepEqual(after.clickEvents.map(e => [e.type, e.kind]), ["mousedown", "mouseup", "click"].flatMap(t => [[t,"capture"],[t,"bubble"]]));
  assert.ok(after.clickEvents.every(e => e.phase === 2 && e.documentOrdinal === 1 && e.trusted && e.targetMatches && e.closestMatches && e.pathIncludes && e.sameButtonAsBefore && e.pointerInsideButton));
  assert.equal(r.snapshots[0].clickEvents.length, 0);
  assert.equal(r.snapshots.at(-1).clickEvents.length, 6);
  assert.equal(after.clickWindowOpen, false); assert.equal(r.captureFailed, false);
  assert.equal(r.handlerAccepted, "UNKNOWN"); assert.equal(r.lockAcquired, "UNKNOWN");
  cleaned(h, old);
});

test("target mismatch, descendants, replacement, effective disabled and absent button remain distinguishable", async () => {
  const h = host(); await h.diagnostic.sample("click-before");
  const oldButton = h.state.button;
  const child = { matches: () => false, closest: () => oldButton };
  h.emit("mousedown", { target: child, composedPath: () => [child, oldButton] });
  h.state.button = button(); h.state.button.inheritedDisabled = true;
  const other = { matches: () => false, closest: () => null };
  h.emit("mouseup", { target: other, composedPath: () => [other], clientX: 70 });
  h.state.button = null; h.emit("click", { target: other, composedPath: () => [other], isTrusted: false });
  await h.diagnostic.finish();
  const events = closed(h).snapshots.at(-1).clickEvents;
  assert.equal(events[0].targetMatches, false); assert.equal(events[0].closestMatches, true); assert.equal(events[0].pathIncludes, true);
  assert.equal(events[2].effectivelyDisabled, true); assert.equal(events[2].sameButtonAsBefore, false); assert.equal(events[2].pointerInsideButton, false);
  assert.equal(events[2].closestMatches, false); assert.equal(events[2].pathIncludes, false);
  assert.equal(events[4].buttonPresent, false); assert.equal(events[4].trusted, false);
});

test("DOM cap is 24 independently of the unchanged outer 80-event cap", async () => {
  const h = host(); await h.diagnostic.sample("click-before");
  for (let i = 0; i < 13; i++) h.emit("mousedown");
  for (let i = 0; i < 81; i++) h.page.emit("framenavigated", h.page);
  await h.diagnostic.sample("click-after"); await h.diagnostic.finish();
  const r = closed(h), s = r.snapshots.at(-1);
  assert.equal(s.clickEvents.length, 24); assert.equal(s.clickTruncated, true);
  assert.equal(r.events.length, 80); assert.equal(r.truncated, true);
});

test("callback exceptions mark capture failure and failed final sampling still removes listeners and references", async () => {
  const h = host({ failFinish: true }); await h.diagnostic.sample("click-before");
  h.emit("click", { composedPath() { throw new Error("mock path failure"); } });
  await h.diagnostic.sample("failure");
  const old = h.window.__su07ContinuationDiagnostic;
  await h.diagnostic.finish();
  const r = closed(h); assert.equal(r.captureFailed, true);
  assert.equal(r.snapshots.at(-1).clickCaptureFailed, true);
  assert.equal(r.snapshots.at(-1).clickWindowOpen, false);
  cleaned(h, old);
});

test("actual serialization drops injected free-form fields and invalid event types at page boundary", async () => {
  const h = host({ inject: true }); await h.diagnostic.sample("click-before");
  h.emit("click"); await h.diagnostic.sample("click-after"); await h.diagnostic.finish();
  const r = closed(h); assert.equal(r.snapshots.at(-1).clickEvents.length, 2);
});

test("page-side projection also rejects injected event values before leaving the document", async () => {
  const h = host(); await h.diagnostic.sample("click-before"); h.emit("click");
  const events = h.window.__su07ContinuationDiagnostic.events;
  events[0].payload = forbidden; events[0].targetMatches = forbidden;
  events.push({ ...events[1], kind: forbidden }, { ...events[1], sequence: forbidden });
  await h.diagnostic.sample("click-after"); await h.diagnostic.finish();
  assert.equal(JSON.stringify(h.returns).includes(forbidden), false);
  const r = closed(h); assert.equal(r.snapshots.at(-1).clickEvents.length, 2);
  assert.equal(r.snapshots.at(-1).clickEvents[0].targetMatches, false);
});

test("existing journey keeps ordinary single click, assertions and error/finally delegation unchanged", () => {
  const section = source.slice(source.indexOf("async function newOrder("), source.indexOf("// ----", source.indexOf("async function newOrder(") + 1));
  assert.match(section, /await page\.getByTestId\("continue-shopping"\)\.click\(\);/);
  assert.match(section, /await ex\(page\.getByTestId\("checkout-empty"\)\)\.toBeVisible\(\);/);
  assert.match(section, /catch\(error\)\{if\(diagnostic\)await diagnostic\.sample\("failure"\);throw error;\}/);
  assert.match(section, /finally\{if\(diagnostic\)await diagnostic\.finish\(\);\}/);
});
