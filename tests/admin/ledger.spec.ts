import { test, expect } from "./fixtures/ledger-identity";
import { mkdir } from "node:fs/promises";

test.describe.configure({ mode: "serial" });

test("approved ledger reproduction and mobile table remain usable", async ({
  page,
}) => {
  await page.goto("/zh-CN/inventory");
  await expect(page.getByRole("heading", { name: "商品与库存" })).toBeVisible();
  await expect(page.locator("tbody tr")).toHaveCount(9);
  await expect(
    page.getByText("隔离本地测试店铺 · 非客户真实库存"),
  ).toBeVisible();
  await page
    .getByRole("radio", { name: "选择 AC-002-BK", exact: true })
    .check();
  await mkdir("output/playwright/ledger-review", { recursive: true });
  await page.screenshot({
    path: "output/playwright/ledger-review/hero-repro.png",
    animations: "disabled",
  });
  await page.screenshot({
    path: "output/playwright/ledger-review/desktop.png",
    animations: "disabled",
  });
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.getByRole("button", { name: "打开导航" })).toBeVisible();
  expect(
    // G-UI8 audit [READ/MEASURE]: measures horizontal overflow (layout read, no state change)
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({
    path: "output/playwright/ledger-review/mobile.png",
    fullPage: true,
    animations: "disabled",
  });
  await page.getByRole("button", { name: "打开导航" }).click();
  // catalog-media: the dead "not connected" entries are gone from the navigation, so no placeholder is reachable.
  for (const name of ["网站客服", "Meta 消息", "平台支持"])
    await expect(page.getByRole("button", { name, exact: true })).toHaveCount(0);
});

test("locale routes preserve the scoped search and explicit choice", async ({
  page,
}) => {
  await page.goto("/zh-CN/inventory?q=HA-001-BE");
  await expect(page.locator("tbody tr")).toHaveCount(1);
  await page.getByRole("combobox", { name: "语言" }).selectOption("zh-TW");
  await expect(page).toHaveURL(/\/zh-TW\/inventory\?q=HA-001-BE/);
  await expect(page.getByRole("heading", { name: "商品與庫存" })).toBeVisible();
  await expect(page.locator("html")).toHaveAttribute("lang", "zh-TW");
  await page.getByRole("combobox", { name: "語言" }).selectOption("en");
  await expect(
    page.getByRole("heading", { name: "Products & inventory" }),
  ).toBeVisible();
  await expect(page).toHaveURL(/\/en\/inventory\?q=HA-001-BE/);
  await page.goto("/inventory?q=AC-002-BK");
  await expect(page).toHaveURL(/\/en\/inventory\?q=AC-002-BK/);
  await expect(page.locator("tbody tr")).toHaveCount(1);
  await page.goto("/en/inventory?q=%25");
  await expect(
    page.getByRole("heading", { name: "No matching SKUs" }),
  ).toBeVisible();
  await expect(page.locator('.message[role="alert"]')).toHaveCount(0);
});

