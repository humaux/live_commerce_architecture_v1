# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: deal-loop.spec.ts >> T12 deal loop: wizard store, Studio, claim source, signed Meta comment, private reply, claim link, cart, checkout, Stripe (MOCK), ship, refund
- Location: tests/e2e/deal-loop.spec.ts:104:1

# Error details

```
Error: expect(locator).toHaveCSS(expected) failed

Locator:  getByTestId('create-order')
Expected: "rgb(36, 121, 101)"
Received: "rgb(20, 41, 66)"
Timeout:  15000ms

Call log:
  - Expect "toHaveCSS" getByTestId('create-order') with timeout 15000ms
  - waiting for getByTestId('create-order')
    34 × locator resolved to <button type="submit" data-testid="create-order" class="primary create-order">建立未付款訂單</button>
       - unexpected value "rgb(20, 41, 66)"

```

```yaml
- link "跳到主要內容":
  - /url: "#main"
- banner:
  - link "返回":
    - /url: /zh-TW/cart
- paragraph: 示意資料 · 測試環境
- main:
  - region "核對訂單":
    - heading "核對訂單" [level=1]
  - region "收件地址":
    - region "核對訂單":
      - paragraph: 宅配 · Synthetic Region · Synthetic City · 99991 · Synthetic Address Ninety One · Synthetic Unit Ninety Two
      - paragraph: Synthetic Gate Recipient
      - paragraph: "電話: +886900000091"
      - group "付款方式":
        - text: 付款方式
        - radio "信用卡" [checked]
        - text: 信用卡
- contentinfo:
  - text: 最終應付
  - button "查看明細"
  - strong: NT$25
  - text: 送出後線上付款
  - button "付款時機說明": "?"
  - button "送出訂單"
- contentinfo:
  - region "T12 Store":
    - paragraph: T12 Store
  - navigation "Legal":
    - list:
      - listitem:
        - link "防詐騙提醒":
          - /url: /zh-TW/legal/anti-fraud
      - listitem:
        - link "資料刪除":
          - /url: /zh-TW/data-deletion
  - navigation "語言":
    - link "简体中文":
      - /url: /zh-CN/checkout
    - link "繁體中文":
      - /url: /zh-TW/checkout
    - link "English":
      - /url: /en/checkout
  - paragraph: © 2026 T12 Store
- alert
```

# Test source

