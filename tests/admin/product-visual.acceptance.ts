// Purpose: Assert and capture real product-editor layouts across modes, locales and viewport sizes.
// Depends on: Playwright; Node evidence I/O; product-editor-copy; the caller's production Next/Go/isolated PG fixture.
// Used by: catalog-core.spec.ts under --browser-product-editor; no DOM mutation or API substitute for UI clicks.
import { expect, test, type Page } from "@playwright/test";
import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import { productEditorCopy } from "../../apps/admin/lib/product-editor-copy";

/** Registers screenshot and geometry cases driven by actual product-editor interactions. */
export function registerProductVisualAcceptance() {
  if (process.env.PRODUCT_VISUAL_PHASE === "after")
    test("product visual: compact bulk, selected rows, tags and persistence", async ({
      page,
    }) => {
      const origin = process.env.LC_BROWSER_PUBLIC_ORIGIN!,
        store = process.env.LC_BROWSER_STORE!;
      await page.goto(`${origin}/en/`);
      await page
        .getByRole("button", { name: "Sign in with identity service" })
        .click();
      await expect(page.getByTestId("nav-orders")).toBeAttached();
      await page.goto(`${origin}/en/products?store=${store}`);
      await page.getByTestId("product-new").click();
      await page
        .getByTestId("product-name")
        .fill(`visual-controls-${process.env.LC_BROWSER_TAG}`);
      await page.getByTestId("axis-add").click();
      await page.getByTestId("axis-name-0").fill("Size");
      await page.getByTestId("axis-values-0").fill("S, M, L");
      await page.getByTestId("axis-values-0").press("Enter");
      await expect(page.locator('[data-testid^="matrix-row-"]')).toHaveCount(3);
      await page.getByRole("button", { name: "Remove L", exact: true }).click();
      await expect(page.locator('[data-testid^="matrix-row-"]')).toHaveCount(2);
      await page.getByTestId("axis-values-0").fill("L");
      await page.getByTestId("axis-values-0").press("Enter");
      await expect(page.locator('[data-testid^="matrix-row-"]')).toHaveCount(3);
      const bulk = async (
        field: string,
        value: string,
        op = "set",
        target = "all",
      ) => {
        await page.getByTestId("bulk-open").click();
        await page.getByTestId("bulk-field").selectOption(field);
        await page.getByTestId("bulk-target").selectOption(target);
        await page
          .getByRole("combobox", { name: "Set to", exact: true })
          .selectOption(op);
        await page.getByTestId("bulk-value").fill(value);
        await page.getByTestId("bulk-apply").click();
      };
      await bulk("price", "80");
      await bulk("compare", "100");
      await bulk("quantity", "5");
      await bulk("code", `VIS${process.env.LC_BROWSER_TAG}`);
      await page.getByTestId("matrix-select-1").check();
      await bulk("code", `VIS${process.env.LC_BROWSER_TAG}`, "set", "selected");
      for (let i = 0; i < 3; i++)
        await expect(page.getByTestId(`matrix-code-${i}`)).toHaveValue(
          `VIS${process.env.LC_BROWSER_TAG}${i + 1}`,
        );
      await bulk("price", "90", "set", "selected");
      await bulk("quantity", "2", "subtract", "selected");
      await bulk("keyword", "VISONE", "set", "selected");
      await expect(page.getByTestId("new-price-0")).toHaveValue("80");
      await expect(page.getByTestId("new-price-1")).toHaveValue("90");
      await expect(page.getByTestId("matrix-quantity-1")).toHaveValue("3");
      await bulk("quantity", "2", "add", "selected");
      await expect(page.getByTestId("matrix-quantity-1")).toHaveValue("5");
      await page.getByTestId("product-create").click();
      const result = page.getByTestId("product-save-result");
      await expect(result).toBeVisible();
      await result.locator('a[href*="/products/"]').click();
      await expect(page.getByTestId("product-save")).toBeEnabled();
      await page.reload();
      await expect(page.getByTestId("new-price-1")).toHaveValue("90");
      await expect(page.getByTestId("new-price-0")).toHaveValue("80");
      await expect(page.getByTestId("matrix-quantity-1")).toHaveValue("5");
      await expect(
        page
          .getByTestId("matrix-row-1")
          .getByRole("textbox", { name: "Live keyword 2", exact: true }),
      ).toHaveValue("VISONE");
      // Native clipboard fixture, not a form write: paste through the real keyboard.
      await page
        .context()
        .grantPermissions(["clipboard-read", "clipboard-write"]);
      await page.evaluate(() => navigator.clipboard.writeText("XL\nXXL"));
      await page.getByTestId("axis-values-0").click();
      await page.keyboard.press("ControlOrMeta+V");
      await page.getByTestId("axis-values-0").press("Enter");
      await expect(page.locator('[data-testid^="matrix-row-"]')).toHaveCount(5);
      await page
        .getByRole("button", { name: "Remove XL", exact: true })
        .click();
      await page
        .getByRole("button", { name: "Remove XXL", exact: true })
        .press("Space");
      await expect(page.getByTestId("axis-values-0")).toBeFocused();
      await expect(page.locator('[data-testid^="matrix-row-"]')).toHaveCount(3);
      await page.getByTestId("axis-add").click();
      await page.getByTestId("axis-name-1").fill("Pack");
      const tooMany = Array.from({ length: 34 }, (_, i) => `P${i}`).join(",");
      await page.getByTestId("axis-values-1").fill(tooMany);
      await page.getByTestId("axis-values-1").press("Enter");
      await expect(page.locator(".pe-document").getByRole("alert")).toHaveText(
        productEditorCopy.en.matrixLimit,
      );
      await expect(page.getByTestId("axis-values-1")).toHaveValue(tooMany);
      await expect(page.locator('[data-testid^="matrix-row-"]')).toHaveCount(3);
      await page.getByTestId("axis-values-1").fill("One, Two");
      await page.getByTestId("axis-values-1").press("Enter");
      await expect(page.locator('[data-testid^="matrix-row-"]')).toHaveCount(6);
      await expect(page.getByTestId("axis-values-1")).toHaveValue("");
      const out = path.resolve(process.env.LC_BROWSER_EVIDENCE!, "product-editor-visual");
      await mkdir(out, { recursive: true });
      await writeFile(
        path.join(out, "controls-ledger.json"),
        JSON.stringify(
          {
            status: "PASS",
            tier: "REAL_PG+BROWSER",
            controls: [
              "add axis",
              "values Enter",
              "remove tag",
              "bulk field five options",
              "all/selected",
              "quantity set/add/subtract",
              "create",
              "reload",
            ],
            assertions:
              "3 variants; selected row price90, others80; stock5; selected keyword persisted",
          },
          null,
          2,
        ),
      );
    });
  test("product visual: full-page new/existing empty/three-variant matrix", async ({
    page,
  }) => {
    test.setTimeout(300000);
    const phase = process.env.PRODUCT_VISUAL_PHASE!;
    const out = path.resolve(process.env.LC_BROWSER_EVIDENCE!, "product-editor-visual", phase);
    await mkdir(out, { recursive: true });
    const origin = process.env.LC_BROWSER_PUBLIC_ORIGIN!;
    const store = process.env.LC_BROWSER_STORE!;
    const url = (locale: string, suffix = "products") =>
      `${origin}/${locale}/${suffix}?store=${store}`;
    const measurements: unknown[] = [];
    page.on("dialog", (d) => void d.accept());
    await page.goto(url("en", ""));
    await page
      .getByRole("button", { name: "Sign in with identity service" })
      .click();
    await expect(page.getByTestId("nav-orders")).toBeAttached();
    async function openNew(locale: string) {
      await page.goto(url(locale));
      await page.getByTestId("product-new").click();
      await expect(page.getByTestId("product-name")).toBeVisible();
    }
    async function threeVariants() {
      await page.getByTestId("axis-add").click();
      await page.getByTestId("axis-name-0").fill("Color");
      await page.getByTestId("axis-values-0").fill("White, Black, Blue");
      await page.getByTestId("axis-values-0").press("Enter");
      await expect(page.locator('[data-testid^="matrix-row-"]')).toHaveCount(3);
      for (let i = 0; i < 3; i++) {
        await page.getByTestId(`new-price-${i}`).fill("80");
        await page.getByTestId(`matrix-quantity-${i}`).fill("5");
      }
    }
    const existing: string[] = [];
    // Independent prefix keeps the frozen PE postflight counts unchanged.
    for (const variants of [false, true]) {
      await openNew("en");
      await page
        .getByTestId("product-name")
        .fill(`visual-${process.env.LC_BROWSER_TAG}-${variants}`);
      if (variants) await threeVariants();
      else {
        await page.getByTestId("product-price").fill("80");
        await page.getByTestId("product-quantity").fill("5");
      }
      await page.getByTestId("product-create").click();
      const result = page.getByTestId("product-save-result");
      await expect(result).toBeVisible();
      await result.locator('a[href*="/products/"]').click();
      await expect(page.getByTestId("product-save")).toBeEnabled();
      existing.push(/products\/([0-9a-f-]{36})/.exec(page.url())![1]);
    }
    for (const locale of ["zh-TW", "zh-CN", "en"] as const) {
      for (const width of [1586, 390]) {
        await page.setViewportSize({
          width,
          height: width === 390 ? 844 : 992,
        });
        for (const mode of ["new", "existing"] as const) {
          for (const variants of [false, true]) {
            if (mode === "new") {
              await openNew(locale);
              if (variants) await threeVariants();
            } else {
              await page.goto(
                url(locale, `products/${existing[variants ? 1 : 0]}`),
              );
              await expect(page.getByTestId("product-save")).toBeEnabled();
            }
            if (variants)
              await page
                .getByTestId("matrix-select-0")
                .scrollIntoViewIfNeeded();
            await page
              .locator("header[data-shell-topbar]")
              .scrollIntoViewIfNeeded();
            await expect.poll(() => page.evaluate(() => scrollY)).toBe(0);
            const file = `${mode}-${variants ? "three" : "empty"}-${locale}-${width}.png`;
            await page.screenshot({
              path: path.join(out, file),
              fullPage: true,
              animations: "disabled",
            });
            // READ/MEASURE only: inspect actual rendered geometry/styles, never modify DOM.
            const metrics = await page.evaluate(() => {
              const rect = (s: string) => {
                const el = document.querySelector(s);
                if (!el) return null;
                const b = el.getBoundingClientRect(),
                  st = getComputedStyle(el);
                return {
                  x: b.x,
                  y: b.y,
                  width: b.width,
                  height: b.height,
                  font: st.fontSize,
                  accent: st.accentColor,
                };
              };
              return {
                overflow: document.documentElement.scrollWidth - innerWidth,
                name: rect('[data-testid="product-name"]'),
                axis: rect('[data-testid="axis-name-0"]'),
                values: rect('[data-testid="axis-values-0"]'),
                remove: rect(".product-axis > button"),
                quantity: rect('[data-testid="matrix-quantity-0"]'),
                stock: rect(".pe-row-stock"),
                row: rect('[data-testid="matrix-row-0"]'),
                helpers: [
                  ...document.querySelectorAll(
                    ".pe-document small,.pe-document .pe-hint",
                  ),
                ].map((el) => ({
                  text: el.textContent,
                  font: getComputedStyle(el).fontSize,
                })),
                nav: [...document.querySelectorAll(".pe-index nav button")].map(
                  (el) => el.textContent,
                ),
                readiness: [...document.querySelectorAll(".pe-readiness")].map(
                  (el) => el.textContent,
                ),
                bulk: [
                  ...document.querySelectorAll(".pe-bulk-controls button"),
                ].map((el) => el.textContent),
                firstVariant: document.querySelector(".pe-matrix-row strong")
                  ?.textContent,
                active: rect('[data-testid="matrix-active-0"]'),
              };
            });
            measurements.push({ file, mode, variants, locale, width, metrics });
            await writeFile(
              path.join(out, "measurements.json"),
              JSON.stringify(measurements, null, 2),
            );
            expect(metrics.overflow).toBeLessThanOrEqual(1);
            if (phase === "after") {
              expect(
                metrics.helpers.every((h) => parseFloat(h.font) >= 12),
              ).toBe(true);
              expect(metrics.name!.width).toBeLessThanOrEqual(680);
              expect(
                metrics.readiness.filter((s) =>
                  s?.includes(productEditorCopy[locale].images)
                    && !s.includes(productEditorCopy[locale].recommendedImages),
                ),
              ).toHaveLength(1);
              // Integrator restored the distinct >=3-image advice: exactly one
              // required image item and one recommendation, never duplicates.
              expect(
                metrics.readiness.filter((s) =>
                  s?.includes(productEditorCopy[locale].recommendedImages),
                ),
              ).toHaveLength(1);
              if (variants) {
                expect(metrics.nav).not.toContain(
                  productEditorCopy[locale].pricing,
                );
                expect(metrics.bulk).toEqual([productEditorCopy[locale].bulk]);
                expect(metrics.firstVariant).toBe("White");
                if (width === 1586) {
                  expect(
                    Math.abs(metrics.axis!.y - metrics.values!.y),
                  ).toBeLessThanOrEqual(1);
                  expect(metrics.axis!.height).toBe(metrics.values!.height);
                  expect(metrics.remove!.y).toBe(metrics.axis!.y);
                  expect(metrics.stock!.height).toBe(44);
                  expect(metrics.row!.height).toBeLessThanOrEqual(70);
                }
              }
              for (let i = 0; i < metrics.nav.length; i++) {
                const button = page.locator(".pe-index nav button").nth(i);
                await button.click();
                await expect(button).toHaveAttribute(
                  "aria-current",
                  "location",
                );
              }
            }
          }
        }
      }
    }
    expect(measurements).toHaveLength(24);
  });
}
