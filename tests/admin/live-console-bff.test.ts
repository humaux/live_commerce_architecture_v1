// Purpose: LC-U1 exact BFF grammar and browser transport fences (MOCK, no PG/browser).
// Depends on: console-request.ts, console-client.ts, real Studio authentication transport; Node test hooks.
// Used by: LC-U1 transport verification and integrator acceptance.
import assert from "node:assert/strict";
import { registerHooks } from "node:module";
import { fileURLToPath, pathToFileURL } from "node:url";
import { test } from "node:test";
import { consoleAny, consoleRoutes, validConsoleBody, validConsoleQuery } from "../../apps/admin/src/features/live/console-request.ts";

const adminRoot = fileURLToPath(new URL("../../apps/admin/", import.meta.url));
registerHooks({ resolve(specifier, context, next) {
  if (specifier === "server-only") return { url: "data:text/javascript,", shortCircuit: true };
  if (specifier === "next/server") return next("next/server.js", context);
  if (specifier === "next/headers") return { url: "data:text/javascript,export function headers(){throw Error('not a page fixture')}", shortCircuit: true };
  if (specifier.startsWith("@/")) return { url: pathToFileURL(adminRoot + specifier.slice(2) + ".ts").href, shortCircuit: true };
  if (context.parentURL?.includes("/apps/admin/") && specifier.startsWith(".") && !specifier.endsWith(".ts")) return next(`${specifier}.ts`, context);
  return next(specifier, context);
} });
const client = await import("../../apps/admin/src/features/live/console-client.ts");
const { sessionBoundary } = await import("../../apps/admin/lib/settings-client.ts");
const { StudioError } = await import("../../apps/admin/lib/studio-client.ts");
const { liveRequest, parseLiveJournal, validLiveRequest } = await import("../../apps/admin/src/features/live/command-journal.ts");
const store = "11111111-1111-4111-8111-111111111111";
const sid = "22222222-2222-4222-8222-222222222222";
const oid = "33333333-3333-4333-8333-333333333333";
const root = `live-sessions/${sid}`;
const at = "2026-10-06T00:00:00Z";
const controlOffer = { offer_id: oid, session_id: sid, keyword: "A1", sku_id: store, sku_code: "S1", product_name: "Synthetic",
  max_quantity_per_claim: 17, active: true, version: 6, activated_at: at, updated_at: at, sku_price_minor: 2000, currency: "TWD", live_price_minor: 1500 };

test("offer controls read authoritative limits without parsing unrelated KEYWORD_QTY_CONTAINS window/stats", async (t) => {
  let reply: unknown = { offers: [controlOffer], window: { match_mode: "KEYWORD_QTY_CONTAINS" }, stats: { unused: true } };
  const signal = new AbortController().signal;
  const fetch = t.mock.method(globalThis, "fetch", async (path: unknown, init?: RequestInit) => {
    assert.equal(path, `/api/stores/${store}/${root}/claims`);
    assert.equal(init?.method, "GET");
    assert.equal(init?.signal, signal);
    assert.equal(init?.cache, "no-store");
    return Response.json(reply, { headers: { "cache-control": "private, no-store" } });
  });
  assert.deepEqual(await client.readOfferControls(store, sid, signal), [controlOffer]);
  for (reply of [null, [], {}, { offers: null }, { offers: [controlOffer, controlOffer] },
    { offers: Array(101).fill(controlOffer) }, { offers: [{ ...controlOffer, session_id: store }] },
    { offers: [{ ...controlOffer, max_quantity_per_claim: 0 }] }])
    await assert.rejects(client.readOfferControls(store, sid, signal), (e: unknown) => e instanceof StudioError && e.code === "unavailable");
  assert.equal(fetch.mock.callCount(), 9);
});

