// Purpose: member privacy copy; depends on LP1 §3 and promoted M7 sources; used by MemberPrivacyDialogs.
export const privacyCopy = {
  "zh-TW": {
    export:"匯出資料", erase:"刪除資料", exportTitle:"匯出會員資料", eraseTitle:"刪除會員資料？",
    exportBody:"下載會員資料與全部積分紀錄（JSON 檔）。檔案不含員工姓名。", caution:"檔案可能含有個人資料，請妥善保存。",
    retained:"點數紀錄保留，會員姓名、聯絡備註與備註會移除；已建立訂單上的訂購人姓名不會變更。",
    irreversible:"積分紀錄中的說明文字也會移除。刪除後無法復原，這位會員也不能再配積分。",
    download:"下載", preparing:"正在準備檔案…", downloaded:"已下載 member-export.json。", failed:"下載沒有完成。可使用同一請求再試一次，不會重複記錄匯出。",
    unknownExport:"匯出結果未確認，檔案尚未下載。可使用同一請求再試一次，不會重複記錄匯出。",
    tooLarge:"匯出資料超過 1 MiB 上限，無法匯出。", unknownTitle:"刪除結果未確認", unknownErase:"會員資料可能已經刪除。請先重新讀取，勿再次刪除。", readOnly:"重新讀取只會查詢，不會重送。",
    refused:"操作未完成。請重新讀取最新資料。", tombstone:"已刪除的會員", retry:"再試一次", refresh:"重新讀取", close:"關閉", cancel:"取消", done:"完成", pending:"處理中…",
  },
  "zh-CN": {
    export:"导出资料", erase:"删除资料", exportTitle:"导出会员资料", eraseTitle:"删除会员资料？",
    exportBody:"下载会员资料与全部积分记录（JSON 文件）。文件不含员工姓名。", caution:"文件可能包含个人资料，请妥善保存。",
    retained:"点数记录保留，会员姓名、联系备注与备注会移除；已建立订单上的订购人姓名不会变更。",
    irreversible:"积分记录中的说明文字也会移除。删除后无法恢复，这位会员也不能再配积分。",
    download:"下载", preparing:"正在准备文件…", downloaded:"已下载 member-export.json。", failed:"下载未完成。可使用同一请求重试，不会重复记录导出。",
    unknownExport:"导出结果未确认，文件尚未下载。可使用同一请求重试，不会重复记录导出。",
    tooLarge:"导出资料超过 1 MiB 上限，无法导出。", unknownTitle:"删除结果未确认", unknownErase:"会员资料可能已经删除。请先重新读取，请勿再次删除。", readOnly:"重新读取只会查询，不会重发。",
    refused:"操作未完成。请重新读取最新资料。", tombstone:"已删除的会员", retry:"重试", refresh:"重新读取", close:"关闭", cancel:"取消", done:"完成", pending:"处理中…",
  },
  en: {
    export:"Export data", erase:"Erase data", exportTitle:"Export member data", eraseTitle:"Erase member data?",
    exportBody:"Download the member record and full points ledger as JSON. Staff names are excluded.", caution:"This file may contain personal data. Keep it safe.",
    retained:"Points records are retained. The member name, contact note and notes will be removed; buyer names on existing orders will not change.",
    irreversible:"Ledger note text will also be removed. Erasure cannot be undone and this member cannot receive further points.",
    download:"Download", preparing:"Preparing file…", downloaded:"Downloaded member-export.json.", failed:"Download did not finish. Retry the same request without recording another export.",
    unknownExport:"Export outcome is unconfirmed; no file was downloaded. Retry the same request without recording another export.",
    tooLarge:"Export exceeds the 1 MiB limit and cannot be downloaded.", unknownTitle:"Erasure outcome unconfirmed", unknownErase:"The member may already be erased. Read the current record before taking further action. Do not erase again.", readOnly:"Refresh only reads; it does not resend.",
    refused:"The operation did not finish. Read the latest record.", tombstone:"Erased member", retry:"Retry", refresh:"Refresh", close:"Close", cancel:"Cancel", done:"Done", pending:"Processing…",
  },
};
