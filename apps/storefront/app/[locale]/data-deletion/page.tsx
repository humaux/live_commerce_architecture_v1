// Owns the public /{locale}/data-deletion route (customers-billing-v1 §5 Storefront line, Q9, Meta App Review
// "data deletion instructions" URL, F-P2): static instructions, no API call, no cookie, no personal data.
// Non-goals: not a Meta data-deletion callback (Q9: instructions page only in v1) and not a request form; store data is
// erased from /{locale}/privacy or by asking the store. Indexable on purpose (App Review must fetch it), unlike the
// rest of the storefront (layout.tsx sets noindex).
// Layout and copy: signed frames 5:15707 / 5:15793 (shopping data, Facebook / Instagram data, contact the store).
// Depends on: lib/privacy-copy.ts (language label), components/privacy/copy.ts, components/privacy/ContactStoreButton.tsx.
import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import LocaleSelect from "../../../components/LocaleSelect";
import ContactStoreButton from "../../../components/privacy/ContactStoreButton";
import { privacyFrameCopy } from "../../../components/privacy/copy";
import { privacyCopy } from "../../../lib/privacy-copy";

export const metadata: Metadata = { robots: { index: true, follow: true } };

export default async function Page({ params }: { params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const f = privacyFrameCopy[locale];
  return (
    <>
      <header className="shop-header">
        <span>{f.deletionTitle}</span>
        <LocaleSelect locale={locale} label={privacyCopy[locale].language} path="/data-deletion" />
      </header>
      <main className="purchase-main claim-main" data-testid="data-deletion">
        <h1>{f.deletionTitle}</h1>
        <p className="claim-intro">{f.deletionSub}</p>
        <section className="claim-preview" aria-labelledby="deletion-shop-title">
          <h2 id="deletion-shop-title">{f.deletionShopTitle}</h2>
          <p>{f.deletionShopText}</p>
          <p className="claim-note">{f.deletionShopKept}</p>
          <p>
            <a className="tap-link" href={`/${locale}/privacy`} data-testid="data-deletion-privacy-link">{f.deletionGo}</a>
          </p>
        </section>
        <section className="claim-preview" aria-labelledby="deletion-meta-title">
          <h2 id="deletion-meta-title">{f.deletionMetaTitle}</h2>
          <p>{f.deletionMetaText}</p>
        </section>
        <p>{f.deletionHelp}</p>
        <p>
          <ContactStoreButton label={f.contact} locale={locale} />
        </p>
      </main>
    </>
  );
}
