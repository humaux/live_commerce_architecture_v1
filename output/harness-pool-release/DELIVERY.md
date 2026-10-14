# HARNESS-POOL-RELEASE

Status: tests-first capacity regression authored; GitHub CI RED pending. This is an author record, not independent
acceptance. All local tests, builds and application runs: `NOT_RUN` by owner rule.

## Tests-first capacity boundary

The guard observes cluster-wide native client backends, including its one dedicated
observer connection. The limit is `max_connections - reserved_connections -
superuser_reserved_connections - 3`. Query/settings/connect/cleanup failures fail
the gate. The log calls its high-water `sampled_peak`; it cannot establish an
unsampled instantaneous peak. No connection settings or DSNs are logged.

TrackingBackfill now first runs a real two-family regression: twelve distinct owned
setup pools acquire a bounded demand until native activity reaches one above that
limit. All acquisition handles return before the guard observes the still-open
idle pools. No pool-release fix exists yet. A genuine CI capacity failure must
precede the safe release implementation at this same boundary. Original browser
assertions and timeouts are unchanged; a failed subtest stops before browser startup.

- Isolated writer `/root/harness_capacity_red`, test_worker, assigned
  `gpt-6.1-sol/high`, base `9604e4278333101125a1d1ab00e63bdf2efb572f`,
  WT `/Volumes/data/live_commerce_architecture_v1/.worktrees/harness-pool-release-red`,
  branch `unit/harness-pool-release-red`, delivered
  `bece98a0acf67ee2c23dbab23f549f132560107c` (fast-forward integrated, exit 0).
  Exclusive paths: `browser_pool_capacity_test.go` and the TrackingBackfill call-in.
- Root owns `harness_pool_capacity_test.go`: observer, boundary and sticky-error tests.
- Readonly E1 guard audit `87477349-729c-48be-b533-f135b939cd76` found no concrete
  P0/P1/P2 at SHA256 `ee543b793c05714fa269b2273ff47d561818661c867d5b88da95fd338c33bfc6`.
  E1 source review does not establish runtime acceptance.
- Planned CI-only command: `gh workflow run gates.yml --ref unit/harness-pool-release
  -f modes='["focused:^TestHarnessPoolCapacity","--browser-tracking-backfill"]'`.
  Actual run IDs and outcomes are registered in the shared canvas immediately
  after dispatch. Compilation, formatting, native PG RED and browser execution
  remain `NOT_RUN` until their actual CI receipts exist.

## Scope and provenance

- Coord: `49e18213-2573-456a-90c0-6c7cdc063e8b`;
  agent `codex-harness-pool-release`.
- Base: `9604e4278333101125a1d1ab00e63bdf2efb572f`, fetched from
  `origin/r3/integration` after PR55 merged at `2026-10-14T03:32:57Z`.
- Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/harness-pool-release`;
  branch `unit/harness-pool-release`.
- Root: integration, assigned `gpt-6.1-sol/ultra`.
- Readonly lifetime audits: `/root/harness_rfx_stripe_audit` and
  `/root/harness_buyer_catalog_audit`, explorer, assigned `gpt-6.1-sol/high`;
  same base/worktree, no write paths and no recursive delegation. These are
  parent assignment records, not independently verified provider telemetry.
- Allowed writes: `tests/foundation/**`, `output/harness-pool-release/**`.
  No product, SQL, pool-capacity configuration, assertions or timeout changes.

Fresh fetch and worktree creation exited 0. No HARNESS CI run or PR exists yet.

## Acceptance contract

1. Audit aliases, live HTTP handlers, background workers, late DB controls and
   teardown before selecting setup-only pools. Shared fixture owner/runtime and
   active buyer checkout/runtime/issuer pools remain available.
2. Reuse a small shared pool-release helper at proven browser boundaries;
   idempotent `Close` does not establish that a pool is no longer borrowed.
3. Add a permanent capacity guard with safe count/limit/margin evidence and
   explicit failure on observation errors. A sampled high-water mark must be
   labelled as sampled; it does not prove an unsampled instantaneous maximum.
4. Obtain a genuine family near-limit RED in GitHub CI before releasing pools,
   then GREEN under the same workload. No retry loop, timeout increase,
   assertion weakening or PostgreSQL capacity increase.
5. Guard positive/negative/observation-error tests, all fifteen affected browser
   modes, required current-head checks and independent review must pass.

Families: rfxNew (Refund, ManualFulfilment, CustomersBilling, E2EDealLoop,
TrackingBackfill); psSetup/stripeStore (StripeCheckout); bhSetup/bcSetup
(BuyerRealChain, BuyerOrderUI, MerchantBuyerRealChain, LiveClaimsRealChain,
CatalogCore, CatalogMedia, StorefrontPublish, ProductMediaV2RealUpload,
StoreDomains).

## Initial source observations — E1 only

- BuyerOrderUI's `/expire` control borrows expiry after startup; it cannot use
  a blanket family close list.
- The existing E2E sampler ignores query errors and only logs its sampled
  maximum. It is not an enforcing capacity guard.
- Prior CARD-PAY-FLAKE red/green receipts motivate this work but are not this
  unit's reproduction, current-head evidence or historical SQLSTATE proof.

Source implementation, guard tests, family RED/GREEN, all affected browser
modes, required CI and independent acceptance: `NOT_RUN`.
Provider LIVE, production, screenshot pixel review and deployment: `NOT_RUN`.
