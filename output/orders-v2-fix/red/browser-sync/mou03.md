# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: orders-ui.spec.ts >> MOU03 delayed old success/error cannot repaint store, filter, locale or new session
- Location: tests/admin/orders-ui.spec.ts:583:1

# Error details

```
Test timeout of 30000ms exceeded.
```

```
Error: locator.click: Test timeout of 30000ms exceeded.
Call log:
  - waiting for getByTestId('nav-group-orders')

```

# Page snapshot

```yaml
- generic [active] [ref=f2e1]:
  - generic [ref=f2e2]:
    - link "Skip to content" [ref=f2e3] [cursor=pointer]:
      - /url: "#main"
    - complementary [ref=f2e4]:
      - generic [ref=f2e5]:
        - generic [aria-hidden] [ref=f2e6]: M
        - generic "merchant order foreign store" [ref=f2e7]
      - navigation "Workspace navigation" [ref=f2e8]:
        - button "Overview" [ref=f2e10] [cursor=pointer]
        - button "Orders & shipping" [ref=f2e15] [cursor=pointer]
        - button "Payments & reports" [ref=f2e20] [cursor=pointer]
    - generic [ref=f2e24]:
      - banner [ref=f2e25]:
        - generic [ref=f2e26]:
          - generic [ref=f2e27]: Switch store
          - combobox "Switch store" [ref=f2e28] [cursor=pointer]:
            - option "payment mock store"
            - option "merchant order foreign store" [selected]
        - generic [ref=f2e29]:
          - generic [ref=f2e30]: Language
          - combobox "Language" [ref=f2e31] [cursor=pointer]:
            - option "简体中文"
            - option "繁體中文"
            - option "English" [selected]
        - group [ref=f2e32]:
          - generic "Help" [ref=f2e33] [cursor=pointer]
        - group [ref=f2e34]:
          - generic "Account" [ref=f2e35] [cursor=pointer]
      - main [ref=f2e36]:
        - navigation "Workspace navigation" [ref=f2e37]:
          - link "Overview" [ref=f2e38] [cursor=pointer]:
            - /url: /en/
        - generic [ref=f2e39]:
          - generic [ref=f2e40]:
            - heading "Dashboard" [level=1] [ref=f2e41]
            - paragraph [ref=f2e42]: What needs you today. Days are Taipei calendar days (UTC+8).
          - generic [ref=f2e44]:
            - text: Store
            - combobox "Store" [ref=f2e45]:
              - option "payment mock store"
              - option "merchant order foreign store" [selected]
          - region "To do" [ref=f2e46]:
            - heading "To do" [level=2] [ref=f2e47]
            - list [ref=f2e48]:
              - listitem [ref=f2e49]:
                - link "Transfers to confirm 0" [ref=f2e50] [cursor=pointer]:
                  - /url: /en/orders?state=AWAITING_TRANSFER&store=a581836c-d930-4cf1-a5a4-2f7c469e4eca
                  - generic [ref=f2e51]: Transfers to confirm
                  - generic [ref=f2e52]: "0"
              - listitem [ref=f2e53]:
                - link "Orders to ship 0" [ref=f2e54] [cursor=pointer]:
                  - /url: /en/orders?state=unshipped&store=a581836c-d930-4cf1-a5a4-2f7c469e4eca
                  - generic [ref=f2e55]: Orders to ship
                  - generic [ref=f2e56]: "0"
              - listitem [ref=f2e57]:
                - link "Convenience-store labels to create 0" [ref=f2e58] [cursor=pointer]:
                  - /url: /en/orders?state=unshipped&store=a581836c-d930-4cf1-a5a4-2f7c469e4eca
                  - generic [ref=f2e59]: Convenience-store labels to create
                  - generic [ref=f2e60]: "0"
              - listitem [ref=f2e61]:
                - link "Low-stock variants (5 or fewer) 0" [ref=f2e62] [cursor=pointer]:
                  - /url: /en/inventory?store=a581836c-d930-4cf1-a5a4-2f7c469e4eca
                  - generic [ref=f2e63]: Low-stock variants (5 or fewer)
                  - generic [ref=f2e64]: "0"
              - listitem [ref=f2e65]:
                - link "Open refunds 0" [ref=f2e66] [cursor=pointer]:
                  - /url: /en/orders?store=a581836c-d930-4cf1-a5a4-2f7c469e4eca
                  - generic [ref=f2e67]: Open refunds
                  - generic [ref=f2e68]: "0"
            - paragraph [ref=f2e69]: Nothing waiting
          - generic [ref=f2e70]:
            - generic [ref=f2e71]:
              - heading "Orders" [level=2] [ref=f2e72]
              - generic [ref=f2e73]:
                - generic [ref=f2e74]:
                  - term [ref=f2e75]: Today
                  - definition [ref=f2e76]: "0"
                - generic [ref=f2e77]:
                  - term [ref=f2e78]: Last 7 days
                  - definition [ref=f2e79]: "0"
              - paragraph [ref=f2e80]: "Placed orders: waiting for payment, waiting for a transfer or confirmed."
            - generic [ref=f2e81]:
              - heading "Sales by payment method" [level=2] [ref=f2e82]
              - paragraph [ref=f2e83]: No sales in the last 7 days.
          - generic [ref=f2e84]:
            - heading "Latest orders" [level=2] [ref=f2e85]
            - table [ref=f2e87]:
              - rowgroup [ref=f2e88]:
                - row [ref=f2e89]:
                  - columnheader "Order" [ref=f2e90]
                  - columnheader "Created (Taipei time)" [ref=f2e91]
                  - columnheader "Status" [ref=f2e92]
                  - columnheader "Payment" [ref=f2e93]
                  - columnheader "Total" [ref=f2e94]
              - rowgroup [ref=f2e95]:
                - row [ref=f2e96]:
                  - cell [ref=f2e97]:
                    - link "1dd66b47" [ref=f2e98] [cursor=pointer]:
                      - /url: /en/orders?order=1dd66b47-7f92-4134-96a8-a7ef74abe489&store=a581836c-d930-4cf1-a5a4-2f7c469e4eca
                  - cell "10/03/2026, 13:28" [ref=f2e99]
                  - cell "Draft" [ref=f2e100]
                  - cell "Card" [ref=f2e101]
                  - cell "NT$12.50" [ref=f2e102]
            - paragraph [ref=f2e103]:
              - link "All orders" [ref=f2e104] [cursor=pointer]:
                - /url: /en/orders?store=a581836c-d930-4cf1-a5a4-2f7c469e4eca
          - region "Quick actions" [ref=f2e105]:
            - heading "Quick actions" [level=2] [ref=f2e106]
            - generic [ref=f2e107]:
              - link "Create an order" [ref=f2e108] [cursor=pointer]:
                - /url: /en/orders/new?store=a581836c-d930-4cf1-a5a4-2f7c469e4eca
              - link "Import or export products" [ref=f2e109] [cursor=pointer]:
                - /url: /en/products/import?store=a581836c-d930-4cf1-a5a4-2f7c469e4eca
              - link "Inventory" [ref=f2e110] [cursor=pointer]:
                - /url: /en/inventory?store=a581836c-d930-4cf1-a5a4-2f7c469e4eca
  - alert [ref=f2e111]
```

