# G-UI9 visual lint — 2c29cbab79 — 2026-10-05T05:51:44.996Z

Verdict: **FAIL** — 188 blocking violation instances (R1/R2/R3/R6/R9 and R7 clipped controls)

Shots: 294 captured of 294 enumerated (27 admin + 17 storefront + 5 platform pages x 3 locales x 2 viewports 1586x992, 390x844). NOT_RUN: 0. Missing: 0. HTTP >= 400: 0. States not reached: 0.

Blocking (exit 1): R1, R2, R3, R6, R9 and R7:clipped-control, R7:text-overflows-control, R7:select-value-clipped; every other finding is WARN for now. Thresholds: `tests/ui/visual-lint-lib.mjs` (T). Instances = every measured occurrence; groups = distinct (rule, kind, selector shape).

## Counts per rule

| rule | what | severity | instances | of which blocking | groups | page-variants affected |
| --- | --- | --- | ---: | ---: | ---: | ---: |
| R1 | horizontal overflow | block | 0 | 0 | 0 | 0 |
| R2 | form-row misalignment | block | 39 | 39 | 39 | 12 |
| R3 | table row consistency | block | 30 | 30 | 9 | 6 |
| R4 | min text size | warn | 277 | 0 | 166 | 51 |
| R5 | tap target (390) | warn | 600 | 0 | 355 | 129 |
| R6 | overlap | block | 4 | 4 | 4 | 4 |
| R7 | clipped text | block for controls, warn for labels/links | 17 | 6 | 17 | 14 |
| R8 | duplicate list label | warn | 15 | 0 | 15 | 15 |
| R9 | edge padding (390) | block | 109 | 109 | 91 | 10 |
| R10 | fixed-bar occlusion | warn | 22 | 0 | 20 | 20 |
| R11 | narrow control | warn | 3 | 0 | 3 | 1 |

## Counts per app (instances)

| app | page-variants | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | blocking |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| admin | 162 | 0 | 39 | 30 | 277 | 272 | 4 | 17 | 15 | 1 | 18 | 3 | 80 |
| platform | 30 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| storefront | 102 | 0 | 0 | 0 | 0 | 328 | 0 | 0 | 0 | 108 | 4 | 0 | 108 |

## Top 15 routes by violations

| # | app | route | state | blocking | warn | total |
| ---: | --- | --- | --- | ---: | ---: | ---: |
| 1 | admin | `/studio/claims` (studio-claims) | registry | 21 | 99 | 120 |
| 2 | admin | `/inventory` (inventory) | registry | 0 | 115 | 115 |
| 3 | admin | `/products/new` (products-new-variants) | create-with-variants | 16 | 75 | 91 |
| 4 | storefront | `/orders/[orderID]` (orders-orderid) | registry | 72 | 17 | 89 |
| 5 | admin | `/products/[product]` (products-product) | registry | 6 | 51 | 57 |
| 6 | storefront | `/orders/lookup` (orders-lookup) | registry | 24 | 23 | 47 |
| 7 | admin | `/settings` (settings) | registry | 0 | 42 | 42 |
| 8 | admin | `/` (home) | registry | 0 | 34 | 34 |
| 9 | storefront | `/order-link` (order-link) | registry | 12 | 20 | 32 |
| 10 | admin | `/products/new` (products-new) | create-empty | 0 | 30 | 30 |
| 11 | admin | `/products` (products) | registry | 24 | 3 | 27 |
| 12 | admin | `/ads/attribution` (ads-attribution) | registry | 3 | 22 | 25 |
| 13 | storefront | `/products/[slug]` (products-slug) | registry | 0 | 24 | 24 |
| 14 | admin | `/team` (team) | registry | 9 | 12 | 21 |
| 15 | storefront | `/cart` (cart) | registry | 0 | 20 | 20 |

## admin

