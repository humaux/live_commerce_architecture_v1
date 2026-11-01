// Purpose: pure display helpers of the buyer order status page (short order number, masked phone, recipient / pickup address lines).
// Depends on: lib/purchase (Order type), lib/cvs-contract (isCvsKind), lib/cvs-copy (chain names), components/order-status/copy.ts.
// Used by: OrderStatusView.tsx, OrderTerminalView.tsx. Invariants: display only, no money maths, no network; the short number is the same
// XXXX-XXXX-XXXX the order mails print (internal/notify OrderNumber: first 12 hex digits of the order id).
import type { Locale } from "@live-commerce/i18n";
import { isCvsKind } from "../../lib/cvs-contract";
import { cvsCopy } from "../../lib/cvs-copy";
import { countryName } from "../../lib/country-name";
import type { Order } from "../../lib/purchase";
import { orderStatusCopy } from "./copy";

/** The mail order number, e.g. 0123-ABCD-4567; the full id is never shown. */
export function shortOrderNumber(orderID: string): string {
  const hex = orderID.replace(/-/g, "").toUpperCase().slice(0, 12);
  return `${hex.slice(0, 4)}-${hex.slice(4, 8)}-${hex.slice(8, 12)}`;
}

/** Fixed-shape mask (first two and last two digits): the frames show 09•• ••• ••01 whatever the length. */
export function maskPhone(phone: string): string {
  const digits = phone.replace(/\D/g, "");
  return digits.length < 6 ? "••••" : `${digits.slice(0, 2)}•• ••• ••${digits.slice(-2)}`;
}

/** Delivery block lines in frame order: service (or pickup chain), recipient · masked phone, [store · code], address. */
export function addressLines(order: Order, locale: Locale): string[] {
  const c = orderStatusCopy[locale];
  const dest = order.snapshot.destination;
  const who = `${dest.recipient_name} · ${maskPhone(dest.phone)}`;
  const tail = dest.country && dest.country !== "TW" ? [countryName(locale, dest.country)] : [];
  if (dest.pickup) {
    const chain = isCvsKind(dest.pickup.kind) ? cvsCopy[locale].chains[dest.pickup.kind] : dest.pickup.kind;
    return [`${cvsCopy[locale].pickupSection} · ${chain}`, who, `${dest.pickup.name} · ${c.codeLabel} ${dest.pickup.code}`, dest.pickup.address, ...tail];
  }
  const svc = order.snapshot.service;
  const name = locale === "en" ? svc.name_en : locale === "zh-TW" ? svc.name_hant : svc.name_hans;
  const home = dest.home_address;
  const address = locale === "en"
    ? [home.line1, home.line2, home.city, home.region, home.postal_code].filter(Boolean).join(", ")
    : `${home.postal_code}${home.region}${home.city}${home.line1}${home.line2 ? ` ${home.line2}` : ""}`;
  return [name, who, address, ...tail];
}
