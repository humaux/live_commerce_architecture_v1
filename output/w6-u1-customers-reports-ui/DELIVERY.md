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

## Kimi P2 fold-in and current seed proof (2026-10-07)

Preserves seed repair19a4 and the separately supplied4e4 readiness/account-popover browser test correction. Latest origin/r3/integration ba1d1579 merged at00979 before this follow-up. No backend/SQL/DTO change.

- Customer leaf now dispatches through the same customerTagsRoute raw grammar as the W6 proxy. Bare/encoded tag keys are refused422 by the legacy closed grammar before upstream; validtag keeps W6 authority. Original five query cases remain, tested through real routes/auth/backend/cookie/CSRF helpers with only upstream fetch synthetic.
- Both report JSON and CSV success headers add Vary: Cookie alongside private,no-store/nosniff; only trusted server bearer reaches Go.
- Actualroute red on00979:6PASS3FAIL exit1, named P2-BARE-TAG-422 and P2-REPORT-VARY-COOKIE, no setup failure. Worker tests-onlyfe6794 green9PASS0FAIL0SKIP exit0; root full focused W6 group82PASS0FAIL exit0. New Node file registered in test-node/GATES.
- Root fresh realPG: LC_FOCUSED_TAGS=browser LC_FOCUSED_TIMEOUT=180s LC_W6UI_CUSTOMERS_ACCEPTANCE=1 LC_W6UI_REPORTS_ACCEPTANCE=1 bash scripts/dev/test-focused.sh '^TestW6UISeedFocused$' -> exit0 PASS1FAIL0SKIP0. Same browser tag and acceptance environment; it constructs the actual sharedseed including Get/SetOwnerTags/notes, then productionGet. No Next/browser launch. Original Get empty-phone-tail and subsequent USD method constraint reds remain in ci-37603477927; wrong setup calibration does not count.
- node --test --experimental-strip-types tests/admin/customer-tags-bff.test.ts tests/admin/customer-tags-proxy.test.ts tests/admin/customer-tags-ui.test.mjs tests/admin/customer-tags-client.test.mjs tests/admin/customer-tags-draft.test.mjs tests/admin/customer-tags-write.test.mjs tests/admin/reports-bff.test.ts tests/admin/reports-render.test.mjs tests/admin/w6-integration.test.ts tests/admin/w6-route-seam.test.mjs -> exit0 (82PASS), p2/node-current.log/status.
- bash scripts/dev/test-node.sh -> exit0, p2/test-node-current.log/status; optional R04 binary suite NOT_RUN.
- pnpm --filter @live-commerce/admin typecheck -> exit0; strict spec tsc for customer-tags.spec.ts and reports.spec.ts -> exit0 (full args in p2/spec-current.status.json).
- bash scripts/dev/check-gates.sh -> exit0 (78 modes;1218 foundation tests distributed, not1218 runtimepasses). LC_HEADERS_STRICT=1 bash scripts/dev/check-headers.sh 00979c2779bcdca384cc8cf31fd8e0678d5a748e -> exit0, owns all7 follow-up source/test/gate paths. Whole strict diff against origin exits1 on five pre-existing inherited ads/settings files; recorded separately, not edited and no thresholds relaxed.
- Samefamily independent narrow source review accepts both P2s with no new concreteP0/P1; p2/REVIEW.md SHA2562b91c3ebd95f4747df7df1f0ef3cda42efa0c65b46cef952ba6323b1687bca46. Kimi cross-family MERGE verdict supplied by integrator applies to the preceding unit review; neither review is full browser runtime proof.

All owned Go/Node/tsc subprocesses finished; focusedPG script removed its own disposable fixture/container and released the heartbeat lock. Current55-file manifest binds actual source, including readiness specs and real sharedhelpers. Index submissions/production links explain the shared grammar and report privacy header; MJS helper entity lookup remains NOT_LINKED where unsupported, source/hash/Node proof retained. Childtest/review worktrees remain recoverable, temporary source copies restored only in owned childWT, no other agent state removed.

