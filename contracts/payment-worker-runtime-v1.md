# Payment worker runtime v1

Design 2026-09-25 on accepted `68f27ad`. This joins the existing payment query and
capture workers to a separately deployable process. It does not approve a provider,
open merchant payment methods, add callback financial authority or replace River.

## Queue ownership and rolling upgrade

Use fixed queues `payment_mock_v1`, `payment_sandbox_v1`, `payment_live_v1` for
exactly `payment_query_v1` and `payment_reconcile_v1`, selected by the immutable
attempt execution profile, never job-supplied environment. A single queue is
unsafe: QueryWorker claims before checking profile, so a wrong-profile consumer
can exhaust another attempt's lease/generation budget. Keep private job arguments
and existing atomic inserts. Both producers explicitly set their profile queue
through one shared mapping, without
changing scheduled time, attempts, receipts or idempotency. No configurable queue
name, generic dispatcher, second ledger or new dependency.

Old binaries may still insert to `default` during rollout, before the attempt
exists. A DEFERRABLE INITIALLY DEFERRED AFTER INSERT constraint trigger on
`river.river_job` resolves exact attempt/QUERY-observation linkage at transaction
commit, routes these two kinds to the frozen profile queue, and rejects any other
kind targeting a reserved payment queue. The job is invisible to other sessions
until routing and domain writes commit together. Re-read the current inserted row
by ID; reject changed kind/args or missing linkage, rather than relying on stale
NEW values. This is routing evidence, not proof of payment. No payload/state
changes. Reject payment inserts naming any queue except default or the correct
profile queue. Owner-only test relocation after insertion stays possible. Reject
non-null unique keys for routed payment inserts: existing producers are nonunique,
and a key computed using another queue must not silently retain different meaning.
No UPDATE trigger: normal River lifecycle updates remain unchanged. A privileged
operator changing routing is outside the buyer/merchant authority boundary.

Reuse the existing NOLOGIN commerce_integration_writer as SECURITY DEFINER owner,
with fixed pg_catalog search_path, fully qualified names, no PUBLIC execute and
only additional UPDATE(queue) privilege. It already has narrow frozen-attempt
and observation SELECT policies. Do not grant caller access to payment records.
`integration.payment_queue_ready()` is a worker-executable, boolean-only startup
audit of trigger timing/enabled state and active job linkage/routing across all
three profiles; it returns no credentials, IDs or reports. Private route helpers
are not executable by runtime logins. Ordinary deferred constraints must remain
enabled; forcing early validation fails closed if domain rows do not exist yet.

River tables do not exist during the current business migration phase. Add a
checksummed `post_river/0001_payment_queue.sql` phase AFTER upstream River migrations,
under the existing migration advisory lock and in its own transaction. Discover
both sets for unknown-version rejection, apply business versions first, then
upstream River, then post-River versions; use the same checksum ledger with full
relative filenames. Never edit already-applied SQL. Interrupted post phase is
resumable and its backfill/router/ledger entry commit together.

First installation takes SHARE ROW EXCLUSIVE on river_job before inspecting and
moving anything. Fail without partial backfill if a nonterminal payment job is
running, uses an unexpected queue, has a unique key, or lacks exact durable
linkage. Query linkage is the immutable attempt job_id and exact operation_id /
version args; reconcile linkage is an authenticated QUERY observation with exact
attempt_id / report_hash / version args. Reject foreign kinds in the reserved
queues. Move only default-queue payment jobs in available, pending, scheduled or
retryable state. Preserve all columns except queue. Terminal historical jobs and
all unrelated jobs remain byte-for-byte unchanged. No runtime repair/backfill.

Upstream River upgrades must rerun fresh/upgrade/router/insert/lifecycle tests.
Polling must still pick up routed old-producer inserts even if the old client's
notification names its originally requested default queue.

## Runtime and credentials

- A new `cmd/payment-worker` is separate from `cmd/api`; no HTTP listener.
- `COMMERCE_PAYMENT_WORKER_ENABLED` accepts empty/0 or 1 only. Disabled mode reads
  only this flag and exits without opening a pool, reading keys or contacting PSP.
