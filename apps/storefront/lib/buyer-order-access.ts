// Purpose: declare the browser-only buyer order session aliases and mode-bound cookie names.
// Depends on: canonical order UUIDs and the two order access modes.
// Used by: the buyer BFF. Shopping and private Go session routes keep their existing contracts.
import { BUYER_UUID, type OrderAccess } from "./buyer-query.ts";

export const ORDER_COOKIES = {
  lookup: "__Host-commerce_buyer_lookup",
  link: "__Host-commerce_buyer_link",
} satisfies Record<OrderAccess, string>;

export function orderCookieLabel(access?: OrderAccess): string {
  return access ? `buyer-order-${access}-cookie-v1` : "buyer-cookie-v1";
}

export function orderSessionRoute(suffix: string, method: string) {
  const [resource, orderID, subresource, action, extra] = suffix.split("/");
  if (resource !== "orders" || subresource !== "session" ||
      (action !== undefined && action !== "logout") || extra !== undefined)
    return undefined;
  if (!BUYER_UUID.test(orderID)) return { known: true, invalidID: true };
  const logout = action === "logout";
  return {
    known: true,
    route: method === (logout ? "POST" : "GET") ? {
      method, privatePath: "session", body: logout ? "empty" as const : undefined,
      session: logout ? "logout" : "status", orderSession: true,
      orderID, query: "orderAccess" as const,
    } : undefined,
  };
}
