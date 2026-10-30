// Purpose: Check signed product rows' real geometry without replacing merchant interactions.
// Depends on: @playwright/test.
// Used by: product-editor.acceptance.ts.
// Invariants: read-only measurement; nonvacuous rows; 44px actual targets and no clipped facts.
import { expect, type Page } from '@playwright/test';
/** Quick-pencil geometry is superseded by the sole Edit target and read-only amount/stock containment. */
export async function assertProductListLayout(page: Page) {
  const table = page.getByTestId('products-table');
  await expect(table).toBeVisible();
  const rows = await table.locator('tbody > tr').evaluateAll(nodes => {
    const rect = (element: Element) => { const r = element.getBoundingClientRect(); return { x: r.x, y: r.y, right: r.right, bottom: r.bottom, width: r.width, height: r.height }; };
    return nodes.map(row => {
      const get = (selector: string) => rect(row.querySelector(selector)!);
      return { selectionCell: get('.pe-selection-cell'), selection: get('.pe-select'), product: get('.product-cell'),
        identity: get('.pe-product-identity'), status: get('.product-status'), statusCell: get('.pe-status-cell'),
        amount: get('.pe-price-cell > span'), amountCell: get('.pe-price-cell'), edit: get('.pe-row-actions a'), editCell: get('.pe-actions-cell') };
    });
  });
  expect(rows.length, 'populated fixture, never vacuous').toBeGreaterThan(0);
  const mobile = (page.viewportSize()?.width ?? 0) <= 680;
  for (const row of rows) {
    for (const target of [row.selection, row.edit]) {
      expect(target.width, '44px actual click target').toBeGreaterThanOrEqual(44);
      expect(target.height, '44px actual click target').toBeGreaterThanOrEqual(44);
    }
    expect(row.selection.x).toBeGreaterThanOrEqual(row.selectionCell.x - 1);
    expect(row.selection.right).toBeLessThanOrEqual(row.selectionCell.right + 1);
    expect(row.status.x).toBeGreaterThanOrEqual(row.statusCell.x - 1);
    expect(row.status.right).toBeLessThanOrEqual(row.statusCell.right + 1);
    expect(row.amount.x).toBeGreaterThanOrEqual(row.amountCell.x - 1);
    expect(row.amount.right).toBeLessThanOrEqual(row.amountCell.right + 1);
    expect(row.edit.x).toBeGreaterThanOrEqual(row.editCell.x - 1);
    expect(row.edit.right).toBeLessThanOrEqual(row.editCell.right + 1);
    if (mobile) {
      expect(row.selection.bottom).toBeGreaterThan(row.product.y);
      expect(row.selection.y).toBeLessThan(row.product.bottom);
      expect(row.product.x).toBeGreaterThanOrEqual(row.selection.right);
      expect(row.status.y).toBeGreaterThanOrEqual(row.product.bottom - 1);
    } else {
      expect(row.identity.right).toBeLessThanOrEqual(row.status.x);
      expect(Math.abs(row.edit.x - rows[0].edit.x)).toBeLessThanOrEqual(1);
      expect(Math.abs(row.amountCell.x - rows[0].amountCell.x)).toBeLessThanOrEqual(1);
    }
  }
  if (mobile) expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1);
}
