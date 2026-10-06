# Reporting v2 — product, channel, funnel and manual-order reports (W6-02B)

Status: **DRAFT 2026-10-06 (REAL_PG on MOCK data; integrator freezes)**. Migration `0147_reports.sql`. Code: `internal/reporting/{report,products,channels,funnel,manual}.go`,
`internal/httpapi/reports.go`. Extends [customers-billing-v1](customers-billing-v1.md) §3.1/§5 (BD7 finance summary), whose money rules these reports share.
Authoritative: 架构 §15.5, invariants I01 (scope), I05 (money: integer minor units, per currency and environment, no conversion).
Non-goals: profit/COGS, customer-service report, ad ROAS (ads has its own), cross-currency totals, caches / summary tables / materialized views,
scheduled mail reports, historical imported orders (W5-03B keeps them in their own archive tables; no report here reads them).

## 1. Common rules
- Read-only: no table, column or role is added; each answer is computed per request inside one `commerce_runtime` READ COMMITTED transaction (60 s budget).
- Range: `from`, `to` are `YYYY-MM-DD` Asia/Taipei days, `0 <= to - from <= 91` (92 days). Otherwise 422 `invalid_request`. A store that is not the caller's is 404.
- Money comes from `payments.facts` (CAPTURED), `payments.refund_facts` (SUCCEEDED, not later FAILED/CANCELED) and the frozen `checkout.orders.snapshot`
  (`quote.lines[].amount.total_minor`) — the exact rows `identity.read_finance_summary` (0107) sums. A fact belongs to the day of its `received_at`.
  Offline money (COD and pay-at-pickup `COLLECTED` on `collected_at`, bank transfer `CONFIRMED` on `confirmed_at`) is a separate `offline_*` figure,
  never part of `captured` or `net`. Every figure is per `(currency, environment)`; SANDBOX and LIVE never add up.
- Parity (RP01): for the same range the sum of `captured_minor`, `refunded_minor`, `net_minor` over a report's rows equals the finance summary totals per
  environment, and the sum of `offline_minor` equals pickup + bank-transfer + COD columns.
- Orders = non-`CANCELLED` orders created in the range (counted by `created_at`); `cancelled_orders` counts the cancelled ones. Money is dated by the fact, so a
  report may hold the money of an order created before the range.
- Permissions: `orders:read` for the four reads (the funnel also `live:read`); `.csv` needs `orders:export` AND `orders:read` (funnel: and `live:read`) and writes exactly one
  `ops.audit_events` row named after the report (`reports.export.products|channels|funnel|manual_orders`) in the same transaction. A refused export writes nothing. Responses are `no-store, private`; the CSV is generated per request.
- Deployment environment (LC-B7 A1): the order counts — funnel `paid`, and `orders` / `cancelled_orders` of the channel and manual-order reports — follow the deployment payment environment
  (`httpapi.Options.PaymentEnvironment`: LIVE on a LIVE deployment, otherwise SANDBOX; the Go caller passes it, a request never does). An order is in the environment of its payment attempt, or in the
  deployment's when it has none (offline modes, unpaid). Money is never filtered: it stays split per `(currency, environment)` and is identical under both profiles.
- Scope comes from the resolved access and the transaction GUCs only; a request never names a tenant.

