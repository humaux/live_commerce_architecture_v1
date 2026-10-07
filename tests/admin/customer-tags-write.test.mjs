// Purpose: regress the actual write hook's UNKNOWN lifetime and explicit same-command retry across auth refusals.
// Depends on: customer-tags-write source, TypeScript API, controlled React hooks and synthetic transport outcomes.
// Used by: W6-U1 local red/green evidence; no browser, PG, note storage or real authority.
import assert from "node:assert/strict";
import { test } from "node:test";
import { readFileSync } from "node:fs";
import { runInNewContext } from "node:vm";
import ts from "typescript-api";

const source = ts.transpileModule(readFileSync(new URL("../../apps/admin/lib/customer-tags-write.ts", import.meta.url), "utf8"), {
  compilerOptions: { target: ts.ScriptTarget.ES2023, module: ts.ModuleKind.CommonJS },
}).outputText;

function harness(outcomes) {
  const slots = []; const effects = []; const cleanups = []; const calls = []; const commits = [];
  let cursor = 0; let keys = 0;
  const react = {
    useState: (initial) => {
      const index = cursor++; if (!(index in slots)) slots[index] = { value: initial };
      return [slots[index].value, (value) => { slots[index].value = typeof value === "function" ? value(slots[index].value) : value; }];
    },
    useRef: (initial) => { const index = cursor++; return slots[index] ??= { current: initial }; },
    useEffect: (effect, deps) => {
      const index = cursor++; const old = slots[index];
      if (!old || deps.some((v, i) => !Object.is(v, old.deps[i]))) {
        slots[index] = { deps }; effects.push(() => { const cleanup = effect(); if (cleanup) cleanups.push(cleanup); });
      }
    },
  };
  const module = { exports: {} };
  runInNewContext(source, { module, exports: module.exports, crypto: { randomUUID: () => `synthetic-key-${++keys}` },
    require: (path) => {
      if (path === "react") return react;
      if (path === "./customer-tags-client") return { sendTagCommand: async (store, command, boundary, parse) => {
        calls.push({ store, command, boundary });
        const outcome = outcomes.shift();
        assert.ok(outcome, "no implicit retry or unexpected command");
        const result = typeof outcome === "function" ? await outcome() : outcome;
        return result.ok ? { ok: true, value: parse(result.value) } : result;
      } };
      throw new Error(`Unexpected fixture import ${path}`);
    },
  });
  const render = () => { cursor = 0; return module.exports.useTagWrite("synthetic-store", "synthetic-boundary"); };
  render(); for (const effect of effects.splice(0)) effect();
  const settle = async () => { for (let n = 0; n < 8; n++) await Promise.resolve(); return render(); };
  const start = (hook, body = "first synthetic note") => hook.run("POST", "customers/abcdef11-1111-4111-8111-111111111111/notes",
    { body }, (v) => v, async (value) => { commits.push(value); });
  return { calls, commits, render, settle, start, keyCount: () => keys, unmount: () => { for (const cleanup of cleanups) cleanup(); } };
}

for (const code of ["unauthorized", "forbidden", "version_changed"]) {
  test(`UNKNOWN survives settled ${code} retry until trusted success; fresh run ignored`, async () => {
    const h = harness([{ ok: false, code: "retry_later", uncertain: true }, { ok: false, code, uncertain: false }, { ok: true, value: "confirmed receipt" }]);
    h.start(h.render()); let hook = await h.settle();
    assert.equal(hook.locked, true); assert.equal(h.calls.length, 1);
    assert.equal(h.commits.length, 0); // No automatic retry and no invented saved state.
    hook.retry(); hook = await h.settle();
    assert.equal(hook.locked, true); assert.equal(hook.uncertain, true); assert.equal(hook.error, code);
    h.start(hook, "different fresh note"); hook = await h.settle();
    assert.equal(h.calls.length, 2); assert.equal(h.keyCount(), 1);
    hook.retry(); hook = await h.settle();
    assert.equal(h.calls.length, 3); assert.equal(hook.locked, false); assert.equal(hook.uncertain, false);
    assert.deepEqual(h.commits, ["confirmed receipt"]);
    for (const call of h.calls) {
      assert.equal(call.command, h.calls[0].command); assert.equal(call.command.key, "synthetic-key-1");
      assert.equal(call.command.method, "POST"); assert.equal(call.command.body, '{"body":"first synthetic note"}');
      assert.equal(call.store, "synthetic-store"); assert.equal(call.boundary, "synthetic-boundary");
    }
  });
}
test("first-attempt preflight refusal is settled and permits a fresh action", async () => {
  const h = harness([{ ok: false, code: "unauthorized", uncertain: false }, { ok: true, value: "new confirmed receipt" }]);
  h.start(h.render()); let hook = await h.settle(); assert.equal(hook.locked, false);
  h.start(hook, "new synthetic note"); hook = await h.settle();
  assert.equal(h.keyCount(), 2); assert.equal(h.calls.length, 2); assert.equal(hook.locked, false);
  assert.notEqual(h.calls[0].command.key, h.calls[1].command.key);
});
test("UNKNOWN and in-flight retries prevent duplicate dispatch; unmount discards stale outcome", async () => {
  let resolve;
  const h = harness([{ ok: false, code: "retry_later", uncertain: true }, () => new Promise((done) => { resolve = done; })]);
  h.start(h.render()); let hook = await h.settle(); hook.retry();
  hook.retry(); h.start(hook, "different synthetic note");
  assert.equal(h.calls.length, 2); assert.equal(h.keyCount(), 1);
  h.unmount(); resolve({ ok: true, value: "stale old-scope receipt" }); await h.settle();
  assert.equal(h.commits.length, 0); assert.equal(h.calls.length, 2);
});
