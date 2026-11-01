// Purpose: independent signed-frame copy, screenshots and click receipts for PARITY-MONEY's existing real-route gates.
// Depends on: @playwright/test and node fs/path; frame authority iJ9lDdFIXgTlKkt5Tvrc0X, page 4:8829, read 2026-10-30.
// Used by: customers-billing.spec.ts, reports.spec.ts, card-payments.spec.ts, settlements.spec.ts, home-cod.spec.ts.
// Invariants: MOCK/SANDBOX only; literal design expectations never imported from product copy; no DOM mutation or money calculation.
import { expect, type Page } from "@playwright/test";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import path from "node:path";

export const moneyLocales = ["zh-TW", "zh-CN", "en"] as const;
export type MoneyLocale = (typeof moneyLocales)[number];
export const moneyWidths = [1440, 390] as const;
export const moneyFrames = {
  overview: ["4:12097", "4:12246"], daily: ["1173:10738", "1173:10739"],
  basis: ["6:18814", "6:18963"], financeDates: ["1173:10742", "1173:10743"],
  financeCSV: ["1173:10744", "1173:10745"], financeFailure: ["1173:10746", "1173:10747"],
  reportDates: ["1151:9432", "1151:9433"], reportFailure: ["1151:9434", "1151:9435"],
  statements: ["1173:10740", "1173:10741"], statement: ["6:18572", "6:18737"],
  returned: ["1248:8745", "1248:8823"], carriedReturn: ["1248:8879", "1248:8960"],
  card: ["6:20884", "6:20993"], billing: ["21:677", "21:1020"],
  billingRefusal: ["30:1909", "30:1960"], portal: ["750:3455", "750:3597"],
  unbilled: ["1220:10379", "1220:10444"], cancelling: ["1220:10490", "1220:10555"],
  overdue: ["1220:10601", "1220:10666"], restricted: ["1220:10712", "1220:10777"],
  updating: ["1227:14766", "1227:14956"], cancelled: ["1227:14848", "1227:15015"],
  cod: ["94:2153", "94:2223"], codReview: ["339:2831", "339:2845"],
} as const;
export type MoneyFrame = keyof typeof moneyFrames;

