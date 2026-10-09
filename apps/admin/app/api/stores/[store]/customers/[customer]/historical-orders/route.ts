// Purpose: exact GET historical-order archive BFF -> Go internal/httpapi/customer_historical.go (W5-03B section 7).
// Depends on: server auth/session/store guards, backend.callBackend, strict import-history projection/query model.
// Used by: CustomerHistoricalOrders; no writes, bodyless GET, safe private no-store projection.
import { authConfig, authenticatedStores, clearAuthCookies, localError, readBody, safeError, sessionToken } from "@/lib/auth";
import { callBackend, fixtureSession } from "@/lib/backend";
import { historicalOrdersQuery, parseHistoricalOrdersPage, validHistoricalOrdersRequest } from "@/lib/import-history-model";

/** Authorize one exact archive read and return only the frozen safe fields, never raw backend error/body data. */
export async function GET(request: Request, context: { params: Promise<{ store: string; customer: string }> }): Promise<Response> {
  const { store, customer } = await context.params;
  if (request.method !== "GET") return localError(405, "method_not_allowed", "GET");
  if (!validHistoricalOrdersRequest(request, store, customer)) return localError(422, "invalid_request");
  let token: string;
  if (authConfig) {
    const current = sessionToken(request);
    if (!current) { const denied = localError(401, "unauthorized"); clearAuthCookies(denied.headers); return denied; }
    const listed = await authenticatedStores(current);
    if (!listed.stores) {
      const denied = await safeError(listed.response); if (denied.status === 401) clearAuthCookies(denied.headers); return denied;
    }
    if (!listed.stores.some((s) => s.id === store)) return localError(404, "not_found");
    token = current;
  } else {
    // Existing development-only loopback fixture adapter; never authority from a browser bearer or tenant field.
    const fixture = fixtureSession(); if (!fixture) return localError(401, "unauthorized");
    if (store !== fixture.storeID) return localError(404, "not_found");
    const url = new URL(request.url);
    if (!["127.0.0.1", "localhost"].includes(url.hostname) || !["127.0.0.1:3100", "localhost:3100"].includes(request.headers.get("host") ?? ""))
      return localError(403, "forbidden");
    token = fixture.token;
  }
  // Calls Go customer_historical.go, migration-import-v1 section 7, customers:read; Go remains scope authority.
  let response: Response;
  try { response = await callBackend(`customers/${customer}/historical-orders${new URL(request.url).search}`, { method: "GET" }, token, store); }
  catch { return localError(503, "retry_later"); }
  if (!response.ok) { const denied = await safeError(response); if (denied.status === 401) clearAuthCookies(denied.headers); return denied; }
  try {
    const body: unknown = JSON.parse(await readBody(response, "application/json", 512 << 10));
    const page = parseHistoricalOrdersPage(body, historicalOrdersQuery(request.url)!);
    return Response.json(page, { headers: { "Cache-Control": "private, no-store", "Vary": "Cookie", "X-Content-Type-Options": "nosniff" } });
  } catch { return localError(503, "retry_later"); }
}
