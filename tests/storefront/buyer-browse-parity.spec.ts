// Purpose: Operate the buyer browsing surfaces (terms, collections, search, product list sort + price filter, product add-to-cart, cart, empty cart, removed product) with real user actions and record which signed frames those assertions really cover.
// Depends on: Playwright, shop-real-gate real-stack fixtures (facts.json), ./browser-engine.mjs ctxOpts.
// Used by: tests/storefront/shop-real-gate.mjs in --browser-storefront GitHub CI.
// Invariants: real click/fill/selectOption only (no force, no DOM mutation); the click ledger lists only operations that ran; a frame is COVERED only when
//   every assertion planned for it passed in all three locales; every other frame is NOT_COVERED / DEFERRED / BLOCKED with a reason; the final log line states the true counts.
import assert from "node:assert/strict";
import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import { expect, type Browser, type Page } from "@playwright/test";
import { ctxOpts } from "./browser-engine.mjs";

export interface ParityContext {
  browser: Browser;
  origin: string;
  evidence: string;
  facts?: any;
}

export interface LedgerEntry {
  frame: string;
  locale: string;
  viewport: string;
  route: string;
  control: string;
  operation: string;
  expected: string;
  actual: string;
  environment: string;
  status: "PASS" | "FAIL";
  error?: string;
}

// plan = what this spec tries to assert for the frame; status (written to the manifest) is decided only from the ledger.
export interface FrameSpec {
  frameId: string;
  name: string;
  bucket: "U:b-home" | "U:b-product" | "U:b-cart" | "U:b-live";
  route: string;
  viewport: "1440x900" | "390x844";
  screenshotFile: string;
  deliveryPng: string;
  plan: "assert" | "NOT_COVERED" | "DEFERRED" | "BLOCKED";
  reason?: string;
  deferredTo?: string;
}

export interface ManifestEntry extends FrameSpec {
  status: "COVERED" | "NOT_COVERED" | "DEFERRED" | "BLOCKED";
  asserted?: string[];
}

const T = 10_000;
const num = (text: string) => Number(text.replace(/[^\d.]/g, ""));
const fmt = (template: string, values: Record<string, string | number>) => template.replace(/\{(\w+)\}/g, (_, k) => String(values[k]));
const ascending = (a: number[]) => a.every((n, i) => i === 0 || n >= a[i - 1]);
// price_desc orders by the HIGHEST variant price (contracts/storefront-v2.md) but a card shows only the lowest ("from"), so a strict
// per-card order cannot be read from the page; the list must still trend down from first to last card.
const trendsDown = (a: number[]) => a.length >= 3 && a[0] >= a[a.length - 1];
const cardPrices = async (page: Page) => (await page.getByTestId("product-card").locator(".sf-card__price > span:first-child").allTextContents()).map(num);

// Locale-specific strings the signed frames show (each asserted only in its own locale). Templates use {q} / {n}.
const copy = {
  "zh-TW": {
    termsHeading: "購物條款",
    collectionsOverview: "系列總覽",
    allProducts: "所有商品",
    searchResults: "「{q}」的搜尋結果 · {n} 件",
    searchEmpty: "沒有符合的商品",
    searchEmptyDesc: "請嘗試其他關鍵字",
    search: "搜尋",
    sortBy: "商品排序",
    sortNote: "只改變顯示順序，不影響購物車。",
    sortOptions: { newest: "最新上架", price_asc: "價格：低到高", price_desc: "價格：高到低", title: "商品名稱" },
    cancelSort: "取消",
    priceRange: "價格",
    priceMin: "最低",
    priceMax: "最高",
    apply: "套用",
    clear: "清除篩選",
    addToCart: "加入購物車",
    buyNow: "立即購買",
    productRemoved: "商品已下架或不存在",
    cartTitle: "購物車",
    cartEmptyHint: "選好商品後，可在購物車確認數量。",
    browseProducts: "瀏覽商品",
  },
  "zh-CN": {
    termsHeading: "购物条款",
    collectionsOverview: "系列总览",
    allProducts: "所有商品",
    searchResults: "「{q}」的搜索结果 · {n} 件",
    searchEmpty: "没有符合的商品",
    searchEmptyDesc: "请尝试其他关键词",
    search: "搜索",
    sortBy: "商品排序",
    sortNote: "只改变显示顺序，不影响购物车。",
    sortOptions: { newest: "最新上架", price_asc: "价格：低到高", price_desc: "价格：高到低", title: "商品名称" },
    cancelSort: "取消",
    priceRange: "价格",
    priceMin: "最低",
    priceMax: "最高",
    apply: "应用",
    clear: "清除筛选",
    addToCart: "加入购物车",
    buyNow: "立即购买",
    productRemoved: "商品已下架或不存在",
    cartTitle: "购物车",
    cartEmptyHint: "选好商品后，可在购物车确认数量。",
    browseProducts: "浏览商品",
  },
  en: {
    termsHeading: "Terms & Conditions",
    collectionsOverview: "Collections",
    allProducts: "All products",
    searchResults: 'Search results for "{q}" · {n} items',
    searchEmpty: "No products found",
    searchEmptyDesc: "Try other keywords",
    search: "Search",
    sortBy: "Sort by",
    sortNote: "Only changes the display order, not your cart.",
    sortOptions: { newest: "Newest", price_asc: "Price: Low to High", price_desc: "Price: High to Low", title: "Alphabetical" },
    cancelSort: "Cancel",
    priceRange: "Price",
    priceMin: "Min",
    priceMax: "Max",
    apply: "Apply",
    clear: "Clear filters",
    addToCart: "Add to cart",
    buyNow: "Buy now",
    productRemoved: "Product unavailable or removed",
    cartTitle: "Shopping cart",
    cartEmptyHint: "You can check quantities in your cart after choosing products.",
    browseProducts: "Browse products",
  },
};

