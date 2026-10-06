# W3-08B fix round: independent re-verification (Opus)

- Reviewer: Claude Opus 5.5. Read-only; the only file written is this report. Date 2026-10-06.
- Targets: fix code `6ef7d8ee` (diff `244a3e93..6ef7d8ee`), logs `49052850`, branch HEAD `d5f85b2e`. HEAD adds a trunk merge
  that brings in 0152/0153 and the R2 pin. None of these touch 0155, the ledger, `record_manual_shipment` or `checkout.orders` writers.
- Local checks run here (DB-free) at HEAD `d5f85b2e`:
  - `go build ./...` ok.
  - `go vet` on returns, fulfillment, httpapi, merchantorders and tests/foundation: ok.
  - `go test` on returns, fulfillment, httpapi, merchantorders and httperror: exit 0.
  - `python3 scripts/check_packet.py`: exit 0.
- Every PG claim below comes from reading the SQL (DESIGN level). `green-fix.log` (53 PASS on `6ef7d8ee`) is the author's evidence. I did not re-run it.

## Verdict: **MERGE** (P1-1 FIXED, P1-2 FIXED, no new P0/P1)
Condition: CI must pass the PG suite at HEAD `d5f85b2e` (see "Still NOT_RUN" below). Four P2s below can follow in a later round.

## 1. P1-1: FIXED

### Void guard
- 0155:804-817 patches the VOID branch of `fulfillment.record_manual_shipment` in place. The anchor must match exactly once or the migration raises (0155:812).
- The new `ELSIF` returns `409 has_returns` when any RMA of the order has `state<>'CANCELLED'`.
  - The RMA states are REGISTERED, RECEIVED, INSPECTED, CLOSED and CANCELLED (0155:45).
  - So the live states are REGISTERED, RECEIVED, INSPECTED and **CLOSED**. CLOSED is the case that matters.
  - A CANCELLED (withdrawn) RMA is excluded.
- Ordering is safe:
  - The check runs after the order `FOR UPDATE` (0107:605-606). `register_rma` takes the same lock (0155:342), so a void and a registration serialize.
  - The function runs as `commerce_checkout_writer` with app.tenant_id/store_id set (0107:584).
  - The `rmas_writer` policy (0155:99-101) therefore shows the store's RMAs. Visibility cannot fail open.
- Go maps the code: `merchantorders/shipments.go:296` returns a ParcelError, which becomes 409 + code (`httpapi/shipments.go:187-188`).
- Other paths that reset `fulfillment_state` to `MANUAL_UNASSIGNED`:
  - The only other one is the CVS abandon at 0073:1546-1548, and it runs only from CREATED. An RMA needs PICKED_UP, so the two cannot meet.
  - Group shipments go through `record_manual_shipment` (0146 header), so the patch covers them.

### Single DEALLOCATE bound (`guard_returns_ledger`, 0155:169-174)
- Every `kind='DEALLOCATE'` row is refused with 42501 in three cases:
  - `checkout_id` is NULL.
  - `delta_allocated >= 0`.
  - `-delta` exceeds `sum(delta_allocated)` of ALLOCATE plus DEALLOCATE rows for `(tenant, store, checkout_id, warehouse_id, sku_id)`. So the bound is **per order line, per (order, warehouse, sku)**.
- It runs before the operation filter, so it covers every writer. A grep over `migrations/` finds four DEALLOCATE writers, all with `checkout_id = p_order`:

  | Writer | Location |
  |---|---|
  | `release_pay_at_pickup` (pay-at-pickup / COD cancel and restock) | 0107:445-448 |
  | Bank-transfer `refund_offline_restock` | 0099:89-92 |
  | RMA restock | 0155:592 |
  | Merchant card cancel | 0155:760 |

- Each matching ALLOCATE carries the same `checkout_id`:
  - Bank confirm: 0099:127-130 and 0088:624-627.
  - BUYER commit: the guard requires `o.id=NEW.checkout_id` (0107:478).
  - Card capture (0061:800, 0062:857): already relied on by the pre-fix RMA and cancel tests.
