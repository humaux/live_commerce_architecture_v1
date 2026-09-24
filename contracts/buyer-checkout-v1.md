# Buyer checkout v1 — authority decision and implementation gate

Status: INTERNAL_AGGREGATE_CONTRACT_FROZEN, 2026-09-24, based on `71c7623`.
This is not a claim that checkout, payment or public purchase is implemented.

## Decisions

1. Keep the existing Go `pricing.Calculate` as the single calculator. Implement
   [locked Quote revalidation](checkout-quote-validation-v1.md) first; immutable
   buyer-inserted JSON is not evidence that every amount is correct.
2. Use a separate, mutually exclusive `commerce_checkout_runtime` authority and
   checked pool. Ordinary buyer, merchant, issuer and worker logins cannot inherit
   it. Startup checks session_user, mixed/privileged roles and SET ROLE capability
   exactly as the current pool gate. It is a trusted internal server boundary,
   never a new buyer identity, merchant membership or public SQL credential.
3. Checkout Go code authenticates the opaque buyer capability and validates current
   Quote/fulfillment/allocation within one bounded transaction. Only checkout
   runtime may EXECUTE a fixed hold writer function. A dedicated
   `commerce_checkout_writer` NOLOGIN/NOINHERIT owns that SECURITY DEFINER function;
   PUBLIC and all ordinary buyer/merchant roles receive no EXECUTE or membership.
   It has only required SQL grants, fixed pg_catalog search_path and qualified names.
4. The writer re-resolves hash/store with `buyer.resolve_scope` in that same
   transaction, overwrites all buyer GUCs and clears merchant principal. Caller
   GUCs, typed IDs or a returned Quote alone are not credentials. The database
   entrypoint's caller is trusted checkout runtime, not an end-user caller who
   can skip the Go price gate. No arbitrary provider/action/URL/amount inputs.
5. Use a private checkout receipt, never `buyer.command_results` (ordinary buyer
   can INSERT that table). Permanent namespace is tenant/store/owner/operation/key;
   creator session is provenance, not a fresh dedup namespace. Canonical request
   digest is server-built. Identical replay returns original IDs without extending
   expiry; changed input conflicts. Replays remain authenticated. Current state
   is read separately, not inferred from a historic command response.
6. BeginCheckout creates `DRAFT order + HELD reservation`; it does not create a
   runnable payment operation. StartPayment must later atomically mark reservation
   PAYMENT_PENDING and create payment attempt + T06 operation/job. Old expiry must
   not release payment-pending stock. An expiry task is not a payment operation.

## Required aggregate/locks

- Immutable order/line snapshots preserve Quote exact prices/tax/quantity and
  product identity. Order commercial, payment and fulfillment facts stay separate.
- Composite FKs bind tenant/store/owner/session/quote/cart/order/reservation.
  Inventory continues using the existing ledger/balance trigger, not a second
  ledger. Add explicit buyer owner/session provenance, nullable merchant principal
  with exactly-one-actor-family CHECK. Buyer UUID is never a membership UUID.
- Inventory ledger's store-global key must be derived from the private checkout
  UUID, not the public owner idempotency key (different owners may choose it).
- Lock order: capability tenant/store/owner/session → private owner receipt key →
  cart/Quote → market/current policy → sorted products/SKUs → pickup source head → destination head →
  merchant service head → allocation head → sorted warehouses/balances → final DB-clock
  expiry → durable facts.
  Reuse no merchant command locks. External I/O happens before/after, not while
  holding the database transaction. Locks do not freeze wall-clock validity.
- Quote method/country is not deliverability. An immutable destination revision
  binds buyer/cart, selected CVS brand/code/namespace/address, source and freshness;
  it is carrier-neutral. Lock the separately enabled merchant fulfillment service
  revision and server-generated allocation as well. Client warehouse, store number
  text, callback URL or `verified=true` is not authority.
