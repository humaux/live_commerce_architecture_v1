// Copy (zh-CN / zh-TW / en) of the guest order lookup page (contracts/storefront-v2.md §E5), rendered by
// app/[locale]/orders/lookup/LookupForm.tsx and the order page shell. One refusal text covers every mismatch on purpose: the page must not say
// whether the order number or the contact was wrong.
// Signed frames: 26:708 / 26:995 (default state) and 1253:17225 / 1253:17256 (not-found state, typed values kept).
import type { Locale } from "@live-commerce/i18n";

export type LookupText = {
  title: string; intro: string; orderRef: string; orderRefHint: string; contact: string; contactHint: string; submit: string; submitting: string;
  noMatch: string; invalid: string; tooMany: string; unavailable: string; sessionNote: string; orderTitle: string; loading: string;
  loadFailed: string; retry: string; lookupLink: string;
  // default state: card heading, intro, foot note; not-found state: button label and the "values kept" line.
  heading: string; introDefault: string; submitMiss: string; footNote: string; kept: string;
};

export const lookupCopy: Record<Locale, LookupText> = {
  "zh-TW": {
    title: "查詢訂單",
    intro: "輸入訂單通知信中的訂單編號，以及下單時使用的電子郵件或收件人手機號碼。",
    orderRef: "訂單編號",
    orderRefHint: "格式如 0123-ABCD-4567",
    contact: "電子郵件或手機號碼",
    contactHint: "與下單時填寫的相同",
    submit: "查詢訂單",
    submitting: "查詢中…",
    noMatch: "找不到符合的訂單，請確認訂單編號與電子郵件或手機號碼。",
    invalid: "請檢查訂單編號與聯絡方式的格式。",
    tooMany: "嘗試次數過多，請稍後再試。",
    unavailable: "暫時無法查詢，請稍後再試。",
    sessionNote: "查詢成功後，此瀏覽器目前的購物內容會被取代，只能查看這筆訂單。",
    orderTitle: "您的訂單",
    loading: "載入中…",
    loadFailed: "無法載入訂單。",
    retry: "重試",
    lookupLink: "改用訂單編號查詢",
    heading: "尋找你的訂單",
    introDefault: "請輸入這家商店的訂單編號及下單時填寫的聯絡資料。",
    submitMiss: "查詢",
    footNote: "找不到訂單時，請核對商店、訂單編號與聯絡資料。",
    kept: "你輸入的內容已保留，可直接修改後再查詢。",
  },
  "zh-CN": {
    title: "查询订单",
    intro: "输入订单通知邮件中的订单编号，以及下单时使用的邮箱或收件人手机号码。",
    orderRef: "订单编号",
    orderRefHint: "格式如 0123-ABCD-4567",
    contact: "邮箱或手机号码",
    contactHint: "与下单时填写的相同",
    submit: "查询订单",
    submitting: "查询中…",
    noMatch: "找不到匹配的订单，请确认订单编号与邮箱或手机号码。",
    invalid: "请检查订单编号与联系方式的格式。",
    tooMany: "尝试次数过多，请稍后再试。",
    unavailable: "暂时无法查询，请稍后再试。",
    sessionNote: "查询成功后，此浏览器当前的购物内容会被取代，只能查看这笔订单。",
    orderTitle: "您的订单",
    loading: "加载中…",
    loadFailed: "无法加载订单。",
    retry: "重试",
    lookupLink: "改用订单编号查询",
    heading: "查找你的订单",
    introDefault: "请输入这家商店的订单编号及下单时填写的联系资料。",
    submitMiss: "查询",
    footNote: "找不到订单时，请核对商店、订单编号与联系资料。",
    kept: "你输入的内容已保留，可直接修改后再查询。",
  },
  en: {
    title: "Find your order",
    intro: "Enter the order number from your order email and the email address or recipient phone number you used at checkout.",
    orderRef: "Order number",
    orderRefHint: "Looks like 0123-ABCD-4567",
    contact: "Email or phone number",
    contactHint: "The same one you gave at checkout",
    submit: "Find order",
    submitting: "Looking…",
    noMatch: "We could not find a matching order. Check the order number and the email or phone number.",
    invalid: "Check the format of the order number and the contact.",
    tooMany: "Too many attempts. Please try again later.",
    unavailable: "Lookup is unavailable right now. Please try again later.",
    sessionNote: "After a successful lookup this browser replaces its current shopping session.",
    orderTitle: "Your order",
    loading: "Loading…",
    loadFailed: "The order could not be loaded.",
    retry: "Try again",
    lookupLink: "Look up another order",
    heading: "Look up your order",
    introDefault: "Enter this store's order number and the contact details you gave at checkout.",
    submitMiss: "Find",
    footNote: "If the order is not found, check the store, the order number and your contact details.",
    kept: "What you entered is kept. Edit it and try again.",
  },
};
