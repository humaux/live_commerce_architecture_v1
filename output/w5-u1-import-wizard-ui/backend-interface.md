<!-- Purpose: exact W5-U1 backend interface map and existing UI helper inventory.
Depends on: Go import/history handlers and DTOs, migration-import-v1, source hashes below.
Used by: W5-U1 root author and independent test author; no implementation or runtime acceptance. -->
# W5-U1 backend interface

- Read-only explorer: `codex-w5-u1-ui-sub-api`; parent task `a83bad3d-2b4d-4b4a-b598-85f9e4a99cea`.
- Source worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/w5-u1-import-wizard-ui`, base/HEAD `dc11b5ce8ceaf3c755d7192545f34f3c579108cf`.
- Owner/parent has authorized W5 from this W6 checkpoint; old DRAFT labels and old merge dependency wording are not a blocker. W6 CI remains independent/pending.
- Read preamble → AGENTS → PROCESS → invariants → UI/customer/order briefs → contract and scoped source. No source edits, tests, PG, build, browser, keys or external actions. All facts below are SOURCE_REVIEW, not a runtime PASS.

## Exact routes

Let `I = /v1/admin/stores/{store_id}/imports` and `C = /v1/admin/stores/{store_id}/customers/{customer_id}`. IDs in URL paths are canonical UUIDs except the archive response's source `order_id`.

| Method/path | Query/body | Success/exception | Source |
| --- | --- | --- | --- |
| `POST I/customers/preview` | Optional `mapping`; raw original CSV bytes, `Content-Type: text/csv` (parameters accepted) | 200 CustomerPreview even with failed rows; dry-run transaction always rolled back | `internal/httpapi/imports.go:36-50`; `internal/migrationimport/customers.go:110-119` |
| `POST I/customers/commit` | Required `expected_apply_rows`; optional `mapping`; same original CSV bytes | 200 CommitResult; special 409 fresh CustomerPreview; coded 409 idempotency conflict; coded 422 nothing to apply | `imports.go:53-68,174-176`; `customers.go:122-161,176-197` |
| `POST I/orders/preview` | Optional `mapping`; raw original CSV bytes | 200 OrderPreview; dry-run rollback | `imports.go:72-86`; `orders.go:77-86` |
| `POST I/orders/commit` | Required `expected_apply_rows`; optional `mapping`; same original CSV bytes | 200 CommitResult; special 409 fresh OrderPreview; same coded conflict/nothing outcomes | `imports.go:88-103`; `orders.go:92-153` |
| `GET I/{batch_id}/results.csv` | No body. Optional query exactly `only=failed`; omit it for all committed results | 200 BOM UTF-8 CSV attachment, fixed filename `customer-import-results.csv` for BOTH kinds | `imports.go:106-123`; `batch.go:142-173` |
| `GET C/historical-orders` | No body/key. `limit=50`; optional `after=<opaque next_cursor>`; omit after at first/end | 200 HistoricalOrdersPage; unknown/other-store/invisible customer is 404. Erasure caveat below. | `customer_historical.go:26-35`; `customers/historical.go:56-105` |

All import routes use `customers:privacy`; archive GET uses `customers:read`. Scope comes from the bearer/session, never CSV/body tenant/store fields. Import routes reject any `Idempotency-Key` header (`merchanttools.go:243-257`), including on preview/results. Archive GET also rejects a body/key (`customers.go:129-132`). No other import method, polling/status route, batch listing or readiness endpoint is registered in `imports.go:33-124`.

Import query grammar (`imports.go:127-143`): only listed keys, each once and non-empty, malformed URL encoding refused, raw query ≤4096 bytes. A query-less preview/results request is valid. `expected_apply_rows` is parsed as an integer in `[0,5000]` (`imports.go:54-58,89-93`); clients should send the server preview's `apply_rows`. JSON bodies, multipart and added file-hash/tenant/store query fields are not this protocol.

## DTOs — JSON keys are exact

CustomerPreview (`customers.go:34-56`):

```ts
{
  file_sha256: string; headers: string[]; mapping: Record<string, string>;
  rows_total: number; new_rows: number; update_rows: number; apply_rows: number;
  failed_rows: number; erased_rows: number; consent_ignored_rows: number;
  rows: {row: number; external_id: string;
    outcome: "created" | "updated" | "failed";
    code?: string; consent_ignored?: boolean}[];
}
```

OrderPreview (`orders.go:34-57`) has the same file/header/mapping and `rows_total`, `new_rows`, `update_rows`, `apply_rows`, `failed_rows`, `erased_rows`, but **replaces** `consent_ignored_rows` with `city_dropped_rows`. Its row shape is `{row, external_id, outcome, code?, warning?: "city_dropped"}`; there is no `consent_ignored`.

Counts satisfy `apply_rows = new_rows + update_rows` and `rows_total = apply_rows + failed_rows`. Customer counts are data rows; order counts are aggregated order units, not CSV lines. Multiple lines with one order number become one unit; `row` is the first contributing non-blank data line number, starting at 1, excluding the header (`orders_csv.go:137-168`). Do not call it a physical CSV line number.

Preview output is a safe projection, not raw cells: no name, phone, email, items, address or raw status. `headers` and mapping names are metadata. `external_id` is present as `""` on EVERY failed preview row (`customers.go:255-260`; `orders.go:241-243`), including erased rows. On successful customers it is the customer source ID; on successful orders it is the source order number. `code`/false `consent_ignored`/empty `warning` are omitted. `erased_rows` counts failed erased entries; the ID must never be reconstructed from the local CSV for display/download.

CommitResult for both kinds (`customers.go:60-65`; `orders.go:61`):

```ts
{batch_id: string; created: number; updated: number; failed: number; replayed: boolean}
```

No status/job/progress/results array/hash/mapping/erased/city/consent count is returned by commit. Preserve the corresponding preview in memory for warning/count display. `replayed:true` means the original summary was replayed, not that another new write occurred. It does not establish that every customer still exists after subsequent erasure.

HistoricalOrdersPage (`customers/historical.go:25-39`):

```ts
{items: {order_id: string; ordered_at: string; status: string;
  total_minor: number; currency: "TWD"; items_summary: string; city: string | null}[];
 next_cursor: string; total: number}
