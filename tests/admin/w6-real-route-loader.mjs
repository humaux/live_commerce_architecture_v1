// Purpose: load actual W6 Next route modules and their real auth/backend helpers for focused native-Request Node tests.
// Depends on: registerCustomerTagsLoader, installed Next server-only empty/headers.js, native fetch/Request and TS source.
// Used by: w6-route-seam.test.mjs and w6-integration.test.ts; only the upstream transport is faked.
import { registerHooks, createRequire } from "node:module";
import { AsyncLocalStorage } from "node:async_hooks";
import { existsSync } from "node:fs";
import { dirname, join } from "node:path";
import { pathToFileURL } from "node:url";
import { registerCustomerTagsLoader } from "./customer-tags-node-loader.mjs";

export const W6_STORE = "11111111-1111-4111-8111-111111111111";
export const W6_TAG = "22222222-2222-4222-8222-222222222222";
export const W6_ORIGIN = "https://admin.example.invalid";
const apiOrigin = "http://127.0.0.1:9100";
let loaded;

/** Resolve only installed/runtime modules and aliases; never replace localError, auth, cookie or CSRF functions. */
export async function loadW6RealRoutes() {
  if (!loaded) loaded = (async () => {
    const app = createRequire(new URL("../../apps/admin/package.json", import.meta.url));
    const nextRoot = dirname(app.resolve("next/package.json"));
    registerCustomerTagsLoader();
    // Next expects the standard Node primitive to be installed by its runtime bootstrap.
    globalThis.AsyncLocalStorage ??= AsyncLocalStorage;
    registerHooks({ resolve(specifier, context, nextResolve) {
      if (specifier === "server-only") return nextResolve(pathToFileURL(join(nextRoot, "dist/compiled/server-only/empty.js")).href, context);
      if (specifier === "next/headers") return nextResolve(pathToFileURL(join(nextRoot, "headers.js")).href, context);
      if (specifier.startsWith("@/")) {
        const base = new URL(`../../apps/admin/${specifier.slice(2)}`, import.meta.url);
        for (const extension of ["", ".ts", ".tsx", ".js"]) {
          const target = new URL(base.href + extension);
          if (existsSync(target)) return nextResolve(target.href, context);
        }
      }
      return nextResolve(specifier, context);
    } });
    Object.assign(process.env, { COMMERCE_IDENTITY_ENABLED: "1", COMMERCE_IDENTITY_ALLOW_LOOPBACK_TESTS: "1", COMMERCE_FIXTURE_ENABLED: "0",
      COMMERCE_PASSWORD_LOGIN_ENABLED: "0", COMMERCE_PUBLIC_ORIGIN: W6_ORIGIN, COMMERCE_API_ORIGIN: apiOrigin,
      COMMERCE_OIDC_ISSUER: "http://127.0.0.1:9101", COMMERCE_BFF_KEY: Buffer.alloc(32, 19).toString("base64url") });
    const customers = await import("../../apps/admin/app/api/stores/[store]/customers/route.ts");
    const reports = await import("../../apps/admin/app/api/stores/[store]/reports/[report]/route.ts");
    const note = await import("../../apps/admin/app/api/stores/[store]/customers/[customer]/notes/[note]/route.ts");
    const tag = await import("../../apps/admin/app/api/stores/[store]/customers/tags/[tag]/route.ts");
    const tags = await import("../../apps/admin/app/api/stores/[store]/customers/tags/route.ts");
    const auth = await import("../../apps/admin/lib/auth.ts");
    return { customers, reports, note, tag, tags, auth };
  })();
  return loaded;
}

/** Fake only upstream fetch while retaining native Requests, real route dispatch and every auth/CSRF/response fence. */
export async function w6RealBoundary() {
  const routes = await loadW6RealRoutes();
  const session = Buffer.alloc(32, 7).toString("base64url"), csrf = Buffer.alloc(32, 11).toString("base64url");
  const original = globalThis.fetch, calls = [];
  globalThis.fetch = async (input, init = {}) => {
    const url = new URL(String(input)); calls.push({ url, init, headers: new Headers(init.headers) });
    if (url.origin !== apiOrigin) throw new Error("unexpected upstream origin");
    if (url.pathname === "/v1/admin/stores") return Response.json({ items: [{ id: W6_STORE, name: "Synthetic", currency: "TWD", permissions: ["customers:read", "orders:read", "orders:export", "live:read"] }] });
    if (url.pathname.endsWith("/customers")) return Response.json({ items: [], next_cursor: "" });
    if (init.method === "DELETE" && url.pathname.endsWith(`/notes/${W6_TAG}`)) return Response.json({ note_id: W6_TAG, deleted: true });
    if (init.method === "DELETE" && url.pathname.endsWith(`/customers/tags/${W6_TAG}`)) return Response.json({ tag_id: W6_TAG, deleted: true });
    if (url.pathname.endsWith("/reports/products")) return Response.json({ from: "2026-09-01", to: "2026-09-30", timezone: "Asia/Taipei", truncated: false, rows: [] }, { headers: { "Cache-Control": "private, no-store" } });
    if (url.pathname.endsWith("/reports/products.csv")) return new Response("sku_id,product_id,code,name,currency,environment,units,captured_minor,refunded_minor,net_minor,offline_units,offline_minor\r\n", { headers: {
      "Cache-Control": "private, no-store", "Content-Type": "text/csv; charset=utf-8", "Content-Disposition": 'attachment; filename="report-products-2026-09-01-2026-09-30.csv"', "Set-Cookie": "untrusted-upstream=1" } });
    throw new Error("unexpected upstream path");
  };
  return { ...routes, calls, session, csrf, restore: () => { globalThis.fetch = original; },
    request(resource, headers = {}) { return new Request(`${W6_ORIGIN}/api/stores/${W6_STORE}/${resource}`, { headers: {
      Cookie: `${routes.auth.SESSION_COOKIE}=${session}; ${routes.auth.CSRF_COOKIE}=${csrf}; ignored-cookie=private`,
      Origin: W6_ORIGIN, "Sec-Fetch-Site": "same-origin", "X-CSRF-Token": csrf, ...headers,
    } }); },
    customerContext: { params: Promise.resolve({ store: W6_STORE }) },
    reportContext(report) { return { params: Promise.resolve({ store: W6_STORE, report }) }; },
  };
}
