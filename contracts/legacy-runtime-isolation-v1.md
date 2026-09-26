# Payment and expiry runtime maintenance isolation v1

Status: **FROZEN / LRI01–06 ACCEPTED / LOCAL ONLY**.
Implementation and independent evidence are recorded in
[local acceptance](../docs/implementation/2026-09-26-legacy-runtime-isolation-acceptance.md).
Accepted source `51619b2`: 562 real-PG/race/vet tests, no failures or skips;
buyer browser evidence covers 23 order and 11 payment cases. This does not
authorize production migration or establish provider qualification.
Independent read-only round-one prereview approved draft SHA256
`86652a6dd379cbcd4570cc2d9257b92051fb2e94617f33c0f4cbd09b371d8cf7`
with no concrete P0/P1/P2. This approves the contract, not the implementation.
Base `c45e8a7`. This amends the shared-schema portions of
[payment runtime](payment-worker-runtime-v1.md) and
[expiry runtime](checkout-expiry-runtime-v1.md); their business, profile,
shutdown, retry, inventory and UNKNOWN-outcome fences remain mandatory.
No customer deployment, process interruption or production migration is authorized.

## Cause and selected boundary

Test-only `d644829` uses real linked production admissions, an actual expiry CLI
as sole leader, paused fetch, and positive scheduler/rescuer/cleaner controls.
Both author and root observed foreign payment-query/external-operation stale
jobs become `discarded`; a terminal payment-reconcile row was deleted. Business
rows remained unchanged during that window. Root command exited **1**, foundation
9.027s, log `/Volumes/data/output/legacy-runtime-isolation-root-real-red-20260926.log`,
SHA256 `8fc59ec77cb532732e33729d69bc4886c25b71e1ae6a5fab297501db164b8176`.
Earlier root launcher attempts skipped for missing fixture environment and are
explicitly NOT_RUN, not passing evidence. Author evidence and source remain in
the independent test worktree; do not merge an unresolved failing product gate.

Pinned River v0.40.0 rescuer selects stuck jobs by schema, not queue. An unknown
kind is discarded. EW03 already allowed foreign scheduled-to-available promotion;
that allowance is not the newly demonstrated loss. Schema-local maintenance is
the fix, not delaying a scheduler, changing rescue defaults or disabling cleanup.

| Family | Native schema | Consumption queues | Lifecycle authority |
| --- | --- | --- | --- |
| Payment query and reconcile | `river_payment` | Existing mock/sandbox/live profile queues | Existing `commerce_worker` |
| Checkout expiry | `river_expiry` | Existing `checkout_expiry_v1` | Existing `commerce_worker` |
| External operation | Existing `river` | Existing external routing | Existing `commerce_worker` |
| Meta inbox | Existing `river_meta` | Existing `meta_inbox` | Existing `commerce_meta_worker` |

Use native fixed `river.Config.Schema` and `rivermigrate.Config.Schema` in the
same PostgreSQL database. No configurable schema name, new broker, custom driver,
queue framework, new pool role or second business transaction engine.
This is **maintenance/execution semantic isolation, not database-principal
containment**: the existing ordinary worker still has the legacy families'
privileges. Do not claim a compromised worker cannot issue cross-schema SQL.
The separately accepted Meta authority boundary must remain unchanged.

All payment profiles register the same query/capture kinds. Their common pinned
production defaults for timeout, retry, rescue and retention are retained; the
CLI supplies `DefaultQueryWorkerOptions`, and no profile-specific maintenance
settings are introduced. Rescuer does not call `Work` or claim a domain generation.
Cross-profile maintenance within `river_payment` is allowed, but cross-profile
fetch/Work/domain claims are not. Tests may retain bounded injected transports;
they do not establish permission for divergent production maintenance policies.

Rejected: queue-only routing/empty-startup checks (maintenance ignores queues),
all-kind worker (expands execution responsibility), timing or pre-promotion tricks
(hide failure), and per-profile schemas/new family DB principals in this unit
(no demonstrated need, extra sequence/UNIQUE, role, RLS and pool-validator work).
Upgrade the boundary if profiles need different maintenance policies or workers
become distinct trust domains; review that explicitly before changing defaults.

