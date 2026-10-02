import {
  authConfig,
  disabledResponse,
  exactJSON,
  localError,
  privateIdentity,
  requireCSRF,
  requireOrigin,
  safeJSON,
  safeError,
  sessionToken,
} from "@/lib/auth";

const names = ["tenant_name", "store_name", "warehouse_name"] as const;
// R5 store-domains (Decisions 1-2): the Go response also carries the assigned handle and the ACTIVE platform subdomain.
const handleShape = /^[a-z0-9]([a-z0-9-]{1,28}[a-z0-9])$/;
const originShape = /^https:\/\/(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z](?:[a-z0-9-]{0,61}[a-z0-9])?$/;

export async function POST(request: Request) {
  if (!authConfig) return disabledResponse();
  if (
    new URL(request.url).search ||
    !requireOrigin(request) ||
    !requireCSRF(request)
  )
    return localError(403, "forbidden");
  const token = sessionToken(request);
  if (!token) return localError(401, "unauthorized");
  const key = request.headers.get("idempotency-key") ?? "";
  if (!/^[A-Za-z0-9_.:-]{8,128}$/.test(key))
    return localError(422, "invalid_request");
  let body: Record<string, unknown>;
  try {
    body = await exactJSON(request, [...names, "currency"]);
  } catch {
    return localError(422, "invalid_request");
  }
  if (
    names.some(
      (name) =>
        typeof body[name] !== "string" ||
        [...(body[name] as string).trim()].length < 1 ||
        [...(body[name] as string).trim()].length > 120,
    ) ||
    typeof body.currency !== "string" ||
    !/^[A-Z]{3}$/.test(body.currency)
  )
    return localError(422, "invalid_request");
  const upstream = await privateIdentity(
    "initial-store",
    {
      tenant_name: (body.tenant_name as string).trim(),
      store_name: (body.store_name as string).trim(),
      warehouse_name: (body.warehouse_name as string).trim(),
      currency: body.currency,
    },
    token,
    key,
  );
  if (!upstream.ok) return safeError(upstream);
  const result = await safeJSON<Record<string, unknown>>(upstream);
  const resultKeys = ["tenant_id", "store_id", "warehouse_id", "handle", "storefront_origin"];
  if (
    !result ||
    Object.keys(result).sort().join(",") !==
      resultKeys.slice().sort().join(",") ||
    ["tenant_id", "store_id", "warehouse_id"].some((key) => !/^[0-9a-f-]{36}$/.test(String(result[key]))) ||
    typeof result.handle !== "string" || !handleShape.test(result.handle) ||
    typeof result.storefront_origin !== "string" ||
    (result.storefront_origin !== "" && !originShape.test(result.storefront_origin))
  )
    return localError(503, "retry_later");
  return Response.json(result, {
    status: 200,
    headers: { "Cache-Control": "no-store" },
  });
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
