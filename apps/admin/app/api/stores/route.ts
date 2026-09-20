import {
  authConfig,
  authenticatedStores,
  clearAuthCookies,
  disabledResponse,
  localError,
  safeError,
  sessionToken,
} from "@/lib/auth";

export async function GET(request: Request) {
  if (!authConfig) return disabledResponse();
  if (new URL(request.url).search || request.headers.get("content-length"))
    return localError(422, "invalid_request");
  const token = sessionToken(request);
  if (!token) return localError(401, "unauthorized");
  const listed = await authenticatedStores(token);
  if (!listed.stores) {
    const response =
      listed.response.status >= 400
        ? await safeError(listed.response)
        : localError(503, "retry_later");
    if (response.status === 401) clearAuthCookies(response.headers);
    return response;
  }
  return Response.json(
    { items: listed.stores },
    { status: 200, headers: { "Cache-Control": "no-store" } },
  );
}

const unsupported = () =>
  authConfig
    ? localError(405, "method_not_allowed", "GET")
    : disabledResponse();
export const POST = unsupported;
export const PUT = unsupported;
export const DELETE = unsupported;
export const PATCH = unsupported;
export const OPTIONS = unsupported;
export const HEAD = unsupported;
