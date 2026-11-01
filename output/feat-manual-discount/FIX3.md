# FEAT-MANUAL-DISCOUNT FIX3 — implementation submitted for CI

## Relay 3 revalidation — 2026-11-01 05:00 UTC — STOP unchanged

- Direct `gh pr view 394` / `gh pr checks 394` readback still identifies head `6b9dea6fb60efac198dbbc9a834f22d6504dbbaa`, base `r3/integration`. Read all five failing jobs through `gh run view 41273609804 --log-failed --job <id>`, filtered before entering context. Foundation 41273609808/job124751222877 passed; Gates 41273609804 failed. No older head substituted for current evidence.
- g05/job124751840657: lines136/206/224/311/373 of `tests/foundation/manual_discount_test.go` respectively fail buyer adjustment projection, two discount refusal codes, directory-authoritative pickup and COD refusal code. g10/job124751840942: `catalog_v2_smoke_test.go:258` receives original 3x2 PNG. unit/job124751840781: `TestGetExactDetailAndMissingOrder` panics on an unexpected authorization query. Receipts/job124754991124 explicitly reports these same three failed modes; required/job124755114247 fails downstream. These are E3 negative evidence, not GREEN.
- Source revalidation confirms `internal/buyerhttp/projections.go` omits the adjustment field and `internal/httperror/error.go` maps unknown codes to `internal`. Neither backend package is in the FIX3 named grant; the preamble explicitly requires STOP for unowned backend packages. The supplied revision resolves the earlier migration/shared-money-package blockers, but supplies no further scope authorization for these paths.
- `mdCVS` still supplies `123456`; that exact code has no match in migration0206's directory. Its current `record_buyer_cvs_store` refuses missing directory entries. The explicit no-test-change instruction remains active. Required integrator ruling: authorize a directory-backed test fixture while preserving every assertion, and the buyerhttp/httperror paths. Do not weaken directory validation, seed synthetic production rows, or edit the applied migration.
- No source/test edit, commit, push, local test/server, recursive delegation, new CI run or additional fix attempt. Only this task report was appended; its preexisting changes and other preexisting untracked evidence were preserved. Humaux/coord/canvas remain unavailable in the callable tool inventory; no shared-state action is claimed. No process, port, container or runtime fixture started.
- WAIT_RUN: NONE (no push; both current-head workflows completed). GREEN: absent. Accepted RED remains Foundation41093578253/job124193423117 and Gates41093578464/jobs124197225702,124197225711,124197225869,124197225752,124197225919. NOT_RUN: all local tests/servers, new implementation verification, independent acceptance, SANDBOX/LIVE and deployment. Author acceptance E0. Resume trigger is an explicit integrator scope/fixture ruling, not another unchanged CI relay.

## Relay 2 on head 6b9dea6fb — STOP, scope and immutable-fixture blockers

Fresh GitHub readback on 2026-11-01 binds all findings below to head `6b9dea6fb60efac198dbbc9a834f22d6504dbbaa`, PR #394 into `r3/integration`. Foundation **41273609808 / 124751222877 passed**; Gates **41273609804 failed**. This is E3 negative runtime evidence, not GREEN or acceptance. Jobs: g05 **124751840657**, g10 **124751840942**, unit **124751840781**; receipts **124754991124** and required **124755114247** failed downstream. Logs were read using `gh run view --log-failed --job` and retained locally as `output/feat-manual-discount/relay2-<job>.log`. Older-head results were not used as current acceptance.

