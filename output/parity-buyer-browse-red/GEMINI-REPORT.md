# GEMINI-REPORT — PARITY-BUYER-BROWSE (ROUND 1: Tests-only RED)

Card: PARITY-BUYER-BROWSE (Round 1: tests-only RED phase, no product code changes).
Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/parity-buyer-browse`
Branch: `unit/parity-buyer-browse`
Base: `origin/r3/integration` (`faf6502b5dfed9c07327828a208b85dab2aa94db`)

## 1. Scope and Design Authority
- **Authority**: Figma file `iJ9lDdFIXgTlKkt5Tvrc0X`, page `4:8829`; `output/ui-parity/FIGMA-INVENTORY.md` §3 row C7 PARITY-BUYER-BROWSE; Appendix A buckets `U:b-home`, `U:b-product`, `U:b-cart`, `U:b-live`.
- **Signed Delivery References**:
  - `output/figma-ui-completion-20261025/buyer-20261026/DELIVERY.md` & `FIX-1.md` (P1-1 tote substring search, P2-2 CTA contrast 14.73:1, P2-3/4 header operations, P2-5 search illustrations, P2-6 planned replay status)
  - `output/figma-ui-completion-20261025/buyer-browse-cart-20261026/DELIVERY.md` (shared cart component, 7-SKU model, sorting, quantity persistence)
  - `output/ui-parity/AUDIT-storefront.md` §2 & §3 (legacy `sf-` layout, teal `--teal` primary actions, dashed borders on disabled buttons, unstyled search/cart states)

## 2. Frame Inventory (73 Frames Total)
- **`U:b-home` (16 frames)**:
  - Desktop: `4:9192` (home), `5:15580` (terms), `6:19933` (daily series), `26:624` (search tote), `26:850` (contact), `29:641` (message sent), `782:3906` (collections overview), `1253:17135` (store paused)
  - Mobile: `4:9296` (home), `5:15658` (terms), `6:19984` (daily series), `26:921` (search tote), `26:1117` (contact), `29:670` (message sent), `782:4089` (collections overview), `1253:17159` (store paused)
- **`U:b-product` (31 frames)**:
  - Desktop: `4:9441` (caramel tote), `6:19772` (all products), `26:676` (empty search), `66:1752` (partial available), `102:2163` (crossbody), `104:2209` (canvas tote), `104:2553` (mini pouch), `104:3072` (simple tote), `104:3416` (beige tote), `104:3758` (compact pouch), `782:3907` (sort price_asc), `782:3967` (sort price_desc), `782:4027` (sort title), `1253:17090` (product removed)
  - Mobile: `4:9532` (caramel tote), `6:19859` (all products), `26:968` (empty search), `51:1380` (spec bottom-sheet), `51:1381` (details & shipping), `66:1753` (partial available), `102:2232` (crossbody), `104:2278` (canvas tote), `104:2622` (mini pouch), `104:3141` (simple tote), `104:3485` (beige tote), `104:3827` (compact pouch), `782:4090` (sort price_asc), `782:4143` (sort price_desc), `782:4196` (sort title), `782:4249` (sort sheet), `1253:17114` (product removed)
- **`U:b-cart` (22 frames)**:
  - Desktop: `4:9668` (cart), `66:1748` (already in cart), `66:1750` (cannot purchase), `102:2267` (crossbody entry), `104:2313` (canvas tote entry), `104:2657` (mini pouch entry), `104:3176` (simple tote entry), `104:3520` (beige tote entry), `104:3862` (compact pouch entry), `105:2439` (empty cart), `782:3904` (checkout context)
  - Mobile: `4:9746` (cart), `66:1749` (already in cart), `66:1751` (cannot purchase), `102:2324` (crossbody entry), `104:2371` (canvas tote entry), `104:2715` (mini pouch entry), `104:3234` (simple tote entry), `104:3578` (beige tote entry), `104:3920` (compact pouch entry), `105:2468` (empty cart), `782:4087` (checkout context)
- **`U:b-live` (4 frames) — BLOCKED (no route yet per FIGMA-INVENTORY C7)**:
  - `6:21646`, `6:21711`, `6:21771`, `6:21825` (deferred until product/routing decision).

## 3. Test Additions and Registration
1. `tests/storefront/buyer-browse-parity.spec.ts`:
   - Real-route assertions across 3 locales (`zh-TW`, `zh-CN`, `en`) and 2 viewports (Desktop `1440x900`, Mobile `390x844`).
   - Asserts signed copy, roles, controls, touch targets (>=44px), and absence of legacy elements (`.sf-hero__old`, legacy teal background `rgb(36, 121, 101)`, dashed borders on disabled buttons, `最低 ( TWD )`, legacy loading placeholder `載入中…`).
   - Captures 69 signed frame screenshots into `parity/<frame-id>.png` (colons sanitized to hyphens).
   - Generates and writes `parity-manifest.json` (73 frames, 69 covered, 4 blocked) and `parity-buyer-browse-click-ledger.json` (real clicks/assertions ledger).
2. `tests/storefront/shop-real-gate.mjs`:
   - Imports and invokes `checkBuyerBrowseParity` after `checkBuyerParityStates` in the real-stack storefront gate (`--browser-storefront`).
3. `scripts/dev/test-local.sh`:
   - Registered `test -f tests/storefront/buyer-browse-parity.spec.ts` under `--browser-storefront` `lc_prepare`.
4. `output/ci-select-backend-browser/browser-ownership.json`:
   - Regenerated via `node scripts/dev/derive-browser-ownership.mjs` (54 modes, 0 unresolved imports, verified with `--check`).

## 4. Failure Classification and Changes

| Failing job / test | Class | What you changed, file:line |
|---|---|---|
| `--browser-storefront` / `TestBrowserStorefront` (`checkBuyerBrowseParity` / `[4:9192] hero-heading: signed hero heading "歡迎來到大夢甄選"`) | (b) | `tests/storefront/buyer-browse-parity.spec.ts:168-171` — asserts signed hero heading on `/[locale]`; reproduces missing Figma home hero in un-migrated `HomeSections.tsx` |
| `--browser-storefront` / `TestBrowserStorefront` (`checkBuyerBrowseParity` / `[4:9192] hero-cta: signed CTA "立即選購"`) | (b) | `tests/storefront/buyer-browse-parity.spec.ts:172-178` — asserts signed CTA role, copy and non-teal color; reproduces legacy teal styling on storefront home |
| `--browser-storefront` / `TestBrowserStorefront` (`checkBuyerBrowseParity` / `[26:624] search-results-summary: signed search summary`) | (b) | `tests/storefront/buyer-browse-parity.spec.ts:211-213` — asserts signed search summary text and item count for "托特包"; reproduces un-migrated search results layout |
| `--browser-storefront` / `TestBrowserStorefront` (`checkBuyerBrowseParity` / `[6:19772] legacy-price-filter-gone: no legacy 最低 ( TWD ) label`) | (b) | `tests/storefront/buyer-browse-parity.spec.ts:249-252` — asserts absence of legacy price filter labels on `/[locale]/products`; reproduces un-migrated filter controls |
| `--browser-storefront` / `TestBrowserStorefront` (`checkBuyerBrowseParity` / `[4:9441] add-to-cart-button: signed add to cart button`) | (b) | `tests/storefront/buyer-browse-parity.spec.ts:286-292` — asserts non-teal primary action on product detail; reproduces legacy teal `ProductBuy.tsx` button |
| `--browser-storefront` / `TestBrowserStorefront` (`checkBuyerBrowseParity` / `[4:9668] legacy-loading-gone: no legacy 載入中 SSR text`) | (b) | `tests/storefront/buyer-browse-parity.spec.ts:327-329` — asserts absence of legacy SSR loading placeholder on `/[locale]/cart`; reproduces un-migrated cart view |
| `scripts/dev/test-local.sh:879` | (a) | `scripts/dev/test-local.sh:879` — registered `tests/storefront/buyer-browse-parity.spec.ts` in `--browser-storefront` `lc_prepare` |
| `tests/storefront/shop-real-gate.mjs:28,321` | (a) | `tests/storefront/shop-real-gate.mjs:28,321` — imported and invoked `checkBuyerBrowseParity` within real-stack gate execution |
| `output/ci-select-backend-browser/browser-ownership.json:1,728,2427` | (a) | `output/ci-select-backend-browser/browser-ownership.json` — regenerated ownership map linking `buyer-browse-parity.spec.ts` to `--browser-storefront` mode |

STATUS: DONE
