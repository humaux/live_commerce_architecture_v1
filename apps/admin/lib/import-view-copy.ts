// Purpose: plain import navigation, retry and in-memory file lifecycle copy.
// Depends on: existing three-locale contract; no uploaded content.
// Used by: ImportWizard and ImportWizardSteps.
import type { Locale } from "@live-commerce/i18n";
/** Labels for explicit same-file retry and safe page navigation; no side effects. */
export const importViewCopy: Record<Locale, { next: string; previous: string; retrySame: string; scopeReset: string; ready: string; sessionUnknown: string; receiptMismatch: string }> = {
  "zh-TW": { next: "下一步", previous: "上一頁", retrySame: "重試同一份匯入", scopeReset: "檔案只在這個頁面處理；離開頁面後，請重新選擇檔案。若未看到完成結果，請使用相同檔案與設定核對。", ready: "顧客匯入已完成，可以匯入歷史訂單。", receiptMismatch: "已收到之前的匯入結果，但數量與預覽不同。重送不會改變結果，請先核對顧客或歷史訂單。", sessionUnknown: "離開頁面或登入狀態改變時，仍有匯入尚未確認。請核對顧客頁，或重新選擇原檔案與相同欄位設定確認結果。" },
  "zh-CN": { next: "下一步", previous: "上一页", retrySame: "重试同一份导入", scopeReset: "文件只在这个页面处理；离开页面后，请重新选择文件。如果未看到完成结果，请使用相同文件与设置核对。", ready: "顾客导入已完成，可以导入历史订单。", receiptMismatch: "已收到之前的导入结果，但数量与预览不同。重新发送不会改变结果，请先核对顾客或历史订单。", sessionUnknown: "离开页面或登录状态改变时，仍有导入尚未确认。请核对顾客页，或重新选择原文件与相同栏位设置确认结果。" },
  en: { next: "Next", previous: "Previous page", retrySame: "Retry this same import", scopeReset: "The file is used only on this page. Choose it again after leaving. If you did not see a completed result, check with the same file and settings.", ready: "Customer import is complete. You can now import historical orders.", receiptMismatch: "A saved import result was found, but its counts differ from the preview. Sending it again will not change the result. Check the customer list or historical orders before importing another file.", sessionUnknown: "An import was still unconfirmed when you left the page or your sign-in changed. Check the customer list, or choose the original file and column mapping again to confirm the result." },
};
