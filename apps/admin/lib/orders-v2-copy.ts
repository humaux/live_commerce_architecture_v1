// Orders v2 list/filter labels; BFF orders?view=v2 -> Go merchantorders.ListV2.
import type { Locale } from "@live-commerce/i18n";
const en = {
  selectedSession: "Selected session",
  moreFilters: "More filters", lessFilters: "Fewer filters", dateHint: "Year / month / day (YYYY-MM-DD)",
  tabs: { all: "All", unpaid: "Awaiting payment", transfer_review: "Review transfers", ready_to_ship: "Ready to ship", ready_to_consign: "Ready to drop off", shipped: "Shipped", completed: "Completed", cancelled: "Cancelled" },
  modes: { card: "Card", bank_transfer: "Bank transfer", pay_at_pickup: "Pay at pickup", cash_on_delivery: "Cash on delivery" },
  deliveries: { home: "Home delivery", cvs_711: "7-ELEVEN", cvs_familymart: "FamilyMart", cvs_hilife: "Hi-Life", cvs_okmart: "OK mart", unknown: "Unknown delivery" },
  search: "Search orders", hint: "Order no., phone last 4, recipient, tracking no. or SKU", apply: "Apply filters", reset: "Clear filters", payment: "Payment method", delivery: "Delivery method", session: "Live session", all: "Any", from: "From (Taipei date)", to: "To (Taipei date)", invalid: "Check the search and date range.", recipient: "Recipient", source: "Source", storefront: "Online store", manual: "Back office", live: "Live claim", attribution: "Live sources show recorded claim purchases, not viewing history.", paymentColumn: "Payment · status", deliveryColumn: "Delivery · status", total: "Matching orders", unavailable: "Unavailable", scope: "Inspection scope", active: "Hide drafts", includingDrafts: "Include drafts", completedNote: "Completion needs pickup with payment, or recorded cash collection. A manual shipment alone is not proof of delivery.", pendingCollection: "Awaiting collection", collected: "Collected", counts: "Order queues (some orders appear in more than one)",
};
export const ordersV2Copy: Record<Locale, typeof en> = {
  en,
  "zh-TW": {
    selectedSession: "已選場次",
    moreFilters: "更多篩選", lessFilters: "收合篩選", dateHint: "年／月／日（YYYY-MM-DD）",
    tabs: { all: "全部", unpaid: "待付款", transfer_review: "待核對轉帳", ready_to_ship: "待出貨", ready_to_consign: "待交寄", shipped: "已出貨", completed: "已完成", cancelled: "已取消" },
    modes: { card: "信用卡", bank_transfer: "銀行轉帳", pay_at_pickup: "取貨付款", cash_on_delivery: "貨到付款" },
    deliveries: { home: "宅配", cvs_711: "7-ELEVEN", cvs_familymart: "全家", cvs_hilife: "萊爾富", cvs_okmart: "OK 超商", unknown: "未知配送" },
    search: "搜尋訂單", hint: "訂單號、電話後 4 碼、收件人、運單號或 SKU", apply: "套用篩選", reset: "清除篩選", payment: "付款方式", delivery: "配送方式", session: "直播場次", all: "不限", from: "開始日期（台北）", to: "結束日期（台北）", invalid: "請檢查搜尋內容與日期範圍。", recipient: "收件人", source: "來源", storefront: "網店", manual: "後台", live: "直播認領", attribution: "直播來源僅顯示有記錄的認領成交，不推測觀看來源。", paymentColumn: "付款方式 · 狀態", deliveryColumn: "配送方式 · 出貨狀態", total: "符合訂單", unavailable: "暫時無法讀取", scope: "檢視範圍", active: "隱藏草稿", includingDrafts: "包含草稿", completedNote: "已完成需有取件及收款證據，或已記錄代收。手工出貨不代表已送達。", pendingCollection: "等待代收", collected: "已代收", counts: "訂單待辦（同筆訂單可出現在多個分類）",
  },
  "zh-CN": {
    selectedSession: "已选场次",
    moreFilters: "更多筛选", lessFilters: "收起筛选", dateHint: "年／月／日（YYYY-MM-DD）",
    tabs: { all: "全部", unpaid: "待付款", transfer_review: "待核对转账", ready_to_ship: "待出货", ready_to_consign: "待交寄", shipped: "已出货", completed: "已完成", cancelled: "已取消" },
    modes: { card: "信用卡", bank_transfer: "银行转账", pay_at_pickup: "取货付款", cash_on_delivery: "货到付款" },
    deliveries: { home: "宅配", cvs_711: "7-ELEVEN", cvs_familymart: "全家", cvs_hilife: "莱尔富", cvs_okmart: "OK 超商", unknown: "未知配送" },
    search: "搜索订单", hint: "订单号、电话后 4 码、收件人、运单号或 SKU", apply: "应用筛选", reset: "清除筛选", payment: "付款方式", delivery: "配送方式", session: "直播场次", all: "不限", from: "开始日期（台北）", to: "结束日期（台北）", invalid: "请检查搜索内容与日期范围。", recipient: "收件人", source: "来源", storefront: "网店", manual: "后台", live: "直播认领", attribution: "直播来源仅显示有记录的认领成交，不推测观看来源。", paymentColumn: "付款方式 · 状态", deliveryColumn: "配送方式 · 出货状态", total: "符合订单", unavailable: "暂时无法读取", scope: "查看范围", active: "隐藏草稿", includingDrafts: "包含草稿", completedNote: "已完成需有取件及收款证据，或已记录代收。手工出货不代表已送达。", pendingCollection: "等待代收", collected: "已代收", counts: "订单待办（同笔订单可出现在多个分类）",
  },
};
