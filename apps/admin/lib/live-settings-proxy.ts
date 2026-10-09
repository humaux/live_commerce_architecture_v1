// Purpose: authenticate and forward only the W3-U2 settings/claims command grammar.
// Depends on: real auth cookie/CSRF/store helpers, frozen live-settings-model and Go private endpoints.
// Used by: dedicated admin BFF leaves; one explicit request, never an automatic write retry.
import {
  authConfig,
  authenticatedStores,
  clearAuthCookies,
  localError,
  readBody,
  requireCSRF,
  requireOrigin,
  sessionToken,
} from "./auth";
import {
  settingsBody,
  settingsCodes,
  settingsData,
  settingsPermissions,
  settingsQuery,
  settingsResource,
  settingsUUID,
} from "./live-settings-model";
const headers = {
  "Cache-Control": "private, no-store",
  "X-Content-Type-Options": "nosniff",
};
function denied(status: number, code: string) {
  const response = localError(status, code);
  for (const [k, v] of Object.entries(headers)) response.headers.set(k, v);
  if (status === 401) clearAuthCookies(response.headers);
  return response;
}
/** Bind each request to the signed server session; no host/body authority and no diagnostic response bodies. */
export async function liveSettingsProxy(
  request: Request,
  store: string,
  path: string,
  transport: typeof fetch,
): Promise<Response> {
  const route = settingsResource(path);
  if (!authConfig || !settingsUUID.test(store) || !route)
    return denied(404, "not_found");
  if (!route.methods.includes(request.method)) {
    const r = denied(405, "method_not_allowed");
    r.headers.set("Allow", route.methods.join(", "));
    return r;
  }
  if (!settingsQuery(route.kind, request.method, new URL(request.url)))
    return denied(422, "invalid_request");
  const token = sessionToken(request);
  if (!token) return denied(401, "unauthorized");
  const read = request.method === "GET";
  const sameOrigin = request.headers.has("origin")
    ? requireOrigin(request)
    : read && request.headers.get("sec-fetch-site") === "same-origin";
  if (!sameOrigin || (!read && !requireCSRF(request)))
    return denied(403, "forbidden");
  const key = request.headers.get("idempotency-key");
  if (read ? key !== null : !key || !/^[A-Za-z0-9_.:-]{8,128}$/.test(key))
    return denied(422, "invalid_request");
  try {
    const listed = await authenticatedStores(token);
    if (!listed.stores)
      return denied(
        listed.response.status === 401
          ? 401
          : listed.response.status === 403
            ? 403
            : 503,
        listed.response.status === 401
          ? "unauthorized"
          : listed.response.status === 403
            ? "forbidden"
            : "retry_later",
      );
    const current = listed.stores.find((s) => s.id === store);
    if (!current) return denied(404, "not_found");
    if (
      current.role !== "owner" &&
      current.permissions &&
      !settingsPermissions(route.kind, request.method).every((p) =>
        current.permissions!.includes(p),
      )
    )
      return denied(403, "forbidden");
    let body: string | undefined;
    const bodyless =
      read || route.kind === "remove" || route.kind === "reminders";
    if (bodyless) {
      const reader = request.body?.getReader();
      if (reader) {
        try {
          for (;;) {
            const chunk = await reader.read();
            if (chunk.done) break;
            if (chunk.value.length) {
              await reader.cancel();
              return denied(422, "invalid_request");
            }
          }
        } finally {
          reader.releaseLock();
        }
      }
    } else {
      try {
        body = await readBody(request, "application/json", 8192);
      } catch {
        return denied(422, "invalid_request");
      }
      if (!settingsBody(route.kind, body))
        return denied(422, "invalid_request");
    }
    // Calls W3-U2/W3-03B/W3-05B with only server session authority; template bodies/notes never enter a URL or log.
    const response = await transport(
      `${authConfig.apiOrigin}/v1/admin/stores/${store}/${path}${new URL(request.url).search}`,
      {
        method: request.method,
        headers: {
          Authorization: `Bearer ${token}`,
          Accept: "application/json",
          ...(key ? { "Idempotency-Key": key } : {}),
          ...(body === undefined ? {} : { "Content-Type": "application/json" }),
        },
        ...(body === undefined ? {} : { body }),
        cache: "no-store",
        redirect: "error",
        signal: AbortSignal.any([
          request.signal,
          AbortSignal.timeout(
            route.kind === "reminders" && !read ? 65000 : 8000,
          ),
        ]),
      },
    );
    if (response.status === 401) return denied(401, "unauthorized");
    if (response.status === 403) return denied(403, "forbidden");
    let value: unknown;
    try {
      value = JSON.parse(
        await readBody(response, "application/json", 1024 * 1024),
      );
    } catch {
      return denied(503, "retry_later");
    }
    if (!response.ok) {
      const code =
        value && typeof value === "object" && "code" in value
          ? value.code
          : null;
      return denied(
        response.status,
        typeof code === "string" && settingsCodes.has(code)
          ? code
          : "retry_later",
      );
    }
    if (
      !response.headers
        .get("cache-control")
        ?.split(",")
        .map((s) => s.trim())
        .includes("no-store")
    )
      return denied(503, "retry_later");
    try {
      const projected = settingsData(
        route.kind,
        request.method,
        value,
      ) as Record<string, unknown>;
      if (
        body !== undefined &&
        (route.kind === "settings" || route.kind === "templates")
      ) {
        const sent = JSON.parse(body) as Record<string, unknown>;
        const fields =
          route.kind === "settings"
            ? ["enabled", "template_id", "template_version"]
            : ["template_id", "public_safe"];
        // A 200 with a different receipt is still UNKNOWN: never advance to a new command from it.
        if (
          fields.some((key) => projected[key] !== sent[key]) ||
          (route.kind === "settings" &&
            projected.version !== Number(sent.expected_version) + 1) ||
          (route.kind === "templates" &&
            JSON.stringify(projected.kinds) !== JSON.stringify(sent.kinds))
        )
          return denied(503, "retry_later");
      }
      return Response.json(projected, { status: 200, headers });
    } catch {
      return denied(503, "retry_later");
    }
  } catch {
    return denied(503, "retry_later");
  }
}
