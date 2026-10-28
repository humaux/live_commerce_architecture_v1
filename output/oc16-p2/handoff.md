# oc16-p2 — CI RED stage

Authority: https://github.com/humaux/live_commerce_architecture_v1/issues/219 (body and all three comments read 2026-10-28).
Assignment: integrator-dispatched implementation card, no additional delegation.
Role/runtime: Codex implementation worker; exact model/reasoning configuration unavailable to this runtime (not inferred).
Base: 5365b5701a0e99ef9fd8fbb4e64a3a9f87aaf248.
Worktree: /Volumes/data/live_commerce_architecture_v1/.worktrees/oc16-p2.
Branch: unit/issue-219-oc16-p2. PR target: r3/integration.
Write paths: issue-named files and direct tests; output/oc16-p2/**; generated dependency map only if required.
Humaux tools are absent from tool inventory; memory/coord/canvas operations NOT_RUN.

## Current unit: issue body item 1

Tests-only addition: TestOrderAttemptExecutorAbortClassification/PT422/bank_transfer_closed.
Expected RED: closed native pair got empty string, want payment_unavailable.
The table also checks wrapped errors and rejects wrong SQLSTATE and message prefixes/suffixes.
Do not implement until GitHub CI confirms this assertion fails.
Minimal next fix: add bank_transfer_closed to the existing PT422 payment_unavailable switch case in internal/orderattempt/executor.go.
Acceptance: own PR CI RED then GREEN, contract drift zero new, independent integrator acceptance.
No local test/server/generator has run.

## Remaining issue units

- Body item 2: buyer contract and storefront-v2 still omit bank_transfer_closed; checkout.safeError forwards it through cvsRefusal. Resolve per issue's allowed alternatives after reading complete affected contracts. Preserve the existing exact buyer refusal assertion in tests/foundation/bank_transfer_closed_test.go:127. Do not alter another module's contract without card scope; report any excluded paths in PR.
- Comment item 1: add privilege pin tests for DELETE/TRUNCATE on checkout.bank_transfer_settings and checkout.bank_transfers, plus UPDATE on immutable transfer bank columns. Do not add migrations; stop if a grant correction requires one.
- Comment item 2: existing TestBtcSaveReplayAfterClose covers equal-body replay and a new key, but not old key/different body. Migration 0202 receipt lookup already raises PT409 idempotency_conflict before the closure gate (lines 54–61). Add regression coverage with CI mutation RED if baseline already behaves correctly.
- Comment item 3: closure also blocks enabled=false; executor mapping requested by body item 1 addresses the stated dependency. No policy/migration change authorized.
- Already fixed on base (static E0 evidence only): internal/httperror/error.go:199 knows bank_transfer_closed; tests/foundation/bank_transfer_closed_test.go:146–148 asserts exact code and retryable=false. Skip changing these OC-15 items.

## Resume protocol

Read red.log and exact CI head/run/job. Use gh run view RUN --log-failed (filtered). If expected new assertion is RED, record run/job and failure in red.log BEFORE implementation. If unrelated gate blocks the card, STOP. At most two targeted attempts per blocker.
After GREEN, final run/job evidence belongs in a PR comment, never an evidence-only commit.
Evidence: E0 static inspection; CI NOT_RUN/PENDING until receipts arrive.

## Resume inspection 2026-10-28

Foundation run 40628067749/job 122617676540 succeeded, but step 12 (Real PostgreSQL, race detector and vet) was skipped. Its failed-job log is empty. This does not establish the required new-test assertion RED. Gates run 40628067835 remains queued; the actual unit job is 122622547420, confirmed against the tests-only head a3b8c31044eefb156eb7607ef1be6d7735185796 through the job API.

No implementation or new push was made. RED is still PENDING. WAIT_RUN: 40628067835. The integrator must relay the unit-job conclusion; on assertion RED, append its exact failure to red.log and continue implementation. Existing uncommitted output files are preserved for the implementation commit. Local tests/servers and Humaux memory/coord/canvas remain NOT_RUN. Evidence level: E0 for implementation acceptance; CI metadata proves only the recorded job states.

## Item 1 implementation after confirmed CI RED — 2026-10-28

Raw job API confirms run 40628067835/job 122622547420 completed failure, tests-only head a3b8c31044eefb156eb7607ef1be6d7735185796; synthetic checkout cb89258d27e6bcf4c824cd84ddaa38bc8e1ca056 merges that head into base 5365b5701a0e99ef9fd8fbb4e64a3a9f87aaf248. At 05:25:41 UTC TestOrderAttemptExecutorAbortClassification/PT422/bank_transfer_closed fails executor_test.go:52: closed native pair: got "" want "payment_unavailable". Receipt is appended to red.log before implementation.

Root cause: exact PT422 pair omitted from executionAbortCode. Minimal fix adds bank_transfer_closed to the payment_unavailable case. Executor.Execute already rolls back C2 then invokes fenced Service.Abort for that terminal code. No schema, grants, signatures, dependencies or header changes. gofmt -d internal/orderattempt/executor.go exited 0 with no output. No local tests/servers or generators run; dependency map unaffected by the unchanged import graph.

Humaux is now available. Read development detail 7bdb220b-0798-4b1d-aff5-16ee94cae75f; applied ponytail; searched rejected decisions (representative 3/9, not full coverage). Stored fix/RED receipt and own subtask canvas oc16-p2-sub-1 under acquired advisory lock; no extra task claim because dispatched worker. Prior unavailable notes above are historical. No recursive delegation.

Next: await own implementation push CI and integrator relay. GREEN PENDING, contractdrift PENDING. Other issue units remain open as listed above; they have NOT been implemented or claimed complete. In particular item 2 must respect the card prohibition on editing another module contract; comment regression tests need their own RED receipts. Keep PR open; no merge/auto-merge/status posting. E3 describes only the witnessed RED; implementation acceptance E0.

## Push receipt / next wake-up

PR #260 implementation head 83deaff3ca56ac4a0832b19cb9e2531baec10546 pushed successfully to unit/issue-219-oc16-p2; PR body updated. WAIT_RUN: 40638790619 (Gates, pending) and 40638790649 (Foundation, queued), both API-bound to that exact head. Test jobs not allocated yet. No GREEN claimed. Integrator owns the wake-up and will relay conclusion in a fresh session. Read the new failed logs before changes; stop on unrelated gate. Remaining #219 units still NOT_DONE. This receipt stays uncommitted to avoid restarting CI; include in next authorized implementation commit, or publish final evidence in PR comment after GREEN.

## Batched remaining regression tests — 2026-10-28

Authority re-read: issue #219 body and all three comments, AGENTS.md, architecture sections 0/1/2/26/27; native grants in migrations/0088_checkout_offline.sql:131-134 and receipt conflict guard in migrations/0202_bank_transfer_closed.sql:54-61. Humaux tools are unavailable in this resumed runtime; memory/coord/canvas NOT_RUN. Existing worker assignment retained; no delegation.

Prior implementation CI: Gates 40638790619 has successful unit job 122685375787 and all allocated matrix/aggregate/receipts jobs successful; final required job 122782931961 was queued at inspection. Foundation 40638790649/job 122651820013 succeeds, but its real-PG step is skipped and supplies no PG acceptance. No failed job was available (`gh run view --log-failed` reports run still in progress). Do not substitute the Foundation result for Go tests.

All remaining in-scope tests are in ONE tests-only push. Extend the existing TestBtcPolicyNotWritableByApp with all-commerce-role DELETE/TRUNCATE checks on both bank tables and UPDATE checks on bank_name/branch/account_name/account_number/window_hours/currency. Extend TestBtcSaveReplayAfterClose with exact PT409 idempotency_conflict for old key/different body and verify the original receipt still replays. Existing assertions remain intact.

The native grants and receipt guard already implement the requested behavior. To prove the added tests detect regressions, RED-only mutations execute in owner transactions with deferred rollback: grant the forbidden privileges; replace the single native receipt hash comparison with false. The replay call runs as SET LOCAL ROLE commerce_runtime. No migration, committed DB mutation or global fixture change. RED must fail on `forbidden bank privilege` and `old-key/different-body after closure`, not setup errors. On confirmed assertion RED, remove ONLY the two explicitly marked RED-only mutation blocks in ONE final implementation push. Keep assertions, scopes, rollback and runtime role intact. No other push is authorized by the two-push batching budget.

Item 2 is SKIPPED under the card's other-module-contract boundary: buyer-checkout-options-v1 is FROZEN and documents only bank_transfer_unavailable; documenting bank_transfer_closed also requires storefront-v2/admin error-table alignment. Preserve the current buyer and merchant exact bank_transfer_closed assertions. Integrator must own this contract decision; PR records the exclusion and must not imply full issue closure acceptance. Comment item 3 depends on the executor mapping already implemented; no enabled=false policy exception or migration is authorized. OC-15 public-code fixes remain skipped as already present.

Static checks: gofmt on the changed Go test file and git diff --check, exit 0. No import or dependency changes; dependency map unchanged. Local tests, servers, generators, PG, independent review and production actions NOT_RUN. Evidence E1 for formatting only; new regression acceptance E0/PENDING until own PR CI. Before final push append native RED run/job/assertion receipt to red.log. Integrator owns wake-up after the tests-only push; record Gates/Foundation RUN ids in this file after allocation, without a third evidence-only push.
