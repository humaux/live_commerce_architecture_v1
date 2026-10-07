// Purpose: admin card-payments/settlements client — browser -> BFF -> Go payment_card/settlement reads.
// Depends on: customers-client.ts (get/WriteOutcome), settings-client.ts (CSRF + session fence), model parsers.
// Used by: CardPayments.tsx, Settlements.tsx.
// BFF `/api/stores/{store}/payments/card` + `/settlements[/{id}]` -> Go `internal/httpapi/payment_card.go`
// (contract stripe-platform-account-v1). The enable/disable PUT is keyless by design: Go's cvsRoute refuses
// an Idempotency-Key on this route because expected_version is the CAS guard; a lost answer is `uncertain`
// and the caller re-GETs instead of retrying blindly (external UNKNOWN rule).
import { get, type WriteOutcome } from "./customers-client";
import { csrfCookie, safeError, sessionBoundary } from "./settings-client";
import { parseCardResult, parseCardSummary, type CardInput, type CardResult, type CardSummary } from "./card-payments-model.ts";
import {
  parseSettlementDetail,
  parseSettlementList,
  type SettlementStatement,
} from "./card-payments-settlements-model.ts";

const base = (store: string) => `/api/stores/${store}`;
// Go GET payments/card (integration:read): platform/store state, terms, descriptor preview, limits.
export const readCardSummary = (store: string, signal: AbortSignal): Promise<CardSummary> =>
  get(`${base(store)}/payments/card`, parseCardSummary, signal);
// Go GET settlements (billing:manage): up to 52 closed weekly statements, no lines.
export const readSettlements = (store: string, signal: AbortSignal): Promise<SettlementStatement[]> =>
  get(`${base(store)}/settlements`, parseSettlementList, signal);
// Go GET settlements/{id} (billing:manage): one statement with every line.
export const readSettlementDetail = (store: string, id: string, signal: AbortSignal): Promise<SettlementStatement> =>
  get(`${base(store)}/settlements/${id}`, parseSettlementDetail, signal);

// One fenced keyless PUT (CSRF cookie + live session boundary, like settings-client writes).
async function put(store: string, resource: string, body: string, boundary: string): Promise<{ response: Response } | { ok: false; code: string; uncertain: boolean }> {
  const csrf = csrfCookie();
  try {
    // Recheck directly before the network write: a changed session sends nothing.
    if (!csrf || (await sessionBoundary(csrf)) !== boundary || csrfCookie() !== csrf)
      return { ok: false, code: "unauthorized", uncertain: false };
  } catch {
    return { ok: false, code: "unauthorized", uncertain: false };
  }
  try {
    const response = await fetch(`${base(store)}/${resource}`, {
      method: "PUT",
      credentials: "same-origin",
      cache: "no-store",
      headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf },
      body,
      signal: AbortSignal.timeout(12000),
    });
    return { response };
  } catch {
    // The PUT is CAS-guarded, not idempotent-key replayable: a lost answer is uncertain; the page re-GETs.
    return { ok: false, code: "retry_later", uncertain: true };
  }
}

// Go PUT payments/card (billing:manage; body exactly {enabled, terms_version, descriptor_suffix,
// expected_version}; 403 not_allowed/blocked, 409 closed/terms stale/version changed, 422 suffix/currency).
export async function setCardPayments(store: string, input: CardInput, boundary: string): Promise<WriteOutcome<CardResult>> {
  const sent = await put(store, "payments/card", JSON.stringify(input), boundary);
  if (!("response" in sent)) return sent;
  const { response } = sent;
  const value: unknown = await response.json().catch(() => null);
  if (!response.ok) return { ok: false, code: safeError(value).code, uncertain: response.status >= 500 };
  try {
    return { ok: true, value: parseCardResult(value) };
  } catch {
    return { ok: false, code: "retry_later", uncertain: true };
  }
}
