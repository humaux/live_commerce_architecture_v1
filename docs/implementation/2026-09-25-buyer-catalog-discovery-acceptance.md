# Buyer catalog discovery acceptance — 2026-09-25

Status: PASS_BOUNDED_PRIVATE_CATALOG. This increment is a private
read-only dependency, not a public shop, automatic payment link, or PSP approval.
Automatic product payment entry PE01–PE10 remain NOT_RUN end-to-end.

## Scope and provenance

- Requirement/contract: `4043740`, follow-up total-protection note `bcb51d4`;
  [product payment entry](../../contracts/product-payment-entry-v1.md) and
  [catalog discovery](../../contracts/buyer-catalog-discovery-v1.md).
- Author: `settings_ui_impl`, commerce_worker, assigned `gpt-6-sol/high`, base
  `4043740`, isolated `/Volumes/data/live-commerce-buyer-http-20260925`, branch
  `commerce/buyer-catalog-20260925`, source commit `77586f1` -> main `9959a97`.
  Five source/unit files in storefront and buyerhttp only; no schema/dependency.
- Root separately authored real PG/HTTP tests `86b1831`; final test isolation
  correction `286bfa0`. Actual final source is `286bfa0`, before documentation.
- Design reviews: `779aa68e-b3e9-4220-ae72-54881eae13e0` for automatic payment
  entry, `79b1f977-250d-4c6c-86cf-d4ac5ec28dee` for catalog preflight.

## Implemented boundary

`GET /v1/buyer/catalog` uses the existing private BFF secret, per-request exact
published-origin resolver and scoped buyer capability. Only this canonical GET
admits bounded `product_id`, `limit`, `cursor`; old routes still reject query.
One joined ordinary-role query returns active product/SKU/current-currency data
with seven concrete display fields. An SHA256-bound keyset cursor is a position,
not authorization; it contains no raw tenant/store IDs or buyer token.

No schema, grants, catalog write, cart/order/stock mutation or provider request
was added. The displayed price is not an immutable Quote or a payment promise.

## Actual test results

Log directory: `/Volumes/data/output/live-commerce-buyer-catalog-tests/`.

| Run | Actual result | SHA256 |
| --- | --- | --- |
| Author unit race | exit 0; storefront + buyerhttp | `3062a5f05138ac07e3281fd086947ff169cf0609fc1223fd758ba535cba530a9` |
| Author old HTTP subset | exit 0; excludes root's new BCAT tests | `0818e3d07751c853c87b3069815b8f17db81757642ea4c41120a96b8b8cf4cc3` |
| Root targeted initial | exit 0; 7 top-level PASS, foundation 5.487s | `6362b95b4138b77063263ee6525abd8bc152073b77eb7b07d21a54417dbbf2cd` |
| Root full initial | exit 1; legacy T04 empty-store control contaminated, foundation 142.683s | `2106ea200ea9a35f7dcabc4c9a50bd15d9c519ffc056c44f421eaa1239b10f22` |
| Root targeted isolated | exit 0; 7 PASS / 0 FAIL / 0 SKIP, foundation 5.231s | `1926d655a210d431853fb003ffcfd745a08ab5ef43357eb2f4e429dede3eac7f` |
| Root full isolated | exit 0; 345 PASS / 0 FAIL / 0 SKIP, foundation 136.359s | `0d54114a827999cb1d375fa2ed57d00755d42df8e9ed52c58122b1710b56bb3c` |

Root commands: `GOFLAGS=-p=1 bash scripts/dev/test-local.sh --buyer-http` and
`GOFLAGS=-p=1 bash scripts/dev/test-local.sh`. The full script runs real isolated
PostgreSQL, `go test -race -count=1` and `go vet ./...`; no developer database.
Root observed the full command's actual exit 0 (session 86127), not only the log
tail. The owned-fixture cleanup marker is present; postflight using the script's
exact `livecommerce.fixture` Docker label returned no remaining containers.

The three new real HTTP/PG tests cover two-page/filter traversal, display field
preservation, exact JSON keys, cross-store/tenant exclusion and a separate-store
positive control, scoped-cursor rejection, real merchant price/archive readback,
no durable purchase-fact deltas, strict query/body/method/authority errors,
unpublished origin/revoked capability, and old-route query rejection.

## Red result and root-cause repair

The initial new test reused shared fixture `storeA2/storeB` with `t04CreateStock`.
The existing T04 gate intentionally reserves them as empty negative controls;
the new data therefore broke its assertion. No product RLS defect was found.
The correction creates two test-owned UUID stores in the disposable cluster and
seeds only the product/SKU rows needed for read isolation. It adds a positive
read of that second store. Existing gates/thresholds are unchanged; the original
failure log remains. The owned cluster, not any customer database, is disposable.

## Independent review and remaining gates

Final review: PASS for source `286bfa0`, no remaining P0/P1/P2; Humaux evidence
`4d930731-7222-429b-a8f9-1633c9816935`. The reviewer independently checked the
full and targeted log counts/hashes and verified the causal fixture correction;
it did not independently rerun the PG suite. It passed storefront race tests;
its separate buyerhttp binary stalled before Go entry (`_dyld_start` sample),
then its own process was terminated and reported ENVIRONMENT_BLOCKED/NOT_RUN,
not PASS. This does not substitute for the root's actual full regression.

Six changed Go files were indexed (131 entities), then the corrected foundation
file re-indexed (19 entities). `ListCatalog` links to the automatic-entry decision;
`bcatForeignStore` links to the causal fixture correction. No customer/production
processes, provider accounts, payment links or live payments were changed.

Still NOT_RUN: public BFF/bootstrap/cookie/CSRF/rate controls, public catalog/SEO,
approved buyer visual comps/three-language pages, automatic merchant share-link
output, market/delivery discovery, Stripe server adapter, actual sandbox payment,
webhook settlement and full SaaS release gates. The prior HKD10 operator-created
Stripe link is not evidence for those features.
