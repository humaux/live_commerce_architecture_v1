# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: catalog-core.spec.ts >> CATV2 signed simplified catalog (MOCK responses) >> PARITY catalog signed controls zh-TW 390
- Location: tests/admin/catalog-v2-simplified.acceptance.ts:154:7

# Error details

```
Error: PARITY-CATALOG: unsigned search removed

expect(locator).toHaveCount(expected) failed

Locator:  getByTestId('products-search')
Expected: 0
Received: 1
Timeout:  10000ms

Call log:
  - PARITY-CATALOG: unsigned search removed getByTestId('products-search') with timeout 10000ms
  - waiting for getByTestId('products-search')
    24 × locator resolved to 1 element
       - unexpected value "1"

```

# Page snapshot

```yaml
- generic [active] [ref=f2e1]:
  - generic [ref=f2e2]:
    - link "跳至內容" [ref=f2e3] [cursor=pointer]:
      - /url: "#main"
    - generic [ref=f2e4]:
      - banner [ref=f2e5]:
        - button "開啟導覽" [ref=f2e6] [cursor=pointer]
        - generic [ref=f2e9]:
          - generic [ref=f2e10]: 切換商店
          - generic [aria-hidden] [ref=f2e11]:
            - generic [ref=f2e12]: fixture-store-b
            - generic [ref=f2e13]: ⌄
          - combobox "切換商店" [ref=f2e14] [cursor=pointer]:
            - option "fixture-store-b" [selected]
        - generic [ref=f2e15]:
          - generic [ref=f2e16]: 語言
          - combobox "語言" [ref=f2e17] [cursor=pointer]:
            - option "简体中文"
            - option "繁體中文" [selected]
            - option "English"
        - group [ref=f2e18]:
          - generic "說明" [ref=f2e19] [cursor=pointer]
        - group [ref=f2e20]:
          - generic "帳號" [ref=f2e21] [cursor=pointer]
      - main [ref=f2e22]:
        - navigation "工作區導覽" [ref=f2e23]:
          - link "總覽" [ref=f2e24] [cursor=pointer]:
            - /url: /zh-TW/
          - generic [aria-hidden] [ref=f2e25]: /
          - generic [ref=f2e26]: 商品與庫存
          - generic [aria-hidden] [ref=f2e27]: /
          - generic [ref=f2e28]: 商品
        - generic [ref=f2e29]:
          - generic [ref=f2e30]:
            - heading "商品" [level=1] [ref=f2e32]
            - link "新增商品" [ref=f2e34] [cursor=pointer]:
              - /url: /zh-TW/products/new?store=dda8ce98-4c10-4bbb-805e-28ec149483e2
          - group "狀態" [ref=f2e36]:
            - button "全部 3" [pressed] [ref=f2e37] [cursor=pointer]
            - button "在售 2" [ref=f2e38] [cursor=pointer]
            - button "草稿 1" [ref=f2e39] [cursor=pointer]
            - button "已歸檔 0" [ref=f2e40] [cursor=pointer]
          - search [ref=f2e41]:
            - searchbox "搜尋名稱、網址識別或 SKU 編碼" [ref=f2e42]
            - button "搜尋" [ref=f2e43] [cursor=pointer]
          - generic [ref=f2e47]:
            - checkbox "選取 3 件" [ref=f2e49]
            - group [ref=f2e51]:
              - generic "進階" [ref=f2e52]
          - table "商品" [ref=f2e53]:
            - rowgroup [ref=f2e54]:
              - row [ref=f2e55]:
                - columnheader "選取商品" [ref=f2e56]
                - columnheader "商品" [ref=f2e57]
                - columnheader "狀態" [ref=f2e58]
                - columnheader "價格 · 庫存" [ref=f2e59]
                - columnheader "編輯" [ref=f2e60]
            - rowgroup [ref=f2e61]:
              - row [ref=f2e62]:
                - cell [ref=f2e63]:
                  - 'checkbox "選取商品: 帆布提袋" [ref=f2e65]'
                - cell [ref=f2e66]:
                  - generic [ref=f2e67]:
                    - img "帆布提袋" [ref=f2e68]:
                      - generic [aria-hidden] [ref=f2e69]: 帆
                    - strong [ref=f2e71]: 帆布提袋
                - cell "在售" [ref=f2e72]
                - cell "NT$1,280 · 可售 3" [ref=f2e73]
                - cell [ref=f2e75]:
                  - link "編輯" [ref=f2e77] [cursor=pointer]:
                    - /url: /zh-TW/products/11111111-1111-4111-8111-000000000001?store=dda8ce98-4c10-4bbb-805e-28ec149483e2
              - row [ref=f2e78]:
                - cell [ref=f2e79]:
                  - 'checkbox "選取商品: 棉質上衣" [ref=f2e81]'
                - cell [ref=f2e82]:
                  - generic [ref=f2e83]:
                    - img "棉質上衣" [ref=f2e84]:
                      - generic [aria-hidden] [ref=f2e85]: 棉
                    - strong [ref=f2e87]: 棉質上衣
                - cell "在售" [ref=f2e88]
                - cell "NT$690 · 售罄" [ref=f2e89]
                - cell [ref=f2e91]:
                  - link "編輯" [ref=f2e93] [cursor=pointer]:
                    - /url: /zh-TW/products/11111111-1111-4111-8111-000000000002?store=dda8ce98-4c10-4bbb-805e-28ec149483e2
              - row [ref=f2e94]:
                - cell [ref=f2e95]:
                  - 'checkbox "選取商品: 保溫杯" [ref=f2e97]'
                - cell [ref=f2e98]:
                  - generic [ref=f2e99]:
                    - img "保溫杯" [ref=f2e100]:
                      - generic [aria-hidden] [ref=f2e101]: 保
                    - strong [ref=f2e103]: 保溫杯
                - cell "草稿" [ref=f2e104]
                - cell "NT$490 · 可售 16" [ref=f2e105]
                - cell [ref=f2e107]:
                  - link "編輯" [ref=f2e109] [cursor=pointer]:
                    - /url: /zh-TW/products/11111111-1111-4111-8111-000000000003?store=dda8ce98-4c10-4bbb-805e-28ec149483e2
        - paragraph [ref=f2e111]: DaWan Live 是 香港大碗貿易有限公司 的直播電商平台
  - alert [ref=f2e112]
```

