// Purpose: C6 customer-import signed states on /customers/import, excluding the prototype-only CSV chooser.
// Depends on: signed #472 FRAME-COPY, migration-import's signed HTTPS fixture and frozen privacy-safe import DTOs.
// Used by: TestBrowserMigrationImport through its explicit Playwright file registration.
// Invariants: synthetic CSV only, no cross-store lists or automatic UNKNOWN replay; mock faults never count as LIVE evidence.
import { expect, test, type Page } from "@playwright/test";
import { action, capture, click, heading, login, origin, recordEvidence } from "./parity-data-ops-support";
// F03-BEGIN imports: existing copy is a locator/text source, not new signed translation authority.
import { dataOpsCopy, dataOpsText } from "../../packages/i18n/src/data-ops";
import { importCopy } from "../../apps/admin/lib/import-copy";
// F03-END imports

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
      if (locale === "zh-TW") {
        await heading(page, "匯入顧客");
        await expect(page.getByRole("button", { name: "預覽可套用資料", exact: true })).toBeVisible();
        await capture(page, frame("6:18298", "6:18486"));
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

    // PR475 P1: a rejected replacement must not strand the errors screen after clearing its old preview.
    for (const entry of ["preview", "result"] as const) for (const invalid of ["oversized", "utf8", "header"] as const)
      test(`customer import ${viewport.width}: ${entry} errors recover after ${invalid} corrected file`, async ({ page }) => {
        await page.setViewportSize(viewport); await owner(); await login(page); await page.goto(href("zh-TW"));
        const prefix = `REPAIR-${test.info().testId.replace(/[^a-z0-9]/gi, "").slice(-16)}`;
        const privateName = `Synthetic ${prefix}`;
        const requests: string[] = [];
        page.on("request", request => {
          if (request.method() === "POST" && new URL(request.url()).pathname === `/api/stores/${store}/imports/customers/commit`) requests.push(request.postData() ?? "");
        });
        const choose = async (button: string, name: string, buffer: Buffer) => action(page, button, "click + native filechooser", "file selection completes without reloading", async () => {
          const [chooser] = await Promise.all([
            page.waitForEvent("filechooser"),
            page.getByRole("button", { name: button, exact: true }).click(),
          ]);
          await chooser.setFiles({ name, mimeType: "text/csv", buffer });
        });
        await choose("更換檔案", "repair-source.csv", Buffer.from(`source_id,name\n${prefix},${privateName}\n,Synthetic excluded\n`));
        await page.getByTestId("import-map-external_id").selectOption("source_id");
        await page.getByTestId("import-map-name").selectOption("name");
        await click(page, "預覽可套用資料", "real preview contains one applicable and one excluded row", async () => {
          await heading(page, "確認前，再核對一次");
          await expect(page.getByText("新增 1 · 更新 0 · 不匯入 1", { exact: true })).toBeVisible();
        });
        if (entry === "result") await click(page, "確認匯入 1 筆", "real import receipt precedes repair", () => heading(page, "顧客匯入結果"));
        await click(page, "查看不匯入原因", "safe error details retain a corrected-file entry", () => heading(page, "回原檔修正這一筆"));
        const baselineCommits = entry === "result" ? 1 : 0;
        expect(requests).toHaveLength(baselineCommits);
        const buffer = invalid === "oversized" ? Buffer.alloc(2 * 1024 * 1024 + 1, 97)
          : invalid === "utf8" ? Buffer.from([0xff, 0xfe, 0x0a]) : Buffer.from('"source_id,name\n');
        const error = invalid === "oversized" ? "請選擇不超過 2 MiB 的檔案。"
          : invalid === "utf8" ? "請另存為 CSV UTF-8，無法匯入 Big5 檔案。" : "請檢查 CSV 格式、每列欄位數與欄位對應。";
        await choose("選擇修正後檔案", `rejected-${invalid}.csv`, buffer);
        await expect(page.getByTestId("import-wizard").getByRole("alert")).toHaveText(error);
        expect(requests, "a rejected corrected file never commits").toHaveLength(baselineCommits);
        for (const value of [prefix, privateName, "Synthetic excluded"]) await expect(page.locator("main")).not.toContainText(value);
        await action(page, "更換檔案", "recoverable picker", "invalid replacement leaves a reachable, enabled picker", async () => {
          // Causal RED at 5ad20902: errors + null preview removes this picker and every recovery action.
          await expect(page.getByRole("button", { name: "更換檔案", exact: true })).toBeVisible();
          await expect(page.getByRole("button", { name: "更換檔案", exact: true })).toBeEnabled();
        });
        await choose("更換檔案", "corrected.csv", Buffer.from(`source_id,name\n${prefix}-corrected,${privateName} corrected\n`));
        await page.getByTestId("import-map-external_id").selectOption("source_id");
        await page.getByTestId("import-map-name").selectOption("name");
        await click(page, "預覽可套用資料", "same-page replacement reaches a fresh real preview", async () => {
          await heading(page, "確認前，再核對一次");
          await expect(page.getByText("新增 1 · 更新 0 · 不匯入 0", { exact: true })).toBeVisible();
          await expect(page.getByRole("heading", { name: "corrected.csv", exact: true })).toBeVisible();
          await expect(page.locator("main")).not.toContainText(privateName);
          expect(requests).toHaveLength(baselineCommits);
        });
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
        // F03-BEGIN preview-check: after the original capture; keep its viewport and all old actions intact.
        await f03SignedRows(page, "zh-TW", [{ row: 3, outcome: "failed", code: "required" }, { row: 1, outcome: "created" }, { row: 2, outcome: "updated" }]);
        // F03-END preview-check
        // #543 H, READ/MEASURE only: a hovered enabled primary keeps a readable label (WCAG AA 4.5:1), not white on the pale hover fill.
        const confirm = page.getByRole("button", { name: "確認匯入 2 筆", exact: true });
        await confirm.hover();
        const contrast = await confirm.evaluate(button => {
          const style = getComputedStyle(button);
          const luminance = (css: string) => {
            const [r, g, b] = (css.match(/[\d.]+/g) ?? []).slice(0, 3).map(n => { const v = Number(n) / 255; return v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4; });
            return 0.2126 * r + 0.7152 * g + 0.0722 * b;
          };
          const [light, dark] = [luminance(style.color), luminance(style.backgroundColor)].sort((a, b) => b - a);
          return { ratio: (light + 0.05) / (dark + 0.05), color: style.color, background: style.backgroundColor };
        });
        expect(contrast.ratio, `hovered 確認匯入 2 筆 label ${JSON.stringify(contrast)}`).toBeGreaterThanOrEqual(4.5);
        await action(page, "返回調整欄位", "click", "column choices are retained", async () => {
          await page.locator(".ops-card").filter({ has: page.getByTestId("import-back") }).getByRole("button", { name: "返回調整欄位", exact: true }).click();
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
          // #475 visual P1 (1028:7221 / 1028:7299), READ/MEASURE only: the 選擇的檔案 card must not sit under the sticky footer,
          // and the shell app bar must be on screen on this route like on every other frame of the flow.
          await expect(page.locator("header[data-shell-topbar]")).toBeInViewport();
          // #543 I(iii): after the scroll to top, screen-reader focus lands on the new route heading, not on body.
          await expect(page.getByRole("heading", { level: 1, name: "回原檔修正這一筆", exact: true })).toBeFocused();
          // #543 I(i): measure the whole card that owns 選擇的檔案, scrolled into view, so 390x844 cannot pass with it below the fold.
          const owner = page.locator("section.ops-card").filter({ has: page.getByRole("heading", { name: "選擇的檔案", exact: true }) });
          await owner.scrollIntoViewIfNeeded();
          const card = await owner.boundingBox();
          const footer = await page.locator(".ops-footer").filter({ has: page.getByRole("button", { name: "返回預覽", exact: true }) }).boundingBox();
          expect(card, "選擇的檔案 card is laid out").toBeTruthy(); expect(footer, "errors footer is laid out").toBeTruthy();
          expect(card!.y + card!.height, `選擇的檔案 card ${JSON.stringify(card)} must end above the footer ${JSON.stringify(footer)}`).toBeLessThanOrEqual(footer!.y);
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
        // F03-BEGIN stale-check: the existing single MOCK409 attempt and screenshot remain unchanged.
        await f03SignedRows(page, "zh-TW", [{ row: 2, outcome: "failed", code: "erased" }, { row: 3, outcome: "failed", code: "required" }, { row: 1, outcome: "created" }]);
        // F03-END stale-check
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
      await action(page, "回顧客", "click", "imported customer remains after refresh", async () => {
        await page.getByRole("link", { name: "回顧客", exact: true }).click();
        await page.reload(); await expect(page.getByText(nameA, { exact: true })).toBeVisible();
      });
    });
  }
});

