# LOCAL logical restore and cold-start acceptance

## Current verdict

**NOT_RUN.** Contract frozen at `efd9d02`; runner selector at `5cda889`.
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
- A different target bootstrap role avoids native `CREATE ROLE` collision.
  The precreated empty DB's owner is an explicit provisioning exception; source
  custom DB ACL/settings fail the gate rather than being silently omitted.
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
bounded direct main-test probes and narrower Meta/social mutation evidence are
still open P2s. These are candidate-test findings, not production failures.

Two short author real-PG attempts are retained, neither is a passing restore:

- Initial `d4522f5`: `pg_dumpall -d postgres` treated `postgres` as a connection
  string. The native error was a missing `=`. Corrected to the native `-l`
  database selector at `542740c`; no gate relaxed. Log
  `/Volumes/data/output/local-recovery-author-first-20260927.log`, exit 1,
  SHA256 `aacc070a19264e224c9d5e4362d9fab596e4045f3a89c77770b77b5b2ebd95df`.
- `542740c`: PostgreSQL 18.6 native roles/full DB dump completed (7,322 / 789,194
  bytes), then pre-restore metadata query failed: `pg_database` has `encoding`,
  not `datencoding`. Log
  `/Volumes/data/output/local-recovery-author-second-20260927.log`, exit 1,
  SHA256 `1f918c2cb2829e2dd0d1f544aeda099a9f37fe43f046cda1c6130829fbd525c9`.

Author stopped PG reruns for consolidated corrections; root will independently
run the corrected candidate. Independent test commit `22917fb` adds real-PG
snapshot mutation counterexamples (built-in role, column ACL, sequence state)
plus a one-byte artifact mutation test. Only its compile and pure-file hash case
have run so far; PG negative cases remain NOT_RUN.

| Gate | State |
|---|---|
| LRC01 target/artifact guards | NOT_RUN |
| LRC02 rows, queues, ledgers, sequences | NOT_RUN |
| LRC03 roles/ACL/RLS/authorization/keys | NOT_RUN |
| LRC04 raw restore then idempotent migration/readiness | NOT_RUN |
| LRC05 default-off and bounded restart | NOT_RUN |
| LRC06 independent/root regression and cleanup | NOT_RUN |

Record actual commands, source SHA, exit code, elapsed observations, log path
and SHA256. Retain failed attempts and root cause; a file or process starting
is not a passing gate. Archives contain only disposable synthetic fixtures;
do not retain or publish passwords, tokens, keys or raw event payloads in logs.

## Scope exclusions

Not a managed PITR/WAL/HA/AZ, object-storage, production key-custody or PSP
reconciliation drill; no production RPO/RTO claim. No physical-device/browser
rerun is implied. Reuse prior UI evidence only after an exact relevant-source
diff check. Full T20/T22 and SaaS release remain unaccepted.
