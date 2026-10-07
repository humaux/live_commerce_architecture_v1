# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: operations-ads.spec.ts >> ledger filters, details, capabilities, object links, next page and store isolation
- Location: tests/admin/operations-ads.spec.ts:96:1

# Error details

```
Error: expect(locator).not.toContainText(expected) failed

Locator: locator('body')
Expected substring: not "SYNTHETIC-DO-NOT-EXPOSE"
Received string: "Skip to contentDaWan LiveOnline storeSettingsSwitch storeTt06-go-store⌄t06-go-storet06-go-other-storeLanguage简体中文繁體中文EnglishHelpChoose a workspace section. Access is limited by your store role. For payment and shipping connections, open Store settings.AccountSign outOverview/Settings/Operations ledgerOperations ledgerReview failed or uncertain external operations. Actions depend on the server's current decision.Store settingsAd settingsLedger storet06-go-storet06-go-other-storeStateNeeds attentionFailedUnknownAcknowledgedBlocked by policyBinding changedReadyRefreshecpay.cvs_createReadyProvider: ecpay_logistics · Attempts: 010/07/2026, 18:46order · 1f9d5672-b26a-4559-9087-a094bc24012dView detailsmeta.ads.pauseReadyProvider: meta_ads · Attempts: 010/07/2026, 18:46ad_draft · eb0d7376-79fa-49dc-9a4f-0d469c57c6b4View detailsmeta.live_videosUnknownProvider: facebook · Attempts: 310/07/2026, 18:46binding · ab5363d0-374f-4da6-bcec-4ea747985405View detailsmeta.live_videosUnknownProvider: facebook · Attempts: 310/07/2026, 18:46binding · 65ba5c03-70d4-4a7e-b7ce-6e715d2a1d0cView detailsmeta.live_videosUnknownProvider: facebook · Attempts: 310/07/2026, 18:46binding · 9f2d4cf8-d0dc-4f56-9f48-bad0bf511224View detailspayment.authorizeFailedProvider: mock_provider · Attempts: 110/07/2026, 18:46—View detailsmeta.live_videosFailedProvider: facebook · Attempts: 210/07/2026, 18:46binding · 8f2cd404-d7cc-4153-9b5a-db509bc9d726View detailspayment.authorizeReadyProvider: mock_provider · Attempts: 010/07/2026, 18:46—View detailspayment.authorizeReadyProvider: mock_provider · Attempts: 010/07/2026, 18:46—View detailspayment.authorizeReadyProvider: mock_provider · Attempts: 010/07/2026, 18:46—View detailspayment.authorizeReadyProvider: mock_provider · Attempts: 010/07/2026, 18:46—View detailspayment.authorizeReadyProvider: mock_provider · Attempts: 010/07/2026, 18:46—View detailspayment.authorizeReadyProvider: mock_provider · Attempts: 010/07/2026, 18:46—View detailspayment.authorizeReadyProvider: mock_provider · Attempts: 010/07/2026, 18:46—View detailspayment.authorizeReadyProvider: mock_provider · Attempts: 010/07/2026, 18:46—View detailspayment.authorizeReadyProvider: mock_provider · Attempts: 010/07/2026, 18:46—View detailspayment.authorizeReadyProvider: mock_provider · Attempts: 010/07/2026, 18:46—View detailspayment.authorizeReadyProvider: mock_provider · Attempts: 010/07/2026, 18:46—View detailspayment.authorizeReadyProvider: mock_provider · Attempts: 010/07/2026, 18:46—View detailspayment.authorizeReadyProvider: mock_provider · Attempts: 010/07/2026, 18:46—View detailsNext pageOperation detailsClose 7625be0c-62e0-40bc-9808-924dd5fd6b25StateReadyActionecpay.cvs_createProviderecpay_logisticsAttempts0Created10/07/2026, 18:46Updated10/07/2026, 18:46Reason—Business objectorder · 1f9d5672-b26a-4559-9087-a094bc24012dSYNTHETIC-DO-NOT-EXPOSEQuery: This operation does not need reconciliation.Cancel operationAuthorize retry: The operation is already queued.Refresh Recent eventsReady · Attempts: 0Additional details are not available.10/07/2026, 18:46Confirm actionThe server will check the current operation and permissions again. Cancellation only cancels a READY local operation; it does not reverse an external action.7625be0c-62e0-40bc-9808-924dd5fd6b25BackConfirm actionDaWan Live is operated by Hong Kong Da Wan Trading Limited"
Timeout: 10000ms

Call log:
  - Expect "not toContainText" locator('body') with timeout 10000ms
  - waiting for locator('body')
    23 × locator resolved to <body>…</body>
       - unexpected value "Skip to contentDaWan LiveOnline storeSettingsSwitch storeTt06-go-store⌄t06-go-storet06-go-other-storeLanguage简体中文繁體中文EnglishHelpChoose a workspace section. Access is limited by your store role. For payment and shipping connections, open Store settings.AccountSign outOverview/Settings/Operations ledgerOperations ledgerReview failed or uncertain external operations. Actions depend on the server's current decision.Store settingsAd settingsLedger storet06-go-storet06-go-other-storeStateNeeds attentionFailedUnknownAcknowledgedBlocked by policyBinding changedReadyRefreshecpay.cvs_createReadyProvider: ecpay_logistics · Attempts: 010/07/2026, 18:46order · 1f9d5672-b26a-4559-9087-a094bc24012dView detailsmeta.ads.pauseReadyProvider: meta_ads · Attempts: 010/07/2026, 18:46ad_draft · eb0d7376-79fa-49dc-9a4f-0d469c57c6b4View detailsmeta.live_videosUnknownProvider: facebook · Attempts: 310/07/2026, 18:46binding · ab5363d0-374f-4da6-bcec-4ea747985405View detailsmeta.live_videosUnknownProvider: facebook · Attempts: 310/07/2026, 18:46binding · 65ba5c03-70d4-4a7e-b7ce-6e715d2a1d0cView detailsmeta.live_videosUnknownProvider: facebook · Attempts: 310/07/2026, 18:46binding · 9f2d4cf8-d0dc-4f56-9f48-bad0bf511224View detailspayment.authorizeFailedProvider: mock_provider · Attempts: 110/07/2026, 18:46—View detailsmeta.live_videosFailedProvider: facebook · Attempts: 210/07/2026, 18:46binding · 8f2cd404-d7cc-4153-9b5a-db509bc9d726View detailspayment.authorizeReadyProvider: mock_provider · Attempts: 010/07/2026, 18:46—View detailspayment.authorizeReadyProvider: mock_provider · Attempts: 010/07/2026, 18:46—View detailspayment.authorizeReadyProvider: mock_provider · Attempts: 010/07/2026, 18:46—View detailspayment.authorizeReadyProvider: mock_provider · Attempts: 010/07/2026, 18:46—View detailspayment.authorizeReadyProvider: mock_provider · Attempts: 010/07/2026, 18:46—View detailspayment.authorizeReadyProvider: mock_provider · Attempts: 010/07/2026, 18:46—View detailspayment.authorizeReadyProvider: mock_provider · Attempts: 010/07/2026, 18:46—View detailspayment.authorizeReadyProvider: mock_provider · Attempts: 010/07/2026, 18:46—View detailspayment.authorizeReadyProvider: mock_provider · Attempts: 010/07/2026, 18:46—View detailspayment.authorizeReadyProvider: mock_provider · Attempts: 010/07/2026, 18:46—View detailspayment.authorizeReadyProvider: mock_provider · Attempts: 010/07/2026, 18:46—View detailspayment.authorizeReadyProvider: mock_provider · Attempts: 010/07/2026, 18:46—View detailspayment.authorizeReadyProvider: mock_provider · Attempts: 010/07/2026, 18:46—View detailsNext pageOperation detailsClose 7625be0c-62e0-40bc-9808-924dd5fd6b25StateReadyActionecpay.cvs_createProviderecpay_logisticsAttempts0Created10/07/2026, 18:46Updated10/07/2026, 18:46Reason—Business objectorder · 1f9d5672-b26a-4559-9087-a094bc24012dSYNTHETIC-DO-NOT-EXPOSEQuery: This operation does not need reconciliation.Cancel operationAuthorize retry: The operation is already queued.Refresh Recent eventsReady · Attempts: 0Additional details are not available.10/07/2026, 18:46Confirm actionThe server will check the current operation and permissions again. Cancellation only cancels a READY local operation; it does not reverse an external action.7625be0c-62e0-40bc-9808-924dd5fd6b25BackConfirm actionDaWan Live is operated by Hong Kong Da Wan Trading Limited"

```

