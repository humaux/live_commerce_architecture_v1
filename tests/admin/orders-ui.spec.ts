import { expect, test, type BrowserContext, type Page } from "@playwright/test";
import { writeFile } from "node:fs/promises";
import * as http from "node:http";

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
  headless: false,
  trace: "retain-on-failure",
  screenshot: "only-on-failure",
});

async function signedLogin(page: Page) {
  await page.goto("/en/");
  await page
    .getByRole("button", { name: "Sign in with identity service" })
    .click();
  await expect(page.getByTestId("nav-orders")).toBeVisible();
  const before = await detailCalls(page);
  await page.getByTestId("nav-orders").click();
  await expect(page.getByTestId("merchant-orders")).toBeVisible();
  await expect(page).toHaveURL(new RegExp(`/en/orders`));
  const selector = page.getByTestId("store-selector");
  if ((await selector.inputValue()) !== store)
    await selector.selectOption(store);
  await expect(page.getByTestId("orders-table")).toBeVisible();
  expect(await detailCalls(page)).toBe(before);
}

async function session(context: BrowserContext, token: string) {
  await context.addCookies([
    {
      name: cookieName,
      value: token,
      url: origin.replace(/^http:/, "https:"),
      secure: true,
      httpOnly: true,
      sameSite: "Lax",
    },
  ]);
}

async function browserRead(page: Page, path: string) {
  return page.evaluate(async (target) => {
    const response = await fetch(target, {
      credentials: "same-origin",
      cache: "no-store",
    });
    return {
      status: response.status,
      cache: response.headers.get("cache-control"),
      body: await response.text(),
    };
  }, path);
}

async function raw(path: string, token: string) {
  const target = new URL(origin);
  return new Promise<{
    status: number;
    cache: string | undefined;
    cookies: string[];
    body: string;
  }>((resolve, reject) => {
    const request = http.request(
      {
        hostname: target.hostname,
        port: target.port,
        path,
        method: "GET",
        headers: { Cookie: `${cookieName}=${token}` },
        timeout: 5000,
      },
      (response) => {
        const chunks: Buffer[] = [];
        response.on("data", (chunk: Buffer) => chunks.push(chunk));
        response.on("end", () =>
          resolve({
            status: response.statusCode ?? 0,
            cache: response.headers["cache-control"],
            cookies: response.headers["set-cookie"] ?? [],
            body: Buffer.concat(chunks).toString("utf8"),
          }),
        );
      },
    );
    request.on("error", reject);
    request.on("timeout", () =>
      request.destroy(new Error("raw request timed out")),
    );
    request.end();
  });
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
  await expect(page.getByTestId("orders-table")).toBeVisible();
  if (
    !(await page.getByTestId(`order-expand-${id}`).count()) &&
    (await page.getByTestId("orders-previous").isEnabled())
  ) {
    await page.getByTestId("orders-previous").click();
    await expect(page).not.toHaveURL(/cursor=/);
    await expect(page.getByTestId("orders-table")).toBeVisible();
  }
  for (let p = 0; p < 3; p++) {
    if (await page.getByTestId(`order-expand-${id}`).count()) {
      const button = page.getByTestId(`order-expand-${id}`);
      if ((await button.getAttribute("aria-expanded")) !== "true")
        await button.click();
      await expect(page.getByTestId("order-detail")).toHaveAttribute(
        "aria-label",
        new RegExp(id),
      );
      return;
    }
    await expect(page.getByTestId("orders-next")).toBeEnabled();
    const previousURL = page.url();
    await page.getByTestId("orders-next").click();
    await expect(page).not.toHaveURL(previousURL);
    await expect(page.getByTestId("orders-table")).toBeVisible();
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
  const list = await browserRead(
    page,
    `/api/stores/${store}/orders?limit=10&state=all`,
  );
  expect(list.status).toBe(200);
  expect(list.cache).toBe("private, no-store");
  const body = JSON.parse(list.body) as {
    items: Array<{ order_id: string }>;
    next_cursor: string;
  };
  expect(body.items).toHaveLength(10);
  expect(body.next_cursor).toMatch(/^[A-Za-z0-9_-]{1,1024}$/);
  await expect(page.getByTestId("orders-table")).toBeVisible();
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
    await test.step(mode, async () => {
      await expand(page, ids[mode]);
      const detail = page.getByTestId("order-detail");
      for (const state of expected)
        await expect(
          detail.locator(`[data-state="${state}"]`).first(),
        ).toBeVisible();
      await expect(detail.locator(".orders-grand dd")).toContainText(
        mode === "expired" ? "12.50" : "25.00",
      );
      await expect(detail).toContainText("Synthetic home address");
      await noPersistentOrderBody(page);
    });
  }
  const detail = await browserRead(
    page,
    `/api/stores/${store}/orders/${ids.pending}`,
  );
  expect(detail.cache).toBe("private, no-store");
  expect(detail.status).toBe(200);
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
  const missing = await browserRead(
    page,
    `/api/stores/${store}/orders/${foreignOrder}`,
  );
  expect(missing.status).toBe(404);
  expect(missing.body).not.toContain("Synthetic Buyer");
  await page.getByTestId("store-selector").selectOption(foreignStore);
  await expect(page).toHaveURL(new RegExp(`store=${foreignStore}`));
  await expand(page, foreignOrder);
  await expect(page.getByTestId("order-detail")).toContainText(
    "Synthetic Buyer",
  );
  const crossStore = await browserRead(
    page,
    `/api/stores/${foreignStore}/orders/${ids.pending}`,
  );
  expect(crossStore.status).toBe(404);
  expect(crossStore.body).not.toContain("Synthetic Buyer");
  for (const badStore of [unlistedStore, "not-a-uuid"]) {
    await page.goto(`/en/orders?store=${badStore}`);
    await noPII(page);
    await expect(page.getByTestId("orders-table")).toHaveCount(0);
  }
  await session(context, secondToken);
  await page.goto(`/en/orders?store=${store}`);
  await noPII(page);
  expect((await raw(`/api/stores/${store}/orders`, secondToken)).status).toBe(
    403,
  );
  await page.goto(`/en/orders?store=${foreignStore}`);
  await expand(page, foreignOrder);
  await session(context, noOrdersToken);
  await page.goto(`/en/orders?store=${store}`);
  await noPII(page);
  expect((await raw(`/api/stores/${store}/orders`, noOrdersToken)).status).toBe(
    403,
  );
  for (const token of [expiredToken, revokedToken, "not-a-session"]) {
    await session(context, token);
    const response = await raw(`/api/stores/${store}/orders`, token);
    expect(response.status).toBe(401);
    expect(response.cache).toBe("no-store");
    expect(response.cookies.join(" ")).toContain(`${cookieName}=`);
    expect(response.cookies.join(" ")).toContain("Max-Age=0");
    await page.goto(`/en/orders?store=${store}`);
    await noPII(page);
  }
});

