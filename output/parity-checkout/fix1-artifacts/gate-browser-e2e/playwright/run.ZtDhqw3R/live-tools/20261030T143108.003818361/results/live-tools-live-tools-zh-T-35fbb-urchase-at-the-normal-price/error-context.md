# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: live-tools.spec.ts >> live tools zh-TW mobile: library import, live price, signed Meta claim, pay at pickup at the live price; direct purchase at the normal price
- Location: tests/e2e/live-tools.spec.ts:125:3

# Error details

```
TimeoutError: locator.selectOption: Timeout 20000ms exceeded.
Call log:
  - waiting for locator('#delivery')
    - locator resolved to <select id="delivery">…</select>
  - attempting select option action
    2 × waiting for element to be visible and enabled
      - did not find some options
    - retrying select option action
    - waiting 20ms
    2 × waiting for element to be visible and enabled
      - did not find some options
    - retrying select option action
      - waiting 100ms
    39 × waiting for element to be visible and enabled
       - did not find some options
     - retrying select option action
       - waiting 500ms

```

# Test source

```ts
  1   | // Purpose: real-click live-price claim checkout versus ordinary catalog-price purchase regression.
  2   | // Depends on: production storefront/admin, real PG and signed MOCK Meta/IdP/provider fixtures.
  3   | // Used by: --browser-e2e TestBrowserLiveTools; no LIVE provider acceptance.
  4   | // Live tools (R4) browser gate: keyword library import + live-only price, claimed through a signed MOCK Meta comment, bought with pay at pickup
  5   | // at the live price, while the same SKU bought directly from the product page pays the normal price. Written from the contract
  6   | // (contracts/live-keyword-claims-v1.md, amendment "Live tools (R4)") by the independent test author. Started only by
  7   | // tests/foundation/browser_live_tools_test.go (TestBrowserLiveTools), which owns every process (production admin + storefront Next builds, Go APIs,
  8   | // PostgreSQL, Meta consumer/poller/dispatcher, fake Graph, signed MOCK IdP, the disposable buyer.example TLS edge + CONNECT proxy) and the
  9   | // runner-only control listener. The browser never touches Go directly; things the outside world does (a Meta comment arrives) and every PostgreSQL
  10  | // assertion go through POST {control}/act?name=... which Go executes on its test goroutine.
  11  | //
  12  | // Matrix (four cells, one scene/post/SKU each, one stack): zh-TW x desktop (1440), zh-TW x 390 px, en x desktop, en x 390 px. Merchant and buyer both
  13  | // run at the cell's viewport. BFF routes exercised: /api/stores/{store}/live-sessions (+ claims library, offer-import, offers, window, claim-source) on
  14  | // the admin side; /api/buyer/session, cart, quotes, destinations, cvs-stores, orders on the storefront side.
  15  | import { expect, test, type BrowserContext, type Locator, type Page } from "@playwright/test";
  16  | import { createHash } from "node:crypto";
  17  | import { readFile, writeFile } from "node:fs/promises";
  18  | import path from "node:path";
  19  | import { claimsCopy, hostPrompt } from "../../apps/admin/lib/claims-copy";
  20  | import { studioCopy } from "../../apps/admin/lib/studio-copy";
  21  | import { claimCopy } from "../../apps/storefront/lib/claim-copy";
  22  | import { cvsCopy } from "../../apps/storefront/lib/cvs-copy";
  23  | import { purchaseCopy } from "../../apps/storefront/lib/purchase-copy";
  24  | import { shopCopy } from "../../apps/storefront/lib/shop-copy";
  25  | 
  26  | const required = (name: string) => {
  27  |   const value = process.env[name];
  28  |   if (!value) throw new Error(`${name} is required`);
  29  |   return value;
  30  | };
  31  | const admin = required("LC_LT_ADMIN_ORIGIN");
  32  | const buyerOrigin = required("LC_LT_BUYER_ORIGIN");
  33  | const evidence = required("LC_LT_EVIDENCE");
  34  | const control = required("LC_LT_CONTROL");
  35  | const controlKey = required("LC_LT_CONTROL_KEY");
  36  | const store = required("LC_LT_STORE");
  37  | 
  38  | test.use({ launchOptions: { proxy: { server: required("LC_LT_PROXY"), bypass: "127.0.0.1" } } });
  39  | test.setTimeout(900_000);
  40  | 
  41  | type Locale = "zh-TW" | "en";
  42  | type Viewport = "desktop" | "mobile";
  43  | const sizes = { desktop: { width: 1440, height: 900 }, mobile: { width: 390, height: 844 } } as const;
  44  | const cells: { n: number; locale: Locale; viewport: Viewport }[] = [
  45  |   { n: 1, locale: "zh-TW", viewport: "desktop" },
  46  |   { n: 2, locale: "zh-TW", viewport: "mobile" },
  47  |   { n: 3, locale: "en", viewport: "desktop" },
  48  |   { n: 4, locale: "en", viewport: "mobile" },
  49  | ];
  50  | // 30000 minor = 300.00 normal price, 20000 = 200.00 live price, quantity 2 on the claim and 1 on the direct purchase.
  51  | const live = { total: "400", unit: "200", normalTotal: "600", normalUnit: "300" };
  52  | // Same rule as tests/storefront/amount-match.mjs before the number: no ASCII letter/digit/","/"." ("NT$400" matches, hex "f200c" does not).
  53  | // After it only a digit is refused: rendered text joins elements, so a label can follow directly ("NT$200NT$300直播價").
  54  | const has = (n: string) => new RegExp(`(?<![0-9A-Za-z,.])${n}(\\.00)?([^0-9]|$)`);
  55  | // Negative price checks scan whole sections, which also show fixture ids built from random hex: the order UUID
  56  | // ("2ef200c7-…"), the SKU code "LT<n>-<TAG>" and the product name "Live tools product <n> <TAG>". Remove exactly those
  57  | // fixture formats first so an id never reads as a price (PR #33 flake: "must not show 200" matched UUID "…f200c…").
  58  | // Positive checks and the amount matcher are unchanged; no rendered amount can match these id formats.
  59  | const withoutFixtureIds = (s: string) =>
  60  |   s.replace(/[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}/gi, " ").replace(/\bLT\d+-[0-9A-Z]+\b/g, " ").replace(/Live tools product \d+ [0-9A-Z]+/g, " ");
  61  | 
  62  | async function act(name: string, body: Record<string, unknown> = {}) {
  63  |   const response = await fetch(`${control}/act?name=${name}`, { method: "POST", headers: { "X-Gate-Key": controlKey, "content-type": "application/json" }, body: JSON.stringify(body) });
  64  |   const answer = (await response.json().catch(() => ({}))) as Record<string, any>;
  65  |   expect(response.status, `control ${name}: ${JSON.stringify(answer)}`).toBe(200);
  66  |   return answer;
  67  | }
  68  | 
  69  | const manifestPath = path.join(evidence, "screenshots.json");
  70  | async function shot(page: Page, name: string, locale: string, viewport: Viewport) {
  71  |   const file = path.join(evidence, `${name}-${locale}-${viewport}.png`);
  72  |   await page.screenshot({ path: file, fullPage: true, animations: "disabled" });
  73  |   expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth), `horizontal overflow: ${name} ${locale} ${viewport}`).toBeLessThanOrEqual(1);
  74  |   let manifest: unknown[] = [];
  75  |   try {
  76  |     manifest = JSON.parse(await readFile(manifestPath, "utf8"));
  77  |   } catch {
  78  |     /* first screenshot */
  79  |   }
  80  |   manifest.push({ File: path.basename(file), Sha256: createHash("sha256").update(await readFile(file)).digest("hex"), Locale: locale, Viewport: viewport });
  81  |   await writeFile(manifestPath, JSON.stringify(manifest, null, 2));
  82  | }
  83  | 
  84  | const pageErrors: string[] = [];
  85  | 
  86  | // Delivery -> quotation -> buyer-entered 7-ELEVEN store -> pay at pickup -> place the order. Returns the order id and the quote section text.
  87  | async function checkoutPayAtPickup(page: Page, locale: Locale, viewport: Viewport, label: string, expectTotal: string, notTotal: string) {
  88  |   const purchase = purchaseCopy[locale];
  89  |   const cvs = cvsCopy[locale];
  90  |   await page.goto(`${buyerOrigin}/${locale}/checkout`);
  91  |   await page.getByRole("button", { name: purchase.delivery, exact: true }).click();
> 92  |   await page.locator("#delivery").selectOption({ label: `${cvs.chains.cvs_711} · TW` });
      |                                   ^ TimeoutError: locator.selectOption: Timeout 20000ms exceeded.
  93  |   const quotation = page.waitForResponse((r) => new URL(r.url()).pathname === "/api/buyer/quotes" && r.request().method() === "POST");
  94  |   await page.getByRole("button", { name: purchase.quote, exact: true }).click();
  95  |   expect((await quotation).status()).toBe(200);
  96  |   await expect(page.getByTestId("address-section")).toBeVisible();
  97  |   const quoted = page.locator(".quotation");
  98  |   await expect(quoted, `${label}: the quotation shows the ${expectTotal} total`).toContainText(has(expectTotal));
  99  |   expect(withoutFixtureIds(await quoted.innerText()), `${label}: the quotation must not show ${notTotal}`).not.toMatch(has(notTotal));
  100 |   await page.getByTestId("cvs-recipient-name").fill("王小明");
  101 |   await page.getByTestId("cvs-recipient-phone").fill("0912345678");
  102 |   await page.getByTestId("cvs-entered-code").fill("131386");
  103 |   await expect(page.getByTestId("cvs-entered-name")).toHaveCount(0);
  104 |   await expect(page.getByTestId("cvs-entered-address")).toHaveCount(0);
  105 |   await page.getByRole("radio", { name: new RegExp(cvs.payAtPickup) }).check();
  106 |   await expect(page.getByTestId("cvs-pay-note")).toContainText(has(expectTotal));
  107 |   await shot(page, `${label}-quote`, locale, viewport);
  108 |   await page.getByTestId("confirm-address").click();
  109 |   await expect(page.getByTestId("cvs-store-name")).toContainText("建盛");
  110 |   await expect(page.getByTestId("cvs-store-address")).toHaveText("新竹市東區建中一路52號1樓");
  111 |   await expect(page.getByTestId("create-order")).toBeEnabled();
  112 |   await page.getByTestId("create-order").click();
  113 |   await expect(page.getByTestId("order-section")).toBeVisible({ timeout: 30_000 });
  114 |   const order = ((await page.getByTestId("order-id").innerText()) ?? "").trim();
  115 |   expect(order).toMatch(/^[0-9a-f-]{36}$/);
  116 |   await expect(page.getByTestId("order-collection")).toHaveAttribute("data-state", "PENDING");
  117 |   expect(await page.getByTestId("pay-order").count(), "a pay-at-pickup order has no card step").toBe(0);
  118 |   await expect(page.getByTestId("order-section")).toContainText(has(expectTotal));
  119 |   expect(withoutFixtureIds(await page.getByTestId("order-section").innerText()), `${label}: the order page must not show ${notTotal}`).not.toMatch(has(notTotal));
  120 |   await shot(page, `${label}-order`, locale, viewport);
  121 |   return order;
  122 | }
  123 | 
  124 | for (const cell of cells) {
  125 |   test(`live tools ${cell.locale} ${cell.viewport}: library import, live price, signed Meta claim, pay at pickup at the live price; direct purchase at the normal price`, async ({ browser }) => {
  126 |     const { n, locale, viewport } = cell;
  127 |     const claims = claimsCopy[locale];
  128 |     const studio = studioCopy[locale];
  129 |     const claim = claimCopy[locale];
  130 |     const run = await act("provision-run", { n });
  131 |     const kw = run.keyword as string;
  132 | 
  133 |     // A failed cell must not leak an open claim window into the next cell (only one window per store can be open):
  134 |     // end-run closes it (idempotent) in a finally, and every browser context closes whatever happens.
  135 |     let scene: string | undefined;
  136 |     let failed = false;
  137 |     const contexts: BrowserContext[] = [];
  138 |     try {
  139 |       // ---------------------------------------------------------------------------------------------------------- merchant
  140 |       const merchantContext = await browser.newContext({ viewport: sizes[viewport] });
  141 |       contexts.push(merchantContext);
  142 |       const merchant = await merchantContext.newPage();
  143 |       merchant.on("pageerror", (error) => pageErrors.push(error.message));
  144 |       await merchant.goto(`${admin}/en/`);
  145 |       await merchant.getByRole("button", { name: "Sign in with identity service", exact: true }).click();
  146 |       await merchant.waitForURL((u) => u.origin === new URL(admin).origin && !u.pathname.startsWith("/api/"), { timeout: 30_000 });
  147 | 
  148 |       async function newScene(name: string) {
  149 |         await merchant.goto(`${admin}/${locale}/studio?store=${store}`);
  150 |         await expect(merchant.getByTestId("merchant-studio")).toBeVisible();
  151 |         await merchant.getByRole("button", { name: studio.newScene }).click();
  152 |         await merchant.getByLabel(studio.name).fill(name);
  153 |         await merchant.getByRole("button", { name: studio.create }).click();
  154 |         await merchant.waitForURL(/scene=[0-9a-f-]{36}/);
  155 |         await expect(merchant.locator(".studio-surface.studio-planning-only")).toBeVisible();
  156 |         await merchant.getByTestId("studio-open-claims").click();
  157 |         await expect(merchant.getByTestId("merchant-claims")).toBeVisible();
  158 |         return new URL(merchant.url()).searchParams.get("scene") as string;
  159 |       }
  160 | 
  161 |       // 1. the keyword library: add this run's SKU keyword in a first scene (the offer form's pickers also feed "Add to library")
  162 |       await newScene(`Live tools library ${n}`);
  163 |       const offerForm = merchant.locator(".claims-offer-form");
  164 |       await offerForm.getByLabel(claims.keyword, { exact: true }).fill(kw);
  165 |       await offerForm.getByLabel(claims.product, { exact: true }).selectOption({ label: run.product_name });
  166 |       await expect(offerForm.getByLabel(claims.sku, { exact: true })).toBeEnabled();
  167 |       await offerForm.getByLabel(claims.sku, { exact: true }).selectOption({ label: run.sku_code });
  168 |       await merchant.getByTestId("claims-add-library").click();
  169 |       await expect(merchant.getByTestId(`library-${kw}`)).toBeVisible();
  170 |       expect(await merchant.getByTestId("claims-library").innerText()).toContain(run.sku_code);
  171 |       await act("check", { name: "library", sku_id: run.sku_id, keyword: kw });
  172 |       await shot(merchant, "merchant-library", locale, viewport);
  173 | 
  174 |       // 2. a NEW scene: import every library keyword in one action, then set the live price in the offers table
  175 |       scene = await newScene(`Live tools claim ${n}`);
  176 |       await expect(merchant.getByTestId("claims-import-tools")).toBeVisible();
  177 |       await merchant.getByRole("button", { name: claims.live.importLibrary }).click();
  178 |       // the store-level library accumulates one keyword per matrix cell; this new scene imports all n of them, none conflicting
  179 |       await expect(merchant.getByTestId("claims-import-result")).toContainText(claims.live.importDone(n, 0));
  180 |       await expect(merchant.getByTestId(`offer-${kw}`)).toContainText(run.sku_code);
  181 |       const price = merchant.getByTestId(`offer-price-${kw}`);
  182 |       await price.locator("input").fill(live.unit);
  183 |       await price.getByRole("button", { name: claims.live.saveLivePrice }).click();
  184 |       await expect(price.locator("input")).toHaveValue(new RegExp(`^${live.unit}(\\.00)?$`));
  185 |       await expect(price.getByRole("button", { name: claims.live.saveLivePrice })).toHaveCount(0);
  186 |       await expect(price).not.toContainText(claims.live.higherWarning("x", "y").split("x")[0]); // lower than the normal price: no warning
  187 |       await act("check", { name: "offer", scene, sku_id: run.sku_id, keyword: kw });
  188 |       await shot(merchant, "merchant-offers-live-price", locale, viewport);
  189 | 
  190 |       // 3. open the window and bind the post (private reply on, in the buyer's language)
  191 |       const quantityRule = merchant.getByLabel(claims.mode, { exact: true });
  192 |       await expect(quantityRule.locator("option")).toHaveCount(3);
```