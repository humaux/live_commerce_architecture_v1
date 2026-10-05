# G-UI9 visual lint — 0cc63a1dc8 — 2026-10-05T04:58:14.655Z

Verdict: **FAIL** — 70 blocking violation instances (R1/R2/R3/R6)

Shots: 294 captured of 294 enumerated (27 admin + 17 storefront + 5 platform pages x 3 locales x 2 viewports 1586x992, 390x844). Missing: 0. HTTP >= 400: 0. States not reached: 0.

R1 R2 R3 R6 are blocking (exit 1); R4 R5 R7 R8 are WARN in this first version. Thresholds: `tests/ui/visual-lint-lib.mjs` (T). Instances = every measured occurrence; groups = distinct (rule, kind, selector shape).

## Counts per rule

| rule | what | severity | instances | groups | page-variants affected |
| --- | --- | --- | ---: | ---: | ---: |
| R1 | horizontal overflow | block | 0 | 0 | 0 |
| R2 | form-row misalignment | block | 39 | 39 | 12 |
| R3 | table row consistency | block | 30 | 9 | 6 |
| R4 | min text size | warn | 277 | 166 | 51 |
| R5 | tap target (390) | warn | 594 | 349 | 126 |
| R6 | overlap | block | 1 | 1 | 1 |
| R7 | clipped text | warn | 0 | 0 | 0 |
| R8 | duplicate list label | warn | 15 | 15 | 15 |

## Counts per app (instances)

| app | page-variants | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| admin | 162 | 0 | 39 | 30 | 277 | 266 | 1 | 0 | 15 |
| platform | 30 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| storefront | 102 | 0 | 0 | 0 | 0 | 328 | 0 | 0 | 0 |

## Top 15 routes by violations

| # | app | route | state | blocking | warn | total |
| ---: | --- | --- | --- | ---: | ---: | ---: |
| 1 | admin | `/inventory` (inventory) | registry | 0 | 115 | 115 |
| 2 | admin | `/studio/claims` (studio-claims) | registry | 18 | 96 | 114 |
| 3 | admin | `/products/new` (products-new-variants) | create-with-variants | 16 | 69 | 85 |
| 4 | admin | `/products/[product]` (products-product) | registry | 6 | 45 | 51 |
| 5 | admin | `/settings` (settings) | registry | 0 | 42 | 42 |
| 6 | admin | `/` (home) | registry | 0 | 32 | 32 |
| 7 | admin | `/products` (products) | registry | 21 | 3 | 24 |
| 8 | admin | `/products/new` (products-new) | create-empty | 0 | 24 | 24 |
| 9 | storefront | `/orders/lookup` (orders-lookup) | registry | 0 | 23 | 23 |
| 10 | admin | `/ads/attribution` (ads-attribution) | registry | 0 | 21 | 21 |
| 11 | storefront | `/cart` (cart) | registry | 0 | 20 | 20 |
| 12 | storefront | `/checkout` (checkout) | registry | 0 | 20 | 20 |
| 13 | storefront | `/collections` (collections) | registry | 0 | 20 | 20 |
| 14 | storefront | `/collections/[slug]` (collections-slug) | registry | 0 | 20 | 20 |
| 15 | storefront | `/data-deletion` (data-deletion) | registry | 0 | 20 | 20 |

## admin

