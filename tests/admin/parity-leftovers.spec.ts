// Purpose: PARITY-LEFTOVERS signed-frame acceptance on real routes for older frames with no v2 equivalent.
// Depends on: Playwright; existing order/product/message routes and their MOCK fixtures.
// Used by: TestBrowserParityLeftovers's direct spec registration and generated browser ownership.
// Invariants: fixtures prepare GETs only; never modify product code; assert signed visible copy and controls.
// Frames: 訂單缺少收件資料 (750:3454/3596), 訂單匯出說明 (808:5618/5619), 媒体上传 (6:22038/22147),
//         批量填充 (48:1277), 消息模板 (5:15002/15135), 新增自訂訊息範本 (673:2847/2916).
import { expect, test, type Page } from "@playwright/test";
import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";

const origin = process.env.LC_BROWSER_PUBLIC_ORIGIN!;
const store = process.env.LC_BROWSER_STORE!;
const evidence = process.env.LC_BROWSER_EVIDENCE!;
const id = (suffix: string) => `90000000-0000-4000-8000-${suffix.padStart(12, "0")}`;
const orderMissing = id("500");
const productId = id("600");

type Click = { route: string; control: string; operation: string; expected: string; actual: string };
const ledger = new WeakMap<Page, Click[]>();
async function action(page: Page, control: string, operation: string, expected: string, run: () => Promise<void>) {
  const row = { route: new URL(page.url()).pathname, control, operation, expected, actual: "FAILED" };
  const rows = ledger.get(page) ?? [];
  rows.push(row); ledger.set(page, rows);
  await test.step(`${operation}: ${control} → ${expected}`, async () => { await run(); row.actual = "observed"; });
}
async function screenshot(page: Page, frame: string) {
  const file = path.join(evidence, "parity", `${frame.replace(":", "-")}.png`);
  await mkdir(path.dirname(file), { recursive: true });
  await page.screenshot({ path: file });
  await test.info().attach(`Figma ${frame}`, { path: file, contentType: "image/png" });
  expect.soft(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1);
}
async function login(page: Page) {
  await page.goto(`${origin}/zh-TW`);
  await page.getByRole("button", { name: "透過身分服務登入", exact: true }).click();
  await expect(page.getByTestId("dashboard-page")).toBeVisible();
  if ((page.viewportSize()?.width ?? 1440) <= 900) {
    await page.locator('button[aria-controls="workspace-navigation"]').click();
    await expect(page.getByTestId("nav-orders")).toBeVisible();
    await page.keyboard.press("Escape");
  } else await expect(page.getByTestId("nav-orders")).toBeVisible();
}

