# product-ui-v2 — final handoff / PG contract ruling required

Scope: `unit/product-ui-v2`, only this worktree. Base `f7e6c843`. No push, deployment, production access or product Go/SQL changes.

Final tested source: `c527c77414a1795ecd73590d7a2cbaa19b6a752c`. Implementation and all six requested browser modes PASS. Of 13 command gates below, 12 exit 0; focused PG exits 1 on one unchanged image-schema assertion. **NOT_RELEASE_READY** until that contract conflict is adjudicated. No remaining known P0/P1 in the scoped UI implementation after independent review.

Delivery ownership: root Codex is sole writer, host model/reasoning not exposed by this runtime. Both independent reviewers are read-only `explorer`, `gpt-6-luna/high`; base/worktree match this unit, no recursive delegation or other worktree writes. Code paths are product-specific admin components/libs/BFF, existing browser drivers, two Go browser fixture tests, gate scripts/docs; no migrations or production backend implementation changes.

## Commits

- `61ab5dac`: authorized merge of `unit/product-ui`; W0 shell/registry/guards and existing gate modes retained.
- `79c6243e`: §g minimal merge-patch, per-SKU shipping, collection membership, stock targets including zero, single-SKU inline / multi-SKU dialog, media editor integration.
- `5bbaec7f`: quick editor border token fallback.
- `90e33089`: mobile select-all remains reachable; translated active-to-draft confirmation.
- `a92b5a5c`: wait for warehouse/collection reads; unknown on-hand cannot be treated as a zero delta baseline.
- `5337047a`: real browser drivers, isolated TWD PE fixture plus unchanged USD CC12 assertions, exact four-stage receipt audit, PE14 model regressions.
- `c527c774`: W0 navigation/quick-edit context guards, stale-lock cleanup, single hard-navigation confirmation and viewport-aligned screenshots; first-red negative assertion retained in evidence.

## Implementation

- W0 owns store switching. Product list has no duplicate store selector or unsupported smart filters. Money/time use existing packages/format re-exports.
- Edit diffs authoritative detail: one PUT command, changed fields only; explicit clears and zero are retained. Unmentioned SKU fields are not reconstructed.
- Existing media uses the existing photo writer. Create retains draft → uploads → reorder → publish sequence.
- Inventory uses on_hand, never available as a target. Ambiguous warehouse disables target editing and links to inventory. Newly generated edit SKUs start with opening quantity 0.
- Live-window refusal copy is explicit in all three languages. Archive confirmation is shared by normal save and unpublish.
- Saved status is synchronized; editing an archived product does not silently unarchive it.
- Collections clear/restore and per-SKU shipping have real UI write + reload assertions.
- Single-SKU quick edit is inside its table cell; multiple SKUs use a dialog. Pending quick edits lock competing list writes; store changes clear quick context.
- Shell navigation and page links share the editor leave guard; quick edits lock shell switching. Dirty edits can be explicitly discarded, while in-flight/UNKNOWN application navigation is blocked. Unmount clears stale parent locks. Browser history/address-bar/forced unload are not claimed to be trapped: the persistent receipt fence remains fail-closed and requires reconciliation after re-entry.

## Final command gates

All commands ran in this worktree. Browser commands use the shared machine lock (`LC_TEST_LOCK_WAIT=7200`); no lock bypass. Evidence tier: local production Next builds + real Go/isolated PostgreSQL + MOCK identity, not external provider or LIVE deployment acceptance.

