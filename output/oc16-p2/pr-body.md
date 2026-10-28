Closes humaux/live_commerce_architecture_v1#219

Item 1: stored bank-transfer plans previously fell through the terminal error table and retried after closure. Map the exact PT422 bank_transfer_closed pair to payment_unavailable so the existing executor rolls back and records a fenced ABORT receipt. All existing test assertions remain intact.

RED: tests-only head a3b8c31044eefb156eb7607ef1be6d7735185796, Gates run 40628067835/job 122622547420, synthetic checkout cb89258d27e6bcf4c824cd84ddaa38bc8e1ca056. New TestOrderAttemptExecutorAbortClassification/PT422/bank_transfer_closed fails on its own assertion: got empty string, want payment_unavailable. Raw receipt: output/oc16-p2/red.log.

Prior implementation evidence (head 83deaff3ca56ac4a0832b19cb9e2531baec10546 only): Gates 40638790619/unit job 122685375787 passes the exact new bank_transfer_closed classification test, and all matrix/aggregate/receipts jobs succeeded; required job 122782931961 was queued at inspection. Synthetic test SHA: 245f0687b2472ddf961bb0e17d89b843d1407ec3. Foundation 40638790649/job 122651820013 succeeds but skips real PG. This is historical evidence, not acceptance of the current head.

Added regression tests pin all commerce roles against DELETE/TRUNCATE on both bank tables and UPDATE on six immutable bank snapshot columns; reusing a pre-closure key with a different body must raise exact PT409 idempotency_conflict and preserve the original receipt. Native grants (0088:131–134) and receipt guard (0202:54–61) already implement these behaviors.

Batched mutation RED: head 96a4a04d3359f4da52692160b78b7c38eef4f6e8, Gates 40680080856/job122821418704 (shard:g09), synthetic c1b634784a65ff4a2ff6a290dcffb9dfa2bf1c9e. Both new assertions fail: 10 forbidden privileges at line102, and old-key/different-body returns nil instead of PT409 at line287. Raw filtered receipt: output/oc16-p2/batch-red-job.log. Final implementation removes only the two rollback-scoped SQL mutations; all assertions remain. E3 applies to RED only. Current-head GREEN and contractdrift PENDING; author acceptance E0. Local tests/servers NOT_RUN; static formatting checked.

Skipped under the card's frozen/other-module-contract boundary: item 2 buyer error-code alignment requires a decision on contracts/buyer-checkout-options-v1.md (FROZEN), contracts/storefront-v2.md and the admin error table. Integrator must own those contract changes; the buyer and merchant exact bank_transfer_closed assertions remain intact. Comment item 3 depends on item 1's terminal mapping; no migration or enabled=false policy exception is introduced. OC-15 public error-code entry and exact real-route assertion already exist on base (internal/httperror/error.go:199; tests/foundation/bank_transfer_closed_test.go original lines 146–148), so they are skipped as already fixed. Do not close/merge until remaining units are resolved or explicitly scoped out by integrator.

Outside card write paths: none. No migrations, frozen files, test weakening or dependency changes. Independent integrator acceptance and production actions NOT_RUN. Rollback: revert this PR's commits through normal review; RED SQL mutations roll back automatically. Execution and next-session handoff: output/oc16-p2/handoff.md and red.log. Humaux memory/coord/canvas unavailable in this resumed runtime; prior stored receipts do not establish current acceptance. Integrator owns the CI wake-up. Final implementation push consumes the remaining batching budget. No evidence-only push after GREEN.

Final implementation CI run ids will be recorded in a PR comment after allocation; integrator relays completion. No GREEN claimed.

🤖 Generated with Codex (dispatched by Claude Code integrator).
