# Captured payment facts and stock commitment

Date: 2026-09-24. Status: PASS_BOUNDED_INTERNAL_CAPTURE_AND_STOCK_COMMIT.
Final source: `84c6af1`; real-PG/race/vet and independent review passed.
Contract: [payment-capture-v1](../../contracts/payment-capture-v1.md).
Baseline: `5eac482`; this is an internal credit-payment increment, not live admission.

## Changed behavior and dependency boundaries

- Signed PAYUNi credit queries project capture amount and the last credit refund
  hint without PAN/raw-body retention. Missing and zero amounts remain distinct.
  ATM/CVS refund status codes do not inherit credit semantics.
- Query observation and `payment_reconcile_v1` River job commit together.
  The queue binds the exact PostgreSQL canonical report hash, attempt and version;
  a terminal/unrelated job cannot substitute for pending reconciliation work.
- A separate, replayable local worker calls only `payments.apply_capture`.
  Verified full capture, ledger, reservation, order confirmation, checkout event
  and durable merchant fulfillment work commit or roll back together.
  Query SUCCESS or card authorization alone never confirms an order.
- The aggregate locks order, reservation, then sorted warehouse/SKU balances.
  It does not take the query path's binding/operation locks. No provider request
  or credential access is made while financial/stock locks are held.
- `inventory.apply_ledger` remains the sole balance writer: capture transfers
  reserved to allocated; physical on-hand decreases only in a future shipment
  flow. Facts/ledger/work have permanent business uniqueness independent of River.
- Original checkout session remains inventory/event provenance even if a newer
  session of the same buyer started the payment. Runtime/worker cannot directly
  insert money facts or system-payment stock movements.
- Partial/missing capture evidence and refund hints are sticky review cases.
  Actual full gross capture remains a fact even when fulfillment must be held.
  Released/cancelled anomalies retain money and `PAID_ALLOCATION_FAILED` work;
  there is no automatic stock reallocation, refund or order reopening.
- Older authorization-only reports do not undo newer capture evidence. Explicit
  adverse states, refund hints and inconsistent amounts still hold fulfillment.
- Reuse PostgreSQL, pgx, River and standard-library crypto; no new dependency,
  queue, stock planner, credential store or carrier job.

## Acceptance evidence

Logs: `/Volumes/data/output/live-commerce-payment-capture-tests/`.
All provider responses are independently signed synthetic fixtures. No real
PSP, live merchant settings, customer orders, refunds or production deployment.

| Gate | Executable evidence |
|---|---|
| CF01 | `internal/integrations/psp/payuni/query_test.go`: signed full/partial/missing/zero/malformed amounts, refund fields, dates, noncard isolation, no PAN projection |
| CF02 | `TestBuyerPaymentCaptureRealRiverSignedQueryChain`: real QueryWorker, persisted observation/job, later real CaptureWorker, facts + stock + confirmed order + durable work |
| CF03 | `AtomicReplayAndFreshSession`, `EvidenceAndStickyReview`, `LaterRefundAndReverseOrder`, `OldAuthorizationIsNotConflict`, `ACLAndObservationBinding`, `EmptyReferenceAndMerchantReadScope`, `QueryJobBindingAndRollback`, `ForgedRiverJobCancelsWithoutMoney` |
| CF04 | `ReleasedAnomalyKeepsRealMoney`, `BalanceWaitIsAtomic`, `MultiSKUSingleCommit`: explicit owner-only anomaly injection is not a production pending-expiry API |
| CF05 | `FaultsAreCausalAndAtomic`: exact injected SQLSTATE after fact, ledger, reservation, order, event and work writes; intake failure rolls back report and job; prior lease-wait fences remain tested |
| CF06 | 284 top-level PASS / 0 FAIL / 0 SKIP; script exit 0 including race/vet. Independent final review: no unresolved P0/P1 in this scope |

### Failures retained, not hidden

1. `subset-01.log`: migration failed because a River table SELECT grant ran
   before River's own migrations created the table. Grant moved to the existing
   post-River ACL phase in `migrations/migrate.go`; no broader privilege added.
2. `subset-02.log`: exit 0, 29 top-level PASS, foundation 44.414s; initial capture
   gates plus existing payment/query regression. Not a full release gate.
3. `full-01.log`: new multi-SKU fixture allocated a second item without seeding
   its SKU. The panic stopped the run. Fixture corrected to create both SKUs;
   no production assertion was removed or weakened.
4. `subset-03.log`: exit 0, foundation 66.670s, including corrected multi-SKU,
   wrong/terminal job binding and forged River job cancellation.
5. `reverse-order-red.log`: exit 1, foundation 73.507s. Root's independent test
   proved old authorization consumed after full capture wrongly held READY work.
   Independent review classified this as P1. The classifier now distinguishes
   lack of later-stage evidence from explicit adverse evidence; the test remains.
6. `full-accepted.log`: exact final source, exit 0, 284 top-level PASS / 0 FAIL /
   0 SKIP, foundation 150.976s, full `go test -race` and `go vet ./...`.

Final log SHA256:
`5eb249ec0a1d06b3b4c0a0e98ff4294382c243751d36c336bd491ebdfd70c555`.
The task-owned `livecommerce.fixture` container list was empty after execution.
No live credentials, secrets or buyer PII were placed in evidence logs.

## Ownership and independent verification

Root integrator owns frozen contract, PG tests, migration ordering integration,
reverse-order correction and final regression. Go author `payment_query_impl`
(gpt-6-sol/high) used isolated branch `codex/payment-capture-go-20260924`, base
`5eac482`, commits `7d75a12`, `d24f631`, `d5049ea`. SQL author
`payment_capture_sql` (gpt-6-sol/high) used isolated branch
`codex/payment-capture-sql-20260924`, base `60b9ed3`, commit `e1abb31`;
only `migrations/0018_payment_capture.sql`. Root integrates the shared migration.
Author static/race checks are not substituted for root real-PG acceptance.

Read-only `payment_query_review` (gpt-6-sol/high) independently reviewed the
contract and Go implementation: Humaux `f6b8560d-323b-45c7-a8a2-78cf2ed6ea11`,
`95f1d204-cdf5-4b44-b902-78470fae8d06`. Final SQL/Go/test adjudication
`426fb968-24b8-431e-8d9b-a562931ff74c` read the final commit and log/hash;
no remaining P0/P1 within this contract. Reviewer did not rerun PG; root did.
SQL author's Humaux tools were unavailable; root claimed, locked and stored its
exact delivery rather than falsely reporting an author-side store.

Canonical `live-commerce` code index: 11 source files submitted, 10 Go files /
317 entities indexed, 0 rejected; SQL remains in the versioned migration artifact.
Capture worker, inverse-order test and causal-fault test link to root fix memory
`2a47876f-0679-44e5-bb36-a951e1a90fcf`. Packet structure validation passed;
five changed/referenced Markdown documents have 47 local links, none missing.

## Still not delivered

This is not merchant payout/settlement, complete refund history, notifications,
disputes, partial/multiple captures, manual review resolution, live account
qualification, hosted-form release, other payment methods or production worker
assembly. Merchant switches stay gated; the requested self-service setup UI,
public buyer checkout and real provider sandbox/live/browser acceptance remain
separate deliverables. No UI changed in this increment, so no new browser gate
is claimed. Global G01–G15 and complete SaaS readiness are NOT_PASSED.
