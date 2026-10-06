# T06 internal external-operation ledger v1

Status: INTERNAL_SLICE_ACCEPTED, 2026-09-20, code baseline `9bac8e4`. Contributes to I04/I06/I14/I20 and G02/G04; not a provider, public HTTP, payment or production gate. Independent bounded review and 106-test real-PG/race/vet evidence: [acceptance](../docs/implementation/2026-09-20-external-operation-acceptance.md). Full T06 remains IN_PROGRESS.

## Boundary and reuse

Go/pgx caller-owned transactions and the pinned River OSS client remain the only queue mechanism. No new dependency, queue engine, generic workflow DSL or secret store. `integration` is an internal module, not a microservice. The later runnable dispatcher is accepted separately in [external-dispatcher-v1.md](external-dispatcher-v1.md); the buyer checkout authority bridge remains unimplemented. A buyer is never impersonated as a merchant membership.

The initial producer is a trusted merchant-domain transaction with a server-resolved `platform.Scope`. A future checkout producer needs a separately reviewed buyer bridge; this slice gives buyer runtime/issuer no integration or River grants. Functions are not mounted on public routes. A typed Scope must match transaction-local tenant/store/principal, not just contain valid UUIDs.

## Data and permanent intent

- `integration.bindings`: internal UUID, tenant/store composite FK, provider identifier, external asset identifier, positive semantic version, enabled flag, creating merchant principal, DB timestamps. Provider/asset identity is immutable; an asset change requires a new binding. Enable/revoke changes increment semantic version. Secret renewal is not a semantic change and secrets are not stored here. This row is a binding reference, NOT evidence of provider permission or production eligibility.
- `integration.operations`: UUID; tenant/store/principal; binding ID and frozen semantic version; provider/asset snapshot; purpose (`transactional`, `service`, `marketing`); action; semantic key; SHA-256 of canonical frozen input; bounded JSON object; initial job ID; state, generation, lease mode/until; bounded result code and provider reference; created/updated DB timestamps. Immutable columns have no runtime/worker UPDATE grant. Composite binding/merchant FKs prevent cross-tenant/store assignment.
- `integration.operation_events`: append-only operation ID + tenant/store FK, generation, state/mode, bounded reason code, DB time. No raw provider errors, credential, address or message body.
- Semantic key is unique per tenant/store, independent of River retention or unique-job windows. Use domain-generated stable keys, not a new key for every HTTP attempt. The canonical input includes principal, binding/version, purpose, action, frozen request; a changed input is a conflict. Existing operation replay returns its original operation/job IDs, even after binding changes. This is historical readback, not dispatch permission.
- Frozen request is an internal object (at most 64 KiB serialized), containing domain IDs and the minimum immutable business values. No access tokens, raw address, unredacted conversation or authentication material. It is not an arbitrary public JSON endpoint. Future adapters own typed request schemas and privacy review.
- River job kind `external_operation_v1`; args EXACTLY `{operation_id, version: 1}`. No tenant authority or frozen payload in job args. The worker obtains scope from the operation row. The first operation, its event, receipt and `InsertTx` job commit or roll back together. No network call occurs in this transaction.

## Producer API

`RegisterBinding(ctx, tx, scope, token, key, provider, assetID)` creates version 1 enabled reference with command receipt/audit; it does not perform OAuth. `SetBindingEnabled(ctx, tx, scope, token, key, bindingID, expectedVersion, enabled)` uses CAS and bumps semantic version, including revoke/re-enable. Both call existing `platform.RequirePermission` for exact `integration:manage` inside the scoped transaction. There is no automatic existing-member grant/backfill. Test fixtures explicitly grant permissions; production authorization and UI remain separate work.

`Plan(ctx, tx, scope, token, key, input)` returns immutable `{operation_id, job_id}`. Require exact `integration:execute` with `platform.RequirePermission`. Input includes binding ID, expected binding version, action, purpose, request object. Validate scope, identifiers, size and canonical JSON before writes. Lock command key, then binding FOR SHARE; active/version must match for a new operation. Replays reuse the command receipt and permanent operation key. Caller rolls back on every error, including River insertion/event/audit/receipt failure. Existing `command.Run` is reused; no separate public idempotency framework.

`Get(ctx, tx, scope, token, operationID)` requires exact `integration:read` and is store-scoped historical readback; another tenant/store sees not found. Actor is provenance; an authorized same-store merchant can read. The public DTO/privacy policy is not defined here.

## Worker lease and state API

