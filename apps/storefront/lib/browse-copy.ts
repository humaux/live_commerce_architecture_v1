// Purpose: localized buyer-browsing labels and mobile product-option task copy.
// Depends on: shared Locale identifiers; contains no merchant facts or transport state.
// Used by: home, product, search and collection surfaces, including MobileProductOptions and components/browse/**.
import type { Locale } from "@live-commerce/i18n";

const en = {
  productDescription: { entry: "Product description & delivery" },
  browse: { welcome: "Welcome to {name}", searchLabel: "Search products", searchHint: "Search by product name or keyword", allProducts: "All products", allCollections: "All collections", viewAllProducts: "View all products", collectionCount: "Products in this collection · {n}" },
  productOptions: { choose: "Choose options", back: "Back to product", limit: "Purchase quantity is limited by product stock.", pickup: "For convenience-store pickup, choose a chain at checkout and enter the store number (name and address are optional)." },
  related: "In the same collection", delivery: "Delivery", payment: "Payment", returns: "Returns", refunds: "Refunds",
  optionsNote: "Available methods depend on your destination and are confirmed at checkout.",
  zoom: "Enlarge photo", close: "Close photo", previous: "Previous photo", next: "Next photo",
  fraud: "Shop safely", fraudHint: "Check the order before transferring. Never share passwords or verification codes.",
};
export const browseCopy: Record<Locale, typeof en> = {
  en,
  "zh-TW": {
    productDescription: { entry: "商品說明與配送" },
    browse: { welcome: "歡迎來到{name}", searchLabel: "搜尋商品", searchHint: "搜尋商品名稱或關鍵字", allProducts: "全部商品", allCollections: "所有系列", viewAllProducts: "查看所有商品", collectionCount: "系列商品 · {n} 件" },
    productOptions: { choose: "選擇規格", back: "返回商品", limit: "每次購買數量依商品庫存限制。", pickup: "門市取貨：結帳時選擇超商品牌，填寫門市號碼（店名、地址可不填）。" },
    related: "同系列商品", delivery: "配送方式", payment: "付款方式", returns: "退貨說明", refunds: "退款說明",
    optionsNote: "可用方式依收貨地點而定，請以結帳時確認的選項為準。",
    zoom: "放大圖片", close: "關閉圖片", previous: "上一張圖片", next: "下一張圖片",
    fraud: "防詐騙提醒", fraudHint: "轉帳前請核對訂單，勿提供密碼或驗證碼。",
  },
  "zh-CN": {
    productDescription: { entry: "商品说明与配送" },
    browse: { welcome: "欢迎来到{name}", searchLabel: "搜索商品", searchHint: "搜索商品名称或关键词", allProducts: "全部商品", allCollections: "所有系列", viewAllProducts: "查看所有商品", collectionCount: "系列商品 · {n} 件" },
    productOptions: { choose: "选择规格", back: "返回商品", limit: "每次购买数量依商品库存限制。", pickup: "门店取货：结账时选择便利店品牌，填写门店号码（店名、地址可不填）。" },
    related: "同系列商品", delivery: "配送方式", payment: "付款方式", returns: "退货说明", refunds: "退款说明",
    optionsNote: "可用方式依收货地点而定，请以结账时确认的选项为准。",
    zoom: "放大图片", close: "关闭图片", previous: "上一张图片", next: "下一张图片",
    fraud: "防诈骗提醒", fraudHint: "转账前请核对订单，勿提供密码或验证码。",
  },
};
