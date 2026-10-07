// Purpose: hook-level regression of useGuardedRead (lib/customers-client.ts): a read that ends signed-out signals the global
//   logout lifecycle once, so every other guarded read on the page (customer list, detail, catalogue) clears its PII too;
//   other failures and a successful read signal nothing (Codex review P2, PR #3).
// Depends on: customers-client source via the TypeScript API, synthetic React hooks; settings-client and session-events are
//   replaced only at their browser edges (cookie fence, event dispatch).
// Used by: scripts/dev/test-node.sh (W6-U1 local MOCK evidence; browser gates remain the click proof).
import assert from "node:assert/strict";
import { test } from "node:test";
import { readFileSync } from "node:fs";
import { runInNewContext } from "node:vm";
import ts from "typescript-api";

const source = ts.transpileModule(readFileSync(new URL("../../apps/admin/lib/customers-client.ts", import.meta.url), "utf8"), {
  compilerOptions: { target: ts.ScriptTarget.ES2023, module: ts.ModuleKind.CommonJS },
}).outputText;

function harness() {
  const slots = []; let cursor = 0; const effects = []; const logouts = [];
  const react = {
    useState: (initial) => { const i = cursor++; if (!(i in slots)) slots[i] = { value: initial };
      return [slots[i].value, (v) => { slots[i].value = typeof v === "function" ? v(slots[i].value) : v; }]; },
    useRef: (initial) => { const i = cursor++; return slots[i] ??= { current: initial }; },
    useCallback: (fn) => fn,
    useEffect: (effect) => { effects.push(effect); },
  };
  const module = { exports: {} };
  const windowStub = { addEventListener() {}, removeEventListener() {} };
  runInNewContext(source, { module, exports: module.exports, AbortController, Error, Promise, setTimeout, URL, URLSearchParams,
    window: windowStub, document: { visibilityState: "visible", addEventListener() {}, removeEventListener() {} },
    BroadcastChannel: class { addEventListener() {} removeEventListener() {} close() {} },
    require: (path) => {
      if (path === "react") return react;
      if (path === "./settings-client") return { csrfCookie: () => "csrf", safeError: (v) => v, sessionBoundary: async () => "session-a" };
      if (path === "./session-events") return { signalLogout: () => logouts.push("logout") }; // event-dispatch edge only
      if (path === "./customers-model") return {};
      throw new Error(`Unexpected fixture import ${path}`);
    },
  });
  const { useGuardedRead, ReadError } = module.exports;
  const mount = (run) => { cursor = 0; const out = useGuardedRead("scope", run, null); for (const e of effects.splice(0)) e(); return out; };
  const peek = (run) => { cursor = 0; const out = useGuardedRead("scope", run, null); effects.splice(0); return out; };
  return { mount, peek, ReadError, logouts };
}
const settle = () => new Promise((resolve) => setTimeout(resolve, 0));

test("a guarded read that ends signed-out signals the global logout once", async () => {
  const h = harness();
  h.mount(async () => { throw new h.ReadError("signed-out"); }); await settle();
  assert.deepEqual(h.logouts, ["logout"]);
});
test("forbidden/unavailable failures and a successful read signal no logout", async () => {
  for (const run of [async () => { throw new Error("boom"); }, async () => ({ ok: true })]) {
    const h = harness(); h.mount(run); await settle(); assert.deepEqual(h.logouts, []);
  }
  const h = harness(); h.mount(async () => { throw new h.ReadError("forbidden"); }); await settle(); assert.deepEqual(h.logouts, []);
});

// Codex review P2 (PR #3): refresh() (the in-place re-GET after a write) handles authority outcomes exactly like load():
// signed-out blocks and signals the global logout; forbidden / not-found clear the data; transient failures change nothing.
async function readyThenRefresh(second) {
  const h = harness(); let calls = 0;
  const run = async () => { if (calls++ === 0) return { pii: "synthetic" }; return second(h); };
  h.mount(run); await settle();
  assert.equal(h.peek(run).status, "ready");
  const result = await h.peek(run).refresh(); await settle();
  return { h, result, view: h.peek(run) };
}
test("refresh ending signed-out clears the view, blocks and signals the global logout once", async () => {
  const { h, result, view } = await readyThenRefresh(async (x) => { throw new x.ReadError("signed-out"); });
  assert.equal(result, false); assert.equal(view.status, "signed-out"); assert.equal(view.data, null); assert.deepEqual(h.logouts, ["logout"]);
});
test("refresh ending forbidden or not-found clears the data without a logout", async () => {
  for (const code of ["forbidden", "not-found"]) {
    const { h, view } = await readyThenRefresh(async (x) => { throw new x.ReadError(code); });
    assert.equal(view.status, code); assert.equal(view.data, null); assert.deepEqual(h.logouts, [], code);
  }
});
test("a transient refresh failure keeps the ready view and signals nothing", async () => {
  for (const fail of [async () => { throw new Error("boom"); }, async (x) => { throw new x.ReadError("unavailable"); }]) {
    const { h, result, view } = await readyThenRefresh(fail);
    assert.equal(result, false); assert.equal(view.status, "ready"); assert.deepEqual(view.data, { pii: "synthetic" }); assert.deepEqual(h.logouts, []);
  }
});
