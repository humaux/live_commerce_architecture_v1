# Captured payment facts and stock commitment v1

Status: FROZEN_IMPLEMENTATION_CONTRACT, 2026-09-24, base `5eac482`. Extends
[query v1](payment-query-v1.md), not merchant payout, refunds or live admission.
Architecture sections 11–12 and I04/I05/I24 remain authoritative.

## Decisions and provider evidence

Official PAYUNi [query v2](https://docs.payuni.com.tw/web/#/7/164), read in the
browser 2026-09-24: credit PaymentType1, AuthType1 is one-time card type;
DataSourceA is complete, B incomplete. CloseStatus2 is successful capture/claim,
CloseAmt is its amount. Credit refunds expose only the LAST refund record.
RefundStatus has different meanings for ATM and credit; never share classifiers.

- Keep immutable payment attempts as initiation records (`state=PAYMENT_PENDING`
  is their original state). Current payment status derives from immutable facts;
  a late report cannot replace a CAPTURED fact with PENDING/FAILED.
- Query SUCCESS alone is not payment. Record AUTHORIZED only for a complete,
  matched credit trade with TradeStatus1 and nonempty pinned provider reference.
  Record CAPTURED only when ALL of those conditions hold and CloseStatus2 and
  CloseAmt exactly match frozen total (DataSource A, matched account/attempt,
  one-time credit PaymentType1/AuthType1, TradeStatus1, nonempty pinned reference).
  Partial/missing capture evidence creates a review case, never a full capture.
- Confirm an order only from full CAPTURED evidence, not authorization. This is
  our explicit engineering policy, not a claim about PAYUNi fulfillment policy.
- A refund hint cannot reconstruct a refund ledger. Retain it, create a sticky
  review case and prevent/hold fulfillment; do not subtract guessed refunds,
  restore inventory, clear a prior case from an older/empty report or auto-refund.
- Keep incoming report persistence separate from local financial reconciliation:
  query report + River reconcile job in one transaction; financial facts + stock
  + order + durable fulfillment work item in another transaction. No network I/O
  holds business locks. The fulfillment work item is merchant work, not a carrier
  request or proof of shipment; later API fulfillment uses the same frozen order.

Rejected: treating authorization/query success as capture; a second stock writer
or planner; mutable last-response-wins money; rebuilding refund totals from the
last refund; automatic retry of an unknown external charge. No new dependencies.

## Frozen Go/wire and job interfaces

PAYUNi Observation adds optional fields (omitempty, missing != zero):

- CloseAmountTWD *int64, from CloseAmt.
- CardRefundType string, CardRefundStatus string, CardRefundAmountTWD *int64,
  CardRefundDay string, CardRemainAmountTWD *int64.

JSON keys are those exact exported field names, matching existing Observation.

Only project these additions for query credit/installment rows. Amounts are
canonical nonnegative integer TWD, bounded by 199999; reject signs, decimals,
overflow and malformed nonempty values. Zero remains distinct from absence.
Credit refund type accepts empty/2/3, status empty/1/2/3/8; day is empty or a valid
19-character `YYYY-MM-DD HH:MM:SS` local timestamp, never a UTC inference.
Do not copy card numbers, PAN fragments, expiration, auth code or raw body.
Existing callback and noncard observations do not acquire card refund semantics.

QueryWorker gains an insert-only River client on its validated worker pool. In
record's existing bounded transaction: compute canonical jsonb SHA256 in PG,
InsertTx private `payment_reconcile_v1` args
`{operation_id, report_hash:lowercase_hex64, version:1}`, then call the new SQL
record signature below. Identical reports may deliver duplicate reconcile jobs;
permanent business uniqueness is in PG, never the River dedup window.

`payments.NewCaptureWorker(ctx,pool)` validates worker authority. Private job args
match the above. One bounded local DB transaction: apply → commit. No provider
or credential access. Fixed errors, rollback on failures; caller owns pool/River.
Business review outcomes commit and complete the local job; DB faults retry it.

## SQL authority and transaction boundaries (0018)

Extend record to `integration.record_payment_query(uuid,bigint,bytea,text,jsonb,bigint)`.
The last argument is the already InsertTx'd River job ID; verify exact kind/args
and canonical report hash before report+Complete. Retain 0017 guards, immutable
identity/amount/method, hash dedup, unique provider reference and final DB-clock
lease fence. All newly ingested reports require this job, including incomplete
ones. No compatibility entrypoint may silently omit financial reconciliation.
Historical observations remain valid evidence but are not automatically backfilled.

One worker-executable SECURITY DEFINER entry, fixed search_path and owner
commerce_checkout_writer (reuse the existing private aggregate writer):

- `payments.apply_capture(attempt_uuid, report_hash_bytea) RETURNS void`.

It locates only a persisted authenticated observation and its exact frozen
attempt/account/profile, not a caller-supplied amount/status/scope. Set scoped
GUCs from that record, never the job. Lock order → reservation → globally sorted
warehouse/SKU balances; query lease is not held by the separate local worker.
Apply independently re-reads and rechecks under the same locks.
It must not lock integration bindings/operations while holding order locks.
Buyer/merchant/runtime/PUBLIC have no execution or financial-table writes.

The signed adapter is the trusted verifier; the database receives its bounded
projection, not an independent cryptographic proof. Worker code is a trusted
boundary. Do not add an integration wrapper that only forwards these arguments.
Validate all observation provenance against frozen attempt/profile/environment,
and exact pinned provider reference. Reconciliation does not depend on current
merchant enablement or qualification still being active.

Append-only `payments.facts`: scoped attempt, kind AUTHORIZED/CAPTURED, amount,
currency, provider reference, profile/environment, source observation hash and
DB receipt time. Unique attempt+kind, no sum of repeated capture snapshots.
`payments.review_cases`: unique attempt+reason, first evidence retained, no
automatic clearing. Cases distinguish insufficient capture, refund history,
conflicting later report and paid allocation failure. No raw error strings/PII.

`fulfillment.payment_work_items`: one immutable identity per scoped order,
capture fact reference, state READY or REVIEW_REQUIRED. It is a durable merchant
fulfillment task, not a fake carrier job. Repeated reports cannot create another.
Once REVIEW_REQUIRED, an older report cannot reopen it. No automatic stock release.

Normal capture: reservation PAYMENT_PENDING and order AWAITING_PAYMENT → facts,
ALLOCATE ledger with reserved -qty,
allocated +qty, on_hand unchanged; reservation COMMITTED; order CONFIRMED; one
work item and checkout event, atomically. Existing immutable reservation lines
remain the committed allocation; the ledger ties each line to its capture fact.
`inventory.apply_ledger` remains the only balance writer.
The new SYSTEM_PAYMENT actor is explicitly fenced by a financial-fact FK, scoped
reservation/order provenance, private-role policy and exact allowed ledger kinds.

Late capture after EXPIRED/RELEASED or cancellation: retain actual CAPTURED fact,
set fulfillment PAID_ALLOCATION_FAILED and create REVIEW_REQUIRED work, no stock
movement and no READY item. Keep commercial state unchanged; financial truth and
fulfillment eligibility are separate. Never silently reopen a cancelled order.
Current expiry only releases DRAFT/HELD, not PAYMENT_PENDING. Automatic late-stock
reallocation is deferred until a payment-expiry/recovery protocol exists; it will
reuse PlanAllocation, not introduce a second planner.

Reports without a capture fact retain review only in payments.review_cases.
Reports indicating refund/conflicting state create/hold REVIEW_REQUIRED work
only when a genuine capture fact exists for its foreign key,
retain captured facts and allocated stock, and do not guess net payment. Refund
operations, dispute/payout facts and manual resolution are follow-up modules.
Review evaluation must precede any already-processed/no-capture early return.
Full capture with a refund hint still records the gross capture fact, but creates
only REVIEW_REQUIRED work and does not allocate stock or confirm the order.
Any refund type/status/day, positive refund amount, or remaining refundable amount
below the frozen total is a conservative sticky review signal (including
DataSource B); a remaining amount equal to total and zero refund amount alone
are neutral. A remaining amount greater than total is conflicting evidence.
Absence on a later or older report never clears review. Incomplete B
alone neither produces financial facts nor downgrades an existing capture.
Partial/missing CloseAmt while CloseStatus2 creates CAPTURE_EVIDENCE_INCOMPLETE;
a later full capture still stays on review until explicit resolution. Conflicting
complete reports after capture likewise hold work without erasing facts/stock.
Durable merchant work is readable by the same scoped merchant read boundary;
processing/resolution UI and explicit review clearance remain NOT_IMPLEMENTED.

## Acceptance gates

CF01 independently signed wire fixtures: missing vs zero, malformed/partial/full
capture, incomplete B, card refund fields and noncard isolation; no PAN retention.
CF02 real River query → durable observation+reconcile job → local worker → exact
fact/order/reservation/ledger/fulfillment work; original on_hand is unchanged.
CF03 replay/concurrency/changed report, authorization-only/partial/refund/conflict,
wrong account/profile/amount, forged jobs and runtime-role negatives.
CF04 real PG expired/released/cancelled anomaly retains capture + review with no
partial stock/reopening; capture/adjust lock concurrency. Do not claim a production
PAYMENT_PENDING expiry path or successful automatic late-stock recovery exists.
CF05 causal failures at report/job, money fact, ledger, order and work item writes
roll back the corresponding entire transaction; post-wait fences remain enforced.
CF06 full PG/race/vet + independent review. Keep prior RED logs; no live PSP,
production merchant setting, customer order, public UI or global-gate upgrade.