if (process.env.LC_BROWSER_PARITY_DENIED !== "1") test.describe("PARITY-LEFTOVERS signed frames (BROWSER MOCK)", () => {
  test.afterEach(async ({ page }, info) => {
    await mkdir(path.join(evidence, "parity"), { recursive: true });
    const file = path.join(evidence, "parity", `clicks-${info.testId.replace(/[^a-z0-9-]/gi, "-")}.json`);
    await writeFile(file, JSON.stringify({ test: info.title, status: info.status, clicks: ledger.get(page) ?? [] }, null, 2));
    await info.attach("click ledger", { path: file, contentType: "application/json" });
  });

  for (const mobile of [false, true]) {
    const size = mobile ? "mobile" : "desktop";

    test(`訂單缺少收件資料 zh-TW ${size}: signed visible copy`, async ({ page }) => {
      await page.setViewportSize({ width: mobile ? 390 : 1440, height: mobile ? 844 : 900 });
      await login(page);
      await page.goto(`${origin}/zh-TW/orders/${orderMissing}?store=${store}`);

      await expect(page.getByRole("heading", { name: "請補齊收件資料", exact: true })).toBeVisible();
      await expect(page.getByText("訂單尚未建立，也尚未收款。原商品與數量仍在草稿中。", { exact: true })).toBeVisible();
      await expect(page.getByRole("heading", { name: "待補資料", exact: true })).toBeVisible();
      await expect(page.getByText("聯絡電話", { exact: true })).toBeVisible();
      await expect(page.getByText("請填寫可聯絡買家的電話。", { exact: true })).toBeVisible();
      await expect(page.getByText("超商品牌與門市號碼", { exact: true })).toBeVisible();
      await expect(page.getByText("請確認超商品牌與門市號碼；店名、地址可不填。", { exact: true })).toBeVisible();
      await expect(page.getByText("資料完整後才能取得正式報價；付款狀態會依實際收款結果更新。", { exact: true })).toBeVisible();
      await expect(page.getByRole("button", { name: "返回草稿補填", exact: true })).toBeVisible();

      await screenshot(page, mobile ? "750:3596" : "750:3454");
    });

    test(`訂單匯出說明 zh-TW ${size}: signed visible copy`, async ({ page }) => {
      await page.setViewportSize({ width: mobile ? 390 : 1440, height: mobile ? 844 : 900 });
      await login(page);
      const sideEffects: string[] = [];
      page.on("request", (request) => {
        if (!["GET", "HEAD"].includes(request.method()) || /\.csv(?:\?|$)/.test(request.url())) sideEffects.push(`${request.method()} ${new URL(request.url()).pathname}`);
      });
      page.on("download", () => sideEffects.push("download"));
      await page.goto(`${origin}/zh-TW/orders/export?store=${store}`);

      await expect(page.getByRole("heading", { name: "訂單匯出說明", exact: true })).toBeVisible();
      await expect(page.getByText("匯出包含收件資料，請妥善保存。", { exact: true })).toBeVisible();
      await expect(page.getByText("只有具備訂單匯出權限的成員可下載未出貨訂單。每次最多 1,000 筆；電話、郵遞區號及門市編號應以文字欄位開啟，保留前面的 0。", { exact: true })).toBeVisible();
      await expect(page.getByRole("button", { name: "返回訂單", exact: true })).toBeVisible();

      await screenshot(page, mobile ? "808:5619" : "808:5618");
      await action(page, "返回訂單", "click", "orders list keeps authenticated store and performs no export", async () => {
        await page.getByRole("button", { name: "返回訂單", exact: true }).click();
        await expect(page).toHaveURL((url) => url.pathname === "/zh-TW/orders" && url.searchParams.get("store") === store);
      });
      expect(sideEffects).toEqual([]);
    });

    test(`媒体上传 zh-TW ${size}: signed visible copy in product editor`, async ({ page }) => {
      await page.setViewportSize({ width: mobile ? 390 : 1440, height: mobile ? 844 : 900 });
      await login(page);
      await page.goto(`${origin}/zh-TW/products/${productId}?store=${store}`);

      const addImagesButton = page.getByRole("button", { name: "加入圖片", exact: true });
      await expect(addImagesButton).toBeVisible();
      await action(page, "加入圖片", "click", "media picker opens", async () => {
        await addImagesButton.click();
        await expect(page.getByRole("heading", { name: "選擇要加入的圖片", exact: true })).toBeVisible();
      });
      await expect(page.getByText("JPG、PNG 或 WebP；每張不超過 5 MB。加入後仍需儲存商品。", { exact: true })).toBeVisible();
      await expect(page.getByText("商品正面 · 示意", { exact: true })).toBeVisible();
      await expect(page.getByRole("button", { name: "選擇此圖片" }).first()).toBeVisible();
      await expect(page.getByText("商品背面 · 示意", { exact: true })).toBeVisible();
      await expect(page.getByText("材質細節 · 示意", { exact: true })).toBeVisible();
      await expect(page.getByRole("button", { name: "取消", exact: true })).toBeVisible();
      await expect(page.getByRole("button", { name: "加入所選圖片", exact: true })).toBeVisible();

      await screenshot(page, mobile ? "6:22147" : "6:22038");
    });

    test(`批量填充 zh-TW ${size}: signed visible copy in product editor`, async ({ page }) => {
      await page.setViewportSize({ width: mobile ? 390 : 1440, height: mobile ? 844 : 900 });
      await login(page);
      await page.goto(`${origin}/zh-TW/products/${productId}?store=${store}`);

      // Existing GET projection has one enabled SKU, no option axes, TWD 1200 minor.
      // Keep that real single-SKU entry RED instead of adding fake variants.
      const writes: string[] = [];
      page.on("request", (request) => {
        if (!["GET", "HEAD"].includes(request.method())) writes.push(`${request.method()} ${new URL(request.url()).pathname}`);
      });
      const draftPrice = page.getByTestId("product-price");
      await expect(draftPrice).toHaveValue("12");
      const bulkFillButton = page.getByRole("button", { name: "批量填充", exact: true });
      const panel = page.getByRole("region", { name: /批量填充/ });
      const priceInput = panel.getByRole("spinbutton", { name: "填入售價（元）", exact: true });
      await expect(bulkFillButton).toBeVisible();
      await action(page, "批量填充", "click", "single-SKU bulk panel opens and focuses amount", async () => {
        await bulkFillButton.click();
        await expect(panel).toBeVisible();
        await expect(priceInput).toBeFocused();
      });
      await expect(panel.getByText("欄位", { exact: true })).toBeVisible();
      // c5.1 / 48:1277 replaces the old field select with three independent actions.
      await expect(panel.getByRole("button", { name: "售價", exact: true })).toBeVisible();
      await expect(panel.getByRole("button", { name: "SKU 貨號", exact: true })).toBeVisible();
      await expect(panel.getByRole("button", { name: "SKU 貨號", exact: true })).toBeDisabled();
      await action(page, "直播關鍵字", "click", "keyword field replaces the price input", async () => {
        await panel.getByRole("button", { name: "直播關鍵字", exact: true }).click();
        await expect(priceInput).toBeHidden();
      });
      await action(page, "售價", "click", "price input returns", async () => {
        await panel.getByRole("button", { name: "售價", exact: true }).click();
        await expect(priceInput).toBeVisible();
      });
      for (const [control, summary] of [
        ["範圍：僅空白", "僅空白"],
        ["範圍：全部啟用的規格", "全部啟用的規格"],
      ] as const) await action(page, control, "click", `scope visibly becomes ${summary}`, async () => {
        await panel.getByRole("button", { name: control, exact: true }).click();
        await expect(panel.getByText(summary, { exact: true })).toBeVisible();
      });
      await action(page, "填入售價（元）", "fill", "amount is 10", async () => {
        await priceInput.fill("10");
        await expect(priceInput).toHaveValue("10");
      });
      for (const [control, expected] of [["+ 10", "20"], ["− 10", "10"]] as const)
        await action(page, control, "click", `amount becomes ${expected}`, async () => {
          await panel.getByRole("button", { name: control, exact: true }).click();
          await expect(priceInput).toHaveValue(expected);
        });
      for (const mode of ["設為", "增加", "減少", "增加"] as const)
        await action(page, mode, "click", `visible operation summary is ${mode}`, async () => {
          await panel.getByRole("button", { name: mode, exact: true }).click();
          // Signed frame has both the action and the independent current-mode summary.
          const visibleMode = panel.getByText(mode, { exact: true });
          await expect(visibleMode).toHaveCount(2);
          await expect(visibleMode.last()).toBeVisible();
        });
      await expect(panel.getByRole("button", { name: "取消", exact: true })).toBeVisible();
      await expect(panel.getByRole("button", { name: "套用批量填充", exact: true })).toBeVisible();
      await screenshot(page, "48:1277");
      await action(page, "取消", "click", "panel closes, price stays 12 and focus returns", async () => {
        await panel.getByRole("button", { name: "取消", exact: true }).click();
        await expect(panel).toBeHidden();
        await expect(draftPrice).toHaveValue("12");
        await expect(bulkFillButton).toBeFocused();
      });
      await action(page, "批量填充 / Escape", "click + key", "reopen and Escape discard without a write", async () => {
        await bulkFillButton.click();
        await expect(panel).toBeVisible();
        await priceInput.fill("99");
        await priceInput.press("Escape");
        await expect(panel).toBeHidden();
        await expect(draftPrice).toHaveValue("12");
        await expect(bulkFillButton).toBeFocused();
      });
      await action(page, "批量填充 / 返回", "click", "return discards panel edits and restores focus", async () => {
        await bulkFillButton.click();
        await expect(panel).toBeVisible();
        await priceInput.fill("99");
        await panel.getByRole("button", { name: "返回", exact: true }).click();
        await expect(panel).toBeHidden();
        await expect(draftPrice).toHaveValue("12");
        await expect(bulkFillButton).toBeFocused();
      });
      await action(page, "套用批量填充", "click", "add 10 to the existing 12 locally", async () => {
        await bulkFillButton.click();
        await expect(panel).toBeVisible();
        await priceInput.fill("10");
        await panel.getByRole("button", { name: "增加", exact: true }).click();
        await panel.getByRole("button", { name: "套用批量填充", exact: true }).click();
        await expect(panel).toBeHidden();
        await expect(draftPrice).toHaveValue("22");
        await expect(bulkFillButton).toBeFocused();
      });
      await action(page, "填入售價（元） / Enter", "click + fill + key", "Enter subtracts 10 locally", async () => {
        await bulkFillButton.click();
        await expect(panel).toBeVisible();
        await priceInput.fill("10");
        await panel.getByRole("button", { name: "減少", exact: true }).click();
        await priceInput.press("Enter");
        await expect(panel).toBeHidden();
        await expect(draftPrice).toHaveValue("12");
        await expect(bulkFillButton).toBeFocused();
      });
      expect(writes, "bulk apply/cancel/return/keyboard never persist the draft").toEqual([]);
    });

    test(`消息模板 zh-TW ${size}: signed visible copy`, async ({ page }) => {
      await page.setViewportSize({ width: mobile ? 390 : 1440, height: mobile ? 844 : 900 });
      await login(page);
      const writes: string[] = [];
      page.on("request", (request) => { if (!["GET", "HEAD"].includes(request.method())) writes.push(`${request.method()} ${new URL(request.url()).pathname}`); });
      await page.goto(`${origin}/zh-TW/messages/templates?store=${store}`);

      // Signed TEXT: desktop 677:2857 differs from mobile appbar 5:15140.
      await expect(page.getByRole("heading", { name: mobile ? "消息模板" : "消息範本", exact: true })).toBeVisible();
      await expect(page.getByText("已發布版本 · 示範資料 6 筆", { exact: true })).toBeVisible();
      await expect(page.getByText("發布不等於發送；實際發送前仍會檢查連接授權與消息期限。", { exact: true })).toBeVisible();
      await expect(page.getByText("既有範本目前無法讀取正文，因此暫不支援編輯。你可以新增自訂範本。", { exact: true })).toBeVisible();
      await expect(page.getByRole("button", { name: "＋ 新增自訂範本", exact: true })).toBeVisible();

      const templates = [
        { name: "關鍵字協助", version: "v2", types: "留言私密回覆", public: "否", date: "2026-10-25", edit: "暫不可用" },
        { name: "結帳說明", version: "v1", types: "私訊", public: "否", date: "2026-10-24", edit: "暫不可用" },
        { name: "物流查詢", version: "v1", types: "私訊", public: "否", date: "2026-10-23", edit: "暫不可用" },
        { name: "人工回覆說明", version: "v1", types: "私訊、留言私密回覆", public: "否", date: "2026-10-22", edit: "暫不可用" },
        { name: "退款說明", version: "v1", types: "私訊", public: "否", date: "2026-10-21", edit: "暫不可用" },
        { name: "直播預告", version: "v1", types: "公開回覆、商品推薦", public: "是", date: "2026-10-20", edit: "暫不可用" },
      ];
      await screenshot(page, mobile ? "5:15135" : "5:15002");
      for (const template of templates) {
        // Scope repeated v1/私訊/否 to its record; preserve every metadata field.
        const item = mobile
          ? page.getByRole("listitem").filter({ has: page.getByText(`${template.name} · ${template.version}`, { exact: true }) })
          : page.getByRole("row").filter({ has: page.getByText(template.name, { exact: true }) });
        await expect(item).toHaveCount(1);
        const texts = mobile
          ? [`${template.name} · ${template.version}`, template.types, `公開安全：${template.public} · 建立：${template.date}`, "編輯不可用：暫無正文讀取功能"]
          : [template.name, template.version, template.types, template.public, template.date, template.edit];
        for (const text of texts) {
          const field = item.getByText(text, { exact: true });
          await field.scrollIntoViewIfNeeded();
          await expect(field).toBeInViewport();
          await expect(field).toBeVisible();
          // Fixed add dock must not cover the final card's metadata.
          expect(await field.evaluate((el) => {
            const r = el.getBoundingClientRect();
            const hit = document.elementFromPoint(r.x + r.width / 2, r.y + r.height / 2);
            return !!hit && (el.contains(hit) || hit.contains(el));
          })).toBe(true);
        }
        await expect(item.getByRole("button", { name: "編輯", exact: true })).toHaveCount(0);
        await expect(item.getByRole("link", { name: "編輯", exact: true })).toHaveCount(0);
      }
      await expect(page.getByRole("textbox", { name: "訊息正文 *" })).toHaveCount(0);
      await action(page, "＋ 新增自訂範本", "click", "new form keeps authenticated store", async () => {
        await page.getByRole("button", { name: "＋ 新增自訂範本", exact: true }).click();
        await expect(page).toHaveURL((url) => url.pathname === "/zh-TW/messages/templates/new" && url.searchParams.get("store") === store);
        await expect(page.getByRole("heading", { name: "新增自訂範本", exact: true })).toBeVisible();
      });
      await action(page, "取消", "click", "template list keeps authenticated store without publishing", async () => {
        await page.getByRole("button", { name: "取消", exact: true }).click();
        await expect(page).toHaveURL((url) => url.pathname === "/zh-TW/messages/templates" && url.searchParams.get("store") === store);
        await expect(page.getByRole("heading", { name: mobile ? "消息模板" : "消息範本", exact: true })).toBeVisible();
      });
      expect(writes).toEqual([]);
    });

    test(`新增自訂訊息範本 zh-TW ${size}: signed visible copy`, async ({ page }) => {
      await page.setViewportSize({ width: mobile ? 390 : 1440, height: mobile ? 844 : 900 });
      await login(page);
      const writes: string[] = [];
      page.on("request", (request) => { if (!["GET", "HEAD"].includes(request.method())) writes.push(`${request.method()} ${new URL(request.url()).pathname}`); });
      await page.goto(`${origin}/zh-TW/messages/templates/new?store=${store}`);

      await expect(page.getByRole("heading", { name: "新增自訂範本", exact: true })).toBeVisible();
      if (mobile) await expect(page.getByRole("heading", { name: "新增範本", exact: true })).toBeVisible();
      await expect(page.getByText("先確認內容與適用類型，再發布版本；發布不會傳送訊息。", { exact: true })).toBeVisible();
      await expect(page.getByText("範本名稱 *", { exact: true })).toBeVisible();
      await expect(page.getByRole("textbox", { name: "範本名稱 *" })).toBeVisible();
      await expect(page.getByText("方便團隊辨認，1–120 個字元。", { exact: true })).toBeVisible();
      await expect(page.getByText("範本識別碼 *", { exact: true })).toBeVisible();
      await expect(page.getByRole("textbox", { name: "範本識別碼 *" })).toBeVisible();
      await expect(page.getByText("1–64 個字元；首字限小寫英文或數字，其餘可含 /、_、-。同一識別碼發布時追加版本。", { exact: true })).toBeVisible();
      await expect(page.getByText("訊息正文 *", { exact: true })).toBeVisible();
      await expect(page.getByRole("textbox", { name: "訊息正文 *" })).toBeVisible();
      await expect(page.getByText("正文最多 2,000 字元；啟用公開安全檢查時，所有類型均限 300 字元。", { exact: true })).toBeVisible();
      await screenshot(page, mobile ? "673:2916" : "673:2847");
      await expect(page.getByText("適用消息類型 *", { exact: true })).toBeVisible();
      await expect(page.getByText("可複選；選擇公開回覆或推薦時，須啟用公開安全檢查。", { exact: true })).toBeVisible();
      await expect(page.getByRole("checkbox", { name: "私訊" })).toBeVisible();
      await expect(page.getByRole("checkbox", { name: "留言的私密回覆" })).toBeVisible();
      await expect(page.getByRole("checkbox", { name: "公開回覆" })).toBeVisible();
      await expect(page.getByRole("checkbox", { name: "商品推薦" })).toBeVisible();
      await expect(page.getByText("啟用公開安全檢查", { exact: true })).toBeVisible();
      await expect(page.getByText("公開內容不可放入網址、聯絡方式、買家資料、訂單資料或付款連結。", { exact: true })).toBeVisible();
      await expect(page.getByText("內容預覽", { exact: true })).toBeVisible();
      await expect(page.getByText("發布前確認", { exact: true })).toBeVisible();
      // One signed TEXT node contains all three lines (673:2847 / 673:2916).
      await expect(page.getByText("1. 選擇至少一種適用消息。 2. 公開回覆與推薦需通過公開安全檢查。 3. 系統固定範本不能覆寫。", { exact: true })).toBeVisible();
      await expect(page.getByText("發送時仍會檢查權限、對話資格與消息期限；發布範本不代表可以立即發送。", { exact: true })).toBeVisible();
      await expect(page.getByRole("button", { name: "取消", exact: true })).toBeVisible();
      await expect(page.getByRole("button", { name: "檢查發布內容", exact: true })).toBeVisible();

      // Scroll every lower control and review item clear of the fixed action dock.
      for (const control of [
        page.getByRole("checkbox", { name: "私訊", exact: true }),
        page.getByRole("checkbox", { name: "留言的私密回覆", exact: true }),
        page.getByRole("checkbox", { name: "公開回覆", exact: true }),
        page.getByRole("checkbox", { name: "商品推薦", exact: true }),
        page.getByRole("checkbox", { name: "啟用公開安全檢查", exact: true }),
        page.getByText("發送時仍會檢查權限、對話資格與消息期限；發布範本不代表可以立即發送。", { exact: true }),
        page.getByRole("button", { name: "取消", exact: true }),
        page.getByRole("button", { name: "檢查發布內容", exact: true }),
      ]) {
        await control.scrollIntoViewIfNeeded();
        await expect(control).toBeInViewport();
        expect(await control.evaluate((el) => {
          const r = el.getBoundingClientRect();
          const hit = document.elementFromPoint(r.x + r.width / 2, r.y + r.height / 2);
          return !!hit && (el.contains(hit) || hit.contains(el));
        })).toBe(true);
      }
      await action(page, "範本名稱 / 範本識別碼 / 訊息正文", "fill", "synthetic content previews locally", async () => {
        await page.getByRole("textbox", { name: "範本名稱 *", exact: true }).fill("合成驗收範本");
        await page.getByRole("textbox", { name: "範本識別碼 *", exact: true }).fill("merchant/parity-check");
        await page.getByRole("textbox", { name: "訊息正文 *", exact: true }).fill("此為合成驗收內容。");
        await page.getByRole("checkbox", { name: "留言的私密回覆", exact: true }).check();
        await page.getByText("此為合成驗收內容。", { exact: true }).scrollIntoViewIfNeeded();
        await expect(page.getByText("此為合成驗收內容。", { exact: true })).toBeVisible();
      });
      await action(page, "檢查發布內容", "click", "structural check neither publishes nor sends", async () => {
        await page.getByRole("button", { name: "檢查發布內容", exact: true }).click();
        await expect(page).toHaveURL((url) => url.pathname === "/zh-TW/messages/templates/new" && url.searchParams.get("store") === store);
        await expect(page.getByRole("button", { name: /^(發布|發送|傳送)$/ })).toHaveCount(0);
      });
      expect(writes).toEqual([]);
    });
  }
});