// Complete inventory of the 73 Appendix A frames. `plan: "assert"` frames are only COVERED after their assertions pass.
export const PARITY_FRAMES: FrameSpec[] = [
  { frameId: "4:9192", name: "Buyer Desktop · 歡迎來到大夢甄選 · SHOPLINE", bucket: "U:b-home", route: "/[locale]", viewport: "1440x900", screenshotFile: "parity/4-9192.png", deliveryPng: "after-4-9192.png", plan: "NOT_COVERED", reason: "home hero removed: the signed hero copy is one demo brand's text (DaWan Selection), not tenant data, so it cannot ship in a multi-tenant home page" },
  { frameId: "4:9296", name: "Buyer Mobile · 歡迎來到大夢甄選 · SHOPLINE", bucket: "U:b-home", route: "/[locale]", viewport: "390x844", screenshotFile: "parity/4-9296.png", deliveryPng: "after-4-9296.png", plan: "NOT_COVERED", reason: "home hero removed: the signed hero copy is one demo brand's text (DaWan Selection), not tenant data, so it cannot ship in a multi-tenant home page" },
  { frameId: "5:15580", name: "Buyer Desktop · 購物條款 · SHOPLINE", bucket: "U:b-home", route: "/[locale]/legal/terms", viewport: "1440x900", screenshotFile: "parity/5-15580.png", deliveryPng: "after-5-15580.png", plan: "assert" },
  { frameId: "5:15658", name: "Buyer Mobile · 購物條款 · SHOPLINE", bucket: "U:b-home", route: "/[locale]/legal/terms", viewport: "390x844", screenshotFile: "parity/5-15658.png", deliveryPng: "after-5-15658.png", plan: "assert" },
  { frameId: "6:19933", name: "Buyer Desktop · 日常隨行系列 · SHOPLINE", bucket: "U:b-home", route: "/[locale]/collections/daily", viewport: "1440x900", screenshotFile: "parity/6-19933.png", deliveryPng: "verified-daily-series.png", plan: "NOT_COVERED", reason: "frame shows the demo series title; the page shows the merchant's own collection name (tenant data), so the frame copy is not assertable" },
  { frameId: "6:19984", name: "Buyer Mobile · 日常隨行系列 · SHOPLINE", bucket: "U:b-home", route: "/[locale]/collections/daily", viewport: "390x844", screenshotFile: "parity/6-19984.png", deliveryPng: "verified-daily-series.png", plan: "NOT_COVERED", reason: "frame shows the demo series title; the page shows the merchant's own collection name (tenant data), so the frame copy is not assertable" },
  { frameId: "26:624", name: "Buyer Desktop · 搜尋結果 · SHOPLINE", bucket: "U:b-home", route: "/[locale]/search?q=托特包", viewport: "1440x900", screenshotFile: "parity/26-624.png", deliveryPng: "after-26-624.png", plan: "assert" },
  { frameId: "26:850", name: "Buyer Desktop · 聯絡商店 · SHOPLINE", bucket: "U:b-home", route: "/[locale]/pages/contact", viewport: "1440x900", screenshotFile: "parity/26-850.png", deliveryPng: "after-26-850.png", plan: "DEFERRED", reason: "contact route removed from this PR (submitted PII by GET into the URL, fake sent state, no backend)", deferredTo: "#375 feat-contact-store" },
  { frameId: "26:921", name: "Buyer Mobile · 搜尋結果 · SHOPLINE", bucket: "U:b-home", route: "/[locale]/search?q=托特包", viewport: "390x844", screenshotFile: "parity/26-921.png", deliveryPng: "after-26-921.png", plan: "assert" },
  { frameId: "26:1117", name: "Buyer Mobile · 聯絡商店 · SHOPLINE", bucket: "U:b-home", route: "/[locale]/pages/contact", viewport: "390x844", screenshotFile: "parity/26-1117.png", deliveryPng: "after-26-1117.png", plan: "DEFERRED", reason: "contact route removed from this PR (submitted PII by GET into the URL, fake sent state, no backend)", deferredTo: "#375 feat-contact-store" },
  { frameId: "29:641", name: "Buyer Desktop · 訊息已送出 · SHOPLINE", bucket: "U:b-home", route: "/[locale]/pages/contact?sent=true", viewport: "1440x900", screenshotFile: "parity/29-641.png", deliveryPng: "after-29-641.png", plan: "DEFERRED", reason: "contact route removed from this PR (submitted PII by GET into the URL, fake sent state, no backend)", deferredTo: "#375 feat-contact-store" },
  { frameId: "29:670", name: "Buyer Mobile · 訊息已送出 · SHOPLINE", bucket: "U:b-home", route: "/[locale]/pages/contact?sent=true", viewport: "390x844", screenshotFile: "parity/29-670.png", deliveryPng: "after-29-670.png", plan: "DEFERRED", reason: "contact route removed from this PR (submitted PII by GET into the URL, fake sent state, no backend)", deferredTo: "#375 feat-contact-store" },
  { frameId: "782:3906", name: "Buyer Desktop · 系列總覽", bucket: "U:b-home", route: "/[locale]/collections", viewport: "1440x900", screenshotFile: "parity/782-3906.png", deliveryPng: "verified-series-overview.png", plan: "assert" },
  { frameId: "782:4089", name: "Buyer Mobile · 系列總覽", bucket: "U:b-home", route: "/[locale]/collections", viewport: "390x844", screenshotFile: "parity/782-4089.png", deliveryPng: "verified-series-overview.png", plan: "assert" },
  { frameId: "1253:17135", name: "DESIGN · Buyer Desktop · 店鋪暫停營業 · 狀態", bucket: "U:b-home", route: "/[locale]", viewport: "1440x900", screenshotFile: "parity/1253-17135.png", deliveryPng: "after-1253-17135.png", plan: "NOT_COVERED", reason: "needs a paused-store fixture; the sandbox control endpoint has no such action" },
  { frameId: "1253:17159", name: "DESIGN · Buyer Mobile · 店鋪暫停營業 · 狀態", bucket: "U:b-home", route: "/[locale]", viewport: "390x844", screenshotFile: "parity/1253-17159.png", deliveryPng: "after-1253-17159.png", plan: "NOT_COVERED", reason: "needs a paused-store fixture; the sandbox control endpoint has no such action" },
  { frameId: "4:9441", name: "DESIGN · Buyer Desktop · 焦糖托特包 · SHOPLINE", bucket: "U:b-product", route: "/[locale]/products/caramel-tote", viewport: "1440x900", screenshotFile: "parity/4-9441.png", deliveryPng: "spacing-product-desktop.png", plan: "assert" },
  { frameId: "4:9532", name: "Buyer Mobile · 焦糖托特包 · touch-first", bucket: "U:b-product", route: "/[locale]/products/caramel-tote", viewport: "390x844", screenshotFile: "parity/4-9532.png", deliveryPng: "spacing-product-mobile.png", plan: "assert" },
  { frameId: "6:19772", name: "Buyer Desktop · 所有商品 · SHOPLINE", bucket: "U:b-product", route: "/[locale]/products", viewport: "1440x900", screenshotFile: "parity/6-19772.png", deliveryPng: "verified-desktop-catalog.png", plan: "assert" },
  { frameId: "6:19859", name: "Buyer Mobile · 所有商品 · SHOPLINE", bucket: "U:b-product", route: "/[locale]/products", viewport: "390x844", screenshotFile: "parity/6-19859.png", deliveryPng: "verified-catalog-mobile-top.png", plan: "assert" },
  { frameId: "26:676", name: "Buyer Desktop · 沒有符合的商品 · SHOPLINE", bucket: "U:b-product", route: "/[locale]/search?q=no_match_xyz", viewport: "1440x900", screenshotFile: "parity/26-676.png", deliveryPng: "after-26-676.png", plan: "assert" },
  { frameId: "26:968", name: "Buyer Mobile · 沒有符合的商品 · SHOPLINE", bucket: "U:b-product", route: "/[locale]/search?q=no_match_xyz", viewport: "390x844", screenshotFile: "parity/26-968.png", deliveryPng: "after-26-968.png", plan: "assert" },
  { frameId: "51:1380", name: "DESIGN · Buyer Mobile · 規格選擇 · bottom-sheet composition", bucket: "U:b-product", route: "/[locale]/products/caramel-tote", viewport: "390x844", screenshotFile: "parity/51-1380.png", deliveryPng: "after-51-1380.png", plan: "NOT_COVERED", reason: "product page has no spec bottom-sheet / details-and-shipping block matching this frame" },
  { frameId: "51:1381", name: "DESIGN · Buyer Mobile · 商品說明與配送", bucket: "U:b-product", route: "/[locale]/products/caramel-tote", viewport: "390x844", screenshotFile: "parity/51-1381.png", deliveryPng: "after-51-1381.png", plan: "NOT_COVERED", reason: "product page has no spec bottom-sheet / details-and-shipping block matching this frame" },
  { frameId: "66:1752", name: "Buyer Desktop · 部分商品可購買", bucket: "U:b-product", route: "/[locale]/products/caramel-tote", viewport: "1440x900", screenshotFile: "parity/66-1752.png", deliveryPng: "after-66-1752.png", plan: "NOT_COVERED", reason: "no partially-purchasable product state exists in the sandbox fixtures or the product UI" },
  { frameId: "66:1753", name: "Buyer Mobile · 部分商品可購買", bucket: "U:b-product", route: "/[locale]/products/caramel-tote", viewport: "390x844", screenshotFile: "parity/66-1753.png", deliveryPng: "after-66-1753.png", plan: "NOT_COVERED", reason: "no partially-purchasable product state exists in the sandbox fixtures or the product UI" },
  { frameId: "102:2163", name: "Buyer Desktop · 日常斜背小包", bucket: "U:b-product", route: "/[locale]/products/crossbody", viewport: "1440x900", screenshotFile: "parity/102-2163.png", deliveryPng: "after-102-2163.png", plan: "NOT_COVERED", reason: "frame shows a demo product; the sandbox fixtures have no such product" },
  { frameId: "102:2232", name: "Buyer Mobile · 日常斜背小包", bucket: "U:b-product", route: "/[locale]/products/crossbody", viewport: "390x844", screenshotFile: "parity/102-2232.png", deliveryPng: "after-102-2232.png", plan: "NOT_COVERED", reason: "frame shows a demo product; the sandbox fixtures have no such product" },
  { frameId: "104:2209", name: "Buyer Desktop · 輕量帆布托特包", bucket: "U:b-product", route: "/[locale]/products/canvas-tote", viewport: "1440x900", screenshotFile: "parity/104-2209.png", deliveryPng: "after-104-2209.png", plan: "NOT_COVERED", reason: "frame shows a demo product; the sandbox fixtures have no such product" },
  { frameId: "104:2278", name: "Buyer Mobile · 輕量帆布托特包", bucket: "U:b-product", route: "/[locale]/products/canvas-tote", viewport: "390x844", screenshotFile: "parity/104-2278.png", deliveryPng: "after-104-2278.png", plan: "NOT_COVERED", reason: "frame shows a demo product; the sandbox fixtures have no such product" },
  { frameId: "104:2553", name: "Buyer Desktop · 迷你收納袋", bucket: "U:b-product", route: "/[locale]/products/mini-pouch", viewport: "1440x900", screenshotFile: "parity/104-2553.png", deliveryPng: "after-104-2553.png", plan: "NOT_COVERED", reason: "frame shows a demo product; the sandbox fixtures have no such product" },
  { frameId: "104:2622", name: "Buyer Mobile · 迷你收納袋", bucket: "U:b-product", route: "/[locale]/products/mini-pouch", viewport: "390x844", screenshotFile: "parity/104-2622.png", deliveryPng: "after-104-2622.png", plan: "NOT_COVERED", reason: "frame shows a demo product; the sandbox fixtures have no such product" },
  { frameId: "104:3072", name: "Buyer Desktop · 簡約提袋", bucket: "U:b-product", route: "/[locale]/products/simple-tote", viewport: "1440x900", screenshotFile: "parity/104-3072.png", deliveryPng: "after-104-3072.png", plan: "NOT_COVERED", reason: "frame shows a demo product; the sandbox fixtures have no such product" },
  { frameId: "104:3141", name: "Buyer Mobile · 簡約提袋", bucket: "U:b-product", route: "/[locale]/products/simple-tote", viewport: "390x844", screenshotFile: "parity/104-3141.png", deliveryPng: "after-104-3141.png", plan: "NOT_COVERED", reason: "frame shows a demo product; the sandbox fixtures have no such product" },
  { frameId: "104:3416", name: "Buyer Desktop · 米白托特包", bucket: "U:b-product", route: "/[locale]/products/beige-tote", viewport: "1440x900", screenshotFile: "parity/104-3416.png", deliveryPng: "after-104-3416.png", plan: "NOT_COVERED", reason: "frame shows a demo product; the sandbox fixtures have no such product" },
  { frameId: "104:3485", name: "Buyer Mobile · 米白托特包", bucket: "U:b-product", route: "/[locale]/products/beige-tote", viewport: "390x844", screenshotFile: "parity/104-3485.png", deliveryPng: "after-104-3485.png", plan: "NOT_COVERED", reason: "frame shows a demo product; the sandbox fixtures have no such product" },
  { frameId: "104:3758", name: "Buyer Desktop · 輕便收納包", bucket: "U:b-product", route: "/[locale]/products/compact-pouch", viewport: "1440x900", screenshotFile: "parity/104-3758.png", deliveryPng: "after-104-3758.png", plan: "NOT_COVERED", reason: "frame shows a demo product; the sandbox fixtures have no such product" },
  { frameId: "104:3827", name: "Buyer Mobile · 輕便收納包", bucket: "U:b-product", route: "/[locale]/products/compact-pouch", viewport: "390x844", screenshotFile: "parity/104-3827.png", deliveryPng: "after-104-3827.png", plan: "NOT_COVERED", reason: "frame shows a demo product; the sandbox fixtures have no such product" },
  { frameId: "782:3907", name: "Buyer Desktop · 所有商品 · price_asc", bucket: "U:b-product", route: "/[locale]/products?sort=price_asc", viewport: "1440x900", screenshotFile: "parity/782-3907.png", deliveryPng: "verified-desktop-sort-asc.png", plan: "assert" },
  { frameId: "782:3967", name: "Buyer Desktop · 所有商品 · price_desc", bucket: "U:b-product", route: "/[locale]/products?sort=price_desc", viewport: "1440x900", screenshotFile: "parity/782-3967.png", deliveryPng: "verified-desktop-sort-desc.png", plan: "assert" },
  { frameId: "782:4027", name: "Buyer Desktop · 所有商品 · title", bucket: "U:b-product", route: "/[locale]/products?sort=title", viewport: "1440x900", screenshotFile: "parity/782-4027.png", deliveryPng: "verified-desktop-sort-title.png", plan: "assert" },
  { frameId: "782:4090", name: "Buyer Mobile · 所有商品 · price_asc", bucket: "U:b-product", route: "/[locale]/products?sort=price_asc", viewport: "390x844", screenshotFile: "parity/782-4090.png", deliveryPng: "verified-sort-ascending.png", plan: "assert" },
  { frameId: "782:4143", name: "Buyer Mobile · 所有商品 · price_desc", bucket: "U:b-product", route: "/[locale]/products?sort=price_desc", viewport: "390x844", screenshotFile: "parity/782-4143.png", deliveryPng: "verified-sort-ascending.png", plan: "assert" },
  { frameId: "782:4196", name: "Buyer Mobile · 所有商品 · title", bucket: "U:b-product", route: "/[locale]/products?sort=title", viewport: "390x844", screenshotFile: "parity/782-4196.png", deliveryPng: "verified-sort-menu.png", plan: "assert" },
  { frameId: "782:4249", name: "Buyer Mobile · 商品排序", bucket: "U:b-product", route: "/[locale]/products", viewport: "390x844", screenshotFile: "parity/782-4249.png", deliveryPng: "verified-sort-menu.png", plan: "assert" },
  { frameId: "1253:17090", name: "Buyer Desktop · 商品已下架或不存在 · 狀態", bucket: "U:b-product", route: "/[locale]/products/non-existent-product-sku", viewport: "1440x900", screenshotFile: "parity/1253-17090.png", deliveryPng: "after-1253-17090.png", plan: "assert" },
  { frameId: "1253:17114", name: "Buyer Mobile · 商品已下架或不存在 · 狀態", bucket: "U:b-product", route: "/[locale]/products/non-existent-product-sku", viewport: "390x844", screenshotFile: "parity/1253-17114.png", deliveryPng: "after-1253-17114.png", plan: "assert" },
  { frameId: "4:9668", name: "Buyer Desktop · 購物車 · SHOPLINE", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "1440x900", screenshotFile: "parity/4-9668.png", deliveryPng: "verified-desktop-cart-initial.png", plan: "assert" },
  { frameId: "4:9746", name: "Buyer Mobile · 購物車 · compact list and bottom checkout", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "390x844", screenshotFile: "parity/4-9746.png", deliveryPng: "mobile-cart-ready.png", plan: "assert" },
  { frameId: "66:1748", name: "Buyer Desktop · 已在購物車中", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "1440x900", screenshotFile: "parity/66-1748.png", deliveryPng: "after-66-1748.png", plan: "NOT_COVERED", reason: "no UI renders 'already in cart' / 'currently unavailable'; asserting the populated cart would not verify this frame" },
  { frameId: "66:1749", name: "Buyer Mobile · 已在購物車中", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "390x844", screenshotFile: "parity/66-1749.png", deliveryPng: "after-66-1749.png", plan: "NOT_COVERED", reason: "no UI renders 'already in cart' / 'currently unavailable'; asserting the populated cart would not verify this frame" },
  { frameId: "66:1750", name: "Buyer Desktop · 目前無法購買", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "1440x900", screenshotFile: "parity/66-1750.png", deliveryPng: "after-66-1750.png", plan: "NOT_COVERED", reason: "no UI renders 'already in cart' / 'currently unavailable'; asserting the populated cart would not verify this frame" },
  { frameId: "66:1751", name: "Buyer Mobile · 目前無法購買", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "390x844", screenshotFile: "parity/66-1751.png", deliveryPng: "after-66-1751.png", plan: "NOT_COVERED", reason: "no UI renders 'already in cart' / 'currently unavailable'; asserting the populated cart would not verify this frame" },
  { frameId: "102:2267", name: "Buyer Desktop · 購物車 · 日常斜背小包入口", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "1440x900", screenshotFile: "parity/102-2267.png", deliveryPng: "after-102-2267.png", plan: "NOT_COVERED", reason: "frame shows a demo product's cart entry; the sandbox fixtures have no such product" },
  { frameId: "102:2324", name: "Buyer Mobile · 購物車 · 日常斜背小包入口", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "390x844", screenshotFile: "parity/102-2324.png", deliveryPng: "after-102-2324.png", plan: "NOT_COVERED", reason: "frame shows a demo product's cart entry; the sandbox fixtures have no such product" },
  { frameId: "104:2313", name: "Buyer Desktop · 購物車 · 輕量帆布托特包入口", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "1440x900", screenshotFile: "parity/104-2313.png", deliveryPng: "after-104-2313.png", plan: "NOT_COVERED", reason: "frame shows a demo product's cart entry; the sandbox fixtures have no such product" },
  { frameId: "104:2371", name: "Buyer Mobile · 購物車 · 輕量帆布托特包入口", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "390x844", screenshotFile: "parity/104-2371.png", deliveryPng: "after-104-2371.png", plan: "NOT_COVERED", reason: "frame shows a demo product's cart entry; the sandbox fixtures have no such product" },
  { frameId: "104:2657", name: "Buyer Desktop · 購物車 · 迷你收納袋入口", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "1440x900", screenshotFile: "parity/104-2657.png", deliveryPng: "after-104-2657.png", plan: "NOT_COVERED", reason: "frame shows a demo product's cart entry; the sandbox fixtures have no such product" },
  { frameId: "104:2715", name: "Buyer Mobile · 購物車 · 迷你收納袋入口", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "390x844", screenshotFile: "parity/104-2715.png", deliveryPng: "after-104-2715.png", plan: "NOT_COVERED", reason: "frame shows a demo product's cart entry; the sandbox fixtures have no such product" },
  { frameId: "104:3176", name: "Buyer Desktop · 購物車 · 簡約提袋入口", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "1440x900", screenshotFile: "parity/104-3176.png", deliveryPng: "after-104-3176.png", plan: "NOT_COVERED", reason: "frame shows a demo product's cart entry; the sandbox fixtures have no such product" },
  { frameId: "104:3234", name: "Buyer Mobile · 購物車 · 簡約提袋入口", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "390x844", screenshotFile: "parity/104-3234.png", deliveryPng: "after-104-3234.png", plan: "NOT_COVERED", reason: "frame shows a demo product's cart entry; the sandbox fixtures have no such product" },
  { frameId: "104:3520", name: "Buyer Desktop · 購物車 · 米白托特包入口", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "1440x900", screenshotFile: "parity/104-3520.png", deliveryPng: "after-104-3520.png", plan: "NOT_COVERED", reason: "frame shows a demo product's cart entry; the sandbox fixtures have no such product" },
  { frameId: "104:3578", name: "Buyer Mobile · 購物車 · 米白托特包入口", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "390x844", screenshotFile: "parity/104-3578.png", deliveryPng: "after-104-3578.png", plan: "NOT_COVERED", reason: "frame shows a demo product's cart entry; the sandbox fixtures have no such product" },
  { frameId: "104:3862", name: "Buyer Desktop · 購物車 · 輕便收納包入口", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "1440x900", screenshotFile: "parity/104-3862.png", deliveryPng: "after-104-3862.png", plan: "NOT_COVERED", reason: "frame shows a demo product's cart entry; the sandbox fixtures have no such product" },
  { frameId: "104:3920", name: "Buyer Mobile · 購物車 · 輕便收納包入口", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "390x844", screenshotFile: "parity/104-3920.png", deliveryPng: "after-104-3920.png", plan: "NOT_COVERED", reason: "frame shows a demo product's cart entry; the sandbox fixtures have no such product" },
  { frameId: "105:2439", name: "Buyer Desktop · 購物車空白", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "1440x900", screenshotFile: "parity/105-2439.png", deliveryPng: "verified-empty.png", plan: "assert" },
  { frameId: "105:2468", name: "Buyer Mobile · 購物車空白", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "390x844", screenshotFile: "parity/105-2468.png", deliveryPng: "mobile-empty-final.png", plan: "assert" },
  { frameId: "782:3904", name: "Buyer Desktop · 結帳 · 共用購物車脈絡", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "1440x900", screenshotFile: "parity/782-3904.png", deliveryPng: "verified-desktop-checkout.png", plan: "NOT_COVERED", reason: "checkout-in-cart-context frame: no matching UI asserted by this spec" },
  { frameId: "782:4087", name: "Buyer Mobile · 結帳 · 共用購物車脈絡", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "390x844", screenshotFile: "parity/782-4087.png", deliveryPng: "verified-checkout-beige-bottom.png", plan: "NOT_COVERED", reason: "checkout-in-cart-context frame: no matching UI asserted by this spec" },
  { frameId: "6:21646", name: "DESIGN · Buyer Desktop · 直播回放 · SHOPLINE", bucket: "U:b-live", route: "none", viewport: "1440x900", screenshotFile: "none", deliveryPng: "after-6-21646.png", plan: "BLOCKED", reason: "live room: no route yet per FIGMA-INVENTORY row C7" },
  { frameId: "6:21711", name: "DESIGN · Buyer Mobile · 直播回放 · SHOPLINE", bucket: "U:b-live", route: "none", viewport: "390x844", screenshotFile: "none", deliveryPng: "after-6-21711.png", plan: "BLOCKED", reason: "live room: no route yet per FIGMA-INVENTORY row C7" },
  { frameId: "6:21771", name: "DESIGN · Buyer Desktop · 秋日選物直播 · SHOPLINE", bucket: "U:b-live", route: "none", viewport: "1440x900", screenshotFile: "none", deliveryPng: "after-6-21771.png", plan: "BLOCKED", reason: "live room: no route yet per FIGMA-INVENTORY row C7" },
  { frameId: "6:21825", name: "DESIGN · Buyer Mobile · 秋日選物直播 · SHOPLINE", bucket: "U:b-live", route: "none", viewport: "390x844", screenshotFile: "none", deliveryPng: "after-6-21825.png", plan: "BLOCKED", reason: "live room: no route yet per FIGMA-INVENTORY row C7" },
];

