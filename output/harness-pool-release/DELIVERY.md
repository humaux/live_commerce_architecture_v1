# HARNESS-POOL-RELEASE

Status at 2026-10-14 05:56 UTC: registration-only commit
`5f59affae1cc60342680070c141d7957642b9c41` has Foundation SUCCESS
(`38517428884`, job `115699361962`, actual `05:35:37–05:40:50Z`), including
213/213 Node checks. Its raw log SHA256 is
`3bac248e2749890b0d6ec31c3623ea330df9968c1eae6de41421e23671d80e6f`.
The independent round-1 review of pool-release source `70c61301` found a watcher
shutdown P1; the lifetime audit itself was clean. Its controlled regression is now
genuinely RED in CI; the source repair below awaits new exact-head CI. This is an author record, not independent
acceptance. All local tests, builds and application runs: `NOT_RUN` by owner rule.

## Tests-first capacity boundary

The guard observes cluster-wide native client backends, including its one dedicated
observer connection. The limit is `max_connections - reserved_connections -
superuser_reserved_connections - 3`. Query/settings/connect/cleanup failures fail
the gate. This sustained guard covers the **post-release browser/retained-service
phase only**, not normal fixture setup or teardown. The deliberate native setup
load below is a separate bounded regression. The log calls its high-water `sampled_peak`; it cannot establish an
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
- Allowed writes: `tests/foundation/**`, `output/harness-pool-release/**`, plus the
  owner's narrow `scripts/dev/shard-plan.json` test-name registration exception.
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

## Round-1 review repair — pending CI

The reviewer found that canceling the watcher context can abort an in-flight
query and invalidate its connection before the final sample. The repair uses
signal-only shutdown, a join before the final sample, independent bounded query
contexts, and a deterministic mid-query stop regression. Only a timed-out
observation may be retried once on a fresh connection; a valid over-limit reading
remains a sticky failure without retry. This does not retry a browser action.

P2 changes also require a two-second native headroom recovery assertion, removal
of direct puddle imports, CardPayments conversion with its original exact 24-pool
release set, and actual Stripe `sbNew(MOCK)` lifetime proof in normal Tracking CI.
The registration-only commit is preserved separately; the cap remains 60.

Genuine stop regression: exact `08121b2831e3a35f9263b82f7f722a2cd4db8662`,
Gates `38518508850`, focused job `115702744696`, actual `05:48:26–05:50:34Z`.
It reached all three expected assertions and exited 1 (one top-level FAIL, no
PASS/SKIP). This models cancellation fallout with a controlled fake observer;
it does not claim a native PG cancellation reproduction. Portable receipt:
`evidence/stop-query-red-08121.log`; full raw SHA256
`de9897949d18f02f3e063419c2fe019b157d8bcb505ff3a0cbac0be785ef98f1`.
The unchanged mid-query/final-sample assertions now exercise signal-only stop,
idempotent stop calls and join. Timeout recovery tests cover one successful
replacement, repeated timeout, replacement failure, and no retry for a real
breach, invalid reading or other query error. No browser action is retried.

P2-4 deliberately uses the owner's permitted operational oracle: failed Acquire
with a still-live bounded context and zero sessions after real Close, plus
protected-pool SELECT 1. It does not assert a specific ErrClosedPool sentinel.
Direct puddle imports were removed without changing go.mod. Historical receipts
below describe the earlier exact-error assertion as it actually ran at 70.

Round-1 isolated assignments (parent dispatch, not verified provider telemetry):

- `/root/harness_stop_query_red`, test_worker, assigned `gpt-6.1-sol/high`,
  base 5f59, WT `harness-pool-stop-red`, branch `unit/harness-pool-stop-red`,
  exclusive `harness_pool_capacity_test.go`, delivered `08121b2831`;
  root integrated as `dd809bcb1` before the minimal stop fix. Child runtime
  model identity was unavailable; the assigned tier is recorded truthfully.
