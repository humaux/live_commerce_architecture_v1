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

Next actual adapter slice must replace disabled-only SQL guard only alongside tested
provider admission and final StartPayment revalidation. This diagnostic API is not a
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
