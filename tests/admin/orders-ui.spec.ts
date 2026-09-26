import { expect, test, type BrowserContext, type Page } from "@playwright/test";

const required = (name: string) => {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
};
const origin = required("LC_BROWSER_PUBLIC_ORIGIN");
const apiOrigin = required("LC_BROWSER_API_ORIGIN");
const store = required("LC_BROWSER_ORDER_STORE");
const ids = JSON.parse(required("LC_BROWSER_ORDER_IDS")) as Record<
  string,
  string
>;
const frozenSKU = required("LC_BROWSER_FROZEN_SKU_CODE");
const foreignStore = required("LC_BROWSER_FOREIGN_STORE");
const foreignOrder = required("LC_BROWSER_FOREIGN_ORDER_ID");
const unlistedStore = required("LC_BROWSER_UNLISTED_STORE");
const secondToken = required("LC_BROWSER_SECOND_TOKEN");
const noOrdersToken = required("LC_BROWSER_NO_ORDERS_TOKEN");
const expiredToken = required("LC_BROWSER_EXPIRED_TOKEN");
const revokedToken = required("LC_BROWSER_REVOKED_TOKEN");
const cookieName = "__Host-commerce_session";
const pii = ["Synthetic Buyer", "+886900000001", "Synthetic home address"];

test.use({
  baseURL: origin,
  trace: "retain-on-failure",
  screenshot: "only-on-failure",
});

async function signedLogin(page: Page) {
  await page.goto("/en/");
  await page
    .getByRole("button", { name: "Sign in with identity service" })
    .click();
  await expect(page.getByTestId("nav-orders")).toBeVisible();
  await page.getByTestId("nav-orders").click();
  await expect(page.getByTestId("merchant-orders")).toBeVisible();
  await expect(page).toHaveURL(new RegExp(`/en/orders`));
}

async function session(context: BrowserContext, token: string) {
  await context.addCookies([
    {
      name: cookieName,
      value: token,
      url: origin,
      secure: true,
      httpOnly: true,
      sameSite: "Lax",
    },
  ]);
}

async function noPII(page: Page) {
  const content = await page.locator("body").innerText();
  for (const value of pii) expect(content).not.toContain(value);
  await expect(page.getByTestId("order-detail")).toHaveCount(0);
}

async function noPersistentOrderBody(page: Page) {
  const storage = await page.evaluate(async () => {
    const cacheNames = "caches" in window ? await caches.keys() : [];
    return JSON.stringify({
      local: { ...localStorage },
      session: { ...sessionStorage },
      cacheNames,
    });
  });
  for (const value of pii) expect(storage).not.toContain(value);
  expect(storage).not.toContain(ids.pending);
}

async function detailCalls(page: Page) {
  const response = await page.request.get(
    `${apiOrigin}/__test/order-observation`,
  );
  expect(response.status()).toBe(200);
  return ((await response.json()) as { details: number }).details;
}

async function expand(page: Page, id: string) {
  for (let p = 0; p < 3; p++) {
    if (await page.getByTestId(`order-expand-${id}`).count()) {
      await page.getByTestId(`order-expand-${id}`).click();
      await expect(page.getByTestId("order-detail")).toContainText(id);
      return;
    }
    await expect(page.getByTestId("orders-next")).toBeEnabled();
    await page.getByTestId("orders-next").click();
  }
  throw new Error("fixture order was absent from three real cursor pages");
}

async function screenshot(page: Page, file: string) {
  await page.screenshot({ path: file, fullPage: false });
  const width = await page.evaluate(
    () => document.documentElement.scrollWidth - innerWidth,
  );
  expect(width).toBeLessThanOrEqual(1);
}