if (process.env.LC_BROWSER_PARITY_DENIED === "1") test.describe("PARITY-LEFTOVERS denied live management (real signed identity, MOCK read)", () => {
  for (const mobile of [false, true]) {
    test(`消息模板 zh-TW ${mobile ? "mobile" : "desktop"}: read allowed, management denied`, async ({ page }) => {
      await page.setViewportSize({ width: mobile ? 390 : 1440, height: mobile ? 844 : 900 });
      await login(page);
      const writes: string[] = [];
      page.on("request", (request) => { if (!["GET", "HEAD"].includes(request.method())) writes.push(`${request.method()} ${new URL(request.url()).pathname}`); });
      await page.goto(`${origin}/zh-TW/messages/templates?store=${store}`);

      await expect(page.getByRole("heading", { name: mobile ? "消息模板" : "消息範本", exact: true })).toBeVisible();
      const add = page.getByRole("button", { name: "＋ 新增自訂範本", exact: true });
      await expect(add).toBeVisible();
      await expect(add).toBeDisabled();
      await expect(add).toHaveAttribute("aria-describedby", "template-manage-required");
      await expect(page.getByText("需要管理直播的權限才能新增範本。", { exact: true })).toBeVisible();

      await page.goto(`${origin}/zh-TW/messages/templates/new?store=${store}`);
      await expect(page.getByRole("heading", { name: "目前無法開啟此頁", exact: true })).toBeVisible();
      await expect(page.getByText("此連結無法開啟，或目前帳號沒有此操作權限。請返回可用的功能。", { exact: true })).toBeVisible();
      await expect(page.getByRole("textbox", { name: "訊息正文 *" })).toHaveCount(0);
      expect(writes).toEqual([]);
    });
  }
});
