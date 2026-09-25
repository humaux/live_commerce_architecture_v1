# Checkout expiry runtime v1

Design on accepted `04ec2f7`, 2026-09-25. This connects the existing
`checkout.ExpiryWorker` to a separately deployable local-only process. It does not
add a new stock writer, payment timeout decision, sweeper or provider adapter.
Contract frozen after independent design preflight (no P0/P1). Implementation
and gates below remain NOT_RUN until independently verified.

## Fixed queue and old producers

Reserve the fixed River queue `checkout_expiry_v1` for exactly the job kind of
the same name. Add `jobqueue.CheckoutExpiry` and set it explicitly on the existing
checkout `InsertTx`, keeping scheduled time, attempts, transaction and arguments.
Do not consume `default`, register external-operation routes, or change payment
queues. Current API inserts the job BEFORE `checkout.begin_hold` creates its
order; therefore a BEFORE INSERT linkage guard is wrong.

Add checksummed `post_river/0002_checkout_expiry_queue.sql` using the existing
post-River migration phase/advisory lock. Take SHARE ROW EXCLUSIVE on river_job.
For active expiry rows, require a durable order with exact `orders.job_id = job.id`
and exact JSON `{order_id: order.id, generation: 1, version: 1}`. Require textual
numeric values `1` as well (JSONB numeric equality alone admits `1.0`, which Go's
integer decoder rejects). The original
generation is always 1 in this producer. Do NOT match the order's current
generation/status: payment-start advances generation, and old tasks must still
reach the existing STALE guard without releasing PAYMENT_PENDING stock.

Before moving anything, reject running active expiry jobs, orphan/malformed
linkage, non-null unique keys, unexpected queues, or foreign kinds in the reserved
queue. Move only default expiry rows in available/pending/scheduled/retryable to
the fixed queue. Keep every other field, all terminal history and all unrelated
jobs unchanged. Backfill, trigger and checksum commit atomically. No runtime repair.

Add private `checkout.expiry_job_linked(bigint) RETURNS boolean`, and private
`checkout.route_expiry_queue_v1()` behind a DEFERRABLE INITIALLY DEFERRED AFTER
INSERT constraint trigger `checkout_expiry_queue_route_v1`. Re-read the final job
under lock, reject missing row or changed kind/args versus NEW, and permit only
default or exact expiry queue with valid immutable linkage and no unique key.
Reject foreign kinds targeting the reserved queue. No UPDATE trigger; ordinary
River lifecycle and owner-only negative fixtures remain possible. Old producers
route at commit when their order exists. A notification on the old requested
default queue must still be handled by the new consumer's normal polling.

All three functions belong to existing NOLOGIN `commerce_checkout_writer`, fixed
`search_path=pg_catalog`, fully qualified names, no PUBLIC execute. It already
reads orders and River jobs; grant only additional `UPDATE(queue)` on river_job.
Expose only `checkout.expiry_queue_ready() RETURNS boolean` to commerce_worker.
It audits exact enabled/deferred trigger identity/definer owner and all active
expiry rows/foreign reserved kinds, including non-null unique keys; no order
details or mutation. Private helpers
are not executable by API, buyer, hosted or worker logins. Preserve RLS/role gates.

## Process and reuse

`checkout.NewExpiryClient(ctx, pool, concurrency) (*river.Client[pgx.Tx], error)`
validates worker-only pool, canonical concurrency 1..16, bounded 5s queue audit,
then registers only existing ExpiryWorker on the one fixed queue. Discard raw
River logs as in the payment process; use fixed diagnostic errors. Do not read
credentials, environment profiles, provider URLs or merchant configuration.

Add `cmd/expiry-worker`, default disabled. `COMMERCE_EXPIRY_WORKER_ENABLED` accepts
empty/0 or 1 only; disabled reads only that flag and opens no resources. Enabled
requires `COMMERCE_EXPIRY_WORKER_DATABASE_URL`; optional canonical integer
`COMMERCE_EXPIRY_WORKER_CONCURRENCY` defaults to4, bounds1..16. OpenWorkerPool only,
no migration rights, API fallback, HTTP listener or arbitrary queue option.

Two real executables need the same lifecycle. Move the already-tested payment
startup watchdog and 15s graceful / 5s cancel shutdown into a small shared
`internal/jobqueue.Run(ctx, *river.Client[pgx.Tx], readyMessage string) error`.
Keep fixed 10s synchronous-start timeout, joined/disarmed watchdog and a lifetime
context that remains live for graceful shutdown. Each caller passes a hardcoded
ready marker (`payment_worker_ready` or `expiry_worker_ready`), after Start has
succeeded. Export only shared fixed ErrStart/ErrStop; commands map them back to
their own existing/new safe diagnostic codes, retain ownership/closing of pools.
Move existing watchdog tests, do not delete/weaken them. No generic process
framework, interface with one implementation or new dependency.

Existing expiry SQL remains authority: only due DRAFT/HELD matching-generation
orders can cancel/release; NOT_DUE snoozes against DB time; stale, paid, pending,
already expired and missing orders cannot release. Replaying a committed release
must not produce additional release ledger/events. Do not equate expiry failure
or abandoned payment handoff with permission to release uncertain payment stock.

## Gates

EW01: default-off flag-only/no resources; invalid flag/DSN/concurrency, worker-role
checks, bounded startup, missing/disabled/immediate router, malformed/orphan/foreign
queue/unique-key drift rejects before consumption. Private helper and ready-function exact
ACL/RLS tests. Expiry binary never reads payment keyring/provider environment.

EW02: actual PG fresh/upgrade/repeat/checksum/unknown version; failed backfill
rolls back all moved rows/router/ledger, original due time and other fields remain.
Terminal and payment/default external-operation jobs unchanged. Old producer
job-before-order transaction routes at commit; wrong args/generation/kind/unique
key/profile queue and stale-NEW rewrites roll back; post-start old notification
is actually consumed via polling. Advanced current order generation remains valid
routing but STALE for business transition.

EW03: actual River/PG two-tenant due expiration to CANCELLED/EXPIRED with exact
balance, one release per allocation line, and one expiry event; early job snoozes
without release; stale generation/payment pending/confirmed cannot release.
Repeat/redelivery/restart after committed expiry is idempotent. Concurrent payment
start versus expiry is serializable through existing order locks: either expired
with no payment attempt, or payment pending with stock retained, never both.
Actual payment/default external-operation jobs remain unclaimed/unexecuted:
their queue, kind, args and domain facts stay unchanged, with no attempt started.
River v0.40's leader JobScheduler is global (JobSchedule has no queue filter),
so normal maintenance may promote a due job in another queue from scheduled to
available. Queue isolation is consumption isolation, not a ban on River's global
maintenance. Tests must distinguish that transition from an expiry claim.

EW04: real enabled binary readiness -> SIGTERM -> exit0/no remaining pool;
real killed process with outstanding work recovers via River lease rescue without
double release. Test clock aging must be disclosed, not sold as wall-clock SLO.
All prior payment CLI/watchdog/worker gates remain green after shared extraction.

EW05: all prior Go/PG/race/vet, affected browser order/payment regression,
independent source/security and causal test review, exact source/log checksums,
dependency/runbook/rollback documentation and owned-fixture cleanup. No customer
production or real provider call. T11/global SaaS remain in progress.

Limitations: River default crash rescue age remains one hour with a 30s scan;
this does not establish business expiry SLA under a crash. Operator observability,
capacity/recovery SLO, expiry reconciliation sweep for terminal failed jobs,
external-operation production routes and authorized deployment remain separate
release gates. Never silently change a failed/terminal job to claim acceptance.
