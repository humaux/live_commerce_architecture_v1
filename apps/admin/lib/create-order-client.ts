// Purpose: transient A15/A16 transport with real session fences and immutable attempt receipts.
// Depends on: settings-client CSRF/sessionBoundary, create-order-model and browser fetch.
// Used by: CreateOrderDrawer; BFF /tools/inbox/order-prefill and /tools/orders/for-buyer -> Go merchanttools.
// Invariants: I01/I02/I05/I08/I09; no storage, logging, automatic retry or client price inputs.
import { csrfCookie, sessionBoundary } from "./settings-client";
import { canonicalUUID } from "./orders-model.ts";
import {
  forBuyerError,
  parseOrderPrefill,
  parseForBuyerResult,
  targetQuery,
  validForBuyerBody,
  type DrawerTarget,
  type OrderPrefill,
  type ForBuyerOutcome,
} from "./create-order-model.ts";

/** Carries only a fixed safe transport code; no private upstream diagnostic text. */
export class CreateOrderError extends Error {
  readonly code: string;
  readonly status: number;
  constructor(code: string, status: number) {
    super(code);
    this.name = "CreateOrderError";
    this.code = code;
    this.status = status;
  }
}
function path(store: string, resource: string): string {
  if (!canonicalUUID.test(store))
    throw new CreateOrderError("invalid_request", 422);
  return `/api/stores/${store}/tools/${resource}`;
}
async function fence(csrf: string, boundary?: string): Promise<string> {
  try {
    const current = await sessionBoundary(csrf);
    if (boundary !== undefined && current !== boundary) throw new Error();
    return current;
  } catch {
    throw new CreateOrderError("unauthorized", 401);
  }
}
const timeout = (signal?: AbortSignal) =>
  signal
    ? AbortSignal.any([signal, AbortSignal.timeout(18000)])
    : AbortSignal.timeout(18000);
function privateJSON(response: Response): boolean {
  return (
    response.headers.get("cache-control")?.includes("no-store") === true &&
    response.headers.get("content-type")?.split(";", 1)[0] ===
      "application/json"
  );
}
/** Read the authorized prefill; abort and discard on drawer/store/session change. */
export async function readOrderPrefill(
  store: string,
  target: DrawerTarget,
  signal?: AbortSignal,
): Promise<OrderPrefill> {
  let resource: string;
  try {
    resource = path(store, `inbox/order-prefill${targetQuery(target)}`);
  } catch {
    throw new CreateOrderError("invalid_request", 422);
  }
  const csrf = csrfCookie(),
    boundary = await fence(csrf);
  signal?.throwIfAborted();
  let response: Response;
  try {
    response = await fetch(resource, {
      method: "GET",
      credentials: "same-origin",
      cache: "no-store",
      redirect: "error",
      referrerPolicy: "no-referrer",
      signal: timeout(signal),
    });
  } catch {
    await fence(csrf, boundary);
    signal?.throwIfAborted();
    throw new CreateOrderError("retry_later", 503);
  }
  let value: unknown;
  let malformed = false;
  try {
    if (!privateJSON(response)) throw new Error();
    value = await response.json();
  } catch {
    malformed = true;
  }
  await fence(csrf, boundary);
  signal?.throwIfAborted();
  if (malformed) throw new CreateOrderError("retry_later", 503);
  if (!response.ok) {
    const coded = forBuyerError(response.status, value);
    throw new CreateOrderError(
      coded?.code ?? "retry_later",
      coded ? response.status : 503,
    );
  }
  if (response.status !== 200) throw new CreateOrderError("retry_later", 503);
  try {
    return parseOrderPrefill(value);
  } catch {
    throw new CreateOrderError("retry_later", 503);
  }
}
/** Submit exactly one immutable attempt. UNKNOWN preserves that receipt; callers explicitly retry it. */
export async function createForBuyerOrder(
  store: string,
  attempt: { key: string; body: string },
  boundary: string,
  signal?: AbortSignal,
): Promise<ForBuyerOutcome> {
  const resource = path(store, "orders/for-buyer");
  let body: unknown;
  try {
    body = JSON.parse(attempt.body);
  } catch {
    throw new CreateOrderError("invalid_request", 422);
  }
  if (!/^[A-Za-z0-9_.:-]{8,128}$/.test(attempt.key) || !validForBuyerBody(body))
    throw new CreateOrderError("invalid_request", 422);
  const csrf = csrfCookie();
  await fence(csrf, boundary);
  signal?.throwIfAborted();
  const unknown: ForBuyerOutcome = {
    ok: false,
    code: "retry_later",
    status: 503,
    uncertain: true,
  };
  let response: Response;
  // Calls A16 through the scoped tools BFF (live-console-v1 §5); same key/body on every explicit replay.
  try {
    response = await fetch(resource, {
      method: "POST",
      credentials: "same-origin",
      cache: "no-store",
      redirect: "error",
      referrerPolicy: "no-referrer",
      signal: timeout(signal),
      headers: {
        "Content-Type": "application/json",
        "X-CSRF-Token": csrf,
        "Idempotency-Key": attempt.key,
      },
      body: attempt.body,
    });
  } catch {
    await fence(csrf, boundary);
    return unknown;
  }
  let value: unknown;
  let malformed = false;
  try {
    if (!privateJSON(response)) throw new Error();
    value = await response.json();
  } catch {
    malformed = true;
  }
  // A cookie change while parsing invalidates every private value, including a successful order response.
  await fence(csrf, boundary);
  if (malformed) return unknown;
  if (!response.ok) {
    const coded = forBuyerError(response.status, value);
    if (!coded) return unknown;
    return {
      ok: false,
      ...coded,
      status: response.status,
      uncertain:
        response.status >= 500 || coded.code === "idempotency_conflict",
    };
  }
  if (response.status !== 200 && response.status !== 201) return unknown;
  try {
    return {
      ok: true,
      value: parseForBuyerResult(value),
      status: response.status,
    };
  } catch {
    return unknown;
  }
}
