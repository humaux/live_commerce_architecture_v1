// Purpose: freeze FEAT-BUYER-RMA's same-origin BFF request/response boundary before implementation.
// Depends on: existing handleBuyerRequest and signed cookie/context lifecycle; upstream replies here are synthetic MOCK data.
// Used by: GitHub Foundation Node suite and Gates unit shard; never executed locally.
// Invariants: no generic proxy, no buyer credentials/PII in output, strict bodies/projections and no automatic mutation retry.
import test from "node:test";
import assert from "node:assert/strict";
import { handleBuyerRequest } from "../lib/buyer-server.ts";

const origin = "https://shop.example";
const api = "http://127.0.0.1:3219";
const id = "11111111-1111-4111-8111-111111111111";
const order = "22222222-2222-4222-8222-222222222222";
const sku = "33333333-3333-4333-8333-333333333333";
const warehouse = "44444444-4444-4444-8444-444444444444";
const at = "2026-10-30T12:00:00Z";
const input = { reason: "damaged", note: "測試商品縫線需要確認", lines: [{ sku_id: sku, warehouse_id: warehouse, quantity: 1 }] };
const entry = {
  id, number: "R-1001", order_id: order, status: "requested", version: 1,
  reason: "damaged", note: input.note, rejection_reason: null,
  return_id: null, return_state: null, created_at: at, updated_at: at,
  lines: [{ sku_id: sku, warehouse_id: warehouse, quantity: 1 }],
  timeline: [{ status: "requested", at }],
};
const createPath = `orders/${order}/return-requests`;
const detailPath = `return-requests/${id}`;

function enabled() {
  process.env.COMMERCE_BUYER_WEB_ENABLED = "1";
  process.env.COMMERCE_BUYER_API_ORIGIN = api;
  process.env.COMMERCE_BUYER_BFF_KEY = Buffer.alloc(32, 1).toString("base64url");
  process.env.COMMERCE_BUYER_COOKIE_KEY = Buffer.alloc(32, 2).toString("base64url");
  process.env.COMMERCE_BUYER_SESSION_TTL = "3600";
}
function request(method, suffix, { cookie, context, body, key } = {}) {
  const headers = new Headers({ Host: "shop.example" });
  if (method !== "GET") headers.set("Origin", origin);
  if (cookie) headers.set("Cookie", cookie);
  if (context) headers.set("X-Buyer-Context", context);
  if (body !== undefined) headers.set("Content-Type", "application/json");
  if (key) headers.set("Idempotency-Key", key);
  return new Request(`${origin}/api/buyer/${suffix}`, { method, headers, body });
}
async function session() {
  const res = await handleBuyerRequest(request("POST", "session/prepare", { body: "{}" }));
  assert.equal(res.status, 200);
  return { cookie: res.headers.get("set-cookie")?.split(";", 1)[0], context: (await res.json()).context };
}

test("BR-BFF exact buyer request routes, cookie authority, keyed create/withdraw and safe persisted projections", async () => {
  enabled();
  const old = globalThis.fetch;
  const calls = [];
  globalThis.fetch = async (url, options) => {
    calls.push({ url: String(url), options });
    assert.equal(options.headers.get("X-Commerce-Storefront-Origin"), origin);
    assert.equal(options.headers.get("X-Commerce-Buyer-BFF-Key"), process.env.COMMERCE_BUYER_BFF_KEY);
    assert.match(options.headers.get("Authorization"), /^Bearer [A-Za-z0-9_-]{43}$/);
    assert.equal(options.headers.has("Cookie"), false);
    assert.equal(options.redirect, "error");
    if (String(url).endsWith("/withdraw")) return Response.json({ ...entry, status: "withdrawn", version: 2, timeline: [...entry.timeline, { status: "withdrawn", at }] });
    if (String(url).endsWith("/v1/buyer/return-requests")) return Response.json({ items: [entry] });
    return Response.json(entry, { status: options.method === "POST" ? 201 : 200 });
  };
  try {
    const auth = await session();
    let response = await handleBuyerRequest(request("POST", createPath, { ...auth, key: "br-bff-create-0001", body: JSON.stringify(input) }));
    assert.equal(response.status, 201, "buyer request BFF create must be reachable");
    assert.deepEqual(await response.json(), entry);
    assert.equal(calls.length, 1);
    assert.equal(calls[0].url, `${api}/v1/buyer/${createPath}`);
    assert.equal(calls[0].options.headers.get("Idempotency-Key"), "br-bff-create-0001");
    assert.deepEqual(JSON.parse(calls[0].options.body), input);
    for (const suffix of [detailPath, "return-requests"]) {
      response = await handleBuyerRequest(request("GET", suffix, auth));
      assert.equal(response.status, 200);
      assert.deepEqual(await response.json(), suffix === detailPath ? entry : { items: [entry] });
      assert.equal(calls.at(-1).options.headers.has("Idempotency-Key"), false);
      assert.equal(response.headers.get("cache-control"), "no-store");
      assert.equal(response.headers.get("set-cookie"), null);
    }
    response = await handleBuyerRequest(request("POST", `${detailPath}/withdraw`, { ...auth, key: "br-bff-withdraw-0001", body: '{"expected_version":1}' }));
    assert.equal(response.status, 200);
    assert.equal((await response.json()).status, "withdrawn");
    assert.equal(calls.length, 4);
  } finally { globalThis.fetch = old; }
});

