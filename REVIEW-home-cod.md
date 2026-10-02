# Independent review: unit home-cod (cash on delivery for home delivery)

Reviewed: branch `unit/home-cod` @ `32a51f2` (implementer: DeepSeek V4-Pro). Reviewer/test author: Claude Sonnet 5.5 (cross-family).
Worktree/branch of this review: `.worktrees/home-cod-tests`, `unit/home-cod-tests`. Product code was not changed (the integrator fixes).
Evidence labels (AGENTS.md): REAL_PG = migrations 0001..0107 + post_river applied to a disposable PG 18.6, real merchant/buyer HTTP handlers;
BROWSER = real Next + Playwright; every "green" in the implementer's commit message was treated as unverified and re-run (`output/home-cod-tests/`).

Verdict: **no P0. Four P1 (all with a red test in `tests/foundation/home_cod_defect_test.go`), eleven P2.** The money path itself (server-computed
amounts, eligibility, cap arithmetic, stock hold/release, shared state machine, ACL, tenant isolation) holds under adversarial tests; the P1s are
about *reporting and display of the collect amount* and *post-collection mutation of the order row*. Merge is blocked by P1 until fixed or ruled.

## 1. Findings

### P1-1 The buyer is never told the cash due after placement (collect amount != total on the buyer page)
- Where: `internal/checkout/checkout.go:119-136` (`Order` DTO has no surcharge) and `:411-416` (`Get` does not read `cod_surcharge_minor`);
  `internal/buyerhttp/projections.go:327-350`; `migrations/0107_home_cod.sql:79` (the column is granted to `commerce_auth` only, so no buyer path can read it);
  `apps/storefront/components/OrderFlow.tsx:769-770` (order page shows `snapshot.quote.amount.total_minor`); `apps/storefront/components/CodOrderStatus.tsx:6-10`
  (the comment admits "the surcharge is never in the buyer order DTO ... states the collection fact without an amount").
- Scenario: surcharge NT$50, basket NT$25. Checkout shows "NT$25 + NT$50 fee". After placing, the order page, the order link, the guest lookup and the
  "placed"/"shipped" mails (P2-7) show NT$25 only; the carrier asks for NT$75 at the door. The brief requires total + surcharge to be the collect
  amount "on the order, buyer page, merchant UI, finance CSV"; only the order row (two columns) and finance satisfy it.
- Red test: `TestHomeCodDefectBuyerOrderAmount` (buyer order DTO must expose 5000 or 7500 under a surcharge/collect/due key; currently `found map[]`).
- Fix direction: add a buyer-readable grant/projection of `cod_surcharge_minor` (or a computed `cod_collect_minor`) to `Order`, render it in `CodOrderStatus` and the mails.

### P1-2 The merchant never sees the collect amount per order, and the manual-fulfilment export cannot tell a COD parcel from a prepaid one
- Where: `migrations/0073_taiwan_cvs_functions.sql:1947` (`export_unshipped_orders` emits `total_minor` only, no payment mode) widened to COD by `migrations/0107_home_cod.sql:590-592`;
  `internal/merchantorders/export.go:46-47` (frozen 20-column header); `apps/admin/components/MerchantOrders.tsx:164` (detail "total") and `:969` (list amount) never render
  `cod_surcharge_minor`, which the DTO does carry (`internal/merchantorders/orders.go:77`); `apps/admin/components/OrderCodCollection.tsx:121-135,148` (collect dialog states no amount).
- Scenario: 黑貓/新竹 are manual (no carrier API); the merchant prepares the waybill from the unshipped export and the order detail. The 代收貨款 amount they can see is NT$25 (total) with no
  indication the parcel is COD at all in the CSV; if they key it as prepaid the cash is never collected, if they key NT$25 the carrier collects the wrong amount and the merchant
  later "records collected" for money that does not match finance (which uses total + surcharge).
- Red test: `TestHomeCodDefectMerchantExportAmount` (export has no `payment_mode`/COD column and no cell equal to total+surcharge 7500).
- Fix direction: contract amendment for the export (append `payment_mode` and `collect_minor`, ruling B19 style), render the collect amount in list/detail and in the collect confirmation.

### P1-3a A shipment void/correction after collection defeats the "collected only after shipped" guard
- Where: `migrations/0063_manual_fulfilment.sql:346-352` (void/correct check only the shipment head and `fulfillment_state`, never `collection_state`) and `:385-387`
  (resets `fulfillment_state` to `MANUAL_UNASSIGNED`); the guard lives only at record time in `migrations/0107_home_cod.sql` `record_collection` (not_shipped).
