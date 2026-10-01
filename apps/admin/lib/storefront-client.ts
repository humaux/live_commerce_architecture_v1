// Admin storefront-publication client: browser -> BFF `/api/stores/{store}/storefront` (GET) and
// `/api/stores/{store}/storefront/publication` (POST) -> Go internal/httpapi/storefront.go (contract
// published-storefront-resolver-v1 "Writer (R3)"). Reads parse the closed Go shape (storefront-model.ts); the write reuses
// writeSettings (cookie CSRF + session fence + Idempotency-Key) and its response is never trusted for state: the card re-GETs.
// Domain binding has NO route here by design: it is the platform operator's CLI (cmd/store-admin), not the merchant's.
import { OrderReadError, read } from "./orders-client";
import { writeSettings } from "./settings-client";
import { parseStorefront, type StorefrontState } from "./storefront-model";

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
