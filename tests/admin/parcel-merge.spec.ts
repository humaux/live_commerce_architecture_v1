// Purpose: W3-07B parcel-merge real-click acceptance on the merchant orders page: banner (masked recipient; COD/CVS orders of the
//   same buyer never suggested) -> merge -> group waybill -> every member 已出貨 (persisted across reload); an OPEN group's panel
//   REAPPEARS after a reload and can be dissolved or shipped (W3-U4 GET parcel-groups); an uncertain dissolve that landed is
//   reconciled; single-order shipment blocked; a stale second tab gets the server 409 in_parcel_group copy. Every assertion is a
//   user-visible result of a real click/fill/selectOption; page.evaluate is used once for a [READ/MEASURE] layout read only and
//   page.route once as [FAULT INJECTION] (the click's own request is failed after it reached the server).
//   Fixture orders come from the Go harness (LC_BROWSER_PARCEL_ORDERS).
// Depends on: @playwright/test; harness env LC_BROWSER_PUBLIC_ORIGIN, LC_BROWSER_ORDER_STORE, LC_BROWSER_PARCEL_ORDERS.
// Used by: scripts/dev/test-local.sh (--browser-merchant-orders-ui), tests/foundation/browser_merchant_orders_ui_test.go.
// Invariants: never imports helpers from orders-ui.spec.ts (module-level test() would re-register); never calls the
//   API directly to mutate; the banner copy must state that only parcels merge, never payments/amounts.
import { expect, test, type Page } from "@playwright/test";

const required = (name: string) => {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
};
const origin = required("LC_BROWSER_PUBLIC_ORIGIN");
const store = required("LC_BROWSER_ORDER_STORE");
// Go harness fixture: three buyer pairs (home delivery, captured, READY) plus two orders of the SHIP pair's buyer that must never
// be suggested (excluded: a COD order and a CVS order). ship -> merged + shipped; dissolve -> merged, reload, dissolved, an
// uncertain dissolve reconciled, re-merged, reload, shipped from the reloaded panel; stale -> merged in a second tab while the
// first tab still shows the single-order form (the server answers 409 in_parcel_group).
const parcelOrders = JSON.parse(required("LC_BROWSER_PARCEL_ORDERS")) as {
  ship: [string, string];
  dissolve: [string, string];
  stale: [string, string];
  excluded: [string, string];
};
const [pm1, pm2] = parcelOrders.ship;
const [pd1, pd2] = parcelOrders.dissolve;
const [ps1] = parcelOrders.stale;

test.use({ baseURL: origin, headless: false, trace: "retain-on-failure", screenshot: "only-on-failure" });

// Minimal duplicates of orders-ui.spec helpers (importing that module would re-register its tests).
async function signedLogin(page: Page) {
  await page.goto(new URL("/en/", origin).toString());
  await page.getByRole("button", { name: "Sign in with identity service" }).click();
  await expect(page.getByTestId("nav-orders")).toBeVisible();
  await page.getByTestId("nav-orders").click();
  await expect(page.getByTestId("merchant-orders")).toBeVisible();
  const selector = page.getByTestId("shell-store-selector");
  if ((await selector.inputValue()) !== store) {
    await selector.selectOption(store);
    await expect(page).toHaveURL(new URL(`/en?store=${store}`, origin).href);
    await expect(page.getByTestId("nav-orders")).toBeVisible();
    await page.getByTestId("nav-orders").click();
    await expect(page).toHaveURL(new RegExp(`/en/orders\\?store=${store}$`));
  }
  await expect(page.getByTestId("orders-table")).toBeVisible();
  const toggle = page.getByTestId("orders-more-filters");
  if ((await toggle.getAttribute("aria-expanded")) === "false") await toggle.click();
  await page.getByTestId("state-filter").selectOption("all");
  await expect(page.getByTestId("orders-table")).toBeVisible();
}

