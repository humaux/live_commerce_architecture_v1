// Purpose: LC-U3 actual browser transport/session/CSRF receipt counterexamples.
// Depends on: real create-order-client and settings-client helpers; only fetch network is faked.
// Used by: focused Node gate; no real cookies, PII or provider calls.
import assert from "node:assert/strict";
import { registerHooks } from "node:module";
import { test } from "node:test";
registerHooks({
  resolve(specifier, context, next) {
    if (
      specifier.startsWith(".") &&
      !context.parentURL?.includes("/node_modules/") &&
      !/\.[a-z]+$/.test(specifier)
    )
      return next(specifier + ".ts", context);
    return next(specifier, context);
  },
});
const { readOrderPrefill, createForBuyerOrder, recoverForBuyerOrder, recoverPendingOrder, CreateOrderError } =
  await import("../../apps/admin/lib/create-order-client.ts");
const { sessionBoundary } =
  await import("../../apps/admin/lib/settings-client.ts");
const { placeManualOrder } =
  await import("../../apps/admin/lib/merchant-tools-client.ts");
const id = "11111111-1111-4111-8111-111111111111";
let cookie = "A".repeat(43);
Object.defineProperty(globalThis, "document", {
  configurable: true,
  value: {
    get cookie() {
      return `__Host-commerce_csrf=${cookie}`;
    },
  },
});
const prefill = {
  items: [],
  bundles: [],
  customer: null,
  last_delivery: null,
  suggested_option_key: null,
  live_price_eligible: false,
  live_price_reason: "no_conversation",
};
const body = JSON.stringify({
  items: [{ sku_id: id, quantity: 1 }],
  customer: { name: "Synthetic buyer", phone: "0912345678", email: "" },
  delivery: {
    option_key: `${id}|TW|home`,
    home_address: {
      region: "",
      city: "City",
      postal_code: "",
      line1: "Address",
      line2: "",
    },
    cvs: null,
  },
  payment_mode: "bank_transfer",
  locale: "en",
  for: { bundle_ids: [id], conversation_id: null },
  send_payment_link: false,
});
const value = {
  order_id: id,
  commercial_state: "AWAITING_TRANSFER",
  payment_mode: "bank_transfer",
  total_minor: 400,
  currency: "TWD",
  expires_at: "2030-01-01T00:00:00Z",
  buyer_link: null,
  link_state: "configured",
  source: "merchant_manual",
  live_price: "not_applied",
  live_price_reason: "no_conversation",
  send: { state: "not_sent", reason: "not_requested" },
};
const reply = (payload: unknown, status = 200) =>
  Response.json(payload, { status, headers: { "Cache-Control": "no-store" } });
