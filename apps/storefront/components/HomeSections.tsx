// The five home section types of the store design document (contracts/storefront-v2.md section B): hero, featured_collection,
// product_grid, rich_text, image_text. Server components. Go reads: catalog-v2 list (product_grid, featured_collection) and
// collection (featured_collection title/existence) via lib/shop-upstream.ts. Each type has its own shape on purpose (full-bleed
// hero, horizontal rail, grid, narrow prose, split image/text) so a home page reads as a composed shop, not a stack of
// identical blocks. A section whose data is missing (hidden/deleted collection, no image) degrades or disappears; it never
// breaks the page. Merchant text is React text; rich text goes through packages/markdown-lite, which escapes before tagging.
import Link from "next/link";
import type { Locale } from "@live-commerce/i18n";
import { renderMarkdown } from "../../../packages/markdown-lite/src/index.ts";
import { internalHref, withPreview } from "../lib/design";
import type { Section } from "../lib/design";
import { collectionPath, storeImage } from "../lib/routes";
import { getCollection, listProducts } from "../lib/shop-upstream";
import { shopCopy } from "../lib/shop-copy";
import { ArrowRightIcon } from "./icons";
import ProductCardView from "./ProductCard";

export function Prose({ markdown }: { markdown: string }) {
  // renderMarkdown emits only p/ul/li/strong/em/br/a(https, nofollow) from fully escaped input.
  return <div className="sf-prose" dangerouslySetInnerHTML={{ __html: renderMarkdown(markdown) }} />;
}

function Hero({ section, locale, preview, h1 }: { section: Extract<Section, { type: "hero" }>; locale: Locale; preview: string | null; h1: boolean }) {
  const href = section.cta_kind ? internalHref(locale, section.cta_kind, section.cta_target) : null;
  const Heading = h1 ? "h1" : "h2";
  return (
    <section className={`sf-hero${section.image_id ? "" : " sf-hero--plain"}`} data-testid="section-hero">
      {section.image_id && <img src={storeImage(section.image_id)} alt="" fetchPriority="high" decoding="async" />}
      <div className="sf-hero__copy">
        {section.heading && <Heading>{section.heading}</Heading>}
        {section.subheading && <p>{section.subheading}</p>}
        {href && section.cta_label && (
          <Link className="sf-btn sf-btn--light" href={withPreview(href, preview)}>
            {section.cta_label}
            <ArrowRightIcon />
          </Link>
        )}
      </div>
    </section>
  );
}

function Empty({ locale }: { locale: Locale }) {
  const copy = shopCopy[locale];
  return (
    <section className="sf-section">
      <div className="sf-empty" data-testid="home-empty">
        <p className="sf-empty__title">{copy.noProducts}</p>
        <p className="sf-muted">{copy.noProductsHint}</p>
      </div>
    </section>
  );
}

async function ProductGridSection({ section, locale, preview }: { section: Extract<Section, { type: "product_grid" }>; locale: Locale; preview: string | null }) {
  const copy = shopCopy[locale];
  const result = await listProducts({ sort: section.sort, limit: section.limit });
  if (result.state === "down") throw new Error("storefront_unavailable");
  if (result.state !== "ok" || result.value.products.length === 0) return <Empty locale={locale} />;
  const sort = section.sort === "newest" ? "" : `?sort=${section.sort}`;
  return (
    <section className="sf-section" data-testid="section-product-grid">
      <div className="sf-section__head">
        <h2>{section.heading ?? copy.newArrivals}</h2>
        <Link className="sf-link" href={withPreview(`/${locale}/products${sort}`, preview)}>
          {copy.viewAll}
        </Link>
      </div>
      <ul className="sf-grid">
        {result.value.products.map((card, i) => (
          <ProductCardView key={card.id} locale={locale} card={card} currency={result.value.store.currency} preview={preview} eager={i < 2} />
        ))}
      </ul>
    </section>
  );
}

async function FeaturedSection({ section, locale, preview }: { section: Extract<Section, { type: "featured_collection" }>; locale: Locale; preview: string | null }) {
  const copy = shopCopy[locale];
  const [info, result] = await Promise.all([getCollection(section.collection_slug), listProducts({ collection: section.collection_slug, limit: section.limit })]);
  if (info.state === "down" || result.state === "down") throw new Error("storefront_unavailable");
  if (info.state !== "ok" || result.state !== "ok" || result.value.products.length === 0) return null;
  return (
    <section className="sf-section sf-section--rail" data-testid="section-featured">
      <div className="sf-section__head">
        <h2>{section.heading ?? info.value.title}</h2>
        <Link className="sf-link" href={withPreview(collectionPath(locale, section.collection_slug), preview)}>
          {copy.viewAll}
        </Link>
      </div>
      <ul className="sf-rail">
        {result.value.products.map((card) => (
          <ProductCardView key={card.id} locale={locale} card={card} currency={result.value.store.currency} preview={preview} />
        ))}
      </ul>
    </section>
  );
}

export default async function HomeSections({ sections, locale, preview, name }: { sections: Section[]; locale: Locale; preview: string | null; name: string }) {
  const firstIsHeroWithHeading = sections[0]?.type === "hero" && !!sections[0].heading;
  return (
    <>
      {!firstIsHeroWithHeading && <h1 className="sr-only">{name}</h1>}
      {sections.map((section, index) => {
        const key = `${index}:${section.type}`;
        switch (section.type) {
          case "hero":
            return <Hero key={key} section={section} locale={locale} preview={preview} h1={index === 0} />;
          case "featured_collection":
            return <FeaturedSection key={key} section={section} locale={locale} preview={preview} />;
          case "product_grid":
            return <ProductGridSection key={key} section={section} locale={locale} preview={preview} />;
          case "rich_text":
            return (
              <section key={key} className="sf-section sf-section--prose" data-testid="section-rich-text">
                {section.heading && <h2>{section.heading}</h2>}
                <Prose markdown={section.body} />
              </section>
            );
          case "image_text":
            return (
              <section key={key} className={`sf-split sf-split--${section.image_side}`} data-testid="section-image-text">
                {section.image_id && (
                  <div className="sf-split__media">
                    <img src={storeImage(section.image_id)} alt="" loading="lazy" decoding="async" />
                  </div>
                )}
                <div className="sf-split__copy">
                  {section.heading && <h2>{section.heading}</h2>}
                  <Prose markdown={section.body} />
                </div>
              </section>
            );
        }
      })}
    </>
  );
}
