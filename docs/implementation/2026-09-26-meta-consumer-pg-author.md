# T07 Meta social consumer independent PG/River acceptance author

Status: **FOCUSED REAL_PG/RACE PASS including populated 0028→0029 upgrade; full repository replay after this change NOT_RUN by this author**.
Role: `test_worker`; actual model `gpt-6-sol/high`. Worktree
`/Volumes/data/worktrees/commerce-meta-inbox-tests-20260926`, branch
`commerce/meta-consumer-tests-20260926`, base `f71f7eb`. Contract:
`23a37bb contracts/meta-consumer-v1.md`. Only author-owned test and this note
are committed here. Integrator/source fixes consumed as cherry-picks:
`149ebea` (load aliases/final authority), `6a3fa00` (Go worker/projection),
`67d4dbf` (fresh role graph at deferred COMMIT), `9be8393` (database-owner
membership), and `6f04653` (Go validator inherited-owner check).

## Executable evidence

Command: `bash scripts/dev/test-local.sh --meta-consumer`.
Actual exit: **0** with the populated upgrade test and tightened assertions.
The script starts a task-owned loopback PostgreSQL 18 container, applies
migrations twice, runs the focused Go foundation selector under `-race
-count=1`, and removes its labelled container. The populated upgrade test
also starts its own labelled loopback PG 18 container at the checked 0028
ledger, then migrates and consumes a pre-existing job. Package result and
immutable final log checksum: 16 top-level `TestMetaConsumer*` cases PASS;
`ok livecommerce/tests/foundation 11.943s`.
Final log: `/Volumes/data/output/meta-consumer-pg-upgrade-final-20260926.log`;
SHA-256 `f54f7900058f44058ec9ea7dd9c3bd2353a8a0da7b6940b9139ef930336801e6`.
Frozen test source commit `0cb5a84`, SHA-256
`155867c189c1124b28f0fd07395193e424bc152c807c23dd80b389a1efff2601`.
The previous passing log, before this upgrade addition, remains at
`/Volumes/data/output/meta-consumer-pg-final-20260926.log` (SHA-256
`a777a009032593e57a34780a32760ac3deca49c1e93519118c353353255fff70`).
An earlier passing log before the existing-conversation sequence fault was
added remains at `/Volumes/data/output/meta-consumer-pg-author-20260926.log`
(SHA-256 `b9d86a41703ef851120849c5323e8ef3b6a058f9d692ac0d0acc4fe198529292`).

