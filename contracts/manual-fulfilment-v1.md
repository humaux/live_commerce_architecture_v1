# Manual fulfilment v1 — merchant-arranged shipment, tracking and unshipped export

Status: **DRAFT**, 2026-09-28 (unit `design-after-payment`). Not frozen; not reviewed.
Evidence label for this file: DESIGN. Every MF gate below is NOT_RUN.

Implements the manual path of architecture §13.4 ("首发provider外保留手工发货/单号导入与面单重打，但手工动作同样留审计")
for R1. Builds on [merchant-orders-v1](merchant-orders-v1.md) (read authority, projection,
`internal/merchantorders`), [merchant-orders-ui-v1](merchant-orders-ui-v1.md) (inline detail row),
[buyer-order-history-v1](buyer-order-history-v1.md) (owned order reads),
[payment-capture-v1](payment-capture-v1.md) (`payment_work_items`, `PAID_ALLOCATION_FAILED`) and the
service-settings rule that merchant-arranged fulfilment never creates a provider operation
([merchant-service-settings-v1](merchant-service-settings-v1.md)). Architecture §12.1, §13.2–§13.4 and
invariants I01, I02, I04, I11, I13, I14, I24 are authoritative; where they conflict, this file is wrong.

## 0. Owner inputs, decisions and rulings needed

Owner inputs: R1 includes "merchant records carrier + tracking, buyer sees it" (PROCESS §1 R1-4).
Carrier contracts and label APIs are owner-only and out of R1. No owner input exists on export
columns, permission names or tracking-URL policy.

| # | Decision | Why / risk closed |
| --- | --- | --- |
| MD1 | **One merchant-arranged shipment per order** in v1: the whole order ships as one parcel record. Package split, partial shipment and merged packages (§13.2 `FulfillmentOrder → Package → PackageItem`) are R2. | Smallest correct model; the frozen order lines are not re-partitioned. |
| MD2 | **Execution is `MERCHANT_ARRANGED` only.** Recording a shipment creates no `integration.operations` row, no River job, no provider call, no label, no tracking poll. Carrier fields are merchant-supplied labels, not an adapter connection or capability claim. | Service-settings decision: merchant-arranged must not create provider operations or pretend pickup. |
| MD3 | **New fulfilment state `MERCHANT_SHIPPED`** on `checkout.orders.fulfillment_state`, meaning only "the merchant attests the parcel was dispatched with this carrier/tracking". It never means `HANDED_OVER`, `IN_TRANSIT` or `DELIVERED` (§13.4: a tracking number is not in-transit evidence; I13). | Keeps states honest; carrier tracking events are an R2 adapter concern. |
| MD4 | **Append-only versions + head CAS.** Every record, correction or void is a new immutable version row; the head points to the current version and is updated with `expected_version` CAS (the `service_heads` pattern). Nothing is overwritten. | §13.4 audit requirement; I14. |
| MD5 | **Void returns the order to `MANUAL_UNASSIGNED`** (mistaken shipment). A voided record stays in history. | Reversible merchant error without deleting evidence. |
| MD6 | **Eligibility to ship** (checked under the order lock): `commercial_state='CONFIRMED'`, `fulfillment_state='MANUAL_UNASSIGNED'`, a `payment_work_items` row with `state='READY'` for the order (implies a matched CAPTURED fact and no review), and, once `stripe-refund-v1` exists, refund `held < captured` (a fully refunded or full-refund-in-flight order cannot be shipped; a partial refund can). `PAID_ALLOCATION_FAILED`, `CANCELLED`, `REVIEW_REQUIRED` work, DRAFT and AWAITING_PAYMENT are refused. | Never ship unpaid, unallocated or refunded goods (I24, capture-v1 "not a fake carrier job"). |
| MD7 | **Corrections** (carrier/tracking typo) are allowed only while the head is `SHIPPED`; void is allowed only while the head is `SHIPPED`. Refunds never change fulfilment state (stripe-refund RD6). | Clear single transition graph. |
| MD8 | **Separate permissions:** `fulfillment:write` to record/correct/void; `orders:export` to download recipient PII as CSV. Neither is implied by `orders:read`; neither is backfilled. | Export is bulk PII (I11); write authority differs from read (merchant-orders-v1). |
| MD9 | **CSV export is generated on request, never stored,** capped, audited, formula-injection-safe, `no-store`. | "No automatic exports" (merchant-orders-v1) stays true: this is an explicit, permissioned, audited action. |
| MD10 | **No buyer message** is sent on shipment in v1; the buyer sees the state on the order page. | Messaging needs consent/window rules (§10) and a separate contract. |

