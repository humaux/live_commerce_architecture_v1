// Purpose: cookie-less public replay BFF, exact list/detail reads only.
// Depends on: getReplays public upstream (verified host origin + BFF key); Go closed replay DTOs.
// Used by: future buyer replay UI (pending PARITY-NEWFEAT).
// Invariants: no caller tenant/store/bearer forwarded; no body/query, retries or caching.
import { getReplays } from "../../../../../lib/shop-upstream";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export async function GET(request: Request, context: { params: Promise<{ replay?: string[] }> }): Promise<Response> {
  const { replay = [] } = await context.params;
  const headers = { "Cache-Control": "no-store", "X-Content-Type-Options": "nosniff" };
  if (replay.length > 1 || replay.length === 1 && !/^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/.test(replay[0] ?? ""))
    return Response.json({ code: "not_found" }, { status: 404, headers });
  if (request.url.includes("?") || request.headers.has("authorization") || request.headers.has("idempotency-key") || await request.text() !== "")
    return Response.json({ code: "invalid_request" }, { status: 422, headers });
  const result = await getReplays(replay[0]);
  if (result.state === "ok") return Response.json(result.value, { headers });
  return Response.json({ code: result.state === "missing" ? "not_found" : "unavailable" }, { status: result.state === "missing" ? 404 : 503, headers });
}