test("MOU03 controlled delayed detail, pagehide, history and cross-tab logout", async ({
  page,
  context,
}, testInfo) => {
  await context.addInitScript(() => {
    window.addEventListener("pageshow", (event) => {
      const log = JSON.parse(
        sessionStorage.getItem("mou-native-pageshows") ?? "[]",
      ) as Array<{ path: string; persisted: boolean }>;
      log.push({ path: location.pathname, persisted: event.persisted });
      sessionStorage.setItem("mou-native-pageshows", JSON.stringify(log));
    });
  });
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
  await expect(page.getByTestId("order-detail")).toHaveAttribute(
    "aria-label",
    new RegExp(ids.captured),
  );
  releaseOld();
  await expect(page.getByTestId("order-detail")).not.toHaveAttribute(
    "aria-label",
    new RegExp(ids.authorized),
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
  const beforeHistory = await page.evaluate(
    () =>
      JSON.parse(sessionStorage.getItem("mou-native-pageshows") ?? "[]")
        .length as number,
  );
  await page.goto("/en/settings");
  await page.goBack();
  const nativeEvents = await page.evaluate(
    (before) =>
      (
        JSON.parse(
          sessionStorage.getItem("mou-native-pageshows") ?? "[]",
        ) as Array<{ path: string; persisted: boolean }>
      ).slice(before),
    beforeHistory,
  );
  const returnEvent = nativeEvents.find((event) => event.path === "/en/orders");
  expect(returnEvent).toBeDefined();
  // The list reporter does not persist body-only attachments for passing tests.
  // Keep the actual native observation even when this scenario is green.
  const nativeHistoryPath = testInfo.outputPath("native-pageshow.json");
  await writeFile(
    nativeHistoryPath,
    JSON.stringify({
      observed: Boolean(returnEvent),
      persisted: returnEvent?.persisted ?? false,
      events: nativeEvents,
    }),
  );
  await testInfo.attach("native-pageshow.json", {
    path: nativeHistoryPath,
    contentType: "application/json",
  });
  await expect(page.getByTestId("merchant-orders")).toBeVisible();
  const otherTab = await context.newPage();
  await otherTab.goto("/en/");
  await page.bringToFront();
  await expand(page, ids.captured);
  await expect(page.getByTestId("order-detail")).toContainText(
    "Synthetic Buyer",
  );
  await otherTab
    .getByTestId("workspace-sign-out")
    .evaluate((button: HTMLElement) => button.click());
  await expect(page.getByTestId("order-detail")).toHaveCount(0);
  await noPII(page);
  await otherTab.close();
});

test("MOU03 actual visibility hide clears PII and visible return needs fresh authorized detail", async ({
  page,
  context,
}) => {
  test.skip(
    process.env.LC_BROWSER_NATIVE_VISIBILITY !== "1",
    "NOT_RUN: this host's Chromium keeps document.visibilityState visible across headed tab switches and minimized windows",
  );
  await signedLogin(page);
  await expand(page, ids.captured);
  await expect(page.getByTestId("order-detail")).toContainText(
    "Synthetic Buyer",
  );
  const devtools = await context.newCDPSession(page);
  // Playwright forces focus in every tab; release that test-only emulation so
  // the headed browser emits an actual visibilitychange on tab switch.
  await devtools.send("Emulation.setFocusEmulationEnabled", { enabled: false });
  const cover = await context.newPage();
  await cover.goto("/en/settings");
  await cover.bringToFront();
  await expect
    .poll(() => page.evaluate(() => document.visibilityState))
    .toBe("hidden");
  await noPII(page);
  let release!: () => void;
  let seen!: () => void;
  const gate = new Promise<void>((resolve) => {
    release = resolve;
  });
  const intercepted = new Promise<void>((resolve) => {
    seen = resolve;
  });
  await page.route(
    `**/api/stores/${store}/orders/${ids.captured}`,
    async (route) => {
      const real = await route.fetch();
      seen();
      await gate;
      await route.fulfill({ response: real });
    },
  );
  await page.bringToFront();
  await intercepted;
  await noPII(page);
  release();
  await expect(page.getByTestId("order-detail")).toContainText(
    "Synthetic Buyer",
  );
  await cover.close();
  await devtools.detach();
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
        headers: { "cache-control": "private, no-store" },
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

test("MOU03 delayed old success/error cannot repaint store, filter, locale or new session", async ({
  page,
  context,
}) => {
  await signedLogin(page);
  async function delayed(path: string, fail: boolean) {
    let release!: () => void;
    let seen!: () => void;
    const gate = new Promise<void>((resolve) => {
      release = resolve;
    });
    const intercepted = new Promise<void>((resolve) => {
      seen = resolve;
    });
    let used = false;
    const handler = async (route: import("@playwright/test").Route) => {
      if (used) return route.continue();
      used = true;
      const real = await route.fetch();
      seen();
      await gate;
      try {
        if (fail)
          await route.fulfill({
            status: 503,
            contentType: "application/json",
            headers: { "cache-control": "private, no-store" },
            body: '{"code":"retry_later","details":{}}',
          });
        else await route.fulfill({ response: real });
      } catch {
        /* A canceled old request may already be gone. */
      }
    };
    await page.route(path, handler);
    return { intercepted, release, remove: () => page.unroute(path, handler) };
  }
  const oldStore = await delayed(`**/api/stores/${store}/orders?*`, false);
  await page.getByTestId("state-filter").selectOption("DRAFT");
  await oldStore.intercepted;
  await page.getByTestId("store-selector").selectOption(foreignStore);
  await expect(page.getByTestId(`order-row-${foreignOrder}`)).toBeVisible();
  oldStore.release();
  await oldStore.remove();
  await expect(page.getByTestId(`order-row-${ids.draft0}`)).toHaveCount(0);
  const oldFilter = await delayed(
    `**/api/stores/${foreignStore}/orders?*`,
    true,
  );
  await page.getByTestId("state-filter").selectOption("CONFIRMED");
  await oldFilter.intercepted;
  await page.getByTestId("state-filter").selectOption("CANCELLED");
  await expect(page.getByTestId("merchant-orders")).toBeVisible();
  oldFilter.release();
  await oldFilter.remove();
  await expect(page).toHaveURL(/state=CANCELLED/);
  const oldLocale = await delayed(
    `**/api/stores/${foreignStore}/orders?*`,
    false,
  );
  await page.getByTestId("state-filter").selectOption("all");
  await oldLocale.intercepted;
  await page.getByTestId("locale-switch").selectOption("zh-CN");
  await expect(page).toHaveURL(/\/zh-CN\/orders/);
  await expect(page.getByTestId(`order-row-${foreignOrder}`)).toBeVisible();
  oldLocale.release();
  await oldLocale.remove();
  await expect(page.getByTestId(`order-row-${foreignOrder}`)).toBeVisible();
  const oldSession = await delayed(
    `**/api/stores/${foreignStore}/orders/${foreignOrder}`,
    false,
  );
  await page.getByTestId(`order-expand-${foreignOrder}`).click();
  await oldSession.intercepted;
  await session(context, noOrdersToken);
  await page.evaluate(() => window.dispatchEvent(new Event("focus")));
  oldSession.release();
  await oldSession.remove();
  await noPII(page);
});

test("MOU05 approved inline comp at desktop/mobile in three locales and page-two locale context", async ({
  page,
}, testInfo) => {
  await signedLogin(page);
  await expand(page, ids.pickup); // Native comp: first row, immediately expanded.
  for (const locale of ["en", "zh-CN", "zh-TW"] as const) {
    await page.getByTestId("locale-switch").selectOption(locale);
    await expect(page).toHaveURL(new RegExp(`/${locale}/orders`));
    await expect(page).toHaveURL(new RegExp(`order=${ids.pickup}`));
    await expect(page.getByTestId("order-detail")).toHaveAttribute(
      "aria-label",
      new RegExp(ids.pickup),
    );
    await page.setViewportSize({ width: 1586, height: 992 });
    await page.evaluate(() =>
      Promise.all(
        document
          .getAnimations()
          .map((animation) => animation.finished.catch(() => undefined)),
      ),
    );
    const cells = await page.getByTestId("order-detail").evaluate((detail) => {
      const product = detail.querySelector(
        ".orders-items tbody td:first-child",
      );
      const price = detail.querySelector(".orders-items tbody td:nth-child(2)");
      if (!product || !price) throw new Error("missing frozen item cells");
      return {
        productRight: product.getBoundingClientRect().right,
        priceLeft: price.getBoundingClientRect().left,
        codeRight:
          product.querySelector("small")?.getBoundingClientRect().right ?? 0,
        nameRight:
          product.querySelector("strong")?.getBoundingClientRect().right ?? 0,
      };
    });
    expect(cells.productRight).toBeLessThanOrEqual(cells.priceLeft + 1);
    expect(cells.codeRight).toBeLessThanOrEqual(cells.productRight + 1);
    expect(cells.nameRight).toBeLessThanOrEqual(cells.productRight + 1);
    await screenshot(
      page,
      testInfo.outputPath(`inline-${locale}-1586x992.png`),
    );
    await page.setViewportSize({ width: 390, height: 844 });
    await page.evaluate(() =>
      Promise.all(
        document
          .getAnimations()
          .map((animation) => animation.finished.catch(() => undefined)),
      ),
    );
    const rail = await page.locator(".rail").evaluate((element) => ({
      open: element.classList.contains("open"),
      right: element.getBoundingClientRect().right,
    }));
    expect(rail.open).toBe(false);
    expect(rail.right).toBeLessThanOrEqual(1);
    const mobileDetail = await page
      .getByTestId("order-detail")
      .evaluate((element) => ({
        width: element.getBoundingClientRect().width,
        recipientWidth:
          element.querySelector(".orders-recipient dd")?.getBoundingClientRect()
            .width ?? 0,
      }));
    expect(mobileDetail.width).toBeGreaterThanOrEqual(320);
    expect(mobileDetail.recipientWidth).toBeGreaterThanOrEqual(120);
    await page.evaluate(() => window.scrollTo(0, 0));
    await screenshot(page, testInfo.outputPath(`inline-${locale}-390x844.png`));
    await page
      .getByTestId("order-detail")
      .evaluate((element) => element.scrollIntoView({ block: "start" }));
    await screenshot(
      page,
      testInfo.outputPath(`inline-detail-${locale}-390x844.png`),
    );
    await page.screenshot({
      path: testInfo.outputPath(`inline-full-${locale}-390.png`),
      fullPage: true,
    });
    await page.getByTestId("state-filter").focus();
    await page.keyboard.press("Tab");
    await expect(page.getByTestId("orders-refresh")).toBeFocused();
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
  await page.setViewportSize({ width: 1586, height: 992 });
  await page.getByTestId("orders-next").click();
  await expand(page, ids.draft0);
  for (const locale of ["en", "zh-CN", "zh-TW"] as const) {
    await page.getByTestId("locale-switch").selectOption(locale);
    await expect(page).toHaveURL(new RegExp(`/${locale}/orders`));
    await expect(page).toHaveURL(new RegExp(`order=${ids.draft0}`));
    await expect(page).toHaveURL(/cursor=/);
    await expect(page.getByTestId("order-detail")).toHaveAttribute(
      "aria-label",
      new RegExp(ids.draft0),
    );
  }
});