```yaml
- link "Skip to content":
  - /url: "#main"
- complementary:
  - text: DaWan Live
  - navigation "Workspace navigation":
    - button "Online store"
  - navigation "Settings":
    - button "Settings"
- banner:
  - text: Switch store
  - combobox "Switch store":
    - option "t06-go-store" [selected]
    - option "t06-go-other-store"
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
    - text: Settings Operations ledger
  - heading "Operations ledger" [level=1]
  - paragraph: Review failed or uncertain external operations. Actions depend on the server's current decision.
  - navigation "Operations ledger":
    - link "Store settings":
      - /url: /en/settings?store=a417c40a-2091-4638-ae00-3f48c4b09d59
    - link "Ad settings":
      - /url: /en/ads?store=a417c40a-2091-4638-ae00-3f48c4b09d59
  - text: Ledger store
  - combobox "Ledger store":
    - option "t06-go-store" [selected]
    - option "t06-go-other-store"
  - text: State
  - combobox "State":
    - option "Needs attention" [selected]
    - option "Failed"
    - option "Unknown"
    - option "Acknowledged"
    - option "Blocked by policy"
    - option "Binding changed"
    - option "Ready"
  - button "Refresh"
  - list:
    - listitem:
      - strong: ecpay.cvs_create
      - text: Ready
      - paragraph: "Provider: ecpay_logistics · Attempts: 0"
      - time: 10/07/2026, 18:46
      - paragraph:
        - link "order · 1f9d5672-b26a-4559-9087-a094bc24012d":
          - /url: /en/orders?store=a417c40a-2091-4638-ae00-3f48c4b09d59&order=1f9d5672-b26a-4559-9087-a094bc24012d
      - button "View details"
    - listitem:
      - strong: meta.ads.pause
      - text: Ready
      - paragraph: "Provider: meta_ads · Attempts: 0"
      - time: 10/07/2026, 18:46
      - paragraph:
        - link "ad_draft · eb0d7376-79fa-49dc-9a4f-0d469c57c6b4":
          - /url: /en/ads?store=a417c40a-2091-4638-ae00-3f48c4b09d59&draft=eb0d7376-79fa-49dc-9a4f-0d469c57c6b4
      - button "View details"
    - listitem:
      - strong: meta.live_videos
      - text: Unknown
      - paragraph: "Provider: facebook · Attempts: 3"
      - time: 10/07/2026, 18:46
      - paragraph: binding · ab5363d0-374f-4da6-bcec-4ea747985405
      - button "View details"
    - listitem:
      - strong: meta.live_videos
      - text: Unknown
      - paragraph: "Provider: facebook · Attempts: 3"
      - time: 10/07/2026, 18:46
      - paragraph: binding · 65ba5c03-70d4-4a7e-b7ce-6e715d2a1d0c
      - button "View details"
    - listitem:
      - strong: meta.live_videos
      - text: Unknown
      - paragraph: "Provider: facebook · Attempts: 3"
      - time: 10/07/2026, 18:46
      - paragraph: binding · 9f2d4cf8-d0dc-4f56-9f48-bad0bf511224
      - button "View details"
    - listitem:
      - strong: payment.authorize
      - text: Failed
      - paragraph: "Provider: mock_provider · Attempts: 1"
      - time: 10/07/2026, 18:46
      - paragraph: —
      - button "View details"
    - listitem:
      - strong: meta.live_videos
      - text: Failed
      - paragraph: "Provider: facebook · Attempts: 2"
      - time: 10/07/2026, 18:46
      - paragraph: binding · 8f2cd404-d7cc-4153-9b5a-db509bc9d726
      - button "View details"
    - listitem:
      - strong: payment.authorize
      - text: Ready
      - paragraph: "Provider: mock_provider · Attempts: 0"
      - time: 10/07/2026, 18:46
      - paragraph: —
      - button "View details"
    - listitem:
      - strong: payment.authorize
      - text: Ready
      - paragraph: "Provider: mock_provider · Attempts: 0"
      - time: 10/07/2026, 18:46
      - paragraph: —
      - button "View details"
    - listitem:
      - strong: payment.authorize
      - text: Ready
      - paragraph: "Provider: mock_provider · Attempts: 0"
      - time: 10/07/2026, 18:46
      - paragraph: —
      - button "View details"
    - listitem:
      - strong: payment.authorize
      - text: Ready
      - paragraph: "Provider: mock_provider · Attempts: 0"
      - time: 10/07/2026, 18:46
      - paragraph: —
      - button "View details"
    - listitem:
      - strong: payment.authorize
      - text: Ready
      - paragraph: "Provider: mock_provider · Attempts: 0"
      - time: 10/07/2026, 18:46
      - paragraph: —
      - button "View details"
    - listitem:
      - strong: payment.authorize
      - text: Ready
      - paragraph: "Provider: mock_provider · Attempts: 0"
      - time: 10/07/2026, 18:46
      - paragraph: —
      - button "View details"
    - listitem:
      - strong: payment.authorize
      - text: Ready
      - paragraph: "Provider: mock_provider · Attempts: 0"
      - time: 10/07/2026, 18:46
      - paragraph: —
      - button "View details"
    - listitem:
      - strong: payment.authorize
      - text: Ready
      - paragraph: "Provider: mock_provider · Attempts: 0"
      - time: 10/07/2026, 18:46
      - paragraph: —
      - button "View details"
    - listitem:
      - strong: payment.authorize
      - text: Ready
      - paragraph: "Provider: mock_provider · Attempts: 0"
      - time: 10/07/2026, 18:46
      - paragraph: —
      - button "View details"
    - listitem:
      - strong: payment.authorize
      - text: Ready
      - paragraph: "Provider: mock_provider · Attempts: 0"
      - time: 10/07/2026, 18:46
      - paragraph: —
      - button "View details"
    - listitem:
      - strong: payment.authorize
      - text: Ready
      - paragraph: "Provider: mock_provider · Attempts: 0"
      - time: 10/07/2026, 18:46
      - paragraph: —
      - button "View details"
    - listitem:
      - strong: payment.authorize
      - text: Ready
      - paragraph: "Provider: mock_provider · Attempts: 0"
      - time: 10/07/2026, 18:46
      - paragraph: —
      - button "View details"
    - listitem:
      - strong: payment.authorize
      - text: Ready
      - paragraph: "Provider: mock_provider · Attempts: 0"
      - time: 10/07/2026, 18:46
      - paragraph: —
      - button "View details"
  - button "Next page"
  - dialog "Operation details":
    - heading "Operation details" [level=2]
    - button "Close"
    - paragraph: 7625be0c-62e0-40bc-9808-924dd5fd6b25
    - term: State
    - definition: Ready
    - term: Action
    - definition: ecpay.cvs_create
    - term: Provider
    - definition: ecpay_logistics
    - term: Attempts
    - definition: "0"
    - term: Created
    - definition: 10/07/2026, 18:46
    - term: Updated
    - definition: 10/07/2026, 18:46
    - term: Reason
    - definition: —
    - term: Business object
    - definition:
      - link "order · 1f9d5672-b26a-4559-9087-a094bc24012d":
        - /url: /en/orders?store=a417c40a-2091-4638-ae00-3f48c4b09d59&order=1f9d5672-b26a-4559-9087-a094bc24012d
    - paragraph: SYNTHETIC-DO-NOT-EXPOSE
    - paragraph: "Query: This operation does not need reconciliation."
    - button "Cancel operation"
    - paragraph: "Authorize retry: The operation is already queued."
    - button "Refresh"
    - heading "Recent events" [level=3]
    - list:
      - listitem:
        - text: "Ready · Attempts: 0"
        - paragraph: Additional details are not available.
        - time: 10/07/2026, 18:46
  - paragraph: DaWan Live is operated by Hong Kong Da Wan Trading Limited
- alert
```

