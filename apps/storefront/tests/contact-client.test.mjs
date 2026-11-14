// Purpose: verify the guest contact transport boundary and client contracts without real customer traffic.
// Depends on: contact-client, contact-copy, contact-contract and ContactStoreButton; replaces only the network edge.
// Used by: scripts/dev/test-node.sh's existing apps/storefront/tests/*.test.mjs glob in GitHub CI.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { postContactMessage } from "../lib/contact-client.ts";
import { contactReceipt, validContactBody, validContactKey } from "../lib/contact-contract.ts";
import { contactCopy } from "../lib/contact-copy.ts";

const key = "fixture-key-123456";
const body = '{ "name":"Synthetic", "email":"fixture@example.invalid", "message":"line 1\\nline 2" }';

async function withEdge(edge, action) {
  const original = globalThis.fetch;
  globalThis.fetch = edge;
  try {
    await action();
  } finally {
    globalThis.fetch = original;
  }
}

test("component delegates contact POST rather than fetching directly", async () => {
  const source = await readFile(new URL("../components/contact/ContactForm.tsx", import.meta.url), "utf8");
  assert.doesNotMatch(source, /\bfetch\s*\(/);
  assert.match(source, /postContactMessage\(current\.key,\s*current\.body\)/);
});

test("single fixed guest POST preserves raw bytes, key and request policy", async () => {
  const calls = [];
  await withEdge(async (...args) => {
    calls.push(args);
    return new Response(null, { status: 503 });
  }, async () => {
    const result = await postContactMessage(key, body);
    assert.equal(result.status, 503);
  });
  assert.equal(calls.length, 1);
  const [url, options] = calls[0];
  assert.equal(url, "/api/buyer/contact-messages");
  assert.deepEqual(options, {
    method: "POST",
    headers: { "Content-Type": "application/json", "Idempotency-Key": key },
    body,
    credentials: "same-origin",
    cache: "no-store",
    redirect: "error",
  });
});

test("network uncertainty has one attempt; explicit recovery uses same bytes and key", async () => {
  const calls = [];
  await withEdge(async (url, options) => {
    calls.push([url, options]);
    if (calls.length === 1) throw new TypeError("network unavailable");
    return Response.json({ message_id: "12345678-1234-1234-1234-123456789abc", status: "received" }, { status: 201 });
  }, async () => {
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
  await withEdge(async () => {
    count++;
    return Response.json({ status: "received" }, { status: 201 });
  }, async () => {
    const response = await postContactMessage(key, body);
    assert.equal(contactReceipt(await response.json()), null);
    assert.equal(count, 1);
  });
});

test("invalid key is rejected before any network call", async () => {
  let count = 0;
  await withEdge(async () => {
    count++;
    throw Error("unexpected network call");
  }, async () => {
    await assert.rejects(postContactMessage("bad", body), /Invalid contact idempotency key/);
    assert.equal(count, 0);
  });
});

test("validation rules enforce name, email, message bounds and honeypot", () => {
  const base = { name: "Buyer", email: "buyer@example.com", message: "Inquiry" };
  assert.ok(validContactBody(base));
  assert.ok(validContactBody({ ...base, order_number: "LC-1001" }));
  assert.ok(validContactBody({ ...base, website: "" }));

  // Required field whitespace or empty
  assert.equal(validContactBody({ ...base, name: "" }), false);
  assert.equal(validContactBody({ ...base, name: "   " }), false);
  assert.equal(validContactBody({ ...base, email: "" }), false);
  assert.equal(validContactBody({ ...base, message: "" }), false);
  assert.equal(validContactBody({ ...base, message: "   \n  " }), false);

  // Bounds
  assert.equal(validContactBody({ ...base, name: "a".repeat(121) }), false);
  assert.ok(validContactBody({ ...base, name: "a".repeat(120) }));
  assert.equal(validContactBody({ ...base, order_number: "x".repeat(65) }), false);
  assert.ok(validContactBody({ ...base, order_number: "x".repeat(64) }));
  assert.equal(validContactBody({ ...base, message: "m".repeat(5001) }), false);
  assert.ok(validContactBody({ ...base, message: "m".repeat(5000) }));

  // Email format & bytes
  assert.equal(validContactBody({ ...base, email: "invalid-email" }), false);
  assert.equal(validContactBody({ ...base, email: "bad@domain with space" }), false);
  assert.equal(validContactBody({ ...base, email: "bad<user>@domain.com" }), false);
  assert.ok(validContactBody({ ...base, email: "valid.user+test@domain.co" }));

  // Honeypot
  assert.equal(validContactBody({ ...base, website: "https://spam.bot" }), false);
});

test("trilingual contact copy has matching keys, DS token style, and does not claim email sent", () => {
  const locales = ["zh-TW", "zh-CN", "en"];
  const requiredKeys = [
    "title", "name", "email", "order", "message", "next", "confirm",
    "send", "edit", "sent", "sentBody", "back", "invalid", "unknown",
    "unknownBody", "recover", "lostBody", "throttle", "conflict", "failed", "storage",
  ];

  for (const locale of locales) {
    const copy = contactCopy[locale];
    assert.ok(copy, `copy must exist for ${locale}`);
    for (const k of requiredKeys) {
      assert.ok(copy[k] && copy[k].trim().length > 0, `copy key ${k} must be non-empty for ${locale}`);
    }
    // "收到201 received才显示收到，不声称已发email"
    assert.doesNotMatch(copy.sentBody, /已發送電子郵件|已发送电子邮件|email has been sent/i);
  }
});

test("ContactStoreButton separates contact intent from after-sales without using testId", async () => {
  const source = await readFile(new URL("../components/privacy/ContactStoreButton.tsx", import.meta.url), "utf8");
  // Ensure routing is decided by explicit prop, not testId
  assert.match(source, /intent/);
  assert.doesNotMatch(source, /testId\s*===/);
  assert.match(source, /\/\[locale\]\/contact`|\/contact`/);
});

test("OrderStatusView preserves contact intent and the order after-sales return entry", async () => {
  const source = await readFile(new URL("../components/order-status/OrderStatusView.tsx", import.meta.url), "utf8");
  assert.match(source, /<ContactStoreButton[^>]*label=\{c\.contact\}[^>]*intent="contact"/);
  assert.match(source, /<a\b[^>]*data-testid="request-after-sales"[^>]*href=\{`\/\$\{locale\}\/orders\/\$\{order\.order_id\}\/return`\}>\{c\.after\}<\/a>/);
});
