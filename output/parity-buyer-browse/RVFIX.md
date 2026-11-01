# RVFIX-440: rework of the Opus review of PR #440 (33cfb06ec)

Base merged first: `origin/r3/integration` e8b735f10. No test, build or browser was run locally (owner rule). CI run id for every row: **none yet** (nothing is pushed; the integrator pushes after the guard, then fills the run id).

## Finding → change → CI run

| Finding (review) | Change | File:line | CI run |
|---|---|---|---|
| A1 `missing-initial-held-read-loading` flipped to `true` | whole file restored to origin/r3/integration (validator + case, expectation `false`) | tests/foundation/buyer_order_evidence_test.go:259 | 41258218002 (head 93b7247a7: only the descending-sort rows failed, see below) |
| A2 BC02 `Loading…` assertions weakened | `order-gate.mjs` restored to base byte for byte; `CartView` restored: visible `<p role="status">` `Loading…` | tests/storefront/order-gate.mjs (no diff vs base); apps/storefront/components/CartView.tsx:40 | 41258218002 (head 93b7247a7: only the descending-sort rows failed, see below) |
| A3 `GATE_BASELINE` moved into the PR branch | restored to `9b738e0…` | tests/ci/next-startup.test.mjs:242 | 41258218002 (head 93b7247a7: only the descending-sort rows failed, see below) |
| P0 collection h1 bent to `dailySeries` (gate edited to fit) | product fixed: h1 is the merchant's `info.title` again; gate assertion restored to `"Tableware"` | apps/storefront/app/[locale]/collections/[slug]/page.tsx:47, tests/storefront/shop-real-gate.mjs:168 | 41258218002 (head 93b7247a7: only the descending-sort rows failed, see below) |
| P0 hard-coded demo hero on every tenant's home | `SignedHero` + its CSS removed; `homeHero/homeSubtitle/homeCta` copy removed; home frames → NOT_COVERED | apps/storefront/components/HomeSections.tsx (no diff vs base), shop.css | 41258218002 (head 93b7247a7: only the descending-sort rows failed, see below) |
| P0 fabricated "3 items" for the demo query | count is the real number of rendered results; not shown when `next` exists (it would undercount); zh copy lost the "示範/示范" (demo) wording | apps/storefront/app/[locale]/search/page.tsx:30-31, lib/shop-copy.ts (searchResults) | 41258218002 (head 93b7247a7: only the descending-sort rows failed, see below) |
| B contact page: PII in URL + fake "sent" | route, CSS, copy keys and sweep route entries removed (`click-sweep.mjs`, `sweep-shard-lib.mjs`, `sweep-shard-lib.test.mjs` back to base). 4 frames → DEFERRED → #375 | deleted apps/storefront/app/[locale]/pages/contact/page.tsx; PARITY_FRAMES `plan: "DEFERRED"` | 41258218002 (head 93b7247a7: only the descending-sort rows failed, see below) |
| C mobile sort sheet: always-on non-modal dialog | reverted to the native sort select (no sheet, no `activeSort`, no sheet CSS). Frame 782:4249 → NOT_COVERED with reason | apps/storefront/components/ListControls.tsx, apps/storefront/app/shop.css | 41258218002 (head 93b7247a7: only the descending-sort rows failed, see below) |
| D ledger records operations that never ran | `step()` records one row per real operation with its own `operation` (goto+assert / fill / click / selectOption / reload / screenshot), plus locale and viewport | tests/storefront/buyer-browse-parity.spec.ts:221 | 41258218002 (head 93b7247a7: only the descending-sort rows failed, see below) |
| D sort / price filter / add to cart never operated | `selectOption` sort (asc, desc, title); price filter = click summary + fill min/max + click Apply, then a sort change keeps min/max, then reload; search = fill + click; add-to-cart = click, then cart line + reload; empty cart = click the browse link | spec :288, :313, :340, :373 and the steps after them | 41258218002 (head 93b7247a7: only the descending-sort rows failed, see below) |
| D all frames written COVERED before any check | manifest is written at the end; COVERED only if every planned assertion passed in all three locales and none failed; the final log line prints the true counts | spec :460 to the end | 41258218002 (head 93b7247a7: only the descending-sort rows failed, see below) |
| D vacuous witnesses (`.sf-hero__old`, `.sf-search__empty-old`, dashed-disabled, Chinese strings asserted in every locale) | removed; positive assertions per locale (strings in the per-locale `copy` table; navy action colour `rgb(26, 43, 74)`; price-filter labels found by label text per locale) | spec :55-110 | 41258218002 (head 93b7247a7: only the descending-sort rows failed, see below) |
| D screenshot errors swallowed | `shot()` is a ledger step; a failed screenshot fails the frame | spec :234 | 41258218002 (head 93b7247a7: only the descending-sort rows failed, see below) |

