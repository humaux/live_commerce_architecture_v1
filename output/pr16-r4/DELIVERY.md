<!-- Purpose: PR16 round-four real mobile axis-removal proof and handoff.
Depends on: review comment 4226876570, product-editor browser mode and isolated PostgreSQL fixture.
Used by: integrator independent review and CI; no production/provider claim. -->
# PR #16 round 4

- Branch `unit/pr1-ui-followups`; base `dc15cf15579ef589513624ae57e06610f3c09faf`; test addition `aa73e2f713b5054b84cadc29d2bb217b45994389`; final tested source `d3d65c21767dabe65a6068cfe0fc6abaf254e52d`. Final delivery commit adds evidence only.
- Scope: mobile browser/Go evidence plus the real product serializer/UI defect exposed by those clicks. `86f30a00` fixes replacement SKU edits; `7e1ac716` simplifies draft initialization to retain the existing line budget. CartProvider, contract, dependency, timeout and threshold unchanged.
- Roles: Codex-3 integrates and runs all runtime gates (parent runtime exact deployment/effort not exposed). Isolated test author and read-only reviewer configured `gpt-6.1-sol/high`; author commit `51bd7f0d` incorporated as `aa73e2f7`. Reviewer found no P1/P2 in the test diff; independent integrator acceptance remains pending.

## Finding and proof

The previous mobile flow operated variant rows but never clicked the axis-removal control. Each existing 390px locale flow now creates a separate persisted Color[White, Black] × Size[S] product, clicks the visible `.pe-axis-remove` control, and requires exactly one remaining Size[S] axis/row. It fills that replacement row, saves through the actual archive confirmation, reloads, and verifies the removed SKU IDs are absent while the new ID and all values persist. Each phase allows exactly one document write. The original R3 assertions remain intact.

Go independently checks exactly ten fixture products (original four, R3 three, R4 three), exact persisted Size[S] options, both old SKUs archived with original values intact, and one active replacement SKU with price 9500, compare-at 14500, tracked stock 9 and server-generated code exactly equal to the product slug. The UI result follows the active-only contract; archived rows remain in PostgreSQL.

Causal RED: temporarily replace the real remove `onClick` with a no-op. After the actual mobile click, the new assertion expects one axis but receives two (`product-editor.acceptance.ts:862`). Frozen CC12 also correctly fails its axis-removal assertion. Exit 1; logs/screenshots in `axis-remove-red.log` and `evidence/red/`. Restore the callback byte-for-byte before normal gates; hashes in `mutations/restored.json`.

## Runtime defect exposed by the new flow

The first normal browser run failed at Save with HTTP 400 `invalid_json`: `editDocument` copied create-only `code` into a new SKU inside the strict edit grammar. `DocumentSKUPatch` has no code field (§g); replacement codes are server-generated. The existing model test incorrectly expected an illegal code. Added a real-serializer negative first (`patch-code-red.log`), then omitted code from edit serialization and disabled edit-mode single/matrix code controls plus the bulk code choice. Create still validates code; edit ignores stale draft code. Exact expected shipping, stock and archive payload assertions remain. Corrected the browser/PG expectation to exact server-generated code, retained through two reloads; every axis/archive/price/stock/count assertion remains.

Evidence: `evidence/strict-edit-red/` and `patch-code-{red,green}.log`. First normal browser exit 1, frozen CC12 passed. Initial post-fix Node/gates caught the form line budget after adding a prop; simplified the existing two-return draft initializer. The waiting browser runner was stopped before any PG/browser test (exit 143). Both failures remain recorded, not counted as PASS.

A subsequent run hit a pre-existing readiness race before the new mobile flow: Playwright set the matrix upload file while the actual input was still disabled. Trace call `call@7780` records the disabled input, and no upload request followed. Added a positive `toBeEnabled()` before the existing `setInputFiles`, retaining the photo-count/publish/reload assertions and all timeouts. Evidence: `evidence/upload-readiness-red/`; source `d3d65c21`.

## Local gates

| Command | Exit | Evidence |
|---|---:|---|
| `bash scripts/dev/test-local.sh --browser-product-editor` (remove callback no-op) | 1 expected | `axis-remove-red.log` |
| `bash scripts/dev/test-local.sh --browser-product-editor` (normal source) | 0 | `browser-product-editor-green-final.log` |
| `bash scripts/dev/test-node.sh` | 0 | `node-green-final.log` |
| `pnpm --dir apps/admin exec tsc --noEmit -p .` | 0 | `admin-tsc-green-final.log` |
| `bash scripts/dev/check-gates.sh` | 0 | `gates-green-final.log` |
| `GOFLAGS=-p=1 go vet -tags browser ./tests/foundation` | 0 | `browser-vet-green-final.log` |

Commands have bounded runners and adjacent status JSON recording HEAD, source SHA-256, exit, elapsed time and unchanged-source verification. Browser gates run serially. The normal browser run waited for another owner's machine-wide PG lock; no lock bypass or timeout change.

## Preservation and acceptance boundary

- Final `TestBrowserCatalogCore` PASS: PE12–17 plus all review regressions, followed by frozen CC12. Exact ten-product and old/replacement SKU PostgreSQL assertions passed.
- `evidence/green/axis-remove-ledger.json`: six R4 ledger rows (remove/save × three locales); full ledger has 196 rows. Three `mobile-axis-remove-*-390-saved.png` screenshots show the reloaded single S row at 390px. Screenshots inspected; rows scroll within the matrix as designed.
- All tracked historical output was restored. All 290 pre-existing untracked files were restored byte-for-byte and the untracked set checked exactly; none is staged. Twelve task-generated screenshots were archived before removal from the historical directory. `preservation.json` records backup paths.
- Committed text evidence normalizes trailing whitespace only; byte-original logs retained in `output/pr16-r4-codex3/raw-delivery-before-whitespace-normalization.tar.gz` at the main checkout.
- Owned temporary author worktree was clean and removed after incorporation; its branch/commit remains. Evidence-unit worker worktrees are retained for the explicitly paused unit. No task processes remain running.

- **E3 at d3d65c21**: all five requested local gates exited 0 with unchanged source. Independent K3/CI acceptance remains with the integrator. Evidence class: real Chromium/Next/BFF/Go and isolated PostgreSQL, MOCK identity only; not production/provider certification.
- CI gates: `--browser-product-editor` plus PR-required checks; integrator pushes and runs independent review/CI.
- NOT_RUN: full foundation, unrelated browser modes, deployment. No push or GitHub thread reply.
- `browser-evidence-untracked` remains owner-paused at `6b62d3b5d5272ea44cdb164b68a4abde2de891bc`; its full acceptance is pending. LC-U3 remains parked.