// F03-BEGIN tests
// Purpose: independent presentation counterexamples using existing native import controls and readonly measurements.
// Depends on: local copy, existing MOCK OIDC/real preview fixture; called by the registered C6 browser suite.
// Invariants: no seed/commit, no uploaded values in evidence, failures-first/source row IDs; geometry cannot swallow pager RED.
type F03Locale = "en" | "zh-TW" | "zh-CN";
type F03Row = { row: number; outcome: "created" | "updated" | "failed"; code?: string };
type F03State = { counts: Record<string, number>; owners: Record<string, string>; batches: unknown[] };

async function f03State(): Promise<F03State> {
  // Readonly fixture snapshot, matching the existing import-wizard rollback oracle; not an import acceptance action.
  const response = await fetch(`${control}/state`, { method: "POST", headers: { "X-MIUI-Gate-Key": controlKey, "Content-Type": "application/json" }, body: JSON.stringify({ mode: "" }) });
  expect(response.status).toBe(200);
  return response.json() as Promise<F03State>;
}

async function f03SignedRows(page: Page, locale: F03Locale, expected: readonly F03Row[]) {
  const table = page.getByTestId("import-rows"), rows = table.locator("tbody tr");
  await expect(table).toBeVisible();
  await expect(rows).toHaveCount(3);
  // Native scrolling only, after old captures when called by old cases; evaluate below only reads DOM geometry.
  const scroll = await table.evaluate(element => element.getBoundingClientRect().top - Math.max(24, (innerHeight - element.getBoundingClientRect().height) / 2));
  await page.mouse.wheel(0, scroll);
  const d = dataOpsCopy(locale), c = importCopy[locale];
  expect.soft(await rows.evaluateAll(elements => elements.map(row => row.getAttribute("data-testid"))), "F03 source row IDs and failures-first order").toEqual(expected.map(row => `import-row-${row.row}`));
  for (const row of expected) {
    const summary = page.getByTestId(`import-row-${row.row}`).locator("td").first();
    const text = dataOpsText(row.outcome === "created" ? d.willCreate : row.outcome === "updated" ? d.willUpdate : d.willSkip, { n: row.row, reason: c.errors[row.code ?? ""] ?? c.unavailable });
    await expect.soft(summary, `F03 nonempty safe summary for source row ${row.row}`).toHaveText(text);
    expect.soft((await summary.innerText()).trim(), `F03 source row ${row.row} summary is nonempty`).not.toBe("");
  }
  // READ/MEASURE only: compare each composite summary cell's edges with its actual row, in CSS pixels.
  const geometry = await rows.evaluateAll(elements => elements.map(row => {
    const bounds = row.getBoundingClientRect(), cell = row.querySelector("td")!.getBoundingClientRect();
    return { id: row.getAttribute("data-testid"), left: Math.abs(cell.left - bounds.left), right: Math.abs(cell.right - bounds.right) };
  }));
  await test.step("F03 full-row composite summary geometry", async () => {
    for (const row of geometry) {
      expect.soft(row.left, `F03 ${row.id} summary left edge <=2 CSSpx`).toBeLessThanOrEqual(2);
      expect.soft(row.right, `F03 ${row.id} summary right edge <=2 CSSpx`).toBeLessThanOrEqual(2);
    }
  });
  await test.step("F03 three-row preview has no visible pager", async () => {
    expect.soft(await page.getByTestId("import-row-previous").isVisible(), "F03 previous hidden for three rows").toBe(false);
    expect.soft(await page.getByTestId("import-row-next").isVisible(), "F03 next hidden for three rows").toBe(false);
  });
  await expect.soft(page.locator("html")).toHaveAttribute("lang", locale);
  expect.soft(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth), "F03 no document horizontal overflow").toBeLessThanOrEqual(1);
  await test.info().attach("F03 safe geometry and pager observations", { contentType: "application/json", body: JSON.stringify({ geometry, previousVisible: await page.getByTestId("import-row-previous").isVisible(), nextVisible: await page.getByTestId("import-row-next").isVisible() }) });
}

