**Verdict: APPROVE**

Independent read-only review of commit 572e239ee9c09d611952174f1095443a54e3116f (branch unit/pay-rm2-p2a, PR #70).
Scope: only this commit's diff (3 test files, +19/-3). Go tests were NOT run (forbidden for this reviewer); CI is the runtime evidence. Status: DESIGN/static review only for runtime behaviour, NOT_RUN for go test.

## 1. reports_test.go amount 100 -> 2500 (reports_test.go:131, :605)

- Constraint: `attempt_method_amount_check` (migrations/0061_stripe_psp.sql:76-79) requires `payments.stripe_amount_ok` for `stripe_checkout`; TWD branch (0061:12) is `BETWEEN 2500 AND 99999900 AND p_minor%100=0`. 2500 satisfies both (lowest valid value). 100 violated it after 3afca04a6 switched method_code payuni_credit -> stripe_checkout but kept PAYUNi's 100. Legitimate fixture conversion.
- Not a weakening. Report money reads facts/refunds, never attempt amount:
  - `identity.report_money_events` (0147_reports.sql:138-170, only definition): captured = `f.amount_minor`, refunded = `r.amount_minor` (stripe_refunds). Attempt row only joined for order_id/environment.
  - `identity.read_finance_summary` (latest 0107_home_cod.sql:819+, used by RP01 parity at reports_test.go:352-381): `sum(f.amount_minor)` / `sum(r.amount_minor)`; no attempt amount.
  - Functions that do compare `f.amount_minor=a.amount_minor` (0078:785, 0139:664, 0152:513, 0157:37, 0107:242, 0155, 0162, 0193) also require `f.connection_id=a.connection_id`; the fixture inserts independent `gen_random_uuid()` connection ids on attempt and fact, so those predicates never matched before or after the change. No observable delta.
  - reports_test.go assertions (e.g. :290, :330-332 "35100/26100") are on fact amounts passed as `$8` / literal 30000, unchanged.
- TestBrowserW6Customers seed (browser_w6_customers_reports_test.go:236-241) overwrites the attempt to USD 700 after `capture`, so the TWD 2500 value is transient there; it only needed to pass the CHECK at INSERT time.

## 2. Webhook endpoint memo (payment_fixture_helpers_test.go:44, :53-66; stripe_flow_test.go:62)

- Double-registration is real: merchant_orders_v2_perf_test.go:44 `q.rm2StripePay` then :62-63 `ready := q; ready.rm2StripeStart; ready.rm2StripePay` (same store/connection). browser_merchant_orders_ui_test.go:132 `clone.rm2StripePay` inside the per-mode loop on the same store. parcel_fixture_test.go:76 also pays every placed order on the same `b.q` (covered by the same fix).
- Each `sflEnv.endpoint` call (stripe_flow_test.go:291-300) creates a new endpoint with a fresh UUID (stripeadmin/registrar.go:402-405).
- **Finding F1 (NIT, comment accuracy, non-blocking):** the new comment at payment_fixture_helpers_test.go:53-54 cites `set_stripe_webhook_endpoint` PT409 "already exists". That branch (0137_platform_stripe.sql:951-953) only fires on the same endpoint_id, which can't happen with a fresh UUID. The actual refusal is `UNIQUE(tenant_id,store_id,connection_id,execution_profile)` on `payments.stripe_webhook_endpoints` (0061_stripe_psp.sql:358) -> SQLSTATE 23505, which `sqlError` maps to `ErrRejected` (registrar.go:200-201). Same symptom ("stripeadmin: rejected"), same fix. Root cause (one endpoint per connection+profile) is correct. Suggest rewording the comment in a later commit; no behaviour impact.
- Reusing endpoint+secret is correct and matches production: the schema allows exactly one endpoint per connection/profile, so a real store gets every event on one endpoint. Each delivery still uses a unique event id (`deliver` -> `swhEventID`, stripe_webhook_http_test.go:133-136), so receipt dedup isn't hit. No rm2 caller asserts a fresh endpoint or secret per payment. The cache key is `s.connection`, and connections are per store (`stripeStore` -> `e.seed(psSetupItemsOn(...))`, stripe_flow_test.go:165-168), so stores stay isolated.
- Sharing: `pqFixture.stripe` is `*sflEnv` (payment_query_test.go:45). Copies `ready := q` / `clone` copy the pointer and the same `stripeStore`, so the map on the shared env is hit. The nil-map lazy init is correct.
- Concurrency: no `t.Parallel` in merchant_orders_test.go, meta_ads_capi_test.go, parcel_fixture_test.go, expiry_runtime_test.go, merchant_orders_v2_perf_test.go, browser_merchant_orders_ui_test.go. The map is touched only from the test goroutine. Separate envs on one fixture (expiry_runtime_test.go:533-541) each use their own `stripeStore` and connection, and only one of them pays.

## 3. Other callers

- All `rm2StripePay` callers go through the patched shared function: parcel_fixture_test.go:76, expiry_runtime_test.go:541, browser_merchant_orders_ui_test.go:132, merchant_orders_test.go:558,616, meta_ads_capi_test.go:102, merchant_orders_v2_perf_test.go:44,63.
- Direct `e.endpoint(t, ...)` callers (customers_billing_restricted_test.go:36, customers_billing_erasure_test.go:116, stripe_refund_flow_test.go:44, platform_stripe_test.go:51, stripe_refund_webhook_test.go:103-104) each register once per freshly created store, and none of them also calls `rm2StripePay` on that env. None is missed.

## 4. Static checks

| Command (cwd .worktrees/pay-rm2-p2a) | Exit |
|---|---|
| `git show 572e239ee` | 0 |
| `gofmt -l tests/foundation/{payment_fixture_helpers,reports,stripe_flow}_test.go` (no output) | 0 |
| `go vet ./tests/foundation/` (no output) | 0 |
| `go test` | NOT_RUN (forbidden for this reviewer) |

## Residual risk
- Runtime confirmation of the 3 previously failing tests (TestBrowserW6Customers, TestBrowserMerchantOrdersUIRealChain, TestMerchantOrdersV2TenThousandScopedSearch) depends on CI at 572e239ee. This review is not E3 evidence on its own.
