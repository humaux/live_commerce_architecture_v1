# W5-02B customer import: independent re-verification of the fix round (Claude Opus, read-only)

- **Scope:** fix commit `833a2b69` against `dddb1452` (`git diff dddb1452 833a2b69`). This worktree's HEAD is `833a2b69` and its tree is clean.
  `r3/integration` is `6f285fe1`, which is the merge base, so the branch has no trunk drift.
- **Inputs:** REVIEW-opus.md, the integrator's tombstone ruling, red-fix.log, red-fix-ci13.log, green-fix.log, the brief and
  `contracts/migration-import-v1.md`.
- **Checks run locally at `833a2b69`, all DB-free:**
  - `go test -count=1 ./internal/migrationimport ./internal/customers ./internal/httpapi` exited 0.
  - `TestExternalIDThatLooksLikeContactIsRefused` passed.
  - `go vet ./internal/migrationimport ./internal/customers ./tests/foundation` and `go build ./...` were clean.
  - `scripts/dev/check-headers.sh` was OK.
- **Not run:** the PostgreSQL suites (CI only).
- **Evidence class:** static review plus DB-free tests. Every PG claim below comes from reasoning about the code or from the
  author's logs.
- **Line references:** `0152:` means `migrations/0152_customer_import.sql` at `833a2b69`.

## Verdict: **MERGE-AFTER-FIX**

| Item | Result |
|---|---|
| P1-1 tombstone | **FIXED** for every row that reaches the database. One gap remains (N-1 below). |
| P1-2 import profile in the merchant export | **FIXED** |
| New P0 / P1 | None |
| New P2 | N-1: the raw erased id is stored again when its row fails Go validation. This breaks the ruling's "no raw id is kept anywhere". |

Merge after three things:
1. Fix N-1. This is one line plus one test row.
2. Fix the CI12 negative-scan token (P3-1).
3. CI runs the PG gates at the merge SHA (E-1).

---

## 1. Tombstone

### 1a. Implemented as ruled: yes

| Ruling item | Evidence |
|---|---|
| Per-store 32-byte salt from two v4 uuids | `0152:102-107`: `CHECK octet_length=32`, PK `(tenant,store)`. `0152:276`: `uuid_send(gen_random_uuid())\|\|uuid_send(gen_random_uuid())`, about 244 random bits. |
| `erased_external_ids(tenant,store,kind,id_digest,erased_at)`, digest = `sha256(salt \|\| canonical id)` | Table at `0152:108-115`. Written at `0152:278-282` as `sha256(sa.salt\|\|convert_to(e.external_id,'UTF8'))`. |
| FORCE RLS with store-scoped policies | `0152:123-126`. The SELECT and INSERT policies use the `app.tenant_id`/`app.store_id` GUC scope (`0152:156-162`). |
| Privacy writer has SELECT and INSERT only | `0152:129`. There is no UPDATE or DELETE grant, and no other role has a grant. The CB02 frozen list pins it (`customers_billing_schema_test.go:222-224`). |
| Digest written before the id is deleted | `0152:275-283` runs before the scrub loop and the DELETE (`0152:284-293`), in the same transaction. |
| Import refuses with `erased`, and the preview counts `erased_rows` | `0152:212-217` returns `{row,outcome:failed,code:erased}` and writes nothing. Go accepts that outcome and clears the id (`internal/migrationimport/customers.go:244-252`). The preview counts it (`customers.go:287`). |
| No raw id kept | Holds for rows that reach the definer. It does **not** hold for rows that fail in Go (N-1). |

### 1b. Canonicalization: identical at erase time and at import time

- **One code path.** The id is canonicalized only in Go: `strings.TrimSpace` followed by `unguardCell` (`customers_csv.go:150`).
  There is no case folding and no NFC on `external_id`; only `name` is NFC-normalized (`customers_csv.go:237`).
- **Erase time:** the digest input is `external_ids.external_id`, the value that `import_customers` stored from the same `v_ext`
  (`0152:199,222-223`).
- **Import time:** the digest input is that same `v_ext`, produced by the same Go path (`0152:215`).
- **Hashing:** both sides hash in SQL with the same expression, `convert_to(...,'UTF8')`. No Go hashing exists, so the two
  sides cannot diverge.
- **Comparison semantics:** the match is byte-exact, the same as the `external_ids` lookup that decides "updated vs created".
  The tombstone is therefore neither stricter nor looser than identity matching.