| Priority / job | Current failure and source-backed cause | Required repair / authorization |
| --- | --- | --- |
| P1 / g05 | Buyer order JSON loses `adjustments` (`manual_discount_test.go:136`), although the persisted quote assertion already passed. `internal/buyerhttp/projections.go:285` defines an explicit order quote DTO with currency/lines/amount/promotion only; `projectOrder` does not copy adjustments. | Authorize `internal/buyerhttp/projections.go` (and its owning buyer contract if needed). FIX3 names storefront/pricing/catalog/merchantorders, not buyerhttp. Add the bounded field to the buyer projection; do not change expected assertion. |
| P1 / g05 | Over-subtotal and COD cap return HTTP422 `internal`, instead of `discount_exceeds_subtotal` / `cod_limit`. `internal/merchanttools/manual.go:611` already produces the codes, but `internal/httperror/error.go:294` replaces codes absent from its fixed message map; both codes are absent. | Authorize `internal/httperror/error.go`, outside the assigned backend packages. Add only the two contract codes; retain the fail-closed map. |
| P1 / g05 | Pickup happy path returns `store_not_found` (`manual_discount_test.go:311`). Its `mdCVS` fixture at line33 uses `123456`; exact search finds no such code in `migrations/0206_cvs_store_directory.sql`. The current directory-authoritative `record_buyer_cvs_store` at lines13717–13720 correctly refuses absent codes. | Integrator must resolve the explicit **no test changes** restriction for this stale fixture, or supply another approved test setup. Do not bypass directory validation, silently translate the code, edit an applied migration, or insert a fake production directory row. |
| P1 / unit | `TestGetExactDetailAndMissingOrder` panics `unexpected authorization query`: `merchantorders.Get` unconditionally performs the new `read_order_adjustments` call even for the legacy storefront/no-discount projection; the existing fake permits the prior reads and final auth check only. | In assigned merchantorders, keep legacy storefront detail reads unchanged and query manual adjustments only for the manual-order source, with full validation. Do not edit/weaken the fake or assertions. |
| P1 / g10 | Picker rendition returns 3x2 PNG (`catalog_v2_smoke_test.go:258`), while its new assertion requires JPEG at width <=360. `internal/httpapi/images.go:54` always calls original-only `catalog.GetImage`, ignoring the width; the existing stored JPEG renditions are not selected. | Reuse `catalog.product_image_sizes` under the same ownership fence and strict width grammar; authorize the catalog HTTP adapter path if not included in the owning-adapter scope. Preserve original fallback/no-store from media-sizes-v1; no decode/write on GET or new pipeline. |

These are root integration omissions plus an obsolete fixture; this relay did not retry the already-resolved route-declaration blocker. **STOP** follows the dispatch's mandatory stop for an unowned backend package and its no-test-change rule. No source/test/fixture/schema edits, no commit/push, no local tests/server, and no new CI run in this relay. `WAIT_RUN: NONE` — both current-head workflow runs already completed. Integrator scope/fixture ruling is the wakeup, not another CI poll. Current tracked diff consists only of this requested task report; all downloaded evidence stays within this card's output directory.

Humaux discovery exposed no callable memory/coord/canvas tools in this relay. No shared-state read/store/update/claim is asserted; as a dispatched worker, no new claim or recursive delegation was made. No production operation or rollback occurred. Existing branch/head is the rollback/reference point; untracked files predating this relay were preserved.

Verification: no local runtime command or test was run. Source-only inspection and GitHub logs establish the causes above; no new E1 build claim is made for this unchanged source. GREEN/E4 remain NOT_RUN. Existing accepted RED remains Foundation41093578253/job124193423117 and Gates41093578464/jobs124197225702,124197225711,124197225869,124197225752,124197225919 at tests-only head fc28de173. Author repair proposals are E0 until implementation and this PR's own CI.

## Relay 4 revalidation — unchanged STOP conditions (2026-11-01)

Fresh `gh pr checks 394` and failed-job reads still bind to head `6b9dea6fb60efac198dbbc9a834f22d6504dbbaa`: Foundation41273609808/job124751222877 passed; Gates41273609804/jobs124751840657,124751840942,124751840781 failed, with receipts124754991124 and required124755114247 downstream. No later head or new run exists on this PR at readback. These are E3 negative evidence; GREEN is absent.

The FIX3 revision in the repeated dispatch authorizes storefront/pricing/catalog/merchantorders but still does not authorize buyerhttp or httperror. Source confirms the same omissions and immutable-fixture blocker documented above. Required integrator ruling: authorize `internal/buyerhttp/projections.go`, `internal/httperror/error.go` and the catalog transport `internal/httpapi/images.go`; permit changing only the directory-backed CVS fixture in `tests/foundation/manual_discount_test.go` while preserving all assertions. No production directory mutation or safety bypass is acceptable. The merchantorders legacy-read fix remains in scope, but the card's mandatory STOP prevents a partial implementation push while these blockers remain.

