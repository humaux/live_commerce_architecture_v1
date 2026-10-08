// Purpose: node test that drives the REAL admin BFF route handler (app/api/stores/[store]/[...resource]/route.ts) for the W3-07B
//   parcel routes with a stub Go upstream: the bodyless dissolve DELETE (P1: Next 16 hands every non-GET request an empty body
//   stream, which the old `request.body !== null` fence turned into a permanent 422), the mandatory CAS query on every parcel
//   DELETE, and the new GET parcel-groups read (exact grammar, no query).
// Depends on: apps/admin/app/api/stores/[store]/[...resource]/route.ts and its lib imports; node:module registerHooks (resolves the
//   `@/` alias, extensionless lib imports, `server-only`); a loopback http stub standing in for the Go API (MOCK).
// Used by: scripts/dev/test-local.sh --browser-merchant-orders-bff and --browser-merchant-orders-ui; scripts/dev/test-node.sh.
// Invariants: a DELETE with an empty body stream and no content-length/transfer-encoding is the normal browser request and must
//   reach Go with the CAS query, no body and no Idempotency-Key; every parcel DELETE must carry exactly ?expected_version=N.
// Status: MOCK (stub upstream, no PG, no browser).
// Run: node --test --experimental-strip-types tests/admin/parcels-bff.test.ts
import assert from "node:assert/strict";
import { randomBytes } from "node:crypto";
import { existsSync } from "node:fs";
import { createServer } from "node:http";
import { registerHooks } from "node:module";
import type { AddressInfo } from "node:net";
import { test } from "node:test";

// route.ts imports through the Next `@/` alias and extensionless specifiers and `server-only`, none of which plain Node resolves.
// One resolve hook (no package added) maps them; `next/headers` loads fine because the handler never calls headers() on this path.
const adminRoot = new URL("../../apps/admin/", import.meta.url);
registerHooks({
  resolve(specifier, context, next) {
    if (specifier === "server-only") return { url: "data:text/javascript,", shortCircuit: true };
    if (/^next\/[a-z-]+$/.test(specifier)) return next(`${specifier}.js`, context); // next ships CJS files without an exports map
    const rel = specifier.startsWith("@/") ? new URL(specifier.slice(2), adminRoot) : /^\.\.?\//.test(specifier) && context.parentURL?.startsWith(adminRoot.href) && !context.parentURL.includes("/node_modules/") ? new URL(specifier, context.parentURL) : null;
    if (rel && !/\.[cm]?[jt]sx?$|\.json$/.test(rel.pathname)) {
      const ts = new URL(`${rel.href}.ts`);
      return next(existsSync(ts) ? ts.href : rel.href, context);
    }
    return next(rel ? rel.href : specifier, context);
  },
});

const store = "11111111-1111-4111-8111-111111111111";
const group = "dddddddd-dddd-4ddd-8ddd-dddddddddddd";
type Seen = { method: string; url: string; headers: Record<string, string | string[] | undefined>; body: string };
const seen: Seen[] = [];
const upstream = createServer((req, res) => {
  const chunks: Buffer[] = [];
  req.on("data", (c: Buffer) => chunks.push(c));
  req.on("end", () => {
    seen.push({ method: req.method ?? "", url: req.url ?? "", headers: { ...req.headers }, body: Buffer.concat(chunks).toString() });
    res.writeHead(200, { "content-type": "application/json" });
    if (req.url === "/v1/admin/stores") res.end(JSON.stringify({ items: [{ id: store, name: "S", currency: "TWD" }] }));
    else if (req.method === "DELETE") res.end(JSON.stringify({ id: group, state: "DISSOLVED", version: 4 }));
    else res.end(JSON.stringify({ items: [] }));
  });
});
await new Promise<void>((ok) => upstream.listen(0, "127.0.0.1", ok));
test.after(() => upstream.close());

const publicOrigin = "https://admin.example.test";
Object.assign(process.env, {
  COMMERCE_IDENTITY_ENABLED: "1",
  COMMERCE_OIDC_ISSUER: "https://idp.example.test",
  COMMERCE_BFF_KEY: randomBytes(32).toString("base64url"),
  COMMERCE_PUBLIC_ORIGIN: publicOrigin,
  COMMERCE_API_ORIGIN: `http://127.0.0.1:${(upstream.address() as AddressInfo).port}`,
});
delete process.env.COMMERCE_FIXTURE_ENABLED;
const route = await import("../../apps/admin/app/api/stores/[store]/[...resource]/route.ts");