- Merchant-arranged (`MANUAL`) service may accept checkout without a carrier
  connection or tracking number: require explicit merchant service activation,
  applicable restrictions and server-priced freight, and record fulfillment as
  awaiting merchant arrangement. No provider job, carrier-verified claim or COD
  capability is inferred. The authorized merchant adds actual carrier/service and
  tracking to the package later, with audit and route-specific store-code mapping;
  this does not rewrite the order's original destination snapshot.
- `API` fulfillment additionally needs an active account binding and service/store
  eligibility before provider operations. MANUAL is a supported option, not a
  replacement for third-party API integration or a silent fallback on API failure.
  See [merchant payment and delivery settings](merchant-service-settings-v1.md).
- Existing merchant ReleaseReservation must not independently mutate a checkout
  aggregate and omit its order/event facts. Checkout release/expiry must preserve
  generation, ledger conservation and aggregate state together.

## Gate before implementing the aggregate

Destination/source semantics are frozen in [buyer destination](buyer-destination-v1.md);
merchant service revisions and server allocation configuration have separate bounded
internal acceptance. Consume them with explicit MANUAL versus API eligibility gates.
A single selected carrier/contract is not a prerequisite for carrier-neutral
selection or merchant-arranged checkout. Real API dispatch still needs its exact
account/service capability; local synthetic adapters never prove that capability.
Also freeze HELD expiry job/worker ownership, state and
generation transition, unique durable checkout/receipt keys and lifecycle FK DDL.
No `hold()` shortcut may ship while these are unresolved. Public mounting additionally
requires trusted published-domain/store resolution, CSRF/cookies/rate limits and
real desktop/mobile checkout browser acceptance.

Acceptance: real ordinary role matrix; direct buyer SQL cannot call hold, pre-seed
private receipt or mutate inventory/order; two buyers compete for stock=1; reverse
multi-SKU contention; owner/store FK denial; same-key concurrent replay and changed
request; price/policy/cart/destination/allocation revocation races; all facts/job/event/
receipt roll back on every injected failure; no runnable job before commit;
expiry/StartPayment race and final expiry after waits; cancelled transaction has
no leaked scope or hold. Provider sandbox/live and complete G03/G04/G05 remain
separate gates and cannot be inferred from internal tests.

## Internal implementation interface (0013)

This section freezes the implementation boundary, not an acceptance result.
Independent preflight: Humaux `b1a34f48-3f8b-438e-9e16-ee6fe5f40ec2`.

### Go and SQL boundary

- `checkout.New(ctx, checkoutPool, jobs) (*Service,error)` checks the exact new
  authority using `platform.ValidateCheckoutPool`; `OpenCheckoutPool` provisions
  no roles, just validates an existing nonprivileged login. Every existing pool
  gate also rejects membership in the checkout role. No role inheritance shortcut.
- `(*Service).Begin(ctx, token, storeID, key, Input) (Result,error)` owns the
  bounded transaction via `buyer.WithScope`. `Input` has only `QuoteID`,
  `DestinationID`, `CartVersion`, `ServiceVersion`, `AllocationVersion`, with JSON
  snake_case tags. Service code, market, country, prices and warehouse plan are
  server-derived. `Result` contains `OrderID`, `ReservationID`, `Generation`,
  `ExpiresAt`, `JobID` with snake_case tags, no token or delivery PII.
- `(*Service).Get(ctx, token, storeID, orderID) (Order,error)` is owner-scoped and
  read-only. `Order` embeds `Result`, plus `CommercialState`, `FulfillmentState`
  and `Snapshot`. `Snapshot` contains exported typed `Quote`, `Destination`,
  `Service`, `Allocation` using existing package types, JSON keys respectively
  `quote`, `destination`, `service`, `allocation`. Its quote lines are immutable
  order-line/price/product snapshots; no second price calculator/table copy.
