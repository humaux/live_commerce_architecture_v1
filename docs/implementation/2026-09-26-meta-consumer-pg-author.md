# T07 Meta social consumer independent PG/River acceptance author

Status: **FOCUSED REAL_PG/RACE PASS; MC07 FULL RELEASE GATE NOT_RUN by this author**.
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
Actual exit: **0** after the final source fixes and committed test source
`b83affe`. This starts a task-owned,
loopback PostgreSQL 18 container, applies migrations twice, runs the focused
Go foundation selector under `-race -count=1`, and removes its labelled
container. Package result `ok livecommerce/tests/foundation 10.274s`.
Log: `/Volumes/data/output/meta-consumer-pg-final-20260926.log`;
SHA-256 `a777a009032593e57a34780a32760ac3deca49c1e93519118c353353255fff70`.
Test source SHA-256 `068ddab35cbb28529145d1764b10e27853fa2422b6ff1fe2ae2280df70964583`.
An earlier passing log before the existing-conversation sequence fault was
added remains at `/Volumes/data/output/meta-consumer-pg-author-20260926.log`
(SHA-256 `b9d86a41703ef851120849c5323e8ef3b6a058f9d692ac0d0acc4fe198529292`).

| Frozen gate | Direct evidence in `tests/foundation/meta_consumer_test.go` |
| --- | --- |
| MC01 | `TestMetaConsumerAuthority`: dedicated pool succeeds; owner, runtime, ordinary River worker, ingress, registrar and curator fail Go admission. Mixed consumer/runtime and consumer/ingress, reverse ingress/consumer, SET-capable membership, predefined `pg_read_all_data`, and inherited custom object owner fail Go/SQL checks. Direct social/private reads and social inserts denied. `TestMetaConsumerSameEventLockAndFinalAuthority`, `TestMetaConsumerWarmedFinalRoleChanges`, and `TestMetaConsumerFinalCommitDatabaseOwner` exercise original and warmed backend GRANT, REVOKE, indirect membership, SET option, and database ownership after finish before COMMIT; each has zero fact/terminal/audit and unchanged sequence. |
| MC03 | `TestMetaConsumerRiverPageAndInstagramFacts`: actual River client uses a separately validated ordinary worker pool and dedicated consumer pool. Page and Instagram messages/comments become distinct scoped facts. Two stores, two tenants, distinct assets and second app are independently asserted. Exact copied AEAD envelope decrypts with original event AAD to the verifier's canonical plaintext. Identity sessions, orders, payment attempts and integration operations have unchanged counts. |
| MC04 | `TestMetaConsumerReverseSequenceReplayAndPurge`: later receipt materializes as server sequence 1, earlier as 2; both source occurred/received times remain exact, replay does not advance sequence, and source-body purge leaves durable social history plus ALREADY with all other load fields NULL. `TestMetaConsumerConcurrentDifferentEventsSamePeer` and `TestMetaConsumerSameEventLockAndFinalAuthority` use observed PostgreSQL lock waits for both distinct-message and same-event races. `TestMetaConsumerSQLIdentityAndRollback` and `TestMetaConsumerCommentRollbackAtEveryWrite` inject real PostgreSQL trigger errors at conversation, message/comment, event terminal and audit writes; fact, sequence, terminal and audit roll back. `TestMetaConsumerCommitGuardActuallyFires` removes the source envelope only after finish and proves deferred COMMIT rejection and rollback. |
| MC05 | SQL load/finish invalid job, attempt, family, subject and family/kind mismatch return 22023. `TestMetaConsumerRouteBindingAndProofWaitFences` observes real blocking on route/binding locks then commits disabled route, changed binding version or expired proof; all return STALE with no materialization. `TestMetaConsumerRescuedAttemptAfterJobLockWait` observes a River-row lock wait, then a rescued attempt returns 22023. `TestMetaConsumerProofExpiresBetweenFinishAndCommit` verifies the database clock crosses the proof deadline after finish; COMMIT returns PT409 with no fact, terminal or audit. |
| MC06 | `TestMetaConsumerCryptoFailureKeepsPendingBody`: actual River attempts with missing key, tampered tag and AEAD-valid but classifier-invalid plaintext become retryable, never processed; age-shifted pending source ciphertext survives curator purge. `TestMetaConsumerReviewedAndStaleNeverProcess`: REVIEWED/STALE load rows expose zero context fields, finish returns PT409, and neither state produces processed fact/audit. |
| MC07 | Focused real PG/race and fresh double migration PASS. Full repository PG/race/vet and independent source release verdict are **NOT_RUN by this author**; the root integrator must replay them after cherry-picking this commit. |

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

The disposable database owner is restored before fixture cleanup. Synthetic
roles, triggers, functions and River clients are cleaned by test scopes; no
provider calls or production changes were made.
