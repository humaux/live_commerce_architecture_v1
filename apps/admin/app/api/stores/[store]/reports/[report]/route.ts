// Purpose: eight exact scoped W6-02B report GET routes; CSV performs one audited export.
// Depends on: auth session/store/Origin/CSRF helpers and trusted apiOrigin, native fetch, report request/response fences.
// Used by: Reports.tsx via reports-client; never a generic proxy or fixture-auth route.
// Invariants: I01 trusted store, I06 no automatic export retry, I11/I15 private output.
import { authConfig, authenticatedStores, sessionToken, clearAuthCookies, localError, requireOrigin, requireCSRF, safeError } from "@/lib/auth";
import { canonicalUUID } from "@/lib/orders-model";
import { reportsRoute, validReportsRequest, type ReportName } from "@/lib/reports-request";
import { reportJSON, reportCSVFilename } from "@/lib/reports-response";

type Context = { params: Promise<{ store: string; report: string }> };
const privateHeaders = { "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff" };

/** Authorize one exact report request and project its response; CSV audits on Go, with no retries. */
export async function GET(request: Request, context: Context): Promise<Response> {
  const { store, report } = await context.params;
  const kind = reportsRoute(request.method, `reports/${report}`);
  if (!authConfig || !canonicalUUID.test(store) || !kind) return localError(404, "not_found");
  if (!validReportsRequest(kind, request)) return localError(422, "invalid_request");
  const token = sessionToken(request);
  if (!token) {
    const denied = localError(401, "unauthorized");
    clearAuthCookies(denied.headers);
    return denied;
  }
  const csv = kind.endsWith("-csv");
  // Same-origin browser GETs can omit Origin. Require their browser-controlled Fetch Metadata instead,
  // plus the existing CSRF pair; never infer origin from Host or forwarded headers.
  const originOK = request.headers.has("origin") ? requireOrigin(request) : request.headers.get("sec-fetch-site") === "same-origin";
  if (csv && (!originOK || !requireCSRF(request))) return localError(403, "forbidden");
  const listed = await authenticatedStores(token);
  if (!listed.stores) {
    const denied = await safeError(listed.response);
    if (denied.status === 401) clearAuthCookies(denied.headers);
    return denied;
  }
  const current = listed.stores.find((s) => s.id === store);
  if (!current) return localError(404, "not_found");
  const name = (csv ? kind.slice(0, -4) : kind) as ReportName;
  const needs = ["orders:read", ...(name === "funnel" ? ["live:read"] : []), ...(csv ? ["orders:export"] : [])];
  if (current.permissions && needs.some((p) => !current.permissions!.includes(p))) return localError(403, "forbidden");
  // Calls reporting-v2 §§1–6 at the trusted API origin only. The generic six-second backend cannot
  // carry Go's sixty-second transaction budget; this exact leaf has its own fixed bounded transport.
  const url = new URL(request.url);
  if (request.signal.aborted) return localError(503, "retry_later");
  let response: Response;
  try {
    response = await fetch(`${authConfig.apiOrigin}/v1/admin/stores/${store}/reports/${report}${url.search}`, {
      method: "GET",
      headers: { Authorization: `Bearer ${token}`, Accept: csv ? "text/csv" : "application/json" },
      cache: "no-store", redirect: "error",
      signal: AbortSignal.any([request.signal, AbortSignal.timeout(65000)]),
    });
  } catch { return localError(503, "retry_later"); }
  if (!response.ok) {
    const denied = await safeError(response);
    if (denied.status === 401) clearAuthCookies(denied.headers);
    return denied;
  }
  try {
    const from = url.searchParams.get("from")!, to = url.searchParams.get("to")!;
    if (csv) {
      if (response.status !== 200) throw new Error("unavailable");
      const filename = reportCSVFilename(response.headers, name, from, to);
      return new Response(response.body, { headers: { ...privateHeaders, "Content-Type": "text/csv; charset=utf-8", "Content-Disposition": `attachment; filename="${filename}"` } });
    }
    const body = await reportJSON(response, name, from, to, url.searchParams.get("session_id") ?? "");
    return Response.json(body, { headers: privateHeaders });
  } catch {
    return localError(503, "retry_later");
  }
}

/** Reject all write verbs on this report-only leaf; no request reaches Go. */
export async function POST() { return localError(405, "method_not_allowed", "GET"); }
/** Reject PUT without forwarding. */
export const PUT = POST;
/** Reject PATCH without forwarding. */
export const PATCH = POST;
/** Reject DELETE without forwarding. */
export const DELETE = POST;
/** HEAD must not trigger a CSV audit via Next's automatic GET fallback. */
export const HEAD = POST;
