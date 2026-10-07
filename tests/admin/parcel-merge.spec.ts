// Purpose: W3-07B parcel-merge real-click acceptance on the merchant orders page: banner (masked recipient; COD/CVS orders of the
//   same buyer never suggested) -> merge -> group waybill -> every member 已出貨 (persisted across reload); an OPEN group's panel
//   REAPPEARS after a reload and can be dissolved or shipped (W3-U4 GET parcel-groups); an uncertain dissolve that landed is
//   reconciled; single-order shipment blocked; a stale second tab gets the server 409 in_parcel_group copy. Every assertion is a
//   user-visible result of a real click/fill/selectOption; page.evaluate is used once for a [READ/MEASURE] layout read only and
//   page.route as [FAULT INJECTION] only (the dissolve click's own request failed after it reached the server; one 503 per parcel read).
//   Both group waybills type EVERY control (carrier select incl. `other` + name, tracking, link, note), after refusals for a missing
//   `other` name and a non-https link; both parcel READ failures (suggestions, OPEN groups) show their error + Retry and recover through a
//   real Retry click (one labelled page.route 503 each); each member's persisted values are read back after a reload (record, history, correction form).
//   Fixture orders come from the Go harness (LC_BROWSER_PARCEL_ORDERS).
// Depends on: @playwright/test; harness env LC_BROWSER_PUBLIC_ORIGIN, LC_BROWSER_ORDER_STORE, LC_BROWSER_PARCEL_ORDERS.
// Used by: scripts/dev/test-local.sh (--browser-merchant-orders-ui), tests/foundation/browser_merchant_orders_ui_test.go.
// Invariants: never imports helpers from orders-ui.spec.ts (module-level test() would re-register); never calls the
//   API directly to mutate; the banner copy must state that only parcels merge, never payments/amounts.
import { expect, test, type Locator, type Page } from "@playwright/test";

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

// Group-waybill values for the two shipped groups (the Go harness asserts the same rows in PG, browser_merchant_orders_ui_test.go):
// the SHIP group uses the `other` branch (carrier name required), the DISSOLVE pair's re-merged group a known carrier with no name.
const shipWaybill = {
  code: "other", shown: "Synthetic Courier", name: "Synthetic Courier", tracking: "SYPARCEL1234567890",
  url: "https://track.example.com/t/SYPARCEL1234567890", note: "Group waybill: handle with care",
};
const knownWaybill = {
  code: "black_cat", shown: "Black Cat", name: "", tracking: "RELOAD1234567890",
  url: "https://track.example.org/r/RELOAD1234567890", note: "Reloaded panel waybill",
};
type Waybill = typeof shipWaybill;

// Fill EVERY group-waybill control with real input: carrier select, carrier name (typed only for `other`), tracking, link, note.
async function fillGroupWaybill(panel: Locator, w: Waybill) {
  await panel.locator('[data-testid="parcel-ship-carrier"]').selectOption(w.code);
  if (w.name) await panel.locator('[data-testid="parcel-ship-carrier-name"]').fill(w.name);
  await panel.locator('[data-testid="parcel-ship-tracking"]').fill(w.tracking);
  await panel.locator('[data-testid="parcel-ship-url"]').fill(w.url);
  await panel.locator('[data-testid="parcel-ship-note"]').fill(w.note);
}

// Read one member's persisted shipment back through the UI: the record (carrier label, tracking, link), the history (note) and the
// correction form, which is prefilled from the stored head (all five fields). Nothing is submitted.
async function expectPersistedWaybill(page: Page, id: string, w: Waybill) {
  await expand(page, id);
  const detail = page.getByTestId("order-detail");
  const record = detail.getByTestId("shipment-record");
  await expect(record).toContainText(w.shown);
  await expect(record.locator("dd.orders-mono")).toHaveText(w.tracking);
  await expect(record.getByRole("link")).toHaveAttribute("href", w.url);
  await expect(record.getByRole("link")).toHaveText(new URL(w.url).hostname);
  await expect(detail.getByTestId("shipment-history")).toContainText(`Note: ${w.note}`);
  await detail.getByTestId("shipment-correct").click();
  await expect(detail.getByTestId("ship-carrier")).toHaveValue(w.code);
  await expect(detail.getByTestId("ship-carrier-name")).toHaveValue(w.name);
  await expect(detail.getByTestId("ship-tracking")).toHaveValue(w.tracking);
  await expect(detail.getByTestId("ship-url")).toHaveValue(w.url);
  await expect(detail.getByTestId("ship-note")).toHaveValue(w.note);
}

