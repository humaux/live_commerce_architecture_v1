// Purpose: C6 signed product-import, inventory and collection acceptance on production routes (MOCK identity, isolated PG).
// Depends on: signed FRAME-COPY.md, catalog-core's real BFF/Go fixture and parity-data-ops-support.
// Used by: TestBrowserCatalogCore (one-warehouse phase, then LC_BROWSER_PHASE=multi-warehouse); generated browser ownership routes this spec to catalog-core/product-editor.
// Invariants: fixture preparation is explicit; UI writes use real clicks; no gallery, client stock math or production access.
import { expect, test, type Page } from "@playwright/test";
import { action, capture, click, heading, login, origin, recordEvidence } from "./parity-data-ops-support";

const store = process.env.LC_BROWSER_STORE!;
const multiWarehouse = process.env.LC_BROWSER_PHASE === "multi-warehouse";
const root = (locale: string, route: string) => `${origin}/${locale}/${route}?store=${store}`;
const signed = {
  "zh-TW": { product: "商品匯入與匯出", preview: "預覽檔案", export: "匯出商品 CSV", inventory: "庫存", collections: "集合與分類", create: "+ 新建集合", legacyProduct: "匯入與匯出商品", legacyPreview: "檢查檔案", legacyInventory: "商品與庫存", legacyCollection: "分類" },
  "zh-CN": { product: "商品导入与导出", preview: "预览文件", export: "导出商品 CSV", inventory: "库存", collections: "集合与分类", create: "+ 新建集合", legacyProduct: "导入与导出商品", legacyPreview: "检查文件", legacyInventory: "商品与库存", legacyCollection: "分类" },
  en: { product: "Product import and export", preview: "Preview file", export: "Export product CSV", inventory: "Inventory", collections: "Collections and categories", create: "+ New collection", legacyProduct: "Import and export products", legacyPreview: "Check the file", legacyInventory: "Products & inventory", legacyCollection: "Collections" },
};

// FIXTURE PREPARATION ONLY: real authenticated BFF, synthetic stock and product facts.
// Never count these API calls as click acceptance or read/store any buyer information.
async function prepare(page: Page) {
  // FIXTURE transport: browser fetch uses the same signed session/edge as the UI.
  // APIRequestContext bypasses that browser transport and returned 401 in RED.
  const warehouses = await page.evaluate(async store => {
    const response = await fetch(`/api/stores/${store}/warehouses`);
    return { ok: response.ok, body: await response.json() };
  }, store);
  expect(warehouses.ok, "catalog fixture warehouse read").toBe(true);
  const wh = warehouses.body.items[0];
  expect(wh?.name).toBeTruthy();
  const cookie = (await page.context().cookies()).find(c => c.name === "__Host-commerce_csrf" || c.name === "commerce_csrf");
  expect(cookie, "fixture session established by real OIDC click").toBeTruthy();
  const tag = test.info().testId;
  const csv = `handle,title,sku,price,description,stock:${wh.name}\nparity-a12,焦糖托特包,A12,1280,${tag},24\nparity-b21,米白托特包,B21,980,${tag},8\nparity-c08,帆布收納袋,C08,300,${tag},0\n`;
  const response = await page.evaluate(async ({ store, csv, csrf }) => {
    const response = await fetch(`/api/stores/${store}/tools/products/import/commit`, {
      method: "POST", headers: { "Content-Type": "text/csv", "X-CSRF-Token": csrf }, body: csv,
    });
    return { status: response.status, body: await response.json() };
  }, { store, csv, csrf: cookie!.value });
  expect(response.status, "synthetic CSV fixture commit").toBe(200);
  expect(response.body.committed).toBe(true);
}

