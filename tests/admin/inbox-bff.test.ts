// Purpose: LC-U2b real-handler Request counterexamples for frozen A8-A14 and published template reads.
// Depends on: Node test/http/registerHooks; actual store catchall BFF and a synthetic Go HTTP boundary.
// Used by: focused Node gate and integrator independent verification; no real PII or provider traffic.
import assert from "node:assert/strict";
import { randomBytes } from "node:crypto";
import { createServer } from "node:http";
import { createRequire, registerHooks } from "node:module";
import type { AddressInfo } from "node:net";
import { test, type TestContext } from "node:test";
import { fileURLToPath, pathToFileURL } from "node:url";

const adminRoot = fileURLToPath(new URL("../../apps/admin/", import.meta.url));
registerHooks({
  resolve(specifier, context, next) {
    if (specifier === "next/constants") return {
      // Next's CJS constants need a named-export bridge under Node strip-types; values remain the installed module's.
      url: "data:text/javascript," + encodeURIComponent(`import constants from ${JSON.stringify(pathToFileURL(adminRoot + "node_modules/next/constants.js").href)}; export const PHASE_PRODUCTION_BUILD = constants.PHASE_PRODUCTION_BUILD;`),
      shortCircuit: true,
    };
    if (specifier === "server-only")
      return { url: "data:text/javascript,", shortCircuit: true };
    // Authenticated Request-driven handlers must never call the ambient fixture adapter.
    if (specifier === "next/headers")
      return {
        url: "data:text/javascript,export function headers(){throw new Error('ambient_headers_forbidden')}",
        shortCircuit: true,
      };
    if (specifier.startsWith("@/"))
      return {
        url: pathToFileURL(adminRoot + specifier.slice(2) + ".ts").href,
        shortCircuit: true,
      };
    if (specifier.startsWith(".") && !context.parentURL?.includes("/node_modules/") && !/\.[a-z]+$/.test(specifier))
      return next(specifier + ".ts", context);
    return next(specifier, context);
  },
});
const store = crypto.randomUUID(),
  cid = crypto.randomUUID(),
  customer = crypto.randomUUID();
const session = randomBytes(32).toString("base64url"),
  csrf = randomBytes(32).toString("base64url");
