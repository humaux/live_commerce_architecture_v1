// Purpose: read and command the authenticated W6-05B ledger, with session fencing and exact response parsing.
// Depends on: orders-client read/error; settings-client CSRF/sessionBoundary; operations-model/request.
// Used by: OperationsLedger; Go operations.go owns permissions, CAS, capability and idempotency.
// Invariants: I01/I02/I06/I11; uncertain writes retain identical key/body until readback.
import { read } from "./orders-client";
import { csrfCookie, sessionBoundary } from "./settings-client";
import {
  parseOperationDetail,
  parseOperationList,
  parseOperationResult,
  type OperationAction,
  type OperationFilter,
} from "./operations-model";
import { operationRetryAfter } from "./operations-request";
const base = (store: string) => `/api/stores/${store}/operations`;
/** Read a filtered page from the server's private, store-authenticated projection. */
export async function readOperations(
  store: string,
  state: OperationFilter,
  cursor: string,
  signal: AbortSignal,
) {
  return parseOperationList(
    await read(
      `${base(store)}?state=${state}&limit=20${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ""}`,
      signal,
    ),
  );
}
/** Read one operation and its newest audit events. */
export async function readOperation(
  store: string,
  id: string,
  signal: AbortSignal,
) {
  const v = parseOperationDetail(await read(`${base(store)}/${id}`, signal));
  if (v.operation_id !== id) throw new Error("operation_id");
  return v;
}
export type OperationWrite =
  | { ok: true }
  | { ok: false; code: string; uncertain: boolean; retryAfter: number | null };
/** Submit one frozen generation using the same session and command key; never blind-retry UNKNOWN. */
export async function writeOperation(
  store: string,
  id: string,
  action: OperationAction,
  body: string,
  key: string,
  boundary: string,
): Promise<OperationWrite> {
  const csrf = csrfCookie();
  try {
    if (
      !csrf ||
      (await sessionBoundary(csrf)) !== boundary ||
      csrfCookie() !== csrf
    )
      throw new Error("session");
  } catch {
    return {
      ok: false,
      code: "unauthorized",
      uncertain: false,
      retryAfter: null,
    };
  }
  let r: Response;
  try {
    // Calls W6-05B operations/{id}/{query|cancel|retry}; Idempotency-Key belongs to this exact generation/body.
    r = await fetch(`${base(store)}/${id}/${action}`, {
      method: "POST",
      credentials: "same-origin",
      headers: {
        "Content-Type": "application/json",
        "X-CSRF-Token": csrf,
        "Idempotency-Key": key,
      },
      body,
      signal: AbortSignal.timeout(8000),
    });
  } catch {
    return {
      ok: false,
      code: "retry_later",
      uncertain: true,
      retryAfter: null,
    };
  }
  const v: unknown = await r.json().catch(() => null);
  if (!r.ok) {
    const o = v && typeof v === "object" ? (v as Record<string, unknown>) : {};
    return {
      ok: false,
      code:
        typeof o.code === "string" && /^[a-z0-9_]{1,64}$/.test(o.code)
          ? o.code
          : "retry_later",
      uncertain: r.status >= 500,
      retryAfter: operationRetryAfter(r.headers.get("retry-after")),
    };
  }
  try {
    if (parseOperationResult(v).operation_id !== id) throw new Error("id");
    return { ok: true };
  } catch {
    return {
      ok: false,
      code: "retry_later",
      uncertain: true,
      retryAfter: null,
    };
  }
}
