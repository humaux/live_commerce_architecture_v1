// Purpose: C6 customer-import signed states on /customers/import, excluding the prototype-only CSV chooser.
// Depends on: signed #472 FRAME-COPY, migration-import's signed HTTPS fixture and frozen privacy-safe import DTOs.
// Used by: TestBrowserMigrationImport through its explicit Playwright file registration.
// Invariants: synthetic CSV only, no cross-store lists or automatic UNKNOWN replay; mock faults never count as LIVE evidence.
import { expect, test, type Page } from "@playwright/test";
import { action, capture, click, heading, login, origin, recordEvidence } from "./parity-data-ops-support";

const store = process.env.LC_MIUI_STORE!;
const control = process.env.LC_MIUI_CONTROL_URL!;
const controlKey = process.env.LC_MIUI_CONTROL_KEY!;
const href = (locale: string) => `${origin}/${locale}/customers/import?store=${store}`;

async function owner() {
  // Fixture actor setup only; login still goes through the actual sign-in button and MOCK OIDC callback.
  const result = await fetch(`${control}/actor`, { method: "POST", headers: { "X-MIUI-Gate-Key": controlKey, "Content-Type": "application/json" }, body: JSON.stringify({ mode: "owner" }) });
  expect(result.status).toBe(204);
}

