<!-- Purpose: W6-U1 source checkpoint and acceptance handoff.
Depends on: frozen W6-01B0139/W6-02B0147 APIs, the source/test manifest and integration branch.
Used by: integrator CI and independent acceptance; no provider or production claim. -->
# W6-U1 customers tags/notes and reports

Status: source delivered for CI acceptance. E3 local MOCK/unit checks; independent source review has no remaining concrete P0/P1. Real browser/PG/Next acceptance is pending. Final SHA is recorded in the main checkout receipt after the final commit. Source hashes in source.sha256 bind the locally tested code to the final commit.

Role: Codex-2/root integrator; UI workers and independent test/review children used isolated worktrees. Configured children gpt-6.1-sol/high; exact deployed runtime model UNKNOWN. Task e052872b-32cf-4a78-bbb4-b8d1e106023f, branch unit/w6-u1-customers-reports-ui, initial base88fe248d. Existing Qwen WIP1694c075 retained and audited. Main evidence directory: /Volumes/data/live_commerce_architecture_v1/output/w6-u1-customers-reports-ui/.

## CI gates

GitHub only, to be run by integrator:
- bash scripts/dev/test-local.sh --browser-customers-billing (preserves CB11 and adds TestBrowserW6Customers)
- bash scripts/dev/test-local.sh --browser-reports
- bash scripts/dev/test-local.sh --browser-admin-shell
- bash scripts/dev/test-local.sh --browser-click-sweep (current10shards + aggregate)
- bash scripts/dev/test-local.sh --browser-visual-lint (current4shards + aggregate)
- pnpm --filter @live-commerce/admin build

Calibration pending on GitHub: LC_W6UI_CALIBRATION=reports-truncate-response with --browser-reports must be RED on RPUI-RED-REPORT-DATA; LC_W6UI_CALIBRATION=customers-drop-response with --browser-customers-billing must be RED on CTUI-RED-NOTE-CONFIRMED. Normal modes must be GREEN. Setup/type/build failures do not count as calibration.

## NOT_RUN

Real browser/PG/Next build and GitHub CI are NOT_RUN locally under owner's RAM rule. Provider SANDBOX/LIVE and production are NOT_RUN. Browser runtime calibration is pending. Current backend omits caller/author display identity, so old notes for write-only staff are conservatively read-only; current-session created-note authorship is proven. Older live-session titles beyond the existing20-item lookup show a localized unavailable fallback. No profit metric or authorization by UI assumption.

## Behavior and review fixes

Customer lists show manual-tag/imported badges and an exact tag filter with search/cursor persistence. Tagged lists use a dedicated closed BFF; untagged requests retain the existing legacy handler. Native tag-manager dialog supports name/color/delete and multi-select assignment (20 cap); notes support create/edit/delete/pages and visible buyer-export/erasure notice. Passive refresh retains edit drafts and original CAS version. An authoritative erased detail immediately unmounts the private notes subtree. Existing consent values are unchanged.

Reports are a finance subpage (nav:false preserves the original nav-finance button), reachable from customers for orders:read. Four tabs, inclusive92-day Taipei ranges, grouped currency/environment amounts, distinct offline collections and audited CSV. SVG only; no new dependencies. Go owns money and store authority. Report-only transport65s matches Go60s budget; CSV75s covers store lookup. Final awaited session fence is followed by synchronous cancellation/visibility/cookie checks before exposing download bytes. No automatic audit retry.

Independent source review found/fixed tag-list search dispatch, stale erased notes, sticky UNKNOWN receipt retention, report budget and post-await download race. UNKNOWN preserves its immutable old command/key through later auth/fence refusals; trusted success alone unlocks it. Initial preflight refusal sends nothing. Unmount/session-scope change drops private in-memory payloads. Review source snapshots: main review-tags.md (18hashes) and review-reports.md (3fix hashes); no runtime review claim.

## Commands actually run

All commands below ran in /Volumes/data/live_commerce_architecture_v1/.worktrees/w6-u1-customers-reports-ui unless a child record says otherwise. Main output paths are relative to /Volumes/data/live_commerce_architecture_v1/output/w6-u1-customers-reports-ui/.

| Command | Exit | Evidence |
| --- | --- | --- |
| git fetch origin r3/integration | 0 | final-fetch.log; origin0d6b4b5f214b99589e459ec6a02cd75a54bf5933 |
| git merge --no-edit origin/r3/integration | 0 | final-origin-merge.log; merge276be024 |
| bash scripts/dev/test-node.sh | 0 | final-test-node.log; optional R04 binary suite NOT_RUN, not PASS |
| node --test --experimental-strip-types tests/admin/customer-tags-bff.test.ts tests/admin/customer-tags-proxy.test.ts tests/admin/customer-tags-ui.test.mjs tests/admin/customer-tags-client.test.mjs tests/admin/customer-tags-draft.test.mjs tests/admin/customer-tags-write.test.mjs tests/admin/reports-bff.test.ts tests/admin/reports-render.test.mjs tests/admin/w6-integration.test.ts | 0 | final-focused-node.log; 78PASS0FAIL |
| pnpm --filter @live-commerce/admin typecheck | 0 | final-admin-tsc.log |
| pnpm exec tsc --noEmit --strict --skipLibCheck --target ES2023 --module esnext --moduleResolution bundler --esModuleInterop --allowImportingTsExtensions --typeRoots apps/admin/node_modules/@types --types node tests/admin/customer-tags.spec.ts tests/admin/reports.spec.ts | 0 | final-spec-tsc.log |
| bash scripts/dev/check-gates.sh | 0 | final-check-gates.log;78modes/alltrackedtests;1217untaggedfoundationtests in10groups;UIarchitecture/header ratchets |
| LC_HEADERS_STRICT=1 bash scripts/dev/check-headers.sh 88fe248d | 0 | final-headers.log |
| GOMAXPROCS=2 GOTOOLCHAIN=go1.27.1 go test -p 1 -tags browser -run '^TestW6UIFaultWrapper$' -count=1 -timeout=120s ./tests/foundation | 0 | final-harness-selftest.log; DB-free selftest only, also compiles browser-tag package |

