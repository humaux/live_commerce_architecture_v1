// INDEPENDENT BFF gate for unit staff-team (contracts/storefront-v2.md §D; docs/delivery/units/staff-team.md): the REAL route
// apps/admin/app/api/team/[action]/route.ts (POST /api/team/{list,invite,revoke-invite,set-role,remove,accept}) driven with Web
// Request objects against a loopback fake of the Go identity upstream (Node, no Next, no browser, no PG).
// Written from the brief and the repository-wide BFF rules (AGENTS.md: scope from server auth only; the password-auth BFF contract
// for Origin + double-submit CSRF), not from the route's internals. Expectations:
//   * browser-origin writes only: exact Origin, double-submit CSRF pair, no query string; every refusal happens BEFORE Go is called;
//   * an owner action without a session is 401; only POST exists;
//   * the request grammar is strict (exact keys, canonical uuids, five roles, three locales, 43-char token) -> 422 locally;
//   * what reaches Go: POST /v1/identity/staff/<action>, the BFF key and the bearer from the HttpOnly cookie, no Cookie/Origin,
//     the body unchanged, exactly one call even when Go fails (an invitation mail must never be sent twice);
//   * Go's answers are rebuilt locally from allow-listed codes: an upstream message (which could echo the token) never reaches
//     the browser, an unknown code or status is a generic 503 retry_later, answers are no-store;
//   * the invitation token is never echoed in any BFF response.
// The module reads process.env once at import, so the environment is set before the dynamic import; "server-only" and the "@/"
// alias are resolved by a registerHooks resolver (no package added), as tests/admin/password-bff.test.ts does for "server-only".
// Run: node --test --experimental-strip-types tests/admin/team-bff.test.ts
import assert from "node:assert/strict";
import { randomBytes } from "node:crypto";
import { createServer } from "node:http";
import { registerHooks } from "node:module";
import type { AddressInfo } from "node:net";
import { fileURLToPath, pathToFileURL } from "node:url";
import { test } from "node:test";

const adminRoot = fileURLToPath(new URL("../../apps/admin/", import.meta.url));
registerHooks({
  resolve(specifier, context, next) {
    if (specifier === "server-only") return { url: "data:text/javascript,", shortCircuit: true };
    if (specifier.startsWith("@/")) {
      const path = adminRoot + specifier.slice(2) + (/\.[a-z]+$/.test(specifier) ? "" : ".ts");
      return { url: pathToFileURL(path).href, shortCircuit: true };
    }
    return next(specifier, context);
  },
});

type Seen = { method: string; url: string; headers: Record<string, string | string[] | undefined>; body: string };
const seen: Seen[] = [];
let answer: { status: number; body: string; type?: string } = { status: 204, body: "" };
const upstream = createServer((req, res) => {
  const chunks: Buffer[] = [];
  req.on("data", (c: Buffer) => chunks.push(c));
  req.on("end", () => {
    seen.push({ method: req.method ?? "", url: req.url ?? "", headers: { ...req.headers }, body: Buffer.concat(chunks).toString() });
    res.writeHead(answer.status, answer.body ? { "content-type": answer.type ?? "application/json" } : {});
    res.end(answer.body);
  });
});
await new Promise<void>((ok) => upstream.listen(0, "127.0.0.1", ok));
const apiOrigin = `http://127.0.0.1:${(upstream.address() as AddressInfo).port}`;
const bffKey = randomBytes(32).toString("base64url");
const publicOrigin = "https://admin.example.test";
Object.assign(process.env, {
  COMMERCE_IDENTITY_ENABLED: "1",
  COMMERCE_PASSWORD_LOGIN_ENABLED: "1",
  COMMERCE_BFF_KEY: bffKey,
  COMMERCE_PUBLIC_ORIGIN: publicOrigin,
  COMMERCE_API_ORIGIN: apiOrigin,
});
const route = await import(pathToFileURL(adminRoot + "app/api/team/[action]/route.ts").href);
test.after(() => upstream.close());

const b64 = () => randomBytes(32).toString("base64url");
const uuid = () => crypto.randomUUID();
const session = b64();
const csrf = b64();
const store = uuid();
const good: Record<string, Record<string, unknown>> = {
  list: { store_id: store },
  invite: { store_id: store, email: "new.staff@example.test", role: "fulfilment", locale: "zh-TW" },
  "revoke-invite": { store_id: store, invite_id: uuid() },
  "set-role": { store_id: store, principal_id: uuid(), role: "viewer" },
  remove: { store_id: store, principal_id: uuid() },
  accept: { token: b64() },
};
const actions = Object.keys(good);

