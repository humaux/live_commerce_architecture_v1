// Purpose: closed W6-01B BFF transport with injectable authority for DB-free tests.
// Depends on: customer-tags-request.ts grammar and an authenticated server adapter.
// Used by: customer-tags-bff.ts and exact tag/note leaf routes; never a catch-all.
import { customerTagsRoute, validCustomerTagsBody, validCustomerTagsRequest } from "./customer-tags-request.ts";

/** Authenticated adapter; only the server supplies credentials and store authority. */
export type TagProxyAdapter = {
  authorize: (request: Request, store: string) => Promise<Response | string>;
  forward: (resource: string, init: RequestInit, token: string, store: string) => Promise<Response>;
  body: (request: Request) => Promise<string>;
  error: (status: number, code: string) => Response;
};

/** Forward one exact tag/note resource after grammar and authenticated authority checks. */
export async function proxyCustomerTags(request: Request, store: string, resource: string, adapter: TagProxyAdapter): Promise<Response> {
  if (!/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(store))
    return adapter.error(404, "not_found");
  const kind = customerTagsRoute(request.method, resource, new URL(request.url).search);
  if (!kind) return adapter.error(405, "method_not_allowed");
  if (!validCustomerTagsRequest(kind, request)) return adapter.error(422, "invalid_request");
  const authority = await adapter.authorize(request, store);
  if (authority instanceof Response) return authority;
  let body = "";
  if (request.method !== "GET") {
    try { body = await adapter.body(request); } catch { return adapter.error(422, "invalid_request"); }
    if (!validCustomerTagsBody(kind, body)) return adapter.error(422, "invalid_request");
  }
  const headers = new Headers({ Accept: "application/json" });
  if (request.method !== "GET") {
    headers.set("Idempotency-Key", request.headers.get("idempotency-key")!);
    if (body) headers.set("Content-Type", "application/json");
  }
  // Calls Go customer_tags.go, customers-billing-v1 Amendment W6-01B; never retries UNKNOWN.
  let response: Response;
  try {
    response = await adapter.forward(resource + new URL(request.url).search,
      { method: request.method, headers, ...(body ? { body } : {}) }, authority, store);
  } catch { return adapter.error(503, "retry_later"); }
  if (response.headers.get("content-type")?.split(";", 1)[0] !== "application/json")
    return adapter.error(503, "retry_later");
  let text: string;
  try {
    // Bound even malformed/hostile backend responses; no note body appears in diagnostics.
    const reader = response.body?.getReader();
    if (!reader) return adapter.error(503, "retry_later");
    const chunks: Uint8Array[] = []; let size = 0;
    try {
      while (true) {
        const chunk = await reader.read(); if (chunk.done) break;
        size += chunk.value.byteLength;
        if (size > 1 << 20) { await reader.cancel(); return adapter.error(503, "retry_later"); }
        chunks.push(chunk.value);
      }
    } finally { reader.releaseLock(); }
    const bytes = new Uint8Array(size); let at = 0;
    for (const chunk of chunks) { bytes.set(chunk, at); at += chunk.byteLength; }
    text = new TextDecoder("utf-8", { fatal: true }).decode(bytes);
    const value: unknown = JSON.parse(text);
    if (!response.ok) {
      const code = value && typeof value === "object" && "code" in value ? (value as { code: unknown }).code : null;
      return adapter.error(response.status, typeof code === "string" && /^[a-z_]{1,48}$/.test(code) ? code : "retry_later");
    }
  } catch { return adapter.error(503, "retry_later"); }
  return new Response(text, { status: response.status, headers: {
    "Content-Type": "application/json", "Cache-Control": "private, no-store", "Vary": "Cookie",
    "X-Content-Type-Options": "nosniff",
  } });
}