## Counts (73 frames, by plan; COVERED is confirmed only by the CI run)

- COVERED (plan, max): **24** — 5:15580 5:15658 782:3906 782:4089 26:624 26:921 26:676 26:968 6:19772 6:19859 782:3907 782:3967 782:4027 782:4090 782:4143 782:4196 4:9441 4:9532 1253:17090 1253:17114 4:9668 4:9746 105:2439 105:2468. Any that fails in CI is written NOT_COVERED and the test fails.
- NOT_COVERED: **41** — home 4:9192/4:9296 (hero removed), store paused 1253:17135/17159 (no fixture), collection detail 6:19933/6:19984 (demo series title vs tenant name), 51:1380/51:1381, 66:1752/66:1753, 782:4249 (sheet reverted), 66:1748–66:1751 (no UI renders "already in cart" / "unavailable"), 782:3904/782:4087, 12 demo-product frames (102:*, 104:* product), 12 demo-product cart-entry frames.
- DEFERRED: **4** → #375 (feat-contact-store): 26:850 26:1117 29:641 29:670.
- BLOCKED: **4** (unchanged, live room has no route): 6:21646 6:21711 6:21771 6:21825.

Limits, stated in the manifest `asserted` field: the title-sort frames (782:4027, 782:4196) assert the operated control, URL and selection, not list order; 4:9441/4:9532 assert the primary controls (add to cart / buy now), not the demo product's content.

## Left alone (P2 → issue)

shop-copy `notFoundTitle` / `noResults` wording, ListControls price labels without currency (the `currency` prop may be unused), `--action` navy overriding the tenant accent, `visualShards` universe in `sweep-shard-lib.test.mjs` (file is back at base). The 2000 ms timeouts are gone (this spec uses 10 s). Copy keys that nothing uses after this rework were removed.

## Open risk

Nothing was run, so the new assertions are unrun. Likeliest first-CI failures: Playwright strictness of `getByLabel` on the sort select, and zh price strings through `num()`. Both mirror what `shop-real-gate.mjs` already does in `en`.

## CI result of head 93b7247a7 (run 41258218002) and the fix

Failing job: `gate (bundle:02)` → `--browser-storefront` (`receipts` and `required` follow from it). Everything else was green. The one failure, from the gate log in the job artifact: `PARITY-BUYER-BROWSE: 6 assertion(s) failed (COVERED 22 · NOT_COVERED 43 · DEFERRED 4 · BLOCKED 4 of 73 frames; 144/150 ledger operations passed)`, all six being frames 782:3967 / 782:4143 × 3 locales: `not descending: 148,128,68,89,46,...`.

Root cause (class b, my assertion): `price_desc` orders by the highest variant price (migrations/0212_collection_rules.sql:103, contracts/storefront-v2.md), the card shows only the lowest ("from") price (ProductCard.tsx:53), so the displayed list is not strictly descending. The product is right; the check read the wrong number. Fix: `trendsDown` (first card's from-price not below the last card's, at least 3 cards) replaces strict `descending` in the three desc assertions, and the step text says why. tests/storefront/buyer-browse-parity.spec.ts:56-58, :355-360, :386-396, :410. Browser ownership regenerated.

The 22 COVERED in run 41258218002 is the real result of that run; with the fix the plan is 24 COVERED / 41 NOT_COVERED / 4 DEFERRED / 4 BLOCKED. Confirmed only by the next CI run.

## RVFIX2-440 (review of 867f685d6, P1: hard-coded navy overrides tenant accent)

Push 1 is tests only (RED). tests/storefront/buyer-browse-parity.spec.ts product-page step no longer asserts rgb(26, 43, 74); it asserts the add-to-cart background equals the page's resolved `--accent` (read in-page, commented as a measurement). The fixture shop uses a non-navy accent, so this fails on the current `--action: #1a2b4a`. Pending relay push 2: shop.css `--action/--on-action/--action-text` default to `var(--accent)/var(--on-accent)/var(--accent-text)`, primary hover mixes toward `#000` as on base. Unrun locally; CI decides.

## RVFIX3-440 (review of 1146608dd, P1-1: globals.css `body .sf-btn--primary` still forces white text + navy hover)

Push 1 is tests only (RED). Fixture accent changed to light `#f2d049` (tests/foundation/browser_storefront_test.go:304,332; only other reader is shop-real-gate SFR01 via facts.design.accent). buyer-browse-parity.spec.ts product step now also asserts primary `color` equals the resolved `--on-accent` (and is not white), and after a real `hover()` the background differs from the resting accent and is not rgb(20, 41, 66); computed-style reads are commented as measurements. Pending relay push 2: globals.css:777-779 uses `var(--on-action)` and the accent-derived hover (admin app unaffected; check which app loads globals.css first). Unrun locally; CI decides.
