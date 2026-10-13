# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: catalog-core.spec.ts >> PE12-17 document workflow, matrix, list actions and click ledger
- Location: tests/admin/product-editor.acceptance.ts:17:3

# Error details

```
Error: expect(received).toBe(expected) // Object.is equality

Expected: true
Received: false

Call Log:
- Timeout 10000ms exceeded while waiting on the predicate
```

# Page snapshot

```yaml
- generic [ref=f8e1]:
  - alert [ref=f8e2]: 新增商品
  - generic [ref=f8e3]:
    - link "跳转到内容" [ref=f8e4] [cursor=pointer]:
      - /url: "#main"
    - generic [ref=f8e5]:
      - banner [ref=f8e6]:
        - button "打开导航" [ref=f8e7] [cursor=pointer]
        - generic [ref=f8e10]:
          - generic [ref=f8e11]: 切换店铺
          - generic [aria-hidden] [ref=f8e12]:
            - generic [ref=f8e13]: fixture-store-b
            - generic [ref=f8e14]: ⌄
          - combobox "切换店铺" [ref=f8e15] [cursor=pointer]:
            - option "fixture-store-b" [selected]
        - generic [ref=f8e16]:
          - generic [ref=f8e17]: 语言
          - combobox "语言" [ref=f8e18] [cursor=pointer]:
            - option "简体中文" [selected]
            - option "繁體中文"
            - option "English"
        - group [ref=f8e19]:
          - generic "帮助" [ref=f8e20] [cursor=pointer]
        - group [ref=f8e21]:
          - generic "账号" [ref=f8e22] [cursor=pointer]
      - main [ref=f8e23]:
        - navigation "工作区导航" [ref=f8e24]:
          - link "总览" [ref=f8e25] [cursor=pointer]:
            - /url: /zh-CN/
          - generic [aria-hidden] [ref=f8e26]: /
          - generic [ref=f8e27]: 商品与库存
          - generic [aria-hidden] [ref=f8e28]: /
          - generic [ref=f8e29]: 新增商品
        - generic [ref=f8e30]:
          - generic [ref=f8e31]:
            - heading "新增商品" [level=1] [ref=f8e33]
            - link "全部商品" [ref=f8e35] [cursor=pointer]:
              - /url: /zh-CN/products?store=75e5a0d4-2509-4322-91ad-d46576b6f1c9
          - generic [ref=f8e36]:
            - complementary [ref=f8e37]:
              - navigation "完成度" [ref=f8e38]:
                - button "商品图片" [ref=f8e39] [cursor=pointer]
                - button "基本信息" [ref=f8e40] [cursor=pointer]
                - button "价格与库存" [ref=f8e41] [cursor=pointer]
                - button "规格" [ref=f8e42] [cursor=pointer]
                - button "分类" [ref=f8e43] [cursor=pointer]
                - button "物流" [ref=f8e44] [cursor=pointer]
                - button "搜索引擎优化" [ref=f8e45] [cursor=pointer]
              - button "完成度" [expanded] [ref=f8e46] [cursor=pointer]
              - generic [ref=f8e47]:
                - heading "完成度" [level=2] [ref=f8e48]
                - heading "必填" [level=3] [ref=f8e49]
                - button "待填写 图片" [active] [ref=f8e50] [cursor=pointer]:
                  - generic [ref=f8e51]: 待填写
                  - generic [ref=f8e52]: 图片
                - button "待填写 商品名称" [ref=f8e53] [cursor=pointer]:
                  - generic [ref=f8e54]: 待填写
                  - generic [ref=f8e55]: 商品名称
                - button "待填写 售价" [ref=f8e56] [cursor=pointer]:
                  - generic [ref=f8e57]: 待填写
                  - generic [ref=f8e58]: 售价
                - button "待填写 库存" [ref=f8e59] [cursor=pointer]:
                  - generic [ref=f8e60]: 待填写
                  - generic [ref=f8e61]: 库存
                - paragraph [ref=f8e62]: "还缺必填项目: 4"
                - heading "建议" [level=3] [ref=f8e63]
                - button "待填写 至少 3 张图片" [ref=f8e64] [cursor=pointer]:
                  - generic [ref=f8e65]: 待填写
                  - generic [ref=f8e66]: 至少 3 张图片
                - button "待填写 描述" [ref=f8e67] [cursor=pointer]:
                  - generic [ref=f8e68]: 待填写
                  - generic [ref=f8e69]: 描述
                - button "待填写 分类" [ref=f8e70] [cursor=pointer]:
                  - generic [ref=f8e71]: 待填写
                  - generic [ref=f8e72]: 分类
                - button "待填写 直播关键字" [ref=f8e73] [cursor=pointer]:
                  - generic [ref=f8e74]: 待填写
                  - generic [ref=f8e75]: 直播关键字
                - button "待填写 搜索引擎优化" [ref=f8e76] [cursor=pointer]:
                  - generic [ref=f8e77]: 待填写
                  - generic [ref=f8e78]: 搜索引擎优化
            - generic [ref=f8e79]:
              - region "商品图片" [ref=f8e80]:
                - region [ref=f8e81]:
                  - heading "主图 0/4" [level=2] [ref=f8e82]:
                    - text: 主图
                    - generic [ref=f8e83]: 0/4
                  - paragraph [ref=f8e84]: 最多4张，第一张为封面。建议使用800px以上的正方形照片。
                  - paragraph [ref=f8e86]: 尚未新增图片。
                  - generic [ref=f8e87] [cursor=pointer]:
                    - generic [ref=f8e88]: 新增图片
                    - 'button "新增图片: 主图" [ref=f8e89]'
                - region [ref=f8e90]:
                  - heading "规格图" [level=2] [ref=f8e91]
                  - paragraph [ref=f8e92]: 每个图片规格值各一张；没有规格图时，买家会看到封面。
                  - generic [ref=f8e93]:
                    - generic [ref=f8e94]: 图片规格
                    - combobox "图片规格" [disabled] [ref=f8e95]:
                      - option "使用第一个规格" [selected]
                  - paragraph [ref=f8e96]: 请先新增并保存商品规格，才能上传规格图。
                - region [ref=f8e97]:
                  - heading "详情图 0/20" [level=2] [ref=f8e98]:
                    - text: 详情图
                    - generic [ref=f8e99]: 0/20
                  - paragraph [ref=f8e100]: 最多20张，可上传长图，高度最多为宽度的6倍。
                  - paragraph [ref=f8e102]: 尚未新增图片。
                  - generic [ref=f8e103] [cursor=pointer]:
                    - generic [ref=f8e104]: 新增图片
                    - 'button "新增图片: 详情图" [ref=f8e105]'
              - group [ref=f8e106]:
                - generic [ref=f8e107]:
                  - heading "基本信息" [level=2] [ref=f8e108]
                  - generic [ref=f8e109]:
                    - text: 商品名称
                    - textbox "商品名称 0 / 120" [ref=f8e110]
                    - generic [ref=f8e111]: 0 / 120
                  - generic [ref=f8e112]:
                    - text: 描述
                    - textbox "描述" [ref=f8e113]
                  - generic [ref=f8e114]:
                    - text: 直播关键字
                    - textbox "直播关键字 选填 · 英文及数字，每个规格一个关键字，店内不可重复。" [ref=f8e115]
                    - generic [ref=f8e116]: 选填 · 英文及数字，每个规格一个关键字，店内不可重复。
                - generic [ref=f8e117]:
                  - heading "价格与库存" [level=2] [ref=f8e118]
                  - generic [ref=f8e119]:
                    - generic [ref=f8e120]:
                      - text: 售价 (NT$)
                      - textbox "售价 (NT$)" [ref=f8e121]
                    - generic [ref=f8e122]:
                      - text: 原价 (NT$)
                      - textbox "原价 (NT$)" [ref=f8e123]
                  - generic [ref=f8e124]:
                    - generic [ref=f8e125]:
                      - radio "追踪数量" [checked] [ref=f8e126]
                      - text: 追踪数量
                    - generic [ref=f8e127]:
                      - radio "不追踪 ∞" [ref=f8e128]
                      - text: 不追踪 ∞
                  - generic [ref=f8e129]:
                    - text: 数量
                    - textbox "数量" [ref=f8e130]
                  - group [ref=f8e131]:
                    - generic "+ 高级" [ref=f8e132] [cursor=pointer]
                - region [ref=f8e133]:
                  - heading "规格" [level=2] [ref=f8e134]
                  - button "添加规格（颜色、尺寸…）" [ref=f8e135] [cursor=pointer]
                - generic [ref=f8e136]:
                  - heading "分类" [level=2] [ref=f8e137]
                  - generic [ref=f8e138]:
                    - text: 搜索分类
                    - searchbox "搜索分类" [ref=f8e139]
                  - paragraph [ref=f8e140]: 暂无分类
                  - link "管理分类" [ref=f8e141] [cursor=pointer]:
                    - /url: /zh-CN/collections?store=75e5a0d4-2509-4322-91ad-d46576b6f1c9
                - group [ref=f8e142]:
                  - generic "+ 物流" [ref=f8e143] [cursor=pointer]
                - group [ref=f8e144]:
                  - generic "+ 搜索引擎优化" [ref=f8e145] [cursor=pointer]
            - generic [ref=f8e147]:
              - button "保存草稿" [ref=f8e148] [cursor=pointer]
              - button "上架" [ref=f8e149] [cursor=pointer]
        - paragraph [ref=f8e151]: DaWan Live 由 香港大碗貿易有限公司 运营
```

