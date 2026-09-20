import { callBackend, fixtureSession } from "@/lib/backend";

const uuid = "[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}";
const routes: Record<string, RegExp> = {
  GET: new RegExp(
    `^(catalog-ledger|products|warehouses|inventory|products/${uuid}/skus)$`,
  ),
  POST: new RegExp(
    `^(products|skus|warehouses|inventory/adjustments|products/${uuid}/archive|skus/${uuid}/(archive|price))$`,
  ),
  PATCH: new RegExp(`^(products/${uuid}|skus/${uuid})$`),
};
type Context = { params: Promise<{ store: string; resource: string[] }> };

const messages: Record<string, string> = {
  unauthorized: "Sign-in required.",
  not_found: "Resource not found.",
  forbidden: "Access denied.",
  json_required: "JSON content required.",
  invalid_request: "Invalid request.",
  invalid_json: "Invalid JSON body.",
  method_not_allowed: "Method not allowed.",
};
function localError(status: number, code: string) {
  const requestID = crypto.randomUUID().replaceAll("-", "");
  return Response.json(
    {
      code,
      message: messages[code] ?? "Request failed.",
      request_id: requestID,
      retryable: false,
      details: {},
    },
    {
      status,
      headers: {
        "Cache-Control": "no-store",
        "X-Request-ID": requestID,
        ...(status === 405 ? { Allow: "GET, POST, PATCH" } : {}),
      },
    },
  );
}

async function proxy(request: Request, context: Context) {
  const error = localError;
  const session = fixtureSession();
  if (!session) return error(401, "unauthorized");
  const { store, resource } = await context.params;
  if (store !== session.storeID) return error(404, "not_found");
  const path = resource.join("/");
  if (!routes[request.method]?.test(path)) return error(404, "not_found");
  const url = new URL(request.url);
  if (url.hostname !== "127.0.0.1" && url.hostname !== "localhost")
    return error(403, "forbidden");
  // Next's dev proxy normalizes request.url to localhost even when the browser
  // uses 127.0.0.1. Bind Origin to an exact allowed Host, never forwarded headers
  // or an arbitrary Host-derived origin (which would admit DNS rebinding).
  const host = request.headers.get("host");
  if (host !== "127.0.0.1:3100" && host !== "localhost:3100")
    return error(403, "forbidden");
  const init: RequestInit = { method: request.method };
  if (request.method !== "GET") {
    // Same-origin writes only; never forward browser-supplied authorization or
    // tenant headers. The development fixture has no cookie-based authority.
    if (request.headers.get("origin") !== `http://${host}`)
      return error(403, "forbidden");
    if (
      request.headers.get("content-type")?.split(";")[0] !== "application/json"
    )
      return error(415, "json_required");
    const key = request.headers.get("idempotency-key") ?? "";
    if (!/^[A-Za-z0-9_.:-]{8,128}$/.test(key))
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
      "Idempotency-Key": key,
    };
  }
  const response = await callBackend(path + url.search, init);
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
const unsupported = () => localError(405, "method_not_allowed");
export const PUT = unsupported;
export const DELETE = unsupported;
export const OPTIONS = unsupported;
export const HEAD = unsupported;