| Frozen gate | Direct evidence in `tests/foundation/meta_consumer_test.go` |
| --- | --- |
| MC01 | `TestMetaConsumerAuthority`: dedicated pool succeeds; owner, runtime, ordinary River worker, ingress, registrar and curator fail Go admission. Mixed consumer/runtime and consumer/ingress, reverse ingress/consumer, SET-capable membership, predefined `pg_read_all_data`, and inherited custom object owner fail Go/SQL checks. Direct social/private reads and social inserts denied. `TestMetaConsumerSameEventLockAndFinalAuthority` checks an after-finish GRANT leaves no message/comment fact, processed audit or terminal state and keeps the pre-existing sequence at 1. `TestMetaConsumerWarmedFinalRoleChanges` checks warmed REVOKE, indirect membership and SET changes leave no message, processed audit or terminal state and keep sequence at 1. `TestMetaConsumerFinalCommitDatabaseOwner` checks no new conversation/message/comment fact, processed audit or terminal state when ownership changes before COMMIT. |
| MC03 | `TestMetaConsumerRiverPageAndInstagramFacts`: actual River client uses a separately validated ordinary worker pool and dedicated consumer pool. Page and Instagram messages/comments become distinct scoped facts. Two stores, two tenants, distinct assets and second app are independently asserted. Exact copied AEAD envelope decrypts with original event AAD to the verifier's canonical plaintext. Identity sessions, orders, payment attempts and integration operations have unchanged counts. |
| MC04 | `TestMetaConsumerReverseSequenceReplayAndPurge`: later receipt materializes as server sequence 1, earlier as 2; both source occurred/received times remain exact, replay does not advance sequence, and source-body purge leaves durable social history plus ALREADY with all other load fields NULL. `TestMetaConsumerConcurrentDifferentEventsSamePeer` and `TestMetaConsumerSameEventLockAndFinalAuthority` use observed PostgreSQL lock waits for both distinct-message and same-event races. `TestMetaConsumerSQLIdentityAndRollback` and `TestMetaConsumerCommentRollbackAtEveryWrite` inject real PostgreSQL trigger errors at conversation, message/comment, event terminal and audit writes; fact, sequence, terminal and audit roll back. `TestMetaConsumerCommitGuardActuallyFires` removes the source envelope only after finish and proves deferred COMMIT rejection and rollback. |
| MC05 | SQL load/finish invalid job, attempt, family, subject and family/kind mismatch return 22023. `TestMetaConsumerRouteBindingAndProofWaitFences` observes real blocking on route/binding locks then commits disabled route, changed binding version or expired proof; all return STALE with no materialization. `TestMetaConsumerRescuedAttemptAfterJobLockWait` observes a River-row lock wait, then a rescued attempt returns 22023. `TestMetaConsumerProofExpiresBetweenFinishAndCommit` verifies the database clock crosses the proof deadline after finish; COMMIT returns PT409 with no fact, terminal or audit. |
| MC06 | `TestMetaConsumerCryptoFailureKeepsPendingBody`: actual River attempts with missing key, tampered tag and AEAD-valid but classifier-invalid plaintext become retryable, never processed; age-shifted pending source ciphertext survives curator purge. `TestMetaConsumerReviewedAndStaleNeverProcess`: REVIEWED/STALE load rows expose zero context fields, finish returns PT409, and neither state produces processed fact/audit. |
| MC07 | `TestMetaConsumerPopulated0028Upgrade` installs the original numbered SQL through 0028 with matching checksums, applies upstream River, mirrors Apply's intermediate River grants and applies all three post-River migrations, then proves `social.messages` absent before upgrade. A valid binding, route, batch, batch-event, event, raw ciphertext, event ciphertext and River job exist before upgrade; their full JSONB snapshots remain exactly equal after `migrations.Apply` twice, with only the 0029 ledger added. The actual new River consumer then completes the old job with one message and one processed audit while retaining the source body. Focused PG/race PASS; full repository PG/race/vet after this addition is **NOT_RUN by this author** and remains with the root integrator. |

MC02 strict classifier's seven-kind/Unicode/attachment matrix is owned by the
Go projection unit tests and separate reviewer; this PG author verifies the
Page/IG message/comment paths and classifier-invalid failure, not the complete
MC02 matrix. This note does not claim provider, public browser, production,
consent/sending or whole-product acceptance.

## Causal failures found before fixes

1. With the pre-`67d4dbf` guard, finish then owner GRANT of `commerce_runtime`
   to the consumer login allowed COMMIT (`err=nil`). The unchanged test now
   gets 42501 and proves zero business effects. Runtime catalog readback in the
   test verifies the deployed deferred guard function and enabled trigger.
2. Before `6f04653`, Go startup accepted a consumer login inheriting a custom
   object-owning role with `SET FALSE`; the SQL definer already rejected it
   with 42501. The same startup test now rejects in both Go and SQL.

The pre-fix failures were observed in tool output but were not saved as log
files; the paths and hashes above attest only the passing post-fix runs.

The new test's first isolated 0028 fixture omitted Apply's River grants between
upstream and post-River phases, so its pre-upgrade HTTP admission returned 503.
This was a fixture-parity defect, not a product permission change. Its initial
failing log is `/Volumes/data/output/meta-consumer-pg-upgrade-20260926.log`
(SHA-256 `b3fae94efb56288978fb4cbaa96e308c0460c7c05b7202e853500d0c2a2b2d94`);
a disposable SQL probe identified the missing River permission at COMMIT in
`/Volumes/data/output/meta-consumer-pg-upgrade-debug3-20260926.log`
(SHA-256 `5e2d79c513f21a023d025c205228483f32c8e44c83003168071535fb349f7ccd`).
The probe was removed from the final test source; after mirroring the original
grant step the populated-upgrade path passed.

The disposable database owner is restored before fixture cleanup. Synthetic
roles, triggers, functions and River clients are cleaned by test scopes; no
provider calls or production changes were made.
