# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: settings-real.spec.ts >> REAL_PG A wizard creates unseeded configuration and preserves safe uncertainty
- Location: tests/admin/settings-real.spec.ts:28:1

# Error details

```
Error: a pending version read is not a saveable baseline

expect(locator).toBeDisabled() failed

Locator:  getByTestId('settings-policy-form').getByRole('button', { name: 'Save pricing policy', exact: true })
Expected: disabled
Received: enabled
Timeout:  10000ms

Call log:
  - a pending version read is not a saveable baseline getByTestId('settings-policy-form').getByRole('button', { name: 'Save pricing policy', exact: true }) with timeout 10000ms
  - waiting for getByTestId('settings-policy-form').getByRole('button', { name: 'Save pricing policy', exact: true })
    24 × locator resolved to <button type="submit" class="primary">Save pricing policy</button>
       - unexpected value "enabled"

```

```yaml
- button "Save pricing policy"
```

```
Error: expect(locator).toBeVisible() failed

Locator: getByTestId('settings-status')
Expected: visible
Timeout: 10000ms
Error: element(s) not found

Call log:
  - Expect "toBeVisible" getByTestId('settings-status') with timeout 10000ms
  - waiting for getByTestId('settings-status')

```

```yaml
- alert: Store settings
- link "Skip to content":
  - /url: "#main"
- complementary:
  - text: DaWan Live
  - navigation "Workspace navigation":
    - button "Overview"
    - button "Live & posts"
    - button "Orders & shipping"
    - button "Products & inventory"
    - button "Customers"
    - button "Marketing"
    - button "Online store"
    - button "Payments & reports"
  - navigation "Settings":
    - button "Settings" [expanded]
    - button "Store settings"
    - button "Team"
    - button "Billing"
- banner:
  - text: Switch store
  - combobox "Switch store":
    - option "Wizard Store" [selected]
  - text: Language
  - combobox "Language":
    - option "简体中文"
    - option "繁體中文"
    - option "English" [selected]
  - group: Help
  - group: Account
- main:
  - navigation "Workspace navigation":
    - link "Overview":
      - /url: /en/
    - text: Settings Store settings
  - heading "Store settings" [level=1]
  - paragraph: Set up your own account and merchant-arranged delivery.
  - paragraph:
    - link "Operations ledger":
      - /url: /en/settings/operations?store=40e1cac8-1fa2-427d-b75c-98c102525387
  - list "Delivery & payment settings":
    - listitem:
      - button "1 Choose platform"
    - listitem:
      - button "2 Connect account"
    - listitem:
      - button "3 Configure methods"
    - listitem:
      - button "4 Check status" [disabled]
  - region "Configure methods":
    - alert:
      - strong: Configuration changed. Reload before saving a corrected version.
    - status: Pricing policy saved. Save the service separately.
    - heading "Market" [level=2]
    - text: Selected market
    - combobox "Selected market":
      - option "Create market"
      - option "Taiwan retail · taiwan · TWD" [selected]
    - button "Reload saved values"
    - heading "Merchant-arranged delivery" [level=2]
    - paragraph: Policy and service are two separate saves. If service saving fails, the policy remains saved.
    - text: Country
    - textbox "Country": TW
    - text: Delivery service
    - combobox "Delivery service":
      - option "New service" [selected]
    - text: Stable service code
    - textbox "Stable service code": home
    - text: Delivery kind
    - combobox "Delivery kind":
      - option "Home delivery" [selected]
      - option "7-ELEVEN pickup"
      - option "FamilyMart pickup"
      - option "Hi-Life"
      - option "OK mart"
    - text: Mode
    - combobox "Mode Available for store-pickup services once the ECPay connection is checked and enabled." [disabled]:
      - option "Manual (you ship)" [selected]
    - text: Available for store-pickup services once the ECPay connection is checked and enabled. Flat shipping (NT$)
    - textbox "Flat shipping (NT$)": "60"
    - text: Free shipping from (NT$)
    - textbox "Free shipping from (NT$) When the goods subtotal reaches this amount the quote charges no shipping. Leave empty for no threshold."
    - text: When the goods subtotal reaches this amount the quote charges no shipping. Leave empty for no threshold. Tax mode
    - combobox "Tax mode":
      - option "No tax" [selected]
      - option "Included"
      - option "Added"
    - text: Tax basis
    - combobox "Tax basis":
      - option "Goods" [selected]
      - option "Goods and shipping"
    - text: Tax rate (basis points)
    - spinbutton "Tax rate (basis points)": "0"
    - text: Quote lifetime (seconds)
    - spinbutton "Quote lifetime (seconds)": "300"
    - checkbox "Enable pricing policy" [checked]
    - text: Enable pricing policy Configuration reference / reason
    - textbox "Configuration reference / reason Required for every policy version. Enter a new nonsecret reference; previous text is not returned."
    - text: Required for every policy version. Enter a new nonsecret reference; previous text is not returned. Simplified Chinese name
    - textbox "Simplified Chinese name": 标准配送
    - text: Traditional Chinese name
    - textbox "Traditional Chinese name": 標準配送
    - text: English name
    - textbox "English name": Standard delivery
    - text: Sort order
    - spinbutton "Sort order": "10"
    - checkbox "Enable delivery service"
    - text: Enable delivery service
    - checkbox "Visible to buyers"
    - text: Visible to buyers
    - paragraph: An active market and explicitly enabled policy are required before saving a service.
    - button "Previous"
    - button "Save delivery service"
  - complementary "Current setup status":
    - heading "Current setup status" [level=2]
    - term: Platform
    - definition: Merchant-arranged delivery
    - term: Selected market
    - definition: Taiwan retail · TWD
    - term: Delivery service
    - definition: Not configured
    - term: Enable pricing policy
    - definition: v1 · Enabled
    - heading "Continue" [level=3]
    - paragraph: An active market and explicitly enabled policy are required before saving a service.
  - region "Storefront publishing":
    - heading "Storefront publishing" [level=2]
    - paragraph: "Publishing is your decision: it lets buyers open your store. Buyers can only reach it once the platform has also bound your store's domain."
    - term: Status
    - definition: Not published
    - term: Domain
    - definition: Awaiting platform domain. The platform operator binds your store's domain after checking ownership and the certificate.
    - paragraph: Awaiting platform domain. The platform operator binds your store's domain after checking ownership and the certificate.
    - button "Publish storefront"
  - region "Store address":
    - heading "Store address" [level=2]
    - paragraph: Buyers reach your store at your platform address automatically. You can also use your own domain name.
    - list:
      - listitem: Platform address
    - text: Domain name
    - textbox "Domain name":
      - /placeholder: www.example.com
    - button "Request verification"
  - region "Facebook Page / Instagram":
    - heading "Facebook Page / Instagram" [level=2]
    - paragraph: Connect your own Facebook Page (and its Instagram account) so live-stream comments reach your claim board and buyers get the claim link by private reply. You sign in to Facebook yourself; we never see your password.
    - status:
      - paragraph: This section could not be loaded. Try again.
      - button "Try again"
  - paragraph: DaWan Live is operated by Hong Kong Da Wan Trading Limited
```