### `/studio/claims` (studio-claims, registry) — blocking 21, warn 99

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 4 | 0 | 6 | 0 | 0 | 3 | 0 | 0 | 0 | 3 | `shots/admin/studio-claims/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 6 | 20 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/studio-claims/en-390x844.png` |
| zh-CN 1586x992 | 0 | 7 | 0 | 6 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/studio-claims/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 6 | 20 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/studio-claims/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 7 | 0 | 6 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/studio-claims/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 6 | 20 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/studio-claims/zh-TW-390x844.png` |

R2 form-row misalignment (blocking) — worst variant zh-CN 1586x992 (7 blocking of 7 instances, 7 groups):
- label-top `select#claims-mode` rect 269,425,1266,77.80000000000001 measured {"labelTops":[425,431.8],"controlTops":[452,458.8],"singleLineHeights":[39,44],"fields":["数量规则","这条帖子或直播所属的主页"],"spread":6.8} crop `crops/admin/studio-claims/zh-CN-1586x992-R2-1.png`
- control-top `select#claims-mode` rect 269,425,1266,77.80000000000001 measured {"labelTops":[425,431.8],"controlTops":[452,458.8],"singleLineHeights":[39,44],"fields":["数量规则","这条帖子或直播所属的主页"],"spread":6.8} crop `crops/admin/studio-claims/zh-CN-1586x992-R2-2.png`
- control-height `select#claims-mode` rect 269,425,1266,77.80000000000001 measured {"labelTops":[425,431.8],"controlTops":[452,458.8],"singleLineHeights":[39,44],"fields":["数量规则","这条帖子或直播所属的主页"],"spread":5} crop `crops/admin/studio-claims/zh-CN-1586x992-R2-3.png`

R7 clipped text (blocking) — worst variant en 1586x992 (3 blocking of 3 instances, 3 groups):
- text-overflows-control `div[data-testid="merchant-claims"] > div.claims-surface:nth-of-type(2) > div.claims-work > section.claims-section:nth-of-type(2) > form.claims-form.claims-offer-form > button.primary:nth-of-type(1)` rect 603.1,1045.2,62.2,16 measured {"hiddenPx":15.7,"boxWidth":46.5,"textWidth":62.2,"text":"Add offer"} crop `crops/admin/studio-claims/en-1586x992-R7-10.png`
- select-value-clipped `select#claims-offer-product` rect 669.5,955.7,65.1,39 measured {"valueWidth":110.5,"availableWidth":21,"selectWidth":65.1,"text":"Choose a product"} crop `crops/admin/studio-claims/en-1586x992-R7-11.png`
- select-value-clipped `select#claims-offer-sku` rect 746.6,955.7,46.5,39 measured {"valueWidth":92.6,"availableWidth":2,"selectWidth":46.5,"text":"Choose a SKU"} crop `crops/admin/studio-claims/en-1586x992-R7-12.png`

