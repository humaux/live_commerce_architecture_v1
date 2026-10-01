// Owns the /{locale}/order-link route (contracts/storefront-v2.md section G3): server-side locale validation and a no-referrer, non-indexed document shell
// for OrderLink, which reads the `#o=&t=` fragment in the browser and posts it once to the BFF POST /api/buyer/orders/link (-> Go internal/buyerhttp/orderlink.go).
// Non-goals: the server never sees the token (it lives in the URL fragment) and renders no order data; a crawler or preview bot GET carries no token and
// writes nothing. The Referrer-Policy / CSP headers for this path are set in next.config.ts, the meta tag below repeats the policy.
import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import OrderLink from "../../../components/OrderLink";

export const metadata: Metadata = {
  referrer: "no-referrer",
  robots: { index: false, follow: false },
};

export default async function Page({ params }: { params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  return <OrderLink locale={locale} />;
}
