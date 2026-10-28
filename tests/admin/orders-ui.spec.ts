// Purpose: Exercises orders, filters and details with supplied isolated browser identities.
// Depends on: @playwright/test, node:fs/promises, node:path, node:http, ./fixtures/native-device; harness env: LC_BROWSER_PUBLIC_ORIGIN, LC_BROWSER_API_ORIGIN, LC_BROWSER_EVIDENCE, LC_BROWSER_ORDER_STORE, LC_BROWSER_ORDER_IDS, LC_BROWSER_FROZEN_SKU_CODE, LC_BROWSER_FOREIGN_STORE, LC_BROWSER_FOREIGN_ORDER_ID, LC_BROWSER_UNLISTED_STORE, LC_BROWSER_SECOND_TOKEN, LC_BROWSER_NO_ORDERS_TOKEN, LC_BROWSER_EXPIRED_TOKEN, LC_BROWSER_REVOKED_TOKEN
// Used by: apps/admin/src/features/orders/routes.ts, scripts/dev/test-local.sh, tests/foundation/browser_merchant_orders_ui_test.go
// Invariants: ORDERS-V2-IMPL uses real clicks and response-derived identities/counts; shipment writes are explicitly MOCK,
//   never reach PG, and preserve this harness's read-only fingerprints. Existing MOU assertions remain unchanged.
import { expect, test, type BrowserContext, type Page } from "@playwright/test";
import { mkdir, writeFile } from "node:fs/promises";
import { resolve } from "node:path";
import * as http from "node:http";
import { nativePage } from "./fixtures/native-device";
import {picklistCopy} from '../../apps/admin/lib/picklist-copy';

const required = (name: string) => {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
};
const origin = required("LC_BROWSER_PUBLIC_ORIGIN");
const apiOrigin = required("LC_BROWSER_API_ORIGIN");
const evidence = required("LC_BROWSER_EVIDENCE");
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
const v2Evidence = resolve(evidence, "../../../orders-v2-fix");

test.use({
  baseURL: origin,
  headless: false,
  // Playwright disables native bfcache by default. This lifecycle suite must
  // exercise browser restoration, not force every history return to reload.
  launchOptions: { ignoreDefaultArgs: ["--disable-back-forward-cache"] },
  trace: "retain-on-failure",
  screenshot: "only-on-failure",
});

async function signedLogin(page: Page) {
  await page.goto(new URL("/en/", origin).toString());
  await page
    .getByRole("button", { name: "Sign in with identity service" })
    .click();
  await expect(page.getByTestId("nav-orders")).toBeVisible();
  const before = await detailCalls(page);
  await page.getByTestId("nav-orders").click();
  await expect(page.getByTestId("merchant-orders")).toBeVisible();
  await expect(page).toHaveURL(new RegExp(`/en/orders`));
  const selector = page.getByTestId("shell-store-selector");
  if ((await selector.inputValue()) !== store)
    await switchOrderStore(page, store);
  await expect(page.getByTestId("orders-table")).toBeVisible();
  // These frozen MOU scenarios explicitly inspect drafts. v2's default is tested
  // separately; choose the all-states inspection scope via the actual control.
  await revealOrderFilters(page);
  await page.getByTestId("state-filter").selectOption("all");
  await expect(page).toHaveURL(/state=all/);
  await expect(page.getByTestId("orders-table")).toBeVisible();
  expect(await detailCalls(page)).toBe(before);
}

// Filters are collapsed on every viewport. Exercise their real disclosure before
// using an existing filter; all state, privacy and response assertions stay intact.
async function revealOrderFilters(page: Page) {
  const toggle = page.getByTestId("orders-more-filters");
  if (await toggle.getAttribute("aria-expanded") === "false") await toggle.click();
}