test("offer controls accept 101 through the backend cap of 200 distinct offers, reject 201", async (t) => {
  const offers = Array.from({ length: 201 }, (_, i) => ({ ...controlOffer, offer_id: `33333333-3333-4333-8333-${String(i + 1).padStart(12, "0")}` }));
  let count = 101;
  t.mock.method(globalThis, "fetch", async () => Response.json({ offers: offers.slice(0, count) }, { headers: { "cache-control": "private, no-store" } }));
  for (count of [101, 200]) assert.equal((await client.readOfferControls(store, sid, new AbortController().signal)).length, count);
  count = 201;
  await assert.rejects(client.readOfferControls(store, sid, new AbortController().signal), (e: unknown) => e instanceof StudioError && e.code === "unavailable");
});

test("offer toggle preserves exactly the authoritative maximum in the real M4 payload", async (t) => {
  const original = Object.getOwnPropertyDescriptor(globalThis, "document");
  Object.defineProperty(globalThis, "document", { configurable: true, value: { cookie: `__Host-commerce_csrf=${"c".repeat(43)}` } });
  t.after(() => { if (original) Object.defineProperty(globalThis, "document", original); else Reflect.deleteProperty(globalThis, "document"); });
  const boundary = await sessionBoundary();
  const fetch = t.mock.method(globalThis, "fetch", async (path: unknown, init?: RequestInit) => {
    assert.equal(path, `/api/stores/${store}/${root}/claims/offers/${oid}`);
    assert.equal(init?.method, "PATCH");
    assert.equal(new Headers(init?.headers).get("idempotency-key"), "offer-preserve-key");
    assert.deepEqual(JSON.parse(init?.body as string), { expected_version: 6, active: false, max_quantity_per_claim: 17 });
    return Response.json({ ...controlOffer, active: false, version: 7 }, { headers: { "cache-control": "private, no-store" } });
  });
  const result = await client.toggleOffer(store, sid, oid, 6, false, 17, "offer-preserve-key", boundary);
  assert.equal(result.max_quantity_per_claim, 17);
  for (const limit of [0, -1, 1.2, 1000])
    await assert.rejects(client.toggleOffer(store, sid, oid, 6, false, limit, "offer-preserve-key", boundary), (e: unknown) => e instanceof StudioError && e.code === "invalid");
  assert.equal(fetch.mock.callCount(), 1);
});

test("console BFF admits exactly A1/A7/A6 and A5 read/copy methods", () => {
  for (const [method, path] of [["GET", `${root}/console`], ["GET", "live-sessions/results"],
    ["POST", `${root}/copy`], ["POST", `${root}/lifecycle`], ["POST", `${root}/claims/offers/${oid}/recommend`]]) {
    assert.equal(consoleAny.test(path), true);
    for (const m of ["GET", "POST", "PUT", "PATCH", "DELETE"]) assert.equal(consoleRoutes[m]?.test(path) ?? false, m === method);
    assert.equal(consoleAny.test(`${path}/extra`), false);
  }
  assert.equal(consoleAny.test(`live-sessions/${sid}/comments`), false);
  assert.equal(consoleAny.test(`live-sessions/${sid}/console%2f`), false);
});

