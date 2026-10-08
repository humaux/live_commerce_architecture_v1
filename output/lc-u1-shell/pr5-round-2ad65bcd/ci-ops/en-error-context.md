# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: ops-polish.spec.ts >> OP4 Studio subtitle is neutral and the dead nav entries are gone in en
- Location: tests/admin/ops-polish.spec.ts:236:3

# Error details

```
Error: expect(received).toEqual(expected) // deep equality

- Expected  - 0
+ Received  + 3

@@ -1,8 +1,11 @@
  Array [
    "nav-group-overview",
    "nav-group-live",
+   "nav-studio",
+   "nav-live-console",
+   "nav-claims",
    "nav-orders",
    "nav-group-catalog",
    "nav-group-marketing",
    "nav-group-storefront",
    "nav-group-finance",

Call Log:
- Timeout 10000ms exceeded while waiting on the predicate
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
        - button "Overview" [ref=f2e8] [cursor=pointer]
        - generic [ref=f2e12]:
          - button "Live" [expanded] [ref=f2e13] [cursor=pointer]:
            - generic [aria-hidden] [ref=f2e17]: −
          - button "Live sessions" [ref=f2e18] [cursor=pointer]
          - button "Live console" [ref=f2e19] [cursor=pointer]
          - button "Live settings" [ref=f2e20] [cursor=pointer]
        - button "Orders & shipping" [ref=f2e22] [cursor=pointer]
        - button "Products & inventory" [ref=f2e27] [cursor=pointer]:
          - generic [aria-hidden] [ref=f2e31]: +
        - button "Marketing" [ref=f2e33] [cursor=pointer]
        - button "Online store" [ref=f2e38] [cursor=pointer]
        - button "Payments & reports" [ref=f2e43] [cursor=pointer]
      - navigation "Settings" [ref=f2e47]:
        - button "Settings" [ref=f2e49] [cursor=pointer]
    - generic [ref=f2e53]:
      - banner [ref=f2e54]:
        - generic [ref=f2e55]:
          - generic [ref=f2e56]: Switch store
          - generic [aria-hidden] [ref=f2e57]:
            - generic [ref=f2e58]: P
            - generic [ref=f2e59]: payment mock store
            - generic [ref=f2e60]: ⌄
          - combobox "Switch store" [ref=f2e61] [cursor=pointer]:
            - option "payment mock store" [selected]
        - generic [ref=f2e62]:
          - generic [ref=f2e63]: Language
          - combobox "Language" [ref=f2e64] [cursor=pointer]:
            - option "简体中文"
            - option "繁體中文"
            - option "English" [selected]
        - group [ref=f2e65]:
          - generic "Help" [ref=f2e66] [cursor=pointer]
        - group [ref=f2e67]:
          - generic "Account" [ref=f2e68] [cursor=pointer]
      - main [ref=f2e69]:
        - navigation "Workspace navigation" [ref=f2e70]:
          - link "Overview" [ref=f2e71] [cursor=pointer]:
            - /url: /en/
          - generic [aria-hidden] [ref=f2e72]: /
          - generic [ref=f2e73]: Live
          - generic [aria-hidden] [ref=f2e74]: /
          - generic [ref=f2e75]: Live sessions
        - generic [ref=f2e76]:
          - generic [ref=f2e77]:
            - generic [ref=f2e78]:
              - heading "Live sessions" [level=1] [ref=f2e79]
              - paragraph [ref=f2e80]: "Prepare your live session: products, comment claims and orders in one place."
            - button "Refresh facts" [ref=f2e82] [cursor=pointer]
          - generic [ref=f2e83]:
            - region "Scenes" [ref=f2e84]:
              - button "＋ New scene" [disabled] [ref=f2e85]
              - heading "Scenes" [level=2] [ref=f2e86]
              - alert [ref=f2e87]: This scene is unavailable in the selected store.
            - region "Scene settings" [ref=f2e88]:
              - heading "Scene settings" [level=2] [ref=f2e89]
              - paragraph [ref=f2e90]: Save the programme before checking rehearsal authority.
              - status [ref=f2e91]: Select a scene to edit it.
        - paragraph [ref=f2e93]: DaWan Live is operated by Hong Kong Da Wan Trading Limited
  - alert [ref=f2e94]
```