# Test source

```ts
  70  |       const file = path.join(out, `${name}.png`);
  71  |       if (/^(list|editor)-(zh-TW|zh-CN|en)-/.test(name)) {
  72  |         await page
  73  |           .locator("header[data-shell-topbar]")
  74  |           .scrollIntoViewIfNeeded();
  75  |         await expect.poll(() => page.evaluate(() => scrollY)).toBe(0);
  76  |         await expect
  77  |           .poll(
  78  |             async () =>
  79  |               (await page.locator("header[data-shell-topbar]").boundingBox())
  80  |                 ?.y,
  81  |           )
  82  |           .toBe(0);
  83  |       }
  84  |       await page.screenshot({ path: file, animations: "disabled" });
  85  |       shots.push({
  86  |         file,
  87  |         geometry: await page.evaluate(() => ({
  88  |           scrollY,
  89  |           width: innerWidth,
  90  |           height: innerHeight,
  91  |           topbarY: document
  92  |             .querySelector("header[data-shell-topbar]")
  93  |             ?.getBoundingClientRect().y,
  94  |         })),
  95  |         sha256: createHash("sha256")
  96  |           .update(await readFile(file))
  97  |           .digest("hex"),
  98  |       });
  99  |     };
  100 |     const fit = async () =>
  101 |       expect(
  102 |         await page.evaluate(
  103 |           () => document.documentElement.scrollWidth - innerWidth,
  104 |         ),
  105 |       ).toBeLessThanOrEqual(1);
  106 |     const url = (locale: string, suffix: string) =>
  107 |       `${origin}/${locale}/${suffix}${suffix.includes("?") ? "&" : "?"}store=${store}`;
  108 |     try {
  109 |       await page.goto(origin + "/en/");
  110 |       await page
  111 |         .getByRole("button", { name: "Sign in with identity service" })
  112 |         .click();
  113 |       await expect(page.getByTestId("nav-orders")).toBeAttached();
  114 |       // Visual/real navigation sweep. Dirty drafts are intentionally discarded by the user-confirmation path.
  115 |       page.on("dialog", (dialog) => void dialog.accept());
  116 |       for (const locale of ["zh-TW", "zh-CN", "en"] as const) {
  117 |         for (const [width, height] of [
  118 |           [1366, 768],
  119 |           [1586, 992],
  120 |           [390, 844],
  121 |           [375, 812],
  122 |         ]) {
  123 |           await page.setViewportSize({ width, height });
  124 |           await page.goto(url(locale, "products"));
  125 |           await expect(page.getByTestId("products-table")).toBeVisible();
  126 |           await fit();
  127 |           const targets = await page
  128 |             .locator(
  129 |               ".pe-select,.pe-row-actions a,[data-testid=quick-price],[data-testid=quick-stock]",
  130 |             )
  131 |             .evaluateAll((nodes) =>
  132 |               nodes
  133 |                 .filter((n) => n.getClientRects().length)
  134 |                 .map((n) => {
  135 |                   const r = n.getBoundingClientRect();
  136 |                   return { width: r.width, height: r.height };
  137 |                 }),
  138 |             );
  139 |           for (const target of targets) {
  140 |             expect(target.width).toBeGreaterThanOrEqual(44);
  141 |             expect(target.height).toBeGreaterThanOrEqual(44);
  142 |           }
  143 |           await shot(`list-${locale}-${width}`);
  144 |           await page.getByTestId("product-new").click();
  145 |           if (width === 390) {
  146 |             const toggle = page.locator(".pe-readiness-toggle");
  147 |             const checklist = page.locator("#pe-readiness-section");
  148 |             await expect(toggle).toBeVisible();
  149 |             await expect(toggle).toHaveAttribute("aria-expanded", "false");
  150 |             await expect(checklist).toBeHidden();
  151 |             await toggle.click();
  152 |             await expect(toggle).toHaveAttribute("aria-expanded", "true");
  153 |             await expect(checklist).toBeVisible();
  154 |             const missing = checklist.locator(".pe-readiness:has([data-ready=false])");
  155 |             const missingCount = await missing.count();
  156 |             expect(missingCount).toBeGreaterThan(0);
  157 |             for (let i = 0; i < missingCount; i++) {
  158 |               const entry = missing.nth(i);
  159 |               const target = await entry.getAttribute("aria-controls");
  160 |               const label = await entry.innerText();
  161 |               expect(target).toBeTruthy();
  162 |               await entry.click();
  163 |               await expect(entry).toHaveAttribute("aria-current", "location");
  164 |               // READ/MEASURE only: the actual checklist click must focus a visible field in its section.
  165 |               await expect.poll(() => page.locator(`[data-testid=product-fields] [id="${target}"]`).evaluate((section) => {
  166 |                 const active = document.activeElement;
  167 |                 if (!(active instanceof HTMLElement) || !section.contains(active)) return false;
  168 |                 const field = active.getBoundingClientRect(), pane = section.closest(".pe-fields")!.getBoundingClientRect();
  169 |                 return field.height > 0 && field.top >= pane.top && field.bottom <= pane.bottom;
> 170 |               })).toBe(true);
      |                   ^ Error: expect(received).toBe(expected) // Object.is equality
  171 |               ledger.push({ page: "new", locale, width, control: `readiness/${label}`, action: "click missing item", expected: `visible field focus in ${target}`, actual: "PASS" });
  172 |             }
  173 |             await toggle.click();
  174 |             await expect(toggle).toHaveAttribute("aria-expanded", "false");
  175 |             await expect(checklist).toBeHidden();
  176 |             ledger.push({ page: "new", locale, width, control: "readiness toggle", action: "click expand, missing items, click collapse", expected: "aria-expanded false→true→false; missing list visible then hidden; entries navigate", actual: "PASS", persistence: "view state only" });
  177 |           }
  178 |           await page
  179 |             .getByTestId("product-name")
  180 |             .fill(`${tag} studio collection`);
  181 |           await page.getByTestId("product-price").fill("60");
  182 |           await page.getByTestId("product-untracked").check();
  183 |           await page.getByTestId("product-max").fill("3");
  184 |           await page
  185 |             .getByTestId("product-description")
  186 |             .fill(
  187 |               locale === "en"
  188 |                 ? "Synthetic product for isolated acceptance only."
  189 |                 : "隔離驗收用的虛構商品。",
  190 |             );
  191 |           await page.getByTestId("product-keyword").fill("P12");
  192 |           await assertProductEditorReservedLayout(
  193 |             page,
  194 |             (width === 1586 || width === 390)
  195 |               ? (section) => shot(`editor-${locale}-${width}-${section}`)
  196 |               : undefined,
  197 |           );
  198 |           await page
  199 |             .getByRole("navigation", {
  200 |               name: productEditorCopy[locale].progress,
  201 |             })
  202 |             .getByRole("button", {
  203 |               name: productEditorCopy[locale].media,
  204 |               exact: true,
  205 |             })
  206 |             .click();
  207 |           await fit();
  208 |           await page.addScriptTag({ content: axe.source });
  209 |           const accessibility = await page.evaluate(
  210 |             async () => await (window as any).axe.run(),
  211 |           );
  212 |           expect(
  213 |             accessibility.violations.filter((v: any) =>
  214 |               ["serious", "critical"].includes(v.impact),
  215 |             ),
  216 |           ).toEqual([]);
  217 |           await shot(`editor-${locale}-${width}`);
  218 |           ledger.push({
  219 |             page: "new",
  220 |             locale,
  221 |             width,
  222 |             control: "section navigation/name/price/untracked/max/keyword",
  223 |             action: "click fill check",
  224 |             expected:
  225 |               "visible draft values and section focus, no page overflow",
  226 |             actual: "PASS",
  227 |             persistence: "draft intentionally unsaved",
  228 |           });
  229 |         }
  230 |       }
  231 |       await page.setViewportSize({ width: 1586, height: 992 });
  232 |       const dirtyURL = page.url();
  233 |       let leavePrompts = 0;
  234 |       page.removeAllListeners("dialog");
  235 |       page.on("dialog", (dialog) => {
  236 |         leavePrompts++;
  237 |         void dialog.dismiss();
  238 |       });
  239 |       await page.getByTestId("nav-products").click();
  240 |       await expect.poll(() => leavePrompts).toBe(1);
  241 |       await expect(page).toHaveURL(dirtyURL);
  242 |       await expect(page.getByTestId("product-price")).toHaveValue("60");
  243 |       await page.getByTestId("product-back").click();
  244 |       await expect.poll(() => leavePrompts).toBe(2);
  245 |       await expect(page).toHaveURL(dirtyURL);
  246 |       page.removeAllListeners("dialog");
  247 |       const hardLeavePrompts: string[] = [];
  248 |       page.on("dialog", (dialog) => {
  249 |         hardLeavePrompts.push(dialog.type());
  250 |         void dialog.accept();
  251 |       });
  252 |       // Re-select the fixture store through the actual shell control; it performs a hard navigation.
  253 |       await page.getByTestId("shell-store-selector").selectOption(store);
  254 |       await page.waitForURL((value) => /^\/en\/?$/.test(value.pathname));
  255 |       expect(hardLeavePrompts).toEqual(["confirm"]);
  256 |       page.removeAllListeners("dialog");
  257 |       page.on("dialog", (dialog) => void dialog.accept());
  258 |       ledger.push({
  259 |         page: "editor",
  260 |         control: "shell and page leave guard",
  261 |         action: "click + dismiss",
  262 |         expected: "one confirmation per navigation; draft retained",
  263 |         actual: "PASS",
  264 |       });
  265 |       await page.goto(url("en", "products/new"));
  266 |       await page.goto(url("en", "collections"));
  267 |       await page.getByTestId("collection-new").click();
  268 |       await page.getByTestId("collection-title").fill(`${tag} picks`);
  269 |       await page.getByTestId("collection-save").click();
  270 |       await expect(page.getByTestId("collection-members")).toBeVisible();
```