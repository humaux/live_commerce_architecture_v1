// Copy for the storefront home (catalog-media CM6), three locales. Generic states (loading/retry/failed) reuse
// purchase-copy.ts so the shop speaks with one voice; only home-specific words live here.
import type { Locale } from "@live-commerce/i18n";

export const homeCopy: Record<
  Locale,
  { from: string; empty: string; emptyHint: string; noPhoto: string; viewProduct: string; products: string }
> = {
  en: {
    from: "From",
    empty: "No products yet",
    emptyHint: "This shop has nothing for sale right now. Please check back soon.",
    noPhoto: "No photo",
    viewProduct: "View",
    products: "Products",
  },
  "zh-CN": {
    from: "起",
    empty: "暂无商品",
    emptyHint: "本店目前没有在售商品，请稍后再来看看。",
    noPhoto: "暂无图片",
    viewProduct: "查看",
    products: "商品",
  },
  "zh-TW": {
    from: "起",
    empty: "尚無商品",
    emptyHint: "本店目前沒有在售商品，請稍後再來看看。",
    noPhoto: "尚無圖片",
    viewProduct: "查看",
    products: "商品",
  },
};