- Scenario: ship, record `collected`, then void the shipment (`wrong_tracking`): HTTP 200, the order is `COLLECTED` + `MANUAL_UNASSIGNED` + `AWAITING_COLLECTION`. `order_money_shippable`
  (collection_state PENDING only) refuses to ship it again, so cash is booked for a parcel that is neither shipped nor shippable. Same hole exists for pay_at_pickup (pre-existing, 0102 only closed the record-time half).
- Red test: `TestHomeCodDefectVoidAfterCollected`.
- Fix direction: refuse VOIDED (and arguably corrections) in `record_manual_shipment` once `collection_state` is not PENDING.

### P1-3b Finance books collected cash on `orders.updated_at`; any later write to the order row moves the money to another day
- Where: `migrations/0107_home_cod.sql:663-671` (cod CTE; comment: "no collected_at column exists, so updated_at"); `migrations/0063_manual_fulfilment.sql:385-387` (every shipment record, including the MD7
  tracking correction that is allowed at any time, rewrites `updated_at`). Same design in the pre-existing pickup CTE (0085/0088).
- Scenario: collected yesterday NT$75 -> finance row for yesterday = 1/7500. Today the merchant fixes a typo in the tracking number -> yesterday = 0/0, today = 1/7500; closed days silently change.
  The CSV export of a closed period is not stable.
