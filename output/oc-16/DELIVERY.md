# OC-16 delivery: server closes new bank-transfer writes

Branch `unit/oc16-bank-transfer-closed`, PR #211 (base `r3/integration`), issue #210 (follow-up). Contract: `contracts/owner-choices-20261026-v1.md` OC-Q1. Design: integrator ruling 2026-10-27 (single-row policy table; legacy flip in the shared fixture). All tests ran on GitHub CI only.

## Implementation
- `migrations/0202_bank_transfer_closed.sql` (number assigned by integrator; may become 0201 if OC-16 merges before OC-02):
  - `checkout.bank_transfer_policy(singleton PK, closed boolean NOT NULL)`, one row inserted `closed=true`. Owner is the migration owner; only `commerce_checkout_writer` has SELECT (it owns the guard definer). No commerce_* role has INSERT/UPDATE/DELETE/TRUNCATE. Reopening requires a new migration (ADR).
  - `checkout.bank_transfer_closed_guard()` SECURITY DEFINER trigger function (owner `commerce_checkout_writer`, no EXECUTE grant) raising `PT422 bank_transfer_closed` while closed. Triggers: BEFORE INSERT on `checkout.bank_transfers` (every way a new transfer order is created, including `checkout.order_attempt_begin`, which then rolls back whole); BEFORE INSERT OR UPDATE on `checkout.bank_transfer_settings` (any new or edited bank detail).
  - `payments.set_bank_transfer_settings` re-created (CREATE OR REPLACE, body of 0088 plus one guard) to refuse right after authorization with `PT422 bank_transfer_closed`.
  - Untouched: `decide_bank_transfer`, `expire_held`, `submit_transfer_proof`, every read and export, report fields. No row is deleted or changed.
- Test fixture (legacy, issue #210): `legacyOpenBankPolicy` in `tests/foundation/foundation_integration_test.go` sets `closed=false` after migrating, called from `newFixture` and from `pwIsolatedFixture` (`payment_runtime_test.go`, the other fresh-database fixture the order_attempt tests use). Owner-only write; no assertion in the other 51 files changed.
- Pins changed in this PR: `tests/foundation/checkout_offline_gate_test.go` triggers on `checkout.bank_transfers` (adds `bank_transfer_closed_guard`); `tests/foundation/r2_integration_upgrade_test.go` migration set 129 -> 130. `contracts/tasks.json` OC-16 `write_paths` updated (0202 file name, the new test, the three pin/fixture files).
- No Go source added: the HTTP error shape for `bank_transfer_closed` (and hiding the UI entry) belongs to OC-08 / OC-15; the SQL layer is what OC-16 owns.

## Tests (`tests/foundation/bank_transfer_closed_test.go`)
- `TestBtcPolicyClosedAfterMigrate`: policy reads closed=true right after `migrations.Apply`, before the fixture flip; exactly one row.
- `TestBtcPolicyNotWritableByApp`: no commerce_* role holds INSERT/UPDATE/DELETE/TRUNCATE; UPDATE as `commerce_checkout_writer` and as the runtime login, and DELETE as the runtime login, fail with 42501.
- `TestBtcRefusesNewWrites`: a new order through the app is refused and leaves nothing; direct transfer INSERT, `set_bank_transfer_settings` (CAS and first-insert shapes), the real settings route, direct settings UPDATE and INSERT are all refused (`PT422 bank_transfer_closed`); saved details unchanged and still read.
- `TestBtcHistoryStaysUsable`: orders placed before closure confirm, reject, and refund_offline through `decide_bank_transfer`; merchant transfer read, buyer view, settings read and the finance CSV (`bank_transfer_confirmed_*` columns) still work; no row deleted.

## CI evidence
- RED: PR #211 head `56b2e0293` (tests only), Gates run 40384465228, job 121792636843 (shard:g09): `TestBtcPolicyClosedAfterMigrate`, `TestBtcPolicyNotWritableByApp`, `TestBtcRefusesNewWrites`, `TestBtcHistoryStaysUsable` FAIL. See `output/oc-16/red.log`.
- Fix iterations (all CI-found): 354dfb16c (fresh isolated fixture also needs the legacy open: `TestOrderAttemptLocalKeyMissingExpiresAtRealDeadline`), e8e0f5f1b (R2 count pin), 84296fcf9/479dc0644 (finance CSV window in the test).
- Run 40399767186 (head e8e0f5f1b), shard g09: the three other Btc tests PASS; `TestBtcHistoryStaysUsable` finance subtest failed on a test-side date range, fixed in 479dc0644. Same job also showed `TestProductEditorMatrixLedgerEvidenceHistoricalPolicy` failing with "needs full history" (shallow checkout; also present in the RED run, unrelated to OC-16).
- GREEN candidate: head `84296fcf9` (after merging `origin/r3/integration`), Foundation checks run 40417883744 success; Gates run 40417883817: see final status in the PR (at the time of writing: 36 jobs success, 0 failed, shard:g09 not yet finished).

## NOT_RUN
- Browser tests are not changed. They go through the same Go fixtures, so they run with the policy open (legacy); they do not assert the closed behaviour (no UI hiding here; that is OC-08).
- No provider sandbox/live (not applicable). Independent review: NOT_RUN.
- Local runs: only `go build`, `go vet`, `gofmt`, `python3 scripts/check_packet.py` (exit 0).

## Risks
- With the policy closed the order_attempt coordinator fails the native begin with an unmapped message `bank_transfer_closed`; `internal/orderattempt/executor.go` only maps known messages. A stored/admitted bank_transfer plan would be retried or failed by the generic path. Admission and the storefront already hide bank_transfer when no settings are enabled; closing admission explicitly belongs to OC-08/OC-15.
- Order matters at the policy flip in production: any in-flight bank_transfer begin at deploy time is refused (rolls back cleanly, no stock held).
- The 0202 vs 0201 numbering depends on merge order (integrator).
- ~51 Go test files still use bank_transfer as a generic mode, relying on the legacy flip (issue #210).