```

`order_id` is external text (1..64 runes), **not UUID**; no internal row/customer ID is exposed. Dates are UTC RFC3339 strings, status is display text ≤40 runes, items summary is text ≤500, `total_minor` is display-only TWD cents ≤10^12, city is null or one of the canonical 22 Taiwan city/county names (`historical.go:48-53`; contract §7). `items` can be empty, `next_cursor` is `""` at end, and `total` is the full archive count.

Historical query uses `after`, **not `cursor`**. Default 50, allowed canonical `limit` 1..100; after ≤1024 bytes, raw query ≤4096. Other/repeated/empty keys and malformed pagination fail 422 `invalid_request` (`customer_tags.go:108-113`; `customers.go:203-254`; `pagination.go:25-99`). Cursor is opaque base64url, bound to tenant/store/customer and collection `customer-historical-orders`, keyset `(ordered_at, internal id)`; forward it unchanged, reset it on scope/customer change (`historical.go:63-70,97-99`). UI brief requests pages of 50. The current customer-list DTO has `imported` but **no `historical_orders_count`**; use archive `total` in the detail section (contract §7:126-128).

Exact-source erasure caveat: the contract/Go comment promises erased-customer 404, but the SQL read calls `tn_owner_visible` (`0156:178-179`), whose current body lacks an active predicate (`0152:355-358`). An erased import-only owner becomes invisible after profile deletion and is 404; an erased owner still visible through a retained checkout order/bundle can receive 200 with an empty archive. Erasure deletes the archive, so this is not proof of surviving private history. The UI must immediately hide/unmount history when authoritative `detail.active` is false; a universal erased-GET404 oracle would overstate the source contract. No backend patch is proposed by this explorer.

## Mapping and file limits

- Body cap 2,097,152 bytes for BOTH kinds; UTF-8, optional BOM, RFC4180 comma CSV; skip blank records; cap 5000 non-blank data LINES for both, BEFORE order aggregation. Old brief's 20,000 order lines is superseded (`batch.go:34-39`; `orders_csv.go:106-145`; contract §7:108).
- `mapping` is URL-encoded JSON object field → original header text, ≤2048 bytes decoded JSON; header value ≤100 runes; `""` explicitly unmaps a field, omitted key permits auto-detection. Unknown field/header, explicitly ambiguous header or assigning one column twice is refused. Preserve the **exact requested mapping used for the commit**, not a subsequently edited/auto-returned variant (`customers.go:87-105`; `customers_csv.go:162-201`). Encoded URL must also fit 4096 bytes.
- Customer mapping keys: required `external_id`, `name`; optional `phone`, `email`, `consent`. `consent` exists only to flag ignored non-empty cells (`customers_csv.go:27-36,154-155`). Notes/tags/address are not import fields. Phone/email unmapped keeps an existing stored value; mapped empty cell clears it. Marketing consent is never created.
- Order keys: required `order_id`, `customer_id`, `ordered_at`, `status`, `total`; optional `item_name`, `item_qty`, `city` (`orders_csv.go:31-44`). Unmapped address/phone/email/notes are never read. Backend aliases are authoritative: customer aliases `customers_csv.go:47-52`; order aliases `orders_csv.go:47-55`. Folding trims/case-folds, folds full-width ASCII, replaces spaces/hyphens with underscore (`customers_csv.go:60-74`).
- There is no header-only HTTP route. A file with no usable required mapping yields coded 422 `required` with no header DTO. A manual-mapping step therefore needs a header-only RFC4180/UTF-8 read of the selected file in memory; retain the original Blob/bytes for sending, and display only server preview projections after mapping. No row contents in storage/logs; only the owner's allowed recent column-name mapping may be saved with try/catch.
- Customer source ID ≤64 and cannot resemble email/TW mobile; name NFC ≤80; optional phone normalized to TW E.164 and email lower-case ≤254. Re-import same customer source ID updates its profile without identity merge (`customers_csv.go:222-273`; contract §3).
- Order date no-zone input means Taipei; accepted date/time layouts are `orders_csv.go:73-75`. Whole NT$ values (optional grouping/NT$/zero `.00`) become cents; non-zero fractions fail. Per-item quantity defaults to 1, range 1..9999, items join with `、`; inconsistent non-empty order-level cells fail the whole unit (`orders_csv.go:202-291`). Per customer ≤2000 stored orders. City allowlist refusal is successful import plus `city_dropped`, never an address/PII echo.

## Errors, replay and UNKNOWN

Normal coded errors use `{code, message, request_id, retryable, details}` (`internal/httperror/error.go:13-18,291-295`). Localized UI copy should use allowlisted codes, not backend English text or uploaded data.

| Status/code | Meaning/current source |
| --- | --- |
| 413 `invalid_request` | >2 MiB, body read error, or zero bytes at HTTP layer (`imports.go:158-162`) |
| 415 `invalid_request` | Media type is not text/csv (`imports.go:149-152`) |
| 422 `encoding_not_utf8`, `too_many_rows`, `required`, `invalid_request` | File refusal: nothing applied; non-empty file with malformed/missing header/required columns enters this class (`customers_csv.go:103-140`; `orders_csv.go:106-145`) |
| 409 fresh Preview DTO | **No `{code:"preview_stale"}` envelope.** Applicable count drifted; transaction rolled back. Parse full preview first, show new counts and require explicit re-confirmation (`imports.go:174-176`) |
| 409 `idempotency_conflict` | Same bytes/kind with different requested mapping or expected count; committed incorrect mapping needs a modified file (contract §4:56-57) |
| 422 `nothing_to_apply` | Re-run yields zero created+updated; no batch is recorded (`customers.go:186-193`; `orders.go:141-149`) |
| 401 `unauthorized`, 403 `forbidden`, 404 `not_found` | Session/permission/scope or unknown/other-store batch/customer. Do not infer authority from client role alone (`batch.go:74-95`; `handler.go:434-445`) |
| 503 `retry_later` / `unavailable` | Deadline/cancellation/backend unavailable; a lost commit response is UNKNOWN to the browser (`imports.go:185-190`; `claims.go:294-306`) |

Customer **row** failure codes: `erased`, `required`, `invalid_external_id`, `name_too_long`, `invalid_name`, `invalid_phone`, `invalid_email`, `duplicate_external_id`, `invalid_request`. Order unit codes: `required`, `invalid_order_id`, `invalid_external_id`, `invalid_date`, `invalid_amount`, `invalid_status`, `invalid_item`, `invalid_quantity`, `inconsistent_order`, `invalid_request`, `customer_not_imported`, `erased`, `order_owner_conflict`, `order_limit` (contract §§3,7 and pure parser/DB loop). Row failures are returned inside successful previews, not whole-file HTTP refusals.

Commit is synchronous and one transaction, with 60-second WithScopeBudget; HTTP body-read deadline 30s and write deadline 75s (`imports.go:154-165`). Backend idempotency is `(store, kind, exact file bytes hash)` plus requested mapping and expected count in `command.Run`; customer key `cimp-<sha[:32]>`, order key `oimp-<sha[:32]>`. Explicitly retry the immutable same Blob/mapping/count after UNKNOWN; never automatically re-upload, re-preview/rebase the command or manufacture a new file/key. There is no GET status recovery API (`customers.go:138-156`; `orders.go:104-122`). Session/store changes must discard sensitive local file/draft data and stale responses.

## Customer readiness and result downloads

- No server store-wide readiness flag. Orders attach only to active, previously imported customer IDs; the SQL checks `external_ids.kind=customers` plus owner scope/active, then tombstone and conflicts (`0156_order_history_import.sql:128-153`). A local same-store/session successful customer commit (created+updated >0, including a truthful replay notice) can be the wizard progression checkpoint required by the brief; it is not proof that every order's customer is ready. Actual order preview remains authority for `customer_not_imported`/`erased`. Do not unlock on a customer preview alone.
- Safe failure download is `GET I/{batch_id}/results.csv?only=failed`, after a successful partial commit. CSV columns exactly `row,external_id,outcome,code`; BOM UTF-8, Excel-safe formula guarding on every text cell. Failed customer IDs are blank; orders batch external IDs are **always blank**. Successful city warnings appear as `code=city_dropped` in the all-results CSV and are omitted from `only=failed` (`batch.go:159-167`; `orders.go:144-147`). No original failed CSV records are returned.
- No server result file exists for an uncommitted preview or an all-failed `nothing_to_apply` refusal. The retained batch is 90-day lazy-pruned; unknown/pruned/other-store batch returns not_found. Never reconstruct a failure download by echoing the local raw row values.
- Backend filename is fixed even for orders. Backend attachment sets `text/csv; charset=utf-8` and `Cache-Control: no-store` (`imports.go:123`; `customers.go:194-200`); the BFF should maintain private/no-store output and trusted filename/closed path. A new BFF URI may mirror these exact paths under `/api/stores/{store}`; no such migration-import BFF existed at the base.

## Existing helpers / reuse boundaries

- `apps/admin/lib/settings-client.ts`: existing CSRF cookie/session-boundary fence and allowlisted `safeError`; auth/store/Origin/CSRF/clear-cookie helpers in `auth.ts` remain server authority. Use established session conceal/unmount behavior; do not invent client tenant/principal fields.
- `customers-client.ts:get/readJSON/useGuardedRead` reuse private GET/session lifecycle for archive reads; `customers-model.ts:object/count/isInstant` and `orders-model.ts:canonicalUUID/canonicalCursor/displayTime`, existing money formatter can validate/display archive facts. `total_minor` is display-only; never add it to finance/revenue or payment actions.
- Existing `tracking-import-client.ts:9-27` is the one-request/explicit-identical-retry/raw Blob transport precedent, with 409-preview-before-coded-error classification and 80s client budget. Its DTOs, 500-row cap, paths and commit counts are DIFFERENT; do not reuse its parsers/caps as migration import models.
- Existing tools BFF has byte-capped `readCapped` (`tools/[...resource]/route.ts:38-61`), native trusted-origin per-mode `upstream` (`:65-84`), 75s import upstream budgets (`:32-35`) and 20s result budget. These are private local functions, a reusable pattern rather than callable exports. Its validated raw `Uint8Array` CSV forwarding preserves bytes. Do not decode/rebuild/stringify the file body.
- Generic `callBackend` → `merchantBackend` overwrites caller timeout with **6 seconds** (`backend.ts:104-108`; `auth.ts:321`), and is unsuitable for the 60s import. Do not silently bypass existing server auth to obtain a longer timeout; keep a bounded import-only native transport after those guards. `auth.readBody` returns decoded text, not the raw bytes needed by the original-file protocol.
- Reuse the existing customer `imported` badge; W6 already displays it. Historical counts are a separate GET `total`; no strict customer-list DTO/schema change is authorized here. No generic import framework/product-import migration is needed.

## Source SHA-256

| Relative source file | SHA-256 |
| --- | --- |
| `internal/httpapi/imports.go` | `c05cd28ad4be644fa658542246dcdaf24d0ada1e3057564ab801c37e4268c6ea` |
| `internal/httpapi/customer_historical.go` | `e7aa32619cb2f59492a4f89880ae26f37248dac193485e9ac7f36e3ff934dd63` |
| `internal/httpapi/customer_tags.go` | `e2cb8b50f483e6ea8f70b5bd42137988fab5d82e72e06969658bd10519c4ce71` |
| `internal/httpapi/merchanttools.go` | `9ddbf280d27845a0072a33adc88ed6aaa554d2952f689228cfd3bd07e9905582` |
| `internal/httpapi/customers.go` | `5f18892ab6028ab94be857e77113ba1ec22e93e69401ded4f447e8b532adffa9` |
| `internal/migrationimport/batch.go` | `a5d3bd073487be75fd9c02cab244574ce4ad2aa99fa845f351277c884b416274` |
| `internal/migrationimport/customers.go` | `10963ee3c7344c26755a8c9f40b3c4cebbf3bf6f93ee5d05d722ae752e47e972` |
| `internal/migrationimport/customers_csv.go` | `a496f5b91eec43b19c9650d7e84e13fc163a7dbf27d659f934a2629888587265` |
| `internal/migrationimport/orders.go` | `7a896e4150917bf4d5ee659b8abbe1709ddff4c1503edc2e698ab9e787720fcb` |
| `internal/migrationimport/orders_csv.go` | `6cd36d8034f05a6d09a9b488736ef77500b8824c5be24bd351339b1f5e6030e4` |
| `internal/customers/historical.go` | `6f8fca0c0ef5fc6bf2274dee0f386c6e41c01782405c7d4820c3e844e928a0e2` |
| `internal/pagination/pagination.go` | `9527fbe5d8211d6afe23f1d78f645b1a939c02c1c10c9b728b0f8d0cc21a0701` |
| `contracts/migration-import-v1.md` | `4fc38eeb70888a055d39b3eec0d19bec25db20446dba50b031033643e5edcdcd` |
| `apps/admin/app/api/stores/[store]/tools/[...resource]/route.ts` | `e4451ac1ba56f6f2c93bdb543f3ef42d4a14af4cfb1c2f4703742a1778272cd3` |
| `apps/admin/lib/tracking-import-client.ts` | `4d919c677da226262e1f8ac236b6385e6e7cbc089bd162fad1a6b1dfdaf8e7e4` |
