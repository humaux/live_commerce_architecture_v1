// Purpose: buyer checkout v2 messages in every supported storefront locale.
// Depends on: packages/i18n Locale; no product, payment or monetary facts.
// Used by: CheckoutFlow and checkout/CheckoutReview.
// Invariants: translation keys match; timing describes the selected existing payment method only.
import type { Locale } from "@live-commerce/i18n";

const en = {
  review: "Review your order", subtotal: "Product subtotal", shipping: "Delivery", fee: "Cash-on-delivery fee",
  total: "Final total", destination: "Deliver to", recipient: "Recipient", payment: "Payment method",
  online: "Pay after placing your order", cod: "Cash on delivery: pay when your parcel arrives", pickup: "Pay at store pickup",
  changed: "Total updated. Please confirm again.", confirm: "Confirm updated total",
  unknown: "Checking your order status", query: "Check order status", submit: "Place order",
  open: "Expand payment help", close: "Collapse payment help",
  help: "Check the amount and delivery details before placing your order. Payment is due at the time shown for your selected method.",
};
export const checkoutCopy: Record<Locale, typeof en> = {
  en,
  "zh-TW": {
    review: "確認訂單", subtotal: "商品小計", shipping: "運費", fee: "貨到付款手續費", total: "最終應付",
    destination: "配送至", recipient: "收件人", payment: "付款方式", online: "送出後付款",
    cod: "貨到付款：到貨時付款", pickup: "門市取貨付款", changed: "金額已更新，請再確認", confirm: "確認更新後的金額",
    unknown: "確認訂單狀態中", query: "查詢訂單狀態", submit: "送出訂單", open: "展開付款說明", close: "收合付款說明",
    help: "送出前請確認金額與配送資料，並依所選方式顯示的時間付款。",
  },
  "zh-CN": {
    review: "确认订单", subtotal: "商品小计", shipping: "运费", fee: "货到付款手续费", total: "应付总额",
    destination: "配送地址", recipient: "收货人", payment: "付款方式", online: "提交订单后付款",
    cod: "货到付款：送达时付款", pickup: "到店取货时付款", changed: "金额已更新，请重新确认", confirm: "确认更新后的金额",
    unknown: "正在查询订单状态", query: "查询订单状态", submit: "提交订单", open: "展开付款说明", close: "收起付款说明",
    help: "提交前请核对金额和收货信息，并在所选付款方式提示的时间付款。",
  },
};
