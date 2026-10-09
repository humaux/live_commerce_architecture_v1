# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: catalog-core.spec.ts >> PE12-17 document workflow, matrix, list actions and click ledger
- Location: tests/admin/product-editor.acceptance.ts:16:3

# Error details

```
Error: expect(locator).toHaveCount(expected) failed

Locator:  getByTestId('photo-row')
Expected: 1
Received: 0
Timeout:  10000ms

Call log:
  - Expect "toHaveCount" getByTestId('photo-row') with timeout 10000ms
  - waiting for getByTestId('photo-row')
    24 × locator resolved to 0 elements
       - unexpected value "0"

```

# Page snapshot

```yaml
- generic [active] [ref=f20e1]:
  - generic [ref=f20e2]:
    - link "Skip to content" [ref=f20e3] [cursor=pointer]:
      - /url: "#main"
    - complementary [ref=f20e4]:
      - generic [ref=f20e5]: DaWan Live
      - navigation "Workspace navigation" [ref=f20e6]:
        - button "Overview" [ref=f20e8] [cursor=pointer]
        - button "Orders & shipping" [ref=f20e13] [cursor=pointer]
        - generic [ref=f20e17]:
          - button "Products & inventory" [expanded] [ref=f20e18] [cursor=pointer]:
            - generic [aria-hidden] [ref=f20e22]: −
          - button "Products" [ref=f20e23] [cursor=pointer]
          - button "Collections" [ref=f20e24] [cursor=pointer]
          - button "Inventory" [ref=f20e25] [cursor=pointer]
        - button "Payments & reports" [ref=f20e27] [cursor=pointer]
    - generic [ref=f20e31]:
      - banner [ref=f20e32]:
        - generic [ref=f20e33]:
          - generic [ref=f20e34]: Switch store
          - generic [aria-hidden] [ref=f20e35]:
            - generic [ref=f20e36]: F
            - generic [ref=f20e37]: fixture-store-b
            - generic [ref=f20e38]: ⌄
          - combobox "Switch store" [ref=f20e39] [cursor=pointer]:
            - option "fixture-store-b" [selected]
        - generic [ref=f20e40]:
          - generic [ref=f20e41]: Language
          - combobox "Language" [ref=f20e42] [cursor=pointer]:
            - option "简体中文"
            - option "繁體中文"
            - option "English" [selected]
        - group [ref=f20e43]:
          - generic "Help" [ref=f20e44] [cursor=pointer]
        - group [ref=f20e45]:
          - generic "Account" [ref=f20e46] [cursor=pointer]
      - main [ref=f20e47]:
        - navigation "Workspace navigation" [ref=f20e48]:
          - link "Overview" [ref=f20e49] [cursor=pointer]:
            - /url: /en/
          - generic [aria-hidden] [ref=f20e50]: /
          - generic [ref=f20e51]: Products & inventory
          - generic [aria-hidden] [ref=f20e52]: /
          - generic [ref=f20e53]: Edit product
        - generic [ref=f20e54]:
          - generic [ref=f20e55]:
            - generic [ref=f20e56]:
              - heading "Edit product" [level=1] [ref=f20e57]
              - paragraph [ref=f20e58]: pecc120ac325bb34 matrix
            - link "All products" [ref=f20e60] [cursor=pointer]:
              - /url: /en/products?store=68e87c0d-a43a-4488-99e2-fadc1b6ffd99
          - generic [ref=f20e61]:
            - complementary [ref=f20e62]:
              - navigation "Readiness" [ref=f20e63]:
                - button "Product images" [ref=f20e64] [cursor=pointer]
                - button "Basic information" [ref=f20e65] [cursor=pointer]
                - button "Variants" [ref=f20e66] [cursor=pointer]
                - button "Collections" [ref=f20e67] [cursor=pointer]
                - button "Shipping" [ref=f20e68] [cursor=pointer]
                - button "Search engine listing" [ref=f20e69] [cursor=pointer]
              - generic [ref=f20e70]:
                - heading "Readiness" [level=2] [ref=f20e71]
                - heading "Required" [level=3] [ref=f20e72]
                - button "To do Images" [ref=f20e73] [cursor=pointer]:
                  - generic [ref=f20e74]: To do
                  - generic [ref=f20e75]: Images
                - button "Ready Product name" [ref=f20e76] [cursor=pointer]:
                  - generic [ref=f20e77]: Ready
                  - generic [ref=f20e78]: Product name
                - button "Ready Price" [ref=f20e79] [cursor=pointer]:
                  - generic [ref=f20e80]: Ready
                  - generic [ref=f20e81]: Price
                - button "Ready Inventory" [ref=f20e82] [cursor=pointer]:
                  - generic [ref=f20e83]: Ready
                  - generic [ref=f20e84]: Inventory
                - paragraph [ref=f20e85]: "Required items remaining: 1"
                - heading "Recommended" [level=3] [ref=f20e86]
                - button "To do At least 3 images" [ref=f20e87] [cursor=pointer]:
                  - generic [ref=f20e88]: To do
                  - generic [ref=f20e89]: At least 3 images
                - button "To do Description" [ref=f20e90] [cursor=pointer]:
                  - generic [ref=f20e91]: To do
                  - generic [ref=f20e92]: Description
                - button "To do Collections" [ref=f20e93] [cursor=pointer]:
                  - generic [ref=f20e94]: To do
                  - generic [ref=f20e95]: Collections
                - button "To do Live keyword" [ref=f20e96] [cursor=pointer]:
                  - generic [ref=f20e97]: To do
                  - generic [ref=f20e98]: Live keyword
                - button "To do Search engine listing" [ref=f20e99] [cursor=pointer]:
                  - generic [ref=f20e100]: To do
                  - generic [ref=f20e101]: Search engine listing
            - generic [ref=f20e102]:
              - generic [ref=f20e104]:
                - region [ref=f20e105]:
                  - heading "Main images 0/4" [level=2] [ref=f20e106]:
                    - text: Main images
                    - generic [ref=f20e107]: 0/4
                  - paragraph [ref=f20e108]: Up to 4 images. The first is the cover. Square photos 800px or larger are recommended.
                  - paragraph [ref=f20e110]: No images yet.
                  - generic [ref=f20e111] [cursor=pointer]:
                    - generic [ref=f20e112]: Add images
                    - 'button "Add images: Main images" [ref=f20e113]'
                - region [ref=f20e114]:
                  - heading "Variant images" [level=2] [ref=f20e115]
                  - paragraph [ref=f20e116]: One image for each value of the image option. Without one, buyers see the cover.
                  - generic [ref=f20e117]:
                    - generic [ref=f20e118]: Image option
                    - combobox "Image option" [ref=f20e119]:
                      - option "Use the first option"
                      - option "Color" [selected]
                      - option "Size"
                  - generic [ref=f20e120]:
                    - generic [ref=f20e121]:
                      - heading "White" [level=3] [ref=f20e122]
                      - paragraph [ref=f20e123]: Use cover image
                      - generic [ref=f20e124] [cursor=pointer]:
                        - generic [ref=f20e125]: Add image
                        - 'button "Add image: White" [ref=f20e126]'
                    - generic [ref=f20e127]:
                      - heading "Black" [level=3] [ref=f20e128]
                      - paragraph [ref=f20e129]: Use cover image
                      - generic [ref=f20e130] [cursor=pointer]:
                        - generic [ref=f20e131]: Add image
                        - 'button "Add image: Black" [ref=f20e132]'
                    - generic [ref=f20e133]:
                      - heading "Blue" [level=3] [ref=f20e134]
                      - paragraph [ref=f20e135]: Use cover image
                      - generic [ref=f20e136] [cursor=pointer]:
                        - generic [ref=f20e137]: Add image
                        - 'button "Add image: Blue" [ref=f20e138]'
                - region [ref=f20e139]:
                  - heading "Detail images 0/20" [level=2] [ref=f20e140]:
                    - text: Detail images
                    - generic [ref=f20e141]: 0/20
                  - paragraph [ref=f20e142]: Up to 20 images. Tall images may be up to 6 times their width.
                  - paragraph [ref=f20e144]: No images yet.
                  - generic [ref=f20e145] [cursor=pointer]:
                    - generic [ref=f20e146]: Add images
                    - 'button "Add images: Detail images" [ref=f20e147]'
              - group [ref=f20e148]:
                - generic [ref=f20e149]:
                  - heading "Basic information" [level=2] [ref=f20e150]
                  - generic [ref=f20e151]:
                    - text: Visibility
                    - 'combobox "Visibility Draft: only you can see it. Shoppers cannot find or buy it." [ref=f20e152]':
                      - option "Draft" [selected]
                      - option "Active"
                    - generic [ref=f20e153]: "Draft: only you can see it. Shoppers cannot find or buy it."
                  - generic [ref=f20e154]:
                    - text: Product name
                    - textbox "Product name 23 / 120" [ref=f20e155]: pecc120ac325bb34 matrix
                    - generic [ref=f20e156]: 23 / 120
                  - generic [ref=f20e157]:
                    - text: Description
                    - textbox "Description" [ref=f20e158]
                - region [ref=f20e159]:
                  - heading "Variants" [level=2] [ref=f20e160]
                  - generic [ref=f20e161]:
                    - generic [ref=f20e162]:
                      - generic [ref=f20e163]:
                        - generic [ref=f20e164]: Option name
                        - combobox "Option name" [ref=f20e165]: Color
                      - generic [ref=f20e166]:
                        - generic [ref=f20e167]: Values
                        - textbox "Values" [ref=f20e168]
                        - generic [ref=f20e169]: Press Enter or use commas / newlines. Up to 50 unique values.
                        - generic [ref=f20e170]:
                          - generic [ref=f20e171]:
                            - text: White
                            - button "Remove White" [ref=f20e172] [cursor=pointer]: ×
                          - generic [ref=f20e173]:
                            - text: Black
                            - button "Remove Black" [ref=f20e174] [cursor=pointer]: ×
                          - generic [ref=f20e175]:
                            - text: Blue
                            - button "Remove Blue" [ref=f20e176] [cursor=pointer]: ×
                      - button "Remove Option name 1" [ref=f20e177] [cursor=pointer]: Remove
                    - generic [ref=f20e178]:
                      - generic [ref=f20e179]:
                        - generic [ref=f20e180]: Option name
                        - combobox "Option name" [ref=f20e181]: Size
                      - generic [ref=f20e182]:
                        - generic [ref=f20e183]: Values
                        - textbox "Values" [ref=f20e184]
                        - generic [ref=f20e185]: Press Enter or use commas / newlines. Up to 50 unique values.
                        - generic [ref=f20e186]:
                          - generic [ref=f20e187]:
                            - text: S
                            - button "Remove S" [ref=f20e188] [cursor=pointer]: ×
                          - generic [ref=f20e189]:
                            - text: M
                            - button "Remove M" [ref=f20e190] [cursor=pointer]: ×
                          - generic [ref=f20e191]:
                            - text: L
                            - button "Remove L" [ref=f20e192] [cursor=pointer]: ×
                          - generic [ref=f20e193]:
                            - text: XL
                            - button "Remove XL" [ref=f20e194] [cursor=pointer]: ×
                      - button "Remove Option name 2" [ref=f20e195] [cursor=pointer]: Remove
                  - button "Add option (color, size…)" [ref=f20e196] [cursor=pointer]
                  - generic [ref=f20e197]:
                    - button "Bulk fill" [ref=f20e199] [cursor=pointer]
                    - paragraph [ref=f20e200]: "12 / 100 · Selected: 0"
                  - region "Variants" [ref=f20e201]:
                    - generic [aria-hidden] [ref=f20e202]:
                      - generic [ref=f20e203]: Variants
                      - generic [ref=f20e204]: Price (NT$)
                      - generic [ref=f20e205]: Compare-at price
                      - generic [ref=f20e206]: Inventory
                      - generic [ref=f20e207]: SKU code
                      - generic [ref=f20e208]: Live keyword
                      - generic [ref=f20e209]: Enabled
                    - generic [ref=f20e210]:
                      - generic "White / S" [ref=f20e211]:
                        - checkbox "Selected White / S" [ref=f20e212]
                        - strong [ref=f20e213]: White / S
                      - textbox "Price White / S" [ref=f20e215]: "60"
                      - textbox "Compare-at price 1" [ref=f20e217]
                      - generic [ref=f20e218]:
                        - generic "Do not track ∞" [ref=f20e219]:
                          - checkbox "Do not track ∞" [ref=f20e220]
                          - text: Do not track ∞
                        - textbox "Resulting on-hand 1" [ref=f20e222]: "5"
                      - textbox "SKU code 1" [disabled] [ref=f20e224]:
                        - /placeholder: Generated on save if blank
                        - text: pecc120ac325bb34-matrix
                      - textbox "Live keyword 1" [ref=f20e226]
                      - checkbox "Enabled 1" [checked] [ref=f20e228]
                    - generic [ref=f20e229]:
                      - generic "White / M" [ref=f20e230]:
                        - checkbox "Selected White / M" [ref=f20e231]
                        - strong [ref=f20e232]: White / M
                      - textbox "Price White / M" [ref=f20e234]: "80"
                      - textbox "Compare-at price 2" [ref=f20e236]
                      - generic [ref=f20e237]:
                        - generic "Do not track ∞" [ref=f20e238]:
                          - checkbox "Do not track ∞" [ref=f20e239]
                          - text: Do not track ∞
                        - textbox "Resulting on-hand 2" [ref=f20e241]: "5"
                      - textbox "SKU code 2" [disabled] [ref=f20e243]:
                        - /placeholder: Generated on save if blank
                        - text: pecc120ac325bb34-matrix-2
                      - textbox "Live keyword 2" [ref=f20e245]
                      - checkbox "Enabled 2" [checked] [ref=f20e247]
                    - generic [ref=f20e248]:
                      - generic "White / L" [ref=f20e249]:
                        - checkbox "Selected White / L" [ref=f20e250]
                        - strong [ref=f20e251]: White / L
                      - textbox "Price White / L" [ref=f20e253]: "80"
                      - textbox "Compare-at price 3" [ref=f20e255]
                      - generic [ref=f20e256]:
                        - generic "Do not track ∞" [ref=f20e257]:
                          - checkbox "Do not track ∞" [ref=f20e258]
                          - text: Do not track ∞
                        - textbox "Resulting on-hand 3" [ref=f20e260]: "5"
                      - textbox "SKU code 3" [disabled] [ref=f20e262]:
                        - /placeholder: Generated on save if blank
                        - text: pecc120ac325bb34-matrix-3
                      - textbox "Live keyword 3" [ref=f20e264]
                      - checkbox "Enabled 3" [checked] [ref=f20e266]
                    - generic [ref=f20e267]:
                      - generic "White / XL" [ref=f20e268]:
                        - checkbox "Selected White / XL" [ref=f20e269]
                        - strong [ref=f20e270]: White / XL
                      - textbox "Price White / XL" [ref=f20e272]: "80"
                      - textbox "Compare-at price 4" [ref=f20e274]
                      - generic [ref=f20e275]:
                        - generic "Do not track ∞" [ref=f20e276]:
                          - checkbox "Do not track ∞" [ref=f20e277]
                          - text: Do not track ∞
                        - textbox "Resulting on-hand 4" [ref=f20e279]: "5"
                      - textbox "SKU code 4" [disabled] [ref=f20e281]:
                        - /placeholder: Generated on save if blank
                        - text: pecc120ac325bb34-matrix-4
                      - textbox "Live keyword 4" [ref=f20e283]
                      - checkbox "Enabled 4" [checked] [ref=f20e285]
                    - generic [ref=f20e286]:
                      - generic "Black / S" [ref=f20e287]:
                        - checkbox "Selected Black / S" [ref=f20e288]
                        - strong [ref=f20e289]: Black / S
                      - textbox "Price Black / S" [ref=f20e291]: "80"
                      - textbox "Compare-at price 5" [ref=f20e293]
                      - generic [ref=f20e294]:
                        - generic "Do not track ∞" [ref=f20e295]:
                          - checkbox "Do not track ∞" [ref=f20e296]
                          - text: Do not track ∞
                        - textbox "Resulting on-hand 5" [ref=f20e298]: "5"
                      - textbox "SKU code 5" [disabled] [ref=f20e300]:
                        - /placeholder: Generated on save if blank
                        - text: pecc120ac325bb34-matrix-5
                      - textbox "Live keyword 5" [ref=f20e302]
                      - checkbox "Enabled 5" [checked] [ref=f20e304]
                    - generic [ref=f20e305]:
                      - generic "Black / M" [ref=f20e306]:
                        - checkbox "Selected Black / M" [ref=f20e307]
                        - strong [ref=f20e308]: Black / M
                      - textbox "Price Black / M" [ref=f20e310]: "80"
                      - textbox "Compare-at price 6" [ref=f20e312]
                      - generic [ref=f20e313]:
                        - generic "Do not track ∞" [ref=f20e314]:
                          - checkbox "Do not track ∞" [ref=f20e315]
                          - text: Do not track ∞
                        - textbox "Resulting on-hand 6" [ref=f20e317]: "5"
                      - textbox "SKU code 6" [disabled] [ref=f20e319]:
                        - /placeholder: Generated on save if blank
                        - text: pecc120ac325bb34-matrix-6
                      - textbox "Live keyword 6" [ref=f20e321]
                      - checkbox "Enabled 6" [checked] [ref=f20e323]
                    - generic [ref=f20e324]:
                      - generic "Black / L" [ref=f20e325]:
                        - checkbox "Selected Black / L" [ref=f20e326]
                        - strong [ref=f20e327]: Black / L
                      - textbox "Price Black / L" [ref=f20e329]: "80"
                      - textbox "Compare-at price 7" [ref=f20e331]
                      - generic [ref=f20e332]:
                        - generic "Do not track ∞" [ref=f20e333]:
                          - checkbox "Do not track ∞" [ref=f20e334]
                          - text: Do not track ∞
                        - textbox "Resulting on-hand 7" [ref=f20e336]: "5"
                      - textbox "SKU code 7" [disabled] [ref=f20e338]:
                        - /placeholder: Generated on save if blank
                        - text: pecc120ac325bb34-matrix-7
                      - textbox "Live keyword 7" [ref=f20e340]
                      - checkbox "Enabled 7" [checked] [ref=f20e342]
                    - generic [ref=f20e343]:
                      - generic "Black / XL" [ref=f20e344]:
                        - checkbox "Selected Black / XL" [ref=f20e345]
                        - strong [ref=f20e346]: Black / XL
                      - textbox "Price Black / XL" [ref=f20e348]: "80"
                      - textbox "Compare-at price 8" [ref=f20e350]
                      - generic [ref=f20e351]:
                        - generic "Do not track ∞" [ref=f20e352]:
                          - checkbox "Do not track ∞" [ref=f20e353]
                          - text: Do not track ∞
                        - textbox "Resulting on-hand 8" [ref=f20e355]: "5"
                      - textbox "SKU code 8" [disabled] [ref=f20e357]:
                        - /placeholder: Generated on save if blank
                        - text: pecc120ac325bb34-matrix-8
                      - textbox "Live keyword 8" [ref=f20e359]
                      - checkbox "Enabled 8" [checked] [ref=f20e361]
                    - generic [ref=f20e362]:
                      - generic "Blue / S" [ref=f20e363]:
                        - checkbox "Selected Blue / S" [ref=f20e364]
                        - strong [ref=f20e365]: Blue / S
                      - textbox "Price Blue / S" [ref=f20e367]: "80"
                      - textbox "Compare-at price 9" [ref=f20e369]
                      - generic [ref=f20e370]:
                        - generic "Do not track ∞" [ref=f20e371]:
                          - checkbox "Do not track ∞" [ref=f20e372]
                          - text: Do not track ∞
                        - textbox "Resulting on-hand 9" [ref=f20e374]: "5"
                      - textbox "SKU code 9" [disabled] [ref=f20e376]:
                        - /placeholder: Generated on save if blank
                        - text: pecc120ac325bb34-matrix-9
                      - textbox "Live keyword 9" [ref=f20e378]
                      - checkbox "Enabled 9" [checked] [ref=f20e380]
                    - generic [ref=f20e381]:
                      - generic "Blue / M" [ref=f20e382]:
                        - checkbox "Selected Blue / M" [ref=f20e383]
                        - strong [ref=f20e384]: Blue / M
                      - textbox "Price Blue / M" [ref=f20e386]: "80"
                      - textbox "Compare-at price 10" [ref=f20e388]
                      - generic [ref=f20e389]:
                        - generic "Do not track ∞" [ref=f20e390]:
                          - checkbox "Do not track ∞" [ref=f20e391]
                          - text: Do not track ∞
                        - textbox "Resulting on-hand 10" [ref=f20e393]: "5"
                      - textbox "SKU code 10" [disabled] [ref=f20e395]:
                        - /placeholder: Generated on save if blank
                        - text: pecc120ac325bb34-matrix-10
                      - textbox "Live keyword 10" [ref=f20e397]
                      - checkbox "Enabled 10" [checked] [ref=f20e399]
                    - generic [ref=f20e400]:
                      - generic "Blue / L" [ref=f20e401]:
                        - checkbox "Selected Blue / L" [ref=f20e402]
                        - strong [ref=f20e403]: Blue / L
                      - textbox "Price Blue / L" [ref=f20e405]: "80"
                      - textbox "Compare-at price 11" [ref=f20e407]
                      - generic [ref=f20e408]:
                        - generic "Do not track ∞" [ref=f20e409]:
                          - checkbox "Do not track ∞" [ref=f20e410]
                          - text: Do not track ∞
                        - textbox "Resulting on-hand 11" [ref=f20e412]: "5"
                      - textbox "SKU code 11" [disabled] [ref=f20e414]:
                        - /placeholder: Generated on save if blank
                        - text: pecc120ac325bb34-matrix-11
                      - textbox "Live keyword 11" [ref=f20e416]
                      - checkbox "Enabled 11" [checked] [ref=f20e418]
                    - generic [ref=f20e419]:
                      - generic "Blue / XL" [ref=f20e420]:
                        - checkbox "Selected Blue / XL" [ref=f20e421]
                        - strong [ref=f20e422]: Blue / XL
                      - textbox "Price Blue / XL" [ref=f20e424]: "80"
                      - textbox "Compare-at price 12" [ref=f20e426]
                      - generic [ref=f20e427]:
                        - generic "Do not track ∞" [ref=f20e428]:
                          - checkbox "Do not track ∞" [ref=f20e429]
                          - text: Do not track ∞
                        - textbox "Resulting on-hand 12" [ref=f20e431]: "5"
                      - textbox "SKU code 12" [disabled] [ref=f20e433]:
                        - /placeholder: Generated on save if blank
                        - text: pecc120ac325bb34-matrix-12
                      - textbox "Live keyword 12" [ref=f20e435]
                      - checkbox "Enabled 12" [checked] [ref=f20e437]
                - generic [ref=f20e438]:
                  - heading "Collections" [level=2] [ref=f20e439]
                  - generic [ref=f20e440]:
                    - text: Find collection
                    - searchbox "Find collection" [ref=f20e441]
                  - generic [ref=f20e443]:
                    - checkbox "pecc120ac325bb34 picks" [ref=f20e444]
                    - text: pecc120ac325bb34 picks
                  - link "Manage collections" [ref=f20e445] [cursor=pointer]:
                    - /url: /en/collections?store=68e87c0d-a43a-4488-99e2-fadc1b6ffd99
                - group [ref=f20e446]:
                  - generic "+ Shipping" [ref=f20e447] [cursor=pointer]
                - group [ref=f20e448]:
                  - generic "+ Search engine listing" [ref=f20e449] [cursor=pointer]
            - generic [ref=f20e451]:
              - button "Save changes" [ref=f20e452] [cursor=pointer]
              - button "Publish" [ref=f20e453] [cursor=pointer]
        - paragraph [ref=f20e455]: DaWan Live is operated by Hong Kong Da Wan Trading Limited
  - alert [ref=f20e456]
```