# Test source

```ts
  1   | import { expect, test, type BrowserContext, type Page } from "@playwright/test";
  2   | import { mkdir, writeFile } from "node:fs/promises";
  3   | import { resolve } from "node:path";
  4   | import * as http from "node:http";
  5   | import { nativePage } from "./fixtures/native-device";
  6   | 
  7   | const required = (name: string) => {
  8   |   const value = process.env[name];
  9   |   if (!value) throw new Error(`${name} is required`);
  10  |   return value;
  11  | };
  12  | const origin = required("LC_BROWSER_PUBLIC_ORIGIN");
  13  | const apiOrigin = required("LC_BROWSER_API_ORIGIN");
  14  | const evidence = required("LC_BROWSER_EVIDENCE");
  15  | const store = required("LC_BROWSER_ORDER_STORE");
  16  | const ids = JSON.parse(required("LC_BROWSER_ORDER_IDS")) as Record<
  17  |   string,
  18  |   string
  19  | >;
  20  | const frozenSKU = required("LC_BROWSER_FROZEN_SKU_CODE");
  21  | const foreignStore = required("LC_BROWSER_FOREIGN_STORE");
  22  | const foreignOrder = required("LC_BROWSER_FOREIGN_ORDER_ID");
  23  | const unlistedStore = required("LC_BROWSER_UNLISTED_STORE");
  24  | const secondToken = required("LC_BROWSER_SECOND_TOKEN");
  25  | const noOrdersToken = required("LC_BROWSER_NO_ORDERS_TOKEN");
  26  | const expiredToken = required("LC_BROWSER_EXPIRED_TOKEN");
  27  | const revokedToken = required("LC_BROWSER_REVOKED_TOKEN");
  28  | const cookieName = "__Host-commerce_session";
  29  | const pii = ["Synthetic Buyer", "+886900000001", "Synthetic home address"];
  30  | const v2Evidence = resolve(evidence, "../../../orders-v2-fix");
  31  | 
  32  | test.use({
  33  |   baseURL: origin,
  34  |   headless: false,
  35  |   // Playwright disables native bfcache by default. This lifecycle suite must
  36  |   // exercise browser restoration, not force every history return to reload.
  37  |   launchOptions: { ignoreDefaultArgs: ["--disable-back-forward-cache"] },
  38  |   trace: "retain-on-failure",
  39  |   screenshot: "only-on-failure",
  40  | });
  41  | 
  42  | async function signedLogin(page: Page) {
  43  |   await page.goto(new URL("/en/", origin).toString());
  44  |   await page
  45  |     .getByRole("button", { name: "Sign in with identity service" })
  46  |     .click();
  47  |   await expect(page.getByTestId("nav-orders")).toBeVisible();
  48  |   const before = await detailCalls(page);
  49  |   await page.getByTestId("nav-orders").click();
  50  |   await expect(page.getByTestId("merchant-orders")).toBeVisible();
  51  |   await expect(page).toHaveURL(new RegExp(`/en/orders`));
  52  |   const selector = page.getByTestId("shell-store-selector");
  53  |   if ((await selector.inputValue()) !== store)
  54  |     await switchOrderStore(page, store);
  55  |   await expect(page.getByTestId("orders-table")).toBeVisible();
  56  |   // These frozen MOU scenarios explicitly inspect drafts. v2's default is tested
  57  |   // separately; choose the all-states inspection scope via the actual control.
  58  |   await page.getByTestId("state-filter").selectOption("all");
  59  |   await expect(page).toHaveURL(/state=all/);
  60  |   await expect(page.getByTestId("orders-table")).toBeVisible();
  61  |   expect(await detailCalls(page)).toBe(before);
  62  | }
  63  | 
  64  | async function switchOrderStore(page: Page, next: string) {
  65  |   // W0 owns store navigation: a real switch goes through overview, not a page-local setter.
  66  |   await page.getByTestId("shell-store-selector").selectOption(next);
  67  |   await expect(page).toHaveURL(new URL(`/en?store=${next}`, origin).href);
  68  |   await expect(page.getByTestId("merchant-orders")).toHaveCount(0);
  69  |   if (!(await page.getByTestId("nav-orders").isVisible()))
> 70  |     await page.getByTestId("nav-group-orders").click();
      |                                                ^ Error: locator.click: Test timeout of 30000ms exceeded.
  71  |   await page.getByTestId("nav-orders").click();
  72  |   await expect(page).toHaveURL(new RegExp(`/en/orders\\?store=${next}$`));
  73  |   await expect(page.getByTestId("shell-store-selector")).toHaveValue(next);
  74  |   await expect(page.getByTestId("store-selector")).toHaveCount(0);
  75  | }
  76  | 
  77  | async function session(context: BrowserContext, token: string) {
  78  |   await context.addCookies([
  79  |     {
  80  |       name: cookieName,
  81  |       value: token,
  82  |       url: origin.replace(/^http:/, "https:"),
  83  |       secure: true,
  84  |       httpOnly: true,
  85  |       sameSite: "Lax",
  86  |     },
  87  |   ]);
  88  | }
  89  | 
  90  | async function browserRead(page: Page, path: string) {
  91  |   // G-UI8 audit [READ/MEASURE]: same-origin GET read of server state through the BFF (no state change)
  92  |   return page.evaluate(async (target) => {
  93  |     const response = await fetch(target, {
  94  |       credentials: "same-origin",
  95  |       cache: "no-store",
  96  |     });
  97  |     return {
  98  |       status: response.status,
  99  |       cache: response.headers.get("cache-control"),
  100 |       body: await response.text(),
  101 |     };
  102 |   }, path);
  103 | }
  104 | 
  105 | async function raw(path: string, token: string) {
  106 |   const target = new URL(origin);
  107 |   return new Promise<{
  108 |     status: number;
  109 |     cache: string | undefined;
  110 |     cookies: string[];
  111 |     body: string;
  112 |   }>((resolve, reject) => {
  113 |     const request = http.request(
  114 |       {
  115 |         hostname: target.hostname,
  116 |         port: target.port,
  117 |         path,
  118 |         method: "GET",
  119 |         headers: { Cookie: `${cookieName}=${token}` },
  120 |         timeout: 5000,
  121 |       },
  122 |       (response) => {
  123 |         const chunks: Buffer[] = [];
  124 |         response.on("data", (chunk: Buffer) => chunks.push(chunk));
  125 |         response.on("end", () =>
  126 |           resolve({
  127 |             status: response.statusCode ?? 0,
  128 |             cache: response.headers["cache-control"],
  129 |             cookies: response.headers["set-cookie"] ?? [],
  130 |             body: Buffer.concat(chunks).toString("utf8"),
  131 |           }),
  132 |         );
  133 |       },
  134 |     );
  135 |     request.on("error", reject);
  136 |     request.on("timeout", () =>
  137 |       request.destroy(new Error("raw request timed out")),
  138 |     );
  139 |     request.end();
  140 |   });
  141 | }
  142 | 
  143 | async function noPII(page: Page) {
  144 |   const content = await page.locator("body").innerText();
  145 |   for (const value of pii) expect(content).not.toContain(value);
  146 |   await expect(page.getByTestId("order-detail")).toHaveCount(0);
  147 | }
  148 | 
  149 | async function noPersistentOrderBody(page: Page) {
  150 |   // G-UI8 audit [READ/MEASURE]: scans client storage for secrets/PII (read only) + CacheStorage names
  151 |   const storage = await page.evaluate(async () => {
  152 |     const cacheNames = "caches" in window ? await caches.keys() : [];
  153 |     return JSON.stringify({
  154 |       local: { ...localStorage },
  155 |       session: { ...sessionStorage },
  156 |       cacheNames,
  157 |     });
  158 |   });
  159 |   for (const value of pii) expect(storage).not.toContain(value);
  160 |   expect(storage).not.toContain(ids.pending);
  161 | }
  162 | 
  163 | async function detailCalls(page: Page) {
  164 |   const response = await page.request.get(
  165 |     `${apiOrigin}/__test/order-observation`,
  166 |   );
  167 |   expect(response.status()).toBe(200);
  168 |   return ((await response.json()) as { details: number }).details;
  169 | }
  170 | 
```