# HARNESS-POOL-RELEASE

Status: genuine capacity RED obtained; phase-2 release source authored, GREEN pending. This is an author record, not independent
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
idle pools. The genuine CI RED below preceded the release implementation at
this same boundary. Original browser
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

Fresh fetch and worktree creation exited 0. The initial CI run is recorded below;
the repair PR and automatic checks are registered in the canvas after creation.

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

Current-source native GREEN, affected browser modes, required CI and independent
acceptance: pending. Local execution remains `NOT_RUN`.
Provider LIVE, production, screenshot pixel review and deployment: `NOT_RUN`.

## 2026-10-14 phase-2 source batch

Genuine tests-first Gates `38508131962`, attempt 1, exact SHA
`a6d47a64a65d29f2d98922e507abe11d30f1c9de`, completed FAILURE. Tracking
job `115670973680` ran `03:52:53–03:55:39Z`: the native two-family barrier
reached 55 client backends, ordinary usable 57, margin 3, twelve distinct pools
and 53 held acquisitions. After returning handles but retaining idle pools,
the guard observed 56 against limit 54 and failed. This deliberately sustained
family demand is not a historical instantaneous peak claim. The subsequent
browser phase was `NOT_RUN` after that early failure.

- Artifact `11869066888`, digest
  `a4723ab74ab7fa522870f63f97a291e2fd3a9bc127c214a86d196a788721538e`.
- Portable `evidence/capacity-red-a6.log`, SHA256
  `ac5e1237991fce643b8ee3003e4ee29b914b54af9ddd989a9bfe533dd410b14d`.
- Focused job `115670973733` SUCCESS `03:52:48–03:54:49Z`: boundary and sticky
  observation-error tests, two top-level PASS, zero FAIL/SKIP.
- Portable `evidence/guard-focused-a6.log`, SHA256
  `b85157d1eb016f51d4f7358a065c80ead7a79248d36b2f1c7f933dfad18c6a06`.
- Dispatch and artifact download exited 0. Raw downloads remain untracked.

The shared planner validates all explicit protected/owned pointers before any
close, rejects nil/alias lists with no partial plan and deduplicates in order.
It never infers a whole fixture's ownership. The native regression retains the
same demand and threshold, then releases twice, checks each pointer's exact
closed-pool error and queries protected owner/runtime successfully. Pure planner
tests cover ordering, deduplication, repeats and invalid lists. These new tests
have not run locally; final-source CI is pending.

Caller release lists follow lifetime audits `df153f8c-6fe9-4c7b-a842-a0db7e3136ba`
and `22fa6f61-7e1e-46bd-a394-05ed1d47c679`. Eight bh/bc callers release
identity/worker/expiry; BuyerOrderUI retains expiry for `/expire`. All retain
buyer runtime/issuer/checkout and shared owner/runtime. LiveClaims additionally
releases completed mi ingress/curator/registrar after the last registrar use.
Independent admin authorities and StoreDomains verifier stay protected.
Refund/ManualFulfilment/CustomersBilling release identity/worker/expiry while
retaining buyer three and shared/hosted/ingress pools. Tracking closes both
six-pool families after seeding. E2E keeps its independent live buyer/worker
lifetime and gains the watcher. Stripe sbNew releases identity/worker/expiry,
retaining buyer three, shared/hosted, registrar controls and active worker.
No generic bhSetup/bcSetup closure was added. Pointer counts are not backend counts.
The sustained watcher covers browser activity and joins before owner teardown.

Isolated phase-2 writers (assignment provenance, not provider telemetry):

- `/root/harness_buyer_release`, integration_worker, `gpt-6.1-sol/high`, base a6,
  WT `harness-pool-release-buyer`, branch `unit/harness-pool-release-buyer`;
  nine exclusive browser files, commit `9197b7773d25fe2118005a217f65ce435be7d9b5`,
  cherry-picked as `c5490e52b`.
- `/root/harness_rfx_release`, integration_worker, `gpt-6.1-sol/high`, base a6,
  WT `harness-pool-release-rfx`, branch `unit/harness-pool-release-rfx`;
  five exclusive browser files, commit `b213666e3f9484c8ed9eafb13bc759bfd67017bb`,
  cherry-picked as `8c21afb69`.
- Root owns the guard, shared release helper, native phase-2 regression and docs.
  Fresh fetch/own-remote merge, source cherry-picks and diff checks exited 0.
- E1 helper delta audit `24635cf8-327d-4357-a172-83a5b539e4df` confirms the
  every-pool closed-error assertion, no scoped P0/P1/P2; runtime remains pending.

Affected CI modes include refund-fulfilment, customers-billing, tracking-backfill,
e2e, buyer, order, merchant-buyer, live-claims, catalog-core, product-editor,
catalog-media, storefront-publish, product-media-v2, store-domains and webkit.
Full `--stripe-browser` is excluded from the CI registry and needs SANDBOX
credentials; it remains `NOT_RUN`. Separate MOCK fixture/runtime coverage must
be recorded before claiming the Stripe family verified. Do not infer fifteen
verified families from the available mode count. No assertion, timeout, capacity
configuration, retry or product change. Current-head CI and independent review
are required before READY.
