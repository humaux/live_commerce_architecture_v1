// Purpose: buyer checkout v2 messages in every supported storefront locale.
// Depends on: shared Locale; no product, payment or monetary facts.
// Used by: CheckoutFlow and checkout/CheckoutReview.
// Invariants: translation keys match; timing describes the selected existing payment method only.
import type { Locale } from "./index";

const en = {
  review: "Review your order", subtotal: "Product subtotal", shipping: "Delivery", fee: "Cash-on-delivery fee",
  total: "Final amount due", destination: "Deliver to", recipient: "Recipient", payment: "Payment method",
  online: "Pay online after placing the order", cod: "Cash on delivery", pickup: "Pay at pickup",
  changed: "Total updated. Please confirm again.", confirm: "Confirm new total",
  unknown: "Checking order status", query: "Check order status", submit: "Place order",
  open: "Expand payment help", close: "Collapse payment help",
  back: "Back", details: "View details", helpName: "Payment timing help", helpMark: "?", submitting: "Submitting order", phone: "Phone", homeDelivery: "Home delivery",
  onlineHelp: "After placing the order, follow the payment instructions on the order page.",
  codHelp: "Pay the delivery driver in cash when the parcel arrives. The site does not charge you beforehand.",
  pickupHelp: "Pay when collecting at your selected store. No online charge.",
  help: "Check the amount and delivery details before placing your order. Payment is due at the time shown for your selected method.",
};
export const checkoutCopy: Record<Locale, typeof en> = {
  en,
  "zh-TW": {
    review: "核對訂單", subtotal: "商品小計", shipping: "運費", fee: "貨到付款手續費", total: "最終應付",
    destination: "配送至", recipient: "收件人", payment: "付款方式", online: "送出後線上付款",
    cod: "貨到付款", pickup: "門市取貨付款", changed: "金額已更新，請再確認", confirm: "確認新金額",
    unknown: "確認訂單狀態中", query: "查詢訂單狀態", submit: "送出訂單", open: "展開付款說明", close: "收合付款說明",
    back: "返回", details: "查看明細", helpName: "付款時機說明", helpMark: "?", submitting: "正在送出訂單", phone: "電話", homeDelivery: "宅配",
    onlineHelp: "送出訂單後，依訂單頁指示完成付款。",
    codHelp: "包裹送達時付現金給物流人員，網站不會先扣款。",
    pickupHelp: "到選定門市取貨時付款，不會線上扣款。",
    help: "送出前請確認金額與配送資料，並依所選方式顯示的時間付款。",
  },
  "zh-CN": {
    review: "核对订单", subtotal: "商品小计", shipping: "运费", fee: "货到付款手续费", total: "最终应付",
    destination: "配送地址", recipient: "收货人", payment: "付款方式", online: "提交后在线付款",
    cod: "货到付款", pickup: "门店取货付款", changed: "金额已更新，请重新确认", confirm: "确认新金额",
    unknown: "正在确认订单状态", query: "查询订单状态", submit: "提交订单", open: "展开付款说明", close: "收起付款说明",
    back: "返回", details: "查看明细", helpName: "付款时间说明", helpMark: "?", submitting: "正在提交订单", phone: "电话", homeDelivery: "宅配",
    onlineHelp: "提交订单后，按订单页提示完成付款。",
    codHelp: "包裹送达时付现金给物流人员，网站不会先扣款。",
    pickupHelp: "到所选门店取货时付款，不会线上扣款。",
    help: "提交前请核对金额和收货信息，并在所选付款方式提示的时间付款。",
  },
};
