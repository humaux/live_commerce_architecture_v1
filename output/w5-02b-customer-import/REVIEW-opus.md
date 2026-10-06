# W5-02B customer import: independent privacy and security review (Claude Opus, read-only)

- Reviewed: `unit/w5-02b-customer-import` at `dddb1452` (author `a96c81a8` plus the trunk merge). Diff `git diff r3/integration...HEAD`;
  `r3/integration` = merge base `6f285fe1`, so nothing on trunk is missing from the diff.
- Inputs: the brief (Integrator 裁决), DELIVERY.md, AGENTS.md, `contracts/customers-billing-v1.md` §3.1 amendment, draft `contracts/migration-import-v1.md`.
- Run locally, DB-free only, at `dddb1452`: `go test -count=1 ./internal/migrationimport ./internal/customers ./internal/httpapi` exited 0;
  `go vet` on both packages was clean; `bash scripts/dev/check-headers.sh` exited 0. PG suites are **NOT_RUN** here (CI only).
- Evidence class of this review: DESIGN/static plus DB-free MOCK. No PG behaviour is asserted beyond what the author's logs show.

## Verdict: **MERGE-AFTER-FIX**

There is no P0. The two P1 findings are both privacy completeness gaps in erasure and the right of access. Each is small, and both
belong in the still-unshipped migration 0152.

---

## P0
None.

## P1

### P1-1 Erasure is not durable: re-importing an export recreates the erased person (tombstone missing)
- **Evidence:**
  - `0152:238` `erase_import_profile` deletes `migrationimport.external_ids`, and nothing records that the id was erased.
  - `0152:165-182` `import_customers` creates a NEW owner, profile and external id whenever the lookup misses.
  - `0152:150-152` documents that a commit racing an erasure also ends in "created as a NEW owner".
  - `contracts/migration-import-v1.md` §5 says the same: "creates a NEW owner (no tombstone is kept)".
- **Why it matters:** merchants keep their SHOPLINE export and re-import it, and W5-03B order import depends on this flow. One
  routine re-run silently restores the name, phone and email of a person who exercised the right to erasure. The merchant gets no
  signal, and the new owner has no ERASURE privacy action, so the list shows a clean customer. This defeats the invariant at
  `0152:14` ("erasure removes every imported datum") in practice, and it is the exact harm erasure law targets: re-collecting
  erased data from a stale copy (Taiwan PDPA Art. 11 deletion duty; GDPR Art. 17 with the suppression-list practice).
- **Fix:** a per-store salted tombstone. See the ruling below. This changes the schema of an unshipped migration, so it costs less
  now than as a later migration.

### P1-2 The merchant privacy export (access request) omits the imported personal data
- **Evidence:**
  - `internal/customers/privacy.go:61-62` builds the export from orders, consents, claims and privacy_actions only.
  - The list and detail rows expose only `display_name` and `phone_last3` (`0152:374-375`), and `commerce_auth` has no SELECT on
    `email` (`0152:113`).
  - The DELIVERY "Risks" section admits the gap.
- **Why it matters:** the full phone, the email and the external id we hold about an imported customer are visible through no
  merchant path. An access or export request answered with `customers.exported` (`record_export`) would claim to be complete
  while omitting the data. The export route already requires `customers:privacy` (`internal/httpapi/customers.go:65-95`), so
  including the data widens nothing.
- **Fix:** add `customers.export_import_profile(tenant, store, owner)`, owned by `commerce_privacy_writer`, with EXECUTE granted to
  nobody or only to the export's definer path. Model it on `customers.export_tags_notes` in 0139. It returns
  `{display_name, phone_e164, email, source, imported_at, updated_at, external_ids[]}`. Add an `import_profile` section to
  `exportDoc`. Pin it with a CI test: exporting an imported customer contains the synthetic email and phone, and the export
  receipt (`command_results`) does not.

## P2