test("Next proxy admits the exact console family and results query before the BFF", async () => {
  const { NextRequest } = await import("../../apps/admin/node_modules/next/server.js");
  const { proxy } = await import("../../apps/admin/proxy.ts");
  const commentRead = new NextRequest(`https://admin.example.test/api/stores/${store}/${root}/comments?after_epoch=0&after_seq=2&limit=50`);
  assert.equal(proxy(commentRead).headers.get("x-middleware-next"),"1","A2 must reach the BFF before Go");
  for(const suffix of ["print","private-reply","public-reply"]) {
    const request=new NextRequest(`https://admin.example.test/api/stores/${store}/${root}/comments/123_456/${suffix}`,{method:"POST",headers:{"Idempotency-Key":"synthetic-key"}});
    assert.equal(proxy(request).headers.get("x-middleware-next"),"1");
  }
  const invoke = (path: string, method = "GET", query = "") => {
    const raw = `https://admin.example.test/api/stores/${store}/${path}${query}`;
    const request = new NextRequest(raw, { method });
    // Production skipProxyUrlNormalize preserves raw '?' for this guard; the standalone constructor normalizes it.
    Object.defineProperty(request, "url", { value: raw });
    return proxy(request);
  };
  for (const [method, path, query] of [["GET", `${root}/console`, ""], ["GET", "live-sessions/results", `?session_id=${sid}`],
    ["POST", `${root}/copy`, ""], ["POST", `${root}/lifecycle`, ""], ["POST", `${root}/claims/offers/${oid}/recommend`, ""],
    ["GET", "live-sessions", "?limit=20"], ["GET", `${root}/claims`, ""], ["POST", "inventory/adjustments", ""]]) {
    const response = invoke(path, method, query);
    assert.equal(response.status, 200, `${method} ${path}${query}`);
    assert.equal(response.headers.get("x-middleware-next"), "1");
  }
  for (const path of [`${root}/console/extra`, `${root}/%63onsole`, `${root}/console%2f`, `live-sessions/${sid}/unknown`])
    assert.equal(invoke(path).status, 404, path);
  for (const [path, query] of [[`${root}/console`, "?"], [`${root}/copy`, "?tenant_id=forbidden"],
    ["live-sessions/results", ""], ["live-sessions/results", `?session_id=${sid}&session_id=${sid}`],
    ["live-sessions/results", `?session_id=${sid}&limit=1`]])
    assert.equal(invoke(path, "GET", query).status, 422, path + query);
});

test("PR18 proxy preserves malformed A2 cursor as 400 invalid_cursor", async () => {
  const { NextRequest } = await import("../../apps/admin/node_modules/next/server.js");
  const { proxy } = await import("../../apps/admin/proxy.ts");
  for(const query of ["after_epoch=1&after_seq=-1","after_epoch=1&after_seq=1.5","after_epoch=1&after_seq=1&after_seq=2","after_epoch=1&after_seq=2&before_cursor=abc","before_cursor=a/b","after_seq=2"]) {
    const result=proxy(new NextRequest(`https://admin.example.test/api/stores/${store}/${root}/comments?${query}`));
    assert.equal(result.status,400,query);assert.equal((await result.json()).code,"invalid_cursor");
  }
  const unknown=proxy(new NextRequest(`https://admin.example.test/api/stores/${store}/${root}/comments?text=private`));
  assert.equal(unknown.status,422);
});

test("results query is canonical repeated session_id 1..50, everything else queryless", () => {
  const url = "http://127.0.0.1/live-sessions/results";
  assert.equal(validConsoleQuery(`${url}?session_id=${sid}&session_id=${oid}`, "live-sessions/results"), true);
  for (const query of ["", "?", `?session_id=${sid}&session_id=${sid}`, `?session_id=${sid}&limit=1`,
    `?session_id=${sid}&`, "?session_id=ABCDEFAB-ABCD-4ABC-8ABC-ABCDEFABCDEF", `?session_id=${sid.replaceAll("-", "%2d")}`,
    `?session%5fid=${sid}`, `?session_id=${sid}+`, `?session_id=${Array(51).fill(sid).join("&session_id=")}`])
    assert.equal(validConsoleQuery(url + query, "live-sessions/results"), false, query);
  for (const path of [`${root}/console`, `${root}/copy`, `${root}/lifecycle`, `${root}/claims/offers/${oid}/recommend`]) {
    assert.equal(validConsoleQuery(`http://127.0.0.1/${path}`, path), true);
    assert.equal(validConsoleQuery(`http://127.0.0.1/${path}?`, path), false);
  }
});