### `/inventory` (inventory, registry) — blocking 0, warn 115

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 3 | 0 | 0 | 0 | 0 | `shots/admin/inventory/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 22 | 14 | 0 | 0 | 0 | `shots/admin/inventory/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 3 | 0 | 0 | 0 | 0 | `shots/admin/inventory/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 21 | 14 | 0 | 0 | 0 | `shots/admin/inventory/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 3 | 0 | 0 | 0 | 0 | `shots/admin/inventory/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 21 | 14 | 0 | 0 | 0 | `shots/admin/inventory/zh-TW-390x844.png` |

### `/studio/claims` (studio-claims, registry) — blocking 18, warn 96

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 4 | 0 | 6 | 0 | 0 | 0 | 0 | `shots/admin/studio-claims/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 6 | 20 | 0 | 0 | 0 | `shots/admin/studio-claims/en-390x844.png` |
| zh-CN 1586x992 | 0 | 7 | 0 | 6 | 0 | 0 | 0 | 0 | `shots/admin/studio-claims/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 6 | 20 | 0 | 0 | 0 | `shots/admin/studio-claims/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 7 | 0 | 6 | 0 | 0 | 0 | 0 | `shots/admin/studio-claims/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 6 | 20 | 0 | 0 | 0 | `shots/admin/studio-claims/zh-TW-390x844.png` |

R2 form-row misalignment — worst variant zh-CN 1586x992 (7 instances, 7 groups):
- label-top `select#claims-mode` rect 269,425,1266,77.80000000000001 measured {"labelTops":[425,431.8],"controlTops":[452,458.8],"singleLineHeights":[39,44],"fields":["数量规则","这条帖子或直播所属的主页"],"spread":6.8} crop `crops/admin/studio-claims/zh-CN-1586x992-R2-1.png`
- control-top `select#claims-mode` rect 269,425,1266,77.80000000000001 measured {"labelTops":[425,431.8],"controlTops":[452,458.8],"singleLineHeights":[39,44],"fields":["数量规则","这条帖子或直播所属的主页"],"spread":6.8} crop `crops/admin/studio-claims/zh-CN-1586x992-R2-2.png`
- control-height `select#claims-mode` rect 269,425,1266,77.80000000000001 measured {"labelTops":[425,431.8],"controlTops":[452,458.8],"singleLineHeights":[39,44],"fields":["数量规则","这条帖子或直播所属的主页"],"spread":5} crop `crops/admin/studio-claims/zh-CN-1586x992-R2-3.png`

### `/products/new` (products-new-variants, create-with-variants) — blocking 16, warn 69

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 2 | 3 | 8 | 0 | 1 | 0 | 1 | `shots/admin/products-new-variants/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 11 | 3 | 0 | 0 | 0 | `shots/admin/products-new-variants/en-390x844.png` |
| zh-CN 1586x992 | 0 | 2 | 3 | 8 | 0 | 0 | 0 | 1 | `shots/admin/products-new-variants/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 11 | 3 | 0 | 0 | 0 | `shots/admin/products-new-variants/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 2 | 3 | 8 | 0 | 0 | 0 | 1 | `shots/admin/products-new-variants/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 11 | 3 | 0 | 0 | 0 | `shots/admin/products-new-variants/zh-TW-390x844.png` |

R2 form-row misalignment — worst variant en 1586x992 (2 instances, 2 groups):
- label-top `input[data-testid="axis-name-0"]` rect 494,788.8,948.5,114 measured {"labelTops":[839.8,788.8],"controlTops":[858.8,807.8],"singleLineHeights":[44],"fields":["Option name","ValuesS, M, LPress Enter or use commas /"],"spread":51} crop `crops/admin/products-new-variants/en-1586x992-R2-1.png`
- control-top `input[data-testid="axis-name-0"]` rect 494,788.8,948.5,114 measured {"labelTops":[839.8,788.8],"controlTops":[858.8,807.8],"singleLineHeights":[44],"fields":["Option name","ValuesS, M, LPress Enter or use commas /"],"spread":51} crop `crops/admin/products-new-variants/en-1586x992-R2-2.png`

R3 table row consistency — worst variant en 1586x992 (3 instances, 1 groups):
- row-stacked `div[data-testid="matrix-row-0"]` rect 494,1101,1047,109 measured {"rowHeight":109,"tallestSingleLineControl":44,"ratio":2.5,"stackedControls":2,"stackedLines":2,"medianRowHeight":109} crop `crops/admin/products-new-variants/en-1586x992-R3-3.png`

R6 overlap — worst variant en 1586x992 (1 instances, 1 groups):
- text-overlap `form[data-testid="product-create-form"] > footer.pe-savebar > span` rect 244,950.5,104.10000000000002,19.899999999999977 measured {"overlapWidth":75.5,"overlapHeight":9.1,"a":"Unsaved changes","b":"Collections"} crop `crops/admin/products-new-variants/en-1586x992-R6-8.png`