# Test source

```ts
  26  | async function clicked(page: Page, id: string, expected: string, verify: () => Promise<void>) {
  27  |   const row = { page: page.url(), control: id, action: "click", expected, actual: "", result: "FAIL" };
  28  |   rows.push(row);
  29  |   try { await page.getByTestId(id).click(); await verify(); row.actual = expected; row.result = "PASS"; }
  30  |   catch (error) { row.actual = error instanceof Error ? error.message : String(error); throw error; }
  31  | }
  32  | test.afterEach(async () => { await writeFile(path.join(evidence, "click-ledger.json"), JSON.stringify(rows, null, 2)); });
  33  | async function login(page: Page, target = origin) {
  34  |   // Distinct signed app fixtures share the loopback hostname; cookies are host-scoped, never port-scoped.
  35  |   await page.context().clearCookies();
  36  |   await page.goto(`${target}/en/`);
  37  |   await page.getByRole("button", { name: "Sign in with identity service" }).click();
  38  |   await expect(page.getByRole("combobox", { name: "Switch store", exact: true })).toBeVisible();
  39  |   await expect(page.getByRole("button", { name: "Sign in with identity service" })).toHaveCount(0);
  40  | }
  41  | async function ledger(page: Page, locale = "en", operation?: string) {
  42  |   await page.goto(`${origin}/${locale}/settings/operations?store=${store}${operation ? `&operation=${operation}` : ""}`);
  43  |   await expect(page.getByTestId("operations-ledger")).toBeVisible();
  44  |   if (operation) await detailLoaded(page, operation);
  45  |   else await expect(page.getByTestId(`operation-row-${ops.query}`)).toBeVisible();
  46  | }
  47  | async function detailLoaded(page: Page, id: string) {
  48  |   await expect(page.getByTestId("operation-drawer")).toBeVisible();
  49  |   await expect(page.getByTestId("operation-drawer")).toContainText(id);
  50  | }
  51  | async function foreignViewLoaded(page: Page) {
  52  |   // Positive row proves the other store's list has finished; the error proves its scoped detail request finished.
  53  |   await expect(page.getByTestId(`operation-row-${ops.foreign}`)).toBeVisible();
  54  |   await expect(page.getByTestId("operation-drawer")).toBeVisible();
  55  |   await expect(page.getByTestId("operation-drawer")).toContainText("Operations could not be loaded. Refresh to try again.");
  56  | }
  57  | async function open(page: Page, name: string) {
  58  |   await clicked(page, `operation-open-${ops[name]}`, "drawer exposes selected operation", async () => {
  59  |     await detailLoaded(page, ops[name]);
  60  |   });
  61  | }
  62  | async function action(page: Page, name: "query" | "cancel" | "retry", expected: string, verify: () => Promise<void>) {
  63  |   await clicked(page, `operation-${name}`, "confirmation dialog opens", async () => { await expect(page.getByTestId("operation-confirm")).toBeVisible(); });
  64  |   await clicked(page, "operation-confirm-submit", expected, verify);
  65  | }
  66  | async function close(page: Page) {
  67  |   await clicked(page, "operation-close", "drawer closes", async () => { await expect(page.getByTestId("operation-drawer")).not.toBeVisible(); });
  68  | }
  69  | async function filter(page: Page, value: string) {
  70  |   const pending = page.waitForResponse(r => r.url().includes(`/operations?`) && r.request().method() === "GET");
  71  |   await page.getByTestId("operations-filter").selectOption(value);
  72  |   const response = await pending; expect(response.status()).toBe(200);
  73  |   await expect(page.getByTestId("operations-filter")).toHaveValue(value);
  74  |   rows.push({ page: page.url(), control: "operations-filter", action: `selectOption(${value})`, expected: `persisted ${value} rows`, actual: `HTTP 200 and selected ${value}`, result: "PASS" });
  75  | }
  76  | async function toolbarAligned(page: Page) {
  77  |   // READ/MEASURE only. G-UI9 R2 locks adjacent action/control alignment to 4px; thresholds are untouched.
  78  |   const field = await page.getByTestId("operations-filter").boundingBox();
  79  |   const action = await page.getByTestId("operations-refresh").boundingBox();
  80  |   if (!field || !action) throw new Error("toolbar controls must have visible geometry");
  81  |   expect(Math.abs(action.y - field.y), "refresh and filter share the control row").toBeLessThanOrEqual(4);
  82  |   expect(Math.abs(action.height - field.height), "single-line control heights match").toBeLessThanOrEqual(4);
  83  | }
  84  | async function shot(page: Page, name: string, locale: string, viewport: string) {
  85  |   // READ/MEASURE only: reads layout dimensions; does not manipulate DOM or state.
  86  |   expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1);
  87  |   const file = `${name}-${locale}-${viewport}.png`;
  88  |   await page.screenshot({ path: path.join(evidence, file), animations: "disabled" });
  89  |   const manifestFile = path.join(evidence, "screenshots.json");
  90  |   let manifest: unknown[] = [];
  91  |   try { manifest = JSON.parse(await readFile(manifestFile, "utf8")) as unknown[]; } catch { /* first shot */ }
  92  |   manifest.push({ File: file, Sha256: createHash("sha256").update(await readFile(path.join(evidence, file))).digest("hex"), Locale: locale, Viewport: viewport });
  93  |   await writeFile(manifestFile, JSON.stringify(manifest, null, 2));
  94  | }
  95  | 
  96  | test("ledger filters, details, capabilities, object links, next page and store isolation", async ({ page }) => {
  97  |   await login(page); await ledger(page);
  98  |   await expect(page.getByTestId(`operation-row-${ops.query}`)).toBeVisible();
  99  |   await expect(page.getByTestId(`operation-row-${ops.foreign}`)).toHaveCount(0);
  100 |   await open(page, "query");
  101 |   await expect(page.getByTestId("operation-drawer")).toContainText("Additional details are not available.");
  102 |   const detailRefresh = page.waitForResponse(r => r.url().endsWith(`/operations/${ops.query}`) && r.request().method() === "GET");
  103 |   await clicked(page, "operation-detail-refresh", "drawer events refresh", async () => { expect((await detailRefresh).status()).toBe(200); await detailLoaded(page, ops.query); await expect(page.getByTestId("operation-drawer")).toContainText("Additional details are not available."); });
  104 |   await expect(page.getByTestId("operation-drawer")).toContainText("The result is unknown. Query");
  105 |   await expect(page.getByTestId("operation-retry")).toHaveCount(0);
  106 |   await close(page);
  107 |   await filter(page, "FAILED");
  108 |   await expect(page.getByTestId(`operation-row-${ops.retry}`)).toBeVisible();
  109 |   await expect(page.getByTestId(`operation-row-${ops.cancel}`)).toHaveCount(0);
  110 |   await open(page, "failed"); await expect(page.getByTestId("operation-retry")).toHaveCount(0);
  111 |   await expect(page.getByTestId("operation-drawer")).toContainText("does not support retry"); await close(page);
  112 |   await filter(page, "UNKNOWN"); await expect(page.getByTestId(`operation-row-${ops.query}`)).toBeVisible();
  113 |   await expect(page.getByTestId(`operation-row-${ops.retry}`)).toHaveCount(0);
  114 |   await filter(page, "READY"); await open(page, "protective");
  115 |   await expect(page.getByTestId("operation-cancel")).toHaveCount(0);
  116 |   await expect(page.getByTestId("operation-drawer")).toContainText("protective operation");
  117 |   const adLink = page.getByTestId("operation-drawer").getByRole("link").filter({ hasText: ops.draft_object });
  118 |   await expect(adLink).toHaveAttribute("href", `/en/ads?store=${store}&draft=${ops.draft_object}`);
  119 |   await adLink.click(); await expect(page).toHaveURL(new RegExp(`/en/ads\\?store=${store}&draft=${ops.draft_object}`));
  120 |   await ledger(page); await open(page, "order");
  121 |   const orderLink = page.getByTestId("operation-drawer").getByRole("link").filter({ hasText: ops.order_object });
  122 |   await expect(orderLink).toHaveAttribute("href", `/en/orders?store=${store}&order=${ops.order_object}`);
  123 |   await expect(orderLink).toBeVisible();
  124 |   // The order projection containing the request canary is loaded before checking the public UI boundary.
  125 |   await detailLoaded(page, ops.order);
> 126 |   await expect(page.locator("body")).not.toContainText("SYNTHETIC-DO-NOT-EXPOSE");
      |                                          ^ Error: expect(locator).not.toContainText(expected) failed
  127 |   await orderLink.click(); await expect(page).toHaveURL(new RegExp(`/en/orders\\?store=${store}&order=${ops.order_object}`));
  128 |   await ledger(page);
  129 |   await clicked(page, "operations-next", "second page loads", async () => { await expect(page.locator('[data-testid^="operation-row-"]')).toHaveCount(19); await expect(page.getByTestId(`operation-row-${ops.query}`)).toHaveCount(0); });
  130 |   const listRefresh = page.waitForResponse(r => r.url().includes(`/stores/${store}/operations?`) && r.request().method() === "GET");
  131 |   await clicked(page, "operations-refresh", "current page refreshed", async () => { expect((await listRefresh).status()).toBe(200); await expect(page.locator('[data-testid^="operation-row-"]')).toHaveCount(19); await expect(page.getByTestId(`operation-row-${ops.query}`)).toHaveCount(0); });
  132 |   await page.reload(); await expect(page.getByTestId(`operation-row-${ops.query}`)).toBeVisible();
  133 |   await page.getByTestId("operations-store").selectOption(otherStore);
  134 |   await expect(page).toHaveURL(new RegExp(`store=${otherStore}`));
  135 |   await expect(page.getByTestId(`operation-row-${ops.foreign}`)).toBeVisible();
  136 |   await page.goto(`${origin}/en/settings/operations?store=${otherStore}&operation=${ops.cancel}`);
  137 |   await foreignViewLoaded(page);
  138 |   await expect(page.getByTestId("operation-drawer")).not.toContainText(ops.cancel);
  139 |   await expect(page.getByTestId(`operation-row-${ops.cancel}`)).toHaveCount(0);
  140 |   await page.reload(); await foreignViewLoaded(page);
  141 |   await expect(page.getByTestId("operation-drawer")).not.toContainText(ops.cancel);
  142 |   await expect(page.getByTestId(`operation-row-${ops.cancel}`)).toHaveCount(0);
  143 | });
  144 | 
  145 | test("cancel, registered read retry and query persist after refresh", async ({ page }) => {
  146 |   await login(page); await ledger(page, "en", ops.cancel);
  147 |   await clicked(page, "operation-cancel", "confirmation opens", async () => { await expect(page.getByTestId("operation-confirm")).toBeVisible(); });
  148 |   await clicked(page, "operation-confirm-dismiss", "dismiss does not cancel", async () => { await expect(page.getByTestId("operation-cancel")).toBeVisible(); await expect(page.getByTestId("operation-confirm")).not.toBeVisible(); });
  149 |   await action(page, "cancel", "cancel persisted", async () => { await expect(page.getByTestId("operation-drawer")).toContainText("Cancelled by merchant"); });
  150 |   await page.reload(); await expect(page.getByTestId("operation-drawer")).toContainText("Cancelled by merchant");
  151 |   await ledger(page, "en", ops.retry);
  152 |   await action(page, "retry", "read-only retry queued", async () => { await expect(page.getByTestId("operation-drawer")).toContainText("Retry authorized"); });
  153 |   await page.reload(); await expect(page.getByTestId("operation-drawer")).toContainText("Retry authorized");
  154 |   await ledger(page, "en", ops.query);
  155 |   await action(page, "query", "query persisted without changing UNKNOWN", async () => { await expect(page.getByTestId("operation-drawer")).toContainText("Status query requested"); });
  156 |   await page.reload(); await expect(page.getByTestId("operation-drawer")).toContainText("Status query requested");
  157 |   await expect(page.getByTestId("operation-drawer")).toContainText("The result is unknown");
  158 |   await expect(page.getByTestId("operation-retry")).toHaveCount(0);
  159 | });
  160 | 
  161 | test("fresh server CAS and daily query cap refuse stale confirmations", async ({ page }) => {
  162 |   await login(page); await ledger(page, "en", ops.cas);
  163 |   await clicked(page, "operation-cancel", "CAS confirmation opens", async () => { await expect(page.getByTestId("operation-confirm")).toBeVisible(); });
  164 |   await control("cas");
  165 |   await clicked(page, "operation-confirm-submit", "operation_changed explains refresh", async () => { await expect(page.getByTestId("operation-feedback")).toContainText("operation changed"); });
  166 |   await page.reload(); await detailLoaded(page, ops.cas); await expect(page.getByTestId("operation-drawer")).not.toContainText("Cancelled by merchant");
  167 |   await ledger(page, "en", ops.limit);
  168 |   await clicked(page, "operation-query", "quota confirmation opens", async () => { await expect(page.getByTestId("operation-confirm")).toBeVisible(); });
  169 |   await control("limit");
  170 |   const rejected = page.waitForResponse(r => r.url().endsWith(`/operations/${ops.limit}/query`) && r.request().method() === "POST");
  171 |   await clicked(page, "operation-confirm-submit", "query_limit shows Retry-After seconds", async () => { await expect(page.getByTestId("operation-feedback")).toContainText("daily query limit"); await expect(page.getByTestId("operation-drawer")).toContainText(/Retry after \(seconds\): [1-9][0-9]*/); });
  172 |   const response = await rejected; expect(response.status()).toBe(429); expect(Number(response.headers()["retry-after"])).toBeGreaterThan(0);
  173 |   await page.reload(); await detailLoaded(page, ops.limit); await expect(page.getByTestId("operation-query-reason")).toContainText("daily query limit"); await expect(page.getByTestId("operation-query")).toHaveCount(0);
  174 | });
  175 | 
  176 | async function ads(page: Page, locale = "en") {
  177 |   await page.goto(`${adsOrigin}/${locale}/ads?store=${adsStore}`);
  178 |   await expect(page.getByTestId("merchant-ads")).toBeVisible();
  179 |   await expect(page.getByTestId("ads-connections")).toContainText(account);
  180 | }
  181 | async function unbind(page: Page, expected: string, verify: () => Promise<void>) {
  182 |   await clicked(page, "ads-unbind", "pause-first/history confirmation opens", async () => {
  183 |     await expect(page.getByTestId("ads-unbind-confirm")).toContainText(/pause/i);
  184 |     await expect(page.getByTestId("ads-unbind-confirm")).toContainText(/histor/i);
  185 |   });
  186 |   await clicked(page, "ads-unbind-yes", expected, verify);
  187 | }
  188 | 
  189 | test("read-only permissions hide mutation controls after reload", async ({ page }) => {
  190 |   await control("readonly");
  191 |   try {
  192 |     await login(page); await ledger(page, "en", ops.cas);
  193 |     await expect(page.getByTestId("operation-cancel")).toHaveCount(0);
  194 |     await expect(page.getByTestId("operation-query")).toHaveCount(0);
  195 |     await expect(page.getByTestId("operation-retry")).toHaveCount(0);
  196 |     await page.reload(); await detailLoaded(page, ops.cas); await expect(page.getByTestId("operation-cancel-reason")).toContainText("permission"); await expect(page.getByTestId("operation-cancel")).toHaveCount(0);
  197 |     await ledger(page, "en", ops.reader); await expect(page.getByTestId("operation-query")).toHaveCount(0);
  198 |     await login(page, adsOrigin); await ads(page);
  199 |     await expect(page.getByTestId("ads-feed-url")).toHaveValue(feedURL);
  200 |     await expect(page.getByTestId("ads-unbind")).toHaveCount(0);
  201 |   } finally { await control("restore"); }
  202 | });
  203 | 
  204 | test("ad unbind refuses counting/in-flight operations, then preserves history after refresh; feed copies", async ({ page, context }) => {
  205 |   await login(page, adsOrigin); await ads(page);
  206 |   await context.grantPermissions(["clipboard-read", "clipboard-write"], { origin: adsOrigin });
  207 |   await expect(page.getByTestId("ads-feed-url")).toHaveValue(feedURL);
  208 |   await clicked(page, "ads-unbind", "confirmation opens", async () => { await expect(page.getByTestId("ads-unbind-confirm")).toBeVisible(); });
  209 |   await clicked(page, "ads-unbind-cancel", "confirmation dismissed without unbinding", async () => { await expect(page.getByTestId("ads-unbind-confirm")).not.toBeVisible(); });
  210 |   await clicked(page, "ads-feed-copy", "public feed copied", async () => {
  211 |     // READ only: verifies clipboard content after the genuine button click.
  212 |     expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(feedURL);
  213 |   });
  214 |   await unbind(page, "binding_in_use explains pause first", async () => { await expect(page.getByTestId("ads-unbind-error")).toContainText(/pause/i); });
  215 |   await page.reload(); await expect(page.getByTestId("ads-unbind")).toBeVisible();
  216 |   await control("ads/inflight");
  217 |   await unbind(page, "operations_in_flight lists the concrete operation", async () => {
  218 |     await expect(page.getByTestId("ads-unbind-in-flight")).toContainText(adOperation);
  219 |     await expect(page.getByTestId("ads-unbind-in-flight")).toContainText("meta.ads.activate");
  220 |     await expect(page.getByTestId("ads-unbind-in-flight")).toContainText("Result unknown");
  221 |   });
  222 |   await clicked(page, "ads-unbind-cancel", "in-flight dialog closes", async () => { await expect(page.getByTestId("ads-unbind-confirm")).not.toBeVisible(); });
  223 |   // The same persisted in-flight operation must use merchant wording in every supported locale.
  224 |   for (const [locale, stateLabel] of [["zh-TW", "結果未知"], ["zh-CN", "结果未知"]] as const) {
  225 |     await ads(page, locale);
  226 |     await clicked(page, "ads-unbind", "localized confirmation opens", async () => { await expect(page.getByTestId("ads-unbind-confirm")).toBeVisible(); });
```