### `/inventory` (inventory, registry) — blocking 0, warn 115

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 3 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/inventory/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 22 | 14 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/inventory/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 3 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/inventory/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 21 | 14 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/inventory/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 3 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/inventory/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 21 | 14 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/inventory/zh-TW-390x844.png` |

### `/products/new` (products-new-variants, create-with-variants) — blocking 16, warn 75

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 2 | 3 | 8 | 0 | 1 | 0 | 1 | 0 | 1 | 0 | `shots/admin/products-new-variants/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 11 | 3 | 0 | 0 | 0 | 0 | 1 | 0 | `shots/admin/products-new-variants/en-390x844.png` |
| zh-CN 1586x992 | 0 | 2 | 3 | 8 | 0 | 0 | 0 | 1 | 0 | 1 | 0 | `shots/admin/products-new-variants/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 11 | 3 | 0 | 0 | 0 | 0 | 1 | 0 | `shots/admin/products-new-variants/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 2 | 3 | 8 | 0 | 0 | 0 | 1 | 0 | 1 | 0 | `shots/admin/products-new-variants/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 11 | 3 | 0 | 0 | 0 | 0 | 1 | 0 | `shots/admin/products-new-variants/zh-TW-390x844.png` |

R2 form-row misalignment (blocking) — worst variant en 1586x992 (2 blocking of 2 instances, 2 groups):
- label-top `input[data-testid="axis-name-0"]` rect 494,788.8,948.5,114 measured {"labelTops":[839.8,788.8],"controlTops":[858.8,807.8],"singleLineHeights":[44],"fields":["Option name","ValuesS, M, LPress Enter or use commas /"],"spread":51} crop `crops/admin/products-new-variants/en-1586x992-R2-1.png`
- control-top `input[data-testid="axis-name-0"]` rect 494,788.8,948.5,114 measured {"labelTops":[839.8,788.8],"controlTops":[858.8,807.8],"singleLineHeights":[44],"fields":["Option name","ValuesS, M, LPress Enter or use commas /"],"spread":51} crop `crops/admin/products-new-variants/en-1586x992-R2-2.png`

R3 table row consistency (blocking) — worst variant en 1586x992 (3 blocking of 3 instances, 1 groups):
- row-stacked `div[data-testid="matrix-row-0"]` rect 494,1101,1047,109 measured {"rowHeight":109,"tallestSingleLineControl":44,"ratio":2.5,"stackedControls":2,"stackedLines":2,"medianRowHeight":109} crop `crops/admin/products-new-variants/en-1586x992-R3-3.png`

R6 overlap (blocking) — worst variant en 1586x992 (1 blocking of 1 instances, 1 groups):
- text-overlap `form[data-testid="product-create-form"] > footer.pe-savebar > span` rect 244,950.5,104.10000000000002,19.899999999999977 measured {"overlapWidth":75.5,"overlapHeight":9.1,"a":"Unsaved changes","b":"Collections"} crop `crops/admin/products-new-variants/en-1586x992-R6-8.png`

### `/products/[product]` (products-product, registry) — blocking 6, warn 51

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 2 | 0 | 5 | 0 | 0 | 0 | 1 | 0 | 1 | 0 | `shots/admin/products-product/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 6 | 3 | 0 | 0 | 0 | 0 | 1 | 0 | `shots/admin/products-product/en-390x844.png` |
| zh-CN 1586x992 | 0 | 2 | 0 | 5 | 0 | 0 | 0 | 1 | 0 | 1 | 0 | `shots/admin/products-product/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 6 | 3 | 0 | 0 | 0 | 0 | 1 | 0 | `shots/admin/products-product/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 2 | 0 | 5 | 0 | 0 | 0 | 1 | 0 | 1 | 0 | `shots/admin/products-product/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 6 | 3 | 0 | 0 | 0 | 0 | 1 | 0 | `shots/admin/products-product/zh-TW-390x844.png` |

R2 form-row misalignment (blocking) — worst variant en 1586x992 (2 blocking of 2 instances, 2 groups):
- label-top `input[data-testid="axis-name-0"]` rect 494,836.6,948.5,114 measured {"labelTops":[887.6,836.6],"controlTops":[906.6,855.6],"singleLineHeights":[44],"fields":["Option name","ValuesT04-5c45b4cad87a-0Press Enter or u"],"spread":51} crop `crops/admin/products-product/en-1586x992-R2-1.png`
- control-top `input[data-testid="axis-name-0"]` rect 494,836.6,948.5,114 measured {"labelTops":[887.6,836.6],"controlTops":[906.6,855.6],"singleLineHeights":[44],"fields":["Option name","ValuesT04-5c45b4cad87a-0Press Enter or u"],"spread":51} crop `crops/admin/products-product/en-1586x992-R2-2.png`

### `/settings` (settings, registry) — blocking 0, warn 42

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/settings/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 9 | 4 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/settings/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/settings/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 9 | 4 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/settings/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/settings/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 9 | 4 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/settings/zh-TW-390x844.png` |

### `/` (home, registry) — blocking 0, warn 34

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/home/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 12 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/home/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/home/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 11 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/home/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/home/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 9 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/home/zh-TW-390x844.png` |

### `/products/new` (products-new, create-empty) — blocking 0, warn 30

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 1 | 0 | 1 | 0 | `shots/admin/products-new/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 2 | 3 | 0 | 0 | 0 | 0 | 1 | 0 | `shots/admin/products-new/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 1 | 0 | 1 | 0 | `shots/admin/products-new/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 2 | 3 | 0 | 0 | 0 | 0 | 1 | 0 | `shots/admin/products-new/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 1 | 0 | 1 | 0 | `shots/admin/products-new/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 2 | 3 | 0 | 0 | 0 | 0 | 1 | 0 | `shots/admin/products-new/zh-TW-390x844.png` |

