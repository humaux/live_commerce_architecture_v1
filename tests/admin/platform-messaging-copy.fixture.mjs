// P2-N1 owner ruling (2026-10-05): shared Node/browser assertions, not runtime copy.
import assert from "node:assert/strict";

const expectations = {
  "zh-TW": {
    comments: [
      "收到的貼文與直播留言",
      "留言者姓名或用戶名稱、識別碼及留言文字",
    ],
    required: [
      "平台權限及訊息規則",
      "代表商家向留言者發送一則含訂單認領連結的私訊回覆",
      "記錄發送結果及 Meta 提供的送達狀態（如有）",
      "Messenger 訊息及 Instagram 私訊僅在 Meta 實際傳送至平台 Webhook 時處理，用於留言收單及訂單跟進",
      "目前未訂閱傳入私訊事件，因此目前不會經此途徑收到私訊",
    ],
    old: [
      "貼文與直播留言、Messenger 訊息及 Instagram 私訊",
      "系統處理 Messenger 訊息及 Instagram 私訊",
    ],
  },
  "zh-CN": {
    comments: ["收到的帖子与直播评论", "评论者姓名或用户名、标识符及评论文字"],
    required: [
      "平台权限及消息规则",
      "代表商家向评论者发送一条含订单认领链接的私信回复",
      "记录发送结果及 Meta 提供的送达状态（如有）",
      "Messenger 消息及 Instagram 私信仅在 Meta 实际传送至平台 Webhook 时处理，用于评论收单及订单跟进",
      "目前未订阅传入私信事件，因此目前不会经此途径收到私信",
    ],
    old: [
      "帖子与直播留言、Messenger 消息及 Instagram 私信",
      "系统处理 Messenger 消息及 Instagram 私信",
    ],
  },
  en: {
    comments: [
      "post and live-video comments received through those accounts",
      "commenter names or usernames, identifiers and comment text",
    ],
    required: [
      "platform permissions and messaging rules",
      "one private reply containing an order-claim link to the commenter on the merchant’s behalf",
      "the send result and any delivery status Meta provides",
      "Messenger messages and Instagram direct messages are processed only if Meta actually delivers them to the platform’s webhook, for comment ordering and order follow-up",
      "Inbound direct-message events are not currently subscribed to, so no incoming direct messages are currently received through this route",
    ],
    old: [
      "post and live-video comments, Messenger messages and Instagram direct messages",
      "it processes Messenger messages and Instagram direct messages",
    ],
  },
};

export function assertPlatformMessagingCopy(text, locale, page) {
  const expected = expectations[locale];
  assert.ok(expected, `known locale ${locale}`);
  for (const clause of expected.required)
    assert.ok(text.includes(clause), `${locale}/${page}: missing ${clause}`);
  for (const clause of expected.old)
    assert.equal(
      text.includes(clause),
      false,
      `${locale}/${page}: old unconditional DM claim`,
    );
  if (page === "privacy") {
    for (const clause of expected.comments)
      assert.ok(text.includes(clause), `${locale}: comment data disclosure`);
    if (locale === "zh-TW") {
      assert.ok(text.includes("Facebook 主頁"));
      assert.equal(
        text.includes("專頁"),
        false,
        "Meta section uses UI term 主頁",
      );
    }
  }
}
