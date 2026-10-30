// Purpose: Bind real growth routes to the signed C3 desktop/mobile Figma states.
// Depends on: Playwright and the existing ads/attribution Go-owned MOCK browser fixtures.
// Used by: ads.spec.ts and attribution.spec.ts; browser ownership follows these imports.
// Invariants: real clicks/fills only; no Meta publication, invented metrics or client-calculated money.
import { expect, test, type Page } from "@playwright/test";
import { mkdir, appendFile } from "node:fs/promises";
import path from "node:path";
import { instantToTaipei } from "../../packages/format/src/index.ts";

type Environment = {
  origin: string; store: string; evidence: string;
  login: (page: Page) => Promise<void>;
};
const devices = [
  { width: 1440, height: 900, side: 0 },
  { width: 390, height: 844, side: 1 },
] as const;
const frames = {
  editor: ["6:20130", "6:20255"], source: ["1049:6945", "1049:7058"],
  settings: ["1049:7120", "1049:7233"], review: ["1049:7295", "1049:7408"],
  saved: ["1049:7470", "1049:7583"], status: ["1049:7645", "1049:7758"],
  leave: ["1049:7820", "1049:7933"], conflict: ["1049:7995", "1049:8108"],
  detail: ["5:17304", "5:17438"], assets: ["5:17516", "5:17633"],
  legend: ["1134:9548", "1134:9549"], loading: ["1134:9550", "1134:9551"],
  list: ["4:11612", "4:11750"], index: ["6:23064", "6:23199"],
  results: ["1151:9420", "1151:9421"], attribution: ["1151:9422", "1151:9423"],
  breakdown: ["1151:9428", "1151:9429"],
} as const;

async function shot(page: Page, env: Environment, frame: string) {
  const directory = path.join(env.evidence, "parity");
  await mkdir(directory, { recursive: true });
  // READ/MEASURE only. Screenshots document the real route, never /ui-gallery.
  expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1);
  await page.screenshot({ path: path.join(directory, `${frame}.png`), animations: "disabled" });
  await appendFile(path.join(directory, "frames.jsonl"), JSON.stringify({ frame, route: page.url(), screenshot: `parity/${frame}.png` }) + "\n");
}
async function click(page: Page, env: Environment, name: string, expected: string) {
  await page.getByRole("button", { name, exact: true }).click();
  await expect(page.getByRole("heading", { name: expected, exact: true })).toBeVisible();
  await appendFile(path.join(env.evidence, "growth-click-ledger.jsonl"), JSON.stringify({ route: page.url(), control: name, operation: "click", expected, actual: "visible", result: "PASS" }) + "\n");
}
async function noLegacyAds(page: Page) {
  await expect(page.getByRole("heading", { level: 1, name: "廣告", exact: true })).toHaveCount(0);
  // Separate task screens replace the legacy stack of connection, editor, CAPI and report panels.
  await expect(page.getByTestId("ads-capi")).toHaveCount(0);
  await expect(page.getByTestId("ads-report")).toHaveCount(0);
}
async function newDraft(page: Page, env: Environment) {
  await page.goto(`${env.origin}/zh-TW/ads?store=${env.store}`);
  await page.getByTestId("ads-new-draft").click();
  await expect(page.getByRole("heading", { name: "建立廣告草稿", exact: true })).toBeVisible();
  await noLegacyAds(page);
}
async function fillDraft(page: Page, source: string) {
  await page.getByRole("button", { name: "選擇貼文或商品", exact: true }).click();
  // The original MA09 fixture saved this source through the real API; no invented source catalogue.
  await page.locator(`[data-source-ref="${source}"]`).getByRole("button", { name: "使用這個來源", exact: true }).click();
  await page.getByRole("button", { name: "修改預算、排期與對象", exact: true }).click();
  await page.getByTestId("ads-f-budget").fill("1500");
  await page.getByTestId("ads-f-starts").fill(instantToTaipei(new Date(Date.now() + 3600_000).toISOString()));
  await page.getByTestId("ads-f-ends").fill(instantToTaipei(new Date(Date.now() + 25 * 3600_000).toISOString()));
  await page.getByRole("button", { name: "套用到草稿", exact: true }).click();
}