# Test source

```ts
  308 |       ).toHaveLength(3);
  309 |       expect(
  310 |         createdWrites.filter((r) => r.path.endsWith("/images/order")),
  311 |       ).toHaveLength(1);
  312 |       for (const write of createdWrites)
  313 |         expect(write.key).toMatch(/^[0-9a-f-]{36}$/);
  314 |       const productLink = page
  315 |         .getByTestId("product-save-result")
  316 |         .getByRole("link", { name: "Save changes" });
  317 |       const href = await productLink.getAttribute("href");
  318 |       const id = /products\/([0-9a-f-]{36})/.exec(href!)![1];
  319 |       await productLink.click();
  320 |       await expect(page).toHaveURL(url("en", "products/" + id));
  321 |       await page.reload();
  322 |       await expect(page.getByTestId("product-price")).toHaveValue("60");
  323 |       await expect(page.getByTestId("product-untracked")).toBeChecked();
  324 |       await expect(page.getByTestId("product-max")).toHaveValue("3");
  325 |       await expect(page.getByTestId("photo-row")).toHaveCount(3);
  326 |       const buyer = await fetch(
  327 |         `${process.env.LC_BROWSER_CONTROL}/?path=${encodeURIComponent(`/v1/buyer/catalog/v2/products?q=${tag}`)}`,
  328 |         { headers: { "X-Gate-Key": process.env.LC_BROWSER_CONTROL_KEY! } },
  329 |       );
  330 |       expect(buyer.status).toBe(200);
  331 |       const readback = await buyer.json();
  332 |       expect(readback.status).toBe(200);
  333 |       expect(readback.body).toContain(id);
  334 |       ledger.push({
  335 |         page: "create",
  336 |         control: "3 file inputs/reorder/publish double click",
  337 |         action: "setInputFiles click dblclick reload",
  338 |         expected:
  339 |           "one document,3 uploads,one order; persisted NT$60 and untracked cap3",
  340 |         actual: "PASS",
  341 |         tier: "BROWSER+REAL_PG; buyer HTTP readback",
  342 |       });
  343 |       await page.goto(url("en", "products/new"));
  344 |       await page.getByTestId("product-name").fill(`${tag} matrix`);
  345 |       const networkBefore = writes.length;
  346 |       await page.getByTestId("axis-add").click();
  347 |       await page.getByTestId("axis-name-0").fill("Color");
  348 |       await page.getByTestId("axis-values-0").fill("White, Black, Blue");
  349 |       await page.getByTestId("axis-values-0").press("Enter");
  350 |       await page.getByTestId("axis-add").click();
  351 |       await page.getByTestId("axis-name-1").fill("Size");
  352 |       await page.getByTestId("axis-values-1").fill("S, M, L, XL");
  353 |       await page.getByTestId("axis-values-1").press("Enter");
  354 |       await expect(page.locator('[data-testid^="matrix-row-"]')).toHaveCount(
  355 |         12,
  356 |       );
  357 |       await assertProductEditorReservedLayout(page);
  358 |       expect(writes.length).toBe(networkBefore);
  359 |       await page.getByTestId("bulk-open").click();
  360 |       await page.getByTestId("bulk-value").fill("80");
  361 |       await page.getByTestId("bulk-value").press("Enter");
  362 |       for (let i = 0; i < 12; i++)
  363 |         await expect(page.getByTestId(`new-price-${i}`)).toHaveValue("80");
  364 |       await page.getByTestId("new-price-0").fill("60");
  365 |       await page.getByTestId("new-price-1").fill("");
  366 |       await page.getByTestId("bulk-open").click();
  367 |       await page
  368 |         .getByRole("region", { name: "Bulk fill Price" })
  369 |         .getByRole("combobox", { name: "Bulk fill", exact: true })
  370 |         .selectOption("empty");
  371 |       await page.getByTestId("bulk-value").fill("80");
  372 |       await page.getByTestId("bulk-value").press("Enter");
  373 |       await expect(page.getByTestId("new-price-0")).toHaveValue("60");
  374 |       await expect(page.getByTestId("new-price-1")).toHaveValue("80");
  375 |       await expect(page.getByTestId("new-price-11")).toHaveValue("80");
  376 |       await page.getByTestId("bulk-open").click();
  377 |       await page.getByTestId("bulk-value").press("Escape");
  378 |       await expect(page.getByTestId("bulk-open")).toBeFocused();
  379 |       await page.getByTestId("bulk-open").click();
  380 |       await page.getByTestId("bulk-field").selectOption("quantity");
  381 |       await page.getByTestId("bulk-value").fill("5");
  382 |       await page.getByTestId("bulk-apply").click();
  383 |       await page.getByText("Shipping", { exact: true }).last().click();
  384 |       const stockWarehouse = page.getByRole("combobox", {
  385 |         name: "Stock warehouse",
  386 |       });
  387 |       await expect(stockWarehouse).toHaveCount(0); // dedicated TWD fixture has exactly one warehouse
  388 |       await shot("matrix-en-1586");
  389 |       await page.getByTestId("product-create").click();
  390 |       await expect(page.getByTestId("product-save-result")).toBeVisible({
  391 |         timeout: 30000,
  392 |       });
  393 |       const matrixLink = page
  394 |         .getByTestId("product-save-result")
  395 |         .getByRole("link", { name: "Save changes" });
  396 |       const matrixHref = await matrixLink.getAttribute("href");
  397 |       await matrixLink.click();
  398 |       await expect(page).toHaveURL(origin + matrixHref);
  399 |       await page.reload();
  400 |       await expect(page.locator('[data-testid^="matrix-row-"]')).toHaveCount(
  401 |         12,
  402 |       );
  403 |       await page.getByTestId("photo-input").setInputFiles({
  404 |         name: "matrix.png",
  405 |         mimeType: "image/png",
  406 |         buffer: png,
  407 |       });
> 408 |       await expect(page.getByTestId("photo-row")).toHaveCount(1);
      |                                                   ^ Error: expect(locator).toHaveCount(expected) failed
  409 |       await page.getByTestId("product-publish").click();
  410 |       await expect(page.getByTestId("product-save")).toBeEnabled();
  411 |       await page.reload();
  412 |       await expect(page.getByTestId("product-status")).toHaveValue("active");
  413 |       await page.getByTestId("matrix-active-0").uncheck();
  414 |       page.removeAllListeners("dialog");
  415 |       const confirmation = page.waitForEvent("dialog");
  416 |       const archiveStart = writes.length;
  417 |       const unpublishClick = page.getByTestId("product-unpublish").click();
  418 |       await (await confirmation).dismiss();
  419 |       await unpublishClick;
  420 |       await expect(page.getByTestId("product-status")).toHaveValue("active");
  421 |       expect(writes).toHaveLength(archiveStart);
  422 |       await page.getByTestId("matrix-active-0").check();
  423 |       page.on("dialog", (dialog) => void dialog.accept());
  424 |       await page.getByTestId("product-unpublish").click();
  425 |       await expect(page.getByTestId("product-status")).toHaveValue("draft");
  426 |       await expect(page.getByTestId("new-price-11")).toHaveValue("80");
  427 |       ledger.push({
  428 |         page: "matrix",
  429 |         control: "axes/bulk only empty/stock/save",
  430 |         action: "click fill Enter reload",
  431 |         expected: "12 local rows, preserved first price60, others80; persisted",
  432 |         actual: "PASS",
  433 |       });
  434 |       await page.goto(url("en", "products"));
  435 |       await page.getByTestId("products-search").fill(tag);
  436 |       await page.getByTestId("products-search-submit").click();
  437 |       await expect(page.locator('[data-testid^="product-row-"]')).toHaveCount(
  438 |         2,
  439 |       );
  440 |       await page
  441 |         .getByTestId(`product-row-${id}`)
  442 |         .getByRole("button", { name: "Copy", exact: true })
  443 |         .click();
  444 |       await expect(page.getByTestId("product-batch-results")).toBeVisible();
  445 |       await page.reload();
  446 |       await expect(page.locator('[data-testid^="product-row-"]')).toHaveCount(
  447 |         3,
  448 |       );
  449 |       await expect(page.getByTestId("products-tab-active")).toHaveText(
  450 |         "Active 1",
  451 |       );
  452 |       await expect(page.getByTestId("products-tab-draft")).toHaveText(
  453 |         "Draft 2",
  454 |       );
  455 |       for (const locale of ["zh-TW", "zh-CN", "en"] as const) {
  456 |         await page.getByTestId("locale-switch").selectOption(locale);
  457 |         await expect(page).toHaveURL(new RegExp(`/${locale}/products`));
  458 |         for (const [width, height] of [[1586, 992], [390, 844]]) {
  459 |           await page.setViewportSize({ width, height });
  460 |           await assertProductListLayout(page);
  461 |           await shot(`list-populated-${locale}-${width}`);
  462 |         }
  463 |       }
  464 |       await page.setViewportSize({ width: 1586, height: 992 });
  465 |       await page.getByRole("checkbox", { name: "All", exact: true }).check();
  466 |       await page
  467 |         .getByTestId("product-batch")
  468 |         .getByRole("button", { name: "Unpublish to draft", exact: true })
  469 |         .click();
  470 |       await expect(page.getByTestId("product-batch-results")).toBeVisible();
  471 |       await page.reload();
  472 |       await expect(page.locator(".product-status").filter({ hasText: /^Draft$/ })).toHaveCount(3);
  473 |       await expect(page.getByTestId("products-tab-active")).toHaveText(
  474 |         "Active 0",
  475 |       );
  476 |       await expect(page.getByTestId("products-tab-draft")).toHaveText(
  477 |         "Draft 3",
  478 |       );
  479 |       await shot("batch-result-en-1586");
  480 |       ledger.push({
  481 |         page: "list",
  482 |         control: "copy/select all/unpublish",
  483 |         action: "click check reload",
  484 |         expected: "copy persisted;3 results;all3draft",
  485 |         actual: "PASS",
  486 |       });
  487 |       for (const [width, height] of [
  488 |         [1366, 768],
  489 |         [1586, 992],
  490 |       ]) {
  491 |         await page.setViewportSize({ width, height });
  492 |         await page.goto(url("en", "inventory"));
  493 |         await page.locator("button.product-name").first().click();
  494 |         const inspector = page.locator(".inspector");
  495 |         await expect(inspector).toBeVisible();
  496 |         await expect(inspector.getByTestId("tray-price")).toBeVisible();
  497 |         await expect(
  498 |           inspector.locator(
  499 |             'input[type="file"],input[name="price"],input[name="name"]',
  500 |           ),
  501 |         ).toHaveCount(0);
  502 |         await expect(inspector.locator("a.tray-edit")).toBeVisible();
  503 |         await fit();
  504 |         await shot(`inventory-en-${width}`);
  505 |       }
  506 |       ledger.push({
  507 |         page: "inventory",
  508 |         control: "product inspector",
```