**Amendment T21-02/T21-03 (migration 0096, unit worker-authority-split): the single shared `commerce_worker` is replaced by one NOLOGIN authority per worker process** -- `commerce_payment_worker` (payment-worker SANDBOX/PROVIDER_MOCK), `commerce_payment_live` (payment-worker LIVE), `commerce_expiry_worker`, `commerce_ads_worker`, `commerce_claims_worker` (claims dispatcher; also the default lane). `commerce_worker` remains only as an EMPTY legacy role and no login may join it. `integration.claim_operation`/`complete_operation` refuse (operation-not-found, before any mutation) an operation whose lane (`integration.operation_lane`: payment = provider stripe|payuni or actor BUYER_PAYMENT_QUERY|PAYMENT_REFUND; ads = meta_ads|meta_dataset; media = MEDIA_ATTEMPT/livekit, never claimable; default = all else) is not owned by the connected login's authority (`session_user` membership, never an argument), and RLS limits each authority's reads to its own lane. The Stripe/PAYUNi `require_*` guards derive the allowed execution profile from the same membership: LIVE only for `commerce_payment_live`, SANDBOX/PROVIDER_MOCK only for `commerce_payment_worker`. The text below describes the worker contract; read "`commerce_worker`" as "the calling process's own worker authority".

`commerce_worker` is an ordinary NOLOGIN role, not inherited by merchant, identity or buyer roles. A separately provisioned login passes `platform.OpenWorkerPool`'s exact-one-authority and privilege/object-owner checks. It can read integration projections and operate River's own schema; business state/event changes are EXECUTE-only through fixed `integration.claim_operation` and `integration.complete_operation`. A non-login/non-inherited `commerce_integration_writer` owns these SECURITY DEFINER functions with fixed pg_catalog search_path and no PUBLIC execution. No worker direct UPDATE/INSERT/DELETE on business tables, identity/capability authority, or catalog/inventory mutation. No database-owner connection may run a worker.

Worker functions accept caller-owned short transactions. Database EXECUTE grants verify worker authority. First perform an unlocked locator read of immutable binding ID, then lock binding FOR SHARE → operation FOR UPDATE and revalidate the composite association. Never hold locks across network I/O. Fixed SQL predicates use `clock_timestamp()` AFTER lock acquisition, not application time or transaction-start `now()`.

`Claim(ctx, tx, operationID, leaseSeconds)` permits lease 5–300 seconds. Go generates a crypto/rand 32-byte lease token, passes it to SQL (only SHA-256 is stored), and returns the raw token only with a committed-claim result to its caller, never a public DTO/log/job. Disposition, mode, generation and this owner token fence completion; reading generation/hash does not let a different claimant complete.

1. Active lease: `busy`, no write/no dispatch permission.
2. Terminal SUCCEEDED/FAILED_FINAL/CANCELLED/BLOCKED_POLICY/STALE_BINDING: `terminal`, no write.
3. READY with enabled binding and exact semantic version/provider/asset: increment generation; state DISPATCHING; mode `dispatch`; DB lease. This permits only an adapter's subsequent current policy check, NOT an unconditional external call.
4. Expired DISPATCHING, UNKNOWN or ACKNOWLEDGED: increment generation; state UNKNOWN; mode `reconcile`; new lease. Never return dispatch, even when the earlier process may have died before sending.
5. Disabled/changed binding: only a never-dispatched READY becomes terminal STALE_BINDING. An expired dispatch/UNKNOWN/ACKNOWLEDGED becomes or remains UNKNOWN with disposition `blocked_binding`, clears lease and prevents calls using the replacement account. Preserve the possible remote side effect. Repeated unchanged blocked-binding reads need not append events. A future authorized reconciliation route must query the frozen asset; no silent new-account fallback.

`Complete(ctx, tx, operationID, generation, leaseToken, outcome)` locks binding then operation, requires matching token digest AND generation AND unexpired lease, then atomically records state/event and clears lease. Base migration 0008 outcomes: SUCCEEDED, FAILED_FINAL, UNKNOWN, ACKNOWLEDGED; migration 0009 additionally accepts BLOCKED_POLICY only under the restricted dispatcher condition below. Result code is machine identifier 1–80 chars; provider reference <=200 chars, no arbitrary error text. Binding change during work does NOT erase an observed remote success: record the result for the frozen action plus event reason `completed_binding_changed`. Unknown stays UNKNOWN. Stale generation/expired lease returns conflict and writes nothing; an outcome arriving after lease loss needs future reconciliation, never a blind redispatch.

No automatic UNKNOWN→READY, no same-action redispatch, no retry under another semantic key, no cancellation-as-remote-reversal claim. ACKNOWLEDGED is not success. Query attempts can repeat under fresh leases but only query/reconcile, never execute. An adapter that proves an action was not performed and wants retry must add a reviewed policy/transition; v1 does not guess it.