- `/root/harness_card_shared_release`, integration_worker, assigned
  `gpt-6.1-sol/high`, base 5f59, WT `harness-pool-card`, branch
  `unit/harness-pool-card`, exclusive `browser_card_payments_test.go`,
  delivered `ccdd6143e54c35e04379b900231500c801dc70e9`, integrated as
  `2cda63aff`. Its original 24 setup-only pointers and protected live borrowers
  are unchanged; it now calls the shared helper and starts the watcher after
  admin facts and release, before buyer browsers. Single and eight-context
  browser assertions and existing deadlines are unchanged. E1 boundary audit
  `855eafc0-8536-4b77-ba40-fd79b70a689b`; current normal-card runtime pending.

Separate Stripe MOCK proof is permanently wired through the existing normal
TrackingBackfill mode (before the main fixture), directly exercising sbNew(MOCK),
retained issuer/runtime/checkout, registrar toggles and hosted-worker REDIRECT.
The new final SHA must produce that receipt; full Stripe SANDBOX/LIVE is NOT_RUN.

Other named harnesses remain unconverted and require their own lifetime audits:

| Harness family | Why no speculative pool closure is added here |
| --- | --- |
| live_tools | Poller, consumer, ingress and buyer/admin API remain borrowers. |
| input_delivery | Two registration fixtures, socket/edge/API and child processes need a joint lifetime audit. |
| studio_ui / studio_bff | Separate authority, API/media workers and child processes need individual boundaries. |
| buyer_comms | `bgNew`, control server and later database assertions need lifetime review. |
| merchant_orders_ui | `pqSetup` is shared by normal/focused paths; no release boundary is established here. |
| live_console_comments_fixture | `lbSetup` lifetime boundary has not been audited. |
| identity chain / meta_connect / migration_import / store_design | Shared-fixture handler/authority lifetimes have not been audited individually. |

E2E retains guard-only treatment: it already closes setup identity/registrar/curator
pools and still uses its independent buyer and worker pools. The earlier observed
33 is a sampled browser high-water, not proof that any additional pool is dead.

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

The pending statements in this section describe the phase-2 submission time;
the timestamped completed-round receipt below supersedes those statuses.

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

## Completed phase-2 CI round — exact SHA 70c61301

Gates `38509983875`, attempt 1, completed SUCCESS at `2026-10-14T05:21:17Z`:
80 completed jobs, no failures. The actual checkout was synthetic merge
`7027b5b746c60ef2efd8f3434b868cfc860197fe`, merging the full SHA above into
`9604e4278333101125a1d1ab00e63bdf2efb572f`. These receipts prove this SHA;
they do not certify the subsequent registry/Stripe MOCK source batch.

Tracking job `115676289205` ran `04:47:18–04:51:45Z`. The same twelve-pool
barrier reached 55 with 53 held acquisitions. Returning handles and releasing
each pool twice left two native client backends; the enforcing guard sampled
3 against limit 54. Each closed-pointer error and both protected SELECTs were
asserted before PASS. The original browser then passed fifteen shipments and
audit rows, sampled 14 across 442 observations, and final Go PASS `123.415s`.
Independent exact-workload receipt audit:
`b3858c4b-b9a0-4d2e-8d42-b7305f4aefe6` (E3, scoped to these receipts).
Portable excerpt: `evidence/capacity-green70.log`; raw log SHA256
`adfd56f3bf7428ce57f2506f1c2980fc806ee9e7948017807eea2a97e6bfb853`.

Affected runtime jobs all succeeded. Observed sampled high-water marks (all
against limit 54; these are not exact unsampled peaks):