test("real BFF route forwards only exact resources, authenticates scope and returns private scrubbed errors", async (t) => {
  // Synthetic in-process upstream; no production key, HTTP server, Next process or PostgreSQL.
  const publicOrigin = "https://admin.example.test";
  const apiOrigin = "http://127.0.0.1:39999";
  const token = Buffer.alloc(32, 4).toString("base64url");
  const csrf = Buffer.alloc(32, 5).toString("base64url");
  Object.assign(process.env, { COMMERCE_IDENTITY_ENABLED: "1", COMMERCE_FIXTURE_ENABLED: "0", COMMERCE_PASSWORD_LOGIN_ENABLED: "1",
    COMMERCE_PUBLIC_ORIGIN: publicOrigin, COMMERCE_API_ORIGIN: apiOrigin, COMMERCE_BFF_KEY: Buffer.alloc(32, 6).toString("base64url") });
  const routes = await import("../../apps/admin/app/api/stores/[store]/[...resource]/route.ts");
  const calls: { path: string; init?: RequestInit }[] = [];
  let upstreamStatus = 409;
  let upstreamBody: unknown = { code: "too_many_open_windows", message: "SENSITIVE upstream diagnostic" };
  let upstreamRaw: string | undefined;
  let stores = [store];
  t.mock.method(globalThis, "fetch", async (path: unknown, init?: RequestInit) => {
    if (path === `${apiOrigin}/v1/admin/stores`) return Response.json({ items: stores.map((id) => ({ id, name: "Synthetic", currency: "TWD", role: "live_operator", permissions: ["live:read", "inventory:live_adjust"] })) });
    calls.push({ path: String(path), init });
    if (upstreamRaw !== undefined) return new Response(upstreamRaw, { status: upstreamStatus, headers: { "content-type": "application/json" } });
    return Response.json(upstreamBody, { status: upstreamStatus });
  });
  const invoke = (method: string, path: string, body?: unknown, query = "", headers: Record<string, string> = {}) => {
    const request = new Request(`${publicOrigin}/api/stores/${store}/${path}${query}`, { method, headers: {
      cookie: `__Host-commerce_session=${token}; __Host-commerce_csrf=${csrf}`, origin: publicOrigin,
      ...(method === "POST" ? { "content-type": "application/json", "x-csrf-token": csrf, "idempotency-key": "console-bff-key" } : {}), ...headers,
    }, ...(body !== undefined ? { body: JSON.stringify(body) } : {}) });
    return (routes as unknown as Record<string, (r: Request, c: { params: Promise<{ store: string; resource: string[] }> }) => Promise<Response>>)[method](request,
      { params: Promise.resolve({ store, resource: path.split("/") }) });
  };
  const body = { action: "start", expected_version: 4 };
  const answer = await invoke("POST", `${root}/lifecycle`, body);
  assert.equal(answer.status, 409);
  assert.equal(answer.headers.get("cache-control"), "private, no-store");
  const err = await answer.json();
  assert.equal(err.code, "too_many_open_windows");
  assert.equal(JSON.stringify(err).includes("SENSITIVE"), false);
  assert.equal(calls[0].path, `${apiOrigin}/v1/admin/stores/${store}/${root}/lifecycle`);
  assert.deepEqual(JSON.parse(calls[0].init?.body as string), body);
  assert.equal(new Headers(calls[0].init?.headers).get("authorization"), `Bearer ${token}`);
  assert.equal(new Headers(calls[0].init?.headers).has("cookie"), false);
  assert.equal(new Headers(calls[0].init?.headers).get("idempotency-key"), "console-bff-key");
  calls.length = 0;
  for (const [method, path, b, query, h, status] of [
    ["POST", `${root}/lifecycle`, body, "?", {}, 422],
    ["POST", `${root}/lifecycle`, { ...body, tenant_id: store }, "", {}, 400],
    ["POST", `${root}/lifecycle`, body, "", { "idempotency-key": "" }, 422],
    ["POST", `${root}/lifecycle`, body, "", { origin: "https://attacker.example" }, 403],
    ["POST", `${root}/lifecycle`, body, "", { "x-csrf-token": "" }, 403],
    ["GET", `${root}/console`, undefined, "", { "idempotency-key": "should-not-be-here" }, 422],
    ["GET", `${root}/lifecycle`, undefined, "", {}, 405],
    ["POST", "live-sessions/results", {}, "", {}, 405],
    ["GET", "live-sessions/results", undefined, `?session_id=${sid}&session_id=${sid}`, {}, 422],
  ] as [string, string, unknown, string, Record<string, string>, number][]) {
    const response = await invoke(method, path, b, query, h);
    assert.equal(response.status, status, `${method} ${path}${query}`);
    assert.equal(response.headers.get("cache-control"), "private, no-store");
  }
  assert.equal(calls.length, 0);
  stores = [];
  assert.equal((await invoke("POST", `${root}/lifecycle`, body)).status, 404);
  assert.equal(calls.length, 0);
  stores = [store]; upstreamStatus = 200; upstreamBody = { recommended_at: at };
  assert.equal((await invoke("POST", `${root}/claims/offers/${oid}/recommend`, { expected_version: 4, post_comment: false })).status, 200);
  upstreamBody = { recommended_at: at, secret: "SENSITIVE" };
  assert.equal((await invoke("POST", `${root}/claims/offers/${oid}/recommend`, { expected_version: 4, post_comment: false })).status, 503);
  upstreamBody = { as_of: at, items: [{ session_id: sid, orders: 0, paid_orders: 0, multi_session_orders: 0, money: [] }] };
  assert.equal((await invoke("GET", "live-sessions/results", undefined, `?session_id=${sid}`)).status, 200);
  assert.equal(calls.at(-1)?.path, `${apiOrigin}/v1/admin/stores/${store}/live-sessions/results?session_id=${sid}`);
  // Exercise the actual route's A1 allowlist and strict parser with LC-B7's wire enum.
  upstreamBody = {
    session: { id: sid, title: "Synthetic", lifecycle: "draft", version: 1, started_at: null, ended_at: null },
    window: { state: "CLOSED", generation: 0, opened_at: null, match_mode: "KEYWORD_QTY_CONTAINS" },
    stats: { comments: { total: null, source: "unavailable" }, keyword_comments: 0, buyers: 0,
      orders: { count: 0, amount_minor: 0 }, paid: { count: 0, amount_minor: 0 }, currency: "TWD", as_of: at },
    offers: [], capabilities: {}, recommended: null,
    stream: { state: "unavailable", poll_interval_ms: 5000, last_ok_at: null, lag_ms: null, source_platform: "instagram", video_embeddable: false },
  };
  const console = await invoke("GET", `${root}/console`);
  assert.equal(console.status, 200);
  assert.deepEqual(await console.json(), upstreamBody);
  assert.equal(calls.at(-1)?.path, `${apiOrigin}/v1/admin/stores/${store}/${root}/console`);
  assert.equal(new Headers(calls.at(-1)?.init?.headers).has("idempotency-key"), false);
  // 200 legal products (120-rune names, three 40-rune option values) exceed the old 256 KiB cap.
  const large = { ...(upstreamBody as object), offers: Array.from({ length: 200 }, (_, i) => ({
    offer_id: `33333333-3333-4333-8333-${String(i + 1).padStart(12, "0")}`, keyword: `K${i}`,
    sku_id: `44444444-4444-4444-8444-${String(i + 1).padStart(12, "0")}`,
    product_name: "😀".repeat(120), variant_label: Array(3).fill("😀".repeat(40)).join(" / "),
    active: true, version: 1, live_price_minor: null, sku_price_minor: 100,
    stock: { tracked: true, sellable: 1, reserved: 0, warehouse_id: oid, balance_version: 1 },
    claimed: { buyers: 0, quantity: 0 }, ordered_qty: 0, paid_qty: 0, paid_amount_minor: 0, sold_out: false, low_stock: true,
  })) };
  upstreamRaw = JSON.stringify(large);
  assert.ok(Buffer.byteLength(upstreamRaw) > 256 * 1024);
  assert.ok(Buffer.byteLength(upstreamRaw) < 512 * 1024);
  const largeConsole = await invoke("GET", `${root}/console`);
  assert.equal(largeConsole.status, 200);
  assert.equal((await largeConsole.json()).offers.length, 200);
  upstreamRaw = " ".repeat(512 * 1024) + upstreamRaw;
  assert.equal((await invoke("GET", `${root}/console`)).status, 503);
  upstreamRaw = undefined;
  // Backend remains the permission/bounds authority; BFF must not require inventory:write.
  const adjustment = { warehouse_id: oid, sku_id: sid, delta: -1, expected_version: 1, reason: "live_console_edit" };
  upstreamBody = { warehouse_id: oid, sku_id: sid, on_hand: 2, reserved: 0, allocated: 0, unavailable: 0, version: 2, available: 2 };
  assert.equal((await invoke("POST", "inventory/adjustments", adjustment)).status, 200);
  assert.equal(calls.at(-1)?.path, `${apiOrigin}/v1/admin/stores/${store}/inventory/adjustments`);
  assert.deepEqual(JSON.parse(calls.at(-1)?.init?.body as string), adjustment);
  assert.equal(new Headers(calls.at(-1)?.init?.headers).get("idempotency-key"), "console-bff-key");
  upstreamStatus = 422; upstreamBody = { code: "below_reserved" };
  const denied = await invoke("POST", "inventory/adjustments", adjustment);
  assert.equal(denied.status, 422);
  assert.equal((await denied.json()).code, "below_reserved");
  upstreamStatus = 403; upstreamBody = { code: "forbidden" };
  assert.equal((await invoke("POST", "inventory/adjustments", adjustment)).status, 403);
});

