# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: live-console.spec.ts >> C4 session products stock and end confirmation 1440
- Location: tests/admin/live-console.spec.ts:680:41

# Error details

```
Error: expect(locator).toBeVisible() failed

Locator: getByRole('heading', { name: '本場商品', exact: true })
Expected: visible
Timeout: 10000ms
Error: element(s) not found

Call log:
  - Expect "toBeVisible" getByRole('heading', { name: '本場商品', exact: true }) with timeout 10000ms
  - waiting for getByRole('heading', { name: '本場商品', exact: true })

```

# Test source

```ts
  584 |   test(`LC-U1 ${locale}: bounded live_adjust works, no-stock permission stays disabled, missing A1 is graceful`, async ({ browser, request }) => {
  585 |     const context = await browser.newContext({ viewport: sizes[1], ignoreHTTPSErrors: true });
  586 |     try {
  587 |       const url = origin.replace(/^http:/, "https:");
  588 |       const csrf = randomBytes(32).toString("base64url");
  589 |       await context.addCookies([
  590 |         { name: "__Host-commerce_session", value: narrowToken, url, secure: true, httpOnly: true, sameSite: "Lax" },
  591 |         { name: "__Host-commerce_csrf", value: csrf, url, secure: true, httpOnly: false, sameSite: "Lax" },
  592 |       ]);
  593 |       const page = await context.newPage();
  594 |       const scene = lateDestination;
  595 |       await page.goto(route(locale, scene));
  596 |       await expect(page.getByTestId("live-console")).toBeVisible();
  597 |       await phase(page, "draft");
  598 |       const body = (await facts(request)).scenes[scene]!;
  599 |       await expect(page.getByTestId("live-primary-action")).toBeDisabled();
  600 |       await expect(page.getByTestId(`live-offer-toggle-${body.Offer}`)).toBeDisabled();
  601 |       await expect(page.getByTestId(`live-recommend-${body.Offer}`)).toBeDisabled();
  602 |       await expect(page.getByTestId(`live-stock-${body.Offer}`)).toBeEnabled();
  603 |       await expect(page.getByTestId("live-console").locator(".live-status-bar > div").first().locator("dd")).toHaveText("—");
  604 |       const before = (await facts(request)).receipts.length;
  605 |       await page.getByTestId(`live-stock-${body.Offer}`).fill(String(body.Stock + 1001));
  606 |       await page.getByTestId(`live-stock-save-${body.Offer}`).click();
  607 |       await expect(page.getByTestId(`live-offer-${body.Offer}`).getByRole("alert")).toHaveText(workspaceCopy[locale].stockInvalid);
  608 |       expect((await facts(request)).receipts).toHaveLength(before);
  609 |       await page.getByTestId(`live-stock-${body.Offer}`).fill(String(body.Stock + 2));
  610 |       await page.getByTestId(`live-stock-save-${body.Offer}`).click();
  611 |       await expect.poll(async () => (await facts(request)).scenes[scene]!.Stock).toBe(body.Stock + 2);
  612 |       await page.reload();
  613 |       await expect(page.getByTestId(`live-stock-${body.Offer}`)).toHaveValue(String(body.Stock + 2));
  614 |       const receipt = (await facts(request)).receipts.at(-1)!;
  615 |       expect(receipt).toMatchObject({ action: "adjustments", status: 200, effect: true });
  616 |       expect(receipt.input).toMatchObject({ delta: 2, expected_version: body.StockVersion, reason: "live_console_edit" });
  617 |       await fault(request, scene, "below_reserved");
  618 |       await page.getByTestId(`live-stock-${body.Offer}`).fill(String(body.Stock + 1));
  619 |       await page.getByTestId(`live-stock-save-${body.Offer}`).click();
  620 |       await expect(page.getByTestId("live-console").getByRole("alert")).toContainText(workspaceCopy[locale].refusals.below_reserved);
  621 |       expect((await facts(request)).scenes[scene]!.Stock).toBe(body.Stock + 2);
  622 |       await screenshot(page, `${locale}-narrow-permission`);
  623 |       // Setup a distinct signed principal with no inventory grant; assertions use the real page.
  624 |       await context.addCookies([{ name: "__Host-commerce_session", value: readToken, url, secure: true, httpOnly: true, sameSite: "Lax" }]);
  625 |       await page.reload();
  626 |       await expect(page.getByTestId(`live-stock-${body.Offer}`)).toBeDisabled();
  627 |       await expect(page.getByText(workspaceCopy[locale].stockPermission, { exact: true })).toBeVisible();
  628 |       await expect(page.getByTestId(`live-stock-save-${body.Offer}`)).toBeDisabled();
  629 |       const after = (await facts(request)).receipts.length;
  630 |       await page.getByTestId("live-console-refresh").click();
  631 |       await expect(page.getByTestId(`live-stock-${body.Offer}`)).toBeDisabled();
  632 |       expect((await facts(request)).receipts).toHaveLength(after);
  633 |       // Authority negative only: direct BFF request supplements the real disabled-control clicks above.
  634 |       const forbidden = await context.request.post(`${origin}/api/stores/${store}/inventory/adjustments`, {
  635 |         headers: { Cookie: await authorityCookie(context), origin, "x-csrf-token": csrf, "idempotency-key": `no-stock-${locale}` },
  636 |         data: { warehouse_id: body.Warehouse, sku_id: body.SKU, delta: 1, expected_version: body.StockVersion + 1, reason: "live_console_edit" },
  637 |       });
  638 |       expect(forbidden.status()).toBe(403);
  639 |       expect((await forbidden.json()).code).toBe("forbidden");
  640 |       expect((await facts(request)).receipts).toHaveLength(after);
  641 |       expect((await facts(request)).scenes[scene]!.Stock).toBe(body.Stock + 2);
  642 |       await screenshot(page, `${locale}-no-stock-permission`);
  643 |       await fault(request, scene, "missing");
  644 |       try {
  645 |         await page.reload();
  646 |         await expect(page.getByTestId("live-console-unavailable")).toBeVisible();
  647 |         await expect(page.getByTestId(`live-offer-${body.Offer}`)).toHaveCount(0);
  648 |         await screenshot(page, `${locale}-missing-A1`);
  649 |       } finally { await fault(request, scene, "restore"); }
  650 |     } finally { await context.close(); }
  651 |   });
  652 | }
  653 | 
  654 | const parityScenes: string[] = JSON.parse(required("LC_BROWSER_CONSOLE_PARITY_SCENES"));
  655 | // C4 uses this existing isolated session/stock/copy/lifecycle fixture. Console source remains owned by PARITY-LIVE.
  656 | for (const viewport of parityViewports) test(`C4 preparation copy next-draft and post-live ${viewport.width}`, async ({page}) => {
  657 |   const c=parityWords["zh-TW"], scene=scenes[0]!;
  658 |   await page.setViewportSize(viewport); await login(page);
  659 |   await page.goto(`${origin}/zh-TW/studio?store=${store}&scene=${scene}#preparation`);
  660 |   await expect(page.getByRole("heading",{name:c.prepare,exact:true})).toBeVisible();
  661 |   for(const title of [c.source,c.device,c.productsCheck]) await expect(page.getByText(title,{exact:true})).toBeVisible();
  662 |   for(const name of [c.console,c.connections,c.settings,c.templates]) await expect(page.getByRole("button",{name,exact:true}).or(page.getByRole("link",{name,exact:true}))).toBeVisible();
  663 |   await parityShot(page,evidence,"zh-TW",viewport.width===1440?"5:17701":"5:17826");
  664 |   await page.goto(`${origin}/zh-TW/studio?store=${store}&scene=${scene}#post-live`);
  665 |   await expect(page.getByRole("heading",{name:c.after,exact:true})).toBeVisible();
  666 |   await expect(page.getByText("認領與付款分開計算。結束抓單不會把未付款訂單變成已付款，也不代表平台已結算。",{exact:true})).toBeVisible();
  667 |   await parityShot(page,evidence,"zh-TW",viewport.width===1440?"19:374":"19:376");
  668 |   await parityClick(page,c.copy,()=>expect(page.getByRole("heading",{name:c.copy,exact:true})).toBeVisible());
  669 |   await page.getByLabel(c.copyName,{exact:true}).fill(`C4 copy ${viewport.width}`);
  670 |   await expect(page.getByText("活動商品與直播價。",{exact:true})).toBeVisible();
  671 |   await expect(page.getByText("貼文來源、已發生的認領、付款連結。新場次的認領窗口為關閉。",{exact:true})).toBeVisible();
  672 |   await parityShot(page,evidence,"zh-TW",viewport.width===1440?"30:1977":"30:2030");
  673 |   await parityClick(page,c.copyReview,()=>expect(page.getByRole("heading",{name:c.copied,exact:true})).toBeVisible());
  674 |   await expect(page.getByText(`C4 copy ${viewport.width}`,{exact:true})).toBeVisible();
  675 |   await parityShot(page,evidence,"zh-TW",viewport.width===1440?"1336:17336":"1336:17337");
  676 |   await page.reload();
  677 |   await expect(page.getByText(`C4 copy ${viewport.width}`,{exact:true})).toBeVisible();
  678 |   await parityClick(page,c.openPrep,()=>expect(page.getByRole("heading",{name:c.preflight,exact:true})).toBeVisible());
  679 | });
  680 | for (const viewport of parityViewports) test(`C4 session products stock and end confirmation ${viewport.width}`,async({page,request})=>{
  681 |   const c=parityWords["zh-TW"], scene=parityScenes[viewport.width===1440?0:1]!, fixture=(await facts(request)).scenes[scene]!;
  682 |   await page.setViewportSize(viewport);await login(page);
  683 |   await page.goto(`${origin}/zh-TW/studio/claims?store=${store}&scene=${scene}#products`);
> 684 |   await expect(page.getByRole("heading",{name:c.products,exact:true})).toBeVisible();
      |                                                                        ^ Error: expect(locator).toBeVisible() failed
  685 |   if(viewport.width===390) await parityShot(page,evidence,"zh-TW","52:1385");
  686 |   await expect(page.getByRole("button",{name:c.stock,exact:true}).first()).toBeVisible();
  687 |   await page.getByRole("button",{name:c.stock,exact:true}).first().click();
  688 |   await expect(page.getByRole("heading",{name:c.stock,exact:true})).toBeVisible();
  689 |   await expect(page.getByText(c.available,{exact:true})).toBeVisible();
  690 |   await expect(page.getByText(c.adjusted,{exact:true})).toBeVisible();
  691 |   const before=(await facts(request)).scenes[scene]!.Stock;
  692 |   await parityClick(page,c.plus,()=>expect(page.getByTestId("session-stock-available")).toHaveText(String(before+1)));
  693 |   await parityClick(page,c.minus,()=>expect(page.getByTestId("session-stock-available")).toHaveText(String(before)));
  694 |   await parityShot(page,evidence,"zh-TW",viewport.width===1440?"1336:17329":"52:1388");
  695 |   await page.reload();await expect(page.getByTestId("session-stock-available")).toHaveText(String(before));
  696 |   expect((await facts(request)).scenes[scene]!.SKU).toBe(fixture.SKU);
  697 |   await page.goto(`${origin}/zh-TW/studio?store=${store}&scene=${scene}#end`);
  698 |   await expect(page.getByRole("heading",{name:c.end,exact:true})).toBeVisible();
  699 |   await expect(page.getByText("結束本場留言收單？平台上的直播不會因此停止。",{exact:true})).toBeVisible();
  700 |   await expect(page.getByText("既有認領、訂單及付款狀態不會被當作已完成。",{exact:true})).toBeVisible();
  701 |   await expect(page.getByRole("button",{name:c.confirmEnd,exact:true})).toBeVisible();
  702 |   await parityShot(page,evidence,"zh-TW",viewport.width===1440?"1336:17332":"1336:17333");
  703 |   await parityClick(page,c.continue,()=>expect(page.getByRole("heading",{name:c.end,exact:true})).toHaveCount(0));
  704 |   await page.goto(`${origin}/zh-TW/studio?store=${store}&scene=${scene}#end`);
  705 |   await expect(page.getByRole("button",{name:c.confirmEnd,exact:true})).toBeVisible();
  706 |   await parityClick(page,c.confirmEnd,()=>expect(page.getByRole("heading",{name:c.after,exact:true})).toBeVisible());
  707 |   expect((await facts(request)).scenes[scene]!.Phase).toBe("ended");
  708 |   await page.reload();await expect(page.getByRole("heading",{name:c.after,exact:true})).toBeVisible();
  709 | });
  710 | 
```