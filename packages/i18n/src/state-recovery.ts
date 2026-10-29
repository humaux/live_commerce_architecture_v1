// Purpose: Three-locale seller copy for the StatePanel/InlineError acceptance fixtures.
// Depends on: Locale from index; no diagnostics or resource identifiers enter displayed copy.
// Used by: UiGalleryRecovery and its SSR/browser acceptance tests.
// Invariants: uncertainty offers a read of the same operation, never another submission.
import type { Locale } from "./index";

const en = {
  title: "Recovery actions", empty: "No products yet", emptyBody: "Add your first product.",
  forbidden: "This page cannot be opened", forbiddenBody: "Return to your store to continue.",
  forbiddenNote: "Ask the store owner if you need access.",
  offline: "The list could not be loaded", offlineBody: "Check your connection and try loading it again.",
  expired: "This link is no longer available", expiredBody: "Return to your store to continue.",
  unknown: "The result is not confirmed", unknownBody: "Check the same operation before trying again.",
  pending: "Waiting for confirmation", checked: "Status checked. Still waiting for confirmation.",
  failed: "The latest status could not be loaded", loaded: "The list has been loaded.",
  retry: "Reload", check: "Check status", back: "Return to store",
};
type RecoveryCopy = typeof en;
export const stateRecoveryCopy: Record<Locale, RecoveryCopy> = {
  en,
  "zh-TW": {
    title: "恢復操作", empty: "還沒有商品", emptyBody: "先新增第一件商品。",
    forbidden: "目前無法開啟此頁", forbiddenBody: "請返回店鋪繼續操作。",
    forbiddenNote: "如需存取，請洽店鋪擁有者。",
    offline: "暫時無法載入列表", offlineBody: "請檢查網路連線，再重新載入。",
    expired: "此連結已無法使用", expiredBody: "請返回店鋪繼續操作。",
    unknown: "操作結果尚未確認", unknownBody: "請先查詢同一筆操作的狀態，確認前請勿再次送出。",
    pending: "等待確認", checked: "已查詢狀態，仍在等待確認。",
    failed: "暫時無法載入最新狀態", loaded: "已重新載入列表。",
    retry: "重新載入", check: "查詢狀態", back: "返回店鋪",
  },
  "zh-CN": {
    title: "恢复操作", empty: "还没有商品", emptyBody: "先添加第一件商品。",
    forbidden: "暂时无法打开此页面", forbiddenBody: "请返回店铺继续操作。",
    forbiddenNote: "如需访问，请联系店铺所有者。",
    offline: "暂时无法加载列表", offlineBody: "请检查网络连接，再重新加载。",
    expired: "此链接已无法使用", expiredBody: "请返回店铺继续操作。",
    unknown: "操作结果尚未确认", unknownBody: "请先查询同一笔操作的状态，确认前请勿再次提交。",
    pending: "等待确认", checked: "已查询状态，仍在等待确认。",
    failed: "暂时无法加载最新状态", loaded: "已重新加载列表。",
    retry: "重新加载", check: "查询状态", back: "返回店铺",
  },
};
