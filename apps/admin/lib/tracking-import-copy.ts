// Purpose: plain three-locale copy for bulk home-delivery tracking updates.
// Depends on: frozen tracking outcomes/codes; no transport or business-rule decisions.
// Used by: TrackingImport on the merchant orders page.
import type { Locale } from "@live-commerce/i18n";
const en = {
  action: "Bulk tracking update", upload: "Upload CSV", paste: "Paste rows", input: "Tracking details", template: "Download template",
  intro: "For home-delivery orders only. Check every row before confirming. Convenience-store orders use their own shipping process.",
  columns: "Order number, carrier and tracking number. In the carrier column, copy 黑貓 (Black Cat), 新竹 (Hsinchu), 郵局 (Post Office) or 其他 (Other). For Other, add a fourth column with the carrier’s name; a fifth column can contain an https tracking link.",
  limits: "Up to 500 rows and 2 MB per file. Save as CSV UTF-8. Keep tracking numbers as text to retain leading zeroes.",
  pasteHint: "Paste rows copied from a spreadsheet, or comma-separated rows. A header row is optional.",
  preview: "Check details", checking: "Checking…", reading: "Reading file…", confirm: (n: number) => `Confirm and mark ${n} shipped`, committing: "Updating…",
  summary: (apply: number, unchanged: number, failed: number) => `${apply} to ship · ${unchanged} already recorded · ${failed} need correction`,
  mail: (h: number) => `Shipping emails are sent in batches. This batch is estimated at about ${h} hour(s); other waiting messages may make it take longer.`,
  row: "Row", order: "Order number", carrier: "Carrier", tracking: "Tracking number", status: "Result", issue: "What to correct",
  carriers: { black_cat: "Black Cat", hsinchu: "Hsinchu", chunghwa_post: "Post Office", other: "Other" }, unknownCarrier: "Unrecognized carrier",
  outcomes: { apply: "Ready to ship", unchanged: "Already recorded", failed: "Needs correction" },
  result: "Tracking update results", completed: (applied: number, unchanged: number, failed: number) => `${applied} shipped · ${unchanged} already recorded · ${failed} not updated`,
  replayed: "The previous update has been confirmed. No duplicate shipment was recorded.", failed: "Download failed rows", failureHint: "Use the row numbers in the report to correct your original file. The report does not include carrier names or tracking links.",
  close: "Close", change: "Choose different details", noChanges: "No orders need shipping. Correct the flagged rows or choose another file.",
  stale: "Some orders changed after the check. Nothing was updated by this attempt. Review the new counts before confirming again.",
  uncertain: "The update’s result is not confirmed yet. Keep the same details and check the previous update; do not start a different file.",
  recover: "A previous update needs confirmation. Select or paste the same original details, then check its result.", retry: "Check the previous update",
  errors: {
    empty: "Choose a CSV file or paste at least one row.", too_large: "This file is larger than 2 MB. Split it into smaller files.", too_many_rows: "Use no more than 500 rows per file.",
    invalid_csv: "Check the columns and quotation marks, or use the template.", encoding_not_utf8: "Save the file as CSV UTF-8, or paste the rows instead.",
    required: "Fill in the order number, carrier and tracking number using the template.", invalid_order_ref: "Check the order number against the orders list.", invalid_carrier: "Copy 黑貓, 新竹, 郵局 or 其他 into the carrier column.",
    carrier_name_required: "For Other, add the carrier’s name in the fourth column.", invalid_tracking: "Use 1–64 letters, digits, spaces or hyphens, starting with a letter or digit.", invalid_url: "Use a plain https tracking link.",
    duplicate_order: "This order appears more than once. Keep one row only.", order_not_found: "This order was not found in this store.", cvs_order: "This is a convenience-store order. Use its own shipping process.",
    already_shipped: "This order is already shipped. Correct its tracking details on the order itself.", not_shippable: "This order is not ready to ship. Check its status first.", version_changed: "This order changed. Check it again.",
    nothing_to_apply: "No orders need shipping.", idempotency_conflict: "This file was used before with a different count. Check the previous update.",
    unauthorized: "Your sign-in changed. Reopen this page after signing in.", forbidden: "You do not have permission to record shipments for this store.",
    storage_unavailable: "The browser cannot keep this update’s recovery details. Enable browser storage before confirming.", different_file: "Use the same original file or pasted rows to confirm the previous update.",
    invalid_response: "The result could not be confirmed. Check the previous update before trying different details.", retry_later: "The connection is unavailable. Please try again.",
  } as Record<string, string>, genericRow: "Check this row against the original order.",
};
export type TrackingCopy = typeof en;
export const trackingCopy: Record<Locale, TrackingCopy> = {
  en,
  "zh-TW": {
    action: "批量回填運單", upload: "上傳 CSV", paste: "貼上多行", input: "運單資料", template: "下載範本",
    intro: "僅適用宅配訂單。先檢查每一行，再確認出貨。超商訂單請使用各自的寄件流程。",
    columns: "依序填入訂單編號、物流商、運單號。物流商可用黑貓、新竹、郵局或其他；選其他時，第四欄需填物流商名稱，第五欄可填 https 追蹤網址。",
    limits: "每份最多 500 行、2 MB。請另存為 CSV UTF-8，運單號以文字儲存，保留開頭的 0。", pasteHint: "可從試算表複製多行貼上，或以逗號分隔。可包含表頭。",
    preview: "檢查資料", checking: "正在檢查…", reading: "正在讀取檔案…", confirm: n => `確認回填並出貨 ${n} 筆`, committing: "正在回填…",
    summary: (a, u, f) => `${a} 筆待出貨 · ${u} 筆已回填 · ${f} 筆需修正`, mail: h => `出貨通知會分批寄出，本批預估約需 ${h} 小時；若有其他等候通知，可能較久。`,
    row: "行號", order: "訂單編號", carrier: "物流商", tracking: "運單號", status: "結果", issue: "需修正的內容",
    carriers: { black_cat: "黑貓", hsinchu: "新竹", chunghwa_post: "郵局", other: "其他" }, unknownCarrier: "無法辨識物流商", outcomes: { apply: "可出貨", unchanged: "已回填", failed: "需修正" },
    result: "回填結果", completed: (a, u, f) => `${a} 筆已出貨 · ${u} 筆原已回填 · ${f} 筆未回填`, replayed: "已確認上次回填結果，沒有重複記錄出貨。",
    failed: "下載失敗明細", failureHint: "請依明細行號修正原始檔案。明細不包含物流商名稱與追蹤網址。", close: "關閉", change: "重新選擇資料", noChanges: "目前沒有需要出貨的訂單。請修正標記的行，或選擇另一份檔案。",
    stale: "部分訂單在檢查後已更新，這次尚未回填。請確認新的筆數與結果，再繼續。", uncertain: "尚未確認這次回填的結果。請保留相同資料，先確認上次結果，不要換另一份檔案。",
    recover: "上次回填尚未確認。請選取或貼上相同的原始資料，再確認上次結果。", retry: "確認上次回填結果",
    errors: {
      empty: "請選擇 CSV 檔案，或至少貼上一行資料。", too_large: "檔案超過 2 MB，請拆成較小的檔案。", too_many_rows: "每份檔案請勿超過 500 行。", invalid_csv: "請檢查欄位與引號，或使用範本。", encoding_not_utf8: "請另存為 CSV UTF-8，或改用貼上資料。",
      required: "請填寫訂單編號、物流商與運單號。", invalid_order_ref: "請對照訂單列表，確認訂單編號。", invalid_carrier: "物流商請填黑貓、新竹、郵局或其他。", carrier_name_required: "選其他時，請在第四欄填入物流商名稱。", invalid_tracking: "運單號需為 1–64 個英文字母、數字、空格或連字號，並以字母或數字開頭。", invalid_url: "請使用一般 https 追蹤網址。",
      duplicate_order: "同一訂單重複出現，請只保留一行。", order_not_found: "本店找不到這張訂單。", cvs_order: "這是超商訂單，請使用其寄件流程。", already_shipped: "這張訂單已出貨，請在該訂單內更正運單資料。", not_shippable: "這張訂單尚不可出貨，請先確認狀態。", version_changed: "訂單已更新，請重新檢查。", nothing_to_apply: "沒有需要出貨的訂單。", idempotency_conflict: "這份檔案曾以不同筆數回填，請先確認上次結果。",
      unauthorized: "登入狀態已變更，請登入後重新開啟此頁。", forbidden: "你沒有本店記錄出貨的權限。", storage_unavailable: "瀏覽器無法保留這次回填的恢復資料，請允許瀏覽器儲存後再確認。", different_file: "請使用相同原始檔案或貼上資料，確認上次結果。", invalid_response: "無法確認結果，請先確認上次回填，再使用其他資料。", retry_later: "目前連線不順，請稍後再試。",
    }, genericRow: "請對照原始訂單檢查這一行。",
  },
  "zh-CN": {
    action: "批量回填运单", upload: "上传 CSV", paste: "粘贴多行", input: "运单资料", template: "下载模板",
    intro: "仅适用宅配订单。先检查每一行，再确认发货。超商订单请使用各自的寄件流程。", columns: "依次填写订单编号、物流商、运单号。物流商可用黑猫、新竹、邮局或其他；选择其他时，第四列需填写物流商名称，第五列可填写 https 追踪网址。",
    limits: "每份最多 500 行、2 MB。请另存为 CSV UTF-8，运单号以文本保存，保留开头的 0。", pasteHint: "可从表格复制多行粘贴，或以逗号分隔。可包含表头。",
    preview: "检查资料", checking: "正在检查…", reading: "正在读取文件…", confirm: n => `确认回填并发货 ${n} 笔`, committing: "正在回填…", summary: (a, u, f) => `${a} 笔待发货 · ${u} 笔已回填 · ${f} 笔需修正`, mail: h => `发货通知会分批发送，本批预计约需 ${h} 小时；若有其他等待通知，可能较久。`,
    row: "行号", order: "订单编号", carrier: "物流商", tracking: "运单号", status: "结果", issue: "需修正的内容", carriers: { black_cat: "黑猫", hsinchu: "新竹", chunghwa_post: "邮局", other: "其他" }, unknownCarrier: "无法识别物流商", outcomes: { apply: "可发货", unchanged: "已回填", failed: "需修正" },
    result: "回填结果", completed: (a, u, f) => `${a} 笔已发货 · ${u} 笔原已回填 · ${f} 笔未回填`, replayed: "已确认上次回填结果，没有重复记录发货。", failed: "下载失败明细", failureHint: "请依明细行号修正原始文件。明细不包含物流商名称与追踪网址。", close: "关闭", change: "重新选择资料", noChanges: "目前没有需要发货的订单。请修正标记的行，或选择另一份文件。",
    stale: "部分订单在检查后已更新，本次尚未回填。请确认新的笔数与结果，再继续。", uncertain: "尚未确认本次回填的结果。请保留相同资料，先确认上次结果，不要换另一份文件。", recover: "上次回填尚未确认。请选择或粘贴相同的原始资料，再确认上次结果。", retry: "确认上次回填结果",
    errors: {
      empty: "请选择 CSV 文件，或至少粘贴一行资料。", too_large: "文件超过 2 MB，请拆成较小的文件。", too_many_rows: "每份文件请勿超过 500 行。", invalid_csv: "请检查列和引号，或使用模板。", encoding_not_utf8: "请另存为 CSV UTF-8，或改用粘贴资料。", required: "请填写订单编号、物流商和运单号。", invalid_order_ref: "请对照订单列表，确认订单编号。", invalid_carrier: "物流商请填写黑猫、新竹、邮局或其他。", carrier_name_required: "选择其他时，请在第四列填写物流商名称。", invalid_tracking: "运单号需为 1–64 个英文字母、数字、空格或连字符，并以字母或数字开头。", invalid_url: "请使用普通 https 追踪网址。",
      duplicate_order: "同一订单重复出现，请只保留一行。", order_not_found: "本店找不到这张订单。", cvs_order: "这是超商订单，请使用其寄件流程。", already_shipped: "这张订单已发货，请在该订单内更正运单资料。", not_shippable: "这张订单尚不可发货，请先确认状态。", version_changed: "订单已更新，请重新检查。", nothing_to_apply: "没有需要发货的订单。", idempotency_conflict: "这份文件曾以不同笔数回填，请先确认上次结果。", unauthorized: "登录状态已变更，请登录后重新打开此页。", forbidden: "你没有本店记录发货的权限。", storage_unavailable: "浏览器无法保留本次回填的恢复资料，请允许浏览器存储后再确认。", different_file: "请使用相同原始文件或粘贴资料，确认上次结果。", invalid_response: "无法确认结果，请先确认上次回填，再使用其他资料。", retry_later: "目前连接不畅，请稍后重试。",
    }, genericRow: "请对照原始订单检查这一行。",
  },
};
