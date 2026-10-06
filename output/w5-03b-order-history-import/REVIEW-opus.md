# W5-03B historical order import: independent privacy review (Opus)

- Reviewer: Claude Opus 5.5, read-only (I did not edit or commit any source). Target: `unit/w5-03b-order-history-import` @ `3f29fe2c` (merged with trunk `16e5c91f`). Diff `r3/integration...HEAD`, 30 files.
- What I ran here (DB-free): `go test ./internal/migrationimport ./internal/httpapi ./internal/customers ./internal/pagination` exit 0; `bash scripts/dev/check-gates.sh` ok (check-headers OK, base 16e5c91f); a scratch probe of the city predicate (orders_csv.go:257, copied verbatim, see P1-1).
- NOT_RUN: everything that needs PostgreSQL (OH01-OH12, CB02/CB05, upgrade). I read `green.log` and `red.log` but did not re-run them.

## Verdict: **MERGE-AFTER-FIX** (no P0; one P1)

## P0
None.

## P1

### P1-1: the city check is not strict. A full street address, a name or an email can be stored as the "city".
- **Evidence.**
  - The city check at `internal/migrationimport/orders_csv.go:257` refuses a cell only when it is longer than 20 runes or contains a `unicode.IsDigit` character. I ran the predicate on sample cells. These were accepted:
    - `臺北市中正區重慶南路一段一二二號` (a complete address, house number in Chinese numerals, 16 runes)
    - `台北市大安區忠孝東路四段`
    - `王小明`
    - `amy@mail.tw`
    - `Zhongxiao East Road`
  - Only addresses written with Arabic or full-width digits are refused (`中山路99號3樓`, `１２號`).
  - The database adds no extra protection. Line 61 of migration 0156 only checks the length (1..20).
  - The read path has the same gap. `internal/customers/historical.go:53` only rejects ASCII digits.
  - The contract overclaims. `contracts/migration-import-v1.md` section 7 says "so a mis-mapped full address is not archived".
- **Why it matters.** The integrator ruling (brief lines 10 and 62, OPEN-1) is: the city level only, and the delivery address is never imported. Mapping the 收件地址 column onto 城市 is exactly the error this check exists to stop. The test (OH04, `orders_test.go:74`) only uses an address with digits, so the gap is invisible. The harm is contained: the value is attached to the same customer, deleted on erasure and included in the export. But the ruling is still broken, and the fix is small.
- **Fix.**
  - Replace the heuristic with an allowlist of Taiwan's 22 cities and counties. Accept both 台 and 臺, simplified Chinese, and optionally the English names. Normalize to one spelling. Anything else is `invalid_city`.
  - Optionally mirror this as a DB CHECK (`city IN (...)`), or at least add a no-digit CHECK, as a backstop.
  - Add test rows: the Chinese-numeral address, `台北市大安區忠孝東路四段`, `王小明` and `amy@mail.tw` must all fail `invalid_city`. Make one of them fail first (red), then fix.
  - Correct the contract sentence.

## P2

### P2-1: the "drift guards" check substrings, not the 0152 bodies.
- **Evidence** (0156:33-42):
  - `record_batch` is guarded only on `p_kind IS DISTINCT FROM 'customers'`.
  - `erase_import_profile` is guarded on "no `historical_orders`" plus "has `DELETE FROM customers.import_profiles p`".
  - `export_import_profile` is guarded on "no `historical_orders`" plus "has `'external_ids'`".
  - The `privacy_audit_insert` guard requires `customers.imported` and the absence of `customers.orders_imported`.
- **What I verified.** I diffed the bodies against 0152. Each is the 0152 body plus only the declared hunks:
  - record_batch: the kind list and the CASE on the audit action.
  - erase_import_profile: the last DELETE.
  - export_import_profile: the `historical_orders` key.

  `CREATE OR REPLACE` keeps the owner and ACL.

  No unit branch currently touches these objects: `unit/w3-05b-blocklist` 0154, `unit/w3-08b-returns` 0155, `unit/ops-02b-support-grant` 0153 and 0150 all scanned clean. None of them mention `apply_erasure`, `erase_import_profile`, `export_import_profile`, `record_batch` or `privacy_audit_insert`. 0156 does not touch `apply_erasure`. So there is no order-sensitive collision today.
- **Why it matters.**
  - A later or parallel patch that keeps those substrings would be silently reverted. For example: a change to `erase_import_profile` that adds a blocklist delete, or a change to the record_batch prune interval.
  - The policy guard is order-sensitive. On a fresh DB, a lower-numbered unit (0154/0155) applies first. If it replaced the policy with "0152 list + X", then 0156 would pass its guard and drop X. On a DB that already has 0156, `migrate.go` gap-fills the lower number later and would drop `customers.orders_imported` instead.
- **Fix.**
  - Guard each replaced function on `md5(prosrc)` equal to the hash of the 0152 body, so any change fails.
  - Guard the policy on the exact expected action set, not a LIKE. A collision then fails loudly and forces the integrator to union the lists.

### P2-2: a mis-mapped PII column into item_name, status or order_id is contained but not refused.
- **Evidence.**
  - `looksLikeContact` (email or TW mobile) runs only on order_id and customer_id (`orders_csv.go:180,208`).
  - `cleanItemName` and status only strip control characters (`orders_csv.go:231,289`).
  - An email or phone mapped to 商品名稱 or 訂單狀態 is therefore archived (status up to 40 runes, items up to 500).
  - A landline or a name in order_id is accepted. It is also echoed in the preview `external_id` of created and updated rows (`orders.go:228-232`).
