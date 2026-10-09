<!-- Purpose: PR16 round-three click coverage, causal runtime proof and preserved-output handoff.
Depends on: comments 4226523807/4226523811, existing browser-order/product-editor gates and real Go/PG fixtures.
Used by: integrator review and PR16 acceptance; not production/provider certification. -->
# PR #16 round 3

- Branch `unit/pr1-ui-followups`, base `989a34e911081582bd0022fadb3dfac69fd69989`; fetch + merge of `origin/unit/pr1-ui-followups` was already up to date. Current trunk was already an ancestor.
- Source commits: `fd64fa4f`, `1c0da6c1`, `16878ecd`, `be04b982`. This delivery commit adds evidence only. No push; Codex-3 retains PR ownership for subsequent packets.
- Scope: five test files only. No app/runtime, migration, dependency, contract or timeout changes. Foundation's Go-vulnerability failure was left to PR #19 as instructed.
- Author/roles: Codex-3 integrates and independently runs gates (parent GPT-6 runtime family; exact deployment/effort not exposed). Two isolated test authors and read-only reviewer configured `gpt-6.1-sol/high`; cart owns order driver/Go evidence validator, matrix owns product-editor driver/Go fixture. Their commits are incorporated; no recursive delegation.

## 4226523807 — real visible Cart Retry

The old gate only clicked checkout's recovery control. Six new cases actually click **CartNotice Retry** on the product drawer and full cart page, in en/zh-CN/zh-TW. The synthetic edge loses the response only after the real Next/BFF/Go/PG cart write commits; it also drops implicit transport resends until the buyer clicks Retry.

Each case proves the frozen journal's key/body match every transmission, exactly one explicit retry PUT, one request-specific `cart.set` receipt, and no additional `cart.updated` event or version advance. A reload only reads and retains the committed cart. The authenticated loopback observer binds tenant/store/cart/owner/session/key; Go independently re-reads the same receipts and lines after Node exits. Existing 26 observations and seven orders remain required; total **32 observations**, **six cart recoveries**, **21 actual-click ledger rows**.

Evidence: `evidence/cart-green/{cart-click-ledger,cart-retry-facts,cart-final-pg-facts,result}.json`. No source scan substitutes for a click.

## 4226523811 — operate the matrix at 390px

Three locales now create axes through real controls, operate row selection, tracking mode, quantity/maximum, price, compare-at price, SKU code, keyword and enabled controls, then create/reopen and edit/save/reload. Every per-row label must be visible and scroll into the viewport; every saved value is read back. Existing SKU codes are correctly verified disabled.

Enabled=false archives a SKU rather than storing a reversible Boolean. The extra archive action saves White's removal and reloads exactly one visible Black row with its original ID/values. PG independently requires two stored SKUs (White archived, Black active), exact prices/stock/code/keyword facts, original four products unchanged plus exactly three new locale products. **135 mobile ledger rows and nine screenshots**: `evidence/matrix-green/`.

## Causal REDs and final gates

| Command / condition | Exit | Evidence |
|---|---:|---|
| `bash scripts/dev/test-local.sh --browser-order`, Retry handler temporarily no-op | 1 expected | `cart-retry-red.log`; `evidence/cart-red/cart-click-ledger.json`: actual visible Retry click fails to clear uncertainty after committed write |
| `bash scripts/dev/test-local.sh --browser-product-editor`, mobile labels temporarily hidden | 1 expected | `matrix-labels-red.log`; `evidence/matrix-red/playwright.log`: new zh-TW White/S price-label visibility assertion fails; frozen CC12 passes |
| First unmutated product-editor run | 1 | `product-editor-green.log`; `evidence/matrix-green-first-failure/`: test incorrectly expected a blank placeholder after archive. Corrected to contract's strict active-only one-row result; PG archive checks retained |
| `bash scripts/dev/test-local.sh --browser-order` | 0 | `cart-order-green.log`: 32 observations, 7 orders, exact independent PG checks |
| `bash scripts/dev/test-local.sh --browser-product-editor` | 0 | `product-editor-green-final.log`: PE12–17/review tests plus frozen CC12, exact original4 + new3 product/SKU facts |
| `bash scripts/dev/test-node.sh` | 0 | `node-final.log` |
| `pnpm --dir apps/admin exec tsc --noEmit -p .` | 0 | `admin-tsc-final.log` |
| `bash scripts/dev/check-gates.sh` | 0 | `gates-final.log` |
| `GOFLAGS=-p=1 go vet -tags browser ./tests/foundation` | 0 | `browser-vet.log` |
| `GOTOOLCHAIN=go1.27.1 go test -tags browser -run '^TestBuyerOrderEvidenceCompleteness$' -count=1 ./tests/foundation` | 0 | `focused-evidence-registry.log` |

Each runtime command has a bounded runner and adjacent `.status.json` with HEAD/source hash, exit and unchanged-source check. Browser modes ran serially. Cart green ran at `16878ecd`; subsequent changes only corrected the product-editor test's archive readback. `scope-and-cleanup.json` proves all three cart test files remain byte-identical; all apps/runtime remain unchanged. Final product-editor/Node/typecheck/gates ran at `be04b982` with unchanged source. Mutations were restored byte-for-byte before normal gates (`mutations/*.json`); none is committed as product behavior.

## Preservation and acceptance boundary

- `git checkout -- output` restored every overwritten tracked evidence file. All **290** pre-existing untracked files were restored and SHA-256 checked; none is staged. Nine task-created matrix screenshots were copied into this delivery and removed from the old output directory. See `preservation.json`.
- Raw traces/backups remain at `/Volumes/data/live_commerce_architecture_v1/output/pr16-r3-codex3/raw`; this committed evidence package is intentionally small. Owned temporary author worktrees were archived and removed; the requested delivery worktree and pre-existing evidence remain.
- **E3: real Chromium, production Next/BFF/Go, isolated PostgreSQL; MOCK identity/network fault only.** Read-only source review findings were addressed; independent K3/CI acceptance remains with the integrator. No real provider, production or deployment claim.
- NOT_RUN: full foundation/vulnerability scan, full CI and WebKit. Shared order script also runs in WebKit CI; only the two requested local modes were run. No G07/runtime/schema change was introduced.