test("new commands reject unknown keys and preserve frozen copy/lifecycle/recommend body", () => {
  const bodies: [string, unknown][] = [[`${root}/copy`, { title: "Synthetic", scheduled_at: null, expected_version: 1 }],
    [`${root}/lifecycle`, { action: "start", expected_version: 1, open_window: false }],
    [`${root}/claims/offers/${oid}/recommend`, { expected_version: 1, post_comment: false }]];
  for (const [path, value] of bodies) {
    assert.equal(validConsoleBody(path, JSON.stringify(value)), true);
    assert.equal(validConsoleBody(path, JSON.stringify({ ...value as object, tenant_id: store })), false);
    assert.equal(validConsoleBody(path, "{}"), false);
    assert.equal(validConsoleBody(path, "null"), false);
    assert.equal(validConsoleBody(path, "{"), false);
    assert.equal(validConsoleBody(path, JSON.stringify({ ...value as object, expected_version: 1.5 })), false);
  }
});

test("browser commands reuse Studio fences, exact no-comment recommend body and backend refusal codes", async (t) => {
  const original = Object.getOwnPropertyDescriptor(globalThis, "document");
  const doc = { cookie: `__Host-commerce_csrf=${"c".repeat(43)}` };
  Object.defineProperty(globalThis, "document", { configurable: true, value: doc });
  t.after(() => { if (original) Object.defineProperty(globalThis, "document", original); else Reflect.deleteProperty(globalThis, "document"); });
  const boundary = await sessionBoundary();
  const calls: { path: unknown; init?: RequestInit }[] = [];
  let status = 200;
  let body: unknown = { recommended_at: at };
  const fetch = t.mock.method(globalThis, "fetch", async (path: unknown, init?: RequestInit) => {
    calls.push({ path, init });
    return new Response(JSON.stringify(body), { status, headers: { "content-type": "application/json", "cache-control": "private, no-store" } });
  });
  await client.recommendOffer(store, sid, oid, 4, "console-key-1", boundary);
  assert.equal(calls[0].path, `/api/stores/${store}/${root}/claims/offers/${oid}/recommend`);
  assert.deepEqual(JSON.parse(calls[0].init?.body as string), { expected_version: 4, post_comment: false });
  assert.equal(calls[0].init?.credentials, "same-origin");
  assert.equal(calls[0].init?.cache, "no-store");
  assert.equal(new Headers(calls[0].init?.headers).get("idempotency-key"), "console-key-1");
  status = 409; body = { code: "version_conflict", message: "must not leak" };
  await assert.rejects(client.changeLifecycle(store, sid, "start", 4, "console-key-2", boundary),
    (e: unknown) => e instanceof StudioError && e.code === "conflict" && e.api === "version_conflict");
  status = 503; body = { code: "retry_later" };
  await assert.rejects(client.recommendOffer(store, sid, oid, 4, "console-key-1", boundary),
    (e: unknown) => e instanceof StudioError && e.code === "uncertain");
  assert.equal(fetch.mock.callCount(), 3);
  doc.cookie = "";
  await assert.rejects(client.changeLifecycle(store, sid, "start", 4, "console-key-2", boundary),
    (e: unknown) => e instanceof StudioError && e.code === "signed-out");
  assert.equal(fetch.mock.callCount(), 3);
});

