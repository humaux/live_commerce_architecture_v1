# LOCAL logical restore and cold-start v1

Status: FROZEN / NOT_RUN (independent prereview incorporated). Base: `519fb16`. This bounded rehearsal contributes to
G12/G14; it does **not** complete T20, T22, or production recovery acceptance.
Approved B product-detail purchase UI and customer production remain unchanged.

## Scope and reuse

Use the pinned PostgreSQL 18 image, existing Go foundation fixtures/producers,
`migrations.Apply`, runtime constructors and compiled API/worker process helpers.
No new backup framework, library, scheduler, transaction engine or product SQL.
One repeatable `--local-recovery` selector runs the isolated real-PG gate.

The source and target are two separately created, task-owned, loopback-only
clusters with synthetic data. The target is initially empty and uses a different
random bootstrap superuser. No supplied database URL, existing customer cluster,
uploaded archive or existing directory is a restore destination. Hold immutable
container IDs and verify their task labels/image/binding before restore/cleanup.
Commands and probes are bounded; cleanup touches only the recorded owned IDs.

## Native recovery sequence

1. Populate source through existing checkout/payment/external-operation and Meta
   producers. Include linked business rows and jobs in all four River schemas,
   a retained revoked/expired authorization fact, encrypted payload/credentials,
   migration ledgers, and a sequence high-water mark above surviving job IDs.
   Keep workers stopped and finish source mutations before taking evidence.
2. Capture canonical row snapshots and catalog evidence; generate a full custom
   database archive (`pg_dump -Fc`) plus native globals
   (`pg_dumpall --roles-only --no-role-passwords`). Record hashes, tool versions,
   bytes and elapsed time, never passwords, tokens, keys or raw payloads.
   Do not use schema/table filters that silently omit dependencies.
3. Accept only this run's generated artifacts, checking their recorded hashes
   and exact fresh target identity before execution. Restore globals with
   `psql -X --set ON_ERROR_STOP=1`, then the database with
   `pg_restore --single-transaction --exit-on-error`. Do not use `--clean`,
   `--create`, `--no-owner` or `--no-acl`; no application migration before raw
   restore comparison. Any tool error fails the gate, even if some rows exist.
   Database provisioning is a named exception: the new bootstrap owns the
   precreated empty database, not the source bootstrap. Require source default
   database ACL/settings or stop (do not silently omit custom settings). Compare
   encoding/locale/provider and runtime CONNECT; runtime must have neither DB
   ownership/CREATE nor authority to assume the target bootstrap role.
4. Compare raw restored data and privileges before running `Apply` twice. It
   must be idempotent and must not repair a silently incomplete restore.
   Restored LOGIN passwords are absent. Set new random test passwords only for
   the scoped logins used by the cold-start check; preserve memberships and
   NOLOGIN/owner boundaries. Restore application test keys separately from DB.
5. Prove default-off startup does not replay payment, expiry, external or Meta
   work. Then explicitly enable only one bounded local Meta-consumer workflow,
   verify exactly one social projection, stop/restart and verify no duplicate.
   No real provider, marketing send or money-moving action is permitted.
   Before the correct-key consume, use the same key ID with different bytes:
   observe an actual retryable attempt, retained encrypted body and no social
   projection or terminal marker. Then supply the separately restored correct
   key and consume that same pre-backup event. These named Meta retry/projection
   changes are expected; they do not relax foreign-family equality.

## Acceptance gates

| Gate | Required observed evidence |
|---|---|
| LRC01 target/artifact safety | Distinct source/target container and PostgreSQL system identifiers; exact owned fresh-target guards; wrong target, nonempty target and changed artifact rejected before restore; source unaffected. Native tools exit zero. |
| LRC02 lossless business/queue restore | Canonical whole-row equality for all application tables, all four River job/queue tables and migration ledgers before workers; semantic job/business IDs remain linked. Every application/River sequence retains `last_value/is_called`, including pruned high-water case. |
| LRC03 authority and keys | Role attributes/membership (excluding new target bootstrap and password rotation), schema/object/function owners, ACL, RLS policies and security-definer settings preserved by name, not OID. Restored runtime can perform an authorized read, cross-tenant/revoked access still denied. Passwords not exported. Correct separately supplied test key decrypts; missing/wrong key cannot project an event. |
| LRC04 migration/readiness | `Apply` twice after raw comparison changes neither rows nor migration ledgers/sequence evidence. Existing runtime readiness passes only for the correct restored role/pool combinations; source/target cross-pool identity check is denied even when logical database IDs match. |
| LRC05 bounded cold start | Compiled production process starts with relevant writers disabled and reaches observed HTTP/process readiness without changing business/queue facts. Explicit local Meta consumer then handles one selected restored event exactly once across restart; payment/expiry/external facts and unrelated tenant data unchanged. |
| LRC06 reproducibility | Author run plus independent root PG/race run, source review, actual exit codes, hashes and owned-fixture cleanup evidence. Full regression/race/vet before merged local acceptance; UI/browser evidence is reusable only if relevant source unchanged. |

Rows are compared without ignoring lifecycle columns. Catalog comparison must
normalize object names/role names instead of comparing cluster-specific OIDs.
If an exception is necessary (e.g. the new bootstrap role), name it explicitly;
never turn exact equality into row-count-only proof. A positive test without its
negative control cannot satisfy a security/identity gate.

## Limits and release stop line

This is a quiescent **LOCAL synthetic logical restore**, not a concurrent backup,
managed PITR, WAL archival, HA/AZ failover, object storage or real KMS recovery.
Elapsed durations are observations on this machine, not an RPO/RTO guarantee.
Production still needs approved recovery points, backup retention/encryption,
real key custody, revocation/deletion tombstone reconciliation, PSP/logistics
reconciliation and a separately authorized staged reopening plan. Never restore
an old financial snapshot over newer real provider facts or blindly replay jobs.

## Official references

- [PostgreSQL 18 pg_dump](https://www.postgresql.org/docs/18/app-pgdump.html):
  full database archive does not include cluster roles.
- [PostgreSQL 18 pg_dumpall](https://www.postgresql.org/docs/18/app-pg-dumpall.html):
  roles-only/no-role-passwords; different bootstrap name avoids role collision.
- [PostgreSQL 18 pg_restore](https://www.postgresql.org/docs/18/app-pgrestore.html):
  strict error handling and single-transaction restore; preserve owners/ACL.