| Command | Exit | Result |
|---|---:|---|
| go build ./... | 0 | go-build-final.log |
| go vet ./... | 0 | go-vet-final.log |
| git ls-files '*.go' \| xargs gofmt -l | 0 | empty gofmt-all.log (all tracked Go files) |
| bash scripts/dev/test-node.sh | 0 | 322 tests across three suites; unrelated live runner NOT_RUN |
| pnpm --filter admin exec tsc --noEmit | 0 | admin-tsc-final.log |
| bash scripts/dev/check-gates.sh | 0 | 59 modes, G-UI1/3/5 |
| bash scripts/dev/test-focused.sh '^(TestProductEditor\|TestCatalog)' | 1 | 58 PASS / 1 FAIL / 0 SKIP; schema conflict below |
| bash scripts/dev/test-local.sh --browser-catalog-core | 0 | catalog-core-final-r4.log: 2 browser cases plus exact PG readback; original assertions retained |
| bash scripts/dev/test-local.sh --browser-catalog-media | 0 | catalog-media-final-r4.log: 20 cases, four locale/viewport cells; exact PG readback |
| bash scripts/dev/test-local.sh --browser-admin-shell | 0 | admin-shell-final-r4.log: 24 matrix cases, role negative, store switching, axe |
| bash scripts/dev/test-local.sh --browser-product-editor | 0 | product-editor-final-r4.log: PE12–17 TWD (1 Playwright case), followed by both frozen CC12 USD cases and exact PG readbacks |
| bash scripts/dev/test-local.sh --browser-merchant-buyer | 0 | merchant-buyer-final-r4.log: 8 cases, 3 locales; exact 4 operations, each once, zero purchase side effects |
| bash scripts/dev/test-local.sh --browser-click-sweep | 0 | click-sweep-final-r4.log and click-sweep-playwright-final.log: 120 variants,988 controls (969 PASS/19 SKIP/0 FAIL),18 journey steps all PASS; 0 page-load failures |

## Frozen PG assertion requiring ruling

`TestCatalogCoreCC02DraftLifecycle` strictly expects image keys `height,id,width`; the integrated media-sizes implementation returns an additional `sizes` field. The existing assertion has NOT been removed or relaxed. Requested authorization to synchronize the schema and add strict sizes validation; no approval recorded yet. This is a merge-contract conflict, not a successful PG gate.

Exact failed assertion: `tests/foundation/catalog_core_gate_test.go:582`; actual tiny fixture image is 3×2 and returns `sizes=[{width:360,pixel_width:3},{width:720,pixel_width:3},{width:1080,pixel_width:3}]`. Proposed ruling: keep strict key equality, update its expected schema to four fields, and add exact array/width/pixel-width checks while retaining all existing ID, dimensions, visibility and isolation checks. Also align the older three-field statement in `contracts/storefront-v2.md`. No such contract/test change is made in this unit without the pending ruling.

## Review and testing boundaries

