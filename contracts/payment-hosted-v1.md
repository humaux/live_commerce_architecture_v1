# Order-bound hosted payment preparation and one-shot handoff v1

Status: DRAFT_REVIEW, 2026-09-25, baseline `f17b915`. Internal implementation
precedes buyer transport/UI and provider qualification. No production enablement.

## Evidence and decisions

- Reuse `PaymentStarter`, permanent command receipts, the frozen attempt, PAYUNi
  `BuildHosted`, query/capture workers and the original inventory aggregate.
- Official [UPP](https://docs.payuni.com.tw/web/#/7/34) requires a merchant trade
  number not repeated within ten minutes. [Errors](https://docs.payuni.com.tw/web/#/7/44)
  include duplicate trade and expired timestamp. Neither is a documented permanent
  idempotency guarantee or permission to repost after an uncertain response.
- `TradeLExpireSec` is a separate 60–600 second page cutoff. The inspected docs
  do not define its start event or the timestamp freshness interval. ReturnURL is
  not financial truth; retain authenticated query/capture as the authority.
  Fresh official retrieval: Humaux `2b63db32-48a6-4679-b130-eca849b38d56`.
- Therefore separate **preparation** (replayable local transaction) from
  **handoff** (at most one server release of the provider form). A handoff is
  consumed before responding. A lost handoff response goes to status/reconciliation,
  not to another form, timestamp, merchant trade number or payment attempt.

| Choice | Fit for this SaaS | Decision |
| --- | --- | --- |
| Reissue/repost forms on every retry | Cannot prove provider deduplication after uncertainty | Rejected |
| One prepared form per existing order/attempt; one-shot handoff | Preserves one financial identity and existing recovery/query engine | Selected |
| Second engine or permanent per-SKU PSP links | Duplicates inventory/payment truth or loses quantity/freight/current price | Rejected |

Preparation, handoff and payment are different facts. `handed_out` means only that
the server committed the handoff; it never means the browser received it, the PSP
accepted it, or funds were captured. A browser can retain already released bytes;
the server cannot revoke them or prove PSP-side exactly-once behavior.

## Internal Go interface

Keep legacy `NewPaymentStarter` / `StartPayment` behavior and digest unchanged.
Extract its existing transaction body into a package-private helper accepting
the existing transaction/scope/digest; do not add a second start implementation.

- `HostedConfig { ReturnURL, NotifyURL string }`: server-only, required fixed
  HTTPS endpoints, no query/fragment/userinfo or nonstandard port. No buyer URL.
- `HostedInput { OrderID, MethodCode string; MethodVersion int64; Locale string }`:
  credit only; locale exactly `zh-CN`, `zh-TW`, `en`. Provider language maps both
  Chinese locales to `zh-tw`, English to `en`.
- `NewHostedPaymentStarter(ctx, checkoutPool, jobs, profile, keys, config)` returns
  an immutable service. Reuse existing pool/job/keyring validation; no provider I/O.
  Keys never become public fields, logs or JSON. Existing query-only clients remain
  unable to generate forms. No injectable real-provider endpoint or transport.
- `BeginHosted(ctx, token, storeID, key, HostedInput) (PaymentResult, error)`:
  one `buyer.WithScope` transaction calls the shared start helper, materializes the
  exact historical credentials, builds the form, persists it and commits together.
  Failure at any stage rolls back attempt/stock/receipt/job/form. The request digest
  includes full hosted input, execution profile and a canonical server endpoint
  config digest, distinguishing legacy starts and changed locale/config requests.
  Same request/key replays the original receipt without rebuilding a form, changing
  timestamps or extending expiry. No form appears in this replayable response.
- `TakeHosted(ctx, token, storeID, orderID) (HostedHandoff, error)`:
  authenticated order-level one-shot command. There is no fresh-attempt fallback.
  Result fields: `order_id`, `disposition` (`ISSUED` or `ALREADY_ISSUED`), `expires_at`,
  optional `form`. Only the first committed take returns the form. Replays under
  any caller request key return `ALREADY_ISSUED` without form/material. Expired or
  revoked authority fails closed. Response is returned only after transaction
  commit; uncertain commit cannot cause a second release.

The internal `PaymentResult` contains private operation/job identifiers: later
buyer HTTP must use a separate minimal projection, not marshal this struct.

## SQL authority and storage (next migration reserved to integrator)

One `checkout.hosted_payment_pages` row per scoped owner/order and attempt,
foreign-keyed to the immutable attempt. Columns include locale, config digest,
prepared_at, expires_at, exact bounded provider form JSON and nullable handed_out_at.
The form has only fixed `action` and four single-valued `fields`: `MerID`,
`Version`, `EncryptInfo`, `HashInfo`. It contains no raw key, IV, card or buyer PII.
No new cart/order/financial state machine. No raw form is written to browser storage.

Use the existing private non-login `commerce_checkout_writer`, forced owner RLS,
fixed `search_path=pg_catalog`, exact column grants and explicit PUBLIC revokes.
Only `commerce_checkout_runtime` may execute the new buyer entry functions.
No ordinary buyer/merchant/worker role receives ciphertext-table SELECT or handoff
execution. The existing leased `integration.load_payment_query` remains unchanged.

Material-loading entry re-authenticates the capability, sets scope from it and
locks order → reservation → payment head → account → binding → qualification,
matching payment-start ordering. It resolves the attempt from the owned order,
never from a caller-supplied account/version. It checks:

- exact scope/attempt/profile/currency/amount and original order total;
- AWAITING_PAYMENT/PAYMENT_PENDING with matching generation/provenance;
- active market, unchanged enabled/visible method head, account, binding and
  credential version, unrevoked matching qualification valid at database time;
- `PROVIDER_MOCK` only uses SANDBOX mock evidence; SANDBOX/LIVE require exact REAL
  qualification. An existing MOCK form never becomes a real profile form;
- no financial review/confirmed result permits another handoff.

The new material function returns only encrypted historical credential material
and immutable form inputs to the trusted keyring boundary, within that transaction.
The writer gets only the needed credential columns under scoped policy; runtime
gets no general credential read. The keyring decrypts through existing AAD/open.
There is no latest-credential substitution after rotation.

Server policy: preparation uses a database timestamp, `TradeLExpireSec=300`, and a
local handoff deadline of the earlier of prepared_at + 60 seconds and qualification
expiry. This is our release window, NOT a claim about the PSP timestamp window or
when its payment-page timer starts. A deadline is never renewed on replay.

Saving the form validates its fixed environment-specific action/field shape,
config digest, amount source and unchanged authority; it repeats capability and
database-time fences after all writes/FK/trigger waits. Handoff takes the same
order-first lock and repeats live eligibility/deadline checks before the first
release. Concurrent takes produce exactly one ISSUED response. The nullable
handoff timestamp may transition once, never clear or extend it.

After handoff, current binding disable/credential rotation still cannot redirect
query/capture to a new account: existing historical query/capture policy wins.
Handoff must not lock integration operations while holding order locks. Financial
read checks do not acquire operation locks or introduce another inventory writer.

## Acceptance gates

HP01 actual PG preparation commits one original start/attempt/query job/receipt/
     stock transition and one form; local crypto/body/storage failure rolls all back.
HP02 identical key/input replays the receipt and exact persisted form/deadline;
     changed input/locale/config/profile conflicts; different keys cannot duplicate
     an order's attempt. Legacy StartPayment tests remain unchanged and pass.
HP03 first committed take returns one form, concurrent/repeated takes return none;
     lost response/ambiguous commit never reissues; no provider network call is made.
HP04 cross-owner/store, forged GUC, expired/revoked capability, wrong role and
     forged provider/action/form are denied; exact grants/policies are asserted.
HP05 qualification/credential/method/binding drift before prepare/take and expiry
     during lock/FK/fault waits fail closed; MOCK cannot qualify SANDBOX/LIVE.
HP06 decrypt the synthetic form using an independent wire verifier: exact frozen
     merchant/trade/amount/language/URLs/timestamp; no secret in outputs/logs; no
     new form or release when already paid/review-required or beyond deadline.
HP07 full Go/actual PG/race/vet and independent source review; no existing payment,
     query, capture, inventory, checkout, history or platform grant gate weakened.

## Subsequent assembly and limits — not waived by this internal gate

- Owned payment options/current financial-state projection and strict private HTTP/
  BFF routes; one explicit Pay click prepares, takes once and navigates to provider.
  UI recovery polls status, never automatically takes/reposts an already issued form.
- Real query/capture worker assembly, safe return page, notification protocol and
  observed provider delivery/ACK behavior, plus actual browser Next→Go→PG→mock PSP.
- An unissued expired form or lost handoff can leave PAYMENT_PENDING requiring
  reconciliation/manual review. Never infer unpaid from timeout/NOT_FOUND, release
  stock or create another payment to improve the happy-path demo. A future proven
  cancellation/retry protocol can resolve this limitation without a second engine.
- Real merchant qualification/enablement and exact authorized SANDBOX transaction
  remain separate. No REAL evidence is synthesized from mocked transactions or
  merely possessing a secret. LIVE operations require approval and rollback scope.
- Existing global SaaS/provider/deployment gates remain open. This contract neither
  proves merchant readiness nor authorizes any charge, customer change or deployment.
