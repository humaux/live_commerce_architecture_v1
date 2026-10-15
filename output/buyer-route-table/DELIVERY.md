READY_FOR_CI / INDEPENDENT_REVIEW — Round 9. E3 **STATIC built-CLI acceptance** at `fb918d9c2659cc81e96bd578267d2a66a45d0237`: branch and pinned #70 overlay exit0/errors=0/no TOUCHED; build/vet/gofmt/prepush exit0. A was a fixture mutation of the dispatcher, not a row-target closure leak. B write/build authority checks reject all eight scratch counterexamples. Local Go tests, CI and independent r9 review NOT_RUN. No push.

# BUYER-ROUTE-TABLE — Round 9 (2026-10-15)

Start `47125030ef6c12efb1047e54d86dd7b1b654744a`; worktree `.worktrees/buyer-route-table`; branch `unit/buyer-route-table`; existing PR #71. Comparison base `e4dad98cc322d35709b475de25063efc92e5d866`. Final accepted source `fb918d9c2659cc81e96bd578267d2a66a45d0237`; later delivery changes contain only docs/evidence. `round9/source-hashes.json` binds 58 scanner/test/fixture/pin/gate-document files; `round9/evidence-hashes.json` binds retained evidence and this delivery. Verify these at the delivery HEAD before relying on the receipts.

Executor: Codex (system GPT-6 identity; actual backend ID/effort not independently exposed), integrator scope; no subagents. Coord `100423e4-74de-41f2-8f52-d51bfc2e5b0f` / `codex-buyer-route-table-r9`. Ownership: `scripts/dev/contractdrift/**`, engineering/gate docs, task output. Read the complete Round 8 delivery and main-repository `output/buyer-route-table/review-r8.md` (absent in this worktree); original review path/hash/verdict in `round9/review-input.json`.

## Separate commits

| Commit | Change |
| --- | --- |
| `0e2155858` | B counterexamples FIRST: N1/N2/N3, exported frozen state, address escape, IncDec, duplicate names, build suffixes/tags, every init. Go tests NOT_RUN. |
| `a98499f73` | A: correct only the row-target mutation anchors; preserve all existing expectations. |
| `a794e95e3` | Additional counterexamples before B implementation: index/field/range/append/pointer-argument writes and a local-shadow control. |
| `3051d0f3b` | B package-wide authority and build-selection checks, init roots, per-row errors and boundary documentation. |
| `fb918d9c2` | Diagnose duplicate declarations using original spec positions rather than synthetic GenDecl line 0. |
| Round 9 delivery commit | This delivery, source manifests, command receipts and cleanup. |

## A — exact root cause and each failing assertion

The scratch diagnostic CLI was built from the unchanged R8 scanner, with only `main` changed to print `scanGo(root).Dispatchers` (`round9/diagnostic-source.json`, successful `diag-build-explicit-receipt.json`). It reproduces the exact CI pair:

- Before: `9b511ed19277ba80190ab04350c84f192e605655209c08a992bf0b40b5da3277`.
- After the original first-occurrence replacement: `e11db674b7929660233974356bb846e256fad0a0c2b2081d30458cad185584e9`.
- Both closures have exactly `Table.HasPrefix, allowed, handler.ServeHTTP, match, pattern`, plus types `Table, entry, handler, route`; no row target, stored target var or leaf method enters the closure. Full lists: `A-before-closure.json`, `A-after-closure.json`.

`strings.Replace(source, "w.WriteHeader(204)", "w.WriteHeader(205)", 1)` changes the first 204, in `handler.ServeHTTP`, before the appended row target. `A-wrong-mutation.diff` shows the actual dispatcher edit. Freeze correctly refuses that body change; no declaration walk or fallback rule wrongly pulled in a target. Changing that rule would weaken the intended dispatcher guard.

The direct case now replaces the full unique `func rowLeaf(...)` declaration. Stored alias/literal/method-value cases replace only their appended `tail`, and then change the 204 within that tail. `A-correct-mutation.diff` shows the direct correction. Expectations and thresholds stay unchanged.

| Failing CI condition | Why the unchanged expectation is now met |
| --- | --- |
| RowHandlerBodyExcluded: target absent before mutation | `rowLeaf` is referenced only by row data and is still absent from the function list. |
| RowHandlerBodyExcluded: post-edit CLI resolves and hash equals before | Only the appended leaf body changes; `ServeHTTP` remains 204 and the complete canonical stream is identical. |
| StoredRowTargetsExcluded / alias | Only `rowLeaf` in the appended alias tail changes; neither alias nor target is reached outside row data. |
| StoredRowTargetsExcluded / literal | Only the anonymous function source in the appended `rowTarget` tail changes; that var is outside the closure. |
| StoredRowTargetsExcluded / method_value | Only `leaf.serve` in the appended tail changes; the row-only method target remains outside the closure. |

Four corresponding corrected scratch fixtures run through the final built CLI: all exit0/errors=0/no TOUCHED with identical pre/post hashes. Exact lists/hashes and CLI summaries are in `A-corrected-closures.json` and `A-correct-*-receipt.json`. These are static CLI observations; the Go test assertions themselves remain NOT_RUN locally.

The extra TOUCHED `ServeHTTP`/`mountBuyer` and CONTRACT_ONLY findings are consequences of withholding table proof after the dispatcher change. The fixture adds those dispatchers and `contracts/route-table-openapi.json` after the `trunk` tag. With no table inventory, the opaque regions remain touched and the two correct contract rows appear unserved. `A-original-report.txt` retains all six errors. The fixture contract file is correct and unchanged; no contract/baseline edit hides these lines.

R8 did not catch this because its local evidence exercised real const/regexp mutations and inspected the exclusion test source without executing those fixtures (owner forbids local Go tests). The mutation anchor was assumed to select the appended target; it was not checked against a source diff. The literal hashes and “target absent” assertion cannot detect that mistake, since the changed body was already legitimately frozen. R9 retains the exact mutation diffs and corrected scratch CLI observations. A attempt 1 resolved without a closure change or re-pin.

## B — authority rule and red map

The old implementation froze each var declaration but never checked who writes it, and last-wins maps silently selected declarations across build variants. Before the fix, the built R8 CLI accepted N1/N2/N3 and address escape with exit0/errors=0/GO_ROUTE_TABLE=4. `red-N3` uses a nil reassignment; `red-exact-N3` additionally reproduces the reviewer's exact `regexp.MustCompile(".*")` assignment in `New`.

The final gate scans every non-test file in the table package. A frozen var with any outside assignment (including fields/indexes/range), IncDec, address-taking (also through pointer call arguments), or append/copy/clear/delete mutation/escape is unproven. The var's own declaration is excluded. Parser-bound locals do not count as writes to package state. Frozen exported vars are refused, preventing writes from any importing package; the sole exception is the marked table's documented fieldless facade, validated by the existing facade grammar.

Every frozen declaration name is counted independently of lookup maps. Duplicates fail with a precise source location; methods are receiver-qualified. All `init` funcs are independent roots, including multiple legal init declarations. Any non-test package file with a Go/legacy build line or recognized OS/architecture suffix fails closed without evaluating tags. Unsafe inputs produce one fatal dispatcher diagnostic per advertised row and never resolve that table.

| Counterexample | Pre-fix built CLI | Final built CLI / evidence |
| --- | --- | --- |
| N1 new `init` reassigns `v2Slug` | exit0/errors0 | assignment at `zz_init.go:3`; 46 freeze errors |
| N2 linux/!linux split declaration | exit0/errors0 | build constraint filename `a_slug_linux.go`; 46 freeze errors |
| N3 assignment in `New`, exact reviewer expression | exit0/errors0 | assignment at `handler.go:57`; 46 freeze errors |
| `p := &v2Slug` in unrelated function | exit0/errors0 | address taken at `zz_pointer.go:2`; 46 freeze errors |
| Additional identical declaration | exit0/errors0 | duplicate frozen name at `zz_duplicate.go:3`; 46 freeze errors |
| Unused custom-tag file | exit0/errors0 | build constraint at `zz_tag.go:1`; 46 freeze errors |
| New init without a frozen-var write | exit0/errors0 | init changes hash; 46 freeze errors |
| Exported `V2Slug`, explicitly pinned using pre-fix candidate | exit0/errors0 | exported frozen var at `catalogv2.go:46`; 46 freeze errors |

`B-mutation-results.json`, `red-*-receipt.json`, `final-*-receipt.json` and reports retain the observed red→green static gate evidence. Eight final CLI runs exit1 as required (each errors=133 including 46 per-row freeze errors and collateral route findings). Table withdrawal is intentional in these negatives, so collateral TOUCHED findings are not acceptance failures.

CI-only counterexamples additionally cover duplicate function declarations, bare OS/arch suffixes, IncDec, composite/index/field/range/append writes, pointer arguments, local-shadow acceptance and hashing both init funcs. Those Go tests are **NOT_RUN**, not inferred PASS. A second targeted B correction fixed synthetic declaration diagnostics reporting line 0; final duplicate evidence reports line 3. B attempt 2 resolved; no further repair attempt was made.

Real package checks found no prohibited writes to `v2Slug`, `idempotencyKey`, `linkTokenPattern`, `feedColumns` or `isoPattern`; real tables still resolve. Runtime sources were not edited. No real re-pin: the hash root specification now includes init, but neither real table package has init funcs, and authority checks do not alter the canonical byte stream. Both real hashes remain buyer `86608e03…ac84` (81 funcs/150 declarations) and attribution `65b79d88…bac1` (10/15); the pin file is byte-identical to starting HEAD.

## Final acceptance, SHA binding and SUMMARY lines

All below use source `fb918d9c2659cc81e96bd578267d2a66a45d0237`. `scanner-build.json` binds the final executable SHA256 `956e87db43a9f0a2413fe1d51421dc8d0cf59e7316dbc4627fd64e7200cfd684`; CLI receipts also carry that hash. Scratch-repository `source_sha` records the scanned clone HEAD; the executable is separately bound to the scanner build, including red controls built from `47125030e`.