Rejected alternatives:
- A mutable `tracking_number` column on `checkout.orders`: overwrites history, fails §13.4 audit.
- Creating an `integration.operations` row "for uniformity": would claim a provider action that never
  happens and could be picked up by dispatch paths.
- Mapping `MERCHANT_SHIPPED` to `IN_TRANSIT`/`DELIVERED`: false evidence (I13).
- Letting the buyer page fetch carrier tracking pages server-side: SSRF surface and unverified formats.
- Building carrier-specific CSV formats (7-ELEVEN/FamilyMart bulk upload) now: their formats are not
  verified in this repo; a generic column set ships first (ruling M-6).

### 0.1 Integrator ruling needed

- **M-1** Permission names `fulfillment:write`, `orders:export`; whether onboarding grants them to new
  store creators (draft: no; explicit provisioning).
- **M-2** Tracking URL policy. Draft: no built-in per-carrier URL templates until each template URL is
  verified with source + retrieval date (none verified in this unit); a merchant may supply an
  explicit `tracking_url` (https only, rules in §3.2) shown to the buyer with its host visible. Alternatives:
  host allowlist per known carrier, or no merchant URL at all.
- **M-3** Carrier code list and display names (draft §3.1). Confirm SF Express and Chunghwa Post
  naming for zh-Hant/zh-Hans/en, and whether Hi-Life/OK Mart CVS are needed for R1.
- **M-4** `shipped_on` merchant-attested date (draft: not collected; server `recorded_at` only).
- **M-5** Export row cap 1000 and "oldest paid first" order; paging beyond the cap (draft: none, the
  response flags truncation).
- **M-6** Export columns (draft §5.3); carrier-specific import formats deferred.
- **M-7** Tracking-number bulk import (§13.4 "单号导入"): draft defers to R2.
- **M-8** Whether a `MERCHANT_SHIPPED` order may still be refunded fully without any return flow
  (draft: yes; refund and return are independent, §12.3/I13).
- **M-9** Dependency on `stripe-refund-v1`: draft assumes 0062 lands before 0063 so the MD6 refund
  check can reference `payments.stripe_refunds`; if refunds slip, the check is added by 0062 instead.

## 1. Existing facts relied on (repository, read 2026-09-28 UTC)

| # | Fact | Source |
| --- | --- | --- |
| E1 | `checkout.orders.fulfillment_state ∈ {MANUAL_UNASSIGNED, CANCELLED, PAID_ALLOCATION_FAILED}`; `commerce_checkout_writer` holds `UPDATE(commercial_state,fulfillment_state,…)`. | `migrations/0013_buyer_checkout.sql:23,175`, `0018_payment_capture.sql:199-201` |
| E2 | `fulfillment.payment_work_items` is one immutable row per order, `state ∈ {READY, REVIEW_REQUIRED}`, FK to the CAPTURED fact; "not a carrier request or proof of shipment". | `0018_payment_capture.sql:102-110`, payment-capture-v1 |
| E3 | Merchant reads go through `identity.read_merchant_orders` (owner `commerce_auth`, `orders:read`, single snapshot, fresh final SQL auth); Go `merchantorders.List/Get`; detail carries frozen items, totals and destination incl. pickup code strings with leading zeroes. | `0027_merchant_orders.sql`, `internal/merchantorders/orders.go:101,150` |
| E4 | Buyer order reads use `commerce_checkout_runtime` under buyer RLS (`internal/checkout/orders.go` `ListOrders`/order GET). | `0013_buyer_checkout.sql:173`, `internal/checkout/orders.go:78-99` |
| E5 | Merchant writes use `ops.command_results(tenant,store,operation,idempotency_key)` and `ops.audit_events(action)`; `expected_version=0` creates, later writes require the current version. | `0002_catalog_inventory.sql:11`, `0001_foundation.sql:54`, merchant-settings-http-v1 |
| E6 | Latest permission vocabulary is 0033's list (adds `live:read`, `live:manage` without onboarding grant). | `0033_live_planning.sql:3-8` |

