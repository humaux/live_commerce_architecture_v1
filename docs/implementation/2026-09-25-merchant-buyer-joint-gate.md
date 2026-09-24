# Merchant save to buyer page joint gate — 2026-09-25

Scope: a real merchant creates a product and its first SKU in the production
admin Next UI, using the existing signed MOCK IdP login and real admin
BFF/Go/PostgreSQL authorization. The UI's configured purchase URL must open the
production buyer Next product route in the same isolated store. No product or
purchase-entry endpoint is mocked. No runtime, schema or authentication code is
changed by this gate.

Run from the repository root:

```sh
GOFLAGS=-p=1 bash scripts/dev/test-local.sh --browser-merchant-buyer
```

The runner builds both Next applications with features disabled at build time,
creates a fresh owned PostgreSQL 18 container, and selects only
`TestBrowserMerchantBuyerRealChain` with race detection and a 180-second timeout.
Its flag was integrated separately by the root agent (source `9d832cd`).

## Accepted evidence

Final command above: **exit 0**, 8 browser cases, 3 locales, one top-level Go
test PASS; foundation package 4.862s. Node syntax check, `git diff --check`, and
`GOTOOLCHAIN=go1.27.1 go vet -tags browser ./tests/foundation` also exit 0.

| Gate | Real evidence |
| --- | --- |
| Merchant save | Browser follows signed MOCK IdP redirect; UI saves one product and one SKU through the actual BFF/Go routes. Independent PG readback matches tenant, store, product, SKU, currency and 12345 minor units; exactly two new merchant command receipts. |
| Scope/stock | Fixture principal is granted only store:read, catalog:read, catalog:write and inventory:read for the selected store. New SKU's merchant ledger available quantity is 0; no stock is invented. |
| Configured URL | Actual four-field purchase-entry response equals the displayed URL for en, zh-CN and zh-TW. English Open purchase page control fresh-reads and navigates to that exact URL. |
| Crawler safety | All three exact URLs serve actual Next documents with JavaScript disabled; all 16 observed database fact counts remain unchanged. |
| Buyer selection | Each locale shows the just-created product, one selected SKU with its actual ID/code and persisted currency/price. Actual buyer catalog read agrees. 390px viewport has no horizontal overflow. |
| Tenant isolation | A real foreign-tenant fixture product returns HTTP 200 with exactly an empty catalog page and no selectable SKU. |
| Unpublish | Using the valid buyer context captured before unpublish, the supported PUT cart command returns exactly 404/not_found. Reload shows product error with no SKU. Merchant projection removes the URL and Open control. All 16 fact counts remain unchanged across denial. |
| No purchase effects | Views do not add cart lines, quotes, storefront events, orders, checkout receipts, payment attempts, reservations, inventory ledger entries or integration operations/events. Actual request counters confirm one product POST and one SKU POST only. |

Retained evidence (local worktree artifacts, not committed):

- Full build/PG/browser log: `/Volumes/data/output/merchant-buyer-joint-final.log`
  SHA-256 `c15159268306b75854f1bf03a747cdf94f92d527883baa5e9193224aeafcd393`.
- Result, application logs and mobile screenshot:
  `output/playwright/merchant-buyer-real-3141202634/`.
- `result.json` SHA-256
  `f74bb6405f1b9cbd7664ef94bb83cc0c4a43a2c56f5fa658c8efb307211b85d5`.
- Screenshot `buyer-from-merchant-mobile.png` was visually inspected: correct
  synthetic product, $123.45, selected SKU and delivery CTA, no overlap.

The reused buyer fixture pre-seeds a cart and quote; assertions compare actual
before/after counts, not fabricated all-zero baselines. Opening the hydrated
buyer page creates its anonymous buyer owner/session, as expected. Crawler GETs
create no buyer or purchase facts. This gate makes no availability promise from
the ability to select a SKU.

## Boundaries and retained failures

The IdP, publication verification evidence and disposable TLS certificate are
synthetic. A test-only CONNECT proxy maps only `buyer.example:443` to the owned
loopback edge; there is no production host map, DNS edit or ingress bypass.
Application requests use ordinary runtime roles. Fixture owner credentials stay
in Go; the privileged facts/unpublish listener is loopback-only, requires a
random key, and its key is excluded from both Next environments and the browser.
The owned PostgreSQL container, Next children, sockets and generated certificate
directory are removed after the run; evidence remains available.

Initial failures are retained at
`/Volumes/data/output/merchant-buyer-joint-initial.log` and
`/Volumes/data/output/merchant-buyer-joint-second.log`, with screenshot/log
directories `merchant-buyer-real-466432083` and
`merchant-buyer-real-659717394`. They exposed test issues: a broad alert locator
also matched Next's route announcer, and POST cart returned 405 because the
supported mutation is PUT. Neither is used as a passing denial signal. The
corrected run passed before the final stock/isolation assertions were added;
the final fresh run passed all assertions.

NOT_RUN: full foundation regression, address/checkout UI, real merchant IdP,
deployment DNS/TLS, carrier/CVS and payment-provider acceptance. Those remain
separate gates owned by the main workstream. No customer production, live
broadcast or external provider was touched.