| Command / scenario | Exit | Evidence in `round9/` |
| --- | --- | --- |
| `go build ./...` | 0 | `build-receipt.json` |
| `go vet ./...` (compiles, does not execute tests) | 0 | `vet-receipt.json` |
| `gofmt -l scripts/dev/contractdrift` | 0, empty output | `gofmt-receipt.json` |
| Existing integrator `prepush.sh <worktree> e4dad98cc` | 0 | `prepush-receipt.json` |
| Built CLI branch `-base e4dad98cc` | 0/errors0/no TOUCHED | `branch-receipt.json` |
| Built CLI pinned #70 overlay, same base | 0/errors0/no TOUCHED | `pay70-receipt.json` |
| Overlay `go build ./cmd/api` | 0 | `pay70-build-receipt.json` |

Branch:
```text
SUMMARY routes go=328 contract=165 bff=301; errors=0 warnings=288 CONTRACT_ONLY=33 GO_405_FALLBACK=4 GO_MOUNT_OR_FALLBACK=89 GO_NOT_IN_BFF=27 GO_ONLY=200 GO_ROUTE_TABLE=4 METHOD_MISMATCH=4 UNRESOLVED=51
```

Pinned #70 overlay:
```text
SUMMARY routes go=328 contract=165 bff=301; errors=0 warnings=288 CONTRACT_ONLY=33 GO_405_FALLBACK=4 GO_MOUNT_OR_FALLBACK=89 GO_NOT_IN_BFF=27 GO_ONLY=200 GO_ROUTE_TABLE=4 METHOD_MISMATCH=4 UNRESOLVED=51
```

Overlay uses only `cmd/api/buyer_payment.go` from `51273344fc12ff4c8062634fdf749d536958bda7`; its file hash and exact one-file diff are in `overlay-manifest.json`. No remote fetch or push. `preservation-audit.json` proves no runtime change since `47125030e`, and byte preservation of the pin, baseline, route inventory tests, R6 negatives and refusal tests/code.

## NOT_RUN, risks and cleanup

- **NOT_RUN:** every local Go test (including the CI failures and all new tests), full CI, independent r9 review, HTTP routing/transport/differential runtime, REAL_PG, browser clicks, SANDBOX/LIVE and release gates. This is E3 STATIC only, READY_FOR_CI / INDEPENDENT_REVIEW, not unit/runtime/release completion or merge approval.
- Syntax checks do not model unsafe/reflection, arbitrary alias effects or mutation inside imported implementations. Cross-package helper bodies remain unfrozen. Same-name method over-inclusion and whole const blocks remain conservative. Filename vocabulary must follow Go's supported OS/architecture names as the toolchain evolves.
- The initial diagnostic build by package path failed because `output/` is outside the module's package tree; the explicit source-file build succeeded. Failed receipt is retained. A code-index handoff initially exceeded the command output cap; the five files were then submitted separately without truncation. Index completed (94 entities); authority and fingerprint entities link to Humaux fix `c7e68495-c8b9-45ea-971d-34bc3a833175`.
- `cleanup.json`: removed only this round's scratch clone, fixture trees, diagnostic sources, binaries and Python cache; **493327147 bytes** removed. Failure/success evidence is retained. No task server/port/container was started; all command processes terminated. No push, PR mutation, merge or deployment.
- Decisions/fixes/acceptance stored in Humaux and own canvas updated. Rollback point is `47125030e`; revert this round's separated commits in reverse order without resetting unrelated work.

---

# Historical delivery below — superseded by Round 9 for this unit

READY_FOR_INDEPENDENT_REVIEW — Round 8. E3 **STATIC built-CLI acceptance** at `7b19d9237030f9bbf7e9c5066aa78e6d3ffa3e0e`: branch and pinned #70 overlay exit0/errors=0/no TOUCHED. Both reviewer mutations now produce 46 per-row freeze ERRORs each. E1 build/vet/gofmt/prepush passed. All local Go tests, CI and independent r8 review NOT_RUN. No push.

# BUYER-ROUTE-TABLE — Round 8 (2026-10-15)

Start `2d8f4604efebb6ef5a593df166e845197bf69b5f`; branch `unit/buyer-route-table`; worktree `.worktrees/buyer-route-table`; existing PR #71. Comparison base `e4dad98cc322d35709b475de25063efc92e5d866`. Acceptance source `7b19d9237030f9bbf7e9c5066aa78e6d3ffa3e0e`. Subsequent delivery changes are docs/evidence only; `round8/source-hashes.json` binds the accepted scanner/tests/pins/runtime bytes and current documentation. Verify those hashes at the final head before using these receipts.

Executor: Codex (GPT-6 system identity; exact backend model ID/effort not independently exposed), integrator scope; no subagents. Coord task `8569112d-4a60-4fd1-8763-78c6172f39ff`, owner `codex-buyer-route-table-r8`. Ownership: `scripts/dev/contractdrift/**`, explicit pin JSON, engineering/gate docs and task output. Runtime/owning route contracts/baseline/dependency locks unchanged.

Read r7 review in full from the main repository's `output/buyer-route-table/review-r7.md` (absent in this worktree), plus the full Round 7 delivery. `round8/review-input.json` records its original path/hash and CHANGES_REQUESTED verdict.

## Changes and commit separation

| Commit | Change |
| --- | --- |
| `06e7f82b6` | Counterexamples first: declaration mutations, direct row-target fallback and generic receiver collisions. Pure row-only tests remove their old direct dispatcher calls to match the clarified boundary. Go tests NOT_RUN. |
| `813b9517b` | Fix traversal/hash, stable qualified keys, declaration inventory and both explicit re-pins; update literal fixture hash anchors. |
| `7b19d9237` | Document declaration freeze and imported-helper boundary. |
| Round 8 delivery commit | Receipts, red map, full declaration lists, cleanup and final gate-doc wording. |

P1: signatures/bodies recursively reach same-package const blocks, var/type specs and functions, including functions referenced by a var initializer. Entire const blocks preserve iota positions and inherited expressions. All declarations use comment-free gofmt source; map key expressions are followed as well. The only data variable excluded is the row table itself.

P2-1: removed the global row-target deny set. Row-only targets are unreachable from the graph and stay excluded; a direct reference from any closure body includes the target and its dependencies, including fallback calls/aliases.

P2-2: pointer, IndexExpr and IndexListExpr receivers unwrap to `Recv.Name`; plain funcs remain `Name`. `sort.SliceStable` compares kind, full name, then full source key (file order/position and existing file:line:column function key). Full kind-qualified declaration lists and function lists must both match pins, alongside SHA256.

Pin comment, gate doc and engineering doc explicitly state: cross-package helpers (e.g. `command.ValidID`) are **NOT frozen**; normal review/other gates cover them. No pin regeneration occurs during normal CLI scans.

## New fingerprints and exact declaration lists

Names below are the complete canonical stream order. `func` entries also form the exact `functions` list. A const block key lists sorted member names; the hashed source retains block order. Whole-block freezing intentionally includes unreferenced neighbours. `declarations` is part of the compared pin contract.

### `attribution.FeedRoutes` — 10 functions, 15 declarations

SHA256: `65b79d881371cc738d37cc31aa915eb175f69c43aa24d4d4b0bab231319ebac1`

Reviewed: `2026-10-15 integrator + independent review r7 (closure read; pending r8 re-check)`

```text
const originHeader
func FeedHandler
func FeedRouteTable.MatchesPath
func addImageLinks
func feedError
func feedMethodAllowed
func feedPathMatches
func feedPrice
func imageLinks
func loadFeed
func writeFeed
type FeedRouteTable
type feedRow
var feedColumns
var isoPattern
```

### `buyerhttp.Routes` — 81 functions, 150 declarations

SHA256: `86608e032f9cd9c41c7caad8c92695d16bc5eca480141aaf6c278d964881ac84`

Reviewed: `2026-10-15 integrator + independent review r7 (closure read; pending r8 re-check)`

```text
const bootstrapRoute,cartRoute,catalogRoute,catalogV2CollectionRoute,catalogV2CollectionsRoute,catalogV2ProductRoute,catalogV2ProductsRoute,checkoutRoute,claimLinkRoute,claimRedeemRoute,collectionMediaRoute,consentsRoute,designPreviewRoute,designPublishedRoute,destinationItemRoute,destinationRoute,mediaRoute,optionsRoute,orderRoute,ordersRoute,paymentCancelRoute,paymentHandoffRoute,paymentPrepareRoute,paymentRefreshRoute,paymentRoute,primaryOriginRoute,privacyErasureRoute,privacyExportRoute,privacyRoute,quoteRoute,quotesRoute,retireRoute,sessionRoute,storeMediaRoute,unknownRoute
const claimDerivedKeyPrefix,claimLinkPath,claimRedeemPath,claimTokenHeader
const designPreviewPath,designPublishedPath,previewTokenHeader
const lookupPath,lookupWindow,routeLookup
const maxJSON
const orderLinkPath,routeOrderLink
const routeCVSSelectionGet,routeCVSSelectionOpen,routeCVSSelectionVerify,routeCVSStoreEnter
const routeTransferGet,routeTransferProof
const v2DefaultLimit,v2MaxLimit,v2MaxMinorMoney,v2MaxOffset,v2MaxQuery,v2Prefix
func RouteTable.HasPrefix
func admittedPaymentMethod
func allowed
func attachV2ImageSizes
func bearer
func bodyTimeout
func buyerTransaction
func canonicalSecret
func catalogRequest
func claimToken
func classify
func contactDigest
func cvsHTTPError
func decodeExact
func decodeJSON
func decodeV2Cursor
func designPGError
func encodeV2Cursor
func forbiddenInput
func handler.ServeHTTP
func handler.authorizeView
func handler.catalogV2Get
func handler.claimRequest
func handler.collectionMediaGet
func handler.consentPut
func handler.cvsRequest
func handler.cvsServe
func handler.designGet
func handler.dispatch
func handler.mediaGet
func handler.orderLink
func handler.orderLookup
func handler.paymentRequest
func handler.primaryOriginGet
func handler.privacyErase
func handler.privacyExport
func handler.privacyGet
func handler.storeMedia
func handler.transferRequest
func handler.viewOnly
func isCatalogV2
func isDesignRoute
func isKeylessPaymentRoute
func isPaymentRoute
func isPublicRoute
func keyFor
func keylessPaymentPath
func lookupClientIP
func mapPublicCatalogError
func matchPattern
func matchRoute
func mobileHint
func noBody
func normalizeRef
func oneHeader
func optInt
func optionsRequest
func ordersRequest
func parseMinor
func parseV2Query
func privacyError
func projectCart
func projectCatalog
func projectCheckout
func projectClaimPreview
func projectClaimRedeem
func projectDestination
func projectOptions
func projectOrder
func projectOrders
func projectPromotion
func projectQuote
func projectQuoteLines
func projectV2
func scoped
func storefrontOrigin
func strictDecode
func v2Query.digest
func viewAllowed
func writeImage
func writeOK
type RouteTable
type cartItemResponse
type cartResponse
type catalogImageResponse
type catalogItemResponse
type catalogResponse
type catalogResult
type checkoutResponse
type claimPreviewLineResponse
type claimPreviewResponse
type claimRedeemResponse
type claimSkippedResponse
type codedResponse
type createdResponse
type destinationResponse
type erasureResponse
type handler
type homeAddressResponse
type lineAmountResponse
type lookupInput
type optionResponse
type optionsResponse
type orderDestinationResponse
type orderLinkInput
type orderPickupResponse
type orderQuoteResponse
type orderResponse
type orderServiceResponse
type orderSnapshotResponse
type orderSummaryResponse
type ordersResponse
type paymentPrepareInput
type paymentPrepareResponse
type pickupResponse
type promotionResponse
type quoteAmountResponse
type quoteLineResponse
type quoteResponse
type responseError
type route
type routeEntry
type routeKind
type v2Axis
type v2Card
type v2CollectionCard
type v2CollectionOne
type v2CollectionRef
type v2Collections
type v2Cursor
type v2Detail
type v2Image
type v2ImageSize
type v2List
type v2OptionImage
type v2Query
type v2Store
type v2Variant
var idempotencyKey
var linkTokenPattern
var v2Slug
```

