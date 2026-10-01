// POST /api/team/{list,invite,revoke-invite,set-role,remove,accept} -> Go POST /v1/identity/staff/{same}
// (internal/identityhttp/staff.go -> internal/identity/staff.go, migration 0089, contracts/storefront-v2.md §D).
// Browser callers: components/Team.tsx and components/TeamInvite.tsx via lib/team-client.ts. Reached through the BFF key on
// the private identity route with the merchant bearer from the HttpOnly session cookie; the Go definers verify the bearer and the
// OWNER role, so this file decides nothing. It never forwards a tenant, never logs or echoes the invitation token, never
// retries (a repeated invite would send a second mail), and rebuilds every error locally from an allow-listed code.
// 404 unless password login is on (invitations are sent by the same SMTP adapter).
import {
  authConfig,
  disabledResponse,
  exactJSON,
  hasNoQuery,
  localError,
  passwordLoginOn,
  privateIdentity,
  requireCSRF,
  requireOrigin,
  safeError,
  safeJSON,
  sessionToken,
} from "@/lib/auth";
import {
  parseInviteResult,
  parseJoined,
  parseTeam,
  teamActions,
  teamCodes,
  teamKeys,
  validTeamBody,
  type TeamAction,
} from "@/lib/team-model";

const nostore = { "Cache-Control": "no-store" };

type Context = { params: Promise<{ action: string }> };

export async function POST(request: Request, context: Context) {
  if (!authConfig || !passwordLoginOn()) return disabledResponse();
  const { action } = await context.params;
  if (!(teamActions as readonly string[]).includes(action)) return localError(404, "not_found");
  const name = action as TeamAction;
  // Browser-origin writes only: exact Origin and the double-submit CSRF pair (the list read is a POST too, so it is covered).
  if (!hasNoQuery(request) || !requireOrigin(request) || !requireCSRF(request)) return localError(403, "forbidden");
  const token = sessionToken(request);
  if (!token) return localError(401, "unauthorized");
  let body: Record<string, unknown>;
  try {
    body = await exactJSON(request, teamKeys[name]);
  } catch {
    return localError(422, "invalid_request");
  }
  if (!validTeamBody(name, body)) return localError(422, "invalid_request");
  // 14 s: Go sends one invitation mail synchronously (<= 10 s). A timeout is reported as retry_later and never repeated here.
  const upstream = await privateIdentity(`staff/${name}`, body, token, undefined, { timeoutMs: name === "invite" ? 14000 : 6000 });
  if (!upstream.ok) {
    const safe = await safeError(upstream);
    const code = (await safe.clone().json().catch(() => ({})) as { code?: string }).code ?? "retry_later";
    // Statuses Go answers on these routes; an unknown code or status becomes a BFF-level retry_later.
    if (!teamCodes.has(code) || ![401, 403, 404, 409, 422, 429, 503].includes(upstream.status)) return localError(503, "retry_later");
    return safe;
  }
  if (upstream.status === 204) return new Response(null, { status: 204, headers: nostore });
  const result = await safeJSON<unknown>(upstream);
  try {
    const parsed = name === "list" ? parseTeam(result) : name === "invite" ? parseInviteResult(result) : name === "accept" ? parseJoined(result) : null;
    if (parsed === null) return localError(503, "retry_later"); // 200 on a route that must answer 204
    return Response.json(parsed, { status: name === "invite" ? 201 : 200, headers: nostore });
  } catch {
    return localError(503, "retry_later");
  }
}

const unsupported = () => (authConfig ? localError(405, "method_not_allowed", "POST") : disabledResponse());
export const GET = unsupported;
export const PUT = unsupported;
export const DELETE = unsupported;
export const PATCH = unsupported;
export const OPTIONS = unsupported;
export const HEAD = unsupported;