const origin = "https://admin.example.test";
const seen: {
  url: string;
  method: string;
  headers: Record<string, unknown>;
  body: string;
}[] = [];
let answer: { status: number; body: string; contentType?: string } = {
  status: 200,
  body: JSON.stringify({ items: [], next_cursor: "", unread_total: 0 }),
};
let storesStatus = 200;
// Network-edge plans exercise scope revocation between the BFF check and the Go read.
type StoreAnswer = { status: number; body: unknown; delay?: number; hold?: boolean; onStart?: () => void; onClose?: () => void };
let storesAnswers: StoreAnswer[] | null = null;
let storeChecks = 0;
const storeAuthorities: boolean[] = [];
let readDelay = 0, afterRead: (() => void) | undefined;
const upstream = createServer((req, res) => {
  const chunks: Buffer[] = [];
  req.on("data", (chunk) => chunks.push(chunk));
  req.on("end", () => {
    res.setHeader("content-type", "application/json");
    if (req.url === "/v1/admin/stores") {
      storeChecks++;
      storeAuthorities.push(req.headers.authorization === `Bearer ${session}`);
      const planned = storesAnswers?.shift();
      if (planned) {
        res.statusCode = planned.status;
        planned.onStart?.();
        if (planned.hold) { res.on("close", () => planned.onClose?.()); return; }
        if (planned.delay) { setTimeout(() => res.end(JSON.stringify(planned.body)), planned.delay); return; }
        res.end(JSON.stringify(planned.body));
        return;
      }
      res.statusCode = storesStatus;
      res.end(
        JSON.stringify(
          storesStatus === 200
            ? {
                items: [
                  { id: store, name: "Synthetic store", currency: "TWD" },
                ],
              }
            : { code: "unauthorized" },
        ),
      );
      return;
    }
    res.setHeader("content-type", answer.contentType ?? "application/json");
    seen.push({
      url: req.url ?? "",
      method: req.method ?? "",
      headers: { ...req.headers },
      body: Buffer.concat(chunks).toString(),
    });
    res.statusCode = answer.status;
    const finish = () => { afterRead?.(); res.end(answer.body); };
    if (readDelay) setTimeout(finish, readDelay); else finish();
  });
});
await new Promise<void>((resolve) => upstream.listen(0, "127.0.0.1", resolve));
Object.assign(process.env, {
  COMMERCE_IDENTITY_ENABLED: "1",
  COMMERCE_PASSWORD_LOGIN_ENABLED: "1",
  COMMERCE_API_ORIGIN: `http://127.0.0.1:${(upstream.address() as AddressInfo).port}`,
  COMMERCE_PUBLIC_ORIGIN: origin,
  COMMERCE_BFF_KEY: randomBytes(32).toString("base64url"),
});
const route = await import(
  pathToFileURL(adminRoot + "app/api/stores/[store]/[...resource]/route.ts")
    .href
);
test.after(() => upstream.close());
type Options = {
  signal?: AbortSignal;
  method?: string;
  query?: string;
  body?: unknown;
  raw?: string;
  headers?: Record<string, string>;
  storeId?: string;
  cookie?: string;
};
async function call(path: string, over: Options = {}) {
  const method = over.method ?? "GET";
  const headers = new Headers({
    cookie:
      over.cookie ??
      `__Host-commerce_session=${session}; __Host-commerce_csrf=${csrf}`,
    origin,
    "x-csrf-token": csrf,
    ...over.headers,
  });
  if (method !== "GET" && method !== "HEAD") {
    headers.set("content-type", "application/json");
    if (!headers.has("idempotency-key"))
      headers.set("idempotency-key", "synthetic-receipt");
  }
  const request = new Request(
    `${origin}/api/stores/${over.storeId ?? store}/${path}${over.query ?? ""}`,
    {
      method,
      headers,
      ...(over.signal ? { signal: over.signal } : {}),
      ...(method !== "GET" && method !== "HEAD"
        ? { body: over.raw ?? JSON.stringify(over.body ?? {}) }
        : {}),
    },
  );
  const response = await route[method](request, {
    params: Promise.resolve({
      store: over.storeId ?? store,
      resource: path.split("/"),
    }),
  });
  return {
    status: response.status,
    headers: response.headers,
    body: await response.json(),
  };
}
const base = `inbox/conversations/${cid}`;
const visibleStores = { items: [{ id: store, name: "Synthetic store", currency: "TWD" }] };
function scopeRace(t: TestContext, fresh: StoreAnswer) {
  const previous = answer;
  storesAnswers = [{ status: 200, body: visibleStores }, fresh];
  storeChecks = 0; storeAuthorities.length = 0;
  answer = { status: 403, body: JSON.stringify({ code: "forbidden", message: "DO_NOT_FORWARD" }) };
  t.after(() => { storesAnswers = null; answer = previous; readDelay = 0; afterRead = undefined; });
}
for (const [path, query] of [
  ["inbox/buyer-panel", `?bundle_id=${cid}`],
  ["inbox/conversations", `?filter=live_comment&session_id=${cid}`],
  ["message-templates", ""],
  [`live-sessions/${cid}/comments`, ""],
]) test(`W3U3 revoked scope after ${path} read admission is 404`, async t => {
  scopeRace(t, { status: 200, body: { items: [] } });
  const before = seen.length, result = await call(path, { query });
  assert.equal(result.status, 404);
  assert.equal(result.body.code, "not_found");
  assert.equal(seen.length, before + 1, "never replay the private read");
  assert.equal(storeChecks, 2, "scope proof is fresh, not the original list");
  assert.equal(storeAuthorities.every(Boolean), true, "same authenticated bearer for both checks");
  assert.match(result.headers.get("cache-control") ?? "", /no-store/);
  assert.equal(result.headers.get("referrer-policy"), "no-referrer");
  assert.equal(result.headers.has("set-cookie"), false, "store loss is not logout");
  assert.doesNotMatch(JSON.stringify(result.body), /DO_NOT_FORWARD/);
});
test("W3U3 genuine read permission denial stays 403", async t => {
  scopeRace(t, { status: 200, body: visibleStores });
  const result = await call("inbox/buyer-panel", { query: `?bundle_id=${cid}` });
  assert.equal(result.status, 403); assert.equal(result.body.code, "forbidden");
  assert.equal(storeChecks, 2); assert.equal(result.headers.has("set-cookie"), false);
});
for (const fresh of [
  { status: 503, body: { code: "unavailable" } },
  { status: 401, body: { code: "unauthorized" } },
  { status: 200, body: { items: [{ id: store, name: null, currency: "TWD" }] } },
]) test(`W3U3 unproven scope loss (${fresh.status}) preserves the denial`, async t => {
  scopeRace(t, fresh);
  const result = await call("inbox/buyer-panel", { query: `?bundle_id=${cid}` });
  assert.equal(result.status, 403); assert.equal(result.body.code, "forbidden");
  assert.equal(storeChecks, 2); assert.equal(result.headers.has("set-cookie"), false);
});
test("W3U3 write denial is not scope-reconciled or replayed", async t => {
  scopeRace(t, { status: 200, body: { items: [] } });
  const before = seen.length;
  const result = await call(`live-sessions/${cid}/comments/123_456/print`, { method: "POST", body: {} });
  assert.equal(result.status, 403); assert.equal(result.body.code, "forbidden");
  assert.equal(storeChecks, 1); assert.equal(seen.length, before + 1);
  assert.equal(seen.at(-1)?.headers["idempotency-key"], "synthetic-receipt");
});
test("W3U3 unknown upstream 403 never becomes an authoritative scope refusal", async t => {
  scopeRace(t, { status: 200, body: { items: [] } });
  answer = { status: 403, body: JSON.stringify({ code: "non_contract_code" }) };
  const result = await call("inbox/buyer-panel", { query: `?bundle_id=${cid}` });
  assert.equal(result.status, 503); assert.equal(result.body.code, "retry_later");
  assert.equal(storeChecks, 1);
});
test("W3U3 successful read keeps one scope check and one private read", async t => {
  scopeRace(t, { status: 200, body: { items: [] } });
  answer = { status: 200, body: JSON.stringify({ items: [], next_cursor: "", unread_total: 0 }) };
  const before = seen.length, result = await call("inbox/conversations");
  assert.equal(result.status, 200); assert.equal(storeChecks, 1); assert.equal(seen.length, before + 1);
});
test("W3U3 stalled scope proof cannot delay a known denial past the privacy budget", { timeout: 10000 }, async t => {
  let closed!: () => void;
  const cancelled = new Promise<void>(resolve => { closed = resolve; });
  scopeRace(t, { status: 200, body: { items: [] }, hold: true, onClose: closed });
  storesAnswers![0].delay = 60; readDelay = 60;
  const before = seen.length, started = performance.now();
  const result = await call("inbox/buyer-panel", { query: `?bundle_id=${cid}` });
  assert.equal(result.status, 403); assert.equal(result.body.code, "forbidden");
  assert.ok(performance.now() - started < 1200, "known denial must not become the client's 8s timeout/503");
  await cancelled;
  assert.equal(storeChecks, 2); assert.equal(seen.length, before + 1);
});
test("W3U3 elapsed request budget skips scope proof rather than prolonging the denial", { timeout: 10000 }, async t => {
  scopeRace(t, { status: 200, body: visibleStores });
  let clock = 100;
  t.mock.method(performance, "now", () => clock);
  afterRead = () => { clock = 7600; };
  const result = await call("inbox/buyer-panel", { query: `?bundle_id=${cid}` });
  assert.equal(result.status, 403); assert.equal(result.body.code, "forbidden");
  assert.equal(storeChecks, 1, "no proof after the absolute deadline");
});
test("W3U3 request cancellation also cancels scope proof and preserves the original denial", { timeout: 10000 }, async t => {
  const controller = new AbortController();
  let closed!: () => void;
  const cancelled = new Promise<void>(resolve => { closed = resolve; });
  scopeRace(t, { status: 200, body: { items: [] }, hold: true, onStart: () => controller.abort(), onClose: closed });
  const before = seen.length, started = performance.now();
  const result = await call("inbox/buyer-panel", { query: `?bundle_id=${cid}`, signal: controller.signal });
  assert.equal(result.status, 403); assert.equal(result.body.code, "forbidden");
  assert.ok(performance.now() - started < 1000, "abort must reach the identity fetch");
  await cancelled;
  assert.equal(seen.length, before + 1);
});
test("PR18 real BFF preserves invalid_ref and cursor errors without upstream retry", async t=>{
  t.after(()=>{answer={status:200,body:JSON.stringify({items:[],next_cursor:"",unread_total:0})};});
  answer={status:422,body:JSON.stringify({code:"invalid_ref"})};
  const denied=await call(`live-sessions/${cid}/comments/123/print`,{method:"POST",body:{}});
  assert.equal(denied.status,422);assert.equal(denied.body.code,"invalid_ref");assert.equal(denied.body.retryable,false);
  const before=seen.length;
  const cursor=await call(`live-sessions/${cid}/comments`,{query:"?after_epoch=1&after_seq=-1"});
  assert.equal(cursor.status,400);assert.equal(cursor.body.code,"invalid_cursor");assert.equal(seen.length,before);
});
test("PR18 valid A2 Unicode/escaped pages exceed 1 MiB but A8 cap stays unchanged",async t=>{
  t.after(()=>{answer={status:200,body:JSON.stringify({items:[],next_cursor:"",unread_total:0})};});
  for(const text of ["漢".repeat(8000),"<".repeat(8000)]) {
    // Network-edge Go encoding: HTML-sensitive runes escape as six ASCII bytes each.
    answer={status:200,body:JSON.stringify({items:Array.from({length:100},(_,i)=>({ref:String(i+1),text,author_name:"名".repeat(255)}))}).replaceAll("<","\\u003c")};
    assert.ok(Buffer.byteLength(answer.body)>1<<20);
    const a2=await call(`live-sessions/${cid}/comments`,{query:"?limit=100"});assert.equal(a2.status,200);assert.equal(a2.body.items.length,100);
    const a8=await call("inbox/conversations");assert.equal(a8.status,503);assert.equal(a8.body.code,"retry_later");
  }
});
test("K3 comment-only no_source cannot widen the existing inbox error seam",async t=>{
  t.after(()=>{answer={status:200,body:JSON.stringify({items:[],next_cursor:"",unread_total:0})};});
  answer={status:409,body:JSON.stringify({code:"no_source",debug:"DO_NOT_FORWARD"})};
  const inbox=await call("inbox/conversations",{query:`?filter=live_comment&session_id=${cid}`});
  assert.equal(inbox.status,503);assert.equal(inbox.body.code,"retry_later");assert.equal(JSON.stringify(inbox.body).includes("DO_NOT_FORWARD"),false);
  const comments=await call(`live-sessions/${cid}/comments`);
  assert.equal(comments.status,409);assert.equal(comments.body.code,"no_source");
});
test("LC-U2a real BFF seam A2-A5 exact grammar, privacy, denial and no diagnostic reflection", async () => {
  const root=`live-sessions/${cid}/comments`;
  answer={status:200,body:JSON.stringify({items:[]})};
  const read=await call(root,{query:"?after_epoch=0&after_seq=7&limit=50"});
  assert.equal(read.status,200);assert.match(read.headers.get("cache-control")??"",/no-store/);assert.equal(read.headers.get("referrer-policy"),"no-referrer");
  for(const suffix of ["private-reply","public-reply","print"]) {
    const result=await call(`${root}/123_456/${suffix}`,{method:"POST",body:suffix==="print"?{}:{text:"Synthetic reply"}});
    assert.equal(result.status,200);assert.equal(seen.at(-1)?.headers["idempotency-key"],"synthetic-receipt");
    assert.equal(seen.at(-1)?.headers.cookie,undefined);
  }
  const before=seen.length;
  for(const opts of [{query:"?text=private"},{headers:{"idempotency-key":"forbidden-read"}}])assert.equal((await call(root,opts)).status,422);
  assert.equal((await call(root,{query:"?after_seq=2"})).status,400); // PR18 frozen invalid_cursor contract.
  assert.equal((await call(`${root}/123/private-reply`,{method:"POST",body:{text:"x",tenant_id:store}})).status,422);
  assert.equal((await call(`${root}/123/private-reply`,{method:"POST",body:{text:"x"},headers:{"x-csrf-token":"wrong"}})).status,403);
  assert.equal(seen.length,before);
  answer={status:422,body:JSON.stringify({code:"public_reply_forbidden_content",debug:"DO_NOT_FORWARD"})};
  const denial=await call(`${root}/123/public-reply`,{method:"POST",body:{text:"Synthetic payment URL"}});
  answer={status:200,body:JSON.stringify({items:[],next_cursor:"",unread_total:0})};
  assert.equal(denial.status,422);assert.equal(denial.body.code,"public_reply_forbidden_content");
  assert.equal(denial.body.message,"Request failed.");assert.deepEqual(denial.body.details,{});assert.equal(denial.body.retryable,false);
  assert.equal(JSON.stringify(denial.body).includes("DO_NOT_FORWARD"),false);
});
const writes: [string, Record<string, unknown>][] = [
  [`${base}/read`, { read_seq: 3 }],
  [`${base}/takeover`, { expected_generation: 0 }],
  [`${base}/release`, { expected_generation: 1 }],
  [
    `${base}/messages`,
    { text: "Synthetic DM sentinel", expected_generation: 0 },
  ],
  [
    `${base}/messages`,
    {
      template_id: "merchant-hello/v1",
      template_version: 1,
      expected_generation: 0,
    },
  ],
  [`${base}/customer-link`, { customer_id: customer, expected_version: 1 }],
  [`${base}/customer-link`, { customer_id: null, expected_version: 2 }],
];

