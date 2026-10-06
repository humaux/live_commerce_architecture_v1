# perf-dashboard-todos delivery
- Branch/commit: unit/perf-dashboard-todos @ (see `git log -1`)   Base: a8d029ea   Model: Claude Sonnet 5.5
- Summary: `TestMerchantToolsDashboardScale` (MTO03, 300 ms budget kept, fixture 10,002 orders kept) failed locally too (worst-of-5 725 ms,
  `dashboard_todos` 517 ms). Root cause: stale planner statistics after the bulk load. In `fulfillment.order_money_shippable` and
  `fulfillment.manual_shipment_eligible` (0107 bodies, called once per unshipped candidate by `identity.dashboard_todos`) the order is read with
  `tenant_id=.. AND store_id=.. AND id=..`. The planner believes the (new/grown) store holds ~1 order, so the unique 3-column lookup and the
  (tenant_id, store_id)-prefix scan of the partial index `checkout_one_active_cart_version` tie on cost and it picks the prefix scan: it reads ALL
  8,002 matching orders of the store and filters on id (2,596 buffers) in each of the pay_at_pickup / bank_transfer / cash_on_delivery branches
  and again in `manual_shipment_eligible`, for each of the 100 candidates: 779,514 buffers per todos call. After `ANALYZE checkout.orders` the same
  call takes 6-9 ms (554 ms -> 9 ms, `explain-analyze.txt`). Production hits the same window after a bulk import or fast store growth.
- Fix: `migrations/0157_dashboard_todos_perf.sql` (CREATE OR REPLACE of those two functions, 0107 bodies verbatim, only the five order-row predicates
  become `o.id=p_order AND o.tenant_id IS NOT DISTINCT FROM p_tenant AND o.store_id IS NOT DISTINCT FROM p_store`). `id` is the only sargable key, so
  the lookup is always the unique `orders_id_key` probe (3 buffers), whatever the statistics say. `dashboard_todos` itself is unchanged. All callers
  of the two functions (order list `ready_to_ship`, unshipped export, pick list, parcel merge, record_manual_shipment) get the same stability.
- No index added: the needed unique indexes already exist (`orders_id_key`, `checkout_orders_attribution_scope`, `orders_unshipped`); the defect
  is plan choice, not a missing index, and a new index cannot break a cost tie. So no CREATE INDEX lock on the pilot tables: CREATE OR REPLACE
  FUNCTION takes no table lock (catalog row only), runs inside the migrator's single transaction.
- Contract/interface changes: none (signatures, owner commerce_checkout_writer, SECURITY INVOKER, STABLE, grants unchanged; COMMENTs extended).
- Measurement (local PG 18.6 container, 1 CPU, -race; `red.log`/`green.log`, `before-run*.log`, `after-*.log`):
  | | before | after |
  |---|---|---|
  | `dashboard_todos` block | 517 ms (581 ms/502 ms in diag runs) | 5-7 ms |
  | MTO03 worst of 5 (budget 300) | 725 ms (FAIL) | 45.9 / 46.1 / 48.1 ms (3 clean runs, PASS), 56-70 ms with diagnostics |
  | todos select, EXPLAIN (ANALYZE, BUFFERS) | 431 ms, 779,514 buffers | 16 ms, 1,914 buffers (`explain-before.txt` / `explain-after.txt`) |
  CI measured todos 575 ms / worst 645 ms; the same fix removes the 100 x 3 full-store scans, so CI should land near 10-20 ms / ~100 ms worst.
- Result equivalence: fixture `dashboard_todos` JSON identical before/after (`todos-before.json` = `todos-after.json`, `cmp` equal; all counters 0
  on that fixture, so it is weak by itself). Strong check (`equiv.txt`, diag `zz_diag_dashboard_todos_test.go.txt` + `diag-hooks.patch`): the 0107
  bodies re-created as perfdiag.old_* compared with the new functions over every order of four real scenarios (card with refund + review
  exclusion, bank transfer, home COD, pay_at_pickup, DRAFT/AWAITING_TRANSFER), plus probes with a foreign tenant, NULL store/tenant and unknown
  order: 22 orders, 0 mismatches (eligible true 10 of them). The MTO02 counters (to_ship 1/2, cvs_awaiting_label 1) pass on the new functions.
- Tests: pins: `tests/foundation/r2_integration_upgrade_test.go` migration set 80 -> 81 (comment extended). No ACL/index pin changes (no new
  function or index; owners/grants unchanged, TestTaiwanCvsSchema "same grants", TestManualFulfilmentMF02Schema, TestWAS06PrivilegeInventory,
  TestMerchantOrdersV2PickListReadAuthority green).
- Gates run (focused, local): `test-focused.sh '^TestMerchantToolsDashboardScale$'` x3 -> exit 0 (before: exit 1, `red.log`);
  `'^TestR2IntegrationUpgradeFromReleaseHead$'` -> 0; batch of 10 (MTO02, TestTaiwanCvsSchema, MF02Schema, V2PickListReadAuthority,
  WAS06, V2TenThousandScopedSearch, HomeCodEligibility, BankTransferLifecycle, PickListRefundAndReviewExclusion, CarrierExportCOD) -> 0 (`after-gates.log`);
  `check-headers.sh` OK, `gofmt -l` clean. `check-gates.sh` stops at the Node suite (`Cannot find module 'typescript-api'`: this worktree has no
  node_modules; environmental, unrelated), so its header/gofmt tail was run directly.
- Evidence class: REAL_PG (disposable PG container), E3 for the equivalence and timing on the final tree; independent review still to do (E4).
- Risks: (1) The `IS NOT DISTINCT FROM` guards are deliberate; "simplifying" them back to `=` re-opens the cost tie. MTO03 is the guard but only
  fails on stale statistics, which the bulk-loaded fixture reproduces. (2) The same stale-statistics tie exists in principle for the other
  (tenant_id, store_id) prefix lookups of the card branch (`payment_attempts`); not measurable on this fixture (the table is empty) and not changed
  here. (3) Residual fixed cost is ~19 buffers / 0.06 ms per candidate, so a 2,000-order unshipped backlog is ~100-130 ms.
- NOT_RUN / BLOCKED: full foundation shards, browser modes (CI only); `check-gates.sh` end to end (node_modules missing locally).
- CI gates needed: `.github/workflows/gates.yml` modes `shard:^TestM` (MTO02/MTO03, MerchantOrdersV2, MF02) and the full set
  `shard:^Test(A|[N-R])` (PickList*, R2Integration upgrade), `shard:^Test[B-K]` (BankTransfer*, CarrierExport*, HomeCod*, Cvs*), `shard:^TestL`,
  `shard:^Test[S-Z]` (TestTaiwanCvsSchema, WAS*), `shard:unit` (no Go change; unchanged).
- Integrator to-do: migration number 0157 as assigned (0154 and 0156 are not in this tree); nothing else.
