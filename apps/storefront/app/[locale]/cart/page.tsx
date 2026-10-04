// GET /{locale}/cart: the cart page. Server part only validates the locale and renders the heading; the cart itself is the
// client island components/CartView.tsx (BFF /api/buyer/cart behind the buyer session cookie; private per buyer, so it is never
// server-rendered or indexed). noindex.
import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import CartView from "../../../components/CartView";
import { shopCopy } from "../../../lib/shop-copy";
import { gate, noindex } from "../../../lib/shop-page";

export async function generateMetadata({ params }: { params: Promise<{ locale: string }> }): Promise<Metadata> {
  const { locale } = await params;
  return isLocale(locale) ? { title: shopCopy[locale].cartTitle, robots: noindex } : {};
}

export default async function Page({ params }: { params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  await gate();
  return (
    <main className="sf-wrap">
      <header className="sf-pagehead">
        <h1>{shopCopy[locale].cartTitle}</h1>
      </header>
      <CartView locale={locale} />
    </main>
  );
}
