import { callBackend, fixtureSession } from "@/lib/backend";
import {
  authConfig,
  authenticatedStores,
  clearAuthCookies,
  localError,
  requireCSRF,
  requireOrigin,
  safeError,
  sessionToken,
} from "@/lib/auth";

const uuid = "[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}";
const account = `provider-accounts(?:/${uuid})?`;
const setting = `markets/${uuid}/countries/[A-Z]{2}/(?:delivery-services|payment-methods)/[a-z][a-z0-9_-]{0,39}`;
const inspect = `markets/${uuid}/countries/[A-Z]{2}/payment-methods/[a-z][a-z0-9_-]{0,39}/inspect`;
const routes: Record<string, RegExp> = {
  GET: new RegExp(
    `^(catalog-ledger|products|warehouses|inventory|products/${uuid}/skus|${account}|${setting})$`,
  ),
  POST: new RegExp(
    `^(products|skus|warehouses|inventory/adjustments|products/${uuid}/archive|skus/${uuid}/(archive|price)|provider-accounts|provider-accounts/${uuid}/rotate|${inspect})$`,
  ),
  PATCH: new RegExp(`^(products/${uuid}|skus/${uuid})$`),
  PUT: new RegExp(`^${setting}$`),
};
const exactStore = new RegExp(`^${uuid}$`);
const inspectRoute = new RegExp(`^${inspect}$`);
type Context = { params: Promise<{ store: string; resource: string[] }> };

async function proxy(request: Request, context: Context) {
  const error = localError;
  const { store, resource } = await context.params;
  const path = resource.join("/");
  if (!exactStore.test(store) || !routes[request.method]?.test(path))
    return error(404, "not_found");
  const accountRoute = path.startsWith("provider-accounts");
  const inspection = request.method === "POST" && inspectRoute.test(path);
  // A local shared fixture bearer is never authority to intake provider secrets.
  if (accountRoute && !authConfig) return error(404, "not_found");
  // URL.search drops an empty trailing '?'. Exact resources must reject that too;
  // only account collection GET inherits the bounded pagination parser in Go.
  const exactResource =
    path.startsWith("markets/") ||
    (accountRoute &&
      !(request.method === "GET" && path === "provider-accounts"));
  if (exactResource && request.url.includes("?"))
    return error(422, "invalid_request");
  const url = new URL(request.url);
  let token: string | undefined;
  if (authConfig) {
    token = sessionToken(request) ?? undefined;
    if (!token) return error(401, "unauthorized");
    if (
      request.method !== "GET" &&
      (!requireOrigin(request) || !requireCSRF(request))
    )
      return error(403, "forbidden");
    const listed = await authenticatedStores(token);
    if (!listed.stores) {
      const denied = await safeError(listed.response);
      if (denied.status === 401) clearAuthCookies(denied.headers);
      return denied;
    }
    if (!listed.stores.some((item) => item.id === store))
      return error(404, "not_found");
  } else {
    const session = fixtureSession();
    if (!session) return error(401, "unauthorized");
    if (store !== session.storeID) return error(404, "not_found");
    if (url.hostname !== "127.0.0.1" && url.hostname !== "localhost")
      return error(403, "forbidden");
    const host = request.headers.get("host");
    if (host !== "127.0.0.1:3100" && host !== "localhost:3100")
      return error(403, "forbidden");
  }
  const init: RequestInit = { method: request.method };
  if (request.method !== "GET") {
    if (!authConfig) {
      const host = request.headers.get("host");
      if (request.headers.get("origin") !== `http://${host}`)
        return error(403, "forbidden");
    }
    if (
      request.headers.get("content-type")?.split(";")[0] !== "application/json"
    )
      return error(415, "json_required");
    const key = request.headers.get("idempotency-key") ?? "";
    if (!inspection && !/^[A-Za-z0-9_.:-]{8,128}$/.test(key))
      return error(422, "invalid_request");
    const reader = request.body?.getReader();
    if (!reader) return error(400, "invalid_json");
    const chunks: Uint8Array[] = [];
    let size = 0;
    while (true) {
      const part = await reader.read();
      if (part.done) break;
      size += part.value.byteLength;
      if (size > 65536) {
        await reader.cancel();
        return error(400, "invalid_json");
      }
      chunks.push(part.value);
    }
    const data = new Uint8Array(size);
    let at = 0;
    for (const chunk of chunks) {
      data.set(chunk, at);
      at += chunk.length;
    }
    init.body = new TextDecoder().decode(data);
    init.headers = {
      "Content-Type": "application/json",
      ...(!inspection ? { "Idempotency-Key": key } : {}),
    };
  }
  const response = await callBackend(path + url.search, init, token, store);
  if (authConfig && response.status === 401) {
    const denied = await safeError(response);
    clearAuthCookies(denied.headers);
    return denied;
  }
  if (!response.ok) return safeError(response);
  return new Response(response.body, {
    status: response.status,
    headers: {
      "Content-Type": "application/json",
      "Cache-Control": "no-store",
      "X-Request-ID": response.headers.get("x-request-id") ?? "",
    },
  });
}
export const GET = proxy;
export const POST = proxy;
export const PATCH = proxy;
export const PUT = proxy;
const unsupported = () =>
  localError(405, "method_not_allowed", "GET, POST, PATCH, PUT");
export const DELETE = unsupported;
export const OPTIONS = unsupported;
export const HEAD = unsupported;