CI retry (GitHub only, integrator owns dispatch/callback): normal --browser-customers-billing and --browser-reports; LC_W6UI_CALIBRATION=customers-drop-response with --browser-customers-billing must RED on CTUI-RED-NOTE-CONFIRMED; LC_W6UI_CALIBRATION=reports-truncate-response with --browser-reports must RED on RPUI-RED-REPORT-DATA. Then normal GREEN. Existing admin-shell/click-sweep/visual/build gates remain listed above. Current full browser/CI and both named calibrations NOT_RUN locally; no heavy mode, provider/production or push.

## Final latest-origin reconciliation

Remote origin advanced during handoff to35abffcadaa1240888d291391712e1411cb820a3 (W6-U2). P2 source checkpoint808766c8 is retained; inbound merge committed d997e786. Two test-local guard/build-list conflicts resolved by retaining both browser-reports and browser-operations-ads. Static check caught the usage line missing operations-ads; added that entry and rechecked, without executing a heavy mode or altering gate thresholds. Both runtime branches and all other modes remain.

After incoming catch-all/Node runner/tagged Go additions, reran affected checks: W6 focused Node82PASS0FAIL exit0 (node-latest), adminTS0 (ts-latest), fulltest-node0 (all-node-latest; optional R04 NOT_RUN), fresh same-browser-tag/both-acceptance-env DB-only TestW6UISeedFocused PASS1FAIL0SKIP0 exit0 (seed-latest). Previous specTS0 binds unchanged4e spec bytes. A wrapper setup NameError occurred before any second PG process/container was started; fixed wrapper and preserved seed-latest-wrapper-setup.log, not a behavioral calibration.

Latest-origin strict headers now exit0, the five inherited earlier ads header gaps were resolved by incomingW6-U2; prior exit1 remains historical. check-gates/bashtable syntax and CI-plan checks pass on the combined mode registry; current manifest56files includes the changed legacy catch-all dependency. Both P2 source hashes remain exactly9cc4be84/08326865, so the narrow readonly review still binds them. All local processes finished; no local browser/Next/full foundation. Final SHA will be appended to the main receipt after FINAL STEP git add -A && git commit.

## Current batch — PR3 packet a83e2aea (2026-10-08)

Base: `a83e2aea` after the required `git fetch origin && git merge --no-edit origin/unit/w6-u1-customers-reports-ui` (both exit0; clean fast-forward from ab3). Branch remains `unit/w6-u1-customers-reports-ui`. Codex-2 owns the patch; one high-effort read-only explorer independently traced CI and checked the narrow diff (exact deployed model ID unavailable). No delegated writer. Integrator owns K3 pre-review and push.

Changes:

- **4212542034:** real tag/note DELETE accepts a zero-byte stream without Content-Type; rejects nonempty bytes even with a forged zero length. JSON MIME checks, session/store/Origin/CSRF and idempotency fences remain. Tests invoke actual route/auth/backend helpers with native Requests and fake only upstream fetch.
- **REAL reports gate:** Go `writeAttachment` overwrote the private response policy with `no-store`; BFF correctly required the frozen `reporting-v2.md` §1 `no-store, private` contract. The shared generated-attachment writer now retains that private policy. Existing attachment, report-export and customer-import assertions are strengthened to the exact policy. No BFF validator relaxation or API/SQL/schema/dependency change.
- **4212542061 (data correctness):** all report panels show response-echoed applied dates and Taipei timezone. Edited inputs show localized pending guidance; CSV stays disabled until Show applies them. Channels/Funnel tests keep September labels while October is only a draft, then verify Show and refresh update the label.
- Local real-click continuation exposed old spec defects after the formerly failing operations: nested dialog locators inside `has`, Next route-announcer matching the global alert selector, a one-shot report fault armed before the previous read settled, and outdated shell-denial/session-expiry expectations. Selectors/readiness now follow the actual feature and shell states. Error, CSV, audit, CAS, UNKNOWN same-key, privacy and permission assertions remain; no timeout increase, skipped test, weaker threshold or fixture replacement.
- A newly added assumption that revoked retries send no browser POST was disproved by trace (the BFF returns401 before any backend note write). It was corrected to assert401, the same key, unchanged backend count, expired shell and private UI absence. This was a test-oracle correction, not a production auth change.

