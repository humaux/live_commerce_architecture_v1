# FEAT-CONTACT-STORE-FIX1 — PR #375

Current authority: the integrator's directed FIX1 card and 2026-10-31 ruling (migration 0211, additive storefront contract, contact handler/SQL, required admin/storefront BFF and one existing notify kind; R2 = base count + new files). Repository AGENTS.md and 架构.md §§0,1,2,26,27 read. Humaux/coord/code-index tools unavailable; no shared-memory mutation, claim or independent review is claimed. Assigned branch/worktree retained; no sub-agents or changes to the main checkout.

## Revisions and rollback

- RED PR head: `ca5a55e5926391aeb1a8eb8c55832e706a3f9d53`; Gates tested merge `174d8ee7748579c4dc303451e59a5aedf6dfc32c`.
- Merged `origin/r3/integration` **first** at `a3dc78e4d5ec72831aaf6a376793f0f6a7a56425`, then refreshed before push to `629f42152f864621a99ac2247c3baf5d751a422f` (no additional migration).
- Implementation rollback point: local merge `69f1762277c2061ace7f23694356485d10e58276` (no migration executed; revert the implementation commit on this branch if withdrawn).
- Base Gates `41105181453` at a3dc78e4d was still queued when read. It is not base GREEN or proof of any identical browser failure.
- Existing uncommitted evidence from the prior relay was preserved. Root `output/FIX1.md` is the historical BLOCKED report; this module report supersedes its current-state conclusions. The filesystem resolves requested `output/feat-contact-store/FIX1.md` to the existing canonical directory `output/FEAT-CONTACT-STORE/FIX1.md`.

## Failing job → test → evidence → class

Fresh `gh run view --job … --log-failed` reads confirmed the Go 404 assertions and browser job. Previously downloaded artifact `gate-browser-click-sweep_7_10`, artifact ID `12981636339`, supplies the browser ledger. All original contact assertions remain. None is an obsolete signed-Figma assertion (class a).

| Run / job | Failing test | Evidence | Class / action |
|---|---|---|---|
| 41056798375 / 124066257869 | contact BFF admits a guest once without a buyer cookie or context and returns only the committed receipt | [foundation.log](FIX1-evidence/foundation.log): 404 !== 201 | b: implement product route |
| same | contact BFF refuses invalid bodies and honeypots before private transport | foundation.log: 404 !== 400 | b: strict BFF validation |
| same | contact BFF enforces method, key, content type and origin independently of a guest session | foundation.log: 404 !== 405 | b: BFF admission fences |
| same | contact BFF accepts Unicode bounds and omitted optional fields | foundation.log: 404 !== 201 | b: Unicode field validation |
| same | contact BFF validates successful upstream receipts without leaking PII or creating sessions | foundation.log: 404 !== 503 | b: closed receipt projection |
| same | contact BFF relays bounded throttling, conflicts and uncertain outcomes without retrying | foundation.log: 404 !== 429 | b: single transport + bounded Retry-After |
| 41056798358 / 124070185925 | TestStoreContactPublicPersistenceReplayAndMerchantHandling | [g10.log](FIX1-evidence/g10.log): line 123, HTTP status=404 want=201 | b: persistence/replay and merchant API |
| same | TestStoreContactValidationAndNoClientScope (19 validation subtests) | g10.log: line 182, HTTP status=404 want=422 | b: authoritative validation |
| same | TestStoreContactPublishedOriginAndMerchantPermissions | g10.log: line 209, HTTP status=404 want=201 | b: published origin + scoped permissions |
| same | TestStoreContactIPAndStoreRateLimitsAndReplay | g10.log: line 265, HTTP status=404 want=201 | b: transactionally serialized budgets |
| same | TestStoreContactConcurrentRetryAndEncryptedAtRest | g10.log: line 297, HTTP status=404 want=201 | b: encrypted envelope + store lock |
| same | TestStoreContactNotificationOutboxAndNoSensitiveLogs | g10.log: line 389, HTTP status=404 want=201 | b: one existing notify outbox intent |
| same | TestStoreContactUnknownMailIsNotRetried | g10.log: line 421, HTTP status=404 want=201 | b: retain existing UNKNOWN protocol |
| same | TestStoreContactConcurrentHandlingConflict | g10.log: line 436, HTTP status=404 want=201 | b: locked CAS + opaque command receipt |
| 41056798358 / 124070185645 | TestBrowserClickSweep | [ledger.md](FIX1-evidence/browser/playwright/run.pZts2ZJb/ui-click-sweep/ledger.md): r00025 admin `/`, desktop zh-TW, `todo-stock`, no visible change within 3 s | c-candidate per explicit ruling; no dashboard/page-path changes; integrator reruns job. No identical base assertion proved. |
| 41056798358 / 124098833489 | receipts | [receipts.log](FIX1-evidence/receipts.log): browser shard and g10 receipt FAIL, test_count=18/246 | dependency-only; no standalone receipt defect |
| 41056798358 / 124098833372 | aggregate | [aggregate.log](FIX1-evidence/aggregate.log): failed gate aggregation | dependency-only |
| 41056798358 / 124099371779 | required | [required.log](FIX1-evidence/required.log): gate/aggregate/receipts failure | dependency-only |

