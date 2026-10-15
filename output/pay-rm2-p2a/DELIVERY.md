# PAY-RM2 phase 2a delivery

Status: **BLOCKED / partial candidate, E1 only. Not merge-ready.** Production removal and new inverse gates are implemented; legacy-success test migration remains incomplete. Full Gates and independent review NOT_RUN (integrator-owned).
Role: commerce_worker; executor: Codex (codex exec), requested reasoning high; actual model identifier not exposed by this runtime.
Base: e4dad98cc322d35709b475de25063efc92e5d866 (r3/integration at assignment).
Worktree: /Volumes/data/live_commerce_architecture_v1/.worktrees/pay-rm2-p2a
Branch: unit/pay-rm2-p2a. No push / PR / merge authorized for this worker.
Scope: remove Go payment execution; migrations, apps, shipping and go.mod/go.sum untouched.

## Initial classification (written before implementation)

Command: `git grep -n -i payuni -- cmd internal tests` (exit 0).
Every initial matching line is covered by the line-number groups below; exact lines are retained in `initial-payuni-grep.txt`.

| File | Initial lines | Classification / disposition |
|---|---|---|
| `cmd/api/buyer_payment.go` | 2, 60, 106 | remove execution/config/form branches; preserve Stripe behavior |
| `cmd/api/buyer_payment_test.go` | 111, 115, 126, 130 | convert positive execution tests to inverse assertions; historical SQL-only fixtures/read assertions retained where applicable |
| `cmd/api/payuni_removed_test.go` | 1, 6, 19, 20, 21, 38, 45, 46 | convert/extend removal assertions; preserve existing RM01–RM03 assertions |
| `cmd/api/stripe_live_test.go` | 127 | convert positive execution tests to inverse assertions; historical SQL-only fixtures/read assertions retained where applicable |
| `cmd/api/stripe_sp15_test.go` | 233 | convert positive execution tests to inverse assertions; historical SQL-only fixtures/read assertions retained where applicable |
| `cmd/payment-worker/doc.go` | 2, 9 | remove execution/config/form branches; preserve Stripe behavior |
| `cmd/payment-worker/main_test.go` | 117 | convert positive execution tests to inverse assertions; historical SQL-only fixtures/read assertions retained where applicable |
| `internal/attribution/pg_flow_test.go` | 208 | historical read/fixture or reference outside write scope; inspect dependency impact and report remaining work, no edits |
| `internal/buyerhttp/payment.go` | 80, 82 | remove execution/config/form branches; preserve Stripe behavior |
| `internal/buyerhttp/payment_test.go` | 68, 69, 70, 71, 79, 80, 134, 159, 160 | convert positive execution tests to inverse assertions; historical SQL-only fixtures/read assertions retained where applicable |
| `internal/checkout/checkout.go` | 2, 24 | remove execution/config/form branches; preserve Stripe behavior |
| `internal/checkout/hosted.go` | 18, 36, 51, 58, 62, 65, 70, 74, 75, 79, 107, 172, 200, 201, 202, 207, 228, 230 | remove execution/config/form branches; preserve Stripe behavior |
| `internal/checkout/hosted_test.go` | 12, 17, 25, 54 | convert positive execution tests to inverse assertions; historical SQL-only fixtures/read assertions retained where applicable |
| `internal/checkout/payment.go` | 69 | remove execution/config/form branches; preserve Stripe behavior |
| `internal/checkout/payment_test.go` | 13, 21, 25, 26, 27, 78 | convert positive execution tests to inverse assertions; historical SQL-only fixtures/read assertions retained where applicable |
| `internal/checkout/payment_view.go` | 53, 56, 80, 81, 82, 83, 86, 119, 135, 201 | historical read: preserve stored attempt state; never offer removed method |
| `internal/checkout/payment_view_test.go` | 19, 73, 115 | convert positive execution tests to inverse assertions; historical SQL-only fixtures/read assertions retained where applicable |
| `internal/checkout/stripe.go` | 139, 147, 204, 205 | remove execution/config/form branches; preserve Stripe behavior |
| `internal/checkout/stripe_test.go` | 50, 60, 110, 111, 112, 113, 115, 116, 118, 145, 149, 173, 190, 192, 224, 227, 231, 232, 249, 268, 284 | convert positive execution tests to inverse assertions; historical SQL-only fixtures/read assertions retained where applicable |
| `internal/httpapi/accounts_test.go` | 30, 54, 55 | historical read/fixture or reference outside write scope; inspect dependency impact and report remaining work, no edits |
| `internal/httpapi/settings_test.go` | 15, 16, 35, 36, 42, 58, 60, 61, 62, 64, 66 | historical read/fixture or reference outside write scope; inspect dependency impact and report remaining work, no edits |
| `internal/integrations/accounts/crypto.go` | 130 | remove active credential/provider wiring; retain metadata reads and shared Stripe keyring |
| `internal/integrations/accounts/crypto_test.go` | 26 | convert positive execution tests to inverse assertions; historical SQL-only fixtures/read assertions retained where applicable |
| `internal/integrations/accounts/hosted.go` | 17, 29, 49, 117, 122, 133, 143 | remove execution/config/form branches; preserve Stripe behavior |
| `internal/integrations/accounts/list_test.go` | 83 | convert positive execution tests to inverse assertions; historical SQL-only fixtures/read assertions retained where applicable |
| `internal/integrations/accounts/payment_query.go` | 13, 23, 24, 49, 54, 65, 71 | remove execution/config/form branches; preserve Stripe behavior |
| `internal/integrations/accounts/payment_query_test.go` | 83, 93 | convert positive execution tests to inverse assertions; historical SQL-only fixtures/read assertions retained where applicable |
| `internal/integrations/accounts/service.go` | 71 | remove active credential/provider wiring; retain metadata reads and shared Stripe keyring |
| `internal/integrations/accounts/stripe_crypto.go` | 16 | remove active credential/provider wiring; retain metadata reads and shared Stripe keyring |
| `internal/integrations/accounts/stripe_crypto_test.go` | 156, 157, 158, 163, 165, 166, 168, 169 | convert positive execution tests to inverse assertions; historical SQL-only fixtures/read assertions retained where applicable |
| `internal/integrations/psp/payuni/acceptance_test.go` | 1, 4, 46, 50, 167, 177, 180, 187, 189, 198, 210, 212, 246, 251, 252, 253, 254, 255, 256, 268, 304, 308, 321, 442, 444, 446 | remove: adapter, amount helpers and provider-positive tests; inverse removal gates replace provider admission |
| `internal/integrations/psp/payuni/amount.go` | 1, 3 | remove: adapter, amount helpers and provider-positive tests; inverse removal gates replace provider admission |
| `internal/integrations/psp/payuni/amount_test.go` | 1 | remove: adapter, amount helpers and provider-positive tests; inverse removal gates replace provider admission |
| `internal/integrations/psp/payuni/client.go` | 1, 6, 7, 34, 35, 36, 37, 38, 39, 68, 70, 78, 80, 230, 234, 236, 238, 265, 278, 286, 288, 314, 316, 322, 324, 326, 574, 622, 623, 636, 638, 645 | remove: adapter, amount helpers and provider-positive tests; inverse removal gates replace provider admission |
| `internal/integrations/psp/payuni/client_test.go` | 1, 18, 119, 145, 159, 172, 173, 174, 175, 176, 185, 207, 212, 222, 235, 241, 247, 254, 258, 294, 295, 316, 322, 338, 355 | remove: adapter, amount helpers and provider-positive tests; inverse removal gates replace provider admission |
| `internal/integrations/psp/payuni/query_test.go` | 1, 23, 38, 51, 68, 85, 97, 120 | remove: adapter, amount helpers and provider-positive tests; inverse removal gates replace provider admission |
| `internal/integrations/shipping/ecpay/keyring.go` | 5 | historical read/reference: shared keyring comment; outside write scope, integrator follow-up |
| `internal/payments/doc.go` | 2, 11 | remove execution/config/form branches; preserve Stripe behavior |
| `internal/payments/methods.go` | 21, 242 | convert to unavailable; retain legacy method vocabulary for historical reads |
| `internal/payments/methods_test.go` | 18, 25, 82 | convert positive execution tests to inverse assertions; historical SQL-only fixtures/read assertions retained where applicable |
| `internal/payments/query_worker.go` | 18, 222, 289, 292, 299 | remove execution/config/form branches; preserve Stripe behavior |
| `internal/payments/query_worker_test.go` | 11, 54, 55, 60, 61, 75, 82, 83 | convert positive execution tests to inverse assertions; historical SQL-only fixtures/read assertions retained where applicable |
| `internal/payments/runtime.go` | 48 | remove execution/config/form branches; preserve Stripe behavior |
| `internal/payments/stripe_refund.go` | 55 | remove execution/config/form branches; preserve Stripe behavior |
| `internal/payments/stripe_refund_test.go` | 40 | convert positive execution tests to inverse assertions; historical SQL-only fixtures/read assertions retained where applicable |
| `internal/pricing/pricing.go` | 280 | remove execution/config/form branches; preserve Stripe behavior |
| `tests/admin/auth-real.spec.ts` | 337, 375, 496, 500, 538 | historical read/fixture or reference outside write scope; inspect dependency impact and report remaining work, no edits |
| `tests/admin/baselines/w0/manifest.json` | 284, 292 | historical read/fixture or reference outside write scope; inspect dependency impact and report remaining work, no edits |
| `tests/admin/settings-real.spec.ts` | 4, 5, 15, 23, 24, 27, 71, 77, 196, 209, 301, 316 | historical read/fixture or reference outside write scope; inspect dependency impact and report remaining work, no edits |
| `tests/deploy/deploy-prep-r3.test.mjs` | 312, 321, 322, 326, 329, 332, 334 | historical read/fixture or reference outside write scope; inspect dependency impact and report remaining work, no edits |
| `tests/foundation/browser_card_payments_test.go` | 238, 240 | convert positive execution tests to inverse assertions; historical SQL-only fixtures/read assertions retained where applicable |
| `tests/foundation/browser_e2e_test.go` | 333 | historical read/fixture or reference outside write scope; inspect dependency impact and report remaining work, no edits |
| `tests/foundation/browser_identity_chain_test.go` | 278 | historical read/fixture or reference outside write scope; inspect dependency impact and report remaining work, no edits |
| `tests/foundation/browser_stripe_test.go` | 284, 285, 301, 366, 599 | convert positive execution tests to inverse assertions; historical SQL-only fixtures/read assertions retained where applicable |
| `tests/foundation/browser_w6_customers_reports_test.go` | 239 | historical read/fixture or reference outside write scope; inspect dependency impact and report remaining work, no edits |
| `tests/foundation/buyer_payment_http_test.go` | 90, 149, 170, 172, 173, 174, 220, 305 | convert positive execution tests to inverse assertions; historical SQL-only fixtures/read assertions retained where applicable |
| `tests/foundation/buyer_payment_view_smoke_test.go` | 18 | convert positive execution tests to inverse assertions; historical SQL-only fixtures/read assertions retained where applicable |
| `tests/foundation/buyer_payment_view_test.go` | 106, 204 | convert positive execution tests to inverse assertions; historical SQL-only fixtures/read assertions retained where applicable |
| `tests/foundation/home_cod_review_test.go` | 717 | historical read/fixture or reference outside write scope; inspect dependency impact and report remaining work, no edits |
| `tests/foundation/hosted_payment_authority_test.go` | 25 | convert positive execution tests to inverse assertions; historical SQL-only fixtures/read assertions retained where applicable |
| `tests/foundation/hosted_payment_test.go` | 68, 102, 260, 264 | convert positive execution tests to inverse assertions; historical SQL-only fixtures/read assertions retained where applicable |
| `tests/foundation/legacy_runtime_upgrade_test.go` | 572, 573, 575 | historical read/fixture or reference outside write scope; inspect dependency impact and report remaining work, no edits |
| `tests/foundation/manual_fulfilment_guards_test.go` | 73 | historical read/fixture or reference outside write scope; inspect dependency impact and report remaining work, no edits |
| `tests/foundation/merchant_account_http_test.go` | 139 | historical read/fixture or reference outside write scope; inspect dependency impact and report remaining work, no edits |
| `tests/foundation/merchant_accounts_test.go` | 64, 432, 454 | historical read/fixture or reference outside write scope; inspect dependency impact and report remaining work, no edits |
| `tests/foundation/merchant_settings_discovery_test.go` | 194, 201 | historical read/fixture or reference outside write scope; inspect dependency impact and report remaining work, no edits |
| `tests/foundation/merchant_settings_http_test.go` | 197, 214 | historical read/fixture or reference outside write scope; inspect dependency impact and report remaining work, no edits |
| `tests/foundation/payment_capture_test.go` | 331 | convert positive execution tests to inverse assertions; historical SQL-only fixtures/read assertions retained where applicable |
| `tests/foundation/payment_methods_test.go` | 46, 104, 134, 266, 286 | convert positive execution tests to inverse assertions; historical SQL-only fixtures/read assertions retained where applicable |
| `tests/foundation/payment_query_test.go` | 348, 453 | convert positive execution tests to inverse assertions; historical SQL-only fixtures/read assertions retained where applicable |
| `tests/foundation/payment_runtime_test.go` | 279 | convert positive execution tests to inverse assertions; historical SQL-only fixtures/read assertions retained where applicable |
| `tests/foundation/payment_start_test.go` | 138, 145, 146, 148, 149, 150, 480, 481 | convert positive execution tests to inverse assertions; historical SQL-only fixtures/read assertions retained where applicable |
| `tests/foundation/payuni_removal_test.go` | 1, 4, 10, 109, 112, 118, 119, 120, 121, 127, 128, 130, 150, 151, 152, 162, 165, 180, 198, 201, 224, 258, 262, 273, 277, 282 | convert/extend removal assertions; preserve existing RM01–RM03 assertions |
| `tests/foundation/platform_operator_inflight_test.go` | 6, 94 | historical read/fixture or reference outside write scope; inspect dependency impact and report remaining work, no edits |
| `tests/foundation/r2_integration_upgrade_test.go` | 57 | historical read/fixture or reference outside write scope; inspect dependency impact and report remaining work, no edits |
| `tests/foundation/reports_test.go` | 130, 604 | historical read/fixture or reference outside write scope; inspect dependency impact and report remaining work, no edits |
| `tests/foundation/stripe_buyer_http_test.go` | 5, 8, 15, 61, 66, 85, 86, 90, 113, 186, 188, 191, 196, 202, 208, 219 | convert positive execution tests to inverse assertions; historical SQL-only fixtures/read assertions retained where applicable |
| `tests/foundation/stripe_flow_test.go` | 68, 139, 140, 141, 147, 148, 159, 387, 397, 398, 524, 529, 541 | convert positive execution tests to inverse assertions; historical SQL-only fixtures/read assertions retained where applicable |
| `tests/foundation/stripe_live_process_test.go` | 236, 351, 451 | convert positive execution tests to inverse assertions; historical SQL-only fixtures/read assertions retained where applicable |
| `tests/foundation/stripe_live_schema_test.go` | 11, 105, 494, 505, 526, 539, 550 | convert positive execution tests to inverse assertions; historical SQL-only fixtures/read assertions retained where applicable |
| `tests/foundation/stripe_process_test.go` | 10, 46, 49, 53 | convert positive execution tests to inverse assertions; historical SQL-only fixtures/read assertions retained where applicable |
| `tests/foundation/stripe_refund_guards_test.go` | 375 | convert positive execution tests to inverse assertions; historical SQL-only fixtures/read assertions retained where applicable |
| `tests/foundation/stripe_refund_http_test.go` | 9, 10, 362, 364 | convert positive execution tests to inverse assertions; historical SQL-only fixtures/read assertions retained where applicable |
| `tests/foundation/stripe_refund_request_test.go` | 64, 76, 77, 93, 363, 364 | convert positive execution tests to inverse assertions; historical SQL-only fixtures/read assertions retained where applicable |
| `tests/foundation/stripe_schema_test.go` | 140, 188, 189, 196, 203, 206, 218, 223, 228, 231, 232 | convert positive execution tests to inverse assertions; historical SQL-only fixtures/read assertions retained where applicable |
| `tests/foundation/stripe_start_test.go` | 109, 113, 262, 345, 380, 381, 390, 410, 425, 426, 450, 481, 637, 712, 724, 734, 738, 744, 747 | convert positive execution tests to inverse assertions; historical SQL-only fixtures/read assertions retained where applicable |
| `tests/foundation/taiwan_cvs_pay_at_pickup_test.go` | 143, 144 | historical read/fixture or reference outside write scope; inspect dependency impact and report remaining work, no edits |
| `tests/foundation/taiwan_cvs_schema_test.go` | 298 | historical read/fixture or reference outside write scope; inspect dependency impact and report remaining work, no edits |
| `tests/foundation/w4_u1_seed_test.go` | 273, 275 | historical read/fixture or reference outside write scope; inspect dependency impact and report remaining work, no edits |
| `tests/payments/stripe-sp14-payuni-handoff-issued.golden.json` | 1 | historical read/fixture or reference outside write scope; inspect dependency impact and report remaining work, no edits |
| `tests/payments/stripe-sp14-payuni-view-not-started.golden.json` | 1 | historical read/fixture or reference outside write scope; inspect dependency impact and report remaining work, no edits |
| `tests/storefront/stripe-browser.mjs` | 43, 153, 272, 418, 522, 575 | historical read/fixture or reference outside write scope; inspect dependency impact and report remaining work, no edits |