### `/products` (products, registry) — blocking 24, warn 3

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 7 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | 0 | `shots/admin/products/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/products/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 7 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | 0 | `shots/admin/products/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/products/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 7 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | 0 | `shots/admin/products/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/products/zh-TW-390x844.png` |

R3 table row consistency (blocking) — worst variant en 1586x992 (7 blocking of 7 instances, 2 groups):
- cell-overflow `table[data-testid="products-table"] > thead > tr > th:nth-of-type(1)` rect 245,363.6,44.5,73.5 measured {"cellWidth":44.5,"cellScrollWidth":66,"cellClientWidth":44,"controlsOutside":[{"id":65,"left":279.5,"right":299.5,"width":20}],"cellLeft":245,"cellRight":289.5} crop `crops/admin/products/en-1586x992-R3-1.png`
- cell-overflow `tr[data-testid="product-row-0e319b08-6c6c-46b7-86d1-b5e70771fb4b"] > td:nth-of-type(1)` rect 245,437.1,44.5,75 measured {"cellWidth":44.5,"cellScrollWidth":66,"cellClientWidth":44,"controlsOutside":[{"id":74,"left":279.5,"right":299.5,"width":20}],"cellLeft":245,"cellRight":289.5} crop `crops/admin/products/en-1586x992-R3-2.png`

R6 overlap (blocking) — worst variant en 1586x992 (1 blocking of 1 instances, 1 groups):
- text-overlap `tr[data-testid="product-row-4fc6146b-237b-4771-b523-68d030ad57c4"] > td:nth-of-type(3) > span.orders-badge.product-status` rect 491.4,617.6,79.5,22.5 measured {"overlapWidth":2.5,"overlapHeight":3.5,"a":"Active","b":"1 variant"} crop `crops/admin/products/en-1586x992-R6-3.png`

### `/ads/attribution` (ads-attribution, registry) — blocking 3, warn 22

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | `shots/admin/ads-attribution/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 4 | 1 | 0 | 2 | 1 | 0 | 0 | 0 | `shots/admin/ads-attribution/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | `shots/admin/ads-attribution/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 4 | 1 | 0 | 1 | 1 | 0 | 0 | 0 | `shots/admin/ads-attribution/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | `shots/admin/ads-attribution/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 4 | 1 | 0 | 1 | 1 | 0 | 0 | 0 | `shots/admin/ads-attribution/zh-TW-390x844.png` |

R7 clipped text (blocking) — worst variant en 390x844 (1 blocking of 2 instances, 2 groups):
- select-value-clipped `select[data-testid="attribution-session"]` rect 201,491.2,173,44 measured {"valueWidth":193.7,"availableWidth":129,"selectWidth":173,"text":"Click sweep scene 784a9fcd1042"} crop `crops/admin/ads-attribution/en-390x844-R7-4.png`

### `/team` (team, registry) — blocking 9, warn 12

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 3 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/team/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 3 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/team/en-390x844.png` |
| zh-CN 1586x992 | 0 | 3 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/team/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 3 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/team/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 3 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/team/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 3 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/team/zh-TW-390x844.png` |

R2 form-row misalignment (blocking) — worst variant en 1586x992 (3 blocking of 3 instances, 3 groups):
- label-top `input[data-testid="team-invite-email"]` rect 271,313.8,400,64 measured {"labelTops":[319.8,313.8],"controlTops":[338.8,332.8],"singleLineHeights":[39,45],"fields":["Email address","RoleOwnerAdminLive operatorFulfilmentVie"],"spread":6} crop `crops/admin/team/en-1586x992-R2-1.png`
- control-top `input[data-testid="team-invite-email"]` rect 271,313.8,400,64 measured {"labelTops":[319.8,313.8],"controlTops":[338.8,332.8],"singleLineHeights":[39,45],"fields":["Email address","RoleOwnerAdminLive operatorFulfilmentVie"],"spread":6} crop `crops/admin/team/en-1586x992-R2-2.png`
- control-height `input[data-testid="team-invite-email"]` rect 271,313.8,400,64 measured {"labelTops":[319.8,313.8],"controlTops":[338.8,332.8],"singleLineHeights":[39,45],"fields":["Email address","RoleOwnerAdminLive operatorFulfilmentVie"],"spread":6} crop `crops/admin/team/en-1586x992-R2-3.png`

### `/orders` (orders, registry) — blocking 1, warn 16

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 3 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/orders/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 1 | 1 | 0 | 1 | 0 | 1 | 0 | 0 | `shots/admin/orders/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 3 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/orders/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 1 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/orders/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 3 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/orders/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 1 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/orders/zh-TW-390x844.png` |

R9 edge padding (390) (blocking) — worst variant en 390x844 (1 blocking of 1 instances, 1 groups):
- control-flush `button[data-testid="orders-bucket-transfer_review"]` rect 245.3,619.1,145.3,44 measured {"leftMargin":245.3,"rightMargin":-0.6,"tag":"button","text":"Review transfers 1"} crop `crops/admin/orders/en-390x844-R9-4.png`

### `/collections` (collections, registry) — blocking 0, warn 15

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/collections/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 2 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/collections/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/collections/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 2 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/collections/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/collections/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 2 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/collections/zh-TW-390x844.png` |