1. **Unscoped PII read policy** (`0152:130`): `auth_import_profile_read … TO commerce_auth USING (true)`.
   - The 2026-09-30 amendment in customers-billing §3.1 allows `USING(true)` only for the three listed `commerce_privacy_writer` reads.
   - This is a new unscoped read of PII. `read_merchant_customers` already verifies `app.tenant_id` and `app.store_id` against
     `resolve_access` (`0152:311-314`), so a GUC-scoped policy works unchanged.
   - Fix: replace the policy with the GUC scope, as for cbsTables[2:], or add it to the amendment's exception list with a reason.
2. **The scoped owner-insert grant has no negative test.** The brief asked for a `buyer_registration_test.go` pin ("owner insert path not abused").
   - CI11 (`customer_import_test.go:491`) proves only that the runtime login cannot insert owners.
   - Add a test: `SET ROLE commerce_privacy_writer` with the GUCs of store A1; inserting an owner for store B, or for A1 with tenant B,
     gives 42501 or a WITH CHECK violation.
   - Record red evidence by dropping the WITH CHECK of `imp_owner_insert`: the test goes red, then green once restored.
3. **No concurrency test.** A double submit is safe by construction:
   - `command.Run` holds the advisory lock on `cimp-<sha>` with `lock_timeout 0` (`internal/command/command.go:47-62`), so the second
     commit waits and then replays.
   - The per-store advisory lock (`0152:161`) and the `external_ids` primary key stop duplicate owners across different files.
   - No test shows either. Add two goroutines committing the same file: one `replayed:false`, one `replayed:true`, exactly N owners.
4. **Batch retention is lazy per store** (`0152:200-202`). A store that never imports again keeps external ids in `batches.results`
   forever, so the 90-day rule is not enforced. Fix: hook the prune into an existing retention sweeper, or document the gap as known.
5. **`external_id` has no shape rule** (`customers_csv.go:228-235`, only length and control characters are checked).
   - A mis-mapped column (phone or email chosen as `external_id`) would put PII into `external_ids`, `batches.results` and results.csv.
     The "no PII in results" invariant then rests on the merchant's mapping.
   - Fix: refuse `@` and anything that `normalizeTWMobile` accepts as an `external_id` (code `invalid_external_id`), or document why not.
6. **Evidence binding:**
   - green.log, red.log and run2.log have no SHA or command header.
   - green.log predates the merge `dddb1452`.
   - The only red run is CI07; the brief's K3 consent counter-example ("consent is not fabricated") is still owed.
   - CI must rerun `^TestCustomerImport$`, the CB02/CB09 pins and `TestR2IntegrationUpgradeFromReleaseHead` at the merge SHA.
7. **Brief and contract drift the integrator must settle:**
   - The commit refuses `Idempotency-Key` (`merchanttools.go:245`), while the brief §4 requires it.
   - The erasure patch went into `apply_erasure`, not `erase_owner`. This is better, because `replay_erasures` inherits it.
   - The migration is 0152, not 0140.
   - Update the brief or freeze the contract text.
8. **Documentation drift:**
   - The header of `customer_import_test.go:3` says "CI01-CI12" but only CI01-CI11 exist.
   - `imports_test.go:3` says "CI01-CI09".
9. **UX:** after a commit, the same bytes with a different mapping are `idempotency_conflict` forever, because the key is the file hash
   alone. A merchant who committed a wrong mapping must change the file. Say so in the contract §4 and in the W5-U1 copy.

## Checklist results (with evidence)