## Root fix and unchanged boundaries

- One migration, 0211. Base R2 exact count 139 + one file = 140. The frozen legacy matcher stays verbatim; route-table test now asserts exactly 47 rows and gives only the new exact contact path its new method oracle. All historical paths/aliases/methods retain the old oracle; no test, skip, fixture, threshold or exemption removed/weakened.
- Public/private BFF, origin authority, encrypted PII, replay-before-throttle, versioned merchant read/handled and one new notify kind implemented. AAD/keys/locks/buckets bind both tenant and store; no assumption of globally unique store UUIDs.
- Notification stays in the existing outbox/worker. Its SQL patch retains current fairness, COD, suspension, caps, attempts and UNKNOWN semantics. No buyer email, new transport, inline SMTP or PII body in the notice. Retention uses bounded cleanup from that same worker, including dormant stores.
- Additive admin BFF grammar reuses signed session/store/CSRF authority and closed response projections. New Go authentication/tamper/rotation and admin BFF negative cases supplement the original RED suites. No product page/component, Dashboard path or browser-sweep exemption changed. No Figma write or UI delivery claim.

## Static checks and CI handoff

Allowed local static checks only: affected-package `go build`, affected-package plus foundation `go vet`, `gofmt`, changed TS `node --check`, `python3 scripts/check_packet.py` (structure only). Exit codes recorded in DELIVERY; no local behavioral tests or servers. TypeScript compiler unavailable locally, so full tsc is CI-only. Generated browser ownership and dependency map use their requested generators, never hand edits.

One FIX1 implementation push; Gates/Foundation workflow IDs are recorded in the PR comment and relay, not an evidence-only follow-up commit. Polling bounded to about ten minutes; integrator is the agreed wakeup/relay owner. If the same assertion fails again after the two directed attempts, report BLOCKED with its exact assertion; no third approach.

Evidence: **E3 historical causal RED only; E1 static structure; current acceptance PENDING; author's assessment E0.** NOT_RUN: local tests/servers, current-head GREEN until CI completes, browser/UI acceptance, independent review, provider SANDBOX/LIVE and production deployment. No task process or fixture/server/container left running at handoff.

## Paths outside explicit product scope

- `tests/foundation/r2_integration_upgrade_test.go`: explicitly authorized pin ruling.
- `internal/buyerhttp/routes_test.go`: exact additive route inventory/oracle in owned transport; original frozen fixture untouched.
- `internal/storefront/contact_test.go`, `internal/notify/contact_test.go`, `tests/admin/live-settings-{bff,model}.test.ts`: direct contact crypto/BFF coverage in existing harnesses.
- `output/ci-select-backend-browser/browser-ownership.json`, `docs/engineering/dependency-map.md` if changed: required generators.
- `output/FEAT-CONTACT-STORE/`: card evidence, report, delivery and PR body. Integrator review requested for these support paths; no other module code changed.
