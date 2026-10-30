// Purpose: verify fixed inbox errors are translated without exposing upstream diagnostic text.
// Depends on: real inbox copy/error mapper and Node test/assert.
// Used by: LC-U2b lightweight acceptance for the three real admin locales.
import test from "node:test";
import assert from "node:assert/strict";
import {
  inboxCopy,
  inboxError,
} from "../../apps/admin/src/features/messages/copy.ts";

test("three unit languages cover identical fixed labels and delivery states", () => {
  const keys = Object.keys(inboxCopy("en"));
  for (const locale of ["zh-TW", "zh-CN", "en"]) {
    const copy = inboxCopy(locale);
    assert.deepEqual(Object.keys(copy).sort(), [...keys].sort());
    assert.ok(
      Object.values(copy).every(
        (value) => typeof value === "string" && value.length > 0,
      ),
    );
    for (const code of [
      "window_closed",
      "takeover_changed",
      "capability",
      "conversation_gone",
      "duplicate_recent",
      "invalid_text",
      "rate_limited",
      "version_conflict",
      "human_takeover",
      "auto_pending_confirm",
      "session_changed",
    ]) {
      assert.ok(inboxError(locale, code).length > 0);
    }
    assert.equal(
      inboxError(locale, "MOCK_PRIVATE_DIAGNOSTIC"),
      copy.unavailable,
    );
    assert.ok(
      !inboxError(locale, "MOCK_PRIVATE_DIAGNOSTIC").includes(
        "MOCK_PRIVATE_DIAGNOSTIC",
      ),
    );
  }
  assert.equal(inboxCopy("zh-TW").send, "送出回覆");
  assert.equal(inboxCopy("zh-CN").send, "发送回复");
  assert.equal(inboxCopy("zh-CN").buyer, "买家资料");
  assert.equal(inboxCopy("zh-CN").title, "消息");
  assert.equal(inboxCopy("en").send, "Send reply");
  assert.notEqual(inboxCopy("zh-CN").uncertain, inboxCopy("zh-TW").uncertain);
});

// INBOX-V2-IMPL: additive signed-copy RED. Access existing resolver by key so
// baseline failures are assertions about behavior, never missing-module errors.
const signedInboxCopy = {
  "zh-TW": {
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
  },
  "zh-CN": {
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
  },
  en: {
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
  },
};
for (const [locale, expected] of Object.entries(signedInboxCopy)) {
  test(`INBOX-V2 signed simplified labels and safe delivery copy in ${locale}`, () => {
    const actual = inboxCopy(locale) as unknown as Record<string, string>;
    for (const [key, value] of Object.entries(expected)) {
      assert.equal(actual[key], value, `signed ${locale} ${key}`);
    }
    assert.equal(actual.deliveryStillUnknown.includes("{channel}"), true,
      "unknown delivery must name the current server channel, not always Messenger");
  });
}