test("A8/A9/A13 and published templates: exact reads forward once with no-store", async () => {
  for (const [path, query] of [
    ["inbox/conversations", "?filter=messenger&limit=50"],
    [`${base}/messages`, "?before_seq=3&limit=2"],
    ["inbox/buyer-panel", `?conversation_id=${cid}`],
    ["inbox/buyer-panel", `?bundle_id=${customer}`],
    ["message-templates", ""],
  ]) {
    const count = seen.length;
    const result = await call(path, { query });
    assert.equal(result.status, 200);
    assert.equal(seen.length, count + 1);
    assert.equal(seen.at(-1)?.url, `/v1/admin/stores/${store}/${path}${query}`);
    assert.match(result.headers.get("cache-control") ?? "", /no-store/);
  }
});
test("real streamed POST bodies: A10-A14 exact keys reach upstream", async () => {
  answer = {
    status: 200,
    body: JSON.stringify({
      read_seq: 3,
      mode: "human",
      takeover_generation: 1,
    }),
  };
  for (const [path, body] of writes) {
    const n = seen.length;
    const result = await call(path, { method: "POST", body });
    assert.equal(result.status, 200);
    assert.equal(seen.length, n + 1);
    assert.deepEqual(JSON.parse(seen.at(-1)!.body), body);
    assert.equal(seen.at(-1)!.headers.authorization, `Bearer ${session}`);
    assert.equal(seen.at(-1)!.headers.cookie, undefined);
    assert.equal(seen.at(-1)!.headers.origin, undefined);
  }
});
test("query, read headers, methods and body allowlists refuse before upstream", async () => {
  const cases: [string, Options][] = [
    ["inbox/conversations", { query: "?" }],
    ["inbox/conversations", { query: "?limit=51" }],
    ["inbox/conversations", { query: "?limit=01" }],
    ["inbox/conversations", { query: "?limit=2&limit=2" }],
    ["inbox/conversations", { query: "?text=Synthetic+DM" }],
    ["inbox/conversations", { query: "?cursor=%GG" }],
    [
      "inbox/conversations",
      { headers: { "idempotency-key": "synthetic-read" } },
    ],
    [`${base}/messages`, { query: "?before_seq=0" }],
    [`${base}/messages`, { query: "?before_seq=01" }],
    [`${base}/messages`, { query: "?limit=1&text=sentinel" }],
    ["inbox/buyer-panel", {}],
    [
      "inbox/buyer-panel",
      { query: `?conversation_id=${cid}&bundle_id=${customer}` },
    ],
    ["inbox/buyer-panel", { query: "?conversation_id=not-uuid" }],
    ["message-templates", { query: "?q=sentinel" }],
    ["message-templates", { method: "POST", body: {} }],
    [`${base}/read`, { method: "GET" }],
    [`${base}/messages`, { method: "PUT", body: {} }],
    ...writes.map(([path, body]): [string, Options] => [
      path,
      { method: "POST", body: { ...body, tenant_id: customer } },
    ]),
    [`${base}/read`, { method: "POST", body: { read_seq: null } }],
    [`${base}/takeover`, { method: "POST", body: { expected_generation: -1 } }],
    [
      `${base}/messages`,
      {
        method: "POST",
        body: {
          text: "hello",
          template_id: "merchant-hello",
          template_version: 1,
          expected_generation: 0,
        },
      },
    ],
    [
      `${base}/messages`,
      {
        method: "POST",
        body: { text: "hello", template_version: 1, expected_generation: 0 },
      },
    ],
    [`${base}/customer-link`, { method: "POST", body: { customer_id: null } }],
  ];
  for (const [path, over] of cases) {
    const n = seen.length;
    const r = await call(path, over);
    assert.ok(
      r.status >= 400,
      `${path}: ${JSON.stringify(over)} => ${r.status}`,
    );
    assert.equal(seen.length, n);
    assert.match(r.headers.get("cache-control") ?? "", /no-store/);
  }
});
test("session, CSRF and store authority precede content forwarding", async () => {
  for (const [over, status] of [
    [{ cookie: "" }, 401],
    [{ headers: { origin: "https://foreign.example" } }, 403],
    [{ headers: { "x-csrf-token": "wrong" } }, 403],
    [{ storeId: customer }, 404],
  ] as [Options, number][]) {
    const n = seen.length;
    const r = await call(`${base}/messages`, {
      method: "POST",
      body: { text: "Synthetic DM sentinel", expected_generation: 0 },
      ...over,
    });
    assert.equal(r.status, status);
    assert.equal(seen.length, n);
    assert.match(r.headers.get("cache-control") ?? "", /no-store/);
    if (status === 401)
      assert.match(r.headers.get("set-cookie") ?? "", /Max-Age=0/);
  }
  storesStatus = 401;
  const n = seen.length;
  const r = await call("inbox/conversations");
  assert.equal(r.status, 401);
  assert.equal(seen.length, n);
  assert.match(r.headers.get("set-cookie") ?? "", /Max-Age=0/);
  storesStatus = 200;
});
test("A12 fixed codes preserved; arbitrary upstream diagnostics stripped", async () => {
  for (const [status, code] of [
    [409, "window_closed"],
    [409, "takeover_changed"],
    [409, "capability"],
    [409, "conversation_gone"],
    [409, "duplicate_recent"],
    [409, "conflict"],
    [422, "invalid_text"],
    [429, "rate_limited"],
    [503, "unavailable"],
  ] as [number, string][]) {
    answer = {
      status,
      body: JSON.stringify({
        code,
        message: "Synthetic DM sentinel",
        details: { recipient_psid: "Synthetic PSID sentinel" },
      }),
    };
    const r = await call(`${base}/messages`, {
      method: "POST",
      body: { text: "hello", expected_generation: 0 },
    });
    assert.equal(r.status, status);
    assert.equal(r.body.code, code);
    assert.doesNotMatch(JSON.stringify(r.body), /sentinel/);
  }
  answer = {
    status: 409,
    body: JSON.stringify({
      code: "synthetic_payload_leak",
      message: "Synthetic DM sentinel",
    }),
  };
  const r = await call(`${base}/messages`, {
    method: "POST",
    body: { text: "hello", expected_generation: 0 },
  });
  assert.equal(r.status, 503);
  assert.equal(r.body.code, "retry_later");
});

