import assert from "node:assert/strict";
import { registerHooks } from "node:module";
import { test, type TestContext } from "node:test";

// Node runs the real browser client with native TS transformation. Resolve only
// this app's extensionless local imports; no replacement client or fetch helper.
const lib = new URL("../../apps/admin/lib/", import.meta.url).href;
registerHooks({ resolve(specifier, context, nextResolve) {
  if (context.parentURL?.startsWith(lib) && specifier.startsWith("./") && !specifier.endsWith(".ts"))
    return nextResolve(`${specifier}.ts`, context);
  return nextResolve(specifier, context);
} });
const { requestStudioInputToken, startStudioInput, stopStudioRehearsal, StudioError } =
  await import("../../apps/admin/lib/studio-client.ts");
const { sessionBoundary } = await import("../../apps/admin/lib/settings-client.ts");

const store = "11111111-1111-4111-8111-111111111111";
const session = "22222222-2222-4222-8222-222222222222";
const attempt = "33333333-3333-4333-8333-333333333333";
const authorization = "44444444-4444-4444-8444-444444444444";
const key = "55555555-5555-4555-8555-555555555555";
const csrf = "c".repeat(43);
const base = `/api/stores/${store}/live-sessions/${session}`;
const token = {
  attempt_id: attempt, room_name: `lc_${attempt.replaceAll("-", "")}`,
  publisher_identity: "lcp_66666666666646668666666666666666",
  url: "wss://127.0.0.1:7880", token: "synthetic.payload.signature", expires_at: 4102444800,
};
const receipt = { session_id: session, attempt_id: attempt, state: "READY" };
function response(body: unknown, status = 200, headers?: Record<string, string>) {
  return new Response(JSON.stringify(body), { status, headers: {
    "content-type": "application/json", "cache-control": "private, no-store", ...headers,
  } });
}
async function fixture(t: TestContext) {
  const original = Object.getOwnPropertyDescriptor(globalThis, "document");
  const document = { cookie: `__Host-commerce_csrf=${csrf}` };
  Object.defineProperty(globalThis, "document", { configurable: true, value: document });
  t.after(() => {
    if (original) Object.defineProperty(globalThis, "document", original);
    else Reflect.deleteProperty(globalThis, "document");
  });
  return { document, boundary: await sessionBoundary() };
}
function errorIs(code: string) {
  return (error: unknown) => error instanceof StudioError && error.code === code && error.message === code;
}

test("input start and token use exact scoped POST, CSRF, key, private transport and real session boundary", async (t) => {
  const { boundary } = await fixture(t);
  const calls: { path: unknown; init: RequestInit | undefined }[] = [];
  t.mock.method(globalThis, "fetch", async (path: unknown, init?: RequestInit) => {
    calls.push({ path, init });
    return response(String(path).endsWith("/token") ? token : receipt);
  });
  assert.deepEqual(await startStudioInput(store, session, authorization, 7, key, boundary), receipt);
  // Avoid printing a credential-bearing object on assertion failure.
  const result = await requestStudioInputToken(store, session, attempt, 7, key, boundary);
  assert.equal(result.attempt_id, attempt);
  assert.equal(result.token === token.token, true);
  await stopStudioRehearsal(store, session, attempt, key, boundary);
  assert.deepEqual(calls.map((call) => call.path), [`${base}/input/start`, `${base}/input/token`, `${base}/rehearsal/stop`]);
  for (const { init } of calls) {
    assert.equal(init?.method, "POST");
    assert.equal(init?.credentials, "same-origin");
    assert.equal(init?.cache, "no-store");
    assert.equal(init?.signal instanceof AbortSignal, true);
    assert.deepEqual(init?.headers, { "Content-Type": "application/json", "X-CSRF-Token": csrf, "Idempotency-Key": key });
  }
  assert.deepEqual(calls.map((call) => JSON.parse(call.init?.body as string)), [
    { authorization_id: authorization, expected_session_version: 7 },
    { attempt_id: attempt, expected_session_version: 7 }, { attempt_id: attempt },
  ]);
});