// zh-TW comes verbatim from the signed product text. Prototype-only notices/actions stay off product screens per card.
// en / zh-CN are independent semantic translations; they are intentionally not imported from implementation keys.
export const moneyDesign = {
  "zh-TW": {
    finance: "財務總覽", other: "其他收款 · 分開核對", cardCount: "信用卡收款筆數", net: "此口徑淨額",
    daily: "每日明細", dailyTitle: "財務每日明細", statements: "結算單", basis: "了解財務口徑", basisTitle: "收款與結算口徑",
    financeDates: "財務日期範圍", chooseDates: "選擇日期範圍", from: "開始日期", to: "結束日期", apply: "套用日期", cancel: "取消",
    csv: "匯出財務 CSV", csvReady: "準備財務 CSV", download: "下載 CSV", backFinance: "返回財務總覽",
    financeFailure: "財務讀取", failure: "讀取失敗", retry: "重新讀取", reportFailure: "報表讀取",
    statement: "結算單明細", composition: "本期結算組成", openStatement: "查看此結算單", backStatements: "返回結算單列表",
    returned: "結算單明細 · 撥付已退回", returnedStatus: "撥付已退回 · 非已結清", carriedReturn: "結算單明細 · 退回款帶回",
    returnContribution: "撥付已退回帶回", card: "收款設定", unified: "平台統一收款", cardBasis: "付款成功，不等於已結算",
    cardIntro: "平台統一收款，已可使用。你不需要申請或填寫任何收款資料。", settlementLink: "查看結算與對帳",
    billing: "方案與帳單", standing: "帳戶狀態", payment: "付款方式與發票", usage: "本期用量", manage: "管理付款與發票",
    billingRefusal: "訂閱需要處理", backBilling: "回到方案與帳單", portalIntro: "開啟 Stripe 託管頁面，可更換信用卡、查看發票與取消訂閱。按下後會離開本站，前往 Stripe。",
    unbilled: "未訂閱", good: "狀態良好", overdue: "付款逾期", restricted: "已受限", cancellation: "週期結束後終止",
    updating: "已收到付款，正在更新訂閱…", cancelled: "已取消結帳，未扣款。",
    cod: "貨到付款設定", codReview: "貨到付款儲存檢查", codEnabled: "在結帳時提供貨到付款", cap: "單筆金額上限（TWD）",
    surcharge: "手續費（TWD）", carrier: "貨運業者", save: "儲存設定", backCod: "返回貨到付款設定",
    oldFinance: "財務", oldApply: "查看", oldBilling: "帳單", oldCard: "信用卡收款", oldCod: "貨到付款", legacyCaptured: "已請款",
  },
  "zh-CN": {
    finance: "财务总览", other: "其他收款 · 分开核对", cardCount: "信用卡收款笔数", net: "此口径净额",
    daily: "每日明细", dailyTitle: "财务每日明细", statements: "结算单", basis: "了解财务口径", basisTitle: "收款与结算口径",
    financeDates: "财务日期范围", chooseDates: "选择日期范围", from: "开始日期", to: "结束日期", apply: "应用日期", cancel: "取消",
    csv: "导出财务 CSV", csvReady: "准备财务 CSV", download: "下载 CSV", backFinance: "返回财务总览",
    financeFailure: "财务读取", failure: "读取失败", retry: "重新读取", reportFailure: "报表读取",
    statement: "结算单明细", composition: "本期结算组成", openStatement: "查看此结算单", backStatements: "返回结算单列表",
    returned: "结算单明细 · 拨付已退回", returnedStatus: "拨付已退回 · 非已结清", carriedReturn: "结算单明细 · 退回款带回",
    returnContribution: "拨付已退回带回", card: "收款设置", unified: "平台统一收款", cardBasis: "付款成功，不等于已结算",
    cardIntro: "平台统一收款，已可使用。你不需要申请或填写任何收款资料。", settlementLink: "查看结算与对账",
    billing: "方案与账单", standing: "账户状态", payment: "付款方式与发票", usage: "本期用量", manage: "管理付款与发票",
    billingRefusal: "订阅需要处理", backBilling: "回到方案与账单", portalIntro: "打开 Stripe 托管页面，可更换信用卡、查看发票与取消订阅。点击后将离开本站，前往 Stripe。",
    unbilled: "未订阅", good: "状态良好", overdue: "付款逾期", restricted: "已受限", cancellation: "周期结束后终止",
    updating: "已收到付款，正在更新订阅…", cancelled: "已取消结账，未扣款。",
    cod: "货到付款设置", codReview: "货到付款保存检查", codEnabled: "在结账时提供货到付款", cap: "单笔金额上限（TWD）",
    surcharge: "手续费（TWD）", carrier: "货运公司", save: "保存设置", backCod: "返回货到付款设置",
    oldFinance: "财务", oldApply: "查看", oldBilling: "账单", oldCard: "信用卡收款", oldCod: "货到付款", legacyCaptured: "已请款",
  },
  en: {
    finance: "Finance overview", other: "Other payments · Reconcile separately", cardCount: "Card payment count", net: "Net on this basis",
    daily: "Daily details", dailyTitle: "Daily finance details", statements: "Settlement statements", basis: "Understand finance figures", basisTitle: "Payments and settlements",
    financeDates: "Finance date range", chooseDates: "Choose date range", from: "Start date", to: "End date", apply: "Apply dates", cancel: "Cancel",
    csv: "Export finance CSV", csvReady: "Prepare finance CSV", download: "Download CSV", backFinance: "Back to finance overview",
    financeFailure: "Finance loading", failure: "Unable to load", retry: "Reload", reportFailure: "Report loading",
    statement: "Settlement statement details", composition: "This settlement's breakdown", openStatement: "View this statement", backStatements: "Back to settlement statements",
    returned: "Settlement statement details · Payout returned", returnedStatus: "Payout returned · Not settled", carriedReturn: "Settlement statement details · Returned funds carried in",
    returnContribution: "Returned payout carried in", card: "Payment settings", unified: "Platform payment collection", cardBasis: "Payment received does not mean settled",
    cardIntro: "Platform payments are ready to use. You do not need to apply or enter payment account details.", settlementLink: "View settlements and reconciliation",
    billing: "Plans and billing", standing: "Account status", payment: "Payment method and invoices", usage: "Usage this period", manage: "Manage payment and invoices",
    billingRefusal: "Subscription needs attention", backBilling: "Back to plans and billing", portalIntro: "Open Stripe's hosted page to change your card, view invoices or cancel your subscription. This takes you away from this site to Stripe.",
    unbilled: "No subscription", good: "In good standing", overdue: "Payment overdue", restricted: "Restricted", cancellation: "Ends at the end of the period",
    updating: "Payment received. Updating your subscription…", cancelled: "Checkout was cancelled. Nothing was charged.",
    cod: "Cash on delivery settings", codReview: "Review cash on delivery settings", codEnabled: "Offer cash on delivery at checkout", cap: "Per-order amount cap (TWD)",
    surcharge: "Collection fee (TWD)", carrier: "Carrier", save: "Save settings", backCod: "Back to cash on delivery settings",
    oldFinance: "Finance", oldApply: "Show", oldBilling: "Billing", oldCard: "Card payments", oldCod: "Cash on delivery", legacyCaptured: "Captured",
  },
} as const;