| # | Topic | Result |
|---|---|---|
| 1 | PII in logs, audit, receipts, results, errors | OK. `abort` keeps only the PG code (`batch.go:65-71`). Parse errors map to codes (`customers_csv.go:117-133`). The audit row holds the action only (`0152:205`). The receipt holds `{batch_id,counts}` (`customers.go:136-148`). Results have no name, phone or email field (`batch.go:112-117`). httpapi and platform have no logging. CI09 scans audit, receipts, batches, external ids, logs and results.csv. P2-5 is the only residual. |
| 1 | Size, row caps, parser | OK. `MaxBytesReader` caps the body at 2 MiB (`imports.go:122`) and Go re-checks it (`customers.go:166`). The 5000-row cap counts non-blank rows (`customers_csv.go:139`). Invalid UTF-8 is refused before parsing. The BOM is stripped. Strict RFC 4180 (no LazyQuotes) refuses bare or unterminated quotes ("quote bombs") as a whole file, bounded by the 2 MiB cap. A field-count mismatch fails the row with no index panic. The external id echo is truncated to 64 runes. Formula cells are unguarded on input (`customers_csv.go:78-83`) and every results.csv cell is guarded (`batch.go:167`). No other CSV export carries customer names (grep). |
| 2 | Consent | OK. No definer writes `consent_events` (`0152:153-188`). A consent column only sets `consentIgnored` (`customers_csv.go:155`). Every send or ads gate goes through `customers.consent_allows` (`0078`, absent = false; `0080:113,151`; `0113:108,136`). An imported owner has no capability session (CI01) and no PSID, so nothing can message it. No reader outside the list consumes `import_profiles` (grep). CI04 asserts 0 events and `allows`=false. |
| 3 | Isolation | OK. Tenant and store come from the bearer through `tn_authority` (GUCs verified against `resolve_access`, `0139` tn_authority), with a final `tn_fence`. Every lookup filters on `tenant_id` and `store_id`, and the PKs include them. FORCE RLS is on all three tables (`0152:97-102`). There is no login grant (CI11). Definers use `search_path=pg_catalog` and schema-qualified names (all four new ones, plus the replaced ones). |
| 3 | `INSERT(tenant_id,store_id)` on buyer.owners | Minimal: column-level, with the GUC-scoped WITH CHECK `imp_owner_insert` (`0152:111,128`). The only INSERT runs after `tn_authority` has proven GUC = p_store and `customers:privacy` (`0152:160,177`). No `commerce_privacy_writer` definer runs dynamic SQL on data paths. Cross-store abuse would need a new privacy_writer definer. Test gap: P2-2. |
| 4 | Erasure | OK, except the tombstone (P1-1). The `apply_erasure` patch sits behind a drift guard (`0152:33-35,258-265`) and keeps the owner and ACL. `replay_erasures` sets GUCs and calls `apply_erasure` (0078), so it re-deletes (CI07). The scrub matches the external id by value across every results entry, failed rows included (`0152:230-237`). |
| 5 | CREATE OR REPLACE | OK. `read_merchant_customers` matches the 0139 body (the highest definition; 0143-0148 do not touch it) apart from the W5-02B hunks (diffed). `tn_lock_owner` and `tn_owner_visible` are the 0139 text plus one EXISTS. Same signature, so the owner and ACL are kept; the comment was refreshed. |
| 6 | Idempotency | OK. The key is `cimp-<sha256[:32]>` and the request hash covers mapping and N (`customers.go:136-140`). A stale or failed commit rolls back and records nothing, so a corrected N can commit. `UNIQUE(file_sha256)` is the backstop. Test gap: P2-3. |
| 7 | Routes | `customers:privacy` on all three routes (`imports.go:77,129`). Bearer is canonical, Idempotency-Key is refused, ids are canonical (`trackingImportRoute`). Responses are no-store. The DB-free full-router test `TestCustomerImportRoutesTransportRules` passes. |
| 8 | Tests and pins | CI01-CI11 cover the brief's acceptance (CI06 403 for customers:write and read-only; CI08 limits and drift). Red evidence exists for CI07 only (P2-6). The pin changes are additive (CB09 `imported` key, CB02 privilege lists, R2 73 to 74) and nothing was loosened. |
| 9 | Headers and doc comments | `check-headers.sh` is OK, and every new object has a COMMENT. Count drift: P2-8. |

---

## Tombstone ruling

**It is necessary (P1-1), and the proposal is privacy-safe under the conditions below. Accept it.**

