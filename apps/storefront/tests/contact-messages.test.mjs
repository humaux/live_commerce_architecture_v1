// Purpose: FEAT-CONTACT-STORE public BFF admission and response/privacy fences before implementation.
// Depends on: the real handleBuyerRequest; MOCK fetch is the private Go transport, no server or provider.
// Used by: scripts/dev/test-node.sh's existing apps/storefront/tests/*.test.mjs glob in GitHub CI.
// Invariants: no buyer login required; exact origin, no client scope or body logging, one keyed upstream mutation.
import test from "node:test";
import assert from "node:assert/strict";
import { handleBuyerRequest } from "../lib/buyer-server.ts";

const origin = "https://shop.example";
const api = "http://127.0.0.1:3219";
const id = "00000000-0000-0000-0000-000000000211";
const key = "contact-test-idempotency-0001";
const good = () => ({ name: "測試買家", email: "buyer@example.test", order_number: "LC-10482", message: "Synthetic question", website: "" });
const receipt = () => ({ message_id: id, status: "received" });

function enabled() {
  process.env.COMMERCE_BUYER_WEB_ENABLED = "1";
  process.env.COMMERCE_BUYER_API_ORIGIN = api;
  process.env.COMMERCE_BUYER_BFF_KEY = Buffer.alloc(32, 1).toString("base64url");
  process.env.COMMERCE_BUYER_COOKIE_KEY = Buffer.alloc(32, 2).toString("base64url");
  process.env.COMMERCE_BUYER_SESSION_TTL = "3600";
}

function request(body = good(), extra = {}, method = "POST", suffix = "") {
  return new Request(`${origin}/api/buyer/contact-messages${suffix}`, {
    method,
    headers: { Host: "shop.example", Origin: origin, "Content-Type": "application/json", "Idempotency-Key": key, ...extra },
    body: method === "GET" ? undefined : typeof body === "string" ? body : JSON.stringify(body),
  });
}

async function mock(run, answer = () => Response.json(receipt(), { status: 201 })) {
  enabled();
  const original = globalThis.fetch;
  const calls = [];
  globalThis.fetch = async (url, options) => {
    calls.push({ url: String(url), ...options });
    return answer();
  };
  try { await run(calls); } finally { globalThis.fetch = original; }
}

test("contact BFF admits a guest once without a buyer cookie or context and returns only the committed receipt", async () => {
  await mock(async (calls) => {
    const response = await handleBuyerRequest(request(good(), { "X-Forwarded-For": "203.0.113.21" }));
    assert.equal(response.status, 201, "public contact endpoint must exist before buyer login");
    assert.deepEqual(await response.json(), receipt());
    assert.equal(response.headers.get("set-cookie"), null);
    assert.match(response.headers.get("cache-control"), /no-store/);
    assert.equal(calls.length, 1);
    const call = calls[0];
    assert.equal(call.url, `${api}/v1/buyer/contact-messages`);
    assert.equal(call.method, "POST");
    assert.deepEqual(JSON.parse(call.body), good());
    assert.equal(call.headers.get("Idempotency-Key"), key);
    assert.equal(call.headers.get("X-Commerce-Storefront-Origin"), origin);
    assert.equal(call.headers.get("X-Commerce-Client-IP"), "203.0.113.21");
    assert.equal(call.headers.get("Authorization"), null);
    assert.equal(call.headers.get("Cookie"), null);
    assert.equal(call.headers.get("X-Tenant-ID"), null);
  });
});

