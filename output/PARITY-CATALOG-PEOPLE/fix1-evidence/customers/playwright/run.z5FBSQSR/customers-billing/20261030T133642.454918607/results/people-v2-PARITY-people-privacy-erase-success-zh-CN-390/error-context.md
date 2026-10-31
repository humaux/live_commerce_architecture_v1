# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: people-v2.spec.ts >> PARITY people privacy erase-success zh-CN-390
- Location: tests/admin/people-v2.spec.ts:296:5

# Error details

```
Error: PARITY-PEOPLE: signed dedicated outcome

expect(locator).toBeVisible() failed

Locator: getByRole('heading', { name: '客户资料已清除', exact: true })
Expected: visible
Timeout: 10000ms
Error: element(s) not found

Call log:
  - PARITY-PEOPLE: signed dedicated outcome getByRole('heading', { name: '客户资料已清除', exact: true }) with timeout 10000ms
  - waiting for getByRole('heading', { name: '客户资料已清除', exact: true })

```

```yaml
- link "跳转到内容":
  - /url: "#main"
- banner:
  - button "打开导航"
  - text: 切换店铺
  - combobox "切换店铺":
    - option "payment mock store" [selected]
  - text: 语言
  - combobox "语言":
    - option "简体中文" [selected]
    - option "繁體中文"
    - option "English"
  - group: 帮助
  - group: 账号
- main:
  - navigation "工作区导航":
    - link "总览":
      - /url: /zh-CN/
    - text: 顾客 顾客详情
  - heading "客户" [level=1]
  - link "返回搜索结果":
    - /url: /zh-CN/customers?store=3c0420c6-d65b-4803-8503-7568578868d6
  - navigation "客户":
    - link "订单":
      - /url: "#people-orders"
    - link "授权记录":
      - /url: "#people-consent"
    - link "隐私":
      - /url: "#people-privacy"
  - complementary:
    - group: 高级
    - region "隐私":
      - heading "隐私" [level=2]
      - button "下载客户资料"
      - status: 客户已抹除：撤回授权 1 项，结束会话 1 个，遮蔽订单信息 0 笔，重新标记登记组 0 个。
  - status: 该客户已被抹除。其订单、付款与出货记录予以保留。
  - region "客户":
    - heading "示範林一" [level=2]
    - paragraph: payment mock store · 客户编号 1cfb962d · 手机尾号 805
    - paragraph: 3 订单 · 实收金额 NT$3,840 首次记录 2026-09-12
  - region "订单":
    - heading "订单" [level=2]
    - region "订单":
      - table:
        - rowgroup:
          - 'row "在订单中打开: 11111111-1111-4111-8111-111111111111 · NT$1,280 已扣款 · 已确认 · 2026-10-25"':
            - 'cell "在订单中打开: 11111111-1111-4111-8111-111111111111 · NT$1,280 已扣款 · 已确认 · 2026-10-25"':
              - 'link "在订单中打开: 11111111-1111-4111-8111-111111111111"':
                - /url: /zh-CN/orders?store=3c0420c6-d65b-4803-8503-7568578868d6&order=11111111-1111-4111-8111-111111111111
                - text: 1111…1111
              - text: · NT$1,280 已扣款 · 已确认 · 2026-10-25
          - 'row "在订单中打开: 22222222-2222-4222-8222-222222222222 · NT$1,280 已扣款 · 已确认 · 2026-10-25"':
            - 'cell "在订单中打开: 22222222-2222-4222-8222-222222222222 · NT$1,280 已扣款 · 已确认 · 2026-10-25"':
              - 'link "在订单中打开: 22222222-2222-4222-8222-222222222222"':
                - /url: /zh-CN/orders?store=3c0420c6-d65b-4803-8503-7568578868d6&order=22222222-2222-4222-8222-222222222222
                - text: 2222…2222
              - text: · NT$1,280 已扣款 · 已确认 · 2026-10-25
          - 'row "在订单中打开: 33333333-3333-4333-8333-333333333333 · NT$1,280 已扣款 · 已确认 · 2026-10-25"':
            - 'cell "在订单中打开: 33333333-3333-4333-8333-333333333333 · NT$1,280 已扣款 · 已确认 · 2026-10-25"':
              - 'link "在订单中打开: 33333333-3333-4333-8333-333333333333"':
                - /url: /zh-CN/orders?store=3c0420c6-d65b-4803-8503-7568578868d6&order=33333333-3333-4333-8333-333333333333
                - text: 3333…3333
              - text: · NT$1,280 已扣款 · 已确认 · 2026-10-25
  - region "授权记录":
    - heading "授权记录" [level=2]
    - list:
      - listitem: 营销消息 · Messenger / IG DM 客户设置 · 2026-10-20 已撤回授权
      - listitem: 个性化广告 · Meta 客户设置 · 2026-10-01 未授权
  - paragraph: DaWan Live 是 香港大碗貿易有限公司 的直播电商平台
- alert
```

