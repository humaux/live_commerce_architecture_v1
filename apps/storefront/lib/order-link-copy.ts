// Copy (zh-CN / zh-TW / en) of the manual-order link page (/{locale}/order-link, components/OrderLink.tsx). ONE refusal text covers every reason
// a link can fail (unknown, wrong, used, expired, other store): the page must not say which.
import type { Locale } from "@live-commerce/i18n";

export type OrderLinkText = { title: string; opening: string; refused: string; refusedHelp: string; unavailable: string; retry: string; lookup: string };

export const orderLinkCopy: Record<Locale, OrderLinkText> = {
  "zh-TW": {
    title: "開啟您的訂單", opening: "正在開啟您的訂單…", refused: "此連結無法使用。",
    refusedHelp: "連結只能使用一次且有時效。請向商家索取新的連結，或用訂單編號與聯絡方式查詢訂單。",
    unavailable: "暫時無法開啟，請稍後再試。", retry: "重試", lookup: "查詢訂單",
  },
  "zh-CN": {
    title: "打开您的订单", opening: "正在打开您的订单…", refused: "此链接无法使用。",
    refusedHelp: "链接只能使用一次且有时效。请向商家索取新的链接，或用订单编号与联系方式查询订单。",
    unavailable: "暂时无法打开，请稍后再试。", retry: "重试", lookup: "查询订单",
  },
  en: {
    title: "Open your order", opening: "Opening your order…", refused: "This link cannot be used.",
    refusedHelp: "A link works once and expires. Ask the shop for a new one, or look the order up with its number and your contact details.",
    unavailable: "Temporarily unavailable. Try again shortly.", retry: "Try again", lookup: "Look up an order",
  },
};
