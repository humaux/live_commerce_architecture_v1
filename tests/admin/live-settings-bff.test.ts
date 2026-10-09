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
let permissions = ["live:read", "live:manage", "inbox:read", "inbox:reply"];
const upstream = createServer((req, res) => {
  const chunks: Buffer[] = [];
  req.on("data", (chunk) => chunks.push(chunk));
  req.on("end", () => {
    res.setHeader("content-type", "application/json");
    res.setHeader("cache-control", "private, no-store");
    if (req.url === "/v1/admin/stores") {
      res.statusCode = storesStatus;
      res.end(
        JSON.stringify(
          storesStatus === 200
            ? {
                items: [
                  {
                    id: store,
                    name: "Synthetic store",
                    currency: "TWD",
                    permissions,
                  },
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