test("contact BFF refuses invalid bodies and honeypots before private transport", async () => {
  await mock(async (calls) => {
    for (const [field, value] of [
      ["name", ""], ["name", "  "], ["name", "買".repeat(121)], ["name", "buyer\r\nInjected"],
      ["email", "wrong"], ["email", "buyer@example.test\r\nBcc: injected@example.test"],
      ["order_number", "x".repeat(65)], ["message", ""], ["message", " \n\t "], ["message", "問".repeat(5001)], ["message", 42],
      ["website", "https://spam.example.test"], ["tenant_id", id], ["store_id", id], ["unexpected", true],
    ]) {
      const response = await handleBuyerRequest(request({ ...good(), [field]: value }));
      assert.equal(response.status, 400, `${field} must be refused locally`);
      const text = await response.text();
      assert.doesNotMatch(text, /buyer@example\.test|injected@example\.test|Synthetic question/);
    }
    for (const field of ["name", "email", "message"]) {
      const body = good(); delete body[field];
      assert.equal((await handleBuyerRequest(request(body))).status, 400, `missing ${field}`);
    }
    for (const text of ["{", "[]", "null", '{"name":"first","name":"second","email":"buyer@example.test","message":"question"}']) {
      assert.equal((await handleBuyerRequest(request(text))).status, 400, "strict JSON object required");
    }
    assert.equal(calls.length, 0);
  });
});

test("contact BFF enforces method, key, content type and origin independently of a guest session", async () => {
  await mock(async (calls) => {
    for (const [label, input, status] of [
      ["GET", request(undefined, {}, "GET"), 405],
      ["wrong origin", request(good(), { Origin: "https://foreign.example.test" }), 403],
      ["tenant header", request(good(), { "X-Tenant-ID": id }), 403],
      ["invalid key", request(good(), { "Idempotency-Key": "x" }), 422],
      ["invalid media", request(good(), { "Content-Type": "text/plain" }), 415],
      ["body size", request("x".repeat(65537)), 413],
      ["query scope", request(good(), {}, "POST", `?store_id=${id}`), 422],
    ]) assert.equal((await handleBuyerRequest(input)).status, status, label);
    const missing = request(); missing.headers.delete("Idempotency-Key");
    assert.equal((await handleBuyerRequest(missing)).status, 422);
    assert.equal(calls.length, 0);
  });
});

test("contact BFF accepts Unicode bounds and omitted optional fields", async () => {
  await mock(async (calls) => {
    const body = { name: "買".repeat(120), email: "buyer@example.test", message: "問".repeat(5000) };
    assert.equal((await handleBuyerRequest(request(body))).status, 201);
    assert.equal(calls.length, 1);
    assert.deepEqual(JSON.parse(calls[0].body), body);
  });
});

test("contact BFF validates successful upstream receipts without leaking PII or creating sessions", async () => {
  for (const invalid of [{}, { message_id: "wrong", status: "received" }, { ...receipt(), email: "buyer@example.test" }, { ...receipt(), status: "sent" }]) {
    await mock(async () => {
      const response = await handleBuyerRequest(request());
      assert.equal(response.status, 503);
      assert.equal(response.headers.get("set-cookie"), null);
      assert.doesNotMatch(await response.text(), /buyer@example\.test/);
    }, () => Response.json(invalid, { status: 201 }));
  }
});

test("contact BFF relays bounded throttling, conflicts and uncertain outcomes without retrying", async () => {
  for (const [status, code] of [[429, "rate_limited"], [409, "conflict"], [503, "unavailable"]]) {
    await mock(async (calls) => {
      const response = await handleBuyerRequest(request());
      assert.equal(response.status, status);
      assert.equal((await response.json()).code, code);
      assert.equal(response.headers.get("set-cookie"), null);
      if (status === 429) assert.equal(response.headers.get("retry-after"), "42");
      assert.equal(calls.length, 1, "uncertain contact commands must not auto-retry");
    }, () => Response.json({ code, message: "Refused", request_id: "0".repeat(32), retryable: false, details: {} }, { status, headers: { "Retry-After": "42" } }));
  }
  await mock(async (calls) => {
    assert.equal((await handleBuyerRequest(request())).status, 503);
    assert.equal(calls.length, 1);
  }, () => { throw new Error("MOCK private transport interruption"); });
});
