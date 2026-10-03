// MOCK transport/journal counterexamples. Not a worker, PG or provider permission acceptance claim.
import assert from "node:assert/strict";
import { test } from "node:test";
import {
  parseAudienceAck,
  parseAudienceJournal,
  audienceStorageKey,
} from "../../apps/admin/lib/attribution-audience.ts";
import { requestAudienceRead } from "../../apps/admin/lib/attribution-client.ts";
import { sessionBoundary } from "../../apps/admin/lib/settings-client.ts";
import { draftID, sessionID } from "./attribution.fixture.ts";
const operation = "44444444-4444-4444-8444-444444444444",
  key = `audience-read-${operation}`;

test("READY is the only acknowledgement; missing/forged operation/state remains unknown", () => {
  assert.deepEqual(
    parseAudienceAck({
      operation_id: operation,
      state: "READY",
      extra: "ignored",
    }),
    { operation_id: operation, state: "READY" },
  );
  for (const v of [
    {},
    null,
    { operation_id: operation, state: "SENT" },
    { operation_id: "invalid", state: "READY" },
    { operation_id: operation },
  ])
    assert.throws(() => parseAudienceAck(v));
});
test("unknown journal retains exact key/boundary and is scoped to store+session; malformed state fails closed", () => {
  const journal = {
    key,
    boundary: "a".repeat(64),
    phase: "unknown",
    operation_id: null,
  };
  assert.deepEqual(parseAudienceJournal(JSON.stringify(journal)), journal);
  assert.notEqual(
    audienceStorageKey(draftID, sessionID),
    audienceStorageKey(sessionID, draftID),
  );
  for (const changed of [
    { ...journal, key: "fresh-key" },
    { ...journal, boundary: "raw-cookie" },
    { ...journal, phase: "completed" },
    { ...journal, phase: "queued" },
    { ...journal, operation_id: operation },
  ])
    assert.throws(() => parseAudienceJournal(JSON.stringify(changed)));
});
test("actual POST transport uses CSRF and session fence; 403 independent of response text", async (t) => {
  const originalDocument = Object.getOwnPropertyDescriptor(
      globalThis,
      "document",
    ),
    originalFetch = globalThis.fetch;
  const fakeDocument = { cookie: `__Host-commerce_csrf=${"A".repeat(43)}` };
  Object.defineProperty(globalThis, "document", {
    value: fakeDocument,
    configurable: true,
  });
  t.after(() => {
    globalThis.fetch = originalFetch;
    if (originalDocument)
      Object.defineProperty(globalThis, "document", originalDocument);
    else Reflect.deleteProperty(globalThis, "document");
  });
  const boundary = await sessionBoundary(),
    sent: {
      url: string;
      key: string | null;
      body: unknown;
      method: string | undefined;
    }[] = [];
  let status = 202,
    value: unknown = { operation_id: operation, state: "READY" };
  globalThis.fetch = async (input, init) => {
    const headers = new Headers(init?.headers);
    sent.push({
      url: String(input),
      key: headers.get("Idempotency-Key"),
      body: init?.body,
      method: init?.method,
    });
    assert.equal(headers.get("X-CSRF-Token"), "A".repeat(43));
    return new Response(JSON.stringify(value), {
      status,
      headers: { "Content-Type": "application/json" },
    });
  };
  assert.equal(
    (await requestAudienceRead(draftID, sessionID, key, boundary)).kind,
    "queued",
  );
  assert.deepEqual(sent[0], {
    url: `/api/stores/${draftID}/ads/sessions/${sessionID}/audience-read`,
    key,
    body: "{}",
    method: "POST",
  });
  status = 403;
  value = { message: "untrusted external prose" };
  assert.equal(
    (await requestAudienceRead(draftID, sessionID, key, boundary)).kind,
    "forbidden",
  );
  status = 503;
  assert.equal(
    (await requestAudienceRead(draftID, sessionID, key, boundary)).kind,
    "unknown",
  );
  assert.equal(
    (await requestAudienceRead(draftID, sessionID, key, boundary)).kind,
    "unknown",
  );
  assert.equal(sent[2].key, sent[3].key);
  status = 202;
  value = { operation_id: operation, state: "COMPLETED" };
  assert.equal(
    (await requestAudienceRead(draftID, sessionID, key, boundary)).kind,
    "unknown",
  );
  const before = sent.length;
  fakeDocument.cookie = `__Host-commerce_csrf=${"B".repeat(43)}`;
  assert.equal(
    (await requestAudienceRead(draftID, sessionID, key, boundary)).kind,
    "signed-out",
  );
  assert.equal(sent.length, before);
});
