# G-UI9 visual lint — b10a8cdda6 — 2026-10-30T14:33:11.857Z

Verdict: **FAIL** — 2 enumerated shots missing

Shots: 81 captured of 83 enumerated (36 admin + 17 storefront + 5 platform pages x 3 locales x 2 viewports 1586x992, 390x844). NOT_RUN: 0. Missing: 2. HTTP >= 400: 0. States not reached: 0.

Missing shots:
- storefront|orders-orderid|zh-TW|1586x992
- storefront|orders-orderid|en|1586x992

Blocking (exit 1): R1, R2, R3, R6, R9 and R7:clipped-control, R7:text-overflows-control, R7:select-value-clipped; every other finding is WARN for now. Thresholds: `tests/ui/visual-lint-lib.mjs` (T). Instances = every measured occurrence; groups = distinct (rule, kind, selector shape).

## Counts per rule

| rule | what | severity | instances | of which blocking | groups | page-variants affected |
| --- | --- | --- | ---: | ---: | ---: | ---: |
| R1 | horizontal overflow | block | 0 | 0 | 0 | 0 |
| R2 | form-row misalignment | block | 0 | 0 | 0 | 0 |
| R3 | table row consistency | block | 0 | 0 | 0 | 0 |
| R4 | min text size | warn | 0 | 0 | 0 | 0 |
| R5 | tap target (390) | warn | 0 | 0 | 0 | 0 |
| R6 | overlap | block | 0 | 0 | 0 | 0 |
| R7 | clipped text | block for controls, warn for labels/links | 0 | 0 | 0 | 0 |
| R8 | duplicate list label | warn | 0 | 0 | 0 | 0 |
| R9 | edge padding (390) | block | 0 | 0 | 0 | 0 |
| R10 | fixed-bar occlusion | warn | 0 | 0 | 0 | 0 |
| R11 | narrow control | warn | 0 | 0 | 0 | 0 |

## Counts per app (instances)

| app | page-variants | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | blocking |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| admin | 42 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| platform | 7 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| storefront | 32 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |

## Top 15 routes by violations

| # | app | route | state | blocking | warn | total |
| ---: | --- | --- | --- | ---: | ---: | ---: |
| 1 | storefront | `/cart` (cart) | registry | 0 | 0 | 0 |
| 2 | storefront | `/checkout` (checkout) | registry | 0 | 0 | 0 |
| 3 | storefront | `/claim` (claim) | registry | 0 | 0 | 0 |
| 4 | storefront | `/collections` (collections) | registry | 0 | 0 | 0 |
| 5 | storefront | `/collections/[slug]` (collections-slug) | registry | 0 | 0 | 0 |
| 6 | platform | `/contact` (contact) | public | 0 | 0 | 0 |
| 7 | platform | `/data-deletion` (data-deletion) | public | 0 | 0 | 0 |
| 8 | storefront | `/data-deletion` (data-deletion) | registry | 0 | 0 | 0 |
| 9 | admin | `/finance` (finance) | registry | 0 | 0 | 0 |
| 10 | admin | `/finance/reports` (finance-reports) | registry | 0 | 0 | 0 |
| 11 | admin | `/` (home) | registry | 0 | 0 | 0 |
| 12 | platform | `/` (home) | public | 0 | 0 | 0 |
| 13 | storefront | `/` (home) | registry | 0 | 0 | 0 |
| 14 | admin | `/inventory` (inventory) | registry | 0 | 0 | 0 |
| 15 | admin | `/invite/[token]` (invite-token) | registry | 0 | 0 | 0 |

## storefront

### `/cart` (cart, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/cart/en-1586x992.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/cart/zh-TW-1586x992.png` |

### `/checkout` (checkout, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/checkout/en-1586x992.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/checkout/zh-TW-1586x992.png` |

### `/claim` (claim, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/claim/en-1586x992.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/claim/zh-TW-1586x992.png` |

### `/collections` (collections, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/collections/en-1586x992.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/collections/zh-TW-1586x992.png` |

### `/collections/[slug]` (collections-slug, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/collections-slug/en-1586x992.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/collections-slug/zh-TW-1586x992.png` |

### `/data-deletion` (data-deletion, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/data-deletion/en-1586x992.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/data-deletion/zh-TW-1586x992.png` |

### `/` (home, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/home/en-1586x992.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/home/zh-TW-1586x992.png` |

### `/legal/anti-fraud` (legal-anti-fraud, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/legal-anti-fraud/en-1586x992.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/legal-anti-fraud/zh-TW-1586x992.png` |

### `/legal/[slug]` (legal-slug, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/legal-slug/en-1586x992.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/legal-slug/zh-TW-1586x992.png` |

### `/order-link` (order-link, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/order-link/en-1586x992.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/order-link/zh-TW-1586x992.png` |

### `/orders/lookup` (orders-lookup, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/orders-lookup/en-1586x992.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/orders-lookup/zh-TW-1586x992.png` |

