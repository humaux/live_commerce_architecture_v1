// R5 store-domains (Decision 1): the onboarding wizard's live handle preview -> Go POST /v1/identity/handle-suggest
// (internal/identityhttp). Read-only (suggests a slug + whether it is free; the DB trigger assigns the real handle from
// the same store name on create), so it requires the origin fence but no CSRF token and no session. A body that fails the
// closed Go shape reads as 503, never as a bogus handle.
import {
  authConfig,
  disabledResponse,
  exactJSON,
  localError,
  privateIdentity,
  requireOrigin,
  safeError,
  safeJSON,
} from "@/lib/auth";

const handleShape = /^[a-z0-9]([a-z0-9-]{1,28}[a-z0-9])$/;

export async function POST(request: Request) {
  if (!authConfig) return disabledResponse();
  if (new URL(request.url).search || !requireOrigin(request))
    return localError(403, "forbidden");
  let body: Record<string, unknown>;
  try {
    body = await exactJSON(request, ["store_name"]);
  } catch {
    return localError(422, "invalid_request");
  }
  const name =
    typeof body.store_name === "string" ? body.store_name.trim() : "";
  if (!name || [...name].length > 120)
    return localError(422, "invalid_request");
  const upstream = await privateIdentity("handle-suggest", { store_name: name });
  if (!upstream.ok) return safeError(upstream);
  const result = await safeJSON<Record<string, unknown>>(upstream);
  if (
    !result ||
    Object.keys(result).sort().join(",") !== "available,suggested" ||
    typeof result.suggested !== "string" ||
    !handleShape.test(result.suggested) ||
    typeof result.available !== "boolean"
  )
    return localError(503, "retry_later");
  return Response.json(result, {
    status: 200,
    headers: { "Cache-Control": "no-store" },
  });
}

const unsupported = () =>
  authConfig ? localError(405, "method_not_allowed", "POST") : disabledResponse();
export const GET = unsupported;
export const PUT = unsupported;
export const DELETE = unsupported;
export const PATCH = unsupported;
export const OPTIONS = unsupported;
export const HEAD = unsupported;
