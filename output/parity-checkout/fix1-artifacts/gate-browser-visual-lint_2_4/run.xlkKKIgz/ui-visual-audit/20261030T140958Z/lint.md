# G-UI9 visual lint — b10a8cdda6 — 2026-10-30T14:19:12.541Z

Verdict: **FAIL** — 2 enumerated shots missing

Shots: 81 captured of 83 enumerated (36 admin + 17 storefront + 5 platform pages x 3 locales x 2 viewports 1586x992, 390x844). NOT_RUN: 0. Missing: 2. HTTP >= 400: 0. States not reached: 0.

Missing shots:
- storefront|orders-orderid|zh-TW|390x844
- storefront|orders-orderid|en|390x844

Blocking (exit 1): R1, R2, R3, R6, R9 and R7:clipped-control, R7:text-overflows-control, R7:select-value-clipped; every other finding is WARN for now. Thresholds: `tests/ui/visual-lint-lib.mjs` (T). Instances = every measured occurrence; groups = distinct (rule, kind, selector shape).

## Counts per rule

| rule | what | severity | instances | of which blocking | groups | page-variants affected |
| --- | --- | --- | ---: | ---: | ---: | ---: |
| R1 | horizontal overflow | block | 0 | 0 | 0 | 0 |
| R2 | form-row misalignment | block | 0 | 0 | 0 | 0 |
| R3 | table row consistency | block | 0 | 0 | 0 | 0 |
| R4 | min text size | warn | 0 | 0 | 0 | 0 |
| R5 | tap target (390) | warn | 7 | 0 | 5 | 5 |
| R6 | overlap | block | 0 | 0 | 0 | 0 |
| R7 | clipped text | block for controls, warn for labels/links | 42 | 0 | 42 | 36 |
| R8 | duplicate list label | warn | 0 | 0 | 0 | 0 |
| R9 | edge padding (390) | block | 0 | 0 | 0 | 0 |
| R10 | fixed-bar occlusion | warn | 0 | 0 | 0 | 0 |
| R11 | narrow control | warn | 0 | 0 | 0 | 0 |

## Counts per app (instances)

| app | page-variants | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | blocking |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| admin | 42 | 0 | 0 | 0 | 0 | 7 | 0 | 42 | 0 | 0 | 0 | 0 | 0 |
| platform | 7 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| storefront | 32 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |

## Top 15 routes by violations

| # | app | route | state | blocking | warn | total |
| ---: | --- | --- | --- | ---: | ---: | ---: |
| 1 | admin | `/settings/operations` (settings-operations) | registry | 0 | 6 | 6 |
| 2 | admin | `/products/new` (products-new) | create-empty | 0 | 4 | 4 |
| 3 | admin | `/orders` (orders) | registry | 0 | 3 | 3 |
| 4 | admin | `/finance` (finance) | registry | 0 | 2 | 2 |
| 5 | admin | `/` (home) | registry | 0 | 2 | 2 |
| 6 | admin | `/inventory` (inventory) | registry | 0 | 2 | 2 |
| 7 | admin | `/messages` (messages) | registry | 0 | 2 | 2 |
| 8 | admin | `/orders/new` (orders-new) | registry | 0 | 2 | 2 |
| 9 | admin | `/products` (products) | registry | 0 | 2 | 2 |
| 10 | admin | `/products/import` (products-import) | registry | 0 | 2 | 2 |
| 11 | admin | `/products/new` (products-new-variants) | create-with-variants | 0 | 2 | 2 |
| 12 | admin | `/products/[product]` (products-product) | registry | 0 | 2 | 2 |
| 13 | admin | `/promotions` (promotions) | registry | 0 | 2 | 2 |
| 14 | admin | `/settings/payments/card` (settings-payments-card) | registry | 0 | 2 | 2 |
| 15 | admin | `/settings/settlements` (settings-settlements) | registry | 0 | 2 | 2 |

## admin

### `/settings/operations` (settings-operations, registry) — blocking 0, warn 6

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 2 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/settings-operations/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 2 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/settings-operations/zh-TW-390x844.png` |

### `/products/new` (products-new, create-empty) — blocking 0, warn 4

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 0 | `shots/admin/products-new/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 0 | `shots/admin/products-new/zh-TW-390x844.png` |

### `/orders` (orders, registry) — blocking 0, warn 3

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 2 | 0 | 0 | 0 | 0 | `shots/admin/orders/zh-CN-390x844.png` |

### `/finance` (finance, registry) — blocking 0, warn 2

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/finance/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/finance/zh-TW-390x844.png` |

### `/` (home, registry) — blocking 0, warn 2

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 0 | `shots/admin/home/zh-CN-390x844.png` |

### `/inventory` (inventory, registry) — blocking 0, warn 2

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/inventory/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/inventory/zh-TW-390x844.png` |