The buyer/feed function sets remain the reviewed 81/10 names. `buildBuyerPayment` remains outside this closure: it is in `cmd/api`, not `internal/buyerhttp`. Pinned #70 overlay changes exactly `cmd/api/buyer_payment.go` from `51273344fc12ff4c8062634fdf749d536958bda7`; its file SHA256 and sole-path diff are in `round8/overlay-manifest.json`.

Fixture literal anchors (source inspection only; not executed Go tests): prefix `f59aafcde055f93dd1f5c01efc3d013c74307bea2ef5b8c960591c5adeed6f5c`, exact `43847f5c3ea2918e1011d8fa0d60b1a54f1ba818580f58e2f80b5ce969f46611`. Corresponding declaration lists are in `round8/prefix-fixture-pin.json` and `exact-fixture-pin.json`.

## Acceptance SUMMARY lines

Branch:

```text
SUMMARY routes go=328 contract=165 bff=301; errors=0 warnings=288 CONTRACT_ONLY=33 GO_405_FALLBACK=4 GO_MOUNT_OR_FALLBACK=89 GO_NOT_IN_BFF=27 GO_ONLY=200 GO_ROUTE_TABLE=4 METHOD_MISMATCH=4 UNRESOLVED=51
```

Pinned #70 overlay:

```text
SUMMARY routes go=328 contract=165 bff=301; errors=0 warnings=288 CONTRACT_ONLY=33 GO_405_FALLBACK=4 GO_MOUNT_OR_FALLBACK=89 GO_NOT_IN_BFF=27 GO_ONLY=200 GO_ROUTE_TABLE=4 METHOD_MISMATCH=4 UNRESOLVED=51
```

| Required check | Exit/result | Evidence under `round8/` |
| --- | --- | --- |
| `go build ./...` | 0 / E1 | `build-receipt.json` |
| `go vet ./...` (static compile/vet of test files; no tests run) | 0 / E1 | `vet-receipt.json` |
| `gofmt -l scripts/dev/contractdrift` | 0, empty output / E1 | `gofmt-receipt.json` |
| integrator `prepush.sh <worktree> e4dad98cc` | 0, prepush: OK / E1 | `prepush-receipt.json`; delivery wording rechecked in `prepush-delivery-receipt.json` |
| build actual CLI | 0 / E1 | `build-cli-receipt.json` |
| built CLI branch vs e4dad98cc | 0, errors=0, no TOUCHED / E3 STATIC | `branch-receipt.json`, `branch-report.txt` |
| #70 scratch overlay API build | 0 / E1 | `pay70-build-receipt.json` |
| same built CLI vs #70 scratch overlay | 0, errors=0, no TOUCHED / E3 STATIC | `pay70-receipt.json`, `pay70-report.txt` |
| reviewer const221→210 scratch mutation | 1, 46 freeze ERRORs / E3 STATIC negative | `green-const-receipt.json`, `green-const-report.txt` |
| reviewer v2Slug→`.*` scratch mutation | 1, 46 freeze ERRORs / E3 STATIC negative | `green-regexp-receipt.json`, `green-regexp-report.txt` |

Actual CLI binary SHA256: `bed4990cbfd973bee083be69e36cb80890cfb80d457b1c79faf7f1658bf5fb43`. Both acceptance receipts and both negative receipts identify this binary. Commands register run IDs, start/end/exit, a 180-second timeout and terminal completion wake. All runs ended; no background work remains. `round8/run.py` retains the small stdlib receipt wrapper; it never runs Go tests.

## Red map — observed and prospective evidence separated

Before changing the scanner, the CLI built at counterexample commit `06e7f82b6` reproduced both r7 bypasses on an owned `git clone --shared` scratch clone. Each mutation returned exit0/errors=0/GO_ROUTE_TABLE=4/no TOUCHED. After the fix, the identical mutated file bytes return exit1/errors=133, including **46 fatal per-row dispatcher freeze UNRESOLVED** findings each; buyer table coverage is withheld. Other errors follow from withholding that proof. `round8/mutation-comparison.json` records both file hashes, old/new binary hashes and counts; the individual mutation manifests record the exact replacements.

| Condition | New/retained test and red assertion | Executed evidence |
| --- | --- | --- |
| One-value const or transitive const change | `TestGoDispatcherFreezeReferencedDeclarationsCLI/one_value_const`, `/transitive_const`: exit1, per-row freeze ERROR, no table proof | Real routeOrderLink221→210: old bypass / new freeze ERROR observed |
| Reorder iota block | same test `/iota_reorder`: changed member position must reject | NOT_RUN (Go tests forbidden locally) |
| Regexp var initializer change | same test `/regexp_initializer`: var source change must reject | Real v2Slug→`.*`: old bypass / new freeze ERROR observed |
| Used type's field change through a var spec | same test `/transitive_type`: changing config.Flag must reject | NOT_RUN |
| Const comment-only edit | same test `/const_comment`: still exit0 and resolves | NOT_RUN |
| Row literal edits are data | retained `TestGoDispatcherFreezeRowsAreDataCLI/add` and `/remove`: route counts change while freeze still resolves | NOT_RUN |
| Row target also directly called as fallback | `TestGoDispatcherFreezeRowTargetAlsoFallbackCLI`: target must be listed, then body change must reject | NOT_RUN |
| Pure row-only target bodies excluded | retained RowHandlerBodyExcluded/StoredRowTargetsExcluded cases: still resolves after target edit; fixtures now have no direct dispatcher call | NOT_RUN |
| Func and generic methods share bare name | `TestGoDispatcherFreezeGenericNamesDeterministic`: exact `Same,box.Same,pair.Same`; same hash over 32 alternating root orders | NOT_RUN |
| Var initializer reaches func/alias/literal/method value | retained PackageFunctionValuesCLI: target body mutation must reject | NOT_RUN |
| Missing/changed/list/unreviewed/unprovable/orphan pins, stale untouched pin | retained R7 CLI cases: fatal rejection independent of TOUCHED; baseline cannot absorb failure | NOT_RUN |
| R6 path negatives and R5 refusal boundaries | ExtraPathDispatchRejectedCLI and HandlerFactoryRefusalLeavesCLI unchanged, along with refusal code and inventory oracle | Byte preservation only; test execution NOT_RUN |

Dropping const-block hashing, var source hashing, type traversal, direct-target reachability or generic qualification makes the corresponding negative/positive assertions fail. Apart from the two real CLI mutations, this is a source-based red map, not a claim of observed fixture-test red→green. CI must execute the full contractdrift suite and its remaining counterexamples. No assertions or acceptance thresholds were relaxed.

An extra orchestration assertion wrongly required the intentional const mutation to have no TOUCHED findings; it stopped after recording the required freeze errors and restoring the file. Withheld table proof yields 4 collateral TOUCHED findings for each negative. `mutation-harness-note.json` records this extra assertion error. No source fix was needed, no const rerun was performed, and both normal acceptance scenarios still strictly require no TOUCHED. Initial candidate-tool package-path compilation was rejected because output is outside the Go module; the second targeted build used explicit source files and succeeded. Initial uncommitted exploration receipts (`fix-build`, candidate inspection) are not the acceptance binding. No blocker exceeded two targeted attempts.

## NOT_RUN, risks and cleanup

- **NOT_RUN:** all local Go tests, new and retained fixture CLI test suites, iota/type/comment/row-data/fallback/generic test execution, full CI, independent r8 review. Scope requested by owner; pins retain pending r8 re-check. Author evidence is not E4 or merge approval.
- **NOT_RUN:** real HTTP runtime routing, transport/differential gates, REAL_PG, browser real-click, SANDBOX/LIVE/release. The executed CLI checks are static inventory evidence only.
- Cross-package helper implementations remain unfrozen; use normal review and other gates. Var initializer source is frozen, runtime mutable values are not snapshots. Same-name method inclusion and whole const blocks are conservative and may require reviewed repins for unrelated neighbouring edits.
- `round8/preservation-audit.json` verifies no `cmd/` or `internal/` diff from starting HEAD and byte preservation of R6 negatives, refusal tests/code and repository inventory tests. Existing routing/runtime code, route contracts, dependency locks and legacy baseline are untouched.
- Three changed Go files indexed (60 entities, status done); `goDispatcherFingerprint` linked to Humaux fix `82d184ed-c652-4dba-9df2-b8902069ed43`. Decisions/fix/acceptance stored; own canvas updated.
- Deleted only this round's owned scratch clone, candidate tools, fixture source trees and binaries: **482859965 bytes**. `round8/cleanup.json` records removal. Unique failure/success evidence retained. No task servers, ports or containers were created; all command processes ended. No push, PR mutation, merge or deployment.
- Rollback point: start `2d8f4604efebb6ef5a593df166e845197bf69b5f`; round-specific commits are separate and can be reverted in reverse order without resetting unrelated work.