### `/customers/[customer]` (customers-customer, registry) — blocking 0, warn 10

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/customers-customer/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 3 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/customers-customer/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/customers-customer/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 3 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/customers-customer/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/customers-customer/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 3 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/customers-customer/zh-TW-390x844.png` |

### `/design` (design, registry) — blocking 0, warn 10

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/design/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 3 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/design/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/design/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 3 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/design/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/design/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 3 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/design/zh-TW-390x844.png` |

### `/studio` (studio, registry) — blocking 0, warn 9

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/studio/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 3 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/studio/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/studio/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 3 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/studio/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/studio/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 3 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/studio/zh-TW-390x844.png` |

### `/ads` (ads, registry) — blocking 0, warn 6

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/ads/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/ads/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/ads/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/ads/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/ads/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/ads/zh-TW-390x844.png` |

### `/orders/cvs-print` (orders-cvs-print, registry) — blocking 0, warn 6

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/orders-cvs-print/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/orders-cvs-print/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/orders-cvs-print/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/orders-cvs-print/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/orders-cvs-print/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/orders-cvs-print/zh-TW-390x844.png` |

### `/products/import` (products-import, registry) — blocking 0, warn 6

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/products-import/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/products-import/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/products-import/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/products-import/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/products-import/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/products-import/zh-TW-390x844.png` |

### `/reset` (reset, registry) — blocking 0, warn 6

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/reset/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/reset/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/reset/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/reset/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/reset/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/reset/zh-TW-390x844.png` |

### `/` (signed-out-home, signed-out) — blocking 0, warn 6

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/signed-out-home/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/signed-out-home/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/signed-out-home/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/signed-out-home/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/signed-out-home/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/signed-out-home/zh-TW-390x844.png` |

### `/signup` (signup, registry) — blocking 0, warn 6

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/signup/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/signup/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/signup/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/signup/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/signup/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/signup/zh-TW-390x844.png` |

### `/promotions` (promotions, registry) — blocking 0, warn 5

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/promotions/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/promotions/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/promotions/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/promotions/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/promotions/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/promotions/zh-TW-390x844.png` |

### `/billing` (billing, registry) — blocking 0, warn 3

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/billing/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/billing/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/billing/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/billing/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/billing/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/billing/zh-TW-390x844.png` |

### `/customers` (customers, registry) — blocking 0, warn 3

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/customers/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/customers/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/customers/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/customers/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/customers/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/customers/zh-TW-390x844.png` |

### `/finance` (finance, registry) — blocking 0, warn 3

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/finance/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/finance/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/finance/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/finance/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/finance/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/finance/zh-TW-390x844.png` |

### `/orders/new` (orders-new, registry) — blocking 0, warn 3

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/orders-new/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/orders-new/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/orders-new/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/orders-new/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/orders-new/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/orders-new/zh-TW-390x844.png` |

