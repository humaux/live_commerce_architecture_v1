# Buyer checkout options acceptance — 2026-09-25

Status: PASS_BOUNDED_PRIVATE_OPTIONS. This is private buyer discovery, not a public
browser storefront, payment-provider connection, stock or delivery guarantee.
Automatic product payment entry PE01–PE10 remains NOT_RUN end-to-end.

## Scope and provenance

- Frozen contract: `6bd8910`, [buyer checkout options](../../contracts/buyer-checkout-options-v1.md).
- Read-only inventory `808994a8-95bd-4853-9e15-8ebde2db9194`; independent preflight
  `1d3b145d-e267-4dbd-a29f-171d7ebf26fa` found no P0/P1/P2 in draft `3ca7043`.
- Author settings_ui_impl, commerce_worker, actual assigned `gpt-6-sol/high`;
  isolated `/Volumes/data/live-commerce-buyer-http-20260925`, branch
  `commerce/buyer-options-20260925`, base `6bd8910`, source `a647be4` plus
  comment-only `8523611`, cherry-picked as `c2cb43d` and `83e5ed6`.
- Root independently authored foundation tests `ef1f0e0`, corrected their fixture
  and expected errors in `197b2ad`. Final source/test state is `83e5ed6`.

The endpoint reuses checkout.Service's validated ordinary checkout runtime pool,
buyer.WithScope and final capability recheck. One SELECT joins current scoped
market/policy/service/allocation/active warehouses. The safe concrete 15-field
projection supplies Quote/Begin input and saved three-language labels. Composite
keyset pagination does not expose credentials, warehouse IDs, bindings or actors.
No new migration, role, grants, dependency, provider call or inventory write.

## Actual evidence

All logs: `/Volumes/data/output/live-commerce-buyer-options-tests/`.

| Run | Result | SHA256 |
| --- | --- | --- |
| author-unit-race.log | exit0; checkout + buyerhttp race | `b76d3c7c927eb21a9c7b25720dcd8088045508764e899b5154d6941e3c406935` |
| author-vet.log | exit0; scoped vet | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` |
| root-targeted-initial.log | exit1; new test setup errors, foundation7.341s | `884d869c9e4d3741fea8c3b554b81264db33035a3b5346737d5e830ea465c6c4` |
| root-targeted-corrected.log | exit0; 12 top-level PASS/0 FAIL/0 SKIP; foundation9.208s | `0574aaf801f63c7113032c2a7c043514dbf6bb05e5b5eb9ebbe9d55ef687f1a2` |
| root-full.log | actual exit0; 353 top-level PASS/0 FAIL/0 SKIP; foundation143.747s | `ec3393d99032e6af4a5a2c66d71add0d439cb2844c082af6c0b87288c4fc7914` |

Root commands: `GOFLAGS=-p=1 bash scripts/dev/test-local.sh --buyer-http`, then
`GOFLAGS=-p=1 bash scripts/dev/test-local.sh`. Each uses its own disposable real
PostgreSQL fixture. Full mode includes `go test -race -count=1` and `go vet ./...`.
Root observed actual process exit0 (session98690), not only the final log marker.
Postflight `docker ps -a --filter label=livecommerce.fixture` returned no fixtures.
The targeted subset is five new foundation tests and seven previous HTTP/catalog
tests, not twelve new independent product capabilities.

Five new tests verify bounded two-page traversal, optional filters, fresh-store
and tenant separation, a positive other-store read, scoped cursor rejection and
zero durable purchase-fact deltas. They cover hidden/disabled/API drafts, stale
and disabled policy, restoration, empty allocation, one inactive warehouse among
two, inactive market, current service rename with historical allocation provenance,
strict query/body/header/method/auth/revocation boundaries and exact JSON fields.
The home flow consumes returned market/method/service/allocation revisions through
real HTTP Quote, destination, Begin and order readback. CVS choices are discovered,
but missing trusted pickup_id is rejected, not fabricated. No PSP is invoked.

## Initial failure and causal correction

All four failed top-level new tests arose from three root-test setup mistakes:

1. New expectations said `invalid` rather than the established `invalid_request`.
   Corrected the tests, not the product error contract.
2. Fresh isolation stores lacked ResolveScope's `store:read` grant. Added only
   those synthetic store grants and cleanup; old storeA2/storeB controls untouched.
3. Changing a store's currency under an existing market was rejected by the
   existing composite FK, SQLSTATE23503. Assert that stronger invariant and the
   unchanged USD choice; do not disable constraints to construct impossible data.

Initial RED log is retained. No old assertion or product protection was relaxed.

## Review, maintenance and boundaries

Final independent source/evidence review: PASS for `83e5ed6`, no remaining
P0/P1/P2; Humaux `ba98ca3d-9cfd-4aaf-b4b2-5c44fc91d0e3`. The reviewer independently
read source, verified the root logs/counts/hashes and the causal test correction;
it did not rerun PostgreSQL. Six changed Go files indexed, 142 entities;
ListOptions linked to the preflight decision evidence. The full fixture postflight
is recorded above; original failed evidence is retained.

Dependencies: existing checkout runtime permissions from migration0013, pricing
current-version heads, fulfillment allocation provenance contract and private
buyer HTTP trust boundary. Future changes must retain every assigned warehouse's
active check and must not require historical allocation.service_version to equal
the current service revision. Comments next to SQL and the position-only digest
explain these non-obvious maintenance decisions.

Still NOT_RUN: public BFF/session bootstrap/cookies/CSRF/rate controls, buyer visual
comps and three-language browser flow, trusted CVS map/selection source, automatic
merchant share-link output, provider payment-page creation, real sandbox payment,
signed settlement callbacks and complete SaaS release gates. No customer live
stream, production system, real provider configuration or funds changed.
