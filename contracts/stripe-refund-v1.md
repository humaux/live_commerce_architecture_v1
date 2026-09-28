# Stripe refund v1 — merchant-initiated full/partial refund of a captured Stripe payment

Status: **DRAFT**, 2026-09-28 (unit `design-after-payment`). Not frozen; not reviewed.
Evidence label for this file: DESIGN. Every RF gate below is NOT_RUN. Nothing here authorizes a
real refund: SANDBOX refunds are test-mode objects only, LIVE stays refused (I16/I17).

Extends [stripe-psp-v1](stripe-psp-v1.md) (§0.1/§0.2 binding; §0.2 overrides its older
single-account text), [payment-capture-v1](payment-capture-v1.md) and
[merchant-orders-v1](merchant-orders-v1.md). Architecture §11.5, §12.1, §12.3, §12.4 and invariants
I01, I02, I04, I05, I06, I13, I14, I18, I20, I23, I24 are authoritative; where they conflict with
this file, this file is wrong. Stage B1 (0061, post_river 0012) must be merged and frozen before
any implementation of this file starts (upstream-interface rule, AGENTS.md).

## 0. Owner inputs, decisions and rulings needed

Owner inputs already recorded: card only, automatic capture (stripe-psp D6/Q3); one Stripe account
per store, direct charges, no Connect (§0.1); late payment after closure is a manual-refund
obligation `CLOSURE_CONTRADICTED` + `REVIEW_REQUIRED` that "the later refund contract consumes"
(§0.2, D13). No owner input exists yet on refund permission, dual control or refund reasons.

