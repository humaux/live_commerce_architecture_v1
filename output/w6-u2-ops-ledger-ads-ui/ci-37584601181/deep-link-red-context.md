# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: operations-ads.spec.ts >> cancel, registered read retry and query persist after refresh
- Location: tests/admin/operations-ads.spec.ts:120:1

# Error details

```
Test timeout of 180000ms exceeded.
```

```
Error: locator.click: Test timeout of 180000ms exceeded.
Call log:
  - waiting for getByTestId('operation-cancel')
    - locator resolved to <button data-testid="operation-cancel">Cancel operation</button>
  - attempting click action
    2 × waiting for element to be visible, enabled and stable
      - element is not visible
    - retrying click action
    - waiting 20ms
    2 × waiting for element to be visible, enabled and stable
      - element is not visible
    - retrying click action
      - waiting 100ms
    348 × waiting for element to be visible, enabled and stable
        - element is not visible
      - retrying click action
        - waiting 500ms

```

# Page snapshot

```yaml
- generic [active] [ref=f2e1]:
  - generic [ref=f2e2]:
    - link "Skip to content" [ref=f2e3] [cursor=pointer]:
      - /url: "#main"
    - complementary [ref=f2e4]:
      - generic [ref=f2e5]: DaWan Live
      - navigation "Workspace navigation" [ref=f2e6]:
        - button "Online store" [ref=f2e8] [cursor=pointer]
      - navigation "Settings" [ref=f2e12]:
        - button "Settings" [ref=f2e14] [cursor=pointer]
    - generic [ref=f2e18]:
      - banner [ref=f2e19]:
        - generic [ref=f2e20]:
          - generic [ref=f2e21]: Switch store
          - generic [aria-hidden] [ref=f2e22]:
            - generic [ref=f2e23]: T
            - generic [ref=f2e24]: t06-go-store
            - generic [ref=f2e25]: ⌄
          - combobox "Switch store" [ref=f2e26] [cursor=pointer]:
            - option "t06-go-other-store"
            - option "t06-go-store" [selected]
        - generic [ref=f2e27]:
          - generic [ref=f2e28]: Language
          - combobox "Language" [ref=f2e29] [cursor=pointer]:
            - option "简体中文"
            - option "繁體中文"
            - option "English" [selected]
        - group [ref=f2e30]:
          - generic "Help" [ref=f2e31] [cursor=pointer]
        - group [ref=f2e32]:
          - generic "Account" [ref=f2e33] [cursor=pointer]
      - main [ref=f2e34]:
        - navigation "Workspace navigation" [ref=f2e35]:
          - link "Overview" [ref=f2e36] [cursor=pointer]:
            - /url: /en/
          - generic [aria-hidden] [ref=f2e37]: /
          - generic [ref=f2e38]: Settings
          - generic [aria-hidden] [ref=f2e39]: /
          - generic [ref=f2e40]: Operations ledger
        - generic [ref=f2e41]:
          - generic [ref=f2e43]:
            - heading "Operations ledger" [level=1] [ref=f2e44]
            - paragraph [ref=f2e45]: Review failed or uncertain external operations. Actions depend on the server's current decision.
          - navigation "Operations ledger" [ref=f2e46]:
            - link "Store settings" [ref=f2e47] [cursor=pointer]:
              - /url: /en/settings?store=e798228f-5567-4948-aa1a-1771c3707fdd
            - link "Ad settings" [ref=f2e48] [cursor=pointer]:
              - /url: /en/ads?store=e798228f-5567-4948-aa1a-1771c3707fdd
          - generic [ref=f2e49]:
            - text: Store settings
            - combobox "Store settings" [ref=f2e50] [cursor=pointer]:
              - option "t06-go-other-store"
              - option "t06-go-store" [selected]
          - generic [ref=f2e51]:
            - generic [ref=f2e52]:
              - text: State
              - combobox "State" [ref=f2e53] [cursor=pointer]:
                - option "Needs attention" [selected]
                - option "Failed"
                - option "Unknown"
                - option "Acknowledged"
                - option "Blocked by policy"
                - option "Binding changed"
                - option "Ready"
            - button "Refresh" [ref=f2e54] [cursor=pointer]
          - list [ref=f2e55]:
            - listitem [ref=f2e56]:
              - generic [ref=f2e57]:
                - strong [ref=f2e58]: ecpay.cvs_create
                - generic [ref=f2e59]: Ready
              - paragraph [ref=f2e60]: "Provider: ecpay_logistics · Attempts: 0"
              - time [ref=f2e61]: 10/07/2026, 17:05
              - paragraph [ref=f2e62]:
                - link "order · 30ebacdd-7184-4658-ba61-66bc11a77cbf" [ref=f2e63] [cursor=pointer]:
                  - /url: /en/orders?store=e798228f-5567-4948-aa1a-1771c3707fdd&order=30ebacdd-7184-4658-ba61-66bc11a77cbf
              - button "View details" [ref=f2e64] [cursor=pointer]
            - listitem [ref=f2e65]:
              - generic [ref=f2e66]:
                - strong [ref=f2e67]: meta.ads.pause
                - generic [ref=f2e68]: Ready
              - paragraph [ref=f2e69]: "Provider: meta_ads · Attempts: 0"
              - time [ref=f2e70]: 10/07/2026, 17:05
              - paragraph [ref=f2e71]:
                - link "ad_draft · 7f6363e8-0bc9-42c6-a5bb-cb85e0a25332" [ref=f2e72] [cursor=pointer]:
                  - /url: /en/ads?store=e798228f-5567-4948-aa1a-1771c3707fdd&draft=7f6363e8-0bc9-42c6-a5bb-cb85e0a25332
              - button "View details" [ref=f2e73] [cursor=pointer]
            - listitem [ref=f2e74]:
              - generic [ref=f2e75]:
                - strong [ref=f2e76]: meta.live_videos
                - generic [ref=f2e77]: Unknown
              - paragraph [ref=f2e78]: "Provider: facebook · Attempts: 3"
              - time [ref=f2e79]: 10/07/2026, 17:05
              - paragraph [ref=f2e80]:
                - generic [ref=f2e81]: binding · e93fab2b-cafd-4edf-8ffc-687d7a61f0d7
              - button "View details" [ref=f2e82] [cursor=pointer]
            - listitem [ref=f2e83]:
              - generic [ref=f2e84]:
                - strong [ref=f2e85]: meta.live_videos
                - generic [ref=f2e86]: Unknown
              - paragraph [ref=f2e87]: "Provider: facebook · Attempts: 3"
              - time [ref=f2e88]: 10/07/2026, 17:05
              - paragraph [ref=f2e89]:
                - generic [ref=f2e90]: binding · 6df8bdf7-486e-4292-a572-ef7f98a40929
              - button "View details" [ref=f2e91] [cursor=pointer]
            - listitem [ref=f2e92]:
              - generic [ref=f2e93]:
                - strong [ref=f2e94]: meta.live_videos
                - generic [ref=f2e95]: Unknown
              - paragraph [ref=f2e96]: "Provider: facebook · Attempts: 3"
              - time [ref=f2e97]: 10/07/2026, 17:05
              - paragraph [ref=f2e98]:
                - generic [ref=f2e99]: binding · ceec0c87-2679-4d23-8ae0-fcbc745039e1
              - button "View details" [ref=f2e100] [cursor=pointer]
            - listitem [ref=f2e101]:
              - generic [ref=f2e102]:
                - strong [ref=f2e103]: payment.authorize
                - generic [ref=f2e104]: Failed
              - paragraph [ref=f2e105]: "Provider: mock_provider · Attempts: 1"
              - time [ref=f2e106]: 10/07/2026, 17:05
              - paragraph [ref=f2e107]: —
              - button "View details" [ref=f2e108] [cursor=pointer]
            - listitem [ref=f2e109]:
              - generic [ref=f2e110]:
                - strong [ref=f2e111]: meta.live_videos
                - generic [ref=f2e112]: Failed
              - paragraph [ref=f2e113]: "Provider: facebook · Attempts: 2"
              - time [ref=f2e114]: 10/07/2026, 17:05
              - paragraph [ref=f2e115]:
                - generic [ref=f2e116]: binding · 83ff18f9-f150-478b-89bc-b68dd0538c28
              - button "View details" [ref=f2e117] [cursor=pointer]
            - listitem [ref=f2e118]:
              - generic [ref=f2e119]:
                - strong [ref=f2e120]: payment.authorize
                - generic [ref=f2e121]: Ready
              - paragraph [ref=f2e122]: "Provider: mock_provider · Attempts: 0"
              - time [ref=f2e123]: 10/07/2026, 17:05
              - paragraph [ref=f2e124]: —
              - button "View details" [ref=f2e125] [cursor=pointer]
            - listitem [ref=f2e126]:
              - generic [ref=f2e127]:
                - strong [ref=f2e128]: payment.authorize
                - generic [ref=f2e129]: Ready
              - paragraph [ref=f2e130]: "Provider: mock_provider · Attempts: 0"
              - time [ref=f2e131]: 10/07/2026, 17:05
              - paragraph [ref=f2e132]: —
              - button "View details" [ref=f2e133] [cursor=pointer]
            - listitem [ref=f2e134]:
              - generic [ref=f2e135]:
                - strong [ref=f2e136]: payment.authorize
                - generic [ref=f2e137]: Ready
              - paragraph [ref=f2e138]: "Provider: mock_provider · Attempts: 0"
              - time [ref=f2e139]: 10/07/2026, 17:05
              - paragraph [ref=f2e140]: —
              - button "View details" [ref=f2e141] [cursor=pointer]
            - listitem [ref=f2e142]:
              - generic [ref=f2e143]:
                - strong [ref=f2e144]: payment.authorize
                - generic [ref=f2e145]: Ready
              - paragraph [ref=f2e146]: "Provider: mock_provider · Attempts: 0"
              - time [ref=f2e147]: 10/07/2026, 17:05
              - paragraph [ref=f2e148]: —
              - button "View details" [ref=f2e149] [cursor=pointer]
            - listitem [ref=f2e150]:
              - generic [ref=f2e151]:
                - strong [ref=f2e152]: payment.authorize
                - generic [ref=f2e153]: Ready
              - paragraph [ref=f2e154]: "Provider: mock_provider · Attempts: 0"
              - time [ref=f2e155]: 10/07/2026, 17:05
              - paragraph [ref=f2e156]: —
              - button "View details" [ref=f2e157] [cursor=pointer]
            - listitem [ref=f2e158]:
              - generic [ref=f2e159]:
                - strong [ref=f2e160]: payment.authorize
                - generic [ref=f2e161]: Ready
              - paragraph [ref=f2e162]: "Provider: mock_provider · Attempts: 0"
              - time [ref=f2e163]: 10/07/2026, 17:05
              - paragraph [ref=f2e164]: —
              - button "View details" [ref=f2e165] [cursor=pointer]
            - listitem [ref=f2e166]:
              - generic [ref=f2e167]:
                - strong [ref=f2e168]: payment.authorize
                - generic [ref=f2e169]: Ready
              - paragraph [ref=f2e170]: "Provider: mock_provider · Attempts: 0"
              - time [ref=f2e171]: 10/07/2026, 17:05
              - paragraph [ref=f2e172]: —
              - button "View details" [ref=f2e173] [cursor=pointer]
            - listitem [ref=f2e174]:
              - generic [ref=f2e175]:
                - strong [ref=f2e176]: payment.authorize
                - generic [ref=f2e177]: Ready
              - paragraph [ref=f2e178]: "Provider: mock_provider · Attempts: 0"
              - time [ref=f2e179]: 10/07/2026, 17:05
              - paragraph [ref=f2e180]: —
              - button "View details" [ref=f2e181] [cursor=pointer]
            - listitem [ref=f2e182]:
              - generic [ref=f2e183]:
                - strong [ref=f2e184]: payment.authorize
                - generic [ref=f2e185]: Ready
              - paragraph [ref=f2e186]: "Provider: mock_provider · Attempts: 0"
              - time [ref=f2e187]: 10/07/2026, 17:05
              - paragraph [ref=f2e188]: —
              - button "View details" [ref=f2e189] [cursor=pointer]
            - listitem [ref=f2e190]:
              - generic [ref=f2e191]:
                - strong [ref=f2e192]: payment.authorize
                - generic [ref=f2e193]: Ready
              - paragraph [ref=f2e194]: "Provider: mock_provider · Attempts: 0"
              - time [ref=f2e195]: 10/07/2026, 17:05
              - paragraph [ref=f2e196]: —
              - button "View details" [ref=f2e197] [cursor=pointer]
            - listitem [ref=f2e198]:
              - generic [ref=f2e199]:
                - strong [ref=f2e200]: payment.authorize
                - generic [ref=f2e201]: Ready
              - paragraph [ref=f2e202]: "Provider: mock_provider · Attempts: 0"
              - time [ref=f2e203]: 10/07/2026, 17:05
              - paragraph [ref=f2e204]: —
              - button "View details" [ref=f2e205] [cursor=pointer]
            - listitem [ref=f2e206]:
              - generic [ref=f2e207]:
                - strong [ref=f2e208]: payment.authorize
                - generic [ref=f2e209]: Ready
              - paragraph [ref=f2e210]: "Provider: mock_provider · Attempts: 0"
              - time [ref=f2e211]: 10/07/2026, 17:05
              - paragraph [ref=f2e212]: —
              - button "View details" [ref=f2e213] [cursor=pointer]
            - listitem [ref=f2e214]:
              - generic [ref=f2e215]:
                - strong [ref=f2e216]: payment.authorize
                - generic [ref=f2e217]: Ready
              - paragraph [ref=f2e218]: "Provider: mock_provider · Attempts: 0"
              - time [ref=f2e219]: 10/07/2026, 17:05
              - paragraph [ref=f2e220]: —
              - button "View details" [ref=f2e221] [cursor=pointer]
          - button "Next page" [ref=f2e222] [cursor=pointer]
        - paragraph [ref=f2e224]: DaWan Live is operated by Hong Kong Da Wan Trading Limited
  - alert [ref=f2e225]
```