### `/pages/[slug]` (pages-slug, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/pages-slug/en-1586x992.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/pages-slug/zh-TW-1586x992.png` |

### `/privacy` (privacy, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/privacy/en-1586x992.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/privacy/zh-TW-1586x992.png` |

### `/products` (products, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/products/en-1586x992.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/products/zh-TW-1586x992.png` |

### `/products/[slug]` (products-slug, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/products-slug/en-1586x992.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/products-slug/zh-TW-1586x992.png` |

### `/search` (search, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/search/en-1586x992.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/storefront/search/zh-TW-1586x992.png` |

## platform

### `/contact` (contact, public) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/contact/zh-CN-1586x992.png` |

### `/data-deletion` (data-deletion, public) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/data-deletion/en-1586x992.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/data-deletion/zh-TW-1586x992.png` |

### `/` (home, public) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/home/zh-CN-1586x992.png` |

### `/privacy` (privacy, public) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/privacy/en-1586x992.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/privacy/zh-TW-1586x992.png` |

### `/terms` (terms, public) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/platform/terms/zh-CN-1586x992.png` |

## admin

### `/finance` (finance, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/finance/en-1586x992.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/finance/zh-TW-1586x992.png` |

### `/finance/reports` (finance-reports, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/finance-reports/zh-CN-1586x992.png` |

### `/` (home, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/home/zh-CN-1586x992.png` |

### `/inventory` (inventory, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/inventory/en-1586x992.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/inventory/zh-TW-1586x992.png` |

### `/invite/[token]` (invite-token, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/invite-token/zh-CN-1586x992.png` |

### `/messages` (messages, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/messages/en-1586x992.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/messages/zh-TW-1586x992.png` |

### `/orders` (orders, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/orders/zh-CN-1586x992.png` |

### `/orders/cvs-print` (orders-cvs-print, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/orders-cvs-print/zh-CN-1586x992.png` |

### `/orders/new` (orders-new, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/orders-new/en-1586x992.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/orders-new/zh-TW-1586x992.png` |

### `/products` (products, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/products/en-1586x992.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/products/zh-TW-1586x992.png` |

### `/products/import` (products-import, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/products-import/en-1586x992.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/products-import/zh-TW-1586x992.png` |

### `/products/new` (products-new, create-empty) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/products-new/en-1586x992.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/products-new/zh-TW-1586x992.png` |

Supplemental viewport en 1440x900: `shots/admin/products-new/en-1440x900.png`; counts {}. Included in the owning row's totals; primary matrix unchanged.

Supplemental viewport zh-TW 1440x900: `shots/admin/products-new/zh-TW-1440x900.png`; counts {}. Included in the owning row's totals; primary matrix unchanged.

### `/products/new` (products-new-variants, create-with-variants) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/products-new-variants/zh-CN-1586x992.png` |

Supplemental viewport zh-CN 1440x900: `shots/admin/products-new-variants/zh-CN-1440x900.png`; counts {}. Included in the owning row's totals; primary matrix unchanged.

### `/products/[product]` (products-product, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/products-product/zh-CN-1586x992.png` |

Supplemental viewport zh-CN 1440x900: `shots/admin/products-product/zh-CN-1440x900.png`; counts {}. Included in the owning row's totals; primary matrix unchanged.

### `/promotions` (promotions, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/promotions/zh-CN-1586x992.png` |

### `/reset` (reset, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/reset/en-1586x992.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/reset/zh-TW-1586x992.png` |

### `/returns` (returns, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/returns/zh-CN-1586x992.png` |

### `/settings` (settings, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/settings/zh-CN-1586x992.png` |

### `/settings/operations` (settings-operations, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/settings-operations/en-1586x992.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/settings-operations/zh-TW-1586x992.png` |

### `/settings/payments/card` (settings-payments-card, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/settings-payments-card/zh-CN-1586x992.png` |

### `/settings/settlements` (settings-settlements, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/settings-settlements/en-1586x992.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/settings-settlements/zh-TW-1586x992.png` |

### `/` (signed-out-home, signed-out) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/signed-out-home/en-1586x992.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/signed-out-home/zh-TW-1586x992.png` |

### `/signup` (signup, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/signup/zh-CN-1586x992.png` |

### `/studio` (studio, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/studio/zh-CN-1586x992.png` |

### `/studio/claims` (studio-claims, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/studio-claims/en-1586x992.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/studio-claims/zh-TW-1586x992.png` |

### `/studio/console` (studio-console, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| zh-CN 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/studio-console/zh-CN-1586x992.png` |

### `/studio/settings` (studio-settings, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/studio-settings/en-1586x992.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/studio-settings/zh-TW-1586x992.png` |

### `/team` (team, registry) — blocking 0, warn 0

| variant | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 | R10 | R11 | shot |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| en 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/team/en-1586x992.png` |
| zh-TW 1586x992 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | `shots/admin/team/zh-TW-1586x992.png` |
