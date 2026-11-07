// Purpose: verify the guest contact transport boundary without real customer traffic.
// Depends on: contact-client and contact-contract; replaces only the network edge.
// Used by: the existing canonical storefront Node test glob.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { postContactMessage } from "../lib/contact-client.ts";
import { contactReceipt } from "../lib/contact-contract.ts";

const key = "fixture-key-123456";
const body = '{ "name":"Synthetic", "email":"fixture@example.invalid", "message":"line 1\\nline 2" }';
async function withEdge(edge, action) {
  const original = globalThis.fetch;
  globalThis.fetch = edge;
  try { await action(); } finally { globalThis.fetch = original; }
}
test("component delegates contact POST rather than fetching directly", async () => {
  const source = await readFile(new URL("../components/contact/ContactForm.tsx", import.meta.url), "utf8");
  assert.doesNotMatch(source, /\bfetch\s*\(/);
  assert.match(source, /postContactMessage\(current\.key,\s*current\.body\)/);
});
test("single fixed guest POST preserves raw bytes, key and request policy", async () => {
  const calls = [];
  await withEdge(async (...args) => { calls.push(args); return new Response(null, { status: 503 }); }, async () => {
    const result = await postContactMessage(key, body);
    assert.equal(result.status, 503);
  });
  assert.equal(calls.length, 1);
  const [url, options] = calls[0];
  assert.equal(url, "/api/buyer/contact-messages");
  assert.deepEqual(options, { method: "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": key }, body, credentials: "same-origin", cache: "no-store", redirect: "error" });
});
test("network uncertainty has one attempt; explicit recovery uses same bytes and key", async () => {
  const calls = [];
  await withEdge(async (url, options) => { calls.push([url, options]); if (calls.length === 1) throw new TypeError("network unavailable"); return Response.json({ message_id: "12345678-1234-1234-1234-123456789abc", status: "received" }, { status: 201 }); }, async () => {
    await assert.rejects(postContactMessage(key, body), /network unavailable/);
    assert.equal(calls.length, 1);
    const receipt = await postContactMessage(key, body);
    assert.ok(contactReceipt(await receipt.json()));
    assert.equal(calls.length, 2);
    assert.equal(calls[1][1].body, calls[0][1].body);
    assert.equal(calls[1][1].headers["Idempotency-Key"], calls[0][1].headers["Idempotency-Key"]);
    assert.ok(calls.every(([url]) => url === "/api/buyer/contact-messages"));
  });
});
test("malformed 201 receipt cannot be accepted or retried by transport", async () => {
  let count = 0;
  await withEdge(async () => { count++; return Response.json({ status: "received" }, { status: 201 }); }, async () => {
    const response = await postContactMessage(key, body);
    assert.equal(contactReceipt(await response.json()), null);
    assert.equal(count, 1);
  });
});
test("invalid key is rejected before any network call", async () => {
  let count = 0;
  await withEdge(async () => { count++; throw Error("unexpected network call"); }, async () => {
    await assert.rejects(postContactMessage("bad", body), /Invalid contact idempotency key/);
    assert.equal(count, 0);
  });
});
