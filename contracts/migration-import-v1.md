# migration-import-v1 — merchant CSV import framework and the customer import (W5-02B)

Status: DRAFT by the unit implementer (Claude Sonnet, 2026-10-06) for integrator freeze. Migration `0152_customer_import.sql`.
Evidence class: REAL_PG + MOCK (synthetic data only). Order-history import (W5-03B) reuses section 2 and is out of scope here.

## 1. Scope and rulings
- One import kind exists: `customers` (SHOPLINE customer CSV). `orders` is reserved in the two shared tables for W5-03B.
- **Consent is always unknown.** The import never writes `customers.consent_events`; a consent column in the file is detected, ignored
  and reported (`consent_ignored`). The SHOPLINE notes column is ignored too (CI-OPEN-2).
- **No identity merge.** An imported customer is its own `buyer.owners` row (no capability session). It is never matched to a buyer who
  later orders, never matched across stores, never across tenants (B24 deferred: needs a verified-contact contract).
- Authority: `customers:privacy` for all three routes (the rows are PII). Tenant and store come from the bearer.
- Fixtures, logs, errors, audit rows and receipts never carry a name, phone, email or any cell of the uploaded file.

## 2. Persistence (schema `migrationimport`, FORCE RLS, no login-role grant; written only by `commerce_privacy_writer` definers)
- `migrationimport.batches(tenant_id, store_id, id, kind CHECK in ('customers','orders'), file_sha256, mapping jsonb <= 2 KiB, rows_total,
  applied, updated, failed, results jsonb, principal_id, created_at, UNIQUE(tenant_id,store_id,kind,file_sha256))`. `applied` = rows that
  created a record; every row is exactly one of created / updated / failed. `results` = `[{row, outcome, code?, external_id?}]`,
  <= 5000 rows. Retention 90 days (lazy prune, <= 50 batches per commit).
- `migrationimport.external_ids(tenant_id, store_id, kind, external_id 1..64, internal_id, PK(tenant_id,store_id,kind,external_id))`.
- `customers.import_profiles(tenant_id, store_id, owner_id, display_name 1..80, phone_e164 NULL, email NULL lower-case <= 254,
  source = 'shopline_csv', imported_at, updated_at)`: personal data, deleted by erasure.
- Definers (SECURITY DEFINER, `search_path=pg_catalog`, EXECUTE `commerce_runtime`): `migrationimport.import_customers(hash, store, rows)`
  (<= 500 rows per call; per-store advisory lock), `migrationimport.record_batch(...)` (batch row, prune, audit `customers.imported`),
  `migrationimport.read_batch_results(hash, store, batch)`. Erasure hook `customers.erase_import_profile` (EXECUTE nobody).
- `commerce_privacy_writer` gains: INSERT(tenant_id, store_id) on `buyer.owners` (policy: GUC scope), table rights on the three new
  tables, and `customers.imported` in the `privacy_audit_insert` policy.

## 3. File rules
- UTF-8 only (BOM allowed), RFC 4180, comma separated, <= 2 MiB, <= 5000 non-blank data rows. Big5 is refused (`encoding_not_utf8`).
  A cell that an earlier export guarded with a leading apostrophe (`'=...`) is unguarded before storing.
- Fields: `external_id` (required, 1..64 chars, no control/format characters), `name` (required, NFC, 1..80 chars), `phone` (optional,
  Taiwan mobile -> `+8869xxxxxxxx`; accepted spellings: `0912-345-678`, `0912345678`, `912345678`, `+886 912 345 678`, `886912345678`,
  `00886912345678`; landlines and anything else are `invalid_phone`), `email` (optional, lower-cased, `invalid_email`).
- Row codes (never a file refusal): `required`, `invalid_external_id`, `name_too_long`, `invalid_name`, `invalid_phone`, `invalid_email`,
  `duplicate_external_id` (every holder of an id that appears twice is refused, none is guessed), `invalid_request` (field-count mismatch).
- File codes (422, nothing written): `encoding_not_utf8`, `too_many_rows`, `required` (empty file or `external_id`/`name` unmapped),
  `invalid_request` (broken CSV, bad mapping). 413 `invalid_request` above 2 MiB; 415 for a non-`text/csv` body.
- **Mapping**: auto-detected from the header (folded: case, full-width, space/hyphen -> underscore; Chinese and English SHOPLINE aliases);
  `?mapping=` is a JSON object `{"external_id","name","phone","email","consent"}` -> header text, <= 2 KiB; a listed field overrides
  auto-detection, `""` unmaps it; an unknown header, a header named twice, or one column for two fields is `invalid_request`.
- A re-import of an `external_id` already imported **updates** the profile and never creates a second owner. A mapped column is
  authoritative (an empty cell clears the optional phone/email); a column the file does not map keeps its stored value.

## 4. HTTP (all `/v1/admin/stores/{store_id}/imports`, private no-store, `customers:privacy`, no `Idempotency-Key` — the file hash is the key)
| Method and path | Body / query | Result |
|---|---|---|
| `POST customers/preview` | `text/csv`; `?mapping=` | 200 `{file_sha256, headers, mapping, rows_total, new_rows, update_rows, apply_rows, failed_rows, consent_ignored_rows, rows:[{row, external_id, outcome created\|updated\|failed, code?, consent_ignored?}]}`. The transaction is always rolled back. |
| `POST customers/commit?expected_apply_rows=N` | `text/csv`; `&mapping=` | 200 `{batch_id, created, updated, failed, replayed}`; 409 `preview_stale` (body = fresh preview, nothing written); 409 `idempotency_conflict` (same bytes with another mapping or N); 422 `nothing_to_apply`. |
| `GET {batch_id}/results.csv[?only=failed]` | none | UTF-8 BOM CSV `row,external_id,outcome,code`, every text cell formula-guarded; 404 for another store's batch. |
- 60 s `WithScopeBudget`; commit is one transaction (`command.Run` operation `migrationimport.customers_commit`, key `cimp-<sha256[:32]>`).
- Audit: `customers.imported` (action only).

## 5. Customer list and detail (amendment to customers-billing-v1)
- `identity.read_merchant_customers` lists customers that have an import profile (no order or bundle needed), orders them by
  `imported_at`, shows the imported `display_name` and phone tail while there is no order (an order's destination wins), and every row
  carries `imported: boolean`. Search (D6) also matches an import profile (name prefix; phone suffix against the local `09...` digits).
- Imported customers can be tagged and noted (`customers.tn_lock_owner` / `tn_owner_visible` accept an import profile).
- Erasure (`apply_erasure`, also on `replay_erasures`) deletes the profile and the customer external id, and scrubs that external id from
  retained `batches.results`. After erasure the same external id in a new file creates a NEW owner (no tombstone is kept; see Risks in DELIVERY).

## 6. Gates
`bash scripts/dev/test-focused.sh '^TestCustomerImport$'` (CI01-CI11), `internal/migrationimport` and `internal/httpapi/imports_test.go`
(DB-free), the schema/comment/upgrade pins (`TestCustomersBillingCB02*`, `TestR2IntegrationUpgradeFromReleaseHead`).
