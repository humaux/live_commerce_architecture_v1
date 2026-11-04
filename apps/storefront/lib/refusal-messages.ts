// Purpose: generic English fallback text for every buyer BFF refusal code; the storefront UI maps the code to localized copy.
// Depends on: the closed refusal-code lists of return-request, CVS (taiwan-cvs-logistics-v1), bank-transfer and discount-code contracts.
// Used by: buyer-server.ts failure envelopes and upstream refusal admission (a code outside this table is never forwarded).
// Invariants: one fallback per admitted code; never a diagnostic, upstream message or PII.
import { RETURN_REQUEST_CODES } from "./return-request-contract.ts";
import { CVS_ERROR_CODES } from "./cvs-contract.ts";
import { TRANSFER_ERROR_CODES } from "./bank-transfer-contract.ts";
import { PROMO_ERROR_CODES } from "./promo-contract.ts";

export const messages: Record<string, string> = {
 ...Object.fromEntries(RETURN_REQUEST_CODES.map(code=>[code,"Request refused."])),
  unauthorized: "Session unavailable.",
  forbidden: "Request not permitted.",
  not_found: "Resource not found.",
  method_not_allowed: "Method not allowed.",
  invalid_request: "Request validation failed.",
  invalid_json: "Malformed JSON body.",
  json_required: "JSON content type required.",
  context_changed: "Buyer session changed. Reload before continuing.",
  conflict: "Request conflicts with current state.",
  insufficient_inventory: "Insufficient available inventory.",
  rate_limited: "Too many requests.",
  unavailable: "Temporarily unavailable.",
  erased: "This data was erased.",
  erasure_blocked: "Erasure is blocked while a payment may still be open.",
  export_too_large: "The export is too large for one file.",
  idempotency_conflict: "Request conflicts with an earlier one.",
  // taiwan-cvs-logistics-v1 §5.2/§16 refusals; the UI maps the code to text, these are generic fallbacks.
  ...Object.fromEntries(
    CVS_ERROR_CODES.map((code) => [code, "Request refused."]),
  ),
  // storefront-v2 §C bank-transfer refusals (422 from the buyer definers); the UI maps the code to text.
  ...Object.fromEntries(
    TRANSFER_ERROR_CODES.map((code) => [code, "Request refused."]),
  ),
  // storefront-v2 §F discount-code refusals (422 from the quote request / BeginCheckout); the UI maps the code to text.
  ...Object.fromEntries(
    PROMO_ERROR_CODES.map((code) => [code, "Request refused."]),
  ),
};
