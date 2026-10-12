# matrix-ledger-enforce — PR28 round-5

- Branch: `unit/matrix-ledger-enforce`; starting remote head `40bad95377741ebcb76b371e9d42f03bce1df581`.
- `git fetch origin && git merge origin/unit/matrix-ledger-enforce && git merge origin/r3/integration` → exit 0; trunk union merge `760e354e8d466e2e5876d1fb5d433d74e3d10d2e`, merged trunk `fef7920378c0b8cde843d22b27d33c62b07451b7`.
- Tested source: **`d93700a4ab0fb541aa8d3ee2d540c0d94b29f50f`**, [SOURCE.json](round5/SOURCE.json). Fourteen unit executable producers (including the modified bulk widget/test selectors) are checkout-bound; twelve unchanged shared context files remain informational. Harness execution is independently content-bound. Golden ledger fixture projects only the 267 mobile rows unchanged from the final real native run, not synthesized; evidence/fixture handoff does not change frozen executable bytes.
- Author: Codex-4 `/root`; parent runtime/effort not exposed. Go worker `test_worker`, `gpt-6.1-sol`, medium, isolated `.worktrees/pr28-r5-run-identity` at `760e354e`; owned only the Go outcome module and [worker proof](round5-run-identity/NOTES.md). Independent scoped reviewer `explorer`, same model/medium, read-only; Humaux `d6cfc5d0-932a-4f75-ad6d-d88f0d849a6e`. No recursive delegation.
- Contract/interface changes: none. Required CI gates: foundation-shards and original `--browser-product-editor`; new CI results pending integrator.
- Native logs/worker patch are retained unchanged, including tool-emitted whitespace. `git diff --cached --check` for source/docs/fixture passes (exit 0); the unrestricted receipt diff reports native whitespace (exit 2) and is not used to rewrite evidence.
- Foreign `output/ext-agents/` is excluded. Application changes are stable test IDs only; original waits/assertions remain, with stronger bulk operation/readback assertions.

## This round: red → green

**4251070834 (P1):** table-only completeness omitted seven actual bulk controls. The JSX scanner derives controls from production select/input/button nodes; the browser enumerates the mounted DOM and binds its call to the real imported recorder method. Removing a rendered control's table row is RED even when the reduced table/ledger agree. Each locale at 390px now actually selects a SKU, opens, fills/cancels/reopens, selects field/target/scope/operation, fills/apply, saves and reloads. Cancel compares every matrix input/select value, checked and disabled state. Apply adds 2 only to White (3→5), leaves Black at 4; exactly one document write, original SKU identities/prices/compare-at/codes and quantities persist after reload and REAL_PG archive readback.

[Initial controls RED](round5/bulk-controls-red.log): exit 1, all seven missing controls reproduced. On final gate, [missing Apply RED](round5/missing-control-red.log) and [swapped/no-op recorder RED](round5/noop-inventory-red.log): exit 1 each; [controls GREEN](round5/bulk-controls-green.log): 9/9, exit 0. [Proof/hash record](round5/bulk-proof.json). Canonical action/outcome/expected-claim gates and identity checks are preserved.

**4251070846 (P2/test integrity):** counts alone admitted a different suite saying 12/2 passed. Go now requires the exact consumer files and complete titles, once each, for all PE/review/feedback and CC12 cases; line numbers/durations may change. [Worker native RED](round5-run-identity/red.receipt.json): exit 1, 12 consistently rehashed wrong-file/title/substitute/drop/duplicate/fake-run corruptions accepted by the old gate. [Worker GREEN](round5-run-identity/green.receipt.json): exit 0 (prior 33 controls + 12 identity controls + metadata positive). This is isolated-fixture evidence; final current-source canonical gate follows below.

The first full run at `d10e499d` had 12+2 Playwright passes and a valid 267-row ledger, but the Go archive expectation still said White=3. Its [actual composite RED](round5/browser-initial-archive-red.json) is exit 1 and retained uncompressed. `d93700a4` changes that exact expected quantity to the saved bulk result 5, retaining Black=4 and every other archive/replacement field and condition. The original full mode then exits 0; this is an updated exact assertion for the added flow, not a relaxed assertion.

## Current actual gates

The original full `--browser-product-editor` mode passes 12 PE/review/feedback + 2 frozen CC12, zero skips. Both native last-run receipts are passed with empty failedTests. Ledger: 322 raw rows, 267 unique required PASS identities, 89 per locale (`zh-TW`, `zh-CN`, `en`) at 390px. [Canonical manifest](round5/browser/browser-evidence.json); five native files copied byte-for-byte, with [external artifact hashes](round5/ARTIFACTS.json). No generated runnable test/spec snapshots are committed.

