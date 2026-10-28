// Purpose: signed simplified inbox copy in the three supported seller languages.
// Depends on: the owner-signed UX-inbox simplify/polish copy table; no customer or network data.
// Used by: admin messages copy resolver; placeholders receive authorized server facts only.
// Invariants: every locale has identical keys; examples, counts and identities are never product defaults.
const en = {
  simpleTitle: "Store inbox", simpleAll: "All", assignedInStore: "Assigned in store",
  customerDetails: "Customer details", sendAction: "Send", clearDraft: "Clear draft",
  queryStatus: "Check status", editOriginal: "Edit original text", locked: "Sending locked",
  deliveryPending: "Sending; delivery unconfirmed. Do not resend.",
  deliverySent: "Sent does not mean read.",
  deliveryRejected: "Send rejected; original text saved for editing.",
  deliveryUnknown: "Delivery unconfirmed; check status without resending.",
  deliveryStillUnknown: "Still unconfirmed; check {channel}. Do not resend.",
  connectionPending: "Connection unconfirmed; replies are unavailable.",
  permissionMissing: "Reply permission is missing.", replyEnded: "The reply window has ended.",
  autoPaused: "Auto-replies paused", storeRead: "Read by store",
  ordersUnavailable: "Orders and lifetime spend: not provided", replyBy: "Reply by {time}",
  connectionHelp: "Check the connection and reply permissions in Settings before sending.",
  windowHelp: "The reply window has ended; wait for the buyer to message again.",
  connectionAction: "Check connection permissions", windowAction: "About the reply window",
};
export type InboxMessages = { [K in keyof typeof en]: string };
const tw: InboxMessages = {
  simpleTitle: "全店訊息", simpleAll: "全部", assignedInStore: "本店分派",
  customerDetails: "顧客資料", sendAction: "傳送", clearDraft: "清除草稿",
  queryStatus: "查詢狀態", editOriginal: "編輯原文", locked: "已鎖定",
  deliveryPending: "傳送中；尚未確認送達，勿重送。",
  deliverySent: "已送出，未代表已讀。",
  deliveryRejected: "傳送遭拒；原文保留，可編輯。",
  deliveryUnknown: "送達待確認；只查詢，不重送。",
  deliveryStillUnknown: "仍待確認；到 {channel} 查看，勿重送。",
  connectionPending: "連接待確認；暫不能回覆。",
  permissionMissing: "缺少回覆權限。", replyEnded: "已超過回覆時間。",
  autoPaused: "自動回覆已停", storeRead: "店內已讀",
  ordersUnavailable: "訂單與累計：未提供", replyBy: "{time} 前回覆",
  connectionHelp: "到設定檢查連接與回覆權限；確認前無法傳送。",
  windowHelp: "已超過回覆時間；等買家再次傳訊息。",
  connectionAction: "檢查連接權限", windowAction: "查看窗口說明",
};
const cn: InboxMessages = {
  simpleTitle: "全店消息", simpleAll: "全部", assignedInStore: "店铺分配",
  customerDetails: "客户资料", sendAction: "发送", clearDraft: "清空草稿",
  queryStatus: "查询状态", editOriginal: "编辑原文", locked: "发送已锁定",
  deliveryPending: "发送中；尚未确认送达，不要重发。",
  deliverySent: "已发送，不代表已读。",
  deliveryRejected: "发送被拒；原文已保留，可以编辑。",
  deliveryUnknown: "送达情况待确认；只查询，不重发。",
  deliveryStillUnknown: "尚未确认；请到 {channel} 查看，不要重发。",
  connectionPending: "连接待确认；暂时无法回复。",
  permissionMissing: "缺少回复权限。", replyEnded: "已超过可回复时间。",
  autoPaused: "自动回复已停", storeRead: "店内已读",
  ordersUnavailable: "订单与累计消费：未提供", replyBy: "请于 {time} 前回复",
  connectionHelp: "请在设置中检查连接和回复权限；确认前无法发送。",
  windowHelp: "已超过可回复时间；请等待买家再次发消息。",
  connectionAction: "检查连接权限", windowAction: "查看回复时限说明",
};
export const inboxMessages = { "zh-TW": tw, "zh-CN": cn, en };
