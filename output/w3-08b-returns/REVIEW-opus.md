# W3-08B returns + merchant cancel — independent money/inventory review (Opus)

- Reviewer: Claude Opus 5.5 (read-only, independent of the author Sonnet 5.5). Date 2026-10-06.
- Target: `unit/w3-08b-returns` HEAD `244a3e93` (author `4ad7073f` + trunk merge), diff `r3/integration...HEAD`.
- Local checks (DB-free only): `go build ./...` ok; `go vet ./internal/returns ./internal/fulfillment ./internal/httpapi` ok;
  `go test ./internal/returns ./internal/fulfillment ./internal/httpapi ./internal/httperror` exit 0. Every PG claim below is
  from reading SQL (DESIGN-level evidence); the REAL_PG gates are CI's (`green.log` is the author's E0/E2 claim, not re-run here).

## Verdict: **MERGE-AFTER-FIX** (2 × P1, no P0)

## Stock-model ruling (question 1): **ACCEPTED** — sellable return = guarded `DEALLOCATE` of `allocated`, `on_hand` unchanged

Evidence that shipping never moves `on_hand` or `allocated`:
- Ledger kinds are only `ADJUST/RESERVE/RELEASE/ALLOCATE` (0018:155) + `DEALLOCATE` (0072:119). No SHIP/CONSUME kind exists.
- Every `INSERT INTO inventory.ledger` in the tree is in 0013, 0018, 0061, 0062, 0073, 0083, 0088, 0099, 0107, 0155; none is in a
  shipping function (0063 / 0107 `record_manual_shipment`, 0073 CVS create/ingest, 0146 group shipment, 0130 pick list). 0107:665
  says so explicitly ("no stock, ledger, reservation or payment write").
- Every paid path writes a per-order `ALLOCATE` with `checkout_id`: card at capture (0061:800 / 0062:857), bank transfer at confirm
  (0088:624 / 0099:127), pay-at-pickup and COD as BUYER ALLOCATE at placement (0107:5, guard 0107:477).

So a shipped unit stays in `on_hand` and in `allocated` forever; `available = on_hand − reserved − allocated − unavailable`
(0086:200) already excludes it. A returned sellable unit is physically back and still counted in `on_hand`, so releasing it from
`allocated` is exactly +1 available; crediting `on_hand` as well would count it twice. A scrapped unit stays in both columns, so it
stays unavailable — also correct. Same model as the §16.8 pay-at-pickup restock (0107:445). Ceiling (pre-existing, not this unit):
`allocated` grows without bound and a merchant who `ADJUST`s `on_hand` down for shipped goods breaks the formula; document it in
returns-v1 §5 as an operator rule.

Per path:

| Path | Register | Close restock | Verdict |
|---|---|---|---|
| Card, manual ship | MERCHANT_SHIPPED, CONFIRMED | per-order alloc from SYSTEM_PAYMENT ALLOCATE | correct |
| Card, CVS (PICKED_UP) | PROVIDER_LABEL_CREATED + PICKED_UP (0155:328) | same | correct, **untested** |
| Pay-at-pickup (collected) | CONFIRMED + COLLECTED | per-order BUYER ALLOCATE, reservation COMMITTED | correct, untested |
| COD (home, collected) | **always `not_returnable`** — see P1-2 | guard also needs CONFIRMED | **broken (fail-closed)** |
| Bank transfer | MERCHANT_SHIPPED, CONFIRMED | per-order MERCHANT ALLOCATE | correct **until a void** — see P1-1 |

Double restock: replay returns the stored body before CAS (0155:540-545); concurrent closes of one RMA serialize on the RMA row lock
and the state machine; two RMAs of one line serialize on the order row lock (0155:548) and the per-order remaining-allocation check
(0155:565-567, guard 0155:169/184); one row per RMA line by the ledger unique key `(operation, command_key, warehouse, sku, kind)`
(0002:147). Cancel vs RMA on one order: cancel needs unshipped and no live RMA (`has_returns`, 0155:707), RMA needs shipped — mutually
exclusive **except through a shipment void** (P1-1).

## P1

### P1-1 Shipment VOID re-opens the order after an RMA restock → double restock (bank transfer) and unledgered re-ship (all modes)
- Evidence: `record_manual_shipment` VOID is allowed for any `MERCHANT_SHIPPED` order with `collection_state` NULL/PENDING and resets
  `fulfillment_state` to `MANUAL_UNASSIGNED` (0107:625-630, 0107:669); it knows nothing about `returns.rmas`. The bank-transfer
  `refund_offline_restock` only requires `MANUAL_UNASSIGNED`, reservation `COMMITTED` (0099:71-84) and its guard checks the *confirm
  ALLOCATE quantity*, not what is still allocated (0107:488-503). The relaxed `ledger_pay_at_pickup_release_once` excludes RMA rows
  (0155:151-152), so the bank DEALLOCATE is the "first" one. The contract amendment even states "a voided shipment returns the order
  to MANUAL_UNASSIGNED and makes it cancellable again" (manual-fulfilment-v1 W3-08B amendment).