// FIXTURE PREPARATION ONLY: set A12's target on-hand per named warehouse through the real BFF CSV import (same transport as prepare).
// Seeds a deterministic active decoy containing C08 (parity-c08-decoy) as a regression witness: if the empty-result check below
// ever reverts to the broad query "q=C08", this active decoy immediately violates the retained empty assertion.
async function stock(page: Page, targets: Record<string, number>) {
  const cookie = (await page.context().cookies()).find(c => c.name === "__Host-commerce_csrf" || c.name === "commerce_csrf");
  expect(cookie, "fixture session established by real OIDC click").toBeTruthy();
  const names = Object.keys(targets);
  const csv = `handle,title,sku,price,description,${names.map(name => `stock:${name}`).join(",")}\nparity-a12,焦糖托特包,A12,1280,${test.info().testId},${names.map(name => targets[name]).join(",")}\nparity-c08-decoy,C08-ACTIVE,C08-ACTIVE,100,${test.info().testId},${names.map(() => 0).join(",")}\n`;
  const response = await page.evaluate(async ({ store, csv, csrf }) => {
    const response = await fetch(`/api/stores/${store}/tools/products/import/commit`, {
      method: "POST", headers: { "Content-Type": "text/csv", "X-CSRF-Token": csrf }, body: csv,
    });
    return { status: response.status, body: await response.json() };
  }, { store, csv, csrf: cookie!.value });
  expect(response.status, "synthetic stock fixture commit").toBe(200);
  expect(response.body.committed).toBe(true);
}

// InventoryView renders the SKU code before a middle-dot separator in one semantic cell.
// Whole-row text can join a product suffix "a1" to the next quantity cell "20".
const inventoryA12Row = (page: Page) => page.getByRole("row").filter({ has: page.getByRole("cell", { name: /^A12 · / }) });

test("MODEL_ONLY: inventory SKU-cell selector rejects cross-cell and name collisions", async ({ browser }) => {
  // Selector-engine calibration only: isolated page, no app DOM/session/fixture changes.
  const calibration = await browser.newPage();
  const rows = '<tr><td>T04-6f0d9f30dfa1-1 · <button>t04-6f0d9f30dfa1</button></td><td>20</td></tr><tr><td>A12 · <button>焦糖托特包</button></td><td>24</td></tr>';
  try {
    await calibration.setContent(`<table><tbody>${rows}</tbody></table>`);
    await expect(calibration.getByRole("row").filter({ hasText: "A12" }), "old locator cross-cell witness").toHaveCount(2);
    await expect(inventoryA12Row(calibration), "SKU-cell uniquely selects true A12").toHaveCount(1);
    await expect(inventoryA12Row(calibration)).toContainText("焦糖托特包");
    await calibration.setContent(`<table><tbody>${rows}<tr><td>A120 · <button>other</button></td><td>1</td></tr><tr><td>OTHER · <button>A12</button></td><td>1</td></tr></tbody></table>`);
    await expect(inventoryA12Row(calibration), "SKU-cell excludes A120 and product-name A12").toHaveCount(1);
    await expect(inventoryA12Row(calibration)).toContainText("焦糖托特包");
  } finally {
    await calibration.close();
  }
});

