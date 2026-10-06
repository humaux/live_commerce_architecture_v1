// Purpose: Owns localized scroll hints and native date-control labels.
// Depends on: Native JavaScript/static data; no imported runtime modules.
// Used by: apps/admin/components/AdsDraft.tsx, apps/admin/components/AdsResults.tsx, apps/admin/components/Attribution.tsx, apps/admin/components/Billing.tsx, apps/admin/components/CustomerDetail.tsx, apps/admin/components/Dashboard.tsx, apps/admin/components/Finance.tsx, apps/admin/components/LedgerTable.tsx, apps/admin/components/MerchantOrders.tsx, apps/admin/components/OrderDetailPanel.tsx, apps/admin/components/OrderListFilters.tsx, apps/admin/components/Promotions.tsx, apps/admin/components/Studio.tsx, tests/admin/attribution.test.ts
// Cross-domain presentation hints only. Business states stay in their domains.
export const presentationCopy = {
  "zh-TW": { previous: "向左捲動", next: "向右捲動", scroll: "左右捲動以查看完整表格", date: "年/月/日", dateTime: "年/月/日 時:分" },
  "zh-CN": { previous: "向左滚动", next: "向右滚动", scroll: "左右滚动以查看完整表格", date: "年/月/日", dateTime: "年/月/日 时:分" },
  en: { previous: "Scroll left", next: "Scroll right", scroll: "Scroll horizontally to view the full table", date: "YYYY-MM-DD", dateTime: "YYYY-MM-DD HH:mm" },
} as const;
