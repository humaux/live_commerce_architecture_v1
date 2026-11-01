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

Outcome RVFIX3-440 (relay 1): head 067f3b447 failed only gate (bundle:02) --browser-storefront (expected RED, run 41267308313). Fix: apps/storefront/app/globals.css:777-778 body .sf-btn--primary uses color var(--on-action) and hover color-mix(in srgb, var(--action) 88%, #000), same as shop.css:117. globals.css is loaded only by the storefront app, admin untouched. Unrun locally; CI decides.

Outcome RVFIX3-440 (relay 2): head bdd44efa - bundle:02 --browser-storefront now green, so the accent fix holds. Only bundle:07 failed: --browser-identity / TestBrowserSettingsWizardRealChain (admin settings wizard, unrelated to this storefront-only diff, no Playwright detail in the job log). receipts/required follow from it. No code change; re-run bundle:07. Unrun locally; CI decides.

## RVFIX4-440 — P1-1 cursor-page count, tests-only RED push (2026-11-01)

Authority: current dispatched RVFIX4-440 ruling; review `output/issue-107/review-440-8763cda7a.md` read from the read-only main checkout; current source and `contracts/storefront-v2.md` catalog list/CC06. The ruling corrects the review's parameter name to `after`. P2 items and the closed accent blocker are outside this card.

Merged `origin/r3/integration` first: fetch and merge exit 0, Already up to date (`fde11c1310fce20b576fc98023bd3003c66c205b`). Entry head: `8763cda7acdcb597c8945925133b03043c9e056d`. Role: dispatched test author (no delegation); runtime model family GPT-6 / effort not exposed. Branch: `unit/parity-buyer-browse`; assigned worktree: `.worktrees/parity-buyer-browse`.

| Finding | Change (file:line) | RED run | GREEN run / outcome |
| --- | --- | --- | --- |
| P1-1: `!next` also holds on a last cursor page; page length is not a total | `tests/storefront/buyer-browse-parity.spec.ts:310`: no-JS context searches `e`, reads the real noscript next href, asserts `after` and retained `q`, clicks it, verifies products and no further next link, then requires the exact localized count line to have count 0; repeats after refresh | Pending tests-only push; append actual run/job IDs below and in red.log after dispatch | NOT_RUN. Relay owns the `!after` guard and explanatory comment in search/page.tsx after CI proves the new assertion RED |

The existing fixture already has 24 Everyday items plus matching non-filler products, so `q=e` exceeds the default 24-card page without changing fixtures, forcing a page size, manufacturing a cursor or calling an API. Existing single-page summary and refresh assertions remain verbatim. New steps run in zh-TW, zh-CN and en at desktop/mobile widths; actual results are written by the existing CI click ledger, not claimed here.

Static evidence E1: `node --check tests/storefront/buyer-browse-parity.spec.ts`, `node scripts/dev/derive-browser-ownership.mjs` (55 modes), and `git diff --check` exit 0. Generated ownership is the only source-adjacent change. No Go dependency changes: depmap regeneration is not needed. No new imports or component harness changes.

Commit the pre-existing untracked FIX2 report byte-for-byte: SHA-256 `9a4a0d36821c4406fd4d51ad563ecf6e553131f1b397643fb17f832f166440f6`. Its historical blocker is not a current claim about this push. Allowed writes: this spec, RVFIX.md, red.log, unchanged FIX2.md and generated browser ownership. Outside-card writes: none.

Humaux search and development rules read; earlier tool-unavailable statement was corrected after broader discovery. This dispatched child uses only its own canvas element `rvfix4-440-sub-1`, without separate claim. NOT_RUN: all local tests/servers (prohibited), new CI assertions pending, product fix, GREEN, independent review, SANDBOX/LIVE external operations. E0 author expectation: current product will fail the new exact-line count-0 assertion on the last page; no observed RED is claimed yet.

RVFIX4 push 1 outcome: tests-only commit `bb0dfc2897ca661fd22bfc5eac0a2be0461eccbe` pushed successfully to existing PR #440. WAIT_RUN Gates `41275173591`; WAIT_RUN Foundation `41275173595`, foundation job `124755805971`. Expected failure is the new `cursor-page result summary` count-0 assertion; actual RED and GREEN remain NOT_RUN/pending, not inferred from older runs. The Gates plan job `124755805994` is not a test witness. Integrator relays on CI conclusion; relay must append actual RED test-job details and later GREEN run/job. This run-id paragraph and red.log append remain local/uncommitted for that relay, avoiding an evidence-only push. No merge/automerge/status posting or product fix performed.


## RVFIX4-440 relay 1 — repair RED setup before implementation (2026-11-01)

Exact-head CI evidence: `bb0dfc2897ca661fd22bfc5eac0a2be0461eccbe`, Gates `41275173591`, bundle:02 job `124756454702`; Foundation `41275173595` job `124755805971` passed. Artifact `gate-browser-storefront` (`13070849643`), `playwright/run.K2PvJNjL/storefront/20261101T051124.349362417/parity-buyer-browse-click-ledger.json` contains exactly six failed cursor-summary steps (three locales × desktop/mobile). Every failure is `locator.click: Timeout 30000ms exceeded`, with `element is not visible` for the genuine noscript link. Product cards/summary on the last page were NOT_REACHED. This is a setup failure, not the required assertion RED; earlier expected-RED wording is not evidence that it happened. Receipts/required failed downstream.

| Finding | Change (file:line) | RED run | GREEN run / outcome |
| --- | --- | --- | --- |
| Real noscript href is available but CI cannot click the hidden link | `tests/storefront/buyer-browse-parity.spec.ts:325-326`: use the exact server-issued URL for navigation, explicitly allowed by the card; ledger operation accurately says follow server href | `41275173591` / `124756454702`: setup failure, causal RED rejected; corrected tests-only push pending | NOT_RUN; keep product unchanged until count-0 fails on its own assertion |

First targeted fix attempt on this setup blocker. Retained real `after`/`q`, product-card visibility, terminal-page assertion, exact localized count-0, refresh persistence, and existing single-page assertions. No fixture, threshold or exemption changes. Repeated fetch/merge of integration before editing: exit 0, Already up to date (`fde11c1310fce20b576fc98023bd3003c66c205b`). The original RED commit already includes FIX2 unchanged; this relay does not modify it.

Authority: current relay/card, current source at bb0dfc289, frozen catalog shape in `contracts/storefront-v2.md:34-35,54,66`, and the above raw CI artifact. Architecture sections 0,1,2,26,27 and development rules `7bdb220b-0798-4b1d-aff5-16ee94cae75f` read. Humaux initial missing-tool report corrected after full discovery; rejected enumeration representative coverage 4/9, not a full inventory. Existing graph did not index loadList, so source navigation used for this isolated test setup. No recursive delegation or new task claim; continuing dispatched child `rvfix4-440-sub-1`. No out-of-card files.

Evidence status: E3 observed CI setup failure bound to bb0dfc289; corrected test E1: `node --check tests/storefront/buyer-browse-parity.spec.ts`, `node scripts/dev/derive-browser-ownership.mjs` (55 modes), and `git diff --check` all exit 0. Product fix, causal RED, GREEN, independent acceptance, all local tests/servers and LIVE actions NOT_RUN. Integrator relay remains the CI completion wake-up path.

RVFIX4-440 relay1 corrected RED push: PR440 head fdf688f9947de85d36c69df1abb9d7b3d3d68e75.
WAIT_RUN Gates: 41278013182 (current head verified; test jobs not yet dispatched).
WAIT_RUN Foundation: 41278013178, foundation job 124764043038 (in progress).
Build job 124764159641 is not an assertion witness; plan job 124764043328 is not a test run.
Prior RED run41275173591/job124756454702 was setup timeout, not count0 failure; causal RED and GREEN still pending. No implementation change.
Integrator relays CI completion per card; stop this session. No evidence-only commit; these IDs remain local and in PR comment for relay.


## RVFIX4-440 relay 2 — STOP: no causal RED; outside-card gates (2026-11-01)

Authority: current RELAY 2 / RVFIX4-440, source at PR head `fdf688f9947de85d36c69df1abb9d7b3d3d68e75`, and raw GitHub CI run `41278013182` (head verified via API). The previous relay1 WAIT_RUN paragraphs above are historical; the following completed receipts supersede their pending status.

| Finding | Change / location | RED run / job | GREEN outcome |
| --- | --- | --- | --- |
| New cursor tests still fail during setup: all 6 locale/viewport cases resolve a real last-page product-card li but it is hidden; count-0 is NOT_REACHED | No test or product edit in relay 2; failing visibility check `tests/storefront/buyer-browse-parity.spec.ts:328`, intended count-0 at :332; pending product guard `apps/storefront/app/[locale]/search/page.tsx:30` | `41278013182` / `124764810812` bundle:02; artifact `13072786797`, `playwright/run.ZU6jHBgT/storefront/20261101T060601.428026212/parity-buyer-browse-click-ledger.json`: 156 operations, 150 PASS, 6 FAIL, all cursor-page result summary / Expected visible / Received hidden / 10000ms. This is NOT accepted causal RED. | NOT_RUN; do not apply the guard before the new count assertion fails |
| Outside-card Inbox gate failure | No change; `TestBrowserInboxUIRealChain`, `INU05 (inbox-ui.spec.ts:472:1)`: 1 failed, 87 passed | Same run / `124764810973` bundle:06 | BLOCKED outside owned storefront card; causality UNKNOWN |
| Outside-card CVS WebKit gate failure | No change; `TestBrowserTaiwanCvs/WebKit` FAIL, MOCK PASS, SANDBOX SKIP | Same run / `124764811243` bundle:07 | BLOCKED outside owned card; causality UNKNOWN |
| Receipt aggregation | No change; receipts `124773781334`, required `124774203197` failed | Same run | Downstream gate failures; no test weakening |

Foundation `41278013178` / `124764043038` passed on exact PR head fdf688f9947d. This does not prove the missing cursor-count regression.

Order honored: fetched and merged latest `origin/r3/integration` (`4b8c04c7213025c173e260db6f9c81cb5b818c61`) locally before any source edit; merge brought only 7 already-integrated output reports. Those are upstream changes, not edits by this card. No merge to integration/main, no push, no new CI run. Existing FIX2.md remains byte-for-byte SHA-256 `9a4a0d36821c4406fd4d51ad563ecf6e553131f1b397643fb17f832f166440f6`; it is already committed by push 1.

STOP follows the card's explicit outside-card gate boundary, without attempting repairs in Inbox/CVS or claiming they are flakes. No additional targeted fix attempt in this relay. Potential next test setup needs integrator routing: keep no-JS context only for reading the genuine cursor, then inspect that exact URL in a JS-enabled context so server-streamed UI can render; this is a proposal, not verified or implemented.

Tools: Humaux memory/coord/code-index absent from the current callable tool inventory after targeted and broad discovery; rejected/development rules, store and canvas update NOT_RUN. No local/platform memory used. Continued dispatched child task without new claim or delegation. Local commands were source/CI inspection, git fetch/merge and static browser ownership generation only; no local tests/servers, no downloaded artifact files retained (zip parsed in memory). Evidence E3 for the observed failures only, no completion/acceptance claim. NOT_RUN: causal count-0 RED, product guard, GREEN, independent review, local tests/servers (prohibited), LIVE. WAIT_RUN: none; prior runs completed, no new push.
## RVFIX4-440 RELAY 3 readback — BLOCKED (2026-11-01)

Current raw GitHub receipts independently re-read: PR #440 head `fdf688f9947de85d36c69df1abb9d7b3d3d68e75`; Gates `41278013182` completed FAILURE; Foundation `41278013178` / job `124764043038` completed SUCCESS on that exact head. Fetch and merge of `origin/r3/integration` exit 0, already up to date; local HEAD remains `e4b8d4927136104db5b40c86e71ed711610fbec8` (unpublished prior integration merge).

| Finding | Change / file:line | RED run / job | GREEN outcome |
| --- | --- | --- | --- |
| All six cursor cases fail setup visibility; count assertion is not reached | No source change; `tests/storefront/buyer-browse-parity.spec.ts:328` precedes count-0 at :332 | `41278013182` / `124764810812`; artifact `13072786797` verified belongs to exact head, ledger 156 operations / 6 FAIL, every failure Expected visible / Received hidden | NOT_RUN; causal RED not established |
| Outside-card gates fail | No edits: Inbox INU05 `inbox-ui.spec.ts:472:1`; Taiwan CVS WebKit | Same Gates run / `124764810973`, `124764811243`; downstream receipts `124773781334`, required `124774203197` | BLOCKED under explicit outside-card STOP rule |

No further targeted fix attempt, product guard, commit, push, rerun, merge to integration, or evidence-only commit. Existing dirty report content preserved. FIX2 was already tracked before this relay; no changes to it. No out-of-card edits. WAIT_RUN: none (all existing runs completed; no new push). Integrator must route outside-card failures and authorize continued setup repair before implementation; integrator relay remains the next-action owner.

Humaux tools became discoverable after the initial inventory: corrected the missing-tool statement; read exact RVFIX4-440 history and development rules, rejected enumeration 4/9 coverage (representative subset). Only Humaux used for memory. Continued dispatched child without duplicate claim or delegation. Artifact zip parsed in memory, no retained downloads; no processes left running. E3 applies only to observed CI failures and Foundation success; author acceptance remains E0. NOT_RUN: causal count-0 RED, implementation, GREEN, independent acceptance, local tests/servers (prohibited), LIVE.

## RVFIX4-440 RELAY 4 readback — STOP (2026-11-01)

Authority: current dispatched card and raw GitHub receipts, independently re-read for PR #440 head `fdf688f9947de85d36c69df1abb9d7b3d3d68e75`. This relay repeats the same completed run as relay 3; it is not another fix attempt or assertion witness. Fetch/merge of `origin/r3/integration` (`4b8c04c7213025c173e260db6f9c81cb5b818c61`) exited 0, Already up to date; local HEAD remains unpublished merge `e4b8d4927136104db5b40c86e71ed711610fbec8`.

| Finding | Change / file:line | RED run / job | GREEN outcome |
| --- | --- | --- | --- |
| All six cursor cases fail before the intended regression assertion | No source change; `tests/storefront/buyer-browse-parity.spec.ts:328` product visibility precedes :332 count-0 | `41278013182` / `124764810812`; exact-head artifact `13072786797`, ledger 156 operations, six cursor failures Expected visible / Received hidden | Causal RED NOT_REACHED; product guard/comment and GREEN NOT_RUN |
| Outside-card Inbox INU05 and Taiwan CVS WebKit fail | No edits outside the card; raw failed job logs re-read | Same run / `124764810973`, `124764811243`; receipts `124773781334`, required `124774203197` downstream | STOP under explicit outside-card gate rule; integrator disposition needed |
| Foundation passes on exact PR head | No change | `41278013178` / `124764043038` SUCCESS | Foundation success only; not cursor-count acceptance |

No new commit, push, rerun, source/test edit, product fix, merge to integration/main, auto-merge or status post. Existing report changes preserved. FIX2 remains tracked and unchanged, SHA-256 `9a4a0d36821c4406fd4d51ad563ecf6e553131f1b397643fb17f832f166440f6`. No outside-card edits or retained temporary files/processes. Evidence E3 applies to observed CI failures only; author acceptance E0. NOT_RUN: causal count-0 RED, product fix, GREEN, independent acceptance, local tests/servers (prohibited), LIVE. WAIT_RUN: none; all existing runs completed and no new push. Integrator is the next-action owner.

Humaux became discoverable after the initial inventory; corrected the initial tool-availability statement. Read exact prior handoff `6f2db53f-d9e7-4d81-9bc7-eb69ca4d7bf6` and development rules `7bdb220b-0798-4b1d-aff5-16ee94cae75f`; rejected enumeration representative coverage 3/9, not a complete inventory. Continuing dispatched child, no duplicate claim or recursive delegation. Shared-state handoff uses an acquired coord lock and only this relay's own child canvas element; no parent canvas replacement.


## CURSOR-440 — root repair after integrator escalation (2026-11-01)

Authority: the current dispatched CURSOR ruling overrides the earlier relay statements that the hidden-card RED was invalid. The tests-only RED is accepted for the product visibility defect; the later count assertion remains NOT_REACHED in that run. Read AGENTS.md, architecture sections 0/1/2/26/27, storefront-v2 catalog list and CC06, current source and raw CI receipts. Humaux decision `5cc50f38-7f3e-452f-ae8c-b430ee605970`; development rules `7bdb220b-0798-4b1d-aff5-16ee94cae75f`; rejected representative subset 3/9, not a complete inventory. Dispatched child `cursor-440-sub-1`, no duplicate claim or delegation. Runtime: Codex, GPT-6 family; exact model/effort not exposed.

Merged `origin/r3/integration` c52b65283a5ac738d0c527a464d0cccc9523b1f4 (includes #478) before edits; merge commit 255afd0994bda7e8a83a823c7a07a03499efb537 includes the required coauthor trailer. The merge brought upstream files only. Existing uncommitted FIX1.md remains untouched. Re-read PR440 checks and raw Gates41278013182/job124764810812 after merging. That run still belongs to prior head fdf688f9947de85d36c69df1abb9d7b3d3d68e75 (tested merge c2ac3ab67f10de7027c916b5d7fb5bbcf913220a), so no post-merge GREEN is inferred. #478's Inbox/CVS changes are inherited; this card adds no repairs to those modules.

| Hop | Finding | Change / file:line |
| --- | --- | --- |
| Link generation | `lib/shop-list.ts:65-71` retains q/filter/sort/limit and sets URLSearchParams `after` exactly once | Unchanged; cursor is URL-safe, no manual encoding |
| Page parsing | `lib/shop-query.ts:22-31` reads/validates `after`; `lib/shop-list.ts:22-37` passes it through | Unchanged; q=e, sort=newest (omitted upstream), no price/collection filters, limit=24 remain the same |
| Go request | `lib/shop-upstream.ts:129-142` uses URLSearchParams `after`, and listProducts calls that path | Unchanged; same cursor and canonical filter inputs in JS-disabled and JS-enabled requests; JavaScript execution is not an input |
| Go validation | `internal/buyerhttp/catalogv2.go:156-158,179-193,243-254` binds the canonical cursor to collection/q/sort/min/max/limit | Unchanged; no mismatch found in this case |
| **Broken rendering hop** | `apps/storefront/app/[locale]/search/loading.tsx:1-6` installs the automatic page Suspense boundary; completed products require an inline-script swap to become visible. Raw artifact13072786797 shows a real product-card li repeatedly resolving hidden, not an absent product. The no-JS buyer cannot see/click the streamed page | **Remove search/loading.tsx**; `search/page.tsx:1-4` records the blocking-SSR invariant. This is the source/docs-based root diagnosis; its runtime repair awaits CI |
| Original P1-1 summary | A last cursor-page length is not the full search total | `search/page.tsx:34-35`: require both no incoming `after` and no outgoing `next` before showing count |
| User operation | Re-enable the real no-JS next-link click now that the product boundary is removed | `tests/storefront/buyer-browse-parity.spec.ts:310,325-326`: click nextLink, then assert the exact real href. Product visibility at :328 and count exclusion at :332 remain verbatim, as do reload and all locale/viewport cases |

First targeted implementation under this escalation; no test deletion, timeout increase, fixture/threshold/exemption change or gate-adapter edit. Existing tests-only RED: Gates41278013182/job124764810812, 6 hidden-card assertion failures (3 locales x 2 widths); count at332 NOT_REACHED. Earlier tests-only head bb0dfc289 / Gates41275173591/job124756454702 also failed clicking the same hidden real link. Implementation follows the current ruling accepting the visibility defect rather than seeking another RED push.

Framework mechanism: [Next loading convention](https://nextjs.org/docs/app/api-reference/file-conventions/loading) (automatic Suspense) and [streaming guide](https://nextjs.org/docs/app/guides/streaming) (inline-script DOM replacement), read 2026-11-01; repository pins Next16.3.5, React19.3.0. These explain the root inference; they are not runtime acceptance evidence. No visual redesign, copy, locale, dependency, backend, contract, migration or R2 pin changes. The same existing localized summary strings apply in zh-TW/zh-CN/en.

E1 static checks exit0: python3 scripts/check_packet.py (packet structure only), node --check tests/storefront/buyer-browse-parity.spec.ts, browser ownership regeneration and --check, git diff --check. Regenerate ownership again after the final spec edit and before commit. No Go dependency changes; depmap not changed. No new/renamed component imports, so harness allowlists are unchanged. Runtime checks only on GitHub CI; all local tests/servers forbidden and NOT_RUN.

Scope extension pre-approved for same storefront module: deletion of apps/storefront/app/[locale]/search/loading.tsx. Generated browser ownership is allowed. Output RVFIX.md/red.log contain preserved earlier relay notes plus this correction. No unrelated module edits. Risk: search waits for the catalog read instead of showing the route skeleton; this deliberately preserves no-JS access. Rollback: revert this implementation commit (restores the tracked loading.tsx and prior summary guard); no DB changes. GREEN, post-merge browser acceptance and independent review NOT_RUN. Integrator relays CI completion; no local process or downloaded artifact retained (old artifact parsed in memory).

STATUS: IMPLEMENTED_E1_PENDING_PUSH_AND_CI