async function expand(page: Page, id: string) {
  await expect(page.getByTestId("orders-table")).toBeVisible();
  for (let p = 0; p < 3; p++) {
    if (await page.getByTestId(`order-expand-${id}`).count()) {
      const button = page.getByTestId(`order-expand-${id}`);
      if ((await button.getAttribute("aria-expanded")) !== "true") await button.click();
      await expect(page.getByTestId("order-detail")).toHaveAttribute("aria-label", new RegExp(id));
      return;
    }
    await expect(page.getByTestId("orders-next")).toBeEnabled();
    const previousURL = page.url();
    await page.getByTestId("orders-next").click();
    await expect(page).not.toHaveURL(previousURL);
  }
  throw new Error("fixture order was absent from three real cursor pages");
}

const suggestionCard = (page: Page, member: string) =>
  page.getByTestId("parcel-suggestion").filter({ has: page.getByTestId(`parcel-member-${member}`) });
const groupPanel = (page: Page, member: string) =>
  page.locator('section[data-testid^="parcel-group-"]').filter({ has: page.getByTestId(`parcel-group-member-${member}`) });

test("W3-07B parcel merge: suggest -> merge -> group waybill -> members shipped -> dissolve -> block -> 409", async ({ page }) => {
  await signedLogin(page);

  await test.step("banner shows the three groups, masked recipient, parcels-only ruling; COD and CVS orders are never suggested", async () => {
    await expect(page.getByTestId("parcel-merge")).toBeVisible();
    // Three eligible pairs only: the COD and the CVS order of the ship pair's buyer (same owner and address) are excluded.
    await expect(page.getByTestId("parcel-merge-count")).toHaveText("3 order groups can ship as one parcel");
    await expect(page.getByTestId("parcel-merge-rule")).toContainText("Only the parcels are merged, never payments or amounts");
    await page.getByTestId("parcel-suggestions-toggle").click();
    await expect(page.getByTestId("parcel-suggestion")).toHaveCount(3);
    await expect(suggestionCard(page, pm1)).toContainText("2 orders");
    await expect(suggestionCard(page, pm1).locator('[data-testid^="parcel-member-"]')).toHaveCount(2);
    for (const excluded of parcelOrders.excluded) await expect(page.getByTestId(`parcel-member-${excluded}`)).toHaveCount(0);
    // Ruling: the banner shows the MASKED recipient like the list row, never the full name.
    await expect(suggestionCard(page, pm1)).toContainText("S***");
    await expect(page.getByTestId("parcel-merge")).not.toContainText("Synthetic Buyer");
    // 390px: [READ/MEASURE] layout read only, no state change.
    await page.setViewportSize({ width: 390, height: 844 });
    expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1);
    await page.setViewportSize({ width: 1280, height: 800 });
  });

  await test.step("merge the ship pair; panel + row badges appear", async () => {
    await suggestionCard(page, pm1).locator('button[data-testid^="parcel-merge-"]').click();
    const panel = groupPanel(page, pm1);
    await expect(panel).toBeVisible();
    await expect(panel.getByTestId(`parcel-group-member-${pm2}`)).toBeVisible();
    await expect(panel.locator('[data-testid^="parcel-state-"]')).toHaveText("Open");
    await expect(page.getByTestId("parcel-merge-count")).toHaveText("2 order groups can ship as one parcel");
    // The server's OPEN-groups read fills in the order number and the masked recipient.
    await expect(panel.getByTestId(`parcel-group-member-${pm1}`)).toContainText(`LC-${pm1.replaceAll("-", "").toUpperCase()} · S***`);
  });

  await test.step("fill the group waybill; both members ship", async () => {
    const panel = groupPanel(page, pm1);
    await panel.locator('[data-testid^="parcel-ship-open-"]').click();
    await panel.locator('[data-testid="parcel-ship-carrier"]').selectOption("black_cat");
    await panel.locator('[data-testid="parcel-ship-tracking"]').fill("PARCEL1234567890");
    await panel.locator('[data-testid^="parcel-ship-submit-"]').click();
    await expect(panel.locator('[data-testid^="parcel-notice-"]')).toContainText("group is shipped");
    await expect(panel.locator('[data-testid^="parcel-state-"]')).toHaveText("Shipped");
    for (const id of [pm1, pm2]) {
      await expand(page, id);
      await expect(page.getByTestId(`parcel-badge-${id}`)).toBeVisible();
      const detail = page.getByTestId("order-detail");
      await expect(detail.locator('[data-state="MERCHANT_SHIPPED"]').first()).toBeVisible();
      await expect(detail.getByTestId("shipment-record")).toContainText("PARCEL1234567890");
    }
  });

  await test.step("reload keeps the shipments; the SHIPPED group is history, so no OPEN panel is rebuilt", async () => {
    await page.reload();
    await expect(page.getByTestId("orders-table")).toBeVisible();
    await expect(page.locator('section[data-testid^="parcel-group-"]')).toHaveCount(0);
    await expand(page, pm1);
    await expect(page.getByTestId("order-detail").getByTestId("shipment-record")).toContainText("PARCEL1234567890");
  });

  await test.step("merge the dissolve pair; after a RELOAD the OPEN panel reappears and dissolves with the confirm pair", async () => {
    await page.getByTestId("parcel-suggestions-toggle").click();
    await suggestionCard(page, pd1).locator('button[data-testid^="parcel-merge-"]').click();
    await expect(groupPanel(page, pd1)).toBeVisible();
    await page.reload();
    await expect(page.getByTestId("orders-table")).toBeVisible();
    const panel = groupPanel(page, pd1); // rebuilt from GET parcel-groups: no session knowledge involved
    await expect(panel).toBeVisible();
    await expect(panel.locator('[data-testid^="parcel-state-"]')).toHaveText("Open");
    await expect(panel.getByTestId(`parcel-group-member-${pd2}`)).toContainText(`LC-${pd2.replaceAll("-", "").toUpperCase()} · S***`);
    // Back out of the confirmation first: the confirmation closes, nothing was sent, the group is still Open with its actions.
    await panel.locator('[data-testid^="parcel-dissolve-"]').first().click();
    await expect(panel.locator('[data-testid^="parcel-dissolve-confirm-"]')).toBeVisible();
    await panel.locator('[data-testid^="parcel-dissolve-back-"]').click();
    await expect(panel.locator('[data-testid^="parcel-dissolve-confirm-"]')).toHaveCount(0);
    await expect(panel.locator('[data-testid^="parcel-dissolve-back-"]')).toHaveCount(0);
    await expect(panel.locator('[data-testid^="parcel-state-"]')).toHaveText("Open");
    await expect(panel.locator('[data-testid^="parcel-ship-open-"]')).toBeEnabled();
    await expect(panel.locator('[data-testid^="parcel-dissolve-"]').first()).toBeEnabled();
    // Then dissolve for real.
    await panel.locator('[data-testid^="parcel-dissolve-"]').first().click();
    await panel.locator('[data-testid^="parcel-dissolve-confirm-"]').click();
    await expect(panel.locator('[data-testid^="parcel-notice-"]')).toContainText("Group dissolved");
    await panel.locator('[data-testid^="parcel-close-"]').click();
    await expect(groupPanel(page, pd1)).toHaveCount(0);
  });

  await test.step("an uncertain dissolve that landed is reconciled: the retry's group_not_open drops the panel instead of leaving it OPEN", async () => {
    await page.getByTestId("parcel-suggestions-toggle").click();
    await suggestionCard(page, pd1).locator('button[data-testid^="parcel-merge-"]').click();
    const panel = groupPanel(page, pd1);
    await expect(panel).toBeVisible();
    // [FAULT INJECTION, not a substitute for a click] The real confirm click below sends its own DELETE; page.route only lets that
    // request reach the server (the group really dissolves) and then fails it in the browser: outcome UNKNOWN. No state is set here.
    let first = true;
    const dissolveRequest = /\/parcel-groups\/[0-9a-f-]{36}\?expected_version=\d+$/;
    await page.route(dissolveRequest, async (route) => {
      if (route.request().method() === "DELETE" && first) {
        first = false;
        await route.fetch();
        await route.abort("failed");
      } else await route.continue();
    });
    await panel.locator('[data-testid^="parcel-dissolve-"]').first().click();
    await panel.locator('[data-testid^="parcel-dissolve-confirm-"]').click();
    await expect(panel.locator('[data-testid^="parcel-problem-"]')).toHaveText("Temporarily unavailable. Try again.");
    await expect(panel.locator('[data-testid^="parcel-state-"]')).toHaveText("Open");
    // Same-version retry: the server answers group_not_open; the UI refetches the OPEN groups and the panel goes away.
    await panel.locator('[data-testid^="parcel-dissolve-confirm-"]').click();
    await expect(groupPanel(page, pd1)).toHaveCount(0);
    await expect(page.getByTestId("parcel-merge-problem")).toContainText("already been shipped or dissolved");
    await page.unroute(dissolveRequest);
  });

  await test.step("re-merge; the single-order form is replaced by the group hint, also after a reload; ship from the reloaded panel", async () => {
    await expect(suggestionCard(page, pd1)).toBeVisible();
    await suggestionCard(page, pd1).locator('button[data-testid^="parcel-merge-"]').click();
    await expect(groupPanel(page, pd1)).toBeVisible();
    await expand(page, pd1);
    const hint = "This order is in a parcel group; fill in the waybill on the group.";
    await expect(page.getByTestId("order-detail").getByTestId("shipment-parcel-block")).toHaveText(hint);
    await expect(page.getByTestId("order-detail").getByTestId("shipment-form")).toHaveCount(0);
    await page.reload();
    await expect(page.getByTestId("orders-table")).toBeVisible();
    const panel = groupPanel(page, pd1);
    await expect(panel).toBeVisible();
    await expand(page, pd1);
    await expect(page.getByTestId(`parcel-badge-${pd1}`)).toBeVisible();
    await expect(page.getByTestId("order-detail").getByTestId("shipment-parcel-block")).toHaveText(hint);
    await panel.locator('[data-testid^="parcel-ship-open-"]').click();
    await panel.locator('[data-testid="parcel-ship-carrier"]').selectOption("black_cat");
    await panel.locator('[data-testid="parcel-ship-tracking"]').fill("RELOAD1234567890");
    await panel.locator('[data-testid^="parcel-ship-submit-"]').click();
    await expect(panel.locator('[data-testid^="parcel-notice-"]')).toContainText("group is shipped");
    for (const id of [pd1, pd2]) {
      await expand(page, id);
      await expect(page.getByTestId("order-detail").getByTestId("shipment-record")).toContainText("RELOAD1234567890");
    }
  });

  await test.step("a stale second tab: the single-order submit gets the server in_parcel_group copy", async () => {
    // Tab A shows the stale-pair order with the single-order form; tab B merges that pair; A (never told) then submits.
    await page.reload();
    await expect(page.getByTestId("orders-table")).toBeVisible();
    await expand(page, ps1);
    const detail = page.getByTestId("order-detail");
    await expect(detail.getByTestId("shipment-form")).toBeVisible();
    const tabB = await page.context().newPage();
    await tabB.goto(new URL(`/en/orders?store=${store}`, origin).toString());
    await expect(tabB.getByTestId("parcel-merge")).toBeVisible();
    await tabB.getByTestId("parcel-suggestions-toggle").click();
    await suggestionCard(tabB, ps1).locator('button[data-testid^="parcel-merge-"]').click();
    await expect(groupPanel(tabB, ps1)).toBeVisible();
    await tabB.close();
    await detail.getByTestId("ship-carrier").selectOption("black_cat");
    await detail.getByTestId("ship-tracking").fill("STALE1234567890AB");
    await detail.getByTestId("shipment-submit").click();
    // Server 409 in_parcel_group maps to the same sentence as the client hint (orders-copy errors.in_parcel_group).
    await expect(detail.getByTestId("shipment-problem")).toHaveText("This order is in a parcel group; fill in the waybill on the group.");
    await expect(detail.getByTestId("shipment-record")).toHaveCount(0);
  });
});