export async function checkBuyerBrowseParity({ browser, origin, evidence, facts }: ParityContext) {
  const parityDir = path.join(evidence, "parity");
  await mkdir(parityDir, { recursive: true });
  const candle = facts?.products?.find((x: any) => x.key === "candle");
  assert(candle, "buyer-browse parity needs the sandbox fixture product `candle` in facts.products");

  const ledger: LedgerEntry[] = [];
  const failures: string[] = [];
  const asserted = new Map<string, Set<string>>();

  for (const [locale, c] of Object.entries(copy)) {
    for (const width of [1440, 390]) {
      const height = width === 1440 ? 900 : 844;
      const isMobile = width === 390;
      const viewport = `${width}x${height}`;
      const f = (desktop: string, mobile: string) => (isMobile ? mobile : desktop);

      const context = await browser.newContext(ctxOpts({ ignoreHTTPSErrors: true, viewport: { width, height }, locale }));
      const page = await context.newPage();

      // One ledger row per operation that really ran; the callback returns what it observed.
      const step = async (frame: string, route: string, control: string, operation: string, expected: string, perform: () => Promise<string | void>) => {
        const row = { frame, locale, viewport, route, control, operation, expected, environment: "REAL-STACK (SANDBOX)" };
        try {
          const actual = await perform();
          ledger.push({ ...row, actual: actual || "assertions passed", status: "PASS" });
          if (!asserted.has(frame)) asserted.set(frame, new Set());
          asserted.get(frame)!.add(`${control}: ${expected}`);
        } catch (error: any) {
          failures.push(`[${frame} ${locale} ${viewport}] ${control}: ${error.message}`);
          ledger.push({ ...row, actual: `failed: ${error.message}`, status: "FAIL", error: error.message });
        }
      };
      // Parity sheet: one screenshot per covered frame (zh-TW run); a failed screenshot is a failed step, never swallowed.
      const shot = async (frame: string, route: string) => {
        if (locale !== "zh-TW") return;
        await step(frame, route, "parity-screenshot", "page.screenshot", `parity/${frame.replace(/:/g, "-")}.png written`, async () => {
          await page.screenshot({ path: path.join(parityDir, `${frame.replace(/:/g, "-")}.png`) });
        });
      };
      const settle = async () => {
        await page.waitForLoadState("networkidle");
      };
      const controls = () => page.getByTestId("list-controls");
      const urlParams = () => new URL(page.url()).searchParams;

      try {
        // ===== Empty cart (fresh context, nothing added yet): /[locale]/cart ==========================================================
        const emptyFrame = f("105:2439", "105:2468");
        const cartRoute = `/${locale}/cart`;
        await page.goto(`${origin}${cartRoute}`);
        await step(emptyFrame, cartRoute, "empty-cart panel", "goto + assert", `cart note "${c.cartEmptyHint}" and link "${c.browseProducts}" visible`, async () => {
          const panel = page.getByTestId("cart-empty").locator('section[data-variant="empty"]');
          await expect(panel).toContainText(c.cartEmptyHint, { timeout: T });
          await expect(panel.getByRole("link", { name: c.browseProducts, exact: true })).toBeVisible({ timeout: T });
        });
        await shot(emptyFrame, cartRoute);
        await step(emptyFrame, cartRoute, `link "${c.browseProducts}"`, "click", "click opens the product list", async () => {
          await page.getByTestId("cart-empty").locator('section[data-variant="empty"]').getByRole("link", { name: c.browseProducts, exact: true }).click();
          await expect(page).toHaveURL(new RegExp(`/${locale}/products/?$`), { timeout: T });
          return page.url();
        });

        // ===== Terms: /[locale]/legal/terms ===========================================================================================
        const termsFrame = f("5:15580", "5:15658");
        const termsRoute = `/${locale}/legal/terms`;
        await page.goto(`${origin}${termsRoute}`);
        await step(termsFrame, termsRoute, "terms heading", "goto + assert", `heading "${c.termsHeading}" visible`, async () => {
          await expect(page.getByRole("heading", { name: c.termsHeading })).toBeVisible({ timeout: T });
        });
        await shot(termsFrame, termsRoute);

        // ===== Collections overview: /[locale]/collections ============================================================================
        const overviewFrame = f("782:3906", "782:4089");
        const overviewRoute = `/${locale}/collections`;
        await page.goto(`${origin}${overviewRoute}`);
        await step(overviewFrame, overviewRoute, "collections heading", "goto + assert", `heading "${c.collectionsOverview}" visible`, async () => {
          await expect(page.getByRole("heading", { name: c.collectionsOverview })).toBeVisible({ timeout: T });
        });
        await shot(overviewFrame, overviewRoute);

        // ===== Search with results: type the fixture product's title into the search box and submit ===================================
        const searchFrame = f("26:624", "26:921");
        const searchRoute = `/${locale}/search`;
        await page.goto(`${origin}${searchRoute}`);
        let expectedSummary = "";
        await step(searchFrame, searchRoute, "search box + submit button", "fill + click", `query "${candle.title}" lands in the URL`, async () => {
          const main = page.locator("main");
          await main.getByRole("searchbox").fill(candle.title);
          await main.getByRole("button", { name: c.search, exact: true }).click();
          await page.waitForURL(/[?&]q=/, { timeout: T });
          assert.equal(urlParams().get("q"), candle.title);
          return page.url();
        });
        const summaryCheck = async () => {
          const cards = await page.getByTestId("product-card").count();
          assert(cards >= 1, `the fixture query must match at least the product itself, got ${cards} cards`);
          expectedSummary = fmt(c.searchResults, { q: candle.title, n: cards });
          await expect(page.getByText(expectedSummary, { exact: true }).filter({ visible: true })).toHaveCount(1, { timeout: T });
          return `${cards} cards; summary "${expectedSummary}"`;
        };
        await step(searchFrame, searchRoute, "result summary", "assert", "summary shows the real result count (= rendered cards) in this locale", summaryCheck);
        await shot(searchFrame, searchRoute);
        await step(searchFrame, searchRoute, "result summary after reload", "reload + assert", "same summary and cards after refresh", async () => {
          await page.reload();
          return summaryCheck();
        });

        await step(searchFrame, searchRoute, "cursor-page result summary", "goto + click + reload + assert", "last search page has product cards but no misleading total, including after refresh", async () => {
          const noJS = await browser.newContext(ctxOpts({ ignoreHTTPSErrors: true, javaScriptEnabled: false, viewport: { width, height }, locale }));
          try {
            const cursorPage = await noJS.newPage();
            // The existing fixture has 24 Everyday items plus other products matching "e": more than the default 24-card page.
            const q = "e";
            await cursorPage.goto(`${origin}${searchRoute}?q=${q}`);
            const nextLink = cursorPage.locator('noscript a[href*="after="]');
            await expect(nextLink).toHaveCount(1, { timeout: T });
            const nextHref = await nextLink.getAttribute("href");
            assert(nextHref, "the no-JS next-page link must expose the server-issued cursor");
            const nextURL = new URL(nextHref, cursorPage.url());
            assert(nextURL.searchParams.get("after"), "the real next-page URL must contain after");
            assert.equal(nextURL.searchParams.get("q"), q);
            // Exercise the real no-JS control; the following URL assertion also verifies its exact server-issued href.
            await nextLink.click();
            await expect(cursorPage).toHaveURL(nextURL.href, { timeout: T });
            const checkLastPage = async () => {
              await expect(cursorPage.getByTestId("product-card").first()).toBeVisible({ timeout: T });
              await expect(cursorPage.locator('noscript a[href*="after="]')).toHaveCount(0);
              const cards = await cursorPage.getByTestId("product-card").count();
              const falseTotal = fmt(c.searchResults, { q, n: cards });
              await expect(cursorPage.getByText(falseTotal, { exact: true })).toHaveCount(0, { timeout: T });
              return cards;
            };
            const cards = await checkLastPage();
            await cursorPage.reload();
            assert.equal(await checkLastPage(), cards, "last-page cards persist after refresh");
            return `${cards} last-page cards; no result count before or after refresh; real no-JS next href followed`;
          } finally {
            await noJS.close();
          }
        });

        // ===== Search without results ==================================================================================================
        const emptySearchFrame = f("26:676", "26:968");
        await page.goto(`${origin}${searchRoute}`);
        await step(emptySearchFrame, searchRoute, "search box + submit button", "fill + click", "no-match query lands in the URL", async () => {
          const main = page.locator("main");
          await main.getByRole("searchbox").fill("xyz_no_matching_product");
          await main.getByRole("button", { name: c.search, exact: true }).click();
          await page.waitForURL(/[?&]q=xyz_no_matching_product/, { timeout: T });
          return page.url();
        });
        await step(emptySearchFrame, searchRoute, "empty result state", "assert", `heading "${c.searchEmpty}" and hint "${c.searchEmptyDesc}" visible, no product cards`, async () => {
          await expect(page.getByRole("heading", { name: c.searchEmpty })).toBeVisible({ timeout: T });
          await expect(page.locator("main").getByText(c.searchEmptyDesc, { exact: true }).filter({ visible: true })).toHaveCount(1, { timeout: T });
          await expect(page.getByTestId("product-card")).toHaveCount(0);
        });
        await shot(emptySearchFrame, searchRoute);

        // ===== Product list: heading, real picker/select sort, price filter (fill + apply), persistence ====================================
        const listFrame = f("6:19772", "6:19859");
        const listRoute = `/${locale}/products`;
        const openList = async (query = "") => {
          await page.goto(`${origin}${listRoute}${query}`);
          await settle();
          await expect(controls()).toHaveCount(1, { timeout: T });
        };
        await openList();
        await step(listFrame, listRoute, "all-products heading", "goto + assert", `heading "${c.allProducts}" visible`, async () => {
          await expect(page.getByRole("heading", { name: c.allProducts })).toBeVisible({ timeout: T });
        });
        await shot(listFrame, listRoute);

        const sortBy = async (value: keyof typeof c.sortOptions) => {
          if (isMobile) {
            await controls().getByRole("button", { name: c.sortBy, exact: true }).click({ timeout: T });
            await page.getByRole("button", { name: c.sortOptions[value], exact: true }).click({ timeout: T });
          } else {
            await controls().locator('select[name="sort"]').selectOption(value);
          }
          await page.waitForURL(new RegExp(`[?&]sort=${value}`), { timeout: T });
          await settle();
          await expect(controls()).toHaveCount(1, { timeout: T });
          await expect(controls().locator('select[name="sort"]')).toHaveValue(value);
        };
        const ascFrame = f("782:3907", "782:4090");
        await step(ascFrame, `${listRoute}?sort=price_asc`, isMobile ? "mobile sort picker" : "sort select", isMobile ? "click entry + price_asc" : "selectOption price_asc", "URL carries sort=price_asc and card prices ascend", async () => {
          await sortBy("price_asc");
          const prices = await cardPrices(page);
          assert(prices.length >= 3 && ascending(prices), `not ascending: ${prices}`);
          return `prices ${prices.join(",")}`;
        });
        await shot(ascFrame, `${listRoute}?sort=price_asc`);
        const descFrame = f("782:3967", "782:4143");
        await step(descFrame, `${listRoute}?sort=price_desc`, isMobile ? "mobile sort picker" : "sort select", isMobile ? "click entry + price_desc" : "selectOption price_desc", "URL carries sort=price_desc and the first card's from-price is not below the last card's (sort key is the highest variant price, which the card does not show)", async () => {
          await sortBy("price_desc");
          const prices = await cardPrices(page);
          assert(trendsDown(prices), `desc list does not trend down: ${prices}`);
          return `prices ${prices.join(",")}`;
        });
        await shot(descFrame, `${listRoute}?sort=price_desc`);
        const titleFrame = f("782:4027", "782:4196");
        await step(titleFrame, `${listRoute}?sort=title`, isMobile ? "mobile sort picker" : "sort select", isMobile ? "click entry + title" : "selectOption title", "URL carries sort=title and the selection is kept (list order not asserted)", async () => {
          await sortBy("title");
          const titles = await page.getByTestId("product-card").locator(".sf-card__title").allTextContents();
          assert(titles.length >= 3, `expected a product list, got ${titles.length} cards`);
          return `${titles.length} cards`;
        });
        await shot(titleFrame, `${listRoute}?sort=title`);

        // Price filter: min/max must survive a later sort change and a reload.
        await openList();
        await step(listFrame, listRoute, "price filter", "click summary + fill min/max + click apply", "URL carries min=15&max=20 and every card price is within 15..20", async () => {
          await controls().locator("summary").click();
          await controls().getByLabel(c.priceMin, { exact: true }).fill("15");
          await controls().getByLabel(c.priceMax, { exact: true }).fill("20");
          await controls().getByRole("button", { name: c.apply, exact: true }).click();
          await page.waitForURL(/[?&]min=15/, { timeout: T });
          await settle();
          assert.equal(urlParams().get("min"), "15");
          assert.equal(urlParams().get("max"), "20");
          const prices = await cardPrices(page);
          assert(prices.length >= 3 && prices.every((n) => n >= 15 && n <= 20), `price filter leaked: ${prices}`);
          return `prices ${prices.join(",")}`;
        });
        await step(listFrame, listRoute, "sort control with active filter", isMobile ? "click entry + price_desc" : "selectOption price_desc", "min and max survive the sort change; prices stay in range and trend down", async () => {
          await expect(controls()).toHaveCount(1, { timeout: T });
          await sortBy("price_desc");
          assert.equal(urlParams().get("min"), "15");
          assert.equal(urlParams().get("max"), "20");
          await expect(controls().getByLabel(c.priceMin, { exact: true })).toHaveValue("15");
          await expect(controls().getByLabel(c.priceMax, { exact: true })).toHaveValue("20");
          const prices = await cardPrices(page);
          assert(trendsDown(prices) && prices.every((n) => n >= 15 && n <= 20), `filtered desc list wrong: ${prices}`);
          return `prices ${prices.join(",")}`;
        });
        await step(listFrame, listRoute, "filter + sort after reload", "reload + assert", "sort, min and max and the filtered list are still there after refresh", async () => {
          await page.reload();
          await settle();
          await expect(controls()).toHaveCount(1, { timeout: T });
          assert.equal(urlParams().get("sort"), "price_desc");
          assert.equal(urlParams().get("min"), "15");
          assert.equal(urlParams().get("max"), "20");
          await expect(controls().locator('select[name="sort"]')).toHaveValue("price_desc");
          await expect(controls().getByLabel(c.priceMin, { exact: true })).toHaveValue("15");
          await expect(controls().getByLabel(c.priceMax, { exact: true })).toHaveValue("20");
          const prices = await cardPrices(page);
          assert(trendsDown(prices) && prices.every((n) => n >= 15 && n <= 20), `reloaded list wrong: ${prices}`);
          return `prices ${prices.join(",")}`;
        });

        // ===== Product page: the signed primary action, then a real add to cart ======================================================
        const productFrame = f("4:9441", "4:9532");
        const productRoute = `/${locale}/products/${candle.slug}`;
        await page.goto(`${origin}${productRoute}`);
        await step(productFrame, productRoute, "add-to-cart + buy-now buttons", "goto + assert", `"${c.addToCart}" (merchant accent background) and "${c.buyNow}" visible and enabled`, async () => {
          const add = page.getByTestId("add-to-cart");
          await expect(add).toHaveText(c.addToCart, { timeout: T });
          await expect(add).toBeEnabled();
          // measurement: the shop has ONE merchant accent (shop.css header contract), so the primary button must equal this
          // page's resolved --accent. Reading the custom property and resolving it to rgb in-page only gets the expected value.
          const accentRgb = await add.evaluate((el) => {
            const probe = document.createElement("span");
            probe.style.color = getComputedStyle(el).getPropertyValue("--accent").trim();
            document.body.appendChild(probe);
            const rgb = getComputedStyle(probe).color;
            probe.remove();
            return rgb;
          });
          assert.notEqual(accentRgb, "", "page has no resolved --accent");
          await expect(add).toHaveCSS("background-color", accentRgb);
          // measurement: the fixture accent is LIGHT, so the label must follow the resolved --on-accent (dark), not a fixed white.
          // Reading the custom property and resolving it to rgb in-page only gets the expected value.
          const onAccentRgb = await add.evaluate((el) => {
            const probe = document.createElement("span");
            probe.style.color = getComputedStyle(el).getPropertyValue("--on-accent").trim();
            document.body.appendChild(probe);
            const rgb = getComputedStyle(probe).color;
            probe.remove();
            return rgb;
          });
          assert.notEqual(onAccentRgb, "rgb(255, 255, 255)", "fixture accent must be light so on-accent is not white");
          await expect(add).toHaveCSS("color", onAccentRgb);
          // real hover; the computed background reads below are measurements: hover must leave the resting accent and not be the fixed navy.
          await add.hover();
          await expect.poll(() => add.evaluate((el) => getComputedStyle(el).backgroundColor), { timeout: T }).not.toBe(accentRgb);
          const hoverBg = await add.evaluate((el) => getComputedStyle(el).backgroundColor);
          assert.notEqual(hoverBg, "rgb(20, 41, 66)", "primary hover is the fixed navy, not accent-derived");
          await page.mouse.move(0, 0);
          await expect(page.getByTestId("buy-now")).toHaveText(c.buyNow);
        });
        await shot(productFrame, productRoute);
        await step(productFrame, productRoute, "add-to-cart button", "click", "the cart drawer opens with its checkout action", async () => {
          await page.getByTestId("add-to-cart").click();
          await expect(page.getByTestId("cart-checkout")).toBeVisible({ timeout: T });
        });

        // ===== Cart with the added product: persists across reload ===================================================================
        const cartFrame = f("4:9668", "4:9746");
        await page.goto(`${origin}${cartRoute}`);
        const cartHasLine = async () => {
          await expect(page.getByRole("heading", { name: c.cartTitle, level: 1 })).toBeVisible({ timeout: T });
          const host = page.getByTestId("cart-page");
          await expect(host.getByTestId("cart-line")).toHaveCount(1, { timeout: T });
          await expect(host.getByTestId("cart-line-qty")).toHaveText("1");
        };
        await step(cartFrame, cartRoute, "cart page", "goto + assert", `heading "${c.cartTitle}" and the added line with quantity 1`, cartHasLine);
        await shot(cartFrame, cartRoute);
        await step(cartFrame, cartRoute, "cart page after reload", "reload + assert", "the line is still in the cart after refresh", async () => {
          await page.reload();
          await cartHasLine();
        });

        // ===== 782:4249: full mobile sorting page, reached by real controls, not a native select or bottom sheet =====================
        // Authority: signed 782:4249 PNG SHA256 19b7f3e19a2ee9889925f0dbeb8906b74dfbce649038be287b8ce96aa204bf56;
        // existing GET enums and price oracles above. No new API/fixture, and the populated cart above is the mutation baseline.
        if (isMobile) {
          const frame = "782:4249";
          const beforeCart = await page.getByTestId("cart-line").allTextContents();
          const beforeQty = await page.getByTestId("cart-line-qty").allTextContents();
          const writes: string[] = [];
          const observeWrite = (request: import("@playwright/test").Request) => {
            if (new URL(request.url()).origin === new URL(origin).origin && !["GET", "HEAD", "OPTIONS"].includes(request.method())) {
              // Record path only: preview credentials and any request body must never enter the ledger.
              writes.push(`${request.method()} ${new URL(request.url()).pathname}`);
            }
          };
          page.on("request", observeWrite);
          const entry = () => page.getByRole("button", { name: c.sortBy, exact: true });
          const option = (value: keyof typeof c.sortOptions) => page.getByRole("button", { name: c.sortOptions[value], exact: true });
          const cancel = () => page.getByRole("button", { name: c.cancelSort, exact: true });
          const openPicker = async () => {
            await entry().click({ timeout: T });
            await expect(page.getByRole("heading", { name: c.sortBy, exact: true })).toBeVisible({ timeout: T });
          };
          let pickerReady = false;
          try {
            await page.goto(`${origin}${listRoute}`);
            await settle();
            await step(frame, listRoute, "mobile sort contents", "click sort + assert full page", "merchant name, title, explanatory note, four sort buttons and cancel; no visible native sort", async () => {
              await openPicker(); // Baseline RED must be this absent UI control, not a fixture or order assumption.
              await expect(page.getByText(c.sortNote, { exact: true })).toBeVisible({ timeout: T });
              await expect(page.getByText(facts.store_name, { exact: true }).filter({ visible: true }).first()).toBeVisible();
              await expect(page.locator('select[name="sort"]').filter({ visible: true })).toHaveCount(0);
              for (const value of Object.keys(c.sortOptions) as (keyof typeof c.sortOptions)[]) await expect(option(value)).toBeVisible();
              await expect(cancel()).toBeVisible();
              // Read-only geometry: the full-width title/options start near the top, unlike a bottom sheet.
              const heading = await page.getByRole("heading", { name: c.sortBy, exact: true }).boundingBox();
              const first = await option("newest").boundingBox();
              assert(heading && heading.y < 135 && first && first.y < 220 && first.width >= 340, "sort must use the signed full-page composition at 390x844");
              await shot(frame, listRoute);
              await cancel().click({ timeout: T });
              await expect(entry()).toBeFocused({ timeout: T });
              await expect(page.locator('select[name="sort"]').filter({ visible: true })).toHaveCount(0);
              pickerReady = true;
            });
            // A missing entry records one causal FAIL per locale. Dependent actions cannot run; the manifest requires every row below.
            if (pickerReady) {
              const collection = facts.collections.find((x: any) => x.slug === "tableware");
              assert(collection, "existing storefront tableware fixture required");
              const scopes = [
                { key: "products", route: listRoute, query: "", products: facts.products.filter((x: any) => x.status === "active") },
                { key: "collection", route: `/${locale}/collections/${collection.slug}`, query: "", products: facts.products.filter((x: any) => x.status === "active" && x.collection === collection.key) },
                { key: "search", route: searchRoute, query: "Everyday", products: facts.products.filter((x: any) => x.status === "active" && x.title.includes("Everyday")) },
              ];
              for (const scope of scopes) {
                for (const value of Object.keys(c.sortOptions) as (keyof typeof c.sortOptions)[]) {
                  await step(frame, scope.route, `mobile sort ${scope.key} ${value}`, "click entry + click option + reload", "existing enum/order oracle and card order persist after refresh", async () => {
                    const params = new URLSearchParams();
                    if (scope.query) params.set("q", scope.query);
                    await page.goto(`${origin}${scope.route}?${params}`);
                    await settle();
                    await openPicker();
                    await option(value).click({ timeout: T });
                    await page.waitForURL((url) => url.pathname === scope.route && url.searchParams.get("sort") === value, { timeout: T });
                    await settle();
                    assert.equal(urlParams().get("q"), scope.query || null);
                    const titles = await page.getByTestId("product-card").locator(".sf-card__title").allTextContents();
                    const prices = await cardPrices(page);
                    assert(titles.length >= 3, "existing scope must contain at least three cards");
                    if (value === "price_asc") assert(ascending(prices), `not ascending: ${prices}`);
                    if (value === "price_desc") assert(trendsDown(prices), `desc list does not trend down: ${prices}`);
                    // The Go fixture creates products in facts order; SFR01 uses the same newest-first oracle.
                    if (value === "newest") assert.equal(titles[0], scope.products.at(-1).title);
                    // Keep the existing title-sort oracle (URL + populated list); do not invent a browser collation.
                    await page.reload();
                    await settle();
                    assert.equal(urlParams().get("sort"), value);
                    assert.equal(urlParams().get("q"), scope.query || null);
                    assert.deepEqual(await page.getByTestId("product-card").locator(".sf-card__title").allTextContents(), titles);
                    assert.deepEqual(await cardPrices(page), prices);
                    return `${value}; ${titles.length} cards; order retained after refresh`;
                  });
                }
                await step(frame, scope.route, `mobile sort ${scope.key} cancel and keyboard`, "keyboard activate + cancel; select with active filters", "cancel restores focus and exact URL; selection preserves query/filter/preview", async () => {
                  assert(facts.preview_token, "existing preview credential required");
                  const params = new URLSearchParams({ sort: "price_asc", min: "15", max: "20", preview: facts.preview_token });
                  if (scope.query) params.set("q", scope.query);
                  await page.goto(`${origin}${scope.route}?${params}`);
                  await settle();
                  const before = page.url();
                  await entry().focus();
                  await page.keyboard.press("Enter");
                  await expect(cancel()).toBeVisible({ timeout: T });
                  await cancel().press("Enter", { timeout: T });
                  assert.equal(page.url(), before, "cancel must not change any query parameter");
                  await expect(entry()).toBeFocused({ timeout: T });
                  await entry().press("Enter", { timeout: T });
                  await option("price_desc").press("Enter", { timeout: T });
                  await page.waitForURL((url) => url.searchParams.get("sort") === "price_desc", { timeout: T });
                  for (const [key, value] of params) if (key !== "sort") assert.equal(urlParams().get(key), value, `${key} must survive sorting`);
                  await page.reload();
                  for (const [key, value] of params) assert.equal(urlParams().get(key), key === "sort" ? "price_desc" : value, `${key} must survive refresh`);
                  return "cancel unchanged; keyboard focus returned; q/min/max/preview retained (credential omitted)";
                });
              }
              const collectionRoute = `/${locale}/collections/${collection.slug}`;
              const newestFirst = facts.products.filter((x: any) => x.status === "active" && x.collection === collection.key).at(-1).title;
              const manualFirst = facts.products.find((x: any) => x.key === "dripper").title;
              assert.notEqual(newestFirst, manualFirst, "fixture must distinguish newest from collection manual order");
              const checkClearedCollection = async (sort: string | null) => {
                assert.equal(new URL(page.url()).pathname, collectionRoute);
                assert.equal(urlParams().get("sort"), sort);
                assert.equal(urlParams().has("min"), false);
                assert.equal(urlParams().has("max"), false);
                assert(urlParams().get("preview") === facts.preview_token, "preview credential must survive Clear");
                const titles = await page.getByTestId("product-card").locator(".sf-card__title").allTextContents();
                assert.equal(titles.length, collection.active_products);
                assert.equal(titles[0], sort === "newest" ? newestFirst : manualFirst);
                await page.reload();
                await settle();
                assert.equal(urlParams().get("sort"), sort);
                assert.equal(urlParams().has("min"), false);
                assert.equal(urlParams().has("max"), false);
                assert(urlParams().get("preview") === facts.preview_token, "preview credential must survive reload");
                assert.deepEqual(await page.getByTestId("product-card").locator(".sf-card__title").allTextContents(), titles);
              };
              const openFilteredCollection = async (sort: boolean, min: string, max: string) => {
                if (!sort) {
                  await page.goto(`${origin}${collectionRoute}?${new URLSearchParams({ preview: facts.preview_token, min, max })}`);
                  await settle();
                  assert.equal(urlParams().get("sort"), null);
                  await expect(controls().getByRole("link", { name: c.clear, exact: true })).toBeVisible({ timeout: T });
                  return;
                }
                await page.goto(`${origin}${collectionRoute}?${new URLSearchParams({ preview: facts.preview_token })}`);
                await settle();
                await openPicker();
                await option("newest").click({ timeout: T });
                await page.waitForURL((url) => url.pathname === collectionRoute && url.searchParams.get("sort") === "newest", { timeout: T });
                await settle();
                await controls().locator("summary").click({ timeout: T });
                await controls().getByLabel(c.priceMin, { exact: true }).fill(min);
                await controls().getByLabel(c.priceMax, { exact: true }).fill(max);
                await controls().getByRole("button", { name: c.apply, exact: true }).click({ timeout: T });
                await page.waitForURL((url) => url.searchParams.get("min") === min && url.searchParams.get("max") === max, { timeout: T });
                await settle();
                assert.equal(urlParams().get("sort"), "newest");
                assert(urlParams().get("preview") === facts.preview_token, "preview credential must survive filter apply");
              };
              await step(frame, collectionRoute, "mobile collection newest nonempty clear", "click newest + fill price + apply + click Clear + reload", "Clear removes price bounds, keeps explicit newest and preview, and restores newest-first cards", async () => {
                await openFilteredCollection(true, "15", "20");
                await expect(page.getByTestId("product-card").first()).toBeVisible({ timeout: T });
                await controls().getByRole("link", { name: c.clear, exact: true }).click({ timeout: T });
                await settle();
                await checkClearedCollection("newest");
                return "explicit newest, preview and newest-first card retained after clear and reload";
              });
              await step(frame, collectionRoute, "mobile collection newest empty clear", "click newest + fill nonmatching price + apply + click empty-state Clear + reload", "Empty-state Clear removes price bounds, keeps explicit newest and preview, and restores newest-first cards", async () => {
                await openFilteredCollection(true, "99999", "100000");
                await expect(page.getByTestId("list-empty")).toBeVisible({ timeout: T });
                await expect(page.getByTestId("product-card")).toHaveCount(0);
                await page.getByTestId("list-empty").getByRole("link", { name: c.clear, exact: true }).click({ timeout: T });
                await settle();
                await checkClearedCollection("newest");
                return "empty-state clear retained explicit newest and preview after reload";
              });
              await step(frame, collectionRoute, "mobile collection absent sort clear", "open filtered collection without sort + click Clear + reload", "Clear leaves absent sort absent and restores the collection's manual first card", async () => {
                await openFilteredCollection(false, "15", "20");
                await controls().getByRole("link", { name: c.clear, exact: true }).click({ timeout: T });
                await settle();
                await checkClearedCollection(null);
                return "absent sort stayed absent; manual first card and preview retained after reload";
              });
              await step(frame, searchRoute, "mobile search query clear", "open filtered search + click Clear + reload", "Clear keeps search q, explicit newest and preview while removing price bounds", async () => {
                const q = "Everyday";
                const params = new URLSearchParams({ q, sort: "newest", min: "15", max: "20", preview: facts.preview_token });
                await page.goto(`${origin}${searchRoute}?${params}`);
                await settle();
                await expect(page.getByTestId("product-card").first()).toBeVisible({ timeout: T });
                await controls().getByRole("link", { name: c.clear, exact: true }).click({ timeout: T });
                await settle();
                const check = async () => {
                  assert.equal(new URL(page.url()).pathname, searchRoute);
                  assert.equal(urlParams().get("q"), q);
                  assert.equal(urlParams().get("sort"), "newest");
                  assert.equal(urlParams().has("min"), false);
                  assert.equal(urlParams().has("max"), false);
                  assert(urlParams().get("preview") === facts.preview_token, "preview credential must survive search Clear");
                  const titles = await page.getByTestId("product-card").locator(".sf-card__title").allTextContents();
                  assert.equal(titles.length, 24);
                  assert.equal(titles[0], "Everyday Item 24");
                  return titles;
                };
                const titles = await check();
                await page.reload();
                await settle();
                assert.deepEqual(await check(), titles);
                return "search q, explicit newest and preview retained after clear and reload";
              });
              await step(frame, cartRoute, "mobile sort cart unchanged", "return to cart + reload", "existing cart line and quantity unchanged; sorting issued no writes", async () => {
                assert.deepEqual(writes, [], "sorting must issue no write request");
                await page.goto(`${origin}${cartRoute}`);
                await cartHasLine();
                assert.deepEqual(await page.getByTestId("cart-line").allTextContents(), beforeCart);
                assert.deepEqual(await page.getByTestId("cart-line-qty").allTextContents(), beforeQty);
                await page.reload();
                await cartHasLine();
                assert.deepEqual(await page.getByTestId("cart-line").allTextContents(), beforeCart);
                return "existing line/quantity unchanged before and after refresh; zero sort writes";
              });
            }
          } finally {
            page.off("request", observeWrite);
          }
        }

        // ===== Removed / unknown product ===============================================================================================
        const removedFrame = f("1253:17090", "1253:17114");
        const removedRoute = `/${locale}/products/non-existent-product-sku`;
        await page.goto(`${origin}${removedRoute}`);
        await step(removedFrame, removedRoute, "not-found heading", "goto + assert", `heading "${c.productRemoved}" visible`, async () => {
          await expect(page.getByRole("heading", { name: c.productRemoved })).toBeVisible({ timeout: T });
        });
        await shot(removedFrame, removedRoute);
      } finally {
        await context.close();
      }
    }
  }

  // Manifest: status comes only from the ledger. COVERED = every planned assertion passed in all three locales, none failed.
  const manifest: ManifestEntry[] = PARITY_FRAMES.map((frame) => {
    if (frame.plan !== "assert") return { ...frame, status: frame.plan };
    const rows = ledger.filter((row) => row.frame === frame.frameId);
    const locales = new Set(rows.filter((row) => row.status === "PASS").map((row) => row.locale));
    const failedRows = rows.filter((row) => row.status === "FAIL");
    if (frame.frameId === "782:4249") {
      const required = ["mobile sort contents", "mobile sort cart unchanged", "mobile collection newest nonempty clear", "mobile collection newest empty clear", "mobile collection absent sort clear", "mobile search query clear", ...["products", "collection", "search"].flatMap((scope) => [
        ...["newest", "price_asc", "price_desc", "title"].map((sort) => `mobile sort ${scope} ${sort}`),
        `mobile sort ${scope} cancel and keyboard`,
      ])];
      const complete = Object.keys(copy).every((locale) => required.every((control) => rows.some((row) => row.locale === locale && row.control === control && row.status === "PASS")));
      if (!complete) return { ...frame, status: "NOT_COVERED", reason: failedRows.length ? `assertion failed: ${failedRows[0].error}` : "full mobile sort plan not executed in all three locales" };
    }
    if (failedRows.length === 0 && locales.size === Object.keys(copy).length) return { ...frame, status: "COVERED", asserted: [...(asserted.get(frame.frameId) ?? [])] };
    return { ...frame, status: "NOT_COVERED", reason: failedRows.length ? `assertion failed: ${failedRows[0].error}` : `no passing assertion in: ${Object.keys(copy).filter((l) => !locales.has(l)).join(", ")}` };
  });
  await writeFile(path.join(evidence, "parity-manifest.json"), JSON.stringify(manifest, null, 2));
  await writeFile(path.join(evidence, "parity-buyer-browse-click-ledger.json"), JSON.stringify(ledger, null, 2));

  const count = (status: ManifestEntry["status"]) => manifest.filter((entry) => entry.status === status).length;
  const summary = `COVERED ${count("COVERED")} · NOT_COVERED ${count("NOT_COVERED")} · DEFERRED ${count("DEFERRED")} · BLOCKED ${count("BLOCKED")} of ${manifest.length} frames; ${ledger.filter((row) => row.status === "PASS").length}/${ledger.length} ledger operations passed`;
  assert.deepEqual(failures, [], `PARITY-BUYER-BROWSE: ${failures.length} assertion(s) failed (${summary})`);
  console.log(`PARITY-BUYER-BROWSE: ${summary}`);
}