test("lost mutation response retries original key and payload only once", async ({
  page,
}) => {
  await page.goto("/en/inventory?q=AC-002-BK");
  await page
    .getByRole("radio", { name: "Select AC-002-BK", exact: true })
    .check();
  const available = Number(
    await page.locator("tbody .available-value").innerText(),
  );
  const requests: { key: string | null; body: string | null }[] = [];
  await page.route("**/api/stores/*/inventory/adjustments", async (route) => {
    requests.push({
      key: route.request().headers()["idempotency-key"],
      body: route.request().postData(),
    });
    if (requests.length === 1) {
      const accepted = await route.fetch();
      expect(accepted.status()).toBe(200);
      await route.abort("connectionreset");
    } else await route.continue();
  });
  await page.getByRole("spinbutton", { name: "Adjustment quantity" }).fill("3");
  await page
    .getByRole("textbox", { name: "Reason (required)" })
    .fill("Browser acceptance: response loss");
  await page.getByRole("button", { name: "Confirm adjustment" }).click();
  await expect(
    page.getByRole("button", { name: "Retry", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("spinbutton", { name: "Adjustment quantity" }),
  ).toBeDisabled();
  await page.getByRole("button", { name: "Retry", exact: true }).click();
  await expect(
    page.getByText("Inventory updated", { exact: true }),
  ).toBeVisible();
  await expect(page.locator("tbody .available-value")).toHaveText(
    String(available + 3),
  );
  expect(requests).toHaveLength(2);
  expect(requests[1]).toEqual(requests[0]);
});

test("uncertain command survives closing a tab and storage denial sends no write", async ({
  page,
  context,
}) => {
  await page.goto("/en/inventory?q=CB-005-TC");
  await page
    .getByRole("radio", { name: "Select CB-005-TC", exact: true })
    .check();
  const available = Number(
    await page.locator("tbody .available-value").innerText(),
  );
  let originalKey = "";
  await page.route("**/api/stores/*/inventory/adjustments", async (route) => {
    originalKey = route.request().headers()["idempotency-key"];
    expect((await route.fetch()).status()).toBe(200);
    await route.abort("connectionreset");
  });
  await page.getByRole("spinbutton", { name: "Adjustment quantity" }).fill("4");
  await page
    .getByRole("textbox", { name: "Reason (required)" })
    .fill("Recover after closing tab");
  await page.getByRole("button", { name: "Confirm adjustment" }).click();
  await expect(
    page.getByRole("button", { name: "Retry", exact: true }),
  ).toBeVisible();
  await page.close();
  const recovered = await context.newPage();
  await recovered.goto("/en/inventory?q=CB-005-TC");
  await expect(
    recovered.getByRole("button", { name: "Retry", exact: true }),
  ).toBeVisible();
  const retried = recovered.waitForRequest("**/inventory/adjustments");
  await recovered.getByRole("button", { name: "Retry", exact: true }).click();
  expect((await retried).headers()["idempotency-key"]).toBe(originalKey);
  await expect(
    recovered.getByText("Inventory updated", { exact: true }),
  ).toBeVisible();
  await expect(recovered.locator("tbody .available-value")).toHaveText(
    String(available + 4),
  );
  await recovered
    .getByRole("radio", { name: "Select CB-005-TC", exact: true })
    .check();
  await recovered.evaluate(() => {
    Storage.prototype.setItem = () => {
      throw new DOMException("Blocked", "QuotaExceededError");
    };
  });
  let writes = 0;
  recovered.on("request", (request) => {
    if (request.method() === "POST") writes++;
  });
  await recovered
    .getByRole("spinbutton", { name: "Adjustment quantity" })
    .fill("2");
  await recovered
    .getByRole("textbox", { name: "Reason (required)" })
    .fill("Must not be sent without a journal");
  await recovered.getByRole("button", { name: "Confirm adjustment" }).click();
  await expect(recovered.locator('.message[role="alert"]')).toContainText(
    "command storage",
  );
  expect(writes).toBe(0);
});

test("stale balance fails closed and refresh enables a new command", async ({
  page,
  request,
}) => {
  await page.goto("/en/inventory?q=HA-001-BE");
  await page
    .getByRole("radio", { name: "Select HA-001-BE", exact: true })
    .check();
  const storeID = process.env.COMMERCE_FIXTURE_STORE_ID!;
  expect(storeID).toBeTruthy();
  const warehouseReply = await request.get(`/api/stores/${storeID}/warehouses`);
  const warehouse = (await warehouseReply.json()).items[0].id;
  const ledger = await request.get(
    `/api/stores/${storeID}/catalog-ledger?warehouse_id=${warehouse}&q=HA-001-BE`,
  );
  const row = (await ledger.json()).items[0];
  // G-UI8 audit [FIXTURE/SETUP]: a second operator's concurrent write (stale-version fixture for the conflict UI)
  const changed = await request.post(
    `/api/stores/${storeID}/inventory/adjustments`,
    {
      headers: {
        Origin: "http://127.0.0.1:3100",
        "Idempotency-Key": crypto.randomUUID(),
      },
      data: {
        warehouse_id: warehouse,
        sku_id: row.sku_id,
        expected_version: row.balance_version,
        delta: 1,
        reason: "Concurrent independent acceptance command",
      },
    },
  );
  expect(changed.status()).toBe(200);
  await page.getByRole("spinbutton", { name: "Adjustment quantity" }).fill("2");
  await page
    .getByRole("textbox", { name: "Reason (required)" })
    .fill("Stale version must fail");
  await page.getByRole("button", { name: "Confirm adjustment" }).click();
  await expect(page.locator('.message[role="alert"]')).toContainText(
    "Inventory changed",
  );
  await page
    .getByRole("region", { name: "Products & inventory", exact: true })
    .getByRole("button", { name: "Refresh", exact: true })
    .click();
  await expect(page.locator("tbody .available-value")).toHaveText(
    String(row.available + 1),
  );
});

// stop-bleed D01 (product-editor §c9): the inline "quick add" (product, then first SKU) is gone from this page. A product is created in the
// full editor (/products/new; the catalog-core gate drives that flow with a real session), so here "Add product" is a link and no create
// panel exists to open.
test("Add product leaves for the product editor; no inline create panel on the inventory page", async ({
  page,
}) => {
  await page.goto("/en/inventory");
  await expect(page.locator(".create-panel")).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Add product", exact: true })).toHaveCount(0);
  const add = page.getByRole("link", { name: "Add product", exact: true });
  await expect(add).toHaveAttribute("href", /^\/en\/products\/new\?store=[0-9a-f-]{36}$/);
  await add.click();
  await expect(page).toHaveURL(/\/en\/products\/new\?store=[0-9a-f-]{36}$/);
  await expect(page.locator(".create-panel")).toHaveCount(0);
});

test("BFF rejects cross-store, foreign origin, route injection and oversized writes", async ({
  request,
  page,
}) => {
  const storeID = process.env.COMMERCE_FIXTURE_STORE_ID!;
  expect(storeID).toBeTruthy();
  const cross = await request.get(
    "/api/stores/00000000-0000-0000-0000-000000000000/products",
  );
  expect(cross.status()).toBe(404);
  // G-UI8 audit [FIXTURE/SETUP]: negative probe: a forged/hostile request no UI can send; the server, not the UI, must refuse (UI click paths of the same route are covered elsewhere)
  const foreign = await request.post(`/api/stores/${storeID}/products`, {
    headers: {
      Origin: "https://example.invalid",
      "Idempotency-Key": crypto.randomUUID(),
    },
    data: { name: "MUST NOT CREATE" },
  });
  expect(foreign.status()).toBe(403);
  const badRoute = await request.get(`/api/stores/${storeID}/secrets`);
  expect(badRoute.status()).toBe(404);
  const bad = await badRoute.json();
  expect(bad.request_id).toMatch(/^[0-9a-f]{32}$/);
  expect(badRoute.headers()["x-request-id"]).toBe(bad.request_id);
  const wrongMethod = await request.delete(`/api/stores/${storeID}/products`);
  expect(wrongMethod.status()).toBe(405);
  expect((await wrongMethod.json()).code).toBe("method_not_allowed");
  // G-UI8 audit [FIXTURE/SETUP]: negative probe: a forged/hostile request no UI can send; the server, not the UI, must refuse (UI click paths of the same route are covered elsewhere)
  const huge = await request.post(`/api/stores/${storeID}/products`, {
    headers: {
      Origin: "http://127.0.0.1:3100",
      "Idempotency-Key": crypto.randomUUID(),
    },
    data: { name: "x".repeat(65537) },
  });
  expect(huge.status()).toBe(400);
  await page.goto("/en/inventory");
  const html = await page.content();
  expect(html).not.toContain(process.env.COMMERCE_FIXTURE_TOKEN!);
});

test("purchase-entry proxy only accepts the scoped GET locale query", async ({
  request,
}) => {
  const store = process.env.COMMERCE_FIXTURE_STORE_ID!;
  const warehouses = await request.get(`/api/stores/${store}/warehouses`);
  const warehouseID = (await warehouses.json()).items[0].id;
  const ledger = await request.get(
    `/api/stores/${store}/catalog-ledger?warehouse_id=${warehouseID}`,
  );
  const productID = (await ledger.json()).items[0].product_id;
  const path = `/api/stores/${store}/products/${productID}/purchase-entry`;
  const valid = await request.get(`${path}?locale=en`);
  expect(valid.status()).toBe(200);
  const projection = await valid.json();
  expect(Object.keys(projection).sort()).toEqual([
    "locale",
    "product_id",
    "state",
    "url",
  ]);
  expect(projection.product_id).toBe(productID);
  // Valid URL encoding is equivalent under the frozen locale contract.
  // Next normalizes the incoming query before this BFF receives Request.url;
  // invalid encodings remain a denial case below, not a locale alias.
  const encoded = await request.get(`${path}?locale=%65n`);
  expect(encoded.status()).toBe(200);
  expect(await encoded.json()).toEqual(projection);
  for (const suffix of [
    "",
    "?",
    "?locale=",
    "?locale=fr",
    "?locale=en&locale=en",
    "?locale=en&extra=1",
    "?locale=%zz",
  ]) {
    const denied = await request.get(path + suffix);
    expect(denied.status(), suffix).toBe(422);
  }
  const key = await request.get(`${path}?locale=en`, {
    headers: { "Idempotency-Key": "unexpected-key" },
  });
  expect(key.status()).toBe(422);
  // G-UI8 audit [FIXTURE/SETUP]: negative probe: a forged/hostile request no UI can send; the server, not the UI, must refuse (UI click paths of the same route are covered elsewhere)
  const post = await request.post(`${path}?locale=en`, { data: {} });
  expect(post.status()).toBe(404);
});

test("purchase-entry read failure says the product is unchanged, offers a refresh and sends no write", async ({
  page,
}) => {
  let writes = 0;
  let reads = 0;
  page.on("request", (request) => {
    if (request.method() === "POST") writes++;
  });
  await page.route(
    /\/api\/stores\/[^/]+\/products\/[^/]+\/purchase-entry\?locale=en$/,
    async (route) => {
      reads++;
      await route.fulfill({
        status: 503,
        contentType: "application/json",
        body: JSON.stringify({
          code: "unavailable",
          message: "",
          request_id: "",
          retryable: true,
          details: {},
        }),
      });
    },
  );
  await page.goto("/en/inventory"); // the fixture pre-selects a row, which loads its purchase entry
  const panel = page.getByTestId("purchase-entry");
  await expect(panel.getByRole("alert")).toContainText("saved product is unchanged");
  expect(reads).toBeGreaterThanOrEqual(1);
  const before = reads;
  await panel.getByRole("button", { name: "Refresh" }).click();
  await expect.poll(() => reads).toBeGreaterThan(before);
  await expect(panel.getByRole("alert")).toContainText("saved product is unchanged");
  expect(writes).toBe(0);
});

test("stock adjustment denial reports permission without claiming the inventory was updated", async ({
  page,
}) => {
  let writes = 0;
  await page.route("**/api/stores/*/inventory/adjustments", async (route) => {
    writes++;
    await route.fulfill({
      status: 403,
      contentType: "application/json",
      body: JSON.stringify({
        code: "forbidden",
        message: "",
        request_id: "",
        retryable: false,
        details: {},
      }),
    });
  });
  await page.goto("/en/inventory?q=AC-002-BK");
  await page
    .getByRole("radio", { name: "Select AC-002-BK", exact: true })
    .check();
  await page.getByRole("spinbutton", { name: "Adjustment quantity" }).fill("1");
  await page
    .getByRole("textbox", { name: "Reason (required)" })
    .fill("Permission probe");
  await page.getByRole("button", { name: "Confirm adjustment" }).click();
  await expect(page.locator('.message[role="alert"]')).toContainText(
    "cannot perform this action",
  );
  await expect(
    page.getByText("Inventory updated", { exact: true }),
  ).toHaveCount(0);
  expect(writes).toBe(1);
});

test("purchase controls recheck current state before copying or opening", async ({
  page,
}) => {
  let available = true;
  let reads = 0;
  await page.route(
    /\/api\/stores\/[^/]+\/products\/[^/]+\/purchase-entry\?locale=en$/,
    async (route) => {
      reads++;
      const requestURL = new URL(route.request().url());
      const productID = requestURL.pathname.split("/").at(-2)!;
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          product_id: productID,
          locale: "en",
          state: available ? "configured" : "storefront_unavailable",
          url: available ? `https://shop.example/en/products/${productID}` : "",
        }),
      });
    },
  );
  await page.goto("/en/inventory");
  const panel = page.getByTestId("purchase-entry");
  await expect(
    panel.getByRole("button", { name: "Copy address" }),
  ).toBeVisible();
  available = false;
  await panel.getByRole("button", { name: "Copy address" }).click();
  await expect(panel).toContainText(
    "No verified, published storefront address is available.",
  );
  await expect(panel.getByRole("button", { name: "Copy address" })).toHaveCount(
    0,
  );
  expect(reads).toBeGreaterThanOrEqual(2);
  available = true;
  await panel.getByRole("button", { name: "Refresh" }).click();
  await expect(
    panel.getByRole("button", { name: "Open purchase page" }),
  ).toBeVisible();
  await page.route("https://shop.example/**", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "text/html",
      body: "<title>Buyer page</title>",
    });
  });
  await panel.getByRole("button", { name: "Open purchase page" }).click();
  await expect(page).toHaveURL(/https:\/\/shop\.example\/en\/products\//);
  expect(reads).toBeGreaterThanOrEqual(4);
});

