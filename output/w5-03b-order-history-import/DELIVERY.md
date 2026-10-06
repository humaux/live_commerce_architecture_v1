# w5-03b-order-history-import delivery
- Branch/commit: unit/w5-03b-order-history-import (see final commit)   Base: r3/integration 16e5c91f (unit started on bcbfddf6)   Model: Claude Sonnet 5.5
- Summary: read-only archive of old SHOPLINE orders attached to already-imported customers.
  - `migrations/0156_order_history_import.sql`: table `customers.historical_orders` (FORCE RLS, privacy-writer definers only), `migrationimport.import_orders`,
    `customers.read_historical_orders`; REPLACE of `record_batch` (kind orders, audit `customers.orders_imported`), `erase_import_profile` (+ DELETE of the owner's
    archive), `export_import_profile` (+ `historical_orders`); `privacy_audit_insert` re-created with the new action. Bodies copied from 0152 behind drift guards.
  - `internal/migrationimport/orders_csv.go` (pure parse + aggregation of one-line-per-item files into one unit per order), `orders.go` (preview/commit, same framework as customers:
    file hash idempotency, command.Run, PreviewStaleError now carries `any`), `customers_csv.go` `resolveMapping` made field-generic.
  - `internal/httpapi/imports.go` (+ `POST orders/preview|commit`), `customer_historical.go` (+ `GET customers/{id}/historical-orders`), `internal/customers/historical.go`,
    `privacy.go` / `types.go` (export key), `internal/pagination` (collection `customer-historical-orders`).
  - Contract: `contracts/migration-import-v1.md` section 7, `customers-billing-v1.md` amendment.
- Contract/interface changes: migration-import-v1 section 7; customers-billing-v1 amendment (above). Export document gains `import_profile.historical_orders`.
- Tests: `bash scripts/dev/test-focused.sh '^TestOrderHistoryImport$'` -> exit 0 (OH01-OH12, green.log); DB-free `go test ./internal/migrationimport ./internal/httpapi ./internal/customers ./internal/pagination` -> ok.
  Red evidence (red.log): migration mutated to drop the erasure DELETE -> OH03 (static function grep) and OH06 fail; restored -> green.
- Gates run (all exit 0): `TestOrderHistoryImport`; `TestCustomerImport` (CI01-CI15, regression); `TestCustomersBillingCB02Schema|Comments` (pins updated: new function, table privileges, DELETE allowance, audit action);
  `TestCustomersBillingCB05Erasure|CB04Consent|CB13GrantScript`; `TestWAS02NonPaymentWorkersHoldNoPaymentPrivilege`; `TestR2IntegrationUpgradeFromReleaseHead` (count 78 -> 79);
  `bash scripts/dev/check-gates.sh` (headers ratchet ok); `python3 scripts/check_packet.py`.
- Evidence class: REAL_PG + MOCK (synthetic data only).
- Risks / decisions the integrator should confirm:
  1. **Row cap is 5000 data lines, not 20000.** The 0152 `migrationimport.batches` CHECKs (rows_total/results <= 5000) are shared; raising them means altering that table for both kinds. OH08 tests 5001 lines -> `too_many_rows`.
  2. **`historical_orders_count` on the customer list row is NOT added.** It changes the strict list-row key set (Go `customerKeys`, `apps/admin/lib/customers-model.ts`, browser mocks) and would turn those red; the read route returns `total` for the detail page. Do it with the W6-U1 UI unit.
  3. **Erasure summary does not count the deleted archive rows** (strict four-key summary in Go `decodeErasure`, storefront `privacy-contract.ts`, admin parser). Deletion is proven by OH06 (row counts before/after, replay_erasures). A count needs a coordinated key change.
  4. Cursor query param is `after` (same grammar as the customer list / notes), not `cursor` as the brief says.
  5. `imported_batch_id` omitted (the batch row is written after the apply); batch results of kind orders carry row/outcome/code only, so erasure has nothing to scrub there.
  6. Reader lives in `customers.read_historical_orders` (owner privacy writer), not `identity.*`, so it needs no extra grants. The export of a customer with 2000 archive rows can exceed the 1 MiB export cap (existing `ErrExportTooLarge`).
  7. No `--migration-import` mode exists in `scripts/dev/test-local.sh`; the new test is picked up by the full foundation suite.
- NOT_RUN / BLOCKED: full foundation suite, `--browser-customers-billing`, `release-gate.sh --strict --only G07` (heavy gates; run on GitHub). No UI touched.
- CI gates: `.github/workflows/gates.yml` full foundation suite (new `TestOrderHistoryImport`); `--browser-customers-billing` regression (no UI/API shape change for list/detail, export document grew one key).
- Integrator to-do: parallel units 0150/0154/0155 may also REPLACE `privacy_audit_insert` -> union the action lists (mine adds `customers.orders_imported`) and the `r2_integration_upgrade_test.go` count (+1 here); confirm decisions 1-3.
