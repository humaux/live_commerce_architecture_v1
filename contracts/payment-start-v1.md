# Buyer payment start v1

Status: PASS_BOUNDED_INTERNAL_PROVIDER_MOCK, 2026-09-24, 246 top-level PG/race/vet
PASS, 0 FAIL/SKIP. See [acceptance](../docs/implementation/2026-09-24-payment-start-acceptance.md).
Internal transaction, not a
public checkout, payment success, hosted form release or provider qualification.

## Boundary

- Reuse the existing checkout pool, buyer capability, private receipt, River and
  integration operation ledger. No merchant principal synthesized for buyers.
- `checkout.NewPaymentStarter(ctx, pool, jobs, profile)` accepts server-owned
  `PROVIDER_MOCK`, `SANDBOX` or `LIVE`; invalid profiles fail construction. The
  normal checkout constructor does not start payments. No HTTP route is added.
- `PaymentStarter.StartPayment(ctx, token, storeID, key, PaymentInput)` accepts
  only `order_id`, `method_code`, `method_version`. This increment admits only
  `payuni_credit`; installment and delayed-payment expiry policies remain closed.
- `PaymentResult`: order_id, attempt_id, operation_id, job_id, generation,
  merchant_trade_no, currency, amount_minor, state. State is PAYMENT_PENDING,
  never PAID. The order's stored total is authoritative (TWD minor units divisible
  by 100, 100..19999900). No client amount, account, scope, URL or credentials.

## Qualification and transaction

- A method revision references scoped account qualification evidence, exact
  account/environment/method/credential version and validity period. The only
  mutable evidence field is revocation time. No application role can issue it;
  no real issuer is implemented here. Test owner seeds explicitly MOCK evidence
  in disposable PG, never merchant HTTP. SANDBOX/LIVE require matching REAL
  evidence and reject MOCK; current production methods remain disabled.
- Go takes `checkout.payment.start|tenant|store|owner|key` advisory lock before
  checking the existing private receipt. Buyer.WithScope must authenticate before
  any receipt read; reauthenticate after waits on both new and replay paths.
  Same key/request returns original IDs,
  without another job or effect; changed profile/input conflicts. Fresh sessions
  of the same owner may replay, revoked sessions cannot.
- Go inserts River `payment_query_v1` args exactly `{operation_id,version:1}` in
  the same transaction, then calls `checkout.start_payment(hash,store,key,
  request_hash,order,method,method_version,profile,attempt,job_id)` returning JSON.
- SQL resolves the buyer again, repeats the command lock, rejects an existing
  receipt (Go handles replay before job insertion), locks order -> reservation -> market
  -> payment head -> account -> binding -> qualification. It checks all scopes,
  current versions, enabled/visible, expiry and final capability after waits.
  An inactive market blocks new payment; price remains the existing order's total.
- The transaction freezes the method/qualification/account/credential/binding,
  total and unique merchant trade number; moves DRAFT/HELD to
  AWAITING_PAYMENT/PAYMENT_PENDING; advances both generations; writes one attempt,
  one query-only UNKNOWN operation, event, receipt and River job. No network I/O.
- Old expiry jobs must never release PAYMENT_PENDING stock. Different keys for
  the same order cannot create another attempt. Exceptions roll back all facts.

## Query-only operation family

- `BUYER_PAYMENT_QUERY` has NULL merchant principal and exact scoped attempt,
  buyer owner and originating payment session. Initial generation 1, UNKNOWN,
  no lease, action `payuni.query`, purpose transactional. READY/dispatch forbidden.
- Generic merchant Get/Dispatcher excludes this family. Existing Claim/Complete
  fencing is reused: query claims reconcile only, and current binding disable or
  revision change does not prevent querying its immutable historical target.
- This increment queues a durable query intent but does not expose a form.
  The following [query increment](payment-query-v1.md) loads historical
  credentials, executes that job and retains authenticated observations with a
  bounded reconciliation horizon. Financial settlement is still separate work.
  UNKNOWN cannot book payment, consume stock or imply safe release.

## Acceptance gates

PS01 actual PG atomic order/stock/attempt/receipt/operation/job and exact replay.
PS02 concurrent same/different keys; expired hold vs start; fault after job and
     attempt writes leaves no partial facts (causal fault-hit evidence).
PS03 cross-owner/store/session and forged authority denied; final wait-time
     revocation/qualification expiry denied; no credential access for buyers.
PS04 MOCK positive + same evidence rejected by SANDBOX/LIVE; disabled, hidden,
     stale method, rotated credential, wrong scope/amount all fail closed.
PS05 old expiry preserves pending stock; generic Dispatcher never dispatches the
     buyer operation; query claim survives disabling its historical binding.

Full PG/race/vet and independent review required. These gates are NOT the global
commerce/provider/UI acceptance gates; no production service is changed.

## Amendment W4-02B (verification attempts)

2026-10-06. The merchant-initiated NT$1 PAYUNi verification is NOT a `checkout.payment_attempts` row (no `purpose` column was added):
it lives in `payments.payuni_verifications` (migration 0137), creating no order, reservation, inventory ledger row, payment fact, review
case or River job, so it can never enter finance summaries. `checkout.start_payment` is unchanged; it keeps refusing a method whose
qualification credential version, proof class (profile) or expiry does not match, which is what retires a rotated credential.
