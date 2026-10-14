// Purpose: Exercises settings controls and client-storage disclosures against the supplied real harness origin.
// Depends on: @playwright/test; harness env: LC_BROWSER_PUBLIC_ORIGIN
// Used by: tests/foundation/browser_identity_chain_test.go
// PAYUNi was removed from the wizard (owner decision 2026-10-14, pay-rm2): the only card path is platform Stripe, reached from the
// wizard through the card-payments link; this spec proves no PAYUNi control or text remains and keeps the safe-uncertainty,
// policy/service and changed-session checks on the merchant-arranged delivery flow.
import { expect, test, type Page, type Route } from "@playwright/test";

const origin = process.env.LC_BROWSER_PUBLIC_ORIGIN;
if (!origin || new URL(origin).origin !== origin)
  throw new Error("real harness origin required");
test.use({ baseURL: origin, trace: "off", actionTimeout: 10_000 });
test.describe.configure({ timeout: 90_000 });

async function noPayuni(page: Page) {
  // G-UI8 audit [READ/MEASURE]: scans visible text and client storage for the removed provider (read only)
  const storage = await page.evaluate(() =>
    JSON.stringify({
      local: { ...localStorage },
      session: { ...sessionStorage },
    }),
  );
  expect(/payuni/i.test(storage)).toBe(false);
  expect(/payuni/i.test(await page.getByTestId("settings-wizard").innerText())).toBe(false);
}

