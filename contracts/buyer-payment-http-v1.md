# Buyer order payment projection and private transport v1

2026-09-25. DRAFT — independent preflight required before implementation.
Base `18f5a2d`; builds on `payment-hosted-v1.md`, `buyer-http-v1.md` and
`buyer-browser-bff-v1.md`. No second payment attempt engine or PSP product link.

## Decisions and scope

Expose the existing hosted core through the same BFF-only buyer router. Add a
read-only original-order payment projection; do not let HTTP/UI infer paid from
the attempt, provider redirect, handed-out form, or commercial order state.
The existing Start/Take transactions retain all monetary/qualification authority.
This increment implements private Go HTTP and disabled-by-default assembly.
Public BFF/client/UI, return/notify routes, worker and provider acceptance follow;
they are not considered delivered by these private HTTP tests.

## Read model and SQL authority

`(*checkout.HostedPaymentStarter).PaymentView(ctx, token, storeID, orderID string)
(OrderPayment, error)` uses its existing dedicated hosted pool/profile/config.
No additional service/queue/table/secret is needed.
Go `PaymentMethodOption` has Code string, Version int64, NameHans/NameHant/NameEN
string. `OrderPayment` uses OrderID/Currency/CommercialState/PaymentState/
HandoffState string, TotalMinor int64, HandoffExpiresAt *time.Time (no omitempty),
Methods []PaymentMethodOption; JSON tags are exactly the snake_case fields below.

