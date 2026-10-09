// Purpose: three-locale claim preview, direct checkout and recovery messages.
// Depends on: @live-commerce/i18n locale type and the Direct checkout product contract.
// Used by: ClaimLink and CheckoutFlow.
// Owns every visible string of the buyer claim page (/{locale}/claim) in zh-CN, zh-TW
// and en, including the contract's 404 and 409 copy (live-keyword-claims-v1 §11.1).
// Non-goals: no money/time formatting (the page uses Intl), no claim rule.
// Depends on: @live-commerce/i18n (Locale) only.

import type { Locale } from "@live-commerce/i18n";

const en = {
  store: "Storefront", language: "Language", demonstration: "Synthetic data · test environment",
  title: "Your claimed items", loading: "Opening your claim…",
  intro: "These items were recorded from your live comments. The SKU and quantity are already chosen. Continue to checkout.",
  price: "Current price, final at checkout", stock: "Claims do not reserve stock. Stock is confirmed at checkout.",
  quantity: "Quantity", keyword: "Code", inCart: "Already in your cart", unavailable: "Not available right now",
  add: "Check out now", adding: "Opening checkout…", addAgain: "Check out now",
  soldOut: "Sold out for this quantity", partial: (n: number) => `Check out (${n} available items)`,
  recovery: "An order is being processed. Finish it first, then reopen the seller’s message link.",
  checkoutNotice: "Your live-claimed items are ready. Other items in your cart will be checked out together. Use ‘Back to cart’ below to remove them.",
  mergeNotice: "Other items in your cart will also be checked out. You can remove them below.",
  previousOrder: "Your previous order stays saved. Its items will not be added again; any unpaid order still needs separate attention.",
  reopen: "Reopen the link in the seller’s message to view these items again, or continue from your cart.",
  cartLink: "View cart",
  expires: (time: string) => `This link works until ${time}.`,
  taipeiTime: "Taipei time (UTC+8)", // same label as purchase-copy (checkout/order expiry)
  added: "Added to your cart.", nothing: "Your cart already has these items.",
  skipped: "Some items were not added because they are not available right now. They stay on this link.",
  notFound: "This link expired or was replaced. Message the seller for a new link.",
  conflict: "Your cart has an item that is no longer available, or the claim changed. Review your cart, then try again.",
  reviewCart: "Review your cart", reload: "Reload claim",
  failed: "We could not confirm the result. Reload the claim before trying again.",
  session: "Your shopping connection timed out. Reconnect to continue.", renew: "Reconnect",
  cart: "Your cart", emptyCart: "Your cart is empty.", otherItem: "Other item",
  remove: "Remove", cartFailed: "Your cart could not be updated. Reload the claim and try again.",
};
export type ClaimCopy = typeof en;

export const claimCopy: Record<Locale, ClaimCopy> = {
  en,
  "zh-CN": {
    store: "店铺前台", language: "语言", demonstration: "合成数据 · 测试环境",
    title: "你登记的商品", loading: "正在打开登记…",
    intro: "这些商品来自你在直播中的评论。商品和数量已选好，可直接结账。",
    price: "当前价格，以结账时为准", stock: "登记不会保留库存，结账时才确认库存。",
    quantity: "数量", keyword: "口令", inCart: "已在购物车中", unavailable: "目前无法购买",
    add: "直接结账", adding: "正在前往结账…", addAgain: "直接结账",
    soldOut: "所需数量已售罄", partial: (n: number) => `结账（${n} 件可买）`,
    recovery: "你有一笔订单正在处理，请先完成；之后再点私信中的链接。",
    checkoutNotice: "直播登记的商品已放入；购物车里其他商品也会一起结账。可点击下方“返回购物车”移除其他商品。",
    mergeNotice: "购物车里的其他商品也会一起结账，可在下方移除。",
    previousOrder: "上一笔订单会保留，这次不会重复加入那笔订单的商品；未付款订单仍需另行处理。",
    reopen: "请重新打开商家私信中的链接查看这些商品，或从购物车继续。",
    cartLink: "查看购物车",
    expires: (time: string) => `此链接有效至 ${time}。`,
    taipeiTime: "台北时间（UTC+8）", // same label as purchase-copy (checkout/order expiry)
    added: "已加入购物车。", nothing: "购物车里已有这些商品。",
    skipped: "部分商品目前无法购买，未加入购物车，仍保留在此链接中。",
    notFound: "链接已失效或已更换，请私信商家重新取得",
    conflict: "你的购物车中有商品已无法购买，或登记内容已变更。请先检查购物车，然后再试一次。",
    reviewCart: "检查购物车", reload: "重新读取登记",
    failed: "无法确认结果。请先重新读取登记，再试一次。",
    session: "购物连接已过期，需要重新连接。", renew: "重新连接",
    cart: "你的购物车", emptyCart: "购物车是空的。", otherItem: "其他商品",
    remove: "移除", cartFailed: "无法更新购物车。请重新读取登记后再试。",
  },
  "zh-TW": {
    store: "商店前台", language: "語言", demonstration: "合成資料 · 測試環境",
    title: "你登記的商品", loading: "正在開啟登記…",
    intro: "這些商品來自你在直播中的留言。商品與數量已選好，可直接結帳。",
    price: "目前價格，以結帳時為準", stock: "登記不會保留庫存，結帳時才確認庫存。",
    quantity: "數量", keyword: "關鍵字", inCart: "已在購物車中", unavailable: "目前無法購買",
    add: "直接結帳", adding: "正在前往結帳…", addAgain: "直接結帳",
    soldOut: "所需數量已售完", partial: (n: number) => `結帳（${n} 件可買）`,
    recovery: "你有一筆訂單正在處理，請先完成；之後再點私訊中的連結。",
    checkoutNotice: "直播登記的商品已放入；購物車裡其他商品也會一起結帳。可點擊下方「返回購物車」移除其他商品。",
    mergeNotice: "購物車裡的其他商品也會一起結帳，可在下方移除。",
    previousOrder: "上一筆訂單會保留，這次不會重複加入那筆訂單的商品；未付款訂單仍需另行處理。",
    reopen: "請重新開啟商家私訊中的連結查看這些商品，或從購物車繼續。",
    cartLink: "查看購物車",
    expires: (time: string) => `此連結有效至 ${time}。`,
    taipeiTime: "台北時間（UTC+8）", // same label as purchase-copy (checkout/order expiry)
    added: "已加入購物車。", nothing: "購物車裡已有這些商品。",
    skipped: "部分商品目前無法購買，未加入購物車，仍保留在此連結中。",
    notFound: "連結已失效或已更換，請私訊商家重新取得",
    conflict: "你的購物車中有商品已無法購買，或登記內容已變更。請先檢查購物車，然後再試一次。",
    reviewCart: "檢查購物車", reload: "重新讀取登記",
    failed: "無法確認結果。請先重新讀取登記，再試一次。",
    session: "購物連線已過期，需要重新連線。", renew: "重新連線",
    cart: "你的購物車", emptyCart: "購物車是空的。", otherItem: "其他商品",
    remove: "移除", cartFailed: "無法更新購物車。請重新讀取登記後再試。",
  },
};
