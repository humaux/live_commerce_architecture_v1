import type { Locale } from "@live-commerce/i18n";

const en = {
  related: "In the same collection", delivery: "Delivery", payment: "Payment", returns: "Returns", refunds: "Refunds",
  optionsNote: "Available methods depend on your destination and are confirmed at checkout.",
  zoom: "Enlarge photo", close: "Close photo", previous: "Previous photo", next: "Next photo",
  fraud: "Shop safely", fraudHint: "Check the order before transferring. Never share passwords or verification codes.",
};
export const browseCopy: Record<Locale, typeof en> = {
  en,
  "zh-TW": {
    related: "同系列商品", delivery: "配送方式", payment: "付款方式", returns: "退貨說明", refunds: "退款說明",
    optionsNote: "可用方式依收貨地點而定，請以結帳時確認的選項為準。",
    zoom: "放大圖片", close: "關閉圖片", previous: "上一張圖片", next: "下一張圖片",
    fraud: "防詐騙提醒", fraudHint: "轉帳前請核對訂單，勿提供密碼或驗證碼。",
  },
  "zh-CN": {
    related: "同系列商品", delivery: "配送方式", payment: "付款方式", returns: "退货说明", refunds: "退款说明",
    optionsNote: "可用方式依收货地点而定，请以结账时确认的选项为准。",
    zoom: "放大图片", close: "关闭图片", previous: "上一张图片", next: "下一张图片",
    fraud: "防诈骗提醒", fraudHint: "转账前请核对订单，勿提供密码或验证码。",
  },
};