test("input client refuses missing, duplicated or replaced login before dispatch", async (t) => {
  const { document, boundary } = await fixture(t);
  const fetch = t.mock.method(globalThis, "fetch", async () => response(token));
  for (const cookie of ["", `__Host-commerce_csrf=${csrf}; __Host-commerce_csrf=${csrf}`,
    `__Host-commerce_csrf=${"d".repeat(43)}`]) {
    document.cookie = cookie;
    await assert.rejects(requestStudioInputToken(store, session, attempt, 7, key, boundary), errorIs("signed-out"));
    await assert.rejects(startStudioInput(store, session, authorization, 7, key, boundary), errorIs("signed-out"));
  }
  assert.equal(fetch.mock.callCount(), 0);
});

test("input token is not delivered when login changes while the response is in flight", async (t) => {
  const { document, boundary } = await fixture(t);
  const fetch = t.mock.method(globalThis, "fetch", async () => {
    document.cookie = `__Host-commerce_csrf=${"d".repeat(43)}`;
    return response(token);
  });
  await assert.rejects(requestStudioInputToken(store, session, attempt, 7, key, boundary), errorIs("signed-out"));
  assert.equal(fetch.mock.callCount(), 1);
});

test("input token rejects wrong attempt, expired grants and non-allowlisted responses without leaking body", async (t) => {
  const { boundary } = await fixture(t);
  const wrongAttempt = { ...token, attempt_id: authorization, room_name: `lc_${authorization.replaceAll("-", "")}` };
  let body: unknown;
  const fetch = t.mock.method(globalThis, "fetch", async () => response(body));
  const cases = [null, {}, wrongAttempt, { ...token, expires_at: 1 },
    { ...token, expires_at: Math.floor(Date.now() / 1000) },
    { ...token, secret: "never-forward" }, { ...token, url: "wss://untrusted.example:7880" },
    { ...token, token: "not-a-token" }];
  for (body of cases) await assert.rejects(requestStudioInputToken(store, session, attempt, 7, key, boundary), errorIs("uncertain"));
  assert.equal(fetch.mock.callCount(), cases.length);
});

test("input commands preserve UNKNOWN on network, server and malformed success without automatic retries", async (t) => {
  const { boundary } = await fixture(t);
  const actions = [
    () => startStudioInput(store, session, authorization, 7, key, boundary),
    () => requestStudioInputToken(store, session, attempt, 7, key, boundary),
  ];
  let reply: () => Response;
  const fetch = t.mock.method(globalThis, "fetch", async () => reply());
  for (reply of [
    () => { throw new Error("synthetic sensitive transport message"); },
    () => response({ secret: "never-forward" }, 503),
    () => new Response("malformed", { headers: { "content-type": "application/json", "cache-control": "private, no-store" } }),
    () => response(token, 200, { "cache-control": "public" }),
    () => response(token, 200, { "content-type": "text/html" }),
  ]) for (const action of actions) {
    const before = fetch.mock.callCount();
    await assert.rejects(action(), errorIs("uncertain"));
    assert.equal(fetch.mock.callCount(), before + 1);
  }
});

test("input commands preserve explicit authorization errors and bind start receipt to requested session", async (t) => {
  const { boundary } = await fixture(t);
  let status = 200;
  const fetch = t.mock.method(globalThis, "fetch", async () => response({ ...receipt, session_id: authorization }, status));
  await assert.rejects(startStudioInput(store, session, authorization, 7, key, boundary), errorIs("uncertain"));
  for (const [code, expected] of [[401, "signed-out"], [403, "forbidden"], [404, "not-found"], [409, "conflict"], [422, "invalid"]] as const) {
    status = code;
    await assert.rejects(requestStudioInputToken(store, session, attempt, 7, key, boundary), errorIs(expected));
  }
  assert.equal(fetch.mock.callCount(), 6);
});
