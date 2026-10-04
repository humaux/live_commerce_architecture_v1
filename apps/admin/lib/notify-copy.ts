// Copy (zh-CN / zh-TW / en) of the Settings new-order-mail card (contracts/storefront-v2.md §E6), rendered by components/NotifySettings.tsx.
// Rule encoded here: the mail names the order numbers and a count, never buyer details, and arrives at most once every five minutes.
import type { Locale } from "@live-commerce/i18n";

export const notifyCopy: Record<
  Locale,
  { title: string; label: string; note: string; save: string; saving: string; saved: string; retry: string; loading: string; loadFailed: string; failed: string; noPermission: string; uncertain: string }
> = {
  en: {
    title: "New-order emails",
    label: "Email the store owners when a new order arrives",
    note: "One email per five minutes at most, listing the order numbers. It never contains buyer details.",
    save: "Save",
    saving: "Saving…",
    saved: "Saved.",
    retry: "Try again",
    loading: "Loading…",
    loadFailed: "This setting could not be loaded.",
    failed: "Could not save. Try again.",
    noPermission: "You do not have permission to change this.",
    uncertain: "The result is unknown. Press Save again; saving twice is harmless.",
  },
  "zh-CN": {
    title: "新订单邮件",
    label: "有新订单时给店铺所有者发邮件",
    note: "每五分钟最多一封，列出订单编号，不包含买家资料。",
    save: "保存",
    saving: "保存中…",
    saved: "已保存。",
    retry: "重试",
    loading: "加载中…",
    loadFailed: "无法加载此设置。",
    failed: "保存失败，请重试。",
    noPermission: "您没有修改此设置的权限。",
    uncertain: "结果未知。请再按一次保存，重复保存不会有影响。",
  },
  "zh-TW": {
    title: "新訂單郵件",
    label: "有新訂單時寄信給店鋪擁有者",
    note: "每五分鐘最多一封，列出訂單編號，不包含買家資料。",
    save: "儲存",
    saving: "儲存中…",
    saved: "已儲存。",
    retry: "重試",
    loading: "載入中…",
    loadFailed: "無法載入此設定。",
    failed: "儲存失敗，請重試。",
    noPermission: "您沒有修改此設定的權限。",
    uncertain: "結果未知。請再按一次儲存，重複儲存不會有影響。",
  },
};