type Over = { origin?: string | null; csrfHeader?: string | null; cookie?: string | null; query?: string; method?: string; body?: unknown; raw?: string; mime?: string };
async function call(action: string, over: Over = {}) {
  const headers = new Headers();
  const origin = over.origin === undefined ? publicOrigin : over.origin;
  if (origin !== null) headers.set("origin", origin);
  const csrfHeader = over.csrfHeader === undefined ? csrf : over.csrfHeader;
  if (csrfHeader !== null) headers.set("x-csrf-token", csrfHeader);
  const cookie = over.cookie === undefined ? `__Host-commerce_session=${session}; __Host-commerce_csrf=${csrf}` : over.cookie;
  if (cookie !== null) headers.set("cookie", cookie);
  headers.set("content-type", over.mime ?? "application/json");
  const method = over.method ?? "POST";
  const text = over.raw ?? JSON.stringify(over.body ?? good[action] ?? {});
  const request = new Request(`${publicOrigin}/api/team/${action}${over.query ?? ""}`, method === "POST" ? { method, headers, body: text } : { method, headers });
  const handler = route[method] as (r: Request, c: { params: Promise<{ action: string }> }) => Promise<Response>;
  const response = await handler(request, { params: Promise.resolve({ action }) });
  return { status: response.status, headers: response.headers, text: await response.text() };
}
const codeOf = (text: string) => (JSON.parse(text) as { code?: string }).code;

test("browser-origin gate: every refusal happens before Go is called, for every action", async () => {
  const before = seen.length;
  const refusals: [string, Over, number][] = [
    ["no Origin header", { origin: null }, 403],
    ["foreign Origin", { origin: "https://evil.example" }, 403],
    ["Origin with a path", { origin: publicOrigin + "/" }, 403],
    ["Origin of another scheme", { origin: "http://admin.example.test" }, 403],
    ["no CSRF header", { csrfHeader: null }, 403],
    ["wrong CSRF header", { csrfHeader: b64() }, 403],
    ["CSRF header not a 32-byte token", { csrfHeader: "short" }, 403],
    ["no CSRF cookie", { cookie: `__Host-commerce_session=${session}` }, 403],
    ["duplicated CSRF cookie", { cookie: `__Host-commerce_session=${session}; __Host-commerce_csrf=${csrf}; __Host-commerce_csrf=${csrf}` }, 403],
    ["query string", { query: "?store=" + store }, 403],
    ["empty query marker with a token", { query: "?token=" + b64() }, 403],
    ["no session cookie", { cookie: `__Host-commerce_csrf=${csrf}` }, 401],
    ["malformed session cookie", { cookie: `__Host-commerce_session=nope; __Host-commerce_csrf=${csrf}` }, 401],
    ["duplicated session cookie", { cookie: `__Host-commerce_session=${session}; __Host-commerce_session=${session}; __Host-commerce_csrf=${csrf}` }, 401],
  ];
  for (const action of actions) {
    for (const [name, over, status] of refusals) {
      const r = await call(action, over);
      assert.equal(r.status, status, `${action}: ${name} -> ${r.status} ${r.text}`);
      assert.match(r.headers.get("content-type") ?? "", /application\/json/, `${action}: ${name}`);
    }
  }
  assert.equal(seen.length, before, "no refused request may reach the Go upstream");
});

test("only POST exists on the team routes", async () => {
  const before = seen.length;
  for (const method of ["GET", "PUT", "PATCH", "DELETE", "OPTIONS"]) {
    const r = await call("list", { method });
    assert.equal(r.status, 405, method);
  }
  assert.equal(seen.length, before);
});

test("unknown action is a local 404 and never reaches Go", async () => {
  const before = seen.length;
  for (const action of ["members", "list/../x", "LIST", "", "transfer-ownership", "set_role"]) {
    const r = await call(action, { body: {} });
    assert.equal(r.status, 404, action);
  }
  assert.equal(seen.length, before);
});

test("strict request grammar: wrong keys, types, ids, roles, locales and token shapes are 422 locally", async () => {
  const before = seen.length;
  const bad: [string, Record<string, unknown> | string][] = [
    ["list", { store_id: "not-a-uuid" }],
    ["list", { store_id: store, tenant_id: uuid() }], // a client-chosen tenant is never accepted
    ["list", {}],
    ["list", { store_id: store.toUpperCase() }],
    ["invite", { ...good.invite, role: "superuser" }],
    ["invite", { ...good.invite, role: "OWNER" }],
    ["invite", { ...good.invite, locale: "fr" }],
    ["invite", { ...good.invite, email: "no-at-sign" }],
    ["invite", { ...good.invite, email: 7 }],
    ["invite", { ...good.invite, tenant_id: uuid() }],
    ["invite", { store_id: store, email: "a@b.test", role: "viewer" }],
    ["revoke-invite", { store_id: store, invite_id: "x" }],
    ["revoke-invite", { store_id: store }],
    ["set-role", { ...good["set-role"], role: "root" }],
    ["set-role", { store_id: store, principal_id: uuid() }],
    ["remove", { store_id: store, principal_id: "../../etc" }],
    ["remove", { ...good.remove, role: "viewer" }],
    ["accept", { token: "short" }],
    ["accept", { token: b64() + "x" }],
    ["accept", { token: b64().slice(0, 42) + "+" }],
    ["accept", { token: b64(), store_id: store }],
    ["accept", {}],
    ["list", "not json"],
    ["list", "[]"],
  ];
  for (const [action, body] of bad) {
    const r = await call(action, typeof body === "string" ? { raw: body } : { body });
    assert.equal(r.status, 422, `${action} ${JSON.stringify(body)} -> ${r.status} ${r.text}`);
    assert.equal(codeOf(r.text), "invalid_request");
  }
  const wrongMime = await call("list", { mime: "text/plain" });
  assert.equal(wrongMime.status, 422);
  assert.equal(seen.length, before, "grammar errors never reach Go");
});

