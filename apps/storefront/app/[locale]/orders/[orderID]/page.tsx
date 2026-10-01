// Owns the /{locale}/orders/{orderID} route: the buyer's own order page, the target of the order mails (contracts/storefront-v2.md §E4) and of
// the guest lookup redirect (§E5). Server-side it validates only the locale and the id shape; OrderPage reads the order through the buyer
// capability cookie in the browser, exactly like the order history. Non-goal: no order data is rendered by the server, and an id that is not the
// visitor's own order reads as "could not be loaded", never as someone else's order.

import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import OrderPage from "./OrderPage";

export const metadata: Metadata = {
  referrer: "no-referrer",
  robots: { index: false, follow: false },
};

export default async function Page({ params }: { params: Promise<{ locale: string; orderID: string }> }) {
  const { locale, orderID } = await params;
  if (!isLocale(locale) || !/^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/.test(orderID)) notFound();
  return <OrderPage locale={locale} orderID={orderID} />;
}