## 2. State machine

`checkout.orders.fulfillment_state` (commercial state untouched by this contract):

| From | Event | To | Guard |
| --- | --- | --- | --- |
| MANUAL_UNASSIGNED | record shipment (v1 SHIPPED) | MERCHANT_SHIPPED | MD6 eligibility |
| MERCHANT_SHIPPED | correct (vN SHIPPED) | MERCHANT_SHIPPED | head SHIPPED, `expected_version` = head |
| MERCHANT_SHIPPED | void (vN VOIDED) | MANUAL_UNASSIGNED | head SHIPPED, reason required |
| MANUAL_UNASSIGNED after a void | record again (vN+1 SHIPPED) | MERCHANT_SHIPPED | MD6 again |
| CANCELLED, PAID_ALLOCATION_FAILED, any non-CONFIRMED | any | unchanged | PT422 `not_shippable` |

Shipment version status is `SHIPPED` or `VOIDED`. The head's current version decides the state; the
order column is written in the same transaction and must always agree (a deferred constraint trigger
checks `head.status='SHIPPED' ⇔ fulfillment_state='MERCHANT_SHIPPED'`).

## 3. Data rules

### 3.1 Carrier

`carrier_code` ∈ {`seven_eleven_cvs` (7-ELEVEN 交貨便/取貨), `familymart_cvs` (全家 店到店),
`sf_express` (順豐速運), `chunghwa_post` (中華郵政), `other`}. `carrier_name` is free text, NFC,
1..80 characters, no control characters: **required** when `other`, optional display override
otherwise. A carrier code is a label, not an integration binding, capability or contract claim (MD2).
The buyer pickup destination (frozen at checkout) is not revalidated or rewritten by the carrier choice.

### 3.2 Tracking

- `tracking_number`: required for `SHIPPED`, trimmed, 1..64 characters, `^[A-Za-z0-9][A-Za-z0-9 -]{0,63}$`
  with no trailing space; stored exactly (no case folding; leading zeroes kept).
- `tracking_url` (optional, M-2): absolute `https://` URL ≤512 bytes, host with at least one dot, no
  userinfo, no explicit port, no fragment, no control/whitespace characters, parsed and re-serialized
  canonically by Go; SQL re-checks prefix and length. The buyer UI renders it as a link with
  `rel="noopener noreferrer nofollow"`, `target="_blank"`, showing the host text next to it. Never
  fetched by the server.
- Built-in URL templates: a Go table `carrierTrackingTemplates` keyed by carrier code, **empty in v1**;
  an entry may be added only with its docs URL and retrieval date (PROCESS §5), and the `{tracking}`
  placeholder is path-escaped. When both exist, the explicit `tracking_url` wins.
- `note` (optional, merchant-only, never shown to the buyer): 0..200 characters. `void_reason`
  ∈ {`wrong_order`, `wrong_tracking`, `not_dispatched`, `other`} required for VOIDED.

## 4. Persistence: `migrations/0063_manual_fulfilment.sql`

