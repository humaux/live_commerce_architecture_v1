# RVFIX305 — OA policy handoff fix; current CI pending

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