Only this existing report was appended; no source, tests, migration or generated file changed, no commit/push/CI rerun, no local tests/server/process/port/container. Preexisting report edits and untracked failure evidence were preserved. No additional targeted repair attempt used. `WAIT_RUN: NONE` because both exact-head workflows completed; integrator scope/fixture ruling is the resume trigger. Author acceptance E0; runtime GREEN, independent review/E4, SANDBOX/LIVE, deployment and UI parity remain NOT_RUN.

Humaux tools are available in this relay: targeted search, rejected enumeration (representative 3/9, coverage0.3333), development-rules original and canvas were read. Findings are stored and this worker's own sub-element is updated under a shared-state advisory lock. This corrects tool availability for this relay only; earlier reports describe their own tool inventory. As a dispatched worker, no new task claim or recursive delegation was made.

## Relay 1 on implementation head 057296e71 — targeted fix attempt 1

Fresh GitHub readback: Foundation **41273018913 / 124749483363** and Gates **41273018917 / 124749483663** both stopped at `ERROR (TOUCHED) GO_ONLY POST /v1/admin/stores/{}/orders/for-buyer`; required **124749633021** failed downstream. These are implementation-gate failures, not a second feature RED. Runtime tests were not reached and GREEN remains pending.

Root cause: the new endpoint table used `/orders/for-buyer` with a prose prefix, whereas `scripts/dev/contractdrift/contracts.go` recognizes only full `/v1/` paths or an explicit `Route base:` directive. Added the full existing route to this amendment without changing older amendments, runtime code, tests or extractor behavior. Removed exactly its now-stale `GO_ONLY POST /v1/admin/stores/{}/orders/for-buyer` baseline row (shrink only; no exemption added). This is the card's explicit stale-baseline exception; **`scripts/dev/contractdrift/baseline.json` is the only additional shared path**, for integrator review. All other baseline bytes remain identical. Dispatcher manifest, route pins, migration and R2 count unchanged.

Rollback point: committed implementation `057296e71ce15c307beba9d04e3e47ce34e35fed`; this fix is a separate reviewable commit. Static checks: depmap regeneration, browser ownership regeneration and `--check`, packet structure check and `git diff --check` exited 0; generated outputs unchanged after restoring only the packet-check timestamp. These are E1, not runtime acceptance. Next push's actual run IDs are reported in the PR comment. No local tests/server or contractdrift CLI run. No new feature, browser spec/page/route or dependency. At most one more targeted attempt is allowed for this blocker if CI still fails on it.

Authority: integrator revision 2026-11-01 in dispatch (migration **0216**, explicit shared-package/contracts authorization), owner words in the card, AGENTS.md, 架构.md §§0/1/2/26/27, storefront-v2 §G3 and cart-quote-v1. Humaux owner original: 1901e990-b789-4ed0-9e2f-1b4f5fff3aea; implementation decision: 555e76c2-4a1b-4969-9601-f7129194ea47. Development detail 7bdb220b-0798-4b1d-aff5-16ee94cae75f read; rejected enumeration 3/9, representative coverage 0.3333 only. Initial discovery did not expose Humaux; renewed discovery did and actual reads/store/own child-canvas update followed.

Dispatched implementation worker, no recursive delegation or new claim. Runtime identifies Codex; exact underlying model/reasoning metadata UNKNOWN (no invented configuration). Own branch `unit/feat-manual-discount`, PR #394 into `r3/integration`; integration base `fde11c1310fce20b576fc98023bd3003c66c205b`, local pre-implementation merge `d1bec260ab78e30eb25c6a8c6ee93a7cbddb9b09`. Integration re-fetched unchanged before push. Only this worktree is writable; main checkout was read-only, including temporary read-only access to installed compiler/dependencies.

## Result and money path (author implementation claim E0)