## Acceptance and evidence

- Local Go tests, PostgreSQL, servers, browser modes: NOT_RUN by task-card prohibition.
- Full Gates CI and money-path independent Opus/K3 review: NOT_RUN, integrator-owned.
- Red/green execution of new inverse tests: NOT_RUN locally; CI must demonstrate the pre-change red and candidate green.

## Implementation and boundaries

- Deleted the PAYUNi adapter package, its hosted signer/material loader, query material loader, hosted form construction, TWD amount helper, non-hosted start SQL dispatch, query wire/record/finish/retry branches and associated API signing configuration reads.
- Buyer HTTP and hosted admission accept only `stripe_checkout`. `StartPayment`, account `Create`/`Rotate` and retired method writes refuse before any PG/queue call. Metadata `Get`/`List`, stored provider strings, method vocabulary and payment state projections remain readable. An old method diagnoses `ADAPTER_UNAVAILABLE`; buyer method choices are filtered out without changing stored attempt state.
- Query worker rejects old operations **before lease claim**, credential load or network I/O. Capture worker rejects their persisted reports before `payments.apply_capture`. River cancellation acknowledges the obsolete queue item; payment operations, observations, events, facts, stock and order state must remain unchanged.
- `HostedProviders.PAYUNi` is an ignored source-compatibility tombstone (`removed provider — read-only, never dispatched`) because the outside-scope `browser_e2e_test.go` still initializes it. `HostedConfig`, the old constructor signature and `HostedHandoff.Form` are inert DTO compatibility only, with no form builder. Removing the tombstones requires caller coordination.
- API uses Stripe return URL only and does not read account key/notify/PAYUNI variables. The old URL normalizer was moved byte-for-byte at the noncomment-token level; LIVE approval remains unchanged. Stripe adapter, Stripe session/query/signal/refund runtime logic and Stripe registrar files were not refactored. Four comment-only production files and the moved normalizer have token hashes in `stripe-static-equivalence.json` (E1, no behaviour execution claim).
- Retained legacy envelope vocabulary in accounts crypto has no production signer/query loader/write caller. It preserves existing cross-envelope rejection/format tests and shared Stripe keyring. No plaintext credential enters a projection.
- `docs/architecture/DEPENDENCIES.md` regenerated with existing `bash scripts/dev/gen-deps.sh` (93 packages).

