READY_FOR_CI — Round 3: both required built-CLI scenarios exit 0, errors=0, no TOUCHED. Build/vet E1; automated static acceptance only. All Go tests, mutation/red→green, runtime, CI and independent acceptance NOT_RUN.

# BUYER-ROUTE-TABLE — Round 3 (2026-10-15)

Source SHA: `913cf96719289a70c12679fc72aadd156fedcc97`.
Base: `e4dad98cc322d35709b475de25063efc92e5d866`.
Branch/worktree: `unit/buyer-route-table`, `/Volumes/data/live_commerce_architecture_v1/.worktrees/buyer-route-table`.
Starting HEAD: `782351ed07e432956f5537ce019fde8d3eaf96f5`.
Executor: Codex, integration_worker scope; runtime model/reasoning identifiers are not independently exposed. No child agents.
Coord: `8569112d-4a60-4fd1-8763-78c6172f39ff`, resumed for this final targeted ruling.

## Round 3 change and checkpoints

| Step | Commit | Result |
| --- | --- | --- |
| Real CLI factory tests first | `1bec0c70f` | Two consumed tables compose; inline path switch and unmarked-package returns stay opaque; execution NOT_RUN |
| Generic returned-handler proof | `913cf9671` | Resolves both buyer assembly factories without changing their source or subtracting dependency spans |
| Delivery/evidence | Subsequent docs-only commit | Exact source SHA and file hashes below remain the acceptance binding |

`go_opaque.go` still discovers the entire local dependency closure and initially
emits the same opaque factory finding. After **all** route tables have been
consumed, `go_route_tables.go:134` invokes
`go_handler_factory.go:66`'s returned-handler proof. The proof follows every
HTTP handler result through nil, supplied fallback/child parameters, local or
imported factories, multiple return slots and local assignments. A table-proven
ServeHTTP receiver type or exact HandlerFunc dispatcher is a known leaf;
constructors such as `buyerhttp.New` are proved from their actual returns.
Middleware must forward proved handlers without adding opaque path/method logic.
Mount conditions reuse the actual table-consumer recognizer
(`tableMountCall`), checking each condition rather than blessing a function
because one table reference occurs somewhere in it.

All assignment alternatives are unioned by AST object, so reassignment, shadowed
locals, captured writes, unknown sources, address escapes and recursive proofs
fail closed. Inline HandlerFuncs with their own path dispatch and handlers from
unmarked packages retain the factory UNRESOLVED finding. No factory-name or
cmd/api exemption was added. No baseline, owning contract, buyer handler or
payment source was edited in Round 3; only scanner/test source changed.

