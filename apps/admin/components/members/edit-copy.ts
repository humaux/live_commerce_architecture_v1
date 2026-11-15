// Purpose: localized profile-edit copy from promoted M7 frames and frozen member contract.
// Depends on: supported admin locales; no runtime data.
// Used by: MemberEditDialog.
export const memberEditCopy = {
  "zh-TW": {
    edit:"編輯資料", title:"編輯會員資料", name:"顯示名稱（必填）", contact:"聯絡備註（選填）", notes:"備註（選填）",
    hint:"只給你自己辨識用，不會傳給買家，也不會自動比對。", points:"積分不能在這裡修改，請用「調整積分」。",
    save:"儲存", cancel:"取消", close:"關閉", refresh:"重新讀取", loading:"讀取中…", pending:"儲存中…",
    empty:"請填寫顯示名稱。", invalidName:"顯示名稱須為 1–60 個字，不能包含控制字元。", invalidContact:"聯絡備註最多 120 個字。", invalidNotes:"備註最多 1000 個字。",
    conflict:"這位會員剛被其他人修改，你的變更沒有儲存。請重新讀取最新資料後再修改。",
    unknownTitle:"儲存結果未確認", unknownBadge:"待確認", unknown:"網路回應中斷，變更可能已經儲存。請先重新讀取，確認後再決定是否修改，勿直接再次儲存。", readOnly:"重新讀取只會查詢，不會重送。",
    refused:"無法儲存變更。請重新讀取最新資料。", erased:"會員資料已刪除。", unchanged:"請先修改資料。",
  },
  "zh-CN": {
    edit:"编辑资料", title:"编辑会员资料", name:"显示名称（必填）", contact:"联系备注（选填）", notes:"备注（选填）",
    hint:"仅供你自己辨识，不会发送给买家，也不会自动匹配。", points:"积分不能在这里修改，请使用“调整积分”。",
    save:"保存", cancel:"取消", close:"关闭", refresh:"重新读取", loading:"读取中…", pending:"保存中…",
    empty:"请填写显示名称。", invalidName:"显示名称须为 1–60 个字，不能包含控制字符。", invalidContact:"联系备注最多 120 个字。", invalidNotes:"备注最多 1000 个字。",
    conflict:"这位会员刚被其他人修改，你的变更没有保存。请重新读取最新资料后再修改。",
    unknownTitle:"保存结果未确认", unknownBadge:"待确认", unknown:"网络响应中断，变更可能已经保存。请先重新读取，确认后再决定是否修改，请勿直接再次保存。", readOnly:"重新读取只会查询，不会重新发送。",
    refused:"无法保存变更。请重新读取最新资料。", erased:"会员资料已删除。", unchanged:"请先修改资料。",
  },
  en: {
    edit:"Edit profile", title:"Edit member profile", name:"Display name (required)", contact:"Contact note (optional)", notes:"Notes (optional)",
    hint:"For your own reference. Never sent to the buyer or matched automatically.", points:"To change points, use Adjust points.",
    save:"Save", cancel:"Cancel", close:"Close", refresh:"Reload current profile", loading:"Loading…", pending:"Saving…",
    empty:"Enter a display name.", invalidName:"Use 1–60 characters without control characters.", invalidContact:"Contact note must be at most 120 characters.", invalidNotes:"Notes must be at most 1000 characters.",
    conflict:"Someone else changed this member. Your changes were not saved. Reload the current profile before editing again.",
    unknownTitle:"Save result unconfirmed", unknownBadge:"Unconfirmed", unknown:"The response was interrupted. Your changes may have been saved. Reload and check before deciding whether to edit again. Do not save again directly.", readOnly:"Reload only reads the profile. It never resends your changes.",
    refused:"Changes could not be saved. Reload the current profile.", erased:"This member has been erased.", unchanged:"Change a field before saving.",
  },
} as const;