test("what reaches Go: BFF key, bearer from the cookie, no Cookie/Origin, body unchanged, exactly one call per request", async () => {
  const outcomes: Record<string, { status: number; body: string }> = {
    list: { status: 200, body: JSON.stringify({ my_role: "owner", members: [], invitations: [] }) },
    invite: { status: 201, body: JSON.stringify({ id: uuid(), expires_at: "2026-10-04T00:00:00Z", mail_state: "SENT" }) },
    "revoke-invite": { status: 204, body: "" },
    "set-role": { status: 204, body: "" },
    remove: { status: 204, body: "" },
    accept: { status: 200, body: JSON.stringify({ store_id: store, role: "fulfilment" }) },
  };
  for (const action of actions) {
    answer = outcomes[action];
    const before = seen.length;
    const r = await call(action);
    assert.equal(r.status, outcomes[action].status, `${action}: ${r.text}`);
    assert.match(r.headers.get("cache-control") ?? "", /no-store/, `${action} must be no-store`);
    assert.equal(seen.length, before + 1, `${action}: exactly one upstream call`);
    const u = seen[seen.length - 1];
    assert.equal(u.method, "POST");
    assert.equal(u.url, `/v1/identity/staff/${action}`, "no query, fixed path");
    assert.equal(u.headers["x-commerce-bff-key"], bffKey);
    assert.equal(u.headers["authorization"], `Bearer ${session}`);
    assert.equal(u.headers["cookie"], undefined, "the browser cookie is never forwarded");
    assert.equal(u.headers["origin"], undefined);
    assert.equal(u.headers["x-csrf-token"], undefined);
    assert.deepEqual(JSON.parse(u.body), good[action], "body forwarded unchanged");
  }
});

test("Go failures are rebuilt locally from allow-listed codes, never retried, and never echo the token", async () => {
  const token = b64();
  const echo = JSON.stringify({ code: "invite_invalid", message: `bad token ${token} for admin@example.test`, request_id: "x", retryable: false, details: { token } });
  const cases: [number, string, number, string][] = [
    [404, echo, 404, "invite_invalid"],
    [401, JSON.stringify({ code: "unauthorized", message: token }), 401, "unauthorized"],
    [403, JSON.stringify({ code: "forbidden", message: token }), 403, "forbidden"],
    [409, JSON.stringify({ code: "already_member", message: token }), 409, "already_member"],
    [409, JSON.stringify({ code: "last_owner", message: token }), 409, "last_owner"],
    [429, JSON.stringify({ code: "too_many_invitations", message: token }), 429, "too_many_invitations"],
    [422, JSON.stringify({ code: "invalid_email", message: token }), 422, "invalid_email"],
    [404, JSON.stringify({ code: "some_new_code", message: token }), 503, "retry_later"], // not allow-listed
    [418, JSON.stringify({ code: "forbidden", message: token }), 503, "retry_later"], // status Go never answers here
    [500, JSON.stringify({ code: "internal", message: token }), 503, "retry_later"],
    [404, "<html>" + token + "</html>", 0, "retry_later"], // non-JSON body: code retry_later, status not pinned
  ];
  for (const [status, body, want, code] of cases) {
    answer = { status, body, type: body.startsWith("<") ? "text/html" : "application/json" };
    const before = seen.length;
    const r = await call("accept", { body: { token } });
    if (want !== 0) assert.equal(r.status, want, `upstream ${status} ${body.slice(0, 40)} -> ${r.status} ${r.text}`);
    assert.equal(codeOf(r.text), code);
    assert.ok(!r.text.includes(token), "the token is never echoed to the browser");
    assert.ok(!r.text.includes("admin@example.test"), "upstream text is not relayed");
    assert.equal(seen.length, before + 1, "never retried");
  }
  // a 200 with an unparsable body, and a 200 on a route that must answer 204, are generic failures
  answer = { status: 200, body: JSON.stringify({ my_role: "god", members: [], invitations: [] }) };
  assert.equal((await call("list")).status, 503);
  answer = { status: 200, body: JSON.stringify({ ok: true }) };
  assert.equal((await call("remove")).status, 503);
  answer = { status: 204, body: "" };
});
