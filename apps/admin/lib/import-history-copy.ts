// Purpose: plain three-language copy for the read-only SHOPLINE historical-order archive.
// Depends on: Locale only; preserves W5 rulings that history is excluded from revenue/reports.
// Used by: CustomerHistoricalOrders and copy/SSR tests; no authority or side effects.
import type { Locale } from "@live-commerce/i18n";
const en = {
  title: "Historical orders (SHOPLINE)", basis: "Imported history is read-only and is excluded from revenue and reports.",
  loading: "Loading historical orders…", empty: "No historical orders have been imported for this customer.",
  emptyPage: "No historical orders on this page. Refresh to view the latest history.",
  forbidden: "You do not have permission to view this customer's historical orders.", signedOut: "Your session ended. Sign in again to continue.",
  notFound: "This customer is no longer available in this store.", unavailable: "Historical orders are temporarily unavailable.",
  retry: "Try again", refresh: "Refresh", previous: "Previous", next: "Next", order: "SHOPLINE order number", date: "Order date", status: "Original status",
  amount: "Historical amount", items: "Items", city: "City", notProvided: "Not provided", scroll: "Scroll horizontally to view all columns.",
  count: (visible: number, total: number) => `${visible} shown · ${total} historical orders in total · up to 50 per page`,
  page: (page: number) => `Page ${page}`,
};
/** Copy shape for the archive; count/page functions format only server facts and local navigation. */
export type ImportHistoryCopy = typeof en;
/** Localized historical-order labels and honest unavailable/permission states. */
export const importHistoryCopy: Record<Locale, ImportHistoryCopy> = {
  en,
  "zh-CN": {
    title: "历史订单（SHOPLINE）", basis: "导入的历史订单只供查看，不计入营收与报表。", loading: "正在读取历史订单…",
    empty: "此客户尚未导入历史订单。", forbidden: "你没有查看此客户历史订单的权限。", signedOut: "会话已结束，请重新登录后继续。",
    emptyPage: "此页没有历史订单，请刷新查看最新资料。",
    notFound: "此客户在当前店铺中已不可用。", unavailable: "历史订单暂时无法读取。", retry: "重试", refresh: "刷新", previous: "上一页", next: "下一页",
    order: "SHOPLINE 单号", date: "下单时间", status: "原订单状态", amount: "历史金额", items: "品项摘要", city: "城市", notProvided: "未提供",
    scroll: "左右滑动查看所有列。", count: (visible, total) => `本页 ${visible} 笔 · 历史订单共 ${total} 笔 · 每页最多 50 笔`, page: (page) => `第 ${page} 页`,
  },
  "zh-TW": {
    title: "歷史訂單（SHOPLINE）", basis: "匯入的歷史訂單只供查看，不計入營收與報表。", loading: "正在讀取歷史訂單…",
    empty: "此客戶尚未匯入歷史訂單。", forbidden: "你沒有查看此客戶歷史訂單的權限。", signedOut: "工作階段已結束，請重新登入後繼續。",
    emptyPage: "此頁沒有歷史訂單，請重新整理查看最新資料。",
    notFound: "此客戶在目前店鋪中已不可用。", unavailable: "歷史訂單暫時無法讀取。", retry: "重試", refresh: "重新整理", previous: "上一頁", next: "下一頁",
    order: "SHOPLINE 單號", date: "下單時間", status: "原訂單狀態", amount: "歷史金額", items: "品項摘要", city: "城市", notProvided: "未提供",
    scroll: "左右滑動查看所有欄位。", count: (visible, total) => `本頁 ${visible} 筆 · 歷史訂單共 ${total} 筆 · 每頁最多 50 筆`, page: (page) => `第 ${page} 頁`,
  },
};
