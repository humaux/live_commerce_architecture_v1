// Purpose: actual-source LP2 write transport counterexamples, E3 MOCK only.
// Depends on: existing Node24 TypeScript hook host; stubbed network/browser session only.
// Used by: test-node; no PG, receipt, UI or browser acceptance is claimed.
import test from "node:test";
import assert from "node:assert/strict";
import { environment } from "./inbox-review-host.test.ts";
const { createMember, adjustPoints, MemberWriteError, searchMembers } =
  await import("../../apps/admin/lib/members-client.ts");
const sid = "11111111-1111-4111-8111-111111111111",
  mid = "22222222-2222-4222-8222-222222222222",
  eid = "33333333-3333-4333-8333-333333333333";
const key = "stable-key:123",
  stamp = "2026-11-09T00:00:00Z";
const input = () => ({
  display_name: "  e\u0301  ",
  contact_note: "PRIVATE_CONTACT",
  notes: null,
});
const member = () => ({
  id: mid,
  display_name: "[erased]",
  contact_note: null,
  notes: null,
  balance: 0,
  version: 2,
  created_at: stamp,
  updated_at: stamp,
  erased_at: stamp,
});
const adjustment = () => ({
  entry: {
    id: eid,
    member_id: mid,
    delta: 2,
    reason: "manual_adjust",
    order_id: null,
    note: null,
    actor_principal_id: sid,
    created_at: stamp,
  },
  balance: 2,
});
const response = (value: unknown, status = 201, headers = {}) =>
  new Response(JSON.stringify(value), {
    status,
    headers: {
      "Content-Type": "application/json",
      "Cache-Control": "private, no-store",
      ...headers,
    },
  });
const signal = () => new AbortController().signal;
const error = (code: string, outcome: string) => (e: unknown) =>
  e instanceof MemberWriteError &&
  e.code === code &&
  e.outcome === outcome &&
  !/PRIVATE|stable-key/.test(String(e));
