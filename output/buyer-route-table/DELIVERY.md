BLOCKED — E1 source/compile only; CI tests and independent acceptance NOT_RUN.

# BUYER-ROUTE-TABLE delivery

Source SHA: `6f04a2aa61ab981166bcf24d52986e9f69a0e86b`. Base: `e4dad98cc322d35709b475de25063efc92e5d866`.
Branch: `unit/buyer-route-table`. Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/buyer-route-table`.
Role: integration_worker (write). Executor: Codex, reasoning high (as dispatched; runtime does not expose an independently verified model id). No child agents.
Coord task: `8569112d-4a60-4fd1-8763-78c6172f39ff`.

The 46 buyerhttp METHOD+path entries now resolve statically. The zero-UNRESOLVED goal for the entire `/v1/buyer` namespace is **not achieved**: the separate feed mount still contributes two findings. The first static scan also exposed 16 NEW GO_ONLY errors. Per the task's stop condition, code changes stopped here; no contracts or baseline were changed. Integrator ruling is required before further implementation or CI acceptance.

## Checkpoints

| Step | Commit | Result |
| --- | --- | --- |
| Table + dispatch | `2066b64a6` | One path/method inventory; original service dispatch kinds retained |
| Buyer tests | `1d82bee51` | Frozen base matcher/method oracle, table walk and unlisted-path test; execution NOT_RUN |
| Scanner | `5ebbfe69e` | Generic marker, consumption checks, table-based owning mount association |
| Scanner tests | `6f04a2aa6` | Real CLI fixtures for resolution, unserved tail, no marker, unrelated mount; execution NOT_RUN |

## Route inventory

Patterns below are the runtime patterns. Contractdrift normalizes each whole parameter segment to `{}`. Details are also in `route-table.json`.

| Method | Path | Handler (existing dispatch branch) | Declaration |
| --- | --- | --- | --- |
| GET | `/v1/buyer/session` | `dispatch: IssueForTrustedStore / scoped session check / Revoke` (`sessionRoute`) | `routes.go:36` |
| POST | `/v1/buyer/session` | `dispatch: IssueForTrustedStore / scoped session check / Revoke` (`sessionRoute`) | `routes.go:37` |
| DELETE | `/v1/buyer/session` | `dispatch: IssueForTrustedStore / scoped session check / Revoke` (`sessionRoute`) | `routes.go:38` |
| POST | `/v1/buyer/session/bootstrap` | `dispatch: RegisterForTrustedStore` (`bootstrapRoute`) | `routes.go:39` |
| POST | `/v1/buyer/session/retire` | `dispatch: RetireForTrustedStore` (`retireRoute`) | `routes.go:40` |
| GET | `/v1/buyer/design/published` | `designGet` (`designPublishedRoute`) | `routes.go:41` |
| GET | `/v1/buyer/design/preview` | `designGet` (`designPreviewRoute`) | `routes.go:42` |
| GET | `/v1/buyer/storefront/primary-origin` | `primaryOriginGet` (`primaryOriginRoute`) | `routes.go:43` |
| GET | `/v1/buyer/catalog` | `dispatch: storefront.ListCatalog` (`catalogRoute`) | `routes.go:44` |
| GET | `/v1/buyer/checkout-options` | `dispatch: checkout.ListOptions` (`optionsRoute`) | `routes.go:45` |
| GET | `/v1/buyer/cart` | `dispatch: storefront.GetCart / SetCart` (`cartRoute`) | `routes.go:46` |
| PUT | `/v1/buyer/cart` | `dispatch: storefront.GetCart / SetCart` (`cartRoute`) | `routes.go:47` |
| POST | `/v1/buyer/quotes` | `dispatch: storefront.CreateQuote` (`quotesRoute`) | `routes.go:48` |
| GET | `/v1/buyer/destination` | `dispatch: CurrentDestination / SetDestination` (`destinationRoute`) | `routes.go:49` |
| PUT | `/v1/buyer/destination` | `dispatch: CurrentDestination / SetDestination` (`destinationRoute`) | `routes.go:50` |
| POST | `/v1/buyer/checkout` | `dispatch: checkout.Begin` (`checkoutRoute`) | `routes.go:51` |
| GET | `/v1/buyer/orders` | `dispatch: checkout.ListOrders` (`ordersRoute`) | `routes.go:52` |
| GET | `/v1/buyer/claim-link` | `claimRequest` (`claimLinkRoute`) | `routes.go:53` |
| POST | `/v1/buyer/claim-link/redeem` | `claimRequest` (`claimRedeemRoute`) | `routes.go:54` |
| GET | `/v1/buyer/privacy` | `privacyGet` (`privacyRoute`) | `routes.go:55` |
| PUT | `/v1/buyer/consents` | `consentPut` (`consentsRoute`) | `routes.go:56` |
| POST | `/v1/buyer/privacy/export` | `privacyExport` (`privacyExportRoute`) | `routes.go:57` |
| POST | `/v1/buyer/privacy/erasure` | `privacyErase` (`privacyErasureRoute`) | `routes.go:58` |
| POST | `/v1/buyer/cvs-selections` | `cvsRequest / cvsServe` (`routeCVSSelectionOpen`) | `routes.go:59` |
| POST | `/v1/buyer/cvs-stores` | `cvsRequest / cvsServe` (`routeCVSStoreEnter`) | `routes.go:60` |
| POST | `/v1/buyer/cvs-selections/{id}/verify` | `cvsRequest / cvsServe` (`routeCVSSelectionVerify`) | `routes.go:61` |
| GET | `/v1/buyer/cvs-selections/{id}` | `cvsRequest / cvsServe` (`routeCVSSelectionGet`) | `routes.go:62` |
| PUT | `/v1/buyer/orders/{id}/bank-transfer/proof` | `transferRequest` (`routeTransferProof`) | `routes.go:63` |
| GET | `/v1/buyer/orders/{id}/bank-transfer` | `transferRequest` (`routeTransferGet`) | `routes.go:64` |
| POST | `/v1/buyer/orders/lookup` | `orderLookup` (`routeLookup`) | `routes.go:65` |
| POST | `/v1/buyer/orders/link` | `orderLink` (`routeOrderLink`) | `routes.go:66` |
| GET | `/v1/buyer/media/s/{image}` | `designGet / storeMedia` (`storeMediaRoute`) | `routes.go:67` |
| GET | `/v1/buyer/catalog/v2/products` | `catalogV2Get` (`catalogV2ProductsRoute`) | `routes.go:68` |
| GET | `/v1/buyer/catalog/v2/collections` | `catalogV2Get` (`catalogV2CollectionsRoute`) | `routes.go:69` |
| GET | `/v1/buyer/catalog/v2/products/{slug_or_id}` | `catalogV2Get` (`catalogV2ProductRoute`) | `routes.go:70` |
| GET | `/v1/buyer/catalog/v2/collections/{slug}` | `catalogV2Get` (`catalogV2CollectionRoute`) | `routes.go:71` |
| GET | `/v1/buyer/media/c/{id}/{image}` | `collectionMediaGet` (`collectionMediaRoute`) | `routes.go:72` |
| GET | `/v1/buyer/media/p/{id}/{image}` | `mediaGet` (`mediaRoute`) | `routes.go:73` |
| POST | `/v1/buyer/orders/{id}/payment/prepare` | `paymentRequest` (`paymentPrepareRoute`) | `routes.go:74` |
| POST | `/v1/buyer/orders/{id}/payment/handoff` | `paymentRequest` (`paymentHandoffRoute`) | `routes.go:75` |
| POST | `/v1/buyer/orders/{id}/payment/refresh` | `paymentRequest` (`paymentRefreshRoute`) | `routes.go:76` |
| POST | `/v1/buyer/orders/{id}/payment/cancel` | `paymentRequest` (`paymentCancelRoute`) | `routes.go:77` |
| GET | `/v1/buyer/orders/{id}/payment` | `paymentRequest` (`paymentRoute`) | `routes.go:78` |
| GET | `/v1/buyer/quotes/{id}` | `dispatch: storefront.GetQuote` (`quoteRoute`) | `routes.go:79` |
| GET | `/v1/buyer/destinations/{id}` | `dispatch: GetDestination` (`destinationItemRoute`) | `routes.go:80` |
| GET | `/v1/buyer/orders/{id}` | `dispatch: checkout.Get` (`orderRoute`) | `routes.go:81` |

## Behaviour-equivalence argument

This is source reasoning plus unexecuted tests, **not a runtime-equivalence PASS**.

| Dispatch branch | Preserved behaviour and reason |
| --- | --- |
| Namespace interception | `cmd/api/buyer.go:40` now calls `Routes.HasPrefix` for the original and `path.Clean` paths. `routes.go:27` returns the identical `strings.HasPrefix(path, "/v1/buyer")`; original path is still passed unchanged. Disabled/nil handling and fallback are unchanged. |
| Exact session/catalog/options/cart/quote-create/destination/checkout/orders/claims/privacy/design/primary-origin paths | Literal values or the same existing constants are table entries; exact segment comparison substitutes the old exact-path switch. No suffix is accepted. |
| CVS | Same exact create/store paths and single nonempty ID selection/verify suffix. Compatibility match/allowed helpers use the table; CVS service code, keyless verify and 201 responses are unchanged. |
| Transfer | Same single nonempty order ID plus bank-transfer/proof suffixes, GET vs PUT unchanged. The existing transferRequest and SQL/service calls are unchanged. |
| Lookup and order link | These exact entries remain before generic `/orders/{id}`, so `lookup` and `link` remain their own POST routes rather than order IDs. |
| Store/product/collection media | Same nonempty segment count and field assignment (`id`/`image`). No cleaning, percent decoding or extra slash. UUID validation remains in ServeHTTP **after** method validation, preserving 405 vs 422 precedence. |
| Catalog v2 | Same product/collection list paths. Product keys retain `(slug grammar and <=80 bytes) OR command.ValidID`; collection slugs retain slug grammar, <=80 bytes and UUID exclusion. Invalid keys still fail matching as 404. |
| Payment suffixes and generic quote/destination/order IDs | Same payment suffix precedence before generic order ID; same nonempty single-segment extraction. GET payment view and POST prepare/handoff/refresh/cancel are table methods. Generic invalid UUIDs still reach the existing 422 check. |
| Method allowance | `allowed` now checks table kind+method. No implicit HEAD/OPTIONS/redirect. Multi-method session/cart/destination entries retain the old method set. |
| Service dispatch and errors | Existing handler.dispatch, paymentRequest, cvsRequest/cvsServe, transferRequest and other service branches retain their code. Payment-disabled 404, CVS-disabled 404, body/key validation, response/error writers and status codes are unchanged. |
| Middleware/authority | New/httperror wrapper, deadlines, forbidden input, EscapedPath rejection, BFF key, origin, match, 404, method, 405, IDs, key, bearer, domain resolution, view-only authorization and service dispatch remain in their original order. |
| Unlisted path | New TestRouteTableUnknownPathStill404 requires visible 404 `not_found` and no Location header with valid BFF/origin headers; table walk requires every entry reaches the visible 405 method guard. Existing tests were not modified. |

`routes_legacy_test.go` freezes the base SHA's matcher and method policy, including resource matchers, without reading Routes. TestRouteTableMatchesLegacy compares route kind/id/image/slug and method allowance over valid examples, malformed IDs/slugs, overlong/uppercase/UUID collection keys, aliases, empty/extra/trailing segments and all standard methods plus lowercase/empty methods. The downstream service switch is unchanged. These assertions await CI.

## Scanner mechanism and limits

- `scripts/dev/contractdrift/go_extract.go:86`: parse comments so the marker is visible; `:239` invokes table resolution before deduplication/opaque mount binding (see actual source line for invocation).
- `scripts/dev/contractdrift/go_route_tables.go:15`: documents `//contractdrift:routes /namespace pathMatcher methodChecker` on one package-level named slice/array literal with Method and Path fields.
- `go_route_tables.go:113`: reads finite literal entries and their constant references. Each resolved route retains its entry's file:line, field dependencies and table span.
- `go_route_tables.go:215`: checks the declared consumers are a complete direct range plus closed fallback. Table aliases, index writes, slices and reassignment in the owning package are unsupported and leave the table opaque.
- `go_route_tables.go:293`: connects the table consumers to the HTTP dispatcher; an unused declaration invents no routes.
- `go_route_tables.go:323`: owning mount must call the actual imported table's receiver method, whose entire body returns strings.HasPrefix of its argument and the declared namespace. Importing the package alone is insufficient.
- `cmd/api/buyer.go` changes only its header and actual mount reference. `go_opaque.go`, baseline.json, contracts, migrations, dependencies and apps remain unchanged.
- Supported table dispatch becomes `GO_ROUTE_TABLE` INFO provenance. Other opaque handlers and separate exact mounts retain UNRESOLVED; there is no namespace exemption or baseline addition.
- Static consumption checks cannot prove arbitrary pattern-helper semantics, service-kind exhaustiveness or external mutation of the exported slice. Those remain reviewed invariants plus runtime tests. This generic recognizer deliberately supports a narrow consumer grammar rather than pretending to understand arbitrary Go control flow.

