// Manual-order buyer link contract (contracts/storefront-v2.md section G3): the `#o=<order id>&t=<token>` fragment grammar, the request body the
// BFF accepts for POST /api/buyer/orders/link (-> Go internal/buyerhttp/orderlink.go POST /v1/buyer/orders/link) and the one answer shape it relays.
// Pure (shared by lib/buyer-server.ts, the browser page and node:test). It never decides whether a link is valid (the database does, single-use and
// <= 7 days); it only keeps malformed input off the private transport and out of the URL: the token is read from the fragment, which a browser never sends.

import { ORDER_ID } from "./lookup-contract.ts";

/** 32 random bytes as unpadded base64url (43 characters). */
export const LINK_TOKEN = /^[A-Za-z0-9_-]{43}$/;
/** The browser-bound proof: 32 random bytes as unpadded base64url (43 characters), minted once per page load and re-sent on a retry.
 *  The BFF derives the capability token from (link token, proof), so a lost response is replayed with the SAME capability (K3 F2). */
export const LINK_PROOF = /^[A-Za-z0-9_-]{43}$/;

/** Reads `#o=<order id>&t=<token>` from a location hash; anything else (extra keys, other order, bad token, query-like noise) is null. */
export function orderLinkFragment(hash: string): { orderID: string; token: string } | null {
  const match = /^#o=([0-9a-f-]{36})&t=([A-Za-z0-9_-]{43})$/.exec(hash);
  return match && ORDER_ID.test(match[1]) && LINK_TOKEN.test(match[2]) ? { orderID: match[1], token: match[2] } : null;
}

export function validLinkBody(value: unknown): value is { order_id: string; proof: string; token: string } {
  if (!value || typeof value !== "object" || Array.isArray(value)) return false;
  const v = value as Record<string, unknown>;
  return Object.keys(v).sort().join(",") === "order_id,proof,token" && typeof v.order_id === "string" && ORDER_ID.test(v.order_id) &&
    typeof v.proof === "string" && LINK_PROOF.test(v.proof) &&
    typeof v.token === "string" && LINK_TOKEN.test(v.token);
}

/** Go answers exactly {order_id}; the BFF additionally requires it to be the order the link named. */
export function validLinkResult(value: unknown, orderID: string): value is { order_id: string } {
  if (!value || typeof value !== "object" || Array.isArray(value)) return false;
  const v = value as Record<string, unknown>;
  return Object.keys(v).join(",") === "order_id" && v.order_id === orderID && ORDER_ID.test(orderID);
}