test("PR32 a changed or missing pre-dispatch CSRF is explicitly never-sent for both order transports", async () => {
  cookie = "A".repeat(43);
  const boundary = await sessionBoundary();
  let calls = 0;
  globalThis.fetch = async () => { calls++; return reply({ code: "retry_later" }, 503); };
  for (const nextCookie of ["B".repeat(43), ""]) {
    cookie = nextCookie;
    await assert.rejects(createForBuyerOrder(id, { key: "synthetic-receipt", body }, boundary), (error: any) => {
      assert.ok(error instanceof CreateOrderError);
      assert.equal(error.code, "unauthorized");
      assert.equal(error.status, 401);
      assert.equal(error.neverSent, true, "the real session fence rejected before fetch");
      return true;
    });
    assert.deepEqual(await placeManualOrder(id, "synthetic-receipt", JSON.parse(body), boundary),
      { ok: false, code: "unauthorized", uncertain: false, status: 401, neverSent: true });
  }
  assert.equal(calls, 0, "neither command was dispatched");
  cookie = "A".repeat(43);
});
test("PR32 an async pre-dispatch CSRF rotation is never-sent, but a post-fetch fence loss is not", async t => {
  cookie = "A".repeat(43);
  const boundary = await sessionBoundary();
  let calls = 0;
  globalThis.fetch = async () => { calls++; return reply(value); };
  const digest = crypto.subtle.digest.bind(crypto.subtle);
  const mock = t.mock.method(crypto.subtle, "digest", async (...args: Parameters<typeof digest>) => {
    const result = await digest(...args); cookie = "B".repeat(43); return result;
  });
  await assert.rejects(createForBuyerOrder(id, { key: "synthetic-receipt", body }, boundary), (error: any) => {
    assert.ok(error instanceof CreateOrderError); assert.equal(error.neverSent, true); return true;
  });
  cookie = "A".repeat(43);
  assert.deepEqual(await placeManualOrder(id, "synthetic-receipt", JSON.parse(body), boundary),
    { ok: false, code: "unauthorized", uncertain: false, status: 401, neverSent: true });
  assert.equal(calls, 0);
  mock.mock.restore(); cookie = "A".repeat(43);
  globalThis.fetch = async () => {
    calls++; const r = reply(value);
    r.json = async () => { cookie = "B".repeat(43); return value; };
    return r;
  };
  await assert.rejects(createForBuyerOrder(id, { key: "synthetic-receipt", body }, boundary), (error: any) => {
    assert.ok(error instanceof CreateOrderError); assert.equal(error.neverSent, false, "the request may have committed before privacy loss"); return true;
  });
  assert.equal(calls, 1); cookie = "A".repeat(43);
});
test("PR32 static validation is explicitly never-sent, unlike a post-dispatch refusal", async () => {
  cookie = "A".repeat(43);
  const boundary = await sessionBoundary();
  let calls = 0;
  globalThis.fetch = async () => { calls++; return reply({ code: "invalid_request" }, 422); };
  for (const [store, attempt] of [
    [id, { key: "bad", body }],
    [id, { key: "synthetic-receipt", body: "{" }],
    [id, { key: "synthetic-receipt", body: body.replace('"quantity":1', '"price_minor":1') }],
    ["bad", { key: "synthetic-receipt", body }],
  ] as const) {
    await assert.rejects(createForBuyerOrder(store, attempt, boundary), (error: any) => {
      assert.ok(error instanceof CreateOrderError);
      assert.equal(error.code, "invalid_request");
      assert.equal(error.status, 422);
      assert.equal(error.neverSent, true, "only actual local validation proves zero dispatch");
      return true;
    });
  }
  assert.equal(calls, 0);
  const sent = await createForBuyerOrder(id, { key: "synthetic-receipt", body }, boundary);
  assert.equal(calls, 1);
  assert.deepEqual(sent, { ok: false, code: "invalid_request", status: 422, uncertain: false });
});
test("authoritative recovery sends the frozen receipt through the real session/CSRF seam and only accepts closed 200 results", async () => {
  cookie = "A".repeat(43);
  const boundary = await sessionBoundary(), attempt = { key: "synthetic-receipt", body };
  let calls = 0;
  for (const result of [{ state: "not_created" }, { state: "created", order_id: id }]) {
    globalThis.fetch = async (input, init) => {
      calls++;
      assert.equal(String(input), `/api/stores/${id}/tools/orders/for-buyer/recover`);
      assert.equal(init!.body, body);
      assert.equal(new Headers(init!.headers).get("Idempotency-Key"), attempt.key);
      assert.equal(new Headers(init!.headers).get("X-CSRF-Token"), cookie);
      assert.equal(init!.credentials, "same-origin"); assert.equal(init!.cache, "no-store");
      assert.equal(init!.referrerPolicy, "no-referrer"); assert.equal(init!.redirect, "error"); assert.ok(init!.signal);
      return reply(result);
    };
    assert.deepEqual(await recoverForBuyerOrder(id, attempt, boundary), { ok: true, status: 200, value: result });
  }
  for (const response of [reply({ state: "not_created" }, 201), reply({ state: "not_created", order_id: id }), reply({ state: "created" }), reply({ state: "pending" }), new Response('{"state":"not_created"}', { headers: { "content-type": "application/json" } })]) {
    globalThis.fetch = async () => { calls++; return response; };
    assert.deepEqual(await recoverForBuyerOrder(id, attempt, boundary), { ok: false, code: "retry_later", status: 503, uncertain: true });
  }
  globalThis.fetch = async () => { calls++; throw new Error("PRIVATE_DIAGNOSTIC"); };
  assert.deepEqual(await recoverForBuyerOrder(id, attempt, boundary), { ok: false, code: "retry_later", status: 503, uncertain: true });
  assert.equal(calls, 8, "one network request per explicit action, including lost recovery responses");
  globalThis.fetch = async () => { calls++; const r = reply({ state: "not_created" }); r.json = async () => { cookie = "B".repeat(43); return { state: "not_created" }; }; return r; };
  await assert.rejects(recoverForBuyerOrder(id, attempt, boundary), e => e instanceof CreateOrderError && e.code === "unauthorized");
  cookie = "A".repeat(43);
});
test("recovery transport auth loss outranks malformed private error bodies", async () => {
  cookie = "A".repeat(43);
  const boundary = await sessionBoundary();
  for (const status of [401, 403]) {
    globalThis.fetch = async () => new Response("PRIVATE_DIAGNOSTIC", { status });
    assert.deepEqual(await recoverForBuyerOrder(id, { key: "synthetic-receipt", body }, boundary),
      { ok: false, code: status === 401 ? "unauthorized" : "forbidden", status, uncertain: false });
  }
});
test("server pending recovery locates the authenticated intent with an empty request and recovery-only keys", async () => {
  cookie = "A".repeat(43); const boundary = await sessionBoundary(); const keys: string[] = [];
  for (const result of [{ state: "none" }, { state: "not_created" }, { state: "created", order_id: id }]) {
    const recoveryKey = crypto.randomUUID();
    globalThis.fetch = async (input, init) => {
      assert.equal(String(input), `/api/stores/${id}/tools/orders/attempt/recover`);
      assert.equal(init!.method, "POST"); assert.equal(init!.body, "{}", "no lost browser body/key or buyer fields are reconstructed");
      const headers = new Headers(init!.headers), key = headers.get("Idempotency-Key")!;
      assert.match(key, /^[0-9a-f-]{36}$/); assert.equal(headers.get("X-CSRF-Token"), cookie); keys.push(key);
      assert.equal(init!.credentials, "same-origin"); assert.equal(init!.cache, "no-store"); assert.equal(init!.redirect, "error"); assert.equal(init!.referrerPolicy, "no-referrer");
      return reply(result);
    };
    assert.deepEqual(await recoverPendingOrder(id, recoveryKey, boundary), { ok: true, status: 200, value: result });
    assert.equal(keys.at(-1), recoveryKey, "caller owns the recovery transport receipt");
  }
  assert.equal(new Set(keys).size, 3, "a recovery-only key never becomes a replacement order key");
  for (const invalid of [{ state: "none", order_id: id }, { state: "none", key: "private" }, { state: "pending" }, { state: "created" }]) {
    globalThis.fetch = async () => reply(invalid);
    assert.deepEqual(await recoverPendingOrder(id, "synthetic-recovery-transport", boundary), { ok: false, code: "retry_later", status: 503, uncertain: true });
  }
  let dispatched = 0; globalThis.fetch = async () => { dispatched++; return reply({ state: "none" }); };
  cookie = "B".repeat(43);
  await assert.rejects(recoverPendingOrder(id, "synthetic-recovery-transport", boundary), error => error instanceof CreateOrderError && error.neverSent === true && error.status === 401);
  assert.equal(dispatched, 0);
  cookie = "A".repeat(43);
  globalThis.fetch = async () => { dispatched++; cookie = "B".repeat(43); return reply({ state: "none" }); };
  await assert.rejects(recoverPendingOrder(id, "synthetic-recovery-transport", boundary), error => error instanceof CreateOrderError && error.neverSent === false && error.status === 401);
  assert.equal(dispatched, 1, "post-response privacy rejection cannot prove prior order absence"); cookie = "A".repeat(43);
});
test("generic recovery lost response retries the caller's same transport key instead of discovering a different intent", async () => {
  cookie = "A".repeat(43);const boundary = await sessionBoundary(), recoveryKey = "synthetic-pending-recovery-transport";
  const requests: { key: string | null; body: string }[] = [];
  globalThis.fetch = async (_input, init) => {
    requests.push({ key: new Headers(init!.headers).get("Idempotency-Key"), body: String(init!.body) });
    if (requests.length === 1) throw new Error("SYNTHETIC_LOST_RECOVERY_RESPONSE");
    return reply({ state: "created", order_id: id });
  };
  assert.deepEqual(await recoverPendingOrder(id, recoveryKey, boundary), { ok: false, code: "retry_later", status: 503, uncertain: true });
  assert.equal(requests.length, 1, "recovery never retries automatically");
  assert.deepEqual(await recoverPendingOrder(id, recoveryKey, boundary), { ok: true, status: 200, value: { state: "created", order_id: id } });
  assert.deepEqual(requests, [{ key: recoveryKey, body: "{}" }, { key: recoveryKey, body: "{}" }]);
  await assert.rejects(recoverPendingOrder(id, "invalid key", boundary), error => error instanceof CreateOrderError && error.status === 422 && error.neverSent === true);
  assert.equal(requests.length, 2, "invalid pre-dispatch transport cannot create a server recovery record");
});
test("real csrf/session helpers fence both reads and immutable receipt writes", async () => {
  const calls: { url: string; init: RequestInit }[] = [];
  globalThis.fetch = async (input, init) => {
    calls.push({ url: String(input), init: init! });
    return reply(init?.method === "POST" ? value : prefill);
  };
  const boundary = await sessionBoundary();
  assert.deepEqual(await readOrderPrefill(id, { bundleId: id }), prefill);
  const attempt = { key: "synthetic-receipt", body };
  assert.equal((await createForBuyerOrder(id, attempt, boundary)).ok, true);
  assert.equal((await createForBuyerOrder(id, attempt, boundary)).ok, true);
  for (const c of calls.slice(1)) {
    assert.equal(c.url, `/api/stores/${id}/tools/orders/for-buyer`);
    assert.equal(c.init.body, body);
    assert.equal(
      new Headers(c.init.headers).get("Idempotency-Key"),
      attempt.key,
    );
    assert.equal(new Headers(c.init.headers).get("X-CSRF-Token"), cookie);
    assert.equal(c.init.cache, "no-store");
    assert.equal(c.init.referrerPolicy, "no-referrer");
    assert.equal(c.init.credentials, "same-origin");
    assert.ok(c.init.signal);
  }
  const before = calls.length;
  await assert.rejects(
    createForBuyerOrder(id, attempt, "bad"),
    CreateOrderError,
  );
  await assert.rejects(
    createForBuyerOrder(
      id,
      { ...attempt, body: body.replace('"quantity":1', '"price_minor":1') },
      boundary,
    ),
    CreateOrderError,
  );
  assert.equal(calls.length, before);
});
test("unknown network/body/status is uncertain, no auto retry, no diagnostic reflected", async () => {
  const boundary = await sessionBoundary();
  let calls = 0;
  globalThis.fetch = async () => {
    calls++;
    throw new Error("PRIVATE_DIAGNOSTIC");
  };
  const network = await createForBuyerOrder(
    id,
    { key: "synthetic-receipt", body },
    boundary,
  );
  assert.deepEqual(network, {
    ok: false,
    code: "retry_later",
    status: 503,
    uncertain: true,
  });
  assert.equal(calls, 1);
  for (const response of [
    reply({ code: "PRIVATE_DIAGNOSTIC" }, 409),
    reply({ secret: "PRIVATE_DIAGNOSTIC" }),
    new Response("bad", { status: 503 }),
  ]) {
    globalThis.fetch = async () => response;
    const result = await createForBuyerOrder(
      id,
      { key: "synthetic-receipt", body },
      boundary,
    );
    assert.equal(result.ok, false);
    if (!result.ok) assert.equal(result.uncertain, true);
    assert.equal(JSON.stringify(result).includes("PRIVATE_DIAGNOSTIC"), false);
  }
  globalThis.fetch = async () =>
    reply(
      {
        code: "bundle_already_ordered",
        details: { order_id: id, secret: "PRIVATE_DIAGNOSTIC" },
      },
      409,
    );
  assert.deepEqual(
    await createForBuyerOrder(id, { key: "synthetic-receipt", body }, boundary),
    {
      ok: false,
      code: "bundle_already_ordered",
      status: 409,
      uncertain: false,
      order_id: id,
    },
  );
});
test("logout during response.json discards private reads and successful write results", async () => {
  cookie = "A".repeat(43);
  const boundary = await sessionBoundary();
  for (const write of [false, true]) {
    cookie = "A".repeat(43);
    globalThis.fetch = async () => {
      const r = reply(write ? value : prefill);
      r.json = async () => {
        cookie = "B".repeat(43);
        return write ? value : prefill;
      };
      return r;
    };
    await assert.rejects(
      write
        ? createForBuyerOrder(id, { key: "synthetic-receipt", body }, boundary)
        : readOrderPrefill(id, { bundleId: id }),
      (e) => e instanceof CreateOrderError && e.code === "unauthorized",
    );
  }
  cookie = "A".repeat(43);
});

