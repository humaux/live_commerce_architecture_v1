import type { Locale } from "@live-commerce/i18n";

const en = {
  title: "Your orders",
  back: "Back to shopping",
  next: "Continue shopping",
  note: "Starting a new purchase does not cancel or pay your existing order.",
  scope:
    "Orders available to this shopping session. Keep access to this browser; a new session cannot recover an expired one.",
  empty: "No orders in this shopping session.",
  loading: "Loading orders…",
  failed: "Orders could not be loaded. Retry without starting a new session.",
  retry: "Retry",
  more: "Older orders",
  details: "View order",
  list: "Back to orders",
};
export const historyCopy: Record<Locale, { [K in keyof typeof en]: string }> = {
  en,
  "zh-CN": {
    title: "我的订单",
    back: "返回选购",
    next: "继续购物",
    note: "开始新的选购不会取消或支付已有订单。",
    scope:
      "显示当前购物会话的订单。请保留此浏览器的访问权限；新会话无法恢复已过期的会话。",
    empty: "当前购物会话还没有订单。",
    loading: "正在读取订单…",
    failed: "暂时无法读取订单，请重试，不要创建新会话。",
    retry: "重试",
    more: "更早的订单",
    details: "查看订单",
    list: "返回订单列表",
  },
  "zh-TW": {
    title: "我的訂單",
    back: "返回選購",
    next: "繼續購物",
    note: "開始新的選購不會取消或支付既有訂單。",
    scope:
      "顯示目前購物工作階段的訂單。請保留此瀏覽器的存取權限；新工作階段無法恢復已過期的工作階段。",
    empty: "目前購物工作階段還沒有訂單。",
    loading: "正在讀取訂單…",
    failed: "暫時無法讀取訂單，請重試，不要建立新工作階段。",
    retry: "重試",
    more: "更早的訂單",
    details: "查看訂單",
    list: "返回訂單列表",
  },
};