---

# Historical delivery below — superseded by Round 8 for this unit

READY_FOR_INDEPENDENT_REVIEW — Round 7. E3 **STATIC built-CLI acceptance** at source `5f6f27b17fc8ccceb5b3f9211db1f0949e04409a`: branch and pinned #70 overlay both exit 0, errors=0, no TOUCHED. E1 build/vet/gofmt/prepush passed. Go tests, observed mutation red→green, CI and independent review NOT_RUN. Pins remain pending independent approval; no push.

# BUYER-ROUTE-TABLE — Round 7 (2026-10-15)

This arbitration replaces Round 6's blocked allowlist approach with reviewed dispatcher change detection. It is a new design, not a third targeted allowlist repair.
Branch/worktree: `unit/buyer-route-table`, `.worktrees/buyer-route-table`; existing PR #71.
Start `13de0deee1fa8e10d3dd753b424070de79debca0`; acceptance source `5f6f27b17fc8ccceb5b3f9211db1f0949e04409a`; comparison base `e4dad98cc322d35709b475de25063efc92e5d866`.
Executor: Codex, integrator scope; actual model/effort identifiers not independently exposed. No subagents. Same coord/owner `8569112d-4a60-4fd1-8763-78c6172f39ff` / `codex-buyer-route-r6`.
Ownership: `scripts/dev/contractdrift/**`, explicit `contracts/route-table-dispatchers.json`, engineering/gate docs and `output/buyer-route-table/**`. Runtime source, baseline, owning route contracts and dependency locks unchanged.
Read Round 6 DELIVERY and original R5 review at the main repository's `output/buyer-route-table/review-r5.md` (absent in this worktree); source review hash recorded in `round7/review-input.json`.

## Changes and commit separation

| Commit | Change |
| --- | --- |
| `bcda30358` | Remove R6 whole-function URL taint gate entirely; restore R5 shape checks. |
| `eec4049c9` | Dispatcher freeze, fatal per-row diagnostics and explicit real-repository pins. |
| `a0d53b08c` | Fixture pins, literal hash anchors and requested freeze CLI tests. |
| `11f64cb1c` | Static self-review hardening: follow package-level stored function aliases/literals/method values. |
| `3877ce305` | Keep row data and stored row-target bodies outside that traversal. |
| `5f6f27b17` | Add stored row-target exclusion counterexamples. |
| Round 7 docs commit | Engineering/gate documentation, this delivery and source-bound receipts. |

R5 shape checks give the existing invalid-consumer/path diagnostics; they are not presented as complete path analysis. R6 taint code is deleted, with no dual gate logic. Freeze requires both SHA256 and exact sorted function list. Missing or changed pins withhold every table route and report ERROR (dispatcher freeze), independently of TOUCHED. Unprovable/orphan/malformed/duplicate pins fail. `-write-baseline` cannot grandfather a freeze failure.
Canonicalization removes comments and whitespace through tokenization, gofmt-normalizes the declarations and hashes sorted receiver-qualified names plus normalized declaration bytes. The pin's `_comment` header and `docs/engineering/contractdrift.md` require explicit independent approval of pin diffs. No automatic pin updates occur during scanning.

## Exact dispatcher closures for independent review

Root selection includes proven HTTP dispatcher functions, both consumers and facade receiver methods. Calls and function values stay within the owning package; ambiguous receiver names conservatively include all same-name methods. Package variable initializers are followed for function targets. Row literals and row handler target bodies are excluded. Current real tables store kinds/method/path data rather than explicit function targets, so no business handler targets were excluded from these real closures.

### `attribution.FeedRoutes` — 10 functions

SHA256: `43d54db98b98e55ba81241abc3bb3c21fa8746f590cc2fce864a4d8720c4c8d8`
Reviewed: `2026-10-15 integrator (pending independent review)`

```text
FeedHandler
FeedRouteTable.MatchesPath
addImageLinks
feedError
feedMethodAllowed
feedPathMatches
feedPrice
imageLinks
loadFeed
writeFeed
```

### `buyerhttp.Routes` — 81 functions

SHA256: `e59984c645e6139403b91cf1a3e88c27e3be101ce2f95d296824a1168ee623fa`
Reviewed: `2026-10-15 integrator (pending independent review)`

```text
RouteTable.HasPrefix
admittedPaymentMethod
allowed
attachV2ImageSizes
bearer
bodyTimeout
buyerTransaction
canonicalSecret
catalogRequest
claimToken
classify
contactDigest
cvsHTTPError
decodeExact
decodeJSON
decodeV2Cursor
designPGError
encodeV2Cursor
forbiddenInput
handler.ServeHTTP
handler.authorizeView
handler.catalogV2Get
handler.claimRequest
handler.collectionMediaGet
handler.consentPut
handler.cvsRequest
handler.cvsServe
handler.designGet
handler.dispatch
handler.mediaGet
handler.orderLink
handler.orderLookup
handler.paymentRequest
handler.primaryOriginGet
handler.privacyErase
handler.privacyExport
handler.privacyGet
handler.storeMedia
handler.transferRequest
handler.viewOnly
isCatalogV2
isDesignRoute
isKeylessPaymentRoute
isPaymentRoute
isPublicRoute
keyFor
keylessPaymentPath
lookupClientIP
mapPublicCatalogError
matchPattern
matchRoute
mobileHint
noBody
normalizeRef
oneHeader
optInt
optionsRequest
ordersRequest
parseMinor
parseV2Query
privacyError
projectCart
projectCatalog
projectCheckout
projectClaimPreview
projectClaimRedeem
projectDestination
projectOptions
projectOrder
projectOrders
projectPromotion
projectQuote
projectQuoteLines
projectV2
scoped
storefrontOrigin
strictDecode
v2Query.digest
viewAllowed
writeImage
writeOK
```

`buildBuyerPayment` is absent from `buyerhttp.Routes`: it lives in `cmd/api`, while the closure is confined to `internal/buyerhttp`. The #70 overlay changes exactly `cmd/api/buyer_payment.go` from pinned `51273344fc12ff4c8062634fdf749d536958bda7`. See `round7/overlay-manifest.json`. A separate detached worktree was used; no runtime file in this branch changed.

## Current acceptance SUMMARY lines

Branch:

```text
SUMMARY routes go=328 contract=165 bff=301; errors=0 warnings=288 CONTRACT_ONLY=33 GO_405_FALLBACK=4 GO_MOUNT_OR_FALLBACK=89 GO_NOT_IN_BFF=27 GO_ONLY=200 GO_ROUTE_TABLE=4 METHOD_MISMATCH=4 UNRESOLVED=51
```

Pinned #70 overlay:

```text
SUMMARY routes go=328 contract=165 bff=301; errors=0 warnings=288 CONTRACT_ONLY=33 GO_405_FALLBACK=4 GO_MOUNT_OR_FALLBACK=89 GO_NOT_IN_BFF=27 GO_ONLY=200 GO_ROUTE_TABLE=4 METHOD_MISMATCH=4 UNRESOLVED=51
```

| Check at source `5f6f27b17fc8ccceb5b3f9211db1f0949e04409a` | Exit / result | Evidence |
| --- | --- | --- |
| `go build ./...` | 0 / E1 | `round7/build-receipt.json` |
| `go vet ./...` | 0 / E1; not a Go test execution | `round7/vet-receipt.json` |
| gofmt all contractdrift Go files | 0, empty / E1 | `round7/gofmt-receipt.json` |
| Integrator `prepush.sh <worktree> e4dad98cc` | 0, `prepush: OK` / E1 | `round7/prepush-receipt.json` |
| Build actual CLI | 0 / E1, binary SHA256 recorded | `round7/build-cli-receipt.json` |
| Actual branch CLI vs e4dad98cc | 0, errors=0, no TOUCHED / E3 STATIC | `round7/branch-receipt.json`, `branch-report.txt` |
| #70 overlay API build | 0 / E1 | `round7/pay70-build-receipt.json` |
| Same built CLI against pinned #70 overlay | 0, errors=0, no TOUCHED / E3 STATIC | `round7/pay70-receipt.json`, `pay70-report.txt` |

All commands have start/end/exit receipts and 180-second timeouts; terminal completion woke the root agent. No task remains running. Initial source `a0d53b08c` receipts are archived under `round7/initial-static-a0d53b08c`; they are historical, not the current acceptance binding. The package-path invocation for the temporary pin-review tool initially failed because output is outside the main Go module; a single correction to explicit source-file arguments succeeded. No acceptance gate failed, no blocker received more than two attempts.

## Red map — prospective test assertions; execution NOT_RUN

`pinFixtureDispatcher` scans through the production in-package fingerprint function, writes the unmodified fixture pin, and asserts literal positive hashes:

- Prefix `bb891b718f9855ea78f60bb77fb3d23fca54c88f933304c21c80bad77198fc89`.
- Exact `52e813a9f77a019c91e6ab9919db7977061658a1e2cc4e019fee4eb9337ee246`.

Negative mutations never repin. Positive listed-path/helper variants explicitly review their own modified dispatcher before invoking the CLI. Two-table fixture setup preserves the other table's pin.