test("results client preserves canonical ids/order and rejects invalid list before fetch", async (t) => {
  const seen: string[] = [];
  const ids = [sid, oid];
  const value = { as_of: at, items: ids.map((session_id) => ({ session_id, orders: 0, paid_orders: 0, multi_session_orders: 0, money: [] })) };
  const signal = new AbortController().signal;
  const fetch = t.mock.method(globalThis, "fetch", async (path: unknown, init?: RequestInit) => {
    seen.push(String(path));
    assert.equal(init?.signal, signal);
    assert.equal(init?.cache, "no-store");
    assert.equal(init?.credentials, "same-origin");
    return Response.json(value, { headers: { "cache-control": "private, no-store" } });
  });
  assert.deepEqual(await client.readSessionResults(store, ids, signal), value);
  assert.deepEqual(seen, [`/api/stores/${store}/live-sessions/results?session_id=${sid}&session_id=${oid}`]);
  for (const invalid of [[], [sid, sid], ["bad"], Array(51).fill(sid)])
    await assert.rejects(client.readSessionResults(store, invalid, signal), (e: unknown) => e instanceof StudioError && e.code === "invalid");
  assert.equal(fetch.mock.callCount(), 1);
});

test("stock client reuses catalog transport/key/parser and fences a changed login without retries", async (t) => {
  const original = Object.getOwnPropertyDescriptor(globalThis, "document");
  const doc = { cookie: `__Host-commerce_csrf=${"c".repeat(43)}` };
  Object.defineProperty(globalThis, "document", { configurable: true, value: doc });
  t.after(() => { if (original) Object.defineProperty(globalThis, "document", original); else Reflect.deleteProperty(globalThis, "document"); });
  const boundary = await sessionBoundary();
  const body = { warehouse_id: oid, sku_id: sid, delta: 2, expected_version: 0, reason: "live_console_edit" as const };
  let status = 200;
  let reply: unknown = { warehouse_id: oid, sku_id: sid, on_hand: 2, reserved: 0, allocated: 0, unavailable: 0, version: 1, available: 2 };
  let changeLogin = false;
  const fetch = t.mock.method(globalThis, "fetch", async (path: unknown, init?: RequestInit) => {
    assert.equal(path, `/api/stores/${store}/inventory/adjustments`);
    assert.equal(init?.cache, "no-store");
    assert.deepEqual(JSON.parse(init?.body as string), body);
    assert.equal(new Headers(init?.headers).get("idempotency-key"), "stock-console-key");
    if (changeLogin) doc.cookie = `__Host-commerce_csrf=${"d".repeat(43)}`;
    return Response.json(reply, { status, headers: { "cache-control": "no-store" } });
  });
  assert.deepEqual(await client.adjustLiveStock(store, body, "stock-console-key", boundary), { sku_id: sid });
  status = 409; reply = { code: "version_conflict" };
  await assert.rejects(client.adjustLiveStock(store, body, "stock-console-key", boundary), (e: unknown) => e instanceof StudioError && e.code === "conflict" && e.api === "version_conflict");
  status = 422; reply = { code: "below_reserved" };
  await assert.rejects(client.adjustLiveStock(store, body, "stock-console-key", boundary), (e: unknown) => e instanceof StudioError && e.code === "invalid" && e.api === "below_reserved");
  status = 503; reply = { code: "retry_later" };
  await assert.rejects(client.adjustLiveStock(store, body, "stock-console-key", boundary), (e: unknown) => e instanceof StudioError && e.code === "uncertain");
  status = 200; reply = { warehouse_id: oid, sku_id: sid }; changeLogin = true;
  await assert.rejects(client.adjustLiveStock(store, body, "stock-console-key", boundary), (e: unknown) => e instanceof StudioError && e.code === "signed-out");
  assert.equal(fetch.mock.callCount(), 5);
  await assert.rejects(client.adjustLiveStock(store, { ...body, delta: 1001 }, "stock-console-key", boundary), (e: unknown) => e instanceof StudioError && e.code === "invalid");
  assert.equal(fetch.mock.callCount(), 5);
});

