# T07 Meta social consumer — bounded implementation evidence

Status: **PASS_LOCAL_MC01_07 / IMPLEMENTED_NOT_PUBLIC**. Contract:
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
   validation rules. Independent pre/post tests exercise this exact boundary.

The author's earliest failing runs were tool output only, not preserved log
files. A new controlled reproduction was therefore run on old product
`983eabe` plus test-only `b83affe` (branch head `09e7669`), with the exact same
test SHA-256 `068ddab35cbb28529145d1764b10e27853fa2422b6ff1fe2ae2280df70964583`.
It exited 1: 12 top-level PASS / 3 FAIL, foundation 9.239s. Go inherited-owner
admission, the original finish/GRANT/COMMIT case, and all three warmed role
mutations failed as expected. The earlier `983eabe` SQL also reached argument
validation instead of denying the inherited owner; this is a different stage
from post-`67d4dbf`/pre-`6f04653`, where SQL denied but Go still admitted it.
Log: `/Volumes/data/output/meta-consumer-prefixed-reproduction-20260926.log`,
SHA-256 `9c67a3ca28973d6fa742b9f57d7a716c23c6902e1b4d1c9d9c23889926dce28e`.
This is newly executed comparative evidence, not a recovered original log.

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
The final full regression and independent MC01–07 evidence below supersede
that limited baseline. Post-`6f04653` old MI subset passed 20 tests,
5.592s, exit 0: `/Volumes/data/output/meta-consumer-root-old-mi-final.log`.

Root subsequently ran actual full PG/race/vet against `c533856` (the final
product fixes plus 15 independent consumer tests): exit 0, **529 top-level
PASS, 0 FAIL, 0 SKIP**, foundation 311.584s. Log:
`/Volumes/data/output/meta-consumer-root-full-final.log`, SHA-256
`80975d274cf9dc7891f670b280f3efae2055834bcefc1210cec69b7ab13105ae`.
Independent evidence review still found MC07's populated `0028` → `0029`
upgrade missing: fresh installation and a repeated Apply are not that gate.
The test author added this path and complete ALREADY-null and negative-commit
rollback assertions in `0cb5a84`, integrated as `5e850a4`. This 529-test result
does not cover those later test additions.

The new focused run covers **16 tests**, exit 0, foundation 11.943s, including
the populated upgrade (1.74s). Log:
`/Volumes/data/output/meta-consumer-pg-upgrade-final-20260926.log`, SHA-256
`f54f7900058f44058ec9ea7dd9c3bd2353a8a0da7b6940b9139ef930336801e6`.
Final consumer test SHA-256:
`155867c189c1124b28f0fd07395193e424bc152c807c23dd80b389a1efff2601`.
The independent test author's [record](2026-09-26-meta-consumer-pg-author.md)
retains intermediate and failed fixture runs.

### Final source replay and independent verdict

Root ran `bash scripts/dev/test-local.sh` on `5e850a4`, including the frozen
upgrade test: actual exit **0**, **530 top-level PASS / 0 FAIL / 0 SKIP**,
including all 16 consumer tests; foundation **309.552s**. The runner executed
`go test -p 1 -race -count=1 -timeout=360s -v ./...` and `go vet ./...`.
Only documentation changed during this run. Log:
`/Volumes/data/output/meta-consumer-root-full-upgraded-final.log`, SHA-256
`453d4efabba1c9eae36b010009dfbe9a9912a5754ab2ccb13a8d13cb0f0f432b`.
The consumer test SHA above was rechecked after the run. Labelled fixture
containers were absent after cleanup; no customer service was stopped.

The independent read-only reviewer verified the exact final source, focused
and full logs, gate mapping and cleanup, then accepted **MC01–07 LOCAL** with
no open P0/P1/P2. Humaux record:
`T07 5e850a4 final independent MC01–07 LOCAL acceptance 530 PASS`.
This accepts this consumer component only, not T07 as a whole or deployment.

## Frozen gate mapping

| Gate | Actual executable coverage | Current adjudication |
| --- | --- | --- |
| MC01 | `TestMetaConsumerAuthority`, `SameEventLockAndFinalAuthority`, `WarmedFinalRoleChanges`, `FinalCommitDatabaseOwner`: dedicated/reverse/mixed/SET/predefined/object-owner authority; warmed post-finish role changes roll back facts, terminal marker, audit and sequence. | Focused PASS; source independently reviewed |
| MC02 | `TestProjectSocialAllSevenKindsAndScopedIdentity`, `RejectsWrongEvidenceAndQuarantine`, `SocialProjectionRedaction`, and consumer error tests: shared classifier, exact source agreement, seven supported kinds, Unicode/attachments, scoped identity and sanitized diagnostics. | Unit/race PASS; source independently reviewed |
| MC03 | `TestMetaConsumerRiverPageAndInstagramFacts`: actual River client with separate worker/consumer pools, Page/IG messages/comments, tenants/stores/apps/assets, original-AAD decryption and unchanged unrelated domain counts. | Focused PASS |
| MC04 | Reverse sequence/replay/source purge; observed lock waits for same event and different events/same peer; injected SQL failures at every write plus existing-conversation sequence update; deferred guard rollback. ALREADY now checks all 13 remaining output fields are NULL. | Focused PASS |
| MC05 | Exact job/attempt validation; observed route/binding/River lock waits; rescued attempt, changed route/binding/proof and proof expiry after finish prevent materialization. | Focused PASS |
| MC06 | Actual River retryable missing-key/tag/classifier failures; pending ciphertext survives purge; reviewed/stale rows expose no context and cannot process; source cleanup preserves social copy. | Focused PASS |
| MC07 | Fresh/repeated migrations; populated pre-0029 fixture with checksummed first 28 SQL files, River and three post-River migrations; eight existing row snapshots unchanged after Apply twice; real consumer completes the pre-upgrade job. Prior MI gates, final full PG/race/vet and independent source/evidence verdict all passed. | PASS_LOCAL |

## Still outside this component

Public runtime/CLI assembly, production key loading, trusted OAuth proof,
social read UI, privacy deletion/retention policy and alerts, outbound policy,
provider qualification, and full SaaS release remain separate gates. No inbox
or social reader is publicly mounted. Comment observations do not pretend to be
an authoritative latest platform snapshot; server sequence is materialization
order, not provider chronology. Browser acceptance is not claimed for this
backend-only component and remains required for its later UI.