```sql
-- permission vocabulary: re-derive 0033 list + 'fulfillment:write','orders:export' (no backfill, no onboarding grant)
ALTER TABLE checkout.orders DROP CONSTRAINT orders_fulfillment_state_check;
ALTER TABLE checkout.orders ADD CONSTRAINT orders_fulfillment_state_check
 CHECK(fulfillment_state IN ('MANUAL_UNASSIGNED','CANCELLED','PAID_ALLOCATION_FAILED','MERCHANT_SHIPPED'));

CREATE TABLE fulfillment.manual_shipment_versions (
 tenant_id uuid NOT NULL, store_id uuid NOT NULL, owner_id uuid NOT NULL, order_id uuid NOT NULL,
 version bigint NOT NULL CHECK(version>0),
 status text NOT NULL CHECK(status IN ('SHIPPED','VOIDED')),
 carrier_code text NOT NULL CHECK(carrier_code IN ('seven_eleven_cvs','familymart_cvs','sf_express','chunghwa_post','other')),
 carrier_name text CHECK(carrier_name IS NULL OR (length(carrier_name) BETWEEN 1 AND 80 AND carrier_name !~ '[[:cntrl:]]')),
 tracking_number text NOT NULL CHECK(tracking_number ~ '^[A-Za-z0-9][A-Za-z0-9 -]{0,63}$' AND tracking_number !~ ' $'),
 tracking_url text CHECK(tracking_url IS NULL OR (octet_length(tracking_url)<=512 AND tracking_url ~ '^https://[!-~]+$')),
 note text CHECK(note IS NULL OR (length(note)<=200 AND note !~ '[[:cntrl:]]')),
 void_reason text CHECK(void_reason IN ('wrong_order','wrong_tracking','not_dispatched','other')),
 principal_id uuid NOT NULL, recorded_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,store_id,order_id,version),
 CHECK((status='VOIDED')=(void_reason IS NOT NULL)),
 CHECK(carrier_code<>'other' OR carrier_name IS NOT NULL),
 FOREIGN KEY(tenant_id,store_id,owner_id,order_id) REFERENCES checkout.orders(tenant_id,store_id,owner_id,id),
 FOREIGN KEY(tenant_id,principal_id) REFERENCES identity.memberships(tenant_id,principal_id)
);
CREATE TABLE fulfillment.manual_shipment_heads (
 tenant_id uuid NOT NULL, store_id uuid NOT NULL, owner_id uuid NOT NULL, order_id uuid NOT NULL,
 current_version bigint NOT NULL CHECK(current_version>0),
 updated_at timestamptz NOT NULL,
 PRIMARY KEY(tenant_id,store_id,order_id),
 FOREIGN KEY(tenant_id,store_id,order_id,current_version)
  REFERENCES fulfillment.manual_shipment_versions(tenant_id,store_id,order_id,version) DEFERRABLE INITIALLY DEFERRED
);
```

- A VOIDED version copies the carrier/tracking of the version it voids (so history reads without joins).
- Versions are append-only (no UPDATE/DELETE grant; trigger refuses both). Heads: only
  `UPDATE(current_version,updated_at)`, monotone +1 (trigger).
- Index `(tenant_id,store_id,order_id,version DESC)` is the PK; export uses the existing
  `checkout.orders(tenant_id,store_id,created_at DESC,id DESC)` index plus a partial index
  `ON checkout.orders(tenant_id,store_id,created_at,id) WHERE commercial_state='CONFIRMED' AND
  fulfillment_state='MANUAL_UNASSIGNED'`.
- **Roles/RLS:** both tables FORCE RLS, PUBLIC revoked. Owner of the write definer is
  `commerce_checkout_writer` (already the order-state writer): SELECT/INSERT on versions, SELECT/INSERT/
  `UPDATE(current_version,updated_at)` on heads, with explicit policies. `commerce_auth` (merchant read
  definer owner) gets column SELECT + SELECT policy. `commerce_checkout_runtime` (buyer) gets SELECT on
  `(tenant_id,store_id,owner_id,order_id,version,status,carrier_code,carrier_name,tracking_number,
  tracking_url,recorded_at)` only — never `note`, `void_reason` or `principal_id` — under the existing
  buyer owner-scope RLS pattern. `commerce_runtime` gets EXECUTE on the definers only; no table access.

### 4.1 SQL entry points (all SECURITY DEFINER, `search_path=pg_catalog`, PUBLIC revoked)

