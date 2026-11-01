# FEAT-MANUAL-DISCOUNT FIX3 — implementation submitted for CI

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
