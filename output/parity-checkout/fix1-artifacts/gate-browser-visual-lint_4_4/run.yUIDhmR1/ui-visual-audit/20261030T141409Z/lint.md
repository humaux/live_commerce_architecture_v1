# G-UI9 visual lint — b10a8cdda6 — 2026-10-30T14:26:50.727Z

Verdict: **FAIL** — 1 enumerated shots missing

Shots: 90 captured of 91 enumerated (36 admin + 17 storefront + 5 platform pages x 3 locales x 2 viewports 1586x992, 390x844). NOT_RUN: 0. Missing: 1. HTTP >= 400: 0. States not reached: 0.

Missing shots:
- storefront|orders-orderid|zh-CN|390x844

Blocking (exit 1): R1, R2, R3, R6, R9 and R7:clipped-control, R7:text-overflows-control, R7:select-value-clipped; every other finding is WARN for now. Thresholds: `tests/ui/visual-lint-lib.mjs` (T). Instances = every measured occurrence; groups = distinct (rule, kind, selector shape).

## Counts per rule

| rule | what | severity | instances | of which blocking | groups | page-variants affected |
| --- | --- | --- | ---: | ---: | ---: | ---: |
| R1 | horizontal overflow | block | 0 | 0 | 0 | 0 |
| R2 | form-row misalignment | block | 0 | 0 | 0 | 0 |
| R3 | table row consistency | block | 0 | 0 | 0 | 0 |
| R4 | min text size | warn | 0 | 0 | 0 | 0 |
| R5 | tap target (390) | warn | 8 | 0 | 7 | 7 |
| R6 | overlap | block | 0 | 0 | 0 | 0 |
| R7 | clipped text | block for controls, warn for labels/links | 72 | 0 | 72 | 60 |
| R8 | duplicate list label | warn | 0 | 0 | 0 | 0 |
| R9 | edge padding (390) | block | 0 | 0 | 0 | 0 |
| R10 | fixed-bar occlusion | warn | 0 | 0 | 0 | 0 |
| R11 | narrow control | warn | 0 | 0 | 0 | 0 |

## Counts per app (instances)

| app | page-variants | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | blocking |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| admin | 66 | 0 | 0 | 0 | 0 | 8 | 0 | 72 | 0 | 0 | 0 | 0 | 0 |
| platform | 8 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| storefront | 16 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |

## Top 15 routes by violations

| # | app | route | state | blocking | warn | total |
| ---: | --- | --- | --- | ---: | ---: | ---: |
| 1 | admin | `/orders` (orders) | registry | 0 | 6 | 6 |
| 2 | admin | `/ads/attribution` (ads-attribution) | registry | 0 | 4 | 4 |
| 3 | admin | `/products/new` (products-new-variants) | create-with-variants | 0 | 4 | 4 |
| 4 | admin | `/products/[product]` (products-product) | registry | 0 | 4 | 4 |
| 5 | admin | `/promotions` (promotions) | registry | 0 | 4 | 4 |
| 6 | admin | `/settings/payments/card` (settings-payments-card) | registry | 0 | 4 | 4 |
| 7 | admin | `/ads` (ads) | registry | 0 | 3 | 3 |
| 8 | admin | `/billing` (billing) | registry | 0 | 3 | 3 |
| 9 | admin | `/collections` (collections) | registry | 0 | 3 | 3 |
| 10 | admin | `/customers` (customers) | registry | 0 | 3 | 3 |
| 11 | admin | `/customers/[customer]` (customers-customer) | registry | 0 | 3 | 3 |
| 12 | admin | `/customers/import` (customers-import) | registry | 0 | 3 | 3 |
| 13 | admin | `/design` (design) | registry | 0 | 3 | 3 |
| 14 | admin | `/finance/reports` (finance-reports) | registry | 0 | 3 | 3 |
| 15 | admin | `/` (home) | registry | 0 | 3 | 3 |

## admin

### `/orders` (orders, registry) — blocking 0, warn 6

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 2 | 0 | 0 | 0 | 0 | `shots/admin/orders/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 2 | 0 | 0 | 0 | 0 | `shots/admin/orders/zh-TW-390x844.png` |

### `/ads/attribution` (ads-attribution, registry) — blocking 0, warn 4

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 0 | `shots/admin/ads-attribution/en-390x844.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/ads-attribution/zh-CN-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/ads-attribution/zh-TW-390x844.png` |

### `/products/new` (products-new-variants, create-with-variants) — blocking 0, warn 4

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 0 | `shots/admin/products-new-variants/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 0 | `shots/admin/products-new-variants/zh-TW-390x844.png` |

### `/products/[product]` (products-product, registry) — blocking 0, warn 4

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/products-product/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/products-product/zh-TW-390x844.png` |

### `/promotions` (promotions, registry) — blocking 0, warn 4

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 0 | `shots/admin/promotions/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 0 | `shots/admin/promotions/zh-TW-390x844.png` |

### `/settings/payments/card` (settings-payments-card, registry) — blocking 0, warn 4

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/settings-payments-card/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 1 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/settings-payments-card/zh-TW-390x844.png` |

### `/ads` (ads, registry) — blocking 0, warn 3

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/ads/en-390x844.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/ads/zh-CN-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/ads/zh-TW-390x844.png` |

### `/billing` (billing, registry) — blocking 0, warn 3

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/billing/en-390x844.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/billing/zh-CN-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/billing/zh-TW-390x844.png` |

### `/collections` (collections, registry) — blocking 0, warn 3

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/collections/en-390x844.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/collections/zh-CN-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/collections/zh-TW-390x844.png` |