function deferred<T>() {
  let resolve!: (v: T) => void;
  const promise = new Promise<T>((r) => {
    resolve = r;
  });
  return { promise, resolve };
}
test("caller key and CSRF are sent exactly once with private body snapshot", async (t) => {
  environment(t);
  const entered = deferred<void>(),
    held = deferred<Response>();
  let calls = 0;
  let request: any;
  globalThis.fetch = async (url, options) => {
    calls++;
    request = { url, ...options };
    entered.resolve();
    return held.promise;
  };
  const body = input(),
    pending = createMember(sid, body, key, signal());
  body.display_name = "CHANGED_PRIVATE";
  body.contact_note = "CHANGED_CONTACT";
  await entered.promise;
  assert.equal(
    request.headers["X-CSRF-Token"],
    "a".repeat(43),
    "write requires CSRF",
  );
  assert.equal(
    request.headers["Idempotency-Key"],
    key,
    "write requires caller key",
  );
  assert.equal(request.method, "POST");
  assert.equal(request.credentials, "same-origin");
  assert.equal(request.cache, "no-store");
  assert.deepEqual(JSON.parse(request.body), {
    display_name: "é",
    contact_note: "PRIVATE_CONTACT",
    notes: null,
  });
  assert.equal(request.url, `/api/stores/${sid}/members`);
  assert.equal(request.url.includes("PRIVATE"), false);
  held.resolve(response({ member: member() }, 200));
  assert.deepEqual(await pending, { member: member() });
  assert.equal(calls, 1);
});
test("adjust uses actual flat Go DTO and frozen target/body; erasure replay note may be null", async (t) => {
  environment(t);
  let request: any;
  globalThis.fetch = async (url, options) => {
    request = { url, ...options };
    return response(adjustment());
  };
  const body = { delta: 2, note: "PRIVATE_NOTE", order_id: null };
  const pending = adjustPoints(sid, mid, body, key, signal());
  body.delta = 7;
  body.note = "CHANGED";
  assert.deepEqual(await pending, adjustment());
  assert.equal(request.url, `/api/stores/${sid}/members/${mid}/adjustments`);
  assert.deepEqual(JSON.parse(request.body), {
    delta: 2,
    note: "PRIVATE_NOTE",
    order_id: null,
  });
  for (const patch of [
    { member_id: sid },
    { actor_principal_id: null },
    { delta: 0 },
    { order_id: eid },
    { reason: "manual_order_award" },
  ]) {
    globalThis.fetch = async () =>
      response({ ...adjustment(), entry: { ...adjustment().entry, ...patch } });
    await assert.rejects(
      adjustPoints(sid, mid, { delta: 2, note: "note" }, key, signal()),
      error("unknown", "unknown"),
    );
  }
});
test("invalid keys/UUID/normalized text/body rejected before dispatch", async (t) => {
  environment(t);
  let calls = 0;
  globalThis.fetch = async () => {
    calls++;
    throw Error();
  };
  for (const k of ["", "short", "x".repeat(129), "PRIVATE\nKEY"])
    assert.throws(
      () => createMember(sid, input(), k, signal()),
      error("invalid_request", "not-sent"),
    );
  for (const name of [" ", "x".repeat(61), "name\nline", "\ud800", "\ufeffx"])
    assert.throws(
      () =>
        createMember(sid, { ...input(), display_name: name }, key, signal()),
      error("invalid_request", "not-sent"),
    );
  assert.throws(
    () => createMember("PRIVATE", input(), key, signal()),
    MemberWriteError,
  );
  assert.throws(
    () =>
      createMember(sid, { ...input(), notes: "x".repeat(1001) }, key, signal()),
    MemberWriteError,
  );
  for (const delta of [0, 1.5, NaN, 1000001])
    assert.throws(
      () => adjustPoints(sid, mid, { delta, note: "note" }, key, signal()),
      MemberWriteError,
    );
  assert.throws(
    () => adjustPoints(sid, mid, { delta: 2, note: "\n" }, key, signal()),
    MemberWriteError,
  );
  assert.equal(calls, 0);
});
test("missing cookie and pre-abort send no mutation", async (t) => {
  const env = environment(t);
  let calls = 0;
  globalThis.fetch = async () => {
    calls++;
    throw Error();
  };
  env.document.cookie = "";
  await assert.rejects(
    createMember(sid, input(), key, signal()),
    error("signed-out", "not-sent"),
  );
  env.document.cookie = `__Host-commerce_csrf=${"a".repeat(43)}`;
  const controller = new AbortController();
  controller.abort();
  await assert.rejects(
    createMember(sid, input(), key, controller.signal),
    (e) => e instanceof MemberWriteError && e.outcome === "not-sent",
  );
  assert.equal(calls, 0);
});
test("network exception is UNKNOWN without retry or private error text", async (t) => {
  environment(t);
  let calls = 0;
  globalThis.fetch = async () => {
    calls++;
    throw new Error("PRIVATE_NETWORK");
  };
  await assert.rejects(
    createMember(sid, input(), key, signal()),
    error("unknown", "unknown"),
  );
  assert.equal(calls, 1);
});
test("late session success is UNKNOWN and signals privacy logout", async (t) => {
  const env = environment(t),
    entered = deferred<void>(),
    held = deferred<Response>();
  let logouts = 0;
  env.window.addEventListener("commerce-session-logout", () => logouts++);
  globalThis.fetch = async () => {
    entered.resolve();
    return held.promise;
  };
  const pending = createMember(sid, input(), key, signal());
  await entered.promise;
  env.document.cookie = `__Host-commerce_csrf=${"b".repeat(43)}`;
  held.resolve(response({ member: member() }));
  await assert.rejects(pending, error("signed-out", "unknown"));
  assert.ok(logouts > 0);
});
test("abort while body stalls settles UNKNOWN even if mock ignores fetch abort", async (t) => {
  environment(t);
  const entered = deferred<void>(),
    controller = new AbortController();
  let canceled = false;
  globalThis.fetch = async () =>
    new Response(
      new ReadableStream(
        {
          pull() {
            entered.resolve();
          },
          cancel() {
            canceled = true;
          },
        },
        { highWaterMark: 0 },
      ),
      {
        status: 201,
        headers: {
          "Content-Type": "application/json",
          "Cache-Control": "private, no-store",
        },
      },
    );
  const pending = createMember(sid, input(), key, controller.signal);
  await entered.promise;
  controller.abort();
  await assert.rejects(pending, error("unknown", "unknown"));
  assert.ok(canceled);
});
test("late body session and invalid MIME/cache/JSON/shape/size cannot report success", async (t) => {
  const env = environment(t);
  for (const r of [
    response({}),
    response({ member: member() }, 201, { "Cache-Control": "public" }),
    response({}, 201, { "Content-Type": "text/plain" }),
    new Response("{", {
      status: 201,
      headers: {
        "Content-Type": "application/json",
        "Cache-Control": "private, no-store",
      },
    }),
    response({ padding: "x".repeat(1048576) }),
  ]) {
    globalThis.fetch = async () => r;
    await assert.rejects(
      createMember(sid, input(), key, signal()),
      error("unknown", "unknown"),
    );
  }
  globalThis.fetch = async () =>
    new Response(
      new ReadableStream({
        pull(c) {
          env.document.cookie = `__Host-commerce_csrf=${"b".repeat(43)}`;
          c.enqueue(
            new TextEncoder().encode(JSON.stringify({ member: member() })),
          );
          c.close();
        },
      }),
      {
        status: 201,
        headers: {
          "Content-Type": "application/json",
          "Cache-Control": "private, no-store",
        },
      },
    );
  await assert.rejects(
    createMember(sid, input(), key, signal()),
    error("signed-out", "unknown"),
  );
});
test("only whitelisted refusal status/code pairs settle rejection; private messages never escape", async (t) => {
  environment(t);
  for (const [status, code, want] of [
    [401, "unauthorized", "signed-out"],
    [403, "forbidden", "forbidden"],
    [404, "not_found", "not-found"],
    [409, "points_insufficient", "points_insufficient"],
    [409, "member_erased", "member_erased"],
    [409, "idempotency_conflict", "idempotency_conflict"],
    [409, "conflict", "conflict"],
    [422, "invalid_request", "invalid_request"],
    [422, "limit_reached", "limit_reached"],
  ] as const) {
    globalThis.fetch = async () =>
      response({ code, message: "PRIVATE_ERROR" }, status, {
        "Cache-Control": "no-store",
      });
    await assert.rejects(
      createMember(sid, input(), key, signal()),
      error(want, "rejected"),
    );
  }
  for (const [status, code] of [
    [409, "PRIVATE_ERROR"],
    [409, "limit_reached"],
    [422, "member_erased"],
    [503, "retry_later"],
    [202, "ok"],
  ] as const) {
    globalThis.fetch = async () => response({ code }, status);
    await assert.rejects(
      createMember(sid, input(), key, signal()),
      error("unknown", "unknown"),
    );
  }
});
test("search remains read-only POST with no caller key or private URL term", async (t) => {
  environment(t);
  let request: any;
  globalThis.fetch = async (url, options) => {
    request = { url, ...options };
    return response({ members: [], next_cursor: null }, 200);
  };
  await searchMembers(sid, "PRIVATE_QUERY", null, signal());
  assert.equal(request.headers["Idempotency-Key"], undefined);
  assert.ok(!request.url.includes("PRIVATE"));
});