recordEvidence();
test.describe("PARITY-DATA-OPS C6 customer import", () => {
  for (const mobile of [false, true]) {
    const viewport = { width: mobile ? 390 : 1440, height: mobile ? 844 : 900 };
    const frame = (desktop: string, phone: string) => mobile ? phone : desktop;
    for (const locale of ["zh-TW", "zh-CN", "en"]) test(`customer import ${locale} ${viewport.width}: signed entry replaces old step picker`, async ({ page }) => {
      await page.setViewportSize(viewport); await owner(); await login(page); await page.goto(href(locale));
      if (locale === "zh-TW") await capture(page, frame("6:18298", "6:18486"));
      if (locale === "zh-TW") {
        await heading(page, "匯入顧客");
        await expect(page.getByRole("button", { name: "預覽可套用資料", exact: true })).toBeVisible();
      } else {
        // FRAME-COPY contains no signed translation table for this module. Do not invent translations.
        // Exact zh-CN/en copy parity is explicitly DEFERRED in the handoff; main screens still run in all three locales.
        await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
        await expect(page.getByTestId("import-wizard").getByRole("combobox")).toHaveCount(4);
      }
      // The historical archive remains a separate flow; it must not be a competing type selector in customer import.
      await expect(page.getByTestId("import-type-orders")).toHaveCount(0);
      await expect(page.locator("html")).toHaveAttribute("lang", locale);
    });

    for (const outcome of ["preview", "errors", "result", "pending", "stale", "stale-unknown"] as const) test(`customer import ${viewport.width}: ${outcome}`, async ({ page }) => {
      await page.setViewportSize(viewport); await owner(); await login(page);
      await page.goto(href("zh-TW")); await heading(page, "匯入顧客");
      const prefix = `PARITY-${test.info().testId.replace(/[^a-z0-9]/gi, "").slice(-16)}`;
      const nameA = `Synthetic ${prefix} A`, nameB = `Synthetic ${prefix} B`;
      const mapping = { external_id: "source_id", name: "name", phone: "phone" };
      const cookie = (await page.context().cookies()).find(c => c.name === "__Host-commerce_csrf");
      expect(cookie).toBeTruthy();
      // FIXTURE PREPARATION ONLY: seed one existing customer in this isolated store, not a product acceptance action.
      const seed = await page.request.post(`${origin}/api/stores/${store}/imports/customers/commit?${new URLSearchParams({ mapping: JSON.stringify(mapping), expected_apply_rows: "1" })}`, {
        headers: { "Content-Type": "text/csv", "X-CSRF-Token": cookie!.value, Origin: origin }, data: `source_id,name,phone\n${prefix}-B,${nameB},\n`,
      });
      expect(seed.status()).toBe(200);
      await page.goto(href("zh-TW")); await heading(page, "匯入顧客");
      // Genuine file-input interaction; Figma's demo-file buttons are deliberately not part of product acceptance.
      await action(page, "更換檔案", "click + setInputFiles", "selected CSV filename is visible", async () => {
        const chooser = page.waitForEvent("filechooser");
        await page.getByRole("button", { name: "更換檔案", exact: true }).click();
        await (await chooser).setFiles({ name: "customers.csv", mimeType: "text/csv", buffer: Buffer.from(`source_id,name,phone\n${prefix}-A,${nameA},\n${prefix}-B,${nameB},\n,Synthetic invalid,\n`) });
        await expect(page.getByText("customers.csv", { exact: true })).toBeVisible();
      });
      // Native column controls preserve omitted email values. No raw row values may be rendered as preview evidence.
      for (const [label, value] of [["來源編號 · 必填", "source_id"], ["姓名 · 必填", "name"], ["電話 · 選填", "phone"]])
        await action(page, label, "selectOption", `mapped to ${value}`, async () => {
          await page.getByRole("combobox", { name: label, exact: true }).selectOption(value);
          await expect(page.getByRole("combobox", { name: label, exact: true })).toHaveValue(value);
        });
      let preview: Record<string, unknown> | undefined;
      page.on("response", async response => {
        if (response.url().includes(`/imports/customers/preview?`) && response.status() === 200) preview = await response.json();
      });
      await click(page, "預覽可套用資料", "safe dry run with new/update/failure counts", async () => {
        await heading(page, "確認前，再核對一次");
        await expect(page.getByRole("heading", { name: "每筆處理方式", level: 2, exact: true })).toBeVisible();
        for (const text of ["不匯入電子郵件", "保留店內原有電子郵件", "行銷同意維持未知；不自動合併買家。"])
          await expect(page.getByText(text, { exact: true })).toBeVisible();
        await expect(page.getByText("新增 1 · 更新 1 · 不匯入 1", { exact: true })).toBeVisible();
        for (const privateValue of [prefix, nameA, nameB, "Synthetic invalid"])
          await expect(page.locator("main")).not.toContainText(privateValue);
        await capture(page, frame("1028:6685", "1028:6763"));
      });
      if (outcome === "preview") {
        await click(page, "返回調整欄位", "column choices are retained", async () => {
          await expect(page.getByRole("combobox", { name: "電話 · 選填", exact: true })).toHaveValue("phone");
        });
        return;
      }
      if (outcome === "errors") {
        await click(page, "查看不匯入原因", "privacy-safe source-row repair guidance", async () => {
          await heading(page, "回原檔修正這一筆");
          await expect(page.getByText("第 3 筆 · 缺少來源編號", { exact: true })).toBeVisible();
          await expect(page.getByText("在原檔的 source_id 欄位填入穩定且唯一的來源編號。不要填電話或電子郵件。", { exact: true })).toBeVisible();
          await capture(page, frame("1028:7221", "1028:7299"));
        });
        await click(page, "返回預覽", "original preview is retained", () => heading(page, "確認前，再核對一次"));
        return;
      }
      const commits = `**/api/stores/${store}/imports/customers/commit?*`;
      const requests: { body: string | null; query: string }[] = [];
      if (outcome !== "result") await page.route(commits, async route => {
        requests.push({ body: route.request().postData(), query: new URL(route.request().url()).search });
        if (outcome === "pending") {
          if (requests.length === 1) { const actual = await route.fetch(); expect(actual.status()).toBe(200); await route.abort("connectionreset"); }
          else await route.continue();
        } else if (outcome === "stale-unknown" && requests.length === 1) {
          // MOCK pre-dispatch loss, then a stale reply. No successful write is invented.
          await route.abort("connectionreset");
        } else {
          expect(preview, "the real preview response was observed before the fault").toBeTruthy();
          // Frozen privacy-safe BFF DTO. Reduce applicability as an erased source would; do not weaken the backend contract.
          const body = { ...preview, new_rows: 1, update_rows: 0, apply_rows: 1, failed_rows: 2, erased_rows: 1,
            rows: [{ row: 1, outcome: "created" }, { row: 2, outcome: "failed", code: "erased" }, { row: 3, outcome: "failed", code: "required" }] };
          await route.fulfill({ status: 409, contentType: "application/json", headers: { "cache-control": "private, no-store" }, body: JSON.stringify(body) });
        }
      });
      await click(page, "確認匯入 2 筆", "signed receipt, stale preview or pending outcome", async () => {
        if (outcome === "result") await heading(page, "顧客匯入結果");
        else if (outcome === "stale") await expect(page.getByText("預覽已變更，請檢查最新數量後再次確認。", { exact: true })).toBeVisible();
        else await heading(page, "尚未確認匯入結果");
      });
      if (outcome === "stale") {
        await expect(page.getByText("預覽已變更，請檢查最新數量後再次確認。", { exact: true })).toBeVisible();
        await capture(page, frame("1198:10518", "1198:10588"));
        expect(requests).toHaveLength(1);
        await expect(page.getByRole("button", { name: "確認", exact: true })).toBeVisible();
        return;
      }
      if (outcome === "pending" || outcome === "stale-unknown") {
        await heading(page, "尚未確認匯入結果");
        await expect(page.getByText("先不要換檔或重新提交", { exact: true })).toBeVisible();
        await capture(page, frame("1028:6953", "1028:7031"));
        await expect(page.getByRole("button", { name: "更換檔案", exact: true })).toBeDisabled();
        await click(page, "重試同一次匯入", "original request is reused", async () => {
          await expect.poll(() => requests.length).toBe(2); expect(requests[1]).toEqual(requests[0]);
        });
        if (outcome === "stale-unknown") {
          await heading(page, "尚未確認匯入結果");
          await expect(page.getByText("匯入結果尚未確認，可用下方按鈕重試同一份匯入；請勿更換檔案或設定。", { exact: true })).toBeVisible();
          await capture(page, frame("1198:10633", "1198:10678"));
          await expect(page.getByRole("button", { name: "重試同一份匯入", exact: true })).toBeVisible();
          return;
        }
      }
      await heading(page, "顧客匯入結果");
      await expect(page.getByText("已匯入 2 筆", { exact: true })).toBeVisible();
      await expect(page.getByText("新增 1 · 更新 1 · 未匯入 1", { exact: true })).toBeVisible();
      await capture(page, frame("1028:6819", "1028:6897"));
      await click(page, "回顧客", "imported customer remains after refresh", async () => {
        await page.reload(); await expect(page.getByText(nameA, { exact: true })).toBeVisible();
      });
    });
  }
});
