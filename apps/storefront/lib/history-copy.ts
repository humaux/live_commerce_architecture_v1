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
      "显示此浏览器可查看的订单。请保留此浏览器的使用状态；购物连接过期后重新开始，无法找回之前的订单。",
    empty: "此浏览器目前还没有订单。",
    loading: "正在读取订单…",
    failed: "暂时无法读取订单，请重试，不要重新开始购物。",
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
      "顯示此瀏覽器可查看的訂單。請保留此瀏覽器的使用狀態；購物連線過期後重新開始，無法找回之前的訂單。",
    empty: "此瀏覽器目前還沒有訂單。",
    loading: "正在讀取訂單…",
    failed: "暫時無法讀取訂單，請重試，不要重新開始購物。",
    retry: "重試",
    more: "更早的訂單",
    details: "查看訂單",
    list: "返回訂單列表",
  },
};
