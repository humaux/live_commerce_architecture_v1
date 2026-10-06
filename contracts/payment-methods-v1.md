# Payment method configuration v1

2026-09-24. Frozen local implementation contract; provider/payment acceptance is
NOT_RUN. Depends on merchant-accounts-v1 and existing merchant scope/command/pricing.

## Scope and API

`internal/payments` owns immutable method revisions, not credentials or payments.
No new dependency, queue, fee calculator, generic provider registry or external call.
The first provider is PAYUNi; other screenshot providers remain planned, not supported.
This unit deliberately supports saved **disabled drafts** until an actual adapter,
merchant entitlement and environment acceptance can be checked. SQL also rejects
enabled=true. Do not add a caller-controlled ready/verified flag to bypass this gate.

Go package functions (caller-owned pgx transaction, no commit):

- `SetMethod(ctx, tx, scope, token, key, MethodInput) (Method,error)`;
  integration:manage, command name `payment.method.set`.
- `GetMethod(ctx, tx, scope, token, marketID, country, code) (Method,error)`;
  integration:read, current revision, missing => command.ErrNotFound.
- `InspectMethod(ctx, tx, scope, token, CheckInput) (Availability,error)`;
  integration:read, current locked metadata, internal merchant diagnostic only.
  It does not authorize a buyer, reserve inventory or authorize a payment attempt.

`MethodInput`: MarketID, Country, Code, Environment, ConnectionID (optional),
BindingVersion (0 iff no connection), ExpectedVersion (0=create), NameHans,
NameHant, NameEN, Enabled, Visible, SortOrder, MinAmountMinor, MaxAmountMinor.
JSON snake_case. `Method` mirrors input except ExpectedVersion becomes Version,
and adds Provider="payuni", Currency from locked market. No secrets/account number.

Codes: `payuni_credit`, `payuni_installment`, `payuni_atm`, `payuni_cvs`,
`payuni_linepay`. These identify intended methods, NOT verified supplier entitlements.
Installment tenors and payment-specific options belong to the adapter follow-up;
no default tenor, surcharge or COD is inferred. CVS code payment is not pickup/COD.
Environment exactly SANDBOX/LIVE, Country exactly TW, Currency exactly TWD for
this first slice. Market UUID and optional connection UUID must be canonical valid
IDs. Labels nonempty after whitespace checking, UTF-8 <=120 printable runes. Sort
0..1000. Amounts integer minor units: 1 <= min <= max <= 1_000_000_000_000;
these are merchant restrictions, not assertions of provider limits or wire units.
ExpectedVersion >=0 and <MaxInt64; BindingVersion positive if linked.

## Persistence, permission and ordering

0015: payments.method_versions and method_heads. Primary key
(tenant,store,market,country,code,version), current head FK. Full tenant/store market
and connection/provider/environment composite FKs. Immutable history, append-only
runtime grants; head grants only UPDATE(current_version); FORCE RLS.
No buyer/worker/checkout/issuer grants, no history DELETE/UPDATE. No new runtime role.

Before permanent replay and after all waits, compare transaction tenant/store/
principal GUCs and resolve current permission against scope revision. Request digest
includes actor and complete input. Same key/input returns original snapshot; changed
input conflicts. Separate keys use CAS and a per-method advisory transaction lock,
with caller timeout restored as in existing fulfillment. In-memory mutex insufficient.
Order: command lock -> pricing market SHARE -> method advisory/head lock -> account
SHARE -> binding SHARE. Read account and binding in separate statements to keep
the same account-before-binding order as credential Rotate.

Draft editing allowed even when market/binding inactive or snapshot binding version
stale: those are diagnostic blockers, not reasons to prevent hiding/unlinking a draft.
When linking/relinking, always require same-scope provider/environment account and
exact current binding version. For an existing same-connection revision, unchanged
BindingVersion may remain stale so a merchant can hide/edit without reauthorizing it.
No account mutation/credential access, binding enable, provider operation or River job.

## Diagnostics

CheckInput: MarketID, Country, Code, ExpectedVersion (>0), Environment, Currency,
AmountMinor (1..1e12). Input syntax invalid => ErrInvalid; missing current => NotFound.
Locks market, current head and account/binding; final permission recheck.
Output Availability: Available=false, MethodVersion, BindingVersion (actual current,
0 unlinked), CredentialVersion (actual current, 0 unlinked), Reasons ([]string).
Reasons in fixed order, each evaluated independently:
METHOD_VERSION_CHANGED, METHOD_DISABLED, METHOD_HIDDEN, MARKET_INACTIVE,
ENVIRONMENT_MISMATCH, CURRENCY_MISMATCH, AMOUNT_OUT_OF_RANGE, CONNECTION_MISSING,
BINDING_VERSION_CHANGED, BINDING_DISABLED, CREDENTIALS_UNVERIFIED,
ADAPTER_UNAVAILABLE. Unlinked skips binding/credential reasons. Last reason always
present in v1; unsupported still fails closed. A credential rotation changes observed
CredentialVersion, never method/binding semantic version. Output has no credentials.

Successor 0016 explicitly replaces the disabled-only SQL guard with a scoped
qualification reference requirement; [payment-start-v1](payment-start-v1.md) rechecks
it transactionally. No application qualification issuer exists, and merchant
SetMethod still rejects enabling. MOCK evidence never qualifies SANDBOX/LIVE.
This diagnostic API is not a
cached reusable authorization ticket. Existing in-flight facts must not be cancelled
or rewritten by method hide/disable/relink.

## Acceptance gates for this internal unit

- real PG Create/Get/update/replay, all five codes; independent visibility and
  environment; stale CAS; concurrent same-key and different-key writes;