## Exact integration delta

- `cmd/api/buyer.go`: insert-only checkout client uses `river_expiry`.
- `cmd/api/buyer_payment.go`: independent hosted-payment client uses `river_payment`.
  Do not add a second client field to `checkout.Service`: the two services already
  have separate constructors. Existing `NewPaymentStarter` callers also receive
  the payment client; constructor signatures need not change.
- `internal/checkout/runtime.go`: expiry consumer uses `river_expiry`.
- `internal/payments/runtime.go` and `query_worker.go`: consumer and transactionally
  inserted reconciliation jobs use `river_payment`.
- External client in `cmd/api/accounts.go`/integration core stays `river`; Meta
  producers, roles, consumers, ciphertext and source evidence stay unchanged.
- In forward SQL rebind `checkout.begin_hold`, `checkout.start_payment`, the
  six-argument `integration.record_payment_query`, payment/expiry link predicates,
  deferred routers and readiness predicates to the correct family table. Audit
  every reference/caller; retain signatures, function owner, safe search_path,
  grants, CAS, locks, amount checks, credential/profile and transaction proof.
- Preserve existing per-profile queues and immutable exact job arguments. River
  insertion plus business record/receipt stays **one PostgreSQL transaction**.
  Wrong client wiring must fail at SQL admission and leave neither job nor receipt.
  Job IDs are family-qualified references, not globally unique across schemas;
  query links must use the right table plus kind/args/domain identity, never ID alone.
- Tests/helpers using current-schema fixtures must follow the same separation;
  historical pre-cutover fixtures must keep their original `river` assumptions.
  Use explicit family helpers where an ID could collide; no blind replacement of
  every `river.river_job` reference or removal of old assertions.

## Forward-only cutover

Integrator reserves `0032_legacy_river_isolation.sql` and
`post_river/0005_legacy_river_isolation.sql`. Preserve earlier SQL checksums.
0032 prepares both schemas with PUBLIC access revoked and replaces both readiness
predicates with fail-closed false. No family use grants before final cutover.
The existing advisory lock/deadline and separate upstream ledgers remain: business
phase, native `river`, `river_meta`, `river_payment`, `river_expiry` migrations,
then one application post-River transaction. Partial failure is resumable, not
whole-Apply atomic; the preparation fence must stay false, even with no jobs.

Before any future approved production run, stop/drain old payment/expiry producers
and consumers. ACCESS EXCLUSIVE locks can block unrelated old-table activity;
production requires a separate impact/backup/approval plan. This local contract
does not grant it. The post-River transaction locks job then queue tables in fixed
order: `river`, `river_payment`, `river_expiry`. Do not lock/migrate Meta data.

Under those locks:

1. Reject any reserved kind/queue source row that is running, foreign, malformed,
   unique-keyed, mismatched-profile/identity, or lacks its exact permanent domain
   link. Validate all retained states, including terminal rows. Historical terminal
   default-queue rows may move unchanged if their original domain link is valid;
   active jobs must already match their old deferred-router queue. Expiry args
   remain generation **1**, even after payment advanced the order generation.
   JSON numeric versions/generation must also satisfy the integer decoder grammar.
   Nonempty destination job tables or conflicting destination queues fail closed.
2. Copy jobs with original IDs and **every persisted field**, only casting the
   schema-local enum through text. Copy corresponding reserved queue rows exactly,
   including paused state; retain a shared old `default` queue for external work.
   Do not copy leaders/clients or regenerate jobs. Prove full-row and count equality
   before removing sources. Already-pruned terminal jobs remain absent.
3. Advance each destination sequence monotonically beyond copied IDs, retained
   domain job references, source sequence high-water and its own high-water.
   Do not lower/rewrite the source sequence, rewrite business IDs or reset attempts.
   Sequence gaps after rollback are acceptable; ID reuse within a family is not.
4. Rebind the fixed-purpose functions above without changing business behavior.
   Install a family guard for **all** INSERTs in each new table and immutable job
   identity on UPDATE. A deferred link/router reads the final locked job and exact
   durable domain record, detects same-transaction rewrites and routes only the
   allowed default-to-fixed queue transition. No foreign kind/queue or orphan may
   enter even through the broadly privileged lifecycle role. Do not fake historical
   admission XIDs. Existing exact identity/no-unique rules remain enforced.
