// Purpose: W3-U2 real Request/handler tests for settings, manual reminders, templates and restricted buyers.
// Depends on: Node test/http/registerHooks; actual store catchall BFF and a synthetic Go HTTP boundary.
// Used by: focused Node gate and integrator independent verification; no real PII or provider traffic.
import assert from "node:assert/strict";
import { existsSync } from "node:fs";
import { randomBytes } from "node:crypto";
import { createServer } from "node:http";
import { createRequire, registerHooks } from "node:module";
import type { AddressInfo } from "node:net";
import { test } from "node:test";
import { fileURLToPath, pathToFileURL } from "node:url";

const adminRoot = fileURLToPath(new URL("../../apps/admin/", import.meta.url));
registerHooks({
  resolve(specifier, context, next) {
    if (specifier === "next/server") return next("next/server.js", context);
    if (specifier === "next/constants")
      return {
        // Next's CJS constants need a named-export bridge under Node strip-types; values remain the installed module's.
        url:
          "data:text/javascript," +
          encodeURIComponent(
            `import constants from ${JSON.stringify(pathToFileURL(adminRoot + "node_modules/next/constants.js").href)}; export const PHASE_PRODUCTION_BUILD = constants.PHASE_PRODUCTION_BUILD;`,
          ),
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
    if (
      specifier.startsWith(".") &&
      !context.parentURL?.includes("/node_modules/") &&
      !/\.[a-z]+$/.test(specifier)
    )
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
let storeVisible = true;
let storesCalls = 0;
let afterResource: (() => void) | undefined;
let permissions: string[] | undefined = ["live:read", "live:manage", "inbox:read", "inbox:reply"];
let role: string | undefined;
const upstream = createServer((req, res) => {
  const chunks: Buffer[] = [];
  req.on("data", (chunk) => chunks.push(chunk));
  req.on("end", () => {
    res.setHeader("content-type", "application/json");
    res.setHeader("cache-control", "private, no-store");
    if (req.url === "/v1/admin/stores") {
      storesCalls++;
      res.statusCode = storesStatus;
      res.end(
        JSON.stringify(
          storesStatus === 200
            ? {
                items: storeVisible ? [
                  {
                    id: store,
                    name: "Synthetic store",
                    currency: "TWD",
                    permissions,
                    role,
                  },
                ] : [],
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
    afterResource?.();
    res.end(answer.body);
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

const sid = crypto.randomUUID(),
  bundle = crypto.randomUUID(),
  entry = crypto.randomUUID();
const roots = `live-sessions/${sid}`;
const leaves: Record<string, string> = {
  "live-settings/sold-out-reply": "live-settings/sold-out-reply",
  "message-templates": "message-templates",
  [`${roots}/reminders`]: "live-sessions/[session]/reminders",
  [`${roots}/claims/blocklist`]: "live-sessions/[session]/claims/blocklist",
  [`${roots}/claims/blocklist/check`]:
    "live-sessions/[session]/claims/blocklist/check",
  [`${roots}/claims/blocklist/entries/${entry}`]:
    "live-sessions/[session]/claims/blocklist/entries/[entry]",
};
async function call(
  path: string,
  method = "GET",
  body?: unknown,
  extra: Record<string, string> = {},
  storeId = store,
) {
  const routePath = path.split("?", 1)[0];
  const candidate =
    adminRoot + `app/api/stores/[store]/${leaves[routePath]}/route.ts`;
  // Before implementation the actual catchall handles the missing leaf and must fail the same positive assertions.
  const file = existsSync(candidate)
    ? candidate
    : adminRoot + "app/api/stores/[store]/[...resource]/route.ts";
  const handlers = await import(pathToFileURL(file).href);
  const headers = new Headers({
    cookie: `__Host-commerce_session=${session}; __Host-commerce_csrf=${csrf}`,
    origin,
    "x-csrf-token": csrf,
    ...(method !== "GET"
      ? { "idempotency-key": "settings-synthetic-key" }
      : {}),
    ...extra,
  });
  if (body !== undefined) headers.set("content-type", "application/json");
  const req = new Request(`${origin}/api/stores/${storeId}/${path}`, {
    method,
    headers,
    ...(body === undefined ? {} : { body: JSON.stringify(body) }),
  });
  const response: Response = await (handlers[method] ?? handlers.GET)(req, {
    params: Promise.resolve({
      store: storeId,
      session: sid,
      entry,
      resource: routePath.split("/"),
    }),
  });
  return {
    status: response.status,
    headers: response.headers,
    body: await response.json(),
  };
}
const setting = {
  enabled: false,
  template_id: "merchant-soldout",
  template_version: 2,
  version: 4,
};
const input = {
  enabled: false,
  template_id: "merchant-soldout",
  template_version: 2,
  expected_version: 3,
};
function reply(body: unknown, status = 200) {
  answer = { status, body: JSON.stringify(body) };
}
test.after(() => upstream.close());
for (const method of ["GET", "POST"]) {
  test(`reminder route absence ${method} is local only after fresh store and permission proof`, async () => {
    answer = { status: 404, body: "404 page not found\n", contentType: "text/plain; charset=utf-8" };
    const before = seen.length, authBefore = storesCalls;
    const result = await call(`${roots}/reminders`, method);
    assert.equal(result.status, 404);
    assert.equal(result.body.code, "reminders_unavailable");
    assert.match(result.headers.get("cache-control")!, /private.*no-store/);
    assert.equal(seen.length, before + 1, "one explicit request, including POST");
    assert.equal(storesCalls, authBefore + 2, "fresh scope proof after the missing route");
    assert.equal(seen.at(-1)!.body, "");
  });
  test(`reminder route absence ${method} never aliases an initially revoked store`, async () => {
    answer = { status: 404, body: "404 page not found\n", contentType: "text/plain" };
    storeVisible = false;
    const before = seen.length;
    try {
      const result = await call(`${roots}/reminders`, method);
      assert.equal(result.status, 404);
      assert.equal(result.body.code, "not_found");
      assert.equal(seen.length, before);
    } finally { storeVisible = true; }
  });
  test(`reminder route absence ${method} detects store loss at the fresh recheck`, async () => {
    answer = { status: 404, body: "404 page not found\n", contentType: "text/plain" };
    afterResource = () => { storeVisible = false; };
    try {
      const result = await call(`${roots}/reminders`, method);
      assert.equal(result.status, 404);
      assert.equal(result.body.code, "not_found");
    } finally { afterResource = undefined; storeVisible = true; }
  });
  test(`reminder route absence ${method} requires fresh complete method permissions`, async () => {
    const required = method === "GET" ? ["inbox:read"] : ["live:manage", "inbox:reply"];
    const original = permissions!;
    try {
      for (const missing of required) {
        permissions = original;
        answer = { status: 404, body: "404 page not found\n", contentType: "text/plain" };
        afterResource = () => { permissions = original.filter((p) => p !== missing); };
        const result = await call(`${roots}/reminders`, method);
        assert.equal(result.status, 403);
        assert.equal(result.body.code, "forbidden");
      }
    } finally { afterResource = undefined; permissions = original; }
  });
  test(`reminder route ${method} preserves JSON not_found and existing authorization failures`, async () => {
    for (const status of [404, 401, 403]) {
      const code = status === 404 ? "not_found" : status === 401 ? "unauthorized" : "forbidden";
      reply({ code, message: "PRIVATE-DIAGNOSTIC" }, status);
      const before = storesCalls, writes = seen.length;
      const result = await call(`${roots}/reminders`, method);
      assert.equal(result.status, status);
      assert.equal(result.body.code, code);
      assert.ok(!JSON.stringify(result.body).includes("PRIVATE"));
      assert.equal(storesCalls, before + 1);
      assert.equal(seen.length, writes + 1);
      if (status === 401) assert.match(result.headers.get("set-cookie")!, /Max-Age=0/);
    }
    reply({ code: "reminders_unavailable" }, 404);
    assert.equal((await call(`${roots}/reminders`, method)).body.code, "retry_later");
  });
  test(`reminder route absence ${method} refuses unknown grants but accepts a current owner`, async () => {
    const original = permissions;
    try {
      answer = { status: 404, body: "404 page not found\n", contentType: "text/plain" };
      afterResource = () => { permissions = undefined; };
      assert.equal((await call(`${roots}/reminders`, method)).body.code, "forbidden");
      role = "owner";
      assert.equal((await call(`${roots}/reminders`, method)).body.code, "reminders_unavailable");
    } finally { afterResource = undefined; permissions = original; role = undefined; }
  });
  test(`reminder route absence ${method} fails closed when fresh authentication cannot be proved`, async () => {
    for (const status of [401, 403, 503]) {
      answer = { status: 404, body: "404 page not found\n", contentType: "text/plain" };
      afterResource = () => { storesStatus = status; };
      try {
        const result = await call(`${roots}/reminders`, method);
        assert.equal(result.status, status);
        assert.equal(result.body.code, status === 401 ? "unauthorized" : status === 403 ? "forbidden" : "retry_later");
        if (status === 401) assert.match(result.headers.get("set-cookie")!, /Max-Age=0/);
      } finally { afterResource = undefined; storesStatus = 200; }
    }
  });
}
test("successful reminder requests retain one store auth check and one command dispatch", async () => {
  for (const method of ["GET", "POST"]) {
    reply(method === "GET"
      ? { sent: [], queued: 0, failed: [], followup: [], link: null }
      : { queued: 1, already_reminded: 0, followup: 0, restricted: 0, refused: 0, truncated: false });
    const before = storesCalls, calls = seen.length;
    assert.equal((await call(`${roots}/reminders`, method)).status, 200);
    assert.equal(storesCalls, before + 1);
    assert.equal(seen.length, calls + 1);
  }
});
test("reminder absence proof rejects arbitrary text, wrong media types and unbounded bodies", async () => {
  for (const [body, contentType] of [
    ["PRIVATE missing store", "text/plain"],
    ["404 page not found\n", "text/html"],
    ["404 page not found\n", "application/json"],
    ["404 page not found\n" + "x".repeat(1024 * 1024), "text/plain"],
  ]) {
    answer = { status: 404, body, contentType };
    const before = storesCalls;
    const result = await call(`${roots}/reminders`);
    assert.equal(result.status, 503);
    assert.equal(result.body.code, "retry_later");
    assert.equal(storesCalls, before + 1);
    assert.ok(!JSON.stringify(result.body).includes("PRIVATE"));
  }
  answer = { status: 404, body: "404 page not found\n", contentType: "text/plain" };
  assert.equal((await call("live-settings/sold-out-reply")).body.code, "retry_later");
});
test("settings actual route permits only auth-scoped live reads/writes and keeps CAS receipt", async () => {
  reply(setting);
  let r = await call("live-settings/sold-out-reply");
  assert.equal(r.status, 200);
  assert.deepEqual(r.body, setting);
  assert.match(r.headers.get("cache-control")!, /private.*no-store/);
  r = await call("live-settings/sold-out-reply", "PUT", input);
  assert.equal(r.status, 200);
  assert.equal(
    seen.at(-1)!.headers["idempotency-key"],
    "settings-synthetic-key",
  );
  assert.deepEqual(JSON.parse(seen.at(-1)!.body), input);
  const n = seen.length;
  assert.equal(
    (
      await call("live-settings/sold-out-reply", "PUT", {
        ...input,
        tenant_id: store,
      })
    ).status,
    422,
  );
  assert.equal(
    (
      await call("live-settings/sold-out-reply", "PUT", input, {
        "x-csrf-token": "",
      })
    ).status,
    403,
  );
  assert.equal(
    (
      await call(
        "live-settings/sold-out-reply",
        "GET",
        undefined,
        {},
        crypto.randomUUID(),
      )
    ).status,
    404,
  );
  permissions = ["live:read"];
  assert.equal(
    (await call("live-settings/sold-out-reply", "PUT", input)).status,
    403,
  );
  permissions = ["live:read", "live:manage", "inbox:read", "inbox:reply"];
  assert.equal(seen.length, n);
  reply({ code: "conflict", message: "PRIVATE diagnostic" }, 409);
  r = await call("live-settings/sold-out-reply", "PUT", input);
  assert.equal(r.status, 409);
  assert.equal(r.body.code, "conflict");
  assert.ok(!JSON.stringify(r.body).includes("PRIVATE"));
});
test("bodyless reminder/DELETE, private blocklist projection and restricted followup copy are exact", async () => {
  reply({
    queued: 1,
    already_reminded: 0,
    followup: 1,
    restricted: 1,
    refused: 0,
    truncated: false,
    results: [],
  });
  assert.equal((await call(`${roots}/reminders`, "POST")).status, 200);
  assert.equal(seen.at(-1)!.body, "");
  const n = seen.length;
  assert.equal((await call(`${roots}/reminders`, "POST", {})).status, 422);
  assert.equal(seen.length, n);
  reply({
    items: [
      {
        id: entry,
        platform: "facebook",
        note: "Synthetic note",
        source_bundle_id: bundle,
        created_at: "2026-10-01T00:00:00Z",
        display_name: "NEVER-IN-BLOCKLIST",
        actor_key: "NEVER-RETURN",
      },
    ],
    next_cursor: "",
  });
  const list = await call(`${roots}/claims/blocklist`);
  assert.equal(list.status, 200);
  assert.ok(!JSON.stringify(list.body).includes("NEVER"));
  reply({ restricted: true });
  assert.deepEqual(
    (await call(`${roots}/claims/blocklist/check?bundle_id=${bundle}`)).body,
    { restricted: true },
  );
  reply({ removed: true });
  assert.equal(
    (await call(`${roots}/claims/blocklist/entries/${entry}`, "DELETE")).status,
    200,
  );
  assert.equal(seen.at(-1)!.body, "");
  reply({
    sent: [],
    queued: 0,
    failed: [],
    followup: [
      {
        bundle_id: bundle,
        display_name: null,
        reminder_state: "claimed",
        reason: "restricted",
        link_copy_allowed: true,
      },
    ],
    link: "https://buyer.example/zh-TW/checkout",
  });
  const report = await call(`${roots}/reminders`);
  assert.equal(report.status, 200);
  assert.equal(report.body.followup[0].link_copy_allowed, false);
});
test("real template publication rejects unknown fields and auth failures clear cookies even without JSON", async () => {
  reply({
    template_id: "merchant-soldout",
    version: 1,
    kinds: ["private_reply"],
    public_safe: false,
  });
  const data = {
    template_id: "merchant-soldout",
    name: "Synthetic reply",
    body: "{{product.name}} sold out",
    kinds: ["private_reply"],
    public_safe: false,
  };
  assert.equal((await call("message-templates", "POST", data)).status, 200);
  const n = seen.length;
  assert.equal(
    (
      await call("message-templates", "POST", {
        ...data,
        actor_key: "forbidden",
      })
    ).status,
    422,
  );
  assert.equal(seen.length, n);
  answer = { status: 401, body: "PRIVATE HTML", contentType: "text/html" };
  const r = await call("live-settings/sold-out-reply");
  assert.equal(r.status, 401);
  assert.match(r.headers.get("set-cookie")!, /Max-Age=0/);
  assert.ok(!JSON.stringify(r.body).includes("PRIVATE"));
});

// Exercise the shared exact Next route used by Inbox, with Go's Unicode-rune name contract.
const compatibleTemplate = {
  template_id: "merchant-compatible",
  version: 1,
  kinds: ["private_reply"],
  public_safe: false,
  name: "Synthetic reply",
  created_at: "2026-10-09T00:00:00Z",
};
for (const [label, rune] of [["BMP", "名"], ["astral", "😀"]]) {
  test(`template compatibility GET preserves a legal 120-rune ${label} name`, async () => {
    const item = { ...compatibleTemplate, name: rune.repeat(120) };
    reply({ items: [item], actor_key: "NEVER-PROJECT" });
    const result = await call("message-templates");
    assert.equal(result.status, 200);
    assert.deepEqual(result.body, { items: [item] });
    assert.match(result.headers.get("cache-control")!, /private.*no-store/);
  });
  test(`template compatibility POST forwards a legal 120-rune ${label} name`, async () => {
    const { template_id, version, kinds, public_safe } = compatibleTemplate;
    reply({ template_id, version, kinds, public_safe });
    const data = { template_id, name: rune.repeat(120), body: "Sold out", kinds, public_safe };
    const result = await call("message-templates", "POST", data);
    assert.equal(result.status, 200);
    assert.deepEqual(result.body, { template_id, version, kinds, public_safe });
    assert.deepEqual(JSON.parse(seen.at(-1)!.body), data);
  });
}
for (const count of [501, 5001]) {
  test(`template compatibility GET preserves all ${count} legal templates under the byte bound`, async () => {
    const items = Array.from({ length: count }, (_, n) => ({
      ...compatibleTemplate,
      template_id: `merchant-${n}`,
    }));
    assert.ok(Buffer.byteLength(JSON.stringify({ items })) < 1024 * 1024);
    reply({ items });
    const result = await call("message-templates");
    assert.equal(result.status, 200);
    assert.deepEqual(result.body, { items });
  });
}
test("template compatibility rejects 121-rune names on GET and before POST forwarding", async () => {
  for (const rune of ["名", "😀"]) {
    reply({ items: [{ ...compatibleTemplate, name: rune.repeat(121) }] });
    assert.equal((await call("message-templates")).status, 503);
    const before = seen.length;
    const { template_id, kinds, public_safe } = compatibleTemplate;
    const result = await call("message-templates", "POST", {
      template_id, name: rune.repeat(121), body: "Sold out", kinds, public_safe,
    });
    assert.equal(result.status, 422);
    assert.equal(seen.length, before);
  }
});
test("template compatibility retains the response byte bound and item shape checks", async () => {
  const items = Array.from({ length: 8000 }, (_, n) => ({
    ...compatibleTemplate,
    template_id: `merchant-${n}`,
  }));
  assert.ok(Buffer.byteLength(JSON.stringify({ items })) > 1024 * 1024);
  reply({ items });
  let result = await call("message-templates");
  assert.equal(result.status, 503);
  assert.equal(result.body.code, "retry_later");
  assert.equal(Object.hasOwn(result.body, "items"), false);
  reply({ items: [{ ...compatibleTemplate, public_safe: "false" }] });
  result = await call("message-templates");
  assert.equal(result.status, 503);
  assert.equal(result.body.code, "retry_later");
  assert.equal(Object.hasOwn(result.body, "items"), false);
});

test("success receipts must describe the exact requested command, otherwise the result remains unknown", async () => {
  reply({ ...setting, enabled: true });
  assert.equal(
    (await call("live-settings/sold-out-reply", "PUT", input)).status,
    503,
  );
  reply({
    template_id: "different-merchant",
    version: 7,
    kinds: ["private_reply"],
    public_safe: false,
  });
  assert.equal(
    (
      await call("message-templates", "POST", {
        template_id: "merchant-soldout",
        name: "Reply",
        body: "Sold out",
        kinds: ["private_reply"],
        public_safe: false,
      })
    ).status,
    503,
  );
});

// Real middleware must admit the same exact resources before the route auth boundary.
test("W3-U2 Next proxy admits settings routes without weakening Studio raw-path guards", async () => {
  const {NextRequest}=await import("../../apps/admin/node_modules/next/server.js");
  const {proxy}=await import("../../apps/admin/proxy.ts");
  const root=`live-sessions/${cid}`;
  const invoke=(path:string,method="GET",query="")=>proxy(new NextRequest(`${origin}/api/stores/${store}/${path}${query}`,{method}));
  for(const [path,method,query] of [[`${root}/reminders`,"GET",""],[`${root}/reminders`,"POST",""],[`${root}/claims/blocklist`,"GET","?limit=20&cursor=opaque"],[`${root}/claims/blocklist`,"POST",""],[`${root}/claims/blocklist/check`,"GET",`?bundle_id=${customer}`],[`${root}/claims/blocklist/entries/${customer}`,"DELETE",""],["live-settings/sold-out-reply","PUT",""],["message-templates","POST",""]]) {
    const response=invoke(path,method,query);
    assert.equal(response.status,200,`${method} ${path}`);
    assert.equal(response.headers.get("x-middleware-next"),"1");
  }
  assert.equal(invoke(`${root}/reminders`,"PUT").status,405);
  assert.equal(invoke(`${root}/reminders`,"GET","?note=forbidden").status,422);
  assert.equal(invoke(`${root}/claims/blocklist`,"GET","?limit=51").status,422);
  assert.equal(invoke(`${root}/claims/blocklist/check`).status,422);
  for(const path of [`${root}/%72eminders`,`${root}/reminders/extra`,`${root}/claims/blocklist/unknown`]) assert.equal(invoke(path).status,404);
});