# Test source

```ts
  60  |     if (resource !== "products/bulk-status" && resource !== `products/${productID}/document`) return route.continue();
  61  |     const body = request.postDataJSON();
  62  |     writes.push({ method: request.method(), resource, body });
  63  |     expect(request.headers()["idempotency-key"], "each UI command has an idempotency key").toBeTruthy();
  64  |     expect(body).not.toHaveProperty("tenant_id");
  65  |     if (resource === "products/bulk-status") {
  66  |       expect(request.method()).toBe("POST");
  67  |       expect(body.status).toBe("archived");
  68  |       expect(body.ids).toEqual(fixture.rows.map((r) => r.id));
  69  |       return respond(route, fixture.rows.map((row, i) => {
  70  |         if (i === 1) return { id: row.id, error: "live_window_open" };
  71  |         row.status = "archived";
  72  |         return { id: row.id, status: row.status };
  73  |       }));
  74  |     }
  75  |     expect(request.method()).toBe("PUT");
  76  |     expect(body.expected_version).toBe(fixture.detail.version);
  77  |     if (body.skus?.some((s: any) => s.stock?.target_qty !== undefined && s.stock.target_qty < (fixture.detail.skus.find((old) => old.id === s.id)?.committed ?? 0)))
  78  |       return respond(route, { code: "insufficient_inventory" }, 409);
  79  |     for (const field of ["name", "description", "status"] as const) if (field in body) fixture.detail[field] = body[field];
  80  |     for (const change of body.skus ?? []) {
  81  |       const sku = fixture.detail.skus.find((s) => s.id === change.id);
  82  |       expect(sku, "edit targets an existing server SKU").toBeTruthy();
  83  |       if (change.price_minor !== undefined) sku!.price_minor = change.price_minor;
  84  |       if (change.stock?.target_qty !== undefined) { sku!.on_hand = change.stock.target_qty; sku!.available = sku!.on_hand - sku!.committed; }
  85  |     }
  86  |     fixture.detail.version++;
  87  |     fixture.rows[0].name = fixture.detail.name;
  88  |     return respond(route, { id: productID });
  89  |   });
  90  |   await page.goto(`${origin}/en/`);
  91  |   await page.getByRole("button", { name: "Sign in with identity service" }).click();
  92  |   await expect(page.getByTestId("nav-orders")).toBeAttached();
  93  |   const open = (suffix = "") => page.goto(`${origin}/${locale}/products${suffix}?store=${store}`);
  94  |   return { ...fixture, writes, open };
  95  | }
  96  | 
  97  | async function action(ledger: Ledger, page: Page, control: string, operation: string, expected: string, run: () => Promise<void>) {
  98  |   const row: Ledger[number] = { page: new URL(page.url()).pathname, control, operation, expected, actual: "assertion pending", status: "FAIL" };
  99  |   ledger.push(row);
  100 |   await run();
  101 |   row.actual = expected; row.status = "PASS";
  102 | }
  103 | async function saveLedger(ledger: Ledger, info: TestInfo) {
  104 |   const file = info.outputPath("catalog-v2-click-ledger.json");
  105 |   await writeFile(file, JSON.stringify({ evidence: "MOCK", revision: process.env.GITHUB_SHA ?? "UNKNOWN", ledger }, null, 2));
  106 |   await info.attach("catalog-v2-click-ledger", { path: file, contentType: "application/json" });
  107 | }
  108 | 
  109 | const mobileDensity = new Map<string, { blocks: number; chars: number }>();
  110 | /** READ/MEASURE only: count visible text parents plus editable values inside the task surface, including scroll content. */
  111 | async function density(page: Page, state: string, locale: Locale) {
  112 |   // Real route captures, kept beside the click ledger for independent signed-frame review.
  113 |   const pairs: Record<string, [string, string]> = {
  114 |     list: ['62828','62906'], active: ['63218','63287'], draft: ['63348','63405'], archived: ['63453','63499'],
  115 |     selected: ['63540','63620'], bulkReview: ['63693','63740'], bulkResult: ['63782','63827'], updated: ['63867','63937'],
  116 |     edit: ['64000','64049'], variants: ['64093','64149'], stock: ['64200','64239'], stockError: ['64273','64314'],
  117 |     media: ['64350','64391'], review: ['64427','64474'], saved: ['64516','64561'],
  118 |   };
  119 |   if (locale === 'zh-TW' && pairs[state]) {
  120 |     const frame = `1429-${pairs[state][page.viewportSize()?.width === 390 ? 1 : 0]}`;
  121 |     const file = test.info().outputPath('parity', `${frame}.png`);
  122 |     await mkdir(test.info().outputPath('parity'), { recursive: true });
  123 |     await page.screenshot({ path: file });
  124 |     await test.info().attach(`parity/${frame}`, { path: file, contentType: 'image/png' });
  125 |   }
  126 |   if (locale !== "zh-CN" || page.viewportSize()?.width !== 390) return;
  127 |   const value = await page.locator('[data-testid="products-page"], [data-testid="product-editor"]').first().evaluate((root) => {
  128 |     const visible = (e: Element) => {
  129 |       for (let p: Element | null = e; p; p = p.parentElement) {
  130 |         const style = getComputedStyle(p);
  131 |         if (style.display === "none" || style.visibility === "hidden" || Number(style.opacity) === 0 || p.hasAttribute("hidden")) return false;
  132 |       }
  133 |       return e.getClientRects().length > 0;
  134 |     };
  135 |     const parents = new Set<Element>();
  136 |     const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
  137 |     while (walker.nextNode()) {
  138 |       const text = walker.currentNode;
  139 |       if (text.textContent?.trim() && text.parentElement && visible(text.parentElement) && !text.parentElement.closest('script,style,[aria-hidden="true"]')) parents.add(text.parentElement);
  140 |     }
  141 |     const texts = [...parents].map((e) => [...e.childNodes].filter((n) => n.nodeType === Node.TEXT_NODE).map((n) => n.textContent).join("").trim()).filter(Boolean);
  142 |     for (const input of root.querySelectorAll<HTMLInputElement | HTMLTextAreaElement>('input:not([type="checkbox"]):not([type="radio"]):not([type="hidden"]):not([type="file"]),textarea'))
  143 |       if (visible(input) && input.value.trim()) texts.push(input.value.trim());
  144 |     return { blocks: texts.length, chars: texts.reduce((n, text) => n + Array.from(text.replace(/\s/g, "")).length, 0) };
  145 |   });
  146 |   mobileDensity.set(state, value);
  147 | }
  148 | 
  149 | /** Register one batch; original CC12/PE assertions and environments are unchanged. */
  150 | export function registerCatalogSimplifiedAcceptance() {
  151 |   test.describe("CATV2 signed simplified catalog (MOCK responses)", () => {
  152 |     test.describe.configure({ mode: "default" });
  153 |     for (const locale of ["zh-TW", "zh-CN", "en"] as const) for (const width of [390, 1440]) {
  154 |       test(`PARITY catalog signed controls ${locale} ${width}`, async ({ page }, info) => {
  155 |         const c = signed[locale], ledger: Ledger = [];
  156 |         await page.setViewportSize({ width, height: width === 390 ? 844 : 900 });
  157 |         try {
  158 |           const f = await setup(page, locale); await f.open();
  159 |           await expect(page.getByTestId(`product-row-${productID}`)).toBeVisible();
> 160 |           await expect(page.getByTestId('products-search'), 'PARITY-CATALOG: unsigned search removed').toHaveCount(0);
      |                                                                                                        ^ Error: PARITY-CATALOG: unsigned search removed
  161 |           await expect(page.getByTestId('products-advanced'), 'PARITY-CATALOG: unsigned advanced removed').toHaveCount(0);
  162 |           await expect(page.getByText(c.select, { exact: true }), 'PARITY-CATALOG: visible selection label').toBeVisible();
  163 |           // READ/MEASURE only: signed chips have a complete rounded border, never an underline-only tab.
  164 |           const chip = await page.getByRole('button', { name: c.all, exact: true }).evaluate(el => {
  165 |             const css = getComputedStyle(el); return { radius: parseFloat(css.borderRadius), top: css.borderTopStyle, bottom: css.borderBottomStyle };
  166 |           });
  167 |           expect(chip.top).toBe('solid'); expect(chip.bottom).toBe('solid'); expect(chip.radius).toBeGreaterThanOrEqual(4);
  168 |           await f.open(`/${productID}`);
  169 |           await expect(page.getByTestId('product-name')).toHaveValue(c.names[0]);
  170 |           const active = { 'zh-TW': '在售', 'zh-CN': '在售', en: 'Active' }[locale];
  171 |           await action(ledger, page, active, 'click', 'signed status radio remains checked', async () => {
  172 |             await page.getByRole('radio', { name: active, exact: true }).click();
  173 |             await expect(page.getByRole('radio', { name: active, exact: true })).toBeChecked();
  174 |           });
  175 |           const bar = page.getByTestId('catalog-save-bar');
  176 |           await expect(bar).toBeVisible();
  177 |           expect(await bar.evaluate(el => getComputedStyle(el).position)).toMatch(/sticky|fixed/);
  178 |           await page.getByRole('button', { name: c.variants, exact: true }).click();
  179 |           await expect(page.getByTestId('catalog-price-0')).toBeVisible();
  180 |           await expect(page.getByTestId('catalog-price-1')).toBeVisible();
  181 |           await page.getByTestId('catalog-stock-0').click();
  182 |           await expect(page.getByTestId('catalog-stock-tracked')).toHaveCount(0);
  183 |           await expect(page.getByTestId('catalog-stock-delta')).toHaveCount(0);
  184 |           await expect(page.getByLabel(c.total, { exact: true })).toBeVisible();
  185 |           expect(f.writes).toHaveLength(0);
  186 |         } finally { await saveLedger(ledger, info); }
  187 |       });
  188 |     }
  189 | 
  190 |     for (const locale of ["zh-TW", "zh-CN", "en"] as const) for (const width of [390, 1440]) {
  191 |       test(`CATV2 list ${locale} ${width}: signed copy, all filters and row identity`, async ({ page }, info) => {
  192 |         const ledger: Ledger = [], c = signed[locale];
  193 |         await page.setViewportSize({ width, height: width === 390 ? 844 : 900 });
  194 |         try {
  195 |           const f = await setup(page, locale); await f.open();
  196 |           const screen = page.getByTestId("products-page");
  197 |           await expect(screen).toBeVisible();
  198 |           await expect(screen.getByText(c.names[0], { exact: true }).first()).toBeVisible();
  199 |           await density(page, "list", locale);
  200 |           // Causal RED assertion on existing rendered controls, never on an absent module import.
  201 |           await expect(screen.getByTestId("quick-price"), "signed rows expose one Edit action rather than inline price/stock editors").toHaveCount(0);
  202 |           await expect(screen.getByTestId("quick-stock")).toHaveCount(0);
  203 |           await expect(screen.getByRole("heading", { name: c.title, exact: true })).toHaveCount(1);
  204 |           for (const name of c.names) await expect(screen.getByText(name, { exact: true }).first()).toBeVisible();
  205 |           await expect(screen.getByText(c.soldOut, { exact: false }).first()).toBeVisible();
  206 |           for (const [status, label, count] of [["active", c.active, 2], ["draft", c.draft, 1], ["archived", c.archived, 0], ["all", c.all, 3]] as const) {
  207 |             await action(ledger, page, label, "click", `${count} matching products after refresh`, async () => {
  208 |               await page.getByTestId(`products-tab-${status}`).click();
  209 |               await expect(page.getByTestId(`products-tab-${status}`)).toHaveAttribute("aria-pressed", "true");
  210 |               await expect(page.locator('[data-testid^="product-row-"]')).toHaveCount(count);
  211 |               if (!count) await expect(screen.getByText(c.empty, { exact: true })).toBeVisible();
  212 |               await density(page, status, locale);
  213 |               await page.reload();
  214 |               await expect(page.locator('[data-testid^="product-row-"]')).toHaveCount(count);
  215 |             });
  216 |           }
  217 |           await action(ledger, page, `${c.names[0]} ${c.edit}`, "click", "the clicked product's editor", async () => {
  218 |             await page.getByTestId(`product-row-${productID}`).getByRole("link", { name: c.edit, exact: true }).click();
  219 |             await expect(page).toHaveURL(new RegExp(`/products/${productID}`));
  220 |             await expect(page.getByLabel(c.name, { exact: true })).toHaveValue(c.names[0]);
  221 |           });
  222 |           // READ/MEASURE only: containment, not DOM mutation or a substitute for user input.
  223 |           expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1);
  224 |           expect(f.writes).toHaveLength(0);
  225 |         } finally { await saveLedger(ledger, info); }
  226 |       });
  227 |     }
  228 |     for (const locale of ["zh-TW", "zh-CN", "en"] as const) for (const width of [390, 1440]) {
  229 |       test(`CATV2 archive ${locale} ${width}: selection, cancel, per-item outcomes and persisted refresh`, async ({ page }, info) => {
  230 |         const ledger: Ledger = [], c = signed[locale];
  231 |         await page.setViewportSize({ width, height: width === 390 ? 844 : 900 });
  232 |         try {
  233 |           const f = await setup(page, locale); await f.open();
  234 |           await expect(page.getByTestId(`product-row-${productID}`)).toBeVisible();
  235 |           await action(ledger, page, c.select, "click", "all three products selected", async () => {
  236 |             await page.getByRole("checkbox", { name: c.select, exact: true }).click();
  237 |             await expect(page.getByRole("button", { name: c.archive, exact: true })).toBeEnabled();
  238 |             await density(page, "selected", locale);
  239 |           });
  240 |           await action(ledger, page, c.archive, "click", "review identities and archive impact without sending", async () => {
  241 |             await page.getByRole("button", { name: c.archive, exact: true }).click();
  242 |             await expect(page.getByRole("heading", { name: c.archiveTitle, exact: true })).toBeVisible();
  243 |             for (const name of c.names) await expect(page.getByText(name, { exact: true }).first()).toBeVisible();
  244 |             await expect(page.getByText(c.archivePolicy, { exact: true })).toBeVisible();
  245 |             await density(page, "bulkReview", locale);
  246 |             expect(f.writes).toHaveLength(0);
  247 |           });
  248 |           await page.getByRole("button", { name: c.backSelection, exact: true }).click();
  249 |           expect(f.writes).toHaveLength(0);
  250 |           await page.getByRole("button", { name: c.archive, exact: true }).click();
  251 |           await action(ledger, page, c.confirmArchive, "click", "two archived, one unchanged with reason", async () => {
  252 |             await page.getByRole("button", { name: c.confirmArchive, exact: true }).click();
  253 |             await expect(page.getByText(c.result, { exact: true })).toBeVisible();
  254 |             await expect(page.getByText(c.live, { exact: true })).toBeVisible();
  255 |             await density(page, "bulkResult", locale);
  256 |             expect(f.writes).toHaveLength(1);
  257 |           });
  258 |           await page.getByRole("button", { name: c.view, exact: true }).click();
  259 |           await page.reload();
  260 |           await expect(page.getByTestId("products-tab-archived")).toContainText("2");
```