- **Residual risk (P3):** if anyone later adds NFC or case folding to `external_id`, every existing tombstone silently stops
  matching. Add a one-line comment at `customers_csv.go:227` saying the canonical form is frozen because of the tombstone
  digests.

### 1c. The salt is not exposed

- `commerce_privacy_writer` is the only grantee (`0152:129`).
- That role is `NOLOGIN NOBYPASSRLS` (`0078:64`), and no `GRANT commerce_privacy_writer TO …` exists in any migration (grep).
- No function returns the salt or a digest. `git grep store_salts` outside 0152 finds only the tests and the CB02 pin.
- `export_import_profile` returns no tombstone data (`0152:305-310`).
- The CI11 "no unexpected table grants in schema migrationimport" check automatically covers the two new tables.

### 1d. The commit-vs-erasure race is closed

There is no test for it, so here is the reasoning:

- **What erasure locks.** Every erasure entry point locks the owner row `FOR UPDATE` before it calls `apply_erasure`:
  - merchant erasure: `0078:478`
  - buyer erasure: `0078:485`
  - `replay_erasures`: `0078:557`

  The hook `erase_import_profile` sits at the top of `apply_erasure` (patched before the CD4 needle, `0152:336`). The same
  transaction then sets `owners.active=false` (`0078:276`).
- **What import locks.** The import lookup is `… JOIN buyer.owners o … AND o.active … FOR UPDATE OF o` (`0152:201-204`). The
  function refuses any isolation level other than READ COMMITTED (`0152:194`).

The four cases:

1. **Erasure commits before the lookup.** The `external_ids` row is gone, so the lookup misses. The `ELSIF EXISTS` is a
   separate SPI statement in a VOLATILE function, so it takes a fresh READ COMMITTED snapshot. It sees the committed tombstone,
   and the row comes back as `erased`.
2. **Erasure holds the owner lock when the lookup runs.** The lookup blocks on `FOR UPDATE OF o`. After the erasure commits,
   PostgreSQL re-checks the row against its newest version, where `o.active=false`, so the row is dropped and `v_owner` is NULL.
   The tombstone check then runs as a new statement after the commit, and the row is `erased`.
3. **The import locks the owner first.** The erasure blocks at `0078:478` until the import commits. The import's `updated` only
   touched the existing profile. The erasure then writes the tombstone and deletes everything.
4. **Two erasures in one store race to create the salt.** The second `INSERT … ON CONFLICT DO NOTHING` waits on the uncommitted
   PK and then does nothing. Its next statement (`0152:278`) takes a new snapshot and finds the committed salt. If the first
   transaction aborted instead, the second one inserts its own salt.

No interleaving creates an owner for a tombstoned id. The `0152:194` guard is essential: under REPEATABLE READ, the re-check in
case 2 would not see the committed erasure. Keep it. A PG test that holds the owner lock in one connection would turn this
reasoning into evidence; it is optional.

### N-1 (new, P2, should be fixed before merge): an erased id is stored again when its row fails validation in Go

- **Evidence:**
  - Only rows with `outcome==""` are sent to the definer (`customers.go:209-210`), so the tombstone check never sees a row that Go
    already refused: `duplicate_external_id`, `invalid_phone`, `invalid_email`, `invalid_name`, `name_too_long`, or `required`
    (missing name).
  - Every row, with `r.externalID`, is written to `batches.results` (`customers.go:190-191`) and served in results.csv
    (`batch.go:167`).
- **Example:** a stale export holds the erased SL-0002 twice, or holds it with a landline or other invalid phone after the phone
  column is mapped. The raw `SL-0002` is then stored again, even though erasure scrubbed it.
- **Why it matters:**
  - Erasure cannot reach the new copy: the owner was already erased, and the scrub at `0152:284-291` will never run again for it.
  - Because the batch prune is lazy (REVIEW P2-4), the copy can be retained indefinitely.
  - This breaks the ruling ("no raw id is kept anywhere") and makes the contract claim at `contracts/migration-import-v1.md`
    §5 false ("the preview row, the stored result and results.csv carry the row number and code only, never the id").
- **Severity:** the id is a pseudonymous merchant id with no name, phone, email or owner link, and triggering this needs a
  stale file in which the erased person's row now fails. That is why it is P2 and not P1.
- **Fix, either of these:**
  - (a) Do not store `external_id` for any failed row: in `customers.go:191`, use `ExternalID` only when `r.outcome != failed`.
    The row number already identifies the row in the merchant's own file.
  - (b) Send failed rows that have a well-formed id through a digest check.

  (a) is one line.