/** Register inside the existing non-refusal ads fixture; all new writes remain local MOCK drafts. */
export function registerGrowthAdsParity(env: Environment & { pageAsset: string }) {
  test.describe("PARITY-GROWTH signed ads states", () => {
    test.describe.configure({ mode: "default" });
    for (const device of devices) {
      const frame = (state: keyof typeof frames) => frames[state][device.side];
      test(`C3 list, state explanations, assets and read feedback ${device.width}`, async ({ page }) => {
        await env.login(page);
        await page.setViewportSize({ width: device.width, height: device.height });
        await page.goto(`${env.origin}/zh-TW/ads?store=${env.store}`);
        await expect(page.getByRole("heading", { name: "廣告草稿", exact: true })).toBeVisible();
        await expect(page.getByRole("button", { name: "新增草稿", exact: true })).toBeVisible();
        await expect(page.getByText("先核對版本，再核准與發布。內部核准不等於 Meta 審核通過。", { exact: true })).toBeVisible();
        await noLegacyAds(page);
        await shot(page, env, frame("list"));
        await click(page, env, "狀態說明", "狀態各自代表什麼");
        for (const state of ["內部已核准", "已建立 · 尚未啟用", "外部結果待確認", "已確認暫停"])
          await expect(page.getByText(state, { exact: true })).toBeVisible();
        await shot(page, env, frame("legend"));
        await page.getByRole("button", { name: "返回廣告列表", exact: true }).click();
        await click(page, env, "查看廣告帳號連結", "資產授權");
        await expect(page.getByRole("heading", { name: "已授權資產", exact: true })).toBeVisible();
        await expect(page.getByRole("button", { name: "重新授權", exact: true })).toBeVisible();
        await shot(page, env, frame("assets"));
        // An unavailable saved selection uses the signed old-detail recovery screen.
        await page.goto(`${env.origin}/zh-TW/ads?store=${env.store}&draft=00000000-0000-4000-8000-000000000001`);
        await expect(page.getByRole("heading", { name: "從列表開啟正確的草稿", exact: true })).toBeVisible();
        await shot(page, env, frame("detail"));
        await page.getByRole("button", { name: "前往廣告列表", exact: true }).click();
        await page.goto(`${env.origin}/zh-TW/ads?store=${env.store}`);
        let release!: () => void;
        const hold = new Promise<void>(resolve => { release = resolve; });
        // Fixture preparation: delay a REAL read, preserve its response body and every write.
        await page.route("**/api/stores/*/ads/drafts", async route => { await hold; await route.continue(); });
        try {
          await page.getByRole("button", { name: "重新讀取", exact: true }).click();
          await expect(page.getByRole("heading", { name: "正在讀取廣告列表", exact: true })).toBeVisible();
          await shot(page, env, frame("loading"));
        } finally { release(); await page.unroute("**/api/stores/*/ads/drafts"); }
        await expect(page.getByRole("heading", { name: "廣告草稿", exact: true })).toBeVisible();
      });
      test(`C3 editor, settings, leave and source ${device.width}`, async ({ page }) => {
        await env.login(page); await page.setViewportSize({ width: device.width, height: device.height }); await newDraft(page, env);
        for (const label of ["01 · 選擇推廣內容", "02 · 總預算與排期", "03 · 投放對象與身份"])
          await expect(page.getByText(label, { exact: true })).toBeVisible();
        await shot(page, env, frame("editor"));
        await click(page, env, "選擇貼文或商品", "選擇這次要推廣的內容");
        await expect(page.getByRole("button", { name: "保留原選擇並返回", exact: true })).toBeVisible();
        await shot(page, env, frame("source"));
        await page.getByRole("button", { name: "保留原選擇並返回", exact: true }).click();
        await click(page, env, "修改預算、排期與對象", "修改預算、排期與對象");
        await expect(page.getByText("自動版位", { exact: true })).toBeVisible();
        await page.getByTestId("ads-f-budget").fill("1500");
        await shot(page, env, frame("settings"));
        await page.getByRole("button", { name: "套用到草稿", exact: true }).click();
        await click(page, env, "離開", "要保留這些未儲存的修改嗎？");
        await shot(page, env, frame("leave"));
        await page.getByRole("button", { name: "繼續編輯", exact: true }).click();
        await page.getByRole("button", { name: "修改預算、排期與對象", exact: true }).click();
        await expect(page.getByTestId("ads-f-budget")).toHaveValue("1500");
        await page.getByRole("button", { name: "取消", exact: true }).click();
      });
      test(`C3 persisted saved, review and status ${device.width}`, async ({ page }) => {
        await env.login(page); await page.setViewportSize({ width: device.width, height: device.height }); await newDraft(page, env);
        await fillDraft(page, `${env.pageAsset}_1234567890`);
        const response = page.waitForResponse(r => r.request().method() === "POST" && /\/ads\/drafts$/.test(new URL(r.url()).pathname));
        await click(page, env, "儲存草稿", "草稿已儲存");
        const saved = await response;
        expect(saved.status()).toBe(201);
        const draft = await saved.json() as { id: string };
        await shot(page, env, frame("saved"));
        await click(page, env, "核對並核准", "核准前確認");
        for (const fact of ["推廣內容", "整段總預算與排期", "投放對象", "核准與發布分開"])
          await expect(page.getByText(fact, { exact: true })).toBeVisible();
        await expect(page.getByTestId("ads-detail")).toContainText("1,500");
        await shot(page, env, frame("review"));
        // Inspect status only; this parity journey never approves or publishes an ad.
        await page.goto(`${env.origin}/zh-TW/ads?store=${env.store}&draft=${draft.id}`);
        await expect(page.getByRole("button", { name: "查看廣告狀態", exact: true })).toBeVisible();
        await click(page, env, "查看廣告狀態", "尚未讀取廣告狀態");
        await shot(page, env, frame("status"));
        await page.getByRole("button", { name: "前往廣告列表", exact: true }).click();
        await page.getByTestId(`ads-open-${draft.id}`).click();
        await expect(page.getByTestId("ads-detail")).toContainText("1,500");
        await page.reload();
        await expect(page.getByTestId("ads-detail")).toContainText("1,500");
      });
      test(`C3 concurrent edit preserves local work ${device.width}`, async ({ page, context }) => {
        await env.login(page); await page.setViewportSize({ width: device.width, height: device.height }); await newDraft(page, env);
        await fillDraft(page, `${env.pageAsset}_1234567890`);
        const created = page.waitForResponse(r => r.request().method() === "POST" && /\/ads\/drafts$/.test(new URL(r.url()).pathname));
        await page.getByRole("button", { name: "儲存草稿", exact: true }).click();
        const draft = await (await created).json() as { id: string };
        const url = `${env.origin}/zh-TW/ads?store=${env.store}&draft=${draft.id}`;
        await page.goto(url); await page.getByTestId("ads-edit").click();
        await page.getByRole("button", { name: "修改預算、排期與對象", exact: true }).click();
        const other = await context.newPage();
        try {
          await other.goto(url); await other.getByTestId("ads-edit").click();
          await other.getByRole("button", { name: "修改預算、排期與對象", exact: true }).click();
          await other.getByTestId("ads-f-budget").fill("1700");
          await other.getByRole("button", { name: "套用到草稿", exact: true }).click();
          const updated = other.waitForResponse(r => r.request().method() === "PUT" && r.url().endsWith(`/drafts/${draft.id}`));
          await other.getByRole("button", { name: "儲存草稿", exact: true }).click();
          expect((await updated).ok()).toBe(true);
          await page.getByTestId("ads-f-budget").fill("1600");
          await page.getByRole("button", { name: "套用到草稿", exact: true }).click();
          await click(page, env, "儲存草稿", "另一位管理員已更新草稿");
          await expect(page.getByText("這次修改尚未覆蓋伺服器內容。", { exact: true })).toBeVisible();
          await shot(page, env, frame("conflict"));
          await page.getByRole("button", { name: "返回保留修改", exact: true }).click();
          await page.getByRole("button", { name: "修改預算、排期與對象", exact: true }).click();
          await expect(page.getByTestId("ads-f-budget")).toHaveValue("1600");
          await page.getByRole("button", { name: "取消", exact: true }).click();
          await page.getByRole("button", { name: "儲存草稿", exact: true }).click();
          await page.getByRole("button", { name: "讀取最新草稿", exact: true }).click();
          await page.getByTestId("ads-edit").click();
          await page.getByRole("button", { name: "修改預算、排期與對象", exact: true }).click();
          await expect(page.getByTestId("ads-f-budget")).toHaveValue("1700");
          await page.reload();
          await expect(page.getByTestId("ads-detail")).toContainText("1,700");
        } finally { await other.close(); }
      });
    }
    test("C3 ads main screen in all three locales", async ({ page }) => {
      await env.login(page);
      for (const [locale, heading, action] of [["zh-TW", "廣告草稿", "新增草稿"], ["zh-CN", "广告草稿", "新增草稿"], ["en", "Ad drafts", "New draft"]]) {
        await page.goto(`${env.origin}/${locale}/ads?store=${env.store}`);
        await expect(page.getByRole("heading", { level: 1, name: heading, exact: true })).toBeVisible();
        await expect(page.getByRole("button", { name: action, exact: true })).toBeVisible();
      }
    });
  });
}