- Sequence: bank-transfer order (2 units) shipped → RMA 2 sellable closed (allocated −2) → VOID shipment → offline refund with restock
  (allocated −2 again). Per-order allocation ends at −2; the global balance CHECK `allocated>=0` only fires if no other order holds
  that SKU, so in a live store it passes silently and 2 phantom units become sellable (oversell). DELIVERY risk 5 claims this "fails
  loudly on the balance CHECK"; that is only true for a store with no other allocation of the SKU.
- Same root, any payment mode: RMA restock → VOID → ship again: the order is `manual_shipment_eligible` again and its units leave the
  shelf while the ledger already gave them back.
- Fix (root, two lines of defense, one migration):
  1. In 0155 patch the VOID branch of `fulfillment.record_manual_shipment`: refuse `409 has_returns` when a non-`CANCELLED` RMA exists
     for the order (a return presumes the parcel left; a void says it never did).
  2. Make the per-order bound generic instead of per-operation: in `inventory.guard_returns_ledger` (already a BEFORE INSERT trigger on
     every ledger row) refuse ANY `DEALLOCATE` whose `-delta_allocated` exceeds the order line's remaining `sum(delta_allocated)`
     (ALLOCATE+DEALLOCATE of that checkout/warehouse/sku). One check covers card cancel, bank restock, §16.8 cancel/restock and RMA.
  3. Tests: RMA close → VOID = 409 `has_returns`; and (bypassing 1 with an owner-role forged ordering) bank `refund_offline_restock`
     after an RMA restock = 42501. Correct DELIVERY risk 5.

### P1-2 Returns of home-COD orders are impossible though the contract promises them
- Evidence: a COD order's `commercial_state` stays `AWAITING_COLLECTION` after collection "by design" (0107:774-775; `record_collection`
  only sets `collection_state`, 0107:343-345). `register_rma` requires `commercial_state='CONFIRMED'` (0155:331), `close_rma` and the guard
  too (0155:557, 0155:183). So every COD return answers `409 not_returnable`. returns-v1 §2 says "a pay-on-delivery order must be
  `COLLECTED`", i.e. supported; DELIVERY lists "RMA of a collected COD order" as implemented-but-untested — it does not work.
- Fix: accept `commercial_state IN ('CONFIRMED','AWAITING_COLLECTION')` with `collection_state='COLLECTED'` for `cash_on_delivery` in
  register, close and the guard; add the COD return test (ship → collect → RMA → restock once). Land it after P1-1 step 2 so COD's
  §16.8 restock and an RMA restock share the per-order bound. If the owner prefers to defer COD returns, change the contract and the
  refusal instead — but not silently.

## P2

1. **Anchor patching (question 2).** It is dynamic SQL: `pg_get_functiondef` → `replace` → `EXECUTE` for `guard_checkout_ledger` and
   `guard_pay_at_pickup_ledger` (0155:126-141), plus `pg_get_constraintdef` for `ledger_checkout_actor` (0155:111-124). It is **not** a
   silent no-op: each anchor must occur exactly once or the migration raises (0155:112, 130, 139); precedent 0072/0088/0091/0107/0151.
   Fresh vs upgrade cannot diverge (same checksummed chain) unless someone hot-patched a function out of band. The real cost is that the
   effective bodies exist in no file, and a future migration that statically copies the 0107/0088 body silently drops the 0155 branches —
   that fails closed (RMA restock and cancel RELEASE are then refused, tests go red), so it is fragility, not a money hole.
   Fix: write both functions as plain `CREATE OR REPLACE` with the full latest body in 0155, preceded by an assertion that the live
   `md5(prosrc)` equals the expected post-0107 / post-0088 hash (fails the migration on drift), and add a foundation pin on `prosrc`
   containing both new branches.
2. **In-flight refund later FAILS after cancel (question 4).** Coverage = refunds not FAILED/CANCELED/REJECTED (0155:716-719, guard
   0155:211-213), as the brief rules. If one later fails, the order is CANCELLED, stock released (correct: the goods never left), and the
   buyer's money is held. Not P1: `payments.request_stripe_refund` has no order-state gate (post_river/0016:470-600), so the merchant can
   refund again on the CANCELLED order, and the refund read shows FAILED. Keep in-flight (requiring SUCCEEDED would block cancels for
   the whole pending window); add a reconciliation flag — a merchant/ops list of `CANCELLED` card orders whose non-failed refunds <
   CAPTURED — and one test: cancel with an in-flight refund → refund FAILED → re-refund on the cancelled order succeeds.
3. **Parcel group lock order (question 5).** The reversed case (0155:739-742: order locked, then a group the order joined after the
   unlocked read at 0155:655) can deadlock with `begin_parcel_group_shipment` (group → order) and leans on the deadlock detector → 503.
   Ordering by id does not help (it is a hierarchy inversion, not a sibling order). Fix: if `v_group2 IS DISTINCT FROM v_group`, fail
   `retry_later` immediately without taking the group lock; the retry then locks group → order. Deterministic, no 1 s deadlock wait.