recordEvidence();
test.describe("PARITY-DATA-OPS C6 signed real routes", () => {
  test.skip(multiWarehouse, "one-warehouse C6 frames already ran in the first TestBrowserCatalogCore phase");
  for (const mobile of [false, true]) {
    const viewport = mobile ? { width: 390, height: 844 } : { width: 1440, height: 900 };
    const frame = (desktop: string, phone: string) => mobile ? phone : desktop;
    for (const locale of ["zh-TW", "zh-CN", "en"] as const) {
      const c = signed[locale];
      for (const module of ["product", "inventory", "collections"] as const) test(`${module} ${locale} ${viewport.width}: signed entry and no legacy layout`, async ({ page }) => {
        await page.setViewportSize(viewport); await login(page);
        const route = { product: "products/import", inventory: "inventory", collections: "collections" }[module];
        await page.goto(root(locale, route));
        // Capture before asserting so RED artifacts show the actual pre-Figma page, not an invented frame.
        if (locale === "zh-TW") await capture(page, { product: frame("5:15211", "5:15352"), inventory: frame("21:480", "21:899"), collections: frame("6:19278", "6:19486") }[module]);
        await heading(page, c[module]);
        await expect(page.getByRole("heading", { name: { product: c.legacyProduct, inventory: c.legacyInventory, collections: c.legacyCollection }[module], exact: true })).toHaveCount(0);
        if (module === "product") {
          await expect(page.getByRole("button", { name: c.preview, exact: true })).toBeVisible();
          await expect(page.getByRole("button", { name: c.export, exact: true })).toBeVisible();
          await expect(page.getByRole("button", { name: c.legacyPreview, exact: true })).toHaveCount(0);
        } else if (module === "collections" || locale === "zh-TW") await expect(page.getByRole("button", { name: module === "inventory" ? "搜尋貨號" : c.create, exact: true })).toBeVisible();
        await expect(page.locator("html")).toHaveAttribute("lang", locale);
      });
    }

    test(`collections ${viewport.width}: create manual collection and persist after refresh`, async ({ page }) => {
      await page.setViewportSize(viewport); await login(page); await page.goto(root("zh-TW", "collections"));
      await heading(page, "集合與分類");
      const name = `Parity collection ${test.info().testId}`;
      await action(page, "名稱", "fill", "new collection name is retained", async () => {
        await page.getByRole("textbox", { name: "名稱", exact: true }).fill(name);
        await expect(page.getByRole("textbox", { name: "名稱", exact: true })).toHaveValue(name);
      });
      await page.getByRole("radio", { name: "手動", exact: true }).check();
      await click(page, "+ 新建集合", "created collection is shown after refresh", async () => {
        await expect(page.getByText(name, { exact: true }).first()).toBeVisible();
        await page.reload(); await expect(page.getByText(name, { exact: true }).first()).toBeVisible();
      });
    });

    for (const invalid of [false, true]) test(`product import ${viewport.width}: ${invalid ? "all-or-nothing errors" : "preview and persisted result"}`, async ({ page }) => {
      await page.setViewportSize(viewport); await login(page);
      await page.goto(root("zh-TW", "products/import"));
      await heading(page, "商品匯入與匯出");
      await prepare(page);
      const data = invalid
        ? "handle,title,sku,price,collections\nparity-error-a,帆布托特包,PARITY-ERR-A,,\nparity-error-b,收納袋,PARITY-ERR-B,300,missing-parity-collection\n"
        : "handle,title,sku,price\nparity-new,日常小包,PARITY-NEW,500\nparity-a12,焦糖托特包,A12,1280\nparity-b21,米白托特包,B21,980\n";
      // Native file-input operation; no prototype file picker, DOM injection or fabricated file upload notice.
      await action(page, "商品資料檔案", "setInputFiles", "selected filename is visible", async () => {
        await page.getByLabel("商品資料檔案", { exact: true }).setInputFiles({ name: "products.csv", mimeType: "text/csv", buffer: Buffer.from(data) });
        await expect(page.getByText("products.csv", { exact: true }).first()).toBeVisible();
      });
      await click(page, "預覽檔案", invalid ? "batch rejected without partial writes" : "server preview before any write", async () => {
        if (invalid) {
          await heading(page, "修正商品匯入資料");
          await expect(page.getByText("本批次未寫入", { exact: true })).toBeVisible();
          await expect(page.getByText("新增商品規格缺少售價。", { exact: true })).toBeVisible();
          await expect(page.getByText("指定的系列不存在。", { exact: true })).toBeVisible();
          await capture(page, frame("750:3456", "750:3598"));
        } else {
          await heading(page, "預覽商品匯入");
          await expect(page.getByText("先檢查檔案，沒有錯誤後才整批匯入。錯誤未修正前不寫入。", { exact: true })).toBeVisible();
          await capture(page, frame("30:1441", "30:1528"));
        }
      });
      if (invalid) {
        await click(page, "重新上傳並預覽", "return to file selection", () => heading(page, "商品匯入與匯出"));
      } else {
        // The signed preview omits a successful-submit label: preserve the existing real submit control's stable id.
        await action(page, "import-commit", "click", "server receipt renders signed result", async () => {
          await page.getByTestId("import-commit").click(); await heading(page, "匯入結果");
        });
        await expect(page.getByText("匯入完成", { exact: true })).toBeVisible();
        await capture(page, frame("30:1574", "30:1635"));
        await page.getByRole("link", { name: "查看商品", exact: true }).click();
        await page.reload(); await expect(page.getByText("日常小包", { exact: true })).toBeVisible();
      }
    });

    for (const outcome of ["review", "result", "conflict", "unknown", "discard"] as const) test(`inventory ${viewport.width}: ${outcome}`, async ({ page }) => {
      await page.setViewportSize(viewport); await login(page);
      await page.goto(root("zh-TW", "inventory")); await heading(page, "庫存");
      await prepare(page); await page.reload();
      const row = inventoryA12Row(page);
      await expect(row, "exactly one A12 SKU before adjustment").toHaveCount(1);
      // Use the product row/card's actual control; no selecting an internal variable or URL-only state.
      await action(page, "A12 / 調整庫存", "click", "selected SKU adjustment opens", async () => {
        await row.getByRole("button", { name: "調整庫存", exact: true }).click(); await heading(page, "調整庫存");
      });
      await expect(page.getByRole("heading", { name: "調整原因（必填）", level: 2, exact: true })).toBeVisible();
      await capture(page, frame("6:19050", "6:19202"));
      await click(page, "增加 3", "preview reflects positive three", () => expect(page.getByText("27", { exact: true }).first()).toBeVisible());
      await click(page, "到貨補錄", "reason selected before review", () => expect(page.getByRole("button", { name: "核對調整", exact: true })).toBeEnabled());
      if (outcome === "discard") {
        await click(page, "取消", "discard confirmation retains unsent values", async () => {
          await heading(page, "放棄本次調整？");
          await expect(page.getByText("選擇「繼續編輯」會保留目前輸入。", { exact: true })).toBeVisible();
          await capture(page, frame("840:5256", "840:5257"));
        });
        await click(page, "繼續編輯", "quantity and reason remain", async () => {
          await expect(page.getByText("到貨補錄", { exact: true })).toBeVisible();
        });
        return;
      }
      await click(page, "核對調整", "review SKU, warehouse, delta and reason", async () => {
        await heading(page, "核對庫存調整");
        for (const text of ["變動件數", "調整原因", "到貨補錄"]) await expect(page.getByText(text, { exact: true })).toBeVisible();
        await capture(page, frame("840:5246", "840:5247"));
      });
      if (outcome === "review") {
        await click(page, "返回編輯", "draft reason survives review", () => expect(page.getByRole("button", { name: "到貨補錄", exact: true })).toBeVisible());
        return;
      }
      const requests: { key?: string; body: string | null }[] = [];
      const adjustments = `**/api/stores/${store}/inventory/adjustments`;
      if (outcome === "conflict") await page.route(adjustments, route => route.fulfill({ status: 409, contentType: "application/json", body: JSON.stringify({ code: "conflict", message: "", request_id: "parity-synthetic", retryable: false, details: {} }) }));
      if (outcome === "unknown") await page.route(adjustments, async route => {
        requests.push({ key: route.request().headers()["idempotency-key"], body: route.request().postData() });
        if (requests.length === 1) { const result = await route.fetch(); expect(result.status()).toBe(200); await route.abort("connectionreset"); }
        else await route.continue();
      }); // MOCK transport loss only; the real Go handler commits before its response is dropped.
      await click(page, "確認調整", "server outcome replaces review", () => heading(page, { conflict: "庫存已被其他操作更新", unknown: "調整結果仍待確認", result: "庫存已調整" }[outcome]));
      if (outcome === "conflict") {
        await heading(page, "庫存已被其他操作更新");
        await capture(page, frame("840:5250", "840:5251"));
        await page.unroute(adjustments);
        await click(page, "重新讀取並核對", "fresh snapshot requires another review", () => heading(page, "調整庫存"));
      } else {
        if (outcome === "unknown") {
          await heading(page, "調整結果仍待確認");
          await capture(page, frame("840:5252", "840:5253"));
          await click(page, "以同一請求重新確認", "same command recovers receipt", () => heading(page, "庫存已調整"));
          expect(requests).toHaveLength(2); expect(requests[1]).toEqual(requests[0]);
        }
        await heading(page, "庫存已調整");
        await capture(page, frame("840:5248", "840:5249"));
        await click(page, "返回庫存", "saved on-hand survives refresh", async () => {
          await page.reload(); await expect(inventoryA12Row(page), "exactly one persisted A12 SKU").toHaveCount(1); await expect(inventoryA12Row(page)).toContainText("27");
        });
      }
    });

    test(`inventory ${viewport.width}: search applies real filter and survives refresh`, async ({ page }) => {
      await page.setViewportSize(viewport); await login(page); await prepare(page);
      await page.goto(root("zh-TW", "inventory")); await heading(page, "庫存");
      await click(page, "搜尋貨號", "search form opens", () => heading(page, "搜尋庫存"));
      await action(page, "貨號或商品名稱", "fill", "search draft is C08", async () => {
        await page.getByRole("textbox", { name: "貨號或商品名稱", exact: true }).fill("C08");
        await expect(page.getByRole("textbox", { name: "貨號或商品名稱", exact: true })).toHaveValue("C08");
      });
      const search = page.getByRole("button", { name: "搜尋", exact: true });
      await expect(search).toBeEnabled();
      await page.mouse.move(0, 0);
      // READ/MEASURE only: enabled, non-hover control; never mutate DOM/style or replace the original click.
      const searchColors = await search.evaluate(button => {
        const style = getComputedStyle(button);
        return { backgroundColor: style.backgroundColor, color: style.color };
      });
      await capture(page, frame("840:5254", "840:5255"));
      // Signed enabled source is navy/white; soft assertions retain the real filter/reload path on visual RED.
      expect.soft(searchColors.backgroundColor).toBe("rgb(20, 41, 66)");
      expect.soft(searchColors.color).toBe("rgb(255, 255, 255)");
      await click(page, "搜尋", "only selected SKU remains after reload", async () => {
        await expect(page).toHaveURL(url => url.searchParams.get("q") === "C08");
        await expect(page.getByText("C08 · 帆布收納袋", { exact: true })).toBeVisible();
        await expect(page.getByText("A12 · 焦糖托特包", { exact: true })).toHaveCount(0);
        await page.reload(); await expect(page.getByText("C08 · 帆布收納袋", { exact: true })).toBeVisible();
        await expect(page.getByText("A12 · 焦糖托特包", { exact: true })).toHaveCount(0);
      });
    });
  }
});