- **Test:** extend CI07b with a second `SL-0002` row (`duplicate_external_id`) or an invalid-phone variant, and assert that no
  batch or results.csv contains `SL-0002`.

## 2. The privacy export includes import_profile

- **Store and owner scope: OK.**
  - `export_import_profile` calls `tn_authority(p_hash,p_store,'customers:privacy')`. That call verifies the bearer and that the GUCs
    match `resolve_access`.
  - It filters on `a.tenant_id`, `p_store` and `owner_id=p_customer` (`0152:304,310`).
  - The `external_ids` subquery is correlated on the same tenant, store and owner (`0152:308-309`).
  - `tn_fence` runs at the end (`0152:311`).
  - Go passes `scope.StoreID` and the path `customer_id` only after `loadDetail` succeeded (`internal/customers/privacy.go:82`).
- **No export values in a receipt, audit row or log: OK.**
  - The `record_export` summary is `exportSummary(...)`, which is counts only (`privacy.go:101`).
  - The body goes to `writeAttachment` (`internal/httpapi/customers.go:79-81`). It is not stored in `ops.command_results`.
  - The audit row is the action only (`0078` record_export).
  - The httpapi and customers packages do no logging.
  - CI12 asserts this against `privacy_actions`, `command_results` and `audit_events` (but see P3-1).
- **SECURITY DEFINER with search_path=pg_catalog: OK** (`0152:300`).
  - Owner is `commerce_privacy_writer`. EXECUTE is revoked from PUBLIC and granted to `commerce_runtime` (`0152:318-324`). The
    CB02 pin covers the grant (`customers_billing_schema_test.go:462`).
  - **Deviation from REVIEW's suggestion (accepted):** REVIEW proposed "EXECUTE nobody". The commit grants EXECUTE to
    `commerce_runtime` behind the same `customers:privacy` bearer check, GUC check and fence as the route itself. That widens
    nothing.
- **Go decode:** `exactKeys` plus a strict decode, with `external_ids` required to be non-nil (`privacy.go:93-98`). A malformed
  result fails closed with `ErrUnavailable`.

## 3. P2 fixes

| Item | Result |
|---|---|
| `commerce_auth` read policy is store-scoped | **FIXED.** `0152:164` uses the GUC scope. The only `commerce_auth` reader, `read_merchant_customers`, verifies both GUCs against `resolve_access` before it reads (`0152:384-385`). Red evidence: red-fix.log shows CI15 failing with qual `"true"` at `dddb1452`. |
| CI13 owner-insert negative test | **Meaningful.** It covers a control insert in scope, another store of the same tenant, another tenant, a scope of another tenant, and the column-grant refusal of `id`. Mutation red is in red-fix-ci13.log: with `WITH CHECK (true)`, it fails "other store, same tenant: … got \<nil\>". |
| CI14 parallel commits | **Weak; it would only sometimes catch a regression.** See below. |
| `external_id` format rule | **OK** for SHOPLINE ids. See below. |

**CI14** (`customer_import_test.go:623-645`) fires two unsynchronized goroutines at 40 rows.
- **What it catches.** If the `command.Run` advisory lock is removed while the two requests actually overlap, the second
  request fails: either the 409 path on `UNIQUE(file_sha256)` or a unique violation on `external_ids`. Either way `st != 200`,
  so CI14 goes red.
- **What it misses.**
  - If the requests do not overlap (40 rows take milliseconds), the second request replays from `command_results`, and CI14
    stays green even with the lock gone.
  - Removing the per-store lock at `0152:197` is never detected, because the same-file case is already serialized by
    `command.Run`.
- **Verdict:** CI14 is a smoke test, not a regression gate.
- **To harden:** add a barrier. For example, hold `pg_advisory_xact_lock` on the command key, or a `FOR UPDATE` on a store row,
  in a third connection. Release it once `pg_stat_activity` shows both requests waiting. Then record one mutation red with the
  lock removed.

**`external_id` format rule** (`customers_csv.go:233,270-276`):
- **What it refuses:**
  - any id containing `@`;
  - any id that `normalizeTWMobile` accepts, after stripping spaces, `-`, `()` and `.`, plus a `+`/`886`/`00886` prefix and one
    leading `0`: a 9-digit `9xxxxxxxx`, a 10-digit `09xxxxxxxx`, or `8869xxxxxxxx` and its `+` and `00` variants.
