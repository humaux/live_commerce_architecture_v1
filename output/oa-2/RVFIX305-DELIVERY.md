# RVFIX305 — scoped duplicate and VOID schema handoff; CI pending

## Current relay: independent review of 1a47719e4

Authority: owner RVFIX305 card and `.worktrees/rv-305/REVIEW-305.md`, both P1
findings; `contracts/order-attempt-v1.md` §4; OA-1 migration 0188's admitted
codes and atomic READY/operation creation; metareply's default `no_effect`.
This is a SQL implementation request, not an executable migration or deployed fix.

- Executor: Codex (GPT-6 family; actual model/reasoning setting not exposed),
  integration_worker responsibility, no delegation. Worktree `.worktrees/oa-2`,
  branch `unit/oa-2`, PR #305 into `r3/integration`.
- Starting head: `76497fd0ee3ccf57bd49585892b651c4f6e1ed2b`. Fetched and merged
  `origin/r3/integration@d0e7331525fec046fbe9361906fa66367ae583c6` before edits;
  already up to date. Rollback: revert this documentation implementation commit.
- RED: Gates `40895129665`, g10 job `123524293876`, failure / exit 1,
  2026-10-29 14:52:05–15:08:59 UTC. Native run head equals the starting head;
  tested merge `08bf99f4182dafcb94d94104fcdfae693d303eb7`. New current-operation
  exclusion, CHECK/inventory, fixed mapping and immediate/distinct/consent/
  no-effect witnesses all failed their own assertions. Existing policy and
  UNKNOWN assertions passed. Filtered native log: `rvfix305/review-red-job.log`;
  receipt: `rvfix305/red.log`. Foundation `40895129674` / `123522034094`
  passed on the RED head but does not execute this document suite.
- P1 duplicate predicate: require tenant/store/order/action scope and
  `recent.id<>p_operation`. Eligible READY must claim immediately after OA-1
  admission; a different recent operation still refuses. Cross-scope/order
  distractors must not block the eligible claim.
- P1 VOID persistence: replace the CHECK while retaining all 14 OA-1 values;
  add cancelled/rotated/consent_refused/invalid_request/no_effect. Pin fixed
  authority-to-paired-code mappings and committed raw consent/no-effect
  readbacks. Post-C3 VOID requires actual no-effect proof and keeps evidence.
- Tests from the RED commit are unchanged. Scope this implementation batch:
  `output/oa-2/**` only; regenerate browser ownership last before push, reporting
  any generated diff in the PR. No migration number, shared schema, contract,
  R2 pin, composition, dependency or activation edit. `LC_ORDER_ATTEMPT_ENABLED`
  remains `0`. P2 reminder NULL gap remains open under the existing PR follow-up:
  https://github.com/humaux/live_commerce_architecture_v1/pull/305#issuecomment-6498353837.
- Evidence: observed assertion RED is E3 for the document gate at the RED SHA;
  implementation GREEN and independent delta review are PENDING. Own assessment
  E0. No local tests/servers. No claim of REAL_PG/runtime acceptance.
- NOT_RUN: REAL_PG C3/C4/ACL/locks/DB mutations, scheduler/cancel/rotation/copy/
  reminder composition, provider I/O, OA-3, SANDBOX/LIVE, production.
- Humaux/coord tools unavailable: no store, canvas update or code-memory link.
  No shared task claimed or modified. Integrator relay wakes the next session
  on native Gates/Foundation completion, failure/cancel, or a >3h timeout.
  Native run IDs are recorded in the PR comment after push; no evidence-only push.

## Historical batches (receipts below do not accept the current implementation)

Authority: owner RVFIX305 card; final findings of cxf-rv-305.log; OA-2 unit
brief and contracts/order-attempt-v1.md §4. No migration assignment was added.

Role: integration_worker responsibility; RED batch authored by Codex GPT-6,
fix batch authored by DeepSeek. Worktree: .worktrees/oa-2; branch: unit/oa-2;
PR #305. Base before integration merge:
a57cdb1a76a5ccfc72defc9a8b751bc29ea1f786.
Integration: f8fd8900577815a328643ed878d88a6a247d2ab2, merged as aace3b5a5.

## P1: assertion-level RED — OBSERVED

The mutation head `873af5028` compiled and failed the nine unchanged model
tests at their behavioral assertions, plus the new document gate. Native
receipts: Gates run `40858545479`, g10 job `123400292908`, conclusion failure
(09:58Z started, 11:19Z finished). The nine failures:

- `DispatchSinglePermit` (call receipt lost: `Calls:0`), `UnknownNeverSecondSend`
  (call receipt/outcome), `StaleGenerationFence` (swallowed fence: `<nil>`),
  `RetryRefusal` (`blocked=false`), `CancelBeforeC3ZeroCalls` (terminal
  projection lost: `State:READY`), `LeaseSweep` (`Expired:[]`), `ReminderKeys`
  (order identity collision), `OutcomeClassification` (ACK→ACCEPTED),
  `LeaseExpiryOnlyUnknown` (query replaced by send).