4. **CVS card order returned unclaimed** has no stock path: not returnable (needs `PICKED_UP`, 0155:328-329), not cancellable (live
   parcel → `already_shipped`, 0155:705-706). Record it as a known gap in returns-v1 or allow RMA on a returned-to-sender parcel.
5. **Contract drift.** returns-v1 §3 says `AWAITING_TRANSFER` → `422 not_cancellable`; the definer and Go reject `expected_state`
   outside DRAFT/AWAITING_PAYMENT/CONFIRMED/AWAITING_COLLECTION with 400 (0155:649, cancel.go). Pick one. The DRAFT release duplicates
   the `expire_held` loop (0155:686-700 vs 0088:240-247) although the brief says "不复制逻辑"; promotions/claim uses are derived from
   `commercial_state` (0091:9, 0105:9-11) so nothing leaks — accept and note the deviation.
6. **Helper ACL pins.** `returns.authorize/reauthorize/line_set/rma_json` and `inventory.guard_returns_ledger` have no grant
   (0155:244-299, 217-218) but no test pins "no EXECUTE for commerce_runtime / workers" for them; add to the MF02/WAS02 lists.
7. **Migration locking.** `DROP/ADD CONSTRAINT ledger_checkout_actor` (full validation) and non-concurrent `DROP/CREATE UNIQUE INDEX` on
   `inventory.ledger` (0155:114-115, 150-151) take ACCESS EXCLUSIVE / SHARE locks for the scan; fine now, note it for the release window.

## Question-by-question notes (no finding)

- **Q3 relaxed index.** Bounded: only rows with `operation='returns.rma.restock'` are excluded (0155:152), and the guard admits such a
  row only as a DEALLOCATE of a CLOSED RMA of that order, `closed_by` = writer, exact `qty_restock`, order CONFIRMED + reservation
  COMMITTED, ≤ remaining per-order allocation (0155:171-186); the ledger unique key makes it one row per RMA line. The hole is not the
  index but the non-RMA writers that don't check remaining allocation (P1-1 step 2).
- **Q4 cancel.** Coverage check matches the brief; one attempt per order (`UNIQUE(tenant,store,order_id)`, 0016:42) so the single
  `SELECT INTO` of the captured attempt is sound; any review case except presentment drift blocks (0155:713-714). `expected_state` CAS
  without a version is acceptable: states move forward only and every rule is re-checked under the order lock. AWAITING_PAYMENT → 409
  always (0155:672) and shipped → 409 (0155:673, 703-706) are correct. COD/pay-at-pickup delegate to `release_pay_at_pickup(cancel)`
  (0155:677-682), which re-authorizes `fulfillment:write`, re-locks the same order (re-entrant) and writes its own receipt under
  `inventory.pay_at_pickup.release`; its `collection_state_changed` maps to `state_changed` (returns.go:69). COD/pickup can't be in a
  parcel group, so skipping group removal there is fine.
- **Q6 security.** Tenant/store/principal come from `identity.resolve_access` before any lock and are re-checked after the writes
  (0155:228-263); store id is a path value validated by it. `returns` schema: REVOKE PUBLIC, runtime USAGE only, tables ENABLE+FORCE RLS
  with tenant/store GUC policies for the owner role only, no runtime table grant (0155:32-100). All definers SECURITY DEFINER,
  `search_path=pg_catalog`, every relation schema-qualified, REVOKE PUBLIC, EXECUTE `commerce_runtime` only, owner
  `commerce_checkout_writer`; WAS02 extended so workers get 42501 on all 8. Permissions: register/receive/cancel-RMA/cancel-order
  `fulfillment:write`; inspect/close `fulfillment:write`+`inventory:write` (Go fence `inventory:write` + SQL both); the two reads
  `orders:read`. Request hashes bind order/RMA id, version and lines (returns.go:216-220, cancel.go:56).
- **Q7 tests.** RT01–RT10 map to real subtests; concurrency (double close, partial RMAs, concurrent cancels, cancel vs payment start)
  and forged-ledger (3 shapes → 42501, with a mutation log) are present. Pins only grow (R2 76→77, 8 new ACL rows, WAS02 +8); none
  weakened. Untested: CVS card return, pay-at-pickup return, COD return (which would have exposed P1-2), void-after-RMA (P1-1),
  refund-fails-after-cancel (P2-2).
- **Q8 headers.** 0155 header, `internal/returns`, `internal/fulfillment/cancel.go`, `internal/httpapi/returns.go` carry
  purpose/depends/used-by/invariants; the migration header lists the deviations. DELIVERY risk 5 text is wrong (P1-1).

## Required before merge
P1-1 (void guard + generic per-order DEALLOCATE bound + tests), P1-2 (COD returns or an explicit contract change), then CI re-run of
`^TestReturns$|^TestMerchantCancel$` plus the delivery's regression list on the new SHA. The P2 items can follow in the same fix commit or
a tracked follow-up.
