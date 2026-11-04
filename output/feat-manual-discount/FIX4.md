# FEAT-MANUAL-DISCOUNT FIX4

Scope authority: integrator's FIX4 dispatch, implementation only; accepted RED needs no repeat tests-only push. PR #394 targets `r3/integration`; branch `unit/feat-manual-discount`; worktree `/Volumes/data/live_commerce_architecture_v1/.worktrees/feat-manual-discount`. Worker: Codex implementation lane; exact model/reasoning settings are not exposed by this runtime. No delegation.

Base/reference HEAD: `6b9dea6fb60efac198dbbc9a834f22d6504dbbaa`. Fetched and merged `origin/r3/integration` at `fde11c1310fce20b576fc98023bd3003c66c205b`; merge returned "Already up to date." Base's highest migration remains 0215. Existing migration 0216 and R2 pin were not changed.

Authority read: AGENTS.md; 架构.md §§0,1,2,26,27; contracts/cart-quote-v1.md manual-discount amendment (buyer projection, server money); contracts/storefront-v2.md §G3 (fixed error codes); contracts/media-sizes-v1.md and migrations/0111_product_image_sizes.sql (stored JPEG bucket lookup and custody); migrations/0206_cvs_store_directory.sql (directory fixture); current code and accepted CI failure receipts. No contract or architecture edits.

## Failing job/test → root cause → change

Previous implementation evidence is Gates **41273609804**, head `6b9dea6fb60efac198dbbc9a834f22d6504dbbaa` (E3 negative evidence). Foundation **41273609808 / 124751222877** passed for that earlier head only.

| Failing job / test | Root cause | Product/fixture change |
| --- | --- | --- |
| g05 / 124751840657; TestMerchantToolsManualOrder/FEAT-MANUAL-DISCOUNT buyer read, manual_discount_test.go:136 | Closed buyer quote DTO discarded persisted adjustments. | internal/buyerhttp/projections.go:290 and :382 copy the frozen `[]pricing.Adjustment` just as merchant detail does. The type contains only kind/currency/name/amount_minor, no staff identity or internal reason. Empty slices still omit the field. |
| g05 / 124751840657; over-goods and shipping-only discount refusals, manual_discount_test.go:206,224 | Domain produced the correct refusal but the transport fixed map did not recognize it. | internal/httperror/error.go:209 registers `discount_exceeds_subtotal` using the map's existing English message style. Unknown-code fallback to `internal` remains unchanged. |
| g05 / 124751840657; COD cap refusal, manual_discount_test.go:373 | Same fixed-map omission for the server's final COD cap refusal. | internal/httperror/error.go:210 registers `cod_limit`; amount calculation and error assertions unchanged. |
| g10 / 124751840942; TestCatalogV2Smoke/FEAT-MANUAL-DISCOUNT_picker_thumbnail, catalog_v2_smoke_test.go:258 | Merchant image transport always selected the original PNG, ignoring `w=360`. | internal/httpapi/images.go:58 admits only exact stored buckets 360/720/1080; internal/catalog/images.go:309 reuses 0111's stored `product_image_sizes` join/fallback under tenant/store/product/image custody. No resize, decode or write occurs on GET. No-width requests retain GetImage. Missing historical buckets return original with no-store via internal/httpapi/images.go:114. |
| g05 / 124751840657; pickup happy path, manual_discount_test.go:311 | New fixture code 123456 was absent from authoritative directory 0206. | tests/foundation/manual_discount_test.go:33 now uses directory row 6477: cvs_711 / 110035 / 一勝 / 台中市大肚區福山村沙田路一段432-1號. This is a public store-directory row; buyer fixtures remain synthetic. Only this fixture line changed; every assertion is unchanged. |
| unit / 124751840781; TestGetExactDetailAndMissingOrder | Get unconditionally read manual adjustments for storefront orders, introducing an irrelevant query into the legacy detail path. | internal/merchantorders/orders.go:220 returns the existing detail after source authorization for storefront orders; manual-created orders retain read_order_adjustments and its authorization/shape/money checks. Fake and assertions unchanged. |

Accepted tests-only RED (head `fc28de173547886f74900c5caf4ab1f7a675b451`): Foundation **41093578253 / 124193423117** (three new Node assertions: thumbnail, BFF discount grammar, result parser); Gates **41093578464**, jobs **124197225702,124197225711,124197225869,124197225752,124197225919**. Existing raw/readback trail is committed as `output/feat-manual-discount/red.log`; FIX4 does not represent those as current GREEN.

## Verification and handoff

E1 only: `go build ./...` exit 0; `go vet ./internal/buyerhttp ./internal/httperror ./internal/httpapi ./internal/catalog ./internal/merchantorders ./tests/foundation` exit 0; `python3 scripts/check_packet.py` exit 0; gofmt and diff whitespace checks exit 0. Commands and source hashes: `output/feat-manual-discount/fix4-static.log`; raw static logs: fix4-build.log, fix4-vet.log, fix4-packet.log. No local tests or servers were run.

`node scripts/dev/derive-browser-ownership.mjs` regenerated the committed artifact (55 modes; only this test file's source hash changed). `bash scripts/dev/depmap.sh` regenerated dependency-map.md with no resulting diff. Existing next-startup gate-adapter assertions, exemption lists, contractdrift baseline, route-table dispatchers, contracts, migrations, lockfiles and shared schemas are unchanged.

Ownership: six source/test paths above are FIX4 plus inherited FIX3 catalog ownership. Additional generated path: output/ci-select-backend-browser/browser-ownership.json (explicitly authorized). Task reports/evidence are under output/feat-manual-discount/. No unapproved source paths changed. Preexisting FIX3.md was retained byte-for-byte at SHA-256 `6831d1987053172abf67a67deef209a83bb399407f4d2fe37818e08e563408f3` and included as requested; existing red.log was included with trailing whitespace normalized; other preexisting untracked files were left in place.

Current GREEN/E3 and independent acceptance/E4: pending this implementation push's own CI. Author acceptance is E0. Final head SHA and actual Gates/Foundation WAIT_RUN IDs will be added to PR #394 as a comment after pushing, without an evidence-only push. Integrator's fresh-session relay is the wakeup; no background polling process will be left running.

NOT_RUN: all local unit/integration/browser/model tests and servers; independent review; SANDBOX/LIVE providers; production deployment. Humaux memory/coord/canvas/code tools are unavailable in the exposed inventory, so no shared-state store/update/claim is asserted; repo report is the available handoff trail. No task processes/ports/containers/runtime fixtures were created. Risk: runtime behavior awaits PR CI; exact bucket fallback is deliberately read-only and missing sizes remain original/no-store.

STATUS: IMPLEMENTATION_READY_CI_PENDING (E1 static; author E0; GREEN/E3 and independent E4 pending)
