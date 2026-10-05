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
  // Diagnostic companion, NOT a replacement for frozen R6/R9. Read the actual
  // clip and native hit-test result; retain any frozen raw-rectangle failure.
  const clippedBounds = await form.evaluate((element) => {
    const nav = element.querySelector(".pe-index nav")!;
    const fields = element.querySelector(".pe-fields")!;
    const bar = element.querySelector(".pe-savebar")!.getBoundingClientRect();
    const n = nav.getBoundingClientRect(), p = fields.getBoundingClientRect();
    const navTargets = [...nav.querySelectorAll("button")].map((button) => {
      const r = button.getBoundingClientRect();
      return { label: button.textContent, rawLeft: r.left, rawRight: r.right,
        visibleLeft: Math.max(n.left, r.left), visibleRight: Math.min(n.right, r.right) };
    }).filter((r) => r.visibleRight > r.visibleLeft);
    const rawTails = [...fields.querySelectorAll("button,input,select,textarea")].flatMap((control) => {
      const r = control.getBoundingClientRect();
      const top = Math.max(r.top, bar.top), bottom = Math.min(r.bottom, bar.bottom);
      if (r.top >= p.bottom || top >= bottom || r.width === 0) return [];
      const hit = document.elementFromPoint(Math.max(p.left, r.left) + 1, (top + bottom) / 2);
      return [{ id: control.id || control.getAttribute("data-testid"), rawBottom: r.bottom,
        paneBottom: p.bottom, barTop: bar.top, clippedTailHittable: !!hit && control.contains(hit) }];
    });
    return { locale: document.documentElement.lang, width: innerWidth,
      navLeft: n.left, navRight: n.right, navTargets, rawTails };
  });
  for (const tail of clippedBounds.rawTails) expect(tail.clippedTailHittable).toBe(false);
  if (clippedBounds.width <= 390) for (const target of clippedBounds.navTargets) {
    expect(target.visibleLeft).toBeGreaterThanOrEqual(8);
    expect(target.visibleRight).toBeLessThanOrEqual(clippedBounds.width - 8);
  }
  console.log("EDITOR_VISIBLE_CLIP_DIAGNOSTIC", JSON.stringify(clippedBounds));
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