Successful proofs become GO_ROUTE_TABLE notes containing the **entire original**
factory finding's locations plus proof dependencies. Both notes retain
`mountFeed` (184–198), CVS helpers, and `buildBuyerPayment` (86–112 on the branch;
76–102 plus its new profile helper at 104–106 in the #70 overlay). Failed proofs leave the old finding and its spans
unchanged. The reports show both `buildBuyerHandler` and `buildBuyerWithCVS`
resolved from returned handler provenance, with no path filter hiding either.

## CLI tests and the CI red assertion (execution NOT_RUN)

`scripts/dev/contractdrift/go_handler_factory_cli_test.go` uses the existing
real Git + built OS CLI fixture helpers; it does not run handlers or an HTTP
server. Its positive fixture composes separate prefix and exact consumed tables,
a transparent wrapper and a multi-return delegating factory. Negative cases
return an inline HandlerFunc switching on `r.URL.Path`, return an unknown
package's handler, reassign/shadow a handler, add another mount path, or call a
hidden path helper. A separate case preserves the opaque factory's TOUCHED span.

CI command (NOT_RUN locally):

```sh
go test ./scripts/dev/contractdrift -run 'TestGo(HandlerFactory|RouteTable|CustomDispatchWithoutTable)' -count=1
```

With the tests retained, revert only scanner commit `913cf9671` in an isolated
CI checkout. `TestGoHandlerFactoryTablesResolveCLI`'s **exit=0 and absence of
UNRESOLVED** assertion must go RED: `buildTop` and `buildComposite` again produce
opaque findings. Its two factory-provenance note assertions also fail. Restore
the scanner and require GREEN with all negative cases. This mutation has **not**
been executed locally; no red→green or suite PASS is claimed. Existing runtime
buyerhttp/cmd/api/attribution suites and the full scanner suite remain CI work.

## Acceptance A — this branch, exit 0

Built CLI command:

```text
output/buyer-route-table/round3/contractdrift -root /Volumes/data/live_commerce_architecture_v1/.worktrees/buyer-route-table -base e4dad98cc
```

Exact stdout SUMMARY, copied unchanged from `round3/branch-report.txt`:

```text
SUMMARY routes go=328 contract=165 bff=301; errors=0 warnings=288 CONTRACT_ONLY=33 GO_405_FALLBACK=4 GO_MOUNT_OR_FALLBACK=89 GO_NOT_IN_BFF=27 GO_ONLY=200 GO_ROUTE_TABLE=4 METHOD_MISMATCH=4 UNRESOLVED=51
```

Receipt: `round3/branch-receipt.json`; `run_id=buyer-route-table-r3-branch-913cf9671`,
exit **0**, source **913cf96719289a70c12679fc72aadd156fedcc97**. No ERROR or TOUCHED.
The 51 remaining UNRESOLVED findings are legacy warnings, not the two resolved
buyer factories. This is zero **errors**, not a claim of zero warnings globally.

## Acceptance B — #70 single-file overlay, exit 0

Created the task-local scratch clone
`output/buyer-route-table/round3/pay70-scratch`, detached at the same source HEAD.
Pinned #70 source: `origin/unit/pay-rm2-p2a` = local `unit/pay-rm2-p2a` =
`51273344fc12ff4c8062634fdf749d536958bda7`.

Only overlaid file: **`cmd/api/buyer_payment.go`**.
SHA256: `f460f3f4e97f9d9dfc090a3be5094e3c9d01f71545eebbcaf9321360a40a044f`.
`git diff --name-only HEAD` in the scratch clone listed exactly this file.
`go build ./cmd/api` exited **0** on the first attempt, so **no additional #70
files were needed**. No overlay commit was created or copied into this branch.
Setup commands, timestamps and build result: `round3/pay70-setup.json`,
`pay70-clone.log`, `pay70-checkout.log`, `pay70-build-1.log`.

Built CLI command:

```text
output/buyer-route-table/round3/contractdrift -root /Volumes/data/live_commerce_architecture_v1/.worktrees/buyer-route-table/output/buyer-route-table/round3/pay70-scratch -base e4dad98cc
```

Exact stdout SUMMARY, copied unchanged from `round3/pay70-report.txt`:

```text
SUMMARY routes go=328 contract=165 bff=301; errors=0 warnings=288 CONTRACT_ONLY=33 GO_405_FALLBACK=4 GO_MOUNT_OR_FALLBACK=89 GO_NOT_IN_BFF=27 GO_ONLY=200 GO_ROUTE_TABLE=4 METHOD_MISMATCH=4 UNRESOLVED=51
```

Receipt: `round3/pay70-receipt.json`; `run_id=buyer-route-table-r3-pay70-913cf9671`,
exit **0**, **no TOUCHED**. This proves the requested buyer_payment.go rebase
scenario passes the static gate; it does not claim that every file/test in #70
has been rebased or that payment runtime acceptance has passed.

## Verification, evidence and remaining work

| Command | Exit | Evidence |
| --- | --- | --- |
| `go build ./...` | 0 | E1; `round3/check-1.log`, `checks.json` |
| `go vet ./internal/buyerhttp ./internal/attribution ./cmd/api ./scripts/dev/contractdrift` | 0 | E1; `round3/check-2.log`, `checks.json`; no test execution |
| `go build -o output/buyer-route-table/round3/contractdrift ./scripts/dev/contractdrift` | 0 | E1; `round3/check-3.log`, `checks.json` |
| Built CLI branch vs `e4dad98cc` | 0 | Automated static source acceptance; branch report/receipt |
| Scratch overlay `go build ./cmd/api` | 0 | E1; `round3/pay70-build-1.log`, `pay70-setup.json` |
| Same built CLI scratch overlay vs `e4dad98cc` | 0 | Automated static source acceptance; pay70 report/receipt |

Both CLI runs used binary SHA256
`145aec6d081e219b1c2c8a7683557b76ca44d8545cf246532fe1ec93c07f5daf`.
Every build/vet command had a 180-second timeout; clone/checkout 60 seconds; CLI
120 seconds. All processes exited. `round3/source-hashes.json` binds source,
tests, route/index and baseline inputs, including the overlay hash;
`round3/evidence-hashes.json` binds the retained receipts and logs.

NOT_RUN: all Go tests, mutation/red→green, runtime/browser/HTTP/PG/provider
checks, CI and independent review. READY_FOR_CI is a static handoff, not full
unit/release acceptance. No push or PR. The proof supports a finite grammar;
unsupported aliases/control flow remain conservatively opaque. Route-table
semantics and runtime equivalence still require their unchanged CI gates.

Task-created scratch clone and built binaries were removed after their hashes,
overlay manifest and raw reports were retained (`round3/cleanup.json`). No
servers, containers or ports were started. No other task directory was cleaned.
Cleanup reclaimed **458,854,080 bytes**; all command processes had exited.

---

> **Round 2 — historical BLOCKED delivery.** Retained verbatim below. Its SHA,
> findings and pending ruling describe Round 2 only; the current status and
> acceptance binding are the Round 3 READY_FOR_CI section above.

BLOCKED — Round 2 static CLI exits 1 with two touched buyer assembly UNRESOLVED findings. E1 only; CI/runtime and independent acceptance NOT_RUN.

# BUYER-ROUTE-TABLE — Round 2 (2026-10-15)

Source SHA: `df8eefa074bcd8577e02363792ee973dde13a317`.
Base: `e4dad98cc322d35709b475de25063efc92e5d866`.
Branch/worktree: `unit/buyer-route-table`, `/Volumes/data/live_commerce_architecture_v1/.worktrees/buyer-route-table`.
Role/executor: integration_worker scope, Codex, reasoning high as dispatched; runtime model id is not independently exposed. No child agents.
Coord: `8569112d-4a60-4fd1-8763-78c6172f39ff`.

## Round 2 ruling and result

The canonical index now records all 46 `buyerhttp.Routes` entries and feed GET/HEAD,
with full method/path syntax, BFF/Bearer/Idempotency-Key policy and owning contract
sections. `contracts/buyer-routes-v1.md` §2.1–§2.3 describes the three requested
transports as **documented-from-code 2026-10-15**, with handler/service line pointers,
request/success/error/size details. In particular, privacy GET currently ignores a
supplied body and enforces no body-size limit; the index records that behaviour.
No owning contract text changed.

`contracts.go:94`'s `mdRoute` requires METHOD + whitespace + an immediate path token,
whose character class excludes backticks. The full prepare path at
`buyer-payment-http-v1.md:100` is written as POST followed by an opening backtick,
so it does not match at all. Relative path forms also need a recognized `Route base:`.
The index places METHOD and full `/v1/buyer/...` path within the same code span;
it leaves the parser and the owning contracts unchanged.

The feed's two entries are declared at `internal/attribution/feed.go:94` and `:95`.
`cmd/api/buyer.go:186` uses `attribution.FeedRoutes.MatchesPath`; the receiver wraps
the complete exact-path table walk (`feed.go:100`, `:102`). The handler's first guard
uses the complete method table walk (`feed.go:111`, `:125`). The path equality and
GET/HEAD method sets are identical to the prior conditions. All methods at that
exact path still reach the BFF check first: missing/duplicate/wrong key remains
401 before handler/SQL, with the existing constant-time compare unchanged
(`cmd/api/buyer.go:190`). Unsupported methods with a valid BFF key still get 405
from the same handler; origin validation, timeouts, SQL, response headers, GET CSV
and HEAD without CSV remain in their original order. This is source reasoning,
not an executed runtime-equivalence claim.

## Round 2 checkpoints

| Step | Commit | Result |
| --- | --- | --- |
| Canonical index and code-derived descriptions | `f40fdf84c` | 48 method/path rows; owning contracts untouched |
| Exact feed table + consumers | `b8a9bdf93` | mount path and handler method guard consume one table |
| Exact recognizer | `1183ed690` | generic final `exact` marker form |
| Require actual owning mount/direct consumers | `c95c28c94` | unused table cannot claim exact coverage |
| CLI positive/counterexamples | `2268af117` | tests committed, execution NOT_RUN |
| Final feed handler pointer | `df8eefa07` | source SHA used by all Round 2 receipts |

## Scanner extension and CI proof (NOT_RUN)

`go_route_tables.go:27` documents the additional marker form:
`//contractdrift:routes /exact/path pathConsumer methodConsumer exact`.
All rows must have that same exact path; the two consumers must be full-table
boolean equality walks with closed false fallbacks. The method dispatcher must
be a direct returned HTTP HandlerFunc beginning with a closed negative guard on
that consumer's actual request Method, and cannot dispatch on Path. The actual
imported table receiver must wrap the path consumer for an owning mount. See
`readRouteTable`, `tableMount`, `goTableExactConsumer`, `goTableExactDispatcher`.
The round-1 prefix grammar remains supported. `go_opaque.go` and baseline are unchanged.

CI must run the existing buyerhttp, cmd/api and attribution suites, plus the full
contractdrift suite, without removing assertions. Focused command:
`go test ./scripts/dev/contractdrift -run 'TestGo(RouteTable|CustomDispatchWithoutTable)' -count=1`.
The new `go_route_table_exact_cli_test.go` cases exercise real Git + built CLI:
exact GET/HEAD resolution and declaration provenance; missing marker; truncated
consumer; a row with a different path; bypassed method guard; extra handler path;
unrelated mount; missing table-consuming mount. Tests do not run HTTP servers.

In a CI-only isolated checkout, restore `go_route_tables.go` from round-1 HEAD
`28aaaea99` while retaining feed/table/tests. `TestGoRouteTableExactSplitResolvesCLI`
must go RED: exit 0, three-route SUMMARY, no-UNRESOLVED and declaration provenance
assertions fail because the fifth marker field is unsupported. Restore the
extension and require GREEN plus the negative cases. This red→green execution,
all existing runtime tests and independent review are **NOT_RUN** locally by
explicit instruction. The current static report also shows that factory coverage
remains unresolved; do not infer full gate acceptance from the fixture plan.

## Static acceptance — BLOCKED, no workaround

The built CLI ran once against an exact source-HEAD local clone, using `-base
e4dad98cc -json`; exit **1**. `round2/static-report.json` is unmodified stdout;
`round2/static-receipt.json` records SHA, exact command, timeout and timestamps.
The SUMMARY below uses `main.go:221`'s text format rendered from that JSON; it is
not a second scan or a claimed literal stdout line from the JSON command:

```text
SUMMARY routes go=328 contract=165 bff=301; errors=2 warnings=288
```

All 48 indexed buyer method/path pairs resolve. The original 16 new buyer GO_ONLY
findings are gone. Path-scoped `/v1/buyer` UNRESOLVED: **0**; new buyer
CONTRACT_ONLY: **0**; new buyer METHOD_MISMATCH: **0**. However, these two buyer
assembly UNRESOLVED errors have **no path field**, so the zero path-scoped count
must not conceal them or be presented as zero buyer unknown coverage:

| Remaining error | Source | Retained dependency span |
| --- | --- | --- |
| ERROR (TOUCHED) UNRESOLVED — opaque Go handler dispatch buildBuyerHandler | `cmd/api/buyer.go:88` (88–91) | includes buildBuyerWithCVS and mountFeed (184–198) |
| ERROR (TOUCHED) UNRESOLVED — opaque Go handler dispatch buildBuyerWithCVS | `cmd/api/buyer.go:96` (96–175) | includes mountFeed (184–198), CVS and payment assembly |

The report contains GO_ROUTE_TABLE provenance for both Routes and FeedRoutes;
the exact mount and feed handler resolve, but the upper factory functions are
not covered by the current table recognizer and remain opaque. Per the ruling,
source work stopped on this first scan. No fix retry, namespace exemption,
probe exclusion, baseline edit, -write-baseline or source reshaping followed.
The two errors are retained verbatim in `round2/errors.json`. Integrator ruling
is required before further source work; this is **not READY_FOR_CI acceptance**.

## Verification and evidence boundary

| Command | Exit | Evidence |
| --- | --- | --- |
| `go build ./...` | 0 | E1; round2/check-1.log, checks.json |
| `go vet ./internal/buyerhttp ./internal/attribution ./cmd/api ./scripts/dev/contractdrift` | 0 | E1; round2/check-2.log, checks.json; no test execution |
| `go build -o output/buyer-route-table/round2/contractdrift ./scripts/dev/contractdrift` | 0 | E1; round2/check-3.log, checks.json |
| Built CLI `-root <exact-HEAD clone> -base e4dad98cc -json` | 1 | Static evidence; round2/static-report.json, static-receipt.json, errors.json |

`round2/source-hashes.json` binds all changed source/test/index files against the
base. Later commits in this round only retain delivery/evidence. No E2/E3/E4 PASS
or unit completion claim is made. A changed source requires fresh evidence.

## NOT_RUN, risks and cleanup

- NOT_RUN: every Go test, mutation/red→green execution, browser, HTTP server,
  database/provider runtime, CI and independent review. No push or PR.
- BLOCKED: the two TOUCHED unscoped buyer factory findings; do not merge as accepted.
- Exported route tables remain mutable by convention; owning-package writes and
  partial consumption are rejected statically, but this is not a whole-program
  mutation/alias proof. The recognizer supports a finite grammar; factory coverage
  is the demonstrated remaining limit. Runtime equivalence still requires CI.
- The task-created exact-HEAD clone, its scanner snapshots and built binary were
  removed (432,881,037 bytes); logs/failing evidence retained in round2. Build/vet/
  CLI processes exited; no servers, containers, ports or runtime fixtures started.
  See round2/cleanup.json. Other task directories were not changed.

---

> **Round 1 — historical BLOCKED delivery.** The following is retained verbatim
> for provenance; its source SHA, counts, limitations and pending ruling describe
> round 1 only. Current status is the Round 2 BLOCKED section above.

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
