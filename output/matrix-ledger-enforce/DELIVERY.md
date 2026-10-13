# matrix-ledger-enforce — historical delivery and current live guard

- Branch: `unit/matrix-ledger-enforce`; previous remote head `875ab57fb0619a845765b0754dd3bae85ad73efa`.
- `git fetch origin && git merge origin/r3/integration` → **0**, merged trunk `5a54d41b7e86a72eeec2afbe1f7b0458daaf742b` as `54eb4cec43d7ebdba8c2e094a7b072ae54d13897`; [merge receipt](historical-evidence/merge.json). This includes #43's legitimate product-editor acceptance changes.
- Tested source: **`2e10084b3f5e96548c15953cc40488aef9e1c86d`**, [SOURCE.json](historical-evidence/SOURCE.json). Fourteen unit executable hashes bind this delivery; shared context and CI-selector source hashes are informational. The final evidence commit changes no frozen executable source.
- Author: Codex-4 `/root`; model/effort not exposed. Read-only explorer `/root/matrix_drift_ci_select`, `gpt-6.1-sol`, medium; E1 [source review](historical-evidence/source-review.json) and [canonical artifact review](historical-evidence/artifact-review.json), no blocker and no recursive delegation. Humaux task `bfd7631f-2768-4c78-8a13-06de60be22d1`.

## Result

Canonical evidence is a historical delivery record. Actual bound-source hash drift reports `canonical evidence is historical (source drift: <files>)` through `t.Skip`. Setting `LC_MATRIX_EVIDENCE_STRICT=1` makes drift fail when verifying a delivery. Missing/malformed receipts, invalid digests and unsafe/read-failed sources remain failures. When source hashes match, exact artifact SHA-256, native result status, file/title/count/no-skip outcomes, ledger identities and expected claims remain mandatory.

The existing live `--browser-product-editor` remains the current-source trunk guard: its Go harness validates the fresh ledger produced by that same Playwright run. CI-SELECT reads the same named producer table as the Go evidence validator through its base/head/merge-checkout source reader and monotonically appends that existing mode. All 14 bound sources select it through the real CLI `GITHUB_OUTPUT` and actual `ci-plan` job matrix; missing historical declarations are allowed, malformed present declarations fail closed. No new mode, browser wait/assertion, control coverage, production endpoint, dependency or migration change.

## Red → green

- [Actual Go policy RED](historical-evidence/policy-red.json): `go test ./tests/foundation -run '^TestProductEditorMatrixLedgerEvidenceHistoricalPolicy$' -count=1 -v` → **1** against the old entry point. The real child incorrectly FAILed default drift; it also lacked the explicit historical reason.
- [Policy/old corruption GREEN](historical-evidence/policy-green.json): `go test ./tests/foundation -run '^TestProductEditorMatrixLedger' -count=1 -v` → **0**. Isolated real test-binary subprocesses prove matching strict PASS; default drift exit 0 with explicit SKIP; strict drift exit 1 with FAIL; matching sources plus tampered log exit 1 with SHA-256 FAIL. Expected child failures are negative controls, not failed parent gates. The old canonical is explicitly SKIP at this pre-refresh step.
- [CI-SELECT RED](historical-evidence/ci-select-red.json): focused `node --test --test-name-pattern='matrix evidence producers|matrix evidence producer discovery|real matrix producer CLI' tests/ci/pr-modes.test.mjs` → **1**, then [all selector tests GREEN](historical-evidence/ci-select-green.json): `node --test tests/ci/pr-modes.test.mjs` → **0**, 61 cases. Existing legacy-monotonicity tests remain green; all 14 producer paths reach the real CI matrix, and base/head/checkout additions/removals are unioned. Focused test-name filtering is used only in this diagnostic red run; required suite commands remain unfiltered.

## Actual final gates

| Exact command | Exit | Evidence |
|---|---:|---|
| `bash scripts/dev/test-local.sh --browser-product-editor` | 0 | [full original browser receipt](historical-evidence/browser-product-editor.json) |
| `bash scripts/dev/test-node.sh` | 0 | [full Node receipt](historical-evidence/test-node.json) |
| `bash scripts/dev/check-gates.sh` | 0 | [exact retry receipt](historical-evidence/check-gates-retry.json) |
| `LC_MATRIX_EVIDENCE_STRICT=1 go test ./tests/foundation -run '^TestProductEditorMatrixLedger' -count=1 -v` | 0 | [strict canonical/corruption receipt](historical-evidence/strict-ledger.json); [final packaged entry-point verification](historical-evidence/strict-final-package.json), also exit 0 |

Native full browser: **12 PE/review/feedback + 2 CC12, zero skips**, both last-run receipts `passed`, empty `failedTests`. Fresh ledger: **331 raw rows, 267 unique required PASS identities, 89 per locale** at 390px. The required identity/expected/status projection matches the unchanged independent golden fixture. [Manifest](historical-evidence/browser/browser-evidence.json), [artifact digest index](historical-evidence/ARTIFACTS.json), [ledger](historical-evidence/browser/product-editor-click-ledger.json), [PE log](historical-evidence/browser/playwright.log), [CC12 log](historical-evidence/browser/cc12-playwright.log). Native copies are byte-identical, with original relative evidence paths retained. Source hashes stayed unchanged before/after the full run. Chromium + isolated MOCK identity/REAL_PG; no provider/live/customer/production actions.

The first unchanged full check-gates attempt exited **1** after 419.415s: matrix execution binding passed, then `TestReviewR1bImportedSinkValuesRealCLI/wrapper` hit its existing CLI context deadline. [Original failure](historical-evidence/check-gates.json) and [uncompressed log](historical-evidence/check-gates.log) are retained. The exact source-identical retry exited **0** after 142.685s (83 modes documented, every tracked test run); no timeout/assertion/source changes, no foreign process or lock action.

## Historical evidence and limits

[Previous DELIVERY](historical-evidence/previous-delivery.source.txt) and [filter-bindings evidence](filter-bindings/SOURCE.json) remain unchanged historical records at `e0fb76ce`; their prior gate exits are not current-source acceptance. Their mismatch after #43 and this gate change is the intended historical/default-SKIP case, not a failed native run.

- Required new CI/final independent review: **NOT_RUN locally**. E1 source review is not K3/CI approval. No PR merge or thread resolution.
- No TypeScript edited by this fix; fresh browser builds/typechecks the merged app. No separate `tsc` claim this round.
- Remaining control coverage is #45; additional flag spellings/per-option bulk/control types are deferred and untouched.
- Bounded owned runner records command, timeout, source before/after and exit; shared browser-build/PG custody respected. Only own temporary Git test fixtures are cleaned. Foreign `output/ext-agents/` remains excluded.