- Four PE14 model regressions were first red then green (`red/patch-model.log`); a fifth covers new-SKU shipping.
- Existing CC12 assertions remain, including write count >=18. Added independent per-SKU shipping and category clear/restore readbacks; did not replace backend assertions with UI assertions.
- PE fixture uses the existing TWD tenantB/storeB/principalB for integer-NT$ acceptance; a separate mandatory CC12-frozen subtest retains tenantA/storeA1/USD and all its original PG assertions. No store currency is changed beneath existing markets or SKUs. A single active fixture warehouse exercises target_qty; ambiguous warehouse is separately covered by PG PE22 and a labeled MOCK-read UI negative.
- PE15 first switches untracked stock to an explicit target 5, saves and reads that known baseline, then asserts delta +2 / target 7 and target 0 / delta -7. Unknown on_hand is never interpreted as 0; its delta input is disabled with an explanation.
- Live-window UI messages are tested by real clicks with a labeled MOCK 409 in all three locales; real backend enforcement is covered by passing PG PE21. Lost-response copy uses route.fetch then abort and proves one committed command plus reload fence.
- Browser old receipt total 3 was superseded by c6/A's exact 4 stages: product.save, catalog.image.upload, catalog.image.reorder, catalog.product.bulk_status. The gate checks both total and each operation exactly once, retaining the one-product/one-SKU and no-purchase-side-effects assertions.
- Full-sweep red evidence remains in output/playwright/click-sweep/20261003T061519.925021000/click-sweep.mjs.log and its screenshots. The mobile checkbox was clipped by the inherited table heading style; its actionable header is now visible. Unpublish lacked confirmation; active-to-draft now confirms. J1 initially allowed Save before warehouse references loaded; Save now waits. The diagnostic also caught a prefix-colliding SKU locator, changed to exact text without changing expected stock quantities 9/7.
- The journeys-only diagnostics are NOT full gates: they necessarily lack the order placed by the full storefront sweep. J1's 8 steps passed in the second diagnostic; sign-out navigation then required waiting for the normalized locale URL before reload. Both subsequent full r2 and final-source r4 passed.
- Navigation guard red: `red/quick-store-guard.log` exit1, PE failure at `catalog-core/20261003T065324.893706000/playwright.log`: shell store selector expected disabled but was enabled. Final PE r4 exit0 verifies the lock, dirty local-link refusal, restored selector after saving, one confirmation for shell/page leave, one confirmation for actual shell hard navigation, and recovery-fence navigation refusal. No old assertion removed.
- Real browser gates use local production builds, real Go/isolated PG, and MOCK identity. No provider/live deployment acceptance.
- Independent read-only review: explorer `p2_contract`, gpt-6-luna/high; found status sync, new-SKU shipping and archive-confirmation gaps, addressed. `p2_test_map`, same model/effort, mapped old drivers and reviewed screenshots. Both read-only in this worktree, no recursive delegation.
- Independent assertion/fixture reviews: Humaux `e5a5e5b9-4c8d-4264-b08e-1922c9831157` and `a8a97c01-5e87-4a01-86e9-12ed24c9ebf8`; no omitted CC12 path or tenant/store cross-use found. Root ran all gates independently.
- Independent final visual readback: Humaux `2fdff031-e293-4161-847a-fbd5f7d26606`, 23/23 image hashes match; 21 top states at scrollY/topbarY=0, two intentionally scrolled states. Giant negative-wheel screenshot scrolling caused the old top gaps; normal topbar scrolling fixes them without changing product or shell CSS. Before images remain in `red/capture-scroll/`.
- Final PE R4 recapture readback: Humaux `4abc3378-265b-43b4-8a24-e37e32363082`; independent reviewer rechecked all 23 hashes/geometry and two representative mobile/desktop images, no recurrence.
- Independent final code review of `c527c774`: `p2_contract` confirmed specified application-owned navigation paths closed, stale Form lock cleanup and one-shot hard-navigation approval present. Static review only; all runtime evidence was produced independently by root. Humaux `df89cfab-a496-4d68-8ba6-661616099c1a`.
- Impeccable audit/polish preserved approved comps 02/03 and W0. Confirmed textarea font inconsistency was corrected; actual hit-target checks added instead of inferring dimensions from screenshots.

## Screenshots, click evidence and cleanup

- `screenshots.json`: 23 PNGs with SHA-256 and viewport/scroll/topbar geometry; root's final hash check 23/23, zero mismatches. Eighteen list/editor combinations: 1366×768, 1586×992 and 375×812, each in zh-TW, zh-CN, en. Five supplementary matrix/batch/inventory/bottom-field views.
- `click-ledger.json`: 21 PE workflow/evidence entries; actual writes are checked in-browser and by PG. `write-metadata.json` records paths/methods/idempotency keys, omitting request bodies.
- Full final click ledger: `../ui-click-sweep/ledger.json` and `.md`; generated `2026-10-03T07:15:32.642Z`. Skips are reported, not treated as clicks: examples include single-option selects and postponing sign-out to its dedicated journey.
- Unreferenced `r*.png` in `../ui-click-sweep/screenshots/` are retained historical red-run images, not failures in that final ledger. This unit's main red examples are also copied under `red/`.
- Failure artifacts retained in `red/`, including the first click sweep, the minimal-patch model red tests, the quick-store guard red assertion and pre-correction screenshot gaps. Earlier green/checkpoint logs are retained; the gate table identifies final-source logs.
- Postflight: all own execution sessions finished; no node/foundation process cwd in this worktree; `docker ps --filter label=livecommerce.fixture` empty. No other task's processes or artifacts were removed.

## NOT_RUN / BLOCKED

- BLOCKED: focused PG image-schema alignment awaits the requested ruling; final-source rerun again confirms 58 PASS / 1 FAIL / 0 SKIP, exit 1 (`focused-pg.log`, 27.155s). The original failing assertion remains intact.
- `tests/media/r04-input-runner.test.mjs`: test-node reports NOT_RUN because COMMERCE_R04_LIVEKIT_BINARY is unset; unrelated to this unit.
- NOT_RUN: full release/G07 strict, production/provider, non-Chromium browser acceptance. These are not implied by the local gates above.
