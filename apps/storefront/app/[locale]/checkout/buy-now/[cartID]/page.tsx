// Purpose: resume an owned Buy now selection on the standard checkout flow.
// Depends on: the storefront gate and the buyer BFF's checkout-only cart read.
// Used by: product Buy now and the checkout's CVS map return.
import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import CheckoutFlow from "../../../../../components/CheckoutFlow";
import { buyerDemoLabel } from "../../../../../lib/demo-label";
import { shopCopy } from "../../../../../lib/shop-copy";
import { gate, noindex } from "../../../../../lib/shop-page";

type Props = { params: Promise<{ locale: string; cartID: string }> };

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { locale } = await params;
  return isLocale(locale) ? { title: shopCopy[locale].checkoutTitle, robots: noindex } : {};
}

export default async function Page({ params }: Props) {
  const { locale, cartID } = await params;
  if (!isLocale(locale) || !/^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/.test(cartID)) notFound();
  await gate();
  return <CheckoutFlow key={cartID} locale={locale} buyNowID={cartID} demonstration={buyerDemoLabel(process.env)} />;
}
