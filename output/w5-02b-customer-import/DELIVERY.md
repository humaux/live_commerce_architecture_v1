# w5-02b-customer-import delivery
- Branch/commit: see `git log -1` on `unit/w5-02b-customer-import`   Base: 8c3851e4 (r3/integration)   Model: Claude Sonnet 5.5
- Summary: CSV customer import (SHOPLINE column guess + merchant mapping) with the minimal W5-01B tables folded in.
  `migrations/0152_customer_import.sql` (schema `migrationimport`: batches, external_ids; `customers.import_profiles`; definers
  `import_customers` / `record_batch` / `read_batch_results`, erasure hook `customers.erase_import_profile`; `read_merchant_customers` body = 0139 body +
  W5-02B lines; `tn_lock_owner`/`tn_owner_visible` accept imported customers). `internal/migrationimport/{batch,customers,customers_csv}.go`,
  `internal/httpapi/imports.go` (3 routes under `/v1/admin/stores/{store_id}/imports`, wired in handler.go), `internal/customers` decodes `imported`.
  Contract draft `contracts/migration-import-v1.md` + Amendment in customers-billing-v1.md.
- Contract/interface changes: new contract file; list/detail rows gain `imported` (customers-billing-v1 Amendment W5-02B).
- Tests: `bash scripts/dev/test-focused.sh '^TestCustomerImport$'` -> exit 0 (CI01-CI11, output/w5-02b-customer-import/green.log);
  red: erasure patch removed from 0152 -> CI07 FAIL exit 1 (red.log), restored -> green. Wider focused run
  `^(TestCustomerImport|TestCustomersBilling|TestCustomerTags|TestR2IntegrationUpgradeFromReleaseHead|TestWAS|TestBuyerRegistration|TestBuyerCapability)`
  (run2.log): all PASS except CB09HTTP list/detail key pin (needed `imported`; fixed, green.log PASS). DB-free: `go test ./internal/migrationimport ./internal/customers ./internal/httpapi` ok.
- Gates run: check-gates.sh ok; check-headers.sh ok; gofmt clean. 5000-row preview+commit measured 6.1 s (13 s under machine load) inside the 60 s budget.
- Evidence class: REAL_PG + MOCK (synthetic names/phones `+8869000000xx`, `example.test`). No real customer data.
- Deviations from the brief (judgement calls):
  1. Migration 0152 (not 0140); `apply_erasure` is patched (0113/0139 prosrc pattern), not `erase_owner`, so `replay_erasures` re-deletes too.
  2. Commit does NOT take an `Idempotency-Key` header (tracking-import precedent: the file hash + mapping + expected rows is the key; the header is refused). Brief CI02 semantics hold. Say so if Codex's UI must send one.
  3. Result field names: commit returns `created/updated/failed` (batches.applied = created). Preview adds `headers`, `mapping`, `new_rows/update_rows/apply_rows/failed_rows/consent_ignored_rows`.
  4. Imported customers can be tagged/noted (tn_lock_owner/tn_owner_visible extended by one EXISTS) — otherwise the detail page would 404 on tag writes.
  5. A column the file does not map keeps its stored value on update; a mapped empty cell clears it.
- Risks / follow-ups:
  - After erasure the external id is deleted (brief CI07), so re-importing the same SHOPLINE export recreates the erased customer. A tombstone (hash of external id) would fix it; needs an owner ruling.
  - Merchant privacy export (`customers.Export`) does not yet include the import profile (name/phone/email). Small follow-up: add `import_profile` to the detail extras + exportDoc.
  - Header aliases are best guesses (CI-OPEN-1: owner's de-identified header row decides). Mapping override works meanwhile.
  - Same-store concurrent imports serialize on a transaction advisory lock (second waits up to lock_timeout, then 503 retry_later).
  - `commerce_privacy_writer` gains INSERT(tenant_id,store_id) on buyer.owners (GUC-scoped policy `imp_owner_insert`) — least privilege needed; please review.
- NOT_RUN / BLOCKED: `--browser-*`, release-gate G07, full foundation suite (CI). No LIVE. W5-U1 UI/BFF not touched.
- CI gates: full foundation suite (`.github/workflows/gates.yml`); new mode `bash scripts/dev/test-local.sh --migration-import` is for the integrator to add (suggest regex `^TestCustomerImport$`); regression `--browser-customers-billing`; `release-gate.sh --strict --only G07`.
- Integrator to-do: migration 0152 slot; r2 count pin is 73 -> 74 (union with parallel units); schema pins already updated in customers_billing_schema_test.go (privacy-writer grants, DELETE list, audit action, erase_import_profile) and customers_billing_http_test.go (`imported` key); freeze contracts/migration-import-v1.md; GATES.md row for the new mode.
