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

## Fix round (Opus review MERGE-AFTER-FIX; code at ca58517e, red 6086047d, logs red-fix.log / green-fix.log, SHA at the top of each)
- P1-1 city: allowlist of the 22 Taiwan cities / counties (`internal/twcity`, 台 folded to 臺; English and simplified spellings NOT accepted: the SHOPLINE Taiwan export is Traditional). Any other cell is never stored: the order imports with `city=NULL`, counted as warning `city_dropped` (preview `city_dropped_rows` + row `warning`; batch results code column). DB `CHECK (city IN (22 names))` backstop; a Go test compares the SQL list with `twcity.Cities`; the reader / export re-validate with `twcity.Valid`. Contract section 7 corrected.
- P2a: item name and status that look like an email or a TW mobile fail the order (`invalid_item`, `invalid_status`), same W5-02B shape rule as ids.
- P2b: export carries the newest 100 archive rows + `historical_orders_total`; OH11 exports a 2000 x 500-character archive (200, < 1 MiB, 100 kept, total 2000).
- P2c: drift guards are now md5(prosrc) of the three 0152 bodies (record_batch, erase_import_profile, export_import_profile) and an exact action-set check of `privacy_audit_insert` (a parallel unit that adds an action fails the migration, forcing a union). Not a fragment guard any more.
- P2d: logs carry the commit SHA; green ran on the committed tree. Test made independent of TestCustomerImport leftovers (own customer ids OHC-xxxx: that test leaves an erasure tombstone and command receipts for SL-0002/SL-0001 files in the shared store).
- Deferred: order-number tombstone (P2-3, an erased person's order number can be re-created under another live customer id if the file says so); an import-vs-erasure race test (code reasoning in the review holds; no test).