test.describe("PARITY-DATA-OPS C6 F03 independent preview layout", () => {
  for (const locale of ["en", "zh-TW", "zh-CN"] as const) for (const width of [1440, 390])
    test(`F03 customer preview ${locale} ${width}: full-row summaries and hidden pager`, async ({ page }) => {
      await page.setViewportSize({ width, height: width === 390 ? 844 : 900 });
      await owner(); await login(page); await page.goto(href(locale));
      const d = dataOpsCopy(locale);
      const prefix = `F03-${test.info().testId.replace(/[^a-z0-9]/gi, "").slice(-16)}`;
      const privateNameA = `Synthetic ${prefix} A`, privateNameB = `Synthetic ${prefix} B`;
      const markers = [prefix, privateNameA, privateNameB, "Synthetic F03 invalid"];
      let commits = 0;
      page.on("request", request => {
        if (request.method() === "POST" && new URL(request.url()).pathname === `/api/stores/${store}/imports/customers/commit`) commits++;
      });
      const before = await f03State();
      expect(before.owners[`${prefix}-A`]).toBeUndefined(); expect(before.owners[`${prefix}-B`]).toBeUndefined();
      await action(page, d.changeFile, "click + native filechooser", "synthetic three-row file is selected without seeding a customer", async () => {
        const chooser = page.waitForEvent("filechooser", { timeout: 15000 });
        const [selected] = await Promise.all([chooser, page.getByRole("button", { name: d.changeFile, exact: true }).click({ timeout: 15000 })]);
        await selected.setFiles({ name: "f03-customers.csv", mimeType: "text/csv", buffer: Buffer.from(`source_id,name,phone\n${prefix}-A,${privateNameA},\n${prefix}-B,${privateNameB},\n,Synthetic F03 invalid,\n`) });
        await expect(page.getByText("f03-customers.csv", { exact: true })).toBeVisible();
      });
      for (const [field, header] of [["external_id", "source_id"], ["name", "name"], ["phone", "phone"]])
        await action(page, `import-map-${field}`, "selectOption", `native mapping retains ${header}`, async () => {
          await page.getByTestId(`import-map-${field}`).selectOption(header);
          await expect(page.getByTestId(`import-map-${field}`)).toHaveValue(header);
        });
      await action(page, d.previewCustomers, "click", "actual unseeded preview reports 2 new, 0 update, 1 failed and renders three safe rows", async () => {
        const response = page.waitForResponse(response => response.request().method() === "POST" && new URL(response.url()).pathname === `/api/stores/${store}/imports/customers/preview`, { timeout: 15000 });
        const [actual] = await Promise.all([response, page.getByRole("button", { name: d.previewCustomers, exact: true }).click({ timeout: 15000 })]);
        expect(actual.status()).toBe(200);
        const preview = await actual.json();
        expect(preview).toMatchObject({ rows_total: 3, new_rows: 2, update_rows: 0, apply_rows: 2, failed_rows: 1 });
        expect(preview.rows.map((row: F03Row) => ({ row: row.row, outcome: row.outcome, ...(row.code ? { code: row.code } : {}) }))).toEqual([{ row: 1, outcome: "created" }, { row: 2, outcome: "created" }, { row: 3, outcome: "failed", code: "required" }]);
        await expect(page.getByTestId("import-rows").locator("tbody tr")).toHaveCount(3);
        await expect(page.getByText(dataOpsText(d.customerTotals, { new: 2, updated: 0, failed: 1 }), { exact: true })).toBeVisible();
      });
      await f03SignedRows(page, locale, [{ row: 3, outcome: "failed", code: "required" }, { row: 1, outcome: "created" }, { row: 2, outcome: "created" }]);
      for (const marker of markers) await expect.soft(page.locator("main")).not.toContainText(marker);
      const after = await f03State();
      for (const key of ["owners", "profiles", "history", "batches", "consents", "receipts", "audits"])
        expect.soft(after.counts[key], `F03 preview rollback ${key}`).toBe(before.counts[key]);
      expect.soft(after.owners, "F03 no owner identity changes").toEqual(before.owners);
      expect.soft(after.batches, "F03 no batch/receipt change").toEqual(before.batches);
      expect.soft(commits, "F03 preview-only flow sends zero commit requests").toBe(0);
    });
});
// F03-END tests
