// Purpose: W3-07B parcel-merge real-click acceptance on the merchant orders page: banner -> merge -> group waybill ->
//   every member 已出貨 (persisted across reload), dissolve -> re-merge -> single-order shipment blocked -> server 409
//   in_parcel_group copy. Every assertion is a user-visible result of a real click/fill/selectOption; page.evaluate is
//   used once for a [READ/MEASURE] layout read only. Fixture orders come from the Go harness (LC_BROWSER_PARCEL_ORDERS).
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
// Go harness fixture: two buyer pairs, home delivery, captured and READY. ship -> merged + shipped; dissolve ->
// merged, dissolved, re-merged (the last group stays OPEN so the stale single-order submit hits in_parcel_group).
const parcelOrders = JSON.parse(required("LC_BROWSER_PARCEL_ORDERS")) as {
  ship: [string, string];
  dissolve: [string, string];
};
const [pm1, pm2] = parcelOrders.ship;
const [pd1] = parcelOrders.dissolve;

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

  await test.step("banner shows both groups with the parcels-only ruling", async () => {
    await expect(page.getByTestId("parcel-merge")).toBeVisible();
    await expect(page.getByTestId("parcel-merge-count")).toHaveText("2 order groups can ship as one parcel");
    await expect(page.getByTestId("parcel-merge-rule")).toContainText("Only the parcels are merged, never payments or amounts");
    await page.getByTestId("parcel-suggestions-toggle").click();
    await expect(page.getByTestId("parcel-suggestion")).toHaveCount(2);
    await expect(suggestionCard(page, pm1)).toContainText("2 orders");
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
    await expect(page.getByTestId("parcel-merge-count")).toHaveText("1 order group can ship as one parcel");
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

  await test.step("reload keeps the shipments; session group panels are gone", async () => {
    await page.reload();
    await expect(page.getByTestId("orders-table")).toBeVisible();
    await expect(page.locator('section[data-testid^="parcel-group-"]')).toHaveCount(0);
    await expand(page, pm1);
    await expect(page.getByTestId("order-detail").getByTestId("shipment-record")).toContainText("PARCEL1234567890");
  });

  await test.step("merge the dissolve pair, then dissolve it with the confirm pair", async () => {
    await page.getByTestId("parcel-suggestions-toggle").click();
    await suggestionCard(page, pd1).locator('button[data-testid^="parcel-merge-"]').click();
    const panel = groupPanel(page, pd1);
    await expect(panel).toBeVisible();
    await panel.locator('[data-testid^="parcel-dissolve-"]').first().click();
    await panel.locator('[data-testid^="parcel-dissolve-confirm-"]').click();
    await expect(panel.locator('[data-testid^="parcel-notice-"]')).toContainText("Group dissolved");
    await panel.locator('[data-testid^="parcel-close-"]').click();
    await expect(groupPanel(page, pd1)).toHaveCount(0);
  });

  await test.step("re-merge; the single-order shipment form is replaced by the group hint", async () => {
    await expect(suggestionCard(page, pd1)).toBeVisible();
    await suggestionCard(page, pd1).locator('button[data-testid^="parcel-merge-"]').click();
    await expect(groupPanel(page, pd1)).toBeVisible();
    await expand(page, pd1);
    const detail = page.getByTestId("order-detail");
    await expect(detail.getByTestId("shipment-parcel-block")).toHaveText("This order is in a parcel group; fill in the waybill on the group.");
    await expect(detail.getByTestId("shipment-form")).toHaveCount(0);
  });

  await test.step("a stale single-order submit after reload gets the server in_parcel_group copy", async () => {
    await page.reload();
    await expect(page.getByTestId("orders-table")).toBeVisible();
    await expand(page, pd1);
    const detail = page.getByTestId("order-detail");
    await expect(detail.getByTestId("shipment-form")).toBeVisible();
    await detail.getByTestId("ship-carrier").selectOption("black_cat");
    await detail.getByTestId("ship-tracking").fill("STALE1234567890AB");
    await detail.getByTestId("shipment-submit").click();
    // Server 409 in_parcel_group maps to the same sentence as the client hint (orders-copy errors.in_parcel_group).
    await expect(detail.getByTestId("shipment-problem")).toHaveText("This order is in a parcel group; fill in the waybill on the group.");
    await expect(detail.getByTestId("shipment-record")).toHaveCount(0);
  });
});
