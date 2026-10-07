# W6-U2 browser tests — E1 structure only

task_id: W6-U2-sub-browser-tests
base_commit: 5d73d90bdc053a2e99e64fd6c49d23577b6f9066
model assignment: gpt-6.1-sol / high; role: test_worker
worktree: .worktrees/w6-u2-browser-sub; branch: unit/w6-u2-browser-sub

Implementation: real-click Playwright spec + browser-tag Go harness reusing oqEnv, adsEnv(noWorker=true), signed MOCK OIDC/production Next and isolated PG. Runner controls are authenticated, loopback-only, fixed synthetic fixture changes; no page.route interception or DOM writes. Tests click filters, refresh, next, drawer/object links, detail refresh, confirmation dismissal and submit; cancel/read-registry retry/query reload, UNKNOWN/protective refusals, CAS, quota+Retry-After, cross-store/read-only permissions. Ads tests confirm pause-first/history, binding_in_use and nonempty in-flight list, then detach with persisted draft/connection history; copy public feed, unpublished copy unavailable. Three locales × desktop/390 with accessible labels and 12 hashed screenshots. SQL owner readback independently verifies events/jobs/audits and credentials destroyed.

Changed paths: tests/admin/operations-ads.spec.ts, tests/foundation/browser_operations_ads_test.go, playwright.config.ts, scripts/dev/test-local.sh, docs/delivery/GATES.md, this browser-notes directory. No production Go, lockfile, commit/push/merge.

Actual commands/exit codes: checks.json, check-1..4.log (all exit 0); dependency-resolution failed attempts preserved in prior-check-failures.log. Runtime gate: bash scripts/dev/test-local.sh --browser-operations-ads. Runtime evidence expected at output/playwright/operations-ads/<timestamp> with click-ledger.json, screenshots.json, pngs, next.log, ads-stack/next.log, playwright.log and retained failure traces.

NOT_RUN: browser, REAL_PG runtime, provider SANDBOX/LIVE, independent runtime review, new browser gate red→green. Root must integrate files, independently rerun static checks and run runtime mode on a frozen SHA before E3. Feed-retry error button and uncertain response-loss repeat controls require a future bounded transport-fault fixture (NOT_RUN coverage); current spec does not claim those paths passed. Production effects/credentials are absent. Fixtures/Next processes are only started by the runtime Go test and have registered cleanup. No processes/containers started in this authoring task.