# Test source

```ts
  1   | // Purpose: W6-U2 real-click independent acceptance for operations and ads-unbind/feed.
  2   | // Depends on: production Next/BFF/Go/isolated PG started by browser_operations_ads_test.go; Playwright.
  3   | // Used by: --browser-operations-ads; server responses are never intercepted.
  4   | // Invariants: control POSTs only prepare synthetic fixtures; assertions use visible UI and reload; no force/evaluate writes.
  5   | import { expect, test, type Page } from "@playwright/test";
  6   | import { createHash } from "node:crypto";
  7   | import { readFile, writeFile } from "node:fs/promises";
  8   | import path from "node:path";
  9   | 
  10  | function required(name: string) { const v = process.env[name]; if (!v) throw new Error(`${name} required`); return v; }
  11  | const origin = required("LC_BROWSER_PUBLIC_ORIGIN"), adsOrigin = required("LC_BROWSER_ADS_ORIGIN");
  12  | const store = required("LC_BROWSER_STORE"), adsStore = required("LC_BROWSER_ADS_STORE"), otherStore = required("LC_BROWSER_OTHER_STORE");
  13  | const evidence = required("LC_BROWSER_EVIDENCE");
  14  | const ops = JSON.parse(required("LC_BROWSER_OPS")) as Record<string, string>;
  15  | const account = required("LC_BROWSER_ADS_ACCOUNT"), adOperation = required("LC_BROWSER_ADS_OPERATION"), draft = required("LC_BROWSER_ADS_DRAFT");
  16  | const feedURL = required("LC_BROWSER_FEED_URL");
  17  | const rows: { page: string; control: string; action: string; expected: string; actual: string; result: string }[] = [];
  18  | test.use({ baseURL: origin });
  19  | test.describe.configure({ mode: "serial" });
  20  | 
  21  | async function control(resource: string) {
  22  |   // Fixture preparation only: hard-coded runner listener changes synthetic PG facts, never production API/UI state.
  23  |   const r = await fetch(`${required("LC_BROWSER_CONTROL")}/${resource}`, { method: "POST", headers: { "X-Gate-Key": required("LC_BROWSER_CONTROL_KEY") } });
  24  |   expect(r.status, `fixture ${resource}`).toBe(204);
  25  | }
  26  | async function clicked(page: Page, id: string, expected: string, verify: () => Promise<void>) {
  27  |   const row = { page: page.url(), control: id, action: "click", expected, actual: "", result: "FAIL" };
  28  |   rows.push(row);
> 29  |   try { await page.getByTestId(id).click(); await verify(); row.actual = expected; row.result = "PASS"; }
      |                                    ^ Error: locator.click: Test timeout of 180000ms exceeded.
  30  |   catch (error) { row.actual = error instanceof Error ? error.message : String(error); throw error; }
  31  | }
  32  | test.afterEach(async () => { await writeFile(path.join(evidence, "click-ledger.json"), JSON.stringify(rows, null, 2)); });
  33  | async function login(page: Page, target = origin) {
  34  |   // Distinct signed app fixtures share the loopback hostname; cookies are host-scoped, never port-scoped.
  35  |   await page.context().clearCookies();
  36  |   await page.goto(`${target}/en/`);
  37  |   await page.getByRole("button", { name: "Sign in with identity service" }).click();
  38  |   await expect(page.getByRole("button", { name: "Sign in with identity service" })).toHaveCount(0);
  39  | }
  40  | async function ledger(page: Page, locale = "en", operation?: string) {
  41  |   await page.goto(`${origin}/${locale}/settings/operations?store=${store}${operation ? `&operation=${operation}` : ""}`);
  42  |   await expect(page.getByTestId("operations-ledger")).toBeVisible();
  43  |   if (operation) await expect(page.getByTestId("operation-drawer")).toContainText(operation);
  44  | }
  45  | async function open(page: Page, name: string) {
  46  |   await clicked(page, `operation-open-${ops[name]}`, "drawer exposes selected operation", async () => {
  47  |     await expect(page.getByTestId("operation-drawer")).toContainText(ops[name]);
  48  |   });
  49  | }
  50  | async function action(page: Page, name: "query" | "cancel" | "retry", expected: string, verify: () => Promise<void>) {
  51  |   await clicked(page, `operation-${name}`, "confirmation dialog opens", async () => { await expect(page.getByTestId("operation-confirm")).toBeVisible(); });
  52  |   await clicked(page, "operation-confirm-submit", expected, verify);
  53  | }
  54  | async function close(page: Page) {
  55  |   await clicked(page, "operation-close", "drawer closes", async () => { await expect(page.getByTestId("operation-drawer")).not.toBeVisible(); });
  56  | }
  57  | async function filter(page: Page, value: string) {
  58  |   const pending = page.waitForResponse(r => r.url().includes(`/operations?`) && r.request().method() === "GET");
  59  |   await page.getByTestId("operations-filter").selectOption(value);
  60  |   const response = await pending; expect(response.status()).toBe(200);
  61  |   await expect(page.getByTestId("operations-filter")).toHaveValue(value);
  62  |   rows.push({ page: page.url(), control: "operations-filter", action: `selectOption(${value})`, expected: `persisted ${value} rows`, actual: `HTTP 200 and selected ${value}`, result: "PASS" });
  63  | }
  64  | async function toolbarAligned(page: Page) {
  65  |   // READ/MEASURE only. G-UI9 R2 locks adjacent action/control alignment to 4px; thresholds are untouched.
  66  |   const field = await page.getByTestId("operations-filter").boundingBox();
  67  |   const action = await page.getByTestId("operations-refresh").boundingBox();
  68  |   if (!field || !action) throw new Error("toolbar controls must have visible geometry");
  69  |   expect(Math.abs(action.y - field.y), "refresh and filter share the control row").toBeLessThanOrEqual(4);
  70  |   expect(Math.abs(action.height - field.height), "single-line control heights match").toBeLessThanOrEqual(4);
  71  | }
  72  | async function shot(page: Page, name: string, locale: string, viewport: string) {
  73  |   // READ/MEASURE only: reads layout dimensions; does not manipulate DOM or state.
  74  |   expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1);
  75  |   const file = `${name}-${locale}-${viewport}.png`;
  76  |   await page.screenshot({ path: path.join(evidence, file), animations: "disabled" });
  77  |   const manifestFile = path.join(evidence, "screenshots.json");
  78  |   let manifest: unknown[] = [];
  79  |   try { manifest = JSON.parse(await readFile(manifestFile, "utf8")) as unknown[]; } catch { /* first shot */ }
  80  |   manifest.push({ File: file, Sha256: createHash("sha256").update(await readFile(path.join(evidence, file))).digest("hex"), Locale: locale, Viewport: viewport });
  81  |   await writeFile(manifestFile, JSON.stringify(manifest, null, 2));
  82  | }
  83  | 
  84  | test("ledger filters, details, capabilities, object links, next page and store isolation", async ({ page }) => {
  85  |   await login(page); await ledger(page);
  86  |   await expect(page.getByTestId(`operation-row-${ops.foreign}`)).toHaveCount(0);
  87  |   await open(page, "query");
  88  |   await expect(page.getByTestId("operation-drawer")).toContainText("synthetic_unknown");
  89  |   await clicked(page, "operation-detail-refresh", "drawer events refresh", async () => { await expect(page.getByTestId("operation-drawer")).toContainText("synthetic_unknown"); });
  90  |   await expect(page.getByTestId("operation-drawer")).toContainText("The result is unknown. Query");
  91  |   await expect(page.getByTestId("operation-retry")).toHaveCount(0);
  92  |   await close(page);
  93  |   await filter(page, "FAILED");
  94  |   await expect(page.getByTestId(`operation-row-${ops.retry}`)).toBeVisible();
  95  |   await expect(page.getByTestId(`operation-row-${ops.cancel}`)).toHaveCount(0);
  96  |   await open(page, "failed"); await expect(page.getByTestId("operation-retry")).toHaveCount(0);
  97  |   await expect(page.getByTestId("operation-drawer")).toContainText("does not support retry"); await close(page);
  98  |   await filter(page, "UNKNOWN"); await expect(page.getByTestId(`operation-row-${ops.query}`)).toBeVisible();
  99  |   await expect(page.getByTestId(`operation-row-${ops.retry}`)).toHaveCount(0);
  100 |   await filter(page, "READY"); await open(page, "protective");
  101 |   await expect(page.getByTestId("operation-cancel")).toHaveCount(0);
  102 |   await expect(page.getByTestId("operation-drawer")).toContainText("protective operation");
  103 |   const adLink = page.getByTestId("operation-drawer").getByRole("link").filter({ hasText: ops.draft_object });
  104 |   await expect(adLink).toHaveAttribute("href", `/en/ads?store=${store}&draft=${ops.draft_object}`);
  105 |   await adLink.click(); await expect(page).toHaveURL(new RegExp(`/en/ads\\?store=${store}&draft=${ops.draft_object}`));
  106 |   await ledger(page); await open(page, "order");
  107 |   const orderLink = page.getByTestId("operation-drawer").getByRole("link").filter({ hasText: ops.order_object });
  108 |   await expect(orderLink).toHaveAttribute("href", `/en/orders?store=${store}&order=${ops.order_object}`);
  109 |   await orderLink.click(); await expect(page).toHaveURL(new RegExp(`/en/orders\\?store=${store}&order=${ops.order_object}`));
  110 |   await ledger(page); await expect(page.locator("body")).not.toContainText("SYNTHETIC-DO-NOT-EXPOSE");
  111 |   await clicked(page, "operations-next", "second page loads", async () => { await expect(page.locator('[data-testid^="operation-row-"]')).toHaveCount(19); await expect(page.getByTestId(`operation-row-${ops.query}`)).toHaveCount(0); });
  112 |   await clicked(page, "operations-refresh", "current page refreshed", async () => { await expect(page.locator('[data-testid^="operation-row-"]')).toHaveCount(19); await expect(page.getByTestId(`operation-row-${ops.query}`)).toHaveCount(0); });
  113 |   await page.reload(); await expect(page.getByTestId(`operation-row-${ops.query}`)).toBeVisible();
  114 |   await page.goto(`${origin}/en/settings/operations?store=${otherStore}&operation=${ops.cancel}`);
  115 |   await expect(page.getByTestId("operation-drawer")).not.toContainText(ops.cancel);
  116 |   await expect(page.getByTestId(`operation-row-${ops.cancel}`)).toHaveCount(0);
  117 |   await page.reload(); await expect(page.getByTestId(`operation-row-${ops.cancel}`)).toHaveCount(0);
  118 | });
  119 | 
  120 | test("cancel, registered read retry and query persist after refresh", async ({ page }) => {
  121 |   await login(page); await ledger(page, "en", ops.cancel);
  122 |   await clicked(page, "operation-cancel", "confirmation opens", async () => { await expect(page.getByTestId("operation-confirm")).toBeVisible(); });
  123 |   await clicked(page, "operation-confirm-dismiss", "dismiss does not cancel", async () => { await expect(page.getByTestId("operation-confirm")).not.toBeVisible(); });
  124 |   await action(page, "cancel", "cancel persisted", async () => { await expect(page.getByTestId("operation-drawer")).toContainText("cancelled_by_merchant"); });
  125 |   await page.reload(); await expect(page.getByTestId("operation-drawer")).toContainText("cancelled_by_merchant");
  126 |   await ledger(page, "en", ops.retry);
  127 |   await action(page, "retry", "read-only retry queued", async () => { await expect(page.getByTestId("operation-drawer")).toContainText("retry_authorized"); });
  128 |   await page.reload(); await expect(page.getByTestId("operation-drawer")).toContainText("retry_authorized");
  129 |   await ledger(page, "en", ops.query);
```