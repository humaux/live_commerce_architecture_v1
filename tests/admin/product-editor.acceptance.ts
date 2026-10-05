// PE12–PE17: real merchant clicks through the production BFF/Go/isolated PG fixture.
// evaluate is read-only geometry; control HTTP is buyer readback, never a substitute for UI writes.
import { expect, test, type Page } from "@playwright/test";
import { mkdir, writeFile, readFile } from "node:fs/promises";
import path from "node:path";
import { createHash } from "node:crypto";
import axe from "axe-core";
import { productEditorCopy } from "../../apps/admin/lib/product-editor-copy";
import { assertProductEditorReservedLayout } from "./product-editor-layout.acceptance";
export function registerProductEditorAcceptance() {
  test("PE12-17 document workflow, matrix, list actions and click ledger", async ({
    page,
  }) => {
    test.setTimeout(300000);
    const origin = process.env.LC_BROWSER_PUBLIC_ORIGIN!,
      store = process.env.LC_BROWSER_STORE!,
      tag = `pe${process.env.LC_BROWSER_TAG}`;
    const out = path.resolve("output/product-ui-v2"),
      evidence = process.env.LC_BROWSER_EVIDENCE!;
    await mkdir(out, { recursive: true });
    const ledger: unknown[] = [],
      shots: unknown[] = [],
      writes: {
        path: string;
        method: string;
        key: string;
        body: string | null;
      }[] = [];
    page.on("request", (r) => {
      if (r.url().includes("/api/stores/") && r.method() !== "GET")
        writes.push({
          path: new URL(r.url()).pathname,
          method: r.method(),
          key: r.headers()["idempotency-key"] ?? "",
          body: r.postData(),
        });
    });
    const saveEvidence = async () => {
      await writeFile(
        path.join(out, "click-ledger.json"),
        JSON.stringify(ledger, null, 2),
      );
      await writeFile(
        path.join(out, "screenshots.json"),
        JSON.stringify(shots, null, 2),
      );
      await writeFile(
        path.join(out, "write-metadata.json"),
        JSON.stringify(
          writes.map(({ body, ...r }) => r),
          null,
          2,
        ),
      );
    };
    const shot = async (name: string) => {
      const file = path.join(out, `${name}.png`);
      if (/^(list|editor)-(zh-TW|zh-CN|en)-/.test(name)) {
        await page
          .locator("header[data-shell-topbar]")
          .scrollIntoViewIfNeeded();
        await expect.poll(() => page.evaluate(() => scrollY)).toBe(0);
        await expect
          .poll(
            async () =>
              (await page.locator("header[data-shell-topbar]").boundingBox())
                ?.y,
          )
          .toBe(0);
      }
      await page.screenshot({ path: file, animations: "disabled" });
      shots.push({
        file,
        geometry: await page.evaluate(() => ({
          scrollY,
          width: innerWidth,
          height: innerHeight,
          topbarY: document
            .querySelector("header[data-shell-topbar]")
            ?.getBoundingClientRect().y,
        })),
        sha256: createHash("sha256")
          .update(await readFile(file))
          .digest("hex"),
      });
    };
    const fit = async () =>
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth - innerWidth,
        ),
      ).toBeLessThanOrEqual(1);
    const url = (locale: string, suffix: string) =>
      `${origin}/${locale}/${suffix}${suffix.includes("?") ? "&" : "?"}store=${store}`;
    try {
      await page.goto(origin + "/en/");
      await page
        .getByRole("button", { name: "Sign in with identity service" })
        .click();
      await expect(page.getByTestId("nav-orders")).toBeAttached();
      // Visual/real navigation sweep. Dirty drafts are intentionally discarded by the user-confirmation path.
      page.on("dialog", (dialog) => void dialog.accept());
      for (const locale of ["zh-TW", "zh-CN", "en"] as const) {
        for (const [width, height] of [
          [1366, 768],
          [1586, 992],
          [375, 812],
        ]) {
          await page.setViewportSize({ width, height });
          await page.goto(url(locale, "products"));
          await expect(page.getByTestId("products-table")).toBeVisible();
          await fit();
          const targets = await page
            .locator(
              ".pe-select,.pe-row-actions a,[data-testid=quick-price],[data-testid=quick-stock]",
            )
            .evaluateAll((nodes) =>
              nodes
                .filter((n) => n.getClientRects().length)
                .map((n) => {
                  const r = n.getBoundingClientRect();
                  return { width: r.width, height: r.height };
                }),
            );
          for (const target of targets) {
            expect(target.width).toBeGreaterThanOrEqual(44);
            expect(target.height).toBeGreaterThanOrEqual(44);
          }
          await shot(`list-${locale}-${width}`);
          await page.getByTestId("product-new").click();
          await page
            .getByTestId("product-name")
            .fill(`${tag} studio collection`);
          await page.getByTestId("product-price").fill("60");
          await page.getByTestId("product-untracked").check();
          await page.getByTestId("product-max").fill("3");
          await page
            .getByTestId("product-description")
            .fill(
              locale === "en"
                ? "Synthetic product for isolated acceptance only."
                : "隔離驗收用的虛構商品。",
            );
          await page.getByTestId("product-keyword").fill("P12");
          await assertProductEditorReservedLayout(
            page,
            locale === "en" && (width === 1586 || width === 375)
              ? (section) => shot(`editor-${locale}-${width}-${section}`)
              : undefined,
          );
          await page
            .getByRole("navigation", {
              name: productEditorCopy[locale].progress,
            })
            .getByRole("button", {
              name: productEditorCopy[locale].media,
              exact: true,
            })
            .click();
          await fit();
          await page.addScriptTag({ content: axe.source });
          const accessibility = await page.evaluate(
            async () => await (window as any).axe.run(),
          );
          expect(
            accessibility.violations.filter((v: any) =>
              ["serious", "critical"].includes(v.impact),
            ),
          ).toEqual([]);
          await shot(`editor-${locale}-${width}`);
          ledger.push({
            page: "new",
            locale,
            width,
            control: "section navigation/name/price/untracked/max/keyword",
            action: "click fill check",
            expected:
              "visible draft values and section focus, no page overflow",
            actual: "PASS",
            persistence: "draft intentionally unsaved",
          });
        }
      }
      await page.setViewportSize({ width: 1586, height: 992 });
      const dirtyURL = page.url();
      let leavePrompts = 0;
      page.removeAllListeners("dialog");
      page.on("dialog", (dialog) => {
        leavePrompts++;
        void dialog.dismiss();
      });
      await page.getByTestId("nav-products").click();
      await expect.poll(() => leavePrompts).toBe(1);
      await expect(page).toHaveURL(dirtyURL);
      await expect(page.getByTestId("product-price")).toHaveValue("60");
      await page.getByTestId("product-back").click();
      await expect.poll(() => leavePrompts).toBe(2);
      await expect(page).toHaveURL(dirtyURL);
      page.removeAllListeners("dialog");
      const hardLeavePrompts: string[] = [];
      page.on("dialog", (dialog) => {
        hardLeavePrompts.push(dialog.type());
        void dialog.accept();
      });
      // Re-select the fixture store through the actual shell control; it performs a hard navigation.
      await page.getByTestId("shell-store-selector").selectOption(store);
      await page.waitForURL((value) => /^\/en\/?$/.test(value.pathname));
      expect(hardLeavePrompts).toEqual(["confirm"]);
      page.removeAllListeners("dialog");
      page.on("dialog", (dialog) => void dialog.accept());
      ledger.push({
        page: "editor",
        control: "shell and page leave guard",
        action: "click + dismiss",
        expected: "one confirmation per navigation; draft retained",
        actual: "PASS",
      });
      await page.goto(url("en", "products/new"));
      await page.goto(url("en", "collections"));
      await page.getByTestId("collection-new").click();
      await page.getByTestId("collection-title").fill(`${tag} picks`);
      await page.getByTestId("collection-save").click();
      await expect(page.getByTestId("collection-members")).toBeVisible();
      await page.goto(url("en", "products"));
      await page.getByTestId("product-new").click();
      await page.getByTestId("product-name").fill(`${tag} single`);
      await page.getByTestId("product-name").focus();
      await page.keyboard.press("Tab");
      await expect(page.getByTestId("product-description")).toBeFocused();
      await page.keyboard.press("Tab");
      await expect(page.getByTestId("product-keyword")).toBeFocused();
      await page
        .locator("#collections")
        .getByRole("checkbox", { name: `${tag} picks`, exact: true })
        .check();
      await page.getByTestId("product-price").fill("60");
      await page.getByTestId("product-untracked").check();
      await page.getByTestId("product-max").fill("3");
      await page
        .getByTestId("product-keyword")
        .fill(`P${Date.now().toString().slice(-10)}`);
      const png = Buffer.from(
        "iVBORw0KGgoAAAANSUhEUgAAAAEAAAAB" +
          "CAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==",
        "base64",
      );
      await page.getByTestId("photo-input").setInputFiles(
        [1, 2, 3].map((i) => ({
          name: `fixture-${i}.png`,
          mimeType: "image/png",
          buffer: png,
        })),
      );
      await expect(page.locator(".pe-photo")).toHaveCount(3);
      await page
        .getByRole("button", { name: "Move later 1", exact: true })
        .click();
      const before = writes.length;
      await page.getByTestId("product-publish").dblclick();
      await expect(page.getByTestId("product-save-result")).toBeVisible({
        timeout: 45000,
      });
      const createdWrites = writes.slice(before);
      expect(
        createdWrites.filter((r) => r.path.endsWith("/products/document")),
      ).toHaveLength(1);
      expect(
        createdWrites.filter((r) => /\/images$/.test(r.path)),
      ).toHaveLength(3);
      expect(
        createdWrites.filter((r) => r.path.endsWith("/images/order")),
      ).toHaveLength(1);
      for (const write of createdWrites)
        expect(write.key).toMatch(/^[0-9a-f-]{36}$/);
      const productLink = page
        .getByTestId("product-save-result")
        .getByRole("link", { name: "Save changes" });
      const href = await productLink.getAttribute("href");
      const id = /products\/([0-9a-f-]{36})/.exec(href!)![1];
      await productLink.click();
      await expect(page).toHaveURL(url("en", "products/" + id));
      await page.reload();
      await expect(page.getByTestId("product-price")).toHaveValue("60");
      await expect(page.getByTestId("product-untracked")).toBeChecked();
      await expect(page.getByTestId("product-max")).toHaveValue("3");
      await expect(page.getByTestId("photo-row")).toHaveCount(3);
      const buyer = await fetch(
        `${process.env.LC_BROWSER_CONTROL}/?path=${encodeURIComponent(`/v1/buyer/catalog/v2/products?q=${tag}`)}`,
        { headers: { "X-Gate-Key": process.env.LC_BROWSER_CONTROL_KEY! } },
      );
      expect(buyer.status).toBe(200);
      const readback = await buyer.json();
      expect(readback.status).toBe(200);
      expect(readback.body).toContain(id);
      ledger.push({
        page: "create",
        control: "3 file inputs/reorder/publish double click",
        action: "setInputFiles click dblclick reload",
        expected:
          "one document,3 uploads,one order; persisted NT$60 and untracked cap3",
        actual: "PASS",
        tier: "BROWSER+REAL_PG; buyer HTTP readback",
      });
      await page.goto(url("en", "products/new"));
      await page.getByTestId("product-name").fill(`${tag} matrix`);
      const networkBefore = writes.length;
      await page.getByTestId("axis-add").click();
      await page.getByTestId("axis-name-0").fill("Color");
      await page.getByTestId("axis-values-0").fill("White, Black, Blue");
      await page.getByTestId("axis-values-0").press("Enter");
      await page.getByTestId("axis-add").click();
      await page.getByTestId("axis-name-1").fill("Size");
      await page.getByTestId("axis-values-1").fill("S, M, L, XL");
      await page.getByTestId("axis-values-1").press("Enter");
      await expect(page.locator('[data-testid^="matrix-row-"]')).toHaveCount(
        12,
      );
      await assertProductEditorReservedLayout(page);
      expect(writes.length).toBe(networkBefore);
      await page.getByTestId("bulk-open").click();
      await page.getByTestId("bulk-value").fill("80");
      await page.getByTestId("bulk-value").press("Enter");
      for (let i = 0; i < 12; i++)
        await expect(page.getByTestId(`new-price-${i}`)).toHaveValue("80");
      await page.getByTestId("new-price-0").fill("60");
      await page.getByTestId("new-price-1").fill("");
      await page.getByTestId("bulk-open").click();
      await page
        .getByRole("region", { name: "Bulk fill Price" })
        .getByRole("combobox", { name: "Bulk fill", exact: true })
        .selectOption("empty");
      await page.getByTestId("bulk-value").fill("80");
      await page.getByTestId("bulk-value").press("Enter");
      await expect(page.getByTestId("new-price-0")).toHaveValue("60");
      await expect(page.getByTestId("new-price-1")).toHaveValue("80");
      await expect(page.getByTestId("new-price-11")).toHaveValue("80");
      await page.getByTestId("bulk-open").click();
      await page.getByTestId("bulk-value").press("Escape");
      await expect(page.getByTestId("bulk-open")).toBeFocused();
      await page.getByTestId("bulk-open").click();
      await page.getByTestId("bulk-field").selectOption("quantity");
      await page.getByTestId("bulk-value").fill("5");
      await page.getByTestId("bulk-apply").click();
      await page.getByText("Shipping", { exact: true }).last().click();
      const stockWarehouse = page.getByRole("combobox", {
        name: "Stock warehouse",
      });
      await expect(stockWarehouse).toHaveCount(0); // dedicated TWD fixture has exactly one warehouse
      await shot("matrix-en-1586");
      await page.getByTestId("product-create").click();
      await expect(page.getByTestId("product-save-result")).toBeVisible({
        timeout: 30000,
      });
      const matrixLink = page
        .getByTestId("product-save-result")
        .getByRole("link", { name: "Save changes" });
      const matrixHref = await matrixLink.getAttribute("href");
      await matrixLink.click();
      await expect(page).toHaveURL(origin + matrixHref);
      await page.reload();
      await expect(page.locator('[data-testid^="matrix-row-"]')).toHaveCount(
        12,
      );
      await page.getByTestId("photo-input").setInputFiles({
        name: "matrix.png",
        mimeType: "image/png",
        buffer: png,
      });
      await expect(page.getByTestId("photo-row")).toHaveCount(1);
      await page.getByTestId("product-publish").click();
      await expect(page.getByTestId("product-save")).toBeEnabled();
      await page.reload();
      await expect(page.getByTestId("product-status")).toHaveValue("active");
      await page.getByTestId("matrix-active-0").uncheck();
      page.removeAllListeners("dialog");
      const confirmation = page.waitForEvent("dialog");
      const archiveStart = writes.length;
      const unpublishClick = page.getByTestId("product-unpublish").click();
      await (await confirmation).dismiss();
      await unpublishClick;
      await expect(page.getByTestId("product-status")).toHaveValue("active");
      expect(writes).toHaveLength(archiveStart);
      await page.getByTestId("matrix-active-0").check();
      page.on("dialog", (dialog) => void dialog.accept());
      await page.getByTestId("product-unpublish").click();
      await expect(page.getByTestId("product-status")).toHaveValue("draft");
      await expect(page.getByTestId("new-price-11")).toHaveValue("80");
      ledger.push({
        page: "matrix",
        control: "axes/bulk only empty/stock/save",
        action: "click fill Enter reload",
        expected: "12 local rows, preserved first price60, others80; persisted",
        actual: "PASS",
      });
      await page.goto(url("en", "products"));
      await page.getByTestId("products-search").fill(tag);
      await page.getByTestId("products-search-submit").click();
      await expect(page.locator('[data-testid^="product-row-"]')).toHaveCount(
        2,
      );
      await page
        .getByTestId(`product-row-${id}`)
        .getByRole("button", { name: "Copy", exact: true })
        .click();
      await expect(page.getByTestId("product-batch-results")).toBeVisible();
      await page.reload();
      await expect(page.locator('[data-testid^="product-row-"]')).toHaveCount(
        3,
      );
      await expect(page.getByTestId("products-tab-active")).toHaveText(
        "Active 1",
      );
      await expect(page.getByTestId("products-tab-draft")).toHaveText(
        "Draft 2",
      );
      await page.getByRole("checkbox", { name: "All", exact: true }).check();
      await page
        .getByTestId("product-batch")
        .getByRole("button", { name: "Unpublish to draft", exact: true })
        .click();
      await expect(page.getByTestId("product-batch-results")).toBeVisible();
      await page.reload();
      await expect(page.locator(".product-status-draft")).toHaveCount(3);
      await expect(page.getByTestId("products-tab-active")).toHaveText(
        "Active 0",
      );
      await expect(page.getByTestId("products-tab-draft")).toHaveText(
        "Draft 3",
      );
      await shot("batch-result-en-1586");
      ledger.push({
        page: "list",
        control: "copy/select all/unpublish",
        action: "click check reload",
        expected: "copy persisted;3 results;all3draft",
        actual: "PASS",
      });
      for (const [width, height] of [
        [1366, 768],
        [1586, 992],
      ]) {
        await page.setViewportSize({ width, height });
        await page.goto(url("en", "inventory"));
        await page.locator("button.product-name").first().click();
        const inspector = page.locator(".inspector");
        await expect(inspector).toBeVisible();
        await expect(inspector.getByTestId("tray-price")).toBeVisible();
        await expect(
          inspector.locator(
            'input[type="file"],input[name="price"],input[name="name"]',
          ),
        ).toHaveCount(0);
        await expect(inspector.locator("a.tray-edit")).toBeVisible();
        await fit();
        await shot(`inventory-en-${width}`);
      }
      ledger.push({
        page: "inventory",
        control: "product inspector",
        action: "click",
        expected:
          "read-only price, no name/price/image editor; link to product editor",
        actual: "PASS",
      });
      await page.setViewportSize({ width: 375, height: 812 });
      await page.goto(url("en", "products/new"));
      await page
        .getByRole("button", { name: "Search engine listing", exact: true })
        .first()
        .click();
      await page
        .getByTestId("product-seo-description")
        .fill("Keyboard and bottom-field reachability");
      const lastField = await page
        .getByTestId("product-seo-description")
        .boundingBox();
      const footer = await page.locator(".pe-savebar").boundingBox();
      expect(lastField!.y + lastField!.height).toBeLessThanOrEqual(footer!.y);
      await shot("editor-bottom-en-375");
      await page.goto(url("en", "products?q=" + tag));
      await page
        .getByTestId(`product-row-${id}`)
        .getByRole("link", { name: "Edit", exact: true })
        .click();
      await expect(
        page.getByTestId("product-save"),
        "PE14 requires safe editable document",
      ).toBeEnabled();
      const editStart = writes.length;
      await page.getByTestId("product-price").fill("80");
      await page.getByTestId("product-save").click();
      await expect(page.getByTestId("product-save")).toBeEnabled();
      await page.reload();
      await expect(page.getByTestId("product-price")).toHaveValue("80");
      const edits = writes
        .slice(editStart)
        .filter((w) => w.path.endsWith("/document"));
      expect(edits).toHaveLength(1);
      const patch = JSON.parse(edits[0].body!);
      expect(Object.keys(patch).sort()).toEqual(["expected_version", "skus"]);
      expect(Object.keys(patch.skus[0]).sort()).toEqual(["id", "price_minor"]);
      expect(patch.skus[0].price_minor).toBe(8000);
      const singleSKU = patch.skus[0].id;
      await expect(page.getByTestId("product-max")).toHaveValue("3");
      await expect(page.getByTestId("photo-row")).toHaveCount(3);
      ledger.push({
        page: "edit",
        control: "price and save",
        action: "fill click reload",
        expected: "one minimal patch; cap and images preserved",
        actual: "PASS",
      });
      await page.goto(url("en", "products?q=" + tag));
      const single = page.getByTestId(`product-row-${id}`);
      const quickStart = writes.length;
      await single.getByTestId("quick-price").click();
      await expect(page.getByTestId("shell-store-selector")).toBeDisabled();
      const quickURL = page.url();
      await page.getByTestId("quick-price-0").fill("90");
      page.removeAllListeners("dialog");
      page.on("dialog", (dialog) => void dialog.dismiss());
      await page.getByTestId("products-ledger-link").click();
      await expect(page).toHaveURL(quickURL);
      await expect(page.getByTestId("quick-price-0")).toBeVisible();
      page.removeAllListeners("dialog");
      page.on("dialog", (dialog) => void dialog.accept());
      await page.getByTestId("quick-price-0").press("Enter");
      await expect(page.getByTestId("product-quick-edit")).toHaveCount(0);
      await expect(page.getByTestId("shell-store-selector")).toBeEnabled();
      await page.reload();
      await expect(single).toContainText("90");
      expect(
        writes.slice(quickStart).filter((w) => w.path.endsWith("/document")),
      ).toHaveLength(1);
      const quick80Start = writes.length;
      await single.getByTestId("quick-price").click();
      await page.getByTestId("quick-price-0").fill("80");
      await page.getByTestId("quick-price-0").press("Enter");
      await expect(page.getByTestId("product-quick-edit")).toHaveCount(0);
      await expect(single).toContainText("NT$80");
      expect(
        writes.slice(quick80Start).filter((w) => w.path.endsWith("/document")),
      ).toHaveLength(1);
      await single.getByTestId("quick-stock").click();
      await page.getByTestId("quick-untracked-0").uncheck();
      await expect(page.getByTestId("quick-delta-0")).toBeDisabled();
      await page.getByTestId("quick-target-0").fill("5");
      await page.getByTestId("quick-save").click();
      await expect(page.getByTestId("product-quick-edit")).toHaveCount(0);
      await single.getByTestId("quick-stock").click();
      await expect(page.getByTestId("quick-target-0")).toHaveValue("5");
      await expect(page.getByTestId("quick-delta-0")).toHaveValue("0");
      await page.getByTestId("quick-delta-0").fill("2");
      await expect(page.getByTestId("quick-target-0")).toHaveValue("7");
      await page.getByTestId("quick-save").click();
      await expect(page.getByTestId("product-quick-edit")).toHaveCount(0);
      await single.getByTestId("quick-stock").click();
      await page.getByTestId("quick-target-0").fill("0");
      await expect(page.getByTestId("quick-delta-0")).toHaveValue("-7");
      await page.getByTestId("quick-save").click();
      await expect(page.getByTestId("product-quick-edit")).toHaveCount(0);
      await page.reload();
      await single.getByTestId("quick-stock").click();
      await expect(page.getByTestId("quick-target-0")).toHaveValue("0");
      await page.keyboard.press("Escape");
      await expect(single.getByTestId("quick-stock")).toBeFocused();
      const matrixId = /products\/([0-9a-f-]{36})/.exec(matrixHref!)![1];
      const matrix = page.getByTestId(`product-row-${matrixId}`);
      await matrix.getByTestId("quick-price").click();
      await expect(page.locator('[data-testid^="quick-sku-"]')).toHaveCount(12);
      await page.getByTestId("quick-price-1").fill("81");
      await page.getByTestId("quick-save").click();
      await expect(page.getByTestId("product-quick-edit")).toHaveCount(0);
      await page.reload();
      await matrix.getByTestId("quick-price").click();
      await expect(page.getByTestId("quick-price-1")).toHaveValue("81");
      await page.keyboard.press("Escape");
      ledger.push({
        page: "list",
        control: "single/multi SKU pencils, inventory delta and zero",
        action: "click fill Enter Escape reload",
        expected:
          "one patch per save, stock0 persists, twelve-SKU popover and focus return",
        actual: "PASS",
      });
      // Controlled read-only ambiguity: do not fabricate a stock quantity when the API has no single warehouse.
      await page.route(
        `**/api/stores/${store}/products/${id}`,
        async (route) => {
          const res = await route.fetch();
          const body = await res.json();
          body.warehouse_id = null;
          for (const s of body.skus) {
            s.on_hand = null;
            s.committed = null;
          }
          await route.fulfill({ response: res, json: body });
        },
      );
      await page.goto(url("en", "products/" + id));
      await expect(page.getByTestId("product-quantity")).toBeDisabled();
      await expect(
        page.getByRole("link", { name: "Open inventory", exact: true }),
      ).toBeVisible();
      await page.unroute(`**/api/stores/${store}/products/${id}`);
      ledger.push({
        page: "edit",
        control: "ambiguous warehouse",
        action: "read-only response MOCK + visible disabled field",
        expected: "no available fallback, disabled target and inventory link",
        actual: "PASS (MOCK read contract)",
      });
      // The PG PE23 gate exercises the actual open-window rule. This isolated
      // response mock proves every locale explains that rule after a real click.
      const documentPattern = `**/api/stores/${store}/products/${id}/document`;
      await page.route(documentPattern, (route) =>
        route.fulfill({ status: 409, json: { code: "live_window_open" } }),
      );
      for (const locale of ["zh-TW", "zh-CN", "en"] as const) {
        await page.goto(url(locale, "products/" + id));
        await page.getByTestId("product-keyword").fill("BLOCKED-LIVE");
        await page.getByTestId("product-save").click();
        await expect(
          page.getByText(productEditorCopy[locale].live_window_open, {
            exact: true,
          }),
        ).toBeVisible();
        ledger.push({
          page: "edit",
          locale,
          control: "live-window keyword refusal",
          action: "click + MOCK 409",
          expected: "explicit translated safety refusal",
          actual: "PASS (MOCK response; real rule covered by PG PE21)",
        });
      }
      await page.unroute(documentPattern);
      await page.goto(url("en", "products"));
      // Fault injection: server commits a copy, but the browser loses that response.
      // After reload the receipt fence must prevent sending a fresh-key duplicate.
      const copyPattern = `**/products/${id}/copy`;
      await page.route(copyPattern, async (route) => {
        await route.fetch();
        await route.abort("failed");
      });
      const beforeUnknown = writes.length;
      await page
        .getByTestId(`product-row-${id}`)
        .getByRole("button", { name: "Copy", exact: true })
        .click();
      await expect(
        page.getByRole("button", {
          name: productEditorCopy.en.retry,
          exact: true,
        }),
      ).toBeVisible();
      await page.reload();
      await expect(
        page.getByText(productEditorCopy.en.listRecoveryRequired, {
          exact: true,
        }),
      ).toBeVisible();
      await expect(page.getByTestId("shell-store-selector")).toBeDisabled();
      const recoveryURL = page.url();
      await page.getByTestId("product-new").click();
      await expect(page).toHaveURL(recoveryURL);
      await expect(
        page
          .getByTestId(`product-row-${id}`)
          .getByRole("button", { name: "Copy", exact: true }),
      ).toBeDisabled();
      expect(
        writes
          .slice(beforeUnknown)
          .filter((r) => r.path.endsWith(`/${id}/copy`)),
      ).toHaveLength(1);
      await page.unroute(copyPattern);
      ledger.push({
        page: "list",
        control: "copy response loss/reload",
        action: "click then injected lost response, reload",
        expected: "one committed command; recovery fence blocks duplicate",
        actual: "PASS",
        tier: "FAULT_INJECTED+REAL_PG",
      });
      await saveEvidence();
    } finally {
      await saveEvidence();
      await writeFile(path.join(evidence, "product-ui-output.txt"), out);
    }
  });
}