test("client preserves codes and receipt; changed/aborted session cannot publish private response", async () => {
  const { inboxRead, inboxWrite, InboxError } = await import(
    pathToFileURL(adminRoot + "lib/inbox-client.ts").href
  ) as typeof import("../../apps/admin/lib/inbox-client");
  const originalFetch = globalThis.fetch;
  Object.defineProperty(globalThis, "document", {
    configurable: true,
    value: { cookie: `__Host-commerce_csrf=${csrf}` },
  });
  const requests: { url: string; init: RequestInit | undefined }[] = [];
  const response = (status = 200, body: unknown = { items: [] }) =>
    Response.json(body, {
      status,
      headers: { "Cache-Control": "private, no-store" },
    });
  try {
    globalThis.fetch = async (input, init) => {
      requests.push({ url: String(input), init });
      return response(409, {
        code: "window_closed",
        message: "Synthetic DM sentinel",
      });
    };
    await assert.rejects(
      inboxWrite(
        store,
        `${base}/messages`,
        { text: "Synthetic DM sentinel", expected_generation: 0 },
        "same-retry-receipt",
      ),
      (error: unknown) =>
        error instanceof InboxError &&
        error.code === "window_closed" &&
        error.status === 409 &&
        !error.message.includes("sentinel"),
    );
    assert.equal(
      new Headers(requests[0].init?.headers).get("Idempotency-Key"),
      "same-retry-receipt",
    );
    assert.equal(requests[0].init?.cache, "no-store");
    assert.equal(requests[0].init?.credentials, "same-origin");
    const before = requests.length;
    await assert.rejects(
      inboxRead(store, "inbox/conversations?text=Synthetic+DM+sentinel"),
      (error: unknown) =>
        error instanceof InboxError && error.code === "invalid_request",
    );
    assert.equal(requests.length, before);
    globalThis.fetch = async () => {
      document.cookie = "";
      return response();
    };
    await assert.rejects(
      inboxRead(store, "inbox/conversations"),
      (error: unknown) =>
        error instanceof InboxError && error.code === "unauthorized",
    );
    document.cookie = `__Host-commerce_csrf=${csrf}`;
    const controller = new AbortController();
    globalThis.fetch = async () => {
      controller.abort();
      return response();
    };
    await assert.rejects(
      inboxRead(store, "inbox/conversations", controller.signal),
      { name: "AbortError" },
    );
    globalThis.fetch = async () =>
      response(409, { code: "untrusted_provider_text" });
    await assert.rejects(
      inboxRead(store, "inbox/conversations"),
      (error: unknown) =>
        error instanceof InboxError &&
        error.code === "retry_later" &&
        error.status === 503,
    );
  } finally {
    globalThis.fetch = originalFetch;
    delete (globalThis as unknown as { document?: unknown }).document;
  }
});

