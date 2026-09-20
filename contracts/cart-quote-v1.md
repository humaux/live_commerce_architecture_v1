# Buyer cart and explicit country-flat Quote v1

Status: interface freeze for internal implementation from `cd6e867`; NOT_PUBLIC.
Contributes T04/T10. Does not claim complete checkout, shipping eligibility or tax
compliance. Architecture §§8/11/17 and buyer-capability-v1 remain authoritative.

## Business boundary

- One current cart per `(tenant,store,buyer owner)`; language and chosen market do
  not create another identity/cart. All writes require `buyer.WithScope`.
- Cart lines set absolute target quantities, never increment-on-retry. Up to 50
  distinct SKU IDs, 1..1e9 units each; empty replacement clears the cart. Product
  and SKU must be active on mutation/quote. Cart reads do not mutate anything.
- Neither cart nor Quote touches inventory/reservations or a provider. Only a
  future explicit BeginCheckout may reserve inventory, with all transaction gates.
- Quote prices come only from locked server catalog rows and an explicitly
  configured, immutable policy version. No client prices, inferred zero freight,
  automatic tax law, FX, coupon or carrier success. Unsupported price rules fail.
- This calculator supports only an explicitly configured **country-flat** price
  rule: same charge within the configured country/delivery method. It does not
  authorize fulfillment to an arbitrary store/address. Address-sensitive rules,
  carrier eligibility, verified Taiwan convenience-store selection and destination
  binding must be integrated before public checkout. An internal Quote by itself
  must never be treated as proof of deliverability or permission to charge.
- Policy methods are `home`, `cvs_711`, `cvs_familymart`; these are independent
  configuration keys, not claims that carrier integrations exist. Only a merchant
  operator's explicit configuration can enable a key. Fixtures are synthetic.

## Frozen Go interfaces

All domain functions receive a caller-owned `pgx.Tx`; errors require rollback.
Use existing `command.ErrInvalid/ErrConflict/ErrNotFound`, `command.CheckMoney`
and `command.ValidID`. No pool, HTTP handler, new dependency or public route.

`internal/pricing` (merchant policy + pure money calculation):

- `Market { ID, Code, Name, Currency string; Version int64; Active bool }`;
  `MarketInput { Code, Name, Currency string }`.
- `CreateMarket(ctx,tx,platform.Scope,key,MarketInput) (Market,error)`: creates
  active version1; currency must equal store currency and is immutable. Code is
  `[a-z][a-z0-9_-]{0,39}`, name1..120 printable characters. No locale mapping.
- `SetMarketActive(ctx,tx,platform.Scope,key,id,expectedVersion,active) (Market,error)`
  changes only active state and increments version with CAS; currency/code do not
  change. Market creation/state writes require `pricing:write` scoped transaction.
- `Policy`: `MarketID, Country, Method, Currency, ShippingMode, TaxMode, TaxBasis string`,
  `Version, ShippingMinor, TaxRateBPS, QuoteTTLSeconds int64`, `Enabled bool`.
- `PolicyInput`: same business fields except Version; `ExpectedVersion int64`,
  `ShippingMinor, TaxRateBPS *int64` to distinguish omission from explicit zero,
  `ConfigurationRef string` (1..240 printable chars, internal operator evidence).
- `SetPolicy(ctx,tx,platform.Scope,key,PolicyInput) (Policy,error)`: version 0
  creates, later exact expected version appends a new immutable revision and moves
  the current head. Currency must match the store. Atomically records merchant
  command receipt and audit. Canonical request includes principal ID. Caller must
  have `pricing:write` in its scoped transaction. This uses existing merchant
  store-global operation/key receipts plus principal-in-hash, so another merchant
  actor reusing that key conflicts; it does NOT claim full actor namespace §17.2.
- `LockCurrent(ctx,tx,tenantID,storeID,marketID,country,method) (Policy,error)`: locks the
  current policy head FOR SHARE; reads immutable version. Missing or disabled
  policy -> `ErrNotFound`. RLS denies wrong scope; no ConfigurationRef is returned.
- `AmountLine { UnitPriceMinor, Quantity int64 }`.
- `LineAmount { SubtotalMinor, DiscountMinor, TaxMinor, TotalMinor int64 }`.
- `Calculation { SubtotalMinor, DiscountMinor, ShippingMinor, ShippingTaxMinor,
  TaxMinor, TotalMinor int64; Lines []LineAmount }`.
- `Calculate(Policy, []AmountLine) (Calculation,error)` is pure and validates its
  inputs. Country is two uppercase letters; method is the above enum; currency
  three uppercase letters matching the store (no currency conversion).

Required policy values: `ShippingMode=country_flat`; explicit nonnegative shipping
minor units <=1e12; `TaxMode=none|exclusive|inclusive`; explicit tax rate 0..10000
basis points (`none` requires 0); `TaxBasis=goods|goods_and_shipping`;
QuoteTTLSeconds 60..1800. Zero is accepted only through an explicit configuration.
All amounts/line sums/final totals are checked against `command.MaxMoney` before
addition; multiplication uses CheckMoney. Nonempty quote, max 50 lines.

Calculation v1: no discounts (stored zero); round nonnegative tax HALF_UP at each
line in integer minor units. Exclusive: `(base*bps+5000)/10000`; inclusive tax
component: `base-(base*10000+(10000+bps)/2)/(10000+bps)` (HALF_UP net first;
the tax is the exact remainder). Shipping tax is rounded separately
only for `goods_and_shipping`. Exclusive adds tax; inclusive reports the included
component without charging it twice. Store these exact allocations for future
partial refunds, never re-price historical snapshots.