// [FAULT INJECTION, not a substitute for a click] The two parcel READ-failure steps make exactly ONE real read answer 503 so the
// error copy and the Retry control can be reached; the Retry click itself is real, the route is removed before it, and the retry
// goes to the real Go/PG. countReads counts every GET of the URL the browser sent (served from the network, never a cache).
function countReads(page: Page, url: RegExp) {
  let n = 0;
  page.on("request", (request) => {
    if (request.method() === "GET" && url.test(request.url())) n += 1;
  });
  return () => n;
}
async function failNextRead(page: Page, url: RegExp) {
  let first = true;
  await page.route(url, async (route) => {
    if (first) {
      first = false;
      await route.fulfill({ status: 503, contentType: "application/json", headers: { "cache-control": "private, no-store" }, body: '{"code":"unavailable"}' });
    } else await route.continue();
  });
}
// Click Retry for real and return once the browser has received a NEW successful answer for url.
async function retryRead(page: Page, url: RegExp, reads: () => number) {
  const before = reads();
  const [response] = await Promise.all([
    page.waitForResponse((r) => r.request().method() === "GET" && url.test(r.url())),
    page.getByTestId("parcel-reload-retry").click(),
  ]);
  expect(response.status()).toBe(200);
  expect(reads()).toBeGreaterThan(before); // a fresh request, not a cached answer
}

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
    // Refusals first (client hints, visible copy, nothing sent, the group stays Open): `other` without a carrier name, then a non-https link.
    await panel.locator('[data-testid="parcel-ship-carrier"]').selectOption(shipWaybill.code);
    await panel.locator('[data-testid="parcel-ship-tracking"]').fill(shipWaybill.tracking);
    await panel.locator('[data-testid^="parcel-ship-submit-"]').click();
    await expect(panel.locator('[data-testid^="parcel-problem-"]')).toHaveText("Enter the carrier name (1–80 characters).");
    await panel.locator('[data-testid="parcel-ship-carrier-name"]').fill(shipWaybill.name);
    await panel.locator('[data-testid="parcel-ship-url"]').fill("http://track.example.com/t/not-https");
    await panel.locator('[data-testid^="parcel-ship-submit-"]').click();
    await expect(panel.locator('[data-testid^="parcel-problem-"]')).toHaveText("Tracking link must be a plain https address without a port or credentials.");
    await expect(panel.locator('[data-testid^="parcel-state-"]')).toHaveText("Open");
    // Then every control with valid input (carrier select incl. the `other` branch + name, tracking, link, note), and submit.
    await fillGroupWaybill(panel, shipWaybill);
    await panel.locator('[data-testid^="parcel-ship-submit-"]').click();
    await expect(panel.locator('[data-testid^="parcel-notice-"]')).toContainText("group is shipped");
    await expect(panel.locator('[data-testid^="parcel-state-"]')).toHaveText("Shipped");
    for (const id of [pm1, pm2]) {
      await expand(page, id);
      await expect(page.getByTestId(`parcel-badge-${id}`)).toBeVisible();
      const detail = page.getByTestId("order-detail");
      await expect(detail.locator('[data-state="MERCHANT_SHIPPED"]').first()).toBeVisible();
      await expect(detail.getByTestId("shipment-record")).toContainText(shipWaybill.tracking);
    }
  });

  await test.step("reload keeps the shipments; the SHIPPED group is history, so no OPEN panel is rebuilt", async () => {
    await page.reload();
    await expect(page.getByTestId("orders-table")).toBeVisible();
    await expect(page.locator('section[data-testid^="parcel-group-"]')).toHaveCount(0);
    // Persisted values of EACH member after the reload (record, link, history note, prefilled correction form).
    for (const id of [pm1, pm2]) await expectPersistedWaybill(page, id, shipWaybill);
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
    await fillGroupWaybill(panel, knownWaybill); // known carrier branch: no carrier name typed
    await panel.locator('[data-testid^="parcel-ship-submit-"]').click();
    await expect(panel.locator('[data-testid^="parcel-notice-"]')).toContainText("group is shipped");
    await page.reload();
    await expect(page.getByTestId("orders-table")).toBeVisible();
    for (const id of [pd1, pd2]) await expectPersistedWaybill(page, id, knownWaybill);
  });

  await test.step("a failed suggestions read shows its error and Retry; the real retry recovers the banner and the card", async () => {
    const url = new RegExp(`/api/stores/${store}/orders/merge-suggestions$`);
    const reads = countReads(page, url);
    await failNextRead(page, url); // [FAULT INJECTION] one 503 on the load below
    await page.reload();
    await expect(page.getByTestId("orders-table")).toBeVisible();
    await expect(page.getByTestId("parcel-merge-problem")).toHaveText("Merge suggestions could not be loaded. Retry, or reload the page.");
    await expect(page.getByTestId("parcel-reload-retry")).toHaveText("Retry loading");
    await expect(page.getByTestId("parcel-suggestion")).toHaveCount(0);
    expect(reads()).toBeGreaterThanOrEqual(1);
    await page.unroute(url); // the retry below reaches the real Go/PG
    await retryRead(page, url, reads);
    await expect(page.getByTestId("parcel-merge-problem")).toHaveCount(0);
    await expect(page.getByTestId("parcel-reload-retry")).toHaveCount(0);
    await expect(page.getByTestId("parcel-merge-count")).toHaveText("1 order group can ship as one parcel");
    await page.getByTestId("parcel-suggestions-toggle").click();
    await expect(suggestionCard(page, ps1)).toBeVisible();
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
    // [FAULT INJECTION / DELIVERY DELAY] Observe the real refusal before its real refresh replaces the form.
    // The OPEN-group hint intentionally replaces the whole form (including its error); no response is fabricated.
    const groupsURL = new RegExp(`/api/stores/${store}/parcel-groups$`);
    const releaseGroups = Promise.withResolvers<void>(), groupsArrived = Promise.withResolvers<void>();
    const groupsDone = Promise.withResolvers<void>();
    await page.route(groupsURL, async (route) => {
      if (route.request().method() !== "GET") { await route.continue(); return; }
      try {
        const response = await boundedPrivacyRead(route.fetch(), "post-refusal Go groups response");
        expect(response.status()).toBe(200);
        groupsArrived.resolve();
        await boundedPrivacyRead(releaseGroups.promise, "post-refusal groups release");
        await route.fulfill({ response });
        groupsDone.resolve();
      } catch (error) { groupsArrived.reject(error); groupsDone.reject(error); }
    });
    try {
      const refusal = page.waitForResponse((response) => response.request().method() === "PUT" &&
        new URL(response.url()).pathname === `/api/stores/${store}/orders/${ps1}/shipment`);
      await detail.getByTestId("shipment-submit").click();
      const response = await refusal;
      expect(response.status()).toBe(409);
      expect(await response.json()).toMatchObject({ code: "in_parcel_group" });
      await boundedPrivacyRead(groupsArrived.promise, "post-refusal groups arrival");
      // Server 409 in_parcel_group maps to the same sentence as the client hint (orders-copy errors.in_parcel_group).
      await expect(detail.getByTestId("shipment-problem")).toHaveText("This order is in a parcel group; fill in the waybill on the group.");
      await expect(detail.getByTestId("shipment-record")).toHaveCount(0);
    } finally {
      releaseGroups.resolve();
      await boundedPrivacyRead(groupsDone.promise, "post-refusal groups completion");
      await page.unroute(groupsURL);
    }
    // WITHOUT a reload: the refusal (onShipmentRefused -> parcel generation -> both parcel reads) rebuilds the group state in this tab.
    const panel = groupPanel(page, ps1);
    await expect(panel).toBeVisible();
    await expect(panel.locator('[data-testid^="parcel-state-"]')).toHaveText("Open");
    await expect(page.getByTestId(`parcel-badge-${ps1}`)).toBeVisible();
    await expect(detail.getByTestId("shipment-parcel-block")).toHaveText("This order is in a parcel group; fill in the waybill on the group.");
    await expect(detail.getByTestId("shipment-form")).toHaveCount(0);
  });

  await test.step("a failed OPEN-groups read shows its error and Retry; the real retry rebuilds the panel and the row badge", async () => {
    // Tab B's merge above left the stale pair in one OPEN group on the server; this tab (after a reload) must learn it from the read.
    const url = new RegExp(`/api/stores/${store}/parcel-groups$`);
    const reads = countReads(page, url);
    await failNextRead(page, url); // [FAULT INJECTION] one 503 on the load below
    await page.reload();
    await expect(page.getByTestId("orders-table")).toBeVisible();
    await expect(page.getByTestId("parcel-merge-problem")).toHaveText("Open parcel groups could not be loaded. Retry, or reload the page.");
    await expect(page.getByTestId("parcel-reload-retry")).toHaveText("Retry loading");
    await expect(groupPanel(page, ps1)).toHaveCount(0); // never silently pretend there is no OPEN group: the error says otherwise
    expect(reads()).toBeGreaterThanOrEqual(1);
    await page.unroute(url);
    await retryRead(page, url, reads);
    await expect(page.getByTestId("parcel-merge-problem")).toHaveCount(0);
    const panel = groupPanel(page, ps1);
    await expect(panel).toBeVisible();
    await expect(panel.locator('[data-testid^="parcel-state-"]')).toHaveText("Open");
    await expect(panel.getByTestId(`parcel-group-member-${ps1}`)).toContainText(`LC-${ps1.replaceAll("-", "").toUpperCase()} · S***`);
    await expand(page, ps1);
    await expect(page.getByTestId(`parcel-badge-${ps1}`)).toBeVisible();
    await expect(page.getByTestId("order-detail").getByTestId("shipment-parcel-block")).toBeVisible();
  });

  // [FAULT INJECTION / CLOCK] Only the existing 20s read poll is advanced; no product state or backend is mutated.
  // Keep ordinary 503 retry local, then change one retry read into an authoritative denial while sibling/poll completions wait.
  async function boundedPrivacyRead<T>(work: Promise<T>, description: string): Promise<T> {
    let timer: ReturnType<typeof setTimeout>;
    const timeout = new Promise<never>((_, reject) => {
      timer = setTimeout(() => reject(new Error(`Timed out waiting for ${description}`)), 10_000);
    });
    try { return await Promise.race([work, timeout]); }
    finally { clearTimeout(timer!); }
  }
  await page.clock.install();
  await page.clock.pauseAt(new Date());
  for (const read of ["suggestions", "groups"] as const) {
    for (const [status, copy] of [
      [401, "Sign in to view orders."],
      [403, "This account does not have permission to read orders."],
      [404, "This order is unavailable in the selected store."],
    ] as const) {
      await test.step(`${read} ${status} clears orders/detail/bulk/groups before stale read completions`, async () => {
        const suggestionsURL = new RegExp(`/api/stores/${store}/orders/merge-suggestions$`);
        const groupsURL = new RegExp(`/api/stores/${store}/parcel-groups$`);
        const ordersURL = new RegExp(`/api/stores/${store}/orders\\?`);
        const targetURL = read === "suggestions" ? suggestionsURL : groupsURL;
        const siblingURL = read === "suggestions" ? groupsURL : suggestionsURL;
        // Keep the recoverable failure until detail/bulk readiness: expanding detail can naturally restart parcel reads.
        await page.route(suggestionsURL, (route) => route.fulfill({ status: 503, contentType: "application/json", headers: { "cache-control": "private, no-store" }, body: '{"code":"unavailable"}' }));
        await page.reload();
        await expect(page.getByTestId("orders-table")).toBeVisible();
        await expect(groupPanel(page, ps1)).toBeVisible();
        await expand(page, ps1);
        await expect(page.getByTestId("order-detail")).toBeVisible();
        await page.getByTestId(`order-row-${ps1}`).getByRole("checkbox").check();
        await expect(page.getByTestId("pick-count")).not.toHaveText("0");
        await expect(page.getByTestId("parcel-reload-retry")).toBeVisible();
        await page.unroute(suggestionsURL);
        const releasePoll = Promise.withResolvers<void>(), releaseSibling = Promise.withResolvers<void>();
        const pollArrived = Promise.withResolvers<void>(), siblingArrived = Promise.withResolvers<void>();
        const pollDone = Promise.withResolvers<void>(), siblingDone = Promise.withResolvers<void>();
        const unexpected: unknown[] = [];
        await page.route(ordersURL, async (route) => {
          pollArrived.resolve();
          await boundedPrivacyRead(releasePoll.promise, "held order poll release");
          try {
            await route.fulfill({ status: 503, contentType: "application/json", headers: { "cache-control": "private, no-store" }, body: '{"code":"unavailable"}' });
          } catch (error) { if (!route.request().failure()) unexpected.push(error); }
          finally { pollDone.resolve(); }
        });
        await page.route(siblingURL, async (route) => {
          // The actual Go response is captured, then its delivery is held until the denial has purged the page.
          const response = await boundedPrivacyRead(route.fetch(), "actual sibling Go response");
          expect(response.status()).toBe(200);
          siblingArrived.resolve();
          await boundedPrivacyRead(releaseSibling.promise, "held sibling release");
          try { await route.fulfill({ response }); }
          catch (error) { if (!route.request().failure()) unexpected.push(error); }
          finally { siblingDone.resolve(); }
        });
        await page.route(targetURL, async (route) => {
          await boundedPrivacyRead(siblingArrived.promise, "sibling read arrival before denial");
          await route.fulfill({ status, contentType: "application/json", headers: { "cache-control": "private, no-store" }, body: '{"code":"MOCK_SCOPE_DENIAL"}' });
        });
        const expectPurged = async () => {
          await expect(page.getByTestId("merchant-orders")).toContainText(copy);
          await expect(page.getByTestId("orders-table")).toHaveCount(0);
          await expect(page.getByTestId("order-detail")).toHaveCount(0);
          await expect(page.getByTestId("pick-toolbar")).toHaveCount(0);
          await expect(page.locator('section[data-testid^="parcel-group-"]')).toHaveCount(0);
          await expect(page.getByTestId("parcel-suggestion")).toHaveCount(0);
          await expect(page.getByTestId("parcel-reload-retry")).toHaveCount(0);
        };
        try {
          await page.clock.fastForward(20_001);
          await boundedPrivacyRead(pollArrived.promise, "actual in-flight order poll arrival");
          await page.getByTestId("parcel-reload-retry").click();
          await expectPurged(); // Both stale completions are STILL held here.
          releasePoll.resolve();
          releaseSibling.resolve();
          await boundedPrivacyRead(Promise.all([pollDone.promise, siblingDone.promise]), "released stale read completions");
          expect(unexpected).toEqual([]);
          // Completion barrier for the released browser reads; lets their actual fetch/React continuations finish.
          await page.waitForLoadState("networkidle");
          await expectPurged();
        } finally {
          releasePoll.resolve();
          releaseSibling.resolve();
          await page.unroute(ordersURL);
          await page.unroute(siblingURL);
          await page.unroute(targetURL);
        }
      }, { timeout: 20_000 });
    }
  }

});
