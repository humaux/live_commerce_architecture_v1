# Meta runtime isolation revision v1

Status: **FROZEN / IMPLEMENTATION_REQUIRED / GATES_NOT_RUN**.
This amends the failed shared-schema portions of [runtime v1](meta-runtime-v1.md),
not its other parsing, lifecycle, tenant, encryption or MR01–05 requirements.
Base candidate `8ed76d6`; observed blocker and logs are in the
[acceptance record](../docs/implementation/2026-09-26-meta-runtime-acceptance.md).
Only local isolated implementation/tests are authorized. No customer deployment,
service shutdown, credential change or production data migration is authorized.
Independent read-only preflight approved reviewed source `84cc26f`: P0/P1/P2
all zero after two bounded rounds. That freezes this contract only; the current
runtime still fails MR04 and is not approved for release.

## Decision and alternatives

Use pinned River v0.40.0's native **same PostgreSQL database, fixed `river_meta`
schema** for Meta jobs, with a dedicated `commerce_meta_worker` lifecycle role.
Both Meta clients specify `Config.Schema` explicitly; the migration runner uses
`rivermigrate.Config.Schema`. Keep the fixed `meta_inbox` queue and existing job
kind/args. Do not expose the schema as environment configuration.

| Option | Fit | Decision |
| --- | --- | --- |
| Native separate schema and lifecycle role | Isolates maintenance and SQL authority while keeping atomic PG admission | Selected; contract review closed, implementation and real gates outstanding |
| Custom driver/SQL maintenance filters or leader suppression | Requires scheduler/rescuer/cleaner/transaction-wrapper upkeep; leader suppression alone breaks retry scheduling or leaves other leaders dangerous | Rejected |
| One all-kind worker | Broadens registered work, credentials and execution scope; still changes unrelated rows in MR04 | Rejected for this contract |
| Timing changes / fixture pre-promotion | Hides the measured violation | Rejected |