- Manual and for-buyer inputs accept a bounded named fixed discount, normalize its name, retain it in response/receipt/audit, and keep every item price server-owned. Optional HTTP field preserves all existing required keys, duplicate/unknown-key and scope protections.
- Merchant-scoped immutable authorization hashes the whole normalized request, including a no-discount tombstone, before quote creation; a changed discount cannot race an unfinished receipt into a different order. Quote binding happens once under both existing permissions. Public buyer CreateQuote has no manual adjustment input. Buyer/checkout roles have read-only owner-fenced access to the authorization.
- The frozen quote stores the named adjustment and exact per-line net money; checkout revalidation proves the same authorization/binding/allocation. Integer proportional floors with bounded last-line remainder and backwards spill conserve every minor unit; 128-bit products avoid overflow. Shipping stays outside the discount. COD checks the final total plus surcharge through existing Begin and exposes `cod_limit` for the manual pipeline's amount refusal.
- Existing payment, bank confirmation/offline refund, pickup/COD collection, amount-based Stripe refund capacity, returns quantity/restock separation, settlement 0208 item completeness and reporting-v2 frozen-line allocation are retained. No provider code or second total source was introduced. API/BFF models preserve adjustments for future UI; no UI pages changed. UI: pending PARITY-MANUAL-ORDER.
- Catalog list/search returns nullable authenticated primary-image URL at `w=360`; no new image pipeline.

## Every shared path and why

| Path | Reason / authorization |
| --- | --- |
| `internal/merchanttools/manual.go` | Owning manual pipeline: validate discount, shared quote entry, audit/receipt and COD code |
| `internal/merchanttools/discount.go` | Owning merchant scope transaction: immutable request authorization and quote binding |
| `internal/merchanttools/order_for_buyer.go` | Shared pipeline input and canonical replay hash retain discount |
| `internal/httpapi/merchanttools.go` | Existing owning API adapter: optional strict discount body for both POSTs; no new route |
| `internal/pricing/pricing.go` | FIX3 shared grant: same pure calculator used for create/revalidation |
| `internal/pricing/manual_discount.go` | FIX3 shared grant: bounded types/validation and overflow-safe proportional allocation |
| `internal/storefront/quote.go` | FIX3 shared grant: server-only manual quote and immutable named adjustment |
| `internal/storefront/manual_discount.go` | FIX3 shared grant: read scoped authorization/bound-quote check |
| `internal/storefront/revalidate.go` | FIX3 shared grant: reproduce frozen discount at Begin |
| `internal/catalog/productlist.go` | FIX3 shared grant: primary authenticated thumbnail projection |
| `internal/merchantorders/orders.go` | FIX3 shared grant: read the same frozen adjustment via permission-fenced definer |
| `apps/admin/lib/merchant-tools-model.ts` | Existing BFF grammar and strict response parser admit only bounded discount |
| `apps/admin/lib/catalog-v2-model.ts` | Strict product parser preserves authenticated nullable 360px URL |
| `apps/admin/lib/orders-model.ts` | Strict detail parser preserves/validates named adjustment |
| `apps/storefront/lib/purchase-types.ts` | FIX3 shared apps grant: buyer order snapshot carries adjustment |
| `apps/storefront/lib/purchase.ts` | Buyer model validates same named line and discounted amount, no new request authority |
| `contracts/storefront-v2.md` | Additive G3/picker amendment: endpoints, JSON, codes, permission, replay/version/retention/Supersedes |
| `contracts/cart-quote-v1.md` | Additive money/allocation/refund/payment/settlement/report authority; no edits to older text |
| `contracts/core-openapi.json` | Assigned shared OpenAPI: discount/adjustment schemas and merchant detail field |
| `contracts/catalog-inventory-openapi.json` | Assigned shared OpenAPI: nullable product thumbnail field |
| `migrations/0216_manual_order_discount.sql` | FIX3 assigned number: additive FORCE-RLS authorization table, grants, bind/read definers; existing 0215 untouched |
| `tests/foundation/r2_integration_upgrade_test.go` | Explicit card pin exception: current base142 + one migration =143; no feature assertion edited |
| `output/ci-select-backend-browser/browser-ownership.json` | Mandatory generated source fingerprint after the assigned R2 pin, generator then --check |
| `output/feat-manual-discount/FIX3.md` | This requested scoped handoff/evidence record |