## Files deleted or changed

`changed-files.txt` is the tracked source diff inventory (D=deleted, M=changed). No migrations, apps, shipping, go.mod or go.sum were changed. Task artifacts are confined to this output directory. The original scan remains in `initial-payuni-grep.txt`; current scan is `remaining-payuni-grep.txt`.

| Status | Tracked source file |
|---|---|
| M | `cmd/api/accounts.go` |
| M | `cmd/api/buyer_payment.go` |
| M | `cmd/api/buyer_payment_pg_test.go` |
| M | `cmd/api/buyer_payment_test.go` |
| M | `cmd/api/payuni_removed_test.go` |
| M | `cmd/api/stripe_live_test.go` |
| M | `cmd/api/stripe_sp15_test.go` |
| M | `cmd/payment-worker/doc.go` |
| M | `cmd/payment-worker/main_test.go` |
| M | `docs/architecture/DEPENDENCIES.md` |
| M | `internal/buyerhttp/payment.go` |
| M | `internal/buyerhttp/payment_test.go` |
| M | `internal/checkout/checkout.go` |
| M | `internal/checkout/hosted.go` |
| M | `internal/checkout/hosted_test.go` |
| M | `internal/checkout/payment.go` |
| M | `internal/checkout/payment_test.go` |
| M | `internal/checkout/payment_view.go` |
| M | `internal/checkout/payment_view_test.go` |
| M | `internal/checkout/stripe.go` |
| M | `internal/checkout/stripe_test.go` |
| M | `internal/integrations/accounts/crypto.go` |
| D | `internal/integrations/accounts/hosted.go` |
| M | `internal/integrations/accounts/hosted_test.go` |
| D | `internal/integrations/accounts/payment_query.go` |
| M | `internal/integrations/accounts/payment_query_test.go` |
| M | `internal/integrations/accounts/service.go` |
| M | `internal/integrations/accounts/stripe_crypto.go` |
| D | `internal/integrations/psp/payuni/acceptance_test.go` |
| D | `internal/integrations/psp/payuni/amount.go` |
| D | `internal/integrations/psp/payuni/amount_test.go` |
| D | `internal/integrations/psp/payuni/client.go` |
| D | `internal/integrations/psp/payuni/client_test.go` |
| D | `internal/integrations/psp/payuni/query_test.go` |
| M | `internal/payments/capture_worker.go` |
| M | `internal/payments/doc.go` |
| M | `internal/payments/methods.go` |
| M | `internal/payments/methods_test.go` |
| M | `internal/payments/query_worker.go` |
| M | `internal/payments/query_worker_test.go` |
| M | `internal/payments/runtime.go` |
| M | `internal/payments/stripe_refund.go` |
| M | `internal/pricing/pricing.go` |
| M | `tests/foundation/buyer_payment_view_smoke_test.go` |
| M | `tests/foundation/payment_query_test.go` |
| M | `tests/foundation/payuni_removal_test.go` |

