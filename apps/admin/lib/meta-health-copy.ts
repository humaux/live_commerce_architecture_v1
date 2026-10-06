// Purpose: plain merchant language for connection advice and capability states in three locales.
// Depends on: meta-connection-health-v1 state/reason vocabulary; no platform identifiers are shown as instructions.
// Used by: MetaHealthBanner and MetaHealthCapabilities.
import type { Locale } from "@live-commerce/i18n";
import type { HealthCapability } from "./meta-health-model";
const en = {
 title: "Facebook and Instagram need attention", warning: "Some Facebook or Instagram features need attention",
 reconnect: "Reconnect", owner: "Ask the store owner to reconnect.", abilities: "Connection features", recheck: "Check again", checking: "Requesting…",
 scheduled: "A new connection check is scheduled. The status will update after the check finishes.",
 unavailable: "Connection details are unavailable right now. Please try again later.", incomplete: "Feature details are unavailable right now. You can still review the connection below.",
 failed: "The check could not be requested. Please try again.", uncertain: "The request may have arrived. Check the status before trying again.",
 tooSoon: "Please wait one minute before requesting another check.", forbidden: "Only a member who manages connections can request a check.", signedOut: "Your sign-in has changed. Sign in again to continue.",
 checked: "Last checked", never: "Not checked yet", readOnly: "Ask the store owner to check or reconnect this Page.",
 capability: { read_comment: "Read comments", private_reply: "Send a private reply to a comment", dm_session: "Reply to messages", reply_public: "Reply publicly" },
 state: { ok: "Available", review_required: "Test accounts only", missing_permission: "Permission needed", missing_task: "Page access needed", not_subscribed: "Reconnect needed", reauth_required: "Reconnect needed", unsupported: "Not available", unknown: "Not confirmed yet" },
 advice: {
  reauth_required: "The connection is no longer valid. Reconnect to restore affected features.",
  missing_permission: "Some permissions are missing. Reconnect and allow the requested access.",
  missing_task: "Ask a Page administrator to allow comment or message management, then reconnect.",
  not_subscribed: "Comments or messages may not be reaching the store. Reconnect to restore them.",
  unsupported: "This connection does not support all of the requested features.",
  unknown: "Some connection features could not be confirmed. Check the connection before your next live sale.",
  probe_failing: "The latest connection check could not finish. Please check again later.",
 },
 expired: "This connection has expired. Reconnect to continue.", revoked: "Access was removed. Reconnect to allow it again.", gone: "This Page could not be found. Check your Page access and reconnect.",
 review: "Available only to accounts registered for testing.", notProbed: "Waiting for a connection check.", receiver: "Incoming messages have not been confirmed for this connection.",
};
type Copy = { [K in keyof typeof en]: typeof en[K] extends string ? string : { [P in keyof typeof en[K]]: string } };
export const metaHealthCopy: Record<Locale, Copy> = {
 en,
 "zh-TW": {
  title: "Facebook 與 Instagram 連線需要處理", warning: "部分 Facebook 或 Instagram 功能需要處理", reconnect: "重新連結", owner: "請店主重新連結。",
  abilities: "連線功能", recheck: "重新檢查", checking: "正在安排…", scheduled: "已安排重新檢查，檢查完成後會更新狀態。",
  unavailable: "目前無法查看連線狀態，請稍後再試。", incomplete: "目前無法顯示功能狀態，你仍可查看下方連線資料。",
  failed: "未能安排檢查，請稍後再試。", uncertain: "檢查要求可能已送達，請先查看狀態，再決定是否重試。", tooSoon: "請等候一分鐘，再重新檢查。", forbidden: "需要管理連線的權限才能重新檢查。", signedOut: "登入狀態已變更，請重新登入後繼續。",
  checked: "上次檢查", never: "尚未檢查", readOnly: "請店主檢查或重新連結此粉絲專頁。",
  capability: { read_comment: "能讀留言", private_reply: "能私訊（留言後）", dm_session: "能回覆私訊", reply_public: "能公開回覆" },
  state: { ok: "可使用", review_required: "僅測試帳號", missing_permission: "需要授權", missing_task: "需要粉絲專頁權限", not_subscribed: "需要重新連結", reauth_required: "連線已失效", unsupported: "目前不支援", unknown: "尚未確認" },
  advice: {
   reauth_required: "連線已失效，請重新連結以恢復受影響的功能。", missing_permission: "部分功能尚未獲得授權，請重新連結並允許所需權限。",
   missing_task: "請粉絲專頁管理員開放留言或訊息管理權限，再重新連結。", not_subscribed: "商店可能收不到留言或訊息，請重新連結以恢復接收。",
   unsupported: "目前的連線尚不支援部分功能。", unknown: "部分連線功能尚未確認，請在下次開播前檢查連線。", probe_failing: "最近一次連線檢查未能完成，請稍後重新檢查。",
  },
  expired: "連線已到期，請重新連結。", revoked: "連線授權已被移除，請重新連結。", gone: "找不到這個粉絲專頁，請確認存取權限後重新連結。",
  review: "目前只供已登記的測試帳號使用。", notProbed: "正在等候連線檢查。", receiver: "尚未確認此連線能接收私訊。",
 },
 "zh-CN": {
  title: "Facebook 与 Instagram 连接需要处理", warning: "部分 Facebook 或 Instagram 功能需要处理", reconnect: "重新连接", owner: "请店主重新连接。",
  abilities: "连接功能", recheck: "重新检查", checking: "正在安排…", scheduled: "已安排重新检查，检查完成后会更新状态。",
  unavailable: "目前无法查看连接状态，请稍后再试。", incomplete: "目前无法显示功能状态，你仍可查看下方连接资料。",
  failed: "未能安排检查，请稍后再试。", uncertain: "检查请求可能已送达，请先查看状态，再决定是否重试。", tooSoon: "请等待一分钟，再重新检查。", forbidden: "需要管理连接的权限才能重新检查。", signedOut: "登录状态已变更，请重新登录后继续。",
  checked: "上次检查", never: "尚未检查", readOnly: "请店主检查或重新连接此公共主页。",
  capability: { read_comment: "能读留言", private_reply: "能私信（留言后）", dm_session: "能回复私信", reply_public: "能公开回复" },
  state: { ok: "可使用", review_required: "仅测试账号", missing_permission: "需要授权", missing_task: "需要公共主页权限", not_subscribed: "需要重新连接", reauth_required: "连接已失效", unsupported: "目前不支持", unknown: "尚未确认" },
  advice: {
   reauth_required: "连接已失效，请重新连接以恢复受影响的功能。", missing_permission: "部分功能尚未获得授权，请重新连接并允许所需权限。",
   missing_task: "请公共主页管理员开放留言或消息管理权限，再重新连接。", not_subscribed: "商店可能收不到留言或消息，请重新连接以恢复接收。",
   unsupported: "目前的连接尚不支持部分功能。", unknown: "部分连接功能尚未确认，请在下次开播前检查连接。", probe_failing: "最近一次连接检查未能完成，请稍后重新检查。",
  },
  expired: "连接已到期，请重新连接。", revoked: "连接授权已被移除，请重新连接。", gone: "找不到这个公共主页，请确认访问权限后重新连接。",
  review: "目前仅供已登记的测试账号使用。", notProbed: "正在等待连接检查。", receiver: "尚未确认此连接能接收私信。",
 },
};
/** Maps fixed reason codes to merchant advice, never prints a raw permission/task/Graph response. */
export function capabilityAdvice(locale: Locale, row: HealthCapability): string {
 const c=metaHealthCopy[locale];
 if(row.reason==="token_expired")return c.expired;
 if(row.reason==="token_revoked")return c.revoked;
 if(row.reason==="page_unavailable")return c.gone;
 if(row.reason==="lc_u11_open")return c.receiver;
 if(row.state==="ok")return "";
 if(row.state==="review_required")return c.review;
 if(row.reason==="not_probed")return c.notProbed;
 return c.advice[row.state as keyof typeof c.advice]??c.advice.unknown;
}