5. Grant only existing producer needs (`SELECT,INSERT,UPDATE(kind)` plus sequence
   USAGE) to checkout runtime on the two new schemas. Fixed SQL writer owners get
   only necessary read/lock/routing privileges. Ordinary worker gets native
   lifecycle DML/sequence access in the two new schemas, excluding migration ledgers
   and CREATE; this does not change its existing business grants. Remove obsolete
   checkout producer/writer old-schema access, while preserving external runtime/
   integration and old-schema worker access genuinely still needed. Reapply only
   documented grants after upstream migrations, not cross-schema defaults.
6. Delete validated copied source jobs/owned queue rows atomically. Replace old
   payment/expiry routers with a guard rejecting future reserved kind **or** queue
   INSERT/conversion in `river`; retain Meta's exclusion guard and external
   producers. An old binary fails closed, never makes a second writable lane.
7. Readiness checks exact enabled trigger metadata, definer ownership and safe
   search_path, both legacy-exclusion and own-family guards, and current active
   job linkage/queue. Empty tables do not bypass missing/tampered guards. Finish
   with the checksum in the same transaction. A failed copy/lock/validation rolls
   back all cutover effects/checksum, not already committed preparation or native
   ledgers. Repeated Apply is idempotent. No automatic down-migration is promised.

## Required acceptance gates

| Gate | Required proof, LOCAL unless stated otherwise |
| --- | --- |
| LRI01 causal regression | Same author RED test after only family-table binding changes becomes GREEN: actual expiry CLI, native paused fetch, own scheduled/retryable/stale/terminal maintenance positive controls, every foreign full row/existence and business receipt unchanged. Preserve RED source/logs. |
| LRI02 mutual runtime boundary | Actual payment consumer with injected MOCK transport and actual expiry CLI in fresh fixtures; exercise each as maintenance leader with reset eligible foreign jobs before each baseline. Own maintenance positive, other family/external/Meta whole rows unchanged. Payment profile queues never consume another profile; no non-MOCK provider calls. Retain shutdown/restart/crash/UNKNOWN gates. |
| LRI03 admission/readiness | Correct API producers and query-to-reconcile same-TX path succeed; wrong schema, foreign kind/queue, orphan, identity rewrite and malformed args fail with exact SQLSTATE and no partial domain writes. Old binaries cannot re-admit moved families. Missing/disabled/wrong-owner/unsafe-search_path guards, empty-table tampering and active poison make startup unready. Preserve ordinary/Meta ACL boundaries. |
| LRI04 populated upgrade | Frozen pre0032 cluster with all legitimate states/profiles, terminal default queues and pruned high references; unchanged source checksums, full job/queue/receipt equality, next IDs above all high-waters, correct post-upgrade consumption. Running/poison/nonempty/conflicting destination/lock failures and partial native phase fail closed, then corrected retry succeeds; second Apply unchanged. Explicit colliding IDs across new families cannot authorize the wrong job. |
| LRI05 full regression | All existing Go/real-PG18/race/vet gates pass with no skips or weakened safety/timeout assertions. Current versus historical fixtures distinguished; causal failure logs retained. Independent author, root rerun and security review use exact SHAs. |
| LRI06 real buyer browser | Re-run affected same-source `--browser-order` and `--browser-payment`: real Next/Go/PG and local mock PSP only; root inspect required desktop/mobile outputs. This proves purchase/order/payment wiring, not real PSP onboarding, capture, customer deployment or complete SaaS release. |

## Ownership and stop lines

Root integrator owns this contract, migrations, migration runner, shared documents
and final merge. Go author owns the five production wiring files listed above
and directly corresponding unit tests; independent test author owns explicitly
assigned foundation test files. Freeze interfaces before dispatching implementation.
Record actual roles/models/reasoning/base/worktree/write_paths and evidence in
handoffs. No recursive delegation. No merge with unresolved P0/P1; two directed
repair rounds then adjudicate. Keep the diagnostic test commit separate until the
combined implementation is independently accepted. Overall SaaS goal remains open.
