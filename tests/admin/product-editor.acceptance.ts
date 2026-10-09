// Purpose: Exercise product-document create/edit, SKU matrix, inventory and publishing through real admin interactions.
// Depends on: Playwright, catalog acceptance fixtures, Node evidence I/O and the caller's production Next/Go/isolated PG stack.
// Used by: catalog-core browser mode PE12–PE17 and --browser-product-editor; provider writes remain MOCK.
// PE12–PE17: real merchant clicks through the production BFF/Go/isolated PG fixture.
// evaluate is read-only geometry; control HTTP is buyer readback, never a substitute for UI writes.
import { expect, test, type Page } from "@playwright/test";
import { mkdir, writeFile, readFile } from "node:fs/promises";
import path from "node:path";
import { createHash } from "node:crypto";
import axe from "axe-core";
import { productEditorCopy } from "../../apps/admin/lib/product-editor-copy";
import { assertProductEditorReservedLayout } from "./product-editor-layout.acceptance";
import { assertProductListLayout } from "./product-list-layout.acceptance";
/** Registers real-click product acceptance cases; the calling runner owns test fixtures and cleanup. */
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
          [390, 844],
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
          if (width === 390) {
            const toggle = page.locator(".pe-readiness-toggle");
            const checklist = page.locator("#pe-readiness-section");
            await expect(toggle).toBeVisible();
            await expect(toggle).toHaveAttribute("aria-expanded", "false");
            await expect(checklist).toBeHidden();
            await toggle.click();
            await expect(toggle).toHaveAttribute("aria-expanded", "true");
            await expect(checklist).toBeVisible();
            const missing = checklist.locator(".pe-readiness:has([data-ready=false])");
            const missingCount = await missing.count();
            expect(missingCount).toBeGreaterThan(0);
            for (let i = 0; i < missingCount; i++) {
              const entry = missing.nth(i);
              const target = await entry.getAttribute("aria-controls");
              const label = await entry.innerText();
              expect(target).toBeTruthy();
              await entry.click();
              await expect(entry).toHaveAttribute("aria-current", "location");
              // READ/MEASURE only: the actual checklist click must focus a visible field in its section.
              await expect.poll(() => page.locator(`[data-testid=product-fields] [id="${target}"]`).evaluate((section) => {
                const active = document.activeElement;
                if (!(active instanceof HTMLElement) || !section.contains(active)) return false;
                const field = active.getBoundingClientRect(), pane = section.closest(".pe-fields")!.getBoundingClientRect();
                return field.height > 0 && field.top >= pane.top && field.bottom <= pane.bottom;
              })).toBe(true);
              ledger.push({ page: "new", locale, width, control: `readiness/${label}`, action: "click missing item", expected: `visible field focus in ${target}`, actual: "PASS" });
            }
            await toggle.click();
            await expect(toggle).toHaveAttribute("aria-expanded", "false");
            await expect(checklist).toBeHidden();
            ledger.push({ page: "new", locale, width, control: "readiness toggle", action: "click expand, missing items, click collapse", expected: "aria-expanded false→true→false; missing list visible then hidden; entries navigate", actual: "PASS", persistence: "view state only" });
          }
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
            (width === 1586 || width === 390)
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
      await expect(page.getByTestId("media-main-list").locator(".pm-photo")).toHaveCount(3);
      await page
        .getByRole("button", { name: "Move later Main images 1", exact: true })
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
      for (const locale of ["zh-TW", "zh-CN", "en"] as const) {
        await page.getByTestId("locale-switch").selectOption(locale);
        await expect(page).toHaveURL(new RegExp(`/${locale}/products`));
        for (const [width, height] of [[1586, 992], [390, 844]]) {
          await page.setViewportSize({ width, height });
          await assertProductListLayout(page);
          await shot(`list-populated-${locale}-${width}`);
        }
      }
      await page.setViewportSize({ width: 1586, height: 992 });
      await page.getByRole("checkbox", { name: "All", exact: true }).check();
      await page
        .getByTestId("product-batch")
        .getByRole("button", { name: "Unpublish to draft", exact: true })
        .click();
      await expect(page.getByTestId("product-batch-results")).toBeVisible();
      await page.reload();
      await expect(page.locator(".product-status").filter({ hasText: /^Draft$/ })).toHaveCount(3);
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
      const editReadiness = page.locator(".pe-readiness-toggle");
      const editChecklist = page.locator("#pe-readiness-section");
      await expect(editReadiness).toBeVisible();
      await expect(editReadiness).toHaveAttribute("aria-expanded", "false");
      await expect(editChecklist).toBeHidden();
      await editReadiness.click();
      await expect(editReadiness).toHaveAttribute("aria-expanded", "true");
      await expect(editChecklist).toBeVisible();
      await editReadiness.click();
      await expect(editReadiness).toHaveAttribute("aria-expanded", "false");
      await expect(editChecklist).toBeHidden();
      ledger.push({ page: "edit", locale: "en", width: 375, control: "readiness toggle", action: "click expand, click collapse", expected: "aria-expanded false→true→false; checklist visible then hidden", actual: "PASS", persistence: "view state only" });
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
      // Keep additional products after the frozen list-count checks. This uses
      // the same isolated PG fixture; every write goes through a real UI save.
      await page.setViewportSize({ width: 390, height: 844 });
      const mobileMatrixTag = `M${Date.now().toString().slice(-10)}`;
      for (const [localeIndex, locale] of (["zh-TW", "zh-CN", "en"] as const).entries()) {
        const c = productEditorCopy[locale];
        await page.goto(url(locale, "products/new"));
        await page.getByTestId("product-name").fill(`${tag} mobile matrix ${locale}`);
        const axesStart = writes.length;
        for (const [i, name, values] of [[0, c.color, "White, Black"], [1, c.size, "S"]] as const) {
          await page.getByTestId("axis-add").click();
          await page.getByTestId(`axis-name-${i}`).fill(name);
          await page.getByTestId(`axis-values-${i}`).fill(values);
          await page.getByTestId(`axis-values-${i}`).press("Enter");
          await expect(page.getByTestId(`axis-name-${i}`)).toHaveValue(name);
          await expect(page.locator(".product-axis").nth(i).locator(".pe-value-chips > span"))
            .toHaveCount(i === 0 ? 2 : 1);
          ledger.push({ page: "mobile matrix new", locale, width: 390, control: `axis-add/axis-name-${i}/axis-values-${i}`, action: "click fill Enter", expected: { name, values }, actual: "PASS", persistence: "local draft until UI save" });
        }
        await expect(page.locator('[data-testid^="matrix-row-"]')).toHaveCount(2);
        expect(writes.length).toBe(axesStart);
        for (const editing of [false, true]) {
          const phase = editing ? "edit" : "new";
          const rows = ["White / S", "Black / S"].map((name, i) => ({
            name,
            price: String((editing ? 80 : 60) + i),
            compare: String((editing ? 120 : 100) + i),
            quantity: String((editing ? 7 : 5) + i),
            untracked: (i === 1) !== editing,
            code: `${mobileMatrixTag}-${localeIndex}-${i}`,
            keyword: `${mobileMatrixTag}${localeIndex}${editing ? "E" : "C"}${i}`,
          }));
          const fieldsFor = (i: number, existing: boolean) => {
            const row = page.getByTestId(`matrix-row-${i}`), state = rows[i];
            return [
              { control: row.getByTestId(`new-price-${i}`), label: c.price, value: state.price },
              { control: row.getByTestId(`matrix-compare-${i}`), label: c.compare, value: state.compare },
              { control: row.getByTestId(`matrix-quantity-${i}`), label: state.untracked ? c.max : existing ? c.targetQty : c.quantity, value: state.quantity },
              { control: row.getByTestId(`matrix-code-${i}`), label: c.code, value: state.code },
              { control: row.getByRole("textbox", { name: `${c.keyword} ${i + 1}`, exact: true }), label: c.keyword, value: state.keyword },
            ];
          };
          for (const [i, state] of rows.entries()) {
            const row = page.getByTestId(`matrix-row-${i}`);
            await expect(row.locator("strong")).toHaveText(state.name);
            const selected = row.getByTestId(`matrix-select-${i}`);
            await selected.check();
            await expect(selected).toBeChecked();
            await selected.uncheck();
            await expect(selected).not.toBeChecked();
            ledger.push({ page: `mobile matrix ${phase}`, locale, width: 390, row: state.name, control: `matrix-select-${i}`, action: "check uncheck", expected: "selected true→false", actual: "PASS", persistence: "selection is view state" });
            const tracking = row.getByRole("checkbox", { name: c.untracked, exact: true });
            // Exercise both tracking states, then leave distinct persisted modes.
            await tracking.check();
            await expect(tracking).toBeChecked();
            await tracking.uncheck();
            await expect(tracking).not.toBeChecked();
            if (state.untracked) await tracking.check();
            await expect(tracking).toBeChecked({ checked: state.untracked });
            await expect(tracking.locator("..")).toHaveText(c.untracked);
            await expect(tracking.locator("..")).toBeVisible();
            ledger.push({ page: `mobile matrix ${phase}`, locale, width: 390, row: state.name, control: c.untracked, action: "check uncheck; choose mode", expected: { untracked: state.untracked }, actual: "PASS", persistence: "verified after UI save below" });
            for (const field of fieldsFor(i, editing)) {
              const label = field.control.locator("..").locator(":scope > span");
              await expect(label, `${locale} ${phase} ${state.name}: visible ${field.label}`).toBeVisible();
              await expect(label).toHaveText(field.label);
              await label.scrollIntoViewIfNeeded();
              await expect(label).toBeInViewport({ ratio: 0.99 });
              if (editing && field.label === c.code) await expect(field.control).toBeDisabled();
              else await field.control.fill(field.value);
              await expect(field.control).toHaveValue(field.value);
              ledger.push({ page: `mobile matrix ${phase}`, locale, width: 390, row: state.name, control: field.label, action: editing && field.label === c.code ? "verify existing SKU code disabled" : "fill", expected: { visibleLabel: field.label, value: field.value }, actual: "PASS", persistence: "verified after UI save below" });
            }
            const active = row.getByTestId(`matrix-active-${i}`);
            await active.uncheck();
            await expect(active).not.toBeChecked();
            await active.check();
            await expect(active).toBeChecked();
            const activeLabel = row.locator("label.pe-check > span");
            await expect(activeLabel).toHaveText(c.active);
            await expect(activeLabel).toBeVisible();
            await activeLabel.scrollIntoViewIfNeeded();
            await expect(activeLabel).toBeInViewport({ ratio: 0.99 });
            ledger.push({ page: `mobile matrix ${phase}`, locale, width: 390, row: state.name, control: `matrix-active-${i}`, action: "uncheck check", expected: { visibleLabel: c.active, enabled: true }, actual: "PASS", persistence: "verified after UI save below" });
          }
          const saveStart = writes.length;
          const response = page.waitForResponse((r) => r.url().includes(`/api/stores/${store}/products/`) && r.url().endsWith("/document") && r.request().method() !== "GET");
          await page.getByTestId(editing ? "product-save" : "product-create").click();
          expect((await response).ok()).toBe(true);
          if (!editing) {
            const result = page.getByTestId("product-save-result");
            await expect(result).toBeVisible();
            await result.getByRole("link", { name: c.save, exact: true }).click();
          }
          await expect(page.getByTestId("product-save")).toBeEnabled();
          await page.reload();
          await expect(page.locator('[data-testid^="matrix-row-"]')).toHaveCount(2);
          for (const [i, name, values] of [[0, c.color, ["White", "Black"]], [1, c.size, ["S"]]] as const) {
            await expect(page.getByTestId(`axis-name-${i}`)).toHaveValue(name);
            const chips = page.locator(".product-axis").nth(i).locator(".pe-value-chips > span");
            await expect(chips).toHaveCount(values.length);
            await expect(chips).toContainText([...values]);
            ledger.push({ page: `mobile matrix ${phase}`, locale, width: 390, control: `saved axis-${i}`, action: "UI save, editor reopen/reload", expected: { name, values }, actual: "PASS", tier: "BROWSER+REAL_PG" });
          }
          for (const [i, state] of rows.entries()) {
            const row = page.getByTestId(`matrix-row-${i}`);
            await expect(row.locator("strong")).toHaveText(state.name);
            for (const field of fieldsFor(i, true)) {
              const label = field.control.locator("..").locator(":scope > span");
              await expect(label).toBeVisible();
              await expect(label).toHaveText(field.label);
              await label.scrollIntoViewIfNeeded();
              await expect(label).toBeInViewport({ ratio: 0.99 });
              await expect(field.control).toHaveValue(field.value);
            }
            await expect(row.getByRole("checkbox", { name: c.untracked, exact: true })).toBeChecked({ checked: state.untracked });
            await expect(row.getByTestId(`matrix-active-${i}`)).toBeChecked();
            await expect(row.getByTestId(`matrix-code-${i}`)).toBeDisabled();
            ledger.push({ page: `mobile matrix ${phase}`, locale, width: 390, row: state.name, control: "saved row readback", action: "UI save, editor reopen/reload, read visible fields", expected: state, actual: "PASS", tier: "BROWSER+REAL_PG" });
          }
          expect(writes.slice(saveStart).filter((r) => r.path.endsWith("/document"))).toHaveLength(1);
          const capture = `mobile-matrix-${locale}-390-${phase}`;
          await page.getByTestId("new-price-0").scrollIntoViewIfNeeded();
          await shot(capture);
          ledger.push({ page: `mobile matrix ${phase}`, locale, width: 390, control: editing ? "product-save" : "product-create", action: "click reload", expected: "one document write; both matrix rows persist", actual: "PASS", screenshot: `${capture}.png`, tier: "BROWSER+REAL_PG" });
        }
        // Enabled=false archives an existing SKU; the detail contract returns
        // active SKUs only. After reload only Black remains visible; White's
        // archived data is verified separately by the Go PG readback.
        const white = page.getByTestId("matrix-row-0"), black = page.getByTestId("matrix-row-1");
        const whiteID = await white.getAttribute("data-sku-id"), blackID = await black.getAttribute("data-sku-id");
        expect(whiteID).toMatch(/^[0-9a-f-]{36}$/);
        expect(blackID).toMatch(/^[0-9a-f-]{36}$/);
        await white.getByTestId("matrix-active-0").uncheck();
        await expect(white.getByTestId("matrix-active-0")).not.toBeChecked();
        const archiveStart = writes.length;
        const archivedResponse = page.waitForResponse((r) => r.url().includes(`/api/stores/${store}/products/`) && r.url().endsWith("/document") && r.request().method() !== "GET");
        await page.getByTestId("product-save").click();
        expect((await archivedResponse).ok()).toBe(true);
        await expect(page.getByTestId("product-save")).toBeEnabled();
        await page.reload();
        const savedMatrix = page.getByTestId("variant-matrix");
        await expect(savedMatrix.locator('[data-testid^="matrix-row-"]')).toHaveCount(1);
        await expect(savedMatrix.locator(`[data-sku-id="${whiteID}"]`)).toHaveCount(0);
        await expect(savedMatrix.locator("strong")).toHaveText(["Black / S"]);
        const savedBlack = savedMatrix.locator(`[data-sku-id="${blackID}"]`);
        await expect(savedBlack).toHaveCount(1);
        const savedPrice = savedBlack.getByTestId("new-price-0");
        await savedPrice.scrollIntoViewIfNeeded();
        await expect(savedPrice).toBeVisible();
        await expect(savedPrice).toBeInViewport({ ratio: 0.99 });
        await expect(savedPrice).toHaveValue("81");
        await expect(savedBlack.getByTestId("matrix-compare-0")).toHaveValue("121");
        await expect(savedBlack.getByTestId("matrix-quantity-0")).toHaveValue("8");
        await expect(savedBlack.getByRole("checkbox", { name: c.untracked, exact: true })).not.toBeChecked();
        await expect(savedBlack.getByTestId("matrix-code-0")).toHaveValue(`${mobileMatrixTag}-${localeIndex}-1`);
        await expect(savedBlack.getByTestId("matrix-code-0")).toBeDisabled();
        await expect(savedBlack.getByRole("textbox", { name: `${c.keyword} 1`, exact: true })).toHaveValue(`${mobileMatrixTag}${localeIndex}E1`);
        await expect(savedBlack.getByTestId("matrix-active-0")).toBeChecked();
        expect(writes.slice(archiveStart).filter((r) => r.path.endsWith("/document"))).toHaveLength(1);
        const archiveCapture = `mobile-matrix-${locale}-390-archived`;
        await shot(archiveCapture);
        ledger.push({ page: "mobile matrix edit", locale, width: 390, control: "matrix-active-0/product-save", action: "uncheck White, click save, reload", expected: { archivedSKU: whiteID, white: "absent from active-only matrix by SKU ID and name", visibleRows: 1, black: { id: blackID, price: "81", compare: "121", trackedQuantity: "8", active: true } }, actual: "PASS", screenshot: `${archiveCapture}.png`, tier: "BROWSER+REAL_PG" });

        // A separate product preserves every preceding create/edit/archive
        // proof while exercising axis removal on persisted mobile SKUs.
        await page.goto(url(locale, "products/new"));
        await page.getByTestId("product-name").fill(`${tag} mobile axis remove ${locale}`);
        for (const [i, name, values] of [[0, c.color, "White, Black"], [1, c.size, "S"]] as const) {
          await page.getByTestId("axis-add").click();
          await page.getByTestId(`axis-name-${i}`).fill(name);
          await page.getByTestId(`axis-values-${i}`).fill(values);
          await page.getByTestId(`axis-values-${i}`).press("Enter");
        }
        await expect(page.locator('[data-testid^="matrix-row-"]')).toHaveCount(2);
        for (let i = 0; i < 2; i++) {
          const row = page.getByTestId(`matrix-row-${i}`);
          await row.getByTestId(`new-price-${i}`).fill(String(90 + i));
          await row.getByTestId(`matrix-compare-${i}`).fill(String(130 + i));
          await row.getByTestId(`matrix-quantity-${i}`).fill(String(3 + i));
          await row.getByTestId(`matrix-code-${i}`).fill(`${mobileMatrixTag}-A${localeIndex}-${i}`);
        }
        const axisCreateStart = writes.length;
        const axisCreated = page.waitForResponse((r) => r.url().includes(`/api/stores/${store}/products/`) && r.url().endsWith("/document") && r.request().method() !== "GET");
        await page.getByTestId("product-create").click();
        expect((await axisCreated).ok()).toBe(true);
        const axisResult = page.getByTestId("product-save-result");
        await expect(axisResult).toBeVisible();
        await axisResult.getByRole("link", { name: c.save, exact: true }).click();
        await expect(page.getByTestId("product-save")).toBeEnabled();
        await page.reload();
        const axisMatrix = page.getByTestId("variant-matrix");
        await expect(axisMatrix.locator('[data-testid^="matrix-row-"]')).toHaveCount(2);
        await expect(axisMatrix.locator("strong")).toHaveText(["White / S", "Black / S"]);
        const oldAxisIDs: string[] = [];
        for (let i = 0; i < 2; i++) {
          const row = page.getByTestId(`matrix-row-${i}`);
          const skuID = await row.getAttribute("data-sku-id");
          expect(skuID).toMatch(/^[0-9a-f-]{36}$/);
          oldAxisIDs.push(skuID!);
          await expect(row.getByTestId(`new-price-${i}`)).toHaveValue(String(90 + i));
          await expect(row.getByTestId(`matrix-compare-${i}`)).toHaveValue(String(130 + i));
          await expect(row.getByTestId(`matrix-quantity-${i}`)).toHaveValue(String(3 + i));
          await expect(row.getByTestId(`matrix-code-${i}`)).toHaveValue(`${mobileMatrixTag}-A${localeIndex}-${i}`);
        }
        expect(new Set(oldAxisIDs).size).toBe(2);
        expect(writes.slice(axisCreateStart).filter((r) => r.path.endsWith("/document"))).toHaveLength(1);
        await expect(page.locator(".product-axis")).toHaveCount(2);
        const removeColor = page.locator(".product-axis").nth(0).locator(".pe-axis-remove");
        await expect(removeColor).toHaveAccessibleName(`${c.remove} ${c.axis} 1`);
        await expect(removeColor).toBeEnabled();
        await removeColor.scrollIntoViewIfNeeded();
        await expect(removeColor).toBeInViewport({ ratio: 0.99 });
        const axisRemoveStart = writes.length;
        await removeColor.click();
        await expect(page.locator(".product-axis")).toHaveCount(1);
        await expect(page.getByTestId("axis-name-0")).toHaveValue(c.size);
        await expect(page.locator(".product-axis").locator(".pe-value-chips > span")).toHaveCount(1);
        await expect(page.locator(".product-axis").locator(".pe-value-chips > span")).toContainText(["S"]);
        await expect(axisMatrix.locator('[data-testid^="matrix-row-"]')).toHaveCount(1);
        await expect(axisMatrix.locator("strong")).toHaveText(["S"]);
        expect(writes.length).toBe(axisRemoveStart);
        ledger.push({ page: "mobile axis remove", locale, width: 390, control: ".pe-axis-remove (Color axis 1)", action: "real click", expected: { axes: [c.size], values: ["S"], visibleRows: "2→1", row: "S" }, actual: "PASS", persistence: "local draft until UI save below" });
        const remaining = page.getByTestId("matrix-row-0");
        await remaining.getByTestId("new-price-0").fill("95");
        await remaining.getByTestId("matrix-compare-0").fill("145");
        await remaining.getByTestId("matrix-quantity-0").fill("9");
        await expect(remaining.getByTestId("matrix-code-0")).toBeDisabled();
        await page.getByTestId("bulk-open").click();
        await expect(page.getByTestId("bulk-field").locator('option[value="code"]')).toHaveCount(0);
        await page.getByTestId("bulk-field").press("Escape");
        await expect(remaining.getByRole("checkbox", { name: c.untracked, exact: true })).not.toBeChecked();
        await expect(remaining.getByTestId("matrix-active-0")).toBeChecked();
        const axisSaved = page.waitForResponse((r) => r.url().includes(`/api/stores/${store}/products/`) && r.url().endsWith("/document") && r.request().method() !== "GET");
        const axisConfirmation = page.waitForEvent("dialog");
        await page.getByTestId("product-save").click();
        expect((await axisConfirmation).message()).toBe(c.archiveRows);
        expect((await axisSaved).ok()).toBe(true);
        await expect(page.getByTestId("product-save")).toBeEnabled();
        await page.reload();
        await expect(page.locator(".product-axis")).toHaveCount(1);
        await expect(page.getByTestId("axis-name-0")).toHaveValue(c.size);
        await expect(page.locator(".product-axis").locator(".pe-value-chips > span")).toHaveCount(1);
        await expect(page.locator(".product-axis").locator(".pe-value-chips > span")).toContainText(["S"]);
        await expect(axisMatrix.locator('[data-testid^="matrix-row-"]')).toHaveCount(1);
        await expect(axisMatrix.locator("strong")).toHaveText(["S"]);
        for (const oldID of oldAxisIDs) await expect(axisMatrix.locator(`[data-sku-id="${oldID}"]`)).toHaveCount(0);
        const newAxisID = await remaining.getAttribute("data-sku-id");
        expect(newAxisID).toMatch(/^[0-9a-f-]{36}$/);
        expect(oldAxisIDs).not.toContain(newAxisID);
        await expect(remaining.getByTestId("new-price-0")).toHaveValue("95");
        await expect(remaining.getByTestId("matrix-compare-0")).toHaveValue("145");
        await expect(remaining.getByTestId("matrix-quantity-0")).toHaveValue("9");
        const generatedCode = await page.getByTestId("product-slug").inputValue();
        expect(generatedCode).not.toBe("");
        await expect(remaining.getByTestId("matrix-code-0")).toHaveValue(generatedCode);
        await page.reload();
        await expect(remaining.getByTestId("matrix-code-0")).toHaveValue(generatedCode);
        await expect(remaining.getByTestId("matrix-code-0")).toBeDisabled();
        await expect(remaining.getByRole("checkbox", { name: c.untracked, exact: true })).not.toBeChecked();
        await expect(remaining.getByTestId("matrix-active-0")).toBeChecked();
        expect(writes.slice(axisRemoveStart).filter((r) => r.path.endsWith("/document"))).toHaveLength(1);
        const removeCapture = `mobile-axis-remove-${locale}-390-saved`;
        await remaining.getByTestId("new-price-0").scrollIntoViewIfNeeded();
        await shot(removeCapture);
        ledger.push({ page: "mobile axis remove", locale, width: 390, control: "product-save", action: "click, confirm archive, reload", expected: { axes: [c.size], values: ["S"], archivedSKUs: oldAxisIDs, activeSKU: newAxisID, generatedCode, row: "S", price: "95", compare: "145", trackedQuantity: "9", oneDocumentWrite: true }, actual: "PASS", screenshot: `${removeCapture}.png`, tier: "BROWSER+REAL_PG" });
      }
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