### `/products/[product]` (products-product, registry) — blocking 6, warn 45

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 2 | 0 | 5 | 0 | 0 | 0 | 1 | `shots/admin/products-product/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 6 | 3 | 0 | 0 | 0 | `shots/admin/products-product/en-390x844.png` |
| zh-CN 1586x992 | 0 | 2 | 0 | 5 | 0 | 0 | 0 | 1 | `shots/admin/products-product/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 6 | 3 | 0 | 0 | 0 | `shots/admin/products-product/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 2 | 0 | 5 | 0 | 0 | 0 | 1 | `shots/admin/products-product/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 6 | 3 | 0 | 0 | 0 | `shots/admin/products-product/zh-TW-390x844.png` |

R2 form-row misalignment — worst variant en 1586x992 (2 instances, 2 groups):
- label-top `input[data-testid="axis-name-0"]` rect 494,836.6,948.5,114 measured {"labelTops":[887.6,836.6],"controlTops":[906.6,855.6],"singleLineHeights":[44],"fields":["Option name","ValuesT04-4a98f4a9b261-0Press Enter or u"],"spread":51} crop `crops/admin/products-product/en-1586x992-R2-1.png`
- control-top `input[data-testid="axis-name-0"]` rect 494,836.6,948.5,114 measured {"labelTops":[887.6,836.6],"controlTops":[906.6,855.6],"singleLineHeights":[44],"fields":["Option name","ValuesT04-4a98f4a9b261-0Press Enter or u"],"spread":51} crop `crops/admin/products-product/en-1586x992-R2-2.png`

### `/settings` (settings, registry) — blocking 0, warn 42

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/settings/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 9 | 4 | 0 | 0 | 0 | `shots/admin/settings/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/settings/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 9 | 4 | 0 | 0 | 0 | `shots/admin/settings/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/settings/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 9 | 4 | 0 | 0 | 0 | `shots/admin/settings/zh-TW-390x844.png` |

### `/` (home, registry) — blocking 0, warn 32

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/home/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 12 | 0 | 0 | 0 | `shots/admin/home/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/home/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 11 | 0 | 0 | 0 | `shots/admin/home/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/home/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 9 | 0 | 0 | 0 | `shots/admin/home/zh-TW-390x844.png` |

### `/products` (products, registry) — blocking 21, warn 3

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 7 | 0 | 0 | 0 | 0 | 0 | `shots/admin/products/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | `shots/admin/products/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 7 | 0 | 0 | 0 | 0 | 0 | `shots/admin/products/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | `shots/admin/products/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 7 | 0 | 0 | 0 | 0 | 0 | `shots/admin/products/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | `shots/admin/products/zh-TW-390x844.png` |

R3 table row consistency — worst variant en 1586x992 (7 instances, 2 groups):
- cell-overflow `table[data-testid="products-table"] > thead > tr > th:nth-of-type(1)` rect 245,363.6,44.5,73.5 measured {"cellWidth":44.5,"cellScrollWidth":66,"cellClientWidth":44,"controlsOutside":[{"id":65,"left":279.5,"right":299.5,"width":20}],"cellLeft":245,"cellRight":289.5} crop `crops/admin/products/en-1586x992-R3-1.png`
- cell-overflow `tr[data-testid="product-row-7d7cc0dc-7f6a-45d6-9e7e-76551d76862e"] > td:nth-of-type(1)` rect 245,437.1,44.5,75 measured {"cellWidth":44.5,"cellScrollWidth":66,"cellClientWidth":44,"controlsOutside":[{"id":74,"left":279.5,"right":299.5,"width":20}],"cellLeft":245,"cellRight":289.5} crop `crops/admin/products/en-1586x992-R3-2.png`

### `/products/new` (products-new, create-empty) — blocking 0, warn 24

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 1 | `shots/admin/products-new/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 2 | 3 | 0 | 0 | 0 | `shots/admin/products-new/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 1 | `shots/admin/products-new/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 2 | 3 | 0 | 0 | 0 | `shots/admin/products-new/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 1 | `shots/admin/products-new/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 2 | 3 | 0 | 0 | 0 | `shots/admin/products-new/zh-TW-390x844.png` |

