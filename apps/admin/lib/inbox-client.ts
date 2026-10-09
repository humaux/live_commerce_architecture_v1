// Purpose: private in-memory inbox transport with cookie session fences and fixed safe error codes.
// Depends on: settings-client CSRF/sessionBoundary and inbox-bff frozen grammar; browser fetch only.
// Used by: Inbox, InboxThread and BuyerPanel; no logging, persistence or automatic send retry.
import { csrfCookie, sessionBoundary } from "./settings-client";
import {
  inboxErrorCode,
  inboxRoute,
  validInboxBody,
  validInboxRequest,
} from "./inbox-bff";

/** Safe transport error; message contains only a closed code, never an upstream payload. */
export class InboxError extends Error {
  readonly code: string;
  readonly status: number;
  readonly retryAfter: number;
  constructor(code: string, status: number, retryAfter = 0) {
    super(code);
    this.name = "InboxError";
    this.code = code;
    this.status = status;
    this.retryAfter = retryAfter;
  }
}
function resourcePath(store: string, resource: string, method: string) {
  if (
    !/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(
      store,
    ) ||
    !inboxRoute(method, resource.split("?", 1)[0])
  )
    throw new InboxError("invalid_request", 422);
  return `/api/stores/${store}/${resource}`;
}
async function transport<T>(
  store: string,
  resource: string,
  signal: AbortSignal | undefined,
  body?: Record<string, unknown>,
  key?: string,
): Promise<T> {
  const path = resourcePath(store, resource, body ? "POST" : "GET");
  const encoded = body ? JSON.stringify(body) : undefined;
  // Validate before constructing fetch: private text and unknown query keys must never enter a URL.
  const request = new Request(`https://inbox.invalid${path}`, {
    method: body ? "POST" : "GET",
    ...(body ? { headers: { "Idempotency-Key": key ?? "" } } : {}),
  });
  if (
    !validInboxRequest(request, resource.split("?", 1)[0]) ||
    (body && !validInboxBody(resource, encoded!))
  )
    throw new InboxError("invalid_request", 422);
  const csrf = csrfCookie();
  let boundary: string;
  try {
    boundary = await sessionBoundary(csrf);
  } catch {
    throw new InboxError("unauthorized", 401);
  }
  signal?.throwIfAborted();
  let response: Response;
  try {
    response = await fetch(path, {
      method: body ? "POST" : "GET",
      credentials: "same-origin",
      cache: "no-store",
      redirect: "error",
      referrerPolicy: "no-referrer",
      signal: signal
        ? AbortSignal.any([signal, AbortSignal.timeout(8000)])
        : AbortSignal.timeout(8000),
      ...(body
        ? {
            headers: {
              "Content-Type": "application/json",
              "X-CSRF-Token": csrf,
              "Idempotency-Key": key ?? "",
            },
            body: encoded,
          }
        : {}),
    });
  } catch {
    signal?.throwIfAborted();
    throw new InboxError("retry_later", 503);
  }
  let value: unknown;
  try {
    if (
      !response.headers.get("cache-control")?.includes("no-store") ||
      response.headers.get("content-type")?.split(";", 1)[0] !==
        "application/json"
    )
      throw new Error();
    value = await response.json();
  } catch {
    throw new InboxError("retry_later", 503);
  }
  // Recheck after payload parsing too: a logout during that await invalidates the private buffer.
  try {
    if ((await sessionBoundary(csrf)) !== boundary) throw new Error();
  } catch {
    throw new InboxError("unauthorized", 401);
  }
  signal?.throwIfAborted();
  if (!response.ok) {
    const code = inboxErrorCode(response.status, value, resource.split("?", 1)[0]);
    const after = response.headers.get("retry-after") ?? "";
    throw new InboxError(code ?? "retry_later", code ? response.status : 503, /^[0-9]{1,4}$/.test(after) ? Math.min(3600,Number(after))*1000 : 0);
  }
  return value as T;
}
/** Read transient authorised facts; the caller aborts on store switch/page hiding. */
export function inboxRead<T>(
  storeId: string,
  resource: string,
  signal?: AbortSignal,
): Promise<T> {
  return transport<T>(storeId, resource, signal);
}
/** Submit once with an explicit receipt; retries must reuse that receipt and body. */
export function inboxWrite<T>(
  storeId: string,
  resource: string,
  body: Record<string, unknown>,
  key: string,
  signal?: AbortSignal,
): Promise<T> {
  return transport<T>(storeId, resource, signal, body, key);
}
