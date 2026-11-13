// Purpose: exhaustive localized buyer aftersales copy and safe refusal dictionary.
// Depends on: Locale and frozen return refusal codes. Called by: ReturnRequestForm/Detail.
// Invariants: never display upstream text; approved/rejected presentation remains outside this slice.
import type { Locale } from "@live-commerce/i18n";
type Copy = {
  title: string; back: string; select: string; quantity: string; reason: string; damaged: string;
  note: string; photos: string; disclaimer: string; submit: string; invalid: string; loading: string;
  detail: string; content: string; progress: string; order: string; requested: string; withdrawn: string;
  withdraw: string; confirmTitle: string; confirmBody: string; confirm: string; cancel: string;
  withdrawnBody: string; pendingBody: string; check: string; unknown: string; failed: string;
  neutral: string; choose: string; noMatches: string; errors: Record<string, string>;
};
const hant: Copy = {
  title: "申請售後", back: "返回訂單", select: "選擇需要協助的商品", quantity: "申請數量", reason: "申請原因", damaged: "商品有瑕疵",
  note: "問題說明（選填，最多 1000 字）", photos: "照片目前不支援", disclaimer: "送出後由商家確認；此步驟不代表已核准退貨或退款。",
  submit: "確認並送出", invalid: "請選擇商品、輸入正整數數量，並確認說明不超過 1000 字且不含控制字元。", loading: "讀取中…",
  detail: "售後申請", content: "申請內容", progress: "處理進度", order: "訂單", requested: "等待商家確認", withdrawn: "已撤回",
  withdraw: "撤回申請", confirmTitle: "撤回這筆申請？", confirmBody: "撤回後將停止等待商家確認；此操作不會發起退款。", confirm: "確認撤回", cancel: "繼續等待商家",
  withdrawnBody: "此申請已撤回；本次撤回未發起退款。", pendingBody: "申請仍在等待商家確認；尚未核准退貨或退款。", check: "讀取申請狀態",
  unknown: "結果尚未確認。請讀取已保存的申請確認狀態；目前不會再次送出。", failed: "目前無法讀取，請稍後再試。", neutral: "目前無法顯示此申請狀態。", choose: "查看已保存的申請", noMatches: "未讀取到此訂單的申請；這不代表送出失敗。",
  errors: { window_closed: "已超過申請期限。", request_open: "此商品已有待確認的申請。", version_changed: "狀態已更新，正在重新讀取。", invalid_state: "目前狀態無法執行此操作。", not_shipped: "商品尚未出貨。", not_returnable: "此商品目前無法申請。", unknown_line: "無法確認選擇的商品。", ambiguous_line: "商家需要確認商品的出貨倉庫。", invalid_quantities: "請確認申請數量。", exceeds_shipped: "申請數量超過可申請數量。", conflict: "此操作與既有申請衝突，請讀取狀態。", invalid_request: "請確認申請內容。", invalid_json: "請確認申請內容。", json_required: "請確認申請內容。", method_not_allowed: "目前無法執行此操作。" },
};
const hans: Copy = {
  title: "申请售后", back: "返回订单", select: "选择需要协助的商品", quantity: "申请数量", reason: "申请原因", damaged: "商品有瑕疵", note: "问题说明（选填，最多 1000 字）", photos: "照片目前不支持", disclaimer: "提交后由商家确认；此步骤不代表已批准退货或退款。", submit: "确认并提交", invalid: "请选择商品、输入正整数数量，并确认说明不超过 1000 字且不含控制字符。", loading: "读取中…", detail: "售后申请", content: "申请内容", progress: "处理进度", order: "订单", requested: "等待商家确认", withdrawn: "已撤回", withdraw: "撤回申请", confirmTitle: "撤回这笔申请？", confirmBody: "撤回后将停止等待商家确认；此操作不会发起退款。", confirm: "确认撤回", cancel: "继续等待商家", withdrawnBody: "此申请已撤回；本次撤回未发起退款。", pendingBody: "申请仍在等待商家确认；尚未批准退货或退款。", check: "读取申请状态", unknown: "结果尚未确认。请读取已保存的申请确认状态；目前不会再次提交。", failed: "目前无法读取，请稍后再试。", neutral: "目前无法显示此申请状态。", choose: "查看已保存的申请", noMatches: "未读取到此订单的申请；这不代表提交失败。",
  errors: { window_closed: "已超过申请期限。", request_open: "此商品已有待确认的申请。", version_changed: "状态已更新，正在重新读取。", invalid_state: "目前状态无法执行此操作。", not_shipped: "商品尚未出货。", not_returnable: "此商品目前无法申请。", unknown_line: "无法确认选择的商品。", ambiguous_line: "商家需要确认商品的出货仓库。", invalid_quantities: "请确认申请数量。", exceeds_shipped: "申请数量超过可申请数量。", conflict: "此操作与既有申请冲突，请读取状态。", invalid_request: "请确认申请内容。", invalid_json: "请确认申请内容。", json_required: "请确认申请内容。", method_not_allowed: "目前无法执行此操作。" },
};
const en: Copy = {
  title: "Request aftersales help", back: "Back to order", select: "Select items needing help", quantity: "Requested quantity", reason: "Reason", damaged: "Item is damaged", note: "Description (optional, up to 1,000 characters)", photos: "Photos are not supported yet", disclaimer: "The merchant will review your request. Submitting does not approve a return or refund.", submit: "Confirm and submit", invalid: "Select items and enter positive whole quantities. Keep the note within 1,000 characters without control characters.", loading: "Loading…", detail: "Aftersales request", content: "Request details", progress: "Progress", order: "Order", requested: "Awaiting merchant confirmation", withdrawn: "Withdrawn", withdraw: "Withdraw request", confirmTitle: "Withdraw this request?", confirmBody: "Withdrawal stops waiting for merchant confirmation. This action does not initiate a refund.", confirm: "Confirm withdrawal", cancel: "Keep waiting for merchant", withdrawnBody: "This request was withdrawn. This withdrawal did not initiate a refund.", pendingBody: "Awaiting merchant confirmation. No return or refund has been approved.", check: "Read request status", unknown: "The outcome is unconfirmed. Check saved requests to confirm the status. This action will not be submitted again.", failed: "Unable to read right now. Please try again later.", neutral: "This request status cannot currently be displayed.", choose: "View saved request", noMatches: "No request for this order was read. This does not prove submission failed.",
  errors: { window_closed: "The request window has closed.", request_open: "This item already has an open request.", version_changed: "The status changed. Reading the current status.", invalid_state: "This action is unavailable in the current state.", not_shipped: "The item has not shipped.", not_returnable: "This item cannot currently be requested.", unknown_line: "The selected item could not be confirmed.", ambiguous_line: "The merchant must confirm the shipping warehouse.", invalid_quantities: "Check the requested quantities.", exceeds_shipped: "The requested quantity exceeds the available quantity.", conflict: "This action conflicts with an existing request. Read the current status.", invalid_request: "Check the request details.", invalid_json: "Check the request details.", json_required: "Check the request details.", method_not_allowed: "This action is currently unavailable." },
};
export const returnRequestCopy: Record<Locale, Copy> = { "zh-TW": hant, "zh-CN": hans, en };