### `/invite/[token]` (invite-token, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/invite-token/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/invite-token/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/invite-token/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/invite-token/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/invite-token/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/invite-token/zh-TW-390x844.png` |

## storefront

### `/orders/[orderID]` (orders-orderid, registry) — blocking 72, warn 17

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/orders-orderid/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 5 | 0 | 0 | 0 | 24 | 0 | 0 | `shots/storefront/orders-orderid/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/orders-orderid/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | 24 | 0 | 0 | `shots/storefront/orders-orderid/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/orders-orderid/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | 24 | 0 | 0 | `shots/storefront/orders-orderid/zh-TW-390x844.png` |

R9 edge padding (390) (blocking) — worst variant en 390x844 (24 blocking of 24 instances, 19 groups):
- text-flush `h1#order-title` rect 0,100.6,266.4,33 measured {"leftMargin":0,"rightMargin":123.6,"text":"Pay the carrier when the parce"} crop `crops/storefront/orders-orderid/en-390x844-R9-2.png`
- text-flush `p[data-testid="order-state"]` rect 0,192.2,169.7,26 measured {"leftMargin":0,"rightMargin":220.3,"text":"Cash on delivery"} crop `crops/storefront/orders-orderid/en-390x844-R9-3.png`
- text-flush `section[data-testid="order-section"] > p:nth-of-type(2)` rect 0,239.3,101.7,18 measured {"leftMargin":0,"rightMargin":288.3,"text":"Order number"} crop `crops/storefront/orders-orderid/en-390x844-R9-4.png`

### `/orders/lookup` (orders-lookup, registry) — blocking 24, warn 23

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/orders-lookup/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | 8 | 0 | 0 | `shots/storefront/orders-lookup/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/orders-lookup/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 8 | 0 | 0 | 0 | 8 | 0 | 0 | `shots/storefront/orders-lookup/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/orders-lookup/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 8 | 0 | 0 | 0 | 8 | 0 | 0 | `shots/storefront/orders-lookup/zh-TW-390x844.png` |

R9 edge padding (390) (blocking) — worst variant en 390x844 (8 blocking of 8 instances, 7 groups):
- text-flush `main[data-testid="order-lookup"] > h1` rect 0,100.6,196.8,33 measured {"leftMargin":0,"rightMargin":193.2,"text":"Find your order"} crop `crops/storefront/orders-lookup/en-390x844-R9-4.png`
- text-flush `main[data-testid="order-lookup"] > p:nth-of-type(1)` rect 0,153.4,387.4,18 measured {"leftMargin":0,"rightMargin":2.6,"text":"Enter the order number from yo"} crop `crops/storefront/orders-lookup/en-390x844-R9-5.png`
- text-flush `main[data-testid="order-lookup"] > form > label:nth-of-type(1)` rect 0,244.8,101.7,18 measured {"leftMargin":0,"rightMargin":288.3,"text":"Order number"} crop `crops/storefront/orders-lookup/en-390x844-R9-6.png`

### `/order-link` (order-link, registry) — blocking 12, warn 20

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/order-link/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | 4 | 0 | 0 | `shots/storefront/order-link/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/order-link/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | 4 | 0 | 0 | `shots/storefront/order-link/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/order-link/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | 4 | 0 | 0 | `shots/storefront/order-link/zh-TW-390x844.png` |

R9 edge padding (390) (blocking) — worst variant en 390x844 (4 blocking of 4 instances, 4 groups):
- text-flush `main[data-testid="order-link"] > h1` rect 0,100.6,211.3,33 measured {"leftMargin":0,"rightMargin":178.7,"text":"Open your order"} crop `crops/storefront/order-link/en-390x844-R9-3.png`
- text-flush `div[data-testid="order-link-refused"] > p:nth-of-type(1)` rect 0,153.4,180.2,18 measured {"leftMargin":0,"rightMargin":209.8,"text":"This link cannot be used."} crop `crops/storefront/order-link/en-390x844-R9-4.png`
- text-flush `div[data-testid="order-link-refused"] > p.order-note:nth-of-type(2)` rect 0,191.2,377.6,17 measured {"leftMargin":0,"rightMargin":12.4,"text":"A link works once and expires."} crop `crops/storefront/order-link/en-390x844-R9-5.png`

