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
import { shopCopy } from "../../apps/storefront/lib/shop-copy.ts";

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
// Browse links must reach their destination heading and, for product results, a visible first-page card.
export async function waitForHomeDestination(page: Page, url: RegExp, heading: string, requireCard = true, timeout = T) {
  await page.waitForURL(url, { timeout });
  await expect(page).toHaveURL(url, { timeout });
  await expect(page.getByRole("heading", { level: 1, name: heading, exact: true })).toBeVisible({ timeout });
  if (requireCard) await expect(page.getByTestId("product-card").first()).toBeVisible({ timeout });
}
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
    chooseSpec: "選擇規格",
    specBack: "返回商品",
    descriptionEntry: "商品說明與配送",
    descriptionTitle: "商品說明",
    quantity: "數量",
    decrease: "減少數量",
    increase: "增加數量",
    specLimit: "每次購買數量依商品庫存限制。",
    specPickup: "門市取貨：結帳時選擇超商品牌，填寫門市號碼（店名、地址可不填）。",
    welcome: "歡迎來到{name}",
    searchLabel: "搜尋商品",
    searchHint: "搜尋商品名稱或關鍵字",
    homeAllProducts: "全部商品",
    allCollections: "所有系列",
    viewAllProducts: "查看所有商品",
    collectionCount: "系列商品 · {n} 件",
    legacyViewAll: "查看全部",
    viewCart: "查看購物車",
    checkout: "前往結帳",
    entryBack: "商品詳情",
    entryTitle: "購物車 · {title}",
    entrySpec: "規格",
    entrySubtotal: "商品小計",
    entryRemove: "移除商品",
    entryShip: "運費依配送方式於結帳時計算。",
    standard: "標準款",
    buyNote: "商品規格與金額以本頁選擇為準；收件資料在結帳時填寫。",
    delivery: "配送方式",
    deliveryMethods: "可用方式依收貨地點而定，請以結帳時確認的選項為準。",
    deliveryFee: "運費於結帳顯示，最終金額以訂單確認為準。",
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
    chooseSpec: "选择规格",
    specBack: "返回商品",
    descriptionEntry: "商品说明与配送",
    descriptionTitle: "商品说明",
    quantity: "数量",
    decrease: "减少数量",
    increase: "增加数量",
    specLimit: "每次购买数量依商品库存限制。",
    specPickup: "门店取货：结账时选择便利店品牌，填写门店号码（店名、地址可不填）。",
    welcome: "欢迎来到{name}",
    searchLabel: "搜索商品",
    searchHint: "搜索商品名称或关键词",
    homeAllProducts: "全部商品",
    allCollections: "所有系列",
    viewAllProducts: "查看所有商品",
    collectionCount: "系列商品 · {n} 件",
    legacyViewAll: "查看全部",
    viewCart: "查看购物车",
    checkout: "前往结账",
    entryBack: "商品详情",
    entryTitle: "购物车 · {title}",
    entrySpec: "规格",
    entrySubtotal: "商品小计",
    entryRemove: "移除商品",
    entryShip: "运费依配送方式于结账时计算。",
    standard: "标准款",
    buyNote: "商品规格与金额以本页选择为准；收件资料在结账时填写。",
    delivery: "配送方式",
    deliveryMethods: "可用方式依收货地点而定，请以结账时确认的选项为准。",
    deliveryFee: "运费于结账显示，最终金额以订单确认为准。",
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
    chooseSpec: "Choose options",
    specBack: "Back to product",
    descriptionEntry: "Product description & delivery",
    descriptionTitle: "Description",
    quantity: "Quantity",
    decrease: "Decrease quantity",
    increase: "Increase quantity",
    specLimit: "Purchase quantity is limited by product stock.",
    specPickup: "For convenience-store pickup, choose a chain at checkout and enter the store number (name and address are optional).",
    welcome: "Welcome to {name}",
    searchLabel: "Search products",
    searchHint: "Search by product name or keyword",
    homeAllProducts: "All products",
    allCollections: "All collections",
    viewAllProducts: "View all products",
    collectionCount: "Products in this collection · {n}",
    legacyViewAll: "View all",
    viewCart: "View cart",
    checkout: "Checkout",
    entryBack: "Product details",
    entryTitle: "Cart · {title}",
    entrySpec: "Option",
    entrySubtotal: "Item subtotal",
    entryRemove: "Remove item",
    entryShip: "Shipping depends on the delivery method and is calculated at checkout.",
    standard: "Standard",
    buyNote: "Options and amount are as selected on this page; delivery details are entered at checkout.",
    delivery: "Delivery",
    deliveryMethods: "Available methods depend on your destination and are confirmed at checkout.",
    deliveryFee: "Shipping is shown at checkout; the order confirmation has the final amount.",
  },
};

