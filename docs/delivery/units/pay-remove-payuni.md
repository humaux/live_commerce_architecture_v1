# Unit PAY-RM1: remove the W4-01B PAYUNi NotifyURL receiver (backend, stub)

**Status.** DRAFT stub, written 2026-10-06. It waits for integrator review.
- Base: `r3/integration` `2fa471a1`.
- Migration placeholder: **0145**. It is a forward migration; 0136 is never edited.
- Worktree `.worktrees/pay-remove-payuni`, branch `unit/pay-remove-payuni`.

## Integrator 裁决

- **Owner decision 2026-10-06:** 「不使用PAYUNi…」. W4-02B and W4-03B are cancelled.
- W4-01B (merge `651c5744`, migration 0136) was **never deployed**: there are no `deploy/` references, and
  `COMMERCE_PAYUNI_NOTIFY_ENABLED` is unset everywhere. It stays disabled until this unit merges. Then it is deleted.
- Scope is **only** the W4-01B additions. The older PAYUNi hosted, query and capture path (0014–0018,
  `internal/integrations/psp/payuni`) is still referenced by the buyer and payment flows and the architecture. Removing
  it is a separate owner/integrator decision (OPEN-1). Do not touch it here.
- **Forward-only.** The migration refuses (`RAISE EXCEPTION`) if any `payuni_notify_receipts` row or
  `review_cases.reason='NOTIFY_MISMATCH'` row exists. Data is never deleted silently.

## Remove (re-derive the list with `git show --stat 651c5744` and `grep -rl payuni_notify\|payuninotify\|PAYUNI_NOTIFY`)

**Go:**
- `cmd/api/payuni_notify.go` + test, and the `cmd/api/main.go` mount (+10 lines);
- `internal/payments/payuninotify/**`;
- `internal/platform/payuni_runtime.go` and its references in `internal/platform/platform.go` / `stripe_runtime.go`
  (if they are W4-01B-only);
- the W4-01B part of `internal/integrations/accounts/payuni_crypto.go` and the `psp/payuni/client.go` notify helpers,
  **only** what is unused after removal (`go vet` + `staticcheck` unused).

**Tests:**
- `tests/foundation/{payuni_notify_test.go,payuni_notify_authority_test.go,k3_w4_01b_adversarial_test.go}`;
- the W4-01B rows in `worker_authority_split_test.go`, `hosted_payment_authority_test.go` and
  `r2_integration_upgrade_test.go` (MCI02 replay line);
- `cmd/api/stripe_sp15_test.go` (payuni env references).

**`migrations/migrate.go`:** delete the `GRANT UPDATE(scheduled_at) ON river_payment.river_job TO
commerce_integration_writer` re-assertion. 0145 then REVOKEs that column grant.

**SQL 0145 `0145_remove_payuni_notify.sql`:**
- DROP the functions `payments.payuni_resolve_endpoint`, `payuni_record_notify`, `set_payuni_notify_endpoint` and
  `guard_payuni_notify_receipt`;
- DROP the tables `payments.payuni_notify_receipts` and `payuni_notify_endpoints`;
- restore `review_cases_reason_check` without `NOTIFY_MISMATCH`. Re-derive it from `pg_get_constraintdef` (0136
  pattern). Drop `review_cases_report_hash_scope`, then `SET NOT NULL` on `source_report_hash`;
- REVOKE and `DROP ROLE commerce_payuni_ingress`, after reassigning nothing (it owns nothing);
- REVOKE `UPDATE(scheduled_at)` on `river_payment.river_job` from `commerce_integration_writer`. Do that in the
  post-River phase if the table is post-River.

**Docs:**
- `contracts/payuni-wire-v1.md`: append "Amendment: W4-01B notify receiver removed (0145)";
- `docs/delivery/units/w4-01b-payuni-notify.md`: status line REMOVED.

## Tests (red first, then green; save the red run to `output/pay-remove-payuni/red.log`)

| # | Case | Expected |
| --- | --- | --- |
| RM01 | `TestRemovePayuniNotifyRM01Schema` (REAL_PG) | after 0145, none of the objects exist (`to_regclass`/`to_regprocedure`/`pg_roles`); `review_cases` CHECK equals the pre-0136 definition; `source_report_hash` is NOT NULL; `commerce_integration_writer` has no `UPDATE(scheduled_at)` on `river_job` |
| RM02 | same, seeded with one receipt row | 0145 aborts with the named exception; nothing changed |
| RM03 | DB-free router build (`cmd/api`) | `POST /v1/hooks/payuni/notify/x` → 404; no other route changed |
| RM04 | upgrade replay | fresh DB and populated-latest DB both reach the same schema hash |

## Gates

**Local:**
- `bash scripts/dev/test-focused.sh '^TestRemovePayuniNotify|^TestWorkerAuthority|^TestHostedPayment'`
- `go build ./... && go vet ./...`
- `bash scripts/dev/check-gates.sh`

**CI gates:** the full foundation suite (MCI02 upgrade replay), `test-local.sh --browser-payment` and
`--payment-worker` (PAYUNi hosted path unchanged), `release-gate.sh --strict`.

## Evidence / role / dependencies

- **Evidence:** REAL_PG + HTTP (no provider).
- **Role:** Claude Sonnet (backend). K3 RM01/RM02. Opus review.
- **Dependencies:** none. Do not run in parallel with W4-S1, which re-creates loaders read by
  `worker_authority_split_test.go` (shared pin file).

## OPEN

- **OPEN-1:** remove or keep the base PAYUNi card path (0014–0018, buyer UI PAYUNi branch). **Recommendation:** keep it
  dormant (no method is enabled in LIVE) until the owner confirms that no Taiwan merchant needs PAYUNi. Then open a
  separate removal unit with an architecture §12.2 amendment.
