"use client";

// Language links: the same page under the other two UI locales (zh-CN / zh-TW / en). No BFF/Go call. The path is the
// current one with its first segment replaced, so switching language never loses the page, the query (filters, search)
// or the preview token. Merchant text does not change with the UI language (contracts/storefront-v2.md preamble).
import { usePathname, useSearchParams } from "next/navigation";
import { localeNames, locales } from "@live-commerce/i18n";
import type { Locale } from "@live-commerce/i18n";
import { shopCopy } from "../lib/shop-copy";

export default function LocaleSwitch({ locale }: { locale: Locale }) {
  const pathname = usePathname() ?? `/${locale}`;
  const search = useSearchParams();
  const rest = pathname.split("/").slice(2).join("/");
  const query = search?.toString();
  return (
    <nav className="sf-locales" aria-label={shopCopy[locale].language}>
      {locales.map((next) => (
        <a
          key={next}
          href={`/${next}${rest ? `/${rest}` : ""}${query ? `?${query}` : ""}`}
          lang={next}
          hrefLang={next}
          aria-current={next === locale ? "true" : undefined}
        >
          {localeNames[next]}
        </a>
      ))}
    </nav>
  );
}