## Converted / deleted test inventory

No test was removed merely for mentioning PAYUNi. Adapter-specific success/codec tests retire with the adapter and are replaced by the stronger invariant that **no execution is reachable**, rather than by a claim that the removed wire protocol still works. Shared financial/authority invariants are retained in existing Stripe tests and the new worker snapshot test. This inventory does **not** claim those tests passed: all runtime tests remain NOT_RUN.

Abbreviations: `NO_START` = `TestRemovedHostedMethodsFailBeforePGOrQueue` + `TestPaymentInputAndProfileBoundary`; `NO_WORK` = `TestRemovePayuniGoRM2WorkerSkipsHistoryWithoutObservation`; `HISTORY` = `TestRemovedPaymentProjectionPreservesHistory` + RM2 worker merchant/buyer reads; `NO_CONFIG` = `TestRemovePayuniGoRM2StripeOnlyConfig`; `NO_ROUTE` = unchanged RM03 notify-route assertions. These names below are existing or added Go test functions, not results.

| File / test | Change and remaining invariant |
|---|---|
| `cmd/api/buyer_payment_pg_test.go` / `TestBuyerPaymentPoolAssemblyRealPG` (changed) | Invalid retired key stage replaced by invalid hosted DSN stage; pool authority/assembly checks retained. CI-only buyerintegration mode. |
| `cmd/api/buyer_payment_test.go` / `TestBuyerPaymentConfigUsesHistoricalKeyParserWithoutMerchantHTTP` (deleted/replaced) | Renamed/replaced by TestBuyerPaymentConfigHasNoRetiredKeyParser: removed notify/account key dependencies, listener/profile/DSN boundaries retained. |
| `cmd/api/buyer_payment_test.go` / `TestBuyerPaymentStripeFlagIsNestedAndNeverReadsStripeSecrets` (changed) | Same nested flag/Stripe-secret and return URL expectations; now requires success with notify/key settings absent. |
| `cmd/api/buyer_payment_test.go` / `TestBuyerPaymentConfigHasNoRetiredKeyParser` (added) | Inverse config test no longer requires removed signing config. NO_CONFIG explicitly detects any retired getenv call. |
| `cmd/api/payuni_removed_test.go` / `TestRemovePayuniNotifyRM03RouterHasNoNotifyRoute` (changed) | Assertions unchanged. Conservative inventory includes trailing appended comments/code; new RM2 test is separate. |
| `cmd/api/payuni_removed_test.go` / `TestRemovePayuniGoRM2StripeOnlyConfig` (added) | Added inverse removal gate; checks no execution/config write or retained historical state as described in its test name and source. |
| `cmd/api/stripe_live_test.go` / `TestBuyerStripeCheckoutLiveNeedsThePair` (changed) | Comment only: historical read-only assembly wording; LIVE approval assertions unchanged. |
| `cmd/api/stripe_sp15_test.go` / `TestStripeSP15Process` (changed) | Removed payuni_notify_absent negative subcase; inverse NO_CONFIG requires success without it. Other process/config boundaries retained. |
| `cmd/payment-worker/main_test.go` / `TestWorkerLiveStripeNeedsThePairAndOthersReadNoLiveName` (changed) | Comment only: retired provider wording; worker LIVE approval assertions unchanged. |
| `internal/buyerhttp/payment_test.go` / `TestBuyerPaymentSignalRoutesAreKeylessPostOnlyAndFailClosed` (changed) | Comment only; signal route/method/fail-closed assertions unchanged. |
| `internal/buyerhttp/payment_test.go` / `TestBuyerPaymentKeylessPathsAreNonRetryableAndPrepareAdmitsStripe` (changed) | PAYUNi prepare expectation becomes refusal; Stripe/keyless contract unchanged. |
| `internal/checkout/hosted_test.go` / `TestHostedFormIsBoundedSingleValueJSON` (deleted/replaced) | Replaced by TestRemovedHostedMethodsFailBeforePGOrQueue. No form builder exists; all five legacy methods refuse before dependencies. |
| `internal/checkout/hosted_test.go` / `TestHostedHandoffNeverReturnsFormAfterIssued` (deleted/replaced) | Replaced by HISTORY and RM2 hosted/historical handoff assertions: no old form/redirect ever released, stored state preserved. |
| `internal/checkout/hosted_test.go` / `TestRemovedHostedMethodsFailBeforePGOrQueue` (added) | Added inverse removal gate; checks no execution/config write or retained historical state as described in its test name and source. |
| `internal/checkout/hosted_test.go` / `TestRemovedPaymentProjectionPreservesHistory` (added) | Added inverse removal gate; checks no execution/config write or retained historical state as described in its test name and source. |
| `internal/checkout/payment_test.go` / `TestPaymentInputAndProfileBoundary` (changed) | Converted successful old-input checks to no-start assertions with deliberately unusable nonnil pool/queue; original profile rejection checks retained. |
| `internal/checkout/payment_test.go` / `TestPaymentResultAndJobBoundary` (changed) | Legacy success result validator deleted; exact River job JSON/kind retained. Stripe closed currency/result tests unchanged except removed comparison with retired validator. |
| `internal/checkout/payment_view_test.go` / `TestPaymentViewBoundedShape` (changed) | Base old projection now has no executable method; malformed shape, money, state and cutoff checks retained. |
| `internal/checkout/stripe_test.go` / `TestHostedProvidersConstructorBoundary` (changed) | Retired PAYUNi provider config cases removed; empty providers now allow historical reads. Invalid ctx/job/profile/Stripe config assertions retained. |
| `internal/checkout/stripe_test.go` / `TestStripeHandoffValidatorAndRedirectShape` (changed) | Legacy cross-type check uses remaining Stripe validator; old byte omission check retained. Stripe redirect cases unchanged. |
| `internal/checkout/stripe_test.go` / `TestStripePaymentResultUsesClosedCurrencyTable` (changed) | Removed comparison with retired TWD-only validator and old method positive input; all Stripe currency/result bounds retained. |
| `internal/checkout/stripe_test.go` / `TestPaymentViewV2Validator` (changed) | Only Stripe method admitted; added single old-method rejection, retained duplicates/extra/unknown and all Stripe state/refund validation. |
| `internal/integrations/accounts/hosted_test.go` / `TestHostedConfigCanonicalDigest` (deleted/replaced) | Retired digest test replaced by TestRemovedAccountCreateNeverReadsOrWrites + NO_CONFIG. Stripe config digest test unchanged; exact return URL normalizer retained. |
| `internal/integrations/accounts/hosted_test.go` / `TestHostedConfigRejectsUnsafeEndpoints` (deleted/replaced) | No old endpoint input is admitted. NO_CONFIG forbids reads; existing API bad-return URL + Stripe config tests cover the remaining endpoint. |
| `internal/integrations/accounts/hosted_test.go` / `TestRemovedAccountCreateNeverReadsOrWrites` (added) | Added inverse removal gate; checks no execution/config write or retained historical state as described in its test name and source. |
| `internal/integrations/accounts/payment_query_test.go` / `TestPaymentQueryProfileFences` (deleted/replaced) | Removed with material loader. TestQueryWorkerOptionsAndMockIsolation and WorkerAuthority tests retain profile boundaries; NO_WORK refuses legacy provider before loading material. |
| `internal/integrations/accounts/payment_query_test.go` / `TestLoadPaymentQueryRejectsUnscopedInputs` (deleted/replaced) | Removed loader cannot be invoked. TestRemovedAccountRotateNeverReadsOrWrites + NO_WORK prevent credential loading; foundation historical ACL assertions retained. |
| `internal/integrations/accounts/payment_query_test.go` / `TestLoadPaymentQueryPreservesOnlyAuthenticatedAgeOnMissingKey` (deleted/replaced) | Removed loader no longer returns age/client/credentials. NO_WORK asserts old attempts untouched irrespective of credential usability. |
| `internal/integrations/accounts/payment_query_test.go` / `TestRemovedAccountRotateNeverReadsOrWrites` (added) | Added inverse removal gate; checks no execution/config write or retained historical state as described in its test name and source. |
| `internal/integrations/psp/payuni/acceptance_test.go` / `TestWireOfficialGoldenAndIndependentCodec` (deleted/replaced) | Deleted with adapter; no executable provider codec remains. NO_START + NO_WORK prevent any PAYUNi wire generation. |
| `internal/integrations/psp/payuni/acceptance_test.go` / `TestWireHostedNodeInteroperability` (deleted/replaced) | Deleted with adapter; hosted form interoperability is retired. NO_START rejects before PG/queue; RM2 unavailable/start test forbids handoff. |
| `internal/integrations/psp/payuni/acceptance_test.go` / `TestWireRejectsUnsafeInputs` (deleted/replaced) | Deleted with adapter; all legacy inputs now rejected (NO_START), rather than validating an executable PAYUNi request. |
| `internal/integrations/psp/payuni/acceptance_test.go` / `TestWireNotificationBindingAndPending` (deleted/replaced) | Deleted with adapter; NO_ROUTE refuses every notify route; RM01/RM02 retain schema/data-removal safety, NO_WORK retains history. |
| `internal/integrations/psp/payuni/acceptance_test.go` / `TestWireMalformedAuthenticatedPayloads` (deleted/replaced) | Deleted with adapter; no PAYUNi notification decoder or receiver remains (NO_ROUTE); Stripe malformed webhook tests unchanged. |
| `internal/integrations/psp/payuni/acceptance_test.go` / `TestWireQueryRequestAndObservation` (deleted/replaced) | Deleted with adapter; NO_WORK pins zero external calls and zero new observations. |
| `internal/integrations/psp/payuni/acceptance_test.go` / `TestWireQueryRejectsAmbiguousAndMalformedResponses` (deleted/replaced) | Deleted with adapter; no response can enter removed decoder (NO_WORK); Stripe webhook/query validation unchanged. |
| `internal/integrations/psp/payuni/acceptance_test.go` / `TestWireQueryTransportFailsClosed` (deleted/replaced) | Deleted with adapter; NO_WORK requires zero calls even for an authenticated historical report. |
| `internal/integrations/psp/payuni/acceptance_test.go` / `TestWireSensitiveFieldsStayOutOfProjectionAndFormatting` (deleted/replaced) | Deleted with adapter; HISTORY plus existing TestCredentialsNeverFormatOrJSONAsPlaintext and Stripe custody redaction tests cover retained projections/material. |
| `internal/integrations/psp/payuni/amount_test.go` / `TestAmountTWDFromMinorPreservesExactValue` (deleted/replaced) | Deleted with removed TWD adapter helper; NO_START eliminates conversion. Stripe TestStripeSP02Currency and pricing currency-step gates unchanged. |
| `internal/integrations/psp/payuni/client_test.go` / `TestOfficialNodeVector` (deleted/replaced) | Deleted with removed adapter; NO_START + NO_WORK prevent protocol use rather than asserting support for the retired vector. |
| `internal/integrations/psp/payuni/client_test.go` / `TestConfigRedactionAndValidation` (deleted/replaced) | Deleted with adapter; NO_CONFIG prevents config reads; existing credentials/Stripe redaction tests remain. |
| `internal/integrations/psp/payuni/client_test.go` / `TestQueryOnlyClientCannotUsePaymentOrNotification` (deleted/replaced) | Deleted with adapter; NO_START + NO_WORK + NO_ROUTE now remove every capability, not just query-only limits. |
| `internal/integrations/psp/payuni/client_test.go` / `TestQueryOnlyClientUsesSignedWire` (deleted/replaced) | Deleted with adapter; inverse NO_WORK requires no wire at all. |
| `internal/integrations/psp/payuni/client_test.go` / `TestBuildHostedBindsOnePaymentSelector` (deleted/replaced) | Deleted with adapter; inverse NO_START refuses all five stored method codes. |
| `internal/integrations/psp/payuni/client_test.go` / `TestNotificationAuthenticationAndBinding` (deleted/replaced) | Deleted with adapter; NO_ROUTE and unchanged RM01/RM02 forbid receiver admission and silent historical deletion. |
| `internal/integrations/psp/payuni/client_test.go` / `TestQueryOneBoundedObservation` (deleted/replaced) | Deleted with adapter; NO_WORK requires no observation or operation update. |
| `internal/integrations/psp/payuni/client_test.go` / `TestQueryRejectsMultirowAndUntrustedEnvelope` (deleted/replaced) | Deleted with adapter; no decoder reachable (NO_WORK); Stripe strict JSON/provider-identity tests unchanged. |
| `internal/integrations/psp/payuni/query_test.go` / `TestSignedCreditQueryCaptureAndRefundProjection` (deleted/replaced) | Deleted with adapter; NO_WORK rejects query and capture; existing TestRefundArgsAndOperationFamily refuses non-Stripe refunds. |
| `internal/integrations/psp/payuni/query_test.go` / `TestSignedInstallmentAndNoncardIsolation` (deleted/replaced) | Deleted with adapter; NO_START rejects all legacy methods; TestQueryFamilyAdmitsOnlyStripe refuses the provider. |
| `internal/integrations/psp/payuni/query_test.go` / `TestSignedCreditQueryRejectsMalformedOptionalFacts` (deleted/replaced) | Deleted with adapter; no optional PAYUNi fact decoder remains. NO_WORK protects stored state; Stripe observation validation unchanged. |
| `internal/payments/methods_test.go` / `TestDiagnosticReasonsOrderAndFailClosed` (changed) | Assertions unchanged; conservative inventory includes the appended removal test. ADAPTER_UNAVAILABLE reason/order remains pinned. |
| `internal/payments/methods_test.go` / `TestRemovedMethodsNeverWriteConfiguration` (added) | Added inverse removal gate; checks no execution/config write or retained historical state as described in its test name and source. |
| `internal/payments/query_worker_test.go` / `TestQueryFamilyBudgetAndRetry` (deleted/replaced) | Replaced by TestQueryFamilyAdmitsOnlyStripe. Legacy retry/budget algorithm deleted; existing Stripe runtime budgets unchanged. |
| `internal/payments/query_worker_test.go` / `TestQueryPanicBecomesFixedFailure` (deleted/replaced) | Replaced by TestRemovedQueryProviderNeverDispatchesAnyStoredState and NO_WORK: provider cannot reach panic/transport/finish paths. |
| `internal/payments/query_worker_test.go` / `TestQueryFamilyAdmitsOnlyStripe` (added) | Added inverse removal gate; checks no execution/config write or retained historical state as described in its test name and source. |
| `internal/payments/query_worker_test.go` / `TestRemovedQueryProviderNeverDispatchesAnyStoredState` (added) | Added inverse removal gate; checks no execution/config write or retained historical state as described in its test name and source. |
| `tests/foundation/buyer_payment_view_smoke_test.go` / `TestBuyerPaymentViewSQLSmoke` (changed) | Old eligible fixture unchanged; inverse expects zero methods, same NOT_STARTED state and zero writes. |
| `tests/foundation/payment_query_test.go` / `TestBuyerPaymentQueryHistoricalCredentialAndACL` (changed) | Uses a new explicitly historical SQL seed (existing fixtures unchanged); retired loader success becomes start refusal. Original function/credential/observation ACL checks and pending state assertion retained. |
| `tests/foundation/payuni_removal_test.go` / `TestRemovePayuniNotifyRM02SeededDataBlocks` (changed) | Assertions unchanged. Conservative inventory includes appended RM2 fixture helpers; original seeded-row refusal assertions retained. |
| `tests/foundation/payuni_removal_test.go` / `TestRemovePayuniGoRM2UnavailableAndNoStart` (added) | Added inverse removal gate; checks no execution/config write or retained historical state as described in its test name and source. |
| `tests/foundation/payuni_removal_test.go` / `TestRemovePayuniGoRM2WorkerSkipsHistoryWithoutObservation` (added) | Added inverse removal gate; checks no execution/config write or retained historical state as described in its test name and source. |

