// Storefront shell for every /{locale}/... page: document language, accent colour variables, announcement bar, header,
// footer, the cart provider/drawer and the preview banner, all from the published design document (or the draft when a
// preview token is present). BFF/Go: GET /v1/buyer/design/published | design/preview through lib/shop-upstream.ts
// (BFF key + Host-derived origin, never a request-supplied store). The cart provider talks to /api/buyer/* (see CartProvider).
// This layout must not throw or 404 when the design is unavailable: claim, privacy and legal pages share it and keep working
// with a neutral default shell; pages that need store data call gate() (lib/shop-page.ts) themselves.
// Indexing: published stores are indexable by default (robots.txt/sitemap exist); preview, cart, checkout, search, claim
// and privacy set noindex in their own metadata; an unpublished/unknown host is noindex here.
import type { Metadata } from "next";
import type { CSSProperties } from "react";
import { notFound } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import type { Locale } from "@live-commerce/i18n";
import CartDrawer from "../../components/CartDrawer";
import CartProvider from "../../components/CartProvider";
import { Announcement, PreviewBanner, ShopFooter, ShopHeader } from "../../components/ShopChrome";
import { accentText, defaultDesign, onAccent } from "../../lib/design";
import { storeImage } from "../../lib/routes";
import { shopCopy } from "../../lib/shop-copy";
import { loadShop, previewToken } from "../../lib/shop-upstream";
import "../globals.css";
import "../shop.css";

export const dynamic = "force-dynamic";

export async function generateMetadata({ params }: { params: Promise<{ locale: string }> }): Promise<Metadata> {
  const { locale } = await params;
  const shop = await loadShop(await previewToken());
  if (!isLocale(locale) || shop.state !== "ok") return { robots: { index: false, follow: false } };
  const { profile } = shop.design;
  return {
    metadataBase: new URL(shop.origin),
    title: { default: profile.name, template: `%s | ${profile.name}` },
    description: profile.tagline ?? shopCopy[locale].shareDescription.replace("{name}", profile.name),
    applicationName: profile.name,
    icons: profile.favicon_image_id ? { icon: storeImage(profile.favicon_image_id) } : undefined,
    robots: shop.draft ? { index: false, follow: false } : { index: true, follow: true },
    openGraph: { siteName: profile.name, type: "website", locale },
  };
}

export default async function Layout({ children, params }: { children: React.ReactNode; params: Promise<{ locale: string }> }) {
  const { locale: raw } = await params;
  if (!isLocale(raw)) notFound();
  const locale = raw as Locale;
  const preview = await previewToken();
  const shop = await loadShop(preview);
  const design = shop.state === "ok" ? shop.design : defaultDesign("");
  const accent = design.profile.accent_color;
  const style = { "--accent": accent, "--on-accent": onAccent(accent), "--accent-text": accentText(accent) } as CSSProperties;
  const copy = shopCopy[locale];
  // Link tokens survive navigation only while the draft is actually being shown (an expired token silently falls back).
  const keep = shop.state === "ok" && shop.draft ? preview : null;
  return (
    <html lang={locale} style={style}>
      <body>
        <CartProvider>
          <a className="sf-skip" href="#main">
            {copy.skip}
          </a>
          {shop.state === "ok" && shop.draft && <PreviewBanner locale={locale} />}
          {design.profile.announcement && <Announcement text={design.profile.announcement} />}
          <ShopHeader locale={locale} design={design} preview={keep} />
          <div id="main" tabIndex={-1} className="sf-main">
            {children}
          </div>
          <ShopFooter locale={locale} design={design} preview={keep} />
          <CartDrawer locale={locale} />
        </CartProvider>
      </body>
    </html>
  );
}