test("BR-BFF denies unknown input authority, bad quantities, missing key/context and wrong routes before fetch", async () => {
  enabled();
  const old = globalThis.fetch;
  let calls = 0;
  globalThis.fetch = async () => { calls++; throw new Error("must never forward rejected request"); };
  try {
    const auth = await session();
    const valid = { ...auth, key: "br-bff-validation", body: JSON.stringify(input) };
    const cases = [
      ["POST", createPath, { ...valid, cookie: undefined }],
      ["POST", createPath, { ...valid, context: Buffer.alloc(32, 3).toString("base64url") }],
      ["POST", createPath, { ...valid, key: undefined }],
      ["POST", createPath, { ...valid, body: JSON.stringify({ ...input, reason: "invented" }) }],
      ["POST", createPath, { ...valid, body: JSON.stringify({ ...input, note: "測".repeat(1001) }) }],
      ["POST", createPath, { ...valid, body: JSON.stringify({ ...input, lines: [] }) }],
      ["POST", createPath, { ...valid, body: JSON.stringify({ ...input, lines: [{ sku_id: sku, quantity: 0 }] }) }],
      ["POST", createPath, { ...valid, body: JSON.stringify({ ...input, lines: [{ sku_id: sku, quantity: 1.5 }] }) }],
      ["POST", createPath, { ...valid, body: JSON.stringify({ ...input, lines: [input.lines[0], input.lines[0]] }) }],
      ["POST", createPath, { ...valid, body: '{"reason":"damaged","reason":"damaged","lines":[]}' }],
      ["POST", `${detailPath}/withdraw`, { ...valid, body: '{"expected_version":0}' }],
      ["DELETE", detailPath, auth],
      ["POST", `${detailPath}/approve`, valid],
      ["GET", `${detailPath}?tenant_id=untrusted`, auth],
    ];
    for (const field of ["tenant_id", "store_id", "owner_id", "amount_minor", "photos"]) {
      cases.push(["POST", createPath, { ...valid, body: JSON.stringify({ ...input, [field]: "untrusted" }) }]);
    }
    for (const [method, path, options] of cases) {
      const response = await handleBuyerRequest(request(method, path, options));
      assert.ok(response.status >= 400 && response.status < 500, `${method} ${path} expected local refusal, got ${response.status}`);
    }
    assert.equal(calls, 0);
  } finally { globalThis.fetch = old; }
});

test("BR-BFF rejects unsafe upstream projection and preserves closed refusal codes without body leakage or retry", async () => {
  enabled();
  const old = globalThis.fetch;
  let reply; let calls = 0;
  globalThis.fetch = async () => { calls++; return reply(); };
  try {
    const auth = await session();
    for (const broken of [
      { ...entry, status: "REGISTERED" },
      { ...entry, order_id: "invalid" },
      { ...entry, lines: [{ sku_id: sku, warehouse_id: warehouse, quantity: -1 }] },
      { ...entry, timeline: [] },
      { ...entry, version: 0 },
      { ...entry, return_id: id, return_state: "INVENTED" },
      { ...entry, tenant_id: "upstream-private-marker" },
    ]) {
      reply = () => Response.json(broken);
      const response = await handleBuyerRequest(request("GET", detailPath, auth));
      assert.equal(response.status, 503, "invalid request projection must fail closed");
      assert.equal((await response.text()).includes("upstream-private-marker"), false);
    }
    for (const [status, code] of [[409, "window_closed"], [409, "request_open"], [409, "version_changed"], [409, "conflict"], [422, "exceeds_shipped"], [422, "unknown_line"], [403, "forbidden"]]) {
      reply = () => Response.json({ code, message: "upstream-private-marker", details: { note: "upstream-private-marker" }, request_id: "upstream-private-marker", retryable: true }, { status });
      const before = calls;
      const response = await handleBuyerRequest(request("POST", createPath, { ...auth, key: "br-bff-coded-error", body: JSON.stringify(input) }));
      assert.equal(response.status, status);
      const raw = await response.text(); const result = JSON.parse(raw);
      assert.equal(result.code, code);
      assert.equal(result.retryable, false);
      assert.deepEqual(result.details, {});
      assert.equal(raw.includes("upstream-private-marker"), false);
      assert.equal(calls, before + 1, "BFF must never retry a buyer mutation");
    }
  } finally { globalThis.fetch = old; }
});
