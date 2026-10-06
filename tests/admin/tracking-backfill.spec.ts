// Purpose: real-click tracking import, three locales, desktop/390px, failure download and recoverable confirmation.
// Depends on: synthetic brf/rfx Next+Go+PG fixture, signed MOCK IdP and Playwright.
// Used by: TestBrowserTrackingBackfill; fixtures/fault injection never replace merchant clicks.
import { test, expect, type Page } from "@playwright/test";
import { writeFile } from "node:fs/promises";
import path from "node:path";

const origin = process.env.LC_BROWSER_PUBLIC_ORIGIN!;
const evidence = process.env.LC_BROWSER_EVIDENCE!;
const store = process.env.LC_TRACKING_STORE!;
const ids = JSON.parse(process.env.LC_TRACKING_IDS!) as Record<string, string>;
const ledger: { page: string; control: string; action: string; expected: string; actual: string; pass: boolean }[] = [];
const text = {
  "zh-TW": { action: "批量回填運單", paste: "貼上多行", input: "運單資料", preview: "檢查資料", confirm: /確認回填並出貨/, result: "回填結果", failed: "下載失敗明細", close: "關閉", upload: "上傳 CSV", template: "下載範本", retry: "確認上次回填結果", change: "重新選擇資料" },
  "zh-CN": { action: "批量回填运单", paste: "粘贴多行", input: "运单资料", preview: "检查资料", confirm: /确认回填并发货/, result: "回填结果", failed: "下载失败明细", close: "关闭", upload: "上传 CSV", template: "下载模板", retry: "确认上次回填结果", change: "重新选择资料" },
  en: { action: "Bulk tracking update", paste: "Paste rows", input: "Tracking details", preview: "Check details", confirm: /Confirm and mark .* shipped/, result: "Tracking update results", failed: "Download failed rows", close: "Close", upload: "Upload CSV", template: "Download template", retry: "Check the previous update", change: "Choose different details" },
};
type Locale = keyof typeof text;
test.use({ actionTimeout: 15000, navigationTimeout: 30000 });
test.beforeEach(async ({}, info) => {
  if (process.env.LC_TRACKING_PROBE_LOST === "1" && !info.title.startsWith("lost reply")) test.skip(true, "Targeted RED probe: other journeys NOT_RUN");
});
async function click(page: Page, control: string, operation: () => Promise<unknown>, verify: () => Promise<unknown>, expected: string, action = "real click/fill/file selection") {
  const row = { page: new URL(page.url()).pathname, control, action, expected, actual: "", pass: false };
  ledger.push(row);
  try { await operation(); await verify(); row.pass = true; row.actual = expected; }
  catch (error) { row.actual = String(error).split("\n")[0]; throw error; }
}
async function login(page: Page, locale: Locale) {
  await page.goto(`${origin}/en/`);
  await page.getByRole("button", { name: "Sign in with identity service" }).click();
  await expect(page.getByTestId("nav-orders")).toHaveCount(1);
  await page.goto(`${origin}/${locale}/orders?store=${store}`);
  await expect(page.getByTestId("orders-table")).toBeVisible();
}
async function open(page: Page, locale: Locale) {
  await expect(page.getByRole("button", { name: text[locale].action, exact: true })).toBeVisible();
  await click(page, "tracking-open", () => page.getByRole("button", { name: text[locale].action, exact: true }).click(), () => expect(page.getByRole("dialog")).toBeVisible(), "tracking dialog visible");
}
async function paste(page: Page, locale: Locale, value: string) {
  await click(page, "paste-tab", () => page.getByRole("tab", { name: text[locale].paste, exact: true }).click(), () => expect(page.getByRole("textbox", { name: text[locale].input, exact: true })).toBeVisible(), "paste editor visible");
  await click(page, "paste-rows", () => page.getByRole("textbox", { name: text[locale].input, exact: true }).fill(value), () => expect(page.getByRole("textbox", { name: text[locale].input, exact: true })).toHaveValue(value.replaceAll(/\r\n?/g, "\n")), "pasted cells retained with native textarea line endings");
}
async function wire() {
  // READ/MEASURE: observer sees the actual product requests; opaque hashes only, no write substitution.
  const response = await fetch(process.env.LC_TRACKING_CONTROL!, { method: "POST", headers: { "X-Tracking-Test": process.env.LC_TRACKING_CONTROL_KEY! }, body: JSON.stringify({ Action: "requests" }) });
  expect(response.status).toBe(200);
  return await response.json() as { path: string; hash: string; count: string; bytes: number }[];
}
const csv = (a: string, b: string, carrier = "黑貓", name = "") => `order_number,carrier,tracking_number,carrier_name\r\n${a},${carrier},0012345678,${name}\r\n${b},新竹,0009876543,\r\nnot-an-order,郵局,001111,\r\n`;
test.afterAll(async () => {
  await writeFile(path.join(evidence, "click-ledger.json"), JSON.stringify(ledger, null, 2));
  await writeFile(path.join(evidence, "click-ledger.md"), "| Page | Control | Action | Expected | Actual | Pass |\n|---|---|---|---|---|---|\n" + ledger.map(r => `| ${r.page} | ${r.control} | ${r.action} | ${r.expected} | ${r.actual.replaceAll("|", "/")} | ${r.pass} |`).join("\n"));
});