test("MOU01/04 real login, cursor pagination, financial state and frozen detail", async ({
  page,
  context,
}) => {
  await signedLogin(page);
  const auth = (await context.cookies()).filter((c) => c.name === cookieName);
  expect(auth).toHaveLength(1);
  expect(auth[0].httpOnly).toBe(true);
  const list = await page.request.get(
    `/api/stores/${store}/orders?limit=10&state=all`,
  );
  expect(list.status()).toBe(200);
  expect(list.headers()["cache-control"]).toBe("private, no-store");
  const body = (await list.json()) as {
    items: Array<{ order_id: string }>;
    next_cursor: string;
  };
  expect(body.items).toHaveLength(10);
  expect(body.next_cursor).toMatch(/^[A-Za-z0-9_-]{1,1024}$/);
  await expect(page.getByTestId("orders-table")).toBeVisible();
  expect(await detailCalls(page)).toBe(0); // No eager recipient fetch for ten rows.
  expect(
    await page.getByTestId("orders-table").getByRole("row").count(),
  ).toBeGreaterThanOrEqual(11);
  await noPII(page); // Detail is not eagerly fetched for every row.
  await page.getByTestId("orders-next").click();
  await expect(page).toHaveURL(/cursor=/);
  await expand(page, ids.draft0);
  await expect(page.getByTestId("order-detail")).toContainText(
    "Synthetic home address",
  );
  await noPersistentOrderBody(page);
  await page.getByTestId("orders-previous").click();
  await expect(page).not.toHaveURL(/cursor=/);
  await expect(page.getByTestId("order-detail")).toHaveCount(0);
  await page.getByTestId("orders-refresh").click();
  await expect(page.getByTestId("orders-table")).toBeVisible();
  for (const [mode, expected] of [
    ["pending", ["AWAITING_PAYMENT", "PENDING", "NONE"]],
    ["authorized", ["AWAITING_PAYMENT", "AUTHORIZED", "NONE"]],
    ["captured", ["CONFIRMED", "CAPTURED", "READY"]],
    ["review", ["AWAITING_PAYMENT", "REVIEW_REQUIRED", "REVIEW_REQUIRED"]],
    [
      "allocation_failed",
      ["CANCELLED", "REVIEW_REQUIRED", "PAID_ALLOCATION_FAILED"],
    ],
    ["expired", ["CANCELLED", "NOT_STARTED", "CANCELLED"]],
  ] as const) {
    await expand(page, ids[mode]);
    const detail = page.getByTestId("order-detail");
    for (const state of expected)
      await expect(
        detail.locator(`[data-state="${state}"]`).first(),
      ).toBeVisible();
    await expect(detail).toContainText("TWD");
    await expect(detail).toContainText("Synthetic home address");
    await noPersistentOrderBody(page);
  }
  const detail = await page.request.get(
    `/api/stores/${store}/orders/${ids.pending}`,
  );
  expect(detail.headers()["cache-control"]).toBe("private, no-store");
  expect(detail.status()).toBe(200);
  await expand(page, ids.pickup);
  await expect(page.getByTestId("order-detail")).toContainText("017888");
  await expect(page.getByTestId("order-detail")).toContainText(frozenSKU);
  await expect(page.getByTestId("order-detail")).not.toContainText("NEW-CODE");
  await expect(page.getByTestId("order-detail")).not.toContainText("99999");
});

test("MOU02 two principals, store authority and invalid sessions never reveal PII", async ({
  page,
  context,
}) => {
  await signedLogin(page);
  const missing = await page.request.get(
    `/api/stores/${store}/orders/${foreignOrder}`,
  );
  expect(missing.status()).toBe(404);
  expect(await missing.text()).not.toContain("Synthetic Buyer");
  await page.getByTestId("store-selector").selectOption(foreignStore);
  await expect(page).toHaveURL(new RegExp(`store=${foreignStore}`));
  await expand(page, foreignOrder);
  await expect(page.getByTestId("order-detail")).toContainText(
    "Synthetic Buyer",
  );
  const crossStore = await page.request.get(
    `/api/stores/${foreignStore}/orders/${ids.pending}`,
  );
  expect(crossStore.status()).toBe(404);
  expect(await crossStore.text()).not.toContain("Synthetic Buyer");
  for (const badStore of [unlistedStore, "not-a-uuid"]) {
    await page.goto(`/en/orders?store=${badStore}`);
    await noPII(page);
    await expect(page.getByTestId("orders-table")).toHaveCount(0);
  }
  await session(context, secondToken);
  await page.goto(`/en/orders?store=${store}`);
  await noPII(page);
  expect((await page.request.get(`/api/stores/${store}/orders`)).status()).toBe(
    403,
  );
  await page.goto(`/en/orders?store=${foreignStore}`);
  await expand(page, foreignOrder);
  await session(context, noOrdersToken);
  await page.goto(`/en/orders?store=${store}`);
  await noPII(page);
  expect((await page.request.get(`/api/stores/${store}/orders`)).status()).toBe(
    403,
  );
  for (const token of [expiredToken, revokedToken, "not-a-session"]) {
    await session(context, token);
    const response = await page.request.get(`/api/stores/${store}/orders`);
    expect(response.status()).toBe(401);
    expect(response.headers()["cache-control"]).toBe("no-store");
    expect(response.headers()["set-cookie"]).toContain(`${cookieName}=`);
    expect(response.headers()["set-cookie"]).toContain("Max-Age=0");
    await page.goto(`/en/orders?store=${store}`);
    await noPII(page);
  }
});