The inventory parser conservatively associates trailing helper text with the preceding test; entries marked assertions unchanged were checked in Git diff. No thresholds were reduced. Existing shared fixtures were not rewritten to restore old execution success. A separate synthetic historical row helper was added solely for the new inverse/history tests, using the unchanged SQL boundary and no credentials/provider call.

## Unused modules (`go mod tidy` owned by integrator)

None identified: removed PAYUNi package used only Go standard-library imports; PG/River dependencies remain in Stripe and shared runtime. go.mod/go.sum untouched; no tidy was run. The removed import path is absent from Go runtime imports (`retired-api-grep.txt`). Integrator must verify tidy on the integrated head.

## Phase 2b: database objects retained, owner decision required

This is a **source catalog audit**, not a live DB inventory; no database connection was made. `phase2b-migration-references.txt` contains exact migration lines. Retention/archival, outstanding-attempt reconciliation and FK-safe cleanup need an owner-approved destructive migration; no DROP/data deletion was performed in phase 2a.

| Object group | Retained legacy rows / dependencies to decide |
|---|---|
| `integration.merchant_accounts`, `integration.account_credentials`, `integration.bindings` | PAYUNi account/binding metadata, historical encrypted credential versions; tables are shared with Stripe/other integrations, never drop wholesale. |
| `payments.method_heads`, `payments.method_versions`, `payments.account_qualifications` | `provider='payuni'`, `payuni_*` codes, qualification/account links and related checks; shared Stripe method tables. |
| `checkout.payment_attempts`, `checkout.command_results`, `checkout.events`, `checkout.orders` | Historical method/attempt/provider references, idempotency receipts, frozen order facts. Preserve merchant/buyer read history. |
| `checkout.hosted_payment_pages` | Old encrypted prepared/issued form material and config digests, plus `hosted_guard`, `load_hosted_material`, `save_hosted_page`, `take_hosted_page` functions (0025 and later replacements). No Go signer/taker remains. |
| `integration.operations`, `integration.operation_events`, payment-family River jobs | Old `payuni.query`/reconcile operations, generation/lease/outcome history, queue routing/guards/legacy handoff dependents. Shared operation and queue machinery must remain for Stripe. |
| `integration.require_payment_query`, `load_payment_query`, `record_payment_query`, `finish_payment_query`; `checkout.start_payment` | SQL legacy execution capabilities, EXECUTE grants, definer owners and profile/queue guards still exist in phase 2a. Removing Go callers does **not** revoke direct SQL authority; phase 2b needs explicit review. |
| `payments.provider_observations`, `payments.facts`, `payments.review_cases`, `fulfillment.payment_work_items`, inventory reservation/ledger rows | Historical financial/evidence/fulfillment references. Shared tables, evidence retention and unpaid stock disposition need decisions; never silently reprocess or erase. |
| `payments.apply_capture_payuni_v1` and shared `payments.apply_capture` dispatcher | 0061 renames old implementation and retains dispatch. Go capture worker now refuses old provider. Phase 2b must remove only legacy dispatch and keep Stripe financial semantics/ACLs. |
| `checkout.hosted_payment_view`, `_v2`, `hosted_payment_provider`, merchant order projections | Read-only legacy vocabulary and joins are intentionally retained; remove executable method choices and adapt history projection before dropping referenced objects. |
| Constraints, triggers, RLS, grants, registries and post-River router functions | References span 0014–0018, 0025/0026 and later Stripe/profile/queue replacements. Audit final installed definitions, not only migration creation order. |

