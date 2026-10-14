// Purpose: Renders one public platform-site page (home, about, privacy, terms, data-deletion, contact) for a locale, with metadata and alternates.
// Depends on: next (Metadata, notFound), lib/company, lib/platform-copy, components/platform/PlatformHome, PlatformAbout, PlatformDocument.
// Used by: Next.js App Router for the platform host (Caddy routes LC_PLATFORM_HOST to /site).
import type { Metadata } from "next";
import { notFound } from "next/navigation";
import {
  company,
  companyConfig,
  platformLocales,
  platformPages,
  platformPath,
  brandedTitle,
  type PlatformLocale,
  type PlatformPage,
} from "../../../../../lib/company";
import { platformCopy } from "../../../../../lib/platform-copy";
import { PlatformHome } from "../../../../../components/platform/PlatformHome";
import {
  PlatformFooter,
  PlatformDocument,
  PlatformAbout,
} from "../../../../../components/platform/PlatformDocument";

type Params = { locale: string; page: string };
function route(p: Params) {
  if (
    !platformLocales.some((v) => v === p.locale) ||
    !platformPages.some((v) => v === p.page)
  )
    notFound();
  return { locale: p.locale as PlatformLocale, page: p.page as PlatformPage };
}
export async function generateMetadata({
  params,
}: {
  params: Promise<Params>;
}): Promise<Metadata> {
  const { locale, page } = route(await params),
    cfg = companyConfig(),
    c = platformCopy[locale];
  return {
    title: brandedTitle(c.pages[page]),
    description: c.intro,
    robots: { index: true, follow: true },
    alternates: {
      canonical: `${cfg.platformOrigin}${platformPath(locale, page)}`,
      languages: Object.fromEntries(
        platformLocales.map((l) => [
          l,
          `${cfg.platformOrigin}${platformPath(l, page)}`,
        ]),
      ),
    },
    other:
      page === "home" && cfg.verification
        ? { "facebook-domain-verification": cfg.verification }
        : undefined,
  };
}
export default async function PlatformPage({
  params,
}: {
  params: Promise<Params>;
}) {
  const { locale, page } = route(await params),
    c = platformCopy[locale],
    cfg = companyConfig();
  return (
    <>
      <a className="ps-skip" href="#main">
        {c.skip}
      </a>
      <header className="ps-header">
        <div className="ps-header-inner">
          <a
            href={platformPath(locale)}
            className="ps-brand"
            aria-label={company.productName}
          >
            {company.productName.split(" ")[0]}{" "}
            <span>{company.productName.split(" ")[1]}</span>
          </a>
          <nav aria-label={c.pages.about} className="ps-languages" data-testid="site-nav">
            <a href={platformPath(locale, "about")}>{c.pages.about}</a>
          </nav>
          <nav aria-label={c.language} className="ps-languages">
            {platformLocales.map((l) => (
              <a
                key={l}
                href={platformPath(l, page)}
                lang={l}
                aria-current={l === locale ? "page" : undefined}
              >
                {{ "zh-TW": "繁體", "zh-CN": "简体", en: "EN" }[l]}
              </a>
            ))}
          </nav>
          <a className="ps-login" href={`${cfg.adminOrigin}/${locale}`}>
            {c.login}
          </a>
        </div>
      </header>
      <main
        id="main"
        tabIndex={-1}
        className={`ps-main ${page === "home" ? "ps-home" : "ps-document"}`}
      >
        {page === "home" ? (
          <PlatformHome locale={locale} />
        ) : page === "about" ? (
          <PlatformAbout locale={locale} />
        ) : (
          <PlatformDocument locale={locale} page={page} />
        )}
      </main>
      <PlatformFooter locale={locale} />
    </>
  );
}