test("MOU03 controlled delayed detail, pagehide, history and cross-tab logout", async ({
  page,
  context,
}, testInfo) => {
  await signedLogin(page);
  await expand(page, ids.captured);
  await expect(page.getByTestId("order-detail")).toContainText(
    "Synthetic Buyer",
  );
  let releaseOld!: () => void;
  const oldGate = new Promise<void>((resolve) => {
    releaseOld = resolve;
  });
  let intercepted!: () => void;
  const oldSeen = new Promise<void>((resolve) => {
    intercepted = resolve;
  });
  await page.route(
    `**/api/stores/${store}/orders/${ids.authorized}`,
    async (route) => {
      const real = await route.fetch();
      intercepted();
      await oldGate;
      await route.fulfill({ response: real });
    },
  );
  const old = page.getByTestId(`order-expand-${ids.authorized}`);
  await old.click();
  await oldSeen;
  // Selection change must invalidate the pending older result immediately.
  await page.getByTestId(`order-expand-${ids.captured}`).click();
  await expect(page.getByTestId("order-detail")).toContainText(ids.captured);
  releaseOld();
  await expect(page.getByTestId("order-detail")).not.toContainText(
    ids.authorized,
  );
  const clearedOnHide = await page.evaluate(() => {
    window.dispatchEvent(
      new PageTransitionEvent("pagehide", { persisted: true }),
    );
    return !document.body.textContent?.includes("Synthetic Buyer");
  });
  expect(clearedOnHide).toBe(true);
  const reappear = await page.getByTestId("order-detail").count();
  expect(reappear).toBe(0);
  await page.evaluate(() => {
    window.addEventListener("pageshow", (event) =>
      sessionStorage.setItem("mou-native-pageshow", String(event.persisted)),
    );
  });
  await page.goto("/en/settings");
  await page.goBack();
  const nativePersisted = await page.evaluate(() =>
    sessionStorage.getItem("mou-native-pageshow"),
  );
  await testInfo.attach("native-pageshow.json", {
    body: JSON.stringify({
      observed: nativePersisted !== null,
      persisted: nativePersisted === "true",
    }),
    contentType: "application/json",
  });
  await expect(page.getByTestId("merchant-orders")).toBeVisible();
  const otherTab = await context.newPage();
  await otherTab.goto("/en/");
  await otherTab.getByRole("button", { name: "Sign out" }).click();
  await expect(page.getByTestId("order-detail")).toHaveCount(0);
  await noPII(page);
  await otherTab.close();
});

test("MOU03 labeled fault injection: invalid DTO, non-JSON and network failure recover via real retry", async ({
  page,
}) => {
  await signedLogin(page);
  await expand(page, ids.captured);
  await expect(page.getByTestId("order-detail")).toContainText(
    "Synthetic Buyer",
  );
  const path = `**/api/stores/${store}/orders?*`;
  for (const fault of ["non-json", "invalid-dto", "network"] as const) {
    let fired = false;
    const handler = async (route: import("@playwright/test").Route) => {
      if (fired) return route.continue();
      fired = true;
      if (fault === "network") return route.abort("failed");
      return route.fulfill({
        status: 200,
        contentType: fault === "non-json" ? "text/html" : "application/json",
        body:
          fault === "non-json"
            ? "<html>unavailable</html>"
            : '{"items":[{"order_id":"bad"}],"next_cursor":""}',
      });
    };
    await page.route(path, handler);
    await page
      .getByTestId("state-filter")
      .selectOption(
        fault === "non-json"
          ? "DRAFT"
          : fault === "invalid-dto"
            ? "CANCELLED"
            : "CONFIRMED",
      );
    await expect.poll(() => fired).toBe(true);
    await noPII(page);
    await page.unroute(path, handler);
    await page.getByTestId("orders-refresh").click();
    await expect(page.getByTestId("orders-table")).toBeVisible();
  }
});

test("MOU05 approved inline comp at desktop/mobile in three locales and page-two locale context", async ({
  page,
}, testInfo) => {
  await signedLogin(page);
  await page.getByTestId("orders-next").click();
  await expand(page, ids.draft0);
  for (const locale of ["en", "zh-CN", "zh-TW"] as const) {
    await page.getByTestId("locale-switch").selectOption(locale);
    await expect(page).toHaveURL(new RegExp(`/${locale}/orders`));
    await expect(page).toHaveURL(new RegExp(`order=${ids.draft0}`));
    await expect(page).toHaveURL(/cursor=/);
    await expect(page.getByTestId("order-detail")).toContainText(ids.draft0);
    await page.setViewportSize({ width: 1586, height: 992 });
    await screenshot(
      page,
      testInfo.outputPath(`inline-${locale}-1586x992.png`),
    );
    await page.setViewportSize({ width: 390, height: 844 });
    await screenshot(page, testInfo.outputPath(`inline-${locale}-390x844.png`));
    const targets = await page
      .getByTestId("merchant-orders")
      .locator("button,select")
      .evaluateAll((items) =>
        items
          .filter((item) => getComputedStyle(item).display !== "none")
          .map((item) => {
            const rect = item.getBoundingClientRect();
            return {
              width: rect.width,
              height: rect.height,
              text: item.textContent ?? "",
            };
          }),
      );
    expect(
      targets
        .filter((target) => target.width > 0)
        .every((target) => target.height >= 44),
    ).toBe(true);
  }
});