Migration 0161 **already** removes the PAYUNi notify endpoint/receipt tables, four functions, role and review policy when its empty-data guards hold. They are not claimed as still-existing objects on a fully migrated database; RM01/RM02 remain unchanged. Installed schema state is NOT_RUN.

## Remaining scan and blocking test contract

Production scan has seven lines: retained provider constant; historical method vocabulary; old-envelope discriminator; buyer historical method filter (comment + predicate); ignored HostedProviders tombstone; one shipping keyring comment **outside write scope**. `remaining-payuni-grep.txt` includes the tests too. The literal acceptance wording “only enum/history/removal tests” still needs integrator disposition for the compatibility tombstone and outside-scope comment; no runtime call remains.

**BLOCKED: legacy-positive test migration is not complete.** Existing assertions still require PAYUNi start/form/query/capture success. Keeping those assertions and fixtures makes their incompatibility visible; this branch cannot honestly be called Full-Gates-ready. Representative direct callers are in `legacy-caller-audit.txt`:

- Within scope: payment_start, hosted_payment, buyer_payment_http/view, payment_query/capture/runtime, payment_methods and Stripe coexistence/process/browser suites. Remaining positive cases must become explicit refusal/unchanged-history assertions without discarding shared authority, atomicity, race or Stripe checks.
- Outside write scope: merchant_accounts/account_http, merchant_orders, merchant_settings, parcel/manual fulfillment, reports, legacy_runtime_upgrade, browser_e2e and related helpers. These depend transitively on retired account/start APIs. Updating only the allowed files would leave these gate modes broken.
- A scope clarification was requested during the task; no expansion was received. No outside-scope test, fixture, threshold, UI file or migration was changed. Integrator must coordinate a unified test-contract migration rather than bypassing old assertions or restoring runtime PAYUNi compatibility.
- No attempts were made to run the prohibited runtime gates. Static vet integration corrections were three distinct issues (removed loader caller, unused local after conversion, wrong market field), each fixed once; no same blocker was retried three times.