export async function moneyClick(page: Page, evidence: string, control: string, expected: string, action: () => Promise<void>) {
  const row = { page: new URL(page.url()).pathname, control, operation: "real user interaction", expected, actual: "", result: "FAIL" };
  try { await action(); row.actual = expected; row.result = "PASS"; }
  catch (error) { row.actual = error instanceof Error ? error.message.slice(0, 300) : "assertion failed"; throw error; }
  finally {
    const file = path.join(evidence, "parity-money-click-ledger.json");
    let previous: unknown[] = [];
    try { previous = JSON.parse(await readFile(file, "utf8")); } catch { /* first receipt */ }
    await writeFile(file, JSON.stringify([...previous, row], null, 2));
  }
}

export async function moneyShot(page: Page, evidence: string, frame: MoneyFrame, locale: MoneyLocale, width: number) {
  const id = moneyFrames[frame][width === 390 ? 1 : 0];
  // GitHub artifact names reject ':': normalize the Figma id to its standard URL spelling.
  const directory = path.join(evidence, "parity", ...(locale === "zh-TW" ? [] : [locale]));
  await mkdir(directory, { recursive: true });
  await page.screenshot({ path: path.join(directory, `${id.replace(":", "-")}.png`), animations: "disabled", fullPage: false });
  // READ/MEASURE only: screenshots and geometry do not substitute for the real clicks recorded above.
  expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth), `PARITY-MONEY ${id}: page overflow`).toBeLessThanOrEqual(1);
  await expect(page.getByText(/原型：切換狀態|原型・示範資料|互動原型|DESIGN ·|原型僅展示|Prototype.*switch/i)).toHaveCount(0);
}

export async function moneyOverview(page: Page, locale: MoneyLocale) {
  const c = moneyDesign[locale];
  await expect(page.getByRole("heading", { level: 1, name: c.finance, exact: true }), "PARITY-MONEY 4:12097/4:12246: signed overview heading").toBeVisible();
  await expect(page.getByRole("heading", { name: c.other, exact: true })).toBeVisible();
  await expect(page.getByText(c.cardCount, { exact: true })).toBeVisible();
  await expect(page.getByText(c.net, { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: c.daily, exact: true })).toBeVisible();
  await expect(page.getByRole("link", { name: c.statements, exact: true })).toBeVisible();
  await expect(page.getByRole("heading", { level: 1, name: c.oldFinance, exact: true })).toHaveCount(0);
  await expect(page.getByRole("button", { name: c.oldApply, exact: true })).toHaveCount(0);
  await expect(page.getByTestId("finance-table")).toHaveCount(0); // daily ledger belongs in the signed daily-details state
}
