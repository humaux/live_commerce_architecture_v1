// Pure wire contract of discount codes for the buyer storefront (contracts/storefront-v2.md §F): the optional `promo_code` on the quote
// request (BFF POST /api/buyer/quotes -> Go POST /v1/buyer/quotes), the optional `promotion` object of a quote/order response, and the closed
// list of coded refusals (422) the quote request and BeginCheckout can answer.
// Owns: shapes, the code mirror, the closed code list. It never decides a price or whether a code is valid (Go + SQL migration 0091 are the
// authority; the amounts shown are the server quote's own discount_minor, never computed here).

export const PROMO_ERROR_CODES = [
  "promo_invalid",
  "promo_not_started",
  "promo_expired",
  "promo_min_subtotal",
  "promo_used_up",
  "promo_buyer_limit",
  "promo_changed",
] as const;
export type PromoErrorCode = (typeof PROMO_ERROR_CODES)[number];
export const isPromoErrorCode = (value: unknown): value is PromoErrorCode =>
  typeof value === "string" && (PROMO_ERROR_CODES as readonly string[]).includes(value);

// The shape Go accepts (internal/promotions.Normalize): 3..24 letters, digits or hyphens, any case.
export const PROMO_CODE_SHAPE = /^[A-Za-z0-9-]{3,24}$/;
// Typed text -> the canonical upper-case code, or null when it cannot be a code (the UI then shows promo_invalid without a request).
export function normalizePromoCode(text: string): string | null {
  const code = text.trim().toUpperCase();
  return PROMO_CODE_SHAPE.test(code) ? code : null;
}

export type QuotePromotion = { code: string; kind: "percent" | "fixed"; percent: number; fixed_minor: number };
const MAX_AMOUNT = 1_000_000_000_000;
// The optional `promotion` key of a quote or order quote: absent when no code applied, otherwise exactly these four keys.
export function validPromotion(value: unknown): value is QuotePromotion {
  if (!value || typeof value !== "object" || Array.isArray(value)) return false;
  const v = value as Record<string, unknown>;
  const int = (x: unknown, max: number) => typeof x === "number" && Number.isSafeInteger(x) && x >= 0 && x <= max;
  return (
    Object.keys(v).sort().join(",") === "code,fixed_minor,kind,percent" &&
    typeof v.code === "string" && /^[A-Z0-9-]{3,24}$/.test(v.code) &&
    (v.kind === "percent" || v.kind === "fixed") &&
    int(v.percent, 90) && int(v.fixed_minor, MAX_AMOUNT) &&
    (v.kind === "percent" ? (v.percent as number) >= 1 && v.fixed_minor === 0 : (v.fixed_minor as number) >= 1 && v.percent === 0)
  );
}