- SQL entrypoint `checkout.begin_hold(bytea,uuid,text,bytea,uuid,jsonb,jsonb,bigint)`
  takes token hash, store, key, request digest, generated order UUID, Snapshot,
  allocated lines, expiry job ID; returns Result JSON. `p_lines` uses existing
  `inventory.Line` JSON (`warehouse_id`, `sku_id`, `quantity`). It is executable
  only by checkout runtime and owned by private checkout writer. Inputs are from
  trusted Go, not a public SQL command. No arbitrary URLs/provider actions.
- SQL `checkout.expire_held(uuid,bigint)` takes order ID and expected generation,
  returning one `(disposition text,retry_at timestamptz)` row: `EXPIRED`, `STALE`
  (missing/terminal/changed generation), or `NOT_DUE`. Only ordinary worker may
  execute; the private writer owns this fixed function as well.
- SQL errors: `PT400` malformed input, `PT401` invalid capability, `PT409`
  stale/conflicting state; `PT402` stock shortage maps to `command.ErrInsufficient` in
  Go. Never expose raw database details or arguments to browser/logs.

### Durable facts and immutable bindings

- New private schema `checkout` contains `orders`, `command_results`, `events`.
  `orders` stores tenant/store/owner/id, creator_session_id, cart_id/cart_version,
  quote_id/destination_id, market_id/country/service_code/service_version/
  allocation_version, currency/total_minor, commercial_state, fulfillment_state,
  generation, expires_at, job_id, snapshot, created_at/updated_at. IDs are UUID;
  generation starts at 1; job_id is unique; ID is globally unique for the worker.
- Order and reservation deliberately share one generated UUID, distinct typed
  domains. Scoped deferred FKs bind both directions, including owner and creator
  session provenance. Quote/cart/destination/service/allocation references are
  composite tenant/store/owner as applicable. SQL validates snapshot metadata,
  bounded sizes/counts, line uniqueness and exact plan-to-quote SKU quantity
  conservation. No mutable commercial/payment/provider facts inside Snapshot.
- `command_results` key is tenant/store/owner/operation/idempotency_key;
  operation fixed `checkout.begin`; columns include creator_session_id,
  request_hash (32 bytes), order_id and response JSON. It is not buyer-writable.
  Digest is SHA256 of validated Input JSON, not mutable reloaded prices.
- One active order per tenant/store/owner/cart/cart_version (DRAFT,
  AWAITING_PAYMENT or CONFIRMED) prevents new-key duplicate holds. CANCELLED
  permits a new checkout, but the old idempotency key always replays old IDs.
- `inventory.reservations` adds nullable checkout_id, buyer_owner_id,
  buyer_session_id and positive generation=1. Merchant rows have all three NULL;
  checkout rows bind the order provenance and share its ID.
- `inventory.ledger` adds checkout_id, buyer_owner_id, buyer_session_id and
  actor_kind (`MERCHANT`, `BUYER`, `SYSTEM_EXPIRY`), default MERCHANT. Only the
  merchant family has principal_id; buyer/system families retain original buyer
  provenance with NULL principal. Exact actor-family constraints, scoped FKs,
  and trigger checks are mandatory. Private UUID-derived ledger command key;
  `inventory.apply_ledger` remains the sole balance writer.
- Restrictive DB policies fence *all* existing merchant reservation state,
  reservation-line and ledger INSERT paths from checkout-owned stock. Ordinary
  buyer/issuer/identity roles cannot write any aggregate or execute these writers.
  Checkout runtime reads only scoped required projections and uses narrow row-lock
  grants with direct UPDATE checks false; no pickup evidence/principal disclosure.

### Transaction sequence and worker

1. Resolve capability; acquire transaction advisory lock
   `checkout.begin|<tenant>|<store>|<owner>|<key>` using hashtextextended(...,0).
   Read private receipt before current-state validation: same digest replays,
   different digest conflicts. Reauthenticate even on replay.