test("A9 admits a full page of valid Unicode replies while bounding private response bytes", async () => {
  answer = {
    status: 200,
    body: JSON.stringify({
      items: Array.from({ length: 50 }, () => ({
        direction: "in",
        text: "界".repeat(2000),
        attachments: [],
      })),
      mode: "auto",
      takeover_generation: 0,
      window_open_until: "2099-01-01T00:00:00Z",
      human_until: null,
    }),
  };
  const full = await call(`${base}/messages`, { query: "?limit=50" });
  assert.equal(full.status, 200);
  assert.equal(full.body.items.length, 50);
  answer = {
    status: 200,
    body: JSON.stringify({ text: "x".repeat((1 << 20) + 1) }),
  };
  const excessive = await call(`${base}/messages`, { query: "?limit=50" });
  assert.equal(excessive.status, 503);
  assert.equal(excessive.body.code, "retry_later");
});

test("non-JSON upstream 401 still expires both authentication cookies without diagnostic leakage", async () => {
  answer = {
    status: 401,
    body: "MOCK_PRIVATE_UPSTREAM_DIAGNOSTIC",
    contentType: "text/html",
  };
  const result = await call(`${base}/messages`);
  assert.equal(result.status, 401);
  assert.equal(result.body.code, "unauthorized");
  const cookies = result.headers.get("set-cookie") ?? "";
  assert.match(cookies, /__Host-commerce_session=.*Max-Age=0/);
  assert.match(cookies, /__Host-commerce_csrf=.*Max-Age=0/);
  assert.match(result.headers.get("cache-control") ?? "", /no-store/);
  assert.doesNotMatch(
    JSON.stringify(result.body),
    /PRIVATE_UPSTREAM_DIAGNOSTIC/,
  );
});