| # | Decision | Why / risk closed |
| --- | --- | --- |
| RD1 | **Scope:** refund of one `stripe_checkout` attempt that has a `CAPTURED` fact, by amount, full or partial, many partial refunds allowed. PAYUNi refunds, disputes, payouts, balance transactions and reconciliation cases (§12.4) are out. | Smallest loop a merchant needs; PAYUNi exposes only its last refund record (capture-v1), so it needs its own contract. |
| RD2 | **Reuse, no parallel engine.** A refund is one `integration.operations` row (new actor family `PAYMENT_REFUND`, `provider='stripe'`, `action='stripe.refund'`), whose id equals the refund id, UNKNOWN from birth with reconcile leases, exactly like the Stripe checkout op. Observations reuse `payments.provider_observations`; review uses `payments.review_cases`; wake-ups reuse `payments.stripe_signals` + `payment_signal_v1`; reconcile reuses `payment_reconcile_v1` → `payments.apply_capture` dispatcher. New: two tables (§5.2) and one River kind `payment_refund_v1`. | AGENTS.md: no second trading engine. Leases/generations (I14) and the lease-fenced credential loader already exist. |
| RD3 | **Capacity under the order lock (§12.3).** `request_stripe_refund` locks the order row `FOR UPDATE` (the order is the fund bucket; same first lock as capture), then requires `held + amount ≤ captured`, where `captured` = the attempt's `CAPTURED` fact amount and `held` = Σ amount of this attempt's refunds that have **no** `FAILED`, `CANCELED` or `REJECTED` refund fact. REQUESTED, SUBMITTING, PENDING, UNKNOWN and SUCCEEDED all hold capacity. | Pending/unknown refunds occupy capacity and are never released by a timeout, so a second refund cannot be sent against the same money (§12.3). |
| RD4 | **Capacity is released only by provider or local proof:** a Stripe-retrieved `failed`/`canceled` status, a first-send definitive rejection (stripe-psp §5.6/§10 rules), or a LOCAL suppression before any send. Never by elapsed time, 404, 5xx or a missing webhook. | Mirrors stripe-psp D5 for stock. |
| RD5 | **One create Idempotency-Key per refund forever:** `lc:stripe:refund:v1:<refund uuid>`, same key and byte-identical body for every resend, only while `now < requested_at + 20 h` (below Stripe's ≥24 h key pruning, F4). After the window: list refunds of the PaymentIntent and match `metadata.lc_refund`; one match pins; zero matches stays UNKNOWN with review `REFUND_UNRESOLVED` and capacity held. | I06/I20. A new key after an uncertain send could create a second refund. |
| RD6 | **No stock, order or fulfilment side effect.** A refund never writes `inventory.ledger`, reservations, `commercial_state`, `fulfillment_state` or `payment_work_items`. Return, restock, cancellation and reshipment are separate actions (§12.3 last paragraph, I13). | Refund ≠ return ≠ restock. A full refund of an unshipped order leaves the allocation in place until a separate cancel/restock contract exists (ruling R-3). |
| RD7 | **Payment state is derived, never stored:** `REFUNDED` when Σ SUCCEEDED-and-not-FAILED/CANCELED = captured; `PARTIALLY_REFUNDED` when > 0. A PENDING refund does not change `payment_state` (request accepted ≠ buyer refunded, §12.3). | §12.1 state set; I13. |
| RD8 | **Webhook is a wake-up, never financial authority** (stripe-psp D8). Refund facts come only from an authenticated retrieve recorded by the worker and applied in PG. | Same as checkout. |
| RD9 | **External refunds are detected, not trusted.** Before every first send, and on every `charge.refunded` signal, the worker retrieves the PaymentIntent with `expand[]=latest_charge` and records `AmountRefunded`. If Stripe's refunded amount exceeds what our non-released refunds explain (a Dashboard refund), apply records review `REFUND_HISTORY` (existing reason), a first send is suppressed (LOCAL, capacity released), and new requests are refused while that review exists. | Prevents a double refund when the merchant also refunded in the Stripe Dashboard. |
| RD10 | **Separate permission `payments:refund`**, never implied by `orders:read` or owner status alone; not backfilled; not added to onboarding (ruling R-1). | §12.3: refund approval authority is independent of general support permission. |
| RD11 | **`REQUESTED → APPROVED` is collapsed** in v1: the requester holding `payments:refund` is the approver, recorded once in the same transaction. Dual control is a ruling (R-2). | Single-merchant R1; the audit row still names the principal. |
| RD12 | **Reasons:** `requested_by_customer` or `duplicate` only. `fraudulent` is not offered because Stripe then adds the card and email to Radar block lists (F-R2). | A merchant-side mis-click must not block a buyer; ruling R-4. |
| RD13 | **Credential:** the worker uses the attempt's account and the account's **current** API credential version, frozen into the refund row at request time; the account id must equal the attempt's frozen account. It never uses a different account's key. | A rotated-away historical key may be revoked; the money lives on the account, not the key. Ruling R-5 (differs from §0.2's frozen-version rule for checkout). |

Rejected alternatives:
- Refunding through the Stripe Dashboard and importing results: no idempotency, no capacity lock, no audit.
- A mutable `refund.status` updated from webhook payloads: last-message-wins money (§12.2).
- Releasing capacity when an UNKNOWN refund times out: permits double refund (§12.3).
- Refund automatically restocking or cancelling the order: conflates I13 states.
- A new job family per event type, or a separate refund worker process: the payment worker already
  serves the profile queue and Stripe runtime.
- Reusing `payments.facts` for refunds: its PK is `(attempt, kind)`, so it cannot hold several partial
  refunds; §12.4 names `RefundFact` separately.

### 0.1 Integrator ruling needed (nothing below is assumed silently)

- **R-1** Permission name `payments:refund`; whether `create_initial_store` grants it to new store
  creators (default in this draft: **no**, explicit provisioning like `live:manage`).
- **R-2** Dual control (`REQUESTED → APPROVED` by a second principal) or single-actor v1 (draft: single).
- **R-3** Full refund of a CONFIRMED, unshipped order: leave allocation (draft) or add a separate
  merchant cancel-and-release operation in this unit.
- **R-4** Exclude `fraudulent` reason (draft: excluded).
- **R-5** Credential for refunds: current head of the same account (draft) vs the attempt's frozen
  version (§0.2 rule for checkout). The draft freezes the chosen version into the refund row.
- **R-6** Restricted-key permissions: stage-A SP16 records the least RAK set for Checkout; refunds need
  Refunds write and PaymentIntents/Charges read. Registrar qualification must prove it (RF10).
- **R-7** Which refunds are allowed when reviews exist (draft §4.2 table).
- **R-8** Long-tail failure detection: Stripe may fail a `succeeded` card refund up to ~30 days later
  (F-R3). Draft relies on `refund.failed` webhooks plus a merchant refresh; no 30-day poller. Accept?
- **R-9** Whether the merchant detail may show the Stripe refund id (`re_…`) for Dashboard tracing
  (draft: no, consistent with MOR "no PSP reference").
- **R-10** SANDBOX refund probe needs a paid test PaymentIntent. Draft: RF10 creates one with
  `payment_method=pm_card_visa` in test mode from the test harness only (not product code). Allowed?
- **R-11** Post-River file number: draft uses `migrations/post_river/0013_stripe_refund.sql`.
- **R-12** Webhook endpoint subscription: the registrar/runbook must add the four refund event types
  to each per-store endpoint (stripe-psp Q8 owner of endpoint creation still open).

## 1. Stripe facts relied on (WebFetch of docs.stripe.com, retrieved 2026-09-28 UTC)

| # | Fact | Source | Status |
| --- | --- | --- | --- |
| F-R1 | `POST /v1/refunds` needs a Charge or PaymentIntent. Optional `amount` is "a positive integer in the smallest currency unit" and "Can refund only up to the remaining, unrefunded amount of the charge"; omitted = full remaining. Partial refunds can repeat "until the entire charge has been refunded". Refunding an already-refunded charge, or more than is left, raises an error. | [create](https://docs.stripe.com/api/refunds/create) | VERIFIED |
| F-R2 | `reason` ∈ `duplicate`, `fraudulent`, `requested_by_customer`; `fraudulent` adds the card and email to block lists. `refund_application_fee`/`reverse_transfer` are Connect-only; `instructions_email`/`origin` are for non-card or customer-balance refunds. | create | VERIFIED |
| F-R3 | Refund `status` ∈ `pending`, `requires_action`, `succeeded`, `failed`, `canceled`. `failure_reason` ∈ `lost_or_stolen_card`, `expired_or_canceled_card`, `charge_for_pending_refund_disputed`, `insufficient_funds`, `declined`, `merchant_request`, `unknown`. `pending_reason` ∈ `processing`, `insufficient_funds`, `charge_pending`. A refund can fail after being submitted; the bank returns funds, which "can take up to 30 days". Cancellation is a type of failure and carries `failure_reason`. Card refund cancellation is Dashboard-only. | [object](https://docs.stripe.com/api/refunds/object), [refunds guide](https://docs.stripe.com/refunds) | VERIFIED |
| F-R4 | Refunds use available balance; if it is insufficient, card refunds are held `pending` until the balance suffices (other methods fail). `requires_action` applies to methods without native refund support (not card). | refunds guide | VERIFIED |
| F-R5 | Events: `refund.created`, `refund.updated` (incl. ARN reference), `refund.failed` (data.object = refund); `charge.refunded` (data.object = charge, "including partial refunds"). `charge.refund.updated` is deprecated. | [event types](https://docs.stripe.com/api/events/types), refunds guide | VERIFIED |
| F-R6 | `GET /v1/refunds?payment_intent=…` returns refunds newest first, `limit` 1..100, `starting_after`/`ending_before` cursors. `GET /v1/refunds/{id}` retrieves one. | [list](https://docs.stripe.com/api/refunds/list) | VERIFIED |
| F-R7 | Charge `amount_captured`, `amount_refunded` ("can be less than the amount … if a partial refund was issued"), `refunded` (true only when fully refunded), `disputed`. | [charge object](https://docs.stripe.com/api/charges/object) | VERIFIED |
| F-R8 | The customer typically sees a card refund "approximately 5-10 business days later"; early refunds may appear as a reversal instead. | refunds guide | VERIFIED |
| F4 | Idempotency semantics (param compare, cached 500, ≥24 h pruning, 429/most 400/401 before idempotency). | stripe-psp §1 | VERIFIED there, reused |

Not verified; the design treats each as possible: whether `refund.created` fires before the create
response returns; the exact RAK permission names for refunds (R-6); whether a `pending` card refund
for insufficient balance has an expiry (docs imply `insufficient_funds` failure "crossed the pending
refund expiry window" without a duration).

## 2. Flow

```
Merchant admin ─POST /api/admin/.../orders/{id}/refunds (Idempotency-Key)─► BFF ─► API (runtime pool)
  one tx, no I/O: auth(payments:refund) → lock order FOR UPDATE → CAPTURED fact + stripe_sessions pins
  → capacity check (RD3) → stripe_refunds row + op(UNKNOWN, stripe.refund) + audit + command result
  + River payment_refund_v1 (ScheduledAt=now) → COMMIT → 201 {state: REQUESTED}
Worker payment_refund_v1 ─claim refund op─► load_stripe_refund (lease-fenced, API material)
  first send only: GET /v1/payment_intents/{pi}?expand[]=latest_charge → record charge obs
     external refund detected → LOCAL suppressed → REJECTED (capacity released) + REFUND_HISTORY
  mark_stripe_refund_sent (commit) → POST /v1/refunds (key lc:stripe:refund:v1:<refund>)
  → record refund obs (pin re_… set-once) + payment_reconcile_v1 → snooze per status
CaptureWorker payment_reconcile_v1 ─► payments.apply_capture dispatcher (report Object=refund|charge)
  → payments.apply_stripe_refund: lock order → refund facts / reviews (idempotent)
Stripe ─refund.* / charge.refunded─► POST /v1/stripe/webhook/{endpoint_id} → receipt + signal
  (refund_id for refund objects; attempt-level for charge objects) → payment_signal_v1 → retrieve
```

## 3. Wire adapter additions (`internal/integrations/psp/stripe`, stage-A package)

Same package rules as stripe-psp §5.1 (stdlib only, pinned `Stripe-Version`, fixed errors, no
logging, docs URL + retrieval date on every wire constant, redacted `String()`).

- `CreateRefund(ctx, RefundParams) (Refund, CallMeta, error)`; `RefundParams{PaymentIntentID,
  AmountMinor, Currency, Reason, RefundRef, AttemptRef}`. The form body has **exactly** the keys
  `payment_intent`, `amount`, `reason`, `metadata[lc_refund]`, `metadata[lc_attempt]`, sorted and
  deterministic; any other key is `ErrInvalid`. `amount` always present (never "omit for full").
  `Idempotency-Key = RefundIdempotencyKey(refund)`.
- `RetrieveRefund(ctx, id)`, `ListRefunds(ctx, paymentIntentID, startingAfter)` (limit 100; ≤10 pages
  per call, more is `ErrUncertain`, never "not found"), `RetrievePaymentCharge(ctx, pi)` =
  `GET /v1/payment_intents/{pi}?expand[]=latest_charge` returning only the fields in §3.1.
- Classification reuses stripe-psp §5.6 unchanged; create meaning is "definitive only on the first
  send", identical to checkout create.
- Webhook verifier projection (§5.8) adds `data.object.payment_intent`, `data.object.metadata.lc_refund`
  and admits `data.object.object ∈ {checkout.session, refund, charge}`. Nothing else is read.

### 3.1 Observation projections (`payments.provider_observations.report`, ≤2048 bytes, exact keys)

Refund report: `Provider`="stripe", `Version`=1, `Object`="refund", `Via` ∈
{`create`,`retrieve`,`list`,`unsent`,`escalate`}, `AccountID`, `KeyVersion`, `RequestID`, `SendCount`,
`RefundRef` (our uuid), `AttemptRef`, `RefundID`, `Status`, `FailureReason`, `PendingReason`,
`Amount?`, `Currency`, `PaymentIntentID`, `Livemode`, `MetadataRefund`, `MetadataAttempt`,
`ErrorClass` ("" | "rejected"), `ErrorCode`, `HTTPStatus`, `ListMatchCount?`, `LocalReason`.

Charge report: `Provider`="stripe", `Version`=1, `Object`="charge", `Via`="retrieve", `AccountID`,
`KeyVersion`, `RequestID`, `PaymentIntentID`, `ChargeID`, `Currency`, `AmountCaptured?`,
`AmountRefunded?`, `Refunded`, `Disputed`, `Livemode`, `LocalReason`.

Never stored: destination_details (ARN), card data, `receipt_url`, billing details, emails, error
messages, raw JSON. `provider_observations.first_generation` is the refund op's generation for
refund reports and the attempt op's generation for charge reports recorded by SignalWorker.

## 4. Persistence: `migrations/0062_stripe_refund.sql` + `migrations/post_river/0013_stripe_refund.sql`

### 4.1 Widened constraints (0062)

- `identity.store_grants` permission CHECK: re-derive the latest list (0033) and add `payments:refund`.
- `integration.operations.operation_actor_family`: re-derive from `pg_get_constraintdef` (keeping
  MERCHANT, BUYER_PAYMENT_QUERY incl. the 0061 Stripe disjunct, MEDIA_ATTEMPT) and add
  `(actor_kind='PAYMENT_REFUND' AND principal_id IS NOT NULL AND media_attempt_id IS NULL
  AND payment_attempt_id IS NOT NULL AND payment_attempt_id<>id AND buyer_owner_id IS NOT NULL
  AND buyer_session_id IS NOT NULL AND provider='stripe' AND action='stripe.refund'
  AND purpose='transactional' AND state NOT IN ('READY','DISPATCHING','BLOCKED_POLICY','STALE_BINDING')
  AND lease_mode<>'dispatch')`. Buyer owner/session are copied from the attempt so the existing
  composite `operation_payment_attempt_fk` is enforced (MATCH SIMPLE would skip it with NULLs).
- `payments.review_cases.reason` adds `REFUND_UNRESOLVED`, `REFUND_AMOUNT_MISMATCH`,
  `REFUND_CONFLICTING` (`REFUND_HISTORY` already exists and is reused for external refunds).
- `payments.stripe_signals`: add `refund_id uuid NULL REFERENCES payments.stripe_refunds(id)` and
  `source` value `MERCHANT_REFRESH`; CHECK `(source='MERCHANT_REFRESH') <= (refund_id IS NOT NULL)`.
  A signal with `refund_id` carries the **refund op id** as its job `operation_id`; one without keeps
  the checkout meaning.
- `payments.stripe_webhook_receipts`: `object_type` admits `refund`, `charge`; `reason` adds
  `unknown_refund`, `unknown_charge`; add nullable immutable `refund_id`.
- `payments.provider_observations`: no DDL change; the refund/charge report shapes are validated by
  the new record definers (the checkout record function keeps rejecting them).

### 4.2 New tables

```sql
CREATE TABLE payments.stripe_refunds (
 tenant_id uuid NOT NULL, store_id uuid NOT NULL, id uuid PRIMARY KEY,           -- = operations.id
 attempt_id uuid NOT NULL, order_id uuid NOT NULL, owner_id uuid NOT NULL,
 principal_id uuid NOT NULL,                                                     -- requester = approver (RD11)
 environment text NOT NULL CHECK(environment='SANDBOX'),                          -- LIVE refused in v1
 account_id text NOT NULL CHECK(account_id ~ '^acct_[A-Za-z0-9]{1,59}$'),
 credential_version bigint NOT NULL CHECK(credential_version>0),                  -- R-5
 payment_intent_id text NOT NULL CHECK(payment_intent_id ~ '^[A-Za-z0-9_]{1,255}$'),
 currency text NOT NULL CHECK(currency ~ '^[A-Z]{3}$'),
 amount_minor bigint NOT NULL CHECK(amount_minor BETWEEN 1 AND 999999999999),
 reason text NOT NULL CHECK(reason IN ('requested_by_customer','duplicate')),
 create_params jsonb NOT NULL CHECK(jsonb_typeof(create_params)='object' AND octet_length(create_params::text)<=2048),
 requested_at timestamptz NOT NULL, resend_until timestamptz NOT NULL,
 first_sent_at timestamptz, last_sent_at timestamptz,
 send_count integer NOT NULL DEFAULT 0 CHECK(send_count BETWEEN 0 AND 200),
 body_sha256 bytea CHECK(body_sha256 IS NULL OR octet_length(body_sha256)=32),
 suppressed_at timestamptz,
 stripe_refund_id text UNIQUE CHECK(stripe_refund_id ~ '^[A-Za-z0-9_]{1,255}$'), pinned_at timestamptz,
 refresh_count integer NOT NULL DEFAULT 0 CHECK(refresh_count BETWEEN 0 AND 30), last_refresh_at timestamptz,
 signal_count integer NOT NULL DEFAULT 0 CHECK(signal_count BETWEEN 0 AND 64),
 CHECK(resend_until=requested_at+interval '20 hours'),
 CHECK((stripe_refund_id IS NULL)=(pinned_at IS NULL)),
 CHECK((first_sent_at IS NULL)=(send_count=0) AND (first_sent_at IS NULL)=(body_sha256 IS NULL)
   AND (first_sent_at IS NULL)=(last_sent_at IS NULL)),
 CHECK(suppressed_at IS NULL OR first_sent_at IS NULL),
 CHECK(payments.stripe_refund_amount_ok(currency,amount_minor)),                  -- step rule below
 FOREIGN KEY(tenant_id,store_id,owner_id,attempt_id) REFERENCES checkout.payment_attempts(tenant_id,store_id,owner_id,id),
 FOREIGN KEY(tenant_id,store_id,id) REFERENCES integration.operations(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,principal_id) REFERENCES identity.memberships(tenant_id,principal_id)
);
CREATE TABLE payments.refund_facts (
 tenant_id uuid NOT NULL, store_id uuid NOT NULL, refund_id uuid NOT NULL REFERENCES payments.stripe_refunds(id),
 attempt_id uuid NOT NULL,
 kind text NOT NULL CHECK(kind IN ('SUCCEEDED','FAILED','CANCELED','REJECTED')),
 amount_minor bigint NOT NULL CHECK(amount_minor>=0), currency text NOT NULL CHECK(currency ~ '^[A-Z]{3}$'),
 stripe_refund_id text CHECK(stripe_refund_id ~ '^[A-Za-z0-9_]{1,255}$'),
 failure_reason text CHECK(failure_reason IN ('lost_or_stolen_card','expired_or_canceled_card',
   'charge_for_pending_refund_disputed','insufficient_funds','declined','merchant_request','unknown',
   'first_send_rejected','external_refund_detected')),
 source_report_hash bytea NOT NULL CHECK(octet_length(source_report_hash)=32),
 received_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,store_id,refund_id,kind),
 CHECK((kind='REJECTED')=(stripe_refund_id IS NULL)),
 FOREIGN KEY(tenant_id,store_id,attempt_id,source_report_hash)
  REFERENCES payments.provider_observations(tenant_id,store_id,attempt_id,report_hash)
);
```

- **Amount step:** TWD refunds must be multiples of 100 minor units (whole NT$, stripe-psp §4); other
  admitted currencies step 1. `payments.stripe_refund_amount_ok(currency, amount)` is IMMUTABLE and
  uses the §0.2 currency table without its minimum (no refund minimum is documented; Stripe's own 400
  on the first send is definitive). Go and SQL parity is RF01.
- **Set-once trigger** `payments.guard_stripe_refund()`: frozen columns never change; markers and
  pins only NULL→value; counters monotone; `stripe_refund_id` also becomes the op `provider_reference`.
- **Fact trigger** `payments.guard_refund_fact()`: `REJECTED` excludes every other kind and requires no
  pin; `SUCCEEDED`/`FAILED`/`CANCELED` require the pinned id; `FAILED` or `CANCELED` may follow
  `SUCCEEDED` (late failure, F-R3); `SUCCEEDED` after `FAILED`/`CANCELED` is refused (→ review
  `REFUND_CONFLICTING` instead); `SUCCEEDED.amount_minor` must equal the refund's amount (I05; a
  mismatch goes to review `REFUND_AMOUNT_MISMATCH`, no fact).
- Append-only facts; refunds updatable only through the guarded columns. Bounded: at most 20 refunds
  per attempt (I23), enforced in the request function.
- **Roles/RLS:** both tables FORCE RLS, PUBLIC revoked, no direct runtime login grants.
  `commerce_checkout_writer` owns request/apply definers: SELECT, INSERT on both tables,
  `UPDATE(refresh_count,last_refresh_at,signal_count)` on refunds. `commerce_integration_writer`: SELECT
  and `UPDATE(first_sent_at,last_sent_at,send_count,body_sha256,suppressed_at,stripe_refund_id,
  pinned_at,signal_count)` on refunds. `commerce_auth` gets column SELECT for the merchant read (§7).
  `commerce_checkout_runtime` gets SELECT on neither table; the buyer view reads through the existing
  `hosted_payment_view_v2` definer successor (§7.2).

### 4.3 Which captured payments may be refunded (R-7)

| Condition on the attempt | Allowed? |
| --- | --- |
| provider `stripe`, `CAPTURED` fact, pinned `payment_intent_id`, no review | yes, full or partial |
| review `PROVIDER_PRESENTMENT_DRIFT` only | yes (money matched, I05) |
| `CLOSURE_CONTRADICTED` / `PAID_ALLOCATION_FAILED` (late payment obligation, D13) | yes, **full remaining only**; this consumes the §0.2 manual-refund obligation. The review is not cleared; the read model shows `refunded_minor` = captured. |
| review `PROVIDER_AMOUNT_MISMATCH`, `PROVIDER_IDENTITY_MISMATCH`, `PROVIDER_SESSION_DUPLICATE`, `CONFLICTING_REPORT`, `REFUND_HISTORY`, `REFUND_UNRESOLVED`, `REFUND_CONFLICTING` | no (`refund_blocked_review`); operator/Dashboard path, then manual resolution (NOT_IMPLEMENTED) |
| no `CAPTURED` fact, PAYUNi, LIVE environment | no |

### 4.4 SQL entry points

| Function | Phase | Owner | EXECUTE | Contract |
| --- | --- | --- | --- | --- |
| `payments.request_stripe_refund(hash bytea, store uuid, order uuid, key text, request_hash bytea, amount bigint, reason text, expected_refundable bigint, refund uuid, job bigint) RETURNS jsonb` | post_river | checkout_writer | `commerce_runtime` | Merchant token auth with `payments:refund` using the 0027 fresh-final-auth pattern (READ COMMITTED, `clock_timestamp()` expiry; PT401/403/404 distinctions). Replays `ops.command_results` (`operation='payments.refund.request'`, I02). Locks order → reads attempt, CAPTURED fact, reviews, `stripe_sessions.payment_intent_id`, account/current credential. Requires §4.3, step rule, ≤20 refunds, `expected_refundable = captured − held` (else PT409 `refundable_changed`), `held + amount ≤ captured` (else PT422 `exceeds_refundable`). Verifies the exact `payment_refund_v1` job, inserts op (UNKNOWN, reconcile), refund row, operation event, `ops.audit_events` `payments.refund_requested`, command result. Returns `{refund_id,state,amount_minor,currency,refundable_minor}`. No provider I/O. |
| `payments.request_stripe_refund_refresh(hash, store, order, refund, signal uuid, job bigint) RETURNS jsonb` | post_river | checkout_writer | runtime | `payments:refund`; throttled `now ≥ last_refresh_at+10 s`, ≤30; not after a terminal fact other than SUCCEEDED (late-failure check allowed). Inserts `stripe_signals(source MERCHANT_REFRESH, refund_id)`; `{scheduled:false}` means roll back the InsertTx. |
| `integration.load_stripe_refund(uuid,bigint,bytea,text) RETURNS TABLE(...)` | 0062 | integration_writer | worker | Lease/token/profile fenced on the refund op (as `load_stripe_credential`); returns frozen refund, attempt scope, account, the frozen credential version's `key_id/nonce/ciphertext`, markers, pins, latest status, terminal flags, DB now. |
| `integration.mark_stripe_refund_sent(uuid,bigint,bytea,text,bytea) RETURNS text` | 0062 | integration_writer | worker | `SEND` (first send, not suppressed, no REJECTED fact), `RESEND` (same body hash, `now < resend_until`), `CLOSED`. Commits before the POST. |
| `integration.record_stripe_refund_observation(uuid,bigint,bytea,text,jsonb,bigint) RETURNS void` | post_river | integration_writer | worker | Exact §3.1 refund keys; identity (`MetadataRefund`=`RefundRef`=refund, `MetadataAttempt`=attempt, `PaymentIntentID`, `AccountID`, `Livemode=false`, `Currency`) or it raises and the op finishes UNKNOWN `stripe_refund_mismatch`. Pins `stripe_refund_id` set-once; LOCAL `unsent` sets `suppressed_at`. Hash-dedup; verifies the `payment_reconcile_v1` job whose `operation_id` = the attempt; lease-fenced Complete UNKNOWN. |
| `integration.record_stripe_charge_observation(uuid,bigint,bytea,text,jsonb,bigint) RETURNS void` | post_river | integration_writer | worker | Charge report under either the refund op lease (pre-send) or the checkout op lease (SignalWorker). Same validation style. |
| `integration.finish_stripe_refund(uuid,bigint,bytea,text,text) RETURNS void` | 0062 | integration_writer | worker | Fixed codes: `stripe_refund_uncertain`, `stripe_retrieve_failed`, `stripe_rate_limited`, `stripe_timeout`, `stripe_panic`, `stripe_record_failed`, `stripe_refund_mismatch`, `stripe_idempotency_alarm`, `stripe_unavailable`, `stripe_budget_exhausted`, `stripe_refund_terminal`. Always UNKNOWN. |
| `payments.apply_capture(uuid,bytea)` dispatcher | 0062 | checkout_writer | worker (existing) | `CREATE OR REPLACE` of the 0061 dispatcher: additionally routes `report->>'Object'='refund'` to `payments.apply_stripe_refund` and `'charge'` to `payments.apply_stripe_charge`; the checkout path is byte-identical (RF12 hashes it). |
| `payments.apply_stripe_refund(uuid,bytea)`, `payments.apply_stripe_charge(uuid,bytea)` | 0062 | checkout_writer | none | §6. |

**Post-River (0013):** `guard_payment_job_family` admits `payment_refund_v1` with args exactly
`{operation_id, version:1}`; `payment_job_queue` linkage `stripe_refunds.id::text=args.operation_id AND
operations.job_id=j.id`, queue from the attempt's `execution_profile`; `payment_signal_v1` linkage
accepts `s.refund_id::text=args.operation_id` when `refund_id` is set; kind lists in
`route_payment_queue_v1`, `payment_queue_ready` and `reject_legacy_family_job` include the new kind.
Old binaries still route correctly.

## 5. State machine

Derived from refund columns and facts, never one stored enum (like stripe-psp §7):

| State | Entered by | Exits |
| --- | --- | --- |
| REQUESTED (= §12.3 REQUESTED+APPROVED, RD11) | request tx | pre-send external refund → REJECTED; `mark_sent=SEND` → SUBMITTING |
| SUBMITTING (unknown) | send committed | 200 → pinned; first-send 4xx/401/403 → REJECTED; uncertain → same-key RESEND until `resend_until`, then list: match → pinned; none → UNKNOWN |
| PENDING (§12.3 ACKNOWLEDGED/PENDING) | pinned, status `pending`/`requires_action` | `succeeded` → SUCCEEDED; `failed` → FAILED; `canceled` → CANCELED; `requires_action` > 60 min → review `REFUND_CONFLICTING` (not expected for card) |
| SUCCEEDED | SUCCEEDED fact | late `failed`/`canceled` (webhook or refresh) → FAILED/CANCELED |
| FAILED / CANCELED | fact | terminal; capacity released |
| REJECTED | REJECTED fact (never reached Stripe, or definitive rejection) | terminal; capacity released |
| UNKNOWN | no pin after the resend window and list = 0, or budget exhausted before a terminal status | manual only (review `REFUND_UNRESOLVED`); capacity held |

**Effect table:**

| Event | Capacity | payment_state | Order / stock / fulfilment / work item |
| --- | --- | --- | --- |
| Request | held | unchanged | unchanged |
| SUCCEEDED | held (consumed) | PARTIALLY_REFUNDED or REFUNDED | unchanged |
| FAILED/CANCELED after SUCCEEDED | released | reverts | unchanged |
| REJECTED / FAILED / CANCELED before success | released | unchanged | unchanged |
| UNKNOWN / timeout / 5xx / missing webhook | **held** | unchanged | unchanged |
| External refund seen on the charge | n/a | REVIEW_REQUIRED (sticky `REFUND_HISTORY`) | unchanged; new refunds refused |

## 6. Worker and apply rules

**RefundWorker step** (inside `cmd/payment-worker`, same `StripeRuntime`, `payment_refund_v1` on the
profile queue; missing runtime → transient `stripe_unavailable` before Claim, as stripe-psp §8):

```
claim refund op → load_stripe_refund → build client from leased material → VerifyAccount(account)
if terminal fact (not SUCCEEDED): finish(stripe_refund_terminal); complete
if SUCCEEDED fact: finish(stripe_refund_terminal); complete   (late failure arrives via signal)
if not pinned:
  if never sent: RetrievePaymentCharge → record charge obs (apply may suppress)
                 if suppressed → record LOCAL unsent; snooze 5 s
                 else mark_sent=SEND → POST /v1/refunds
  else if now < resend_until: mark_sent=RESEND → POST (same key, same bytes)
  else: ListRefunds(pi) → match metadata.lc_refund: 1 → record via=list (pin); 0 → record via=list
        (ListMatchCount=0) → apply records REFUND_UNRESOLVED
  200 → record via=create (pin); ErrRejected/ErrAuthentication on first send → record rejected
  uncertain/409/429/idempotency_error → finish(code); snooze 5,15,45,120 s (cap 120)
else: RetrieveRefund → record via=retrieve
snooze: pending → 60 s for 1 h, then 15 min until op MaxAge 24 h, then finish(stripe_budget_exhausted)
        (PENDING kept; merchant refresh/webhook re-wake)
```

All HTTP calls in one claim share the existing `CallTimeout` budget (`call + 2*DB + 1 s < lease`).

**SignalWorker:** a signal with `refund_id` claims the refund op and retrieves that refund; an
attempt-level signal whose receipt `object_type='charge'` claims the checkout op (only after a terminal
CAPTURED fact) and records a charge report. Staleness (10 min) and busy rules unchanged.

**`payments.apply_stripe_refund(attempt, hash)`** — lock order → refund row; never lock operations:
1. Identity guard (defensive; record already refused) → review `PROVIDER_IDENTITY_MISMATCH`, return.
2. `Via=create ∧ rejected ∧ SendCount=1 ∧ no pin` → REJECTED fact (`first_send_rejected`).
   LOCAL `unsent` with `suppressed_at` and never sent → REJECTED (`external_refund_detected`).
3. `Via=list ∧ ListMatchCount=0 ∧ now ≥ resend_until` → review `REFUND_UNRESOLVED`; no fact.
4. Money: `Currency ≠ refund.currency` or `Amount ≠ refund.amount` → review `REFUND_AMOUNT_MISMATCH`; no fact (I05).
5. `succeeded` → SUCCEEDED fact (refuse if FAILED/CANCELED exists → `REFUND_CONFLICTING`);
   `failed` → FAILED fact with `FailureReason`; `canceled` → CANCELED fact.
6. Invariant re-check under the lock: Σ SUCCEEDED-not-failed ≤ captured, else review `REFUND_CONFLICTING`.
7. Anything else (`pending`, `requires_action`) → no-op.

**`payments.apply_stripe_charge(attempt, hash)`** — lock order: if `AmountRefunded` > Σ amount of this
attempt's refunds that are PENDING/SUCCEEDED/SUBMITTING/UNKNOWN (i.e. Stripe knows about more refund
money than we sent or may have sent) → review `REFUND_HISTORY`; if the same report is the pre-send
check of a REQUESTED refund, mark it for LOCAL suppression. `AmountCaptured ≠ captured fact` →
`CONFLICTING_REPORT`. `Disputed=true` → review `CONFLICTING_REPORT` (disputes are out of scope; refunds
are then blocked). Every write is idempotent by PK; replay in any order converges.

## 7. HTTP and UI surfaces

### 7.1 Merchant admin (Go private API; the admin BFF mirrors it under `/api/admin/`)

| Method/path | Input | Success |
| --- | --- | --- |
| POST `/v1/admin/stores/{store_id}/orders/{order_id}/refunds` | `Idempotency-Key` (required); body exactly `{amount_minor, reason, expected_refundable_minor}`; no query | 201 `{refund_id, state, amount_minor, currency, refundable_minor}` after COMMIT. 409 key conflict / `refundable_changed`; 422 `exceeds_refundable`, `amount_step`, `not_refundable`, `refund_blocked_review`, `refund_limit`; 403 missing `payments:refund`; 404 missing/other-store (indistinguishable). Currency is never accepted from the client. |
| GET `/v1/admin/stores/{store_id}/orders/{order_id}/refunds` | none | `{captured_minor, refunded_minor, pending_minor, refundable_minor, currency, items:[{refund_id, amount_minor, reason, state, requested_at, updated_at, failure_reason?}]}` (≤20). Requires `orders:read`. |
| POST `.../refunds/{refund_id}/refresh` | no body, no key | `{refund_id, scheduled}`; `payments:refund`; throttled in SQL. |

Existing rules apply: 64 KiB JSON, unknown/duplicate fields rejected, private/no-store, fixed error
codes, no Stripe ids, no raw provider strings. Merchant-orders summary/detail (`identity.read_merchant_orders`
replaced in 0062, same single-snapshot and fresh-final-auth rules): `payment_state` adds
`PARTIALLY_REFUNDED`, `REFUNDED` with precedence `NOT_STARTED > REVIEW_REQUIRED > REFUNDED >
PARTIALLY_REFUNDED > CAPTURED > AUTHORIZED > PENDING`; summary adds `refunded_minor`, `refund_pending_minor`.

**Admin UI** (`merchant-orders-ui` amendment, ui_worker): inside the approved C inline detail row, a
"Refund" section shows captured / refunded / in progress / refundable amounts and the refund list with
state badges (REQUESTED, SUBMITTING, PENDING shown as "處理中/处理中/Processing"; SUCCEEDED; FAILED with
reason; UNKNOWN as "Needs support"). The action appears only with `payments:refund` and a refundable
amount; the dialog takes amount (default = refundable, formatted via the existing currency formatter,
TWD whole dollars) and reason, restates amount + currency for confirmation, and sends one idempotency
key per dialog open (kept for retries of that submission). No optimistic success; state comes from GET.

### 7.2 Buyer

`GET /v1/buyer/orders/{id}/payment` (via the `hosted_payment_view_v2` successor): `payment_state` adds
`PARTIALLY_REFUNDED`/`REFUNDED`; new `refund: null | {refunded_minor, pending_minor}` with no ids or
reasons. The storefront order page shows "Refund processing" when `pending_minor>0`, and "Refunded X —
your bank may take 5–10 business days" (F-R8) only for succeeded amounts. No buyer refund request
route in v1.

### 7.3 Webhook admission deltas (stripe-psp §0.2 prepare)

Subscribed types add `refund.created`, `refund.updated`, `refund.failed`, `charge.refunded`.
Mapping is scoped to the endpoint's account: `refund` object → by pinned `stripe_refund_id`, else by
`metadata.lc_refund` = a refund of an attempt on that account whose `metadata.lc_attempt` also matches;
none → `IGNORED unknown_refund` (a Dashboard refund; `charge.refunded` covers it). `charge` object →
by `payment_intent` = a pinned `stripe_sessions.payment_intent_id` on that account with a CAPTURED fact;
none → `IGNORED unknown_charge`. Signal caps (`signal_count ≤ 64`) apply per refund and per attempt.
Dedupe, ACK-after-commit and deferred reciprocity triggers are unchanged.

## 8. Idempotency and uniqueness

| Scope | Key |
| --- | --- |
| Merchant request | `ops.command_results(tenant,store,'payments.refund.request',Idempotency-Key)` + `expected_refundable_minor` CAS |
| Stripe create | `lc:stripe:refund:v1:<refund uuid>`, every send, only before `requested_at+20h` |
| Stripe retrieve/list | GET, no key |
| Op | `semantic_key = payment.stripe.refund:<refund uuid>`; `UNIQUE(tenant,store,semantic_key)` |
| Facts | `(refund, kind)`; `stripe_refunds.stripe_refund_id` UNIQUE; op `provider_reference` |
| Webhook | stripe-psp §0.2 `(endpoint_id,event_id)` |

## 9. Test gates (tiers as stripe-psp §14: UNIT, REAL_PG, MOCK, HTTP_PG, SANDBOX, BROWSER)

| Gate | Test | Tier | Required |
| --- | --- | --- | --- |
| RF01 | `TestStripeRF01Wire` | UNIT | Golden refund body bytes, exact key set, forbidden keys (`charge`, `fraudulent`, `reverse_transfer`, `refund_application_fee`, `instructions_email`, `origin`) → `ErrInvalid`; resend byte-identical; key format; §5.6 classification for create/retrieve/list/PI; list 10-page cap → uncertain; §3.1 projections exact keys, ≤2048 bytes, no ARN/card/receipt URL; amount-step Go↔SQL parity (TWD %100). |
| RF02 | `TestStripeRF02WebhookProjection` + Node vectors | UNIT | refund/charge objects project `payment_intent`, `lc_refund`; other objects unchanged; strict JSON rules unchanged. |
| RF03 | `TestStripeRF03Schema` | REAL_PG | Fresh + populated upgrade from 0061; family CHECK keeps every prior family; new CHECKs negative; set-once and fact triggers (REJECTED exclusivity, SUCCEEDED→FAILED allowed, FAILED→SUCCEEDED refused, amount equality); FORCE RLS, column-privilege matrix, definer owner/`proconfig`/ACL, PUBLIC revoked; dispatcher checkout body unchanged; post-River routing/guard/readiness include `payment_refund_v1`; LIVE environment rejected; permission added without backfill or onboarding grant. |
| RF04 | `TestStripeRF04Request` | REAL_PG | Atomic request (op, refund, event, audit, command result, job); replay same key/body; key with different body 409; `expected_refundable` 409; two concurrent requests whose sum exceeds captured → exactly one wins (real two-transaction witness); pending/unknown refunds hold capacity; §4.3 matrix incl. late-payment full-only; PAYUNi/no-capture/other-store/revoked permission/expired session; 21st refund refused; **zero** changes to ledger, balances, reservations, order states, work items. |
| RF05 | `TestStripeRF05HappyMock` | MOCK | Real worker + `stripetest` fake: full refund → SUCCEEDED → REFUNDED; two partials → PARTIALLY_REFUNDED then REFUNDED; exactly one create key per refund; pre-send charge read occurs; stock and order unchanged; op ends `stripe_refund_terminal`. |
| RF06 | `TestStripeRF06Unknown` | MOCK | Drop-after-execute → same-key replay pins the same refund; cached 500 → resend → at `resend_until` list match pins / no match → UNKNOWN + `REFUND_UNRESOLVED`, capacity held, no second POST ever; first-send 400 → REJECTED + released; 400 after uncertain send → no release; 409/429 backoff; `idempotency_error` alarm; fake asserts ≤1 distinct key per refund; clock aging disclosed. |
| RF07 | `TestStripeRF07Lifecycle` | MOCK | pending (insufficient balance) → succeeded; succeeded → later `refund.failed` → FAILED fact, capacity released, payment_state reverts; canceled; failure reasons mapped; amount/currency drift → review, no fact; replay in any order converges. |
| RF08 | `TestStripeRF08Webhook` | HTTP_PG | Four new types admitted and deduped; unknown refund/charge ignored; forged `lc_refund` of another account/store → ignored; Dashboard refund (charge `amount_refunded` > ours) → `REFUND_HISTORY`, new requests 422, a REQUESTED refund suppressed before send; ACK only after commit; no body/signature/ARN in logs or rows. |
| RF09 | `TestStripeRF09AdminHTTP` + BFF Node tests | HTTP_PG + Node | Exact request/response keys and codes; permission matrix (`orders:read` alone cannot refund); strict body/query/method rules; refresh throttle; merchant read `payment_state` precedence and new fields; buyer `refund` projection has no ids; PAYUNi responses byte-identical to golden files. |
| RF10 | `TestStripeRF10Sandbox` | **SANDBOX** | Refuses unless test key, `livemode=false`, registered sandbox account; (R-10) a test PaymentIntent with `pm_card_visa`; partial then full refund via the real adapter; same-key replay returns the same `re_…` with `Idempotent-Replayed`; changed params same key → `idempotency_error`; over-refund → 400; list by PI finds both; records the least RAK permission set. Test-mode only; SKIP is NOT_RUN. |
| RF11 | `refund-browser.spec.ts` | **BROWSER** | After SP18 (4242 payment): merchant with `payments:refund` refunds partially then fully in the admin page; buyer order page shows processing → refunded; merchant without permission sees no action; 3 locales, desktop + mobile Chromium; screenshots hashed. |
| RF12 | `TestStripeRF12Guards` + root | REVIEW + regression | D9/PROCESS §5 comments; only `psp/stripe` dials Stripe; logs carry no key/whsec/body/ARN; full `go test -race ./...`, `go vet ./...`, `python3 scripts/check_packet.py`; every SP and prior gate unchanged; independent test_worker + security_reviewer verdicts. |

A gate that cannot fail is not a gate: each records one red run before its green run (PROCESS §2.4).

## 10. Ownership (PROCESS §2, stripe-psp §15 roles)

| Artifact | Owner |
| --- | --- |
| adapter additions + RF01/RF02 unit tests | integration_worker |
| `stripetest` refund endpoints, `tests/foundation/stripe_refund_*_test.go`, vectors, `refund-browser.spec.ts` | independent test_worker |
| `internal/payments/stripe_refund*.go`, SignalWorker branch, `internal/merchantorders` read extension | commerce_worker |
| `0062_stripe_refund.sql`, `post_river/0013_stripe_refund.sql`, `jobqueue`, admin HTTP mount, `core-openapi.json`, `tasks.json`, `sources.json` rows F-R1..F-R8, runbook webhook events | integrator |
| admin refund section, buyer refund display, BFF routes | ui_worker |

Sequence: B1 frozen and merged → freeze this file → 0062/post_river + RF03 (integrator) ‖ adapter RF01–02
→ commerce_worker RF04–RF08 → HTTP RF09 → SANDBOX RF10 → UI + RF11 → RF12 verdicts.

## 11. Known limits / NOT_RUN

- Evidence: DESIGN only; RF01–RF12 NOT_RUN. SANDBOX/BROWSER NOT_RUN without test keys; LIVE NOT_APPLICABLE.
- Card refunds only; no `requires_action` flow, no refund cancellation (Dashboard-only for cards, F-R3).
- No disputes/chargebacks, no balance-transaction or payout reconciliation (§12.4), no fee accounting.
- Late refund failure up to ~30 days relies on webhooks and merchant refresh (R-8).
- UNKNOWN refunds and review cases need manual operator resolution; resolution UI NOT_IMPLEMENTED.
- A full refund does not cancel the order or release stock (R-3); no return/RMA (§13.4).
- No buyer notification message; the buyer sees state only on the order page.
- Stripe availability of balance: pending-for-balance refunds may stay PENDING past the 24 h poll budget.

## Integrator rulings (2026-09-29, binding; supersede the defaults in §0.1)

- R-1 Permission `payments:refund`. The store creator receives it for their own store through the
  owner-provisioning unit (0065, see below); no backfill of other principals.
- R-2 Single actor in v1 (no dual control). R-3 Cancel-and-release of a refunded unshipped order
  is a separate R1 follow-up unit, not part of this contract.
- R-4 The `fraudulent` reason is excluded. R-5 Current key version, frozen into the refund row.
- R-6 Registrar qualification must prove Refunds write + PaymentIntents/Charges read for a
  restricted key (extends SP21; RF gate owns the refund half).
- R-7 Draft §4.3 accepted. R-8 Webhook + merchant refresh only; no 30-day poller (documented limit).
- R-9 **Changed:** the merchant sees the Stripe refund id (`re_…`) for reconciliation with the
  Stripe Dashboard. It is not a secret; buyers never see it.
- R-10, R-11 (post-River 0013), R-12 accepted.
- Owner provisioning (applies to refund, fulfilment and live features): a new migration
  `0065_owner_provisioning.sql` makes `identity.create_initial_store` grant the creator
  `live:read`, `live:manage`, `payments:refund`, `fulfillment:write`, `orders:export` on the new
  store (in addition to today's set). Reason: 0033 deferred this "to an explicit provisioning
  decision"; without it an onboarded merchant cannot use live selling, refunds or shipping at all.
  No backfill for existing principals (no production data exists).
