// Admin discount-code client: browser -> BFF `/api/stores/{store}/promotions[/{id}]` -> Go internal/httpapi/promotions.go (storefront-v2 §F).
// Reads parse the frozen DTO (promotions-model.ts); writes reuse writeSettings (cookie CSRF + session fence + Idempotency-Key) and answer
// with the stored code. Callers re-read the list after every answer: the server owns the version, the usage count and every rule.
import { writeSettings } from "./settings-client";
import { OrderReadError, read } from "./orders-client";
import { ReadError } from "./customers-client";
import { parsePromotion, parsePromotions, type CreateBody, type Promotion, type UpdateBody } from "./promotions-model";

const path = (store: string, id?: string) => `/api/stores/${store}/promotions${id ? `/${id}` : ""}`;

// Go GET promotions (pricing:read). Throws ReadError for useGuardedRead; 403 = this member may not manage codes.
export async function readPromotions(store: string, signal: AbortSignal): Promise<Promotion[]> {
  try {
    return parsePromotions(await read(path(store), signal));
  } catch (error) {
    if (error instanceof OrderReadError) throw new ReadError(error.code);
    throw new ReadError("unavailable"); // a body that fails the frozen shape reads as unavailable
  }
}

export type PromoWrite = { ok: true; value: Promotion } | { ok: false; code: string; uncertain: boolean };
async function write(store: string, resource: string, key: string, body: string, boundary: string): Promise<PromoWrite> {
  try {
    const result = await writeSettings<unknown>(store, { key, method: "POST", resource }, body, boundary);
    if (result.error === null && !result.uncertain) {
      try {
        return { ok: true, value: parsePromotion(result.value) };
      } catch {
        return { ok: false, code: "retry_later", uncertain: true };
      }
    }
    return { ok: false, code: result.error?.code ?? "retry_later", uncertain: result.uncertain };
  } catch {
    return { ok: false, code: "unauthorized", uncertain: false }; // session_changed: nothing was sent
  }
}
// Go POST promotions (pricing:write): 409 promo_exists on a duplicate code, 422 invalid_promotion on a rule violation.
export const createPromotion = (store: string, key: string, body: CreateBody, boundary: string) =>
  write(store, "promotions", key, JSON.stringify(body), boundary);
// Go POST promotions/{id} (pricing:write): version CAS; pausing is status "paused". 409 version_changed on a stale version.
export const updatePromotion = (store: string, id: string, key: string, body: UpdateBody, boundary: string) =>
  write(store, `promotions/${id}`, key, JSON.stringify(body), boundary);