for (const [locale, code] of [["zh-TW", "tw"], ["zh-CN", "cn"], ["en", "en"]] as const) for (const width of [1440, 390]) {
  test(`${locale} ${width}: upload/paste -> row preview -> confirm -> persisted shipments and failure CSV`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 });
    await login(page, locale); await open(page, locale);
    await click(page, "keyboard-tabs", async () => {
      await page.getByRole("tab", { name: text[locale].upload, exact: true }).focus();
      await page.keyboard.press("ArrowRight");
      await expect(page.getByRole("tab", { name: text[locale].paste, exact: true })).toBeFocused();
      await expect(page.getByRole("tab", { name: text[locale].paste, exact: true })).toHaveAttribute("aria-selected", "true");
      await page.keyboard.press("Home");
    }, () => expect(page.getByRole("tab", { name: text[locale].upload, exact: true })).toBeFocused(), "arrow/Home tab selection and focus agree", "real keyboard");
    await click(page, "escape-close", () => page.keyboard.press("Escape"), async () => {
      await expect(page.getByRole("dialog")).toHaveCount(0);
      await expect(page.getByTestId("orders-tracking-backfill")).toBeFocused();
    }, "Escape closes the dialog and returns focus", "real keyboard");
    await open(page, locale);
    await page.getByRole("dialog").screenshot({ path: path.join(evidence, `input-${locale}-${width}.png`) });
    const template = page.waitForEvent("download");
    await click(page, "template-download", () => page.getByRole("button", { name: text[locale].template, exact: true }).click(), async () => { expect((await template).suggestedFilename()).toMatch(/\.csv$/); }, "template CSV downloaded");
    const name = `${code}-${width === 390 ? "phone" : "wide"}`;
    const input = csv(`LC-${ids[`${name}-a`].replaceAll("-", "").toUpperCase()}`, ids[`${name}-b`], locale === "en" ? "其他" : "黑貓", locale === "en" ? "Synthetic carrier" : "");
    if (width === 390) {
      await paste(page, locale, "discarded\t黑貓\t001");
      await click(page, "upload-tab", () => page.getByRole("tab", { name: text[locale].upload, exact: true }).click(), () => expect(page.locator("#tracking-file")).toBeVisible(), "upload tab selected and stale paste discarded");
      await paste(page, locale, input);
    }
    else {
      const chooser = page.waitForEvent("filechooser");
      await click(page, "upload-file", () => page.locator("#tracking-file").click(), async () => {
        await (await chooser).setFiles({ name: "synthetic-tracking.csv", mimeType: "text/csv", buffer: Buffer.from(input) });
        await expect(page.getByRole("button", { name: text[locale].preview, exact: true })).toBeEnabled();
      }, "CSV file selected");
    }
    await click(page, "preview", () => page.getByRole("button", { name: text[locale].preview, exact: true }).click(), async () => {
      await expect(page.getByTestId("tracking-preview")).toBeVisible();
      await expect(page.getByTestId("tracking-row")).toHaveCount(3);
      await expect(page.getByTestId("tracking-row").first()).toHaveAttribute("data-outcome", "failed");
      await expect(page.getByTestId("tracking-row")).toContainText(["not-an-order", "0012345678", "0009876543"]);
    }, "failure-first preview with leading-zero tracking numbers");
    if (width === 390) {
      await click(page, "choose-different-preview", () => page.getByRole("button", { name: text[locale].change, exact: true }).click(), () => expect(page.getByTestId("tracking-preview")).toHaveCount(0), "old preview invalidated before new details");
      await paste(page, locale, input);
      await page.getByRole("button", { name: text[locale].preview, exact: true }).click();
      await expect(page.getByTestId("tracking-preview")).toBeVisible();
    }
    await page.getByRole("dialog").screenshot({ path: path.join(evidence, `preview-${locale}-${width}.png`) });
    // READ/MEASURE only: no DOM mutation or product API call.
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
    await click(page, "confirm", () => page.getByRole("button", { name: text[locale].confirm }).click(), () => expect(page.getByRole("heading", { name: text[locale].result, exact: true })).toBeVisible(), "two shipments confirmed, one failure reported");
    await page.getByRole("dialog").screenshot({ path: path.join(evidence, `result-${locale}-${width}.png`) });
    const download = page.waitForEvent("download");
    await click(page, "failure-download", () => page.getByRole("link", { name: text[locale].failed, exact: true }).click(), async () => { expect((await download).suggestedFilename()).toBe("tracking-import-result.csv"); }, "failure CSV downloaded");
    const stream = await (await download).createReadStream();
    const chunks: Buffer[] = []; for await (const part of stream!) chunks.push(Buffer.from(part));
    const report = Buffer.concat(chunks).toString("utf8");
    expect(report).toContain("invalid_order_ref");
    expect(report).not.toMatch(/recipient|phone|address/);
    expect(report).not.toContain("0012345678"); expect(report).not.toContain("0009876543");
    await click(page, "choose-different-result", () => page.getByRole("button", { name: text[locale].change, exact: true }).click(), () => expect(page.getByRole("heading", { name: text[locale].result, exact: true })).toHaveCount(0), "result reset for a new file without losing order refresh");
    await click(page, "close", () => page.getByRole("button", { name: text[locale].close, exact: true }).click(), () => expect(page.getByRole("dialog")).toHaveCount(0), "result closed and orders refreshed");
    await page.reload();
    await expect(page.getByTestId("orders-table")).toBeVisible();
    for (const [suffix, number] of [["a", "0012345678"], ["b", "0009876543"]]) {
      if (await page.getByTestId("state-filter").inputValue() !== "all") {
        if (!await page.getByTestId("state-filter").isVisible()) {
          await expect(page.getByTestId("orders-v2-filters")).toHaveAttribute("data-expanded", "false");
          await click(page, "expand-filters-after-reload", () => page.getByTestId("orders-more-filters").click(),
            () => expect(page.getByTestId("state-filter")).toBeVisible(), "state filter expands before persisted-order lookup");
        }
        await expect(page.getByTestId("state-filter")).toBeVisible();
        await page.getByTestId("state-filter").selectOption("all");
      }
      await page.getByTestId("orders-search").fill(ids[`${name}-${suffix}`]);
      await page.getByTestId("orders-search").press("Enter");
      await expect(page.getByTestId(`order-expand-${ids[`${name}-${suffix}`]}`)).toBeVisible();
      await page.getByTestId(`order-expand-${ids[`${name}-${suffix}`]}`).click();
      await expect(page.getByTestId("order-detail")).toContainText(number);
    }
  });
}
test("stale preview makes no second write until fresh counts are explicitly confirmed", async ({ page }) => {
  const beforeWire = (await wire()).length;
  const attempts: { bytes: Buffer | null; count: string | null }[] = [];
  page.on("request", r => { if (r.url().includes("tracking-import/commit")) attempts.push({ bytes: r.postDataBuffer(), count: new URL(r.url()).searchParams.get("expected_apply_rows") }); });
  await login(page, "zh-TW"); await open(page, "zh-TW");
  await paste(page, "zh-TW", `${ids["stale-a"]}\t黑貓\tSTALE001\n${ids["stale-b"]}\t新竹\tSTALE002`);
  await page.getByRole("button", { name: text["zh-TW"].preview, exact: true }).click();
  await expect(page.getByTestId("tracking-preview")).toBeVisible();
  // FIXTURE/FAULT injection: another authorized synthetic merchant ships the first order.
  const response = await fetch(process.env.LC_TRACKING_CONTROL!, { method: "POST", headers: { "X-Tracking-Test": process.env.LC_TRACKING_CONTROL_KEY! }, body: JSON.stringify({ Action: "ship-stale" }) });
  expect(response.status).toBe(200);
  await page.getByRole("button", { name: text["zh-TW"].confirm }).click();
  await expect(page.getByTestId("tracking-stale")).toBeVisible();
  await expect(page.getByRole("button", { name: text["zh-TW"].confirm })).toContainText("1");
  expect(attempts).toHaveLength(1);
  // READBACK: authenticated owner fixture observes the real failed click's rollback, no write substitution.
  expect((await fetch(process.env.LC_TRACKING_CONTROL!, { method: "POST", headers: { "X-Tracking-Test": process.env.LC_TRACKING_CONTROL_KEY! }, body: JSON.stringify({ Action: "assert-stale-unshipped" }) })).status).toBe(200);
  // FAULT injection: a structurally valid receipt with another count must stay UNKNOWN, even after the server committed.
  await page.route("**/tracking-import/commit?*", async route => { const response = await route.fetch(); await route.fulfill({ response, json: { ...await response.json(), applied: 2 } }); });
  await page.getByRole("button", { name: text["zh-TW"].confirm }).click();
  await expect(page.getByRole("button", { name: text["zh-TW"].retry, exact: true })).toBeEnabled();
  await expect(page.getByRole("heading", { name: text["zh-TW"].result, exact: true })).toHaveCount(0);
  await page.unroute("**/tracking-import/commit?*");
  await page.getByRole("button", { name: text["zh-TW"].retry, exact: true }).click();
  await expect(page.getByRole("heading", { name: text["zh-TW"].result, exact: true })).toBeVisible();
  expect(attempts.map(a => a.count)).toEqual(["2", "1", "1"]);
  const actual = (await wire()).slice(beforeWire).filter(r => r.path.endsWith("/commit"));
  expect(actual).toHaveLength(3); expect(actual.map(r => r.count)).toEqual(["2", "1", "1"]);
  expect(new Set(actual.map(r => `${r.hash}:${r.bytes}`)).size).toBe(1);
});
test("lost reply recovers the same file/count; viewer gets no write action", async ({ page, browser }) => {
  const beforeWire = (await wire()).length;
  await login(page, "en"); await open(page, "en"); await paste(page, "en", `${ids.lost}\t郵局\t001234`);
  await page.getByRole("button", { name: text.en.preview, exact: true }).click();
  await expect(page.getByTestId("tracking-preview")).toBeVisible();
  const bodies: { bytes: Buffer | null; count: string | null }[] = [];
  page.on("request", r => { if (r.url().includes("tracking-import/commit")) bodies.push({ bytes: r.postDataBuffer(), count: new URL(r.url()).searchParams.get("expected_apply_rows") }); });
  // FAULT injection: commit the real click, then drop only its reply.
  await page.route("**/tracking-import/commit?*", async route => { await route.fetch(); await route.abort("failed"); });
  await page.getByRole("button", { name: text.en.confirm }).click();
  await expect(page.getByTestId("tracking-uncertain")).toBeVisible();
  await expect(page.getByRole("button", { name: text.en.retry, exact: true })).toBeEnabled();
  await page.unroute("**/tracking-import/commit?*");
  expect(bodies).toHaveLength(1); // No autonomous retry before the next real click.
  // Hard navigation drops the CSV Blob: metadata must still allow the original pasted bytes to be restored.
  await page.reload(); await open(page, "en");
  // A wrong recovery paste must not lock the editor or submit a different request.
  await paste(page, "en", `${ids.lost}\t郵局\tWRONG`);
  await page.getByRole("button", { name: text.en.retry, exact: true }).click();
  await expect(page.getByRole("dialog").getByRole("alert")).toContainText("Use the same original file");
  await expect(page.getByRole("textbox", { name: text.en.input, exact: true })).toBeEditable();
  expect(bodies).toHaveLength(1);
  await paste(page, "en", `${ids.lost}\t郵局\t001234`);
  await expect(page.getByRole("button", { name: text.en.retry, exact: true })).toBeEnabled();
  await page.getByRole("button", { name: text.en.retry, exact: true }).click();
  await expect(page.getByRole("heading", { name: text.en.result, exact: true })).toBeVisible();
  expect(bodies).toHaveLength(2); expect(bodies.map(b => b.count)).toEqual(["1", "1"]);
  const actual = (await wire()).slice(beforeWire).filter(r => r.path.endsWith("/commit"));
  expect(actual).toHaveLength(2); expect(actual[0]).toEqual(actual[1]);
  const context = await browser.newContext();
  // AUTHENTICATE the synthetic viewer through the actual signed MOCK IdP, not a guessed cookie name.
  const viewerOrigin = process.env.LC_TRACKING_VIEWER_ORIGIN!;
  const viewer = await context.newPage(); await viewer.goto(`${viewerOrigin}/en/`);
  await viewer.getByRole("button", { name: "Sign in with identity service" }).click();
  await expect(viewer.getByTestId("nav-orders")).toHaveCount(1);
  await viewer.goto(`${viewerOrigin}/en/orders?store=${store}`);
  await expect(viewer.getByTestId("orders-table")).toBeVisible();
  await expect(viewer.getByRole("button", { name: text.en.action, exact: true })).toHaveCount(0);
  await context.close();
});
test("foreign and duplicate rows are visible failures and cannot be confirmed", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 900 });
  await login(page, "zh-TW"); await open(page, "zh-TW");
  await paste(page, "zh-TW", `${ids.foreign}\t黑貓\t001\n${ids.lost}\t新竹\t002\n${ids.lost}\t新竹\t003`);
  await click(page, "invalid-preview", () => page.getByRole("button", { name: text["zh-TW"].preview, exact: true }).click(), async () => {
    await expect(page.getByTestId("tracking-row")).toHaveCount(3);
    await expect(page.getByTestId("tracking-preview")).toContainText("本店找不到");
    await expect(page.getByTestId("tracking-preview")).toContainText("重複出現");
    await expect(page.getByRole("button", { name: text["zh-TW"].confirm })).toBeDisabled();
  }, "all invalid rows are explained and no shipment action is enabled");
});