## Static scan and new drift — BLOCKED

`static-report.json` is the unmodified JSON output from the built CLI at source SHA `6f04a2aa61ab981166bcf24d52986e9f69a0e86b`, scanned against base e4dad98cc in an exact-HEAD local clone under this task's output directory. The clone kept the scanner's temporary snapshot writes inside the allowed output path and has been removed. `static-receipt.json` records the command/exit. This was a static source scan, not a Go test or a CI run.

Summary: Go **326**, contracts **148**, BFF **301**; **18 errors**, **291 warnings** overall. All 18 errors are the 16 newly visible buyer GO_ONLY errors below plus two touched feed UNRESOLVED findings. New buyer CONTRACT_ONLY: **0**. New buyer METHOD_MISMATCH: **0**. Nine buyer GO_NOT_IN_BFF informational findings are retained in `buyer-findings.json`.

| New error | Method | Normalized path | Runtime declaration |
| --- | --- | --- | --- |
| GO_ONLY | GET | `/v1/buyer/catalog/v2/collections` | `internal/buyerhttp/routes.go:69` |
| GO_ONLY | GET | `/v1/buyer/catalog/v2/collections/{}` | `internal/buyerhttp/routes.go:71` |
| GO_ONLY | GET | `/v1/buyer/cvs-selections/{}` | `internal/buyerhttp/routes.go:62` |
| GO_ONLY | GET | `/v1/buyer/privacy` | `internal/buyerhttp/routes.go:55` |
| GO_ONLY | GET | `/v1/buyer/storefront/primary-origin` | `internal/buyerhttp/routes.go:43` |
| GO_ONLY | POST | `/v1/buyer/cvs-selections` | `internal/buyerhttp/routes.go:59` |
| GO_ONLY | POST | `/v1/buyer/cvs-selections/{}/verify` | `internal/buyerhttp/routes.go:61` |
| GO_ONLY | POST | `/v1/buyer/cvs-stores` | `internal/buyerhttp/routes.go:60` |
| GO_ONLY | POST | `/v1/buyer/orders/{}/payment/cancel` | `internal/buyerhttp/routes.go:77` |
| GO_ONLY | POST | `/v1/buyer/orders/{}/payment/handoff` | `internal/buyerhttp/routes.go:75` |
| GO_ONLY | POST | `/v1/buyer/orders/{}/payment/prepare` | `internal/buyerhttp/routes.go:74` |
| GO_ONLY | POST | `/v1/buyer/orders/{}/payment/refresh` | `internal/buyerhttp/routes.go:76` |
| GO_ONLY | POST | `/v1/buyer/privacy/erasure` | `internal/buyerhttp/routes.go:58` |
| GO_ONLY | POST | `/v1/buyer/privacy/export` | `internal/buyerhttp/routes.go:57` |
| GO_ONLY | POST | `/v1/buyer/session/retire` | `internal/buyerhttp/routes.go:40` |
| GO_ONLY | PUT | `/v1/buyer/orders/{}/bank-transfer/proof` | `internal/buyerhttp/routes.go:63` |