| Function | Owner | EXECUTE | Contract |
| --- | --- | --- | --- |
| `fulfillment.record_manual_shipment(hash bytea, store uuid, order uuid, key text, request_hash bytea, expected_version bigint, status text, carrier_code text, carrier_name text, tracking_number text, tracking_url text, note text, void_reason text) RETURNS jsonb` | checkout_writer | `commerce_runtime` | Merchant token auth (`fulfillment:write`) with the 0027 fresh-final-auth pattern after all locks (READ COMMITTED only; PT401/403/404 distinctions; missing and other-store orders indistinguishable). Replays `ops.command_results` (`operation='fulfillment.manual_shipment.record'`; same key + different hash → PT409). Lock order: order `FOR UPDATE` → work item → refund rows (if present) → head `FOR UPDATE`. `expected_version` must equal the head (0 = no head) else PT409 `version_changed`. Applies §2/MD6/MD7 guards; inserts version, upserts head, updates `fulfillment_state` and `updated_at`, inserts `ops.audit_events` action `fulfillment.shipment_recorded` / `fulfillment.shipment_corrected` / `fulfillment.shipment_voided`, and the command result. Returns the version DTO. No job, no operation, no provider call. |
| `identity.read_merchant_orders(...)` (replaced) | commerce_auth | runtime | Same signature and rules as 0027 (or its latest successor, incl. 0062 refund fields); `state` filter adds `shipped` meaning `fulfillment_state='MERCHANT_SHIPPED'`, and adds `unshipped` = CONFIRMED + MANUAL_UNASSIGNED + READY work; summary `fulfillment_state` admits `MERCHANT_SHIPPED`; detail adds `shipment: null | {version,status,carrier_code,carrier_name,tracking_number,tracking_url,recorded_at}` from the current head in the same snapshot. |
| `fulfillment.read_manual_shipment_history(hash, store, order) RETURNS jsonb` | commerce_auth | runtime | `orders:read`; all versions ascending including `note`, `void_reason`, `principal_id` (principal as UUID only). |
| `identity.export_unshipped_orders(hash bytea, store uuid, row_limit integer) RETURNS jsonb` | commerce_auth | runtime | `orders:export` **and** `orders:read`; row_limit 1..1001 (Go asks 1001 to detect truncation); eligible orders per MD6 ordered `created_at ASC, id ASC`; one snapshot; fresh final auth before returning; inserts `ops.audit_events` action `orders.export_unshipped` (commerce_auth receives INSERT on `ops.audit_events` only for this, via an explicit policy). Returns frozen destination and items per row. |

## 5. HTTP and UI

### 5.1 Merchant admin routes (Go private API; admin BFF mirrors under `/api/admin/`)

| Method/path | Input | Success |
| --- | --- | --- |
| PUT `/v1/admin/stores/{store_id}/orders/{order_id}/shipment` | `Idempotency-Key` required; body exactly `{expected_version, status, carrier_code, carrier_name, tracking_number, tracking_url, note, void_reason}` (nullable fields explicit `null`); no query | 200 version DTO after COMMIT. 409 key conflict / `version_changed`; 422 `not_shippable`, `invalid_carrier`, `invalid_tracking`, `invalid_url`, `void_requires_shipped`; 403 missing `fulfillment:write`; 404 missing/other-store. |
| GET `/v1/admin/stores/{store_id}/orders/{order_id}/shipment/history` | none | `{items:[version…]}` (merchant-only fields included) |
| GET `/v1/admin/stores/{store_id}/orders/unshipped.csv` | no query | `200 text/csv; charset=utf-8`, `Content-Disposition: attachment; filename="unshipped-<store8>-<UTC yyyymmddHHMM>.csv"`, `Cache-Control: no-store, private`, header `X-Export-Truncated: true|false`. |

Existing merchant HTTP rules apply (64 KiB JSON, unknown/duplicate fields rejected, HEAD rejected on
writes, fixed error envelope, tokens from cookie transport only). The PUT is the single command route;
correction and void are distinct `status`/version transitions of it, not separate endpoints.

### 5.2 Buyer surface

