// Purpose: typed three-locale signed template copy without a publication claim.
// Depends on: Locale from @live-commerce/i18n.
// Used by: MessageTemplateList and MessageTemplateForm.
import type { Locale } from "@live-commerce/i18n";
const tw = {
  title: "消息範本",
  mobileTitle: "消息模板",
  count: "已發布版本 · 示範資料 {n} 筆",
  disclaimer: "發布不等於發送；實際發送前仍會檢查連接授權與消息期限。",
  unreadable:
    "既有範本目前無法讀取正文，因此暫不支援編輯。你可以新增自訂範本。",
  add: "＋ 新增自訂範本",
  manageRequired: "需要管理直播的權限才能新增範本。",
  name: "範本",
  version: "版本",
  kinds: "適用消息類型",
  safe: "公開安全",
  date: "建立時間",
  created: "建立",
  edit: "編輯",
  unavailable: "暫不可用",
  editHint: "編輯不可用：暫無正文讀取功能",
  yes: "是",
  no: "否",
  empty: "尚無已發布範本。",
  formTitle: "新增自訂範本",
  formMobile: "新增範本",
  back: "← 返回消息範本",
  intro: "先確認內容與適用類型，再發布版本；發布不會傳送訊息。",
  nameLabel: "範本名稱 *",
  nameHint: "方便團隊辨認，1–120 個字元。",
  idLabel: "範本識別碼 *",
  idHint:
    "1–64 個字元；首字限小寫英文或數字，其餘可含 /、_、-。同一識別碼發布時追加版本。",
  bodyLabel: "訊息正文 *",
  bodyHint: "正文最多 2,000 字元；啟用公開安全檢查時，所有類型均限 300 字元。",
  kindsLabel: "適用消息類型 *",
  kindsHint: "可複選；選擇公開回覆或推薦時，須啟用公開安全檢查。",
  private: "私訊",
  reply: "留言的私密回覆",
  replyList: "留言私密回覆",
  public: "公開回覆",
  recommend: "商品推薦",
  safeLabel: "啟用公開安全檢查",
  safeHint: "公開內容不可放入網址、聯絡方式、買家資料、訂單資料或付款連結。",
  preview: "內容預覽",
  review: "發布前確認",
  rules:
    "1. 選擇至少一種適用消息。\n2. 公開回覆與推薦需通過公開安全檢查。\n3. 系統固定範本不能覆寫。",
  qualification:
    "發送時仍會檢查權限、對話資格與消息期限；發布範本不代表可以立即發送。",
  cancel: "取消",
  check: "檢查發布內容",
  localOnly: "僅檢查本地格式；尚未發布或傳送。公開安全仍須由伺服器確認。",
  invalid:
    "請確認必填欄位、長度、識別碼及消息類型。公開回覆與推薦須勾選公開安全檢查。",
  exampleName: "商家回覆說明",
  exampleBody: "謝謝留言，我們已看到你的問題，將由商家確認後回覆。",
};
type Copy = Record<keyof typeof tw, string>;
const cn: Copy = {
  title: "消息模板",
  mobileTitle: "消息模板",
  count: "已发布版本 · 示例资料 {n} 笔",
  disclaimer: "发布不等于发送；实际发送前仍会检查连接授权与消息期限。",
  unreadable:
    "现有模板目前无法读取正文，因此暂不支持编辑。你可以新增自定义模板。",
  add: "＋ 新增自定义模板",
  manageRequired: "需要管理直播的权限才能新增模板。",
  name: "模板",
  version: "版本",
  kinds: "适用消息类型",
  safe: "公开安全",
  date: "创建时间",
  created: "创建",
  edit: "编辑",
  unavailable: "暂不可用",
  editHint: "编辑不可用：暂无正文读取功能",
  yes: "是",
  no: "否",
  empty: "暂无已发布模板。",
  formTitle: "新增自定义模板",
  formMobile: "新增模板",
  back: "← 返回消息模板",
  intro: "先确认内容与适用类型，再发布版本；发布不会发送消息。",
  nameLabel: "模板名称 *",
  nameHint: "方便团队辨认，1–120 个字符。",
  idLabel: "模板标识码 *",
  idHint:
    "1–64 个字符；首字限小写英文或数字，其余可含 /、_、-。同一标识码发布时追加版本。",
  bodyLabel: "消息正文 *",
  bodyHint: "正文最多 2,000 字符；启用公开安全检查时，所有类型均限 300 字符。",
  kindsLabel: "适用消息类型 *",
  kindsHint: "可多选；选择公开回复或推荐时，须启用公开安全检查。",
  private: "私信",
  reply: "评论的私密回复",
  replyList: "评论私密回复",
  public: "公开回复",
  recommend: "商品推荐",
  safeLabel: "启用公开安全检查",
  safeHint: "公开内容不可放入网址、联系方式、买家资料、订单资料或付款链接。",
  preview: "内容预览",
  review: "发布前确认",
  rules:
    "1. 选择至少一种适用消息。\n2. 公开回复与推荐需通过公开安全检查。\n3. 系统固定模板不能覆盖。",
  qualification:
    "发送时仍会检查权限、对话资格与消息期限；发布模板不代表可以立即发送。",
  cancel: "取消",
  check: "检查发布内容",
  localOnly: "仅检查本地格式；尚未发布或发送。公开安全仍须由服务器确认。",
  invalid:
    "请确认必填字段、长度、标识码及消息类型。公开回复与推荐须勾选公开安全检查。",
  exampleName: "商家回复说明",
  exampleBody: "谢谢留言，我们已看到你的问题，将由商家确认后回复。",
};
const en: Copy = {
  title: "Message templates",
  mobileTitle: "Message templates",
  count: "Published versions · {n} records",
  disclaimer:
    "Publishing is not sending. Connection authorization and message eligibility are checked before sending.",
  unreadable:
    "Existing template bodies cannot be read, so editing is unavailable. You can add a custom template.",
  add: "＋ Add custom template",
  manageRequired:
    "Managing live sessions permission is required to add a template.",
  name: "Template",
  version: "Version",
  kinds: "Message types",
  safe: "Public safety",
  date: "Created at",
  created: "Created",
  edit: "Edit",
  unavailable: "Unavailable",
  editHint: "Editing unavailable: body reading is not supported",
  yes: "Yes",
  no: "No",
  empty: "No published templates yet.",
  formTitle: "Add custom template",
  formMobile: "Add template",
  back: "← Back to message templates",
  intro:
    "Review content and types before publishing a version. Publishing does not send messages.",
  nameLabel: "Template name *",
  nameHint: "A recognizable name, 1–120 characters.",
  idLabel: "Template identifier *",
  idHint:
    "1–64 characters; start with a lowercase letter or digit, followed by lowercase letters, digits, /, _ or -. Publishing the same identifier appends a version.",
  bodyLabel: "Message body *",
  bodyHint:
    "Up to 2,000 characters; all types are limited to 300 with public safety checking.",
  kindsLabel: "Applicable message types *",
  kindsHint:
    "Choose one or more. Public replies and recommendations require public safety checking.",
  private: "Direct message",
  reply: "Private reply to a comment",
  replyList: "Private comment reply",
  public: "Public reply",
  recommend: "Product recommendation",
  safeLabel: "Enable public safety checking",
  safeHint:
    "Public content must not contain URLs, contact details, buyer data, order data or payment links.",
  preview: "Content preview",
  review: "Before publishing",
  rules:
    "1. Choose at least one message type.\n2. Public replies and recommendations require public safety checking.\n3. System templates cannot be overwritten.",
  qualification:
    "Permissions, conversation eligibility and message windows are checked before sending. Publishing a template does not grant sending eligibility.",
  cancel: "Cancel",
  check: "Check publishing content",
  localOnly:
    "Local format check only. Nothing has been published or sent. Public safety still requires server validation.",
  invalid:
    "Check required fields, lengths, identifier and message types. Public replies and recommendations require public safety checking.",
  exampleName: "Merchant reply guidance",
  exampleBody:
    "Thank you for your comment. We have seen your question and the merchant will reply after checking.",
};
export const messageTemplateCopy = {
  "zh-TW": tw,
  "zh-CN": cn,
  en,
} satisfies Record<Locale, Copy>;