### `/messages` (messages, registry) — blocking 0, warn 2

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/messages/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/messages/zh-TW-390x844.png` |

### `/orders/new` (orders-new, registry) — blocking 0, warn 2

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/orders-new/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/orders-new/zh-TW-390x844.png` |

### `/products` (products, registry) — blocking 0, warn 2

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/products/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/products/zh-TW-390x844.png` |

### `/products/import` (products-import, registry) — blocking 0, warn 2

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/products-import/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/products-import/zh-TW-390x844.png` |

### `/products/new` (products-new-variants, create-with-variants) — blocking 0, warn 2

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 0 | `shots/admin/products-new-variants/zh-CN-390x844.png` |

### `/products/[product]` (products-product, registry) — blocking 0, warn 2

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/products-product/zh-CN-390x844.png` |

### `/promotions` (promotions, registry) — blocking 0, warn 2

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 0 | `shots/admin/promotions/zh-CN-390x844.png` |

### `/settings/payments/card` (settings-payments-card, registry) — blocking 0, warn 2

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/settings-payments-card/zh-CN-390x844.png` |

### `/settings/settlements` (settings-settlements, registry) — blocking 0, warn 2

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/settings-settlements/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/settings-settlements/zh-TW-390x844.png` |

### `/studio/claims` (studio-claims, registry) — blocking 0, warn 2

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/studio-claims/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/studio-claims/zh-TW-390x844.png` |

### `/studio/settings` (studio-settings, registry) — blocking 0, warn 2

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/studio-settings/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/studio-settings/zh-TW-390x844.png` |

### `/team` (team, registry) — blocking 0, warn 2

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/team/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/team/zh-TW-390x844.png` |

### `/finance/reports` (finance-reports, registry) — blocking 0, warn 1

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/finance-reports/zh-CN-390x844.png` |

### `/orders/cvs-print` (orders-cvs-print, registry) — blocking 0, warn 1

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/orders-cvs-print/zh-CN-390x844.png` |

### `/returns` (returns, registry) — blocking 0, warn 1

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/returns/zh-CN-390x844.png` |

### `/settings` (settings, registry) — blocking 0, warn 1

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/settings/zh-CN-390x844.png` |

### `/studio` (studio, registry) — blocking 0, warn 1

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/studio/zh-CN-390x844.png` |

### `/studio/console` (studio-console, registry) — blocking 0, warn 1

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/studio-console/zh-CN-390x844.png` |

### `/invite/[token]` (invite-token, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/invite-token/zh-CN-390x844.png` |

### `/reset` (reset, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/reset/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/reset/zh-TW-390x844.png` |

### `/` (signed-out-home, signed-out) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/signed-out-home/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/signed-out-home/zh-TW-390x844.png` |

### `/signup` (signup, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/signup/zh-CN-390x844.png` |

## storefront

### `/cart` (cart, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/cart/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/cart/zh-TW-390x844.png` |

### `/checkout` (checkout, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/checkout/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/checkout/zh-TW-390x844.png` |

### `/claim` (claim, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/claim/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/claim/zh-TW-390x844.png` |

### `/collections` (collections, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/collections/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/collections/zh-TW-390x844.png` |

### `/collections/[slug]` (collections-slug, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/collections-slug/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/collections-slug/zh-TW-390x844.png` |

### `/data-deletion` (data-deletion, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/data-deletion/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/data-deletion/zh-TW-390x844.png` |

### `/` (home, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/home/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/home/zh-TW-390x844.png` |

### `/legal/anti-fraud` (legal-anti-fraud, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/legal-anti-fraud/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/legal-anti-fraud/zh-TW-390x844.png` |

### `/legal/[slug]` (legal-slug, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/legal-slug/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/legal-slug/zh-TW-390x844.png` |

### `/order-link` (order-link, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/order-link/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/order-link/zh-TW-390x844.png` |

### `/orders/lookup` (orders-lookup, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/orders-lookup/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/orders-lookup/zh-TW-390x844.png` |

### `/pages/[slug]` (pages-slug, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/pages-slug/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/pages-slug/zh-TW-390x844.png` |

### `/privacy` (privacy, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/privacy/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/privacy/zh-TW-390x844.png` |

### `/products` (products, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/products/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/products/zh-TW-390x844.png` |

### `/products/[slug]` (products-slug, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/products-slug/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/products-slug/zh-TW-390x844.png` |

### `/search` (search, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/search/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/search/zh-TW-390x844.png` |

## platform

### `/contact` (contact, public) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/contact/zh-CN-390x844.png` |

### `/data-deletion` (data-deletion, public) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/data-deletion/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/data-deletion/zh-TW-390x844.png` |

### `/` (home, public) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/home/zh-CN-390x844.png` |

### `/privacy` (privacy, public) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/privacy/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/privacy/zh-TW-390x844.png` |

### `/terms` (terms, public) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/terms/zh-CN-390x844.png` |