| Family/mode | Job | Sampled high-water |
| --- | --- | --- |
| StoreDomains | 115676288835 | 15 |
| CatalogMedia | 115676288849 | 17 |
| BuyerOrderUI | 115676288807 | 12 |
| CustomersBilling | 115676288846 | 25 |
| ManualFulfilment / Refund | 115676288872 | 21 / 22 |
| BuyerRealChain | 115676288997 | 10 |
| MerchantBuyerRealChain | 115676289073 | 16 |
| E2EDealLoop | 115676288948 | 33 |
| WebKit Buyer / Order / Merchant | 115676289087 | 11 / 12 / 14 |
| TrackingBackfill | 115676289205 | 14 |
| LiveClaimsRealChain | 115676289133 | 16 |
| StorefrontPublish | 115676289209 | 16 |
| CatalogCore | 115676289972 | 11 |
| Product editor | 115676290045 | 10 / 10 |
| ProductMediaV2RealUpload | 115676290368 | 16 |

Independent immutable receipt audits: `7a0f7cb1-b33f-4cdf-b362-170f9384f1ac`,
`fbb52f97-20bf-481a-9939-4396cd71d785`,
`f982a7d8-5814-4775-8585-d78c8ce0beef`,
`f0f5c15b-10c6-46b1-9c9f-0773ad7de554`,
`77e59943-7065-484e-b2a9-728ef7bfd03c`. Editor finished Go PASS `170.400s`
(one top-level, two subtests); ProductMedia finished Go PASS `62.824s`
(one top-level). Neither emitted test FAIL/SKIP. ProductMedia also emitted 34
TLS unknown-certificate handshake errors; their cause/expectedness was not
verified by the receipt audit, so this is not a claim of error-free logs.
WebKit's seven mode summaries passed;
only the three affected Buyer/Order/Merchant JSONLs were independently inspected.
Historical SANDBOX/LIVE/upgrade skips stay `NOT_RUN`. Screenshot pixels and real
provider/DNS/deploy acceptance are not established.

Foundation `38509983803`, job `115676027878`, failed at `04:15:04Z`:
Node tests 212 PASS / 1 FAIL / 0 SKIP. The shard-plan drift test counted 58
unregistered actual tests plus three synthetic names, exceeding the unchanged
cap of 60. The owner's repair registers only the three new native capacity
tests, sorted in existing g06: Boundary, ObservationFailureIsSticky, ReleasePlan
(all prefixed `TestHarnessPoolCapacity`). They already executed through catch-all
in successful g06 job `115676288772`, including fifteen new pure subcases.
Execution selection, estimates, groups, generator, cap and assertions remain
unchanged. Registration does not exclude a test or raise the cap. Portable RED:
`evidence/foundation70-shard-plan-red.log` SHA256
`ad5711df5940fbca5f4b92dcd1f8d81af4df5a75fdd027f7b697f8167ac39158`.

The separate permanent Stripe MOCK lifetime gate is assigned to isolated
`/root/harness_stripe_mock_lifetime`, test_worker, `gpt-6.1-sol/high`, base full70,
WT `/Volumes/data/live_commerce_architecture_v1/.worktrees/harness-pool-release-stripe-mock`,
branch `unit/harness-pool-release-stripe-mock`. Its exclusive paths are the new
`browser_stripe_pool_lifetime_test.go` and a TrackingBackfill subtest insertion.
Delivered commit `fc38f4c71cf04d082fc8ad462940c909db01ba78`, fast-forward
integrated (exit 0). It reuses `sbNew(MOCK)`, checks the three released pointers and retained
issuer/runtime/checkout/registrar/hosted-worker behavior. The Tracking mode
permanently invokes it before its main fixture. Source SHA256:
`455c4ce7db4eafd3b7a8b94947cd8b618c71d6384939eef135651f399011d386`
(helper), `bb4f4bf70d21361403bc4fcda5d41b813b99645f24919129c03d75ffc4557443`
(Tracking). Browser-tagged standalone tests are outside the existing untagged
shard registry; the nested call provides permanent normal-mode execution.
This batch's compilation, formatting and current-head CI remain `NOT_RUN`
until the next timestamped receipt; original Stripe SANDBOX
guards stay intact. Assignment records are not provider runtime telemetry.