Integrator reserves migration0026 for
`checkout.hosted_payment_view(bytea,uuid,uuid,text,bytea) RETURNS jsonb`:
capability hash, store, owned order, execution profile, canonical config digest.
SECURITY DEFINER owned by checkout_writer, fixed pg_catalog search_path, PUBLIC
revoked, EXECUTE only commerce_hosted_runtime. Reuse existing table grants; do
not expose tables, form JSON, credentials or broader column grants to callers.
Resolve capability and replace GUC scope before reading. Explicit tenant/store/
owner/order predicates are mandatory (capture's broader attempt RLS is not enough).
Only READ COMMITTED, read-only business effects, final active-capability recheck
in SQL and Go. Missing/foreign order returns PT404; invalid input PT400;
absent/revoked capability PT401; an existing different-profile attempt PT409.
Extend checkout.safeError with PT404 -> command.ErrNotFound, retaining all existing
error mappings. Verify the actual missing/foreign SQL -> Go -> HTTP404 chain.

One statement reads order, original attempt, facts/review, page metadata and
method candidates from the same snapshot. No additional business-row/advisory
locks or provider requests are introduced; retain resolve_scope's existing
capability FOR SHARE locks. Eligibility is a snapshot, never a
reservation or admission ticket; Begin/Take recheck their full existing gates.

Exact `OrderPayment` wire shape (same internal Go result, no private IDs):

- `order_id`, `currency`, `total_minor`, `commercial_state` from the owned order.
- `payment_state`: `NOT_STARTED` if no attempt; otherwise `REVIEW_REQUIRED` when
  a review row exists, then `CAPTURED`, then `AUTHORIZED`, otherwise `PENDING`.
  Facts qualify only with the original attempt's exact scope, amount, currency,
  connection, profile and environment. Full CAPTURED is not bank settlement.
  An attempt/UNKNOWN/NOT_FOUND/elapsed deadline alone never means paid or failed.
- `handoff_state`: `NONE` without page; otherwise `ISSUED` if already handed out;
  else `UNAVAILABLE` if config digest differs; else `EXPIRED` if DB time is at or
  after its deadline; otherwise `PREPARED`. PREPARED is not permission to bypass
  live Take eligibility. Never expose stored form bytes on this GET.
- `handoff_expires_at`: null without page, UTC timestamp otherwise.
- `methods`: non-null array, currently 0 or 1 item because only `payuni_credit`
  is implemented. Each item has exactly `code`, `version`, `name_hans`,
  `name_hant`, `name_en`. Names come from current merchant method configuration.
  No environment, connection, binding, qualification or credential identifiers.

Methods appear only when no original attempt exists and order DRAFT + matching
HELD reservation/provenance/generation have not expired. Match current active
market/currency; method head/version enabled+visible and order country/market;
TWD whole-dollar amount 100..19999900 minor units and configured limits; exact
payuni merchant account/environment, enabled binding + semantic version/asset,
current credential-qualified proof observed <= DB now < expiry, non-revoked,
and exact PROVIDER_MOCK / REAL_SANDBOX / REAL_LIVE profile rules from0016/0025.
No alternative method is invented when admission fails. View uses DB time, and
the final capability check may only remove stale time-sensitive options or mark
an expired unissued handoff expired; it must not renew eligibility/deadlines.

## Private routes and projections

All routes reuse private BFF-key, exact published origin resolution, capability,
path/body/header validation, 10-second deadline and no-store headers. Disabled
payment service returns404; no caller-supplied provider/profile/amount/URLs/forms.

| Method/path | Input | Exact success response |
| --- | --- | --- |
| GET `/v1/buyer/orders/{id}/payment` | No query/body/idempotency key | OrderPayment above |
| POST `/v1/buyer/orders/{id}/payment/prepare` | Required Idempotency-Key; JSON exactly method_code, method_version, locale | order_id, state PAYMENT_PENDING, currency, amount_minor only |
| POST `/v1/buyer/orders/{id}/payment/handoff` | No query/body/idempotency key | Existing HostedHandoff: order_id, disposition, expires_at; form only for ISSUED |

Prepare injects path order ID into HostedInput; locale exactly zh-CN/zh-TW/en.
It projects the repeatable receipt; never serializes PaymentResult private attempt,
operation, job, generation or merchant trade identifiers. Replaying prepare does
not convey current payment status; GET does. Take is mutating despite empty body.
Every handoff error (including early auth/routing/service failures) is nonretryable.
No implicit retry, response/form cache or idempotent response journal for handoff.
The one-shot DB marker still protects against repeated explicit requests.

`buyerhttp.New` retains existing call sites via at most one optional concrete
`*checkout.HostedPaymentStarter` argument after ttl; nil/absent disables payment.
More than one optional service is invalid. No service-locator/provider abstraction.

## Default-closed API assembly

New `COMMERCE_BUYER_PAYMENT_ENABLED` flag follows existing flag grammar. Disabled
mode reads only this flag. Enabled mode requires enabled buyer API/private listener
and a separate `COMMERCE_HOSTED_DATABASE_URL` verified by OpenHostedPool.
It is nested in buyerConfig: a disabled root buyer API still ignores all buyer
subconfiguration and reads only its existing root flag. When the root is enabled,
loadBuyerConfig calls the private payment loader; buildBuyerHandler keeps its
existing public signature and consumes the nested optional payment config.
Require `COMMERCE_PAYMENT_PROFILE` in PROVIDER_MOCK/SANDBOX/LIVE and fixed valid
`COMMERCE_PAYMENT_RETURN_URL`, `COMMERCE_PAYMENT_NOTIFY_URL`.
Reuse the existing COMMERCE_ACCOUNT_ACTIVE_KEY_ID / KEYS_JSON / REPLAY_KEY parser
and Keyring, extracting its private parser without changing old validation. Do
not require enabling merchant credential HTTP solely to read historical keys.
No secret values, DSNs or raw provider form enter logs/errors. Startup/partial
failure closes each owned pool once. River client only enqueues, starts no worker.
Enabling the private feature does not issue REAL qualification or enable methods.

## Acceptance gates

BPH01 real PG view: eligible DRAFT + 3-name values, all no-option admission drifts,
expiry and owned absent/foreign/scope/profile denial; exact JSON with no secrets.
BPH02 pending/authorized/captured/review precedence from actual signed test reports
and existing capture path; page NONE/PREPARED/ISSUED/EXPIRED/config mismatch;
view has zero payment/stock/event/job/receipt mutations. Redirect is not evidence.
BPH03 actual private HTTP + PG prepare -> view -> one-shot handoff -> view/repeat;
exact keys/projections, original frozen amount, 3 locales, no provider request.
BPH04 all new routes: early auth/origin/revocation/foreign/path/query/null/unknown
field/method/key denials, repeated/concurrent handoff returns at most one form;
every handoff failure nonretryable, including unavailable domain/disabled feature.
BPH05 config negative/disabled-not-reading-secrets; exact hosted DSN role and
key/config assembly; no changes to existing disabled/API routes or account parser.
BPH06 full real PG/race/vet plus independent source and evidence review; no old
HP01–07 or buyer/financial/grant gate weakened. Public browser gate remains NOT_RUN.

## Next boundary and rejected shortcuts

Public BFF must keep CSRF/Origin/context checks even when handoff has no replay
key. It must never use its retry journal for handoff, store form bytes, accept an
action URL from the buyer, or call Take on mount/reload/background polling.
One explicit Pay action prepares and takes once; recovery is original-order
status plus explicit continuation if appropriate, never a new attempt or timeout
stock release. Safe return correlation, three-locale/mobile page flow and mock
PSP browser gate must be frozen and verified before buyer-facing enablement.
Real qualification, sandbox transaction, notifications/workers and full SaaS
release remain required. This contract does not authorize production changes.
