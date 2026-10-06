# migration-import-v1 — merchant CSV import framework and the customer import (W5-02B)

Status: DRAFT by the unit implementer (Claude Sonnet, 2026-10-06) for integrator freeze. Migration `0152_customer_import.sql`.
Evidence class: REAL_PG + MOCK (synthetic data only). The order-history import (W5-03B, migration `0156`) is section 7.

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
  created a record; every row is exactly one of created / updated / failed. `results` = `[{row, outcome, code?, external_id?}]` (external_id only on created/updated rows; a failed row keeps its row number and code only),
  <= 5000 rows. Retention 90 days (lazy prune, <= 50 batches per commit).
- `migrationimport.external_ids(tenant_id, store_id, kind, external_id 1..64, internal_id, PK(tenant_id,store_id,kind,external_id))`.
- Erasure tombstone: `migrationimport.store_salts(tenant_id, store_id, salt bytea 32)` (two v4 uuids, pgcrypto is not installed) and
  `migrationimport.erased_external_ids(tenant_id, store_id, kind, id_digest bytea 32, erased_at, PK(tenant_id,store_id,kind,id_digest))`
  with `id_digest = sha256(salt || external_id as stored)`. Insert-only (privacy writer SELECT+INSERT, FORCE RLS, store-scoped policies).
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
- `external_id` format rule: an id containing `@` or accepted as a Taiwan mobile number is refused (`invalid_external_id`), so a mis-mapped
  phone/email column cannot copy PII into results. A numeric source id that is also a valid mobile number must be prefixed.
- Row codes (never a file refusal): `erased` (the source id carries an erasure tombstone), `required`, `invalid_external_id`, `name_too_long`, `invalid_name`, `invalid_phone`, `invalid_email`,
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
| `POST customers/preview` | `text/csv`; `?mapping=` | 200 `{file_sha256, headers, mapping, rows_total, new_rows, update_rows, apply_rows, failed_rows, erased_rows, consent_ignored_rows, rows:[{row, external_id, outcome created\|updated\|failed, code?, consent_ignored?}]}`. The transaction is always rolled back. |
| `POST customers/commit?expected_apply_rows=N` | `text/csv`; `&mapping=` | 200 `{batch_id, created, updated, failed, replayed}`; 409 `preview_stale` (body = fresh preview, nothing written); 409 `idempotency_conflict` (same bytes with another mapping or N); 422 `nothing_to_apply`. |
| `GET {batch_id}/results.csv[?only=failed]` | none | UTF-8 BOM CSV `row,external_id,outcome,code`, every text cell formula-guarded; 404 for another store's batch. |
- A committed file is keyed by its bytes forever: the same bytes with another mapping or count is `idempotency_conflict`, so a merchant who
  committed a wrong mapping must change the file (W5-U1 copy must say so). The brief's `Idempotency-Key` header is deliberately NOT used.
- 60 s `WithScopeBudget`; commit is one transaction (`command.Run` operation `migrationimport.customers_commit`, key `cimp-<sha256[:32]>`).
- Audit: `customers.imported` (action only).