const session = randomBytes(32).toString("base64url");
const csrf = randomBytes(32).toString("base64url");
const auth = { origin: publicOrigin, cookie: `__Host-commerce_session=${session}; __Host-commerce_csrf=${csrf}`, "x-csrf-token": csrf };
const url = (path: string, query = "") => `${publicOrigin}/api/stores/${store}/${path}${query}`;
const call = (handler: (r: Request, c: { params: Promise<{ store: string; resource: string[] }> }) => Promise<Response>, method: string, path: string, query = "", headers: Record<string, string> = {}) => {
  // Next 16 gives every non-GET request a body stream even when the client sent none: model exactly that (an empty, closed stream).
  const body = method === "GET" ? undefined : new ReadableStream({ start: (c) => c.close() });
  const request = new Request(url(path, query), { method, headers: { ...auth, ...headers }, body, ...(body ? { duplex: "half" } : {}) } as RequestInit);
  seen.length = 0;
  return handler(request, { params: Promise.resolve({ store, resource: path.split("/") }) });
};
const backendCalls = () => seen.filter((s) => s.url !== "/v1/admin/stores");

test("bodyless dissolve DELETE (empty body stream, no content-length) reaches Go with the CAS query, no body, no key", async () => {
  const response = await call(route.DELETE, "DELETE", `parcel-groups/${group}`, "?expected_version=3");
  assert.equal(response.status, 200);
  assert.equal(response.headers.get("cache-control"), "private, no-store");
  assert.deepEqual(await response.json(), { id: group, state: "DISSOLVED", version: 4 });
  const [forwarded, ...rest] = backendCalls();
  assert.equal(rest.length, 0);
  assert.equal(forwarded.method, "DELETE");
  assert.equal(forwarded.url, `/v1/admin/stores/${store}/parcel-groups/${group}?expected_version=3`);
  assert.equal(forwarded.body, "");
  assert.equal(forwarded.headers["idempotency-key"], undefined);
});

test("every parcel DELETE must carry exactly one expected_version >= 1 (never forwarded without it)", async () => {
  for (const query of ["", "?", "?expected_version=0", "?expected_version=", "?expected_version=3&x=1", "?x=1", "?expected_version=3&expected_version=4", "?expected_version=03", "?expected_version=-1"]) {
    const response = await call(route.DELETE, "DELETE", `parcel-groups/${group}`, query);
    assert.equal(response.status, 422, query);
    assert.equal(backendCalls().length, 0, `${query} must not reach Go`);
  }
});

test("a DELETE that declares a body or chunked framing is refused", async () => {
  for (const headers of [{ "content-length": "2" }, { "transfer-encoding": "chunked" }, { "idempotency-key": "abcdefgh12345678" }]) {
    const response = await call(route.DELETE, "DELETE", `parcel-groups/${group}`, "?expected_version=3", headers);
    assert.equal(response.status, 422, JSON.stringify(headers));
    assert.equal(backendCalls().length, 0);
  }
});

test("DELETE needs the session, origin and CSRF like every command; other DELETEs stay unsupported", async () => {
  assert.equal((await call(route.DELETE, "DELETE", `parcel-groups/${group}`, "?expected_version=3", { "x-csrf-token": randomBytes(32).toString("base64url") })).status, 403);
  assert.equal((await call(route.DELETE, "DELETE", `parcel-groups/${group}`, "?expected_version=3", { origin: "https://evil.example.test" })).status, 403);
  assert.equal((await call(route.DELETE, "DELETE", `parcel-groups/${group}`, "?expected_version=3", { cookie: "" })).status, 401);
  assert.equal((await call(route.DELETE, "DELETE", `orders/${group}`)).status, 405);
  assert.equal((await call(route.DELETE, "DELETE", "parcel-groups", "?expected_version=3")).status, 405);
  assert.equal(backendCalls().length, 0);
});

test("GET parcel-groups (open groups read) and merge-suggestions pass with no query; a query or key is refused", async () => {
  for (const path of ["parcel-groups", "orders/merge-suggestions"]) {
    const ok = await call(route.GET, "GET", path);
    assert.equal(ok.status, 200, path);
    assert.equal(ok.headers.get("cache-control"), "private, no-store");
    assert.equal(backendCalls()[0].url, `/v1/admin/stores/${store}/${path}`);
    for (const query of ["?limit=5", "?"]) assert.equal((await call(route.GET, "GET", path, query)).status, 422, `${path}${query}`);
    assert.equal((await call(route.GET, "GET", path, "", { "idempotency-key": "abcdefgh12345678" })).status, 422);
  }
});
