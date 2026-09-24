# Payment query and authenticated observations v1

Status: IMPLEMENTING, 2026-09-24. Follows payment-start-v1 at 30a0498.
This is the real query execution path and durable authenticated report intake,
not financial capture, refund, stock allocation or permission to release forms.

## Reuse and authority

- Reuse River `payment_query_v1`, core Claim/Complete, immutable attempts,
  account ciphertext versions, Keyring.open and the existing PAYUNi Query wire.
  No additional broker, operation registry, dynamic endpoint or credential API.
- `integration.load_payment_query(operation, generation, token, profile)` is
  SECURITY DEFINER with fixed search_path, callable only by commerce_worker.
  Lock binding then operation like Claim. Require exact BUYER_PAYMENT_QUERY /
  reconcile / UNKNOWN, current generation and constant-sized token SHA256, live
  database-clock lease both before and after SQL waits. Join the scoped attempt,
  immutable account, binding and exact credential version, never current key.
- Return tenant/store/connection/provider/environment/account/version, key ID,
  nonce/ciphertext, frozen merchant trade number/currency/amount/method, existing
  provider reference, and database-clock attempt age. No caller-selected scope,
  account, credentials or URLs. Ordinary roles have no ciphertext SELECT.
- Server-owned profile must equal attempt profile; SANDBOX/LIVE must also equal
  account environment. Historical disabled/revised methods or bindings, rotated
  keys and expired admission evidence do not erase outstanding payment facts.
  A missing historical key fails closed, never tries the latest credentials.

## Go contract and worker

- `accounts.Keyring.LoadPaymentQuery(ctx, tx, operationID, generation, token,
  profile) (PaymentQueryMaterial,error)` validates inputs, calls the private SQL,
  opens the exact AAD/version and constructs the existing redacted payuni.Client.
  Material has Client, Expected payuni.ExpectedTrade and Age time.Duration.
  Fixed error sentinel; no plaintext key returned, logged or serialized.
- `payments.NewQueryWorker(ctx,pool,keys,profile,QueryWorkerOptions)` validates
  real worker session authority. `DefaultQueryWorkerOptions`: lease30s, DB2s,
  shared load+wire call10s, retry2min, maxage24h, maxgeneration720. All bounded;
  call + 2*DB +1s < lease. The caller owns River/pool lifecycle.
- private job args exactly operation_id/version1. Read exact family before
  Claim; never claim or complete a merchant operation. Claim commits before
  loading/querying. Load transaction commits before one wire query. No dispatch.
- Persist one verified projection via `integration.record_payment_query(op,
  generation,token,profile,jsonb)` and Complete UNKNOWN/payment_report_observed
  in the same transaction; then snooze. No financial interpretation here.
- Failure/panic/timeout/missing key only fixed persisted codes and UNKNOWN,
  never fail/release stock or expose raw wire/PG error. Parent cancellation does
  not detach external I/O. Completion uses bounded context if parent still live.
- If frozen attempt age or claim generation exceeds budget, persist
  UNKNOWN/payment_query_budget_exhausted and cancel River. On later job retry,
  that persisted code with no live lease cancels without provider calls. A
  manual recovery command is future work, not an implicit automatic reset.

## Observation storage and gates

`payments.provider_observations` append-only, scope+attempt FK, source QUERY,
execution profile/environment, first claim generation and database receipt time,
bounded report JSON and sha256 canonical jsonb. Same attempt+report hash dedup.
The projected JSON uses the existing ten Go Observation field names. SQL checks
exact keys/types and frozen amount/trade/method, provider response SUCCESS,
credit PaymentType1/AuthType1/CardInst0, TradeStatus 0/1/2/3/4/8/9,
DataSource A/B and optional CloseStatus 1/2/3/7/9. Not a payment-success flag.
An established nonempty TradeNo cannot change; one provider trade cannot attach
to two attempts of the same account. All writes and completion recheck lease
after waits; faults/stale token/profile mismatch commit no partial report.

PQ01 real PG exact historical decryption after rotation/disable; malformed AAD,
missing key, wrong profile/token/generation/role deny without plaintext output.
PQ02 actual River worker + signed mock HTTP response, one query-only request,
report+completion atomic; duplicate/stale/conflicting report fences.
PQ03 lock-wait lease expiry and triggered write rollback cause zero report;
cross-tenant/account and forged merchant job rejected; ACLs remain least privilege.
PQ04 invalid signature/amount/merchant, timeout/panic, exhausted horizon/generation
and repeat cancelled job: no unverified report, payment/stock remains pending.
Full real PG/race/vet and independent review required. No production/real PSP call.
