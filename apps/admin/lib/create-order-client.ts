// Purpose: transient A15/A16 and manual recovery transport with real session fences and immutable attempt receipts.
// Depends on: settings-client CSRF/sessionBoundary, create-order-model, manual body validation and browser fetch.
// Used by: CreateOrderDrawer and ManualOrder; scoped tools BFF -> Go merchanttools.
// Invariants: I01/I02/I05/I08/I09; no storage, logging, automatic retry or client price inputs.
import { csrfCookie, sessionBoundary } from "./settings-client";
import { canonicalUUID } from "./orders-model.ts";
import { validClaimRestriction } from "./claims-request.ts";
import { validManualBody } from "./merchant-tools-model.ts";
import {
  forBuyerError,
  parseOrderPrefill,
  parseForBuyerResult,
  parseForBuyerRecovery,
  targetQuery,
  validForBuyerBody,
  type DrawerTarget,
  type OrderPrefill,
  type ForBuyerOutcome,
  type ForBuyerRecovery,
} from "./create-order-model.ts";

/** Carries only a fixed safe transport code; no private upstream diagnostic text. */
export class CreateOrderError extends Error {
  readonly code: string;
  readonly status: number;
  readonly neverSent: boolean;
  constructor(code: string, status: number, neverSent = false) {
    super(code);
    this.name = "CreateOrderError";
    this.code = code;
    this.status = status;
    this.neverSent = neverSent;
  }
}
function path(store: string, resource: string): string {
  if (!canonicalUUID.test(store))
    throw new CreateOrderError("invalid_request", 422, true);
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
/** Read the warning for server-known bundle references; never changes restrictions or order eligibility. */
export async function readOrderRestricted(store: string, session: string, bundles: readonly string[], signal?: AbortSignal): Promise<boolean> {
  if (!canonicalUUID.test(store) || !canonicalUUID.test(session) || !bundles.length || bundles.length > 50 || bundles.some(id => !canonicalUUID.test(id)))
    throw new CreateOrderError("invalid_request", 422);
  const csrf = csrfCookie(), boundary = await fence(csrf), budget = timeout(signal);
  let restricted = false, failed = false;
  for (const bundle of new Set(bundles)) {
    budget.throwIfAborted();
    try {
      const response = await fetch(`/api/stores/${store}/live-sessions/${session}/claims/blocklist/check?bundle_id=${bundle}`, {
        method: "GET", credentials: "same-origin", cache: "no-store", redirect: "error", referrerPolicy: "no-referrer", signal: budget,
      });
      if (response.status === 401 || response.status === 403) throw new CreateOrderError("unauthorized", response.status);
      if (response.status !== 200 || !privateJSON(response)) throw new CreateOrderError("unavailable", 503);
      const value: unknown = await response.json();
      await fence(csrf, boundary);
      budget.throwIfAborted();
      if (!validClaimRestriction(value)) throw new CreateOrderError("unavailable", 503);
      restricted ||= value.restricted;
    } catch (cause) {
      await fence(csrf, boundary);
      budget.throwIfAborted();
      if (cause instanceof CreateOrderError && [401, 403].includes(cause.status)) throw cause;
      failed = true;
    }
  }
  await fence(csrf, boundary);
  budget.throwIfAborted();
  if (restricted) return true;
  if (failed) throw new CreateOrderError("unavailable", 503);
  return false;
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
  return sendForBuyer(store, attempt, boundary, signal, "orders/for-buyer", parseForBuyerResult, [200, 201]);
}
/** Resolve/seal the original key explicitly; absence is authoritative only after the server's durable seal. */
export async function recoverForBuyerOrder(
  store: string, attempt: { key: string; body: string }, boundary: string, signal?: AbortSignal,
): Promise<{ ok: true; value: ForBuyerRecovery; status: number } | Extract<ForBuyerOutcome, { ok: false }>> {
  return sendForBuyer(store, attempt, boundary, signal, "orders/for-buyer/recover", parseForBuyerRecovery, [200]);
}
/** Resolve/seal an ordinary manual attempt through the same strict recovery transport, without guessing buyer identity. */
export async function recoverManualOrder(
  store: string, attempt: { key: string; body: string }, boundary: string, signal?: AbortSignal,
): Promise<{ ok: true; value: ForBuyerRecovery; status: number } | Extract<ForBuyerOutcome, { ok: false }>> {
  return sendForBuyer(store, attempt, boundary, signal, "orders/manual/recover", parseForBuyerRecovery, [200], validManualBody);
}
async function sendForBuyer<T>(
  store: string, attempt: { key: string; body: string }, boundary: string, signal: AbortSignal | undefined,
  endpoint: string, parse: (value: unknown) => T, accepted: readonly number[],
  validBody: (body: unknown) => boolean = validForBuyerBody,
): Promise<{ ok: true; value: T; status: number } | Extract<ForBuyerOutcome, { ok: false }>> {
  const resource = path(store, endpoint);
  let body: unknown;
  try {
    body = JSON.parse(attempt.body);
  } catch {
    throw new CreateOrderError("invalid_request", 422, true);
  }
  if (!/^[A-Za-z0-9_.:-]{8,128}$/.test(attempt.key) || !validBody(body))
    throw new CreateOrderError("invalid_request", 422, true);
  const csrf = csrfCookie();
  await fence(csrf, boundary);
  signal?.throwIfAborted();
  const unknown: Extract<ForBuyerOutcome, { ok: false }> = {
    ok: false,
    code: "retry_later",
    status: 503,
    uncertain: true,
  };
  let response: Response;
  // Calls A16/A16R through the scoped tools BFF (live-console-v1 §5/§5.1.1); same key/body on every explicit replay.
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
  // Permission loss is authoritative even when an intermediary replaces the error body with HTML.
  if (response.status === 401 || response.status === 403) {
    await fence(csrf, boundary);
    return { ok: false, code: response.status === 401 ? "unauthorized" : "forbidden", status: response.status, uncertain: false };
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
  signal?.throwIfAborted();
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
  if (!accepted.includes(response.status)) return unknown;
  try {
    return {
      ok: true,
      value: parse(value),
      status: response.status,
    };
  } catch {
    return unknown;
  }
}
