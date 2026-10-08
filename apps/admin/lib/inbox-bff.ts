// Purpose: closed A8-A14 and read-only published-template BFF request grammar and safe refusal codes.
// Depends on: live-console-v1 §§3/11/12; native Request/URL/JSON; Go retains scope and send authority.
// Used by: store catchall BFF and inbox browser transport; I01/I02/I11/I14/I15.

const id = "[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}";
const uuid = new RegExp(`^${id}$`);
const conversation = `inbox/conversations/${id}`;
const readPath = new RegExp(
  `^(?:inbox/conversations|${conversation}/messages|inbox/buyer-panel|message-templates)$`,
);
const writePath = new RegExp(
  `^${conversation}/(?:read|takeover|release|messages|customer-link)$`,
);
/** Identify only frozen resource paths, independently of the attempted method. */
export function inboxResource(path: string) {
  return readPath.test(path) || writePath.test(path);
}
/** Identify allowed methods; template publish is deliberately outside this unit. */
export function inboxRoute(method: string, path: string) {
  return method === "GET"
    ? readPath.test(path)
    : method === "POST" && writePath.test(path);
}

const filters = new Set([
  "all",
  "unreplied",
  "messenger",
  "instagram",
  "live_comment",
]);
const positive = (value: string, max = Number.MAX_SAFE_INTEGER) =>
  /^[1-9][0-9]*$/.test(value) &&
  Number.isSafeInteger(Number(value)) &&
  Number(value) <= max;
/** Reject unknown, duplicated, malformed and private query inputs before any upstream call. */
export function validInboxRequest(request: Request, path: string): boolean {
  if (!inboxRoute(request.method, path)) return false;
  const marker = request.url.indexOf("?");
  const raw = marker < 0 ? "" : request.url.slice(marker + 1);
  if (
    (marker >= 0 && !raw) ||
    raw.length > 4096 ||
    /%(?![0-9a-f]{2})/i.test(raw)
  )
    return false;
  const query = new URL(request.url).searchParams;
  if (request.method !== "GET")
    return (
      marker < 0 &&
      /^[A-Za-z0-9_.:-]{8,128}$/.test(
        request.headers.get("idempotency-key") ?? "",
      ) &&
      !request.headers.has("transfer-encoding")
    );
  if (
    request.body !== null ||
    request.headers.has("idempotency-key") ||
    request.headers.has("transfer-encoding") ||
    (request.headers.has("content-length") &&
      request.headers.get("content-length") !== "0")
  )
    return false;
  if (path === "message-templates") return marker < 0;
  if (path === "inbox/buyer-panel") {
    const entries = [...query];
    return (
      entries.length === 1 &&
      ["conversation_id", "bundle_id"].includes(entries[0][0]) &&
      uuid.test(entries[0][1])
    );
  }
  for (const [key, value] of query) {
    if (!value || query.getAll(key).length !== 1) return false;
    if (key === "limit" && positive(value, 50)) continue;
    if (path.endsWith("/messages") && key === "before_seq" && positive(value))
      continue;
    if (path === "inbox/conversations") {
      if (key === "filter" && filters.has(value)) continue;
      if (key === "session_id" && uuid.test(value)) continue;
      // The opaque cursor can carry only a bounded base64url receipt, never arbitrary chat text.
      if (key === "cursor" && /^[A-Za-z0-9_-]{1,1024}$/.test(value)) continue;
    }
    return false;
  }
  return true;
}

const integer = (value: unknown) =>
  typeof value === "number" && Number.isSafeInteger(value) && value >= 0;
/** Validate exact JSON bodies; missing/null versions and text/template ambiguity are refused. */
export function validInboxBody(path: string, raw: string): boolean {
  let value: unknown;
  try {
    value = JSON.parse(raw);
  } catch {
    return false;
  }
  if (!value || typeof value !== "object" || Array.isArray(value)) return false;
  const body = value as Record<string, unknown>;
  const exact = (keys: string[]) =>
    Object.keys(body).length === keys.length &&
    keys.every((key) => Object.hasOwn(body, key));
  if (path.endsWith("/read"))
    return exact(["read_seq"]) && integer(body.read_seq);
  if (path.endsWith("/takeover") || path.endsWith("/release"))
    return exact(["expected_generation"]) && integer(body.expected_generation);
  if (path.endsWith("/customer-link"))
    return (
      exact(["customer_id", "expected_version"]) &&
      integer(body.expected_version) &&
      (body.customer_id === null ||
        (typeof body.customer_id === "string" && uuid.test(body.customer_id)))
    );
  if (path.endsWith("/messages")) {
    if (!integer(body.expected_generation)) return false;
    if (exact(["text", "expected_generation"]))
      return typeof body.text === "string" && body.text.length > 0;
    return (
      exact(["template_id", "template_version", "expected_generation"]) &&
      typeof body.template_id === "string" &&
      /^[a-z0-9][a-z0-9/_-]{0,63}$/.test(body.template_id) &&
      integer(body.template_version) &&
      Number(body.template_version) > 0
    );
  }
  return false;
}

const errors: Record<number, Set<string>> = {
  400: new Set(["invalid_json", "invalid_filter", "invalid_request"]),
  401: new Set(["unauthorized"]),
  403: new Set(["forbidden"]),
  404: new Set(["not_found"]),
  405: new Set(["method_not_allowed"]),
  409: new Set([
    "window_closed",
    "takeover_changed",
    "capability",
    "conversation_gone",
    "duplicate_recent",
    "version_conflict",
    "conflict",
  ]),
  415: new Set(["json_required"]),
  422: new Set(["invalid_request", "invalid_text"]),
  429: new Set(["rate_limited"]),
  503: new Set(["retry_later", "unavailable"]),
};
/** Keep frozen codes, flatten all provider/debug strings to a fixed safe error. */
export function inboxErrorCode(status: number, value: unknown): string | null {
  const code =
    value && typeof value === "object" && "code" in value ? value.code : null;
  return typeof code === "string" && errors[status]?.has(code) ? code : null;
}