GO_ONLY means absent from the scanner's contract inventory, not proof that the text is genuinely absent from every document. The integrator must decide whether these need contract additions or contract-parser corrections; this unit did neither.

### Feed remains opaque (two findings)

| Kind | Method | Path | Evidence |
| --- | --- | --- | --- |
| UNRESOLVED | EXACT | `/v1/buyer/feeds/meta.csv` | `cmd/api/buyer.go:183` (mountFeed; gate marks TOUCHED) |
| UNRESOLVED | GET | `/v1/buyer/feeds/meta.csv` | `cmd/api/buyer.go:178`, `contracts/ads-openapi.json:1051`, `contracts/catalog-inventory-v1.md:170` in full report |

Feed was **not already resolved**. It is owned by `mountFeed` plus `attribution.FeedHandler`, outside buyerhttp.Routes; its handler source permits GET and HEAD. Retaining its opaque findings avoids hiding an independent custom dispatcher. Zero namespace UNRESOLVED is therefore still a blocker, and no feed-completion claim is made.

## Red → green CI plan (NOT_RUN)

1. Run existing buyerhttp and cmd/api test suites without removing assertions, including all new TestRouteTable* cases. CI command: `go test ./internal/buyerhttp ./cmd/api` (and project-required race/modes as selected by integrator).
2. Run scanner CLI suite: `go test ./scripts/dev/contractdrift -run 'TestGo(RouteTable|CustomDispatchWithoutTable)' -count=1`, then the full scanner suite.
3. In a **CI-only isolated checkout**, leave all table/runtime/tests present, revert only scanner support by restoring go_extract.go from the base and removing go_route_tables.go. TestGoRouteTableResolvesCLI must go RED: its `code == 0`, `SUMMARY routes go=3 contract=3 bff=1`, no-UNRESOLVED and GO_ROUTE_TABLE/source-line assertions fail.
4. Restore scanner support; the same assertions must go GREEN. TestGoRouteTableUnservedTailRejectedCLI changes the path matcher to `Routes[:len(Routes)-1]`: the table still lists the second route but runtime never visits it. The statically enforceable full-range check must reject it and emit UNRESOLVED. The no-marker and unrelated-mount cases must remain UNRESOLVED.
5. Record CI run/job ids and exact tested SHA. After integrator resolves the 16 new drift findings and feed scope, the actual "Contract drift" plan must report zero buyer UNRESOLVED and zero new errors. Until that happens neither the new gate's red→green proof nor this unit's E3 acceptance exists.

