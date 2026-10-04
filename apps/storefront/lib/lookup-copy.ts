// Copy (zh-CN / zh-TW / en) of the guest order lookup page (contracts/storefront-v2.md §E5), rendered by
// app/[locale]/orders/lookup/LookupForm.tsx and the order page shell. One refusal text covers every mismatch on purpose: the page must not say
// whether the order number or the contact was wrong.
import type { Locale } from "@live-commerce/i18n";

export type LookupText = {
  title: string; intro: string; orderRef: string; orderRefHint: string; contact: string; contactHint: string; submit: string; submitting: string;
  noMatch: string; invalid: string; tooMany: string; unavailable: string; sessionNote: string; orderTitle: string; loading: string;
  loadFailed: string; retry: string; lookupLink: string;
};

export const lookupCopy: Record<Locale, LookupText> = {
  "zh-TW": {
    title: "查詢訂單",
    intro: "輸入訂單通知信中的訂單編號，以及下單時使用的電子郵件或收件人手機號碼。",
    orderRef: "訂單編號",
    orderRefHint: "格式如 0123-ABCD-4567",
    contact: "電子郵件或手機號碼",
    contactHint: "與下單時填寫的相同",
    submit: "查詢",
    submitting: "查詢中…",
    noMatch: "找不到符合的訂單，請確認訂單編號與電子郵件或手機號碼。",
    invalid: "請檢查訂單編號與聯絡方式的格式。",
    tooMany: "嘗試次數過多，請稍後再試。",
    unavailable: "暫時無法查詢，請稍後再試。",
    sessionNote: "查詢成功後，此瀏覽器會取代目前的購物工作階段。",
    orderTitle: "您的訂單",
    loading: "載入中…",
    loadFailed: "無法載入訂單。",
    retry: "重試",
    lookupLink: "改用訂單編號查詢",
  },
  "zh-CN": {
    title: "查询订单",
    intro: "输入订单通知邮件中的订单编号，以及下单时使用的邮箱或收件人手机号码。",
    orderRef: "订单编号",
    orderRefHint: "格式如 0123-ABCD-4567",
    contact: "邮箱或手机号码",
    contactHint: "与下单时填写的相同",
    submit: "查询",
    submitting: "查询中…",
    noMatch: "找不到匹配的订单，请确认订单编号与邮箱或手机号码。",
    invalid: "请检查订单编号与联系方式的格式。",
    tooMany: "尝试次数过多，请稍后再试。",
    unavailable: "暂时无法查询，请稍后再试。",
    sessionNote: "查询成功后，此浏览器会取代当前的购物会话。",
    orderTitle: "您的订单",
    loading: "加载中…",
    loadFailed: "无法加载订单。",
    retry: "重试",
    lookupLink: "改用订单编号查询",
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
  },
};
