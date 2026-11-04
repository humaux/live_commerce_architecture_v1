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
- Both market commands use `command.Run` and atomic `command.Audit`, including
  principal ID in the canonical request, with the same store-global replay scope
  documented for SetPolicy. Failed audit rolls back market and receipt together.
- `LockMarket(ctx,tx,tenantID,storeID,id) (Market,error)` locks FOR SHARE and
  returns active state (including inactive). Quote rejects inactive markets;
  merchant policy configuration may prepare an inactive market for later use.
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
`Calculation.TaxMinor` is the total of all line tax plus ShippingTaxMinor;
exclusive total is subtotal + shipping + total tax (shipping tax is not added twice).

`internal/buyer`:

- `CheckScope(ctx,tx,Scope) error` validates canonical IDs and equality to the
  transaction-local tenant/store/buyer/session settings, with merchant principal
  cleared. Reads and command replay use it; it is not a token authenticator.
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
- Future BeginCheckout must in one transaction recheck expiry, cart version,
  active catalog and versions, active market/version, current enabled policy head,
  and verified destination binding before inventory locking/reservation. This
  contract does not implement or bypass those gates.

The later [RevalidateQuote prerequisite](checkout-quote-validation-v1.md) now
implements locked current-price/catalog/policy/expiry checks without durable writes.
It does not grant checkout SQL authority or destination eligibility, and callers
must still check expiry after later allocation/balance lock waits before holding stock.

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

Market create/state same-key replay, different-body conflict, CAS concurrency,
audit-failure rollback; pure money boundary/rounding/overflow tests; real PG both audiences, two tenants,
multiple stores/owners; cart no-write GET, absolute quantity replacement, same-key
replay/conflict, expected-version concurrency, no stock mutation, archived SKU;
missing/disabled/changed policy, currency mismatch, explicit zero vs omitted;
immutable snapshots after catalog/policy edits, expired replay no renewal;
cross-owner receipt key independence and resource privacy; exact SQLSTATE
permission/FK negatives; atomic event-failure rollback; catalog/policy/cart locks
against races; root full race/vet regression and independent P0/P1 closure.
Policy configuration/Quote creation are not provider sandbox/live, legal approval,
verified address/deliverability, public UI, payment or global G03/G05 acceptance.

## Amendment 2026-11-01 — FEAT-MANUAL-DISCOUNT (0220)

Supersedes: this contract's original no-coupon/no-adjustment boundary **only for a merchant-authorized named fixed discount** in storefront-v2 §G3. Older text and promotion-code rules remain historical; public buyer quote requests gain no money field.

The server-only `storefront.CreateManualQuote` reads `storefront.manual_order_discounts` for the synthetic owner created by the merchant pipeline. `CreateQuote` (public buyer handler) never applies that authorization. Authorization is written under the existing `inventory:reserve` + `catalog:read` scope, immutable per `(tenant_id,store_id,owner_id)`, with the normalized request hash (also for an order with no discount). A different request conflicts even if the pipeline has not yet produced its command receipt. `fulfillment.bind_manual_order_discount` binds the authorization to exactly one quote before Begin. The merchant bearer and both permissions are rechecked; buyer/checkout roles have SELECT only, fenced by buyer owner and tenant/store. No new item-price API, promotion, provider call or image pipeline exists.

The quote adds an optional field; legacy quotes omit it:
```json
{"adjustments":[{"kind":"manual_discount","name":"直播優惠","amount_minor":100,"currency":"TWD"}],
 "amount":{"subtotal_minor":2500,"discount_minor":100,"shipping_minor":0,"shipping_tax_minor":0,"tax_minor":0,"total_minor":2400}}
```
`adjustments` has exactly one line when present. Name is trimmed, printable UTF-8, 1..30 Unicode characters. `amount_minor` is a positive integer <= 1e12 AND <= the locked server merchandise subtotal; shipping is never discountable. Currency is the server quote currency. No stacking with a promotion code. `pricing.CalculateAdjusted` is shared by creation and revalidation; Begin revalidates the authorization's name/amount, bound quote ID, all line allocations, catalog/policy/cart versions and all preexisting payment constraints. Amounts are not rounded to a promotion-code step: manual bank transfer may retain any valid minor unit; pickup/COD whole-TWD rules still refuse fractional final totals. COD final total **plus its surcharge** must fit `cod_max_minor`.

Allocation/refunds: for each frozen quote line, calculate `floor(discount * line_subtotal / goods_subtotal)` using a 128-bit integer product; assign the remaining units to the **last** quote line, then work backwards only if that line cannot absorb them. Each share is in `[0,line_subtotal]`, shares sum exactly to the adjustment, and net goods + retained shipping (+ exclusive tax) equal the frozen final total. Exclusive/inclusive tax is recomputed on each net line through the existing calculator. Example: three equal 1250 lines, discount 100 -> shares 33/33/34, paid line totals 1217/1217/1216. For three 1-minor lines, discount 2 -> 0/1/1 (never a negative last line). Full goods discount retains shipping and shipping tax.

A partial goods refund uses each line's frozen `amount.total_minor` (net of its share and including its applicable tax), never list price; quantities within a line use proportional floor shares and a final-quantity remainder so cumulative goods refunds cannot exceed that paid line. Existing refund APIs remain **amount based**, not automatic per-RMA refunds: Stripe `held + requested <= CAPTURED` and `expected_refundable` CAS still enforce the global paid cap (stripe-refund-v1 RD3); pending/UNKNOWN refunds consume capacity. The returns-v1 RMA quantity/restock lifecycle does not issue money and cannot restore a discount or expand that cap. Offline full-refund paths use confirmed/collected discounted money and retain their one-refund state guard. No new partial offline-refund API is added.

The same frozen line is embedded unchanged in `checkout.Service.Begin`'s snapshot; `checkout.begin_hold` still requires stored quote equality. Buyer order/link GETs retain `snapshot.quote.adjustments`. Bank-transfer instructions and confirmation, pickup/COD collection, Stripe session amount if subsequently applicable, refunds and settlement consume the existing order total/payment facts. Settlement item completeness (0208) still compares provider balance transaction ID/currency/net; no separate discount deduction is made in settlement. reporting-v2 uses payment facts and frozen net line totals for product allocation, so it cannot report gross pre-discount collections. These downstream algorithms are reused, not replaced.

Retention: authorization name/amount, hash and quote binding follow the order/quote financial evidence retention; no plaintext customer/contact/capability is stored here. Immutable snapshots and receipts are never repriced or backfilled. No automatic cleanup or legal-retention interval is introduced. Permission revocation is checked on every merchant request including replay. Stale cart/policy/service/allocation versions retain their existing 409 conflict rules; GET does not refresh the quote TTL.

Gate: existing CI-only `TestManualDiscountValidation`, `TestMerchantToolsManualOrder/FEAT-MANUAL-DISCOUNT`, `TestLiveConsoleOrderForBuyerLivePrice/FEAT-MANUAL-DISCOUNT`, `TestCatalogV2Smoke/FEAT-MANUAL-DISCOUNT_picker_thumbnail` plus merchant-tools Node/BFF parser assertions. RED: Foundation 41093578253 and Gates 41093578464 at head fc28de173547886f74900c5caf4ab1f7a675b451. GREEN must come from this PR's implementation head; no local runtime tests are allowed.
