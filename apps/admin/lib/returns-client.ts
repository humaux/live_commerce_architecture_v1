// Purpose: browser client for the merchant returns/cancel BFF routes (contracts/returns-v1.md §6):
//   GET orders/{id}/returns, GET returns[?state=], GET orders/cancel-refund-gaps, POST orders/{id}/cancel,
//   POST orders/{id}/returns, POST returns/{id}/{receive|inspect|close|cancel}.
// Depends on: ./orders-client.ts (read envelope + WriteResult), ./settings-client.ts (writeSettings: cookie CSRF +
//   session fence + Idempotency-Key), ./returns-model.ts (strict parsers); BFF grammar in ./orders-request.ts.
// Used by: apps/admin/components/OrderReturns.tsx, apps/admin/components/OrderDetailPanel.tsx, apps/admin/components/ReturnsList.tsx.
// Invariants: reads parse the frozen DTOs (a drifted answer is "unavailable", never rendered); writes never trust
//   their own response body — the caller re-GETs; one Idempotency-Key per distinct body, reused only for a
//   byte-identical retry after an unknown outcome (§4). Status: MOCK evidence (no provider involved).
import { read, type WriteResult } from "./orders-client";
import { writeSettings } from "./settings-client";
import {
  parseRefundGaps, parseRmaList, type RefundGap, type Rma, type RmaState,
} from "./returns-model";

async function get<T>(path: string, parse: (value: unknown) => T, signal: AbortSignal): Promise<T> {
  return parse(await read(path, signal));
}

/** Go GET orders/{id}/returns (orders:read): the RMAs of one order. */
export const readOrderReturns = (store: string, orderId: string, signal: AbortSignal): Promise<Rma[]> =>
  get(`/api/stores/${store}/orders/${orderId}/returns`, parseRmaList, signal);
/** Go GET returns (orders:read): the store's 100 newest RMAs, optionally one state filter. */
export const readReturns = (store: string, state: RmaState | "", signal: AbortSignal): Promise<Rma[]> =>
  get(`/api/stores/${store}/returns${state ? `?state=${state}` : ""}`, parseRmaList, signal);
/** Go GET orders/cancel-refund-gaps (orders:read): cancelled card orders still owing the buyer (§3). */
export const readCancelRefundGaps = (store: string, signal: AbortSignal): Promise<RefundGap[]> =>
  get(`/api/stores/${store}/orders/cancel-refund-gaps`, parseRefundGaps, signal);

async function write(store: string, resource: string, key: string, body: string, boundary: string): Promise<WriteResult> {
  try {
    const result = await writeSettings<unknown>(store, { key, method: "POST", resource }, body, boundary);
    if (result.error === null && !result.uncertain) return { ok: true };
    return { ok: false, code: result.error?.code ?? "retry_later", uncertain: result.uncertain };
  } catch {
    // session_changed: nothing was sent.
    return { ok: false, code: "unauthorized", uncertain: false };
  }
}

/** Go POST orders/{id}/cancel (fulfillment:write; body exactly {expected_state, reason}). Never starts a refund (§3). */
export const postCancelOrder = (store: string, orderId: string, key: string, body: string, boundary: string) =>
  write(store, `orders/${orderId}/cancel`, key, body, boundary);
/** Go POST orders/{id}/returns (fulfillment:write; body exactly {reason, lines}). */
export const postRegisterReturn = (store: string, orderId: string, key: string, body: string, boundary: string) =>
  write(store, `orders/${orderId}/returns`, key, body, boundary);
/** Go POST returns/{id}/{step}: receive/cancel are fulfillment:write; inspect/close open with inventory:write (§6). */
export const postRmaStep = (
  store: string, rmaId: string, step: "receive" | "inspect" | "close" | "cancel", key: string, body: string, boundary: string,
) => write(store, `returns/${rmaId}/${step}`, key, body, boundary);