### `/products/[slug]` (products-slug, registry) — blocking 0, warn 24

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/products-slug/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/products-slug/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/products-slug/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | 0 | 2 | 0 | `shots/storefront/products-slug/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/products-slug/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | 0 | 2 | 0 | `shots/storefront/products-slug/zh-TW-390x844.png` |

### `/cart` (cart, registry) — blocking 0, warn 20

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/cart/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/cart/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/cart/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/cart/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/cart/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/cart/zh-TW-390x844.png` |

### `/checkout` (checkout, registry) — blocking 0, warn 20

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/checkout/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/checkout/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/checkout/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/checkout/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/checkout/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/checkout/zh-TW-390x844.png` |

### `/collections` (collections, registry) — blocking 0, warn 20

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/collections/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/collections/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/collections/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/collections/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/collections/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/collections/zh-TW-390x844.png` |

### `/collections/[slug]` (collections-slug, registry) — blocking 0, warn 20

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/collections-slug/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/collections-slug/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/collections-slug/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/collections-slug/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/collections-slug/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/collections-slug/zh-TW-390x844.png` |

### `/data-deletion` (data-deletion, registry) — blocking 0, warn 20

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/data-deletion/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/data-deletion/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/data-deletion/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/data-deletion/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/data-deletion/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/data-deletion/zh-TW-390x844.png` |

### `/legal/anti-fraud` (legal-anti-fraud, registry) — blocking 0, warn 20

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/legal-anti-fraud/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/legal-anti-fraud/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/legal-anti-fraud/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/legal-anti-fraud/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/legal-anti-fraud/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/legal-anti-fraud/zh-TW-390x844.png` |

### `/pages/[slug]` (pages-slug, registry) — blocking 0, warn 20

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/pages-slug/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/pages-slug/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/pages-slug/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/pages-slug/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/pages-slug/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/pages-slug/zh-TW-390x844.png` |

### `/products` (products, registry) — blocking 0, warn 20

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/products/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/products/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/products/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/products/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/products/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/products/zh-TW-390x844.png` |

### `/search` (search, registry) — blocking 0, warn 20

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/search/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/search/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/search/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/search/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/search/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/search/zh-TW-390x844.png` |

### `/claim` (claim, registry) — blocking 0, warn 17

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/claim/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 5 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/claim/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/claim/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/claim/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/claim/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/claim/zh-TW-390x844.png` |

### `/` (home, registry) — blocking 0, warn 17

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/home/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 5 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/home/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/home/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/home/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/home/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/home/zh-TW-390x844.png` |

### `/legal/[slug]` (legal-slug, registry) — blocking 0, warn 17

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/legal-slug/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 5 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/legal-slug/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/legal-slug/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/legal-slug/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/legal-slug/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/legal-slug/zh-TW-390x844.png` |

### `/privacy` (privacy, registry) — blocking 0, warn 17

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/privacy/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 5 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/privacy/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/privacy/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/privacy/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/privacy/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/privacy/zh-TW-390x844.png` |

## platform

### `/contact` (contact, public) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/contact/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/contact/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/contact/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/contact/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/contact/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/contact/zh-TW-390x844.png` |

### `/data-deletion` (data-deletion, public) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/data-deletion/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/data-deletion/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/data-deletion/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/data-deletion/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/data-deletion/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/data-deletion/zh-TW-390x844.png` |

### `/` (home, public) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/home/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/home/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/home/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/home/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/home/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/home/zh-TW-390x844.png` |

### `/privacy` (privacy, public) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/privacy/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/privacy/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/privacy/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/privacy/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/privacy/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/privacy/zh-TW-390x844.png` |

### `/terms` (terms, public) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/terms/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/terms/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/terms/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/terms/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/terms/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/terms/zh-TW-390x844.png` |
