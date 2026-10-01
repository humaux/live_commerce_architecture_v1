// GET /{locale}/pages/{slug}: a merchant-written page (about, shipping notes...) from the design document `pages[]`.
// Go: design/published|preview only (the page lives in the document, no catalog read). Body = restricted markdown rendered by
// packages/markdown-lite (escape-first, https links only). Unknown slug -> 404.
import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import { Prose } from "../../../../components/HomeSections";
import PageHead from "../../../../components/PageHead";
import { withPreview } from "../../../../lib/design";
import { shopCopy } from "../../../../lib/shop-copy";
import { alternates, gate } from "../../../../lib/shop-page";

type Params = Promise<{ locale: string; slug: string }>;

export async function generateMetadata({ params }: { params: Params }): Promise<Metadata> {
  const { locale, slug } = await params;
  if (!isLocale(locale)) return {};
  const { shop } = await gate();
  const page = shop.design.pages.find((p) => p.slug === slug);
  if (!page) return { robots: { index: false } };
  const plain = page.body.replace(/[*\-[\]()]/g, "").replace(/\s+/g, " ").trim().slice(0, 160);
  return { title: page.title, description: plain || undefined, alternates: alternates(locale, `/pages/${slug}`) };
}

export default async function Page({ params }: { params: Params }) {
  const { locale, slug } = await params;
  if (!isLocale(locale)) notFound();
  const { shop, preview } = await gate();
  const page = shop.design.pages.find((p) => p.slug === slug);
  if (!page) notFound();
  const copy = shopCopy[locale];
  return (
    <main className="sf-wrap sf-wrap--narrow">
      <PageHead crumbs={[{ href: withPreview(`/${locale}`, preview), label: copy.home }]} label={copy.breadcrumb} title={page.title} />
      <Prose markdown={page.body} />
    </main>
  );
}
