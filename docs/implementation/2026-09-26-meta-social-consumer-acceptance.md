# T07 Meta social consumer — bounded implementation evidence

Status: **IMPLEMENTED / FINAL_MC_ACCEPTANCE_PENDING**. Contract:
[Meta consumer v1](../../contracts/meta-consumer-v1.md). This is a local,
read-side component, not a public Meta connection, sending engine or complete
SaaS deployment gate. No customer production state was changed.

## Dependency and authority path

`River(meta_inbox_v1)` using an ordinary worker pool calls `ConsumerWorker`
using a different, dedicated consumer pool. The latter can only call two fixed
definers: `load_social_event` and `finish_social_event`. Decrypt and replay the
existing classifier between them in one bounded transaction; final constraint
triggers validate scope, current route/proof, running attempt and exact copied
ciphertext before COMMIT. The source event remains the AAD authority.

`0029_meta_social_consumer.sql` owns the separate social conversations, messages
and immutable comment observations. It never links buyer identities or writes
webchat, support, order, stock, payment or outbound consent/window state. No
external call, extra dependency, broker or new service was introduced.
See [dependency map](dependencies.md) for maintenance callers and rerun gates.

## Independent engineering record

- Integrator froze `23a37bb` after independent design closure at `80e678f`.
  SQL/platform source: `f71f7eb`; field/final-authority fixes: `149ebea`.
- `integration_worker`, actual `gpt-6-sol/high`, Go author `93d0086`, integrated
  as `6a3fa00`. Isolated branch `commerce/meta-consumer-go-20260926`, existing
  worktree `/Volumes/data/worktrees/commerce-meta-inbox-go-20260926`, base
  `23a37bb`. Only four consumer/projection Go files and its
  [author record](2026-09-26-meta-consumer-go-author.md) were authored there.
- Independent `test_worker`, actual `gpt-6-sol/high`, base `f71f7eb`, branch
  `commerce/meta-consumer-tests-20260926`, existing worktree
  `/Volumes/data/worktrees/commerce-meta-inbox-tests-20260926`; test-only
  `tests/foundation/meta_consumer_test.go` and its author record are reserved.
- Independent `security_reviewer`, actual `gpt-6-astra/high`, read-only root
  review. No recursive delegation; at most two simultaneous implementation
  writers. Root reruns actual PG/race/vet rather than relying on author claims.

## Failures retained and repaired

1. Fresh business migration runs before River's own table migration. A declared
   `river_job%ROWTYPE` prevented initial migration; the function now uses a
   runtime record. `meta-consumer-migration-smoke-1.log` retains the failure;
   second run passed all 20 existing MI tests in 5.564s.
2. `RETURNS TABLE` scope names collided with unqualified body columns on READY.
   `149ebea` qualifies them and adds final authority validation. Creation-only
   migration tests could not establish the READY path; actual River tests do.
3. A repeated commit guard based on `pg_has_role` allowed a real concurrent
   mixed-authority GRANT between finish and COMMIT. The trigger was installed
   and firing; a separate missing-body test proved commit rollback. `67d4dbf`
   adds an authoritative fresh recursive catalog predicate in a separate
   VOLATILE/READ COMMITTED statement, preserving the original checks. The
   unchanged causal failing case then returned 42501 with no committed fact.
   `9be8393` explicitly includes current-database ownership, which is not an
   ordinary `pg_auth_members` row.
4. A custom object-owner role inherited with `SET FALSE` passed the Go startup
   validator, while SQL correctly denied it. `6f04653` rejects privilege reached
   through either SET or inherited USAGE; ordinary roles retain their previous
   validation rules. Independent pre/post tests retain this exact boundary.

For (3), stale authority is directly reproduced; the detailed cache invalidation
mechanism is an inference supported by official PG18 sources, not backend
tracing: [membership cache](https://github.com/postgres/postgres/blob/REL_18_STABLE/src/backend/utils/adt/acl.c),
[relation-lock invalidation](https://github.com/postgres/postgres/blob/REL_18_STABLE/src/backend/storage/lmgr/lmgr.c),
[SPI snapshots](https://github.com/postgres/postgres/blob/REL_18_STABLE/src/backend/executor/spi.c).
The fresh predicate uses the documented
[membership options](https://www.postgresql.org/docs/18/catalog-pg-auth-members.html).
A first-time diagnostic trigger can refresh caches and is not evidence that the
original warmed guard was current; the unchanged warmed regression is decisive.

## Evidence and remaining gate

Root source `983eabe` before the final authority fixes: full actual PG/race/vet,
514 top-level PASS, zero FAIL/SKIP, foundation 307.759s, exit 0;
`/Volumes/data/output/meta-consumer-root-full-1.log`, SHA-256
`fae9eee8d07972964881b9fe2d240cf2aea792418a86733eaf1c63b9a6685b55`.
This does **not** cover the newly found authority failures or final fixed source.
Final full regression and independent MC01–07 evidence must replace the pending
status before bounded acceptance. Post-`6f04653` old MI subset passed 20 tests,
5.592s, exit 0: `/Volumes/data/output/meta-consumer-root-old-mi-final.log`.

## Still outside this component

Public runtime/CLI assembly, production key loading, trusted OAuth proof,
social read UI, privacy deletion/retention policy and alerts, outbound policy,
provider qualification, and full SaaS release remain separate gates. No inbox
or social reader is publicly mounted. Comment observations do not pretend to be
an authoritative latest platform snapshot; server sequence is materialization
order, not provider chronology. Browser acceptance is not claimed for this
backend-only component and remains required for its later UI.
