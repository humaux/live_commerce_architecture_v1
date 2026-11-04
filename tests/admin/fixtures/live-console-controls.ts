// Purpose: Reach signed console tabs and disclosures through the same real clicks in dependent browser suites.
// Depends on: Playwright locators and LiveConsole/CommentStream's existing tab/disclosure markup.
// Used by: live-console.spec.ts and live-settings.spec.ts; no fixture or response substitution.
// Invariants: disclosure identities stay stable as open state changes; never mutate DOM or bypass actionability.
import { expect, type Page } from "@playwright/test";

export async function showLivePanel(page: Page, panel: "live" | "comments" | "buyer") {
  // Desktop keeps the mobile tablist CSS-hidden; include it for the attachment check only.
  // The visibility guard still ensures that every tab interaction is a real, actionable click.
  const tabs = page.getByTestId("live-console").getByRole("tab", { includeHidden: true });
  await expect(tabs.first()).toBeAttached();
  if (await tabs.first().isVisible()) {
    const tab = tabs.nth(panel === "live" ? 0 : panel === "comments" ? 1 : 2);
    await tab.click();
    await expect(tab).toHaveAttribute("aria-selected", "true");
  }
}

export async function showLiveBuyerDetails(page: Page) {
  await showLivePanel(page, "buyer");
  const details = page.locator("[data-live-buyer-details]");
  await expect(details.locator(":scope > summary")).toBeVisible();
  if (await details.getAttribute("open") === null) await details.locator(":scope > summary").click();
  await expect(page.getByTestId("buyer-panel")).toBeVisible();
}

export async function showLiveTools(page: Page) {
  await showLivePanel(page, "live");
  // Index the stable disclosures, never :not([open]): clicking removes a match from that filtered list.
  const tools = page.locator("[data-live-tools]");
  // Primary actions are visible directly; only remaining secondary disclosures need opening.
  if (await tools.count() === 0) return;
  await expect(tools.first()).toBeAttached();
  for (let index = 0; index < await tools.count(); index++) {
    const details = tools.nth(index);
    if (await details.getAttribute("open") === null) await details.locator(":scope > summary").click();
    await expect(details).toHaveAttribute("open", "");
  }
}