## Red to green and failed setup records

- Actual customer URL/imported-row regression: integration-red.log2FAIL → integration-green.log2PASS; actual list dispatch/nav/erased subtree now in final5integrationtests.
- Actual tag proxy query: tag-filter-red.log405 vs200 exit1 → tag-filter-green.log6PASS exit0; hostile/duplicate/tag/tenant queries still refused.
- Actual erased Body: erasure-red.log cached-childmount1 vs0 exit1 → erasure-green.log5PASS exit0.
- Worker actual effects: tags-worker red-draft.log3FAIL → green-draft.log3PASS; preserves draft and old CAS version across passive refresh.
- Sticky UNKNOWN: tags-worker UNKNOWN-DELIVERY.md / red-unknown-sticky.log5PASS6FAIL exit1 → green-unknown-sticky.log11PASS0FAIL exit0; root final hook suite independently rerun.
- Report request/DTO tests reject old synthetic500 cap and exercise literal actual output/splits. reports-worker DELIVERY.md has initial setup defects distinguished from behavioral red. reports-worker p1-red.log exit1 → p1-green.log29PASS exit0 (65s transport and no download after final-await cancel/hide/cookie rotation).
- Browser wrapper calibration: browser-worker red-wrapper.log exit1 on W6UI-DROP-AFTER-COMMIT → green-wrapper.log exit0 after restore. Real browser fault calibration remains CI NOT_RUN.
- Preserved historical Qwen output/red.log is a loader/setup failure, not acceptance evidence. Initial WIP baseline13PASS6FAIL also includes setup defects. Early spec tsc omitted Node typeRoots (setupfail); final strictspec tsc0. Early check-gates rejected local Intl note formatter; replaced with shared Taipei displayTime, final0. Code-index oversized output reads failed; bounded root11file index completed (120entities) and child indexes succeeded, proxyCustomerTags memory link confirmed.

## Authors and cleanup

Root integration is Codex-2 at own branch; tags ui_worker commits a285c924 and df4217cf; reports ui_worker00911df0 and f4bf6528; independent test_worker37ac2512; read-only review child (same configured gpt-6.1-sol/high, exact deployed runtime modelUNKNOWN). Child source patches applied by ownership, no release merge/push. Frozen Go/SQL/OpenAPI/dependencies unchanged. No browser/Next/PG fixtures or containers started locally. All own Node/tsc/Go processes exited. Isolated child worktrees remain as recoverable review checkpoints; no other agent state was removed.

Final diff check: git diff --check exit0 (final-diff-check.log). Only source/docs/tests were committed; no push.

## CI seed repair (2026-10-07)

GitHub run37603477927 at dc11b5ce failed only --browser-customers-billing: legacyCB11PASS, newW6seed customers.Get unavailable before browser. Earlier normal/calibration reports runs37602961907/37602957580 at276be also failed on the same seed, so neither qualifies as intended calibration red.

Root cause: report-only rpWorld.order snapshots lack destination.phone (SQLlast3 becomes non-NULL empty string, strictCustomer rejects) and the frozen merchant order shape. Removing the two invalid checkout anchors retains legitimate import-profile-only customers, the historical archive,21tags/52notes and every browser assertion;0152 declares them visible. Checkout coverage stays in the oldCB11. Next independent seed defect: its USD fact reused a TWD-only PayUni attempt; now Stripe checkout method/USD/700 matches the USDorder/fact and0061constraint. No validators, schema, thresholds or assertions changed.

Local own focused realPG only, noNext/browser:
- LC_FOCUSED_TAGS=browser LC_FOCUSED_TIMEOUT=180s bash scripts/dev/test-focused.sh '^TestW6UISeedFocused$' → exit1 (PASS0FAIL1) reproduces customerGet: ci-37603477927/red-seed.log.
- after first fixture fix → exit1 (PASS0FAIL1) exposes attempt_method_amount_check: red-usd-seed.log.
- after both valid-fixture corrections → exit0 (PASS1FAIL0SKIP0): green-seed-final.log. Test constructs the actual shared seed and calls productionGet; does not certify browser/permission/CAS/report totals.
- mandatory git merge --no-edit origin/r3/integration (origin0813424a) → exit0 beforefocusedrun; check-gates → exit0; git diff --check → exit0.

Readonly independent source re-review accepts both fixture changes, no gate weakening. Same-familysource-only, fullnormal browser +twoactualcalibrations MUST rerun on GitHub at newcommit. Alllocalfocusedprocesses finished, statusJSON/runids/logs inmainci-37603477927/. Noheavy localmode started. Integrator stillowns push/CIcallbacks.