| Condition / regression | Test that goes red; assertion |
| --- | --- |
| (a) Pinned fixture resolves | Existing `TestGoRouteTableResolvesCLI`, `TestGoRouteTableExactSplitResolvesCLI`; new `TestGoDispatcherFreezePinnedAndChangesCLI/pinned`: exit0, GO_ROUTE_TABLE, no UNRESOLVED. Removing freeze alone does not make these positive controls red. |
| (b) Reached helper semantic byte changes (`id !=` → `id ==`) | `PinnedAndChangesCLI/reached helper one byte`: shared `assertFreezeCLI` requires exit1, fatal per-row UNRESOLVED and no GO_ROUTE_TABLE. **Removing freeze makes this assertion red.** |
| (c) Comment-only/doc edits | `PinnedAndChangesCLI/comment only`, `/doc comment`: exit0, GO_ROUTE_TABLE; reintroducing comment/position churn makes these red. |
| (d) Row target body excluded | `TestGoDispatcherFreezeRowHandlerBodyExcludedCLI`, `TestGoDispatcherFreezeStoredRowTargetsExcludedCLI/{alias,literal,method value}`: both CLI runs resolve; body edit keeps exact fingerprint. Including target bodies makes these red. |
| (e) Rows added/removed are data | `TestGoDispatcherFreezeRowsAreDataCLI/{add,remove}`: no UNRESOLVED, GO_ROUTE_TABLE, exact new go/contract counts 4/4 or 2/2. Hashing row data makes these red. |
| (f) Missing file/entry | `TestGoDispatcherFreezeMissingPinsCLI/{file,entry}`: fatal per-route rejection with no GO_ROUTE_TABLE. **Removing freeze makes both red.** Wrong hash/list and unreviewed cases extend this boundary. |
| (g) Pin cannot be proved | `TestGoDispatcherFreezeUnprovedPinErrorsCLI/{unmarked,unconsumed,orphan,invalid rows}`: fatal ERROR naming the unproved pin, no table proof except the unrelated valid table in orphan case. **Removing orphan-pin enforcement makes this red**, even when shape checks still reject the table. |
| Independent of TOUCHED/baseline | `TestGoDispatcherFreezeUntouchedStillErrorsCLI`: stale dispatcher and pin committed at trunk; fatal per-row errors with no TOUCHED, and baseline seeding fails. **Removing freeze or downgrading it to touched-only makes this red.** |
| Stored helpers remain in closure | `TestGoDispatcherFreezePackageFunctionValuesCLI/{alias,literal,method value}`: target body edit alone produces fatal per-route rejection. **Dropping initializer/function-value traversal makes these red.** |
| R6 counterexamples remain effective | Unchanged `TestGoRouteTableExtraPathDispatchRejectedCLI`: all negatives require exit1/UNRESOLVED/no GO_ROUTE_TABLE. Removing freeze while retaining R5 shapes makes the ten R6 alias/helper/Contains/EqualFold/URL method/Opaque/storage cases red; direct equality/prefix/suffix cases can still be rejected by the first shape layer. |
| R5 P2 stays fixed | Unchanged `go_refusal_leaf.go` and `TestGoHandlerFactoryRefusalLeavesCLI`; len-outside-ruling, shadowed writers, unlisted wrapper and below400 cases retained. Reintroducing len folding breaks its negative assertion; freeze removal does not replace this check. |

This is a source-based red map, **not observed mutation red→green**. CI must run the unchanged R6 counterexamples and full contractdrift suite (including TestGoRepositoryInventory), and deliberately remove/restore the freeze checks to record red→green at the acceptance source. New gate existence is not claimed from unexecuted tests.

## NOT_RUN, risks and cleanup

- All local Go tests, fixture CLI test executions, mutation red→green, CI and independent acceptance: NOT_RUN by owner instruction. Pins are candidates marked pending independent review; do not merge on author evidence alone.
- REAL_PG, browser real-click, runtime transport/differential, SANDBOX/LIVE/release: NOT_RUN; this task changes only static inventory tooling/contracts.
- Fingerprints establish reviewed change detection within same-package function boundaries. Imported implementations and other mutable/global data are outside the specified function-source fingerprint. Existing transport gates and independent review remain required; no complete URL proof is claimed.
- Conservative same-name receiver inclusion and current kind-based tables intentionally freeze additional business helpers. Legitimate function edits need a visible reviewed pin diff; table row and comment edits do not.
- `round7/preservation-audit.json` confirms the complete R6 extra-path/refusal test bodies, refusal constant folding code, TestGoRepositoryInventory file, cmd/api/buyer.go and buyerhttp/handler.go are byte-identical to the starting HEAD; the entire runtime diff is empty.
- `round7/source-hashes.json` binds scanner, tests, pins, runtime boundary and docs; `round7/evidence-hashes.json` binds the receipts. Documentation/evidence-only commits preserve acceptance source bytes.
- Code index: eight changed Go files indexed (104 entities, done); `goDispatcherFingerprint` linked to Humaux fix `cd1b543a-139c-4c6f-9a46-930cd8dae4ec`. Retired taint file deletion submitted to the index.
- Only task-owned scratch trees/binaries and detached overlay are removed; unique receipts/reports stay. Cleanup receipt `round7/cleanup.json`; no task processes, servers, ports or containers remain. No push, PR update or merge performed.

---

# Historical delivery below — superseded by Round 7 arbitration and STATIC acceptance

BLOCKED — Round 6 (last targeted attempt). Source `4eb31d7de3f948e452f96078075cb6384f410d67`. Real built CLI exit 1, errors=106, 21 TOUCHED findings. Implementation E1 only; no accepted E3 completion. Integrator arbitration required. STOP applied; no runtime edits, exemption, push, or further targeted repair.

# BUYER-ROUTE-TABLE — Round 6 (2026-10-15)

Branch/worktree: `unit/buyer-route-table`, `.worktrees/buyer-route-table`; existing PR #71.
Starting HEAD `e0855512fd45164328b55fbdce63f0f89cc5ff62`; comparison base `e4dad98cc322d35709b475de25063efc92e5d866`.
Coord `8569112d-4a60-4fd1-8763-78c6172f39ff`: BLOCKED (`input_required`).
Executor: Codex, integration scope; actual model/effort identifiers are not independently exposed. No subagents.
Ownership: `scripts/dev/contractdrift/**`, `output/buyer-route-table/**`; no runtime/schema/lockfile edits.
Humaux problem: `7d5cafee-731f-467a-a111-be1b43a78a22`; own canvas updated; code index submitted for five changed Go files and linked to `goTablePathPolicy`.

## Stop reason and exact uses

The real branch CLI at the source SHA produced explicit `URL-derived use outside route-table allowlist` findings for:

- `internal/buyerhttp/handler.go:279`: `strings.HasPrefix(path, "/v1/buyer/orders/")`.
- `internal/buyerhttp/handler.go:283`: `strings.HasSuffix(path, suffix)`.
- Both receive the URL-derived parameter through `handler.go:293`: `keylessPaymentPath(r.URL.Path)`.

These real uses alone violate the required allowlist. They classify payment failures rather than add routes, but the ruling admits no purpose-based exception. Runtime code was left unchanged. No third targeted repair will be attempted.

Other observed findings: `forbiddenInput` at lines 146–149/159 uses URL nil guards, the reverse canonical equality (`EscapedPath()==Path`), query/ForceQuery values and concatenated path constants; ServeHTTP has URL nil guards at 291/293/301. Full exact locations are in `round6/blocked-findings.txt`.

Known conservative limitations also contribute findings: `r.Context()` at 305 is rejected as an unanalysed request method; the canonical refusal at 315 uses a local `fail` closure outside the currently proven refusal-call grammar; forwarding `r` to receiver method `h.dispatch` at 388 is unproven. These are recognizer limitations, not claims of runtime bugs. The exact feed table is also withheld: `internal/attribution/feed.go:133` tests `r.URL.RawQuery != ""`, outside the literal URL-use allowlist; its generic table/dispatcher findings are retained. The 106 errors include cascading missing-route/contract findings after the 48 buyer/feed table rows are withheld, not 106 independent bugs.

Arbitration is required on the conflict between the specified URL-use policy and existing legitimate uses, plus the remaining proof limitations. The candidate scanner is not presented as accepted or fully validated.

## Commits and preservation

| Item | Commit | Change |
| --- | --- | --- |
| Counterexamples FIRST | `f94f55291` | Ten URL-derived rejected cases, listed-only helper positive, P2 boundary cases. |
| Allowlist candidate | `1d5e17305` | Remove old denylist branches; add monotone local/parameter taint and allowed-parent checks. |
| P2 constant folding | `e534c72af` | Drop len folding; retain explicit constants and simple arithmetic. |
| Additional static boundary review | `4eb31d7de` | Apply policy to exact handler candidates and reject unproved request-method escapes. |

`round6/preservation-audit.json` confirms all 11 original extra-path cases remain unchanged, the entire existing listed-policy positive is byte-identical, and `go_extract_test.go` (including TestGoRepositoryInventory), `cmd/api/buyer.go`, and `internal/buyerhttp/handler.go` are byte-identical to the starting HEAD. The refusal positive previously using len now uses `600-199`; its previous len shape is a new negative.

## Allowlist table

One comment block and one data variable: `scripts/dev/contractdrift/go_table_path_policy.go`, `goTableURLUseAllowlist`.

| Allowed position | Proof / candidate behavior |
| --- | --- |
| Proven table path consumer argument | Plain identifier/URL-field argument; consumer body is the existing table-proof boundary. Transform calls do not qualify. |
| == / != or switch equality with listed path | Constant string value must exactly equal one declared table path. Variables are not folded. |
| Exact `r.URL.EscapedPath() != r.URL.Path` refusal | Same lexical request; exact if guard, recognised refusal call then return. Current recognised refusal call is imported `http.NotFound`. |
| Analysed helper argument | Named local/imported function with parameter taint, analysed to fixed point. Every reached use is checked; derived returns/storage/unknown helpers reject. |
| Request forwarding to proven leaf | Imported `http.NotFound` and analysed helper forwarding supported; dynamic handler/middleware/receiver forms remain UNRESOLVED when proof is unavailable. |

Local variable assignment transfers provenance rather than sanitising it. Struct/global/blank stores, transformed values, URL method values, async uses and escaping URL-capturing closures reject. Immediate synchronous closures are inspected in their enclosing body; unknown closure-parameter forwarding rejects. Request Host and ordinary non-URL fields are outside URL taint. Request aliases preserve the source object. Stdlib type-field identity distinguishes URL fields from method values. The old path/string denylist was deleted; existing consumer/mount grammar remains a separate prerequisite.