### `/ads/attribution` (ads-attribution, registry) — blocking 0, warn 21

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | `shots/admin/ads-attribution/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 4 | 1 | 0 | 0 | 1 | `shots/admin/ads-attribution/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | `shots/admin/ads-attribution/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 4 | 1 | 0 | 0 | 1 | `shots/admin/ads-attribution/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | `shots/admin/ads-attribution/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 4 | 1 | 0 | 0 | 1 | `shots/admin/ads-attribution/zh-TW-390x844.png` |

### `/team` (team, registry) — blocking 9, warn 9

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 3 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/team/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 3 | 0 | 0 | 0 | `shots/admin/team/en-390x844.png` |
| zh-CN 1586x992 | 0 | 3 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/team/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 3 | 0 | 0 | 0 | `shots/admin/team/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 3 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/team/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 3 | 0 | 0 | 0 | `shots/admin/team/zh-TW-390x844.png` |

R2 form-row misalignment — worst variant en 1586x992 (3 instances, 3 groups):
- label-top `input[data-testid="team-invite-email"]` rect 271,313.8,400,64 measured {"labelTops":[319.8,313.8],"controlTops":[338.8,332.8],"singleLineHeights":[39,45],"fields":["Email address","RoleOwnerAdminLive operatorFulfilmentVie"],"spread":6} crop `crops/admin/team/en-1586x992-R2-1.png`
- control-top `input[data-testid="team-invite-email"]` rect 271,313.8,400,64 measured {"labelTops":[319.8,313.8],"controlTops":[338.8,332.8],"singleLineHeights":[39,45],"fields":["Email address","RoleOwnerAdminLive operatorFulfilmentVie"],"spread":6} crop `crops/admin/team/en-1586x992-R2-2.png`
- control-height `input[data-testid="team-invite-email"]` rect 271,313.8,400,64 measured {"labelTops":[319.8,313.8],"controlTops":[338.8,332.8],"singleLineHeights":[39,45],"fields":["Email address","RoleOwnerAdminLive operatorFulfilmentVie"],"spread":6} crop `crops/admin/team/en-1586x992-R2-3.png`

### `/collections` (collections, registry) — blocking 0, warn 15

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 0 | `shots/admin/collections/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 2 | 1 | 0 | 0 | 0 | `shots/admin/collections/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 0 | `shots/admin/collections/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 2 | 1 | 0 | 0 | 0 | `shots/admin/collections/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 0 | `shots/admin/collections/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 2 | 1 | 0 | 0 | 0 | `shots/admin/collections/zh-TW-390x844.png` |

### `/orders` (orders, registry) — blocking 0, warn 15

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 3 | 0 | 0 | 0 | 0 | `shots/admin/orders/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 1 | 1 | 0 | 0 | 0 | `shots/admin/orders/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 3 | 0 | 0 | 0 | 0 | `shots/admin/orders/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 1 | 1 | 0 | 0 | 0 | `shots/admin/orders/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 3 | 0 | 0 | 0 | 0 | `shots/admin/orders/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 1 | 1 | 0 | 0 | 0 | `shots/admin/orders/zh-TW-390x844.png` |

### `/customers/[customer]` (customers-customer, registry) — blocking 0, warn 9

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/customers-customer/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 3 | 0 | 0 | 0 | `shots/admin/customers-customer/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/customers-customer/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 3 | 0 | 0 | 0 | `shots/admin/customers-customer/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/customers-customer/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 3 | 0 | 0 | 0 | `shots/admin/customers-customer/zh-TW-390x844.png` |

### `/design` (design, registry) — blocking 0, warn 9

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/design/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 3 | 0 | 0 | 0 | `shots/admin/design/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/design/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 3 | 0 | 0 | 0 | `shots/admin/design/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/design/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 3 | 0 | 0 | 0 | `shots/admin/design/zh-TW-390x844.png` |

### `/studio` (studio, registry) — blocking 0, warn 9

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/studio/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 3 | 0 | 0 | 0 | `shots/admin/studio/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/studio/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 3 | 0 | 0 | 0 | `shots/admin/studio/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/studio/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 3 | 0 | 0 | 0 | `shots/admin/studio/zh-TW-390x844.png` |

### `/ads` (ads, registry) — blocking 0, warn 6

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/ads/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | `shots/admin/ads/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/ads/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | `shots/admin/ads/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/ads/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | `shots/admin/ads/zh-TW-390x844.png` |

