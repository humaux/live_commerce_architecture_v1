// Purpose: adversarial tests of actual fenced tag/note write transport and explicit UNKNOWN replay.
// Depends on: customer-tags-client, synthetic fetch/cookie fixtures, focused in-memory TSX loader.
// Used by: W6-U1 local MOCK evidence; no real sessions, network calls or buyer data.
import assert from "node:assert/strict";
import { test } from "node:test";
import { registerCustomerTagsLoader } from "./customer-tags-node-loader.mjs";
registerCustomerTagsLoader();
const { sendTagCommand } = await import("../../apps/admin/lib/customer-tags-client.ts");
const { sessionBoundary } = await import("../../apps/admin/lib/settings-client.ts");
const cookie = "a".repeat(43);
const command = { key: "synthetic-key-1234", method: "PATCH", resource: "customers/abcdef11-1111-4111-8111-111111111111/notes/22222222-2222-4222-8222-222222222222", body: '{"body":"synthetic note","version":2}' };
const originalFetch = globalThis.fetch;
const originalDocument = Object.getOwnPropertyDescriptor(globalThis, "document");
async function fixture(run) {
  const doc = { cookie: `__Host-commerce_csrf=${cookie}` };
  Object.defineProperty(globalThis, "document", { configurable: true, value: doc });
  try { await run(doc, await sessionBoundary()); }
  finally { globalThis.fetch = originalFetch; if (originalDocument) Object.defineProperty(globalThis, "document", originalDocument); else delete globalThis.document; }
}
const success = () => Response.json({ ok: true }, { headers: { "Cache-Control": "private, no-store" } });

test("session change before network sends nothing", async () => fixture(async (doc, boundary) => {
  let calls = 0; globalThis.fetch = async () => { calls++; return success(); };
  doc.cookie = `__Host-commerce_csrf=${"b".repeat(43)}`;
  assert.deepEqual(await sendTagCommand("synthetic-store", command, boundary, (v) => v), { ok: false, code: "unauthorized", uncertain: false });
  assert.equal(calls, 0);
}));
test("session changes after network or during body parse drop the response", async () => fixture(async (doc, boundary) => {
  for (const duringJSON of [false, true]) {
    doc.cookie = `__Host-commerce_csrf=${cookie}`;
    globalThis.fetch = async () => {
      const r = success();
      if (duringJSON) r.json = async () => { doc.cookie = `__Host-commerce_csrf=${"b".repeat(43)}`; return { ok: true }; };
      else doc.cookie = `__Host-commerce_csrf=${"b".repeat(43)}`;
      return r;
    };
    const result = await sendTagCommand("synthetic-store", command, boundary, (v) => v);
    assert.equal(result.ok, false); assert.equal(result.code, "unauthorized");
    assert.equal(result.uncertain, true); // Dispatch already happened; a failed session fence is not a no-effect receipt.
  }
}));
test("transient session digest failure is settled only before dispatch", async () => fixture(async (_doc, boundary) => {
  const digest = crypto.subtle.digest;
  let calls = 0;
  try {
    crypto.subtle.digest = async () => { throw new Error("synthetic digest unavailable"); };
    globalThis.fetch = async () => { calls++; return success(); };
    assert.deepEqual(await sendTagCommand("synthetic-store", command, boundary, (v) => v), { ok: false, code: "unauthorized", uncertain: false });
    assert.equal(calls, 0);
    crypto.subtle.digest = digest;
    globalThis.fetch = async () => {
      calls++; crypto.subtle.digest = async () => { throw new Error("synthetic post-dispatch digest unavailable"); };
      return success();
    };
    assert.deepEqual(await sendTagCommand("synthetic-store", command, boundary, (v) => v), { ok: false, code: "unauthorized", uncertain: true });
    assert.equal(calls, 1);
  } finally { crypto.subtle.digest = digest; }
}));
test("lost response followed by a changed session is UNKNOWN after dispatch", async () => fixture(async (doc, boundary) => {
  let calls = 0;
  globalThis.fetch = async () => { calls++; doc.cookie = `__Host-commerce_csrf=${"b".repeat(43)}`; throw new Error("synthetic lost reply"); };
  assert.deepEqual(await sendTagCommand("synthetic-store", command, boundary, (v) => v), { ok: false, code: "unauthorized", uncertain: true });
  assert.equal(calls, 1);
}));
test("lost response does not auto-retry; explicit replay keeps method/key/payload/scope", async () => fixture(async (_doc, boundary) => {
  const calls = [];
  globalThis.fetch = async (url, init) => { calls.push({ url, init }); if (calls.length === 1) throw new Error("synthetic lost response"); return success(); };
  const result = await sendTagCommand("synthetic-store", command, boundary, (v) => v);
  assert.equal(result.uncertain, true); assert.equal(calls.length, 1);
  const retried = await sendTagCommand("synthetic-store", command, boundary, (v) => v);
  assert.equal(retried.ok, true); assert.equal(calls.length, 2);
  for (const call of calls) {
    assert.equal(call.init.method, "PATCH"); assert.equal(call.init.body, command.body);
    assert.equal(call.init.headers["Idempotency-Key"], command.key); assert.equal(call.init.headers["X-CSRF-Token"], cookie);
    assert.equal(call.url, `/api/stores/synthetic-store/${command.resource}`);
  }
}));
test("5xx or malformed success is uncertain, coded refusal is settled", async () => fixture(async (_doc, boundary) => {
  for (const [response, expected] of [[Response.json({ code: "retry_later" }, { status: 503 }), true],
    [new Response("bad", { headers: { "Content-Type": "application/json", "Cache-Control": "private, no-store" } }), true],
    [Response.json({ code: "version_changed" }, { status: 409 }), false]]) {
    globalThis.fetch = async () => response;
    const result = await sendTagCommand("synthetic-store", command, boundary, (v) => v);
    assert.equal(result.ok, false); assert.equal(result.uncertain, expected);
  }
}));
