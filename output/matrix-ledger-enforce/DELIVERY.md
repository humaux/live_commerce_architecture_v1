# matrix-ledger-enforce delivery — template fragments and evidence integrity

- Branch: `unit/matrix-ledger-enforce`; pulled `da911f4251434460b62cca81eb52c5652072a87f` (exit 0).
- Tested source: **`e15311a729fe085a0c19323ad3332cbef295991c`**, with 18 Git/checkout hashes in [SOURCE.json](template-fragments/SOURCE.json). The final evidence commit changes only documentation and receipts.
- Author: Codex-4 `/root`, GPT-6 family; exact parent runtime variant/effort is not exposed. Read-only peer: `gpt-6.1-sol`, high, subcanvas `eaf4cbfd-sub-template`.
- Owned scope: the template signature helper, its real-source regression and the existing Go evidence gate plus unit evidence. Foreign `output/ext-agents/` is excluded. No application, API, SQL or public contract change; browser fixtures, waits and assertions are unchanged.

## Result and review replies

TemplateHead, TemplateMiddle and TemplateTail now retain their literal text as well as their kind. The exact savedBlack fallback-selector mutation can no longer erase the Black-SKU identity constraint. Existing plain-string/no-substitution equivalence and wrapper normalization remain.

The Go gate follows the canonical directory DELIVERY selects. It verifies every file in that browser directory against `ARTIFACTS.json`, including both Playwright logs, the ledger, manifest and last-run receipts. All five native copies must also match `copied_native_bytes`. The manifest is hashed externally, avoiding a self-hash cycle. SOURCE, index and manifest must bind the same tested SHA and identical source maps. Every source digest is checked against safe, actual checkout bytes; all 18 tested producers, including the evidence gate itself, are mandatory. Removing hash entries cannot exempt required files. Existing exact identity, PASS, expected-field and duplicate checks remain.

| Thread | Fix SHA | One-line reply for integrator |
| --- | --- | --- |
| `4249311520` | `e15311a7` | Template signatures preserve Head/Middle/Tail text; the exact savedBlack fallback-selector mutation is RED and the original source is GREEN. |
| `4249391831` | `e15311a7` | The manifest's full source map is verified against actual checkout bytes, including this gate; changed spec or gate source is RED, with fresh browser evidence on the final source. |
| `4249391853` | `e15311a7` | Every canonical artifact is SHA-256 checked against the external index and native-copy record; tampered logs or a manifest edit retaining its commit are RED. |

## Red → green

[red-tests.json](template-fragments/red-tests.json) binds exact commands, source/snapshot hashes, logs and exit codes. The fallback regression first failed on the old helper (exit 1). Final-source fallback injection exits 1; restoring the original source exits 0. Focused Node is **52 PASS / 0 FAIL / 0 SKIP**.

The Go validator was extracted without changing its original checks, then six isolated checkout corruptions were added. All six initially failed the required rejection (exit 1). After the fix all six pass (exit 0), rejecting arbitrary PE and CC12 logs, edited ledger bytes, a manifest edit retaining the source commit, changed acceptance source and changed gate source. The test never mutates the delivered artifacts in place. [RED](template-fragments/evidence-red.log), [GREEN](template-fragments/evidence-green.log); forensic snapshots end in `.source.txt` and are not runnable test filenames.

[Scoped audit](template-fragments/AUDIT.md): peer actual-source JS probes are E3, Go control-flow review is E1. No new K3, Claude or CI approval is claimed.

## Actual gates on frozen e15311a7

[Local receipts](template-fragments/local-gates.json), [browser receipt](template-fragments/browser-run.json), [Go/vet receipts](template-fragments/final-gates.json) and [check-gates receipt](template-fragments/check-gates.json) record bounded runner PIDs, commands, timestamps, exits and log hashes. All 18 source hashes remain unchanged.

| Command | Exit | Evidence |
| --- | --- | --- |
| `bash scripts/dev/test-node.sh` | 0 | [test-node.log](template-fragments/test-node.log), 133.958 s; includes 52 matrix tests |
| `pnpm typecheck:admin` | 0 | [typecheck.log](template-fragments/typecheck.log), 2.098 s |
| `go test ./tests/foundation -run '^TestProductEditorMatrixLedger' -count=1 -v` | 0 | [go-ledger.log](template-fragments/go-ledger.log), 5.339 s; current canonical evidence and six corruption controls |
| `go vet -tags browser ./tests/foundation` | 0 | [go-vet.log](template-fragments/go-vet.log), 1.992 s; empty success output |
| `bash scripts/dev/check-gates.sh` | 0 | [check-gates.log](template-fragments/check-gates.log), 216.651 s; all 82 modes documented, all tracked tests run, contract drift errors=0 |
| `bash scripts/dev/test-local.sh --browser-product-editor` | 0 | [browser-product-editor.log](template-fragments/browser-product-editor.log), 120.285 s; independent original complete run |

Browser evidence uses `output/playwright/pr28-integrity-repeat-20261012T151736Z`, Chromium and the bounded shared-PG lock. No grep, calibration, added test retry or relaxed wait. **12 PE/review cases + 2 mandatory CC12 cases passed, zero skips.** Go/isolated PG verifies **231 required unique PASS identities**, 77 each in zh-TW, zh-CN and en at 390px; raw ledger has 286 rows.

Canonical native bytes: [ledger](template-fragments/browser/product-editor-click-ledger.json), [PE/review log](template-fragments/browser/playwright.log), [CC12 log](template-fragments/browser/cc12-playwright.log), last-run receipts and [manifest](template-fragments/browser/browser-evidence.json). No screenshot delivery claim. Copies are hashed without reformatting native bytes.

## Historical failures and limits

The first full browser attempt on the same frozen source exited **1**: PE readiness navigation's original focus-in-pane assertion at line 170 failed in zh-CN/390px; 11 remaining PE cases did not run, while CC12 passed. [Attempt receipt](template-fragments/browser-attempt1.json), [wrapper log](template-fragments/browser-attempt1.log) and [native failure](template-fragments/failed-browser/playwright.log) are retained and are **not acceptance**. Trace shows the image-checklist click before the warehouse-reference response; the existing product layout hook attempts focus while the upload input is disabled and does not replay it after enablement. This application hook was not changed in this batch. The second independent original run passed; this does not claim the existing timing issue is fixed or establish flake-free repetition.

Earlier `round1`, `round2`, `round3`, `round3-canonical` and root `browser/` artifacts remain historical at their recorded SHA. Round1 predates the #23 path merge. Preserved fixes include `bcc24338` receiver/effect binding, `ec94cb05` native-dialog postcondition, `05c2af77` canonical expected claims and `da911f42` DELIVERY-selected evidence. All 77 original identity/expected declarations remain unchanged.

Evidence: **E3 in the automated tested environment — BROWSER + isolated REAL_PG, synthetic MOCK identity/provider.** New-head CI/external approval, full foundation, provider SANDBOX/LIVE and deployment are **NOT_RUN**. CI gates: foundation-shards and the affected browser planner set including `--browser-product-editor`. No production, live keys, real money/messages or real buyer PII.

Normal branch push is authorized for this batch after gates. Thread replies/resolution and merging remain with the integrator. No thread was resolved; no merge, rebase, deploy or foreign cleanup was performed. Own browser fixtures exited and the shared browser-build lock was released.
