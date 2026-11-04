// Purpose: typed three-locale copy of the signed 訂單缺少收件資料 frame (750:3454 / 750:3596); zh-TW is the signed text verbatim.
// Depends on: Locale from @live-commerce/i18n.
// Used by: ManualOrderMissing.
import type { Locale } from "@live-commerce/i18n";
export const manualOrderMissingCopy = {
  "zh-TW": {
    title: "請補齊收件資料",
    lead: "訂單尚未建立，也尚未收款。原商品與數量仍在草稿中。",
    pending: "待補資料",
    phone: "聯絡電話",
    phoneHint: "請填寫可聯絡買家的電話。",
    store: "超商品牌與門市號碼",
    storeHint: "請確認超商品牌與門市號碼；店名、地址可不填。",
    note: "資料完整後才能取得正式報價；付款狀態會依實際收款結果更新。",
    back: "返回草稿補填",
  },
  "zh-CN": {
    title: "请补齐收件信息",
    lead: "订单尚未创建，也尚未收款。原商品与数量仍在草稿中。",
    pending: "待补信息",
    phone: "联系电话",
    phoneHint: "请填写可联系买家的电话。",
    store: "便利店品牌与门店编号",
    storeHint: "请确认便利店品牌与门店编号；店名、地址可不填。",
    note: "信息完整后才能取得正式报价；付款状态会依实际收款结果更新。",
    back: "返回草稿补填",
  },
  en: {
    title: "Complete the recipient details",
    lead: "No order has been created and no payment has been taken. The items and quantities are still in the draft.",
    pending: "Missing details",
    phone: "Contact phone",
    phoneHint: "Enter a phone number the buyer can be reached on.",
    store: "Convenience store brand and store number",
    storeHint: "Confirm the convenience store brand and store number; the store name and address are optional.",
    note: "The final quote is available once the details are complete; the payment status follows the actual payment.",
    back: "Back to the draft",
  },
} satisfies Record<
  Locale,
  { title: string; lead: string; pending: string; phone: string; phoneHint: string; store: string; storeHint: string; note: string; back: string }
>;
