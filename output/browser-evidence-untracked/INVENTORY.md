<!-- Purpose: browser evidence producer/consumer inventory for this mechanical hygiene unit.
Depends on: base50086616 and current tracked browser source; generated detailed inventories remain source-only.
Used by: integrator review/CI; runtime coverage is listed in DELIVERY, not inferred here. -->
# Browser evidence inventory

All paths below are relative to `output/` unless marked otherwise. Ordinary gates allocate a unique ignored `output/playwright/run.XXXXXXXX` root before builds/log writes. Explicit roots are preserved. Shared Go helpers use that root; existing unique ignored builders already safe remain unchanged. Historical artifacts and golden inputs are retained.

| Modes / caller | Producer | Former destination | New destination |
|---|---|---|---|
| --browser-product-editor | tests/admin/product-{editor,review,visual}.acceptance.ts | product-ui-v2, product-ui-v2-fix, product-editor-visual | Go suite evidence/<same-name> |
| --browser-admin-legacy | entry.spec.ts, ledger.spec.ts, visual-states.spec.ts; playwright.config.ts | admin-ui-fixes; fixed ledger-review; Playwright report/results | suite evidence/<label>; report/results in per-run playwright |
| --browser-admin-shell | tests/admin/shell-runner.mjs; test-local.sh | ui-w0-shell, ui-w0-shell/brand-browser | run root/ui-w0-shell; brand-browser child |
| --browser-picklist | tests/admin/picklist.spec.ts; test-local.sh | ci-gates/picklist | run root/picklist |
| --browser-platform-site | tests/admin/platform-runner.mjs, tests/deploy/platform-edge.mjs; test-local.sh | platform-site; .impeccable/review/hero-repro.png | run root/platform-site; hero remains inside that directory |
| --browser-home-cod / --browser-checkout-offline | home-cod.spec.ts, home-cod-buyer.mjs | home-cod-ui copies | existing admin/buyer suite evidence/home-cod-ui |
| --browser-buyer / --browser-order / --browser-payment / --browser-webkit / --browser-e2e | storefront browser-gate.mjs/order-gate.mjs; test-local.sh | static storefront review copies; webkit aggregate logs | existing suite evidence/review/<label>; run root/webkit |
| --browser-cvs | taiwan-cvs.spec.ts, cvs-pap-only.mjs | admin-visual/cvs-print; static cvs review copies | existing suite evidence/cvs-print/<stamp> and review |
| --browser-storefront | media-sizes-perf.mjs; browser_storefront_test.go | media-sizes/<phase>.json + screenshots | existing suite evidence/media-sizes; after baseline via explicit readonly LC_MEDIA_SIZES_BASELINE |
| standalone legal pages | legal-pages.mjs | stripe-live-tests/lg01 | suite evidence/lg01 (standalone unique fallback) |
| --browser-claim-checkout | browser_claim_checkout_test.go | claim-direct-checkout evidence summary copies | current claim checkout suite evidence/claim-direct-checkout |
| --stripe-browser | browser_stripe_test.go; test-local.sh | stripe-b2-browser-tests | run root/stripe-b2-browser-tests |
| --browser-tracking-backfill / --browser-meta-health-ui | browser_tracking_backfill_test.go; test-local.sh | tracking-backfill and meta-health-ui roots | run root/tracking-backfill and meta-health-ui |
| --browser-live-console | browser_live_console_test.go | CI evidence fallback ci-gates | run root/live-console timestamped suite |
| --browser-click-sweep | browser_click_sweep_test.go; click-sweep.mjs; test-local.sh | ui-click-sweep journeys/ledger/results | run root/ui-click-sweep; stack logs in suite evidence |
| --browser-visual-lint | browser_click_sweep_test.go; visual-audit.mjs; test-local.sh | ui-visual-audit + global LATEST; shared sweep journeys | run root/ui-visual-sweep fixtures and sibling ui-visual-audit/<stamp>/LATEST |
| all browser mode CI logs | gates.yml | ci-gates browser mode logs | run root/ci-gates; artifacts include ignored output/playwright; reader descends nested paths |

Root/source coverage: **51 browser mode strings and 49 Go source files** enumerated in `mode-inventory.json` and `path-source-inventory.json`. These inventories are source-only, not a claim that all modes ran. `ratchet-before.log` has 72 candidate write destinations; temp checkout `next-env.d.ts` entries were classified separately from actual historical evidence. The registered checker now reports zero unsafe destinations.

Full mode strings inspected (including already safe modes):

- `--browser-admin-legacy`
- `--browser-admin-shell`
- `--browser-ads-attribution`
- `--browser-buyer`
- `--browser-buyer-comms`
- `--browser-card-payments`
- `--browser-catalog-core`
- `--browser-catalog-media`
- `--browser-checkout-offline`
- `--browser-claim-checkout`
- `--browser-click-sweep`
- `--browser-customers-billing`
- `--browser-cvs`
- `--browser-design`
- `--browser-e2e`
- `--browser-home-cod`
- `--browser-identity`
- `--browser-inbox`
- `--browser-input-delivery`
- `--browser-live-claims`
- `--browser-live-console`
- `--browser-manual-order`
- `--browser-merchant-buyer`
- `--browser-merchant-orders-bff`
- `--browser-merchant-orders-ui`
- `--browser-meta-ads`
- `--browser-meta-connect`
- `--browser-meta-health-ui`
- `--browser-migration-import`
- `--browser-operations-ads`
- `--browser-ops-polish`
- `--browser-order`
- `--browser-password-auth`
- `--browser-payment`
- `--browser-picklist`
- `--browser-platform-site`
- `--browser-product-editor`
- `--browser-product-media-v2`
- `--browser-promotions`
- `--browser-refund-fulfilment`
- `--browser-reports`
- `--browser-returns-ui`
- `--browser-store-domains`
- `--browser-storefront`
- `--browser-storefront-publish`
- `--browser-studio-bff`
- `--browser-studio-ui`
- `--browser-tracking-backfill`
- `--browser-visual-lint`
- `--browser-webkit`
- `--stripe-browser`

Read-only / intentional exceptions:

- `tests/admin/shell-runner.mjs --baseline` is explicit golden maintenance, not an ordinary gate run. Baselines stay under `tests/admin/baselines/w0`.
- Historical media `before.json` remains readable via `LC_MEDIA_SIZES_BASELINE`; after output is isolated. Existing SHA/image/70% reduction/LCP comparisons remain unchanged.
- Aggregation scans downloaded artifact subdirectories for `ui-click-sweep` or `ui-visual-audit` basenames and still requires every shard. CI uploads `output/playwright/`; LATEST lives inside each run root.
- Static ratchet follows common literal/Join/template/local-alias/import-alias write sinks. It is a regression check, not a general language interpreter or security sandbox. Runtime status/hash acceptance provides the independent filesystem check.
