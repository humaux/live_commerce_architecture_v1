// Purpose: Owns the sample navigation of the design-system gallery: the NEEDS section 1.1 information architecture (home link + nine groups with second-level pages) in three locales.
// Depends on: @live-commerce/i18n (Locale type only); no routes, permissions or API: hrefs are inert "?nav=<id>" anchors on the gallery itself.
// Used by: apps/admin/components/UiGallery.tsx.
import type { Locale } from "@live-commerce/i18n";

type Tri = readonly [en: string, zhTW: string, zhCN: string];
const pick = (t: Tri, locale: Locale) => (locale === "en" ? t[0] : locale === "zh-TW" ? t[1] : t[2]);

const SOURCE: ReadonlyArray<{ id: string; label: Tri; icon: string; items: ReadonlyArray<readonly [string, Tri]> }> = [
  { id: "home", label: ["Workbench", "工作台", "工作台"], icon: "⌂", items: [] },
  { id: "orders", label: ["Orders", "訂單", "订单"], icon: "▤", items: [
    ["order-list", ["Orders", "訂單列表", "订单列表"]],
    ["order-new", ["Create order", "手工建單", "手工建单"]],
    ["returns", ["Returns", "售後退貨", "售后退货"]],
    ["fulfilment", ["Fulfilment centre", "發貨中心", "发货中心"]],
  ] },
  { id: "live", label: ["Live", "直播", "直播"], icon: "◉", items: [
    ["sessions", ["Sessions", "場次列表", "场次列表"]],
    ["console", ["Live console", "直播控台", "直播控台"]],
    ["keywords", ["Keyword library", "關鍵詞庫", "关键词库"]],
    ["templates", ["Message templates", "訊息模板", "消息模板"]],
    ["live-settings", ["Live settings", "直播設定", "直播设置"]],
  ] },
  { id: "messages", label: ["Messages", "訊息", "消息"], icon: "✉", items: [
    ["inbox", ["Inbox", "收件匣", "收件箱"]],
  ] },
  { id: "catalog", label: ["Products", "商品", "商品"], icon: "▣", items: [
    ["products", ["Products", "商品", "商品"]],
    ["collections", ["Collections", "集合", "集合"]],
    ["inventory", ["Inventory", "庫存", "库存"]],
  ] },
  { id: "customers", label: ["Customers", "客戶", "客户"], icon: "☺", items: [
    ["customer-list", ["Customers", "客戶列表", "客户列表"]],
  ] },
  { id: "marketing", label: ["Marketing", "行銷", "营销"], icon: "✦", items: [
    ["promotions", ["Promo codes", "優惠碼", "优惠码"]],
    ["ads", ["Ads", "廣告", "广告"]],
  ] },
  { id: "storefront", label: ["Online store", "網店", "网店"], icon: "◫", items: [
    ["design", ["Design", "裝修", "装修"]],
    ["pages", ["Pages", "頁面", "页面"]],
    ["media", ["Media library", "媒體庫", "媒体库"]],
    ["domains", ["Domains & policies", "網域與政策", "域名与政策"]],
  ] },
  { id: "finance", label: ["Data & finance", "數據與財務", "数据与财务"], icon: "◔", items: [
    ["overview", ["Overview", "概覽", "概览"]],
    ["reports", ["Reports", "報表", "报表"]],
    ["card-settlements", ["Card settlements", "信用卡結算", "信用卡结算"]],
  ] },
  { id: "settings", label: ["Settings", "設定", "设置"], icon: "⚙", items: [
    ["store-profile", ["Store profile", "店鋪資料", "店铺资料"]],
    ["shipping", ["Shipping", "配送", "配送"]],
    ["payments", ["Payments", "收款", "收款"]],
    ["notifications", ["Notifications", "通知", "通知"]],
    ["channels", ["Channel connections", "渠道連接", "渠道连接"]],
    ["team", ["Team", "團隊", "团队"]],
    ["billing", ["Plan & billing", "方案與帳單", "方案与账单"]],
    ["import-export", ["Import / export", "匯入匯出中心", "导入导出中心"]],
    ["jobs", ["Jobs & exceptions", "任務與異常", "任务与异常"]],
  ] },
];

/** The home link (no second level) plus the nine groups; the id of a leaf is unique across the whole tree. */
export function galleryNav(locale: Locale) {
  return SOURCE.map((g) => ({
    id: g.id,
    label: pick(g.label, locale),
    icon: g.icon,
    href: `?nav=${g.id}`,
    items: g.items.map(([id, label]) => ({ id, label: pick(label, locale), href: `?nav=${id}` })),
  }));
}
