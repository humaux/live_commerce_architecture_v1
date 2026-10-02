// Admin storefront-publication client: browser -> BFF `/api/stores/{store}/storefront` (GET) and
// `/api/stores/{store}/storefront/publication` (POST) -> Go internal/httpapi/storefront.go (contract
// published-storefront-resolver-v1 "Writer (R3)"), plus the R5 merchant self-service domain routes on the same card:
// GET/POST `storefront/domains` and POST `storefront/domains/{suspend,detach}` (Decision 3). Reads parse the closed Go
// shape (storefront-model.ts); writes reuse writeSettings (cookie CSRF + session fence + Idempotency-Key). The publication
// and move responses are never trusted for state (re-GET), but the domain-request response IS parsed: its DNS instructions
// (TXT token) appear only there. The operator CLI (cmd/store-admin) remains the break-glass for the same state.
import { OrderReadError, read } from "./orders-client";
import { writeSettings } from "./settings-client";
import {
  parseDomainRequest,
  parseStorefront,
  parseStorefrontDomains,
  type DomainRequestResult,
  type StorefrontDomains,
  type StorefrontState,
} from "./storefront-model";

export type { StorefrontDomain, StorefrontState } from "./storefront-model";
export type WriteResult = { ok: true } | { ok: false; code: string; uncertain: boolean };

// Go GET storefront (integration:read). 403 = this member may not see the card.
export async function readStorefront(store: string, signal: AbortSignal): Promise<StorefrontState> {
  const value = await read(`/api/stores/${store}/storefront`, signal);
  try {
    return parseStorefront(value);
  } catch {
    throw new OrderReadError("unavailable"); // a body that fails the closed shape reads as unavailable
  }
}

// Go POST storefront/publication (integration:manage): body exactly {published, expected_version}; a stale version is
// 409 `conflict`. The Idempotency-Key is required by the BFF only; the Go write is a compare-and-set on the version.
export async function setPublication(
  store: string,
  key: string,
  published: boolean,
  expectedVersion: number,
  boundary: string,
): Promise<WriteResult> {
  try {
    const body = JSON.stringify({ published, expected_version: expectedVersion });
    const result = await writeSettings<unknown>(store, { key, method: "POST", resource: "storefront/publication" }, body, boundary);
    if (result.error === null && !result.uncertain) return { ok: true };
    return { ok: false, code: result.error?.code ?? "retry_later", uncertain: result.uncertain };
  } catch {
    return { ok: false, code: "unauthorized", uncertain: false }; // session_changed: nothing was sent
  }
}

export type DomainRequestOutcome =
  | { ok: true; result: DomainRequestResult }
  | { ok: false; code: string; uncertain: boolean };

// Go GET storefront/domains (integration:read): every domain row of the store, any state, with the
// pending verification token (the merchant's own; the card shows it, nothing logs it).
export async function readDomains(store: string, signal: AbortSignal): Promise<StorefrontDomains> {
  const value = await read(`/api/stores/${store}/storefront/domains`, signal);
  try {
    return parseStorefrontDomains(value);
  } catch {
    throw new OrderReadError("unavailable");
  }
}

// Go POST storefront/domains (integration:manage): body {hostname}. Unlike publication, this write
// response IS parsed: the one-time DNS instructions (TXT token) appear only here, so a re-GET would
// never recover them. A body that fails the closed shape reads as uncertain, never as saved.
export async function requestDomain(
  store: string,
  key: string,
  hostname: string,
  boundary: string,
): Promise<DomainRequestOutcome> {
  try {
    const body = JSON.stringify({ hostname });
    const result = await writeSettings<unknown>(
      store, { key, method: "POST", resource: "storefront/domains" }, body, boundary,
    );
    if (result.error !== null || result.uncertain || result.value === null || result.value === undefined)
      return { ok: false, code: result.error?.code ?? "retry_later", uncertain: result.uncertain };
    try {
      const parsed = parseDomainRequest(result.value);
      if (parsed.origin !== `https://${hostname}`) throw new Error("storefront_shape");
      return { ok: true, result: parsed };
    } catch {
      return { ok: false, code: "retry_later", uncertain: true };
    }
  } catch {
    return { ok: false, code: "unauthorized", uncertain: false }; // session_changed: nothing was sent
  }
}

// Go POST storefront/domains/{suspend,detach} (integration:manage): body {origin}. The card re-GETs
// (the write response is never trusted for state), so the outcome is just ok/refused/uncertain.
export async function moveDomain(
  store: string,
  key: string,
  origin: string,
  action: "suspend" | "detach",
  boundary: string,
): Promise<WriteResult> {
  try {
    const body = JSON.stringify({ origin });
    const result = await writeSettings<unknown>(
      store, { key, method: "POST", resource: `storefront/domains/${action}` }, body, boundary,
    );
    if (result.error === null && !result.uncertain) return { ok: true };
    return { ok: false, code: result.error?.code ?? "retry_later", uncertain: result.uncertain };
  } catch {
    return { ok: false, code: "unauthorized", uncertain: false }; // session_changed: nothing was sent
  }
}