test("blocklist warning uses real session fences, closed booleans and every known bundle", async () => {
  cookie = "A".repeat(43);
  const { readOrderRestricted } = await import("../../apps/admin/lib/create-order-client.ts");
  const other = "22222222-2222-4222-8222-222222222222";
  const calls: string[] = [];
  globalThis.fetch = async (url, init) => { calls.push(String(url)); assert.equal(init?.method, "GET"); assert.equal(init?.cache, "no-store"); assert.equal(init?.referrerPolicy, "no-referrer"); return reply({ restricted: String(url).endsWith(other) }); };
  assert.equal(await readOrderRestricted(id, id, [id, other]), true);
  assert.equal(calls.length, 2);
  assert.ok(calls.every(u => u.startsWith(`/api/stores/${id}/live-sessions/${id}/claims/blocklist/check?bundle_id=`)));
  for (const value of [{ restricted: "false" }, { restricted: false, note: "PRIVATE_NOTE" }, null]) {
    globalThis.fetch = async () => reply(value);
    await assert.rejects(readOrderRestricted(id, id, [id]), /unavailable/);
  }
  globalThis.fetch = async () => reply({ restricted: false });
  assert.equal(await readOrderRestricted(id, id, [id]), false);
  globalThis.fetch = async () => new Response('{"restricted":true}', { status: 200, headers: { "cache-control": "no-store", "content-type": "application/json" } });
  const responseFetch = globalThis.fetch;
  globalThis.fetch = async (...args) => { const r = await responseFetch(...args); cookie = "B".repeat(43); return r; };
  await assert.rejects(readOrderRestricted(id, id, [id]), /unauthorized/);
  cookie = "A".repeat(43);
  // A prior restricted result cannot preserve private UI after any later permission failure.
  for (const status of [401, 403]) {
    globalThis.fetch = async (url) => reply({ restricted: true }, String(url).endsWith(other) ? status : 200);
    await assert.rejects(readOrderRestricted(id, id, [id, other]), /unauthorized/);
  }
  globalThis.fetch = async (url) => reply({ restricted: true }, String(url).endsWith(other) ? 503 : 200);
  assert.equal(await readOrderRestricted(id, id, [id, other]), true);
  globalThis.fetch = async (url) => reply({ restricted: false }, String(url).endsWith(other) ? 503 : 200);
  await assert.rejects(readOrderRestricted(id, id, [id, other]), /unavailable/);
  for (const revoke of ["cookie", "abort"]) {
    const duringParse = new AbortController();
    globalThis.fetch = async () => {
      const response = reply({ restricted: true });
      response.json = async () => {
        if (revoke === "cookie") cookie = "B".repeat(43);
        else duringParse.abort();
        return { restricted: true };
      };
      return response;
    };
    await assert.rejects(readOrderRestricted(id, id, [id], duringParse.signal));
    cookie = "A".repeat(43);
  }
  const abort = new AbortController(); abort.abort();
  await assert.rejects(readOrderRestricted(id, id, [id], abort.signal));
});