`internal/buyer`:

- Add `RunCommand(ctx,tx,Scope,operation,key,request,result,fn) error` using the
  same canonical JSON/advisory replay pattern as merchant command.Run, but a new
  private receipt table keyed by tenant/store/owner/session/operation/key. Do not
  reuse merchant membership FK or unify identities. Same key/body replays the
  exact successful result even after later cart edits or quote expiry; replay
  never extends expiry. Changed body conflicts. Request <=64KiB; result <=1MiB.
- Reject invalid result pointer, key, operation or scope before callback. Validate
  scope equals current transaction-local buyer IDs; RLS independently enforces it.

`internal/storefront` (buyer cart/Quote):

- `Item { SKUID string; Quantity int64 }`; JSON names snake_case everywhere.
- `Cart { ID, Currency string; Version int64; Items []Item }`.
- `CartInput { ExpectedVersion int64; Items []Item }`.
- `GetCart(ctx,tx,buyer.Scope) (Cart,error)`: absent -> ID empty/version0/empty
  items/current store currency, no insert. Existing items sorted by SKU ID.
- `SetCart(ctx,tx,buyer.Scope,key,CartInput) (Cart,error)`: canonicalizes order,
  rejects duplicate SKUs, locks/creates owner cart, checks version, locks catalog,
  replaces all lines and increments version once. Atomic receipt/event; failures
  leave no partial cart. Initial expected version0 only; others must match.
- `QuoteInput { CartVersion int64; MarketID, Country, Method string }`.
- `QuoteLine { SKUID, ProductID, Code, Name, Description string; SKUVersion,
  ProductVersion, Quantity, UnitPriceMinor int64; Amount pricing.LineAmount }`.
- `Quote { ID, CartID, Currency, CalculationVersion string; CartVersion, MarketVersion int64;
  Policy pricing.Policy; Lines []QuoteLine; Amount pricing.Calculation;
  CreatedAt, ExpiresAt time.Time }`.
- `CreateQuote(ctx,tx,buyer.Scope,key,QuoteInput) (Quote,error)`: locks owned cart
  and validates nonempty/current version; locks active market, current enabled policy; locks all
  referenced products then all SKUs in sorted stable order; validates active state,
  currency and money; snapshots names/descriptions/prices/versions/allocations;
  uses DB clock expiry. Saves immutable quote + event + replay receipt together.
- `GetQuote(ctx,tx,buyer.Scope,id) (Quote,error)`: exact immutable historical
  snapshot only, including expired ones; cross-owner/store uniformly NotFound.
  Reading a quote is not a current-price/inventory/checkout authorization check.

## Frozen persistence (root migration 0007)

- `pricing.markets`: tenant/store/id PK, unique store/code, immutable currency FK
  to `(control.stores.tenant_id,id,currency)`, active/version. Quote locks market
  FOR SHARE and snapshots market version; one country may have multiple markets.
- `pricing.policy_versions`: tenant/store/market/country/method/version composite PK,
  immutable policy fields, configuration_ref, principal_id FK to membership.
- `pricing.policy_heads`: same key without version; current_version FK to the
  immutable version. Policy edit serializes that key before creating/updating head.
- `buyer.command_results`: scoped owner/session/operation/key PK, request_hash,
  response JSON, created_at; composite FK to capability session. Append-only.
- `storefront.carts`: `(tenant_id,store_id,owner_id,id)` PK, unique owner scope,
  currency, version>=0; cart_lines FK to exact cart and store-scoped SKU. The
  transient new version0 head must be updated within the same successful command.
- `storefront.quotes`: id + exact owner/cart/creator-session, cart_version,
  market_id/market_version/country/method/policy_version FK, created/expires DB times, JSON snapshot <=1MiB.
  Quote JSON is a fully typed server-generated immutable value. IDs/times/policy
  binding must match the relational header on insert/read; no client JSON stored.
- `storefront.events`: scope/owner/session/cart, optional quote ID, action only
  cart.updated or quote.created, created_at. Same transaction as command.

All new tables FORCE RLS. Merchant runtime can change policies only in its
tenant/store and cannot access buyer carts/quotes/receipts. Buyer runtime can
read public policy columns but not configuration_ref or principal_id; cannot
change policy values. Catalog read grants are column-scoped to needed public
fields. Row locks use exact UPDATE-column grants with UPDATE visibility policies
and WITH CHECK(false), never permit actual catalog/policy mutation. Cart/Quote
RLS binds tenant/store/owner; INSERT creator session matches current session.
Private buyer credential tables remain inaccessible. No grants to buyer issuer.
Add explicit `pricing:read` and `pricing:write` permission identifiers. New-store
owners receive these through the existing trusted onboarding function; do not
silently backfill new rights to existing memberships. Tests grant synthetic users
explicit rights. Market/policy tables are not a second catalog or FX engine.

## Acceptance gate

Pure money boundary/rounding/overflow tests; real PG both audiences, two tenants,
multiple stores/owners; cart no-write GET, absolute quantity replacement, same-key
replay/conflict, expected-version concurrency, no stock mutation, archived SKU;
missing/disabled/changed policy, currency mismatch, explicit zero vs omitted;
immutable snapshots after catalog/policy edits, expired replay no renewal;
cross-owner receipt key independence and resource privacy; exact SQLSTATE
permission/FK negatives; atomic event-failure rollback; catalog/policy/cart locks
against races; root full race/vet regression and independent P0/P1 closure.
Policy configuration/Quote creation are not provider sandbox/live, legal approval,
verified address/deliverability, public UI, payment or global G03/G05 acceptance.
