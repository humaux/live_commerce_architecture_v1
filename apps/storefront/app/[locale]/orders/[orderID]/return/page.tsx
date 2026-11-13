// Purpose: private localized buyer return route; server renders no protected order facts.
// Depends on: Next routing, Locale and ReturnRequestForm. Called by: staged buyer entry integration.
// Invariants: UUIDs validated; request is a UI-only exact-detail selector, never an API query alias.
import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import ReturnRequestForm from "../../../../../components/order-status/ReturnRequestForm";
export const metadata: Metadata = { referrer: "no-referrer", robots: { index: false, follow: false } };
export default async function Page({ params, searchParams }: {
  params: Promise<{ locale: string; orderID: string }>;
  searchParams: Promise<{ request?: string | string[] }>;
}) {
  const { locale, orderID } = await params;
  const { request } = await searchParams;
  const uuid = /^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/;
  if (!isLocale(locale) || !uuid.test(orderID) || (request !== undefined && (typeof request !== "string" || !uuid.test(request)))) notFound();
  return <ReturnRequestForm key={`${locale}:${orderID}:${request ?? ""}`} locale={locale} orderID={orderID} requestID={request} />;
}
