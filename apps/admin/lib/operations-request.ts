// Purpose: close W6-05B BFF paths, query/body grammar and 24-hour Retry-After.
// Depends on: operations-model ID grammar and native URL/Request.
// Used by: admin BFF, operations-client, Node acceptance tests.
import { operationID } from "./operations-model.ts";
export type OperationRoute = "list" | "detail" | "command";
/** Match only the five frozen operation resources. */
export function operationRoute(
  method: string,
  path: string,
): OperationRoute | null {
  const parts = path.split("/");
  if (parts[0] !== "operations") return null;
  if (method === "GET" && parts.length === 1) return "list";
  if (!operationID.test(parts[1] ?? "")) return null;
  if (method === "GET" && parts.length === 2) return "detail";
  if (
    method === "POST" &&
    parts.length === 3 &&
    ["query", "cancel", "retry"].includes(parts[2])
  )
    return "command";
  return null;
}
/** Validate exact query/header declarations before authenticating or forwarding. */
export function validOperationsRequest(
  kind: OperationRoute,
  request: Request,
): boolean {
  const url = new URL(request.url);
  const q = url.searchParams;
  if (request.url.length > 8192 || (request.url.includes("?") && !url.search))
    return false;
  if (kind !== "list" && request.url.includes("?")) return false;
  if (kind === "list") {
    for (const [k, v] of q) {
      if (q.getAll(k).length !== 1 || !v) return false;
      if (k === "state") {
        if (
          ![
            "attention",
            "FAILED",
            "FAILED_FINAL",
            "UNKNOWN",
            "ACKNOWLEDGED",
            "BLOCKED_POLICY",
            "STALE_BINDING",
            "READY",
          ].includes(v)
        )
          return false;
      } else if (k === "limit") {
        if (!/^[1-9][0-9]{0,2}$/.test(v) || Number(v) > 100) return false;
      } else if (k === "cursor") {
        if (!/^[A-Za-z0-9_-]{1,4096}$/.test(v)) return false;
      } else return false;
    }
  }
  if (
    request.headers.has("transfer-encoding") ||
    request.headers.has("if-match")
  )
    return false;
  if (kind !== "command")
    return (
      request.method === "GET" &&
      !request.headers.has("idempotency-key") &&
      request.body === null &&
      (!request.headers.has("content-length") ||
        request.headers.get("content-length") === "0")
    );
  return (
    request.method === "POST" &&
    /^[A-Za-z0-9._:-]{8,128}$/.test(
      request.headers.get("idempotency-key") ?? "",
    ) &&
    request.headers
      .get("content-type")
      ?.split(";", 1)[0]
      .trim()
      .toLowerCase() === "application/json" &&
    Number(request.headers.get("content-length") ?? "0") <= 256
  );
}
/** The command body has one non-negative safe integer; duplicate JSON keys are refused. */
export function validOperationBody(raw: string): boolean {
  if (
    raw.length > 256 ||
    !/^\s*\{\s*"expected_attempts"\s*:\s*(?:0|[1-9][0-9]*)\s*\}\s*$/.test(raw)
  )
    return false;
  try {
    const v = JSON.parse(raw);
    return (
      !!v &&
      typeof v === "object" &&
      !Array.isArray(v) &&
      Object.keys(v).length === 1 &&
      Number.isSafeInteger(v.expected_attempts) &&
      v.expected_attempts >= 0
    );
  } catch {
    return false;
  }
}
/** Preserve bounded numeric backoff through the rolling 24-hour query cap. */
export function operationRetryAfter(raw: string | null): number | null {
  return raw && /^[1-9][0-9]{0,4}$/.test(raw) && Number(raw) <= 86400
    ? Number(raw)
    : null;
}

/** Remaining query backoff for one operation; another operation keeps its own server capability. */
export function operationQueryWaitSeconds(
  id: string,
  wait: { id: string; until: number } | null,
  now = Date.now(),
): number {
  return wait?.id === id
    ? Math.max(0, Math.ceil((wait.until - now) / 1000))
    : 0;
}
