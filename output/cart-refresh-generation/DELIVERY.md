# cart-refresh-generation · PR #25 round 1 delivery

- Branch/worktree: `unit/cart-refresh-generation`, `.worktrees/cart-refresh-generation`; round base `ae509c18`. Tested source **a4989bc6e55c8f813acf99f95850e98213f4d264**; final commit adds evidence only. `git fetch origin && git merge origin/r3/integration` → exit0, no-op before gates and handoff; PR22 registry retained.
- Author: Codex-4; exact parent runtime model/effort not exposed. Browser test worker: GPT-6.1-sol/high, separate `codex/pr25-browser-race` worktree, three test files only plus temporary counterfactual/rollback. No recursive delegation. Task `c4bf3500-dfb8-4fe5-8ef8-c4a0e76d81ea`.
- Packet: `output/integrator/triage/pr25-ae17bc99.md`; threads **4229535990 / 4229536000**. No production Go/API/SQL/schema/dependency/copy change.

## Changes

`CartProvider` distinguishes issued requests from successful observations and tracks the pending authoritative observation. A failed later session refresh cannot make that observation ready while its cart is pending. Successful newer contexts/absent observations and command/unmount fences retain their authority. Node transport/503 cases assert ready=false before releasing the valid cart; the 14 component/client cases retain the old safety assertions.

**BC02** runs through real clicks, production Next/BFF/Go and isolated PostgreSQL: create a nonempty cart, retain its drawer line while a real focus-triggered cart GET is held, fail exactly one later session GET with the fixture edge's `session503` fault, then exercise a fresh cart-page mount with the same race. Two physical clicks through a same-origin fixture iframe emit trusted native parent blur/focus; no synthetic event, private browser override or cart-response mock is used. Both cart GET bodies remain unchanged real 200 responses.

Before releasing the held cart, the test waits for the actual native 503 fetch, actual session-lock rejection and a queued native lock grant/release. Passive observers return the original Promise and preserve arguments/callback/Response/error. DOM sampling refuses empty states and any loss or quantity change after a visible line. A fresh mount may show only its explicitly measured visible initial/detail loading states before its first line; negative controls reject blank/error/empty/incorrect-quantity/lost-line samples. PG comparison checks every scalar and full semantic Items JSON, with 11 changed-fact controls; 16 sample sequences protect the loading/continuity parser. Pure validators live in the existing DB-free evidence file; old case registry and test bodies are unchanged.

## Gates and exact evidence

Commands, exit codes, versions, source hashes and raw artifact hashes: `pr25-r1/FINAL-RESULTS.json` and its linked manifests. Raw evidence is uncompressed under `/Volumes/data/live_commerce_architecture_v1/output/cart-refresh-generation/pr25-r1/`.

| Command | Exit | Evidence |
| --- | ---: | --- |
| `node --test --experimental-strip-types apps/storefront/tests/cart-provider-retry.test.mjs` before readiness fix | 1 | 2 readiness counterexamples RED; `readiness-red.log` |
| Same focused Node command after fix | 0 | 14 PASS; `readiness-green.log` |
| `bash scripts/dev/test-node.sh` | 0 | 1189 PASS /0 FAIL; `v2-node.log` |
| `pnpm --filter @live-commerce/storefront typecheck` | 0 | `v2-typecheck.log` |
| `bash scripts/dev/test-local.sh --list` | 0 | `v2-registry-list.log` |
| `bash scripts/dev/check-gates.sh` | 0 | 82 modes documented, tracked suites assigned, headers green; `v2-check-gates.log` |
| `GOTOOLCHAIN=go1.27.2 go test -tags browser -run '^TestBuyerOrder(Evidence\|Refresh)' -count=1 -v ./tests/foundation` | 0 | DB-free evidence/fact/sample controls; `v2-go-evidence.log` |
| Same Go regex without `-tags browser` | 0 | Pure helpers compile and run in default foundation; `v2-go-evidence-default.log` |
| `LC_BROWSER_ENGINE=chromium bash scripts/dev/test-local.sh --browser-order` on readiness-reverted provider | 1 | Exact BC02 mount empty1 after real failure barrier; `v2-red-manifest.json` |
| Same readiness-reverted command with `LC_BROWSER_ENGINE=webkit` | 1 | Same precise RED, matching final three test hashes |
| Same Chromium command with complete pre-PR25 provider from `ddba31c9` | 1 | Exact BC02 RED; temporary source hash and byte-exact rollback verified, `full-provider-revert.json` |
| `LC_BROWSER_ENGINE=chromium LC_TEST_LOCK_WAIT=300 bash scripts/dev/test-local.sh --browser-order` | 0 | 33 cases, 30 real-click ledger rows, independent exact PG facts; `v2-green-chromium.json` |
| Same fixed-source order command with `LC_BROWSER_ENGINE=webkit` | 0 | 33 cases, 30 clicks, same strict facts; `v2-green-webkit.json` |
| `LC_BROWSER_ENGINE=chromium LC_TEST_LOCK_WAIT=300 bash scripts/dev/test-local.sh --browser-buyer` | 0 | 13 cases, one order/hold; `v2-green-buyer.json` |

Both order GREENs preserve the frozen 32 cases plus BC02 and 21 old plus nine new clicks. For BC02, drawer/mount empty=0 and line_loss=0; the cart/receipt/version/write/quantity remain exactly1 before, after and after reload, verified by Go's independent PG read. RED calibrations use the same final three test hashes; the complete-provider control also removes the earlier observation-generation fix. All owned heavy runs were serial, normal fixtures closed and the build lock released. Packaged `check-gates.sh` and staged `git diff --check` also exit0.

**E3: tested MOCK buyer + real browser/Next/Go/isolated REAL_PG environment**, bound to the source hashes above. Read-only independent source/evidence audit PASS (`69d9e7b7-7c81-4385-976e-6be83c6ea6a3`), including raw file hashes and RED/GREEN facts; no independent runtime rerun claimed. Earlier pre-barrier/JSON-format/detail-loading diagnostics and previous generic browser evidence are retained as historical, not final acceptance. Canonical raw logs are exact; committed review copies trim trailing whitespace only, with hashes retained.

## CI gates / NOT_RUN

`node scripts/dev/pr-modes.mjs origin/r3/integration HEAD` selects **53** modes (no selection removed). Order and buyer modes ran locally; the other51 modes, full required GitHub set, independent runtime rerun and K3 review on this handoff are **NOT_RUN** locally. The complete `pr-modes-final.json` list is the CI request. No production/platform/live acceptance or deployment claimed. No unresolved scoped P0/P1 identified. Integrator runs K3 and pushes this batch; author commits and stops, never pushes.