## Current built-CLI SUMMARY and gates

Branch command: built `contractdrift -root <this-worktree> -base e4dad98cc`; receipt `round6/branch-receipt.json`, full output `round6/branch-report.txt`.

```text
SUMMARY routes go=280 contract=165 bff=301; errors=106 warnings=288 BFF_NO_GO=37 CONTRACT_ONLY=81 GO_405_FALLBACK=4 GO_MOUNT_OR_FALLBACK=89 GO_NOT_IN_BFF=16 GO_ONLY=200 METHOD_MISMATCH=4 UNRESOLVED=72
```

Required #70 overlay (`cmd/api/buyer_payment.go` from pinned `51273344fc12ff4c8062634fdf749d536958bda7`): **NOT_RUN**, no SUMMARY exists this round. Owner's STOP condition triggered on the first real-repository scenario; no overlay checkout was created. See `round6/pay70-receipt.json`. Historical Round 5 green does not apply.

| Check at source SHA `4eb31d7de` | Exit / result | Receipt |
| --- | --- | --- |
| `go build ./...` | 0 / E1 | `round6/build-receipt.json` |
| `go vet ./...` | 0 / E1 | `round6/vet-receipt.json` |
| `go build -o <round6>/contractdrift ./scripts/dev/contractdrift` | 0 / E1 | `round6/build-cli-receipt.json` |
| `gofmt -l` all contractdrift Go files | 0, empty / E1 | `round6/gofmt-receipt.json` |
| `bash output/integrator/tools/prepush.sh . e4dad98cc` | 0, `prepush: OK` / E1 | `round6/prepush-receipt.json` |
| Real branch built CLI | 1, errors=106, TOUCHED=21 / acceptance FAIL | `round6/branch-receipt.json` |
| #70 overlay built CLI | NOT_RUN after STOP | `round6/pay70-receipt.json` |

The prepush tool was copied byte-for-byte from the existing integrator tool into the requested relative path, fingerprinted in `round6/prepush-tool.json`, then the task-owned copy was removed. Every invoked command has a timeout and start/end/exit receipt. No Go test command was run.

## CI red map — prospective assertions, execution NOT_RUN

All ten new URL negatives use `TestGoRouteTableExtraPathDispatchRejectedCLI`, assertion at `go_route_table_cli_test.go:172–173`: exit must be 1, UNRESOLVED present, GO_ROUTE_TABLE absent. Replacing this allowlist with the old denylist is expected to restore table coverage and trip that assertion for each row. This is source reasoning, not an observed red→green receipt.

| New case | Source line | Old-denylist blind spot / red assertion |
| --- | --- | --- |
| local alias equality | 151 | Comparison sees p rather than r.URL.Path; common negative assertion 172–173. |
| two assignment alias | 152 | No transitive alias provenance; common negative assertion. |
| path parameter helper | 153 | Helper compares a parameter, caller has only an argument; common negative assertion. |
| contains | 154 | Unknown string-function shape; common negative assertion. |
| equal fold | 155 | Unknown string-function shape; common negative assertion. |
| escaped path equality | 156 | URL method result omitted by direct-selector matcher; common negative assertion. |
| URL string | 157 | URL method result omitted; common negative assertion. |
| URL request URI | 158 | URL method result omitted; common negative assertion. |
| URL opaque | 159 | URL field not in old direct-selector set; common negative assertion. |
| struct field storage | 160 | Assignment not checked, comparison reads box.path; common negative assertion. |
| listed-only helper positive | 194–203 | Positive control; exit0/no UNRESOLVED/GO_ROUTE_TABLE at 202–203. Old denylist also passes; this is not a negative mutation witness. |

Existing `/prefix` and `/suffix` cases at 145–146 and their assertion remain unchanged; owner's CI run 38716294967 is the supplied historical red evidence, not a current green run.

| P2 counterexample / positive | Assertion and regression |
| --- | --- |
| len outside ruling (145) | RefusalLeavesCLI negative 196–197 goes red if old len folding is restored and leaf becomes accepted. |
| constant expression below400 (146, `600-400`) | Negative 196–197 guards range checking after arithmetic. Reverting only URL allowlist does not affect it. |
| shadowed http (147) | Negative 196–197 guards import/object identity. |
| shadowed httperror (148) | Negative 196–197 guards import/object identity. |
| unlisted wrapper (149) | Negative 196–197 guards the narrow refusal wrapper grammar. |
| arithmetic positive (`600-199`,144) | Positive 188–193 retains constant arithmetic acceptance. |

## NOT_RUN, risks, handoff

- All local Go tests, including TestGoRepositoryInventory and every new CLI counterexample: NOT_RUN by owner instruction. The inventory acceptance oracle is unchanged; no green claim is made.
- Observed mutation red→green, CI, independent acceptance, REAL_PG, browser/runtime/release gates: NOT_RUN. New gate existence/complete allowlist soundness is not claimed from E1.
- Pinned overlay scenario: NOT_RUN because the real branch hit the explicit STOP rule.
- Conservative proof gaps are listed above; no runtime bug is inferred from them. Unknown forms stay UNRESOLVED. Do not merge this candidate without arbitration and independent acceptance.
- No push, no external mutations, no runtime changes or new exemptions. Own binaries and copied tool removed; no fixture worktrees, containers, ports or pending local processes. See `round6/cleanup.json`.
- Source hashes: `round6/source-hashes.json`; evidence hashes: `round6/evidence-hashes.json`. A subsequent evidence-only commit preserves these source hashes; SHA-bound CLI and static results apply only to those exact source bytes.

---

# Historical delivery below — superseded by Round 6 BLOCKED

READY_FOR_CI — Round 5. Both required built-CLI scenarios: exit 0, errors=0, no TOUCHED. E1 build/vet; static acceptance only. Go tests, mutation red→green, CI and independent acceptance NOT_RUN.

# BUYER-ROUTE-TABLE — Round 5 (2026-10-15)

Source SHA: `5288fd866aec57c5bc1dc9ff4ac6834cae372782`. Starting HEAD: `73fdaea041818f4324d1cc24089a3d874eba60fb`.
Base: `e4dad98cc322d35709b475de25063efc92e5d866`.
Branch/worktree: `unit/buyer-route-table`, `.worktrees/buyer-route-table`; existing PR #71.
Coord: `8569112d-4a60-4fd1-8763-78c6172f39ff` (same owner identity continued from Round 4).
Executor: Codex, integration scope; actual model/reasoning identifiers not independently exposed. No child agents.
Only these four source/test files changed this round: `go_handler_factory.go`,
`go_refusal_leaf.go`, `go_handler_factory_cli_test.go`, `go_extract_test.go`, all
under `scripts/dev/contractdrift/`. Evidence: **STATIC_SOURCE_ONLY**.
This status supersedes Round 4 and all older sections below.

## Changes and separate commits

| Item | Commit | Change and source pointers |
| --- | --- | --- |
| R5-1 tests, committed before fix | `762ab2ea1` | `go_handler_factory_cli_test.go:137` adds the mountFeed-shaped positive and refusal boundary counterexamples. Existing `TestGoHandlerFactoryInlineTerminalRejectedCLI` at `:124` is byte-identical. |
| R5-1 scanner fix | `38798d8c5` | `go_refusal_leaf.go:20` is the single documented writer list; `:41` validates inline shape, parameters, statements, constants and request nonuse; `:193` requires same-table-branch proof. `go_handler_factory.go:344` separates refusals from proven leaves and `:389` applies the branch requirement. |
| R5-2 inventory truth | `5288fd866` | `go_extract_test.go:274` replaces buyer opaque pins with table-derived rows/provenance and no-unresolved checks. Independent AST oracle at `:499`; function-span check at `:608`. |

The writer identities are `net/http.Error(w, M, S)` and the scanned module's
`internal/httperror.Write(w, S, M)`. Only inline FuncLits, optionally wrapped in
http.HandlerFunc/httperror.Middleware, qualify. Each contains exactly the writer
call, optionally followed by an empty return; S is a constant integer 400..599,
M a constant string, and the request parameter is never referenced. The same
table-selected branch must contain a proven handler leaf. Refusals append no
routes and cannot independently prove a handler. Supplied fallbacks outside that
branch do not satisfy this condition.

The branch grammar recognizes a positive table guard body, a negative table
guard's explicit else, or its continuation after an unconditional return.
Unsupported shapes fail closed. Constant recognition supports literals, explicit
AST const bindings, imported net/http constants, simple constant arithmetic and
fixed-array constant len expressions; variables are never folded from initializers.
No runtime code changed, including **cmd/api/buyer.go**. No baseline, owning
contract, dependency or namespace exemption changed.

## Per-condition red map — CI execution NOT_RUN

All entries use the real built-CLI test helper. This table records the exact
assertion and deliberate regression CI must exercise; it is **not an executed
red→green claim**. `go_handler_factory_cli_test.go:176` is the positive check:
exit=0, exact five-route SUMMARY, no UNRESOLVED; `:180` requires both factory
proof notes. `:184` is the negative check: exit=1, both buildComposite/buildTop
opaque findings present, neither resolved factory note present.

