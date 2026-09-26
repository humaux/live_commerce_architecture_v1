# LOCAL logical restore and cold-start v1

Status: FROZEN amendment 1 / ACCEPTED_LOCAL at source `c7d2703` (failed attempts retained).
See [actual evidence](../docs/implementation/2026-09-27-local-recovery-acceptance.md).
Base: `519fb16`. This bounded rehearsal contributes to
G12/G14; it does **not** complete T20, T22, or production recovery acceptance.
Approved B product-detail purchase UI and customer production remain unchanged.

## Scope and reuse

Use the pinned PostgreSQL 18 image, existing Go foundation fixtures/producers,
`migrations.Apply`, runtime constructors and compiled API/worker process helpers.
No new backup framework, library, scheduler, transaction engine or product SQL.
One repeatable `--local-recovery` selector runs the isolated real-PG gate.

The source and target are two separately created, task-owned, loopback-only
clusters with synthetic data. The target is initially empty and preserves the
source bootstrap role name and OID 10, with an independently random password.
Different bootstrap names failed native PG18 role-grant restoration; an ordinary
restored superuser does not inherit the bootstrap grantor exception. No supplied database URL, existing customer cluster,
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
   and exact fresh target identity before execution. Preserve and hash the raw
   passwordless roles dump. Derive a second input by removing exactly one full
   `CREATE ROLE` statement for the verified synthetic bootstrap role, which
   already exists on the target. All other bytes, including every `ALTER ROLE`,
   `GRANT` and `GRANTED BY`, must remain identical. Missing/duplicate/mismatched
   bootstrap creation or either bootstrap OID not 10 fails before `psql`.
   Hash and recheck both raw and derived inputs; test these negative cases.
   Reject unexpected password clauses before execution. Unexpected target
   bootstrap password drift fails; do not repair it and continue the gate.
   Restore globals with
   `psql -X --set ON_ERROR_STOP=1`, then the database with
   `pg_restore --single-transaction --exit-on-error`. Do not use `--clean`,
   `--create`, `--no-owner` or `--no-acl`; no application migration before raw
   restore comparison. Any tool error fails the gate, even if some rows exist.
   The same named bootstrap owns the precreated empty database; compare its
   ownership by name as well as all bootstrap attributes and memberships.
   Require source default
   database ACL/settings or stop (do not silently omit custom settings). Compare
   encoding/locale/provider and runtime CONNECT; runtime must have neither DB
   ownership/CREATE nor authority to assume the target bootstrap role.
4. Compare raw restored data and privileges before running `Apply` twice. It
   must be idempotent and must not repair a silently incomplete restore.
   Newly restored LOGIN passwords are absent. The pre-existing target bootstrap
   retains only its independently generated password; prove the source bootstrap
   password cannot authenticate. Set new random test passwords only for
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
| LRC03 authority and keys | Role attributes/membership including bootstrap and grantor names (passwords excluded), database/schema/object/function owners, ACL, RLS policies and security-definer settings preserved by name, not OID. Restored runtime can perform an authorized read, cross-tenant/revoked access still denied. Passwords not exported. Correct separately supplied test key decrypts; missing/wrong key cannot project an event. |
| LRC04 migration/readiness | `Apply` twice after raw comparison changes neither rows nor migration ledgers/sequence evidence. Existing runtime readiness passes only for the correct restored role/pool combinations; source/target cross-pool identity check is denied even when logical database IDs match. |
| LRC05 bounded cold start | Compiled production process starts with relevant writers disabled and reaches observed HTTP/process readiness without changing business/queue facts. Explicit local Meta consumer then handles one selected restored event exactly once across restart; payment/expiry/external facts and unrelated tenant data unchanged. |
| LRC06 reproducibility | Author run plus independent root PG/race run, source review, actual exit codes, hashes and owned-fixture cleanup evidence. Full regression/race/vet before merged local acceptance; UI/browser evidence is reusable only if relevant source unchanged. |

Rows are compared without ignoring lifecycle columns. Catalog comparison must
normalize object names/role names instead of comparing cluster-specific OIDs.
Relation ACLs compare native `aclexplode` entries (grantee/grantor names,
privilege and grant option), substituting `acldefault` only for a NULL ACL:
type `r` for tables and `s` for sequences. Preserve empty-vs-default distinction.
Prove explicit/default equivalence and detection of owner privilege removal,
non-owner grants and grant-option changes. Include database owner in snapshots
after restore and after migration, not only target provisioning checks.
If an exception is necessary (e.g. independently provisioned bootstrap password), name it explicitly;
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
  roles-only/no-role-passwords; bootstrap creation requires explicit handling.
- [PostgreSQL 18 GRANT](https://www.postgresql.org/docs/18/sql-grant.html) and
  [the matching native restore report](https://www.postgresql.org/message-id/CA%2BC_kKWHMP4c56jx1BPvP1jmjp2pmBu0Cw07fPVECUmkJSnT4w%40mail.gmail.com):
  only the real bootstrap grantor is exempt from explicit ADMIN OPTION.
- [PostgreSQL 18 pg_restore](https://www.postgresql.org/docs/18/app-pgrestore.html):
  strict error handling and single-transaction restore; preserve owners/ACL.
- [PostgreSQL 18 ACL functions](https://www.postgresql.org/docs/18/functions-info.html)
  and [privileges](https://www.postgresql.org/docs/18/ddl-priv.html): NULL means
  the built-in default ACL, not an empty grant set.