Evidence is preserved in the **main checkout**: `/Volumes/data/live_commerce_architecture_v1/output/w6-u1-customers-reports-ui/round-a83e2aea/`. `acceptance-source.json` binds production/spec bytes; `receipt.json` binds the final commit after commit. CI source traces from run37654986134 are retained in `ci-customers/` and `ci-reports/`; local failure and green browser traces/ledgers are retained separately.

| Command | Actual result | Evidence |
|---|---|---|
| `node --test tests/admin/w6-route-seam.test.mjs` | RED exit1:5PASS/2FAIL → GREEN exit0:7PASS | delete-{red,green}.log + status |
| `go test ./internal/httpapi -run '^TestWriteAttachment$' -count=1` before header repair | RED exit1 | csv-cache-red.log |
| `go test ./internal/httpapi -run '^Test(WriteAttachment\|ReportRoutesTransportRulesBeforeDatabase\|ParseReportQuery)$' -count=1` | GREEN exit0 | csv-cache-green.log |
| focused Reports SSR date regression; then full reports-render suite | RED exit1:0PASS/1FAIL → GREEN exit0:27PASS | dates-{red,green}.log |
| `bash scripts/dev/test-focused.sh '^Test(ReportRP06ExportPermissionAuditAndCSV\|CustomerImport)$'` | exit0:2 top-level PASS/0FAIL/0SKIP, real PG | focused-pg.log |
| `bash scripts/dev/test-local.sh --browser-customers-billing` | final exit0:legacy admin12PASS + buyer flow; W6 9PASS | browser-customers-billing-attempt4.log; local-customers-green/ |
| `bash scripts/dev/test-local.sh --browser-reports` | final exit0:7PASS | browser-reports-attempt3.log; local-reports-green/ |
| `bash scripts/dev/test-node.sh` | exit0:736PASS/0FAIL/0SKIP (optional R04 separately NOT_RUN) | node.log; handoff-node.log |
| `pnpm --filter @live-commerce/admin typecheck` | exit0 | types.log; handoff-types.log |
| strict `pnpm exec tsc --noEmit --strict --skipLibCheck --target ES2023 --module esnext --moduleResolution bundler --esModuleInterop --allowImportingTsExtensions --typeRoots apps/admin/node_modules/@types --types node tests/admin/customer-tags.spec.ts tests/admin/reports.spec.ts` | exit0 | handoff-spec-types.log |
| `bash scripts/dev/check-gates.sh` | exit0;79 modes documented, header ratchet passes | gates.log; handoff-gates.log |

Earlier browser attempts remain honestly RED: customers attempts1/2/3 each exit1 (nested `has`, route-announcer ambiguity, then the newly introduced over-strong POST-count assumption); reports attempts1/2 each exit1 (one-shot fault race, then route-guard expectation). They are distinct diagnosed failures, not repeated blind retries. The final seven reports and nine W6 customer scenarios have no skips. Per-command status JSON records real child exits rather than wrapper exits. PG/test-local modes ran strictly one at a time using the heartbeat lock. Next production builds ran as part of the explicitly requested browser modes.

The older planner on this branch requires two arguments: the protocol's one-argument command exited1 with usage; `node scripts/dev/pr-modes.mjs origin/r3/integration HEAD` exited0, selected49 modes (`required-plan.json`). No planner changes belong to this batch.

**Deferred:** 4212542044/4212542053 (CSS), 4212542070 (tag directory navigation), 4212542075 (note editing UX) are recorded in `FOLLOWUPS.md`. Both final requested browser modes pass with report CSS unchanged. Known browser-identity trunk flake remains Codex-1's lane.

**NOT_RUN / remaining:** K3 pre-review and new GitHub CI at the final SHA; other planner-selected modes/full foundation/admin-shell/click-sweep/visual-lint; provider SANDBOX/LIVE; optional R04 binary. Existing calibration fault modes were not rerun in this repair batch. No production access, live keys, real money, push, PR creation or release-branch merge. Acceptance is **BROWSER MOCK + REAL_PG, E3** in the measured local environment, not deployment/provider acceptance. Stop after commit for integrator review.
