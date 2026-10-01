// Page-level guards for server components: how a page reacts when the store design or catalog cannot be read.
// closed (unknown host, unpublished store, 404 from Go) -> notFound() so the HTTP status is a real 404 and
// app/[locale]/not-found.tsx can explain it; down (Go unreachable/5xx/garbled) -> throw so app/[locale]/error.tsx renders the
// branded retry state with a 500. Never renders a half page. No BFF/Go call of its own beyond lib/shop-upstream.ts.
import { notFound } from "next/navigation";
import type { Locale } from "@live-commerce/i18n";
import { loadShop, previewToken } from "./shop-upstream";
import type { Fetched, Shop } from "./shop-upstream";
import { shopCopy } from "./shop-copy";
import type { Metadata } from "next";

export async function gate(): Promise<{ shop: Extract<Shop, { state: "ok" }>; preview: string | null }> {
  const preview = await previewToken();
  const shop = await loadShop(preview);
  if (shop.state === "closed") notFound();
  if (shop.state === "down") throw new Error("storefront_unavailable");
  return { shop, preview: shop.draft ? preview : null };
}

export function must<T>(result: Fetched<T>): T {
  if (result.state === "missing") notFound();
  if (result.state === "down") throw new Error("storefront_unavailable");
  return result.value;
}

// canonical + hreflang for a path that exists under all three UI locales.
export function alternates(locale: Locale, path: string): NonNullable<Metadata["alternates"]> {
  return {
    canonical: `/${locale}${path}`,
    languages: { "zh-CN": `/zh-CN${path}`, "zh-TW": `/zh-TW${path}`, en: `/en${path}` },
  };
}

export const noindex: Metadata["robots"] = { index: false, follow: false };
export const chrome = (locale: Locale) => shopCopy[locale];
