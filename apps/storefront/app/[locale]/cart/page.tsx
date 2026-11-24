// GET /{locale}/cart: the cart page. Server part only validates the locale and renders the heading; the cart itself is the
// client island components/CartView.tsx (BFF /api/buyer/cart behind the buyer session cookie; private per buyer, so it is never
// server-rendered or indexed). noindex.
import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import CartView from "../../../components/CartView";
import { ChevronLeftIcon } from "../../../components/icons";
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
  const copy = shopCopy[locale];
  return (
    <main className="sf-wrap sf-cart-main">
      <header className="sf-pagehead">
        <Link className="sf-link" href={`/${locale}/products`}>
          <ChevronLeftIcon />
          {copy.continueShopping}
        </Link>
        <h1>{copy.cartTitle}</h1>
        <p className="sf-pagehead__intro">{copy.cartIntro}</p>
      </header>
      <CartView locale={locale} />
    </main>
  );
}
