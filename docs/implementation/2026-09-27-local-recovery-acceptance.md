# LOCAL logical restore and cold-start acceptance

## Current verdict

**NOT_ACCEPTED overall.** Amendment 1 author and independent root recovery
subsets pass. Full regression at `1fbd997` has one existing Meta worker-start
failure; retain this stop line until investigated. Original contract `efd9d02`;
runner selector `5cda889`.
This document records only [LRC01–06](../../contracts/local-recovery-v1.md).
The preceding [LRI acceptance](2026-09-26-legacy-runtime-isolation-acceptance.md)
at main `519fb16` is a prerequisite, not evidence that restore has passed.

No production code, migration, provider credential or customer service is in
this increment's write scope. B product-detail purchase UI is unchanged.

## Work ownership

| Work | Role / observed model | Workspace and permitted writes |
|---|---|---|
| Contract, runner, integration | root integrator | `commerce/local-recovery-20260927`; contract, runner and this evidence record |
| Source inventory and implementation | platform_explorer → test_worker, `gpt-6-sol/high` | `commerce-meta-inbox-go-20260926`, branch `commerce/local-recovery-tests-20260927`, base `efd9d02`; only `tests/foundation/local_recovery*.go` |
| Independent counterexample design | test_worker, `gpt-6-sol/high` | read-only until a candidate and exact test ownership are assigned |
| Security preflight/final source review | security_reviewer, `gpt-6-astra/high` | read-only; cannot fix their own findings and approve without new evidence |

Existing free worktrees were reused; no new dependencies or recursive agents.
The root controls the shared contract and runner. Heavy PG runs are sequenced.

## Frozen preflight decisions

- Native full custom DB archive plus passwordless roles dump, with source data
  and role mutation quiescent. No archive uploaded from outside this test run.
- Amendment 1 replaces the failed different-bootstrap design with the same
  bootstrap name/OID 10 and independent random target password. Only the single
  exact bootstrap `CREATE ROLE` is omitted from a separately hashed derived
  input; raw artifact and every ALTER/GRANT remain intact. Negative controls
  reject missing, duplicate, mismatched or non-bootstrap targets. Compare
  bootstrap authority and DB ownership exactly; custom DB ACL/settings fail.
- Strict single-transaction DB restore; no `--create`/`--clean` and no owner/ACL
  stripping. A failed globals or DB restore discards that test target.
- Compare the raw restored rows/catalogs before running `migrations.Apply`.
- Same-ID wrong key must cause an actual retryable event attempt; correct key
  supplied independently must process that same retained pre-backup event.
- Preserve the prior unexplained one-off 8-second Meta process-readiness failure
  in the LRI record. This slice must not silently increase existing per-process
  deadlines or claim that earlier failure has a proven root cause.

## Evidence to fill from actual execution

Early compile-only candidate passed `go test -run '^$' ./tests/foundation`;
this executes no PG acceptance. Before its first PG run, review identified:

- Fixed-name runtime lookup must use the actual restored LOGIN name.
- Restart proof must observe an actual re-delivery attempt, not a 200 ms sleep.
- Final foreign equality must cover all untouched tables/queues/sequences,
  not only a hand-picked subset; authorized reads must execute scoped SQL.
- Source cleanup must use its recorded immutable container ID, not only name.
- Role membership evidence must include grants of built-in `pg_*` roles to
  application logins; include column ACLs, not just relation ACLs.
- DB probes need bounded contexts in addition to command/package deadlines.

Independent source review at `d4522f5` closed the exact-ID and catalog P1s;
`2441652` closed narrower Meta/social mutation evidence, and `caf2366` closed
the remaining inherited unbounded-probe P2. These were candidate-test findings,
not production failures. Amendment 1 implementation needs a fresh review.

Three short author real-PG attempts are retained, none is a passing restore:

- Initial `d4522f5`: `pg_dumpall -d postgres` treated `postgres` as a connection
  string. The native error was a missing `=`. Corrected to the native `-l`
  database selector at `542740c`; no gate relaxed. Log
  `/Volumes/data/output/local-recovery-author-first-20260927.log`, exit 1,
  SHA256 `aacc070a19264e224c9d5e4362d9fab596e4045f3a89c77770b77b5b2ebd95df`.
- Diagnostic replay before fixing the selector: same native error, exit 1;
  `/Volumes/data/output/local-recovery-author-diagnostic-20260927.log`, SHA256
  `d4d885294914bc5285acd25f666325166811e7b5ba4d1b249077136b1b34af2b`.
- `542740c`: PostgreSQL 18.6 native roles/full DB dump completed (7,322 / 789,194
  bytes), then pre-restore metadata query failed: `pg_database` has `encoding`,
  not `datencoding`. Log
  `/Volumes/data/output/local-recovery-author-second-20260927.log`, exit 1,
  SHA256 `1f918c2cb2829e2dd0d1f544aeda099a9f37fe43f046cda1c6130829fbd525c9`.

Author stopped PG reruns for consolidated corrections. Independent `22917fb` adds real-PG
snapshot mutation counterexamples (built-in role, column ACL, sequence state)
plus a one-byte artifact mutation test. Both PASS in root `8fe2791` focused1
(2.21s / 0.00s) and `caf2366` diagnostic2 (2.09s / 0.00s); independently read back.

Root runs both exit 1 at native roles restore, before DB restore:

- `/Volumes/data/output/local-recovery-root-focused1-20260927.log`, SHA256
  `0c9a2d6b84995aad98b3a08b0d18e0ee596a4df3fcc9fa06cc03d834a9cef79b`.
