# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: catalog-core.spec.ts >> CC12 desktop en >> catalog v2: draft product, 2x3 matrix, prices, compare-at, stock, photo, collection, activate, shopper view, archive a variant, deactivate (desktop, en)
- Location: tests/admin/catalog-core.spec.ts:110:5

# Error details

```
Error: expect(locator).toHaveCount(expected) failed

Locator:  getByTestId('axis-name-2')
Expected: 0
Received: 1
Timeout:  10000ms

Call log:
  - Expect "toHaveCount" getByTestId('axis-name-2') with timeout 10000ms
  - waiting for getByTestId('axis-name-2')
    24 × locator resolved to 1 element
       - unexpected value "1"

```

# Page snapshot

```yaml
- generic [ref=f3e1]:
  - alert [ref=f3e2]: New product
  - generic [ref=f3e3]:
    - link "Skip to content" [ref=f3e4] [cursor=pointer]:
      - /url: "#main"
    - complementary [ref=f3e5]:
      - generic [ref=f3e6]: DaWan Live
      - navigation "Workspace navigation" [ref=f3e7]:
        - button "Overview" [ref=f3e9] [cursor=pointer]
        - button "Orders & shipping" [ref=f3e14] [cursor=pointer]
        - generic [ref=f3e18]:
          - button "Products & inventory" [expanded] [ref=f3e19] [cursor=pointer]:
            - generic [aria-hidden] [ref=f3e23]: −
          - button "Products" [ref=f3e24] [cursor=pointer]
          - button "Collections" [ref=f3e25] [cursor=pointer]
          - button "Inventory" [ref=f3e26] [cursor=pointer]
        - button "Marketing" [ref=f3e28] [cursor=pointer]
        - button "Online store" [ref=f3e33] [cursor=pointer]
        - button "Payments & reports" [ref=f3e38] [cursor=pointer]
      - navigation "Settings" [ref=f3e42]:
        - button "Settings" [ref=f3e44] [cursor=pointer]
    - generic [ref=f3e48]:
      - banner [ref=f3e49]:
        - generic [ref=f3e50]:
          - generic [ref=f3e51]: Switch store
          - generic [aria-hidden] [ref=f3e52]:
            - generic [ref=f3e53]: F
            - generic [ref=f3e54]: fixture-store-a1
            - generic [ref=f3e55]: ⌄
          - combobox "Switch store" [ref=f3e56] [cursor=pointer]:
            - option "fixture-store-a1" [selected]
        - generic [ref=f3e57]:
          - generic [ref=f3e58]: Language
          - combobox "Language" [ref=f3e59] [cursor=pointer]:
            - option "简体中文"
            - option "繁體中文"
            - option "English" [selected]
        - group [ref=f3e60]:
          - generic "Help" [ref=f3e61] [cursor=pointer]
        - group [ref=f3e62]:
          - generic "Account" [ref=f3e63] [cursor=pointer]
      - main [ref=f3e64]:
        - navigation "Workspace navigation" [ref=f3e65]:
          - link "Overview" [ref=f3e66] [cursor=pointer]:
            - /url: /en/
          - generic [aria-hidden] [ref=f3e67]: /
          - generic [ref=f3e68]: Products & inventory
          - generic [aria-hidden] [ref=f3e69]: /
          - generic [ref=f3e70]: New product
        - generic [ref=f3e71]:
          - generic [ref=f3e72]:
            - heading "New product" [level=1] [ref=f3e74]
            - link "All products" [ref=f3e76] [cursor=pointer]:
              - /url: /en/products?store=d048f32f-7be4-4cea-b1ad-adbdf22e2a09
          - generic [ref=f3e77]:
            - complementary [ref=f3e78]:
              - navigation "Readiness" [ref=f3e79]:
                - button "Product images" [ref=f3e80] [cursor=pointer]
                - button "Basic information" [ref=f3e81] [cursor=pointer]
                - button "Variants" [ref=f3e82] [cursor=pointer]
                - button "Collections" [ref=f3e83] [cursor=pointer]
                - button "Shipping" [ref=f3e84] [cursor=pointer]
                - button "Search engine listing" [ref=f3e85] [cursor=pointer]
              - generic [ref=f3e86]:
                - heading "Readiness" [level=2] [ref=f3e87]
                - heading "Required" [level=3] [ref=f3e88]
                - button "To do Images" [ref=f3e89] [cursor=pointer]:
                  - generic [ref=f3e90]: To do
                  - generic [ref=f3e91]: Images
                - button "Ready Product name" [ref=f3e92] [cursor=pointer]:
                  - generic [ref=f3e93]: Ready
                  - generic [ref=f3e94]: Product name
                - button "To do Price" [ref=f3e95] [cursor=pointer]:
                  - generic [ref=f3e96]: To do
                  - generic [ref=f3e97]: Price
                - button "To do Inventory" [ref=f3e98] [cursor=pointer]:
                  - generic [ref=f3e99]: To do
                  - generic [ref=f3e100]: Inventory
                - paragraph [ref=f3e101]: "Required items remaining: 3"
                - heading "Recommended" [level=3] [ref=f3e102]
                - button "To do At least 3 images" [ref=f3e103] [cursor=pointer]:
                  - generic [ref=f3e104]: To do
                  - generic [ref=f3e105]: At least 3 images
                - button "Ready Description" [ref=f3e106] [cursor=pointer]:
                  - generic [ref=f3e107]: Ready
                  - generic [ref=f3e108]: Description
                - button "To do Collections" [ref=f3e109] [cursor=pointer]:
                  - generic [ref=f3e110]: To do
                  - generic [ref=f3e111]: Collections
                - button "To do Live keyword" [ref=f3e112] [cursor=pointer]:
                  - generic [ref=f3e113]: To do
                  - generic [ref=f3e114]: Live keyword
                - button "To do Search engine listing" [ref=f3e115] [cursor=pointer]:
                  - generic [ref=f3e116]: To do
                  - generic [ref=f3e117]: Search engine listing
            - generic [ref=f3e118]:
              - region "Product images" [ref=f3e119]:
                - region [ref=f3e120]:
                  - heading "Main images 0/4" [level=2] [ref=f3e121]:
                    - text: Main images
                    - generic [ref=f3e122]: 0/4
                  - paragraph [ref=f3e123]: Up to 4 images. The first is the cover. Square photos 800px or larger are recommended.
                  - paragraph [ref=f3e125]: No images yet.
                  - generic [ref=f3e126] [cursor=pointer]:
                    - generic [ref=f3e127]: Add images
                    - 'button "Add images: Main images" [ref=f3e128]'
                - region [ref=f3e129]:
                  - heading "Variant images" [level=2] [ref=f3e130]
                  - paragraph [ref=f3e131]: One image for each value of the image option. Without one, buyers see the cover.
                  - generic [ref=f3e132]:
                    - generic [ref=f3e133]: Image option
                    - combobox "Image option" [ref=f3e134]:
                      - option "Use the first option" [selected]
                      - option "顏色"
                      - option "尺寸"
                      - option
                  - generic [ref=f3e135]:
                    - generic [ref=f3e136]:
                      - heading "紅" [level=3] [ref=f3e137]
                      - paragraph [ref=f3e138]: Use cover image
                      - generic [ref=f3e139] [cursor=pointer]:
                        - generic [ref=f3e140]: Add image
                        - 'button "Add image: 紅" [ref=f3e141]'
                    - generic [ref=f3e142]:
                      - heading "藍" [level=3] [ref=f3e143]
                      - paragraph [ref=f3e144]: Use cover image
                      - generic [ref=f3e145] [cursor=pointer]:
                        - generic [ref=f3e146]: Add image
                        - 'button "Add image: 藍" [ref=f3e147]'
                - region [ref=f3e148]:
                  - heading "Detail images 0/20" [level=2] [ref=f3e149]:
                    - text: Detail images
                    - generic [ref=f3e150]: 0/20
                  - paragraph [ref=f3e151]: Up to 20 images. Tall images may be up to 6 times their width.
                  - paragraph [ref=f3e153]: No images yet.
                  - generic [ref=f3e154] [cursor=pointer]:
                    - generic [ref=f3e155]: Add images
                    - 'button "Add images: Detail images" [ref=f3e156]'
              - group [ref=f3e157]:
                - generic [ref=f3e158]:
                  - heading "Basic information" [level=2] [ref=f3e159]
                  - generic [ref=f3e160]:
                    - text: Product name
                    - textbox "Product name 25 / 120" [ref=f3e161]: cc30ffeab3cd8bd Linen Tee
                    - generic [ref=f3e162]: 25 / 120
                  - generic [ref=f3e163]:
                    - text: Description
                    - textbox "Description" [ref=f3e164]: Soft linen. <b>not bold</b>
                - region [ref=f3e165]:
                  - heading "Variants" [level=2] [ref=f3e166]
                  - generic [ref=f3e167]:
                    - generic [ref=f3e168]:
                      - generic [ref=f3e169]:
                        - generic [ref=f3e170]: Option name
                        - combobox "Option name" [ref=f3e171]: 顏色
                      - generic [ref=f3e172]:
                        - generic [ref=f3e173]: Values
                        - textbox "Values" [ref=f3e174]
                        - generic [ref=f3e175]: Press Enter or use commas / newlines. Up to 50 unique values.
                        - generic [ref=f3e176]:
                          - generic [ref=f3e177]:
                            - text: 紅
                            - button "Remove 紅" [ref=f3e178] [cursor=pointer]: ×
                          - generic [ref=f3e179]:
                            - text: 藍
                            - button "Remove 藍" [ref=f3e180] [cursor=pointer]: ×
                      - button "Remove Option name 1" [ref=f3e181] [cursor=pointer]: Remove
                    - generic [ref=f3e182]:
                      - generic [ref=f3e183]:
                        - generic [ref=f3e184]: Option name
                        - combobox "Option name" [ref=f3e185]: 尺寸
                      - generic [ref=f3e186]:
                        - generic [ref=f3e187]: Values
                        - textbox "Values" [ref=f3e188]
                        - generic [ref=f3e189]: Press Enter or use commas / newlines. Up to 50 unique values.
                        - generic [ref=f3e190]:
                          - generic [ref=f3e191]:
                            - text: S
                            - button "Remove S" [ref=f3e192] [cursor=pointer]: ×
                          - generic [ref=f3e193]:
                            - text: M
                            - button "Remove M" [ref=f3e194] [cursor=pointer]: ×
                          - generic [ref=f3e195]:
                            - text: L
                            - button "Remove L" [ref=f3e196] [cursor=pointer]: ×
                      - button "Remove Option name 2" [ref=f3e197] [cursor=pointer]: Remove
                    - generic [ref=f3e198]:
                      - generic [ref=f3e199]:
                        - generic [ref=f3e200]: Option name
                        - combobox "Option name" [ref=f3e201]
                      - generic [ref=f3e202]:
                        - generic [ref=f3e203]: Values
                        - textbox "Values" [ref=f3e204]
                        - generic [ref=f3e205]: Press Enter or use commas / newlines. Up to 50 unique values.
                      - button "Remove Option name 3" [active] [ref=f3e206] [cursor=pointer]: Remove
                  - button "Add option (color, size…)" [disabled] [ref=f3e207]
                  - generic [ref=f3e208]:
                    - button "Bulk fill" [ref=f3e210] [cursor=pointer]
                    - paragraph [ref=f3e211]: "6 / 100 · Selected: 0"
                  - region "Variants" [ref=f3e212]:
                    - generic [aria-hidden] [ref=f3e213]:
                      - generic [ref=f3e214]: Variants
                      - generic [ref=f3e215]: Price (USD)
                      - generic [ref=f3e216]: Compare-at price
                      - generic [ref=f3e217]: Inventory
                      - generic [ref=f3e218]: SKU code
                      - generic [ref=f3e219]: Live keyword
                      - generic [ref=f3e220]: Enabled
                    - generic [ref=f3e221]:
                      - generic "紅 / S" [ref=f3e222]:
                        - checkbox "Selected 紅 / S" [ref=f3e223]
                        - strong [ref=f3e224]: 紅 / S
                      - textbox "Price 紅 / S" [ref=f3e226]
                      - textbox "Compare-at price 1" [ref=f3e228]
                      - generic [ref=f3e229]:
                        - generic "Do not track ∞" [ref=f3e230]:
                          - checkbox "Do not track ∞" [ref=f3e231]
                          - text: Do not track ∞
                        - textbox "Quantity 1" [ref=f3e233]
                      - textbox "SKU code 1" [ref=f3e235]:
                        - /placeholder: Generated on save if blank
                      - textbox "Live keyword 1" [ref=f3e237]
                      - checkbox "Enabled 1" [checked] [ref=f3e239]
                    - generic [ref=f3e240]:
                      - generic "紅 / M" [ref=f3e241]:
                        - checkbox "Selected 紅 / M" [ref=f3e242]
                        - strong [ref=f3e243]: 紅 / M
                      - textbox "Price 紅 / M" [ref=f3e245]
                      - textbox "Compare-at price 2" [ref=f3e247]
                      - generic [ref=f3e248]:
                        - generic "Do not track ∞" [ref=f3e249]:
                          - checkbox "Do not track ∞" [ref=f3e250]
                          - text: Do not track ∞
                        - textbox "Quantity 2" [ref=f3e252]
                      - textbox "SKU code 2" [ref=f3e254]:
                        - /placeholder: Generated on save if blank
                      - textbox "Live keyword 2" [ref=f3e256]
                      - checkbox "Enabled 2" [checked] [ref=f3e258]
                    - generic [ref=f3e259]:
                      - generic "紅 / L" [ref=f3e260]:
                        - checkbox "Selected 紅 / L" [ref=f3e261]
                        - strong [ref=f3e262]: 紅 / L
                      - textbox "Price 紅 / L" [ref=f3e264]
                      - textbox "Compare-at price 3" [ref=f3e266]
                      - generic [ref=f3e267]:
                        - generic "Do not track ∞" [ref=f3e268]:
                          - checkbox "Do not track ∞" [ref=f3e269]
                          - text: Do not track ∞
                        - textbox "Quantity 3" [ref=f3e271]
                      - textbox "SKU code 3" [ref=f3e273]:
                        - /placeholder: Generated on save if blank
                      - textbox "Live keyword 3" [ref=f3e275]
                      - checkbox "Enabled 3" [checked] [ref=f3e277]
                    - generic [ref=f3e278]:
                      - generic "藍 / S" [ref=f3e279]:
                        - checkbox "Selected 藍 / S" [ref=f3e280]
                        - strong [ref=f3e281]: 藍 / S
                      - textbox "Price 藍 / S" [ref=f3e283]
                      - textbox "Compare-at price 4" [ref=f3e285]
                      - generic [ref=f3e286]:
                        - generic "Do not track ∞" [ref=f3e287]:
                          - checkbox "Do not track ∞" [ref=f3e288]
                          - text: Do not track ∞
                        - textbox "Quantity 4" [ref=f3e290]
                      - textbox "SKU code 4" [ref=f3e292]:
                        - /placeholder: Generated on save if blank
                      - textbox "Live keyword 4" [ref=f3e294]
                      - checkbox "Enabled 4" [checked] [ref=f3e296]
                    - generic [ref=f3e297]:
                      - generic "藍 / M" [ref=f3e298]:
                        - checkbox "Selected 藍 / M" [ref=f3e299]
                        - strong [ref=f3e300]: 藍 / M
                      - textbox "Price 藍 / M" [ref=f3e302]
                      - textbox "Compare-at price 5" [ref=f3e304]
                      - generic [ref=f3e305]:
                        - generic "Do not track ∞" [ref=f3e306]:
                          - checkbox "Do not track ∞" [ref=f3e307]
                          - text: Do not track ∞
                        - textbox "Quantity 5" [ref=f3e309]
                      - textbox "SKU code 5" [ref=f3e311]:
                        - /placeholder: Generated on save if blank
                      - textbox "Live keyword 5" [ref=f3e313]
                      - checkbox "Enabled 5" [checked] [ref=f3e315]
                    - generic [ref=f3e316]:
                      - generic "藍 / L" [ref=f3e317]:
                        - checkbox "Selected 藍 / L" [ref=f3e318]
                        - strong [ref=f3e319]: 藍 / L
                      - textbox "Price 藍 / L" [ref=f3e321]
                      - textbox "Compare-at price 6" [ref=f3e323]
                      - generic [ref=f3e324]:
                        - generic "Do not track ∞" [ref=f3e325]:
                          - checkbox "Do not track ∞" [ref=f3e326]
                          - text: Do not track ∞
                        - textbox "Quantity 6" [ref=f3e328]
                      - textbox "SKU code 6" [ref=f3e330]:
                        - /placeholder: Generated on save if blank
                      - textbox "Live keyword 6" [ref=f3e332]
                      - checkbox "Enabled 6" [checked] [ref=f3e334]
                - generic [ref=f3e335]:
                  - heading "Collections" [level=2] [ref=f3e336]
                  - generic [ref=f3e337]:
                    - text: Find collection
                    - searchbox "Find collection" [ref=f3e338]
                  - paragraph [ref=f3e339]: No collections yet
                  - link "Manage collections" [ref=f3e340] [cursor=pointer]:
                    - /url: /en/collections?store=d048f32f-7be4-4cea-b1ad-adbdf22e2a09
                - group [ref=f3e341]:
                  - generic "+ Shipping" [ref=f3e342] [cursor=pointer]
                  - option "Select warehouse" [selected]
                  - option "t04-9cff8fa7bbe6"
                  - option "allocation-1e0f7549ea39"
                  - option "t04-90cb6998d3d7"
                  - option "allocation-6bbafa84a386"
                - group [ref=f3e343]:
                  - generic "+ Search engine listing" [ref=f3e344] [cursor=pointer]
            - generic [ref=f3e345]:
              - generic [ref=f3e346]: Unsaved changes
              - generic [ref=f3e347]:
                - button "Save draft" [ref=f3e348] [cursor=pointer]
                - button "Publish" [ref=f3e349] [cursor=pointer]
        - paragraph [ref=f3e351]: DaWan Live is operated by Hong Kong Da Wan Trading Limited
```