Native schema support: [River documentation](https://riverqueue.com/docs/alternate-schema).
Why queues alone do not isolate maintenance:
[maintainer explanation](https://github.com/riverqueue/river/discussions/343).
Source checks use the pinned dependency, not an assumed latest API.

## Authority and Go delta

- New NOLOGIN, non-owner `commerce_meta_worker`, with no superuser, bypass-RLS,
  role/database creation or replication. A separately provisioned login inherits
  only this authority. `COMMERCE_META_WORKER_DATABASE_URL` now requires it;
  ordinary `commerce_worker` is rejected, not silently accepted for compatibility.
- Add `platform.OpenMetaWorkerPool(ctx, dsn)` and
  `platform.ValidateMetaWorkerPool(ctx, pool)`, reusing the existing authority
  machinery, deadlines, redaction and borrowed-pool rules. All role classifiers
  and SQL mixed-authority gates must recognize the new role, including inherited
  USAGE/SET-reachable roles and owner reachability. No mixed worker/ingress/
  consumer/runtime/registrar/curator role is admissible.
- Lifecycle role: River lifecycle table DML and sequence privileges only in
  `river_meta`; no migration ledger DML, business/private/social table access or
  CREATE. Only `meta_inbox.runtime_ready()` execution and required schema USAGE.
  The separate projection consumer pool remains unchanged in purpose.
- Ingress: `SELECT,INSERT,UPDATE(kind)` on `river_meta.river_job` and sequence
  USAGE, not lifecycle mutation. Revoke its corresponding old `river` grants.
  NOLOGIN Meta writer keeps only the SELECT and row-lock privileges needed by
  its fixed SQL functions, now on the new job table, not the old one.
- Ordinary `commerce_worker` gets no `river_meta` privileges. Dedicated Meta
  roles must fail startup when given cross-domain role authority; verify direct
  and inherited effective privileges in acceptance. PUBLIC grants may not
  reopen either lifecycle boundary.
- Change `NewInbox` and `NewConsumerClient` together to fixed `river_meta`.
  CLI opens `OpenMetaWorkerPool`; constructor calls `ValidateMetaWorkerPool`.
  No API shape change, new broker, worker framework, custom River driver or
  third database pool. API still starts no workers.

## Forward-only migration and cutover

Integrator owns all migrations and shared platform code. Preserve checksums of
0001–0030 and prior post-River SQL. Add `0031_meta_river_isolation.sql` for role/
schema preparation and `post_river/0004_meta_river_isolation.sql` for cutover.
The 0031 business transaction must also replace `runtime_ready()` with a
fail-closed `false` predicate. No grant enabling ingress or lifecycle use of
the new lane takes effect until the final cutover transaction. An interrupted
Apply therefore stays non-ready, including when all source jobs are absent;
never automatically restore old readiness after an error.
`migrations.Apply` retains its advisory-lock and timeout design: business SQL,
upstream `river` migration, upstream `river_meta` migration, then application
post-River transaction. Both upstream ledgers are distinct; no replay/reset.
Scope grants to each role/schema explicitly; do not grant all schemas or use
cross-schema default privileges. An interrupted upstream phase is resumable;
it must not expose a partly migrated Meta runtime as ready.

The cutover is one bounded owner transaction. Before inspecting rows, lock
`river.river_job`, `river.river_queue`, `river_meta.river_job`, then
`river_meta.river_queue` in that fixed order with ACCESS EXCLUSIVE, excluding
concurrent DML/claims/maintenance during cutover. Recheck under those locks;
do not rely on a prior count or runtime flag. This may briefly block old-schema
workers, so any production migration needs explicit impact approval; local
acceptance is not approval to interrupt customers.

1. **Before calling Apply**, stop old Meta ingress/consumer processes after an
   authorized drain. This document does not grant that production authority.
   Reject migration if any reserved source job is `running`; do not forcibly
   cancel, discard, reset attempts or recover unknown effects inside migration.
   Failure or lock timeout rolls back the entire cutover and its post0004
   checksum, including job/queue copy or deletion. Earlier 0031 preparation and
   upstream ledgers may already be committed and remain for retry; readiness
   stays false. Do not claim whole-Apply atomicity across upstream transactions.
2. Validate every source row matching reserved kind **or** queue. It must have
   exact family/args/key and a matching completed ROUTED event/job link. Poison
   rows, nonempty destination jobs or conflicting destination queue data fail
   closed; never delete them as cleanup. Already-pruned terminal jobs remain
   absent, with permanent event receipts unchanged.
3. Copy valid Meta jobs with **the same IDs and every persisted field**, casting
   only the schema-local enum type through text. Preserve state, attempts,
   timestamps, errors, metadata, tags and uniqueness columns. Do not reinsert
   with `InsertTx` or regenerate args/IDs. Preserve the `meta_inbox` queue row,
   including its paused state; do not copy old clients, leaders or unrelated
   queues. Compare source/destination complete rows before source deletion.
4. Advance the new ID sequence without lowering it: next ID must exceed copied
   IDs, retained event `job_id` references (including pruned jobs), source
   sequence high-water and current destination sequence high-water. Sequence
   gaps after rollback are acceptable, reused IDs are not. Do not change the
   old sequence or unrelated jobs.
5. In the same transaction rebind `complete_event`, `check_event`, `purgeable`,
   `lock_purgeable`, `social_source`, job-family/link guards and `runtime_ready`
   to the new table. Audit callers; retain signatures, safe search_path, owner,
   grants and transaction-bound evidence checks. Install new insert guards
   after validated historical copying so old admission XIDs are not forged.
   Keep ciphertext, nonce, AAD, event IDs, provenance and social facts identical.
   Unlike the old shared-table guard, the new schema guard applies to **every**
   INSERT: exact Meta family/args and dedicated ingress authority only. No
   non-Meta family is admissible even from the lifecycle role. UPDATE never
   changes job identity/family. Readiness rejects any foreign new-schema job,
   not merely malformed reserved jobs.
6. Remove copied source Meta jobs only after equality checks. Install a guard
   rejecting any future reserved-kind/queue insert or conversion in old
   `river.river_job`, while preserving unrelated producers. Revoke old ingress/
   Meta writer job access and ordinary-worker runtime predicate execution.
   An old binary may fail closed; it must not create a second writable Meta lane.
7. Predicate readiness verifies new exact guard metadata, active jobs and old
   schema exclusion guard; consumers must not infer readiness from empty jobs.
   Commit the application migration checksum with all cutover effects. A second
   `Apply` preserves rows and does not repeat deletion/copying. No down migration
   or production rollback-by-copy is promised.

## Acceptance gates (MIso)

| Gate | Required actual evidence |
| --- | --- |
| MIso01 authority | Fresh PG: correct roles succeed; old/mixed/owner/system/USAGE/SET-reachable authorities fail before work. Direct SQL cannot mutate the other lifecycle schema or migration ledgers. Disabled flags still do nothing. |
| MIso02 populated cutover | Build real 0030 data via old ingress, including Page/IG, duplicate/quarantine, scheduled/retryable/terminal and pruned terminal jobs; paused queue preserved. Full source/destination job equality, event/body/social equality, no unrelated change, sequence above all retained IDs; Apply twice. |
| MIso03 failure and resume | Running or poisoned reserved jobs, nonempty destination, lock contention and partial-phase interruption fail closed; no partial copy/delete or ready state. Resume on the same isolated fixture without rewriting checksums. Old producer writes are rejected after cutover. |
| MIso04 real maintenance isolation | Keep deterministic MR04 due scheduled payment unchanged. Also snapshot unrelated retryable, stale-running and retention-eligible terminal jobs. Start the real Meta CLI; observe positive-control Meta scheduler/rescuer/cleaner work, not merely a sleep, then prove unrelated full rows unchanged. For the converse, stop Meta before the snapshot, then start an actual old-schema worker and observe its positive controls without changes to Meta rows. No test-only production flags. |
| MIso05 regressions | Actual API Page/IG exactly-once, bad paths/signatures, key-loss retention and restoration/restart, signal/pool cleanup; all MI/MC/MR suites, complete PG/race/vet and relevant browser regression; independent source/evidence review. |

Historical fixture compatibility is test-only. `mcPre0029Fixture` must select
the explicit old numbered/post-River baseline, not require that the entire
current repository still contains only 30/3 files. The existing populated
0028→0029 and 0029→0030 tests apply their **exact original next migration(s)**
and retain their historical old-lane/full-row assertions. They must not call
latest `Apply` and then pretend no schema move should have happened.

Seed those old databases through a narrow test-only old-lane transaction helper
using the actual original SQL functions, dedicated ingress authority and River
`Schema:"river"`, with valid encrypted envelopes independently checked by the
existing decryption helpers. Reuse existing role/route/receipt fixtures; do not
insert fabricated owner-only event receipts or bypass old deferred constraints.
This is old SQL/authority protocol coverage, not execution of an old API binary.
Do not add a production configurable schema, compatibility fallback/view,
legacy-source fork or mutable search_path solely for these tests. MIso02 then
separately exercises latest `migrations.Apply` over a populated exact 0030
baseline and validates the actual schema move. Preserve both historical and
current upgrade evidence; do not replace either with a clean install.

Tests may use legitimate isolated fixture-owner setup before the baseline
snapshot. Do not turn unrelated scheduled rows available or postpone their
deadlines to evade maintenance. Assertions must exercise the root failure.
For separate scheduler/rescuer/cleaner positive controls, legitimate linked
Meta jobs may be seeded scheduled, stale-running and retention-eligible
terminal; a paused Meta queue suppresses fetching but not maintenance. Observe
actual expected transitions/deletion on those controls within bounded defaults,
not successful process startup alone. Do not mix this paused maintenance gate
with the separate unpaused exactly-once processing gate.

## Ownership, limits and next signal

Following contract freeze, allow at most two isolated
implementation writers: integrator owns shared SQL/platform/migration runner;
Go author owns Meta clients/CLI only after shared APIs freeze. Independent test
author owns new isolation tests and necessary schema-specific existing tests;
existing upgrade tests must continue asserting their historical baselines.

Moving Meta does **not** resolve old payment/expiry/external workers' partial
kind maps in shared `river`. That cross-maintenance risk remains a separate P1
deployment gate and needs the same source-plus-process audit before release.
Do not imply whole-SaaS safety from Meta isolation. No runtime is enabled here.
Large historical migrations exceeding the current bounded Apply budget require
a separate reviewed migration plan, not a silent timeout increase or online
dual-write path. Production backup, drain, impact approval and rollout remain
separate from all local gates above.
