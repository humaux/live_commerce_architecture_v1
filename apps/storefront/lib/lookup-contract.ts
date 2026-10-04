// Guest order lookup contract (contracts/storefront-v2.md §E5): the request body the BFF accepts for POST /api/buyer/orders/lookup (-> Go
// internal/buyerhttp/lookup.go POST /v1/buyer/orders/lookup) and the one answer shape it relays. Shared by lib/buyer-server.ts and the form.
// It never decides a match (the database does); it only keeps malformed input off the private transport. Go re-validates everything.

const HEX12 = /^[0-9a-fA-F]{4}[ -]?[0-9a-fA-F]{4}[ -]?[0-9a-fA-F]{4}$/;
const UUID = /^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/;
const PHONE_CHARS = /^[+0-9() -]+$/;
export const ORDER_ID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

/** The order number as printed in the mails (XXXX-XXXX-XXXX, any case, dashes or spaces optional) or a full order id. */
export function validOrderRef(value: unknown): value is string {
  return typeof value === "string" && value.length <= 64 && (HEX12.test(value.trim()) || UUID.test(value.trim()));
}

/** The buyer's email (one @, no whitespace, <= 254) or the delivery phone (digits with + ( ) - and spaces, 6..20 digits). */
export function validContact(value: unknown): value is string {
  if (typeof value !== "string") return false;
  const c = value.trim();
  if (c.length === 0 || c.length > 254) return false;
  if (c.includes("@")) return /^[^\s@]+@[^\s@]+$/.test(c);
  const digits = c.replace(/[^0-9]/g, "").length;
  return PHONE_CHARS.test(c) && digits >= 6 && digits <= 20;
}

export function validLookupBody(value: unknown): value is { order_ref: string; contact: string } {
  if (!value || typeof value !== "object" || Array.isArray(value)) return false;
  const v = value as Record<string, unknown>;
  return Object.keys(v).sort().join(",") === "contact,order_ref" && validOrderRef(v.order_ref) && validContact(v.contact);
}

/** Go answers exactly {order_id}; anything else is treated as unavailable by the BFF. */
export function validLookupResult(value: unknown): value is { order_id: string } {
  if (!value || typeof value !== "object" || Array.isArray(value)) return false;
  const v = value as Record<string, unknown>;
  return Object.keys(v).join(",") === "order_id" && typeof v.order_id === "string" && ORDER_ID.test(v.order_id);
}