```ts
  156 |   await offerForm.getByLabel("SKU", { exact: true }).selectOption({ label: fixtures.sku_code });
  157 |   await offerForm.getByLabel("Max per claim", { exact: true }).fill("5");
  158 |   await offerForm.getByRole("button", { name: "Add offer" }).click();
  159 |   await expect(merchant.getByTestId("offer-A1")).toContainText(fixtures.sku_code);
  160 |   const source = merchant.getByTestId("claims-source");
  161 |   await source.getByLabel("Post or media link or ID", { exact: true }).fill(fixtures.post_url);
  162 |   await source.getByLabel("Reply language", { exact: true }).selectOption("zh-TW");
  163 |   await source.getByLabel("Send a private reply with the cart link").check();
  164 |   await source.getByRole("button", { name: "Save comment source" }).click();
  165 |   await expect(source.getByTestId("claims-source-object")).toHaveText(`${fixtures.page_asset}_${new URL(fixtures.post_url).pathname.split("/").pop()}`);
  166 |   await expect(source.getByRole("alert")).toHaveCount(0);
  167 |   await act("check", { name: "source" });
  168 |   await shot(merchant, "merchant-claims-source-bound", "en", "desktop", false);
  169 |   pass("Studio: a scene is created, the claim window opened, offer A1 bound to the SKU, the scene bound to the Facebook post with private replies (claim-source UI); Go readback matches");
  170 | 
  171 |   // ------------------------------------------------------------------------------------------------ 3. signed Meta comment
  172 |   await act("comment", { text: "A1+2", kind: "first" });
  173 |   const reply = await act("await-reply");
  174 |   expect(reply.sends).toBe(1);
  175 |   await act("check", { name: "claimed" });
  176 |   const bundle = merchant.locator('[data-testid^="bundle-"]').first();
  177 |   // MDEF-1 probe: the merchant must see the Meta-sourced claim (contracts/meta-claims-intake-v1.md §8: bundles.platform widens to
  178 |   // facebook/instagram, label NULL). Go answers 200 with platform "facebook"; the admin client validator (apps/admin/lib/
  179 |   // claims-model.ts, `platform: "manual"`) rejects it and the page shows "Claims are temporarily unavailable".
  180 |   async function merchantSeesClaim(name: string) {
  181 |     await merchant.getByRole("button", { name: "Refresh facts" }).click();
  182 |     try {
  183 |       await expect(bundle).toContainText("A1 × 2", { timeout: 8_000 });
  184 |       return true;
  185 |     } catch {
  186 |       await shot(merchant, name, "en", "desktop", false);
  187 |       if (!defects.some((d) => d.id === "MDEF-1"))
  188 |         defects.push({ id: "MDEF-1", what: `the merchant Claims page cannot show a Meta-sourced bundle: it renders "${(await merchant.getByTestId("merchant-claims").getByRole("alert").first().textContent().catch(() => merchant.locator("main").innerText()))?.toString().trim()}" although GET .../claims/bundles answered 200 (platform "facebook", empty label); apps/admin/lib/claims-model.ts validates platform === "manual"` });
  189 |       return false;
  190 |     }
  191 |   }
  192 |   if (await merchantSeesClaim("merchant-claims-after-comment")) {
  193 |     await expect(bundle).toContainText("Not opened yet");
  194 |     await shot(merchant, "merchant-claims-after-comment", "en", "desktop", false);
  195 |   }
  196 |   pass("a SIGNED Meta comment 'A1+2' entered the real ingress over HTTP; consumer + intake created one claim bundle (A1 x 2) and exactly one private reply reached the fake Graph with the claim link (Go-verified); merchant visibility is checked separately (MDEF-1)");
  197 | 
  198 |   // ------------------------------------------------------------------------------------------------ 4. negative loop
  199 |   const replay = await act("replay");
  200 |   expect(replay.replay_status).toBe(200);
  201 |   await act("negatives");
  202 |   await merchantSeesClaim("merchant-claims-after-negatives");
  203 |   pass("negative loop: an identical webhook replay creates nothing and sends no second reply; '不要A1' and 'A1是不是红色' create no claim and no send");
  204 | 
  205 |   // ------------------------------------------------------------------------------------------------ 5. buyer: claim link, cart
  206 |   const buyerContext = await browser.newContext({ ignoreHTTPSErrors: true, viewport: { width: 1440, height: 900 }, locale: "zh-TW" });
  207 |   let hostedVisits = 0;
  208 |   await buyerContext.route(/^https:\/\/checkout\.stripe\.com\//, (route) => {
  209 |     hostedVisits++;
  210 |     return route.fulfill({ status: 200, contentType: "text/html; charset=utf-8", body: "<!doctype html><title>Synthetic Stripe hosted page</title><main>Synthetic hosted page (MOCK)</main>" });
  211 |   });
  212 |   const buyer = await buyerContext.newPage();
  213 |   buyer.on("pageerror", (error) => pageErrors.push(error.message));
  214 |   const link = reply.link as string;
  215 |   expect(link).toMatch(/^https:\/\/buyer\.example\/zh-TW\/claim#t=[A-Za-z0-9_-]{43}$/);
  216 |   const token = link.split("#t=")[1];
  217 |   const claim = claimCopy[L];
  218 |   const buyerRequests: string[] = [];
  219 |   buyer.on("request", (request) => buyerRequests.push(request.url()));
  220 |   const opened = await buyer.goto(link);
  221 |   expect(opened?.status()).toBe(200);
  222 |   await expect(buyer.getByRole("heading", { level: 1, name: claim.title })).toBeVisible();
  223 |   await expect(buyer.getByTestId("claim-line-A1")).toContainText(`${claim.quantity} 2`);
  224 |   await expect(buyer.getByTestId("claim-line-A1")).toContainText(fixtures.sku_code);
  225 |   expect(new URL(buyer.url()).hash).toBe("");
  226 |   await storageScanSelfTest(buyer, token);
  227 |   await storageLacks(buyer, [token]);
  228 |   await buyerShots(buyer, "buyer-claim");
  229 |   await buyer.getByRole("button", { name: claim.add }).click();
  230 |   await buyer.waitForURL((url) => url.pathname === `/${L}/checkout`);
  231 |   await expect(buyer.getByTestId("claim-checkout-notice")).toHaveText(claim.checkoutNotice);
  232 |   await expect(buyer.locator(`[data-testid="cart-line"][data-sku="${fixtures.sku_id}"]`)).toBeVisible();
  233 |   await expect(buyer.locator(`[data-testid="cart-line"][data-sku="${fixtures.sku_id}"]`).locator(".sf-line__unit")).toContainText("× 2");
  234 |   await act("check", { name: "cart" });
  235 |   await buyerShots(buyer, "buyer-claim-added");
  236 |   pass("the buyer opens the private-reply link (zh-TW): prefilled claim A1 x 2, fragment dropped, token not stored; one explicit checkout click puts A1 x 2 in the server cart and opens checkout");
  237 | 
  238 |   // ------------------------------------------------------------------------------------------------ 6. checkout
  239 |   const purchase = purchaseCopy[L];
  240 |   // The storefront shell's checkout page (/{locale}/checkout) opens on the claimed cart: A1 x 2 (read-only lines show "× 2").
  241 |   await buyer.goto(`${buyerOrigin}/${L}/checkout`);
  242 |   await expect(buyer.locator(".sf-line__unit", { hasText: "× 2" })).toBeVisible();
  243 |   await buyer.getByRole("button", { name: purchase.delivery, exact: true }).click();
  244 |   const quotation = buyer.waitForResponse((r) => new URL(r.url()).pathname === "/api/buyer/quotes" && r.request().method() === "POST");
  245 |   await buyer.getByRole("button", { name: purchase.quote, exact: true }).click();
  246 |   expect((await quotation).status()).toBe(200);
  247 |   await expect(buyer.getByTestId("address-section")).toBeVisible();
  248 |   for (const [key, value] of Object.entries(pii)) await buyer.locator(`input[name="${key}"]`).fill(value);
  249 |   // Before the address is confirmed the create button is disabled, visibly grey and described; confirmed, it wears the full action colour
  250 |   // (visual QA: the faded teal read as an enabled button gone wrong).
  251 |   await expect(buyer.getByTestId("create-order")).toBeDisabled();
  252 |   await expect(buyer.getByTestId("create-order")).toHaveCSS("background-color", "rgb(231, 237, 243)");
  253 |   await expect(buyer.getByTestId("create-order")).toHaveAccessibleDescription(/\S{6,}/);
  254 |   await buyer.getByTestId("confirm-address").click();
  255 |   await expect(buyer.getByTestId("create-order")).toBeEnabled();
> 256 |   await expect(buyer.getByTestId("create-order")).toHaveCSS("background-color", "rgb(36, 121, 101)");
      |                                                   ^ Error: expect(locator).toHaveCSS(expected) failed
  257 |   await expect(buyer.getByTestId("create-order")).toHaveCSS("opacity", "1");
  258 |   await buyer.getByTestId("create-order").click();
  259 |   await expect(buyer.getByTestId("order-section")).toBeVisible();
  260 |   const order = ((await buyer.getByTestId("order-id").innerText()) ?? "").trim();
  261 |   expect(order).toMatch(/^[0-9a-f-]{36}$/);
  262 |   await expect(buyer.getByTestId("payment-status")).toHaveAttribute("data-state", "NOT_STARTED");
  263 |   await act("check", { name: "ordered", order });
  264 |   await buyerShots(buyer, "buyer-order-created");
  265 |   pass("the buyer checks out the claimed cart (home delivery; the storefront states that convenience-store pickup is not connected yet): one unpaid order, total NT$25, 2 units held");
  266 | 
  267 |   // ------------------------------------------------------------------------------------------------ 7. pay (Stripe MOCK)
  268 |   const pay = paymentCopy[L];
  269 |   const popup = buyer.waitForEvent("popup");
  270 |   await buyer.getByRole("button", { name: pay.pay, exact: true }).click();
  271 |   const hosted = await popup;
  272 |   await hosted.waitForURL((u) => u.origin === "https://checkout.stripe.com", { waitUntil: "commit" });
  273 |   expect(await hosted.evaluate(() => window.opener)).toBeNull();
  274 |   expect(hostedVisits).toBeGreaterThan(0);
  275 |   expect(buyer.url()).not.toContain("stripe");
  276 |   await act("pay", { order });
  277 |   await hosted.close();
  278 |   await buyer.bringToFront();
  279 |   await expect(async () => {
  280 |     await buyer.getByTestId("refresh-order").click();
  281 |     await expect(buyer.getByTestId("payment-status")).toHaveAttribute("data-state", "CAPTURED", { timeout: 4_000 });
  282 |   }).toPass({ timeout: 90_000, intervals: [1_000, 3_000, 5_000] });
  283 |   await expect(buyer.getByTestId("payment-status")).toHaveText(`${pay.paymentState}: ${pay.CAPTURED}`);
  284 |   await expect(buyer.getByTestId("payment-commercial-status")).toHaveText(`${pay.orderState}: ${orderCopy[L].CONFIRMED}`);
  285 |   await act("check", { name: "paid" });
  286 |   await buyerShots(buyer, "buyer-paid");
  287 |   pass("Stripe (MOCK): the buyer pays in a new tab, the signed webhook captures it; the order is CONFIRMED and 2 units are allocated");
  288 | 
  289 |   // ------------------------------------------------------------------------------------------------ 8. merchant ships
  290 |   await merchant.goto(`${admin}/en/orders?store=${store}`);
  291 |   await expect(merchant.getByTestId("merchant-orders")).toBeVisible();
  292 |   await expect(merchant.getByTestId("orders-table")).toBeVisible();
  293 |   const detail = await expandOrder(merchant, order);
  294 |   await expect(detail.locator('[data-state="CONFIRMED"]').first()).toBeVisible();
  295 |   await expect(detail).toContainText(money(2500));
  296 |   await shot(merchant, "merchant-order-confirmed", "en", "desktop", false);
  297 |   await detail.getByLabel(/carrier|物流|承運/i).first().selectOption("seven_eleven_cvs");
  298 |   await detail.getByLabel(/tracking number|物流單號|追蹤單號/i).fill(fixtures.tracking);
  299 |   await detail.getByLabel(/tracking (url|link)|追蹤連結/i).fill(`https://track.example.com/t?n=${fixtures.tracking}`);
  300 |   await detail.getByRole("button", { name: /mark shipped|標記已出貨/i }).click();
  301 |   await detail.getByTestId("shipment-confirm").click();
  302 |   await expect(detail.locator('[data-state="MERCHANT_SHIPPED"]').first()).toBeVisible();
  303 |   await expect(detail).toContainText(fixtures.tracking);
  304 |   await expect(detail).not.toContainText(/delivered|in transit|已送達|運送中/i);
  305 |   await act("check", { name: "shipped" });
  306 |   await shot(merchant, "merchant-order-shipped", "en", "desktop", false);
  307 |   pass("the merchant marks the paid order shipped (7-ELEVEN + tracking) in the orders UI: shipment v1 SHIPPED, MERCHANT_SHIPPED, audited, no provider operation, stock unchanged");
  308 | 
  309 |   // ------------------------------------------------------------------------------------------------ 9. buyer sees the shipment
  310 |   async function openOrder(page: Page) {
  311 |     await page.goto(`${buyerOrigin}/${L}/checkout`); // order history lives on the checkout page (storefront shell)
  312 |     await page.getByTestId("toggle-order-history").click();
  313 |     const detailResponse = page.waitForResponse((r) => new URL(r.url()).pathname === `/api/buyer/orders/${order}` && r.request().method() === "GET");
  314 |     await page.locator(`button[data-order-id="${order}"]`).click();
  315 |     expect((await detailResponse).status()).toBe(200);
  316 |     await expect(page.getByTestId("order-id")).toHaveText(order);
  317 |   }
  318 |   await openOrder(buyer);
  319 |   await expect(buyer.getByTestId("order-shipment")).toBeVisible();
  320 |   await expect(buyer.getByTestId("shipment-carrier")).toContainText(carrierNames[L].seven_eleven_cvs);
  321 |   await expect(buyer.getByTestId("shipment-tracking")).toHaveText(fixtures.tracking);
  322 |   expect(await buyer.locator("body").innerText()).not.toMatch(/delivered|in transit|已送達|運送中/i);
  323 |   await buyerShots(buyer, "buyer-shipped");
  324 |   pass("the buyer sees the carrier and tracking number of the merchant's shipment, never 'delivered'");
  325 | 
  326 |   // ------------------------------------------------------------------------------------------------ 10. refund
  327 |   await merchant.reload();
  328 |   await expect(merchant.getByTestId("orders-table")).toBeVisible();
  329 |   const refundDetail = await expandOrder(merchant, order);
  330 |   await refundDetail.getByRole("button", { name: /^(refund|退款|發起退款|发起退款|issue refund)/i }).click();
  331 |   const dialog = merchant.locator("dialog[open]");
  332 |   await expect(dialog).toBeVisible();
  333 |   const amount = dialog.getByLabel(/amount|金額|金额/i);
  334 |   await expect(amount).toHaveValue(/^25(\.00)?$/);
  335 |   await amount.fill(String(refundMinor / 100));
  336 |   await dialog.getByLabel(/reason|原因/i).selectOption("requested_by_customer");
  337 |   await dialog.getByRole("button", { name: /review|核對|核对/i }).click();
  338 |   await expect(dialog).toContainText(String(refundMinor / 100));
  339 |   await dialog.getByRole("button", { name: /confirm|確認|确认/i }).last().click();
  340 |   await expect(dialog).toHaveCount(0);
  341 |   await act("settle-refund");
  342 |   await expect(async () => {
  343 |     const refresh = refundDetail.getByRole("button", { name: /refresh|重新整理|刷新/i });
  344 |     if (await refresh.count()) await refresh.first().click();
  345 |     await expect(refundDetail.locator('[data-state="SUCCEEDED"]').first()).toBeVisible({ timeout: 3_000 });
  346 |   }).toPass({ timeout: 60_000, intervals: [1_000, 3_000, 5_000] });
  347 |   await expect(refundDetail).toContainText(/re_[A-Za-z0-9_]+/);
  348 |   await expect(refundDetail).toContainText(money(2500 - refundMinor));
  349 |   await act("check", { name: "refunded" });
  350 |   await shot(merchant, "merchant-refund-succeeded", "en", "desktop", false);
  351 |   await openOrder(buyer);
  352 |   await expect(buyer.getByTestId("refund-succeeded")).toContainText(pay.refunded.split("{amount}")[0].trim());
  353 |   await expect(buyer.getByTestId("payment-status")).toHaveAttribute("data-state", "PARTIALLY_REFUNDED");
  354 |   await buyerShots(buyer, "buyer-refunded");
  355 |   pass("the merchant issues a partial refund (NT$10); the real worker settles it at the fake Stripe; the merchant sees SUCCEEDED and the re_ id, the buyer sees the refunded amount, the order and the stock are unchanged");
  356 | 
```