// ---- stop-bleed D01 (UI architecture v2 §10.7): the selected-row tray is stock only -----------------------------------
// The tray used to carry product editing, price edit, photos and archive in one flex row that overflowed above 1280px
// (REPORT-admin-vqa D01). It now keeps stock info, the explicit stock adjustment (reason required), the read-only price and a link
// to the product page. Geometry is asserted on every visible atom of the tray at the owner/report sizes; screenshots go to
// output/admin-ui-fixes/ (UI_SHOT_PHASE=before|after, default after).
const shotPhase = process.env.UI_SHOT_PHASE ?? "after"; // "before" only captures screenshots on the pre-fix code

for (const [width, height] of [
  [1366, 768],
  [1586, 992],
  [2000, 1100],
  [375, 812],
] as const) {
  test(`inventory tray has no overlap, clipping or overflow at ${width}x${height}`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height });
    await page.goto("/en/inventory");
    const tray = page.locator("section.inspector");
    await expect(tray).toBeVisible(); // the fixture pre-selects a row
    await mkdir("output/admin-ui-fixes", { recursive: true });
    await page.screenshot({
      path: `output/admin-ui-fixes/${shotPhase}-d01-inventory-${width}.png`,
      fullPage: true,
      animations: "disabled",
    });
    if (shotPhase === "before") return; // capture-only run on the pre-fix code (the red run is recorded in the unit notes)
    const geometry = await tray.evaluate((root) => {
      const atoms = Array.from(
        root.querySelectorAll<HTMLElement>(
          "button, a, input, select, textarea, label, h2, dt, dd, p, small, .status, img, svg",
        ),
      ).filter((el) => {
        const r = el.getBoundingClientRect();
        return r.width > 0 && r.height > 0 && getComputedStyle(el).visibility !== "hidden";
      });
      const box = (el: Element) => el.getBoundingClientRect();
      const tr = box(root);
      const problems: string[] = [];
      const name = (el: HTMLElement) =>
        `${el.tagName.toLowerCase()}${el.className ? "." + String(el.className).split(" ")[0] : ""}[${(el.textContent ?? "").trim().slice(0, 18)}]`;
      for (const el of atoms) {
        const r = box(el);
        if (r.left < tr.left - 1 || r.right > tr.right + 1)
          problems.push(`${name(el)} leaves the tray (${Math.round(r.left)}..${Math.round(r.right)} vs ${Math.round(tr.left)}..${Math.round(tr.right)})`);
        if (r.right > innerWidth + 1 || r.left < -1)
          problems.push(`${name(el)} leaves the viewport`);
      }
      for (let i = 0; i < atoms.length; i++)
        for (let j = i + 1; j < atoms.length; j++) {
          const a = atoms[i], b = atoms[j];
          if (a.contains(b) || b.contains(a)) continue;
          const ra = box(a), rb = box(b);
          const w = Math.min(ra.right, rb.right) - Math.max(ra.left, rb.left);
          const h = Math.min(ra.bottom, rb.bottom) - Math.max(ra.top, rb.top);
          if (w > 2 && h > 2) problems.push(`${name(a)} overlaps ${name(b)} (${Math.round(w)}x${Math.round(h)})`);
        }
      return {
        problems,
        pageOverflow: document.documentElement.scrollWidth > innerWidth,
        atoms: atoms.length,
      };
    });
    expect(geometry.atoms).toBeGreaterThan(5);
    expect(geometry.pageOverflow, "no horizontal page scroll").toBe(false);
    expect(geometry.problems).toEqual([]);
  });
}