- Red test: `TestHomeCodDefectFinanceDayDrift` (yesterday's row must still be 1/7500 after a correction).
- Fix direction: persist `collected_at` (set once in `record_collection`) and group on it for COD (and pickup).

### P2 (11)
1. P2-1 Buyer-seen vs charged surcharge race: options read the surcharge at display time, `begin_hold` snapshots the then-current setting (`migrations/post_river/0020_home_cod_begin_hold.sql:241-246`); there is no expected-surcharge in the request, so a settings change between display and Begin charges a fee the buyer never saw. (`OrderFlow.tsx:440-450`)
2. P2-2 Storefront BFF does not pass the new codes through: `cash_on_delivery_unavailable|amount_exceeds|limit` are not in `apps/storefront/lib/cvs-contract.ts:345-358`, so `buyer-server.ts:147-160` turns them into the generic `unavailable`, and the 429 `cash_on_delivery_limit` is marked retryable (DEFINITE_CVS_CODES lists only `pay_at_pickup_limit`).
3. P2-3 The offer ignores the cap: `internal/checkout/options.go:307-320` lists COD on every home row while enabled even when `total + surcharge` can exceed `max_twd` (the cap is read into `codOffer.maxTWD` and never used), so the buyer finds out at Begin (and then sees P2-2's generic message).
4. P2-4 The carrier label (black_cat/hsinchu) is dead data: not snapshotted on the order, not in the options DTO, shown nowhere, although `COMMENT ON COLUMN ...carrier` (0107) and the settings note "orders already placed keep ... the carrier they were shown" (`apps/admin/lib/cod-copy.ts` setNote) say otherwise; the manual-shipment carrier enum has no 黑貓/新竹 (`migrations/0063_manual_fulfilment.sql:314-315`), the implementer's own tests ship COD with `sf_express`.
5. P2-5 `commercial_state` stays `AWAITING_COLLECTION` after COLLECTED/RETURNED/RESTOCKED/REFUNDED_OFFLINE (design decision, asserted by the browser gate): the buyer headline "Waiting for cash on delivery" (`apps/storefront/lib/order-copy.ts:46`) sits next to "Paid on delivery", the merchant filter `AWAITING_COLLECTION` keeps listing closed orders, the one-active-cart index (`migrations/0107_home_cod.sql:70-72`) keeps the cart version blocked, and promo/live-price consumption (`commercial_state<>'CANCELLED'`, 0091/0105) is never released for a returned parcel.
6. P2-6 COD has no recipient-reachability gate and no `country='TW'` check: pay_at_pickup requires `fulfillment.ecpay_recipient_ok` (`migrations/post_river/0020_home_cod_begin_hold.sql:209`), the COD branch (`:232-253`) only checks `delivery_kind='home'` and TWD, so a TWD market with a non-TW home service offers 黑貓/新竹 COD, and a junk name/phone commits stock until the merchant cancels.
7. P2-7 Buyer mails print "Order total" = `total_minor` only (`migrations/0098_buyer_comms_fixes.sql` claim_batch, `internal/notify/render.go:241`); no COD amount/surcharge in placed/shipped mails.
8. P2-8 The store-wide COD open cap re-uses the CVS knob `pay_at_pickup_max_open` (`migrations/post_river/0020_home_cod_begin_hold.sql:245-252`): a merchant who raises it for pickup (<=500) silently raises it for anonymous COD; stock of open COD orders has no expiry (same as pickup).
9. P2-9 Shipment void/correction is also allowed after RETURNED/RESTOCKED/REFUNDED_OFFLINE (same hole as P1-3a, no cash recorded as collected so lower impact).
10. P2-10 `read_cod_offer` is `STABLE` but calls `buyer.resolve_scope` which sets GUCs (`migrations/0107_home_cod.sql:188-190`); works today (tests green) but differs from "volatile where GUCs are set" siblings; make it VOLATILE.
11. P2-11 Erasure (0078:515) is not blocked by an in-flight shipped-but-uncollected COD order, so the carrier's cash reconciliation can lose the recipient snapshot (same for pickup).

## 2. Money path: what was verified (and how)

| Area | Result | Test |
| --- | --- | --- |
| Client amount/surcharge/cap ignored (server recomputes) | OK: nine extra money keys over the real `POST /v1/buyer/checkout` are refused (4xx, zero rows) or ignored (order carries 2500/5000); seven spellings of the mode refused | `TestHomeCodReviewTamperedAmounts` |
| Whole-TWD, cap and surcharge bounds | OK: total+surcharge == cap accepted, +NT$1 and +NT$0.50 refused; NT$19000+1000 under 20000 ok, NT$19001 refused; settings 20001/1001 refused; table CHECKs hold for the owner role | `TestHomeCodReviewCapBoundary`, `TestHomeCodReviewSchemaInvariants` |
| Setting off / cap lowered between quote and begin | OK: PT422, zero rows, quote not poisoned | `TestHomeCodReviewSettingChangedAfterQuote` |
| COD only on home + setting on; CVS never | OK (service, real buyer HTTP route, options row) | `TestHomeCodEligibility` (implementer), `TestHomeCodReviewTamperedAmounts` |
| Surcharge snapshot vs later settings change; finance = total+surcharge | OK (order, merchant detail JSON, finance, CSV) | `TestHomeCodReviewSurchargeSnapshot`, `TestHomeCodReviewFinanceBoundaries` |
| One shared state machine (not a copy) | OK: exactly one `record_collection`/`release_pay_at_pickup`, both admit both modes, only 3 COD-named functions exist | `TestHomeCodReviewSharedStateMachine` |
| collected only after shipped; returned restocks once | OK at record time; **defeated afterwards by P1-3a**. Concurrent double `returned` and double `restock` (two keys and same key): exactly one winner, one DEALLOCATE, allocation released once (gated on the row lock, mutation M5/M6 prove it can fail) | `TestHomeCodReviewConcurrency`, `TestHomeCodReviewTransitionMatrix` |
| Cancel racing the manual shipment | OK: never CANCELLED and MERCHANT_SHIPPED together | `TestHomeCodReviewConcurrency` |
| Idempotent retries | OK: Begin replay (also after COD was switched off), collection replay, same key other body -> 409 | `TestHomeCodReviewIdempotentRetries` |
| Stock hold/release | OK: committed at placement, expiry job answers STALE and releases nothing, start_payment/payment routes/card refund refused for a COD order | `TestHomeCodReviewStockHoldAndNoPayment` |
| Open-orders limit | OK incl. two concurrent Begins at one free slot -> one order, one 429 | `TestHomeCodReviewConcurrency`, `TestHomeCodEligibility` |
| ACL/definers | OK: exact EXECUTE grantees (no PUBLIC, no commerce_worker), SECURITY DEFINER + `search_path=pg_catalog`, settings table FORCE RLS and writer-only, nobody can UPDATE `cod_surcharge_minor`; `check-gates.sh` forbids grants to the retired role | `TestHomeCodReviewACLInventory` |
| Tenant/store from server auth only | OK: other tenant and same-tenant other-store tokens get 403/404 on settings, collection and release; another tenant's COD switch does not touch this store | `TestHomeCodReviewIsolation` |
| Ledger guard widened for COD must not over-admit | OK: DEALLOCATE refused while PENDING, BUYER-actor DEALLOCATE refused, bank-transfer restock op refused for a COD order, BUYER ALLOCATE refused on COLLECTED and CANCELLED orders, second DEALLOCATE per line refused | `TestHomeCodReviewLedgerGuard` |
| Settings wire strictness + CAS | OK: 14 malformed bodies, missing/short key, query string -> 4xx and nothing written; two writers on one expected version -> one 200, one 409 | `TestHomeCodReviewSettingsWire` |
| Finance columns, header 13, date-range boundary | OK at the instant level (first microsecond of a day, last microsecond of the previous, to-day inclusive, pending/returned/restocked/refunded/cancelled excluded, never in captured/net/pickup/transfer); **not stable over time: P1-3b** | `TestHomeCodReviewFinanceBoundaries` |
| pay_at_pickup regression from the shared refactor | OK: `TestCvs*` and `TestCheckoutOffline*` green on the branch (see section 4) | existing suites |
| UI never implies a carrier API | OK: settings copy, collect dialog and order panel describe manual shipping and "recorded by you"; see P2-4 for the dead carrier label | read of `cod-copy.ts`, `CodSettings.tsx`, `OrderCodCollection.tsx` |

## 3. Tests added (all REAL_PG, `tests/foundation/`)
- `home_cod_review_test.go`: 16 tests (about 30 subtests), 0 failures on `32a51f2` (`TestHomeCodReview*` above).
- `home_cod_defect_test.go`: 4 tests, **red on `32a51f2` by design** (`TestHomeCodDefectBuyerOrderAmount`, `...MerchantExportAmount`, `...VoidAfterCollected`, `...FinanceDayDrift`); they turn green with the P1 fixes and must not be weakened.

## 4. Mutation proofs (assertions can fail; each mutation reverted, `git diff` on `migrations/ internal/ apps/` empty afterwards)
Logs: `output/home-cod-tests/mutation-*-red.log`.
| Id | Mutation | Red tests |
| --- | --- | --- |
| M1 | `record_collection`: drop the collected-after-shipped guard (`IF false AND ...`) | `TestHomeCodLifecycle/collected_only_after_shipping...`, `TestHomeCodReviewTransitionMatrix/PENDING_and_unshipped` |
| M2 | finance cod CTE: `sum(total_minor + cod_surcharge_minor)` -> `sum(total_minor)` | `TestHomeCodLifecycle/collected_only_after_shipping...`, `TestHomeCodReviewSurchargeSnapshot`, `TestHomeCodReviewFinanceBoundaries` |
| M3 | `begin_hold`: admit COD on a non-home destination | `TestHomeCodEligibility/unavailable:_no_settings_row,_then_a_CVS_destination` |
| M4 | `begin_hold`: surcharge snapshot forced to 0 (server ignores its own setting) | 11 tests incl. `TestHomeCodLifecycle/placement...`, `TestHomeCodEligibility/amount...`, `TestHomeCodReviewCapBoundary`, `...SurchargeSnapshot`, `...SettingChangedAfterQuote` |
| M5 | `record_collection`: remove `FOR UPDATE` on the order | `TestHomeCodReviewConcurrency/two_concurrent_returned_records` ("2 winners, 0 losers", 2 audit rows) |
| M6 | `release_pay_at_pickup`: remove `FOR UPDATE` on the order | `TestHomeCodReviewConcurrency/two_concurrent_restocks_with_the_SAME_key`, `.../cancel_racing_the_manual_shipment` |

## 5. Commands run on `32a51f2` + the test files (evidence: `output/home-cod-tests/`, untracked; main-checkout copy not made, worktree is disposable)
| Command | Exit | Counts |
| --- | --- | --- |
| `bash scripts/dev/test-focused.sh '^(TestHomeCod\|TestCvs\|TestCheckoutOffline\|TestOpsPolish\|TestR2IntegrationUpgradeFromReleaseHead\|TestT06WorkerAuthorityAndFunctionACL)'` | 1 | PASS=40 FAIL=4 SKIP=0; the 4 FAIL are exactly `TestHomeCodDefect*` (`focused-full-with-defects.log`) |
| same regex without `TestHomeCodDefect*` | 0 | PASS=40 FAIL=0 SKIP=0 (`focused-full-green.log`) |
| `bash scripts/dev/test-local.sh --browser-home-cod` | 0 | `TestBrowserHomeCod` PASS, 1/1 |
| `bash scripts/dev/test-local.sh --browser-checkout-offline` | 0 | PASS |
| `bash scripts/dev/test-local.sh --browser-cvs` | 0 | TCV08 MOCK PASS (SANDBOX/WebKit variants NOT_RUN by the script) |
| `go build ./...`, `go vet ./...`, `go vet -tags browser ./tests/foundation/`, `gofmt -l .` | 0 / 0 / 0 / empty | |
| `bash scripts/dev/check-gates.sh` | 0 | 56 modes documented, every tracked test file run |
| `bash scripts/dev/test-node.sh` | 0 | 270 tests, 270 pass, 0 fail, 0 skipped |
| `git diff -- migrations internal apps scripts` | | empty (no product change) |

## 6. Not covered / NOT_RUN
- PSP/carrier: none involved (MOCK-free path); no LIVE evidence exists or is claimed.
- `tests/media/r04-input-runner.test.mjs` NOT_RUN by the repo script (needs `COMMERCE_R04_LIVEKIT_BINARY`).
- No test of non-TW home destinations (P2-6) and no UI rendering test for P1-2/P1-1 (the UI will be reworked by another agent; the PG DTO/CSV tests are the contract).
- Real 黑貓/新竹 waybill field mapping is outside this repo (manual fulfilment, no carrier API).