Dispatcher extension: forward migration `0009_dispatch_policy_outcome.sql` also permits
BLOCKED_POLICY only with a valid `dispatch` lease and empty provider reference.
It does not allow reconciliation to erase a possible remote effect. Migration 0008
and the historical 106-test ledger acceptance remain unchanged; see the dispatcher
contract for callback ownership, timing, durable budget and later acceptance.

## Locking and recovery limits

- Producer lock order: existing command advisory key → binding row → allocate operation UUID → River job → operation insert → event/audit/receipt. The job cannot become visible before commit. Replay is read-only except repairing a missing command receipt from the immutable operation.
- Worker: binding FOR SHARE → operation FOR UPDATE. Binding mutation locks binding only. No worker takes producer command locks.
- All operation result writes compare generation; lease expiry alone rejects a result even before a successor claims. Claim/complete commit outcomes may be unknown: reread state/generation using the same operation ID. Never call an adapter from an uncommitted claim.
- River is at-least-once. Neither operation fencing nor Go cancellation prevents an old in-flight remote action. Stable provider idempotency keys and current adapter policy checks remain mandatory at the later dispatcher/provider boundary.

## Acceptance gates

Real isolated PostgreSQL, ordinary runtime + worker logins (not only mocks):

1. Successful Plan creates exactly one operation/job/event/receipt; rollback, forced River insertion failure and final audit failure leave zero facts.
2. Concurrent same-key Plan yields identical IDs and one job; changed body/actor/binding conflicts; permanent operation still deduplicates if its command receipt is absent. Job args contain exactly two safe fields.
3. Tenant/store/binding FK and RLS isolation; explicit integration permissions required; buyer/issuer cannot read/mutate integration; merchant cannot change operation state/generation or River execution state; worker cannot directly update any business state/intent/event or issue identities. Mixed-role/owner/superuser pools rejected.
4. Concurrent Claim gives exactly one dispatch lease; live duplicate is busy. Expired dispatch claim only reconciles. UNKNOWN/ACKNOWLEDGED never redispatch. Terminal duplicate does nothing.
5. Wrong owner token, old generation and elapsed lease cannot complete; latest valid completion writes one event. Binding revoke before READY blocks dispatch; post-dispatch binding change preserves known outcome/UNKNOWN and cannot redirect to a new asset.
6. Forced event insertion failure rolls back claim/completion. Transaction/lock cancellation does not leak scope or orphan partial state.
7. Full existing Go real-PG race + vet suite remains passing. Review by an agent other than the implementation author.

A real River probe worker using the ordinary restricted worker login has started, consumed its exact fixture job, persisted `completed`, and stopped under the test harness. This proves queue lifecycle privileges only, not an external-operation dispatcher.

Later internal dispatcher acceptance at `8e7c4d8` adds actual River consumption,
mock callback dispatch/query policy, bounded deadlines/retries, shutdown and real
cross-process crash recovery; [123-test evidence](../docs/implementation/2026-09-20-dispatcher-acceptance.md).
This supersedes the earlier dispatcher NOT_RUN status, not the historical probe's scope.

Still NOT_RUN: production adapters/eligibility/credentials and provider sandbox/live,
global quotas/durable Retry-After, inbox/webhook ingress, authorized cancellation/requeue
UI, buyer checkout, full global gates and full T06.

## Amendment by claims-retention-purge-v1 (integrator, 2026-09-30, U08 merge)

Recorded from `contracts/claims-retention-purge-v1.md` §6 (FROZEN 2026-09-30); that file is the source of the rows.

- Clause 6 (IR-3): a terminal `meta.private_reply` operation's `request.comment_ref` and `semantic_key` may be redacted
  by U08 only (`commerce_retention_writer`, `semantic_key LIKE 'mpr-%'`); `request_hash` stays the hash of the original
  request.

## Amendment W6-05B operations ledger (unit w6-05b-operations-ledger, migration `0159_operations_ledger.sql`, 2026-10-07)

