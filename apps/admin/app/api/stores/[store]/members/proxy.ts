// Purpose: LP-1 members BFF, forwarding the frozen eight operations without retry.
// Depends on: real auth.ts session/store/CSRF guards and Go's strict member DTO validation.
// Used by: members route leaves; permissions remain enforced by Go/PG, never client grants.
import { authConfig, authenticatedStores, clearAuthCookies, localError, readBody, requireCSRF, requireOrigin, safeError, sessionToken } from "@/lib/auth";
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
type Context = { params: Promise<{ store: string; member?: string }> };
/** Exact leaf adapter; only ids/cursors may enter URLs and raw JSON goes to Go unchanged. */
export async function prepare(request: Request, context: Context, operation: string) {
  const { store, member } = await context.params;
  if (!uuid.test(store) || (member !== undefined && !uuid.test(member))) return localError(404, "not_found");
  const url = new URL(request.url);
  const queryKey = operation === "detail" && request.method === "GET" ? "ledger_cursor" : operation === "entries" ? "order_id" : "";
  if (url.href.includes("#") || url.href.endsWith("?") || [...url.searchParams].some(([k, v]) => k !== queryKey || !v || v.length > 1024 || url.searchParams.getAll(k).length !== 1) ||
      (operation === "entries" && !uuid.test(url.searchParams.get("order_id") ?? ""))) return localError(422, "invalid_request");
  const read = request.method === "GET" || operation === "search";
  const key = request.headers.get("idempotency-key");
  if ((read && key !== null) || (!read && !/^[A-Za-z0-9_.:-]{8,128}$/.test(key ?? "")) || request.headers.has("transfer-encoding")) return localError(422, "invalid_request");
  if (!authConfig) return localError(401, "unauthorized");
  const token = sessionToken(request);
  if (!token) { const denied = localError(401, "unauthorized"); clearAuthCookies(denied.headers); return denied; }
  if (request.method !== "GET" && (!requireOrigin(request) || !requireCSRF(request))) return localError(403, "forbidden");
  const listed = await authenticatedStores(token);
  if (!listed.stores) return safeError(listed.response);
  if (!listed.stores.some((s) => s.id === store)) return localError(404, "not_found");
  let body: string | undefined;
  if (request.method !== "GET") {
    if (request.headers.get("content-type")?.split(";", 1)[0].trim().toLowerCase() !== "application/json") return localError(415, "json_required");
    try {
      body = await readBody(request, "application/json", 16 << 10);
      // Keep the exact bytes: Go studioUniqueJSON rejects duplicate/unknown keys at every depth.
      const value: unknown = JSON.parse(body);
      if (!value || typeof value !== "object" || Array.isArray(value)) return localError(400, "invalid_json");
    } catch { return localError(400, "invalid_json"); }
  } else if (request.body !== null || (request.headers.get("content-length") ?? "0") !== "0") return localError(422, "invalid_request");
  const headers = new Headers({ Accept: "application/json" });
  if (body !== undefined) headers.set("Content-Type", "application/json");
  if (key) headers.set("Idempotency-Key", key);
  return { token, headers, body, store, member };
}
/** Preserve status and safe errors; export headers are rebuilt from ids only. */
export async function finish(response: Response, member?: string, operation = "") {
  if (!response.ok) return safeError(response);
  if (![200, 201].includes(response.status) || response.headers.get("content-type")?.split(";", 1)[0] !== "application/json") { await response.body?.cancel(); return localError(503, "retry_later"); }
  let text: string;
  try { text = await readBody(response, "application/json", 1 << 20); JSON.parse(text); } catch { return localError(503, "retry_later"); }
  return new Response(text, { status: response.status, headers: { "Content-Type": "application/json", "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff", ...(operation === "export" ? { "Content-Disposition": `attachment; filename="member-${member}.json"` } : {}) } });
}
