// Purpose: Reads populated product-table geometry and asserts layout without replacing merchant interactions.
// Depends on: @playwright/test
// Used by: tests/admin/product-editor.acceptance.ts
// ADM23 additive geometry checks for the real catalog browser fixture.
// Call after navigating/clicking to a populated product list at desktop/mobile widths;
// evaluate only reads layout and never replaces merchant clicks or persisted readback.
import { expect, type Page } from "@playwright/test";

/** Reads browser geometry and asserts table layout without mutating products or styles. */
export async function assertProductListLayout(page: Page) {
  const table = page.getByTestId("products-table");
  await expect(table).toBeVisible();
  const rows = await table.locator("tbody > tr").evaluateAll((nodes) => {
    const rect = (element: Element) => {
      const r = element.getBoundingClientRect();
      return {
        x: r.x,
        y: r.y,
        right: r.right,
        bottom: r.bottom,
        width: r.width,
        height: r.height,
      };
    };
    return nodes.map((row) => {
      const get = (selector: string) => rect(row.querySelector(selector)!);
      return {
        selectionCell: get(".pe-selection-cell"),
        selection: get(".pe-select"),
        product: get(".product-cell"),
        identity: get(".pe-product-identity"),
        status: get(".product-status"),
        statusCell: get(".pe-status-cell"),
        price: get("[data-testid=quick-price]"),
        stock: get("[data-testid=quick-stock]"),
        priceValue: get(".pe-price-cell .pe-number-edit > span"),
        stockValue: get(".pe-stock-cell .pe-number-edit > span"),
      };
    });
  });
  expect(
    rows.length,
    "populated fixture, never a vacuous layout pass",
  ).toBeGreaterThan(0);
  const mobile = (page.viewportSize()?.width ?? 0) <= 680;
  for (const row of rows) {
    for (const target of [row.selection, row.price, row.stock]) {
      expect(target.width, "44px click target").toBeGreaterThanOrEqual(44);
      expect(target.height, "44px click target").toBeGreaterThanOrEqual(44);
    }
    expect(row.selection.x).toBeGreaterThanOrEqual(row.selectionCell.x - 1);
    expect(row.selection.right).toBeLessThanOrEqual(
      row.selectionCell.right + 1,
    );
    expect(row.status.x).toBeGreaterThanOrEqual(row.statusCell.x - 1);
    expect(row.status.right).toBeLessThanOrEqual(row.statusCell.right + 1);
    expect(row.priceValue.right).toBeLessThanOrEqual(row.price.x);
    expect(row.stockValue.right).toBeLessThanOrEqual(row.stock.x);
    if (mobile) {
      expect(row.selection.bottom).toBeGreaterThan(row.product.y);
      expect(row.selection.y).toBeLessThan(row.product.bottom);
      expect(row.product.x).toBeGreaterThanOrEqual(row.selection.right);
      expect(row.status.y).toBeGreaterThanOrEqual(row.product.bottom - 1);
    } else {
      expect(row.identity.right).toBeLessThanOrEqual(row.status.x);
      expect(Math.abs(row.price.x - rows[0].price.x)).toBeLessThanOrEqual(1);
      expect(Math.abs(row.stock.x - rows[0].stock.x)).toBeLessThanOrEqual(1);
    }
  }
  if (mobile) {
    // READ/MEASURE: page containment, not DOM mutation or a write substitute.
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth - innerWidth,
      ),
    ).toBeLessThanOrEqual(1);
  }
}