The existing owned order detail `GET /v1/buyer/orders/{id}` adds
`shipment: null | {status:"SHIPPED", carrier_code, carrier_name, tracking_number, tracking_url, recorded_at}`
— only when the head is SHIPPED (a voided head returns `null`). History list `fulfillment_state`
admits `MERCHANT_SHIPPED`. The storefront order page shows "Shipped by the seller" (三語), carrier
display name, tracking number with a copy button, and the link per §3.2; it never says "in transit"
or "delivered". No other buyer fields or routes change; recipient PII rules unchanged.

### 5.3 CSV export

- UTF-8 with BOM (for spreadsheet tools used in Taiwan), CRLF line ends, RFC 4180 quoting.
- Columns (M-6): `order_id, created_at_utc, service_code, destination_kind, recipient_name, phone,
  country, region, city, postal_code, line1, line2, pickup_namespace, pickup_code, pickup_name,
  pickup_address, items, total_minor, currency`. `items` = `code×quantity` joined with `; `.
  Pickup codes are text (leading zeroes kept).
- Formula-injection guard: any cell starting with `=`, `+`, `-`, `@`, tab or CR is prefixed with `'`.
- At most 1000 rows; more sets `X-Export-Truncated: true` (M-5). The file is streamed from memory,
  never written to disk, cache, logs, object storage or browser storage; the BFF passes it through as
  an attachment. Logs record only store UUID, row count and duration.

### 5.4 Admin UI (`merchant-orders-ui` amendment, ui_worker)

Inside the approved C inline detail row: a "Shipment" section. With `fulfillment:write` and an
eligible order it shows a form (carrier select with the §3.1 names, carrier name when `other`,
tracking number, optional tracking URL, optional note) and a "Mark shipped" action; with a SHIPPED head
it shows the record plus "Correct" and "Void" (void asks for a reason); history is a disclosure list.
Orders list gains filters `unshipped` and `shipped`, and an "Export unshipped (CSV)" button only with
`orders:export`. One idempotency key per dialog submission; no optimistic state; after success the
row re-reads the detail. Existing MOU rules (locale routes, keyboard access, empty/loading/error
states) apply.

## 6. Test gates (tiers: UNIT, REAL_PG, HTTP_PG, BROWSER)

| Gate | Test | Tier | Required |
| --- | --- | --- | --- |
| MF01 | `TestManualFulfilmentMF01Validation` | UNIT | Carrier/name/tracking/URL/note/void rules incl. unicode, control chars, trailing space, leading zeroes, `https` only, userinfo/port/fragment rejected, canonical re-serialization; empty template table; CSV encoder: BOM, quoting, CRLF, every injection prefix, 1000/1001 truncation flag. |
| MF02 | `TestManualFulfilmentMF02Schema` | REAL_PG | Fresh + populated upgrade (after 0062); CHECK negatives; append-only versions (UPDATE/DELETE refused for every role); head monotone; deferred head⇔order-state agreement trigger; FORCE RLS, column grants (buyer cannot read note/void_reason/principal), definer owner/`proconfig`/ACL, PUBLIC revoked; permissions added without backfill/onboarding grant; merchant read function keeps 0027 auth behaviour. |
| MF03 | `TestManualFulfilmentMF03Transitions` | REAL_PG | Real CAPTURED/READY order (via existing capture path, no fabricated facts): record → MERCHANT_SHIPPED; correct → v2; void → MANUAL_UNASSIGNED; re-record → v4; every refused source state (DRAFT, AWAITING_PAYMENT, CANCELLED, PAID_ALLOCATION_FAILED, REVIEW_REQUIRED work, full refund held/succeeded) → 422 with zero rows; partial refund still ships; replay same key; key+different body 409; stale `expected_version` 409; two concurrent records → exactly one version 1; zero changes to ledger, balances, reservations, facts, work items, operations, River jobs; exactly one audit row per accepted command. |
| MF04 | `TestManualFulfilmentMF04Authority` | REAL_PG | Two tenants, two stores, two buyers: other-store order indistinguishable 404; `orders:read` alone cannot write or export; `orders:export` alone cannot export (needs read) or write; revoked session/grant during a blocked lock (pg_blocking_pids witness) is rejected by the fresh final auth with no row written. |
| MF05 | `TestManualFulfilmentMF05HTTP` | HTTP_PG | Exact PUT/GET/CSV keys, codes and headers (`no-store`, attachment, truncation flag); strict body/query/method rules; merchant list filters `shipped`/`unshipped`; buyer detail `shipment` only for SHIPPED head, no merchant-only fields; buyer history state; no PII in logs (log-capture sentinel). |
| MF06 | `TestManualFulfilmentMF06Export` | REAL_PG + HTTP_PG | Eligibility exactly MD6; frozen destination/pickup values after catalog/address edits; oldest-first; 1000 cap; audit row per export; nothing persisted (DB-wide scan finds no CSV bytes). |
| MF07 | `manual-fulfilment.spec.ts` | BROWSER | Merchant marks a paid order shipped (7-ELEVEN CVS, leading-zero tracking), corrects, voids, re-ships; buyer order page shows carrier + tracking + link host and never "delivered"; export downloads a CSV that opens with the expected header; merchant without `fulfillment:write` sees no action; 3 locales, desktop + mobile Chromium; screenshots hashed. |
| MF08 | `TestManualFulfilmentMF08Guards` + root | REVIEW + regression | PROCESS §5 comments/`COMMENT ON`; no provider dial, job or operation from `fulfillment` code (source guard); full `go test -race ./...`, `go vet ./...`, `python3 scripts/check_packet.py`; MOR/MOU/BPH/CF and RF gates unchanged; independent test_worker + security_reviewer verdicts. |