test("real M7 response keeps no-referrer after matching Next production header rules", async () => {
  const nextConfig = (await import(pathToFileURL(adminRoot + "next.config.ts").href)).default("inbox-node-test");
  const rules = await nextConfig.headers();
  const { getPathMatch } = createRequire(adminRoot + "package.json")("next/dist/shared/lib/router/utils/path-match.js");
  const path = `live-sessions/${cid}/claims/bundles/${customer}/link`;
  for (const status of [200, 401, 403]) {
    answer = { status, body: JSON.stringify(status === 200 ? { token: "M".repeat(42) + "A", generation: 1,
      expires_at: "2099-01-01T00:00:00Z", released: false, replayed: false } : { code: status === 401 ? "unauthorized" : "forbidden" }) };
    const result = await call(path, { method: "POST", body: { expected_generation: 0, release_binding: false } });
    assert.equal(result.status, status);
    assert.equal(result.headers.get("referrer-policy"), "no-referrer", "actual handler with real Request");
    const effective = new Headers(result.headers);
    for (const rule of rules) {
      if (getPathMatch(rule.source)(`/api/stores/${store}/${path}`))
        for (const header of rule.headers) effective.set(header.key, header.value);
    }
    assert.equal(effective.get("referrer-policy"), "no-referrer", "global Next header rules must not replace M7 privacy policy");
  }
});
