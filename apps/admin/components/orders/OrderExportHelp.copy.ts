// Purpose: typed three-locale export-help copy without export authority.
// Depends on: Locale from @live-commerce/i18n.
// Used by: OrderExportHelp.
import type { Locale } from "@live-commerce/i18n";
export const orderExportHelpCopy = {
  "zh-TW": {
    title: "訂單匯出說明",
    warning: "匯出包含收件資料，請妥善保存。",
    body: "只有具備訂單匯出權限的成員可下載未出貨訂單。每次最多 1,000 筆；電話、郵遞區號及門市編號應以文字欄位開啟，保留前面的 0。",
    back: "返回訂單",
  },
  "zh-CN": {
    title: "订单导出说明",
    warning: "导出包含收件资料，请妥善保存。",
    body: "只有具备订单导出权限的成员可下载未发货订单。每次最多 1,000 笔；电话、邮政编码及门店编号应以文本字段打开，保留前面的 0。",
    back: "返回订单",
  },
  en: {
    title: "Order export guide",
    warning: "Exports contain recipient details. Store them securely.",
    body: "Only members with order export permission can download unshipped orders. Each export includes at most 1,000 orders. Open phone numbers, postal codes and pickup store codes as text to preserve leading zeros.",
    back: "Back to orders",
  },
} satisfies Record<
  Locale,
  { title: string; warning: string; body: string; back: string }
>;
