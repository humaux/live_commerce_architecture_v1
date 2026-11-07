# LANE-REPORT — PARITY-BUYER-BROWSE (ROUND 1: Tests-only RED)

Card: PARITY-BUYER-BROWSE (Round 1: tests-only RED phase, no product code changes).
Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/parity-buyer-browse`
Branch: `unit/parity-buyer-browse`
Base: `origin/r3/integration` (`faf6502b5dfed9c07327828a208b85dab2aa94db`)
Lane: OpenCode (opencode-go/kimi-k2.7-code)

## Scope and Design Authority
- Figma file `iJ9lDdFIXgTlKkt5Tvrc0X`, page `4:8829`; `output/ui-parity/FIGMA-INVENTORY.md` §3 row C7 PARITY-BUYER-BROWSE; Appendix A buckets `U:b-home`, `U:b-product`, `U:b-cart`.
- `U:b-live` (4 frames) is BLOCKED: no route yet per FIGMA-INVENTORY row C7.
- 73 frames inventoried; 69 covered by real-route assertions + parity screenshots; 4 blocked.

## Changes (tests-only RED)

| Failing job / test | Class | What you changed, file:line |
|---|---|---|
| `--browser-storefront` / `TestBrowserStorefront` (`checkBuyerBrowseParity` / `[4:9192] hero-heading: signed hero heading "歡迎來到大夢甄選"`) | (b) | `tests/storefront/buyer-browse-parity.spec.ts:291-293` — asserts signed hero heading on `/[locale]`; reproduces missing Figma home hero in un-migrated `HomeSections.tsx` |
| `--browser-storefront` / `TestBrowserStorefront` (`checkBuyerBrowseParity` / `[4:9192] hero-cta: signed CTA "立即選購"`) | (b) | `tests/storefront/buyer-browse-parity.spec.ts:294-300` — asserts signed CTA role, copy and non-teal color; reproduces legacy teal styling on storefront home |
| `--browser-storefront` / `TestBrowserStorefront` (`checkBuyerBrowseParity` / `[26:624] search-results-summary: signed search summary`) | (b) | `tests/storefront/buyer-browse-parity.spec.ts:347-349` — asserts signed search summary text and item count for "托特包"; reproduces un-migrated search results layout |
| `--browser-storefront` / `TestBrowserStorefront` (`checkBuyerBrowseParity` / `[6:19772] legacy-price-filter-gone: no legacy 最低 ( TWD ) label`) | (b) | `tests/storefront/buyer-browse-parity.spec.ts:395-398` — asserts absence of legacy price filter labels on `/[locale]/products`; reproduces un-migrated filter controls |
| `--browser-storefront` / `TestBrowserStorefront` (`checkBuyerBrowseParity` / `[4:9441] add-to-cart-button: signed add to cart button`) | (b) | `tests/storefront/buyer-browse-parity.spec.ts:440-446` — asserts non-teal primary action on product detail; reproduces legacy teal `ProductBuy.tsx` button |
| `--browser-storefront` / `TestBrowserStorefront` (`checkBuyerBrowseParity` / `[4:9668] legacy-loading-gone: no legacy 載入中 SSR text`) | (b) | `tests/storefront/buyer-browse-parity.spec.ts:494-496` — asserts absence of legacy SSR loading placeholder on `/[locale]/cart`; reproduces un-migrated cart view |
| `--browser-storefront` / `TestBrowserStorefront` (`checkBuyerBrowseParity` / `[U:b-live] blocked frames`) | (c) | No code change; `tests/storefront/buyer-browse-parity.spec.ts:239-243` records 4 `U:b-live` frames as BLOCKED (no route yet) in `parity-manifest.json` |
| `scripts/dev/test-local.sh:879` | (a) | `scripts/dev/test-local.sh:879` — registered `tests/storefront/buyer-browse-parity.spec.ts` in `--browser-storefront` `lc_prepare` |
| `tests/storefront/shop-real-gate.mjs:28,321` | (a) | `tests/storefront/shop-real-gate.mjs:28,321` — imported and invoked `checkBuyerBrowseParity` within real-stack gate execution |
| `output/ci-select-backend-browser/browser-ownership.json:728,2427-2464` | (a) | `output/ci-select-backend-browser/browser-ownership.json` — regenerated via `node scripts/dev/derive-browser-ownership.mjs` (then `--check`) linking `buyer-browse-parity.spec.ts` to `--browser-storefront` mode |

## Class Key
- (a) Registration / infrastructure / ownership changes required to run the new spec in CI.
- (b) Real-route assertions that fail because product code still renders the pre-Figma UI.
- (c) Blocked / deferred scope documented in the parity manifest (no product or test code change).

STATUS: DONE