Necessity: the merchant's SHOPLINE export is a standing copy that our erasure cannot reach. Without a suppression record, an erasure lasts
only until the next import. A minimal record kept only to honor an erasure is accepted practice: it is the deletion duty's own
necessity, the same pattern as a marketing suppression list.

Privacy safety:
- The tombstone holds a salted hash of one pseudonymous merchant id: no name, no phone, no email, and no link to an owner id.
- The salt is per store, so the same SHOPLINE id in two stores gives two unrelated digests. Nothing cross-store or cross-tenant is
  learnable.
- The salt lives in a separate table, so a leak of the tombstone table alone cannot be brute-forced. An attacker holding the whole DB
  could enumerate sequential ids, but would learn only "SHOPLINE id X of this store was erased". The merchant already holds that fact
  and needs it to delete at the source too.
- The digest is still pseudonymous data. Keep it for the store's lifetime (the basis is the erasure duty), and delete the salt with the
  store, which crypto-shreds every tombstone.

Minimal schema (append to 0152):
```sql
-- one random 32-byte salt per store (two v4 uuids, the 0125:129 precedent: pgcrypto is absent and hidden by search_path=pg_catalog)
CREATE TABLE migrationimport.store_salts(tenant_id uuid NOT NULL, store_id uuid NOT NULL,
 salt bytea NOT NULL CHECK (octet_length(salt)=32), PRIMARY KEY (tenant_id,store_id));
CREATE TABLE migrationimport.erased_external_ids(tenant_id uuid NOT NULL, store_id uuid NOT NULL,
 kind text NOT NULL CHECK (kind IN ('customers','orders')), id_digest bytea NOT NULL CHECK (octet_length(id_digest)=32),
 erased_at timestamptz NOT NULL DEFAULT clock_timestamp(), PRIMARY KEY (tenant_id,store_id,kind,id_digest));
-- id_digest = pg_catalog.sha256(salt || convert_to(<canonical external_id exactly as stored in external_ids>,'UTF8'))
```
- **Rights:** FORCE RLS and GUC-scoped policies on both tables. `commerce_privacy_writer` gets SELECT and INSERT only: no UPDATE, and no
  DELETE (except by store deletion). No login role gets any grant. Extend the CB02 privilege and DELETE pins.
- **Erasure:** `erase_import_profile` lazily creates the salt (`INSERT … ON CONFLICT DO NOTHING`), then inserts the digest of each
  external id of the owner. It does this BEFORE deleting `external_ids`. `replay_erasures` inherits the step through `apply_erasure`.
- **Import:** in the create branch of `import_customers`, check the digest in its own statement after the owner lookup, so a
  just-committed racing erasure is visible in READ COMMITTED. A match returns outcome `failed` with code `erased`: no owner, profile or
  external id is written.
- **Go:** accept the `failed/erased` outcome from the definer and count it as failed. It is excluded from `expected_apply_rows`. Add an
  `erased_rows` counter to the preview. The preview row and the stored results carry the row number and code only, and **omit
  `external_id`**: the row number refers to the merchant's own file, so no new identity is disclosed.
- **Tests:**
  - CI07b re-imports the CI01 file after erasing SL-0002: SL-0002 is `erased`, with 0 new owners for it; `SL-0002` appears in no
    preview, batch, results.csv or receipt; the digest is not `sha256(external_id)` unsalted; the same id in tenant B's store still
    imports.
  - Red evidence: comment out the digest insert, and CI07b goes red.
- **Known limits (record them, do not build now):** a person who is erased and re-appears under a new SHOPLINE id, or who erased
  themselves as a buyer and therefore has no external id, is not caught. That needs the verified-contact contract (B24). There is no
  merchant override in v1: a returning customer comes back through the buyer flows with fresh consent, never through an import.

## Fix list for MERGE
1. P1-1 tombstone as ruled, with CI07b red then green.
2. P1-2 `import_profile` in the merchant export, with a CI test.
3. Strongly recommended in the same pass: P2-1 (scope the policy) and P2-2 (owner-insert negative test). The rest can follow up.