- **Locking.** The guard takes no lock of its own. It is sound because every writer holds two locks before its INSERT:
  - The order row `FOR UPDATE`: 0099:51, 0107:404, 0155:572 and 0155:684.
  - `inventory.lock_balance(warehouse, sku)`: 0107:444, 0155:587, 0155:721 and 0155:755.
  - The guard is a VOLATILE plpgsql function under READ COMMITTED, so its SELECT takes a fresh snapshot after those locks. It sees any competing DEALLOCATE that has committed. Two concurrent DEALLOCATEs cannot both pass.
  - Ceiling: a future DEALLOCATE writer that skips the order lock would rely on `lock_balance` alone. The guard comment should state this rule for future writers (P2-d).
- **Visibility.** The guard is SECURITY DEFINER, owned by `commerce_checkout_writer`.
  - The ledger has FORCE RLS. The `checkout_writer_read` SELECT policy (0013:196-199) is scoped by the app.tenant_id/store_id GUCs, and all four writers set them.
  - If the GUCs were missing, the sum would be 0 and the row refused. The guard fails closed.
- **Legitimate paths that could now be refused wrongly: none found.**
  - The expiry worker and the payment worker write RELEASE or ALLOCATE, never DEALLOCATE. The grep shows no DEALLOCATE in 0018, 0061 or 0062. The `checkout.merchant_cancel` RELEASE of unpaid holds is `kind='RELEASE'`, so the bound skips it.
  - Normal bank restock, pay-at-pickup cancel/restock and COD cancel each deallocate exactly the line's full allocation (sum = q), so they pass.
  - A collected COD order can only move COLLECTED to REFUNDED_OFFLINE (0107:338-339). A COD RMA can therefore never be followed by a §16.8 restock.

### Bank-transfer test bypass (returns_test.go:904-938)
- It is a legitimate defense-in-depth test, in the same disclosed owner-fixture style as the forged-ledger subtest.
  - It first asserts the void guard itself: `409 has_returns` at :910.
  - It then rewrites only `fulfillment_state` with the constraint trigger disabled.
  - The refund path does not update `checkout.orders`, so the re-enabled trigger cannot refuse for an unrelated reason.
  - After the rewrite, every other precondition of `refund_offline_restock` holds: transfer CONFIRMED, order CONFIRMED, reservation COMMITTED, no CVS row. The bank branch of `guard_pay_at_pickup_ledger` (0107:488-503) also passes.
  - So the only thing that can refuse the DEALLOCATE is the new bound.
- The void guard is also tested on its own for a card order (:480-497): refused while live, allowed after the RMA is withdrawn.

## 2. P1-2: FIXED
- 0107 facts:
  - `payment_mode='cash_on_delivery'`.
  - `collection_state` takes PENDING, COLLECTED, RETURNED, REFUNDED_OFFLINE, CANCELLED or RESTOCKED (0072:97).
  - `record_collection` sets only `collection_state` and `collected_at` (0107:343-345). The order stays `AWAITING_COLLECTION` (0107:774-775).
  - Pay-at-pickup orders are CONFIRMED.
- `returns.order_returnable` (0155:301-305) is true in two cases:
  - CONFIRMED with collection NULL or COLLECTED.
  - AWAITING_COLLECTION with `cash_on_delivery` and COLLECTED.
- That matches the real columns and values. It is IMMUTABLE, owner-only and has no grant.
- All three call sites use it: register (0155:354), close (0155:580) and the ledger guard (0155:195).
- Test (returns_test.go:500-527):
  - An uncollected COD order answers `409 not_returnable`.
  - Collected, the order is asserted to be still AWAITING_COLLECTION.
  - Close restocks: `allocated` drops by exactly 2, and a replay leaves exactly one restock ledger row.