2. Acquire `checkout.cart|<tenant>|<store>|<owner>` advisory lock, then reuse
   `RevalidateQuote`. Refuse another active order for this cart/version before
   touching inventory. Derive code from `delivery:<code>` policy method.
3. Revalidate current destination, then lock service head and allocation head.
   Service must be visible, enabled, MANUAL, same market/country/currency and
   exact quoted policy revision/method, with the expected service version and
   matching destination kind. Reject API until its real dependency gates exist.
   Allocation expected version is current and nonempty; its observed service
   version is provenance, not an equality gate after a harmless service rename.
4. Lock active warehouses by sorted UUID, then all warehouse/SKU balances by
   sorted pair (absent balance means zero). Call existing `PlanAllocation` in
   configured preference order. Up to 16 warehouses x 50 SKUs = 800 plan lines;
   do not reuse merchant Reserve's 50-pair input limit or impersonate membership.
5. Recheck database clock after all waits, including quote/destination/source and
   capability expiry. Generate order UUID and enqueue private River
   `checkout_expiry_v1` args `{order_id,generation:1,version:1}` using InsertTx,
   scheduled for database-now + 15 minutes, then call `begin_hold` in SAME tx.
   Writer re-resolves capability, exact scope, final clock and immutable references;
   creates DRAFT / MANUAL_UNASSIGNED order, HELD reservation, exact lines,
   RESERVE ledger, event, receipt. It verifies the same-tx job kind/args. Hold TTL
   is database acceptance time + 15 minutes, not a buyer input. No payment job.
6. Worker uses a checked ordinary worker pool and bounded transaction. Fixed
   expiry function looks up private order, sets scope from durable provenance,
   locks order then reservation then sorted balances. Matching generation +
   DRAFT + HELD + final DB time due releases exact original lines once, marks
   reservation EXPIRED and commercial order CANCELLED and appends SYSTEM_EXPIRY
   event atomically. Stale jobs do nothing; early jobs return River JobSnooze
   based on retry_at. Pending/committed states are never released. No token,
   address, phone, merchant credential or snapshot in job args.
7. `checkout.NewExpiryWorker(ctx, workerPool)` returns a typed River worker usable
   with `river.AddWorker`. Its Args type remains private, as in T06 dispatcher.
   Worker/client lifecycle belongs to the process; no fake standalone daemon.

### Acceptance required for this unit

Real PG tests must cover role exclusivity/direct-SQL denials, exact durable
facts+job in one transaction, owner replay across sessions and input conflict,
same-cart different-key conflict, two-buyer stock=1 and reversed SKU contention,
cart/price/destination/source/service/allocation/warehouse stale gates and clock
expiry after actual lock waits, injected order/line/ledger/event/job/receipt
rollback, old merchant write paths denied, and actual River due/early/stale/
duplicate expiry with conservation. Readback and job/receipt privacy are required.
Future StartPayment race is NOT_RUN until StartPayment exists; fixture-seeded
PAYMENT_PENDING protection is only a state-fence test, not that full integration.
Public checkout UI, payment/provider sandbox/live and full SaaS gates remain open.

## Rejected alternatives and upgrade signals

- Buyer-executable complete SECURITY DEFINER command while assuming Go validation
  already ran: bypassable because buyer can insert Quote JSON; rejected.
- Recompute pricing again in PL/pgSQL or mint HMAC validation tickets just to avoid
  one pool: duplicates policy or adds key lifecycle/TOCTOU surface; rejected here.
- Reuse buyer receipts or impersonate a shared merchant principal: forgeable replay
  or lost buyer provenance; rejected.
- HELD plus runnable payment job: old TTL can release stock while payment proceeds;
  rejected. More pools/roles are not created for every module; this one exists for
  a concrete SQL authority split. Revisit only with an independently verified
  equivalent database-attested quote pipeline, not for line-count reduction.

Independent design evidence: `983eebef-5249-4b20-ac67-5f7854d241d4` and
`BeginCheckout authority recommendation: dedicated checkout runtime and Go transaction`.
