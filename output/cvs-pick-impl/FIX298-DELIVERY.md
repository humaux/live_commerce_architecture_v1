# FIX298 root-cause handoff — 2026-10-29

Authority: dispatched FIX #298 card; current PR head and original GitHub job/artifact receipts; repository AGENTS.md and 架构.md §§0/1/2/26/27. Humaux tools unavailable: no store/canvas/coord action claimed.

## Cause and imported repair

PR head before repair: 32cc17cf1a6164a4093dd44b99ec33a35cebdbc4. Gates 40830596290/job123306495317 (bundle:08) fails TestBrowserOC. Artifact12879638106 gate-browser-password-auth was downloaded and read. Its OC Playwright log reports 1 failed / 8 passed: zh-TW orders fails in createStore at reauth.spec.ts:120 with page.goto net::ERR_ABORTED. This is setup/navigation RED, not an assertion RED and not a CVS product failure. Synthetic checkout d9ba08f0e6f440b38d81252afc171afe828675e8. Filtered original evidence: fix298-red.log. Prior RED history remains in red.log untouched.

Merged origin/r3/integration at d8093bc43f16f8eb4e209eeb6c7b698efcd27b0e. Its existing tests/admin/oc/reauth.spec.ts change waits for PasswordAuth's location.assign to reach the canonical locale URL before createStore initiates navigation. This directly addresses the observed race; effectiveness on this PR remains PENDING until CI. The file has no PR diff against the target integration branch. No new backend, other-app or shared-contract edits were authored. No assertion, timeout, retry, fixture or threshold was weakened. No second tests-only push is needed for this merge-only repair; original failing receipt retained without claiming a new assertion witness.

## Scope and validation

New authored paths: output/cvs-pick-impl/FIX298-DELIVERY.md and fix298-red.log only, within the card evidence directory. Existing dirty DELIVERY.md/red.log and all prior untracked files preserved and excluded from staging. Integration-parent changes belong to the target branch, not a new outside-card implementation. PR-wide prior outside-card disclosure remains scripts/dev/test-node.sh (integrator approval required) and generated browser ownership (pre-approved).

Static check: git diff --check exit0. No local tests/build/servers/browser/PG or generators run. No spec/page/route was added, renamed or deleted by this repair. Source/header/import/ownership changes were imported intact from integration; no new component harness dependency.

E0 author diagnosis; E3 original CI failure only. Repair GREEN and independent review PENDING. No merge into integration, auto-merge, commit status or production action. New CI run IDs must be recorded in a PR comment and the final WAIT_RUN, never an evidence-only commit after GREEN. Integrator relays completion; no local process/container/server to retain. Raw failure artifacts retained for review.
