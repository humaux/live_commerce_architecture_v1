// Purpose: LC-U3 real-handler Request counterexamples for frozen A15/A16.
// Depends on: Node test/http/registerHooks; actual store catchall BFF and a synthetic Go HTTP boundary.
// Used by: focused Node gate and integrator independent verification; no real PII or provider traffic.
import assert from "node:assert/strict";
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
const upstream = createServer((req, res) => {
  const chunks: Buffer[] = [];
  req.on("data", (chunk) => chunks.push(chunk));
  req.on("end", () => {
    res.setHeader("content-type", "application/json");
    if (req.url === "/v1/admin/stores") {
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
const route = await import(
  pathToFileURL(
    adminRoot + "app/api/stores/[store]/tools/[...resource]/route.ts",
  ).href
);
const rootRoute = await import(pathToFileURL(adminRoot + "app/api/stores/[store]/[...resource]/route.ts").href);
test.after(() => upstream.close());
type Options = {
  root?: boolean;
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
    `${origin}/api/stores/${over.storeId ?? store}/${over.root ? "" : "tools/"}${path}${over.query ?? ""}`,
    {
      method,
      headers,
      ...(method !== "GET" && method !== "HEAD"
        ? { body: over.raw ?? JSON.stringify(over.body ?? {}) }
        : {}),
    },
  );
  const response = await (over.root ? rootRoute : route)[method](request, {
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
const item = () => ({
  sku_id: customer,
  offer_id: customer,
  keyword: "A1",
  name: "Synthetic item",
  variant: "SKU",
  quantity: 1,
  live_price_minor: null,
  catalog_price_minor: 100,
  sellable: true,
  live_quantity_remaining: 1,
});
const prefill = () => ({
  items: [item()],
  bundles: [customer],
  customer: null,
  last_delivery: null,
  suggested_option_key: null,
  live_price_eligible: false,
  live_price_reason: "no_conversation",
});
const body = () => ({
  items: [{ sku_id: customer, quantity: 1 }],
  customer: { name: "Synthetic buyer", phone: "0912345678", email: "" },
  delivery: {
    option_key: `${customer}|TW|home`,
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
  for: { bundle_ids: [customer], conversation_id: null },
  send_payment_link: false,
});
const result = () => ({
  order_id: customer,
  commercial_state: "AWAITING_TRANSFER",
  payment_mode: "bank_transfer",
  total_minor: 100,
  currency: "TWD",
  expires_at: "2030-01-01T00:00:00Z",
  buyer_link: null,
  link_state: "configured",
  source: "merchant_manual",
  live_price: "not_applied",
  live_price_reason: "no_conversation",
  send: { state: "not_sent", reason: "not_requested" },
});
test("A15 real Request exact XOR selectors, bearer/store authorization, private closed success", async () => {
  answer = { status: 200, body: JSON.stringify(prefill()) };
  const response = await call("inbox/order-prefill", {
    query: `?bundle_id=${customer}`,
  });
  assert.equal(response.status, 200);
  assert.match(response.headers.get("cache-control") ?? "", /no-store/);
  assert.equal(response.headers.get("referrer-policy"), "no-referrer");
  assert.deepEqual(response.body, prefill());
  assert.equal(
    seen.at(-1)?.url,
    `/v1/admin/stores/${store}/inbox/order-prefill?bundle_id=${customer}`,
  );
  assert.equal(seen.at(-1)?.headers.authorization, `Bearer ${session}`);
  assert.equal(seen.at(-1)?.headers.cookie, undefined);
  const before = seen.length;
  for (const query of [
    "",
    "?",
    `?bundle_id=${customer}&bundle_id=${customer}`,
    `?bundle_id=${customer}&conversation_id=${cid}`,
    `?conversation_id=${cid}&name=SECRET`,
  ])
    assert.equal((await call("inbox/order-prefill", { query })).status, 422);
  assert.equal(
    (
      await call("inbox/order-prefill", {
        query: `?conversation_id=${cid}`,
        cookie: "",
      })
    ).status,
    401,
  );
  assert.equal(
    (
      await call("inbox/order-prefill", {
        query: `?conversation_id=${cid}`,
        storeId: crypto.randomUUID(),
      })
    ).status,
    404,
  );
  assert.equal(seen.length, before);
  answer = {
    status: 200,
    body: JSON.stringify({ ...prefill(), secret: "PRIVATE_DIAGNOSTIC" }),
  };
  assert.equal(
    (await call("inbox/order-prefill", { query: `?bundle_id=${customer}` }))
      .status,
    503,
  );
});
test("A16 actual CSRF origin exact body/key, replay null-link and safe conflict detail", async () => {
  answer = { status: 201, body: JSON.stringify(result()) };
  const created = await call("orders/for-buyer", {
    method: "POST",
    body: body(),
  });
  assert.equal(created.status, 201);
  assert.deepEqual(created.body, result());
  assert.equal(seen.at(-1)?.headers["idempotency-key"], "synthetic-receipt");
  assert.deepEqual(JSON.parse(seen.at(-1)?.body ?? ""), body());
  const before = seen.length;
  for (const over of [
    { headers: { "x-csrf-token": "wrong" } },
    { headers: { origin: "https://other.invalid" } },
    { headers: { "idempotency-key": "bad" } },
    { query: "?extra=1" },
    { body: { ...body(), price_minor: 1 } },
    {
      body: {
        ...body(),
        for: { bundle_ids: [customer, customer], conversation_id: null },
      },
    },
  ]) {
    const res = await call("orders/for-buyer", {
      method: "POST",
      body: body(),
      ...over,
    });
    assert.ok([403, 422].includes(res.status));
  }
  assert.equal(seen.length, before);
  answer = {
    status: 409,
    body: JSON.stringify({
      code: "bundle_already_ordered",
      message: "PRIVATE_DIAGNOSTIC",
      details: { order_id: customer, secret: "PRIVATE_DIAGNOSTIC" },
    }),
  };
  const conflict = await call("orders/for-buyer", {
    method: "POST",
    body: body(),
  });
  assert.equal(conflict.status, 409);
  assert.deepEqual(conflict.body.details, { order_id: customer });
  assert.equal(
    JSON.stringify(conflict.body).includes("PRIVATE_DIAGNOSTIC"),
    false,
  );
  answer = {
    status: 409,
    body: JSON.stringify({
      code: "bundle_already_ordered",
      details: { order_id: null },
    }),
  };
  assert.deepEqual(
    (await call("orders/for-buyer", { method: "POST", body: body() })).body
      .details,
    { order_id: null },
  );
  answer = {
    status: 409,
    body: JSON.stringify({
      code: "bundle_already_ordered",
      details: { order_id: "PRIVATE_DIAGNOSTIC" },
    }),
  };
  assert.equal(
    (await call("orders/for-buyer", { method: "POST", body: body() })).status,
    503,
  );
  answer = {
    status: 409,
    body: JSON.stringify({
      code: "PRIVATE_DIAGNOSTIC",
      details: { order_id: customer },
    }),
  };
  const unknown = await call("orders/for-buyer", {
    method: "POST",
    body: body(),
  });
  assert.equal(unknown.status, 503);
  assert.equal(unknown.body.code, "retry_later");
});

test("drawer blocklist check exact real BFF GET and boolean-only private response", async () => {
  const resource = `live-sessions/${cid}/claims/blocklist/check`;
  answer = { status: 200, body: JSON.stringify({ restricted: true }) };
  const good = await call(resource, { root: true, query: `?bundle_id=${customer}` });
  assert.equal(good.status, 200);
  assert.deepEqual(good.body, { restricted: true });
  assert.match(good.headers.get("cache-control") ?? "", /no-store/);
  assert.equal(good.headers.get("referrer-policy"), "no-referrer");
  assert.equal(seen.at(-1)?.url, `/v1/admin/stores/${store}/${resource}?bundle_id=${customer}`);
  const before = seen.length;
  for (const query of ["", `?bundle_id=${customer}&bundle_id=${customer}`, `?bundle_id=${customer}&actor_key=x`, `?%62undle_id=${customer}`, `?bundle_id=${customer.replaceAll("-", "%2D")}`])
    assert.equal((await call(resource, { root: true, query })).status, 422);
  assert.equal((await call(resource, { root: true, method: "POST", query: `?bundle_id=${customer}` })).status, 405);
  assert.equal(seen.length, before);
  answer = { status: 201, body: JSON.stringify({ restricted: true }) };
  assert.equal((await call(resource, { root: true, query: `?bundle_id=${customer}` })).status, 503);
  for (const body of [{ restricted: "true" }, { restricted: true, note: "PRIVATE_BLOCK_NOTE" }, { actor_key: "PRIVATE_ACTOR" }, null]) {
    answer = { status: 200, body: JSON.stringify(body) };
    const bad = await call(resource, { root: true, query: `?bundle_id=${customer}` });
    assert.equal(bad.status, 503);
    assert.ok(!JSON.stringify(bad.body).includes("PRIVATE_"));
  }
});
