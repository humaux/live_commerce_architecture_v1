// Purpose: Verify real editor navigation, visible field focus and reserved savebar geometry without synthetic DOM changes.
// Depends on: Playwright Page/expect and the production ProductDocumentForm markup; caller-owned browser/PG fixture.
// Used by: product-editor.acceptance.ts under --browser-product-editor; reads geometry and performs real nav clicks.
import { expect, type Page } from "@playwright/test";

// Runtime acceptance, not a style/threshold test. evaluate only reads geometry
// and focus; section opening and scrolling are driven by actual nav clicks.
// Nested scroll panes require a viewport capture per section: a full-page image
// cannot reveal content outside the pane's current scroll position.
/** Clicks every available section and checks that focus remains inside its visible scroll pane. */
export async function assertProductEditorReservedLayout(
  page: Page,
  capture?: (section: string) => Promise<void>,
) {
  const form = page.locator("form.pe-document");
  const pane = form.getByTestId("product-fields");
  const footer = form.locator(".pe-savebar");
  const buttons = form.locator(".pe-index nav button[aria-controls]");
  await expect(pane).toBeVisible();
  expect(await buttons.count()).toBeGreaterThanOrEqual(6);
  const assertReserved = async () => {
    await expect
      .poll(
        async () =>
          pane.evaluate((element) => {
            const save = element.closest("form")!.querySelector(".pe-savebar")!;
            const area = element.getBoundingClientRect();
            const bar = save.getBoundingClientRect();
            const viewport = window.visualViewport;
            const top = viewport?.offsetTop ?? 0;
            const bottom = top + (viewport?.height ?? innerHeight);
            return (
              element.clientHeight > 0 &&
              element.scrollHeight > element.clientHeight &&
              area.bottom <= bar.top &&
              bar.top >= top &&
              bar.bottom <= bottom + 1
            );
          }),
        "scrollable fields and visible savebar occupy separate reserved rows",
      )
      .toBe(true);
    await expect(footer.getByTestId("product-publish")).toBeVisible();
    const viewport = page.viewportSize()!;
    if (viewport.width <= 900) {
      const nav = await form.locator(".pe-index").boundingBox();
      const area = await pane.boundingBox();
      expect(nav!.y + nav!.height).toBeLessThanOrEqual(area!.y);
    }
  };
  await assertReserved();
  // Iterate the controls that actually exist (single-SKU has a pricing section,
  // matrix mode does not). No fixture DOM mutation or synthetic event dispatch.
  const ids = await buttons.evaluateAll((nodes) =>
    nodes.map((n) => n.getAttribute("aria-controls")!),
  );
  for (const id of ids) {
    const button = form.locator(`.pe-index nav button[aria-controls="${id}"]`);
    await button.click();
    await expect(button).toHaveAttribute("aria-current", "location");
    const section = pane.locator(`[id="${id}"]`);
    await expect
      .poll(
        () =>
          section.evaluate((element) => {
            const root = element.closest(".pe-fields")!;
            const area = root.getBoundingClientRect();
            const target = element.getBoundingClientRect();
            const focused = document.activeElement;
            if (!(focused instanceof HTMLElement) || !element.contains(focused))
              return false;
            const field = focused.getBoundingClientRect();
            return (
              target.bottom > area.top &&
              target.top < area.bottom &&
              field.top >= area.top &&
              field.bottom <= area.bottom
            );
          }),
        `${id} actual focused control is inside the scroll pane, not under footer`,
      )
      .toBe(true);
    await assertReserved();
    if (capture) await capture(id);
  }
}