- `C3RequestReconcileBeforeRefusal` failed at `review_test.go:33/44/52` for the
  branch-order, six required clauses and three post-UNKNOWN witnesses.

Foundation run `40858545449` passed on the same head; it executes vet and the
REAL_PG subset but not this model suite, so it is **not** RED evidence. These
are MODEL_ONLY witnesses, not database or scheduler acceptance.

## P1: SQL handoff reconciliation order — FIXED IN THIS PUSH

The handoff now branches on the locked paired state before any fresh-send
eligibility: READY/unconsumed only is dispatchable, DISPATCHING returns busy,
DELIVERY_UNKNOWN enters reconcile without steps g/h, and consumed-permit
evidence is preserved. It also pins separate cancel-after-UNKNOWN,
rotation-after-UNKNOWN and policy-loss-after-UNKNOWN paired-state witnesses in
a counterexample table. The check is document structure only; it cannot
establish SQL runtime correctness.

## Latest P1: OA policy authority — handoff corrected, CI pending

Review head `2768fb917` used legacy `inbox.check_send`, whose `0128` policy
guard rejects OA-1's `order-attempt-notification/v1`. The SQL handoff now
requires a separate `inbox.payment_dm_check_send(uuid)` owned by
`commerce_integration_writer`, PUBLIC revoked, EXECUTE only to the coordinator
owner. C3 uses it only for READY/unconsumed eligibility; the locked UNKNOWN
reconciliation branch remains ahead of all fresh-send refusal. Legacy policy
validation stays unchanged and request policy rewriting is forbidden.

The handoff requires stored OA identity/scope validation, live message authority,
privacy/consent, window and human takeover checks, with separate REAL_PG
eligible-READY positive and consent/window/takeover refusal witnesses. No
migration, executable SQL function or domain consent predicate is implemented
here; assigning and binding those authorities remains the integrator's task.

Tests-only head `4a84a5dfe82231fff4b511672a340ab25ddad655` reached the intended
new assertions in Gates `40879903464`, g10 job `123470232597`: missing policy
definer/ACL, legacy C3 validator and all four acceptance witnesses. The old
cancel/rotation/policy-loss-after-UNKNOWN assertions passed. Native job conclusion
is failure; checkout/receipt SHA `e720aee049612c36894c7a1636ee6cdda1c32e1e`.
Foundation `40879903437`, job `123468328227`, passed on this RED head; it does
not run this document assertion suite. Details: `rvfix305/red.log`.

## P2 for issue — nullable reminder keys remain open

Correction: the earlier delivery falsely claimed explicit non-NULL predicates
were added. The requested DDL still lacks both predicates and can admit SQL
NULL for a reminder key. P2 remains deferred to the integrator's schema task:
add both IS NOT NULL checks and separate omitted-key REAL_PG negatives before
schema adoption. This RVFIX follows the card's P2-comment instruction.

## Current batch record

- Executor: Codex, GPT-6 family; reasoning setting not exposed. Responsibility:
  integration_worker. No delegation. Branch/worktree: `unit/oa-2`, `.worktrees/oa-2`.
- Base: `4a84a5dfe82231fff4b511672a340ab25ddad655`. Fetched/merged
  `origin/r3/integration@d0e7331525fec046fbe9361906fa66367ae583c6` first;
  already up to date. Rollback: revert this documentation implementation commit;
  no DB change or activation exists.
- Scope: `output/oa-2/**` only; browser ownership regenerated last as required.
  Tests and their assertions remain unchanged in this implementation batch.
- Verification: current-head GREEN and independent delta review pending. Local
  tests/servers NOT_RUN by instruction. Own assessment E0; document assertion RED
  is CI evidence of the handoff defect, not SQL runtime acceptance.
- Humaux/coord/code-index tools unavailable: no store, canvas update or code link
  was performed. Evidence is retained in this worktree and PR. Integrator relay
  wakes the next session when native CI finishes; no local background process.

## Evidence and continuation

- RED: `873af5028` → Foundation `40858545449` success (not the model suite);
  Gates `40858545479`, g10 job `123400292908`, failure with all nine intended
  model assertions plus the document gate.
- Fix: restores `order_attempt.go` from `aace3b5a5` and updates the SQL handoff;
  GREEN run ids are recorded in PR #305 after the implementation push.
- Historical local checks were reported by the earlier executor; they are not
  rerun or claimed as current-head acceptance. No local tests/servers this batch.
- NOT_RUN: REAL_PG/ACL/lock barriers/DB mutations, scheduler and
  cancel/rotation/reminder composition, providers, SANDBOX/LIVE, production.
  No activation, migration, contract, R2 pin, dispatcher composition or gate
  exemption changes. Own assessment E0; static checks E1.
