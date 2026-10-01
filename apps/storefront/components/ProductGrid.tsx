"use client";

// Product grid with cursor "Load more" for the list pages (collection, all products, search) and the home product_grid.
// BFF/Go: GET /api/shop/products?... (app/api/shop/products/route.ts) -> Go /v1/buyer/catalog/v2/products. The first page is
// server-rendered by the parent page (crawlable, works without JS); this island only appends later pages. `query` is the
// canonical list query WITHOUT the cursor; the cursor travels as `after` and is opaque (Go binds it to the filter).
// Without JS the parent renders a plain "next page" link as a fallback.
import { useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import type { ProductCard } from "../lib/shop-contract";
import { parseProductList } from "../lib/shop-contract";
import { shopCopy } from "../lib/shop-copy";
import ProductCardView from "./ProductCard";

export default function ProductGrid({
  locale,
  initial,
  next,
  currency,
  query,
  preview,
}: {
  locale: Locale;
  initial: ProductCard[];
  next: string | null;
  currency: string;
  query: string;
  preview: string | null;
}) {
  const copy = shopCopy[locale];
  const [cards, setCards] = useState(initial);
  const [cursor, setCursor] = useState(next);
  const [state, setState] = useState<"idle" | "loading" | "failed">("idle");
  async function more() {
    if (!cursor || state === "loading") return;
    setState("loading");
    try {
      const params = new URLSearchParams(query);
      params.set("after", cursor);
      const response = await fetch(`/api/shop/products?${params}`, { cache: "no-store", credentials: "omit" });
      const page = response.ok ? parseProductList(await response.json()) : null;
      if (!page) throw new Error("bad page");
      setCards((old) => {
        const seen = new Set(old.map((c) => c.id));
        return [...old, ...page.products.filter((c) => !seen.has(c.id))];
      });
      setCursor(page.next);
      setState("idle");
    } catch {
      setState("failed");
    }
  }
  return (
    <>
      <ul className="sf-grid" data-testid="product-grid">
        {cards.map((card, i) => (
          <ProductCardView key={card.id} locale={locale} card={card} currency={currency} preview={preview} eager={i < 4} />
        ))}
      </ul>
      {cursor && (
        <div className="sf-more">
          <button type="button" className="sf-btn sf-btn--ghost" onClick={() => void more()} disabled={state === "loading"} data-testid="load-more">
            {state === "loading" ? copy.loading : copy.loadMore}
          </button>
          {state === "failed" && (
            <p className="sf-muted" role="alert">
              {copy.loadFailed}
            </p>
          )}
        </div>
      )}
    </>
  );
}
