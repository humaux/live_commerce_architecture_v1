// Purpose: exact, keyless Meta health BFF grammar and bounded recheck body.
// Depends on: meta-connection-health-v1 §9 B1/B2; native Headers only.
// Used by: the authenticated store BFF and Node counterexamples.
/** Recognizes only the two existing backend routes with their exact methods. */
export function metaHealthRoute(method: string, path: string): "status" | "recheck" | null {
  return method === "GET" && path === "meta/health" ? "status" : method === "POST" && path === "meta/health/recheck" ? "recheck" : null;
}
/** Rejects queries, idempotency keys and unexpected read bodies before proxying. */
export function validMetaHealthRequest(request: { method: string; url: string; body: unknown; headers: Headers }): boolean {
  if (request.url.includes("?") || request.headers.has("idempotency-key") || request.headers.has("transfer-encoding")) return false;
  if (request.method === "GET") return request.body === null && (!request.headers.has("content-length") || request.headers.get("content-length") === "0");
  return request.method === "POST" && request.body !== null && request.headers.get("content-type")?.split(";")[0] === "application/json" &&
    (!request.headers.has("content-length") || /^(?:[1-9]\d{0,2}|10[01]\d|102[0-4])$/.test(request.headers.get("content-length")!));
}
/** Allows exactly one numeric Page identifier, never scope or capability fields. */
export function validRecheckBody(text: string): boolean {
  try {
    const value: unknown = JSON.parse(text);
    return !!value && typeof value === "object" && !Array.isArray(value) && Object.keys(value).join() === "page_id" &&
      typeof (value as { page_id?: unknown }).page_id === "string" && /^\d{1,40}$/.test((value as { page_id: string }).page_id);
  } catch { return false; }
}