### `/products/import` (products-import, registry) — blocking 0, warn 6

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/products-import/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | `shots/admin/products-import/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/products-import/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | `shots/admin/products-import/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/products-import/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | `shots/admin/products-import/zh-TW-390x844.png` |

### `/reset` (reset, registry) — blocking 0, warn 6

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/reset/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | `shots/admin/reset/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/reset/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | `shots/admin/reset/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/reset/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | `shots/admin/reset/zh-TW-390x844.png` |

### `/` (signed-out-home, signed-out) — blocking 0, warn 6

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/signed-out-home/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | `shots/admin/signed-out-home/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/signed-out-home/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | `shots/admin/signed-out-home/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/signed-out-home/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | `shots/admin/signed-out-home/zh-TW-390x844.png` |

### `/signup` (signup, registry) — blocking 0, warn 6

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/signup/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | `shots/admin/signup/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/signup/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | `shots/admin/signup/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/signup/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | `shots/admin/signup/zh-TW-390x844.png` |

### `/billing` (billing, registry) — blocking 0, warn 3

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/billing/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | `shots/admin/billing/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/billing/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | `shots/admin/billing/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/billing/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | `shots/admin/billing/zh-TW-390x844.png` |

### `/customers` (customers, registry) — blocking 0, warn 3

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/customers/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | `shots/admin/customers/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/customers/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | `shots/admin/customers/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/customers/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | `shots/admin/customers/zh-TW-390x844.png` |

### `/finance` (finance, registry) — blocking 0, warn 3

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/finance/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | `shots/admin/finance/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/finance/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | `shots/admin/finance/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/finance/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | `shots/admin/finance/zh-TW-390x844.png` |

### `/orders/new` (orders-new, registry) — blocking 0, warn 3

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/orders-new/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | `shots/admin/orders-new/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/orders-new/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | `shots/admin/orders-new/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/orders-new/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | `shots/admin/orders-new/zh-TW-390x844.png` |

### `/promotions` (promotions, registry) — blocking 0, warn 3

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/promotions/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | `shots/admin/promotions/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/promotions/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | `shots/admin/promotions/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/promotions/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | `shots/admin/promotions/zh-TW-390x844.png` |

### `/invite/[token]` (invite-token, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/invite-token/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/invite-token/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/invite-token/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/invite-token/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/invite-token/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/invite-token/zh-TW-390x844.png` |

### `/orders/cvs-print` (orders-cvs-print, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/orders-cvs-print/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/orders-cvs-print/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/orders-cvs-print/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/orders-cvs-print/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/orders-cvs-print/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/orders-cvs-print/zh-TW-390x844.png` |

## storefront

### `/orders/lookup` (orders-lookup, registry) — blocking 0, warn 23

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/orders-lookup/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | `shots/storefront/orders-lookup/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/orders-lookup/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 8 | 0 | 0 | 0 | `shots/storefront/orders-lookup/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/orders-lookup/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 8 | 0 | 0 | 0 | `shots/storefront/orders-lookup/zh-TW-390x844.png` |

### `/cart` (cart, registry) — blocking 0, warn 20

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/cart/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | `shots/storefront/cart/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/cart/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | `shots/storefront/cart/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/cart/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | `shots/storefront/cart/zh-TW-390x844.png` |

### `/checkout` (checkout, registry) — blocking 0, warn 20

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/checkout/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | `shots/storefront/checkout/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/checkout/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | `shots/storefront/checkout/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/checkout/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | `shots/storefront/checkout/zh-TW-390x844.png` |

### `/collections` (collections, registry) — blocking 0, warn 20

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/collections/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | `shots/storefront/collections/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/collections/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | `shots/storefront/collections/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/collections/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | `shots/storefront/collections/zh-TW-390x844.png` |

### `/collections/[slug]` (collections-slug, registry) — blocking 0, warn 20

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/collections-slug/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | `shots/storefront/collections-slug/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/collections-slug/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | `shots/storefront/collections-slug/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/collections-slug/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | `shots/storefront/collections-slug/zh-TW-390x844.png` |

### `/data-deletion` (data-deletion, registry) — blocking 0, warn 20

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/data-deletion/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | `shots/storefront/data-deletion/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/data-deletion/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | `shots/storefront/data-deletion/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/data-deletion/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | `shots/storefront/data-deletion/zh-TW-390x844.png` |

