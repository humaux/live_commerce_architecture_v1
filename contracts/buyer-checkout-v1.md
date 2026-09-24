# Buyer checkout v1 — authority decision and implementation gate

Status: AUTHORITY_DECISION_FROZEN; aggregate schema and delivery/expiry contracts
PENDING, 2026-09-20, based on `909cb3b` and prerequisite `dc73e67`.
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
  cart/Quote → market/current policy → sorted products/SKUs → destination/source →
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

Freeze destination-directory provenance, merchant service revisions and server
allocation configuration, with explicit MANUAL versus API eligibility gates.
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
