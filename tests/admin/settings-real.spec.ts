import { expect, test, type Page } from "@playwright/test";

const origin = process.env.LC_BROWSER_PUBLIC_ORIGIN;
if (!origin || new URL(origin).origin !== origin)
  throw new Error("real harness origin required");
test.use({ baseURL: origin, trace: "off", actionTimeout: 10_000 });
test.describe.configure({ timeout: 90_000 });

async function noSecrets(page: Page, secrets: string[]) {
  // G-UI8 audit [READ/MEASURE]: scans client storage for secrets/PII (read only)
  const storage = await page.evaluate(() =>
    JSON.stringify({
      local: { ...localStorage },
      session: { ...sessionStorage },
    }),
  );
  const html = await page.content();
  for (const secret of secrets) {
    // Boolean assertions avoid echoing credentials into failed assertion logs.
    expect(storage.includes(secret)).toBe(false);
    expect(html.includes(secret)).toBe(false);
  }
}

test("REAL_PG A wizard creates unseeded configuration and preserves safe uncertainty", async ({
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
  await page.getByLabel("PAYUNi payment", { exact: true }).check();
  await page.getByRole("button", { name: "Continue", exact: true }).click();
  const accountForm = page.getByTestId("settings-account-form");
  await expect(accountForm).toBeVisible();
  await expect(
    page.getByText("Loading saved configuration…", { exact: true }),
  ).toBeHidden();
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
    path: testInfo.outputPath("a-desktop-account.png"),
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
    path: testInfo.outputPath("a-mobile-account.png"),
    fullPage: true,
    animations: "disabled",
  });
  await page.setViewportSize({ width: 1586, height: 992 });

  await accountForm
    .getByLabel("Merchant ID (MerID)", { exact: true })
    .fill("wizard_fixture");
  // Lose only the response after the real command commits. No mocked success.
  const accountRequests: Array<{ key: string; body: string }> = [];
  let loseAccountResponse = true;
  let accountCommitted!: () => void;
  const committedAccount = new Promise<void>((resolve) => {
    accountCommitted = resolve;
  });
  let releaseLostResponse!: () => void;
  const lostResponse = new Promise<void>((resolve) => {
    releaseLostResponse = resolve;
  });
  const accountPath = `/api/stores/${receipt.store_id}/provider-accounts`;
  await page.route(`**${accountPath}`, async (route) => {
    if (route.request().method() !== "POST") return route.continue();
    accountRequests.push({
      key: route.request().headers()["idempotency-key"],
      body: route.request().postData() ?? "",
    });
    if (loseAccountResponse) {
      loseAccountResponse = false;
      const response = await route.fetch();
      expect(response.status()).toBe(200);
      accountCommitted();
      // Refresh before the UI receives any result: exercise the crash window,
      // not only the catch handler that marks a completed network error unknown.
      await lostResponse;
      await route.abort("failed").catch(() => undefined);
    } else await route.continue();
  });
  // Synthetic fixture strings, never actual merchant credentials.
  const hashKey = "K".repeat(32),
    hashIV = "V".repeat(16);
  await accountForm.getByLabel("HashKey", { exact: true }).fill(hashKey);
  await accountForm.getByLabel("HashIV", { exact: true }).fill(hashIV);
  await accountForm
    .getByRole("button", { name: "Save account and continue", exact: true })
    .click();
  await committedAccount;
  await page.reload();
  releaseLostResponse();
  const pendingControls = page.getByTestId("settings-pending");
  await expect(pendingControls).toBeVisible();
  await expect(
    pendingControls.getByLabel("HashKey", { exact: true }),
  ).toHaveValue("");
  await noSecrets(page, [hashKey, hashIV]);
  await pendingControls
    .getByLabel("HashKey", { exact: true })
    .fill("W".repeat(32));
  await pendingControls.getByLabel("HashIV", { exact: true }).fill(hashIV);
  const mismatchedRetry = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname === accountPath &&
      response.request().method() === "POST",
  );
  await page
    .getByRole("button", { name: "Retry pending command", exact: true })
    .click();
  expect((await mismatchedRetry).status()).toBe(409);
  await expect(pendingControls).toBeVisible();
  await noSecrets(page, [hashKey, hashIV, "W".repeat(32)]);
  await pendingControls.getByLabel("HashKey", { exact: true }).fill(hashKey);
  await pendingControls.getByLabel("HashIV", { exact: true }).fill(hashIV);
  await page
    .getByRole("button", { name: "Retry pending command", exact: true })
    .click();
  await expect(page.getByTestId("settings-market-form")).toBeVisible();
  expect(accountRequests.length).toBe(3);
  expect(
    accountRequests.every((item) => item.key === accountRequests[0].key),
  ).toBe(true);
  expect(accountRequests[0].body === accountRequests[1].body).toBe(false);
  expect(accountRequests[0].body === accountRequests[2].body).toBe(true);
  await noSecrets(page, [hashKey, hashIV]);

  const marketForm = page.getByTestId("settings-market-form");
  await marketForm.getByLabel("Market code", { exact: true }).fill("taiwan");
  await marketForm
    .getByLabel("Market name", { exact: true })
    .fill("Taiwan retail");
  await marketForm
    .getByRole("button", { name: "Create market", exact: true })
    .click();
  const paymentForm = page.getByTestId("settings-payment-form");
  await expect(paymentForm).toBeVisible();
  await paymentForm
    .getByLabel("Simplified Chinese name", { exact: true })
    .fill("信用卡");
  await paymentForm
    .getByLabel("Traditional Chinese name", { exact: true })
    .fill("信用卡");
  await paymentForm
    .getByLabel("English name", { exact: true })
    .fill("Credit card");
  await paymentForm
    .getByLabel("Minimum amount (NT$)", { exact: true })
    .fill("1"); // stop-bleed D02: money fields are NT$ whole dollars now (1 = 100 minor on the wire)
  await paymentForm
    .getByLabel("Maximum amount (NT$)", { exact: true })
    .fill("1000");
  const methodRequests: Array<{ key: string; body: string }> = [];
  let loseMethod = true;
  await page.route("**/payment-methods/payuni_credit", async (route) => {
    if (route.request().method() !== "PUT") return route.continue();
    methodRequests.push({
      key: route.request().headers()["idempotency-key"],
      body: route.request().postData() ?? "",
    });
    if (loseMethod) {
      loseMethod = false;
      expect((await route.fetch()).status()).toBe(200);
      await route.abort("failed");
    } else await route.continue();
  });
  await paymentForm
    .getByRole("button", { name: "Save disabled payment draft", exact: true })
    .click();
  await expect(pendingControls).toBeVisible();
  await page
    .getByRole("button", { name: "Retry pending command", exact: true })
    .click();
  await expect.poll(() => methodRequests.length).toBe(2);
  expect(
    methodRequests[0].key === methodRequests[1].key &&
      methodRequests[0].body === methodRequests[1].body,
  ).toBe(true);
  await page
    .getByRole("button", { name: "Check status", exact: false })
    .click();
  await page
    .getByRole("button", { name: "Inspect runtime availability", exact: true })
    .click();
  await expect(page.getByTestId("settings-status")).toContainText(
    "Unavailable",
  );
  await page.reload();
  await expect(page.getByTestId("settings-wizard")).toContainText(
    "wizard_fixture",
  );
  await noSecrets(page, [hashKey, hashIV]);

  // A newer server revision must never be borrowed for older dirty form fields.
  await page
    .getByRole("button", { name: "Configure methods", exact: false })
    .click();
  await paymentForm
    .getByLabel("English name", { exact: true })
    .fill("Local unsaved name");
  // G-UI8 audit [FIXTURE/SETUP]: a concurrent write by another operator (stale-form fixture)
  const concurrent = await page.evaluate(async (store) => {
    const prefix = `/api/stores/${store}/markets`;
    const markets = await (await fetch(prefix)).json();
    const path = `${prefix}/${markets.items[0].id}/countries/TW/payment-methods/payuni_credit`;
    const saved = await (await fetch(path)).json();
    const {
      market_id,
      country,
      code,
      name_hans,
      name_hant,
      enabled,
      visible,
      sort_order,
      min_amount_minor,
      max_amount_minor,
    } = saved;
    const csrf =
      document.cookie
        .split(";")
        .map((part) => part.trim())
        .find((part) => part.startsWith("__Host-commerce_csrf="))
        ?.split("=")[1] ?? "";
    const createdAccount = await fetch(
      `/api/stores/${store}/provider-accounts`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Idempotency-Key": crypto.randomUUID(),
          "X-CSRF-Token": csrf,
        },
        body: JSON.stringify({
          provider: "payuni",
          environment: "SANDBOX",
          account_id: "second_fixture",
          credentials: { hash_key: "H".repeat(32), hash_iv: "I".repeat(16) },
        }),
      },
    );
    if (createdAccount.status !== 200)
      return { status: createdAccount.status, marketID: "", connectionID: "" };
    const accounts = await (
      await fetch(`/api/stores/${store}/provider-accounts`)
    ).json();
    // Bind the method to the NON-first account, regardless of random UUID order.
    const bound = accounts.items[1];
    const status = (
      await fetch(path, {
        method: "PUT",
        headers: {
          "Content-Type": "application/json",
          "Idempotency-Key": crypto.randomUUID(),
          "X-CSRF-Token": csrf,
        },
        body: JSON.stringify({
          market_id,
          country,
          code,
          environment: bound.environment,
          connection_id: bound.id,
          binding_version: bound.binding_version,
          name_hans,
          name_hant,
          name_en: "Remote saved name",
          enabled,
          visible,
          sort_order,
          min_amount_minor,
          max_amount_minor,
          expected_version: saved.version,
        }),
      })
    ).status;
    return {
      status,
      marketID: market_id,
      connectionID: bound.id,
      firstConnectionID: accounts.items[0].id,
    };
  }, receipt.store_id);
  expect(concurrent.status).toBe(200);

  // Locale routes preserve nonsecret setup and the store's TWD currency.
  for (const [locale, label, heading] of [
    ["zh-CN", "Language", "物流与收款设置"],
    ["zh-TW", "语言", "物流與收款設定"],
    ["en", "語言", "Delivery & payment settings"],
  ]) {
    await page.getByLabel(label, { exact: true }).selectOption(locale);
    await expect(
      page.getByRole("heading", { name: heading, exact: true }),
    ).toBeVisible();
    await expect(page.getByTestId("settings-wizard")).toContainText("TWD");
    await page.screenshot({
      path: testInfo.outputPath(`a-${locale}-status.png`),
      fullPage: true,
      animations: "disabled",
    });
  }
  await expect(
    paymentForm.getByLabel("English name", { exact: true }),
  ).toHaveValue("Local unsaved name");
  await page
    .getByRole("button", { name: "Connect account", exact: false })
    .click();
  await accountForm
    .getByRole("combobox", { name: "Account", exact: true })
    .selectOption(concurrent.firstConnectionID!);
  await accountForm
    .getByRole("button", { name: "Continue", exact: true })
    .click();
  const staleSave = paymentForm.getByRole("button", {
    name: "Save disabled payment draft",
    exact: true,
  });
  if (await staleSave.isEnabled()) await staleSave.click();
  await expect(
    page.getByRole("alert").filter({ hasText: /Configuration changed/ }),
  ).toBeVisible();
  // Explicit discard/reload is required; locale navigation cannot silently rebase.
  await page.getByTestId("settings-reload-current").click();
  await expect(
    paymentForm.getByLabel("English name", { exact: true }),
  ).toHaveValue("Remote saved name");
  await page
    .getByRole("button", { name: "Connect account", exact: false })
    .click();
  await expect(
    accountForm.getByRole("combobox", { name: "Account", exact: true }),
  ).toHaveValue(concurrent.connectionID);
  await accountForm
    .getByRole("button", { name: "Continue", exact: true })
    .click();

  // A new tab has no draft: hydration must follow the saved method's account,
  // not the first account returned by the catalog. This is read-only acceptance.
  const fresh = await context.newPage();
  await fresh.goto(page.url());
  await fresh.getByRole("button", { name: "Continue", exact: true }).click();
  await fresh
    .getByTestId("settings-account-form")
    .getByRole("button", { name: "Continue", exact: true })
    .click();
  await fresh
    .getByRole("combobox", { name: "Selected market", exact: true })
    .selectOption(concurrent.marketID);
  await expect(
    fresh
      .getByTestId("settings-payment-form")
      .getByLabel("English name", { exact: true }),
  ).toHaveValue("Remote saved name");
  await fresh
    .getByRole("button", { name: "Connect account", exact: false })
    .click();
  await expect(
    fresh
      .getByTestId("settings-account-form")
      .getByRole("combobox", { name: "Account", exact: true }),
  ).toHaveValue(concurrent.connectionID);
  await noSecrets(fresh, [hashKey, hashIV, "H".repeat(32), "I".repeat(16)]);
  await fresh.close();

  await page
    .getByRole("button", { name: "Choose platform", exact: false })
    .click();
  await page.getByLabel("Merchant-arranged delivery", { exact: true }).check();
  await page.getByRole("button", { name: "Continue", exact: true }).click();
  await page.getByRole("button", { name: "Continue", exact: true }).click();
  const policyForm = page.getByTestId("settings-policy-form");
  await page.getByLabel("Stable service code", { exact: true }).fill("home");
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
  await expect(page.getByText("NT$ amounts are whole dollars, for example 60.")).toBeVisible();
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
  await page.reload();
  await expect(page.getByTestId("settings-status")).toContainText("home");

  await page
    .getByRole("button", { name: "Configure methods", exact: false })
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
    page.waitForResponse((r) => new URL(r.url()).pathname === "/api/auth/logout" && r.request().method() === "POST"),
    page.getByTestId("workspace-sign-out").click(),
  ]);
  const logout = logoutResponse.status();
  expect(logout).toBe(204);
});