Status: IMPLEMENTED on REAL_PG with a MOCK provider (evidence class `REAL_PG`); awaiting independent (Opus) review. Source: owner decision 2026-10-07, IMPLEMENTATION-PLAN W6-05B (M21 #3, M07 #5, D6).
This amendment is the "reviewed policy/transition" the base text above says v1 does not guess. It adds a merchant surface over the existing ledger; it adds no provider client and
changes no adapter. Everything in the base contract still holds: UNKNOWN is never blindly retried, no same-action redispatch of a possibly-sent effect, no retry under another
semantic key, ACKNOWLEDGED is not success, cancellation never claims remote reversal.

### Scope and visibility
The ledger lists the caller's store's operations with `actor_kind='MERCHANT'` and `integration.operation_lane` in (`default`, `ads`), through the existing `operation_read` RLS policy and
fixed definers. The `ads` lane (providers `meta_ads`, `meta_dataset`) is read + cancel only: `query` and `retry` answer `409 lane_unsupported` there, because post_river/0015
`guard_ads_job_link` admits exactly one job per ads operation (the one in the immutable `operations.job_id`), so a follow-up job could not commit; opening the lane needs a reviewed
amendment of that guard. Payment (`BUYER_PAYMENT_QUERY`, `PAYMENT_REFUND`) and media (`MEDIA_ATTEMPT`) operations are not in the ledger (RLS hides them from the merchant; refunds keep their own
state machine). The store comes from the server-side authenticated scope, never from a request field. Reads need `integration:read`; the three actions need `integration:execute`
(both existing permissions; no automatic grant/backfill, tests grant explicitly).

### Read model (exact keys; nothing else is serialised)
`operation_id, provider, action, purpose, state, reason_code (= result_code), attempts (= generation), created_at, updated_at, object, actions`. `object` is `{kind,id}` or `null`, derived
only from internal ids already frozen in the request: `ecpay.cvs_create` -> `order`; `meta.private_reply|meta.public_reply|meta.offer_recommend|meta.dm_send` -> `conversation` (from
`inbox.outbound_messages.conversation_id`) else `claim_bundle` (`request.bundle_id`); `meta.ads.*` -> `ad_draft`; `meta.capi.purchase` -> `payment_attempt`; `meta.live_videos` -> `binding`;
`meta.live_insights` -> `live_session`; anything else, a missing key or a non-UUID value -> `null`. `actions` is `{query,cancel,retry}`, each `{available: bool, reason: <code>}`
(`reason` is `""` when available). The detail adds `events` (newest 50: `generation, state, reason_code, created_at`). Never exposed: request, semantic_key, request_hash, external_asset_id,
provider_reference, principal_id, binding ids, lease token/digest/mode, provider bodies, comment or message text, buyer names/addresses.
List order is `(created_at DESC, id DESC)`; `state` filter: `attention` (default: FAILED_FINAL, UNKNOWN, ACKNOWLEDGED, BLOCKED_POLICY, STALE_BINDING, READY), or exactly one of
`FAILED|FAILED_FINAL|UNKNOWN|ACKNOWLEDGED|BLOCKED_POLICY|STALE_BINDING|READY` (`FAILED` is `FAILED_FINAL`). The detail read works for any state of the store's visible operations.

### Actions: states and transitions (the only transitions this unit adds)
Every action: one transaction under `platform.WithScope`; `integration:execute`; `Idempotency-Key` (existing `command.Run` replay, same key + same body = same stored answer, different body =
conflict); body exactly `{"expected_attempts": n}` which must equal the locked row's generation (else `409 operation_changed`); lock order binding FOR SHARE -> operation FOR UPDATE (the worker
order); the decision uses `clock_timestamp()` after the lock; one `integration.operation_events` row and one `ops.audit_events` row; `409 <reason>` when unavailable. A new River job
(`external_operation_v1`, args exactly `{operation_id, version:1}`, queue `default`, River default priority) is inserted by the API in the same transaction and
verified by SQL (`xmin = current transaction`, exact args/queue/priority); any refusal rolls the job back with everything else.