async function switchOrderStore(page: Page, next: string) {
  // W0 owns store navigation: a real switch goes through overview, not a page-local setter.
  await page.getByTestId("shell-store-selector").selectOption(next);
  await expect(page).toHaveURL(new URL(`/en?store=${next}`, origin).href);
  await expect(page.getByTestId("merchant-orders")).toHaveCount(0);
  // This fixture's role has one Orders route: wait for the singleton navigation
  // after the full reload instead of branching on a pre-hydration snapshot.
  await expect(page.getByTestId("nav-orders")).toBeVisible();
  await page.getByTestId("nav-orders").click();
  await expect(page).toHaveURL(new RegExp(`/en/orders\\?store=${next}$`));
  await expect(page.getByTestId("shell-store-selector")).toHaveValue(next);
  await expect(page.getByTestId("store-selector")).toHaveCount(0);
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
  // G-UI8 audit [READ/MEASURE]: same-origin GET read of server state through the BFF (no state change)
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
  // G-UI8 audit [READ/MEASURE]: scans client storage for secrets/PII (read only) + CacheStorage names
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
  await page.screenshot({ path: file, fullPage: false, animations: "disabled" });
  // G-UI8 audit [READ/MEASURE]: measures horizontal overflow (layout read, no state change)
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
      // stop-bleed D02: a whole TWD amount reads "NT$25" (no ".00"); an amount with cents keeps them ("NT$12.50")
      await expect(detail.locator(".orders-grand dd")).toContainText(
        mode === "expired" ? /NT\$12\.50/ : /NT\$25(?![\d.,])/,
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
  await switchOrderStore(page, foreignStore);
  await expect(page).toHaveURL(new RegExp(`store=${foreignStore}`));
  await revealOrderFilters(page);
  await page.getByTestId("state-filter").selectOption("all");
  await expect(page.getByTestId(`order-row-${ids.pending}`)).toHaveCount(0);
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
  await revealOrderFilters(page);
  await page.getByTestId("state-filter").selectOption("all"); // explicit access to the foreign-store draft, only with its authorized principal
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
  // G-UI8 audit [EXTERNAL-MOCK]: injects a pagehide(persisted) lifecycle event: the default headless page cannot enter the bfcache by input; the real goto/goBack path follows and MOU03 uses the native device
  const clearedOnHide = await page.evaluate(() => {
    // G-UI8 audit [EXTERNAL-MOCK]: dispatchEvent(pagehide) of the injected lifecycle event above
    window.dispatchEvent(
      new PageTransitionEvent("pagehide", { persisted: true }),
    );
    return !document.body.textContent?.includes("Synthetic Buyer");
  });
  expect(clearedOnHide).toBe(true);
  const reappear = await page.getByTestId("order-detail").count();
  expect(reappear).toBe(0);
  // G-UI8 audit [READ/MEASURE]: reads the native pageshow journal length
  const beforeHistory = await page.evaluate(
    () =>
      JSON.parse(sessionStorage.getItem("mou-native-pageshows") ?? "[]")
        .length as number,
  );
  await page.goto("/en/settings");
  // A restored document does not emit a new load event.
  await page.goBack({ waitUntil: "commit" });
  // Commit precedes pageshow on both cached and freshly loaded returns.
  // goBack(commit) can leave the first evaluate in the document that is being replaced ("Execution context was destroyed", ~1 run
  // in 3 under machine load), and expect.poll does not retry a thrown evaluate. toPass does; the assertion itself is unchanged:
  // a pageshow for /en/orders must have been logged after the history length recorded before leaving.
  await expect(async () => {
    expect(
      // G-UI8 audit [READ/MEASURE]: reads the native pageshow journal
      await page.evaluate(
        (before) =>
          (
            JSON.parse(sessionStorage.getItem("mou-native-pageshows") ?? "[]") as
              Array<{ path: string; persisted: boolean }>
          ).slice(before).some((event) => event.path === "/en/orders"),
        beforeHistory,
      ),
    ).toBe(true);
  }).toPass({ timeout: 10_000 });
  // G-UI8 audit [READ/MEASURE]: reads the native pageshow journal
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
  // G-UI8 audit [READ/MEASURE]: reads Navigation Timing notRestoredReasons
  const notRestoredReasons = await page.evaluate(() => {
    const navigation = performance.getEntriesByType("navigation")[0] as
      PerformanceNavigationTiming & {
        notRestoredReasons?: { toJSON(): unknown } | null;
      };
    return navigation?.notRestoredReasons?.toJSON() ?? null;
  });
  // The list reporter does not persist body-only attachments for passing tests.
  // Keep the actual native observation even when this scenario is green.
  const nativeHistoryPath = testInfo.outputPath("native-pageshow.json");
  await writeFile(
    nativeHistoryPath,
    JSON.stringify({
      observed: Boolean(returnEvent),
      persisted: returnEvent?.persisted ?? false,
      notRestoredReasons,
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
  // The user signs out in the other tab by the real path: Account menu -> Sign out (the button lives inside the closed Account disclosure).
  await otherTab.locator("header[data-shell-topbar] summary", { hasText: /^Account$/ }).click();
  await otherTab.getByTestId("workspace-sign-out").click();
  await expect(page.getByTestId("order-detail")).toHaveCount(0);
  await noPII(page);
  // signOut() ends with location.replace("/en/"); closing mid-navigation hung MOU03 (#39, #44 CI). Wait for the
  // signed-out landing to render before closing the tab.
  await expect(otherTab.getByRole("button", { name: "Sign in with identity service" })).toBeVisible();
  await otherTab.close();
});

test("MOU03 actual visibility hide clears PII and visible return needs fresh authorized detail", async () => {
  const { page, close } = await nativePage(evidence, "orders-native-profile-");
  let cover: Page | undefined;
  let release = () => {};
  try {
    await signedLogin(page);
    await expand(page, ids.captured);
    await expect(page.getByTestId("order-detail")).toContainText("Synthetic Buyer");
    const beforeHide = await detailCalls(page);
    await page.bringToFront();
    // G-UI8 audit [READ/MEASURE]: reads document.visibilityState (the tab switch itself is native: bringToFront on the native device)
    await expect.poll(() => page.evaluate(() => document.visibilityState)).toBe("visible");
    // G-UI8 audit [READ/MEASURE]: installs a read-only visibilitychange recorder (isTrusted evidence)
    await page.evaluate(() => {
      const observed = window as typeof window & { mouVisibility?: { state: string; trusted: boolean }[] };
      observed.mouVisibility = [];
      document.addEventListener("visibilitychange", (event) =>
        observed.mouVisibility?.push({ state: document.visibilityState, trusted: event.isTrusted }));
    });
    cover = await page.context().newPage();
    await cover.goto(new URL("/en/settings", origin).toString());
    await cover.bringToFront();
    // G-UI8 audit [READ/MEASURE]: reads document.visibilityState
    await expect.poll(() => page.evaluate(() => document.visibilityState)).toBe("hidden");
    await noPII(page);
    const whileHidden = await detailCalls(page);
    expect(whileHidden).toBe(beforeHide);
    let seen!: () => void;
    const gate = new Promise<void>((resolve) => { release = resolve; });
    const intercepted = new Promise<void>((resolve) => { seen = resolve; });
    await page.route(`**/api/stores/${store}/orders/${ids.captured}`, async (route) => {
      const real = await route.fetch();
      seen();
      await gate;
      await route.fulfill({ response: real });
    });
    await page.bringToFront();
    // G-UI8 audit [READ/MEASURE]: reads document.visibilityState
    await expect.poll(() => page.evaluate(() => document.visibilityState)).toBe("visible");
    await intercepted;
    await noPII(page);
    release();
    await expect(page.getByTestId("order-detail")).toContainText("Synthetic Buyer");
    const afterReturn = await detailCalls(page);
    expect(afterReturn).toBeGreaterThan(whileHidden);
    // G-UI8 audit [READ/MEASURE]: reads the recorded native visibility events
    const events = await page.evaluate(() =>
      (window as typeof window & { mouVisibility?: { state: string; trusted: boolean }[] }).mouVisibility);
    expect(events).toEqual([{ state: "hidden", trusted: true }, { state: "visible", trusted: true }]);
    await writeFile(`${evidence}/native-visibility.json`, JSON.stringify({
      events, beforeHide, whileHidden, afterReturn, piiCleared: true, revalidated: true,
    }), { mode: 0o600 });
  } finally {
    release();
    await cover?.close().catch(() => {});
    await close();
  }
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
    await revealOrderFilters(page);
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

test("MOU03 delayed old success/error cannot repaint filter, locale or new session; shell store switch cancels the in-flight old-store read", async ({
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
  // This subcase proves only that the shell store switch (a full navigation) cancels the
  // in-flight old-store read: the late response is never observed repainting. It does NOT
  // prove the in-page generation fence for a store change, because a store can no longer
  // change in-page. The filter/locale cases below exercise the late-response generation fences.
  await revealOrderFilters(page);
  await page.getByTestId("state-filter").selectOption("DRAFT");
  await oldStore.intercepted;
  await switchOrderStore(page, foreignStore);
  await revealOrderFilters(page);
  await page.getByTestId("state-filter").selectOption("DRAFT");
  await expect(page.getByTestId(`order-row-${foreignOrder}`)).toBeVisible();
  oldStore.release();
  await oldStore.remove();
  await expect(page.getByTestId(`order-row-${ids.draft0}`)).toHaveCount(0);
  const oldFilter = await delayed(
    `**/api/stores/${foreignStore}/orders?*`,
    true,
  );
  await revealOrderFilters(page);
  await page.getByTestId("state-filter").selectOption("CONFIRMED");
  await oldFilter.intercepted;
  await revealOrderFilters(page);
  await page.getByTestId("state-filter").selectOption("CANCELLED");
  await expect(page.getByTestId("merchant-orders")).toBeVisible();
  oldFilter.release();
  await oldFilter.remove();
  await expect(page).toHaveURL(/state=CANCELLED/);
  const oldLocale = await delayed(
    `**/api/stores/${foreignStore}/orders?*`,
    false,
  );
  await revealOrderFilters(page);
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
  // G-UI8 audit [EXTERNAL-MOCK]: injects a window focus event after the session cookie changed: the default headless page cannot receive OS focus; native visibility/focus returns are covered by MOU03
  await page.evaluate(() => window.dispatchEvent(new Event("focus")));
  oldSession.release();
  await oldSession.remove();
  await noPII(page);
});

async function paymentBadgesFit(page: Page) {
  // Global fixed-table column rules must never hide "authorized, not captured".
  const badges = await page
    .locator("[data-testid=order-payment-cell] .orders-badge")
    .evaluateAll((elements) => elements.map((element) => {
      const cell = element.closest("td")!.getBoundingClientRect();
      const badge = element.getBoundingClientRect();
      return {
        withinCell: badge.left >= cell.left - 1 && badge.right <= cell.right + 1,
        textFits: element.scrollWidth <= element.clientWidth + 1,
        label: element.textContent,
      };
    }));
  expect(badges.length).toBeGreaterThan(0);
  expect(badges.filter((badge) => !badge.withinCell || !badge.textFits)).toEqual([]);
}

test("MOU07 v2 private search, SQL queues, filters and three-language ledger", async ({ page }) => {
  test.setTimeout(120_000);
  await mkdir(v2Evidence, { recursive: true });
  await signedLogin(page);
  const clicks: Array<{control:string; result:string}> = [];
  await switchOrderStore(page, foreignStore);
  await revealOrderFilters(page);
  await page.getByTestId("state-filter").selectOption("all");
  await expect(page.getByTestId(`order-row-${foreignOrder}`)).toBeVisible();
  await expect(page.getByTestId(`order-row-${ids.pending}`)).toHaveCount(0);
  await page.reload();
  await expect(page.getByTestId(`order-row-${foreignOrder}`)).toBeVisible();
  await expect(page.getByTestId(`order-row-${ids.pending}`)).toHaveCount(0);
  await switchOrderStore(page, store);
  await expect(page.getByTestId(`order-row-${ids.pending}`)).toBeVisible();
  await expect(page.getByTestId(`order-row-${foreignOrder}`)).toHaveCount(0);
  await revealOrderFilters(page);
  await page.getByTestId("state-filter").selectOption("all");
  clicks.push({control:"shell store A → B → reload B → A", result:"real shell selection + Orders navigation; selected store persisted; other store rows absent in both directions"});
  async function apply(control: string, action: () => Promise<unknown>) {
    // Filters stay mounted during loading so late-response tests can change
    // intent. The SQL readback must first await the current read, not capture
    // that older response as if it belonged to the next click.
    await expect(page.getByTestId("orders-table")).toBeVisible();
    const response = page.waitForResponse(r => r.url().includes(`/api/stores/${store}/orders`) && r.url().includes("view=v2")).then(async r => { expect(r.status(), control).toBe(200); expect(r.headers()["cache-control"]).toBe("private, no-store"); return r.json(); });
    await action();
    const data = await response;
    await expect(page.getByTestId("orders-total")).toHaveText(`Matching orders: ${data.total}`);
    for (const [bucket, count] of Object.entries(data.counts)) await expect(page.getByTestId(`orders-count-${bucket}`)).toHaveText(String(count));
    clicks.push({ control, result: `visible SQL total ${data.total}; counts match response` });
    return data;
  }
  await revealOrderFilters(page);
  await apply("explicit cancelled state", () => page.getByTestId("state-filter").selectOption("CANCELLED"));
  await apply("cancelled queue retains state", () => page.getByTestId("orders-bucket-cancelled").click());
  await expect(page.getByTestId("state-filter")).toHaveValue("CANCELLED");
  await expect(page).toHaveURL(/state=CANCELLED/);
  await page.reload();
  await expect(page.getByTestId("state-filter")).toHaveValue("CANCELLED");
  await expect(page.getByTestId("orders-bucket-cancelled")).toHaveAttribute("aria-pressed", "true");
  await apply("all queue retains state", () => page.getByTestId("orders-bucket-all").click());
  await expect(page.getByTestId("state-filter")).toHaveValue("CANCELLED");
  await revealOrderFilters(page);
  await apply("default hide drafts", () => page.getByTestId("state-filter").selectOption("active"));
  await expect(page.getByTestId(`order-row-${ids.draft0}`)).toHaveCount(0);
  for (const bucket of ["unpaid","transfer_review","ready_to_ship","ready_to_consign","shipped","completed","cancelled","all"]) {
    await apply(`queue ${bucket}`, () => page.getByTestId(`orders-bucket-${bucket}`).click());
    await expect(page.getByTestId(`orders-bucket-${bucket}`)).toHaveAttribute("aria-pressed","true");
  }
  for (const [label, query, target] of [["order",ids.shipped,ids.shipped],["tracking","SYNTHETIC-MOU-TRACK",ids.shipped],["phone last4","0001",ids.shipped],["recipient","Synthetic Buyer",ids.shipped],["frozen SKU",frozenSKU,ids.shipped]] as const) {
    await page.getByTestId("orders-search").fill(query);
    const data = await apply(`search ${label}`, () => page.getByTestId("orders-apply").click());
    expect(data.total).toBeGreaterThan(0);
    await expect(page.getByTestId(`order-row-${target}`)).toBeVisible();
    expect(new URL(page.url()).searchParams.has("q")).toBe(false);
    await noPII(page);
  }
  await page.reload();
  await expect(page.getByTestId("orders-search")).toHaveValue(""); // deliberate privacy rule, not storage
  clicks.push({control:"refresh private search",result:"query cleared, not persisted in URL or storage"});
  for (const payment of ["cash_on_delivery","bank_transfer","pay_at_pickup","card"]) {
    await revealOrderFilters(page);
    await page.getByTestId("orders-payment-filter").selectOption(payment);
    await apply(`payment ${payment}`, () => page.getByTestId("orders-apply").click());
    await expect(page).toHaveURL(new RegExp(`payment_mode=${payment}`));
  }
  await revealOrderFilters(page);
  await page.getByTestId("orders-delivery-filter").selectOption("home");
  await apply("delivery home", () => page.getByTestId("orders-apply").click());
  await page.reload();
  await expect(page.getByTestId("orders-payment-filter")).toHaveValue("card");
  await expect(page.getByTestId("orders-delivery-filter")).toHaveValue("home");
  await apply("reset filters", () => page.getByTestId("orders-reset").click());
  await revealOrderFilters(page);
  await apply("inspect drafts for recorded live claim", () => page.getByTestId("state-filter").selectOption("all"));
  await revealOrderFilters(page);
  await page.getByTestId("orders-session-filter").selectOption(ids.live_session);
  const live = await apply("live session", () => page.getByTestId("orders-apply").click());
  expect(live.total).toBe(1);
  await expect(page.getByTestId(`order-row-${ids.live_order}`)).toContainText("Live claim");
  await page.reload();
  await expect(page.getByTestId("orders-session-filter")).toHaveValue(ids.live_session);
  await apply("reset live filter", () => page.getByTestId("orders-reset").click());
  const today = new Intl.DateTimeFormat("en-CA",{timeZone:"Asia/Taipei",year:"numeric",month:"2-digit",day:"2-digit"}).format(new Date());
  await revealOrderFilters(page);
  await page.getByTestId("orders-from").fill(today);
  await revealOrderFilters(page);
  await page.getByTestId("orders-to").fill(today);
  const dated = await apply("Taipei day", () => page.getByTestId("orders-apply").click());
  expect(dated.total).toBeGreaterThan(0);
  await page.reload();
  await expect(page.getByTestId("orders-from")).toHaveValue(today);
  await expect(page.getByTestId("orders-to")).toHaveValue(today);
  await apply("reset before visual acceptance", () => page.getByTestId("orders-reset").click());
  await revealOrderFilters(page);
  await apply("active ledger", () => page.getByTestId("state-filter").selectOption("active"));
  for (const locale of ["zh-TW","zh-CN","en"]) {
    await page.getByTestId("locale-switch").selectOption(locale);
    await expect(page).toHaveURL(new RegExp(`/${locale}/orders`));
    await expect(page.getByRole("heading", {level:1})).toHaveText(locale === "zh-TW" ? "訂單" : locale === "zh-CN" ? "订单" : "Orders");
    await expect(page.getByTestId("orders-table")).toBeVisible();
    for (const [width,height] of [[1586,992],[1366,768],[390,844]]) {
      await page.setViewportSize({width,height});
      if (width === 390) {
        // Real responsive transition must finish; never screenshot a half-open rail.
        await expect.poll(() => page.locator("aside[data-shell-rail]").evaluate(node => node.getBoundingClientRect().right)).toBeLessThanOrEqual(0);
        const more = page.getByTestId("orders-more-filters");
        await expect(more).toHaveAttribute("aria-expanded", "false");
        await expect(page.getByTestId("orders-payment-filter")).toBeHidden();
        await expect(page.getByTestId("state-filter")).toBeHidden();
        await more.click();
        await expect(more).toHaveAttribute("aria-expanded", "true");
        await revealOrderFilters(page);
        await page.getByTestId("orders-payment-filter").selectOption("card");
        await page.getByTestId("orders-apply").click();
        await expect(page).toHaveURL(/payment_mode=card/);
        await expect(more).toHaveAttribute("aria-expanded", "false");
        await more.click();
        await expect(page.getByTestId("orders-payment-filter")).toHaveValue("card");
        await expect(page.getByTestId("orders-from")).toBeVisible();
        await page.getByTestId("orders-reset").click();
        await expect(page).not.toHaveURL(/payment_mode=/);
        await expect(more).toHaveAttribute("aria-expanded", "false");
        await more.click();
        const refreshRead = page.waitForResponse(r => r.url().includes(`/api/stores/${store}/orders?`) && r.status() === 200);
        await page.getByTestId("orders-refresh").click();
        await refreshRead;
        await expect(page.getByTestId("orders-table")).toBeVisible();
        await more.click();
        await expect(more).toHaveAttribute("aria-expanded", "false");
        clicks.push({control:`mobile refresh ${locale}`,result:"open secondary controls, real refresh/readback, collapse"});
      }
      if (width === 390) {
        const rail = page.getByTestId("orders-tabs");
        // Native click/keyboard drive scrolling; evaluate below only measures geometry.
        await rail.hover();
        await page.mouse.wheel(1200, 0);
        await expect.poll(() => rail.evaluate(node => node.scrollLeft)).toBeGreaterThan(0);
        await page.getByTestId("orders-bucket-cancelled").click();
        await expect(page.getByTestId("orders-bucket-cancelled")).toHaveAttribute("aria-pressed", "true");
        await page.reload();
        await expect.poll(() => rail.evaluate(node => node.scrollLeft)).toBeGreaterThan(0);
        const bounds = await rail.boundingBox(), active = await page.getByTestId("orders-bucket-cancelled").boundingBox();
        expect(active!.x).toBeGreaterThanOrEqual(bounds!.x - 1);
        expect(active!.x + active!.width).toBeLessThanOrEqual(bounds!.x + bounds!.width + 1);
        const tabs = await rail.locator("button").evaluateAll(nodes => nodes.map(node => ({y:node.getBoundingClientRect().y,h:node.getBoundingClientRect().height,w:node.getBoundingClientRect().width})));
        expect(new Set(tabs.map(tab => Math.round(tab.y))).size).toBe(1);
        expect(tabs.every(tab => tab.h >= 44 && tab.w >= 44)).toBe(true);
        await screenshot(page,resolve(v2Evidence,`queue-scroll-${locale}-390x844.png`));
        await page.getByTestId("orders-bucket-all").click();
        await expect(page.getByTestId("orders-bucket-all")).toHaveAttribute("aria-pressed", "true");
        clicks.push({control:`horizontal queue ${locale}`,result:"real horizontal wheel then click, single row, 44px targets, cancelled visible after reload; returned to all"});
      }
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
      await page.getByRole("heading",{level:1}).scrollIntoViewIfNeeded();
      await screenshot(page,resolve(v2Evidence,`orders-${locale}-${width}x${height}.png`));
      if (width === 390) {
        const row = await page.locator('[data-testid^="order-expand-"]').first().boundingBox();
        expect(row).not.toBeNull();
        expect(row!.y).toBeLessThan(height); // The first order, not only a filter form, is in the initial viewport.
        const pageSelect=page.getByRole('checkbox',{name:picklistCopy[locale as keyof typeof picklistCopy].page,exact:true});
        await expect(pageSelect,'page selection remains visible outside the clipped mobile table header').toBeVisible();
        await pageSelect.check(); await expect(pageSelect).toBeChecked();
        await pageSelect.uncheck(); await expect(pageSelect).not.toBeChecked();
        clicks.push({control:`mobile secondary filters ${locale}`, result:`native toggle, select, apply, reopen persisted value, reset; first order y=${row!.y} < ${height}`});
      }
      await page.getByTestId("orders-table").scrollIntoViewIfNeeded();
      await screenshot(page,resolve(v2Evidence,`ledger-${locale}-${width}x${height}.png`));
    }
  }
  await writeFile(resolve(v2Evidence,"click-ledger.json"),JSON.stringify(clicks.map(item => ({page:"Orders",control:item.control,action:item.control,expected:"Assertions and persisted readback described by the named scenario pass",actual:item.result,status:"PASS"})),null,2));
});

for (const privateCursor of [false, true]) test(`MOU08 authoritative poll denial clears list and full detail (private cursor=${privateCursor})`, async ({page}) => {
  test.setTimeout(60_000);
  await signedLogin(page);
  if (privateCursor) {
    await page.getByTestId("orders-search").fill("0001");
    const searchResponse = page.waitForResponse(r => r.request().method() === "POST" && r.url().includes("/orders/search?") && r.status() === 200);
    await page.getByTestId("orders-apply").click();
    await searchResponse;
    await expect(page.getByTestId("orders-next")).toBeEnabled();
    const nextResponse = page.waitForResponse(r => r.request().method() === "POST" && r.url().includes("cursor=") && r.status() === 200);
    await page.getByTestId("orders-next").click();
    const next = await (await nextResponse).json();
    await expect(page.getByTestId(`order-expand-${next.items[0].order_id}`)).toBeVisible();
    await page.getByTestId(`order-expand-${next.items[0].order_id}`).click();
    expect(new URL(page.url()).searchParams.has("q")).toBe(false);
    expect(new URL(page.url()).searchParams.has("cursor")).toBe(false);
  } else await expand(page,ids.captured);
  await expect(page.getByTestId("order-detail")).toContainText("Synthetic home address");
  // Explicit fault injection of a backend permission revocation. No DOM events or synthetic clicks.
  await page.route(`**/api/stores/${store}/orders${privateCursor ? "/search" : ""}?*`,route=>route.fulfill({status:403,contentType:"application/json",headers:{"cache-control":"private, no-store"},body:'{"code":"forbidden"}'}));
  await expect(page.getByTestId("orders-table")).toHaveCount(0,{timeout:25_000});
  await expect(page.getByTestId("order-detail")).toHaveCount(0);
  await expect(page.getByTestId("merchant-orders")).toContainText("This account does not have permission to read orders.");
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
    // G-UI8 audit [READ/MEASURE]: waits for CSS animations to finish before measuring (read/wait)
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
    await paymentBadgesFit(page);
    await screenshot(
      page,
      testInfo.outputPath(`inline-${locale}-1586x992.png`),
    );
    await page.setViewportSize({ width: 390, height: 844 });
    // G-UI8 audit [READ/MEASURE]: waits for CSS animations to finish before measuring (read/wait)
    await page.evaluate(() =>
      Promise.all(
        document
          .getAnimations()
          .map((animation) => animation.finished.catch(() => undefined)),
      ),
    );
    const rail = await page.locator("[data-shell-rail]").evaluate((element) => ({
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
    await paymentBadgesFit(page);
    // G-UI8 audit [FIXTURE/SETUP]: scrolls to the top before a screenshot (viewport positioning)
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
    await revealOrderFilters(page);
    await expect(page.getByTestId("state-filter")).toBeVisible();
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

// ORDERS-V2-IMPL: additive acceptance on the registered signed-MOCK-identity chain.
// Count/row assertions read real BFF responses. Only the manual shipment command/readback below is MOCK.
type OrdersDesignPage = {
  items: Array<{ order_id: string; order_number: string }>;
  counts: Record<string, number>;
  total: number;
  next_cursor: string;
};
const designQueues = ["all", "unpaid", "transfer_review", "ready_to_ship", "ready_to_consign", "shipped", "completed", "cancelled"] as const;
const designOrderNumber = (id: string) => `LC-${id.replaceAll("-", "").toUpperCase()}`;

async function designClick(control: string, expected: string, action: () => Promise<void>) {
  const directory = resolve(evidence, "orders-v2-impl", test.info().testId);
  await mkdir(directory, { recursive: true });
  const record = { page: "Orders", control, action: control, expected, actual: "", status: "FAIL" };
  try {
    await action();
    record.actual = expected;
    record.status = "PASS";
  } catch (error) {
    record.actual = error instanceof Error ? error.message : String(error);
    throw error;
  } finally {
    // One file per action retains a failed assertion too; never pre-fill a PASS ledger.
    await writeFile(resolve(directory, `${control.replaceAll(/[^a-zA-Z0-9-]/g, "-")}.json`), JSON.stringify(record, null, 2));
  }
}

async function designRead(page: Page, action: () => Promise<unknown>, bucket = "all", cursor?: string): Promise<OrdersDesignPage> {
  const response = page.waitForResponse(r => {
    const url = new URL(r.url());
    return [ `/api/stores/${store}/orders`, `/api/stores/${store}/orders/search` ].includes(url.pathname)
      && url.searchParams.get("view") === "v2"
      && (url.searchParams.get("bucket") ?? "all") === bucket
      && (cursor === undefined || (url.searchParams.get("cursor") ?? "") === cursor);
  });
  await action();
  const result = await response;
  expect(result.status()).toBe(200);
  expect(result.headers()["cache-control"]).toBe("private, no-store");
  const data = await result.json() as OrdersDesignPage;
  await expect(page.getByTestId("orders-total")).toHaveText(new RegExp(`: ${data.total}$`));
  await expect(page.locator('[data-testid^="order-row-"]')).toHaveCount(data.items.length);
  await expect.poll(() => page.locator('[data-testid^="order-row-"]').evaluateAll(nodes => nodes.map(node => node.getAttribute("data-testid")!.slice("order-row-".length)))).toEqual(data.items.map(row => row.order_id));
  for (const key of designQueues) await expect(page.getByTestId(`orders-count-${key}`)).toHaveText(String(data.counts[key]));
  if (data.next_cursor) await expect(page.getByTestId("orders-next")).toBeEnabled();
  else await expect(page.getByTestId("orders-next")).toBeDisabled();
  return data;
}

async function designActiveVisible(page: Page, bucket: string) {
  const rail = page.getByTestId("orders-tabs");
  const active = page.getByTestId(`orders-bucket-${bucket}`);
  await expect(active).toHaveAttribute("aria-pressed", "true");
  // READ/MEASURE only: geometry cannot scroll a chip or change selection.
  await expect.poll(async () => {
    const parent = await rail.boundingBox(), chip = await active.boundingBox();
    return !!parent && !!chip && chip.x >= parent.x - 1 && chip.x + chip.width <= parent.x + parent.width + 1;
  }, { message: `The whole active ${bucket} chip is inside its horizontal viewport` }).toBe(true);
}

test("ORDERS-V2-IMPL queue tabs, all paginated rows and result total agree with the server after refresh", async ({ page }) => {
  test.setTimeout(120_000);
  await signedLogin(page);
  for (const bucket of designQueues) await designClick(`queue-${bucket}-and-refresh`, "Every page has the response rows/counts; unique traversed rows equal the queue count and result total", async () => {
    let data = await designRead(page, () => page.getByTestId(`orders-bucket-${bucket}`).click(), bucket, "");
    expect(data.total).toBe(data.counts[bucket]);
    const total = data.total;
    data = await designRead(page, () => page.reload(), bucket, "");
    expect(data.total).toBe(total);
    await expect(page.getByTestId(`orders-bucket-${bucket}`)).toHaveAttribute("aria-pressed", "true");
    const seen = new Set<string>();
    for (let number = 0; ; number++) {
      expect(number, "Bounded synthetic fixture traversal").toBeLessThan(10);
      for (const row of data.items) {
        expect(seen.has(row.order_id), "No duplicate order across cursor pages").toBe(false);
        seen.add(row.order_id);
      }
      if (!data.next_cursor) break;
      data = await designRead(page, () => page.getByTestId("orders-next").click(), bucket, data.next_cursor);
      expect(data.total).toBe(total);
      expect(data.counts[bucket]).toBe(total);
    }
    expect(seen.size).toBe(total);
  });
});

for (const locale of ["zh-TW", "zh-CN", "en"] as const) {
  test(`ORDERS-V2-IMPL mobile later queue stays visible through detail, back and refresh (${locale})`, async ({ page }) => {
    await signedLogin(page);
    await page.setViewportSize({ width: 390, height: 844 });
    await page.getByTestId("locale-switch").selectOption(locale);
    await expect(page.getByTestId("orders-table")).toBeVisible();
    let selected = "";
    await designClick("later-queue", "Cancelled chip is fully visible after native click and refresh", async () => {
      // Native wheel reveals later queues; there is no evaluate scroll mutation.
      await page.getByTestId("orders-tabs").hover();
      await page.mouse.wheel(1200, 0);
      const data = await designRead(page, () => page.getByTestId("orders-bucket-cancelled").click(), "cancelled");
      expect(data.items.length).toBeGreaterThan(0);
      selected = data.items[0].order_id;
      await designActiveVisible(page, "cancelled");
      await designRead(page, () => page.reload(), "cancelled");
      await designActiveVisible(page, "cancelled");
    });
    const before = page.url();
    await designClick("detail-and-return", "Visible return control restores the same filter/page and active chip", async () => {
      await page.getByTestId(`order-expand-${selected}`).click();
      await expect(page.getByTestId("order-detail")).toBeVisible();
      await page.getByTestId("order-detail-back").click();
      await expect(page.getByTestId("order-detail")).toHaveCount(0);
      await expect(page).toHaveURL(before);
      await designActiveVisible(page, "cancelled");
      await designRead(page, () => page.reload(), "cancelled");
      await designActiveVisible(page, "cancelled");
    });
  });
}

test("ORDERS-V2-IMPL mobile sticky search and private page survive detail/back without persisting the search on refresh", async ({ page }) => {
  await signedLogin(page);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.getByTestId("orders-search").fill("0001");
  let data = await designRead(page, () => page.getByTestId("orders-apply").click());
  await designClick("sticky-search", "Search stays fully on screen while order rows scroll", async () => {
    const first = page.getByTestId(`order-row-${data.items[0].order_id}`);
    const before = await first.boundingBox();
    await first.hover();
    await page.mouse.wheel(0, 500);
    // READ/MEASURE: prove the document actually moved, then inspect the search viewport.
    await expect.poll(async () => (await first.boundingBox())!.y).toBeLessThan(before!.y - 50);
    const search = await page.getByTestId("orders-search").boundingBox();
    expect(search).not.toBeNull();
    expect(search!.y).toBeGreaterThanOrEqual(0);
    expect(search!.y + search!.height).toBeLessThanOrEqual(844);
  });
  await designClick("private-page-detail-back", "Search and page-two rows survive detail/back; refresh clears only private search", async () => {
    expect(data.next_cursor).not.toBe("");
    data = await designRead(page, () => page.getByTestId("orders-next").click(), "all", data.next_cursor);
    const before = page.url(), rows = data.items.map(row => row.order_id);
    await page.getByTestId(`order-expand-${rows[0]}`).click();
    await expect(page.getByTestId("order-detail")).toBeVisible();
    await page.getByTestId("order-detail-back").click();
    await expect(page).toHaveURL(before);
    await expect(page.getByTestId("orders-search")).toHaveValue("0001");
    await expect.poll(() => page.locator('[data-testid^="order-row-"]').evaluateAll(nodes => nodes.map(node => node.getAttribute("data-testid")!.slice("order-row-".length)))).toEqual(rows);
    expect(new URL(page.url()).searchParams.has("q")).toBe(false);
    expect(new URL(page.url()).searchParams.has("cursor")).toBe(false);
    await page.reload();
    await expect(page.getByTestId("orders-search")).toHaveValue("");
    await expect(page.getByTestId("state-filter")).toHaveValue("all");
    await expect(page.getByTestId("orders-bucket-all")).toHaveAttribute("aria-pressed", "true");
    await noPersistentOrderBody(page);
  });
});

test("ORDERS-V2-IMPL desktop selection names the actual orders and uses the existing preview/cancel flow", async ({ page }) => {
  await signedLogin(page);
  const chosen = [ids.captured, ids.authorized];
  await designClick("select-orders-and-view", "Selection bar names both selected order numbers and preview uses the original pick-list endpoint", async () => {
    for (const id of chosen) await page.getByTestId(`order-row-${id}`).getByRole("checkbox").check();
    const bar = page.getByTestId("orders-selection-bar");
    await expect(bar).toBeVisible();
    for (const id of chosen) await expect(bar).toContainText(designOrderNumber(id));
    await expect(bar.getByTestId("orders-selection-count")).toHaveText("2");
    // READ/MEASURE only: sticky positioning belongs to the whole selection bar, not a screenshot.
    expect(await bar.evaluate(node => getComputedStyle(node).position)).toBe("sticky");
    await bar.getByTestId("orders-selection-view").click();
    const preview = page.waitForResponse(r => new URL(r.url()).pathname === `/api/stores/${store}/orders/pick-list` && r.request().method() === "POST");
    await page.getByTestId("pick-toolbar").getByRole("button", { name: "Pick list", exact: true }).click();
    const response = await preview;
    expect(response.status()).toBe(200);
    expect(response.request().postDataJSON()).toEqual({ order_ids: chosen });
    const document = await response.json() as { orders: Array<{ order_id: string; order_number: string }> };
    expect(document.orders.length).toBeGreaterThan(0);
    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible();
    for (const row of document.orders) await expect(dialog).toContainText(row.order_number);
    await dialog.getByRole("button", { name: "Close", exact: true }).click();
    await expect(dialog).toHaveCount(0);
    for (const id of chosen) await expect(bar).toContainText(designOrderNumber(id));
    await bar.getByTestId("orders-selection-cancel").click();
    await expect(page.getByTestId("pick-count")).toHaveText("0");
    for (const id of chosen) await expect(page.getByTestId(`order-row-${id}`).getByRole("checkbox")).not.toBeChecked();
    await page.reload();
    for (const id of chosen) await expect(page.getByTestId(`order-row-${id}`).getByRole("checkbox")).not.toBeChecked();
  });
});

for (const [locale, width] of [["en", 1586], ["zh-TW", 390], ["zh-CN", 390]] as const) {
  test(`ORDERS-V2-IMPL MOCK manual shipment requires confirmation, one identity, cancel and refreshed readback (${locale}/${width})`, async ({ page }) => {
    await signedLogin(page);
    await page.setViewportSize({ width, height: width === 390 ? 844 : 992 });
    await page.getByTestId("locale-switch").selectOption(locale);
    await expect(page.getByTestId("orders-table")).toBeVisible();
    const id = ids.captured;
    const path = `/api/stores/${store}/orders/${id}`;
    let saved = false;
    const submissions: Array<Record<string, unknown>> = [];
    const head = { version: 1, status: "SHIPPED", carrier_code: "black_cat", carrier_name: null, tracking_number: "MOCK-ORDERS-V2-001", tracking_url: null, recorded_at: "2026-10-28T00:00:00Z" };
    // MOCK preparation/readback: intercept the existing route only. A real PUT would invalidate the PG read-only harness.
    await page.route(url => url.pathname === `${path}/shipment`, async route => {
      expect(route.request().method()).toBe("PUT");
      expect(route.request().headers()["idempotency-key"]).toBeTruthy();
      submissions.push(route.request().postDataJSON() as Record<string, unknown>);
      saved = true;
      await route.fulfill({ status: 200, contentType: "application/json", headers: { "cache-control": "private, no-store" }, body: JSON.stringify(head) });
    });
    await page.route(url => url.pathname === path, async route => {
      // fetch prepares a synthetic readback from the unchanged real fixture; it never validates a user action.
      const response = await route.fetch();
      const detail = await response.json();
      await route.fulfill({ response, json: saved ? { ...detail, fulfillment_state: "MERCHANT_SHIPPED", shipment: head } : detail });
    });
    await page.route(url => url.pathname === `${path}/shipment/history`, async route => {
      if (!saved) { await route.continue(); return; }
      await route.fulfill({ status: 200, contentType: "application/json", headers: { "cache-control": "private, no-store" }, body: JSON.stringify({ items: [{ ...head, note: null, void_reason: null, principal_id: "10000000-0000-4000-8000-000000000001" }] }) });
    });
    await expand(page, id);
    await expect(page.getByTestId("shipment-form")).toBeVisible();
    await page.getByTestId("ship-carrier").selectOption("black_cat");
    await page.getByTestId("ship-tracking").fill(head.tracking_number);
    await designClick("review-before-shipment-write", "Review contains one order identity, carrier, tracking and the manual fulfilment note; no command sent", async () => {
      await page.getByTestId("shipment-submit").click();
      const confirm = page.getByRole("dialog").filter({ has: page.getByTestId("shipment-confirm") });
      await expect(confirm).toBeVisible();
      const text = await confirm.innerText();
      expect(text.split(designOrderNumber(id)).length - 1).toBe(1);
      await expect(confirm).toContainText(head.tracking_number);
      await expect(confirm).toContainText(locale === "en" ? /Black Cat/i : /黑貓|黑猫/);
      await expect(confirm.getByTestId("shipment-manual-note")).toContainText(locale === "en" ? /label|carrier/i : /面單|面单/);
      expect(submissions).toHaveLength(0);
      await confirm.getByTestId("shipment-confirm-cancel").click();
      await expect(confirm).toHaveCount(0);
      expect(submissions).toHaveLength(0);
      await expect(page.getByTestId("ship-tracking")).toHaveValue(head.tracking_number);
    });
    await designClick("confirm-shipment-and-refresh", "Exactly one existing shipment PUT follows confirm; its MOCK state persists after refresh", async () => {
      await page.getByTestId("shipment-submit").click();
      await page.getByTestId("shipment-confirm").click();
      await expect(page.getByTestId("shipment-record")).toContainText(head.tracking_number);
      expect(submissions).toHaveLength(1);
      expect(submissions[0]).toEqual({ expected_version: 0, status: "SHIPPED", carrier_code: "black_cat", carrier_name: null, tracking_number: head.tracking_number, tracking_url: null, note: null, void_reason: null });
      await page.reload();
      await expect(page.getByTestId("shipment-record")).toContainText(head.tracking_number);
      expect(submissions).toHaveLength(1);
    });
  });
}

for (const state of ["PROVIDER_LABEL_CREATED", "REVIEW_REQUIRED"] as const) {
  test(`ORDERS-V2-IMPL status help explains ${state} without treating it as shipment/payment`, async ({ page }) => {
    await signedLogin(page);
    const id = state === "REVIEW_REQUIRED" ? ids.review : ids.captured;
    if (state === "PROVIDER_LABEL_CREATED") {
      // MOCK list projection: the existing chain has no CVS CREATED fixture. Keep that boundary explicit;
      // this projection tests help only, never proves a real provider label or carrier handover.
      await page.route(url => url.pathname === `/api/stores/${store}/orders/search`, async route => {
        const response = await route.fetch();
        const data = await response.json();
        expect(data.items).toHaveLength(1);
        await route.fulfill({ response, json: { ...data,
          items: [{ ...data.items[0], fulfillment_state: state }],
          counts: Object.fromEntries(designQueues.map(key => [key, key === "all" || key === "ready_to_consign" ? 1 : 0])),
        } });
      });
    }
    await page.getByTestId("orders-search").fill(id);
    await designRead(page, () => page.getByTestId("orders-apply").click());
    await designClick(`status-help-${state}`, "Status help opens beside the badge, distinguishes the fact and closes by the same real control", async () => {
      const row = page.getByTestId(`order-row-${id}`);
      const help = row.getByTestId(`order-status-help-${state}`);
      await help.click();
      await expect(help).toHaveAttribute("aria-expanded", "true");
      const note = row.getByTestId(`order-status-explanation-${state}`);
      await expect(note).toBeVisible();
      await expect(note).toContainText(state === "REVIEW_REQUIRED" ? /review|paid/i : /carrier|hand|print/i);
      await help.click();
      await expect(help).toHaveAttribute("aria-expanded", "false");
      await expect(note).not.toBeVisible();
      await page.reload();
      await expect(page.getByTestId("orders-search")).toHaveValue("");
    });
  });
}