| Regression / dropped condition | Test / source line | Assertion that must go RED |
| --- | --- | --- |
| Remove refusal exception entirely | `RefusalLeavesCLI/mountFeed` `:142`; also literal+return `:143` and constant expression `:144` | Positive `:176` and factory-proof `:180`: the rejection again poisons the parents. |
| Widen inline terminals to any content leaf | Existing `InlineTerminalRejectedCLI` `:124`, unchanged inline 204 | Existing negative `:130`: requires exit=1 and both opaque factories with no proof notes. |
| (a) Widen recognized writers to arbitrary calls | `/unrecognised writer` `:153` | Negative `:184`; treating pretendError as a writer wrongly resolves both factories. |
| (a)/(c) Permit statements after the writer | `/second statement` `:151` | Negative `:184`; ignoring the second WriteHeader would resolve the leaf. |
| (b) Remove error-status range requirement | `/2xx error writer` `:145`, `/3xx error writer` `:146` | Negative `:184`; a 204/302 written with http.Error remains rejected. |
| (b) Fold variables as constants | `/variable status` `:148` | Negative `:184`; treating a variable initializer as a compile-time status resolves it. |
| (b) Permit a dynamic message | `/dynamic message` `:152` | Negative `:184`; treating the message variable as constant resolves it. |
| (c) Permit request references | `/request reference constant message` `:150` | Negative `:184`; `401 + 0*len([1]*http.Request{r})` is constant, so removing only the request-reference check must admit it and break this assertion. Positive constant expression `:144` uses nil instead. |
| (c) Read r.URL.Path | `/request read` `:149` | Negative `:184` stays required; this case also violates constant-message (b), so the preceding constant-expression case isolates (c). |
| (d) Count only refusal leaves as proof / count an outside fallback | `/refusal only` `:147` | Negative `:184`; removing same-branch sibling validation makes the outside `next` fallback falsely suffice. |
| (d) Drop table-branch containment | `/outside table guard` `:154` | Negative `:184`; the refusal precedes table interception and cannot borrow the later child proof. |

CI should run the unchanged inline-terminal test, new RefusalLeavesCLI and full
contractdrift suite, plus TestGoRepositoryInventory and existing buyer/cmd/api
gates. CI-only mutation runs should apply the regressions above independently,
observe the named red assertion, restore source, then record green at the same
source revision. No local Go test or mutation command was run.

## R5-2 replacement and grep results

The source oracle parses the marked private tables behind buyerhttp.Routes and
attribution.FeedRoutes, resolves source constants/concatenations, and derives all
expected normalized METHOD+path keys. It requires 46 distinct buyer rows plus
the two feed GET/HEAD rows. Every key must exist in inv.Routes with its exact
owning source row location and a GO_ROUTE_TABLE note whose owning-table span
covers that row (`internal/buyerhttp/routes.go` / `internal/attribution/feed.go`).
No inv.Unresolved may have a `/v1/buyer` path, name one of buildBuyerHandler /
buildBuyerWithCVS / mountFeed, or overlap those cmd/api/buyer.go function spans.

Other opaque entries remain byte-identical: PREFIX /v1/meta, PREFIX /v1/stripe,
PREFIX /.well-known/lc-domain-check/, EXACT /v1/platform/stripe/webhook, EXACT
/internal/tls-ask. Their existing span pairs and check bodies are unchanged.
`round5/preservation-audit.json` verifies this after removing only the two buyer
list entries and the buyer span pair. It also verifies the existing inline 204
test and cmd/api/buyer.go are byte-identical to the starting HEAD.

Grep of **every contractdrift *test.go** for /v1/buyer found the obsolete opaque
expectations only in TestGoRepositoryInventory. Other matches are finite-route
BFF assertions/fixtures in `bff_test.go:82`, `:87`, `:107` and
`bff_review_r1b_cli_test.go:76`, `:94`; they do not pin buyer opacity and were
left unchanged. Full grep evidence: `round5/buyer-test-grep.txt`.
The preservation comparator's first attempt removed overlapping text in the
wrong order; its second attempt corrected the comparator and passed, with no
source change. This is recorded in preservation-audit.json.

## Static acceptance receipts

Branch SUMMARY (exact stdout):

```text
SUMMARY routes go=328 contract=165 bff=301; errors=0 warnings=288 CONTRACT_ONLY=33 GO_405_FALLBACK=4 GO_MOUNT_OR_FALLBACK=89 GO_NOT_IN_BFF=27 GO_ONLY=200 GO_ROUTE_TABLE=4 METHOD_MISMATCH=4 UNRESOLVED=51
```

Pinned #70 overlay SUMMARY (exact stdout):

```text
SUMMARY routes go=328 contract=165 bff=301; errors=0 warnings=288 CONTRACT_ONLY=33 GO_405_FALLBACK=4 GO_MOUNT_OR_FALLBACK=89 GO_NOT_IN_BFF=27 GO_ONLY=200 GO_ROUTE_TABLE=4 METHOD_MISMATCH=4 UNRESOLVED=51
```

Both receipts record exit **0**, errors **0**, empty ERROR and TOUCHED arrays,
source SHA and scanner binary SHA256. This is static contract drift acceptance,
not execution of handlers or of the new Go tests. The unchanged global inventory
still has 51 baseline/non-touched UNRESOLVED and 288 warnings; neither count is
presented as runtime acceptance or a clean global inventory.

| Command / scenario | Exit / result | Receipt (under round5/) |
| --- | --- | --- |
| `go build ./...` | 0, E1 | `build-receipt.json` |
| `go vet ./internal/buyerhttp ./internal/attribution ./cmd/api ./scripts/dev/contractdrift` | 0, E1, includes test compilation/static analysis only | `vet-receipt.json` |
| `gofmt -l` on all changed Go files vs e4dad98cc | 0, empty stdout | `gofmt-receipt.json` |
| `git diff 73fdaea04 --check -- cmd internal scripts contracts` | 0 | `source-diff-check-receipt.json` |
| `go build -o <round5>/contractdrift ./scripts/dev/contractdrift` | 0 | `build-cli-receipt.json` |
| Built CLI `-root <this-worktree> -base e4dad98cc` | 0, errors=0, no TOUCHED | `branch-receipt.json`, `branch-report.txt` |
| Built CLI `-root <round5>/pay70-scratch -base e4dad98cc` | 0, errors=0, no TOUCHED | `pay70-receipt.json`, `pay70-report.txt` |
| Overlay `go build -o <round5>/pay70-api ./cmd/api` | 0 | `pay70-build-receipt.json` |
| `bash output/integrator/tools/prepush.sh . e4dad98cc` | 0, **prepush: OK** | `prepush-receipt.json`, `prepush-report.txt` |

The overlay clone was detached at 5288fd866aec57c5bc1dc9ff4ac6834cae372782. Its **only** working-tree diff
was cmd/api/buyer_payment.go from pinned
`51273344fc12ff4c8062634fdf749d536958bda7`, SHA256
`f460f3f4e97f9d9dfc090a3be5094e3c9d01f71545eebbcaf9321360a40a044f`.
No extra #70 files were required. See pay70-setup.json and clone/checkout receipts.
The missing relative prepush tool was copied unchanged from the main workspace,
run, then removed; prepush-tool.json records origin/hash. The requested
integrator-notes.md was also absent here and was read from the main workspace
without changes; handoff-read.json records that source/hash.

Initial branch acceptance also exited 0; initial-branch-* evidence is retained.
The final gate runner inadvertently invoked the identical branch scan three
times while checking three output tokens; all exited 0 and the final receipt is
retained. No source changed between them; this added no acceptance scope.
Every command had a timeout and foreground completion observation; no wait or
unmonitored background process remains.

## NOT_RUN, risks and handoff

- **NOT_RUN**: all Go tests (including new positive/counterexamples and inventory
  oracle), mutation/red→green execution, runtime/PG/browser/HTTP, CI jobs and
  independent review. Required next step is integrator CI + independent acceptance.
- **Risks**: this remains a narrow static AST recognizer. Unsupported constant or
  control-flow forms stay opaque. net/http named statuses use the local Go
  toolchain's export importer; unavailable export data fails closed. No runtime
  completeness/performance/release claim is made.
- **Cleanup**: only this task's clone and binaries removed, **471,982,906 bytes**;
  cleanup.json lists each. No server, browser, port, container or runtime fixture
  was started. Logs, source/evidence hashes and failure information are retained.
- **Binding**: source-hashes.json covers all unit source/contracts changed vs the
  base, including the four R5 files. Evidence-hashes.json binds retained R5
  evidence. Later delivery-only commits do not change these source bytes.
- Code index updated for the four changed files (58 entities); refusal/refusalBranch
  linked to Humaux fix memory `49903b8f-f79b-4a8f-b184-5a30b23e9b6b`.
- **No push**, PR mutation, merge, deployment or production action.

---

> **Round 4 — historical BLOCKED evidence.** Retained unchanged below; current
> status is the Round 5 READY_FOR_CI section above.

BLOCKED — Round 4: both required built-CLI scenarios exit 1, errors=2. Strict inline-terminal rejection exposes the existing feed-auth refusal. Build/vet E1; prepush OK; gofmt empty. No runtime/CI acceptance.

# BUYER-ROUTE-TABLE — Round 4 (2026-10-15)

Source SHA: `8025b50108eacf2c2f0fbc2d2d6b4c76d6ccc945`. Starting HEAD: `b5aabc8a41c2d007eacb2de3ae40efa3512a7c1d`.
Base: `e4dad98cc322d35709b475de25063efc92e5d866`.
Branch/worktree: `unit/buyer-route-table`, `/Volumes/data/live_commerce_architecture_v1/.worktrees/buyer-route-table`.
Coord: `8569112d-4a60-4fd1-8763-78c6172f39ff`; executor Codex, integration scope.
Actual runtime model/reasoning identifiers are not independently exposed. No child agents.
Evidence mode: **STATIC_SOURCE_ONLY**. Go tests, mutations/red→green, runtime,
PG, browsers, CI and independent acceptance: **NOT_RUN** per the owner restriction.
This section supersedes earlier READY_FOR_CI statuses below.

## Targeted changes and commits

| Review item | Change | Commit(s) |
| --- | --- | --- |
| 1 — hidden path dispatch | Both table resolution and factory Dispatch proof check reached functions. Unlisted Path/RawPath/RequestURI comparisons, switch cases, prefix/suffix calls and indexed lookups stay opaque; literal/constant equality must name an actual nonparameter row. Transformed path comparisons/switches fail closed. Row.Path is distinguished from request.URL.Path. | `21540055e`, `c4cd5456b`, `f62c1b946`, `8025b5010` |
| 2 — inline terminal | Removed terminal relaxation entirely. Inline FuncLits must forward a proved handler; they never gain leaf coverage from a table mount guard. Actual consumed Dispatch functions and handler parameters retain their proof. | `13ddd1dd0` |
| 3 — mutable exports | `routeRows` and `feedRows` are private slices; exported `Routes` and `FeedRoutes` are fieldless struct values exposing HasPrefix/MatchesPath. `mount=...` binds the marker to this validated form. Exported row slices are rejected; full-range owning-package read whitelist remains. External indexing/field writes through these values are a compile-time impossibility. CI fixtures now use private rows; tail counterexamples remain valid source. | `5b418bc5b`, `17dd04b56` |
| 4 — duplicate feed path | Removed unused production `feedPath`; TestMountFeed owns its fixture path and asserts FeedRoutes.MatchesPath before use. | `a13b7c5e3` |
| 5 — line cites | Corrected primary noBody `:21`, PrimaryOrigin `:25`, proof decodeJSON `:36`, and every other stale pointer found in the canonical index. All 80 references re-read, with line text and source SHA256 in `round4/citations.json`; checked against committed source. | `029cf3137` |