## 3. P2s from the prior review
| Item | Status | Evidence |
|---|---|---|
| P2-2 `GET /orders/cancel-refund-gaps` | done | Tenant comes from `returns.authorize` (0155:828). Store is the path value validated by `identity.resolve_access` (house pattern). Requires `orders:read` (Go fence `returns.go` plus SQL), then reauthorize. The query is filtered by tenant and store, CANCELLED card orders whose non-failed refunds are below CAPTURED and that have a `checkout.merchant_cancel` DEALLOCATE, capped at 100. Output is only order_id, amounts, `cancelled_at` and reason: no buyer PII. ACL pinned in MF02, V2 and WAS02. Test covers healthy=0, failed=1, re-refund=0 and no-orders:read=403. |
| P2-3 group race | done | Membership is re-read under the order lock, and `retry_later` is returned before the replay lookup and before any write (0155:690-691). Go maps it to 503 (`returns.go` codes, `returns_test.go`, `httpapi/returns_test.go`). TestMerchantCancelGroupRace asserts 503, the order still CONFIRMED/COMMITTED, and the retry dissolving the group. |
| P2-5 AWAITING_TRANSFER | done | Accepted as expected_state (0155:672, `cancel.go`), falls through to `422 not_cancellable` (0155:784). Tested. |
| P2-6 helper ACL pins | done | `rtHelperSigs` pins six helpers as owner-only, with no PUBLIC and no other `commerce_*` EXECUTE, each commented. The patched-body pins cover `guard_checkout_ledger`, `guard_pay_at_pickup_ledger`, `record_manual_shipment` and `guard_returns_ledger`. |
| P2-4 CVS limit | documented | returns-v1 §7a:116. |
| P2-1 / P2-7 | unchanged (accepted as fragility / release note) | n/a |

## 4. No regression
- The R2 pin is 79 at HEAD (`r2_integration_upgrade_test.go:65-67`) after the merge of 0152 and 0153. There are 121 migration files.
- The trunk merge diff touches no file of the fix.
- No assertions were removed. Diffing `244a3e93..6ef7d8ee` over all test files shows only additions, plus one comment change in `merchant_orders_v2_acl_test.go`.
- Pins only grew: MF02 +1, V2 +1, WAS02 +1, plus the new helper and body pins.

## 5. Evidence honesty (would each test fail if its fix were reverted?)
- **Void guard:** yes. `red-fix.log` shows 200 VOIDED at returns_test.go:487 and :910.
- **COD returnable:** yes. Red shows `409 not_returnable` at :514.
- **AWAITING_TRANSFER:** yes. Red shows `422 invalid_request`.
- **Gaps route:** yes. Red shows 422 (no route).
- **Group race:** yes. Red shows 200 DISSOLVED.
- **Helper pins:** only `order_returnable` was red. The other five helpers already existed, which is expected for a regression pin.
- **DEALLOCATE bound: never observed red.**
  - The red run stopped at the void assertion (:910) before it reached the bypass.
  - By reasoning, reverting only the bound makes the bank refund succeed (200), so :930 would fail. I trace this from the guard bodies above, but it was never executed.
  - Two weaknesses remain:
    - :930 asserts only `st != 200`, not the expected code.
    - The body pin's needle `"v_alloc"` (:971) predates the fix, so it would not catch removal of the bound.

## P2 follow-ups (not blocking)
- a. In `TestReturnsBankTransfer`, assert the exact status and code of the refused refund. Change the `guard_returns_ledger` body-pin needle to `deallocate exceeds the remaining allocation`. Record one mutation run (bound removed → refund 200, balance moved) in a log.
- b. `TestBankTransferK302RefundRestock` (`checkout_offline_fix_test.go:115`) covers the normal bank restock, which now passes through the bound. It is not in the focused green run, so CI's full foundation suite must show it PASS.
- c. `cancelled_at` is `orders.updated_at`, which any later update to the order moves. It is acceptable for a worklist; `audit_events` would give a stable time.
- d. Add a line to the `guard_returns_ledger` COMMENT: "every DEALLOCATE writer must hold the order row lock before inserting".

## Still NOT_RUN
These need CI: the PG suites at HEAD `d5f85b2e` (green-fix ran on `6ef7d8ee`, before the merge), K302, `--expiry-worker`, `--payment-worker` and R2 upgrade 79.