# Test source

```ts
  227 |       await act(page, copy.en.pending, "existing unconfirmed request entry is visible", () => page.getByRole("button", { name: copy.en.pending, exact: true }).click(), () => expect(page.getByRole("button", { name: copy.en.viewData, exact: true })).toBeVisible());
  228 |       await act(page, copy.en.viewData, "customer stays readable with both privacy commands locked", () => page.getByRole("button", { name: copy.en.viewData, exact: true }).click(), async () => { await expect(page.getByTestId("customer-download")).toBeDisabled(); await expect(page.getByTestId("customer-erase")).toBeDisabled(); });
  229 |       await act(page, "Refresh", "unconfirmed request is not silently replayed or unlocked", () => page.reload(), async () => { await expect(page.getByTestId("customer-download")).toBeDisabled(); await expect(page.getByTestId("customer-erase")).toBeDisabled(); }, "reload");
  230 |       expect(f.posts).toHaveLength(1);
  231 |       await act(page, copy.en.pendingBack, "read-only return preserves customer identity", () => page.getByRole("button", { name: copy.en.pendingBack, exact: true }).click(), () => expect(page.getByTestId("customer-detail")).toContainText(name));
  232 |       expect(f.posts).toHaveLength(1); await fits(page);
  233 |     });
  234 |   }
  235 |   // Reach the shared uncertainty fence independently of the negative name/checkbox assertions above.
  236 |   // These positive preparations still use real controls and the same closed DTO/fault fixture.
  237 |   for (const reload of [false, true]) {
  238 |     test(`PEOPLE-V2 independent uncertainty fence ${reload ? "reload" : "return"} en-${width}`, async ({ page }) => {
  239 |       await signed(page, width); const f = await fixture(page, "erase"); await page.goto(detailURL("en"));
  240 |       await expect(page.getByTestId("customer-erase")).toBeEnabled();
  241 |       await act(page, copy.en.erase, "erasure review opens", () => page.getByTestId("customer-erase").click(), () => expect(page.getByTestId("erase-dialog")).toBeVisible());
  242 |       await act(page, name, "exact customer name is visible", () => page.getByTestId("erase-typed").fill(name), () => expect(page.getByTestId("erase-typed")).toHaveValue(name), "fill");
  243 |       await act(page, copy.en.checked, "scope is explicitly acknowledged", () => page.getByRole("checkbox", { name: copy.en.checked, exact: true }).check(), () => expect(page.getByTestId("erase-submit")).toBeEnabled(), "check");
  244 |       expect(f.posts).toHaveLength(0);
  245 |       await act(page, copy.en.confirmErase, "lost response reaches the unconfirmed result", () => page.getByTestId("erase-submit").click(), () => expect(page.getByRole("button", { name: copy.en.pending, exact: true })).toBeVisible());
  246 |       expect(f.posts).toHaveLength(1);
  247 |       await act(page, copy.en.pending, "pending entry offers the read-only customer view", () => page.getByRole("button", { name: copy.en.pending, exact: true }).click(), () => expect(page.getByRole("button", { name: copy.en.viewData, exact: true })).toBeVisible());
  248 |       await act(page, copy.en.viewData, "customer profile remains readable", () => page.getByRole("button", { name: copy.en.viewData, exact: true }).click(), () => expect(page.getByTestId("customer-detail")).toContainText(name));
  249 |       if (reload) await act(page, "Refresh", "server customer remains readable after reload", () => page.reload(), () => expect(page.getByTestId("customer-detail")).toContainText(name), "reload");
  250 |       await expect(page.getByTestId("customer-download"), `PEOPLE-V2-UNKNOWN-FENCE-${reload ? "RELOAD" : "RETURN"}: export stays locked`).toBeDisabled();
  251 |       await expect(page.getByTestId("customer-erase"), `PEOPLE-V2-UNKNOWN-FENCE-${reload ? "RELOAD" : "RETURN"}: erasure stays locked`).toBeDisabled();
  252 |       expect(f.posts).toHaveLength(1);
  253 |       await fits(page);
  254 |     });
  255 |   }
  256 | }
  257 | 
  258 | 
  259 | // PARITY: independent signed-control assertions and full frame captures on the real routed pages.
  260 | const parityCopy = {
  261 |   'zh-TW': { advanced: '進階', marketing: '行銷訊息：已撤回', downloaded: '資料已下載', erased: '顧客已抹除', limit: '資料超出下載上限', blocked: '目前無法抹除', back: '返回顧客資料' },
  262 |   'zh-CN': { advanced: '高级', marketing: '营销消息：已撤回授权', downloaded: '资料已下载', erased: '客户资料已清除', limit: '资料超出下载上限', blocked: '目前无法清除', back: '返回客户资料' },
  263 |   en: { advanced: 'Advanced', marketing: 'Marketing messages: Consent withdrawn', downloaded: 'Data downloaded', erased: 'Customer erased', limit: 'Download limit exceeded', blocked: 'Cannot erase right now', back: 'Back to customer' },
  264 | } as const;
  265 | async function parity(page: Page, desktop: string, mobile: string) {
  266 |   const frame = `1429-${page.viewportSize()?.width === 390 ? mobile : desktop}`;
  267 |   const dir = test.info().outputPath('parity'); await mkdir(dir, { recursive: true });
  268 |   const file = path.join(dir, `${frame}.png`); await page.screenshot({ path: file });
  269 |   await test.info().attach(`parity/${frame}`, { path: file, contentType: 'image/png' });
  270 | }
  271 | for (const locale of ['zh-TW', 'zh-CN', 'en'] as const) for (const width of [1440, 390]) {
  272 |   test(`PARITY people signed controls ${locale}-${width}`, async ({ page }) => {
  273 |     const c = copy[locale], p = parityCopy[locale];
  274 |     await signed(page, width); await fixture(page); await page.goto(mainURL(locale));
  275 |     await expect(page.getByTestId(`customer-open-${customer}`)).toBeVisible();
  276 |     await expect(page.getByRole('button', { name: c.submit, exact: true })).toBeVisible();
  277 |     await expect(page.locator('summary', { hasText: p.advanced }), 'PARITY-PEOPLE: unsigned list disclosure removed').toHaveCount(0);
  278 |     await parity(page, '44136', '44240');
  279 |     await act(page, c.search, 'signed search input is editable', () => page.getByTestId('customers-search').fill('示範林'), () => expect(page.getByTestId('customers-search')).toHaveValue('示範林'), 'fill');
  280 |     await act(page, c.submit, 'signed search results', () => page.getByRole('button', { name: c.submit, exact: true }).click(), () => expect(page.getByTestId(`customer-open-${customer}`)).toBeVisible());
  281 |     await parity(page, '44287', '44349');
  282 |     await act(page, c.view, 'signed profile opens', () => page.getByTestId(`customer-open-${customer}`).click(), () => expect(page.getByTestId('customer-consent')).toBeVisible());
  283 |     await expect(page.locator('summary', { hasText: p.advanced }), 'PARITY-PEOPLE: unsigned detail disclosure removed').toHaveCount(0);
  284 |     await parity(page, '44396', '44485');
  285 |     await act(page, c.download, 'download review opens before POST', () => page.getByRole('button', { name: c.download, exact: true }).click(), () => expect(page.getByRole('button', { name: c.confirmDownload, exact: true })).toBeVisible());
  286 |     await parity(page, '44734', '44772');
  287 |     await page.getByRole('button', { name: p.back, exact: true }).click();
  288 |     await act(page, c.erase, 'erasure review opens', () => page.getByRole('button', { name: c.erase, exact: true }).click(), () => expect(page.getByTestId('erase-typed')).toBeVisible());
  289 |     await parity(page, '45148', '45193');
  290 |     await page.getByTestId('erase-typed').fill(name);
  291 |     await page.getByRole('checkbox', { name: c.checked, exact: true }).check();
  292 |     await expect(page.getByRole('button', { name: c.confirmErase, exact: true })).toBeEnabled();
  293 |     await parity(page, '45229', '45272');
  294 |   });
  295 |   for (const fault of ['none', 'limit', 'blocked', 'read-export', 'read-erase', 'erase-success'] as const) {
  296 |     test(`PARITY people privacy ${fault} ${locale}-${width}`, async ({ page }) => {
  297 |       const c = copy[locale], p = parityCopy[locale];
  298 |       await signed(page, width); const f = await fixture(page, fault); await page.goto(detailURL(locale));
  299 |       const erase = fault === 'blocked' || fault === 'read-erase' || fault === 'erase-success';
  300 |       await expect(page.getByTestId(erase ? 'customer-erase' : 'customer-download')).toBeEnabled();
  301 |       if (erase) {
  302 |         await page.getByTestId('customer-erase').click();
  303 |         await page.getByTestId('erase-typed').fill(name);
  304 |         await page.getByRole('checkbox', { name: c.checked, exact: true }).check();
  305 |       } else await page.getByTestId('customer-download').click();
  306 |       let downloads = 0; page.on('download', () => downloads++);
  307 |       await act(page, erase ? c.confirmErase : c.confirmDownload, 'one privacy command only', () => page.getByRole('button', { name: erase ? c.confirmErase : c.confirmDownload, exact: true }).click(), () => expect.poll(() => f.posts.length).toBe(1));
  308 |       if (fault.startsWith('read-')) {
  309 |         await expect(page.getByRole('button', { name: c.pending, exact: true })).toBeVisible();
  310 |         await parity(page, erase ? '45306' : '44801', erase ? '45347' : '44842');
  311 |         const reads = f.detailReads;
  312 |         await act(page, c.pending, 'read existing request without repeating POST', () => page.getByRole('button', { name: c.pending, exact: true }).click(), async () => {
  313 |           await expect.poll(() => f.detailReads, 'PARITY-PEOPLE: pending lookup actually reads server').toBeGreaterThan(reads);
  314 |           await expect(page.getByRole('button', { name: c.viewData, exact: true })).toBeVisible();
  315 |         });
  316 |         await parity(page, erase ? '45379' : '44874', erase ? '45424' : '44919');
  317 |         await page.getByRole('button', { name: c.viewData, exact: true }).click();
  318 |         await expect(page.getByTestId('customer-download')).toBeDisabled();
  319 |         await expect(page.getByTestId('customer-erase')).toBeDisabled();
  320 |         await parity(page, erase ? '45460' : '44955', erase ? '45516' : '45011');
  321 |         // Restore reads only (fixture preparation), then refresh; the uncertainty marker must remain fenced.
  322 |         await page.unrouteAll({ behavior: 'wait' }); await fixture(page); await page.reload();
  323 |         await expect(page.getByTestId('customer-download')).toBeDisabled();
  324 |         await expect(page.getByTestId('customer-erase')).toBeDisabled();
  325 |       } else {
  326 |         const title = fault === 'limit' ? p.limit : fault === 'blocked' ? p.blocked : fault === 'erase-success' ? p.erased : p.downloaded;
> 327 |         await expect(page.getByRole('heading', { name: title, exact: true }), 'PARITY-PEOPLE: signed dedicated outcome').toBeVisible();
      |                                                                                                                          ^ Error: PARITY-PEOPLE: signed dedicated outcome
  328 |         await parity(page, fault === 'limit' ? '45105' : fault === 'blocked' ? '45563' : fault === 'erase-success' ? '45606' : '45058', fault === 'limit' ? '45131' : fault === 'blocked' ? '45589' : fault === 'erase-success' ? '45634' : '45086');
  329 |         if (fault !== 'none') expect(downloads).toBe(0);
  330 |         else await expect.poll(() => downloads).toBe(1);
  331 |         await act(page, p.back, 'same customer remains readable', () => page.getByRole('button', { name: p.back, exact: true }).last().click(), () => expect(page.getByTestId('customer-orders').locator('tbody tr')).toHaveCount(3));
  332 |         if (fault === 'erase-success') {
  333 |           await expect(page.getByTestId('customer-erased')).toBeVisible();
  334 |           await parity(page, '45653', '45704');
  335 |           await page.reload();
  336 |           await expect(page.getByTestId('customer-erased')).toBeVisible();
  337 |           await expect(page.getByTestId('customer-orders').locator('tbody tr')).toHaveCount(3);
  338 |         }
  339 |       }
  340 |       expect(f.posts).toHaveLength(1);
  341 |     });
  342 |   }
  343 | }
  344 | 
  345 | for (const width of [1440, 390]) {
  346 |   test(`PARITY withdrawn list is a server fact ${width}`, async ({ page }) => {
  347 |     await signed(page, width); await fixture(page, 'none', 'withdrawn'); await page.goto(mainURL('zh-TW'));
  348 |     await expect(page.getByTestId(`customer-row-${customer}`), 'PARITY-CONSENT-STATE: signed list distinguishes recorded withdrawal').toContainText(parityCopy['zh-TW'].marketing);
  349 |     await page.getByTestId(`customer-open-${customer}`).click();
  350 |     await expect(page.getByTestId('consent-marketing_messages')).toContainText('已撤回');
  351 |   });
  352 |   test(`PARITY privacy permission refusal ${width}`, async ({ page }) => {
  353 |     await signed(page, width); const f = await fixture(page, 'forbidden'); await page.goto(detailURL('zh-TW'));
  354 |     await page.getByTestId('customer-download').click();
  355 |     await page.getByRole('button', { name: copy['zh-TW'].confirmDownload, exact: true }).click();
  356 |     await expect(page.getByTestId('customer-download'), 'PARITY-PEOPLE: denied privacy commands stay disabled').toBeDisabled();
  357 |     await expect(page.getByTestId('customer-erase')).toBeDisabled();
  358 |     await expect(page.getByTestId('customer-orders').locator('tbody tr')).toHaveCount(3);
  359 |     await parity(page, '44564', '44654');
  360 |     expect(f.posts).toHaveLength(1);
  361 |   });
  362 | }
  363 | 
```