Each gate records one red run before its green run (PROCESS §2.4).

## 7. Ownership and sequence

| Artifact | Owner |
| --- | --- |
| `internal/merchantorders` shipment/export additions, `internal/checkout` buyer projection | commerce_worker |
| `0063_manual_fulfilment.sql`, admin HTTP mount, `core-openapi.json`, `tasks.json` | integrator |
| `tests/foundation/manual_fulfilment_*_test.go`, `manual-fulfilment.spec.ts` | independent test_worker |
| admin shipment section, export button, storefront shipment display, BFF routes | ui_worker |

Sequence: freeze this file → 0063 + MF02 (integrator; after 0062) → commerce_worker MF01/MF03–MF06
→ UI + MF07 → MF08 verdicts. This unit may run in parallel with refund implementation only after both
contracts' SQL interfaces are frozen (the MD6 refund check reads `payments.stripe_refunds`).

## 8. Known limits / NOT_RUN

- Evidence: DESIGN only; MF01–MF08 NOT_RUN.
- One parcel per order; no split/merge/partial shipment, no package items (§13.2) — R2.
- No carrier API, label purchase, pickup booking, tracking events, delivery confirmation, RMA or
  returns (§13.3/§13.4) — T13/R2. `MERCHANT_SHIPPED` is merchant attestation only.
- No built-in carrier tracking URLs until verified (M-2); no tracking import (M-7).
- No buyer notification on shipment (MD10).
- Export capped at 1000 rows without paging (M-5); generic columns, not carrier upload formats (M-6).
- Cash-on-delivery is not modelled; a CVS carrier label here never implies COD collection.

## Integrator rulings (2026-09-29, binding; supersede the defaults in §0.1)

- M-1 Names accepted; creator grant via `0065_owner_provisioning.sql` (see stripe-refund-v1 rulings).
- M-2 Merchant-supplied https tracking URL; no built-in templates until verified.
- M-3 **Changed:** carrier codes add `hilife_cvs` (萊爾富) and `okmart_cvs` (OK mart) — the launch
  market is Taiwan and buyers pick among the four CVS chains.
- M-4 Server time only. M-5 1000-row cap with a truncation flag. M-6 Generic CSV columns.
- M-7 Bulk tracking-number CSV import is deferred **but is the first R1 follow-up** (live sessions
  produce many orders at once); design it as `manual-fulfilment-import-v1` after MF gates pass.
- M-8 Accepted. M-9 Migration 0062 (refund) before 0063 (fulfilment).
