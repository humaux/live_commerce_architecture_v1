# OA-2 — order-attempt notification authority

Status: DESIGN / NOT_RUN (META-00 R6, 2026-10-28); continues OA-1, does not rewrite its C1/C2 core.
Purpose: complete C3/C4 and the order/outbox/operation/payment-entry command boundary.
Depends on: accepted META-00 and merged OA-1 core; re-read current integration before implementation.
Used by: OA-3 public transport, LC-U3-ATTEMPT and existing dispatcher/reminder/cancellation paths.
Invariants: one locked dispatch permit, paired outcomes in one commit, cancel-first zero sends, UNKNOWN never resend.
Authority: `contracts/order-attempt-v1.md` §§3.1/4/5/6g,l, OA-1 unit boundary and META-00 R5/R6.

## Scope and write paths

- Implement C3 authorization/lease and C4 generation-fenced outcome/reconciliation; route payment-DM before any
  generic operation-first lock path. Keep `external_operation_v1` args exactly `{operation_id, version: 1}`.
- Complete cancel/rotation invalidation, read-only copy, reminder semantic-key replay and exact-order batches;
  command/domain helpers here, versioned public HTTP/BFF and buyer consumer in OA-3. READY/VOID cannot be retried
  through generic operation APIs. Unbound refusal remains VOID + NULL operation, no fabricated binding/job.
- Wire startup/30-second PaymentDMLeaseSweep and both-state lease transitions; a discarded job must not strand
  DISPATCHING or authorize a second call. Preserve order→conversation→binding→outbox→operation lock suffix.
- Write paths: `internal/inbox/order_attempt_notification*.go`,
  `internal/integrations/metareply/order_attempt*.go`, `tests/foundation/order_attempt_notification*.go`,
  `output/oa-2/**`. Required dispatcher/cancel/reminder/scheduler composition paths and SQL must be assigned
  by integrator before edits (including `cmd/claims-worker/`); do not bypass them with no-op hooks.
- Root/post-River migration numbers, exact ACL/COMMENT/upgrade pins and R2 pin: **integrator assigns**.
  Shared schema/OpenAPI, dependencies and lock files remain integrator-owned. Keep deployment activation untouched:
  `LC_ORDER_ATTEMPT_ENABLED="0"` until OA-3 acceptance, not merely this unit's GREEN.

## Batch RED list (GitHub CI only)

1. Cancel before C3 → zero provider calls and revoked entry (M2a/M2b separately); authorization-first retains
   remote-outcome evidence and invalidates payment. Two-connection barriers prove the real lock race.
2. Duplicate jobs, C3/pre-I/O crash, fake provider accepts then disconnects, expired/discarded lease → paired
   DELIVERY_UNKNOWN/UNKNOWN; no second send. Sweep mutation M7 must turn the periodic-path assertion red.
3. Stale outcome generation cannot overwrite current disposition; every outbox/operation outcome commits
   together. Invalid pair/NULL operation outside before-C3 VOID is rejected by actual DB constraints.
4. Generic retry bypass M4 fails for READY/VOID/FAILED_FINAL with no calls. Query/reconcile is not dispatch.
5. Concurrent copy/reminder/CAS rotation produces no dead candidate; copy never rotates/extends TTL. Reminder
   key replay across versions returns the original row; buyer A cannot block buyer B. Rotation blocks only
   DISPATCHING and needs explicit CAS; UNKNOWN/ACCEPTED rotation preserves evidence and never resends.
6. Actual runtime/worker roles enforce exact grants, private key custody, scope/privacy and no plaintext leaks.

One tests-only push batches all assertions/mutations; each RED must reach its own intended assertion in CI,
then one implementation push. Preserve designated §6g/l counterexamples and M2a/M2b/M4/M7 evidence separately.

## Acceptance gate and handoff

PR-own REAL_PG + fake Graph CI with actual logins, deterministic lock barriers and raw paired-state readbacks;
§6g/l and affected cancellation/reminder/dispatcher regressions, exact ACL/R2/contract drift, independent
money/privacy review. E3 receipts must bind current head and tested merge SHA, run/job, fault, exit and artifact.
Expose frozen command signatures/result DTOs to OA-3; OA-1 historical GREEN is not dispatch acceptance.
No local tests/servers. CI-only RED→GREEN, no weakened assertions/thresholds/exemptions; stop after two fixes
on the same blocker or immediately for a gate outside scope. No merge, activation, production or live messages.

NOT_RUN: implementation, all CI/REAL_PG/mutation/review gates until exact receipts; public API/buyer/UI cutover
(OA-3), Meta/Stripe SANDBOX/LIVE, App Review, real messages and production. Provider proof remains MOCK.