| Action | From state | Extra guard | Result | Event reason |
|---|---|---|---|---|
| `query` | UNKNOWN, ACKNOWLEDGED, DISPATCHING with expired lease | no active lease; binding enabled at the frozen version/provider/asset; no other live job; last `query_requested` event older than 60 s | state unchanged; `generation_floor := generation`; one new job | `query_requested` |
| `cancel` | READY | none (READY means no dispatch claim exists since the last READY) | CANCELLED, generation + 1 (the table CHECK needs generation > 0 outside READY, as claim_operation's READY -> STALE_BINDING), result_code `cancelled_by_merchant`; the existing job, if any, finishes as a terminal no-op | `cancelled_by_merchant` |
| `retry` (re-open) | FAILED_FINAL | kind is in the retry registry below; binding unchanged | READY, generation kept, `generation_floor := generation`, result_code `retry_authorized`; one new job | `retry_authorized` |
| `retry` (re-queue) | READY | no live job for the operation | state unchanged; one new job | `requeue_authorized` |

Refusals (`409` unless noted; `reason` is the machine code): an operation of a payment/media lane or a non-MERCHANT row is invisible (`404 not_found`, like another store's id); `query`/`retry` of an ads-lane operation -> `lane_unsupported`; `query` on READY or a terminal state -> `not_in_doubt`;
`query` with a lease -> `lease_active`; binding disabled/changed (including a stored `binding_changed`) -> `binding_changed`; `query` with a live job -> `query_in_progress`; second query
within 60 s -> `query_too_soon`; `cancel` on DISPATCHING/UNKNOWN/ACKNOWLEDGED -> `already_dispatched`, on a terminal state -> `operation_closed`; `retry` on UNKNOWN, ACKNOWLEDGED or expired
DISPATCHING -> `reconcile_first`, on an active DISPATCHING lease -> `lease_active`, on SUCCEEDED -> `already_succeeded`, on CANCELLED/BLOCKED_POLICY/STALE_BINDING -> `operation_closed`, on FAILED_FINAL of an
unregistered kind -> `retry_not_supported`, on READY with a live job -> `already_queued`; unknown id or another store's id -> `404 not_found`; missing/insufficient permission -> `403`.

### Retry registry (policy, not a mechanism: extending it needs a reviewed row)
A FAILED_FINAL operation may be re-opened only for a kind (provider, action) listed here, because re-opening reuses the same operation, semantic key and `lc:<operation_id>` provider
idempotency key and must be unable to duplicate an effect or resurrect deleted custody:

| provider | action | basis |
|---|---|---|
| `facebook` | `meta.live_videos` | read-only Graph call; Finish is a latest-wins idempotent snapshot upsert (0118) |
| `facebook` | `meta.live_insights` | read-only Graph call; Finish is a latest-wins idempotent snapshot upsert (0113) |

Deliberately NOT registered (each needs its own reviewed amendment; the owning domain surface creates a fresh attempt instead): `meta.dm_send|meta.private_reply|meta.public_reply|meta.offer_recommend`
(the sealed dispatch copy is wiped by `inbox.wipe_send_secret` on every terminal state, so a re-opened send could not even be dispatched), `meta.capi.purchase` (`orders.clear_terminal_capi_ip` clears
the single-send identifiers on terminal states), `meta.ads.*` (publish chain steps and `ads.remote_objects` are driven by the draft state machine), `ecpay.cvs_create`
(`fulfillment.settle_cvs_attempt` marks the old attempt FAILED and allows a new attempt; re-opening the old operation would create a second parcel). The re-queue row above applies to every kind because a
READY operation has never been claimed for dispatch since it became READY.

UNKNOWN has no retry at all: `retry` answers `409 reconcile_first`. The proof that an action was NOT performed is exactly an adapter's reconcile-mode completion to FAILED_FINAL (reached through
`query`); a FAILED_FINAL of a registered kind may then be re-opened. A reconcile that finds the effect completes SUCCEEDED, which is terminal and not retryable.

### Dispatcher budget (amends external-dispatcher-v1 "bounded number of claimed generations")
`integration.operations.generation_floor bigint NOT NULL DEFAULT 0 CHECK (0 <= generation_floor <= generation)`, writable only by `commerce_integration_writer`. The dispatcher's budget comparisons use
`generation - generation_floor` instead of `generation`. Default 0 leaves every existing operation byte-for-byte unchanged. `query` and `retry` raise the floor to the current generation, which is what lets
a `reconcile_budget_exhausted` operation be queried again (otherwise an exhausted operation is cancelled by the dispatcher before any claim and a merchant could never ask again) and gives a
re-opened operation a fresh budget. Fencing is untouched: generation stays monotonic and every claim/complete keeps its generation + lease-token check.

### Gates (real isolated PostgreSQL, MOCK provider)
Store isolation of list/detail/actions (another store's id is 404, another store's grants are not enough); ads lane read + cancel only; exact DTO keys with a secret/PII canary never present; cancel refused after dispatch and after
terminal; retry refused for UNKNOWN without proof and for unregistered FAILED_FINAL; query -> reconcile proof -> retry on a registered kind runs the fake provider exactly once more with the same idempotency
key and never a second effect; two concurrent retries yield one success and one provider call; audit and operation_event rows per action; budget-exhausted operation recovers through `query`; River job
verification refuses a job of the wrong queue/args; permission `integration:read` vs `integration:execute` split. Pins: R2 migration count (81 -> 82), the external authority function inventory (89 -> 96: three private helpers + four merchant definers),
the legacy-upgrade historical-row equality (`integration.operations` gains `generation_floor:0`). Gate: `bash scripts/dev/test-local.sh --operations-queue`. NOT_RUN: real Meta/ECPay/ads providers, the admin UI (W6-U2), LIVE traffic.
