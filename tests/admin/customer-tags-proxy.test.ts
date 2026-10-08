// Purpose: adversarial DB-free tests of the real W6-01B BFF forwarding boundary.
// Depends on: customer-tags-proxy closed grammar, synthetic injected server authority.
// Used by: W6-U1 local Node evidence; browser and real auth middleware remain separate gates.
import assert from "node:assert/strict";
import { test } from "node:test";
import { proxyCustomerTags, type TagProxyAdapter } from "../../apps/admin/lib/customer-tags-proxy.ts";

const store = "abcdef11-1111-4111-8111-111111111111";
const customer = "22222222-2222-4222-8222-222222222222";
const origin = "http://localhost:3100";
function harness(authority: Response | string = "synthetic-authority", reply?: Response) {
  const calls: { resource: string; init: RequestInit; token: string; store: string }[] = [];
  let auth = 0;
  const adapter: TagProxyAdapter = {
    authorize: async () => { auth++; return authority; },
    forward: async (resource, init, token, selected) => { calls.push({ resource, init, token, store: selected });
      return reply ?? Response.json({ note_id: customer }); },
    body: async (r) => r.text(), error: (status, code) => Response.json({ code }, { status, headers: { "Cache-Control": "private, no-store" } }),
  };
  return { adapter, calls, auth: () => auth };
}
function req(method: string, resource: string, body?: unknown, extra?: Record<string, string>) {
  return new Request(`${origin}/api/stores/${store}/${resource}`, { method,
    headers: { ...(method === "GET" ? {} : { "Idempotency-Key": "synthetic-key-1234" }), ...(body === undefined ? {} : { "Content-Type": "application/json" }), ...extra },
    ...(body === undefined ? {} : { body: JSON.stringify(body) }),
  });
}

test("bad path/store/method/query/header is refused without authority or backend calls", async () => {
  for (const [method, resource, selected, headers] of [
    ["GET", "customers/tags", "wrong", {}], ["PUT", "customers/tags", store, {}],
    ["GET", "customers/tags?extra=x", store, {}], ["POST", "customers/tags", store, { "Idempotency-Key": "short" }],
    ["GET", "customers/tags", store, { "Idempotency-Key": "synthetic-key-1234" }],
  ] as const) {
    const h = harness(); const resourcePath = resource.split("?")[0];
    assert.ok((await proxyCustomerTags(req(method, resource, method === "POST" ? {} : undefined, headers), selected, resourcePath, h.adapter)).status >= 400);
    assert.equal(h.auth(), 0); assert.equal(h.calls.length, 0);
  }
});
test("the actual proxy admits a tagged customer list with scoped search/cursor and rejects malformed tag authority", async () => {
  const h = harness();
  const path = `customers?limit=50&q=Buyer&after=Abc-_1&tag=${customer}`;
  const response = await proxyCustomerTags(req("GET", path), store, "customers", h.adapter);
  assert.equal(response.status, 200);
  assert.equal(h.calls.length, 1);
  assert.equal(h.calls[0].resource, path);
  assert.equal(h.calls[0].store, store);
  for (const query of ["tag=bad", `tag=${customer}&tag=${customer}`, `tag=${customer}&tenant_id=${store}`, `tag=${customer}&limit=101`]) {
    const refused = harness();
    assert.equal((await proxyCustomerTags(req("GET", `customers?${query}`), store, "customers", refused.adapter)).status, 422);
    assert.equal(refused.auth(), 0);
    assert.equal(refused.calls.length, 0);
  }
});
test("signed-out/cross-store/Origin-CSRF denials short circuit before body or forwarding", async () => {
  for (const status of [401, 403, 404]) {
    const h = harness(Response.json({ code: "forbidden" }, { status }));
    h.adapter.body = async () => { throw new Error("must not read body after denied authority"); };
    assert.equal((await proxyCustomerTags(req("POST", "customers/tags", {}), store, "customers/tags", h.adapter)).status, status);
    assert.equal(h.calls.length, 0);
  }
});
test("closed bodies and DELETE payloads cannot reach Go", async () => {
  for (const [method, resource, body] of [
    ["POST", "customers/tags", { name: "VIP", color: "pink" }],
    ["POST", "customers/tags", { name: "VIP", color: "blue", tenant_id: store }],
    ["PUT", `customers/${customer}/tags`, { tag_ids: [], tags_revision: "a".repeat(64) }],
    ["PATCH", `customers/${customer}/notes/${customer}`, { body: "synthetic", version: 0 }],
    ["POST", `customers/${customer}/notes`, { body: "x".repeat(1001) }],
    ["DELETE", `customers/tags/${customer}`, {}],
  ] as const) {
    const h = harness();
    assert.equal((await proxyCustomerTags(req(method, resource, body), store, resource, h.adapter)).status, 422);
    assert.equal(h.calls.length, 0);
  }
});
test("all write methods preserve exact key/body/store and private success status", async () => {
  for (const [method, resource, body] of [
    ["POST", "customers/tags", { name: "VIP", color: "blue" }],
    ["PATCH", `customers/tags/${customer}`, { color: "teal" }],
    ["DELETE", `customers/tags/${customer}`, undefined],
    ["PUT", `customers/${customer}/tags`, { tag_ids: [], revision: "a".repeat(64) }],
    ["POST", `customers/${customer}/notes`, { body: "synthetic" }],
    ["PATCH", `customers/${customer}/notes/${customer}`, { body: "synthetic", version: 2 }],
    ["DELETE", `customers/${customer}/notes/${customer}`, undefined],
  ] as const) {
    const h = harness("synthetic-authority", Response.json({ ok: true }, { status: method === "POST" ? 201 : 200 }));
    const response = await proxyCustomerTags(req(method, resource, body), store, resource, h.adapter);
    assert.equal(response.status, method === "POST" ? 201 : 200);
    assert.equal(h.calls.length, 1); const call = h.calls[0];
    assert.equal(call.init.method, method); assert.equal(call.resource, resource); assert.equal(call.store, store);
    assert.equal(call.token, "synthetic-authority");
    assert.equal(new Headers(call.init.headers).get("Idempotency-Key"), "synthetic-key-1234");
    assert.equal(call.init.body, body === undefined ? undefined : JSON.stringify(body));
    assert.equal(response.headers.get("cache-control"), "private, no-store"); assert.equal(response.headers.get("vary"), "Cookie");
  }
});
test("note pagination preserved and hostile backend errors/bodies sanitized", async () => {
  const h = harness();
  const resource = `customers/${customer}/notes`;
  await proxyCustomerTags(req("GET", `${resource}?limit=100&after=Abc-_1`), store, resource, h.adapter);
  assert.equal(h.calls[0].resource, `${resource}?limit=100&after=Abc-_1`);
  const bad = harness("synthetic-authority", Response.json({ code: "forbidden", body: "must never echo synthetic body" }, { status: 403 }));
  const response = await proxyCustomerTags(req("GET", "customers/tags"), store, "customers/tags", bad.adapter);
  assert.deepEqual(await response.json(), { code: "forbidden" });
  for (const reply of [new Response("bad", { headers: { "Content-Type": "text/html" } }), Response.json({ huge: "x".repeat((1 << 20) + 1) })]) {
    const oversize = harness("synthetic-authority", reply);
    assert.equal((await proxyCustomerTags(req("GET", "customers/tags"), store, "customers/tags", oversize.adapter)).status, 503);
  }
});