- **What the docs say about the SHOPLINE id format:**
  - The brief only says `external_id` = "SHOPLINE 顾客 ID" (brief §3). The real header sample is still CI-OPEN-1, so no
    document fixes the format.
  - The repo's SHOPLINE study records SHOPLINE ids as Mongo ObjectIds (`docs/discovery/2026-10-02-shopline-admin-study.md:10`,
    `68f75a08…`).
- **Effect on ObjectIds:** a 24-hex ObjectId has no `@` and never reduces to 9 digits, so it is never refused. The DB-free test
  covers `5f1c2a9e8b7d4e0012345678`, `10023` and `SL-0001`.
- **Residual:** if a merchant maps a numeric membership-number column whose values happen to be 9 digits starting with `9`, or
  10 digits starting with `09`, those rows fail. The contract tells the merchant to prefix such ids. Acceptable; re-check when
  CI-OPEN-1 delivers the real header.

## 4. No regression

- **Function bodies:**
  - The fix replaces no existing function body.
  - `import_customers`, `erase_import_profile` and `export_import_profile` are new `CREATE FUNCTION`s in this still-unshipped
    migration.
  - The `apply_erasure` patch (`0152:331-338`), `tn_lock_owner`, `tn_owner_visible` and `read_merchant_customers` are byte-identical
    to `dddb1452` (no hunks in the diff). REVIEW verified those against their highest prior definitions (0078/0113/0139 for
    `apply_erasure` through the drift guard at `0152:35-37`; 0139 for the others).
  - Trunk has not moved since: `r3/integration` = merge base `6f285fe1`, and no newer migration exists.
- **ACLs:**
  - Additions are exactly the ruled ones: SELECT and INSERT on the two tombstone tables for `commerce_privacy_writer`, and EXECUTE
    on `export_import_profile` for `commerce_runtime`.
  - The only change to an existing object is `auth_import_profile_read`, and it is a tightening (`USING(true)` becomes the GUC
    scope).
  - The `commerce_auth` column grant, the `buyer.owners` column INSERT and `imp_owner_insert` are unchanged. The CB02 pins were
    extended additively.

## 5. Evidence honesty

- **E-1 (gap, must close in CI):** green-fix.log says "working tree on top of dddb1452 … with the fix round applied". It is not
  bound to `833a2b69` and records no file hashes. The tree it ran on cannot be reconstructed. The timestamps make the gap
  concrete:

  | Time | Event |
  |---|---|
  | 21:35:20 | commit time, which is also the mtime of green-fix.log |
  | 21:34:56 | mtime of `0152_customer_import.sql`, which is also the mtime of red-fix-ci13.log (the CI13 mutation was written into this file and reverted) |
  | 86 s | the green run's own reported duration |

  If green-fix.log was written live, the green run started about 21:33:54 and overlapped the CI13 mutation window in the same
  worktree. If the header was added afterwards, the run came before the mutation. Either way, the file that was committed was
  last written after, or during, the green run.
- **What is verified:** the committed `imp_owner_insert` is correct (`0152:154`, `WITH CHECK (%s)`), and the DB-free suites pass
  at `833a2b69`.
- **Required:** CI must run `^TestCustomerImport$`, `TestCustomersBillingCB02*`, CB05, `TestCustomerTags` and
  `TestR2IntegrationUpgradeFromReleaseHead` at `833a2b69` (or the fix SHA) before merge.
- **Red evidence:**
  - red-fix.log is bound to `dddb1452` and shows CI07b, CI12 and CI15 red before the fix. That is a valid red for the tombstone.
    It is not the ruled "comment out the digest insert" mutation, but it shows the same thing: without the digest the erased id
    is `created`.
  - CI13 has a mutation red.
  - CI14 has no red (see §3).

## P3 (docs and tests, non-blocking)

1. **CI12 checks the wrong phone.** Its negative scan uses `"900000011"` (`customer_import_test.go:571`). That is the old CI03
   phone; after the CI07b re-import, SL-0001's phone is `+886900000001`. The phone part of the scan is vacuous. Use
   `"900000001"` and `secretName`.
2. **Stale function comment.** `COMMENT ON FUNCTION migrationimport.import_customers` (`0152:571`) still says it returns
   `outcome created|updated`. It now also returns `failed/erased`.
3. **Crypto-shred claim has no mechanism.** The contract and comments say "deleting the salt with the store crypto-shreds every
   tombstone". `store_salts` has no FK to `control.stores` and no role holds DELETE, so no store-deletion path removes it yet.
   Either word it as a future step or add the cascade when store deletion exists.
4. **Frozen canonical form.** Add a comment on the frozen `external_id` canonical form (§1b).