# Test source

```ts
  440 |   await fresh
  441 |     .getByRole("button", { name: "Connect account", exact: false })
  442 |     .click();
  443 |   await expect(
  444 |     fresh
  445 |       .getByTestId("settings-account-form")
  446 |       .getByRole("combobox", { name: "Account", exact: true }),
  447 |   ).toHaveValue(concurrent.connectionID);
  448 |   await noSecrets(fresh, [hashKey, hashIV, "H".repeat(32), "I".repeat(16)]);
  449 |   await fresh.close();
  450 | 
  451 |   await page
  452 |     .getByRole("button", { name: "Choose platform", exact: false })
  453 |     .click();
  454 |   await page.getByLabel("Merchant-arranged delivery", { exact: true }).check();
  455 |   await page.getByRole("button", { name: "Continue", exact: true }).click();
  456 |   await page.getByRole("button", { name: "Continue", exact: true }).click();
  457 |   const policyForm = page.getByTestId("settings-policy-form");
  458 |   // Network scheduling only: use the REAL PG replies, but hold the read baseline
  459 |   // and PUT receipt independently. A fast merchant can otherwise save before a
  460 |   // baseline arrives, or a PUT callback can overwrite a newer sibling read.
  461 |   function heldResponse() {
  462 |     let release!: () => void;
  463 |     const released = new Promise<void>((resolve) => { release = resolve; });
  464 |     let arrived = false;
  465 |     return {
  466 |       release,
  467 |       arrived: () => arrived,
  468 |       deliver: async (route: Route) => {
  469 |         const response = await route.fetch();
  470 |         arrived = true;
  471 |         await released;
  472 |         await route.fulfill({ response });
  473 |       },
  474 |     };
  475 |   }
  476 |   const policyRead = heldResponse(), serviceRead = heldResponse(), policyWrite = heldResponse();
  477 |   const policyURL = /\/delivery-services\/home\/policy$/;
  478 |   const serviceURL = /\/delivery-services\/home$/;
  479 |   await page.route(policyURL, (route) =>
  480 |     (route.request().method() === "GET" ? policyRead : policyWrite).deliver(route));
  481 |   await page.route(serviceURL, (route) => route.request().method() === "GET"
  482 |     ? serviceRead.deliver(route) : route.continue());
  483 |   const savePolicy = policyForm.getByRole("button", { name: "Save pricing policy", exact: true });
  484 |   await page.getByLabel("Stable service code", { exact: true }).fill("home");
  485 |   await expect.poll(policyRead.arrived).toBe(true);
  486 |   await expect.soft(savePolicy, "a pending version read is not a saveable baseline").toBeDisabled();
  487 |   policyRead.release();
  488 |   await expect(savePolicy).toBeEnabled();
  489 |   // stop-bleed D02: NT$ amounts are whole dollars; a decimal is refused locally with its own sentence and nothing is sent
  490 |   let policyWrites = 0;
  491 |   page.on("request", (r) => {
  492 |     if (r.method() === "PUT" && r.url().includes("/policy")) policyWrites++;
  493 |   });
  494 |   await policyForm
  495 |     .getByLabel("Flat shipping (NT$)", { exact: true })
  496 |     .fill("60.5");
  497 |   await policyForm
  498 |     .getByLabel(/^Configuration reference \/ reason/)
  499 |     .fill("Fixture explicit merchant tariff; not provider validation");
  500 |   await policyForm
  501 |     .getByRole("button", { name: "Save pricing policy", exact: true })
  502 |     .click();
  503 |   await expect(
  504 |     page.getByText("NT$ amounts are whole dollars, for example 60."),
  505 |   ).toBeVisible();
  506 |   expect(policyWrites).toBe(0);
  507 |   await policyForm
  508 |     .getByLabel("Flat shipping (NT$)", { exact: true })
  509 |     .fill("60");
  510 |   await policyForm
  511 |     .getByRole("combobox", { name: "Tax mode", exact: true })
  512 |     .selectOption("none");
  513 |   await policyForm.getByLabel("Enable pricing policy", { exact: true }).check();
  514 |   await policyForm
  515 |     .getByRole("button", { name: "Save pricing policy", exact: true })
  516 |     .click();
  517 |   await expect.poll(policyWrite.arrived).toBe(true);
  518 |   serviceRead.release();
  519 |   // READ/MEASURE only: prove the service GET has been applied while the policy
  520 |   // PUT receipt is held, not merely that its HTTP response arrived. No DOM or
  521 |   // storage mutation; only the synthetic draft's numeric version is returned.
  522 |   await expect.poll(() => page.evaluate(() => {
  523 |     const key = Object.keys(sessionStorage).find((item) => item.startsWith("commerce-settings-draft:"));
  524 |     return key ? JSON.parse(sessionStorage.getItem(key)!).draft.serviceObservation?.version : null;
  525 |   })).toBe(0);
  526 |   policyWrite.release();
  527 |   const serviceForm = page.getByTestId("settings-service-form");
  528 |   await serviceForm
  529 |     .getByLabel("Simplified Chinese name", { exact: true })
  530 |     .fill("标准配送");
  531 |   await serviceForm
  532 |     .getByLabel("Traditional Chinese name", { exact: true })
  533 |     .fill("標準配送");
  534 |   await serviceForm
  535 |     .getByLabel("English name", { exact: true })
  536 |     .fill("Standard delivery");
  537 |   await serviceForm
  538 |     .getByRole("button", { name: "Save delivery service", exact: true })
  539 |     .click();
> 540 |   await expect(page.getByTestId("settings-status")).toBeVisible();
      |                                                     ^ Error: expect(locator).toBeVisible() failed
  541 |   await page.unroute(policyURL);
  542 |   await page.unroute(serviceURL);
  543 |   await page.reload();
  544 |   await expect(page.getByTestId("settings-status")).toContainText("home");
  545 | 
  546 |   await page
  547 |     .getByRole("button", { name: "Configure methods", exact: false })
  548 |     .click();
  549 |   await expect(
  550 |     page
  551 |       .getByTestId("settings-service-form")
  552 |       .getByLabel("English name", { exact: true }),
  553 |   ).toHaveValue("Standard delivery");
  554 |   const originalCSRF = (await context.cookies()).find(
  555 |     (c) => c.name === "__Host-commerce_csrf",
  556 |   )!;
  557 |   let changedSessionWrites = 0;
  558 |   page.on("request", (r) => {
  559 |     if (
  560 |       ["POST", "PUT"].includes(r.method()) &&
  561 |       new URL(r.url()).pathname.startsWith(`/api/stores/${receipt.store_id}/`)
  562 |     )
  563 |       changedSessionWrites++;
  564 |   });
  565 |   await context.addCookies([{ ...originalCSRF, value: `${"X".repeat(42)}A` }]);
  566 |   await page
  567 |     .getByLabel("Flat shipping (NT$)", { exact: true })
  568 |     .fill("61");
  569 |   await page
  570 |     .getByLabel(/^Configuration reference \/ reason/)
  571 |     .fill("Must not be sent under changed session");
  572 |   await page
  573 |     .getByRole("button", { name: "Save pricing policy", exact: true })
  574 |     .click();
  575 |   await expect(page.getByText(/Your session changed\./)).toBeVisible();
  576 |   expect(changedSessionWrites).toBe(0);
  577 |   await context.addCookies([originalCSRF]);
  578 |   // End the real session the way the merchant does, after proving old-tab writes stayed zero: Account menu -> Sign out (a real click path; the
  579 |   // BFF answer is read from the response the click triggers).
  580 |   await page.locator("header[data-shell-topbar] summary", { hasText: /^Account$/ }).click();
  581 |   const [logoutResponse] = await Promise.all([
  582 |     page.waitForResponse(
  583 |       (r) => new URL(r.url()).pathname === "/api/auth/logout" && r.request().method() === "POST",
  584 |     ),
  585 |     page.getByTestId("workspace-sign-out").click(),
  586 |   ]);
  587 |   const logout = logoutResponse.status();
  588 |   expect(logout).toBe(204);
  589 | });
  590 | 
```