test("REAL_PG A wizard has no PAYUNi, creates unseeded configuration and preserves safe uncertainty", async ({
  page,
  context,
}, testInfo) => {
  await page.goto("/en/");
  await page
    .getByRole("button", { name: "Sign in with identity service", exact: true })
    .click();
  await page
    .getByLabel("Merchant name", { exact: true })
    .fill("Wizard Merchant");
  await page
    .getByRole("button", { name: "Next: store settings", exact: true })
    .click();
  await page.getByLabel("Store name", { exact: true }).fill("Wizard Store");
  await page.getByLabel("Transaction currency").selectOption("TWD");
  await page
    .getByRole("button", { name: "Next: warehouse", exact: true })
    .click();
  await page
    .getByLabel("Initial warehouse name", { exact: true })
    .fill("Wizard Warehouse");
  const created = page.waitForResponse(
    (r) => new URL(r.url()).pathname === "/api/onboarding/initial-store",
  );
  await page
    .getByRole("button", { name: "Create internal workspace", exact: true })
    .click();
  const receipt = (await (await created).json()) as { store_id: string };
  await page
    .getByRole("button", { name: "Open workspace", exact: true })
    .click();
  // W0 shell: the owner sees Settings, Team and Billing under the Settings group, so the group button expands it and the
  // Settings entry (registry id "settings") is the real click that opens the wizard.
  await page.getByTestId("nav-group-settings").click();
  await page.getByTestId("nav-settings").click();
  await expect(page.getByTestId("settings-wizard")).toBeVisible();
  await expect(
    page.getByText("Loading saved configuration…", { exact: true }),
  ).toBeHidden();
  // Step 1: no provider choice, no credential form; the platform card-payments entry is the only payment control.
  await expect(page.getByRole("radio")).toHaveCount(0);
  await expect(page.getByLabel("HashKey", { exact: true })).toHaveCount(0);
  await expect(page.getByTestId("settings-account-form")).toHaveCount(0);
  await noPayuni(page);
  await page.getByTestId("settings-card-payments-link").click();
  await expect(page).toHaveURL(/\/settings\/payments\/card/);
  await page.goBack();
  await expect(page.getByTestId("settings-wizard")).toBeVisible();
  await page.getByRole("button", { name: "Continue", exact: true }).click();
  // Step 2: merchant-arranged delivery cards. The old PAYUNi account form must not appear.
  await expect(page.getByTestId("settings-account-form")).toHaveCount(0);
  // G-UI8 audit [READ/MEASURE]: computes rendered text contrast (read only)
  const renderedStyle = await page.evaluate(() => {
    const subtitle = document.querySelector(".settings-heading p")!;
    const color = getComputedStyle(subtitle).color;
    let parent: Element | null = subtitle;
    let background = "rgba(0, 0, 0, 0)";
    while (parent && background === "rgba(0, 0, 0, 0)") {
      background = getComputedStyle(parent).backgroundColor;
      parent = parent.parentElement;
    }
    const luminance = (rgb: string) => {
      const c = (rgb.match(/[\d.]+/g) ?? [])
        .slice(0, 3)
        .map(Number)
        .map((value) => {
          const v = value / 255;
          return v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4;
        });
      return c[0] * 0.2126 + c[1] * 0.7152 + c[2] * 0.0722;
    };
    const l = [luminance(color), luminance(background)].sort((a, b) => b - a);
    return {
      color,
      background,
      fontSize: getComputedStyle(subtitle).fontSize,
      contrast: (l[0] + 0.05) / (l[1] + 0.05),
    };
  });
  await testInfo.attach("settings-rendered-style.json", {
    body: JSON.stringify(renderedStyle),
    contentType: "application/json",
  });
  expect(renderedStyle.contrast).toBeGreaterThanOrEqual(4.5);
  console.info("Settings rendered contrast:", JSON.stringify(renderedStyle));
  await page.screenshot({
    path: testInfo.outputPath("a-hero-repro.png"),
    fullPage: false,
    animations: "disabled",
  });
  await page.screenshot({
    path: testInfo.outputPath("a-desktop-delivery.png"),
    fullPage: true,
    animations: "disabled",
  });
  await page.setViewportSize({ width: 390, height: 844 });
  await expect
    .poll(async () => {
      const rail = await page.locator("#workspace-navigation").boundingBox(); // W0 shell rail (off-canvas drawer at 390px)
      return rail ? Math.round(rail.x + rail.width) : 0;
    })
    .toBeLessThanOrEqual(0);
  expect(
    // G-UI8 audit [READ/MEASURE]: measures horizontal overflow (layout read, no state change)
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({
    path: testInfo.outputPath("a-mobile-delivery.png"),
    fullPage: true,
    animations: "disabled",
  });
  await page.setViewportSize({ width: 1586, height: 992 });

  await page.getByRole("button", { name: "Continue", exact: true }).click();
  const marketForm = page.getByTestId("settings-market-form");
  await expect(marketForm).toBeVisible();
  await marketForm.getByLabel("Market code", { exact: true }).fill("taiwan");
  await marketForm
    .getByLabel("Market name", { exact: true })
    .fill("Taiwan retail");
  // Lose only the response after the real command commits. No mocked success.
  const marketRequests: Array<{ key: string; body: string }> = [];
  let loseMarketResponse = true;
  let marketCommitted!: () => void;
  const committedMarket = new Promise<void>((resolve) => {
    marketCommitted = resolve;
  });
  let releaseLostResponse!: () => void;
  const lostResponse = new Promise<void>((resolve) => {
    releaseLostResponse = resolve;
  });
  const marketPath = `/api/stores/${receipt.store_id}/markets`;
  await page.route(`**${marketPath}`, async (route) => {
    if (route.request().method() !== "POST") return route.continue();
    marketRequests.push({
      key: route.request().headers()["idempotency-key"],
      body: route.request().postData() ?? "",
    });
    if (loseMarketResponse) {
      loseMarketResponse = false;
      const response = await route.fetch();
      expect(response.status()).toBe(200);
      marketCommitted();
      // Refresh before the UI receives any result: exercise the crash window,
      // not only the catch handler that marks a completed network error unknown.
      await lostResponse;
      await route.abort("failed").catch(() => undefined);
    } else await route.continue();
  });
  await marketForm
    .getByRole("button", { name: "Create market", exact: true })
    .click();
  await committedMarket;
  await page.reload();
  releaseLostResponse();
  const pendingControls = page.getByTestId("settings-pending");
  await expect(pendingControls).toBeVisible();
  await expect(pendingControls.getByLabel("HashKey", { exact: true })).toHaveCount(0);
  await page
    .getByRole("button", { name: "Retry pending command", exact: true })
    .click();
  await expect(page.getByTestId("settings-policy-form").or(page.getByLabel("Stable service code", { exact: true }))).toBeVisible();
  await expect(pendingControls).toBeHidden();
  expect(marketRequests.length).toBe(2);
  expect(marketRequests[0].key === marketRequests[1].key).toBe(true);
  expect(marketRequests[0].body === marketRequests[1].body).toBe(true);
  await noPayuni(page);

  // Locale routes preserve nonsecret setup and the store's TWD currency.
  for (const [locale, label, heading] of [
    ["zh-CN", "Language", "店铺设置"],
    ["zh-TW", "语言", "商店設定"],
    ["en", "語言", "Store settings"],
  ]) {
    await page.getByLabel(label, { exact: true }).selectOption(locale);
    await expect(
      page.getByRole("heading", { name: heading, exact: true }),
    ).toBeVisible();
    await expect(page.getByTestId("settings-wizard")).toContainText("TWD");
    await noPayuni(page);
    await page.screenshot({
      path: testInfo.outputPath(`a-${locale}-delivery.png`),
      fullPage: true,
      animations: "disabled",
    });
  }

  const policyForm = page.getByTestId("settings-policy-form");
  // Network scheduling only: use the REAL PG replies, but hold the read baseline
  // and PUT receipt independently. A fast merchant can otherwise save before a
  // baseline arrives, or a PUT callback can overwrite a newer sibling read.
  function heldResponse() {
    let release!: () => void;
    const released = new Promise<void>((resolve) => { release = resolve; });
    let arrived = false;
    return {
      release,
      arrived: () => arrived,
      deliver: async (route: Route) => {
        const response = await route.fetch();
        arrived = true;
        await released;
        await route.fulfill({ response });
      },
    };
  }
  const policyRead = heldResponse(), serviceRead = heldResponse(), policyWrite = heldResponse();
  const policyURL = /\/delivery-services\/home\/policy$/;
  const serviceURL = /\/delivery-services\/home$/;
  await page.route(policyURL, (route) =>
    (route.request().method() === "GET" ? policyRead : policyWrite).deliver(route));
  await page.route(serviceURL, (route) => route.request().method() === "GET"
    ? serviceRead.deliver(route) : route.continue());
  const savePolicy = policyForm.getByRole("button", { name: "Save pricing policy", exact: true });
  await page.getByLabel("Stable service code", { exact: true }).fill("home");
  await expect.poll(policyRead.arrived).toBe(true);
  await expect.soft(savePolicy, "a pending version read is not a saveable baseline").toBeDisabled();
  policyRead.release();
  await expect(savePolicy).toBeEnabled();
  // stop-bleed D02: NT$ amounts are whole dollars; a decimal is refused locally with its own sentence and nothing is sent
  let policyWrites = 0;
  page.on("request", (r) => {
    if (r.method() === "PUT" && r.url().includes("/policy")) policyWrites++;
  });
  await policyForm
    .getByLabel("Flat shipping (NT$)", { exact: true })
    .fill("60.5");
  await policyForm
    .getByLabel(/^Configuration reference \/ reason/)
    .fill("Fixture explicit merchant tariff; not provider validation");
  await policyForm
    .getByRole("button", { name: "Save pricing policy", exact: true })
    .click();
  await expect(
    page.getByText("NT$ amounts are whole dollars, for example 60."),
  ).toBeVisible();
  expect(policyWrites).toBe(0);
  await policyForm
    .getByLabel("Flat shipping (NT$)", { exact: true })
    .fill("60");
  await policyForm
    .getByRole("combobox", { name: "Tax mode", exact: true })
    .selectOption("none");
  await policyForm.getByLabel("Enable pricing policy", { exact: true }).check();
  await policyForm
    .getByRole("button", { name: "Save pricing policy", exact: true })
    .click();
  await expect.poll(policyWrite.arrived).toBe(true);
  serviceRead.release();
  // READ/MEASURE only: prove the service GET has been applied while the policy
  // PUT receipt is held, not merely that its HTTP response arrived. No DOM or
  // storage mutation; only the synthetic draft's numeric version is returned.
  await expect.poll(() => page.evaluate(() => {
    const key = Object.keys(sessionStorage).find((item) => item.startsWith("commerce-settings-draft:"));
    return key ? JSON.parse(sessionStorage.getItem(key)!).draft.serviceObservation?.version : null;
  })).toBe(0);
  policyWrite.release();
  const serviceForm = page.getByTestId("settings-service-form");
  await serviceForm
    .getByLabel("Simplified Chinese name", { exact: true })
    .fill("标准配送");
  await serviceForm
    .getByLabel("Traditional Chinese name", { exact: true })
    .fill("標準配送");
  await serviceForm
    .getByLabel("English name", { exact: true })
    .fill("Standard delivery");
  await serviceForm
    .getByRole("button", { name: "Save delivery service", exact: true })
    .click();
  await expect(page.getByTestId("settings-status")).toBeVisible();
  await expect(page.getByTestId("settings-status")).not.toContainText(/PAYUNi/i);
  await page.unroute(policyURL);
  await page.unroute(serviceURL);
  await page.reload();
  await expect(page.getByTestId("settings-status")).toContainText("home");

  await page
    .getByRole("button", { name: "Market and delivery", exact: false })
    .click();
  await expect(
    page
      .getByTestId("settings-service-form")
      .getByLabel("English name", { exact: true }),
  ).toHaveValue("Standard delivery");
  const originalCSRF = (await context.cookies()).find(
    (c) => c.name === "__Host-commerce_csrf",
  )!;
  let changedSessionWrites = 0;
  page.on("request", (r) => {
    if (
      ["POST", "PUT"].includes(r.method()) &&
      new URL(r.url()).pathname.startsWith(`/api/stores/${receipt.store_id}/`)
    )
      changedSessionWrites++;
  });
  await context.addCookies([{ ...originalCSRF, value: `${"X".repeat(42)}A` }]);
  await page
    .getByLabel("Flat shipping (NT$)", { exact: true })
    .fill("61");
  await page
    .getByLabel(/^Configuration reference \/ reason/)
    .fill("Must not be sent under changed session");
  await page
    .getByRole("button", { name: "Save pricing policy", exact: true })
    .click();
  await expect(page.getByText(/Your session changed\./)).toBeVisible();
  expect(changedSessionWrites).toBe(0);
  await context.addCookies([originalCSRF]);
  // End the real session the way the merchant does, after proving old-tab writes stayed zero: Account menu -> Sign out (a real click path; the
  // BFF answer is read from the response the click triggers).
  await page.locator("header[data-shell-topbar] summary", { hasText: /^Account$/ }).click();
  const [logoutResponse] = await Promise.all([
    page.waitForResponse(
      (r) => new URL(r.url()).pathname === "/api/auth/logout" && r.request().method() === "POST",
    ),
    page.getByTestId("workspace-sign-out").click(),
  ]);
  const logout = logoutResponse.status();
  expect(logout).toBe(204);
});