### `/customers` (customers, registry) — blocking 0, warn 3

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/customers/en-390x844.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/customers/zh-CN-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/customers/zh-TW-390x844.png` |

### `/customers/[customer]` (customers-customer, registry) — blocking 0, warn 3

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/customers-customer/en-390x844.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/customers-customer/zh-CN-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/customers-customer/zh-TW-390x844.png` |

### `/customers/import` (customers-import, registry) — blocking 0, warn 3

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/customers-import/en-390x844.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/customers-import/zh-CN-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/customers-import/zh-TW-390x844.png` |

### `/design` (design, registry) — blocking 0, warn 3

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/design/en-390x844.png` |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/design/zh-CN-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/design/zh-TW-390x844.png` |

### `/finance/reports` (finance-reports, registry) — blocking 0, warn 3

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 0 | `shots/admin/finance-reports/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/finance-reports/zh-TW-390x844.png` |

### `/` (home, registry) — blocking 0, warn 3

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/home/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 0 | `shots/admin/home/zh-TW-390x844.png` |

### `/settings/operations` (settings-operations, registry) — blocking 0, warn 3

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 2 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/settings-operations/zh-CN-390x844.png` |

### `/studio/console` (studio-console, registry) — blocking 0, warn 3

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 0 | `shots/admin/studio-console/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/studio-console/zh-TW-390x844.png` |

### `/orders/cvs-print` (orders-cvs-print, registry) — blocking 0, warn 2

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/orders-cvs-print/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/orders-cvs-print/zh-TW-390x844.png` |

### `/products/new` (products-new, create-empty) — blocking 0, warn 2

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 0 | `shots/admin/products-new/zh-CN-390x844.png` |

### `/returns` (returns, registry) — blocking 0, warn 2

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/returns/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/returns/zh-TW-390x844.png` |

### `/settings` (settings, registry) — blocking 0, warn 2

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/settings/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/settings/zh-TW-390x844.png` |

### `/studio` (studio, registry) — blocking 0, warn 2

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/studio/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/studio/zh-TW-390x844.png` |

### `/team` (team, registry) — blocking 0, warn 2

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 2 | 0 | 0 | 0 | 0 | `shots/admin/team/zh-CN-390x844.png` |

### `/finance` (finance, registry) — blocking 0, warn 1

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/finance/zh-CN-390x844.png` |

### `/inventory` (inventory, registry) — blocking 0, warn 1

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/inventory/zh-CN-390x844.png` |

### `/messages` (messages, registry) — blocking 0, warn 1

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/messages/zh-CN-390x844.png` |

### `/orders/new` (orders-new, registry) — blocking 0, warn 1

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/orders-new/zh-CN-390x844.png` |

### `/products` (products, registry) — blocking 0, warn 1

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/products/zh-CN-390x844.png` |

### `/products/import` (products-import, registry) — blocking 0, warn 1

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/products-import/zh-CN-390x844.png` |

### `/settings/settlements` (settings-settlements, registry) — blocking 0, warn 1

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/settings-settlements/zh-CN-390x844.png` |

### `/studio/claims` (studio-claims, registry) — blocking 0, warn 1

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/studio-claims/zh-CN-390x844.png` |

### `/studio/settings` (studio-settings, registry) — blocking 0, warn 1

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | `shots/admin/studio-settings/zh-CN-390x844.png` |

### `/invite/[token]` (invite-token, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/invite-token/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/invite-token/zh-TW-390x844.png` |

### `/reset` (reset, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/reset/zh-CN-390x844.png` |

### `/` (signed-out-home, signed-out) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/signed-out-home/zh-CN-390x844.png` |

### `/signup` (signup, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/signup/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/signup/zh-TW-390x844.png` |

## storefront

### `/cart` (cart, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/cart/zh-CN-390x844.png` |

### `/checkout` (checkout, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/checkout/zh-CN-390x844.png` |

### `/claim` (claim, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/claim/zh-CN-390x844.png` |

### `/collections` (collections, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/collections/zh-CN-390x844.png` |

### `/collections/[slug]` (collections-slug, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/collections-slug/zh-CN-390x844.png` |

### `/data-deletion` (data-deletion, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/data-deletion/zh-CN-390x844.png` |

### `/` (home, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/home/zh-CN-390x844.png` |

### `/legal/anti-fraud` (legal-anti-fraud, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/legal-anti-fraud/zh-CN-390x844.png` |

### `/legal/[slug]` (legal-slug, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/legal-slug/zh-CN-390x844.png` |

### `/order-link` (order-link, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/order-link/zh-CN-390x844.png` |

### `/orders/lookup` (orders-lookup, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/orders-lookup/zh-CN-390x844.png` |

### `/pages/[slug]` (pages-slug, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/pages-slug/zh-CN-390x844.png` |

### `/privacy` (privacy, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/privacy/zh-CN-390x844.png` |

### `/products` (products, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/products/zh-CN-390x844.png` |

### `/products/[slug]` (products-slug, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/products-slug/zh-CN-390x844.png` |

### `/search` (search, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/search/zh-CN-390x844.png` |

## platform

### `/contact` (contact, public) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/contact/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/contact/zh-TW-390x844.png` |

### `/data-deletion` (data-deletion, public) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/data-deletion/zh-CN-390x844.png` |

### `/` (home, public) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/home/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/home/zh-TW-390x844.png` |

### `/privacy` (privacy, public) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/privacy/zh-CN-390x844.png` |

### `/terms` (terms, public) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/terms/en-390x844.png` |
| zh-TW 390x844 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/terms/zh-TW-390x844.png` |