### `/legal/anti-fraud` (legal-anti-fraud, registry) — blocking 0, warn 20

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/legal-anti-fraud/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | `shots/storefront/legal-anti-fraud/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/legal-anti-fraud/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | `shots/storefront/legal-anti-fraud/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/legal-anti-fraud/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | `shots/storefront/legal-anti-fraud/zh-TW-390x844.png` |

### `/order-link` (order-link, registry) — blocking 0, warn 20

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/order-link/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | `shots/storefront/order-link/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/order-link/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | `shots/storefront/order-link/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/order-link/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | `shots/storefront/order-link/zh-TW-390x844.png` |

### `/pages/[slug]` (pages-slug, registry) — blocking 0, warn 20

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/pages-slug/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | `shots/storefront/pages-slug/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/pages-slug/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | `shots/storefront/pages-slug/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/pages-slug/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | `shots/storefront/pages-slug/zh-TW-390x844.png` |

### `/products` (products, registry) — blocking 0, warn 20

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/products/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | `shots/storefront/products/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/products/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | `shots/storefront/products/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/products/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | `shots/storefront/products/zh-TW-390x844.png` |

### `/products/[slug]` (products-slug, registry) — blocking 0, warn 20

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/products-slug/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | `shots/storefront/products-slug/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/products-slug/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | `shots/storefront/products-slug/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/products-slug/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | `shots/storefront/products-slug/zh-TW-390x844.png` |

### `/search` (search, registry) — blocking 0, warn 20

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/search/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | `shots/storefront/search/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/search/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | `shots/storefront/search/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/search/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 7 | 0 | 0 | 0 | `shots/storefront/search/zh-TW-390x844.png` |

### `/claim` (claim, registry) — blocking 0, warn 17

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/claim/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 5 | 0 | 0 | 0 | `shots/storefront/claim/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/claim/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | `shots/storefront/claim/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/claim/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | `shots/storefront/claim/zh-TW-390x844.png` |

### `/` (home, registry) — blocking 0, warn 17

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/home/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 5 | 0 | 0 | 0 | `shots/storefront/home/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/home/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | `shots/storefront/home/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/home/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | `shots/storefront/home/zh-TW-390x844.png` |

### `/legal/[slug]` (legal-slug, registry) — blocking 0, warn 17

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/legal-slug/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 5 | 0 | 0 | 0 | `shots/storefront/legal-slug/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/legal-slug/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | `shots/storefront/legal-slug/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/legal-slug/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | `shots/storefront/legal-slug/zh-TW-390x844.png` |

### `/orders/[orderID]` (orders-orderid, registry) — blocking 0, warn 17

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/orders-orderid/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 5 | 0 | 0 | 0 | `shots/storefront/orders-orderid/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/orders-orderid/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | `shots/storefront/orders-orderid/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/orders-orderid/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | `shots/storefront/orders-orderid/zh-TW-390x844.png` |

### `/privacy` (privacy, registry) — blocking 0, warn 17

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/privacy/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 5 | 0 | 0 | 0 | `shots/storefront/privacy/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/privacy/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | `shots/storefront/privacy/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/privacy/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 6 | 0 | 0 | 0 | `shots/storefront/privacy/zh-TW-390x844.png` |

## platform

### `/contact` (contact, public) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/contact/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/contact/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/contact/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/contact/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/contact/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/contact/zh-TW-390x844.png` |

### `/data-deletion` (data-deletion, public) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/data-deletion/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/data-deletion/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/data-deletion/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/data-deletion/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/data-deletion/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/data-deletion/zh-TW-390x844.png` |

### `/` (home, public) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/home/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/home/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/home/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/home/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/home/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/home/zh-TW-390x844.png` |

### `/privacy` (privacy, public) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/privacy/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/privacy/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/privacy/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/privacy/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/privacy/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/privacy/zh-TW-390x844.png` |

### `/terms` (terms, public) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/terms/en-1586x992.png` |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/terms/en-390x844.png` |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/terms/zh-CN-1586x992.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/terms/zh-CN-390x844.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/terms/zh-TW-1586x992.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/terms/zh-TW-390x844.png` |
