# OA-2 delivery — C3/C4 command surface (partial)

- Branch: `unit/oa-2`; RED base: merged OA-1 / META-00 `7ce347b70729733f18cc35372c3f7161eda64c0d`.
  The branch was synced with `origin/r3/integration` `d8093bc43` by merge `3cb6787a`.
- Authority: `docs/delivery/units/oa-2-order-attempt-notify.md` and
  `contracts/order-attempt-v1.md` §4/§6g,l. No contract text was changed.
- Status: **BLOCKED on the integrator-assigned migration and composition paths**.
  This is not OA-2 acceptance and not dispatch activation.

## Task record

- Role: integration_worker (backend); model: deepseek-flash; reasoning: harness default.
- Base SHA: `7ce347b70` (RED), synced with `origin/r3/integration` `d8093bc43` by merge `3cb6787a`.
- Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/oa-2`.
- Allowed write paths: `internal/inbox/order_attempt_notification*.go`,
  `internal/integrations/metareply/order_attempt*.go`,
  `tests/foundation/order_attempt_notification*.go`, `output/oa-2/**`.
  `output/ci-select-backend-browser/browser-ownership.json` was regenerated only to
  register the new test file (hash-checked by `scripts/dev/derive-browser-ownership.mjs`).

## What is implemented

| File | Content |
| --- | --- |
| `internal/integrations/metareply/order_attempt.go` | Frozen C3/C4 command and result DTOs for OA-3; one-permit dispatch state machine; ambiguous-outcome handling; stale-generation error mapping; lease sweep; `NotRetryable` hook; reminder semantic/intent keys; pgx coordinator seam for the five requested C3/C4/sweep definers. |
| `tests/foundation/order_attempt_notification_dispatch_test.go` | MODEL tests: one permit, duplicate job zero calls, accepted-then-disconnected → DELIVERY_UNKNOWN, stale generation refusal, M4 retry refusal for READY/VOID/FAILED_FINAL/DISPATCHING/ACCEPTED/UNKNOWN, M2a/M2b cancel-before-C3 zero calls, M7 sweep, reminder key isolation/replay, dispatch-mode VOID refusal. |
| `output/oa-2/SQL-REQUEST.md` | Exact schema delta, definer signatures/lock order, grants, and the composition paths the integrator must assign. No migration number is invented. |

`LC_ORDER_ATTEMPT_ENABLED` stays `0`; no `cmd/`, `apps/`, migration, dispatcher,
cancel, reminder or scheduler path is edited.

## Why blocked

1. `inbox.order_payment_notifications` has no `UPDATE` grant to any runtime role
   (`migrations/0188_order_attempt_notification.sql:44-52`) and the claims worker
   has no outbox `SELECT`; its paired trigger requires outbox and operation to
   change in one Tx (`0188:426-470`).
2. `integration.claim_operation` locks `integration.bindings` before the
   operation (`migrations/0096_worker_authorities.sql:119-168`), which is not the
   OA-2 order→conversation→binding→outbox→operation suffix.
3. Reminder identity has no columns: the outbox has no `client_reminder_key` or
   cross-version `reminder_intent_key` (`0188:9-31`), so §4 reminder replay cannot
   be enforced by a constraint.
4. Cancel/rotation/copy/reminder/dispatcher/scheduler composition files are
   integrator-assigned by the unit brief; no no-op hook was added.

## Evidence

- RED: tests-only `48546ecd6`. Foundation checks run `40844363540` (job
  `123348127167`) failed at `tests/foundation/order_attempt_notification_dispatch_test.go:19`:
  `undefined: metareply.OrderPaymentClaim`. Gates run `40844363876` failed on the
  same compile break. This is a tests-first API-absence RED, **not** a behavioral
  assertion and not E3.
- IMPL: **MODEL_ONLY** at `5edaaa082`. Foundation checks run `40849140762`
  (SUCCESS, 09:22:37Z→09:31:02Z) and Gates run `40849140547` (SUCCESS,
  09:22:37Z→09:59:05Z; plan/build, shards unit/g01–g10 and browser-e2e,
  receipts, required). The nine model tests are in the catch-all g10 per the
  Foundation shard-plan log. E3 remains NOT_RUN: no REAL_PG, ACL, lock-barrier,
  mutation or scheduler receipt exists, so this is not OA-2 acceptance.

## NOT_RUN

- REAL_PG C3/C4 paired-state, two-connection lock barriers, actual-login ACL and
  the M2a/M2b/M4/M7 DB mutations.
- Periodic `PaymentDMLeaseSweep` startup wiring; cancel/rotation/copy/reminder
  composition; provider I/O.
- Independent money/privacy review, OA-3 public API/BFF/buyer cutover,
  Meta/Stripe SANDBOX/LIVE, App Review, production, real messages.