- `/Volumes/data/output/local-recovery-root-diagnostic2-20260927.log`, SHA256
  `ca0522e50e6baebf42f3a78c472acc13789f8f755972063ac1fd8cdae5e341c7`.
  Restricted stderr (0600, 158 bytes) hash
  `8470e356b8aaeaae47f3f2230b781b49c7eccc1e320a69de9a13d64650ef3698`.
  Root cause: preserved `GRANTED BY` names the source bootstrap, restored as a
  non-bootstrap superuser on the differently named target; PG18 requires
  explicit ADMIN OPTION in that case. No errors/grants will be ignored or
  rewritten. Amendment 1 addresses the provisioning assumption, not product SQL.

Amendment candidate `ade2fed` author run and root diagnostic `a35b6c3` both
completed native roles and DB restore, then stopped at strict relation catalog
comparison. Logs (exit 1):

- `/Volumes/data/output/local-recovery-author-bootstrap-20260927.log`, SHA256
  `be3d69e4f746ff543188d9a197da0c576ca33d4750ea3da45f5cc973f58d8f9a`.
- `/Volumes/data/output/local-recovery-root-catalog3-20260927.log`, SHA256
  `eec1e24cdae487d9d61b5076add21f4966215b06d2a22fe0fa3457ee9cba9d51`.

Root inspected the retained 0600 catalog JSON: exactly four of 103 relations
differ, all four `river_migration` tables. Source has an explicit owner-only
default ACL; target has NULL (native default), with identical owner/RLS flags.
Use PostgreSQL native default expansion and compare every grantee/grantor,
privilege and grant option; never omit ACLs. Independent mutation tests must
prove that actual privilege changes still fail. The native bootstrap ALTER
contains no PASSWORD clause. Independent review additionally requires an
unexpected clause/password drift to fail rather than be repaired, and DB owner
to be compared after restore/Apply as well as before restore. These corrections
were implemented at `5338007` (root `3a945c4`), with independent negative tests
`6bfd2c7` (root `1fbd997`). Both independent reviews closed the source findings.

## Latest actual acceptance evidence

- Author `5338007`, `bash scripts/dev/test-local.sh --local-recovery`: exit 0,
  two top-level tests PASS, foundation 19.882s. Log
  `/Volumes/data/output/local-recovery-author-acl-20260927.log`, SHA256
  `27497ceec06107f2c205c947b70a62d6ff20de43fc38b965f60ee9bd5764a330`.
- Root `1fbd997`, same selector: exit 0, five top-level PASS / zero FAIL or SKIP,
  foundation 19.805s. Log
  `/Volumes/data/output/local-recovery-root-focused4-20260927.log`, SHA256
  `afb42b91486b2961beb62db360fcdc7c98b259f58e3a9f649daffe240adc4496`.
- Root `1fbd997`, `bash scripts/dev/test-local.sh`: exit 1, 566 top-level PASS /
  one FAIL / zero SKIP. Foundation 456.654s, Meta integration 2.082s. Log
  `/Volumes/data/output/local-recovery-root-full1-20260927.log`, SHA256
  `01d60982ed9ed7d3b5e38b9463b6fc7c60fa13588d0e9c40fe80be3de13c8d92`.
  The subsequent `go vet` command did not run because the script failed closed.

The sole full-suite failure is `TestMetaRuntimeIsolationTwoWayRealMaintenance`
(6.00s): the worker exits 1 before readiness. Preserved process log
`/Volumes/data/output/meta-runtime-process-meta-maintenance-f735720689cc.log`
has SHA256 `a839430c392238aebfc3217e8bd1385a01e633742e26b2d5e49aa4abf2288e9a`
and only the fixed `meta_worker_start_failed` marker. The code path narrows this
to `jobqueue.Run` / `startWorker(client.Start)`, after successful constructor
preflight; the underlying cause is not available. This is not proof of the
same cause as the earlier zero-log readiness timeout. No blind rerun, timeout
increase, or whole-LRC acceptance follows from the passing subset. A separate
bounded safe-startup-diagnostic task is in progress.

The three restricted native/catalog diagnostic files were copied unchanged
(0600) into `/Volumes/data/output/local-recovery-evidence-20260927/`; their
hashes are unchanged. Task-owned recovery container listing was empty after
the focused and full runs; no customer container cleanup was performed.
Unchanged-source comparison against `519fb16` covered apps, commands, internal
code, migrations, dependency manifests and buyer order/payment browser tests.
Prior order 23 / payment 11 browser evidence is retained, not claimed rerun.
Final test symbols were indexed and linked to the bootstrap/ACL rationale.

| Gate | State |
|---|---|
| LRC01 target/artifact guards | PASS_LOCAL focused4 |
| LRC02 rows, queues, ledgers, sequences | PASS_LOCAL focused4 |
| LRC03 roles/ACL/RLS/authorization/keys | PASS_LOCAL focused4, independent mutation controls |
| LRC04 raw restore then idempotent migration/readiness | PASS_LOCAL focused4 |
| LRC05 default-off and bounded restart | PASS_LOCAL focused4 |
| LRC06 independent/root regression and cleanup | BLOCKED: full1 Meta startup failure; no merge acceptance |

Record actual commands, source SHA, exit code, elapsed observations, log path
and SHA256. Retain failed attempts and root cause; a file or process starting
is not a passing gate. Archives contain only disposable synthetic fixtures;
do not retain or publish passwords, tokens, keys or raw event payloads in logs.

## Scope exclusions

Not a managed PITR/WAL/HA/AZ, object-storage, production key-custody or PSP
reconciliation drill; no production RPO/RTO claim. No physical-device/browser
rerun is implied. Reuse prior UI evidence only after an exact relevant-source
diff check. Full T20/T22 and SaaS release remain unaccepted.
