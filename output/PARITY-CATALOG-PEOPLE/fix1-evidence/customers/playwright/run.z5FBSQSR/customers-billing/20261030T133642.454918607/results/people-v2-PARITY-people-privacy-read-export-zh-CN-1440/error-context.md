# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: people-v2.spec.ts >> PARITY people privacy read-export zh-CN-1440
- Location: tests/admin/people-v2.spec.ts:296:5

# Error details

```
Error: PARITY-PEOPLE: pending lookup actually reads server

PARITY-PEOPLE: pending lookup actually reads server

expect(received).toBeGreaterThan(expected)

Expected: > 1
Received:   1

Call Log:
- Timeout 10000ms exceeded while waiting on the predicate
```

# Page snapshot

```yaml
- generic [ref=f2e1]:
  - generic [ref=f2e2]:
    - link "跳转到内容" [ref=f2e3] [cursor=pointer]:
      - /url: "#main"
    - complementary [ref=f2e4]:
      - generic [ref=f2e5]: DaWan Live
      - navigation "工作区导航" [ref=f2e6]:
        - button "总览" [ref=f2e8] [cursor=pointer]
        - button "直播" [ref=f2e13] [cursor=pointer]:
          - generic [aria-hidden] [ref=f2e17]: +
        - button "订单与发货" [ref=f2e19] [cursor=pointer]
        - button "商品与库存" [ref=f2e24] [cursor=pointer]:
          - generic [aria-hidden] [ref=f2e28]: +
        - button "顾客" [ref=f2e30] [cursor=pointer]
        - button "营销优惠" [ref=f2e35] [cursor=pointer]
        - button "网店" [ref=f2e40] [cursor=pointer]
        - button "收款与报表" [ref=f2e45] [cursor=pointer]
      - navigation "设置" [ref=f2e49]:
        - button "设置" [ref=f2e51] [cursor=pointer]:
          - generic [aria-hidden] [ref=f2e55]: +
    - generic [ref=f2e56]:
      - banner [ref=f2e57]:
        - generic [ref=f2e58]:
          - generic [ref=f2e59]: 切换店铺
          - generic [aria-hidden] [ref=f2e60]:
            - generic [ref=f2e61]: P
            - generic [ref=f2e62]: payment mock store
            - generic [ref=f2e63]: ⌄
          - combobox "切换店铺" [ref=f2e64] [cursor=pointer]:
            - option "payment mock store" [selected]
        - generic [ref=f2e65]:
          - generic [ref=f2e66]: 语言
          - combobox "语言" [ref=f2e67] [cursor=pointer]:
            - option "简体中文" [selected]
            - option "繁體中文"
            - option "English"
        - group [ref=f2e68]:
          - generic "帮助" [ref=f2e69] [cursor=pointer]
        - group [ref=f2e70]:
          - generic "账号" [ref=f2e71] [cursor=pointer]
      - main [ref=f2e72]:
        - navigation "工作区导航" [ref=f2e73]:
          - link "总览" [ref=f2e74] [cursor=pointer]:
            - /url: /zh-CN/
          - generic [aria-hidden] [ref=f2e75]: /
          - generic [ref=f2e76]: 顾客
          - generic [aria-hidden] [ref=f2e77]: /
          - generic [ref=f2e78]: 顾客详情
        - generic [ref=f2e79]:
          - generic [ref=f2e80]:
            - heading "客户" [level=1] [ref=f2e82]
            - link "返回搜索结果" [ref=f2e84] [cursor=pointer]:
              - /url: /zh-CN/customers?store=3c0420c6-d65b-4803-8503-7568578868d6
          - status [ref=f2e85]:
            - heading "已有下载申请待处理" [level=2] [ref=f2e86]
            - paragraph [ref=f2e87]: payment mock store · 示範林一客户编号 1cfb962d · 手机尾号 805
            - paragraph [ref=f2e88]: 暂时不能下载或清除资料，请先查看客户记录。
            - button "查看客户资料" [active] [ref=f2e89] [cursor=pointer]
        - paragraph [ref=f2e91]: DaWan Live 是 香港大碗貿易有限公司 的直播电商平台
  - alert [ref=f2e92]
```

# Test source

```ts
  213 |     await expect(page.getByTestId("customer-erased")).toHaveCount(0);
  214 |     await act(page, "Refresh", "refused erasure keeps all three server orders", () => page.reload(), async () => { await expect(page.getByTestId("customer-orders").locator("tbody tr")).toHaveCount(3); await expect(page.getByTestId("customer-erased")).toHaveCount(0); }, "reload");
  215 |     expect(f.posts).toHaveLength(1);
  216 |   });
  217 |   for (const kind of ["export", "erase"] as const) {
  218 |     test(`PEOPLE-V2 ${kind} uncertain read-only return en-${width}`, async ({ page }) => {
  219 |       await signed(page, width); const f = await fixture(page, kind); await page.goto(detailURL("en"));
  220 |       if (kind === "erase") { await expect(page.getByTestId("customer-erase")).toBeVisible(); await eraseReady(page); }
  221 |       else {
  222 |         await expect(page.getByTestId("customer-download")).toBeVisible();
  223 |         await act(page, copy.en.download, "PEOPLE-V2-EXPORT-CONFIRM: opens confirmation", () => page.getByTestId("customer-download").click(), () => expect(page.getByRole("button", { name: copy.en.confirmDownload, exact: true })).toBeVisible());
  224 |       }
  225 |       await act(page, kind === "erase" ? copy.en.confirmErase : copy.en.confirmDownload, "unconfirmed result offers only the signed read-only path", () => kind === "erase" ? page.getByTestId("erase-submit").click() : page.getByRole("button", { name: copy.en.confirmDownload, exact: true }).click(), () => expect(page.getByRole("button", { name: copy.en.pending, exact: true })).toBeVisible());
  226 |       expect(f.posts).toHaveLength(1);
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
> 313 |           await expect.poll(() => f.detailReads, 'PARITY-PEOPLE: pending lookup actually reads server').toBeGreaterThan(reads);
      |                                                                                                         ^ Error: PARITY-PEOPLE: pending lookup actually reads server
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
  327 |         await expect(page.getByRole('heading', { name: title, exact: true }), 'PARITY-PEOPLE: signed dedicated outcome').toBeVisible();
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