# Checkout quote revalidation v1

Status: FROZEN_FOR_IMPLEMENTATION, 2026-09-20, baseline `909cb3b`.
Mandatory executable prerequisite of BeginCheckout; not checkout/fulfillment/payment
acceptance. Extends `cart-quote-v1.md`, reusing the one existing pricing calculator.

## Surface and ownership

`storefront.RevalidateQuote(ctx, tx, buyer.Scope, quoteID string, cartVersion int64)
(Quote,error)` operates inside the caller's already authenticated buyer transaction.
It introduces no pool, credential, receipt, inventory write, dependency or public route.
The returned Quote is a verified snapshot for that transaction, **not a reusable
authorization ticket**. No client price, SKU list, warehouse, tenant or provider input.

Validate canonical quoteID, positive cartVersion and `buyer.CheckScope` first.
Read owned immutable Quote with existing GetQuote relational consistency checks;
other owner/store or absent ID -> ErrNotFound, malformed input -> ErrInvalid.
Lock owned cart FOR SHARE, current active market FOR SHARE, enabled current policy
head FOR SHARE, all products then all SKUs in sorted order using existing helpers.
Do not acquire merchant command locks or perform network I/O in this transaction.

Require quote/cart ID, currency, caller expected version and current cart version
to agree. Cart must be nonempty and at most 50 canonical distinct valid SKU/quantity
entries. Require exact market version, currency and active status, exact complete
policy (not just its version), current active catalog identity/version/price/text
and quantities. Recalculate with existing `pricing.Calculate`, compare every line
and aggregate amount, including discount and shipping tax; no silent repricing.
Any stale version/content/amount or expired/future-dated snapshot -> ErrConflict.
Missing/disabled policy may retain existing ErrNotFound; no raw SQL/provider errors
become user-visible claims. A snapshot mismatch is rejected, not repaired.

Check DB clock after **all** locks/calculation, not before a lock wait. CreatedAt
must be no later than that clock, ExpiresAt must still be future and consistent
with the immutable policy TTL. Historical GetQuote behavior remains unchanged.
Locks last through the caller transaction; no snapshot read guarantees wall-clock
expiry throughout later waits. BeginCheckout must recheck expiry immediately before
its first durable hold write after destination/allocation/balance waits; that write
and all facts must commit/rollback together. This helper alone grants no extra SQL
authority and does not verify destination deliverability.

## Gate

Real isolated PG with existing ordinary buyer/merchant roles:

1. Matching snapshot is returned unchanged; no receipts/events/reservations/jobs
   or other durable facts added, and normal historical GetQuote still works.
2. Cross owner/store, malformed ID/version, mismatched typed scope and missing
   Quote fail closed without durable writes.
3. Changed cart, active/catalog/version/currency/price, market/version, policy
   head/disabled, forged policy/lines/aggregate values, future/expired/incorrect
   TTL snapshot all fail; integer calculation remains the existing implementation.
4. Real second connections demonstrate cart/policy/catalog locks hold until the
   caller ends the transaction. Concurrent change before lock release causes
   revalidation to reject; quote expiring while waiting must reject after the wait.
5. Cancellation/rollback releases locks and leaves the connection reusable with
   no scope contamination. Full real-PG/race/vet regression and independent review.

Buyer runtime is trusted application DB authority, not an end-user connection;
its ability to insert Quote rows does not make every JSON amount validated. This
helper explicitly checks them. The future checkout writer authority must be reviewed
separately; a direct SQL command cannot assume a caller previously used this helper.