## 5. Customer list and detail (amendment to customers-billing-v1)
- `identity.read_merchant_customers` lists customers that have an import profile (no order or bundle needed), orders them by
  `imported_at`, shows the imported `display_name` and phone tail while there is no order (an order's destination wins), and every row
  carries `imported: boolean`. Search (D6) also matches an import profile (name prefix; phone suffix against the local `09...` digits).
- Imported customers can be tagged and noted (`customers.tn_lock_owner` / `tn_owner_visible` accept an import profile).
- Erasure (`apply_erasure`, also on `replay_erasures`; the hook lives in `apply_erasure`, not `erase_owner`) writes the salted tombstone
  digest of each customer external id FIRST, then deletes the profile and the external id and scrubs that id from retained
  `batches.results`. A later import of the same source id in the same store fails that row as `erased` (never creates an owner); the preview
  counts `erased_rows` and the preview row, the stored result and results.csv carry the row number and code only, never the id. A commit
  racing an erasure is safe: the create branch re-checks the tombstone in a new READ COMMITTED statement after the owner lock released.
  The digest is per store (salt), so other stores and tenants are unaffected; deleting the store's salt crypto-shreds every tombstone.
  **Known limits (not solved here):** the same person under a NEW source id, and a buyer who erased themselves (no external id exists),
  are not caught; that needs the verified-contact contract (B24). There is no merchant override in v1.
- Privacy export: `customers.export_import_profile(hash, store, customer)` (customers:privacy, EXECUTE `commerce_runtime`) returns
  `{display_name, phone, email, source, imported_at, updated_at, external_ids[]}`; the merchant export (`POST customers/{id}/exports`) adds it as
  `import_profile` (absent for non-imported customers). The EXPORT receipt and audit stay counts-only.
- `commerce_auth` reads `customers.import_profiles` through a policy scoped to `app.tenant_id` and `app.store_id`.
- Retention gap (known): batches are pruned lazily on the next commit of the same store (90 days); a store that never imports again keeps
  `batches.results` (row numbers and external ids) until then. A sweeper hook is a follow-up.

## 6. Gates
`bash scripts/dev/test-focused.sh '^TestCustomerImport$'` (CI01-CI15), `internal/migrationimport` and `internal/httpapi/imports_test.go`
(DB-free), the schema/comment/upgrade pins (`TestCustomersBillingCB02*`, `TestR2IntegrationUpgradeFromReleaseHead`).

## 7. Historical order import (W5-03B, migration `0156_order_history_import.sql`) — a READ-ONLY ARCHIVE
Status: DRAFT by the unit implementer (Claude Sonnet, 2026-10-07) for integrator freeze. Evidence class REAL_PG + MOCK (synthetic data).
- **Rulings.** A historical order is display data of an old SHOPLINE order. It is never a `checkout.orders` row and is never read by
  payments, inventory, finance, reports, CAPI or attribution (I05; pinned by a static grep over every function body, view and foreign key).
  It attaches only to a customer imported earlier (`external_ids` kind `customers`); a missing customer fails the row
  `customer_not_imported` (no customer is created); an erased customer's source id fails the row `erased` (section 5 tombstone).
  Amount = display `total_minor` (whole NT$ x 100) + `currency='TWD'`. No payment method / card / bank account; no address beyond the
  city (OH-OPEN-1: city only). The ONLY stored city is one of Taiwan's 22 cities / counties (`internal/twcity`; 台 is folded to the canonical 臺;
  English and simplified spellings are not accepted: the SHOPLINE Taiwan export is Traditional Chinese). Any other cell (a street, a house
  number, a name, an email) is NEVER stored: the order still imports with `city = NULL` and the unit carries the counted warning `city_dropped`
  (preview `city_dropped_rows`, row `warning`; batch results put it in the code column of a created / updated row). The table backs this with a
  `CHECK (city IN (the 22 names))`, so no writer can store anything else. Item name, status, order id and customer id that look like an email or a
  Taiwan mobile number refuse the order (`invalid_item`, `invalid_status`, `invalid_order_id`, `invalid_external_id`). not archived). Unmapped columns (address, phone, email, notes) are never read.
- **Table** `customers.historical_orders(tenant_id, store_id, id, owner_id, external_order_id 1..64, ordered_at, status 1..40, total_minor
  0..10^12, currency='TWD', items_summary <= 500, city NULL or one of the 22 cities, imported_at, updated_at, UNIQUE(tenant,store,external_order_id))`,
  FORCE RLS, no login-role grant (privacy-writer definers only). <= 2000 rows per customer (`order_limit`). No `imported_batch_id`: the
  batch row is written after the apply, and batch results carry no order id.