## 2. `GET /v1/admin/stores/{store_id}/reports/products?from&to` — `identity.read_report_products`
Rows per `(sku_id, currency, environment)`: `product_id, code, name` (from the newest frozen snapshot line), `units`, `captured_minor`, `refunded_minor`,
`net_minor = captured - refunded`, `offline_units`, `offline_minor`. `units` = quantity of the lines of orders with a CAPTURED fact in the range (card);
`offline_units` likewise for offline collections.
Allocation: a payment fact (captured, refund, offline) of an order is spread over that order's snapshot lines in proportion to the frozen line total
(all-free order: to the quantities); the parts are floored and the remainder goes one minor unit at a time to the largest fractions, so the parts add up to the fact exactly.
Shipping, the COD surcharge, order-level discounts and partial refunds are not line items: they ride on the order's facts and are therefore spread proportionally over the SKU rows like any other
fact, so the SKU rows add up to the finance totals exactly (a SKU's `captured_minor` is its share of the facts, not its list price times quantity). Current prices are never read. Sorted by `net_minor` descending then `sku_id`; at most 1000 rows, `truncated: true` when more existed.

## 3. `GET .../reports/channels?from&to` — `identity.read_report_channels`
Rows per `(channel, currency)` in the order `facebook_live, instagram_live, storefront, manual`: `orders`, `cancelled_orders`, and `money[]` per environment
(`captured_count, captured_minor, refunded_minor, net_minor, offline_count, offline_minor`).
Channel (one order, exactly one channel): any origin bundle (`claims.order_origins` UNION `claims.live_price_uses`) with `platform=facebook` -> `facebook_live`; else
`instagram` -> `instagram_live`; else `checkout.orders.source = merchant_manual` -> `manual`; else `storefront`. A merchant-created order of a facebook bundle is therefore
`facebook_live` here and still appears in the manual-order report (§5).

## 4. `GET .../reports/funnel?from&to[&session_id]` — `identity.read_report_funnel`
Counts follow the deployment environment (§1): a CAPTURED fact of the other environment does not make a bundle `paid`; offline collections count in both. Cohort: bundles (de-duplicated) with an ACCEPTED `claims.events` row whose `occurred_at` is in the range (optional session of the store; a foreign session is 404).
Later stages are not date-limited and nested: `claimed >= link_sent >= ordered >= paid`.
`link_sent` = claimed bundles with a SUCCEEDED `meta.private_reply` operation naming the bundle (a `meta.dm_send` link is not counted: its operations are not readable by the claims
owner role; widening that is a later decision). `ordered` = `link_sent` bundles with a non-CANCELLED order (origin or live-price use). `paid` = `ordered` bundles whose order has a CAPTURED fact,
a COLLECTED COD / pay-at-pickup, or a CONFIRMED bank transfer (refunds do not unpay). `ordered_without_link` = ordered bundles whose link was not sent by the system. No actor key or buyer data is returned.

## 5. `GET .../reports/manual-orders?from&to` — `identity.read_report_manual_orders`
Merchant-created orders (`source = merchant_manual`) created in the range, per `(principal_id, session_id, currency)` with `orders`, `cancelled_orders` and `money[]` as in §3.
`principal_id` = the `ops.command_results` receipt `merchanttools.order.manual` of the order (null when missing); `session_id` = the lowest session of its origin bundles (null: none).
The creator and counts are not the channel report's `manual` row: that row holds orders with no facebook/instagram origin, this report lists every merchant-created order including those of a live bundle.
Orders are counted by creation date in the range (and the deployment environment); money is the facts dated in the range, so an order created in the range and paid after it shows orders 1 and money 0 until the later range.
Ids only — the admin UI resolves them to a staff name and a session title.

## 6. CSV (`.csv` suffix; same query; `Content-Disposition: attachment; filename="report-<name>-<from>-<to>.csv"`)
One CSV row per money row (channels, manual orders: per environment; a bucket without money gives one all-zero row); products one row per SKU row; funnel one row.
Merchant text (`code`, `name`) is formula-guarded (a leading `= + - @`, TAB, CR or LF gets an apostrophe).

## 7. ACL / SQL (0147)
Definers owned by `commerce_auth`, EXECUTE `commerce_runtime`: `read_report_products|channels|funnel|manual_orders` (all but products take `p_environment`), `export_report(report text, …, p_environment)` (dispatcher: re-runs the read definer, then audits).
Helpers owned by `commerce_auth`, invoker rights (not SECURITY DEFINER), no EXECUTE for anyone else (pinned by `TestReportInternalHelpersAreNotCallable`): `identity.report_open`, `identity.report_money_events`. Domain definers, EXECUTE `commerce_auth` only:
`claims.report_order_bundles`, `claims.report_funnel_bundles` (owner `commerce_claims_writer`), `checkout.report_manual_creators` (owner `commerce_checkout_writer`).
No new table or column grant. Two indexes: `claims.order_origins(tenant_id,store_id,bundle_id)` and `claims.events(tenant_id,store_id,occurred_at)`; the funnel reads operations only for `created_at` in `[from, to + 7 days)`.
One policy `auth_report_export_audit` (INSERT of the four `reports.export.<report>` actions, GUC-scoped).

## 8. Gates
`bash scripts/dev/test-focused.sh '^TestReport'` (RP01 numbers + finance parity + offline, RP04 funnel, RP05 range, RP06 export + audit, RP07 cross-store, RP08 timing at 10,000 orders in each of two tenants; environment profiles LIVE and SANDBOX);
DB-free: `go test ./internal/reporting ./internal/httpapi -run 'Report'`. RP03 (historical imports never appear) is structural: no report reads a table other than those named above.
