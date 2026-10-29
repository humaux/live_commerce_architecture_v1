# RVFIX305 — assertion RED observed, fix batch

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

## P2 for issue — predicate tightened, negatives still open

The reminder identity CHECK now requires both `client_reminder_key` and
`reminder_intent_key` to be non-NULL for reminder rows (explicit predicates, not
SQL three-valued pass-through). Negative REAL_PG cases for each omitted key
remain NOT_RUN until the migration exists.

## Evidence and continuation

- RED: `873af5028` → Foundation `40858545449` success (not the model suite);
  Gates `40858545479`, g10 job `123400292908`, failure with all nine intended
  model assertions plus the document gate.
- Fix: restores `order_attempt.go` from `aace3b5a5` and updates the SQL handoff;
  GREEN run ids are recorded in PR #305 after the implementation push.
- Local: gofmt, `GOOS=linux go build`, `GOOS=linux go vet ./...`, contractdrift
  and prepush only. No local tests/servers.
- NOT_RUN: REAL_PG/ACL/lock barriers/DB mutations, scheduler and
  cancel/rotation/reminder composition, providers, SANDBOX/LIVE, production.
  No activation, migration, contract, R2 pin, dispatcher composition or gate
  exemption changes. Own assessment E0; static checks E1.
