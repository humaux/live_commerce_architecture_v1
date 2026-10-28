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
