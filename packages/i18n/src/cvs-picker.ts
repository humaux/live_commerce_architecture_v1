// Purpose: signed buyer store-picker copy for all three shipping locales.
// Depends on: Locale type from the shared i18n entry point.
// Used by: storefront CvsStorePicker and CVS locale acceptance.
// Invariants: directory membership never promises delivery; raw recipient data is not copy.
import type { Locale } from "./index.ts";

const tw = {
  title: "選擇取貨門市", open: "選擇取貨門市", close: "關閉", search: "搜尋", region: "依地區",
  browse: "依縣市／區找門市", helper: "輸入門市地址、名稱或編號", best: "最符合", back: "返回",
  query: "地址、門市名稱或編號", county: "縣市", district: "行政區", empty: "找不到符合的門市",
  confirm: "確認取貨門市", review: "核對取貨門市", change: "更換", more: "載入更多",
  missing: "此編號不在門市清單：", mismatch: "請核對編號或切換品牌。", hint: "請核對地址或切換品牌。",
  source: "資料：綠界門市清單 · 更新 {date}", loading: "載入中…", failed: "暫時無法載入門市，請重試。",
  retry: "重試", confirming: "確認中…", choose: "請先選擇門市", busy: "請稍後再確認", other: "其他",
  chains: { UNIMART: "7-ELEVEN", UNIMARTFREEZE: "7-ELEVEN", FAMI: "全家", HILIFE: "萊爾富", OKMART: "OK" },
};
type Copy = typeof tw;
export const cvsPickerCopy: Record<Locale, Copy> = {
  "zh-TW": tw,
  "zh-CN": {
    title: "选择自提门店", open: "选择自提门店", close: "关闭", search: "搜索", region: "按地区",
    browse: "按县市／区查门店", helper: "输入自提门店的地址、名称或编号", best: "最匹配", back: "返回",
    query: "地址、门店名称或编号", county: "县市", district: "区", empty: "未找到匹配的门店",
    confirm: "确认自提门店", review: "核对自提门店", change: "更换", more: "加载更多",
    missing: "该编号未收录在门店清单中：", mismatch: "请核对编号，或换一个品牌。", hint: "请核对地址，或换一个品牌。",
    source: "数据：绿界门店清单 · 更新 {date}", loading: "加载中…", failed: "暂时无法加载门店，请重试。",
    retry: "重试", confirming: "确认中…", choose: "请先选择门店", busy: "请稍后再确认", other: "其他",
    chains: { UNIMART: "7-ELEVEN", UNIMARTFREEZE: "7-ELEVEN", FAMI: "全家", HILIFE: "莱尔富", OKMART: "OK" },
  },
  en: {
    title: "Choose pickup store", open: "Choose pickup store", close: "Close", search: "Search", region: "By area",
    browse: "Browse by city / district", helper: "Enter a pickup store address, name or code", best: "Best match", back: "Back",
    query: "Address, store name or code", county: "City", district: "District", empty: "No matching stores",
    confirm: "Confirm pickup store", review: "Review pickup store", change: "Change", more: "Load more",
    missing: "Code not in store list: ", mismatch: "Check the code or switch brand.", hint: "Check the address or switch brand.",
    source: "Source: ECPay stores · Updated {date}", loading: "Loading…", failed: "Unable to load stores. Please retry.",
    retry: "Retry", confirming: "Confirming…", choose: "Choose a store first", busy: "Please wait before confirming again", other: "Other",
    chains: { UNIMART: "7-ELEVEN", UNIMARTFREEZE: "7-ELEVEN", FAMI: "FamilyMart", HILIFE: "Hi-Life", OKMART: "OK" },
  },
};