| Exact command | Exit | Evidence |
|---|---:|---|
| `bash scripts/dev/test-local.sh --browser-product-editor` | 0 | [native/full receipt](round5/browser-product-editor.json) |
| `bash scripts/dev/test-node.sh` | 0 | [final source receipt](round5/test-node-final.json) |
| `bash scripts/dev/check-gates.sh` | 0 | [receipt](round5/check-gates.json), all 83 modes documented/run and contract drift errors=0 |
| `pnpm typecheck:admin` | 0 | [receipt](round5/typecheck-final.json) |
| `go vet -tags browser ./tests/foundation` | 0 | [receipt](round5/go-vet.json), identical final file hashes before source commit |
| `node --test tests/ci/product-editor-matrix-controls.test.mjs` | 0 | [9 tests](round5/bulk-controls-green.log) |
| `go test ./tests/foundation -run '^TestProductEditorMatrixLedger' -count=1 -v` | 0 | [current canonical and all corruption controls](round5/go-ledger.json) |

| `bash scripts/dev/check-gates.sh` (after actual execution-binding injections/restoration) | 0 | [final restored receipt](round5/check-gates-restored.json), errors=0 and all modes documented |
| `go test ./tests/foundation -run '^TestProductEditorMatrixLedger' -count=1 -v` (final staged artifact set) | 0 | [final receipt](round5/go-ledger-final.json) |

## Retained round-4 fixes

P1-a still requires both native receipts passed, no failed tests, exact log outcomes and manifest/ledger agreement. P1-b still independently requires active Node argv and balanced foundation CI selector forwarding (now also the new controls suite). Prefix/postfix operator signatures and refusal of both mutation hooks whenever CI is present (even empty) are retained. [Historical round-4 proof](round4/PROOF.json), [outcome red](round4/outcome-red.json), [Node removal red](round4/binding-after-red.json), [Go skip red](round4/go-binding-after-red.json), [unary/CI-hook red-green](round4/js-proof/green.log). These historical runs bind their own source; round5 alone is the designated current canonical evidence.

The current-source P1-b injection actually removes the ledger suite from Node argv, then separately excludes the Go evidence test in the CI dispatch: [Node RED](round5/node-binding-final-red.json) and [Go CI RED](round5/go-ci-binding-final-red.json) each exit 1. Both receipts prove byte-for-byte restoration. [Restored final check-gates](round5/check-gates-restored.json) and [final canonical Go](round5/go-ledger-final.json) both exit 0 on the final staged artifact/fixture set.

Independent scoped artifact acceptance [audit](round5/audit/canonical-audit.json) passes 128 assertions plus an [independent actual Go run](round5/audit/go-ledger-independent.receipt.json), exit 0. Its 59-artifact snapshot was audited before this final metadata/receipt expansion; it is local E3 artifact proof, not new full browser, CI or K3 approval.

| Review item | Integrator reply text |
|---|---|
| 4251070834 | All seven rendered bulk controls now receive real select/fill/click actions, Cancel is unchanged-state checked, Apply persists White=5/Black=4 through save/reload and REAL_PG; actual JSX/DOM completeness rejects a missing Apply row. |
| 4251070846 | Canonical logs require the exact 12 PE/review/feedback and two CC12 file/title identities once each; consistently rehashed wrong-file/title and same-count substitutions are rejected. |
| R4 P1-a | Both last-run receipts and logs require passed outcomes with no failures/skips, and manifest counts must match the real ledger; all rehashed outcome corruptions are rejected. |
| R4 P1-b | Removing active Node argv or excluding the Go test from the actual CI selector turns check-gates red; byte-exact restoration is green. |
| R4 cheap P2s | Unary operators remain signature-bound and CI presence refuses either mutation hook; full current Node suite is green. |

## Limits

- MOCK identity + isolated REAL_PG/browser only; no real provider, production, deployment, PR merge or thread resolution.
- New CI/K3/external approval: NOT_RUN; focused read-only source review is not a new full independent approval.
- Optional media r04 runner remains NOT_RUN because its pinned binary is unset; required Node suites pass.
- One initial artifact preparation attempt used the wrong fixture projection and DELIVERY header; [receipt](round5/canonical-preparation-error.json) is exit 1, retained as a preparation error, not red-test acceptance. After restoring the original projection/header contract, the same complete Go gate exits 0.
- The initial full composite failure remains honestly recorded; its passing Playwright receipts are not designated final complete evidence.
- Accepted FOLLOWUPS (source-list/path expansion, self-attesting receipts, unrelated merge-ref context drift) remain unchanged. Execution content checks support the current bounded runner shapes, not arbitrary shell control flow.

Owned isolated worker worktree was removed after byte-exact proof/patch comparison (exit 0); native test processes completed and the browser-build resource was released. Parent checkout retains all RED/GREEN evidence.
