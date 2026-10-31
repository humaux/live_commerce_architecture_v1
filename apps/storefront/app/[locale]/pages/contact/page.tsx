// Purpose: render the signed buyer contact page (form + ?sent=true confirmation) for the PARITY-BUYER-BROWSE Figma frames.
// Depends on: @live-commerce/i18n isLocale, PageHead, shop-copy, Next metadata/navigation/Link.
// Used by: /{locale}/pages/contact (storefront route; not linked from the legal footer).
// GET /{locale}/pages/contact[?sent=true]: a signed contact form and a simple sent-confirmation state.
// This is a parity page for the buyer Figma frames; it does not persist messages (no backend endpoint) and is not linked from
// the legal footer. The form is static and submits to the same route with ?sent=true for the confirmation state.
import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import PageHead from "../../../../components/PageHead";
import { shopCopy } from "../../../../lib/shop-copy";

type Params = Promise<{ locale: string }>;
type Search = Promise<{ sent?: string }>;

export async function generateMetadata({ params }: { params: Params }): Promise<Metadata> {
  const { locale } = await params;
  return isLocale(locale) ? { title: shopCopy[locale].contactHeading } : {};
}

export default async function Page({ params, searchParams }: { params: Params; searchParams: Search }) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const { sent } = await searchParams;
  const copy = shopCopy[locale];
  const isSent = sent === "true";
  return (
    <main className="sf-wrap sf-wrap--narrow">
      <PageHead crumbs={[{ href: `/${locale}`, label: copy.home }]} label={copy.breadcrumb} title={isSent ? copy.contactSent : copy.contactHeading} />
      {isSent ? (
        <div className="sf-empty sf-empty--compact" data-testid="contact-sent">
          <p className="sf-muted">{copy.contactSent}</p>
          <Link className="sf-btn sf-btn--primary" href={`/${locale}`}>
            {copy.contactBack}
          </Link>
        </div>
      ) : (
        <form className="sf-contact-form" action={`/${locale}/pages/contact`} method="get" data-testid="contact-form">
          <input type="hidden" name="sent" value="true" />
          <label>
            <span>{copy.contactName}</span>
            <input name="name" type="text" autoComplete="name" required />
          </label>
          <label>
            <span>{copy.contactEmail}</span>
            <input name="email" type="email" autoComplete="email" required />
          </label>
          <label>
            <span>{copy.contactMessage}</span>
            <textarea name="message" rows={5} required />
          </label>
          <button type="submit" className="sf-btn sf-btn--primary sf-btn--block">
            {copy.contactSubmit}
          </button>
        </form>
      )}
    </main>
  );
}
