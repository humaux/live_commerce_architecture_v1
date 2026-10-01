// Owns the /{locale}/orders/lookup route (contracts/storefront-v2.md §E5): server-side locale validation and a non-indexed, no-referrer document
// shell for LookupForm, which posts to the BFF POST /api/buyer/orders/lookup (-> Go internal/buyerhttp/lookup.go).
// Non-goals: the server renders no order data and never sees the order number or the contact (the form posts them from the browser).

import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import LookupForm from "./LookupForm";

export const metadata: Metadata = {
  referrer: "no-referrer",
  robots: { index: false, follow: false },
};

export default async function Page({ params }: { params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  return <LookupForm locale={locale} />;
}
