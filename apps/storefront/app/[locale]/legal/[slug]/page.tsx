// Owns the /{locale}/legal/{slug} route (slug in lib/legal-copy.ts legalSlugs): a static, server-rendered
// policy page. Calls no BFF / no Go endpoint (static); reads only lib/legal-copy.ts.
// Non-goals: no consent capture, no deletion execution (data-deletion and privacy self-service belong
// to customers-billing-ui), no per-store text. Unknown locale or slug -> notFound().
// Owner text: a page is shown only when every text on it is final (legalPublished). Otherwise buyers get an ordinary page that says the
// store has not published it yet (data-testid="legal-unpublished", noindex), never a developer placeholder; the footer links only
// published pages. LC_LEGAL_REQUIRE_FINAL (LG01) fails while any page renders the unpublished page.

import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import PageHead from "../../../../components/PageHead";
import { legalChrome, legalPage, legalPublished, legalSlugs } from "../../../../lib/legal-copy";
import type { LegalSlug } from "../../../../lib/legal-copy";
import { shopCopy } from "../../../../lib/shop-copy";
import styles from "../legal.module.css";

type Params = Promise<{ locale: string; slug: string }>;
const isSlug = (v: string): v is LegalSlug => (legalSlugs as readonly string[]).includes(v);

export async function generateMetadata({ params }: { params: Params }): Promise<Metadata> {
  const { locale, slug } = await params;
  if (!isLocale(locale) || !isSlug(slug)) return {};
  return { title: legalPage(locale, slug).title, ...(legalPublished(locale, slug) ? {} : { robots: { index: false, follow: false } }) };
}

export default async function Page({ params }: { params: Params }) {
  const { locale, slug } = await params;
  if (!isLocale(locale) || !isSlug(slug)) notFound();
  const page = legalPage(locale, slug);
  const copy = shopCopy[locale];
  const head = <PageHead crumbs={[{ href: `/${locale}`, label: copy.home }]} label={copy.breadcrumb} title={page.title} />;
  if (!legalPublished(locale, slug))
    return (
      <main lang={locale} className="sf-wrap sf-wrap--narrow" data-testid="legal-unpublished" data-legal-slug={slug}>
        {head}
        <p className="sf-prose">{copy.legalUnpublished}</p>
        <p className={styles.back}>
          <Link className="sf-btn sf-btn--ghost" href={`/${locale}`}>
            {copy.backHome}
          </Link>
        </p>
      </main>
    );
  const Wrap = slug === "contact" ? "address" : "div";
  return (
    <main lang={locale} className={`sf-wrap sf-wrap--narrow ${styles.page}`} data-testid="legal-page" data-legal-slug={slug}>
      {head}
      <p className={styles.updated}>
        {legalChrome(locale).updated}: {page.updated.text}
      </p>
      {page.sections.map((s) => (
        <section key={s.heading}>
          <h2>{s.heading}</h2>
          <Wrap>
            {s.body.map((t, i) => (
              <p key={i}>{t.text}</p>
            ))}
          </Wrap>
        </section>
      ))}
    </main>
  );
}