- **Definers** (owner `commerce_privacy_writer`, SECURITY DEFINER, `search_path=pg_catalog`, EXECUTE `commerce_runtime`):
  `migrationimport.import_orders(hash, store, rows)` (<= 500 per call, per-store advisory lock, owner row locked FOR UPDATE with its
  active test, tombstone re-check in a new statement), `customers.read_historical_orders(hash, store, customer, limit, after_ts, after_id)`
  (`customers:read`). `migrationimport.record_batch` now accepts kind `orders` and audits `customers.orders_imported` (action only; the
  `privacy_audit_insert` policy lists it). `customers.erase_import_profile` also deletes the owner's historical orders (same transaction,
  also on `replay_erasures`); `customers.export_import_profile` also returns `historical_orders`.
- **File rules** as section 3 (UTF-8, <= 2 MiB, RFC 4180, <= 5000 data LINES, the 0152 batch cap; a larger file is `too_many_rows`).
  Fields: `order_id`, `customer_id`, `ordered_at`, `status`, `total` (required); `item_name`, `item_qty`, `city` (optional); Chinese and
  English aliases auto-detected, `?mapping=` as in section 3. One order = one unit: lines with the same `order_id` merge (SHOPLINE writes one
  line per item); `items_summary` = `name x qty` joined with a Chinese enumeration comma (truncated at 500 characters). Order-level cells
  may be empty on later lines; non-empty cells that disagree fail the whole order `inconsistent_order`. Amounts accept `1280`, `1,280`,
  `NT$1,280`, `1280.00`; a non-zero fraction is `invalid_amount`. Dates accept `2026-03-05[ 14:30[:00]]`, `/` separators and RFC 3339; no zone
  = Asia/Taipei. Unit codes: `required`, `invalid_order_id`, `invalid_external_id` (customer id shaped like a phone or email),
  `invalid_date`, `invalid_amount`, `invalid_status`, `invalid_item`, `invalid_quantity`, `inconsistent_order`, `invalid_request`
  (field-count mismatch), plus the database codes `customer_not_imported`, `erased`, `order_owner_conflict` (the order number belongs to
  another customer), `order_limit`. `rows_total / applied / updated / failed` of the batch count units; the row number of a unit is its first line.
- **HTTP** (`/v1/admin/stores/{store_id}/imports`, `customers:privacy`, no `Idempotency-Key`): `POST orders/preview` and
  `POST orders/commit?expected_apply_rows=N` with the same bodies, answers, 409 `preview_stale` / `idempotency_conflict`, 422
  `nothing_to_apply` and 60 s budget as section 4 (preview: `{file_sha256, headers, mapping, rows_total, new_rows, update_rows, apply_rows,
  failed_rows, erased_rows, rows:[{row, external_id, outcome, code?}]}`; a failed row never echoes an id). `GET {batch_id}/results.csv`
  serves orders batches too (external_id column empty). Read: `GET /v1/admin/stores/{store_id}/customers/{customer_id}/historical-orders[?limit&after]`
  (`customers:read`, default 50, newest first, same limit/after grammar as the customer notes) answers
  `{items:[{order_id, ordered_at, status, total_minor, currency, items_summary, city}], next_cursor, total}`; another store's, an erased or
  an unknown customer is 404. The merchant privacy export (`POST customers/{id}/exports`) adds `import_profile.historical_orders` (the NEWEST 100 rows, so a 2000-row archive never exceeds the 1 MiB export cap) and `import_profile.historical_orders_total` (the full count); the merchant sees every row through the read route.
- **Not done here (integrator decisions):** `historical_orders_count` on the customer list row (it changes the strict list-row key set of
  Go `customerKeys`, `apps/admin` `customerKeys` and every browser mock; `total` of the read route serves the detail page meanwhile) and a
  count of erased archive rows in the erasure summary (the strict four-key summary of `decodeErasure` and the storefront / admin parsers).