// #475 review P1: the signed toolbar keeps the business controls of the base ledger. The Go harness runs this group alone
// after the one-warehouse phase, with a second active warehouse, the archived parity-c08 product and a real cursor.
test.describe("PARITY-DATA-OPS inventory warehouse, status, refresh and cursor controls", () => {
  test.skip(!multiWarehouse, "needs the Go-seeded second active warehouse (LC_BROWSER_PHASE=multi-warehouse)");
  const primary = process.env.LC_BROWSER_PARITY_PRIMARY!, primaryID = process.env.LC_BROWSER_PARITY_PRIMARY_ID!;
  const second = process.env.LC_BROWSER_PARITY_SECOND!, secondID = process.env.LC_BROWSER_PARITY_SECOND_ID!;
  const cursor = process.env.LC_BROWSER_PARITY_CURSOR!;
  for (const mobile of [false, true]) {
    const viewport = mobile ? { width: 390, height: 844 } : { width: 1440, height: 900 };
    test(`inventory ${viewport.width}: second warehouse, archived filter, refresh and cursor reset`, async ({ page }) => {
      await page.setViewportSize(viewport); await login(page);
      await page.goto(root("zh-TW", "inventory")); await heading(page, "庫存");
      await stock(page, { [primary]: 24, [second]: 5 });
      await page.goto(`${root("zh-TW", "inventory")}&q=A12`); await heading(page, "庫存");
      const warehouse = page.getByRole("combobox", { name: "倉庫", exact: true });
      const status = page.getByRole("combobox", { name: "狀態", exact: true });
      const onHand = () => page.getByRole("row").filter({ hasText: "A12 · 焦糖托特包" }).locator("td").nth(1);
      const empty = page.getByRole("heading", { name: "沒有符合條件的 SKU", exact: true });
      await expect(warehouse).toBeVisible(); await expect(status).toBeVisible();

      // (a) a real select switches to the second warehouse, shows its rows and the adjustment targets it.
      await action(page, "倉庫", "selectOption", "primary warehouse balance", async () => {
        await warehouse.selectOption({ label: primary }); await expect(onHand()).toHaveText("24");
      });
      await action(page, "倉庫", "selectOption", "second warehouse balance, no cursor", async () => {
        await warehouse.selectOption({ label: second });
        await expect(page).toHaveURL(url => url.searchParams.get("warehouse") === secondID && !url.searchParams.has("cursor"));
        await expect(warehouse).toHaveValue(secondID); await expect(onHand()).toHaveText("5");
      });
      // (c) FIXTURE change on the server; the page shows it only after Refresh re-reads.
      await stock(page, { [second]: 6 });
      await expect(onHand()).toHaveText("5");
      await click(page, "重新整理", "refresh re-reads the second warehouse", async () => {
        await expect(onHand()).toHaveText("6");
        await expect(page).toHaveURL(url => url.searchParams.get("warehouse") === secondID);
      });
      await action(page, "A12 / 調整庫存", "click", "second-warehouse adjustment opens", async () => {
        await page.getByRole("row").filter({ hasText: "A12 · 焦糖托特包" }).getByRole("button", { name: "調整庫存", exact: true }).click();
        await heading(page, "調整庫存"); await expect(page.getByText(second, { exact: true }).first()).toBeVisible();
      });
      await click(page, "增加 3", "forecast starts from the second warehouse", () => expect(page.getByText("9", { exact: true }).first()).toBeVisible());
      await click(page, "到貨補錄", "reason selected before review", () => expect(page.getByRole("button", { name: "核對調整", exact: true })).toBeEnabled());
      await click(page, "核對調整", "review the second-warehouse adjustment", () => heading(page, "核對庫存調整"));
      const sent = page.waitForRequest(request => request.method() === "POST" && new URL(request.url()).pathname === `/api/stores/${store}/inventory/adjustments`);
      await click(page, "確認調整", "the command carries the second warehouse", async () => {
        expect((await sent).postDataJSON().warehouse_id).toBe(secondID); await heading(page, "庫存已調整");
      });
      await click(page, "返回庫存", "second warehouse keeps the adjusted balance after reload", async () => {
        await page.reload(); await expect(warehouse).toHaveValue(secondID); await expect(onHand()).toHaveText("9");
      });
      await action(page, "倉庫", "selectOption", "primary warehouse is untouched", async () => {
        await warehouse.selectOption({ label: primary });
        await expect(page).toHaveURL(url => url.searchParams.get("warehouse") === primaryID); await expect(onHand()).toHaveText("24");
      });

      // (b) archived shows the archived SKU, active hides it.
      await page.goto(`${root("zh-TW", "inventory")}&q=${encodeURIComponent("帆布收納袋")}`); await heading(page, "庫存");
      await action(page, "狀態", "selectOption", "archived shows the archived C08", async () => {
        await status.selectOption({ label: "已封存" });
        await expect(page).toHaveURL(url => url.searchParams.get("status") === "archived");
        await expect(page.getByText("C08 · 帆布收納袋", { exact: true })).toBeVisible();
      });
      await action(page, "狀態", "selectOption", "active hides the archived C08", async () => {
        await status.selectOption({ label: "販售中" });
        await expect(page).toHaveURL(url => url.searchParams.get("status") === "active");
        await expect(empty).toBeVisible(); await expect(page.getByText("C08 · 帆布收納袋", { exact: true })).toHaveCount(0);
      });

      // (d) a real Go-encoded cursor (second warehouse, q=A12, active, after A12) is dropped by either selector.
      const paged = `${root("zh-TW", "inventory")}&warehouse=${secondID}&q=A12&status=active&cursor=${cursor}`;
      await page.goto(paged); await heading(page, "庫存");
      await expect(empty).toBeVisible(); await expect(page.getByTestId("inventory-page").getByRole("alert")).toHaveCount(0); // the page error, not Next's hidden route announcer
      await action(page, "狀態", "selectOption", "status change drops the cursor", async () => {
        await status.selectOption({ label: "全部狀態" });
        await expect(page).toHaveURL(url => url.searchParams.get("status") === "all" && !url.searchParams.has("cursor"));
        await expect(onHand()).toHaveText("9");
      });
      await page.goto(paged); await heading(page, "庫存"); await expect(empty).toBeVisible();
      await action(page, "倉庫", "selectOption", "warehouse change drops the cursor", async () => {
        await warehouse.selectOption({ label: primary });
        await expect(page).toHaveURL(url => url.searchParams.get("warehouse") === primaryID && !url.searchParams.has("cursor"));
        await expect(onHand()).toHaveText("24");
      });
    });
  }
});