test("inventory tray keeps stock and read-only price; product editing lives on the product page", async ({
  page,
}) => {
  await page.goto("/en/inventory");
  const tray = page.locator("section.inspector");
  await expect(tray).toBeVisible();
  // kept: stock info, the explicit adjustment with a required reason
  await expect(tray.getByRole("heading", { name: "Inventory", exact: true })).toBeVisible();
  await expect(tray.getByRole("spinbutton", { name: "Adjustment quantity" })).toBeVisible();
  await expect(tray.getByRole("textbox", { name: "Reason (required)" })).toBeVisible();
  await expect(tray.getByRole("button", { name: "Confirm adjustment" })).toBeVisible();
  // read-only price in whole dollars, never minor units or decimals
  const price = tray.getByTestId("tray-price");
  await expect(price).toHaveText(/^Price: NT\$[\d,]+$/);
  await expect(price.locator("input")).toHaveCount(0);
  // the link to the product page
  const edit = tray.getByRole("link", { name: "Edit product →" });
  await expect(edit).toHaveAttribute("href", /^\/en\/products\/[0-9a-f-]{36}(\?store=[0-9a-f-]{36})?$/);
  // removed (product-editor §c9): name/description/price/photo/archive editing and the inline quick-add panel
  for (const id of ["product-edit", "price-edit", "archive-actions", "archive-sku", "archive-product", "photo-manager"])
    await expect(tray.getByTestId(id), id).toHaveCount(0);
  await expect(tray.locator("input[name=name], input[name=price], textarea[name=description]")).toHaveCount(0);
  await expect(page.locator(".create-panel")).toHaveCount(0);
  // the header "Add product" is a plain link to the full editor, not a panel toggle
  const add = page.getByRole("link", { name: "Add product", exact: true });
  await expect(add).toHaveAttribute("href", /^\/en\/products\/new(\?store=[0-9a-f-]{36})?$/);
  await add.click();
  await expect(page).toHaveURL(/\/en\/products\/new/);
});

