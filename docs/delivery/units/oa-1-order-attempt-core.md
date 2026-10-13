# OA-1 — atomic order-attempt core

Owner dispatch: 2026-10-13. Contract: `contracts/order-attempt-v1.md`, frozen
at PR #50 `5e6ef2e97fabd587936be8bd5c951767017b4ed5`. This implementation
brief does not amend that contract. Backend work is explicitly authorized;
all execution and tests run in GitHub CI only. PR #32 stays untouched.

## Ownership and boundaries

- Implementer: Codex-3; reasoning: high; base: `55e98ba0434557e94e3b7c9e31ff0e98352623b1`
  (frozen #50 plus trunk `221539a7f6634824dbabb5e0f3681a270a3e7b8b`).
- Worktree: `.worktrees/oa-1-order-attempt-core`; branch: `unit/oa-1-order-attempt-core`.
- Allowed paths: this brief; `output/oa-1/`; new OA migrations under `migrations/`
  and `migrations/post_river/`; `internal/orderattempt/`; transaction-aware helpers
  in `internal/{buyer,storefront,checkout,claims,merchanttools,inbox,platform}/`;
  composition roots `cmd/{api,claims-worker}/`; associated exact ACL/upgrade,
  schema, model, real-PG and process-wiring tests under `tests/foundation/`;
  generated dependency documentation and CI selection only where required.
- Excluded: UI, production, provider I/O, dependency/lock files, shared OpenAPI,
  old migrations, PR #32, and changes to frozen contract semantics.
- Provisional migration slots: root 0170 onward, post-River 0023 onward;
  re-fetch/check the trunk maximum before the PR and record exact final slots.

OA-1 implements C1, C2, order-attempt jobs and sweep. Source inspection shows
that complete C2 also needs the *local* pay-entry/notification plan and the
subject/erasure lock prefix. Those prerequisites belong in this transaction
unit. OA-2 retains C3/C4, dispatch lease sweep, copy/reminder/CAS and their
public commands. OA-3 retains versioned public admission/recovery APIs,
retention/legacy cutover, A15 and the UI-only rebase. This is a unit-boundary
adjustment, not permission to omit a C2 effect.

Do not activate new placement until the dependent dispatcher, cancellation,
redemption and expiry paths can enforce the new provenance and payment-entry
rules. Dormant internal scaffolding is not complete C2 acceptance. Never use
no-op hooks or synthetic capability sessions to pass a gate.

## Implementation seam

- `ops.order_attempt_scopes`, `ops.order_attempts`,
  `ops.order_attempt_subjects` implement the §2 logical records.
- `ops.order_attempt_creators` owns the immutable business identity
  `(tenant_id,store_id,buyer_owner_id,attempt_id)` used by domain FKs. It holds
  no scope, generation, principal, hashes, receipt or timestamp. Replay records
  reference that same identity, but can be removed at the 90-day tombstone
  boundary without invalidating legally retained business orders. Permanent
  denial markers must retain no join to either record. Internal child records
  name the identity `create_attempt_id`; this refines the contract's logical
  records without changing their authority or retention.
- `commerce_order_attempt_owner` is a NOLOGIN coordinator, never a member of
  domain-owner roles and never a holder of direct domain-table write grants.
- Public execution entries are the frozen `ops.lock_order_attempt`,
  `ops.execute_order_attempt`, `ops.abort_order_attempt`; all parameter lists
  are pinned by real-PG tests before callers are wired.
- A Go-owned `pgx.Tx` is the only C2 commit owner. Domain helpers verify the
  stored attempt identity and plan digest. They neither call `WithScope` nor
  acquire an earlier lock level. Existing storefront rules stay unchanged.
- Pricing uses the existing pure Go pricing calculation in C1. C2 materializes
  the accepted plan and rechecks only the frozen contract's safety predicates.

## Required evidence

| Clause | Acceptance surface |
| --- | --- |
| §1/§2/§6k | `TestOrderAttemptSchema*`: all-state identity, RLS, exact ACL; actual-login negative and cross-pool tests follow |
| §2 creator chain/§6j | `TestOrderAttemptProvenance*`: exclusive session/attempt FKs, own carts, per-origin duplicate SKU lines |
| §3/§6 model | `TestOrderAttemptModelSeeds`, `FuzzOrderAttemptModel`: independent MODEL_ONLY oracle, injected invalid traces rejected |
| §6a–f/h–k | REAL_PG C1/C2 barriers, crash/ack loss, CAS, settled replay, rotation, erasure, cancellation, deadline and frozen facts |
| §6g/l | Transaction-local pay-entry/outbox checks here; full provider/dispatch acceptance belongs to OA-2 |
| §6m | Subject cleanup/erasure lock prefix here; versioned HTTP, retention and old-client cutover in OA-3 |

The schema/model gates do not substitute for the real-PG matrix. Preserve each
red SHA/run and green SHA/run. Required mutations are assigned by their actual
acceptance case; no generic passing concurrency test counts as a mutation.

Full relevant foundation shards, exact migration-count/upgrade/ACL pins,
contract drift, affected process/browser modes and independent money/privacy
review remain required before merge. No local command executes these gates.