Runtime route rows, precedence, matching, method policy and authentication/response
implementation remain intact. No baseline edits, subtraction from opaque spans,
namespace exemptions or changes to owning contracts. Private storage changes do
not change table order or values. Final delivery-only edits do not alter the
source binding: `round4/source-hashes.json` covers all 11 changed source/doc files.

## Required existing literal comparisons

All are accepted because their resolved values equal actual table row paths;
`round4/literal-comparisons.json` records the row locations.

| Handler source | Exact table paths checked |
| --- | --- |
| `handler.go:147` | `/v1/buyer/catalog`, `/v1/buyer/checkout-options`, `/v1/buyer/orders`, `v2Prefix+"products"` = `/v1/buyer/catalog/v2/products` |
| `handler.go:159` | `claimLinkPath` = `/v1/buyer/claim-link`; `claimRedeemPath` = `/v1/buyer/claim-link/redeem` |
| `handler.go:291` | `/v1/buyer/session` |
| `handler.go:301` | The same claim-link/redeem pair |

Existing paired same-request URL.EscapedPath/URL.Path comparisons at 146/159/315
remain canonicality checks. Pattern-helper semantics and arbitrary Go alias/data
flow remain outside this narrow recognizer's runtime proof; no runtime-completeness
claim is made. Unknown supported path decisions retain UNRESOLVED.

## CLI counterexamples and the exact red assertions

These are committed **real built-CLI** tests using the existing Git/OS process
helpers, not a parser stand-in. **Execution and fix-reversion mutations NOT_RUN**
locally. The assertions below describe what CI must demonstrate; they are not a
red→green PASS.

| New counterexample | Assertion that must go red when its corresponding fix is reverted |
| --- | --- |
| `TestGoRouteTableExtraPathDispatchRejectedCLI/path equality` — extra `/v1/table/debug` before both consumers | exit=1, contains UNRESOLVED, **contains no GO_ROUTE_TABLE**; reverted reachability-only proof resolves it |
| Same test `/raw path equality` | same rejection assertion for URL.RawPath |
| Same test `/request URI equality` | same rejection assertion for RequestURI |
| Same test `/constant equality` | same rejection assertion for constant debugPath |
| Same test `/path switch` | same rejection assertion for switch URL.Path with unlisted case |
| Same test `/prefix` | same rejection assertion for HasPrefix(URL.Path, unlisted prefix) |
| Same test `/suffix` | same rejection assertion for HasSuffix(URL.Path, unlisted suffix) |
| Same test `/map lookup` | same rejection assertion for map[URL.Path] |
| Same test `/transformed path` | same rejection assertion when TrimPrefix(path) is compared with a listed literal |
| Same test `/transformed switch` | same rejection assertion when transformed switch tag has a listed case |
| Same test `/reached helper` | same rejection assertion for a called helper comparing URL.Path with debugPath |
| `TestGoHandlerFactoryInlineTerminalRejectedCLI` — inline 204 under exact table mount guard | exit=1, both buildComposite/buildTop opaque findings present, **neither resolved factory note present**; reverting terminal fix restores the false proofs |
| `TestGoRouteTableMutationRejectedCLI/prefix` and `/exact` — owning-package rows[0].Method assignment | exit=1, invalid/incompletely-consumed table finding, **no GO_ROUTE_TABLE**; removing the owning-package write guard makes the counterexample known |
| `TestGoRouteTableExportedMutationRejectedCLI` — old exported table + importing-package method assignment | exit=1, UNRESOLVED, **no GO_ROUTE_TABLE**; reverting private-table/facade recognition lets this exported slice resolve again. The new runtime facade itself cannot be indexed; no local compile-failure test was run |
| `TestGoRouteTableListedPathPolicyAcceptedCLI` — listed constant equality plus tagless switch | exit=0, no UNRESOLVED, GO_ROUTE_TABLE present; reverting nil-tag handling reproduces the CLI panic, not success |

The feed fixture equality assertion is also committed in TestMountFeed. Test suites
and all fix-reversion runs belong to CI; no local test command was executed.

## Required static acceptance — FAIL, honest BLOCKED

Both commands used the same built scanner binary (SHA256 recorded in receipts):

```text
output/buyer-route-table/round4/contractdrift -root /Volumes/data/live_commerce_architecture_v1/.worktrees/buyer-route-table -base e4dad98cc
output/buyer-route-table/round4/contractdrift -root /Volumes/data/live_commerce_architecture_v1/.worktrees/buyer-route-table/output/buyer-route-table/round4/pay70-scratch -base e4dad98cc
```

Both exact stdout SUMMARY lines are identical:

```text
SUMMARY routes go=328 contract=165 bff=301; errors=2 warnings=288 CONTRACT_ONLY=33 GO_405_FALLBACK=4 GO_MOUNT_OR_FALLBACK=89 GO_NOT_IN_BFF=27 GO_ONLY=200 GO_ROUTE_TABLE=2 METHOD_MISMATCH=4 UNRESOLVED=53
```

`round4/branch-receipt.json` and `round4/pay70-receipt.json`: exit **1** each;
**2 ERROR (TOUCHED) UNRESOLVED** each, on `buildBuyerHandler` and
`buildBuyerWithCVS`. Full stdout/stderr are retained in `round4/branch-report.txt`,
`round4/branch-stderr.log`, `round4/pay70-report.txt`, `round4/pay70-stderr.log`.
These are failed acceptance receipts, not errors=0.

The #70 scenario uses the Round 3 method: task-local shared clone detached at
this exact source SHA; only `cmd/api/buyer_payment.go` overlaid from pinned
`51273344fc12ff4c8062634fdf749d536958bda7`, SHA256
`f460f3f4e97f9d9dfc090a3be5094e3c9d01f71545eebbcaf9321360a40a044f`.
`go build -o <task-output>/pay70-api ./cmd/api` exits **0** on first attempt;
no extra #70 files required. Setup and command receipts are in `round4/pay70-setup.json`,
`round4/pay70-build-receipt.json`, `round4/pay70-clone-receipt.json`,
`round4/pay70-checkout-receipt.json`.

### Exact blocker

`cmd/api/buyer.go:188` passes an inline FuncLit returning 401 through
httperror.Middleware and calls ServeHTTP in mountFeed's BFF-auth refusal branch.
The tightened item-2 rule rejects **any inline terminal leaf**. The actual
buyerhttp.New/ServeHTTP and attribution.FeedHandler leaves still prove, but this
refusal leaf poisons mountFeed and therefore both parent factories. Diagnostic
built-CLI traces in `round4/factory-debug-stderr.log` show that chain; temporary
instrumentation was restored, and is absent from committed source.

Resolving this requires an explicit scope/rule ruling for the real refusal path.
This round leaves the runtime authentication flow intact and does not add a
status-code exception, parameter-name exception, baseline or fake table coverage.
Required errors=0 is **unmet**; status is **BLOCKED**, not READY_FOR_CI.

Initial source `17dd04b56` also hit an AST nil-tag panic. Its failed exit-2
receipt/stdout/stderr are preserved as `round4/initial-failed-branch-*`.
The nil-tag fix is committed; final CLI runs finish normally with exit 1.
Earlier factory-failure receipts at `f62c1b946` remain `round4/factory-failed-*`.

## Other checks, cleanup and limits

| Command | Exit / observed result | Evidence |
| --- | --- | --- |
| `go build ./...` | 0 — E1 | `round4/build-receipt.json` and stdout/stderr |
| `go vet ./internal/buyerhttp ./internal/attribution ./cmd/api ./scripts/dev/contractdrift` | 0 — E1, includes test compilation/static analysis; no execution | `round4/vet-receipt.json` |
| `go build -o output/buyer-route-table/round4/contractdrift ./scripts/dev/contractdrift` | 0 — E1 | `round4/build-cli-receipt.json` |
| `bash output/integrator/tools/prepush.sh /Volumes/data/live_commerce_architecture_v1/.worktrees/buyer-route-table e4dad98cc` | 0, prints **prepush: OK** | `round4/prepush.log`, `round4/static-checks.json` |
| `gofmt -l` on all changed Go files vs e4dad98cc | 0, **empty output** | `round4/gofmt.log`, `round4/static-checks.json` |
| `git diff b5aabc8a4 --check -- cmd internal scripts contracts` | 0, empty output | `round4/source-diff-check.log` |

The existing prepush tool was absent in this worktree. The identical script
from the main workspace's output/integrator/tools was temporarily copied to the
specified relative location, executed unchanged, then removed. Its path/hash are
recorded. Header/package blank lines remain intact; depmap check passes.
An additional whole-unit `git diff e4dad98cc --check` exits 2 due to trailing
spaces in **historical raw Round 3 CLI stdout**, retained as evidence rather than
rewritten; `round4/diff-check.log` records it. Source-only whitespace check passes.

Task-owned scratch clone, binaries, debug source backup and obsolete temporary
build logs were removed (465230309 bytes); `round4/cleanup.json` lists them.
All foreground processes exited. No servers, browser sessions, ports, containers
or runtime fixtures were started. Failure evidence is retained. Source and
nonmanifest evidence SHA256 receipts are `round4/source-hashes.json` and
`round4/evidence-hashes.json`.

No push, PR, merge, deployment, live credential/payment/customer action or data
mutation. No E2/E3/E4 success or independent-acceptance claim. Handoff requires
the refusal-path ruling above, then both errors=0 CLI scenarios and CI/independent
review on the resulting exact source SHA.

---

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