# Test source

```ts
  124 |     await page.clock.runFor(19_000);
  125 |     await pause(300);
  126 |     expect(probe.calls.length, "a poll fired before 19 s had passed since the previous one").toBe(afterFirst);
  127 |     await page.clock.runFor(1_500);
  128 |     await expect.poll(() => probe.calls.length, { message: "the next poll must fire at 20 s" }).toBe(afterFirst + 1);
  129 |     await expect.poll(() => probe.inflight).toBe(0);
  130 | 
  131 |     // the new order carries a visible marker and the tab title the count; the seen orders do not
  132 |     await expect(marker(page, one, /^New$/)).toBeVisible();
  133 |     await expect(page.getByText(/^New$/)).toHaveCount(1);
  134 |     await expect.poll(() => page.title()).toMatch(/^\(1\) /);
  135 |     // brief OP2: the badge names the orders page ("(N) 訂單"), not the static layout title (fixed by agent/kimi-p2-ui)
  136 |     expect(await page.title()).toMatch(/^\(1\) (Orders|订单|訂單)$/);
  137 |     // filters and the open order are exactly as the merchant left them
  138 |     expect(page.url()).toBe(urlBefore);
  139 |     await expect(page.getByTestId("state-filter")).toHaveValue("CONFIRMED");
  140 |     await expect(page.getByTestId("order-detail")).toBeVisible();
  141 |     await expect(page.getByTestId(`order-expand-${openId}`)).toHaveAttribute("aria-expanded", "true");
  142 | 
  143 |     // opening the new order acknowledges it: marker and count go away
  144 |     await page.getByTestId(`order-expand-${one}`).click();
  145 |     await expect(page.getByText(/^New$/)).toHaveCount(0);
  146 |     await expect.poll(() => page.title()).toBe(baseTitle);
  147 | 
  148 |     // two more orders: the count is cumulative over unseen orders
  149 |     const two = [await placeOrder(), await placeOrder()];
  150 |     await runUntilPoll(page, probe);
  151 |     for (const id of two) await expect(marker(page, id, /^New$/)).toBeVisible();
  152 |     await expect.poll(() => page.title()).toMatch(/^\(2\) (Orders|订单|訂單)$/);
  153 | 
  154 |     // never overlapping: a slow list, three more intervals pass while it is in flight, still exactly one request
  155 |     probe.hold = 1_500;
  156 |     const beforeSlow = probe.calls.length;
  157 |     await page.clock.runFor(20_000);
  158 |     await expect.poll(() => probe.inflight).toBe(1);
  159 |     await page.clock.runFor(60_000);
  160 |     await pause(200);
  161 |     expect(probe.calls.length, "a second list request started while the first was in flight").toBe(beforeSlow + 1);
  162 |     await expect.poll(() => probe.inflight, { timeout: 10_000 }).toBe(0);
  163 |     probe.hold = 0;
  164 |     expect(probe.maxInflight).toBe(1);
  165 | 
  166 |     // paused while hidden: long hidden spells fire no list request at all
  167 |     await setVisibility(page, "hidden");
  168 |     await pause(300);
  169 |     const hiddenAt = probe.calls.length;
  170 |     await page.clock.runFor(120_000);
  171 |     await pause(400);
  172 |     expect(probe.calls.length, "list requests were made while the tab was hidden").toBe(hiddenAt);
  173 |     const whileHidden = await placeOrder();
  174 |     await page.clock.runFor(60_000);
  175 |     await pause(300);
  176 |     expect(probe.calls.length).toBe(hiddenAt);
  177 |     // coming back reads the list at once and the order that arrived meanwhile is marked
  178 |     await setVisibility(page, "visible");
  179 |     await expect.poll(() => probe.calls.length, { timeout: 15_000 }).toBeGreaterThan(hiddenAt);
  180 |     await expect(marker(page, whileHidden, /^New$/)).toBeVisible({ timeout: 15_000 });
  181 |     await expect.poll(() => page.title()).toMatch(/^\(\d+\) /);
  182 |     // and the cadence resumes
  183 |     const resumed = await placeOrder();
  184 |     await runUntilPoll(page, probe);
  185 |     await expect(marker(page, resumed, /^New$/)).toBeVisible();
  186 |     expect(probe.maxInflight).toBe(1);
  187 |   } finally {
  188 |     await cover?.close().catch(() => {});
  189 |     await native.close();
  190 |   }
  191 | });
  192 | 
  193 | for (const [locale, word] of [["zh-TW", /^新$/], ["zh-CN", /^新$/]] as const) {
  194 |   test(`OP2 new marker and title count in ${locale}`, async ({ page }) => {
  195 |     test.setTimeout(120_000);
  196 |     const probe = await probeList(page);
  197 |     await page.clock.install();
  198 |     await signedLogin(page);
  199 |     await openOrders(page, locale);
  200 |     const baseTitle = await page.title();
  201 |     // G-UI8 audit [READ/MEASURE]: reads Date.now() to anchor the fake clock
  202 |     await page.clock.pauseAt(new Date((await page.evaluate(() => Date.now())) + 100));
  203 |     await pause(300);
  204 |     const id = await placeOrder();
  205 |     await runUntilPoll(page, probe);
  206 |     await expect(marker(page, id, word)).toBeVisible();
  207 |     await expect.poll(() => page.title()).toMatch(/^\(1\) (Orders|订单|訂單)$/);
  208 |   });
  209 | }
  210 | 
  211 | // ---- OP4: copy and navigation ------------------------------------------------------------------------------------------------------
  212 | const removedNav: Record<string, string[]> = {
  213 |   en: ["Website service", "Meta messages", "Platform support"],
  214 |   "zh-CN": ["网站客服", "Meta 消息", "平台支持"],
  215 |   "zh-TW": ["網站客服", "Meta 訊息", "平台支援"],
  216 | };
  217 | async function assertRegistryNavigation(page: Page, locale: string) {
  218 |   const nav = page.locator("[data-shell-rail]");
  219 |   // Exact authorized groups replace the obsolete nine flat-button assumption.
  220 |   // Do not grant Customers, add placeholder routes, or derive expected IDs from
  221 |   // the implementation registry: these are the fixture's independent contract.
  222 |   await expect.poll(() => nav.locator('button[data-testid^="nav-"]').evaluateAll(
  223 |     (buttons) => buttons.map((button) => button.getAttribute("data-testid")),
> 224 |   )).toEqual([
      |      ^ Error: expect(received).toEqual(expected) // deep equality
  225 |     "nav-group-overview", "nav-group-live", "nav-orders", "nav-group-catalog",
  226 |     "nav-group-marketing", "nav-group-storefront", "nav-group-finance", "nav-group-settings",
  227 |   ]);
  228 |   await nav.getByTestId("nav-group-catalog").click();
  229 |   for (const id of ["products", "collections", "inventory"]) await expect(nav.getByTestId(`nav-${id}`)).toBeVisible();
  230 |   await expect(nav.locator('button[data-testid^="nav-"]')).toHaveCount(11);
  231 |   for (const id of ["nav-group-messages", "nav-group-customers", "nav-siteChat", "nav-meta", "nav-support", "nav-billing", "nav-team"]) await expect(nav.getByTestId(id)).toHaveCount(0);
  232 |   const labels = (await nav.getByRole("button").allInnerTexts()).map((s) => s.trim());
  233 |   for (const dead of removedNav[locale]) expect(labels, `nav still lists "${dead}"`).not.toContain(dead);
  234 | }
  235 | for (const locale of ["en", "zh-CN", "zh-TW"]) {
  236 |   test(`OP4 Studio subtitle is neutral and the dead nav entries are gone in ${locale}`, async ({ page }) => {
  237 |     await signedLogin(page);
  238 |     await page.goto(new URL(`/${locale}/studio?store=${store}`, origin).toString());
  239 |     const studio = page.getByTestId("merchant-studio");
  240 |     await expect(studio).toBeVisible();
  241 |     const heading = studio.locator("header").first();
  242 |     const text = (await heading.innerText()).trim();
  243 |     expect(text, "the Studio heading must carry a subtitle line").toMatch(/\n./);
  244 |     expect(text, "the local rehearsal wording must be gone").not.toMatch(/MOCK|rehears|演练|演練|模拟|模擬|local/i);
  245 |     await assertRegistryNavigation(page, locale);
  246 |     // none of the remaining entries is a placeholder panel
  247 |     await expect(page.getByText(/not connected in the current build|当前版本尚未连接|目前版本尚未連線|目前版本尚未连接/)).toHaveCount(0);
  248 |   });
  249 | }
  250 | test("OP4 the same nav on the orders page", async ({ page }) => {
  251 |   await signedLogin(page);
  252 |   for (const locale of ["en", "zh-CN", "zh-TW"]) {
  253 |     await openOrders(page, locale);
  254 |     await assertRegistryNavigation(page, locale);
  255 |   }
  256 | });
  257 | 
  258 | // ---- OP3 UI half: the finance page and its CSV link ------------------------------------------------------------------------------
  259 | test("OP3 finance page shows a separate pay-at-pickup column in every locale; CSV link carries it", async ({ page }) => {
  260 |   await signedLogin(page);
  261 |   const today = new Date(Date.now() + 8 * 3600_000).toISOString().slice(0, 10);
  262 |   const header: Record<string, RegExp> = { en: /pay[- ]at[- ]pickup/i, "zh-TW": /取貨付款/, "zh-CN": /取货付款/ };
  263 |   for (const locale of ["en", "zh-TW", "zh-CN"]) {
  264 |     await page.goto(new URL(`/${locale}/finance?store=${store}`, origin).toString());
  265 |     await expect(page.getByTestId("finance-page")).toBeVisible();
  266 |     const from = page.getByTestId("finance-from");
  267 |     await from.fill(today);
  268 |     await page.getByTestId("finance-to").fill(today);
  269 |     await page.getByTestId("finance-show").click();
  270 |     const table = page.getByTestId("finance-table");
  271 |     await expect(table).toBeVisible();
  272 |     await expect(table.locator("thead th").filter({ hasText: header[locale] })).toHaveCount(1);
  273 |     const line = page.getByTestId(`finance-row-${today}-TWD`);
  274 |     await expect(line).toBeVisible();
  275 |     // the carrier-collected money is in its own cell, with the order count; captured and net stay at zero
  276 |     await expect(line.locator("td").filter({ hasText: new RegExp(`\\(${seed.collected.length}\\)`) })).toHaveCount(1);
  277 |   }
  278 |   const href = await page.getByTestId("finance-csv").getAttribute("href");
  279 |   expect(href).toBeTruthy();
  280 |   // in-page fetch: the Secure __Host- session cookie is sent by the browser, not by the APIRequestContext jar over http
  281 |   // G-UI8 audit [READ/MEASURE]: same-origin GET read of server state through the BFF (no state change) (finance CSV)
  282 |   const csv = await page.evaluate(async (u) => {
  283 |     const r = await fetch(u, { credentials: "same-origin" });
  284 |     return { status: r.status, text: await r.text() };
  285 |   }, new URL(href!, origin).toString());
  286 |   expect(csv.status).toBe(200);
  287 |   const lines = csv.text.trim().split(/\r?\n/);
  288 |   // 0088 (checkout-offline) appended the bank-transfer columns; 0107 (home-cod) appended the COD columns
  289 |   expect(lines[0]).toBe("day,currency,environment,captured_count,captured_minor,refunded_minor,net_minor,pickup_collected_count,pickup_collected_minor,bank_transfer_confirmed_count,bank_transfer_confirmed_minor,cod_collected_count,cod_collected_minor");
  290 |   const mine = lines.slice(1).find((l) => l.startsWith(`${today},TWD,LIVE`));
  291 |   expect(mine, `no LIVE row for ${today}: ${lines.join(" | ")}`).toBe(`${today},TWD,LIVE,0,0,0,0,${seed.collected.length},${seed.collectedMinor},0,0,0,0`);
  292 | });
  293 | 
```