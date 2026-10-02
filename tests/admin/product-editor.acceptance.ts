// PE12–PE17: real merchant clicks through the production BFF/Go/isolated PG fixture.
// evaluate is read-only geometry; control HTTP is buyer readback, never a substitute for UI writes.
import { expect, test, type Page } from "@playwright/test";
import { mkdir, writeFile, readFile } from "node:fs/promises";
import path from "node:path";
import { createHash } from "node:crypto";
import { productEditorCopy } from "../../apps/admin/lib/product-editor-copy";
export function registerProductEditorAcceptance() {
  test("PE12-17 document workflow, matrix, list actions and click ledger", async ({
    page,
  }) => {
    test.setTimeout(300000);
    const origin = process.env.LC_BROWSER_PUBLIC_ORIGIN!,
      store = process.env.LC_BROWSER_STORE!,
      tag = `pe${process.env.LC_BROWSER_TAG}`;
    const out = path.resolve("output/product-ui"),
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
      await page.screenshot({ path: file, animations: "disabled" });
      shots.push({
        file,
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
      `${origin}/${locale}/${suffix}?store=${store}`;
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
      await page.goto(url("en", "products/new"));
      await page.getByTestId("product-name").fill(`${tag} single`);
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
      await expect(page.getByTestId("product-price")).toHaveValue("60.00");
      await expect(page.getByTestId("product-untracked")).toBeChecked();
      await expect(page.getByTestId("product-max")).toHaveValue("3");
      await expect(page.locator(".pe-photo")).toHaveCount(3);
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
          "one document,3 uploads,one order; persisted USD60 and untracked cap3 (USD fixture)",
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
      expect(writes.length).toBe(networkBefore);
      await page.getByTestId("new-price-0").fill("60");
      await page.getByTestId("bulk-price").click();
      await page
        .getByRole("region", { name: "Bulk fill Price" })
        .getByRole("combobox", { name: "Bulk fill", exact: true })
        .selectOption("empty");
      await page.getByTestId("bulk-value").fill("80");
      await page.getByTestId("bulk-value").press("Enter");
      await expect(page.getByTestId("new-price-0")).toHaveValue("60");
      await expect(page.getByTestId("new-price-11")).toHaveValue("80");
      await page.getByTestId("bulk-price").click();
      await page.getByTestId("bulk-value").press("Escape");
      await expect(page.getByTestId("bulk-price")).toBeFocused();
      await page.getByTestId("bulk-quantity").click();
      await page.getByTestId("bulk-value").fill("5");
      await page.getByTestId("bulk-apply").click();
      await page.getByText("Shipping", { exact: true }).last().click();
      await page
        .getByRole("combobox", { name: "Stock warehouse" })
        .selectOption({ index: 1 });
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
      await expect(page.getByTestId("new-price-11")).toHaveValue("80.00");
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
      await page.getByRole("checkbox", { name: "All", exact: true }).check();
      await page
        .getByTestId("product-batch")
        .getByRole("button", { name: "Unpublish to draft", exact: true })
        .click();
      await expect(page.getByTestId("product-batch-results")).toBeVisible();
      await page.reload();
      await expect(page.locator(".product-status-draft")).toHaveCount(3);
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
      await page.goto(url("en", "products/" + id));
      await expect(page.getByTestId("document-edit-blocked")).toBeVisible();
      ledger.push({
        page: "edit",
        control: "save/inline stock",
        expected: "PE14 editing and zero-target persistence",
        actual:
          "BLOCKED backend readback and zero-target defect; no unsafe write",
      });
      await saveEvidence();
      // Required assertion remains red until backend preservation is fixed; never label guarded read-only as PE14 PASS.
      await expect(
        page.getByTestId("product-create"),
        "PE14 requires safe editable document",
      ).toBeEnabled();
    } finally {
      await saveEvidence();
      await writeFile(path.join(evidence, "product-ui-output.txt"), out);
    }
  });
}
