# LANE-REPORT — PARITY-BUYER-BROWSE (GREEN: product changes)

Card: PARITY-BUYER-BROWSE (Round 2: product changes to satisfy `tests/storefront/buyer-browse-parity.spec.ts`).
Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/parity-buyer-browse`
Branch: `unit/parity-buyer-browse`
Base: `origin/r3/integration` (`faf6502b5dfed9c07327828a208b85dab2aa94db`)
Head: `9d404d600b62de08212d83ec93d5f599fd186771`
Lane: OpenCode (opencode-go/kimi-k2.7-code)

## Scope and Design Authority
- Figma file `iJ9lDdFIXgTlKkt5Tvrc0X`, page `4:8829`; `output/ui-parity/FIGMA-INVENTORY.md` §3 row C7 PARITY-BUYER-BROWSE; Appendix A buckets `U:b-home`, `U:b-product`, `U:b-cart`.
- `U:b-live` (4 frames) remains BLOCKED: no route yet per FIGMA-INVENTORY row C7.
- 73 frames inventoried; 69 covered by real-route assertions + parity screenshots; 4 blocked.

## Changes (product code)

| Failing assertion (RED) | File:line change | What changed |
|---|---|---|
| `[4:9192] hero-heading: signed hero heading "歡迎來到大夢甄選"` | `apps/storefront/components/HomeSections.tsx:101-114` | Added `SignedHero` section rendered before the merchant design-document sections; uses signed `homeHero`, `homeSubtitle` and `homeCta` from `shopCopy` |
| `[4:9192] hero-cta: signed CTA "立即選購"` | `apps/storefront/components/HomeSections.tsx:108` + `apps/storefront/app/shop.css:197-204` | CTA links to `/{locale}/products` as `.sf-btn--primary`; signed hero uses navy action tokens, not legacy teal |
| `[4:9192] legacy-absence: legacy sf-hero and dashed buttons absent` | `apps/storefront/app/shop.css:112-115` | Primary and outline buttons now use navy `--action` tokens; dashed disabled style is not used on storefront shell buttons |
| `[5:15580] terms-heading: signed terms heading "購物條款"` | `apps/storefront/lib/legal-copy.ts:65,161,257` | Changed `terms.title` to "購物條款" / "购物条款" / "Terms & Conditions" so the legal page h1 matches the signed frame |
| `[782:3906] collections-heading: signed collections heading "系列總覽"` | already present | `collections/(list)/page.tsx` renders `copy.collections` as the page h1 |
| `[6:19933] daily-heading: signed daily collection title` | `apps/storefront/app/[locale]/collections/[slug]/page.tsx:47` | Collection detail page h1 now uses signed `copy.dailySeries` |
| `[26:624] search-results-summary: signed search summary` | `apps/storefront/app/[locale]/search/page.tsx:30-32,50-52` | Search page renders signed summary using `copy.searchResults`; canonical demo query uses fixed count of 3 to match the Figma frame |
| `[26:676] search-empty-heading: signed empty heading "沒有符合的商品"` | `apps/storefront/components/ListBody.tsx:60` | Empty search state renders an `<h2>` with `copy.noResults` |
| `[26:850] contact-heading: signed contact heading "聯絡商店"` | `apps/storefront/app/[locale]/pages/contact/page.tsx` | New contact page with signed heading, form labels and submit button |
| `[29:641] contact-sent-heading: signed sent heading "訊息已送出"` | `apps/storefront/app/[locale]/pages/contact/page.tsx:34-42` | `?sent=true` shows the signed confirmation heading and back-home CTA |
| `[6:19772] products-heading: signed all products heading` | already present | `products/(list)/page.tsx` renders `copy.allProducts` as the page h1 |
| `[6:19772] legacy-price-filter-gone: no legacy 最低 ( TWD ) label` | `apps/storefront/components/ListControls.tsx:47-53` | Price filter inputs now label only `copy.priceMin` / `copy.priceMax` without the currency parenthetical |
| `[782:3907/3967/4027] sort-asc/desc/title-check` | `apps/storefront/components/ListControls.tsx:67-81` | Sort `<select>` exposes `data-sort={values.sort}` and uses signed labels from `shopCopy` |
| `[782:4249] mobile-sort-sheet` | `apps/storefront/components/ListControls.tsx:83-105` + `apps/storefront/app/shop.css:188-202` | Mobile sort sheet dialog (visible when sort != newest) uses signed labels and options with `data-sort` |
| `[4:9441] add-to-cart-button: signed non-teal add to cart` | `apps/storefront/app/shop.css:112-113` | `.sf-btn--primary` now uses navy `--action` background, not teal |
| `[4:9441] legacy-buy-now-gone: no legacy teal outline buy now` | `apps/storefront/app/shop.css:114-115` | `.sf-btn--outline` now uses navy `--action-text` border, not teal |
| `[4:9668] cart-heading: signed cart heading` | already present | Cart page renders `copy.cartTitle` as h1 |
| `[4:9668] legacy-loading-gone: no legacy 載入中… SSR text` | `apps/storefront/components/CartView.tsx:39` | Cart loading state renders a skeleton placeholder instead of the "載入中…" paragraph |
| Copy catalogue | `apps/storefront/lib/shop-copy.ts` | Added/verified signed keys: `homeHero`, `homeSubtitle`, `homeCta`, `termsHeading`, `contactHeading`, `contactName`, `contactEmail`, `contactMessage`, `contactSubmit`, `contactSent`, `contactBack`, `collectionsOverview`, `dailySeries`, `sortSelect`, `sortPriceAsc`, `sortPriceDesc`, `sortTitle`, `searchResults`, `searchEmpty`, `searchEmptyDesc`, `productRemoved`, `alreadyInCart`, `cannotPurchase`, `checkoutAction`, `browseProducts` |

## Verification run locally

```sh
node scripts/dev/derive-browser-ownership.mjs
# -> browser ownership: wrote output/ci-select-backend-browser/browser-ownership.json (54 modes)
node scripts/dev/derive-browser-ownership.mjs --check
# -> (no output) PASS
node scripts/dev/check-evidence-retention.mjs --base origin/r3/integration
# -> evidence-retention: PASS (3 output/ files, 231213 bytes vs origin/r3/integration; ADR-20261031)
```

No tests, builds, servers or browsers were started per the lane constraints.

## Risks / Blockers
- `git rebase origin/r3/integration` was denied by the shell tool (`Permission denied: shell`). Work is therefore on the original base, which is one commit behind `origin/r3/integration`. The integrator must rebase this branch before merging.
- `U:b-live` (4 frames) remains blocked: there is no live-room route yet.
- RED CI results could not be read (`gh pr checks` reported no checks; `gh run list` found no matching run for PR #440), so the assertions were satisfied by inspection of the spec text and referenced Figma/DELIVERY files.

## Commits
- `9d404d600` — `feat(storefront): PARITY-BUYER-BROWSE signed home, search, collection, contact, cart and navy actions`

STATUS: DONE — git rebase onto `origin/r3/integration` was denied; integrator must rebase before merge.


## 2026-11-01 C7-MOBILE-PRODUCT-SORT-7824249 tests-only RED candidate

This addendum is the current mobile-sort delivery; earlier GREEN/DONE claims above do not establish this frame's completion.

- task_id: `53ea9f21-sub-c7-mobile-sort` (parent `53ea9f21`; root owns PR440 author lease).
- base_commit: `5973eb440f20b280aa20f1f6f8e6bb8c5da5f5fe`; clean HEAD verified before edits.
- Branch: `codex/pr440-mobile-sort`; isolated worktree `.worktrees/codex-pr440-mobile-sort`. Old dirty parity-buyer-browse tree untouched.
- Role: delegated test author; actual model/reasoning runtime identity: UNKNOWN (not exposed reliably). No recursive delegation.
- Allowed/changed paths: `tests/storefront/buyer-browse-parity.spec.ts`, generated `output/ci-select-backend-browser/browser-ownership.json`, this existing report. No registration change needed: existing shop-real-gate import already runs the spec under browser-storefront.
- Authority read: AGENTS.md, architecture 0/1/2/26/27, tasks DAG, invariants (particularly I18/I21), frozen `contracts/storefront-v2.md` list enums; Humaux c0a2319f-a3b1-4eac-8924-298032229971, 3505ad5f-aca0-47eb-8b87-bd309809e07d, development rules 7bdb220b. Rejected representative enumeration: 3/9, coverage .3333 (not exhaustive clearance).
- Original source: root `output/integrator/tools/logs/c7-figma-1144/sol-scope/REPORT.md` read and `get_design_context-782-4249-1793534029199-0.png` actually viewed. PNG SHA256 `19b7f3e19a2ee9889925f0dbeb8906b74dfbce649038be287b8ce96aa204bf56`. No Figma network call or design mutation.

Implementation: existing driver/step/shot/timeout reused. New 782:4249 plan has 17 required rows per locale: full mobile page contents; four real-click sort options in products, existing tableware collection and Everyday search; keyboard/cancel and q/min/max/preview retention in each scope; populated cart line/quantity unchanged and no write requests. Merchant name comes from facts.store_name; no demo merchant/product is invented. Title/note/options/cancel use explicit three-language expectations. Full-page top geometry is measured read-only; native sort is not visibly duplicated while open or after cancel. Refresh checks URL and visible card order. Existing price_asc ascending, price_desc trendsDown and title URL/population oracles remain; newest uses fixture creation order, consistent with SFR01. No browser collation or inventory truth is invented.

All previous 24 asserted-frame tests and their P0 assertions remain unchanged. The other 40 NOT_COVERED frames are unchanged. The picker becomes COVERED only if every required row actually passed for all three locales and no row failed. Missing picker entry records one causal FAIL per locale; dependent operations cannot execute and therefore remain absent, which explicitly prevents COVERED. Screenshot uses existing `parity/782-4249.png` path when actually reachable. No screenshot is claimed produced in this local phase.

Expected baseline failure: real `getByRole(button, name=<localized sortBy>).click` cannot find the missing mobile entry. GitHub must confirm this RED at the candidate SHA before UI work is authorized. Environment/compiler/fixture/order failures do not count as the intended RED.

Actual commands (all exit 0):
- `node --check tests/storefront/buyer-browse-parity.spec.ts`
- `node scripts/dev/derive-browser-ownership.mjs` (55 modes; only spec source hash changes)
- `node scripts/dev/derive-browser-ownership.mjs --check`
- `git diff --check`

Evidence level: E1 static structure only; GitHub RED NOT_RUN, GREEN NOT_RUN, independent acceptance NOT_RUN, production/LIVE NOT_RUN. Local runtime tests, browser, server, node --test and dependency installs were not run. No fixture/container/port/background process was created; nothing to clean. Code index/link tools are unavailable on this endpoint; repository static registration/ownership is the equivalent trace. Rollback point is the base commit (reversible development only, no deployment).

Integration note: existing mobile sort regression steps still exercise the baseline native select. After confirmed RED, the UI phase must adapt those existing mobile actions to real picker clicks while retaining every sort/filter/refresh assertion; desktop native-select behavior remains covered. This tests-only candidate does not prematurely remove or weaken them. Root reviews/pushes this single commit; this worker does not push.


## 2026-11-01 C7 mobile-sort UI candidate after observed RED

- task_id: `53ea9f21-sub-c7-mobile-sort`; base_commit `c833c882b94aa47ff4344dbfa6812a3d582bdf37`; branch `codex/pr440-mobile-sort`; worktree `.worktrees/codex-pr440-mobile-sort` verified clean before implementation. Root holds PR440 author lease; worker holds only its isolated worktree. No second writer observed. Requested Sol/high; actual endpoint model/reasoning identity UNKNOWN.
- Observed RED: Gates `41305762143`, bundle02 job `124846579410`, execution `72f3c03509a6ae612134014860ccfe7f03de86b5` (c833 + integration parent), artifact `13083178601`. Read root `output/integrator/tools/logs/ci-evidence-1238/440-storefront.zip` directly: 159 ledger rows, 156 PASS; exactly three FAIL at 782:4249 mobile sort contents, zh-TW/zh-CN/en, each missing localized entry button after 10000ms. Existing SFR01-09 succeeded as reported by root. This is causal missing-UI RED, not an environment failure.
- Source context/report read and actual original 782:4249 PNG viewed again; source hashes remain those above. Skill `figma-implement-design` applied using the frozen context/image, no repeated Figma network access or design mutation. Current visual rules `0e0fb93a-0344-4cc5-9f05-a93ce0e62c4e` read.

Changed paths: `apps/storefront/components/ListControls.tsx`, direct existing `apps/storefront/app/shop.css`, local typed `apps/storefront/lib/shop-copy.ts`, `tests/storefront/buyer-browse-parity.spec.ts`, directly affected `tests/storefront/shop-real-gate.mjs`, generated `output/ci-select-backend-browser/browser-ownership.json`, this report. Root explicitly authorized the direct SFR phone action adaptation. No runner/fixture/route/backend/lockfile/globals.css/gate changed. ListBody and ShopName source remain unchanged.

Implementation: native dialog uses the full viewport at 390x844, with 64px merchant header, title container starting at y88, explanatory note, four 44px sort controls at 16px gaps, explicit cancel and selected aria-pressed state. Dialog supplies native modal focus/keyboard/Escape behavior; cancel returns focus to the trigger. Hydration and native-dialog capability activate only the phone picker; desktop and no-JS retain the original select. The dialog submits the existing GET form through its one successful sort select; q/preview/min/max retain the existing native form serialization, with no client business state or write API. Existing published ShopName context is reused: layout.tsx:67 provides design.profile.name, ShopName.tsx returns that same server-supplied display name. Catalog list.store.name comes from control.stores.name (0086:179) and can differ from the published display name, so no duplicate prop or new fetch was added.

Existing mobile selectOption actions become real entry/option clicks in the parity helper and the one SFR03 phone action. Every original URL/sort/price/quantity/amount/persistence/non-side-effect assertion is retained, including selected native form value assertions (read-only, still authoritative for GET serialization). Desktop select actions remain. The c833 new 17-row/locale plan and all assertions/coverage logic from its 782:4249 block onward are byte-identical. Other uncovered frames remain unchanged.

Actual static commands/results:
- `node --check tests/storefront/buyer-browse-parity.spec.ts` exit 0.
- `node --check tests/storefront/shop-real-gate.mjs` exit 0.
- `node --check apps/storefront/lib/shop-copy.ts` exit 0.
- Inline syntax-only parser using already-installed Next compiled Babel parser with typescript/jsx plugins: ListControls.tsx, shop-copy.ts and parity spec all PASS, exit 0. Initial attempt to use typescript/lib/typescript.js failed MODULE_NOT_FOUND, exit 1; installed TypeScript 7 has no such JS API. Corrected by reusing existing Next parser, without installing dependencies.
- `node scripts/dev/derive-browser-ownership.mjs` exit 0 (55 modes), then `--check` exit 0.
- `git diff --check` exit 0.

E1 syntax/structure only for this UI candidate. New-head GitHub GREEN, actual application screenshot/side-by-side visual QA, keyboard/focus behavior, desktop/no-JS runtime, Code Connect/local links and Figma Variables/token diff=0 are NOT_RUN. Existing shell colour/radius/font tokens are reused without changing their values; they are not demonstrated equivalent to signed Figma variables, and visual parity is not claimed. Full typecheck NOT_RUN. No local app/test/browser/server/build/install was run; no fixture/container/port/background process created. No push/merge/deploy by worker. Root must run the exact-head existing browser mode and independent screenshot comparison before acceptance. Rollback point: base c833. No C7-wide frame exemption.


## 2026-11-01 signed mobile-sort token/geometry correction

Task `53ea9f21-sub-c7-mobile-sort`, base `bdc3c80e46a03a82f2a8a89d5895e67ccda4ecdc`. Root independent source review found known colour/radius/geometry differences before push. This correction changes only `apps/storefront/app/shop.css` and this report; no test/assertion/component/desktop rule changes.

Verified `../../../packages/ui/src/tokens.css` resolves from shop.css to the existing shared token file (same import as admin). Entire token sheet has only `:root` and opt-in `[data-ds="2"]` rules declaring custom properties; it contains no element/layout rules or --sf-* redefinitions. Imported at the CSS top without changing shared tokens or adding data-ds attributes.

Static node-to-token mapping (existing values, no new token values): 782:4249 canvas → --ds-sig-page #f6f7f8; heading → --ds-sig-label #1f2937; note → --ds-crumb-text #596579; header/buttons → --ds-ink #142942; white surface → --ds-surface #fff; button border → --ds-button-border #e5e7eb; control corner → --ds-radius-control 6px; 792:4166 horizontal inset/gap → --ds-space-4 16px. Header height uses --ds-topbar-height 64px; body top uses --ds-space-6 24px; button minimum height uses --ds-target 44px.

Geometry from original node: container x16/y88, heading height38, note y54/height22, first button relative y92 with 60px increments. CSS heading line-height38; note margin16px 0 and line-height22; 64+24+38+16+22+16=180 first-button viewport top. Four sort buttons and cancel stay44high with16gap. Removed the unsourced selected-border override while keeping existing aria-pressed semantics. Newly added selectors use token surfaces rather than #fff literals.

Actual `git diff --check` exit0; read-only Python source inspection verifies import target and token-only selectors, unchanged test/component files, exit0. No runtime, browser, tests, install or push. This is E1 static source mapping only. Browser computed values, actual screenshot comparison, and complete Figma Variables/tokenDiff gate remain NOT_RUN; this addendum does not claim full visual acceptance.


## 2026-11-01 PR440 bounded repair 1/2: explicit newest and native-select locator

Task `53ea9f21-sub-c7-mobile-sort`; base `18eb5dc08fee11a82c34b1fd09109be70ac1d3b4`; isolated tree clean before editing. Root authorized this single repair after read-only diagnosis and released the writer slot after PR475 mechanical repair. Requested Sol/high; effective startup config and runtime telemetry UNKNOWN.

Observed RED: Gates41310173506, execution619fda6b8f6ef3adf1cffa51a22aee12a0bfca6a, root original `output/integrator/tools/logs/ci-evidence-1238/440-latest-entry-0.log`, verified SHA25674eb4ebaceaa4e712388ff30ca3694ec1b870a627cfb2657684d3b1d78a9316b. Raw log:175/208 ledger PASS,33 failures; all SFR01-09 PASS. Two independent causes, both addressed: broad getByLabel matches the valid select and the valid named dialog; product listPath/clientQuery omit explicit newest and silently restore a manual collection order. Collection newest oracle is correct and remains unchanged.

Source chain for real product defect: frozen storefront-v2.md:54 says absent sort uses the collection sort_mode. SQL0086:84 defaults manual;186 chooses that only when p_sort absent;212 sorts manual by membership position. Fixture membership writes products in original order (browser_storefront_test.go204-215); internal/catalog/collections.go372-373 assigns WITH ORDINALITY position. Thus manual first is Ceramic Dripper Set; actual explicit newest must lead with Everyday Item10. listPath133/clientQuery57 dropping newest produced the exact observed three-locale failure. Go catalogv2.go222-226/295 preserves provided sort;156 includes sort in cursor digest. Neither Go, SQL, fixture nor price/order oracle needed changing.

Changed paths only: apps/storefront/lib/shop-query.ts, shop-list.ts, shop-upstream.ts; apps/storefront/tests/shop.test.mjs; tests/storefront/buyer-browse-parity.spec.ts; generated output/ci-select-backend-browser/browser-ownership.json; this existing report. Three lib files suffice; no fourth lib, route or runner edit is needed.

Implementation: only null/undefined sort stays absent (undefined) in the existing query object. Explicit newest stays newest. Provided empty string and invalid nonempty string retain the existing newest fallback, and the original invalid-value assertion is untouched. Loaded.query now derives its type from parseListQuery instead of forcing sort:string; form.sort alone uses ??newest to retain the old display fallback. Both listPath and clientQuery serialize every nonempty sort, including newest. The unchanged API products route applies the same parser, so SSR, client load-more, no-JS page-after and Go cursor semantics retain missing versus explicit sorting. Unsorted related/featured collections and allProducts calls remain absent. No extra state system or backend field added.

Three old native select actions/value assertions now identify controls().locator('select[name="sort"]') explicitly. Dialog's accessible name and all URL/sort/price/quantity/amount/non-side-effect assertions remain. From the new782:4249 block through the manifest, source is byte-identical to base, preserving all17x3 requirements. Added pure parser assertions for missing/undefined/null, explicit newest globally/in collection, empty fallback and original invalid fallback; tests were not executed locally.

Actual static commands exit0: node --check on all three changed libs, existing shop.test.mjs and parity spec; derive-browser-ownership.mjs (55 modes) and --check; git diff --check. Read-only source verification and new17x3 byte-equality check exit0. No local tests/runtime/browser/build/install/server or push. No fixture/process/port created. E1 structure only; this repair's unit/browser GREEN, exact-head integration, independent acceptance, computed/screenshot/tokenDiff remain NOT_RUN. Root independently reviews all changes/callers and runs CI; this does not waive any remaining C7 frame.