- scopes/permissions/revocation including replay; raw SQL FK/RLS/immutable grants;
- enabled rejected in Go and SQL; independent diagnostic conditions under owner-only
  fixtures cannot become publicly available; rotation and binding-disable readback;
- insert/head/audit/receipt fault injection rolls back all configuration facts;
- full local race/vet regression, independent review; no provider/production calls.

MS01–MS08 remain NOT_RUN_PRODUCT: no settings HTTP/UI, buyer API, actual payment,
refund or settlement; local config is not merchant connectivity.

## Amendment W4-02B activation (PAYUNi merchant self-serve; migration 0137)

2026-10-06. Supersedes the "SQL also rejects enabled=true / no application qualification issuer exists / SetMethod still rejects
enabling" statements above (0016 already replaced the SQL guard by a qualification reference; this amendment adds the issuers).
Evidence: PROVIDER_MOCK (REAL_PG + MOCK transport); SANDBOX and LIVE are NOT_RUN. Owner rulings 2026-10-06: Taiwan card payments =
PAYUNi merchant self-serve; Stripe stays operator-registered for the owner's HK store; the platform switch `LC_PAYUNI_ENABLED`
(default 0) stays off until W4-02B and W4-03B (refund) are both merged and the PAYUNI-SANDBOX gate passes.

- **State machine** per connection (a connection already has exactly one environment): `CONFIGURED_UNVERIFIED -> VERIFYING ->
  VERIFIED_SANDBOX` (SANDBOX connection) or `VERIFIED_LIVE_PROBE` (LIVE connection). Only `payuni_credit` is activatable; the other
  four codes remain disabled drafts (`ADAPTER_UNAVAILABLE`).
- **SANDBOX/MOCK verification** (`payments.payuni_verifications`, not `checkout.payment_attempts`: a verification has no order, session
  or query job). NT$1 hosted trade, `merchant_trade_no` = `V`+base64url(uuid). A qualification is issued **iff** (1) the merchant-
  triggered query of that trade, authenticated with the connection's key, projects to the payment-capture-v1 CAPTURED rule (TradeNo,
  DataSource A, TradeStatus 1, CloseStatus 2, CloseAmt = NT$1, no refund hint; re-derived in SQL), **and** (2) a signed notify receipt
  (W4-01B: SUCCESS, TradeStatus 1, NT$1, same connection, trade and endpoint profile) exists, **and** (3) the connection still has the
  credential version the verification pinned. Two independent channels: the merchant API role can record the query but cannot forge
  the notify receipt (ingress role). Issuer `payments.issue_payuni_qualification` is EXECUTE-able by `commerce_payment_worker` only
  (the worker sweeps every 5 s); proof class is derived (`PROVIDER_MOCK` for a PROVIDER_MOCK profile, else `REAL_SANDBOX`), never a
  parameter; valid 180 days. A verification expires after 2 h.
- **LIVE probe**: one read-only signed query for a MerTradeNo that must not exist (payuni-wire amendment); only when the reply
  authenticates with the LIVE HashKey/HashIV and carries no trade row is a probe row written (`signature_verified` pinned true) and
  `payments.issue_payuni_live_qualification` (commerce_runtime; row must be <= 10 min old, same credential version) issues
  `REAL_LIVE` with `evidence_ref='live-probe:<probe id>'`. No transaction is ever created. Deployment profile must be LIVE.
- **Enabling**: `SetMethod(Enabled=true)` requires `payuni_credit`, a connection, `LC_PAYUNI_ENABLED=1` (Go; else `409
  platform_disabled`) and then `payments.enable_payuni_method` (the only writer of an enabled revision; a RESTRICTIVE RLS policy
  refuses `enabled=true` INSERTs by `commerce_runtime`) which finds the latest unrevoked, unexpired qualification of the connection's
  CURRENT credential version and environment (LIVE: REAL_LIVE; SANDBOX: REAL_SANDBOX or PROVIDER_MOCK) else `409 not_qualified`.
  Disabling/editing keeps the plain draft INSERT.
- **Rotation**: a credential rotation changes `integration.merchant_accounts.credential_version`; the qualification is not rewritten
  (account_qualifications is revoke-only, 0077) but every consumer compares versions: `InspectMethod` reports `CREDENTIAL_ROTATED`
  and `checkout.start_payment` / the buyer view already refuse `q.credential_version <> a.credential_version`.
- **Diagnostics** `Reasons` order now: ... `BINDING_DISABLED`, `CREDENTIALS_UNVERIFIED` (no current valid qualification), `NOT_QUALIFIED`,
  `CREDENTIAL_ROTATED`, `QUALIFICATION_EXPIRED`, `PLATFORM_DISABLED`, `ADAPTER_UNAVAILABLE` (code other than payuni_credit).
  `Available` is true iff the list is empty. `Method` gains `qualification_id` (enabled revisions only).
- **HTTP** (integration:manage unless noted): `POST /v1/admin/stores/{store}/payments/payuni/verify` {connection_id} + Idempotency-Key;
  `POST .../verify/{id}/check` {connection_id}; `POST .../live-probe` {connection_id}; `GET .../connections/{id}/status`
  (integration:read). Codes: `not_qualified`, `platform_disabled`, `profile_not_allowed`, `payuni_probe_failed`.
- NOT_VERIFIED against official PAYUNi docs (docs.payuni.com.tw is a JS SPA; fetch 2026-10-06 returned no content): the wire shape of
  the "no such trade" reply and whether PAYUNi signs it. The probe fails closed on anything but an authenticated reply with no trade row.
