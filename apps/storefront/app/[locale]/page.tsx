// GET /{locale}: the shop home, server-rendered from the published design document (or the draft in preview mode).
// Go: design/published|preview (layout + here, one cached fetch) and catalog-v2 lists inside components/HomeSections.tsx.
// Crawlable: every section is HTML in the first response. Title = store name; Open Graph image = first hero/logo (absolute
// via metadataBase from the layout). A store with no published version renders the contract default (name + product grid).
import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import HomeSections from "../../components/HomeSections";
import { storeImage } from "../../lib/routes";
import { alternates, gate } from "../../lib/shop-page";

export async function generateMetadata({ params }: { params: Promise<{ locale: string }> }): Promise<Metadata> {
  const { locale } = await params;
  if (!isLocale(locale)) return {};
  const { shop } = await gate();
  const { profile, home } = shop.design;
  const hero = home.sections.find((s) => s.type === "hero" && s.image_id);
  const image = hero && hero.type === "hero" && hero.image_id ? hero.image_id : profile.logo_image_id;
  return {
    title: { absolute: profile.tagline ? `${profile.name} | ${profile.tagline}` : profile.name },
    alternates: alternates(locale, ""),
    openGraph: { title: profile.name, description: profile.tagline ?? undefined, images: image ? [storeImage(image)] : undefined, url: `/${locale}` },
  };
}

export default async function Page({ params }: { params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const { shop, preview } = await gate();
  return (
    <main>
      <HomeSections sections={shop.design.home.sections} locale={locale} preview={preview} name={shop.design.profile.name} />
    </main>
  );
}