/** Uses the original real PG/Go attribution fixture; no reports or balances are synthesized. */
export function registerGrowthAttributionParity(env: Environment & { draftID: string; from: string; to: string }) {
  for (const device of devices) test(`C3 signed attribution task screens ${device.width}`, async ({ page }) => {
    await env.login(page); await page.setViewportSize({ width: device.width, height: device.height });
    const base = `${env.origin}/zh-TW/ads/attribution?store=${env.store}&from=${env.from}&to=${env.to}`;
    await page.goto(base);
    await expect(page.getByRole("heading", { name: "歸因報表", exact: true })).toBeVisible();
    await expect(page.getByRole("heading", { name: "廣告歸因與直播復盤", exact: true })).toHaveCount(0);
    await expect(page.getByText("先選一份廣告草稿或一場直播，再核對它帶來的訂單。", { exact: true })).toBeVisible();
    await expect(page.getByTestId("attribution-session-panel")).toHaveCount(0);
    await expect(page.getByTestId("attribution-draft-panel")).toHaveCount(0);
    await shot(page, env, frames.index[device.side]);
    await click(page, env, "成效總覽", "成效總覽");
    for (const label of ["本平台 · 信用卡收付款", "Meta · 刊登成效", "Meta · 回報購買"])
      await expect(page.getByRole("heading", { name: label, exact: true })).toBeVisible();
    await shot(page, env, frames.results[device.side]);
    await page.getByRole("button", { name: "查看廣告／直播歸因", exact: true }).click();
    await page.getByTestId(`attribution-open-${env.draftID}`).click();
    await expect(page.getByRole("heading", { name: "廣告歸因", exact: true })).toBeVisible();
    await expect(page.getByText("待收款 · 不算入已收款", { exact: true })).toBeVisible();
    await shot(page, env, frames.attribution[device.side]);
    await click(page, env, "Meta 分組明細", "Meta 分組明細");
    await expect(page.getByText("Meta 彙總，不是買家個人資料。", { exact: true })).toBeVisible();
    await shot(page, env, frames.breakdown[device.side]);
    await page.getByRole("button", { name: "重新讀取", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Meta 分組明細", exact: true })).toBeVisible();
    await page.getByRole("button", { name: "返回廣告歸因", exact: true }).click();
    await expect(page.getByRole("heading", { name: "廣告歸因", exact: true })).toBeVisible();
  });
  test("C3 attribution main screen in all three locales", async ({ page }) => {
    await env.login(page);
    for (const [locale, heading, action] of [["zh-TW", "歸因報表", "成效總覽"], ["zh-CN", "归因报表", "成效总览"], ["en", "Attribution report", "Results overview"]]) {
      await page.goto(`${env.origin}/${locale}/ads/attribution?store=${env.store}&from=${env.from}&to=${env.to}`);
      await expect(page.getByRole("heading", { level: 1, name: heading, exact: true })).toBeVisible();
      await expect(page.getByRole("button", { name: action, exact: true })).toBeVisible();
    }
  });
}