- **Containment, verified.**
  - Archive rows are deleted on erasure and included in the export.
  - Batch results, results.csv and the audit row carry row, outcome and code only (`orders.go:140-144`, `0156:200`). OH10 and OH12 check this.
  - Errors carry PG codes only (`batch.go:65-71`).
  - No logging in these packages.
- **Fix (cheap).** Apply `looksLikeContact` to each item name and to status. Fail the unit with `invalid_item` or `invalid_status`. Add one DB-free test row each.

### P2-3: only customer ids are tombstoned, not order external ids.
- **Customer-id path.** This path is correct and race-safe. The lookup locks the owner `FOR UPDATE OF o` with `o.active` in the WHERE (0156:120-123). The tombstone EXISTS check runs as a separate statement (0156:125-128). Every erasure caller locks the owner before `apply_erasure` (0078:478 merchant, :485 buyer, :557 replay), and `erase_import_profile` runs inside that lock. So an import cannot write after an erasure has committed, and an erasure always sees committed archive rows.
- **Order-id path.** After erasure, `OH-1001` is gone. A later file that carries the same order number under another live customer id creates it there, with the erased person's items and city.
- **Severity.** Low. The source file has to assert that new owner.
- **Fix.** Optional: in `erase_import_profile`, also write `kind='orders'` digests of the owner's `external_order_id` values (the CHECK already allows `orders`), and check them in `import_orders`.

### P2-4: the privacy export can become unavailable for a customer with a large archive.
- **Evidence.** The cap is 2000 rows × `items_summary` up to 500 runes, which is about 1.5 KB UTF-8 per row. That is several MiB, against the 1 MiB export cap (DELIVERY risk 6, `ErrExportTooLarge`).
- **Why it matters.** The data-subject access export then fails as a whole.
- **Fix.** Pick one:
  - lower the per-customer cap;
  - cap `items_summary` lower in the export;
  - page the archive section of the export.

  At minimum, add a test at the cap that shows the outcome.

### P2-5: evidence hygiene.
- `green.log`, `red.log` and the other logs carry no commit SHA or file hash, so they cannot be bound to `3f29fe2c`.
- The red test is meaningful: removing the erasure DELETE turns OH03's exact function list red (`red.log` line 5), and OH06 fails too. But it is not tied to a SHA.
- OH06 does not exercise the commit-versus-erasure race. The code reasoning above holds; this is a test gap only.
- Re-run OH01-OH12 on the post-fix SHA and record it.

## Checklist results (verified OK)

1. **No commerce leakage.**
   - `git grep historical_orders` outside the migration finds only:
     - `internal/customers/historical.go` and `privacy.go`
     - `internal/httpapi/customer_historical.go`
     - `internal/migrationimport/orders.go`
     - pagination
     - tests and contracts
   - Nothing in `apps/`, reports 0147, console 0148, settlement 0150, CAPI, attribution, the customer list or buyer APIs reads it.
   - OH03 pins the exact set of DB functions whose source names the table (erase / export / read / import_orders). It also checks that no view or matview names it and that there is no foreign key in or out except `buyer.owners`. Because it is an exact-equality list, any future reader turns it red.
   - The money-table row counts and the finance summary are unchanged across a re-import.
2. **PII.**
   - Only mapped columns are read (`orders_csv.go:150-157`). The address column has no alias. OH04 scans every base table for the address text.
   - No address, contact or payment column exists on the table.
   - Results, audit rows, receipts and errors carry no cells. A failed preview row echoes no id.
   - The city check fails (P1-1). Mis-mapped columns are contained (P2-2).
3. **Erasure.**
   - `erase_import_profile` deletes the archive unconditionally by owner (0156:231).
   - `replay_erasures` sets the GUCs (0078:556) and reaches it through `apply_erasure`. OH06 re-inserts a row and replays it.
   - Re-import of an erased customer id fails with `erased`, also inside a race (see P2-3).
   - Keeping the summary shape does not hide failure: a failing DELETE aborts the erasure transaction.
   - An archive-only owner is unreachable. Archive rows need an active owner with an external id, and `external_ids`, `import_profiles` and the archive are deleted together, only in `erase_import_profile`. So erasure cannot "succeed while archive rows remain".
4. **Function re-creation.** The bodies are 0152 plus the declared hunks. The guards are weak (P2-1). No current order-sensitive interaction.
5. **Definer hygiene.**
   - Both new definers: owner `commerce_privacy_writer`, `SECURITY DEFINER`, `search_path=pg_catalog`, all names schema-qualified, `REVOKE PUBLIC`, `EXECUTE commerce_runtime` only.
   - Each calls `tn_authority` (GUC = authenticated scope) and `tn_fence`.
   - The table has ENABLE+FORCE RLS with GUC-scoped policies. Only `commerce_privacy_writer` holds grants (OH07 checks this; runtime gets 42501).
   - Routes take the store from the path and verify it against the bearer (`platform.WithScope`). Permission is `customers:privacy` for import and `customers:read` for the read.
   - The DB-free router test covers the new rows (`imports_test.go:37-45,130-152`).
   - The reader re-validates every item.
6. **Tests.**
   - OH01-OH12 encode the acceptance criteria. Gaps: P1-1 inputs, the P2-2 rows, the race, and the export at the cap.
   - Pins were extended, not weakened:
     - CB02: new function row, table privileges, the DELETE allowlist gains `historical_orders`, the audit action list gains `customers.orders_imported`.
     - Upgrade count 78 -> 79. The integrator must re-count when 0154/0155 land.