// ---- stop-bleed D04 + M07/M08: no invented channel status, and the nav scrolls so Settings and Sign out stay reachable ---------------
test("rail has no hard-coded channel status and scrolls to Settings and Sign out at 1366x768", async ({ page }) => {
  await page.setViewportSize({ width: 1366, height: 768 });
  await page.goto("/en/inventory");
  const rail = page.locator("aside[data-shell-rail]");
  await expect(rail).toBeVisible();
  // the fake "Channel status" block (Storefront/Facebook/Instagram/WhatsApp/LINE, always "Not connected") is gone
  await expect(rail.getByText("Channel status")).toHaveCount(0);
  for (const fake of ["WhatsApp", "LINE", "Facebook", "Instagram", "Not connected"]) await expect(rail.getByText(fake, { exact: true })).toHaveCount(0);
  // W0 scrolls the registry navigation independently of pinned Settings; the
  // minimal-permission fixture no longer exposes fourteen unrelated routes.
  const navigation = rail.getByRole("navigation", { name: "Workspace navigation", exact: true });
  const geometry = await navigation.evaluate((el) => ({ overflowY: getComputedStyle(el).overflowY, height: el.getBoundingClientRect().height }));
  expect(geometry.overflowY).toBe("auto");
  expect(geometry.height).toBeLessThanOrEqual(768);
  // Exercise real overflow with this role's small route set, without injecting
  // fake entries or granting more domains merely to make the rail tall.
  await page.setViewportSize({ width: 1366, height: 300 });
  expect(await navigation.evaluate((el) => el.scrollHeight > el.clientHeight), "the constrained viewport really needs the scroll").toBe(true);
  await page.getByTestId("nav-inventory").evaluate((el) => el.scrollIntoView({ block: "center", inline: "nearest" }));
  await expect(page.getByTestId("nav-inventory")).toBeInViewport({ ratio: 1 });
  await page.setViewportSize({ width: 1366, height: 768 });
  await mkdir("output/admin-ui-fixes", { recursive: true });
  await page.screenshot({ path: `output/admin-ui-fixes/${shotPhase}-d04-rail-top-1366x768.png`, animations: "disabled" });
  await page.locator("summary").filter({ hasText: /^Account$/ }).click();
  for (const [name, button] of [["Settings", page.getByTestId("nav-group-settings")], ["Sign out", page.getByTestId("workspace-sign-out")]] as const) {
    await button.scrollIntoViewIfNeeded();
    await expect(button).toBeInViewport({ ratio: 1 });
    // nothing paints over it: the element at its centre is the button (or inside it)
    const hit = await button.evaluate((el) => {
      const r = el.getBoundingClientRect();
      const top = document.elementFromPoint(r.left + r.width / 2, r.top + r.height / 2);
      return top === el || el.contains(top);
    });
    expect(hit, `${name} is reachable (nothing covers it)`).toBe(true);
  }
  await page.screenshot({ path: `output/admin-ui-fixes/${shotPhase}-d04-rail-bottom-1366x768.png`, animations: "disabled" });
  await page.locator("summary").filter({ hasText: /^Account$/ }).click();
  // Settings stays in the drawer; Sign out lives in the reachable top-bar menu.
  await page.setViewportSize({ width: 375, height: 667 });
  await page.getByRole("button", { name: "Open navigation" }).click();
  const drawer = page.locator("aside[data-shell-rail]");
  await drawer.getByTestId("nav-group-settings").scrollIntoViewIfNeeded();
  await expect(drawer.getByTestId("nav-group-settings")).toBeInViewport({ ratio: 1 });
  await page.keyboard.press("Escape");
  await page.locator("summary").filter({ hasText: /^Account$/ }).click();
  await expect(page.getByTestId("workspace-sign-out")).toBeInViewport({ ratio: 1 });
});
