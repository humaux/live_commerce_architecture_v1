# Buyer payment public transport v1

2026-09-25. FROZEN_IMPLEMENTATION after independent contract preflight and
prepared-amount/name-bound corrections. Base `5258928`.
Extends `buyer-payment-http-v1.md` and `buyer-browser-bff-v1.md`.

## Smallest boundary

Reuse the existing Next BFF, signed HttpOnly buyer cookie, origin/context checks,
single-fetch transport and private Go payment service. No new auth, payment
engine, merchant authority, dependency or payment journal. The existing private
payment flag remains the feature gate; this does not enable any merchant method.
The B product detail composition is unchanged by this transport increment.
Public UI, form submission, provider return and real-browser payment acceptance
remain required follow-ups, not claims made by these transport tests.

## Public routes

Public paths replace `/v1/buyer/` with `/api/buyer/` on the three private routes.
Exact lowercase UUID order IDs only. No query, fragment, caller amount/provider,
redirect URL, profile, credential or form input. All current forbidden headers,
cookie/context/CSRF checks, body bounds, timeout, redirect:error and no-store
behavior apply. Forward one request only, never retry.

- GET `orders/{id}/payment`: no body or Idempotency-Key.
- POST `orders/{id}/payment/prepare`: JSON exactly `method_code`,
  `method_version`, `locale`, required valid Idempotency-Key. All fields required;
  code exactly `payuni_credit`, positive safe integer version, locale exactly
  `zh-CN`, `zh-TW` or `en`. Reject duplicates, null, extras and wrong types before
  upstream fetch. Current purchase/session writes retain their existing policy.
- POST `orders/{id}/payment/handoff`: no body or Idempotency-Key. It is still a
  mutation with same-origin and exact buyer-context checks. Classify this raw
  path before configuration/auth/routing so EVERY error is nonretryable,
  including disabled, wrong method/ID/query, early503, deadline, network loss,
  malformed success and upstream429/503. Never expose upstream error text.

`buyerRequest` permits absent body/key ONLY for the exact POST handoff suffix
above, and rejects either supplied body or key. It must still reject unresolved
cookie/session journal state and invalid contexts before fetch. Other writes
continue to require their body/key. The lower transport still makes one fetch.
This is not permission for callers to replay handoff: payment UI will use only
an explicit user action and GET recovery, never mount/reload retry.

## Exact safe projections

Add a dependency-free `apps/storefront/lib/payment-contract.ts` shared by BFF
and the forthcoming client controller; no network, DOM or storage in this file.
Exports `OrderPayment`, `PaymentPrepared`, `HostedHandoff`, `HostedForm` types and
`validOrderPayment(value, orderID)`, `validPaymentPrepared(value, orderID)`,
`validHostedHandoff(value, orderID)` type guards. Reuse existing Go wire names;
do not expose private attempt, operation, job, account, binding or trade IDs.

For new payment routes only, require HTTP200 and a bounded application/json
response before returning success. Reject duplicate keys (including nested
fields), invalid UTF8, unknown/missing fields and mismatching path orderID.
Existing strictJSON scanner may gain an explicit allowNull option, defaultfalse;
only payment response parsing permits null. Do not loosen request validation.

`OrderPayment` exactly matches the private contract's nine fields. Monetary
values are safe integers 0..1e12; currency uppercase ISO-shaped three letters;
commercial state DRAFT/AWAITING_PAYMENT/CONFIRMED/CANCELLED; payment state
NOT_STARTED/PENDING/AUTHORIZED/CAPTURED/REVIEW_REQUIRED; handoff state
NONE/PREPARED/ISSUED/EXPIRED/UNAVAILABLE; test_mode boolean. The handoff timestamp
is null or parseable UTC RFC3339 (Go fractional seconds accepted); NONE requires
null, PREPARED/ISSUED/EXPIRED require non-null. UNAVAILABLE permits either.
Methods is an array of zero or one exact five-field record with payuni_credit,
positive safe version and three-language names of 1..120 Unicode code points,
not whitespace-only and without control characters (matching SQL0015). Methods may
appear only for NOT_STARTED + DRAFT + NONE. Currency/amount must not be inferred
from browser state; this read response is informational, not admission authority.

`PaymentPrepared` has exactly order_id, state PAYMENT_PENDING, currency TWD and
amount_minor as a safe integer100..19999900 divisible by100, matching the current
hosted credit-card admission (not the broader historical view). No form on prepare.

`HostedHandoff` has exactly order_id, disposition, expires_at plus form ONLY
when disposition ISSUED. The only other success is ALREADY_ISSUED with no form.
Expiry must be a valid UTC RFC3339 timestamp; client time is not admission proof.
Form exactly action + fields. Action exactly one of the Go-allowed URLs:
`https://sandbox-api.payuni.com.tw/api/upp` or
`https://api.payuni.com.tw/api/upp`. Fields exactly Version (`2.0`), MerID
(`[A-Za-z0-9_-]{1,64}`), EncryptInfo (lowercase even-length hex,16..24576 chars),
HashInfo (64 uppercase hex chars). No permissive URL prefix or arbitrary fields.
The Go service remains responsible for matching the action to its execution
profile. Invalid response yields sanitized503, nonretryable for handoff.

## Independent gates

- BPT01 BFF positive view, prepare in all three locales and one-shot handoff,
  exact upstream headers/path/body/key, one fetch, cookie bearer never public.
- BPT02 strict input/path/query/body/key/CSRF/context/header denials; absent or
  expired/forged cookie; unknown/null/duplicate prepare fields. Existing routes
  and disabled-root behavior remain unchanged.
- BPT03 hostile upstream payment responses: extra private fields, wrong order,
  wrong enums/money/timestamps/methods, prepared zero/USD/non-whole-TWD/over-limit,
  duplicate JSON, unexpected status, invalid
  content type/encoding/size, malicious form URL/field/length; none leak onward.
- BPT04 every handoff error path nonretryable, sanitized and no-store; network,
  abort/deadline/503/429 do not replay. ALREADY_ISSUED never includes form.
- BPT05 browser request helper exact no-body/no-key exception, invalid variants
  locally rejected, cookie journal/context guard retained, network loss one
  fetch and zero form persistence. Existing purchase/client suites still pass.
- BPT06 root independently reruns Node tests, strict typecheck, production build
  and existing actual Go/PG/race/vet regression. Independent source/evidence
  reviewer must clear P0/P1 before acceptance. UI/browser/provider gates remain
  explicitly NOT_RUN in this increment.

## Required next unit

Extend existing owned OrderDetails for both new purchases and order history:
one explicit Pay action, original-order prepare, one-shot handoff and native
form POST; no form in storage/log/URL. GET-only reload/uncertain/return recovery;
never infer paid from a redirect. Preserve B geometry and three locales. Add
precise form-action policy, actual Next→Go→PG→mock PSP browser gates (desktop,
mobile, concurrent click, response loss, refresh/return). Provider mock results
are not authorization for real sandbox/live, qualification or production.