## Verification receipts

| Command | Exit | Evidence level / receipt |
| --- | --- | --- |
| `go build ./...` | 0 | E1, `check-1.log`, `checks.json` |
| `go vet ./internal/buyerhttp ./cmd/api ./scripts/dev/contractdrift` | 0 | E1 (includes compile/static vet of test files, no execution), `check-2.log` |
| `go build -o output/buyer-route-table/contractdrift ./scripts/dev/contractdrift` | 0 | E1, `check-3.log`; temporary binary removed |
| Built static contractdrift CLI, base e4dad98cc, JSON output | 1 | Static blocking findings above; `static-report.json`, `static-receipt.json` |
| `git diff e4dad98cc --check` | 0 | Source whitespace check only |

All source receipts bind `6f04a2aa61ab981166bcf24d52986e9f69a0e86b`. `source-hashes.json` binds each changed Go source/test by SHA-256 so a later output-only delivery commit does not imply rerun evidence for changed code.

## NOT_RUN / risks / cleanup

- NOT_RUN: every Go test, red→green/mutation execution, browsers, servers, PG, integration/runtime, CI plans and independent review; explicitly prohibited locally by the task. No E2/E3/E4 PASS.
- BLOCKED: 16 NEW contract-inventory errors and two feed UNRESOLVED; no retries or contract/baseline bypass attempted. Do not merge this branch as accepted.
- Risk: typed exported route slice is mutable by convention. The source recognizer rejects owning-package writes but is not a whole-program alias or dynamic execution proof. Pattern-helper and service dispatch guarantees need the submitted differential/runtime gates.
- Risk: matching now performs a small linear walk and segment splits; no performance claim or benchmark was made for this 46-entry transport.
- Temporary clone, scanner binary and snapshot directories owned by this task were removed. No servers, browsers, containers, ports or test fixtures were started. Logs and failing findings are retained.
- No push, PR, deployment, production action, contract edit or baseline edit. Remaining decisions and CI execution belong to the integrator.
