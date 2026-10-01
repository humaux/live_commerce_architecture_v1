// GET /{locale}/checkout: delivery, quotation, address, payment and order screens (components/CheckoutFlow.tsx).
// BFF: /api/buyer/* (cart, checkout-options, quotes, destination, checkout, orders) -> Go /v1/buyer/*. The path is on the CVS map return
// allowlist (internal/checkout/cvs.go returnPathPattern, migration 0093). The server part validates the locale and the store; buyer capability activation and
// every private read happen in the browser coordinator, never SSR. noindex (private to the buyer).
import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import CheckoutFlow from "../../../components/CheckoutFlow";
import { buyerDemoLabel } from "../../../lib/demo-label";
import { shopCopy } from "../../../lib/shop-copy";
import { gate, noindex } from "../../../lib/shop-page";

export async function generateMetadata({ params }: { params: Promise<{ locale: string }> }): Promise<Metadata> {
  const { locale } = await params;
  return isLocale(locale) ? { title: shopCopy[locale].checkoutTitle, robots: noindex } : {};
}

export default async function Page({ params }: { params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  await gate();
  return <CheckoutFlow locale={locale} demonstration={buyerDemoLabel(process.env)} />;
}