# Test source

```ts
  1   | // CC12's assertions translated to the one-document UI; all mutations remain real clicks.
  2   | import { expect, type Page } from "@playwright/test";
  3   | import { productEditorCopy } from "../../apps/admin/lib/product-editor-copy";
  4   | export async function driveDocument(
  5   |   page: Page,
  6   |   L: "en" | "zh-TW",
  7   |   uniq: string,
  8   |   writes: { method: string }[],
  9   |   shop: (p: string) => Promise<any>,
  10  |   shot: (name: string) => Promise<void>,
  11  |   save: () => Promise<void>,
  12  | ) {
  13  |   const c = productEditorCopy[L];
  14  |   await page.getByTestId("product-new").click();
  15  |   await expect(page).toHaveURL(/\/products\/new/);
  16  |   await expect(page.getByTestId("product-create-form")).toBeVisible();
  17  |   await page.getByTestId("product-name").fill(`${uniq} Linen Tee`);
  18  |   await page
  19  |     .getByTestId("product-description")
  20  |     .fill("Soft linen. <b>not bold</b>");
  21  |   await shot("create");
  22  |   for (const [i, name, values] of [
  23  |     [0, "顏色", "紅, 藍"],
  24  |     [1, "尺寸", "S, M, L"],
  25  |   ] as const) {
  26  |     await page.getByTestId("axis-add").click();
  27  |     await page.getByTestId(`axis-name-${i}`).fill(name);
  28  |     await page.getByTestId(`axis-values-${i}`).fill(values);
  29  |     await page.getByTestId(`axis-values-${i}`).press("Enter");
  30  |   }
  31  |   await page.getByTestId("axis-add").click();
  32  |   await expect(page.getByTestId("axis-name-2")).toBeVisible();
  33  |   await expect(page.getByTestId("axis-add")).toBeDisabled();
  34  |   await page.locator(".product-axis").nth(2).getByRole("button").last().click();
> 35  |   await expect(page.getByTestId("axis-name-2")).toHaveCount(0);
      |                                                 ^ Error: expect(locator).toHaveCount(expected) failed
  36  |   const rows = page.locator('[data-testid^="matrix-row-"]');
  37  |   await expect(rows).toHaveCount(6);
  38  |   const titles = await rows.locator("strong").allTextContents();
  39  |   expect([...titles].sort(), "2x3 matrix").toEqual(
  40  |     ["紅 / S", "紅 / M", "紅 / L", "藍 / S", "藍 / M", "藍 / L"].sort(),
  41  |   );
  42  |   await page.getByTestId("product-create").click();
  43  |   await expect(page.getByTestId("product-message")).toBeVisible();
  44  |   const whole = (await page.locator(".pe-matrix-head").innerText()).includes(
  45  |     "NT$",
  46  |   );
  47  |   const shownPrice = (minor: number) =>
  48  |     whole ? String(minor / 100) : (minor / 100).toFixed(2);
  49  |   for (let i = 0; i < 6; i++) {
  50  |     await page.getByTestId(`new-price-${i}`).fill(String(10 + 2 * i));
  51  |     await page.getByTestId(`matrix-quantity-${i}`).fill("0");
  52  |   }
  53  |   if (whole) {
  54  |     await page.getByTestId("new-price-0").fill("10.5");
  55  |     await page.getByTestId("product-create").click();
  56  |     await expect(page.getByTestId("product-message")).toContainText(
  57  |       c.invalid_price,
  58  |     );
  59  |     await page.getByTestId("new-price-0").fill("10");
  60  |   }
  61  |   await shot("matrix");
  62  |   await page.getByTestId("product-create").click();
  63  |   await expect(page.getByTestId("product-save-result")).toBeVisible();
  64  |   await page
  65  |     .getByTestId("product-save-result")
  66  |     .getByRole("link", { name: c.save, exact: true })
  67  |     .click();
  68  |   await expect(page.getByTestId("product-form")).toBeVisible();
  69  |   const productId = /\/products\/([0-9a-f-]{36})/.exec(page.url())![1];
  70  |   await expect(page.getByTestId("product-status")).toHaveValue("draft");
  71  |   await expect(page.getByTestId("product-slug")).toHaveValue(
  72  |     `${uniq}-linen-tee`,
  73  |   );
  74  |   const before = writes.filter((w) => w.method === "PUT").length;
  75  |   await page.getByTestId("product-save").click();
  76  |   await expect(page.getByTestId("product-message")).toContainText(c.noChanges);
  77  |   expect(
  78  |     writes.filter((w) => w.method === "PUT"),
  79  |     "no-op sends no command",
  80  |   ).toHaveLength(before);
  81  |   await expect(page.locator(".product-editor b"), "no raw HTML").toHaveCount(0);
  82  |   await page.locator("#seo summary").click();
  83  |   await page.getByTestId("product-seo-title").fill("Linen tee, soft");
  84  |   await page
  85  |     .getByTestId("product-seo-description")
  86  |     .fill("A soft linen tee for summer.");
  87  |   await expect(
  88  |     page.locator(".product-seo .product-count").first(),
  89  |   ).toContainText("15");
  90  |   await expect(
  91  |     page.locator(".product-seo .product-count").first(),
  92  |   ).toContainText("70");
  93  |   await expect(
  94  |     page.locator(".product-seo .product-count").nth(1),
  95  |   ).toContainText("28");
  96  |   await expect(
  97  |     page.locator(".product-seo .product-count").nth(1),
  98  |   ).toContainText("160");
  99  |   await save();
  100 |   await expect(page.getByTestId("product-seo-title")).toHaveValue(
  101 |     "Linen tee, soft",
  102 |   );
  103 |   expect(
  104 |     (await shop(`/v1/buyer/catalog/v2/products?q=${uniq}`)).body.products,
  105 |     "draft hidden",
  106 |   ).toHaveLength(0);
  107 |   expect(
  108 |     (await shop(`/v1/buyer/catalog/v2/products/${productId}`)).status,
  109 |   ).toBe(404);
  110 |   const idByTitle: Record<string, string> = {},
  111 |     ids: string[] = [];
  112 |   await expect(rows).toHaveCount(6);
  113 |   for (let i = 0; i < 6; i++) {
  114 |     const id = (await rows.nth(i).getAttribute("data-sku-id"))!;
  115 |     ids.push(id);
  116 |     idByTitle[(await rows.nth(i).locator("strong").innerText()).trim()] = id;
  117 |   }
  118 |   const rowFor = (id: string) => page.locator(`[data-sku-id="${id}"]`),
  119 |     priceOf = (title: string) => 1000 + 200 * titles.indexOf(title);
  120 |   for (const t of titles)
  121 |     await expect(
  122 |       rowFor(idByTitle[t]).locator('[data-testid^="new-price-"]'),
  123 |     ).toHaveValue(shownPrice(priceOf(t)));
  124 |   expect(
  125 |     (await shop(`/v1/buyer/catalog/v2/products?q=${uniq}`)).body.products,
  126 |     "still draft",
  127 |   ).toHaveLength(0);
  128 |   const cheapest = idByTitle[titles[0]];
  129 |   await rowFor(cheapest).locator('[data-testid^="matrix-compare-"]').fill("5");
  130 |   await page.getByTestId("product-save").click();
  131 |   await expect(page.getByTestId("product-message")).toBeVisible();
  132 |   await rowFor(cheapest).locator('[data-testid^="matrix-compare-"]').fill("25");
  133 |   await save();
  134 |   await expect(
  135 |     rowFor(cheapest).locator('[data-testid^="matrix-compare-"]'),
```
