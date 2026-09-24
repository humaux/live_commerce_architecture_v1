# Buyer payment start v1

Status: FROZEN, implementation and gates pending. Internal transaction, not a
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
  checking the existing private receipt. Same key/request returns original IDs,
  without another job or effect; changed profile/input conflicts. Fresh sessions
  of the same owner may replay, revoked sessions cannot.
- Go inserts River `payment_query_v1` args exactly `{operation_id,version:1}` in
  the same transaction, then calls `checkout.start_payment(hash,store,key,
  request_hash,order,method,method_version,profile,attempt,job_id)` returning JSON.
- SQL resolves the buyer again, repeats the command lock, rejects an existing
  receipt (Go handles replay before job insertion), locks order -> reservation
  -> payment head -> account -> binding -> qualification. It checks all scopes,
  current versions, enabled/visible, expiry and final capability after waits.
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
- This increment queues a durable query intent but does not register a payment
  query worker or expose a form. Credentials, verified observations, settlement,
  reconciliation horizon and actual provider workers are following work.
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