// Complete inventory of the 73 Appendix A frames. `plan: "assert"` frames are only COVERED after their assertions pass.
export const PARITY_FRAMES: FrameSpec[] = [
  { frameId: "4:9192", name: "Buyer Desktop · 歡迎來到大夢甄選 · SHOPLINE", bucket: "U:b-home", route: "/[locale]", viewport: "1440x900", screenshotFile: "parity/4-9192.png", deliveryPng: "after-4-9192.png", plan: "assert" },
  { frameId: "4:9296", name: "Buyer Mobile · 歡迎來到大夢甄選 · SHOPLINE", bucket: "U:b-home", route: "/[locale]", viewport: "390x844", screenshotFile: "parity/4-9296.png", deliveryPng: "after-4-9296.png", plan: "assert" },
  { frameId: "5:15580", name: "Buyer Desktop · 購物條款 · SHOPLINE", bucket: "U:b-home", route: "/[locale]/legal/terms", viewport: "1440x900", screenshotFile: "parity/5-15580.png", deliveryPng: "after-5-15580.png", plan: "assert" },
  { frameId: "5:15658", name: "Buyer Mobile · 購物條款 · SHOPLINE", bucket: "U:b-home", route: "/[locale]/legal/terms", viewport: "390x844", screenshotFile: "parity/5-15658.png", deliveryPng: "after-5-15658.png", plan: "assert" },
  { frameId: "6:19933", name: "Buyer Desktop · 日常隨行系列 · SHOPLINE", bucket: "U:b-home", route: "/[locale]/collections/daily", viewport: "1440x900", screenshotFile: "parity/6-19933.png", deliveryPng: "verified-daily-series.png", plan: "assert" },
  { frameId: "6:19984", name: "Buyer Mobile · 日常隨行系列 · SHOPLINE", bucket: "U:b-home", route: "/[locale]/collections/daily", viewport: "390x844", screenshotFile: "parity/6-19984.png", deliveryPng: "verified-daily-series.png", plan: "assert" },
  { frameId: "26:624", name: "Buyer Desktop · 搜尋結果 · SHOPLINE", bucket: "U:b-home", route: "/[locale]/search?q=托特包", viewport: "1440x900", screenshotFile: "parity/26-624.png", deliveryPng: "after-26-624.png", plan: "assert" },
  { frameId: "26:850", name: "Buyer Desktop · 聯絡商店 · SHOPLINE", bucket: "U:b-home", route: "/[locale]/pages/contact", viewport: "1440x900", screenshotFile: "parity/26-850.png", deliveryPng: "after-26-850.png", plan: "DEFERRED", reason: "#375 merged at 2faa925d0 (POST /api/buyer/contact-messages); the storefront contact form UI is planned as a later #440 round", deferredTo: "#440 contact round" },
  { frameId: "26:921", name: "Buyer Mobile · 搜尋結果 · SHOPLINE", bucket: "U:b-home", route: "/[locale]/search?q=托特包", viewport: "390x844", screenshotFile: "parity/26-921.png", deliveryPng: "after-26-921.png", plan: "assert" },
  { frameId: "26:1117", name: "Buyer Mobile · 聯絡商店 · SHOPLINE", bucket: "U:b-home", route: "/[locale]/pages/contact", viewport: "390x844", screenshotFile: "parity/26-1117.png", deliveryPng: "after-26-1117.png", plan: "DEFERRED", reason: "#375 merged at 2faa925d0 (POST /api/buyer/contact-messages); the storefront contact form UI is planned as a later #440 round", deferredTo: "#440 contact round" },
  { frameId: "29:641", name: "Buyer Desktop · 訊息已送出 · SHOPLINE", bucket: "U:b-home", route: "/[locale]/pages/contact?sent=true", viewport: "1440x900", screenshotFile: "parity/29-641.png", deliveryPng: "after-29-641.png", plan: "DEFERRED", reason: "#375 merged at 2faa925d0 (POST /api/buyer/contact-messages); the storefront contact form UI is planned as a later #440 round", deferredTo: "#440 contact round" },
  { frameId: "29:670", name: "Buyer Mobile · 訊息已送出 · SHOPLINE", bucket: "U:b-home", route: "/[locale]/pages/contact?sent=true", viewport: "390x844", screenshotFile: "parity/29-670.png", deliveryPng: "after-29-670.png", plan: "DEFERRED", reason: "#375 merged at 2faa925d0 (POST /api/buyer/contact-messages); the storefront contact form UI is planned as a later #440 round", deferredTo: "#440 contact round" },
  { frameId: "782:3906", name: "Buyer Desktop · 系列總覽", bucket: "U:b-home", route: "/[locale]/collections", viewport: "1440x900", screenshotFile: "parity/782-3906.png", deliveryPng: "verified-series-overview.png", plan: "assert" },
  { frameId: "782:4089", name: "Buyer Mobile · 系列總覽", bucket: "U:b-home", route: "/[locale]/collections", viewport: "390x844", screenshotFile: "parity/782-4089.png", deliveryPng: "verified-series-overview.png", plan: "assert" },
  { frameId: "1253:17135", name: "DESIGN · Buyer Desktop · 店鋪暫停營業 · 狀態", bucket: "U:b-home", route: "/[locale]", viewport: "1440x900", screenshotFile: "parity/1253-17135.png", deliveryPng: "after-1253-17135.png", plan: "DEFERRED", reason: "integrator ruling 2026-11-04: a paused-by-owner state is not distinguishable from any other closed host (one 404); keep the current closed copy, no fabricated paused state", deferredTo: "#545" },
  { frameId: "1253:17159", name: "DESIGN · Buyer Mobile · 店鋪暫停營業 · 狀態", bucket: "U:b-home", route: "/[locale]", viewport: "390x844", screenshotFile: "parity/1253-17159.png", deliveryPng: "after-1253-17159.png", plan: "DEFERRED", reason: "integrator ruling 2026-11-04: a paused-by-owner state is not distinguishable from any other closed host (one 404); keep the current closed copy, no fabricated paused state", deferredTo: "#545" },
  { frameId: "4:9441", name: "DESIGN · Buyer Desktop · 焦糖托特包 · SHOPLINE", bucket: "U:b-product", route: "/[locale]/products/caramel-tote", viewport: "1440x900", screenshotFile: "parity/4-9441.png", deliveryPng: "spacing-product-desktop.png", plan: "assert" },
  { frameId: "4:9532", name: "Buyer Mobile · 焦糖托特包 · touch-first", bucket: "U:b-product", route: "/[locale]/products/caramel-tote", viewport: "390x844", screenshotFile: "parity/4-9532.png", deliveryPng: "spacing-product-mobile.png", plan: "assert" },
  { frameId: "6:19772", name: "Buyer Desktop · 所有商品 · SHOPLINE", bucket: "U:b-product", route: "/[locale]/products", viewport: "1440x900", screenshotFile: "parity/6-19772.png", deliveryPng: "verified-desktop-catalog.png", plan: "assert" },
  { frameId: "6:19859", name: "Buyer Mobile · 所有商品 · SHOPLINE", bucket: "U:b-product", route: "/[locale]/products", viewport: "390x844", screenshotFile: "parity/6-19859.png", deliveryPng: "verified-catalog-mobile-top.png", plan: "assert" },
  { frameId: "26:676", name: "Buyer Desktop · 沒有符合的商品 · SHOPLINE", bucket: "U:b-product", route: "/[locale]/search?q=no_match_xyz", viewport: "1440x900", screenshotFile: "parity/26-676.png", deliveryPng: "after-26-676.png", plan: "assert" },
  { frameId: "26:968", name: "Buyer Mobile · 沒有符合的商品 · SHOPLINE", bucket: "U:b-product", route: "/[locale]/search?q=no_match_xyz", viewport: "390x844", screenshotFile: "parity/26-968.png", deliveryPng: "after-26-968.png", plan: "assert" },
  { frameId: "51:1380", name: "DESIGN · Buyer Mobile · 規格選擇 · bottom-sheet composition", bucket: "U:b-product", route: "/[locale]/products/caramel-tote", viewport: "390x844", screenshotFile: "parity/51-1380.png", deliveryPng: "after-51-1380.png", plan: "assert" },
  { frameId: "51:1381", name: "DESIGN · Buyer Mobile · 商品說明與配送", bucket: "U:b-product", route: "/[locale]/products/caramel-tote", viewport: "390x844", screenshotFile: "parity/51-1381.png", deliveryPng: "after-51-1381.png", plan: "assert" },
  { frameId: "66:1752", name: "Buyer Desktop · 部分商品可購買", bucket: "U:b-product", route: "/[locale]/products/caramel-tote", viewport: "1440x900", screenshotFile: "parity/66-1752.png", deliveryPng: "after-66-1752.png", plan: "DEFERRED", reason: "C8 /claim ownership, integrator transfer", deferredTo: "later claim-parity unit (C8)" },
  { frameId: "66:1753", name: "Buyer Mobile · 部分商品可購買", bucket: "U:b-product", route: "/[locale]/products/caramel-tote", viewport: "390x844", screenshotFile: "parity/66-1753.png", deliveryPng: "after-66-1753.png", plan: "DEFERRED", reason: "C8 /claim ownership, integrator transfer", deferredTo: "later claim-parity unit (C8)" },
  { frameId: "102:2163", name: "Buyer Desktop · 日常斜背小包", bucket: "U:b-product", route: "/[locale]/products/ceramic-dripper-set", viewport: "1440x900", screenshotFile: "parity/102-2163.png", deliveryPng: "after-102-2163.png", plan: "assert" },
  { frameId: "102:2232", name: "Buyer Mobile · 日常斜背小包", bucket: "U:b-product", route: "/[locale]/products/ceramic-dripper-set", viewport: "390x844", screenshotFile: "parity/102-2232.png", deliveryPng: "after-102-2232.png", plan: "assert" },
  { frameId: "104:2209", name: "Buyer Desktop · 輕量帆布托特包", bucket: "U:b-product", route: "/[locale]/products/item-01", viewport: "1440x900", screenshotFile: "parity/104-2209.png", deliveryPng: "after-104-2209.png", plan: "assert" },
  { frameId: "104:2278", name: "Buyer Mobile · 輕量帆布托特包", bucket: "U:b-product", route: "/[locale]/products/item-01", viewport: "390x844", screenshotFile: "parity/104-2278.png", deliveryPng: "after-104-2278.png", plan: "assert" },
  { frameId: "104:2553", name: "Buyer Desktop · 迷你收納袋", bucket: "U:b-product", route: "/[locale]/products/item-02", viewport: "1440x900", screenshotFile: "parity/104-2553.png", deliveryPng: "after-104-2553.png", plan: "assert" },
  { frameId: "104:2622", name: "Buyer Mobile · 迷你收納袋", bucket: "U:b-product", route: "/[locale]/products/item-02", viewport: "390x844", screenshotFile: "parity/104-2622.png", deliveryPng: "after-104-2622.png", plan: "assert" },
  { frameId: "104:3072", name: "Buyer Desktop · 簡約提袋", bucket: "U:b-product", route: "/[locale]/products/item-03", viewport: "1440x900", screenshotFile: "parity/104-3072.png", deliveryPng: "after-104-3072.png", plan: "assert" },
  { frameId: "104:3141", name: "Buyer Mobile · 簡約提袋", bucket: "U:b-product", route: "/[locale]/products/item-03", viewport: "390x844", screenshotFile: "parity/104-3141.png", deliveryPng: "after-104-3141.png", plan: "assert" },
  { frameId: "104:3416", name: "Buyer Desktop · 米白托特包", bucket: "U:b-product", route: "/[locale]/products/wool-knit-scarf", viewport: "1440x900", screenshotFile: "parity/104-3416.png", deliveryPng: "after-104-3416.png", plan: "assert" },
  { frameId: "104:3485", name: "Buyer Mobile · 米白托特包", bucket: "U:b-product", route: "/[locale]/products/wool-knit-scarf", viewport: "390x844", screenshotFile: "parity/104-3485.png", deliveryPng: "after-104-3485.png", plan: "assert" },
  { frameId: "104:3758", name: "Buyer Desktop · 輕便收納包", bucket: "U:b-product", route: "/[locale]/products/item-04", viewport: "1440x900", screenshotFile: "parity/104-3758.png", deliveryPng: "after-104-3758.png", plan: "assert" },
  { frameId: "104:3827", name: "Buyer Mobile · 輕便收納包", bucket: "U:b-product", route: "/[locale]/products/item-04", viewport: "390x844", screenshotFile: "parity/104-3827.png", deliveryPng: "after-104-3827.png", plan: "assert" },
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
  { frameId: "66:1748", name: "Buyer Desktop · 已在購物車中", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "1440x900", screenshotFile: "parity/66-1748.png", deliveryPng: "after-66-1748.png", plan: "DEFERRED", reason: "C8 /claim ownership, integrator transfer", deferredTo: "later claim-parity unit (C8)" },
  { frameId: "66:1749", name: "Buyer Mobile · 已在購物車中", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "390x844", screenshotFile: "parity/66-1749.png", deliveryPng: "after-66-1749.png", plan: "DEFERRED", reason: "C8 /claim ownership, integrator transfer", deferredTo: "later claim-parity unit (C8)" },
  { frameId: "66:1750", name: "Buyer Desktop · 目前無法購買", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "1440x900", screenshotFile: "parity/66-1750.png", deliveryPng: "after-66-1750.png", plan: "DEFERRED", reason: "C8 /claim ownership, integrator transfer", deferredTo: "later claim-parity unit (C8)" },
  { frameId: "66:1751", name: "Buyer Mobile · 目前無法購買", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "390x844", screenshotFile: "parity/66-1751.png", deliveryPng: "after-66-1751.png", plan: "DEFERRED", reason: "C8 /claim ownership, integrator transfer", deferredTo: "later claim-parity unit (C8)" },
  { frameId: "102:2267", name: "Buyer Desktop · 購物車 · 日常斜背小包入口", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "1440x900", screenshotFile: "parity/102-2267.png", deliveryPng: "after-102-2267.png", plan: "assert" },
  { frameId: "102:2324", name: "Buyer Mobile · 購物車 · 日常斜背小包入口", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "390x844", screenshotFile: "parity/102-2324.png", deliveryPng: "after-102-2324.png", plan: "assert" },
  { frameId: "104:2313", name: "Buyer Desktop · 購物車 · 輕量帆布托特包入口", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "1440x900", screenshotFile: "parity/104-2313.png", deliveryPng: "after-104-2313.png", plan: "assert" },
  { frameId: "104:2371", name: "Buyer Mobile · 購物車 · 輕量帆布托特包入口", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "390x844", screenshotFile: "parity/104-2371.png", deliveryPng: "after-104-2371.png", plan: "assert" },
  { frameId: "104:2657", name: "Buyer Desktop · 購物車 · 迷你收納袋入口", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "1440x900", screenshotFile: "parity/104-2657.png", deliveryPng: "after-104-2657.png", plan: "assert" },
  { frameId: "104:2715", name: "Buyer Mobile · 購物車 · 迷你收納袋入口", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "390x844", screenshotFile: "parity/104-2715.png", deliveryPng: "after-104-2715.png", plan: "assert" },
  { frameId: "104:3176", name: "Buyer Desktop · 購物車 · 簡約提袋入口", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "1440x900", screenshotFile: "parity/104-3176.png", deliveryPng: "after-104-3176.png", plan: "assert" },
  { frameId: "104:3234", name: "Buyer Mobile · 購物車 · 簡約提袋入口", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "390x844", screenshotFile: "parity/104-3234.png", deliveryPng: "after-104-3234.png", plan: "assert" },
  { frameId: "104:3520", name: "Buyer Desktop · 購物車 · 米白托特包入口", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "1440x900", screenshotFile: "parity/104-3520.png", deliveryPng: "after-104-3520.png", plan: "assert" },
  { frameId: "104:3578", name: "Buyer Mobile · 購物車 · 米白托特包入口", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "390x844", screenshotFile: "parity/104-3578.png", deliveryPng: "after-104-3578.png", plan: "assert" },
  { frameId: "104:3862", name: "Buyer Desktop · 購物車 · 輕便收納包入口", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "1440x900", screenshotFile: "parity/104-3862.png", deliveryPng: "after-104-3862.png", plan: "assert" },
  { frameId: "104:3920", name: "Buyer Mobile · 購物車 · 輕便收納包入口", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "390x844", screenshotFile: "parity/104-3920.png", deliveryPng: "after-104-3920.png", plan: "assert" },
  { frameId: "105:2439", name: "Buyer Desktop · 購物車空白", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "1440x900", screenshotFile: "parity/105-2439.png", deliveryPng: "verified-empty.png", plan: "assert" },
  { frameId: "105:2468", name: "Buyer Mobile · 購物車空白", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "390x844", screenshotFile: "parity/105-2468.png", deliveryPng: "mobile-empty-final.png", plan: "assert" },
  { frameId: "782:3904", name: "Buyer Desktop · 結帳 · 共用購物車脈絡", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "1440x900", screenshotFile: "parity/782-3904.png", deliveryPng: "verified-desktop-checkout.png", plan: "DEFERRED", reason: "owned by PARITY-CHECKOUT (integrator ruling 2026-11-04)", deferredTo: "PARITY-CHECKOUT" },
  { frameId: "782:4087", name: "Buyer Mobile · 結帳 · 共用購物車脈絡", bucket: "U:b-cart", route: "/[locale]/cart", viewport: "390x844", screenshotFile: "parity/782-4087.png", deliveryPng: "verified-checkout-beige-bottom.png", plan: "DEFERRED", reason: "owned by PARITY-CHECKOUT (integrator ruling 2026-11-04)", deferredTo: "PARITY-CHECKOUT" },
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
  const notRun: { frame: string; locale: string; subcase: string; reason: string }[] = [];

  for (const [locale, c] of Object.entries(copy)) {
    for (const width of [1440, 390]) {
      const height = width === 1440 ? 900 : 844;
      const isMobile = width === 390;
      const viewport = `${width}x${height}`;
      const f = (desktop: string, mobile: string) => (isMobile ? mobile : desktop);

      const context = await browser.newContext(ctxOpts({ ignoreHTTPSErrors: true, viewport: { width, height }, locale }));
      // `let`: the b-cart entry frames re-point page at their own context (fresh cart) at the end of this iteration.
      let page = await context.newPage();

      // One ledger row per operation that really ran; the callback returns what it observed.
      const step = async (frame: string, route: string, control: string, operation: string, expected: string, perform: () => Promise<string | void>) => {
        // Injected contact POST responses exercise the real shell but never reach Go/PG.
        // Keep their evidence tier distinct from the separately asserted real-producer receipt.
        const environment = control.startsWith("contact ") && control !== "contact real producer receipt"
          ? "MOCK (intercepted contact POST; REAL-STACK shell)" : "REAL-STACK (SANDBOX)";
        const row = { frame, locale, viewport, route, control, operation, expected, environment };
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
        // Protocol candidate: the route is wholly intercepted and never sends a buyer message.
        // The three synthetic field values stay in memory; evaluate only reads browser storage for privacy assertions.
        {
          const frame = f("26:850", "26:1117");
          const route = `/${locale}/contact`;
          const contactPage = await context.newPage();
          const attempts: { key: string | undefined; body: string | null }[] = [];
          const synthetic = { name: "Fixture Contact", email: "fixture@example.invalid", message: "Fixture inquiry only" };
          let reply: "uncertain" | "received" = "uncertain";
          const intercept = async (r: import("@playwright/test").Route) => {
            const request = r.request();
            if (request.method() !== "POST") return r.abort();
            attempts.push({ key: request.headers()["idempotency-key"], body: request.postData() });
            if (reply === "uncertain") return r.fulfill({ status: 503, json: { code: "unavailable" } });
            return r.fulfill({ status: 201, json: { message_id: "00000000-0000-4000-8000-000000000001", status: "received" } });
          };
          try {
            await contactPage.route("**/api/buyer/contact-messages", intercept);
            await contactPage.goto(`${origin}${route}`);
            await step(frame, route, "contact protocol entry", "open contact page", "required contact form is visible", async () => {
              await expect(contactPage.getByTestId("contact-form")).toBeVisible({ timeout: 2000 });
            });
            if (!(await contactPage.getByTestId("contact-form").isVisible())) {
              notRun.push({ frame, locale, subcase: `${viewport} contact protocol`, reason: "contact form absent; failure, same-key recovery and privacy checks not run" });
            } else {
              await step(frame, route, "contact uncertainty recovery", "fill and submit; explicitly recover the original request; refresh", "no automatic retry, original key/body retained, confirmed receipt survives refresh, no field values in URL or web storage", async () => {
                await contactPage.getByTestId("contact-name").fill(synthetic.name);
                await contactPage.getByTestId("contact-email").fill(synthetic.email);
                await contactPage.getByTestId("contact-message").fill(synthetic.message);
                await contactPage.getByTestId("contact-next").click();
                await expect(contactPage.getByTestId("contact-confirm")).toBeVisible({ timeout: T });
                assert.equal(attempts.length, 0, "next must not send the message");
                for (const value of Object.values(synthetic)) await expect(contactPage.getByTestId("contact-confirm")).toContainText(value);
                const otherTab = await context.newPage();
                await otherTab.goto(`${origin}${route}`);
                await contactPage.getByTestId("contact-send").click();
                await expect(contactPage.getByTestId("contact-unknown")).toBeVisible({ timeout: T });
                await expect(otherTab.getByTestId("contact-unknown")).toBeVisible({ timeout: T });
                await expect(otherTab.getByTestId("contact-recover")).toHaveCount(0);
                await otherTab.close();
                assert.equal(attempts.length, 1, "uncertain result must not be retried automatically");
                assert(attempts[0].key && /^[A-Za-z0-9_.:-]{8,128}$/.test(attempts[0].key), "valid idempotency key required");
                assert.deepEqual(JSON.parse(attempts[0].body ?? "null"), synthetic);
                const storage = await contactPage.evaluate(() => JSON.stringify({ local: { ...localStorage }, session: { ...sessionStorage } }));
                for (const value of Object.values(synthetic)) {
                  assert(!contactPage.url().includes(value), "contact value leaked into URL");
                  assert(!storage.includes(value), "contact value leaked into browser storage");
                }
                reply = "received";
                await contactPage.getByTestId("contact-recover").click();
                await expect(contactPage.getByTestId("contact-sent")).toBeVisible({ timeout: T });
                assert.equal(attempts.length, 2, "one explicit recovery request required");
                assert.deepEqual(attempts[1], attempts[0], "recovery must use the same key and exact body");
                await contactPage.reload();
                await expect(contactPage.getByTestId("contact-sent")).toBeVisible({ timeout: T });
                assert.equal(attempts.length, 2, "refresh must not resend");
              });
              await step(frame, route, "contact uncertainty refresh", "start a separate uncertain attempt and refresh", "unknown remains visible, no automatic resend or false success, and fields are absent from browser storage", async () => {
                const fresh = await context.newPage();
                reply = "uncertain";
                const start = attempts.length;
                try {
                  await fresh.route("**/api/buyer/contact-messages", intercept);
                  await fresh.goto(`${origin}${route}`);
                  await fresh.getByTestId("contact-name").fill(synthetic.name);
                  await fresh.getByTestId("contact-email").fill(synthetic.email);
                  await fresh.getByTestId("contact-message").fill(synthetic.message);
                  await fresh.getByTestId("contact-next").click();
                  await expect(fresh.getByTestId("contact-confirm")).toBeVisible({ timeout: T });
                  await fresh.getByTestId("contact-send").click();
                  await expect(fresh.getByTestId("contact-unknown")).toBeVisible({ timeout: T });
                  assert.equal(attempts.length, start + 1);
                  await fresh.reload();
                  await expect(fresh.getByTestId("contact-unknown")).toBeVisible({ timeout: T });
                  await expect(fresh.getByTestId("contact-sent")).toHaveCount(0);
                  assert.equal(attempts.length, start + 1, "refresh must not resend an uncertain submission");
                  const storage = await fresh.evaluate(() => JSON.stringify({ local: { ...localStorage }, session: { ...sessionStorage } }));
                  for (const value of Object.values(synthetic)) assert(!storage.includes(value), "contact value leaked into browser storage");
                } finally {
                  await fresh.unroute("**/api/buyer/contact-messages", intercept);
                  await fresh.close();
                }
              });
            }
          } finally {
            await contactPage.unroute("**/api/buyer/contact-messages", intercept);
            await contactPage.close();
          }
        }

        // Contract-shaped failure injections: no POST reaches the real contact producer in these cases.
        {
          const frame = f("26:850", "26:1117"), route = `/${locale}/contact`;
          for (const status of [429, 409, 422]) {
            await step(frame, route, `contact ${status} refusal`, "fill, review and submit once", "visible refusal, no sent state and no automatic retry (MOCK)", async () => {
              const isolated = await browser.newContext(ctxOpts({ ignoreHTTPSErrors: true, viewport: { width, height }, locale }));
              const buyer = await isolated.newPage(); let posts = 0;
              try {
                await isolated.route("**/api/buyer/contact-messages", async r => { posts++; await r.fulfill({ status, json: { code: status === 429 ? "rate_limited" : status === 409 ? "conflict" : "invalid_request" } }); });
                await buyer.goto(`${origin}${route}`);
                await buyer.getByTestId("contact-name").fill("Fixture Contact");
                await buyer.getByTestId("contact-email").fill("fixture@example.invalid");
                await buyer.getByTestId("contact-message").fill("Fixture inquiry only");
                await buyer.getByTestId("contact-next").click();
                await buyer.getByTestId("contact-send").click();
                await expect(buyer.locator("main").getByRole("alert")).toBeVisible({ timeout: T });
                await expect(buyer.getByTestId("contact-sent")).toHaveCount(0);
                assert.equal(posts, 1, "a refusal must not trigger another POST");
              } finally { await isolated.close(); }
            });
          }
          await step(frame, route, "contact concurrent tabs", "submit from two already-open tabs concurrently", "one intercepted POST, both tabs visibly uncertain (MOCK)", async () => {
            const isolated = await browser.newContext(ctxOpts({ ignoreHTTPSErrors: true, viewport: { width, height }, locale }));
            const first = await isolated.newPage(), second = await isolated.newPage(); let posts = 0;
            try {
              await isolated.route("**/api/buyer/contact-messages", async r => { posts++; await r.fulfill({ status: 503, json: { code: "unavailable" } }); });
              for (const buyer of [first, second]) {
                await buyer.goto(`${origin}${route}`);
                await buyer.getByTestId("contact-name").fill("Fixture Contact");
                await buyer.getByTestId("contact-email").fill("fixture@example.invalid");
                await buyer.getByTestId("contact-message").fill("Fixture inquiry only");
                await buyer.getByTestId("contact-next").click();
              }
              await Promise.allSettled([first.getByTestId("contact-send").click({ timeout: 1500 }), second.getByTestId("contact-send").click({ timeout: 1500 })]);
              await expect.poll(() => posts, { timeout: T }).toBe(1);
              await expect(first.getByTestId("contact-unknown")).toBeVisible({ timeout: T });
              await expect(second.getByTestId("contact-unknown")).toBeVisible({ timeout: T });
              assert.equal(posts, 1, "competing tabs must not submit twice");
            } finally { await isolated.close(); }
          });
          await step(frame, route, "contact unavailable locks", "submit in a browser without Web Locks", "visible refusal and no POST (MOCK)", async () => {
            const isolated = await browser.newContext(ctxOpts({ ignoreHTTPSErrors: true, viewport: { width, height }, locale }));
            await isolated.addInitScript(() => Object.defineProperty(navigator, "locks", { value: undefined }));
            const buyer = await isolated.newPage(); let posts = 0;
            try {
              await isolated.route("**/api/buyer/contact-messages", async r => { posts++; await r.abort(); });
              await buyer.goto(`${origin}${route}`);
              await buyer.getByTestId("contact-name").fill("Fixture Contact");
              await buyer.getByTestId("contact-email").fill("fixture@example.invalid");
              await buyer.getByTestId("contact-message").fill("Fixture inquiry only");
              await buyer.getByTestId("contact-next").click();
              await buyer.getByTestId("contact-send").click();
              await expect(buyer.locator("main").getByRole("alert")).toBeVisible({ timeout: T });
              assert.equal(posts, 0, "unsupported locking must fail closed");
            } finally { await isolated.close(); }
          });
        }

        // One synthetic message traverses the real Next BFF, Go handler and isolated PG fixture.
        // The fixture owns random encryption material and no mail worker; failure-injection cases above are MOCK.
        if (locale === "en" && isMobile) {
          const frame = "26:1117";
          const route = `/${locale}/contact`;
          await step(frame, route, "contact real producer receipt", "fill, review, submit once and reload", "real BFF/Go returns 201, receipt survives refresh without another POST", async () => {
            const isolated = await browser.newContext(ctxOpts({ ignoreHTTPSErrors: true, viewport: { width, height }, locale }));
            const buyer = await isolated.newPage();
            const posts: import("@playwright/test").Request[] = [];
            buyer.on("request", request => {
              if (new URL(request.url()).pathname === "/api/buyer/contact-messages" && request.method() === "POST") posts.push(request);
            });
            try {
              await buyer.goto(`${origin}${route}`);
              await buyer.getByTestId("contact-name").fill("Synthetic Test Buyer");
              await buyer.getByTestId("contact-email").fill("sfr-buyer@example.invalid");
              await buyer.getByTestId("contact-message").fill("Synthetic storefront browser fixture");
              await buyer.getByTestId("contact-next").click();
              await expect(buyer.getByTestId("contact-confirm")).toBeVisible({ timeout: T });
              assert.equal(posts.length, 0, "review must not send");
              const receipt = buyer.waitForResponse(response => new URL(response.url()).pathname === "/api/buyer/contact-messages" && response.request().method() === "POST", { timeout: T });
              await buyer.getByTestId("contact-send").click();
              assert.equal((await receipt).status(), 201, "real producer must return a committed receipt");
              await expect(buyer.getByTestId("contact-sent")).toBeVisible({ timeout: T });
              assert.equal(posts.length, 1);
              await buyer.reload();
              await expect(buyer.getByTestId("contact-sent")).toBeVisible({ timeout: T });
              assert.equal(posts.length, 1, "reload must not resend the contact message");
            } finally { await isolated.close(); }
          });
        }

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

        // ===== Home: /[locale] (4:9192 / 4:9296). Signed layout with this tenant's data: welcome title from the store name, search field,
        // inset hero card from the published hero section, collection chips, the product grid with a bottom "view all products" button.
        const homeFrame = f("4:9192", "4:9296");
        const homeRoute = `/${locale}`;
        const storeName: string = facts?.design?.name ?? "";
        const heroHeading: string = facts?.design?.hero ?? "";
        const published = (facts?.collections ?? []).filter((x: any) => x.active_products > 0);
        const main = page.locator("main");
        const box = async (locator: ReturnType<Page["locator"]>) => {
          const b = await locator.boundingBox({ timeout: T });
          if (!b) throw new Error("element has no layout box");
          return b;
        };
        await page.goto(`${origin}${homeRoute}`);
        await settle();
        await step(homeFrame, homeRoute, "home search field", "assert", `label "${c.searchLabel}", placeholder "${c.searchHint}"`, async () => {
          assert(storeName && heroHeading, "facts.design.name / facts.design.hero missing");
          const field = main.getByRole("searchbox", { name: c.searchLabel, exact: true });
          await expect(field).toBeVisible({ timeout: T });
          await expect(field).toHaveAttribute("placeholder", c.searchHint);
          await expect(main.getByText(c.searchLabel, { exact: true })).toBeVisible();
        });
        // The signed search field anchors this layout: without it every later home step would only wait out its timeout, so they are
        // recorded NOT_RUN (the frame stays NOT_COVERED through the failed anchor row) instead of being run.
        const homeSigned = await main.getByRole("searchbox", { name: c.searchLabel, exact: true }).isVisible();
        if (!homeSigned) notRun.push({ frame: homeFrame, locale, subcase: `${viewport} home layout, hero, chips, clicks`, reason: "signed search field absent; dependent steps not run" });
        else {
          await step(homeFrame, homeRoute, "home welcome title", "goto + assert", `title "${fmt(c.welcome, { name: storeName })}" visible above the search field`, async () => {
            assert(storeName && heroHeading, "facts.design.name / facts.design.hero missing");
            await expect(main.getByText(fmt(c.welcome, { name: storeName }), { exact: true })).toBeVisible({ timeout: T });
          });
          await step(homeFrame, homeRoute, "home layout order", "assert", "welcome title < search < inset hero card < chips < product grid, hero card inset in the page gutter with rounded corners", async () => {
            const title = await box(main.getByText(fmt(c.welcome, { name: storeName }), { exact: true }));
            const search = await box(main.getByRole("searchbox", { name: c.searchLabel, exact: true }));
            const hero = await box(page.getByTestId("section-hero"));
            const chips = await box(main.getByTestId("collection-chips").filter({ visible: true }));
            const grid = await box(page.getByTestId("section-product-grid"));
            assert(title.y < search.y && search.y < hero.y && hero.y < chips.y && chips.y < grid.y, `order title ${title.y} search ${search.y} hero ${hero.y} chips ${chips.y} grid ${grid.y}`);
            assert(hero.x >= 15.5 && hero.x + hero.width <= width - 15.5, `hero card must sit inside the gutter, got x=${hero.x} w=${hero.width}`);
            const radius = await page.getByTestId("section-hero").evaluate((el) => parseFloat(getComputedStyle(el).borderTopLeftRadius)); // read-only measurement
            assert(radius >= 8, `hero card corner radius ${radius}`);
            return `title ${title.y} search ${search.y} hero ${hero.x},${hero.y} ${hero.width}w r${radius} chips ${chips.y} grid ${grid.y}`;
          });
          await step(homeFrame, homeRoute, "home hero card", "assert", `published hero heading "${heroHeading}" and its CTA inside the card`, async () => {
            const hero = page.getByTestId("section-hero");
            await expect(hero.getByRole("heading", { name: heroHeading, exact: true })).toBeVisible({ timeout: T });
            await expect(hero.getByRole("link")).toHaveCount(1);
          });
          await step(homeFrame, homeRoute, "home chips", "assert", `chips "${c.homeAllProducts}", every published collection, "${c.allCollections}"`, async () => {
            const chips = main.getByTestId("collection-chips").filter({ visible: true });
            await expect(chips.getByRole("link", { name: c.homeAllProducts, exact: true })).toBeVisible({ timeout: T });
            for (const col of published) await expect(chips.getByRole("link", { name: col.title, exact: true })).toHaveAttribute("href", new RegExp(`/${locale}/collections/${col.slug}$`));
            await expect(chips.getByRole("link", { name: c.allCollections, exact: true })).toHaveAttribute("href", new RegExp(`/${locale}/collections$`));
            return `${published.length} collection chips`;
          });
          await step(homeFrame, homeRoute, "home legacy grid link gone", "assert", `product grid head has no "${c.legacyViewAll}" link; "${c.viewAllProducts}" sits below the grid`, async () => {
            const grid = page.getByTestId("section-product-grid");
            await expect(grid.getByRole("link", { name: c.legacyViewAll, exact: true })).toHaveCount(0);
            const button = await box(grid.getByRole("link", { name: c.viewAllProducts, exact: true }));
            const lastCard = await box(grid.getByTestId("product-card").last());
            assert(button.y >= lastCard.y + lastCard.height, `view-all button ${button.y} must be below the last card ${lastCard.y + lastCard.height}`);
          });
          await shot(homeFrame, homeRoute);
          await step(homeFrame, homeRoute, "home search field", "fill + Enter", `query "${candle.title}" opens the search results`, async () => {
            await main.getByRole("searchbox", { name: c.searchLabel, exact: true }).fill(candle.title, { timeout: T });
            await main.getByRole("searchbox", { name: c.searchLabel, exact: true }).press("Enter", { timeout: T });
            await waitForHomeDestination(page, new RegExp(`/${locale}/search\\?`), fmt(shopCopy[locale as keyof typeof shopCopy].resultsFor, { q: candle.title }));
            assert.equal(urlParams().get("q"), candle.title);
            await expect(page.getByTestId("product-card").filter({ hasText: candle.title }).first()).toBeVisible({ timeout: T });
            return page.url();
          });
          await page.goto(`${origin}${homeRoute}`);
          await step(homeFrame, homeRoute, `hero CTA link`, "click", "the published CTA (all_products) opens the product list", async () => {
            await page.getByTestId("section-hero").getByRole("link").click({ timeout: T });
            await waitForHomeDestination(page, new RegExp(`/${locale}/products/?$`), c.allProducts);
            return page.url();
          });
          await page.goto(`${origin}${homeRoute}`);
          await step(homeFrame, homeRoute, `chip "${c.allCollections}"`, "click", "opens the collections overview", async () => {
            await main.getByTestId("collection-chips").filter({ visible: true }).getByRole("link", { name: c.allCollections, exact: true }).click({ timeout: T });
            await waitForHomeDestination(page, new RegExp(`/${locale}/collections$`), c.collectionsOverview, false);
            return page.url();
          });
          await page.goto(`${origin}${homeRoute}`);
          await step(homeFrame, homeRoute, `button "${c.viewAllProducts}"`, "click", "opens the product list", async () => {
            await page.getByTestId("section-product-grid").getByRole("link", { name: c.viewAllProducts, exact: true }).click({ timeout: T });
            await waitForHomeDestination(page, new RegExp(`/${locale}/products/?$`), c.allProducts);
            return page.url();
          });
        }

        // ===== Collection detail: /[locale]/collections/home-fragrance (6:19933 / 6:19984), the merchant's collection in the signed layout.
        const collectionFrame = f("6:19933", "6:19984");
        const fragrance = (facts?.collections ?? []).find((x: any) => x.slug === "home-fragrance");
        const collectionRoute = `/${locale}/collections/home-fragrance`;
        await page.goto(`${origin}${collectionRoute}`);
        await settle();
        await step(collectionFrame, collectionRoute, "collection search field", "assert", `label "${c.searchLabel}", placeholder "${c.searchHint}"`, async () => {
          assert(fragrance, "facts.collections home-fragrance missing");
          const field = main.getByRole("searchbox", { name: c.searchLabel, exact: true });
          await expect(field).toBeVisible({ timeout: T });
          await expect(field).toHaveAttribute("placeholder", c.searchHint);
        });
        const collectionSigned = await main.getByRole("searchbox", { name: c.searchLabel, exact: true }).isVisible();
        if (!collectionSigned) notRun.push({ frame: collectionFrame, locale, subcase: `${viewport} collection count, chips, layout, clicks`, reason: "signed search field absent; dependent steps not run" });
        else {
          await step(collectionFrame, collectionRoute, "collection title + count", "goto + assert", `h1 = collection title; heading "${c.collectionCount}" with the real product count = rendered cards`, async () => {
            assert(fragrance, "facts.collections home-fragrance missing");
            await expect(page.getByRole("heading", { level: 1 })).toHaveText(fragrance.title, { timeout: T });
            const expected = fmt(c.collectionCount, { n: fragrance.active_products });
            await expect(main.getByRole("heading", { name: expected, exact: true })).toBeVisible({ timeout: T });
            await expect(page.getByTestId("product-card")).toHaveCount(fragrance.active_products, { timeout: T });
            return expected;
          });
          await step(collectionFrame, collectionRoute, "collection chips", "assert", `"${c.allProducts}", this collection current, "${c.allCollections}"`, async () => {
            const chips = main.getByTestId("collection-chips").filter({ visible: true });
            await expect(chips.getByRole("link", { name: c.allProducts, exact: true })).not.toHaveAttribute("aria-current", "page");
            await expect(chips.getByRole("link", { name: fragrance.title, exact: true })).toHaveAttribute("aria-current", "page");
            await expect(chips.getByRole("link", { name: c.allCollections, exact: true })).toBeVisible();
          });
          await step(collectionFrame, collectionRoute, "collection layout order", "assert", "title < search < chips < count heading < grid < view-all button", async () => {
            const title = await box(page.getByRole("heading", { level: 1 }));
            const search = await box(main.getByRole("searchbox", { name: c.searchLabel, exact: true }));
            const chips = await box(main.getByTestId("collection-chips").filter({ visible: true }));
            const count = await box(main.getByRole("heading", { name: fmt(c.collectionCount, { n: fragrance.active_products }), exact: true }));
            const card = await box(page.getByTestId("product-card").last());
            const button = await box(main.getByRole("link", { name: c.viewAllProducts, exact: true }));
            assert(title.y < search.y && search.y < chips.y && chips.y < count.y && count.y < card.y && card.y + card.height <= button.y,
              `order title ${title.y} search ${search.y} chips ${chips.y} count ${count.y} card ${card.y} button ${button.y}`);
          });
          await shot(collectionFrame, collectionRoute);
          await step(collectionFrame, collectionRoute, `button "${c.viewAllProducts}"`, "click", "opens the product list", async () => {
            await main.getByRole("link", { name: c.viewAllProducts, exact: true }).click({ timeout: T });
            await expect(page).toHaveURL(new RegExp(`/${locale}/products/?$`), { timeout: T });
            await waitForHomeDestination(page, new RegExp(`/${locale}/products/?$`), c.allProducts);
            return page.url();
          });
          await page.goto(`${origin}${collectionRoute}`);
          await step(collectionFrame, collectionRoute, `chip "${c.allCollections}"`, "click", "opens the collections overview", async () => {
            await main.getByTestId("collection-chips").filter({ visible: true }).getByRole("link", { name: c.allCollections, exact: true }).click({ timeout: T });
            await expect(page).toHaveURL(new RegExp(`/${locale}/collections$`), { timeout: T });
            await waitForHomeDestination(page, new RegExp(`/${locale}/collections$`), c.collectionsOverview, false);
            return page.url();
          });
          await page.goto(`${origin}${collectionRoute}`);
          await step(collectionFrame, collectionRoute, "collection search field", "fill + Enter", `query "${candle.title}" opens the search results`, async () => {
            await main.getByRole("searchbox", { name: c.searchLabel, exact: true }).fill(candle.title, { timeout: T });
            await main.getByRole("searchbox", { name: c.searchLabel, exact: true }).press("Enter", { timeout: T });
            await page.waitForURL(new RegExp(`/${locale}/search\\?`), { timeout: T });
            assert.equal(urlParams().get("q"), candle.title);
            await waitForHomeDestination(page, new RegExp(`/${locale}/search\\?`), fmt(shopCopy[locale as keyof typeof shopCopy].resultsFor, { q: candle.title }));
            await expect(page.getByTestId("product-card").filter({ hasText: candle.title }).first()).toBeVisible({ timeout: T });
            return page.url();
          });
        }

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
                await Promise.all([
                  page.waitForURL((url) => url.pathname === collectionRoute && !url.searchParams.has("min") && !url.searchParams.has("max"), { timeout: T }),
                  controls().getByRole("link", { name: c.clear, exact: true }).click({ timeout: T }),
                ]);
                await settle();
                await checkClearedCollection("newest");
                return "explicit newest, preview and newest-first card retained after clear and reload";
              });
              await step(frame, collectionRoute, "mobile collection newest empty clear", "click newest + fill nonmatching price + apply + click empty-state Clear + reload", "Empty-state Clear removes price bounds, keeps explicit newest and preview, and restores newest-first cards", async () => {
                await openFilteredCollection(true, "99999", "100000");
                await expect(page.getByTestId("list-empty")).toBeVisible({ timeout: T });
                await expect(page.getByTestId("product-card")).toHaveCount(0);
                await Promise.all([
                  page.waitForURL((url) => url.pathname === collectionRoute && !url.searchParams.has("min") && !url.searchParams.has("max"), { timeout: T }),
                  page.getByTestId("list-empty").getByRole("link", { name: c.clear, exact: true }).click({ timeout: T }),
                ]);
                await settle();
                await checkClearedCollection("newest");
                return "empty-state clear retained explicit newest and preview after reload";
              });
              await step(frame, collectionRoute, "mobile collection absent sort clear", "open filtered collection without sort + click Clear + reload", "Clear leaves absent sort absent and restores the collection's manual first card", async () => {
                await openFilteredCollection(false, "15", "20");
                await Promise.all([
                  page.waitForURL((url) => url.pathname === collectionRoute && !url.searchParams.has("min") && !url.searchParams.has("max"), { timeout: T }),
                  controls().getByRole("link", { name: c.clear, exact: true }).click({ timeout: T }),
                ]);
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
                await Promise.all([
                  page.waitForURL((url) => url.pathname === searchRoute && !url.searchParams.has("min") && !url.searchParams.has("max"), { timeout: T }),
                  controls().getByRole("link", { name: c.clear, exact: true }).click({ timeout: T }),
                ]);
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

        // ===== 51:1380: current live 4:9532 task/dock entries -> full mobile option composition ======================================
        // Source: ci-evidence-1238/c7-live-1853/{base,options}-context.txt, original PNGs viewed; human decision 46d89ab2.
        // Keep this after every existing sort/Clear/cart assertion: its single successful add must not change their baseline.
        if (isMobile) {
          const frame = "51:1380";
          const params = new URLSearchParams();
          if (facts.preview_token) params.set("preview", facts.preview_token);
          const productURL = `${origin}${productRoute}${params.size ? `?${params}` : ""}`;
          const sessionResponse = await context.request.get(`${origin}/api/buyer/session`);
          assert(sessionResponse.ok(), `existing buyer session read failed (HTTP ${sessionResponse.status()})`);
          const session = await sessionResponse.json() as { state?: unknown; context?: unknown };
          assert(session.state === "active", "options browsing requires the existing active buyer session");
          const buyerContext = session.context;
          assert(typeof buyerContext === "string" && buyerContext.length > 0, "existing active buyer context is required");
          // Reuse the existing private-read contract; never create a session or expose its context in evidence.
          const readCart = () => context.request.get(`${origin}/api/buyer/cart`, { headers: { "X-Buyer-Context": buyerContext } });
          const beforeCart = await readCart();
          assert(beforeCart.ok(), `existing cart must be readable before options browsing (HTTP ${beforeCart.status()})`);
          const baseline = await beforeCart.json() as { version: number; items: { sku_id: string; quantity: number }[] };
          // GETs only: read the existing real-PG product/cart, never prepare the tested action through an API write.
          const detailResponse = await context.request.get(`${origin}/api/shop/product/${candle.slug}`);
          assert(detailResponse.ok(), "existing real-PG product detail must be readable");
          const detail = await detailResponse.json() as {
            id: string; title: string; images: { id: string }[];
            options: { name: string; values: string[] }[];
            variants: { sku_id: string; price_minor: number; option_values: string[]; image_id: string | null; stock: string }[];
          };
          const price = (minor: number) => {
            const currency = facts.currency;
            const digits = new Intl.NumberFormat(locale, { style: "currency", currency }).resolvedOptions().maximumFractionDigits ?? 2;
            const display = new Intl.NumberFormat(locale, { style: "currency", currency, minimumFractionDigits: minor % 10 ** digits === 0 ? 0 : digits });
            return display.formatToParts(minor / 10 ** digits).map((part) => currency === "TWD" && part.type === "currency" ? "NT$" : part.value).join("");
          };
          const task = () => page.getByTestId("product-options-task");
          const entry = (surface: "task" | "dock") => page.getByTestId(`product-options-${surface}-entry`).getByRole("button", { name: c.chooseSpec, exact: true });
          const back = () => task().getByRole("button", { name: c.specBack, exact: true });
          const quantity = () => task().getByTestId("qty-input");
          const writes: { method: string; pathname: string; request: import("@playwright/test").Request }[] = [];
          const observeWrite = (request: import("@playwright/test").Request) => {
            const url = new URL(request.url());
            if (url.origin === new URL(origin).origin && !["GET", "HEAD", "OPTIONS"].includes(request.method())) {
              // Request objects stay in memory for CAS verification; no headers, bodies, credentials or full URLs enter artifacts.
              writes.push({ method: request.method(), pathname: url.pathname, request });
            }
          };
          const unchanged = async () => {
            assert.equal(writes.length, 0, "selection/return must issue zero writes");
            const response = await readCart();
            assert(response.ok(), "cart readback failed");
            assert.deepEqual(await response.json(), baseline, "selection/return changed the server cart");
          };
          const open = async (surface: "task" | "dock") => {
            await entry(surface).click({ timeout: T });
            await expect(task().getByRole("heading", { name: c.chooseSpec, exact: true })).toBeVisible({ timeout: T });
            assert.equal(new URL(page.url()).pathname, productRoute);
            for (const [key, value] of params) assert(urlParams().get(key) === value, `${key} must survive option navigation`);
          };
          page.on("request", observeWrite);
          let reachable = false;
          try {
            await page.goto(productURL);
            for (const surface of ["task", "dock"] as const) {
              await step(frame, productRoute, `spec ${surface} entry`, "click selection entry + click back", "source-defined entry opens the option task; back returns to the same product and focus, with zero cart writes", async () => {
                await open(surface);
                await expect(back()).toBeVisible();
                await back().click({ timeout: T });
                await expect(task()).toHaveCount(0);
                await expect(entry(surface)).toBeFocused({ timeout: T });
                await unchanged();
                reachable = true;
                return "real entry/back clicks returned focus and preserved server cart";
              });
              // A failed entry is the causal RED. Do not fabricate dependent PASS rows or reset product state through DOM.
              if (await task().count()) await page.keyboard.press("Escape");
            }
            if (reachable) {
              await step(frame, productRoute, "spec composition", "click task entry + inspect", "390x844 full task with 64px header, 84px fixed dock, server title/price/image, option axes, quantity and one add action", async () => {
                await open("task");
                const header = await task().getByTestId("product-options-header").boundingBox();
                const dock = await task().getByTestId("product-options-dock").boundingBox();
                const body = await task().boundingBox();
                assert(header && dock && body, "mobile task regions must be visible");
                assert(Math.abs(body.x) <= 1 && Math.abs(body.width - 390) <= 1 && Math.abs(body.y) <= 1 && Math.abs(body.height - 844) <= 1, "task must fill the viewport");
                assert(Math.abs(header.y) <= 1 && Math.abs(header.height - 64) <= 1, "header must occupy y=0..64");
                assert(Math.abs(dock.y - 760) <= 1 && Math.abs(dock.height - 84) <= 1, "dock must occupy y=760..844");
                const summary = task().getByTestId("product-options-summary");
                await expect(summary).toContainText(detail.title);
                const sku = await task().getByTestId("product-buy").getAttribute("data-sku");
                const selected = detail.variants.find((v) => v.sku_id === sku);
                assert(selected, "selection must resolve to a server SKU");
                await expect(summary).toContainText(price(selected.price_minor));
                for (const axis of detail.options) await expect(task().getByRole("group", { name: new RegExp(axis.name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")) })).toBeVisible();
                await expect(task().getByText(c.specLimit, { exact: true })).toBeVisible();
                await expect(task().getByText(c.specPickup, { exact: true })).toBeVisible();
                await expect(task().getByText(c.quantity, { exact: true })).toBeVisible();
                await expect(quantity()).toHaveValue("1");
                await expect(task().getByTestId("buy-now")).toHaveCount(0);
                await expect(task().getByTestId("add-to-cart")).toHaveCount(1);
                await expect(task().getByTestId("add-to-cart")).toHaveText(`${c.addToCart} · ${price(selected.price_minor)}`);
                await page.mouse.move(0, 0);
                await expect(task().getByTestId("add-to-cart")).toHaveCSS("background-color", "rgb(20, 41, 66)");
                await expect(task().getByTestId("add-to-cart")).toHaveCSS("color", "rgb(255, 255, 255)");
                const imageID = selected.image_id ?? detail.images[0]?.id;
                if (imageID) await expect(summary.locator("img")).toHaveAttribute("src", new RegExp(`/media/p/${detail.id}/${imageID}(?:\\?|$)`));
                else notRun.push({ frame, locale, subcase: "summary image", reason: "existing real product has no server image" });
                await unchanged();
              });
              await shot(frame, productRoute);
              let chosen: (typeof detail.variants)[number] | undefined;
              let draftQty = "1";
              const initial = await task().getByTestId("product-buy").getAttribute("data-sku");
              const alternative = detail.variants.find((v) => v.stock !== "out" && v.sku_id !== initial);
              if (!alternative) notRun.push({ frame, locale, subcase: "alternative option SKU price image", reason: "existing real product has no second available SKU; option-switch evidence remains missing" });
              else await step(frame, productRoute, "spec option SKU price image", "click real option radios", "each available option keeps chosen SKU, displayed price, summary image and active controls consistent", async () => {
                for (let axis = 0; axis < alternative.option_values.length; axis++) {
                  // Label click is a real user action; value is taken from the server, not a screenshot fixture.
                  const value = alternative.option_values[axis];
                  const radio = task().getByRole("radio", { name: value, exact: true });
                  const label = radio.locator("..");
                  const box = await label.boundingBox();
                  assert(box && box.width >= 44 && box.height >= 44, "option touch target must be at least 44px");
                  await label.click({ timeout: T });
                  await expect(radio).toBeChecked();
                }
                chosen = alternative;
                await expect(task().getByTestId("product-buy")).toHaveAttribute("data-sku", chosen.sku_id);
                await expect(task().getByTestId("variant-price")).toHaveText(price(chosen.price_minor));
                await expect(task().getByTestId("add-to-cart")).toHaveText(`${c.addToCart} · ${price(chosen.price_minor)}`);
                const imageID = chosen.image_id ?? detail.images[0]?.id;
                if (imageID) await expect(task().getByTestId("product-options-summary").locator("img")).toHaveAttribute("src", new RegExp(`/media/p/${detail.id}/${imageID}(?:\\?|$)`));
                await unchanged();
              });
              await step(frame, productRoute, "spec quantity bounds", "click minus/plus + fill quantity + blur", "quantity stays within existing 1..99 bounds; all changes issue zero writes", async () => {
                const minus = task().getByRole("button", { name: c.decrease, exact: true });
                const plus = task().getByRole("button", { name: c.increase, exact: true });
                await expect(minus).toBeDisabled();
                await plus.click({ timeout: T });
                await expect(quantity()).toHaveValue("2");
                await minus.click({ timeout: T });
                await expect(quantity()).toHaveValue("1");
                await quantity().fill("99");
                await quantity().press("Tab");
                await expect(plus).toBeDisabled();
                await quantity().fill("0");
                await quantity().press("Tab");
                await expect(quantity()).toHaveValue("1");
                await plus.click({ timeout: T });
                draftQty = "2";
                await expect(quantity()).toHaveValue(draftQty);
                await unchanged();
              });
              await step(frame, productRoute, "spec return draft", "click back + reopen; Escape + reopen", "both return methods restore entry focus, preserve chosen SKU/quantity/preview and leave cart unchanged", async () => {
                assert(chosen, "option selection must pass before draft preservation");
                for (const method of ["back", "Escape"] as const) {
                  if (method === "back") await back().click({ timeout: T });
                  else await page.keyboard.press("Escape");
                  await expect(task()).toHaveCount(0);
                  await expect(entry("task")).toBeFocused({ timeout: T });
                  await open("task");
                  await expect(task().getByTestId("product-buy")).toHaveAttribute("data-sku", chosen.sku_id);
                  await expect(quantity()).toHaveValue(draftQty);
                  await unchanged();
                }
              });
              await step(frame, productRoute, "spec entry return cart unchanged", "read server cart after all local choices", "all option/quantity/return operations issued zero writes and preserved the exact existing server cart", unchanged);
              // The independent lost-response case below uses a fresh buyer context and the real cart producer.
              const out = detail.variants.find((v) => v.stock === "out");
              if (!out) notRun.push({ frame, locale, subcase: "sold-out option", reason: "existing selected product has no sold-out variant; an in-stock SKU is not sold-out evidence" });
              else {
                const value = out.option_values.find((v, i) => !detail.variants.some((candidate) => candidate.stock !== "out" && candidate.option_values[i] === v));
                if (!value) {
                  notRun.push({ frame, locale, subcase: "sold-out option", reason: "out SKU has no independently unavailable axis value; combination not reachable through this UI" });
                } else await step(frame, productRoute, "spec sold-out option", "inspect unavailable real option", "out-of-stock alternative stays disabled", async () => {
                  await expect(task().getByRole("radio", { name: value, exact: true })).toBeDisabled();
                });
              }
              await step(frame, productRoute, "spec one add CAS and refreshed cart", "click add once + server readback + reload", "one existing CAS cart write adds the selected SKU/quantity, then locale cart opens and refresh persists it", async () => {
                assert(chosen, "option selection must pass before add");
                assert.equal(writes.length, 0, "no write may precede the add click");
                const add = task().getByTestId("add-to-cart");
                // Hold, then continue the real PUT unchanged so busy/disabled is observable without synthetic domain responses.
                let release = () => {};
                const held = new Promise<void>((resolve) => { release = resolve; });
                const holdWrite = async (route: import("@playwright/test").Route) => {
                  if (route.request().method() === "PUT") await held;
                  await route.continue();
                };
                await page.route("**/api/buyer/cart", holdWrite);
                try {
                  await add.click({ timeout: T });
                  await expect(add).toBeDisabled({ timeout: T });
                } finally {
                  release();
                  await page.unroute("**/api/buyer/cart", holdWrite);
                }
                await page.waitForURL((url) => url.pathname === cartRoute, { timeout: T });
                const cartWrites = writes.filter((w) => w.pathname === "/api/buyer/cart");
                assert.equal(cartWrites.length, 1, "one click must produce exactly one cart write");
                assert.equal(cartWrites[0].method, "PUT");
                const payload = cartWrites[0].request.postDataJSON();
                assert.equal(payload.expected_version, baseline.version, "existing CAS version must be used");
                const response = await cartWrites[0].request.response();
                assert(response?.ok(), "the add must have an authoritative successful response");
                const expectedQty = (baseline.items.find((i) => i.sku_id === chosen!.sku_id)?.quantity ?? 0) + Number(draftQty);
                const readback = await readCart();
                assert(readback.ok());
                const stored = await readback.json();
                assert.equal(stored.items.find((i: { sku_id: string }) => i.sku_id === chosen!.sku_id)?.quantity, expectedQty);
                for (const item of baseline.items.filter((i) => i.sku_id !== chosen!.sku_id)) assert(stored.items.some((i: { sku_id: string; quantity: number }) => i.sku_id === item.sku_id && i.quantity === item.quantity), "unrelated cart line must be preserved");
                const line = page.getByTestId("cart-page").locator(`[data-testid="cart-line"][data-sku="${chosen.sku_id}"]`);
                await expect(line.getByTestId("cart-line-qty")).toHaveText(String(expectedQty), { timeout: T });
                await expect(page.getByTestId("cart-drawer")).not.toBeVisible();
                await page.reload();
                await expect(line.getByTestId("cart-line-qty")).toHaveText(String(expectedQty), { timeout: T });
                assert.equal(writes.length, 1, "refresh must not replay the add or create another side effect");
                return "single successful CAS add; chosen SKU/quantity persisted after locale cart refresh";
              });
            } else {
              notRun.push({ frame, locale, subcase: "dependent options composition/selection/add", reason: "source-defined option entries did not reach task; see causal entry FAIL rows" });
            }
          } finally {
            page.off("request", observeWrite);
          }
        }

        // Independent sold-out witness: the candle has no unavailable value, while the existing
        // real-stack scarf fixture has one. Do not mutate stock or infer sold-out from an in-stock SKU.
        if (isMobile) {
          const frame = "51:1380";
          const scarf = facts.products.find((item: any) => item.key === "scarf");
          assert(scarf, "sold-out witness requires the existing scarf fixture");
          const route = `/${locale}/products/${scarf.slug}`;
          await step(frame, route, "spec sold-out option on independent fixture", "open options, select available option, inspect unavailable option, refresh and reopen", "real unavailable option stays disabled without a cart write", async () => {
            const response = await context.request.get(`${origin}/api/shop/product/${scarf.slug}`);
            assert(response.ok(), "scarf product detail must be readable");
            const detail = await response.json() as { variants: { option_values: string[]; stock: string }[] };
            const out = detail.variants.find((v) => v.stock === "out" && v.option_values.some((value, axis) =>
              !detail.variants.some((other) => other.stock !== "out" && other.option_values[axis] === value)));
            assert(out, "fixture must have an independently unavailable option value");
            const axis = out.option_values.findIndex((value, i) => !detail.variants.some((other) => other.stock !== "out" && other.option_values[i] === value));
            const unavailable = out.option_values[axis];
            const available = detail.variants.filter((v) => v.stock !== "out" && v.option_values[axis] !== unavailable);
            assert(available.length >= 2, "fixture needs two available alternatives for a real selection");
            const sessionResponse = await context.request.get(`${origin}/api/buyer/session`);
            assert(sessionResponse.ok(), "buyer session must be readable");
            const session = await sessionResponse.json() as { context?: unknown };
            assert(typeof session.context === "string" && session.context.length > 0, "active buyer context required");
            const readCart = () => context.request.get(`${origin}/api/buyer/cart`, { headers: { "X-Buyer-Context": session.context as string } });
            const before = await readCart();
            assert(before.ok(), "cart baseline must be readable");
            const baseline = await before.json();
            let writes = 0;
            const observe = (request: import("@playwright/test").Request) => {
              if (new URL(request.url()).pathname === "/api/buyer/cart" && request.method() === "PUT") writes++;
            };
            page.on("request", observe);
            try {
              const target = new URL(`${origin}${route}`);
              if (facts.preview_token) target.searchParams.set("preview", facts.preview_token);
              await page.goto(target.toString());
              for (let pass = 0; pass < 2; pass++) {
                await page.getByTestId("product-options-task-entry").getByRole("button", { name: c.chooseSpec, exact: true }).click({ timeout: T });
                const task = page.getByTestId("product-options-task");
                await expect(task.getByRole("radio", { name: unavailable, exact: true })).toBeDisabled();
                const first = task.getByRole("radio", { name: available[0].option_values[axis], exact: true });
                const other = task.getByRole("radio", { name: available[1].option_values[axis], exact: true });
                await (await first.isChecked() ? other : first).click({ timeout: T });
                await expect(task.getByRole("radio", { name: unavailable, exact: true })).toBeDisabled();
                if (pass === 0) await page.reload();
              }
              assert.equal(writes, 0, "selecting options must not write the cart");
              const after = await readCart();
              assert(after.ok(), "cart readback must be readable");
              assert.deepEqual(await after.json(), baseline, "sold-out inspection must preserve the cart");
            } finally {
              page.off("request", observe);
            }
          });
        }

        if (isMobile) {
          const frame = "51:1380";
          const scarf = facts.products.find((item: any) => item.key === "scarf");
          assert(scarf, "cart recovery witness requires the existing scarf fixture");
          const route = `/${locale}/products/${scarf.slug}`;
          await step(frame, route, "spec lost cart response recovery", "add once, lose committed reply, explicitly recover and reload cart", "one durable cart addition; explicit recovery uses the identical key and body; refresh does not resend", async () => {
            const isolated = await browser.newContext(ctxOpts({ ignoreHTTPSErrors: true, viewport: { width, height }, locale }));
            const witness = await isolated.newPage();
            const calls: { key: string | undefined; body: string | null }[] = [];
            const intercept = async (r: import("@playwright/test").Route) => {
              const request = r.request();
              if (request.method() !== "PUT") return r.continue();
              calls.push({ key: request.headers()["idempotency-key"], body: request.postData() });
              if (calls.length === 1) {
                // The real PG producer must commit before the response is discarded; no synthetic cart success.
                const result = await r.fetch();
                assert.equal(result.status(), 200, "lost response must follow an actual committed cart write");
                return r.abort("failed");
              }
              return r.continue();
            };
            try {
              await witness.route("**/api/buyer/cart", intercept);
              const target = new URL(`${origin}${route}`);
              if (facts.preview_token) target.searchParams.set("preview", facts.preview_token);
              await witness.goto(target.toString());
              await witness.getByTestId("product-options-task-entry").getByRole("button", { name: c.chooseSpec, exact: true }).click({ timeout: T });
              const task = witness.getByTestId("product-options-task");
              await task.getByTestId("add-to-cart").click({ timeout: T });
              const problem = task.getByTestId("cart-problem");
              await expect(problem).toBeVisible({ timeout: T });
              assert.equal(calls.length, 1, "unknown cart outcome must not automatically retry");
              await problem.getByRole("button").click({ timeout: T });
              await expect(problem).toHaveCount(0, { timeout: T });
              assert.equal(calls.length, 2);
              assert(calls[0].key, "original cart write must carry an idempotency key");
              assert.deepEqual(calls[1], calls[0], "explicit recovery must preserve key and body");
              await witness.goto(`${origin}/${locale}/cart`);
              await expect(witness.getByTestId("cart-line-qty")).toHaveText("1", { timeout: T });
              await witness.reload();
              await expect(witness.getByTestId("cart-line-qty")).toHaveText("1", { timeout: T });
              assert.equal(calls.length, 2, "refresh must not resend");
            } finally {
              await witness.unroute("**/api/buyer/cart", intercept);
              await isolated.close();
            }
          });
        }

        // ===== 51:1381: description entry/content/return plus the delivery section (integrator ruling 2026-11-04, round b-product) ======
        // Root freeze fcb50a14; original source 06f274c4. The delivery copy stays generic: which methods a store enabled is only known at checkout.
        if (isMobile) {
          const frame = "51:1381";
          const params = new URLSearchParams();
          if (facts.preview_token) params.set("preview", facts.preview_token);
          const productURL = `${origin}${productRoute}${params.size ? `?${params}` : ""}`;
          let originalURL = "";
          let buyerContext = "";
          let baseline: unknown;
          let detail: {
            title: string; description: string;
            options: { name: string; values: string[] }[];
            variants: { sku_id: string; option_values: string[]; stock: string }[];
          } | undefined;
          let alternative: NonNullable<typeof detail>["variants"][number] | undefined;
          const task = () => page.getByTestId("product-description-task");
          const entry = () => page.getByTestId("product-description-task-entry").getByRole("button", { name: c.descriptionEntry, exact: true });
          const options = () => page.getByTestId("product-options-task");
          const optionsEntry = () => page.getByTestId("product-options-task-entry").getByRole("button", { name: c.chooseSpec, exact: true });
          const readCart = () => context.request.get(`${origin}/api/buyer/cart`, { headers: { "X-Buyer-Context": buyerContext }, timeout: T });
          let phase: "setup-product-get" | "setup-merchant-data" | "setup-session-get" | "setup-session-data" | "setup-cart-get" | "setup-cart-data"
            | "setup-navigation" | "setup-base" | "setup-alternative" | "entry-visible" | "entry-click" | "dialog-visible" | "heading"
            | "url-preservation" | "zero-writes" | "readback-get" | "readback-data" | "merchant-content" | "layout"
            | "header-return" | "dock-return" | "escape-return" | "dialog-closed" | "entry-focus"
            | "options-entry" | "options-heading" | "options-selection" | "options-quantity" | "options-return" | "options-closed" | "options-focus"
            | "draft-base" | "draft-reopened" | "delivery" = "setup-product-get";
          // Reuse the ledger, but never serialize raw Playwright/request/assertion errors (which can include preview/session data).
          const descriptionStep = (control: string, operation: string, expected: string, perform: () => Promise<string | void>) =>
            step(frame, productRoute, control, operation, expected, async () => {
              try { return await perform(); }
              catch { throw new Error(`${control} failed at ${phase}; raw browser/request details withheld`); }
            });
          let setup = false;
          await descriptionStep("description setup", "GET existing product/session/cart + fresh product navigation", "real nonempty merchant description, active session, readable cart and a real available alternative SKU; setup failure is not a missing-entry RED", async () => {
            phase = "setup-product-get";
            const response = await context.request.get(`${origin}/api/shop/product/${candle.slug}`, { timeout: T });
            assert(response.ok(), "product read setup failed");
            phase = "setup-merchant-data";
            detail = await response.json();
            assert(detail && typeof detail.title === "string" && detail.title.trim().length > 0, "merchant title setup missing");
            assert(typeof detail.description === "string" && detail.description.trim().length > 0, "merchant description setup missing");
            phase = "setup-session-get";
            const sessionResponse = await context.request.get(`${origin}/api/buyer/session`, { timeout: T });
            assert(sessionResponse.ok(), "session read setup failed");
            phase = "setup-session-data";
            const session = await sessionResponse.json() as { state?: unknown; context?: unknown };
            assert(session.state === "active" && typeof session.context === "string" && session.context.length > 0, "active session setup missing");
            buyerContext = session.context;
            phase = "setup-cart-get";
            const cart = await readCart();
            assert(cart.ok(), "cart read setup failed");
            phase = "setup-cart-data";
            baseline = await cart.json();
            phase = "setup-navigation";
            await page.goto(productURL, { timeout: T });
            originalURL = page.url();
            assert(new URL(originalURL).pathname === productRoute, "product route setup failed");
            for (const [key, value] of params) assert(urlParams().get(key) === value, "preview setup failed");
            phase = "setup-base";
            const base = page.getByTestId("product-buy");
            await expect(base).toBeVisible({ timeout: T });
            const initial = await base.getAttribute("data-sku", { timeout: T });
            assert(detail.options.length > 0 && detail.variants.some((v) => v.sku_id === initial), "real option setup missing");
            phase = "setup-alternative";
            alternative = detail.variants.find((v) => ["in", "low"].includes(v.stock) && v.sku_id !== initial && v.option_values.length === detail!.options.length);
            assert(alternative, "real available alternative SKU setup missing");
            setup = true;
            return "existing real product/session/cart and alternative SKU preflight passed; no fixture writes";
          });
          if (!setup) {
            notRun.push({ frame, locale, subcase: "description entry/content/returns/draft/zero-write", reason: "description setup FAIL; missing product/session/cart/merchant description/alternative SKU is not causal missing-entry evidence" });
          } else {
            // Fresh observer and post-options-add baseline: never inherit the earlier legitimate add request or cart snapshot.
            const writes: { method: string; pathname: string }[] = [];
            const observeWrite = (request: import("@playwright/test").Request) => {
              const url = new URL(request.url());
              if (url.origin === new URL(origin).origin && !["GET", "HEAD", "OPTIONS"].includes(request.method())) {
                writes.push({ method: request.method(), pathname: url.pathname });
              }
            };
            const unchanged = async () => {
              phase = "url-preservation";
              assert(page.url() === originalURL, "description must preserve the complete original URL");
              assert(new URL(page.url()).pathname === productRoute, "description must preserve the locale product path");
              for (const [key, value] of params) assert(urlParams().get(key) === value, "description must preserve preview");
              phase = "zero-writes";
              assert.equal(writes.length, 0, "description and local draft actions must issue zero nonread requests");
              phase = "readback-get";
              const response = await readCart();
              assert(response.ok(), "description cart readback failed");
              phase = "readback-data";
              assert.deepEqual(await response.json(), baseline, "description must preserve the exact server cart");
            };
            const open = async () => {
              phase = "entry-click";
              await entry().click({ timeout: T });
              phase = "dialog-visible";
              await expect(task()).toBeVisible({ timeout: T });
              phase = "heading";
              await expect(task().getByRole("heading", { name: c.descriptionTitle, exact: true })).toBeVisible({ timeout: T });
              await unchanged();
            };
            const close = async (method: "header" | "dock" | "Escape") => {
              phase = method === "Escape" ? "escape-return" : method === "header" ? "header-return" : "dock-return";
              if (method === "Escape") await page.keyboard.press("Escape");
              else await task().getByTestId(`product-description-${method}`).getByRole("button", { name: c.specBack, exact: true }).click({ timeout: T });
              phase = "dialog-closed";
              await expect(task()).toHaveCount(0, { timeout: T });
              phase = "entry-focus";
              await expect(entry()).toBeFocused({ timeout: T });
              await unchanged();
            };
            let reachable = false;
            page.on("request", observeWrite);
            try {
              await descriptionStep("description task entry", "inspect exact entry + real click", `exact button "${c.descriptionEntry}" opens description on the same route; first causal product RED follows successful setup`, async () => {
                phase = "entry-visible";
                await expect(entry()).toBeVisible({ timeout: T });
                await open();
                reachable = true;
                return "real description entry reached its task; URL/preview/cart unchanged";
              });
              if (!reachable) {
                notRun.push({ frame, locale, subcase: "description content/layout/header/dock/Escape/draft/zero-write", reason: "description entry/open acceptance failed; dependent behavior NOT_RUN, see causal description task entry FAIL" });
              } else {
                await descriptionStep("description partial composition", "inspect visible merchant text + task bounds", "merchant title/description remain unchanged in each locale; task 390x844, header64 and dock84; no delivery promise asserted", async () => {
                  phase = "merchant-content";
                  assert(detail);
                  await expect(task().getByRole("heading", { name: detail.title, exact: true })).toBeVisible({ timeout: T });
                  // Playwright exact text normalizes whitespace, including the real merchant description's paragraph breaks.
                  await expect(task().getByText(detail.description, { exact: true })).toBeVisible({ timeout: T });
                  phase = "layout";
                  const body = await task().boundingBox({ timeout: T });
                  const header = await task().getByTestId("product-description-header").boundingBox({ timeout: T });
                  const dock = await task().getByTestId("product-description-dock").boundingBox({ timeout: T });
                  assert(body && header && dock, "description regions must be visible");
                  assert(Math.abs(body.x) <= 1 && Math.abs(body.y) <= 1 && Math.abs(body.width - 390) <= 1 && Math.abs(body.height - 844) <= 1, "description task fills viewport");
                  assert(Math.abs(header.y) <= 1 && Math.abs(header.height - 64) <= 1, "description header occupies y=0..64");
                  assert(Math.abs(dock.y - 760) <= 1 && Math.abs(dock.height - 84) <= 1, "description dock occupies y=760..844");
                  await unchanged();
                });
                await descriptionStep("description delivery section", "inspect task delivery section", `heading "${c.delivery}", generic methods note, pickup note and fee note "${c.deliveryFee}"; zero writes`, async () => {
                  phase = "delivery";
                  const delivery = task().getByTestId("product-description-delivery");
                  await expect(delivery.getByRole("heading", { name: c.delivery, exact: true })).toBeVisible({ timeout: T });
                  await expect(delivery.getByText(c.deliveryMethods, { exact: true })).toBeVisible();
                  await expect(delivery.getByText(c.specPickup, { exact: true })).toBeVisible();
                  await expect(delivery.getByText(c.deliveryFee, { exact: true })).toBeVisible();
                  const description = await task().getByText(detail!.description, { exact: true }).boundingBox();
                  const heading = await delivery.getByRole("heading", { name: c.delivery, exact: true }).boundingBox();
                  assert(description && heading && description.y + description.height <= heading.y, "delivery section must follow the merchant description");
                  await unchanged();
                });
                await shot(frame, productRoute);
                let returned = true;
                for (const method of ["header", "dock", "Escape"] as const) {
                  if (!returned) {
                    notRun.push({ frame, locale, subcase: `description ${method} return`, reason: "previous description return failed; dependent reopening NOT_RUN" });
                    continue;
                  }
                  returned = false;
                  await descriptionStep(`description ${method} return`, `real ${method} return + focus/readback`, "close restores invoking entry focus and exact locale/preview URL; zero writes and unchanged server cart", async () => {
                    if (method !== "header") await open();
                    await close(method);
                    returned = true;
                  });
                }
                if (!returned) {
                  notRun.push({ frame, locale, subcase: "description selected draft preservation", reason: "description return failed; options preparation and dependent description reopen NOT_RUN" });
                } else {
                  let draftReady = false;
                  await descriptionStep("description draft setup", "real options entry/radios/quantity/back clicks", "select a real alternative SKU and quantity2 through accepted options controls without adding", async () => {
                    phase = "setup-alternative";
                    assert(alternative);
                    phase = "options-entry";
                    await optionsEntry().click({ timeout: T });
                    phase = "options-heading";
                    await expect(options().getByRole("heading", { name: c.chooseSpec, exact: true })).toBeVisible({ timeout: T });
                    phase = "options-selection";
                    for (const value of alternative.option_values) {
                      const radio = options().getByRole("radio", { name: value, exact: true });
                      await radio.locator("..").click({ timeout: T });
                      await expect(radio).toBeChecked({ timeout: T });
                    }
                    await expect(options().getByTestId("product-buy")).toHaveAttribute("data-sku", alternative.sku_id, { timeout: T });
                    phase = "options-quantity";
                    await expect(options().getByTestId("qty-input")).toHaveValue("1", { timeout: T });
                    await options().getByRole("button", { name: c.increase, exact: true }).click({ timeout: T });
                    await expect(options().getByTestId("qty-input")).toHaveValue("2", { timeout: T });
                    phase = "options-return";
                    await options().getByRole("button", { name: c.specBack, exact: true }).click({ timeout: T });
                    phase = "options-closed";
                    await expect(options()).toHaveCount(0, { timeout: T });
                    phase = "options-focus";
                    await expect(optionsEntry()).toBeFocused({ timeout: T });
                    await unchanged();
                    draftReady = true;
                  });
                  for (const method of ["header", "dock", "Escape"] as const) {
                    if (!draftReady) {
                      notRun.push({ frame, locale, subcase: `description ${method} selected draft preservation`, reason: "real options draft preparation or prior preservation failed; dependent case NOT_RUN" });
                      continue;
                    }
                    draftReady = false;
                    await descriptionStep(`description ${method} selected draft preservation`, "open description + close + reopen options", "base and reopened options retain real chosen SKU, exact selected radios and quantity2; zero cart/session writes", async () => {
                      phase = "draft-base";
                      assert(alternative);
                      const base = page.getByTestId("product-buy");
                      await expect(base).toHaveAttribute("data-sku", alternative.sku_id, { timeout: T });
                      await expect(base.getByTestId("qty-input")).toHaveValue("2", { timeout: T });
                      await open();
                      await close(method);
                      phase = "draft-base";
                      await expect(base).toHaveAttribute("data-sku", alternative.sku_id, { timeout: T });
                      await expect(base.getByTestId("qty-input")).toHaveValue("2", { timeout: T });
                      phase = "options-entry";
                      await optionsEntry().click({ timeout: T });
                      phase = "draft-reopened";
                      await expect(options().getByTestId("product-buy")).toHaveAttribute("data-sku", alternative.sku_id, { timeout: T });
                      await expect(options().getByTestId("qty-input")).toHaveValue("2", { timeout: T });
                      for (const value of alternative.option_values) await expect(options().getByRole("radio", { name: value, exact: true })).toBeChecked({ timeout: T });
                      phase = "options-return";
                      await options().getByRole("button", { name: c.specBack, exact: true }).click({ timeout: T });
                      phase = "options-closed";
                      await expect(options()).toHaveCount(0, { timeout: T });
                      phase = "options-focus";
                      await expect(optionsEntry()).toBeFocused({ timeout: T });
                      await unchanged();
                      draftReady = true;
                    });
                  }
                }
              }
            } finally {
              page.off("request", observeWrite);
            }
          }
        }

        // ===== b-product single-SKU / one-axis frames (integrator ruling 2026-11-04): ONE buy card on every product (add-to-cart AND
        // buy-now), plus the frames' default-variant tag, quantity, note, description card, desktop back link, mobile header bar and mobile
        // bottom action. Demo products are a data gap: each frame pair maps to a real fixture product of the same shape (mapping table in
        // output/parity-buyer-browse/FRAME-TRIAGE-20261104.md). Runs after every cart baseline above; no later step reads this context's cart.
        const productMap: [string, string, string][] = [
          ["102:2163", "102:2232", "dripper"], ["104:2209", "104:2278", "item01"], ["104:2553", "104:2622", "item02"],
          ["104:3072", "104:3141", "item03"], ["104:3758", "104:3827", "item04"], ["104:3416", "104:3485", "scarf"],
        ];
        const backNav = () => page.getByTestId("product-back");
        const backLink = () => backNav().getByRole("link", { name: c.allProducts, exact: true });
        let productSigned = true;
        for (const [desktopFrame, mobileFrame, key] of productMap) {
          const frame = f(desktopFrame, mobileFrame);
          const item = (facts?.products ?? []).find((x: any) => x.key === key);
          const route = `/${locale}/products/${item?.slug ?? key}`;
          if (!productSigned) {
            notRun.push({ frame, locale, subcase: `${viewport} ${key} product page`, reason: "signed back link absent on the first mapped product; dependent product frames not run" });
            continue;
          }
          await page.goto(`${origin}/${locale}/search?q=${encodeURIComponent(item?.title ?? key)}`);
          let opened = false;
          await step(frame, route, `search result card "${item?.title ?? key}"`, "click", "the card opens this product's page", async () => {
            assert(item, `facts.products ${key} missing`);
            await page.getByTestId("product-card").filter({ hasText: item.title }).first().getByRole("link").click({ timeout: T });
            await expect(page).toHaveURL(new RegExp(`/${locale}/products/${item.slug}$`), { timeout: T });
            await expect(page.getByTestId("product-buy")).toBeVisible({ timeout: T });
            opened = true;
            return page.url();
          });
          if (!opened) {
            notRun.push({ frame, locale, subcase: `${viewport} ${key} product page`, reason: "product page not reached by the card click; dependent steps not run" });
            continue;
          }
          await settle();
          // The signed back link anchors this layout (bounded like the home/collection anchors): when the first mapped product lacks it,
          // every later mapped product is NOT_RUN instead of each waiting out its timeout.
          await step(frame, route, "back link", "assert", `"‹ ${c.allProducts}" link to the product list, above the product title`, async () => {
            await expect(backLink()).toBeVisible({ timeout: T });
            await expect(backLink()).toHaveAttribute("href", new RegExp(`/${locale}/products$`));
            const back = await box(backNav());
            const title = await box(page.getByRole("heading", { level: 1, name: item.title, exact: true }));
            assert(back.y + back.height <= title.y, `back nav ${back.y}+${back.height} must sit above the title ${title.y}`);
          });
          if (!(await backLink().isVisible())) {
            productSigned = false;
            notRun.push({ frame, locale, subcase: `${viewport} ${key} buy card, description card, header bar, bottom action, clicks`, reason: "signed back link absent; dependent steps not run" });
            continue;
          }
          const single = !item.axes?.length;
          const variant = item.variants.filter((v: any) => v.stock > 0).sort((a: any, b: any) => a.price_minor - b.price_minor)[0];
          const buy = page.getByTestId("product-buy");
          await step(frame, route, "one buy card", "assert", `title, price, ${single ? `tag "${c.standard}"` : "the selected option chip"}, "${c.quantity}" 1, note "${c.buyNote}", then "${c.addToCart}" and "${c.buyNow}", both enabled`, async () => {
            await expect(page.getByRole("heading", { level: 1, name: item.title, exact: true })).toBeVisible({ timeout: T });
            const shown = num(await buy.getByTestId("variant-price").innerText({ timeout: T }));
            assert.equal(shown, variant.price_minor / 100, "displayed price must be the server variant price");
            const pick = single ? buy.getByTestId("default-variant-tag") : buy.getByRole("radio", { name: variant.values[0], exact: true });
            if (single) await expect(pick).toHaveText(c.standard, { timeout: T });
            else {
              await expect(pick).toBeChecked({ timeout: T });
              await expect(buy.getByTestId("default-variant-tag")).toHaveCount(0);
            }
            await expect(buy.getByText(c.quantity, { exact: true })).toBeVisible();
            await expect(buy.getByTestId("qty-input")).toHaveValue("1");
            await expect(buy.getByTestId("product-buy-note")).toHaveText(c.buyNote);
            for (const [id, label] of [["add-to-cart", c.addToCart], ["buy-now", c.buyNow]]) {
              await expect(buy.getByTestId(id)).toHaveText(label);
              await expect(buy.getByTestId(id)).toBeEnabled();
            }
            const order = [await box(pick), await box(buy.getByTestId("qty-input")), await box(buy.getByTestId("product-buy-note")), await box(buy.getByTestId("add-to-cart")), await box(buy.getByTestId("buy-now"))];
            assert(order.every((b, i) => i === 0 || order[i - 1].y < b.y), `buy card order must be tag < quantity < note < add < buy-now, got ${order.map((b) => Math.round(b.y)).join(", ")}`);
            return `price ${shown}; y ${order.map((b) => Math.round(b.y)).join(" < ")}`;
          });
          await step(frame, route, "description card", "assert", `card "${c.descriptionTitle}" with the merchant description, below the ${isMobile ? "buy card" : "gallery"}`, async () => {
            const card = page.getByTestId("product-description-card");
            await expect(card.getByRole("heading", { name: c.descriptionTitle, exact: true })).toBeVisible({ timeout: T });
            await expect(card.getByText(item.description, { exact: true })).toBeVisible();
            const above = await box(isMobile ? buy.getByTestId("buy-now") : page.getByTestId("product-gallery").first());
            const own = await box(card);
            assert(own.y >= above.y + above.height, `description card ${own.y} must sit below ${above.y + above.height}`);
          });
          await step(frame, route, isMobile ? "mobile header bar" : "desktop back link shape", "assert", isMobile ? `full-width bar: 44px back control and the title "${item.title}"` : `visible text "${c.allProducts}", no bar title`, async () => {
            const link = await box(backLink());
            const title = backNav().getByTestId("product-back-title");
            if (isMobile) {
              await expect(title).toHaveText(item.title, { timeout: T });
              const bar = await box(backNav());
              assert(link.width >= 44 && link.height >= 44, `back control ${link.width}x${link.height} below 44px`);
              assert(Math.abs(bar.x) <= 1 && Math.abs(bar.width - width) <= 1, `bar must span the viewport, got x=${bar.x} w=${bar.width}`);
            } else {
              await expect(backLink()).toContainText(c.allProducts);
              await expect(title).toBeHidden();
            }
          });
          await shot(frame, route);
          const add = () => (isMobile && single ? page.getByTestId("product-dock-add").getByRole("button", { name: c.addToCart, exact: true }) : buy.getByTestId("add-to-cart"));
          if (isMobile) await step(frame, route, "mobile bottom action", "assert", single ? `fixed bottom "${c.addToCart}"` : `fixed bottom "${c.chooseSpec}" (master 4:9532 dock wins, P2 #546)`, async () => {
            const action = single ? add() : page.getByTestId("product-options-dock-entry").getByRole("button", { name: c.chooseSpec, exact: true });
            await expect(action).toBeVisible({ timeout: T });
            const b = await box(action);
            assert(b.y + b.height <= height + 1 && b.y >= height - 84, `bottom action must sit in the bottom 84px, got y=${b.y} h=${b.height}`);
          });
          await step(frame, route, isMobile && single ? "bottom add-to-cart" : "add-to-cart button", "click + Escape", "the cart drawer opens with its checkout action; Escape closes it", async () => {
            await add().click({ timeout: T });
            await expect(page.getByTestId("cart-checkout")).toBeVisible({ timeout: T });
            await page.keyboard.press("Escape");
            await expect(page.getByTestId("cart-drawer")).not.toBeVisible({ timeout: T });
          });
          await step(frame, route, isMobile ? "header back control" : `link "‹ ${c.allProducts}"`, "click", "opens the product list", async () => {
            await backLink().click({ timeout: T });
            await expect(page).toHaveURL(new RegExp(`/${locale}/products/?$`), { timeout: T });
            await expect(page.getByRole("heading", { level: 1, name: c.allProducts, exact: true })).toBeVisible({ timeout: T });
            return page.url();
          });
        }

        // ===== Removed / unknown product ===============================================================================================
        const removedFrame = f("1253:17090", "1253:17114");
        const removedRoute = `/${locale}/products/non-existent-product-sku`;
        await page.goto(`${origin}${removedRoute}`);
        await step(removedFrame, removedRoute, "not-found heading", "goto + assert", `heading "${c.productRemoved}" visible`, async () => {
          await expect(page.getByRole("heading", { name: c.productRemoved })).toBeVisible({ timeout: T });
        });
        await shot(removedFrame, removedRoute);

        // ===== b-cart entry frames (round b): the cart entered from a product page by a real add-to-cart click, then 「查看購物車」.
        // Frame -> fixture mapping as the b-product round (FRAME-TRIAGE-20261104.md). Own context = fresh cart, so the 4:9668/4:9746
        // one-line baseline above is untouched; the cart grows by one line per product. The entry panel anchors the layout (bounded
        // like the b-product back link): when the first product lacks it, the rest are NOT_RUN instead of each waiting out T.
        const entryMap: [string, string, string][] = [
          ["102:2267", "102:2324", "dripper"], ["104:2313", "104:2371", "item01"], ["104:2657", "104:2715", "item02"],
          ["104:3176", "104:3234", "item03"], ["104:3520", "104:3578", "scarf"], ["104:3862", "104:3920", "item04"],
        ];
        const entryContext = await browser.newContext(ctxOpts({ ignoreHTTPSErrors: true, viewport: { width, height }, locale }));
        try {
          page = await entryContext.newPage();
          let entrySigned = true;
          let lines = 0;
          for (const [i, [desktopFrame, mobileFrame, key]] of entryMap.entries()) {
            const frame = f(desktopFrame, mobileFrame);
            const item = (facts?.products ?? []).find((x: any) => x.key === key);
            if (!entrySigned) {
              notRun.push({ frame, locale, subcase: `${viewport} ${key} cart entry`, reason: "cart entry panel absent on the first mapped product; dependent cart entry frames not run" });
              continue;
            }
            const productRoute = `/${locale}/products/${item?.slug ?? key}`;
            await page.goto(`${origin}${productRoute}`);
            let inCart = false;
            await step(frame, productRoute, `"${c.addToCart}" then drawer link "${c.viewCart}"`, "click + click", "the add lands in the cart and the drawer link opens the cart page", async () => {
              assert(item, `facts.products ${key} missing`);
              const single = !item.axes?.length;
              const add = isMobile && single ? page.getByTestId("product-dock-add").getByRole("button", { name: c.addToCart, exact: true }) : page.getByTestId("product-buy").getByTestId("add-to-cart");
              await add.click({ timeout: T });
              await page.getByTestId("cart-drawer").getByRole("link", { name: c.viewCart, exact: true }).click({ timeout: T });
              await expect(page).toHaveURL(new RegExp(`/${locale}/cart$`), { timeout: T });
              lines += 1;
              await expect(page.getByTestId("cart-page").getByTestId("cart-line")).toHaveCount(lines, { timeout: T });
              inCart = true;
              return page.url();
            });
            if (!inCart) {
              notRun.push({ frame, locale, subcase: `${viewport} ${key} cart entry panel, back, remove`, reason: "add-to-cart / view-cart did not reach the cart page; dependent steps not run" });
              continue;
            }
            const variant = item.variants.filter((v: any) => v.stock > 0).sort((a: any, b: any) => a.price_minor - b.price_minor)[0];
            const entry = page.getByTestId("cart-entry");
            const backLink = () => entry.getByRole("link", { name: c.entryBack });
            await step(frame, cartRoute, "cart entry panel", "assert", `"‹ ${c.entryBack}", heading "${fmt(c.entryTitle, { title: item.title })}", the added line (${c.entrySpec}, ${c.quantity} 1, ${c.entrySubtotal}, "${c.entryRemove}"), note "${c.entryShip}", "${c.checkout}", then the cart list`, async () => {
              await expect(entry.getByRole("heading", { name: fmt(c.entryTitle, { title: item.title }), exact: true })).toBeVisible({ timeout: T });
              await expect(backLink()).toHaveAttribute("href", new RegExp(`/${locale}/products/${item.slug}$`));
              const line = entry.getByTestId("cart-entry-line");
              await expect(line.getByText(item.title, { exact: true })).toBeVisible();
              await expect(line.getByTestId("cart-entry-spec")).toContainText(c.entrySpec);
              await expect(line.getByTestId("cart-entry-spec")).toContainText(item.axes?.length ? variant.values[0] : c.standard);
              await expect(line.getByTestId("cart-entry-qty")).toContainText(c.quantity);
              await expect(line.getByTestId("cart-entry-qty")).toContainText("1");
              await expect(line.getByTestId("cart-entry-subtotal")).toContainText(c.entrySubtotal);
              assert.equal(num(await line.getByTestId("cart-entry-subtotal").innerText()), variant.price_minor / 100, "entry subtotal must be the server variant price x 1");
              await expect(line.getByRole("button", { name: c.entryRemove, exact: true })).toBeEnabled();
              await expect(entry.getByText(c.entryShip, { exact: true })).toBeVisible();
              await expect(entry.getByRole("link", { name: c.checkout })).toHaveAttribute("href", new RegExp(`/${locale}/checkout$`));
              const order = [await box(backLink()), await box(line), await box(entry.getByRole("link", { name: c.checkout })), await box(page.getByTestId("cart-page"))];
              assert(order.every((b, n) => n === 0 || order[n - 1].y < b.y), `entry order must be back < line < checkout < cart list, got ${order.map((b) => Math.round(b.y)).join(", ")}`);
              return `subtotal ${variant.price_minor / 100}; y ${order.map((b) => Math.round(b.y)).join(" < ")}`;
            });
            if (!(await entry.isVisible())) {
              entrySigned = false;
              notRun.push({ frame, locale, subcase: `${viewport} ${key} cart entry back, remove, reload`, reason: "cart entry panel absent; dependent steps not run" });
              continue;
            }
            await shot(frame, cartRoute);
            if (i < entryMap.length - 1) {
              await step(frame, cartRoute, `link "‹ ${c.entryBack}"`, "click", "opens the product the cart was entered from", async () => {
                await backLink().click({ timeout: T });
                await expect(page).toHaveURL(new RegExp(`/${locale}/products/${item.slug}$`), { timeout: T });
                await expect(page.getByRole("heading", { level: 1, name: item.title, exact: true })).toBeVisible({ timeout: T });
              });
              continue;
            }
            await step(frame, cartRoute, `button "${c.entryRemove}" + reload`, "click + reload", "the entered line leaves the cart list and stays removed after refresh", async () => {
              // facts variants carry no sku_id: the listed line is the one titled with this product (one line per product in this cart).
              const listed = page.getByTestId("cart-page").getByTestId("cart-line").filter({ hasText: item.title });
              await expect(listed).toHaveCount(1, { timeout: T });
              await entry.getByTestId("cart-entry-line").getByRole("button", { name: c.entryRemove, exact: true }).click({ timeout: T });
              await expect(listed).toHaveCount(0, { timeout: T });
              await expect(page.getByTestId("cart-page").getByTestId("cart-line")).toHaveCount(lines - 1, { timeout: T });
              await page.reload();
              await expect(page.getByTestId("cart-page").getByTestId("cart-line")).toHaveCount(lines - 1, { timeout: T });
              await expect(listed).toHaveCount(0);
            });
          }
        } finally {
          await entryContext.close();
        }
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
    if (frame.frameId === "51:1380") {
      const required = ["spec task entry", "spec dock entry", "spec composition", "spec option SKU price image", "spec quantity bounds", "spec return draft", "spec entry return cart unchanged", "spec one add CAS and refreshed cart"];
      const complete = Object.keys(copy).every((locale) => required.every((control) => rows.some((row) => row.locale === locale && row.viewport === "390x844" && row.control === control && row.status === "PASS")));
      const screenshot = rows.some((row) => row.locale === "zh-TW" && row.control === "parity-screenshot" && row.status === "PASS");
      if (!complete || !screenshot) return { ...frame, status: "NOT_COVERED", reason: failedRows.length ? `assertion failed: ${failedRows[0].error}` : "complete mobile option plan and actual parity screenshot not executed in all three locales" };
    }
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
  await writeFile(path.join(evidence, "parity-buyer-browse-not-run.json"), JSON.stringify(notRun, null, 2));

  const count = (status: ManifestEntry["status"]) => manifest.filter((entry) => entry.status === status).length;
  const summary = `COVERED ${count("COVERED")} · NOT_COVERED ${count("NOT_COVERED")} · DEFERRED ${count("DEFERRED")} · BLOCKED ${count("BLOCKED")} of ${manifest.length} frames; ${ledger.filter((row) => row.status === "PASS").length}/${ledger.length} ledger operations passed`;
  assert.deepEqual(failures, [], `PARITY-BUYER-BROWSE: ${failures.length} assertion(s) failed (${summary})`);
  console.log(`PARITY-BUYER-BROWSE: ${summary}`);
}
