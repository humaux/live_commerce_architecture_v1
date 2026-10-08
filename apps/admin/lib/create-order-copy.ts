// Purpose: three-locale buyer-order guidance and closed reason/error labels.
// Depends on: Locale and the existing manual-order error copy; never renders backend prose.
// Used by: CreateOrderDrawer and LC-U3 copy acceptance.
import type { Locale } from "@live-commerce/i18n";
import { toolsCopy } from "./merchant-tools-copy.ts";
const en = {
  title: "Create an order for this buyer",
  claims: "Claim limits",
  claimRemaining: (n: number) => `Remaining live-price claim quantity: ${n}`,
  unavailableItem: "Currently unavailable",
  close: "Close",
  create: "Create order",
  retry: "Retry the same request",
  loading: "Loading order details…",
  linked: "Manually linked customer",
  liveWarning:
    "Live pricing does not apply to this order. Send the claim link instead.",
  quote:
    "Catalog unit prices are for reference. The server confirms the order total and any eligible live pricing.",
  permission: "Order-read and inventory-reservation permission are required.",
  livePermission: "Live management permission is required for live pricing.",
  send: "Send the payment link by message",
  sendDisabled: "Messaging is unavailable for this conversation or account.",
  review: "Provider review is required for messaging.",
  window:
    "The messaging window has closed. The order can be created; copy the payment link if it cannot be sent.",
  uncertain:
    "The order may already exist. Retry the same request to check it; do not create another order.",
  guard:
    "An earlier order request is unresolved. Check existing orders before continuing. This session cannot start another order.",
  orders: "View orders",
  view: "View order",
  copy: "Copy payment link",
  copied: "Payment link copied",
  copyFailed: "Could not copy the payment link. Try the copy button again.",
  success: "Order created",
  applied: "Live pricing applied",
  notApplied: "Live pricing not applied",
  queued: "Payment message queued",
  notSent: "Payment message not sent",
  unknown: "Details are unavailable.",
  tooMany:
    "This conversation has more than five bundles. Open one claim bundle to create its order.",
  duplicate: "This bundle already has an order.",
  mismatch:
    "The bundles do not belong to this buyer. Check the selected conversation.",
  conflict:
    "The original request cannot be changed. Check the existing order before trying again.",
};
type Copy = typeof en;
const tw: Copy = {
  title: "幫他建立訂單",
  claims: "認領限制",
  claimRemaining: (n: number) => `直播價剩餘認領數量：${n}`,
  unavailableItem: "目前無法購買",
  close: "關閉",
  create: "建立訂單",
  retry: "重試同一請求",
  loading: "正在載入訂單資料…",
  linked: "已手動連結",
  liveWarning: "直播價不適用代建訂單，請改傳認領連結",
  quote: "目錄單價僅供參考。訂單總額與適用的直播價由伺服器確認。",
  permission: "需要讀取訂單及預留庫存權限。",
  livePermission: "使用直播價需要直播管理權限。",
  send: "透過私訊傳送付款連結",
  sendDisabled: "此對話或帳號目前無法傳送私訊。",
  review: "私訊功能需要平台審查。",
  window: "私訊視窗已關閉。仍可建立訂單；若無法傳送，請複製付款連結。",
  uncertain: "訂單可能已建立。請重試同一請求確認，勿另建訂單。",
  guard:
    "先前的訂單請求尚未確認。請先查看現有訂單；此工作階段暫時無法另建訂單。",
  orders: "查看訂單列表",
  view: "查看訂單",
  copy: "複製付款連結",
  copied: "已複製付款連結",
  copyFailed: "無法複製付款連結，請再次點選複製。",
  success: "訂單已建立",
  applied: "已套用直播價",
  notApplied: "未套用直播價",
  queued: "付款訊息已排程",
  notSent: "未傳送付款訊息",
  unknown: "目前無法取得詳細資料。",
  tooMany: "此對話超過五個認領包，請開啟單一認領包建立訂單。",
  duplicate: "此認領包已有訂單。",
  mismatch: "認領包不屬於此買家，請確認所選對話。",
  conflict: "原請求不可更改，請先確認現有訂單。",
};
const cn: Copy = {
  title: "帮他建立订单",
  claims: "认领限制",
  claimRemaining: (n: number) => `直播价剩余认领数量：${n}`,
  unavailableItem: "目前无法购买",
  close: "关闭",
  create: "建立订单",
  retry: "重试同一请求",
  loading: "正在加载订单资料…",
  linked: "已手动关联",
  liveWarning: "直播价不适用代建订单，请改传认领链接",
  quote: "目录单价仅供参考。订单总额与适用的直播价由服务器确认。",
  permission: "需要读取订单及预留库存权限。",
  livePermission: "使用直播价需要直播管理权限。",
  send: "通过私信发送付款链接",
  sendDisabled: "此对话或账号目前无法发送私信。",
  review: "私信功能需要平台审核。",
  window: "私信窗口已关闭。仍可建立订单；若无法发送，请复制付款链接。",
  uncertain: "订单可能已建立。请重试同一请求确认，勿另建订单。",
  guard: "先前的订单请求尚未确认。请先查看现有订单；此会话暂时无法另建订单。",
  orders: "查看订单列表",
  view: "查看订单",
  copy: "复制付款链接",
  copied: "已复制付款链接",
  copyFailed: "无法复制付款链接，请再次点击复制。",
  success: "订单已建立",
  applied: "已应用直播价",
  notApplied: "未应用直播价",
  queued: "付款消息已排程",
  notSent: "未发送付款消息",
  unknown: "目前无法获取详细资料。",
  tooMany: "此对话超过五个认领包，请打开单一认领包建立订单。",
  duplicate: "此认领包已有订单。",
  mismatch: "认领包不属于此买家，请确认所选对话。",
  conflict: "原请求不可更改，请先确认现有订单。",
};
export const createOrderCopy: Record<Locale, Copy> = {
  en,
  "zh-TW": tw,
  "zh-CN": cn,
};
const reasons: Record<string, [string, string, string]> = {
  no_bundle: [
    "No claim bundle was selected.",
    "未選擇認領包。",
    "未选择认领包。",
  ],
  no_conversation: [
    "No verified messaging conversation.",
    "沒有已驗證的私訊對話。",
    "没有已验证的私信对话。",
  ],
  bundle_buyer_unverified: [
    "The claim buyer has not been verified.",
    "認領買家尚未驗證。",
    "认领买家尚未验证。",
  ],
  bundle_buyer_mismatch: [
    "The claim buyer does not match.",
    "認領買家不符。",
    "认领买家不符。",
  ],
  permission: [
    "Live management permission is missing.",
    "缺少直播管理權限。",
    "缺少直播管理权限。",
  ],
  no_live_line: [
    "No qualifying live-price item.",
    "沒有符合直播價的品項。",
    "没有符合直播价的商品。",
  ],
  live_price_unavailable: [
    "Live pricing is no longer available.",
    "直播價目前無法使用。",
    "直播价目前无法使用。",
  ],
  some_lines_catalog: [
    "Some items use catalog pricing.",
    "部分品項使用目錄價格。",
    "部分商品使用目录价格。",
  ],
  not_requested: ["Sending was not requested.", "未要求傳送。", "未要求发送。"],
  send_unavailable: [
    "Messaging is unavailable.",
    "私訊暫時無法使用。",
    "私信暂时无法使用。",
  ],
  link_unavailable: [
    "The payment link is unavailable.",
    "付款連結目前無法使用。",
    "付款链接目前无法使用。",
  ],
  window_closed: [
    "The messaging window has closed.",
    "私訊視窗已關閉。",
    "私信窗口已关闭。",
  ],
  capability: [
    "Messaging is not enabled for this connection.",
    "此連線未開放私訊。",
    "此连接未开放私信。",
  ],
  takeover_changed: [
    "Conversation assignment changed.",
    "對話接手狀態已變更。",
    "对话接手状态已变更。",
  ],
  duplicate_recent: [
    "A matching message was sent recently.",
    "近期已傳送相同訊息。",
    "近期已发送相同消息。",
  ],
  rate_limited: [
    "Messaging is temporarily rate limited.",
    "私訊暫時受到頻率限制。",
    "私信暂时受到频率限制。",
  ],
  invalid_text: [
    "The message could not be prepared.",
    "無法準備訊息。",
    "无法准备消息。",
  ],
};
/** Unknown backend reasons use safe localized fallback, never raw machine strings. */
export function createOrderReason(locale: Locale, code: string) {
  return code === ""
    ? ""
    : (reasons[code]?.[locale === "en" ? 0 : locale === "zh-TW" ? 1 : 2] ??
        createOrderCopy[locale].unknown);
}
const extraErrors: Record<string, [string, string, string]> = {
  capability: [
    "This account cannot perform this operation.",
    "此帳號無法執行此操作。",
    "此账号无法执行此操作。",
  ],
  cod_surcharge_changed: [
    "The cash-on-delivery fee changed. Review delivery before submitting again.",
    "貨到付款費用已變更，請確認配送方式後再提交。",
    "货到付款费用已变更，请确认配送方式后再提交。",
  ],
  cvs_recipient_rejected: [
    "Check the pickup recipient name and phone number.",
    "請確認取貨人的姓名與電話。",
    "请确认取货人的姓名与电话。",
  ],
  cvs_environment_mismatch: [
    "This pickup option is not available in the current environment.",
    "此取貨選項不適用目前環境。",
    "此取货选项不适用当前环境。",
  ],
  cvs_source_mismatch: [
    "Choose a pickup store using the delivery option supported by this shop.",
    "請使用商店支援的配送選項選擇取貨門市。",
    "请使用商店支持的配送选项选择取货门店。",
  ],
  max_per_order_exceeded: [
    "The quantity exceeds the per-order limit.",
    "數量超過每筆訂單上限。",
    "数量超过每笔订单上限。",
  ],
  service_unavailable: [
    "The selected delivery service is unavailable.",
    "所選配送服務暫時無法使用。",
    "所选配送服务暂时无法使用。",
  ],
  bad_store_code: [
    "Check the pickup store number.",
    "請確認取貨門市編號。",
    "请确认取货门店编号。",
  ],
  bad_store_name: [
    "Check the pickup store name.",
    "請確認取貨門市名稱。",
    "请确认取货门店名称。",
  ],
  bad_store_address: [
    "Check the pickup store address.",
    "請確認取貨門市地址。",
    "请确认取货门店地址。",
  ],
  pay_at_pickup_limit: [
    "Too many pickup orders are pending. Choose another payment method.",
    "待處理的取貨付款訂單過多，請選擇其他付款方式。",
    "待处理的取货付款订单过多，请选择其他付款方式。",
  ],
  too_many_stores_entered: [
    "Too many pickup-store attempts. Wait before trying again.",
    "選擇門市次數過多，請稍後再試。",
    "选择门店次数过多，请稍后再试。",
  ],
  rate_limited: [
    "Too many requests. Wait before retrying this request.",
    "請求過於頻繁，請稍後重試同一請求。",
    "请求过于频繁，请稍后重试同一请求。",
  ],
};
const aliases: Record<string, string> = {
  invalid_json: "invalid_request",
  json_required: "invalid_request",
  version_changed: "conflict",
  cvs_amount_exceeds: "pay_at_pickup_amount_exceeds",
  throttled: "rate_limited",
};
/** Specific money-path refusals override generic manual-order copy without exposing response prose. */
export function createOrderError(locale: Locale, code: string) {
  const c = createOrderCopy[locale],
    key = aliases[code] ?? code;
  return (
    (
      {
        bundle_already_ordered: c.duplicate,
        bundle_buyer_mismatch: c.mismatch,
        idempotency_conflict: c.conflict,
        too_many_bundles: c.tooMany,
      } as Record<string, string>
    )[key] ??
    extraErrors[key]?.[locale === "en" ? 0 : locale === "zh-TW" ? 1 : 2] ??
    toolsCopy[locale].manual.errors[key] ??
    toolsCopy[locale].manual.errors.default
  );
}