No other implementation path changed. Depmap was regenerated using `scripts/dev/depmap.sh`; final content unchanged. Packet check only changed its timestamp output, which was restored. No go.mod/go.sum/lockfile/.github/route-table-dispatcher/contractdrift-baseline changes. No new or changed route/page/browser spec, so no dispatcher pin movement. No tests or assertions changed except the expressly assigned R2 count.

## RED (E3 negative CI evidence)

Existing tests-only head `fc28de173547886f74900c5caf4ab1f7a675b451`; local `red.log` preserves the original readbacks (raw logs are not added to this implementation commit per evidence-retention policy).

| Run | Job | New assertions that failed |
| --- | --- | --- |
| Foundation 41093578253 | 124193423117 | Thumbnail missing; BFF discount grammar false; strict result parser rejected discount (501/504 Node assertions passed) |
| Gates 41093578464 | 124197225702 | Manual discount validation ignored/absent |
| Gates 41093578464 | 124197225711 | Same three Node assertions in manual-order bundle |
| Gates 41093578464 | 124197225869 | Manual order body rejected new discount, HTTP400 |
| Gates 41093578464 | 124197225752 | For-buyer shared pipeline rejected discount, HTTP400 |
| Gates 41093578464 | 124197225919 | Catalog nullable thumbnail absent |

Receipts 124214127123 and required 124214304243 failed downstream. These are the already-observed RED accepted by the integrator revision, not a new tests-only push. GREEN/WAIT_RUN: see the PR's latest implementation-push comment and integrator relay; no runtime success claimed here.

## Static verification (E1 only)

- `go build ./...`: exit 0; `go vet ./internal/merchanttools ./internal/storefront ./internal/pricing ./internal/catalog ./internal/merchantorders ./internal/httpapi ./tests/foundation`: exit 0. Raw local build/vet logs empty; no tests executed.
- `gofmt -w` on changed Go files; `node --check` on all five changed TypeScript files: exit 0.
- `bash scripts/dev/depmap.sh`, `node scripts/dev/derive-browser-ownership.mjs`, then `--check`: exit 0. Generated dependency map unchanged; ownership pin included.
- `python3 scripts/check_packet.py`: exit 0, PASS_PACKET_STRUCTURE_ONLY (not Go/PG verification). `git diff --check`: exit 0.
- Full local admin/storefront `tsc --noEmit --incremental false`: initially exit127 (no installed worktree compiler); with temporary symlinks to main-checkout dependencies exit1 due missing/stale workspace package exports (`@live-commerce/ui`, `@live-commerce/format`, i18n exports and React dependencies). No touched-file diagnostic was found in those logs. This is NOT a typecheck pass. No dependency installation, lockfile edit or product workaround; temporary symlinks removed. CI owns canonical type verification.

Runtime acceptance gate: PR #394's own CI (existing Node tests, unit, foundation REAL_PG g05/g06/g10, affected browser and required aggregation). Author self-review is E0; static checks E1; RED is E3 negative evidence only for its tested SHA. No independent approval/GREEN/E4 is claimed.

## Risks, NOT_RUN and handoff

NOT_RUN: every local test/server (owner prohibited); implementation Go/PG/browser acceptance until CI; canonical typecheck success; independent money/RLS/API review; SANDBOX/LIVE providers; deployment; UI design/click acceptance (separate parity card). No real PII, keys, external message, money movement or production mutation. Migration has not been applied locally or to production. Forward-only and no existing data/schema destruction.

No process/server/container/port/runtime fixture remains. Only failure evidence/logs are retained locally. Rollback point: committed pre-implementation head `d1bec260ab78e30eb25c6a8c6ee93a7cbddb9b09`; integrator may revert this implementation commit through normal review before deployment. Applied schema would require a separate forward fix; no production migration/revert is authorized. This worker never merges/enables auto-merge or pushes integration/main. After the single implementation push, record actual Gates/Foundation run IDs, stop within the CI wait budget, and let integrator relay conclusions into a fresh session.
