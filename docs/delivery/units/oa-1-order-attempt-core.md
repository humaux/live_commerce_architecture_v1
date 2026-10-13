# OA-1 — atomic order-attempt core

Owner dispatch: 2026-10-13. Contract: `contracts/order-attempt-v1.md`, frozen
with A1/A1.1 and merged as PR #50 `2d97c2027eefaa7df6553a832ffa38aa9fcd9d93`. This implementation
brief does not amend that contract. Backend work is explicitly authorized;
all execution and tests run in GitHub CI only. PR #32 stays untouched.

## Ownership and boundaries

- Implementer: Codex-3; reasoning: high; current trunk base:
  `228a1c877f63b2be0955060d03ed621a05f4bdcf`, fetched and merged in
  `a354230555e572a5cb488f7847db938b5ffbaa1e`. The contract branch is no longer
  the dependency base; all implementation and retained tests remain on this branch.
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

The first execution seam is
`ops.lock_order_attempt(auth_hash bytea, store_id uuid, attempt_id uuid,
intent_generation bigint) RETURNS jsonb`. Runtime supplies current merchant
read authentication; the actual claims-worker login supplies NULL auth hash.
The first locking statement is the attempt row. It returns the immutable
binding and encrypted plan only for the exact admitted generation; terminal
rows return their stored receipt without reapplying placement gates. Worker
access never admits a new attempt. A protected transaction-ID witness binds
later private execution helpers to this held fence; caller-set GUCs do not.
This internal parameter choice implements §1/§3, not a new public HTTP API.

The C2 new-order identifier is the admitted attempt UUID. The order lock uses
`pg_advisory_xact_lock(hashtextextended('checkout.order|' || tenant || '|' ||
store || '|' || order, 0))`, with canonical UUID text and server-derived scope.
This gives the not-yet-inserted order the same serialization key as its later
lifecycle commands. Existing cancel, expiry and capture entrances must adopt
that key before activation; a private C2 helper alone does not establish the
cross-command lock order. Catalog locks follow that order key, before service,
pickup/destination and inventory. Allocation aggregates demands by SKU using
the existing Go allocator, while quote/order lines retain separate origins.

The composite entry uses one internal signature:
`ops.execute_order_attempt(attempt uuid, generation bigint, exact_plan text,
step text, input jsonb) RETURNS jsonb`. `prepare`, `begin`, `pay`, and `finish` are
in-transaction call stages, not durable attempt states or separate commits.
`prepare` takes the owner/claim/order prefix, materializes the quote and
destination, and returns locked allocation facts. Go runs the existing pure
allocator. `begin` validates that result, writes native order effects and returns
the context needed for local encryption. Go prepares the order-only credential;
`pay` validates/stores its versioned envelope and returns the current notification
preparation facts under the retained order lock. Go seals the notification.
`finish` validates/stores that artifact, completes
source/claims/outbox and the terminal receipt. Every call revalidates the held
current-transaction fence and the exact stored plan; native creator identities
prove the earlier stage. Only the Go executor commits. Deferred constraints
reject partial materialization; no runtime may call the domain helpers directly.
This preserves the frozen single-transaction design while keeping pricing,
allocation and cryptography in their existing Go implementations.

The internal `pay` input is exactly `{key_version, token_hash_hex,
credential_ciphertext_hex}` (32-byte digest and 71-byte envelope). Its result is
`{stage: "PAY_PREPARED", pay_entry: <safe stored metadata>, notification: <private
preparation DTO>}`. The additional call stage is necessary because notification
preparation verifies the stored credential; it adds no commit or durable state.
The `finish` input is the exact notification artifact (including the one-field
`NOT_REQUESTED` disposition). SQL derives the receipt from native order facts;
no caller-supplied order, money or receipt is accepted. The terminal receipt is
`{state, attempt_id, intent_generation, order_id, commercial_state, payment_mode,
total_minor, currency, send}` and contains no bearer or contact. All merchant
attempts emit `order.manual_created` with only attempt/order/generation metadata,
as the existing shared ManualOrders placement does for both entry flows. Its
membership FK takes KEY SHARE on the membership row, not a recursive principal
lock; held-principal/scope tests must prove the frozen lock rule. Missing native
FK targets remain an internal retryable fault, not an inferred business refusal.

Go execution is exposed internally as `NewExecutor(pool, planKeys, payEntryKeys,
inboxService)` plus `Execute(ctx, merchantToken, store, attempt, generation)`.
These concrete dependencies use one pool/transaction; no testing hook replaces
domain effects. Inline and River use that same method and the same fixed error
classification. The return is the durable metadata receipt; bearer disclosure
belongs to the later locked read surface.

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
