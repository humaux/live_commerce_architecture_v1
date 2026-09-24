# Delivery warehouse allocation v1

Status: FROZEN_FOR_IMPLEMENTATION, 2026-09-24, parent `669fbbb`.
This is a merchant configuration and pure allocation prerequisite, NOT a
checkout, stock hold, carrier connection, public API or payment implementation.

## Configuration

Reuse `fulfillment` configuration, merchant authentication, command receipts and
audit. `SetAllocation(ctx, tx, scope, token, key, AllocationInput)` requires
`integration:manage`; `GetAllocation(ctx, tx, scope, token, marketID, country,
code)` requires `integration:read`. Both validate exact transaction scope and
opaque credentials before any read or replay. Caller owns commit/rollback.

Input: market_id/country/code (same validation as Service), expected_version
nonnegative and below MaxInt64, expected_service_version positive, ordered
warehouse_ids (0..16 distinct canonical UUIDs). Nil and empty canonicalize to an
empty array. Output: same logical service key, version, service_version observed
when saved, ordered warehouse_ids. Authenticated principal is part of the command
digest. Identical replay returns the original revision; changed input conflicts.

Configuration belongs to the logical service, not its display name or fee revision.
The observed service version is provenance and a write CAS guard, NOT a requirement
to resave warehouse preferences after every service rename/price change. Checkout
must separately lock and validate the CURRENT service and allocation revisions.
Disabled/hidden service drafts can be configured. Empty warehouse list explicitly
clears readiness without deleting history; it cannot yield a fulfillable plan.

All selected warehouses must exist in the same tenant/store and be active at save.
Historical reads may include a now-inactive warehouse; configuration is not a
promise of current warehouse activity or stock availability. Runtime checkout must
recheck both under transaction locks. Carrier/provider IDs are not part of this
configuration; no single carrier is required.

Lock order: credential -> command receipt -> service head FOR SHARE -> allocation
advisory/head -> warehouses by UUID FOR SHARE. Do not lock policy/catalog afterward.
Use a scoped `inventory.lock_warehouse` SECURITY DEFINER read/lock helper, matching
`lock_balance`; never grant runtime warehouse UPDATE just to acquire row locks.
Checkout will lock catalogue first, then service/allocation, then sorted warehouses
and every candidate balance in warehouse/SKU order, not preference order.

Migration 0011: immutable allocation_versions + allocation_warehouses + mutable
head. Composite FKs bind service revision, actor membership, and each warehouse.
Position 1..16 is unique; warehouse is unique per revision. A deferred header INSERT
constraint checks exact count and contiguous positions at commit, including empty
list. Forced store RLS, actor-bound version INSERT; runtime SELECT/INSERT and head
UPDATE(current_version) only. No buyer/worker/issuer permissions or history edits.

## Pure planner

`inventory.PlanAllocation(warehouseIDs []string, demands []Demand, balances
[]Balance) ([]Line, error)` where Demand contains SKUID and Quantity. This is
internal arithmetic, not authentication, SQL validation or authority to hold stock.
Only the future checkout transaction may supply its revalidated quote and locked
server-selected balances. There is no client warehouse/amount input endpoint.

Validate 1..16 unique canonical warehouses, 1..50 unique canonical demanded SKUs,
quantity 1..MaxQuantity, at most 800 unique in-scope warehouse/SKU balance rows.
Validate stock columns 0..MaxQuantity, nonnegative version, and
reserved+allocated+unavailable <= on_hand. Ignore the redundant Available field:
always recompute it. Reject extraneous, duplicate or malformed rows. Missing row
means zero available, never synthetic stock. Input slices remain unchanged.

Process SKUs deterministically; greedily allocate by merchant warehouse priority,
splitting a SKU across warehouses when necessary. If ANY SKU is short, return nil
and ErrInsufficient, not a partial plan. Return positive lines globally sorted by
warehouse then SKU, each SKU sum exactly demand. Max 16*50=800 derived lines is
intentional; do not feed a valid split plan through old merchant Reserve's
50-pair canonicalizer. Reuse Line/Balance/MaxQuantity and the existing ledger when
actual checkout is implemented; this unit writes NO stock or reservation.

## Acceptance and remaining boundaries

- Real PG: create/read/append/clear, actor-bound idempotent replay, concurrent CAS,
  stale service guard, authority/revocation BEFORE replay, cross-store warehouses,
  inactive warehouses, forced RLS/FKs/grant denials, complete/ordered children,
  atomic rollback, service/warehouse row-lock interaction.
- Unit/fuzz planner: preference order differs from lock/output order; split and
  missing stock; short demand returns no partial plan; zero/overflow/duplicates/
  extraneous rows rejected; input permutations deterministic; 800-line bound;
  conservation. Full Go race/vet + actual PG regression.
- NOT_RUN: buyer checkout, locked allocation consumption and contention, expiry,
  order snapshots, public API/UI/browser checkout, provider sandbox/live, payment.
  Existing browser identity regression does not count as checkout acceptance.

Dependency: [delivery service](delivery-service-v1.md),
[buyer checkout authority](buyer-checkout-v1.md). No dependency added.
