// Purpose: closed guest store-contact admission (body, Idempotency-Key, Retry-After) and receipt validation for the buyer BFF.
// Depends on: nothing; origin, transport and the keyed single upstream call stay in buyer-server.ts.
// Used by: buyer-server.ts contactMessage() and the existing GitHub CI buyer BFF suites (contact-messages.test.mjs).
// Invariants: limits mirror Go admission (contracts/storefront-v2.md contact form); a receipt is exactly {message_id, status:"received"}.
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

const text = (value: unknown, max: number, required = false, multiline = false) =>
  typeof value === "string" && [...value].length <= max && (!required || value.trim().length > 0) &&
  !(multiline ? /[\u0000-\u0008\u000b\u000c\u000e-\u001f\u007f-\u009f]/u : /\p{Cc}/u).test(value);

export function validContactBody(v: Record<string, unknown>): boolean {
  return text(v.name, 120, true) && text(v.message, 5000, true, true) &&
    text(v.email, 254, true) && new TextEncoder().encode(v.email as string).length <= 254 &&
    /^[^\s<>"\\@]+@[^\s<>"\\@]+$/.test(v.email as string) &&
    (v.order_number === undefined || text(v.order_number, 64)) &&
    (v.website === undefined || v.website === "");
}

export const validContactKey = (key: string | null): key is string => key !== null && /^[A-Za-z0-9_.:-]{8,128}$/.test(key);

// Go's contact throttle windows are 600 s, so only a 429 with 1..600 seconds is forwarded.
export const contactRetryAfter = (status: number, value: string | null): string | null =>
  status === 429 && value !== null && /^\d{1,3}$/.test(value) && +value >= 1 && +value <= 600 ? value : null;

export function contactReceipt(value: unknown): { message_id: string; status: "received" } | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) return null;
  const r = value as Record<string, unknown>;
  return Object.keys(r).length === 2 && typeof r.message_id === "string" && UUID.test(r.message_id) && r.status === "received"
    ? { message_id: r.message_id, status: "received" }
    : null;
}