- Enabled configuration requires `COMMERCE_PAYMENT_WORKER_DATABASE_URL`, exact
  `COMMERCE_PAYMENT_WORKER_PROFILE` SANDBOX or LIVE, and bounded concurrency
  `COMMERCE_PAYMENT_WORKER_CONCURRENCY` (default 4; canonical integer 1..16).
  The deployable CLI never enables PROVIDER_MOCK, arbitrary URLs or transports.
- Reuse the account keyring env format by moving its strict parser from
  cmd/api/accounts.go to the accounts package. Preserve API disabled behavior and
  generic error mapping; all current keyring negative tests remain applicable.
  No credential values or raw DSNs in logs or errors.
- Open only `platform.OpenWorkerPool`. Reject owner, mixed/API/checkout/hosted and
  other inappropriate credentials through the existing role gate. No API/hosted
  pool fallback and no schema migration privilege in the process.
- Before starting River, verify the expected routing trigger is installed and
  enabled and initially deferred, each active payment job matches its frozen
  profile queue, and no foreign kind is in a reserved queue. Other valid profiles
  may coexist. Fail closed before consumption; do not silently move/delete jobs.
- A small shared payment-client assembly function registers the existing query
  and capture workers, one fixed profile queue, bounded workers and existing defaults.
  Tests may inject the existing signed MOCK query transport through the existing
  QueryWorkerOptions seam; production CLI cannot configure this seam.
- SIGINT/SIGTERM stops fetching, waits a bounded 15s for work, then uses River's
  cancellation stop with a separate 5s bound, and closes the owned pool. Startup
  failures close acquired resources. Errors are fixed diagnostic codes, not
  remote response/SQL/secret dumps. Use River's existing durable retry semantics.
  Emit the fixed `payment_worker_ready` log only after successful startup and
  watchdog disarm. It carries no fields or secrets and is not a payment claim.

The query worker continues using frozen historical account, credential version
and execution profile even if current account/method qualification changes.
New-payment qualification is NOT a condition for observing an existing remote
effect. Capture uses only persisted authenticated query evidence and existing SQL.
Do not infer Paid from process liveness, query success, authorization, callback or
return. Do not release uncertain stock, create replacement attempts or refund.

## Gates and independent ownership

PW01: disabled flag-only/no resources; invalid flag/profile/concurrency/keyring;
generic errors; valid startup rejects wrong/mixed/owner roles; missing/disabled
router and stale/foreign queue data fail before processing. No live PSP request.

PW02: actual PG fresh and upgrade migrations, repeated application/checksum and
unknown-version rejection; causal running-job and malformed/orphan/unique-key
rollback; exact valid legacy job transfer and immutable fields; unrelated rows
unchanged. Old-style default inserts route after migration; no unknown kind can
be inserted into a reserved queue. A transaction inserting the job before its
attempt must commit and route correctly; a concurrently active different profile
must not be claimed by this consumer or lose operation generations. Preserve
normal non-payment River behavior.

PW03: actual River + PG signed MOCK query -> persisted observation + reconcile
job -> capture/order/stock, at least two tenants; duplicate/restart/idempotency;
default expiry and external-operation jobs remain untouched. Not a direct Work
call or fake transport-only success. Isolate this assembly fixture from other
tests' intentionally relocated pq_/pc_ jobs; do not hide production admission.

PW04: real process disabled/startup rejection and graceful signal exit/connection
cleanup; shared runtime's crash/restart and lease recovery; production binary has
no MOCK transport env. Real-provider network remains NOT_RUN without approval.

PW05: all prior Go/PG/race/vet gates plus focused API keyring regression; independent
source/security and causal test review. Record commands, exact tree, failures and
checksums, dependency/lifecycle notes and retained limitations. T11 remains open.

Notification intake/ACK is a separate contract: official ACK/retry behavior and
safe historical candidate selection are not established. No affirmative callback
ACK or direct callback-to-financial-fact shortcut is added here.

References: PostgreSQL 18 [constraint trigger timing](https://www.postgresql.org/docs/18/sql-createtrigger.html)
and [SECURITY DEFINER safety](https://www.postgresql.org/docs/18/sql-createfunction.html).
