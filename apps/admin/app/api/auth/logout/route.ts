import {
  authConfig,
  clearAuthCookies,
  disabledResponse,
  exactJSON,
  localError,
  privateIdentity,
  requireCSRF,
  requireOrigin,
  sessionToken,
} from "@/lib/auth";

export async function POST(request: Request) {
  if (!authConfig) return disabledResponse();
  if (
    new URL(request.url).search ||
    !requireOrigin(request) ||
    !requireCSRF(request)
  )
    return localError(403, "forbidden");
  const token = sessionToken(request);
  if (!token) {
    const response = localError(401, "unauthorized");
    clearAuthCookies(response.headers);
    return response;
  }
  try {
    await exactJSON(request, []);
  } catch {
    return localError(422, "invalid_request");
  }
  const upstream = await privateIdentity("logout", {}, token);
  if (upstream.status !== 204 && upstream.status !== 401)
    return localError(503, "retry_later");
  const response =
    upstream.status === 204
      ? new Response(null, {
          status: 204,
          headers: { "Cache-Control": "no-store" },
        })
      : localError(401, "unauthorized");
  clearAuthCookies(response.headers);
  return response;
}

const unsupported = () =>
  authConfig
    ? localError(405, "method_not_allowed", "POST")
    : disabledResponse();
export const GET = unsupported;
export const PUT = unsupported;
export const DELETE = unsupported;
export const PATCH = unsupported;
export const OPTIONS = unsupported;
export const HEAD = unsupported;