test("journal replay sends the exact original method/path/body/key for all five commands", async (t) => {
  const original = Object.getOwnPropertyDescriptor(globalThis, "document");
  Object.defineProperty(globalThis, "document", { configurable: true, value: { cookie: `__Host-commerce_csrf=${"c".repeat(43)}` } });
  t.after(() => { if (original) Object.defineProperty(globalThis, "document", original); else Reflect.deleteProperty(globalThis, "document"); });
  const boundary = await sessionBoundary(), seen: { path: unknown; init?: RequestInit }[] = [];
  let status = 503;
  t.mock.method(globalThis, "fetch", async (path: unknown, init?: RequestInit) => {
    seen.push({ path, init }); return Response.json({ code: "retry_later" }, { status, headers: { "cache-control": "private, no-store" } });
  });
  const commands = [
    liveRequest(store, sid, "lifecycle", "POST", { action: "start", expected_version: 1 }),
    liveRequest(store, sid, "copy", "POST", { title: "Next", scheduled_at: null, expected_version: 1 }),
    liveRequest(store, sid, `claims/offers/${oid}/recommend`, "POST", { expected_version: 1, post_comment: false }),
    liveRequest(store, sid, `claims/offers/${oid}`, "PATCH", { expected_version: 1, active: false, max_quantity_per_claim: 7 }),
    liveRequest(store, sid, "inventory/adjustments", "POST", { warehouse_id: oid, sku_id: sid, delta: 2, expected_version: 0, reason: "live_console_edit" }),
  ];
  for (const command of commands) {
    const restored = parseLiveJournal(JSON.stringify({ ...command, key: oid, mayHaveCommitted: true }), `${store}:${sid}`)!;
    for (let attempt = 0; attempt < 2; attempt++) {
      await assert.rejects(client.executeLiveRequest(restored, restored.key, boundary), (e: unknown) => e instanceof StudioError && e.code === "uncertain");
      const sent = seen.at(-1)!; assert.equal(sent.path, command.path); assert.equal(sent.init?.method, command.method); assert.equal(sent.init?.body, command.body);
      assert.equal(new Headers(sent.init?.headers).get("idempotency-key"), oid);
    }
  }
  const stock = commands.at(-1)!;
  for (const code of [401, 403, 404, 409, 422, 429]) {
    status = code; await assert.rejects(client.executeLiveRequest(stock, oid, boundary), (e: unknown) => e instanceof StudioError && e.status === code);
  }
  assert.equal(validConsoleBody(`${root}/claims/offers/${oid}/recommend`, JSON.stringify({ expected_version: 1, post_comment: true })), false);
  assert.equal(validLiveRequest({ ...commands[0], body: JSON.stringify({ action: "start", expected_version: 1, open_window: false }) }, `${store}:${sid}`), false);
  for (const poisoned of [{ ...commands[0], path: "https://attacker.invalid" }, { ...commands[0], method: "DELETE" }, { ...commands[0], body: '{"expected_version":1,"action":"start"}' }])
    assert.equal(validLiveRequest(poisoned, `${store}:${sid}`), false);
  assert.equal(parseLiveJournal(JSON.stringify({ ...commands[0], key: oid, mayHaveCommitted: true }), `${oid}:${sid}`), null);
});