## Evidence, NOT_RUN and risks

| Static command | Exit | Evidence tier |
|---|---|---|
| `go build ./...` | 0 | E1 |
| `go vet ./...` | 0 | E1 |
| `go vet -tags=buyerintegration ./cmd/api` | 0 | E1 |

Source manifest SHA-256: `4323500cb663298bb4c5c824fb5a32718d4d03a2a84af93b0ddf29871b58cda1`. Sources unchanged during all checks: `True`. `scope-check.json`: no forbidden-path edits; diff check exit 0.


Final local static command receipts are `static-checks.json`, `static-checks.log` and `source-hashes.sha256`; hashes bind all tracked Go sources and module files read by build/vet. Earlier failed static diagnostics are preserved in vet*.log. Empty compiler output alone is not a PASS receipt; use recorded exit codes and matching source hashes.

- Allowed static `go build ./...` and `go vet ./...`: see exact exit codes in `static-checks.json` (E1 only).
- Full Gates CI, all foundation shards, browser payment modes/real-click ledger, buyerintegration-tag tests, race/PG/runtime/provider execution: **NOT_RUN**, task-card CI-only prohibition. No local Go test, PG instance or server was run.
- New inverse gate pre-change RED / current-head GREEN: **NOT_RUN**; integrator must run on CI. The new gate is source-ready only, not established E3 evidence.
- Opus/K3 independent money-path review: **NOT_RUN**, integrator arranges it. Author self-review is not independent acceptance.
- Runtime SQL/ACL correctness of the new capture provider read and new historical fixture assertions: **NOT_RUN**. Immutable provider/worker visibility assumptions need CI and review.
- Existing PAYUNi attempts remain frozen; outstanding money/stock/manual reconciliation needs owner handling. No automatic refund/retry/data cleanup is introduced.
- No push, PR, merge, deployment, production key, outbound PSP request or production write. Final local commit is a partial handoff, not a release or task acceptance claim.
