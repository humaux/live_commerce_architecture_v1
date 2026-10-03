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
        ) : (
          <PlatformDocument locale={locale} page={page} />
        )}
      </main>
      <PlatformFooter locale={locale} />
    </>
  );
}
