# matrix-ledger-enforce delivery — PR merge-ref source scope

- Branch: `unit/matrix-ledger-enforce`; starting head `3dac7467e6bbbf7a9c0e4466c4c030b0969083ad`.
- Fetch/merge exit 0: `origin/r3/integration` at `1200178c5ffe3decaee09077e2bb8b7f59ea715c` (#40), merged without conflicts as `64fc88fe44afa418037f36fc4398fa4c9720a5c2`.
- Tested source: **`81cb5558070718446a61868681bbc08c06aa6a3c`**, eight mandatory executable-source hashes plus ten informational context hashes in [SOURCE.json](merge-ref/SOURCE.json). The final evidence commit changes documentation and receipts only.
- Author: Codex-4 `/root`, GPT-6 family; exact parent runtime variant/effort is not exposed. Read-only peer: `gpt-6.1-sol`, high, own subcanvas `ae630788-sub-review`.
- Owned changes: Go evidence gate and this unit's evidence. Foreign `output/ext-agents/` is excluded. Application code, public contracts, SQL, browser fixtures, waits and assertions are unchanged.

## Root cause and fix

The old gate compared all 18 recorded source hashes with the checkout. CI tests a PR merge ref, so an unrelated trunk edit to `scripts/dev/test-node.sh` rejected unchanged unit evidence. The comparison now requires the fixed eight unit-owned executable producers: Go gate and browser harness, acceptance helper, ledger module, cases JSON, static gate and claims/source modules. Every required digest must be present and match safe actual checkout bytes; deleting a receipt entry cannot exempt drift.

Shared harness (`test-node.sh`, `test-local.sh`), configuration and product-context hashes are recorded separately as `informational_source_files_sha256`. They describe the actual run; later shared-file changes do not invalidate this unit. All 18 context hashes were frozen before/after this batch's runs. Legacy extra hashes remain readable without becoming checkout gates.

Artifact integrity is unchanged: every canonical browser file, including both logs, ledger, last-run receipts and manifest, matches `ARTIFACTS.json`; all five native copies also match `copied_native_bytes`. The external manifest hash avoids a self-hash cycle. SOURCE, index and manifest agree on the tested SHA and hard source map. Exact identities, expected claims, PASS and duplicate rejection remain.

## Red → green

[RED receipt](merge-ref/red.json) / [log](merge-ref/red.log): the new ten isolated shared-file mutations all fail with the old loop (**exit 1**). The [forensic source](merge-ref/red-gate.source.txt) has no runnable test filename.

[GREEN receipt](merge-ref/green.json) / [log](merge-ref/green.log): those ten cases pass after the fix, and all six existing corruption controls still reject PE/CC12 log, ledger, manifest-retaining-commit, acceptance-source and gate-source tampering (**exit 0**). Exact command: `go test ./tests/foundation -run '^TestProductEditorMatrixLedgerEvidence(SharedSources|Corruption)$' -count=1 -v`. These controls mutate isolated temporary checkouts, never delivered evidence.

## Actual gates on frozen 81cb5558

[Local receipts](merge-ref/local-gates.json), [browser receipt](merge-ref/browser-run.json), [vet receipt](merge-ref/go-vet.json) and [structure receipt](merge-ref/check-gates.json) and [Go receipt](merge-ref/go-ledger.json) bind exact commands, bounded PIDs, timestamps, exits, log hashes and unchanged hard8/info10 run context.

| Command | Exit | Evidence |
| --- | --- | --- |
| `bash scripts/dev/test-node.sh` | 0 | [test-node.log](merge-ref/test-node.log), 56.085 s; matrix static and claims suites included |
| `pnpm typecheck:admin` | 0 | [typecheck.log](merge-ref/typecheck.log), 1.542 s |
| `go vet -tags browser ./tests/foundation` | 0 | [go-vet.log](merge-ref/go-vet.log), 3.632 s; empty success output |
| `bash scripts/dev/check-gates.sh` | 0 | [check-gates.log](merge-ref/check-gates.log), 316.535 s; 83 documented modes, tracked tests run, contract drift errors=0 |
| `bash scripts/dev/test-local.sh --browser-product-editor` | 0 | [browser-product-editor.log](merge-ref/browser-product-editor.log), 930.315 s including bounded PG wait; one original complete run |
| `go test ./tests/foundation -run '^TestProductEditorMatrixLedger' -count=1 -v` | 0 | [go-ledger.log](merge-ref/go-ledger.log), 11.155 s; current canonical evidence, ledger controls, six tamper refusals and ten shared-drift cases |

Browser: Chromium, `LC_TEST_LOCK_WAIT=1800`, outer bound 2400 s. The prior PG holder was alive with a fresh heartbeat; the mode waited and acquired the lock normally. [Initial observation](merge-ref/pg-lock-read.json), [own acquisition](merge-ref/pg-lock-acquired.json). No lock deletion/takeover, grep, calibration, added retry or relaxed wait. **12 PE/review + 2 mandatory CC12 cases passed, zero skips.** The Go harness and isolated PG verify **231 required unique PASS identities**, 77 each in zh-TW, zh-CN and en at 390px; raw ledger has 286 rows.

Native run root: `output/playwright/pr28-merge-ref-20261012T160744Z`. Canonical byte copies: [ledger](merge-ref/browser/product-editor-click-ledger.json), [PE/review log](merge-ref/browser/playwright.log), [CC12 log](merge-ref/browser/cc12-playwright.log), last-run receipts and [manifest](merge-ref/browser/browser-evidence.json). No screenshot delivery claim; native bytes are copied without reformatting.

## Historical evidence and limits

Prior `template-fragments` evidence at `e15311a7` and pushed head `3dac7467` is historical, including its initial focus failure and later successful complete run. Earlier `round1`, `round2`, `round3`, `round3-canonical` and root `browser/` retain their original SHA; round1 predates #23. Receiver/effect binding, native-dialog postcondition, expected claims, template-fragment identity and DELIVERY-selected artifact integrity remain. Historical runs are not current-source acceptance.

[Scoped readonly audit](merge-ref/AUDIT.md): source E1 and final artifact-consistency PASS. It verified the actual E3 receipts, hard8/info10 metadata, all 27 audit-time artifact digests and five byte-exact native copies. No new K3, Claude or CI approval is claimed.

Evidence: **E3 in the automated tested environment — BROWSER + isolated REAL_PG, synthetic MOCK identity/provider.** New-head CI/external approval, full foundation, provider SANDBOX/LIVE and deployment are **NOT_RUN**. CI gates: foundation-shards and affected browser planner modes including `--browser-product-editor`. The same CI run's live-console timeout is outside this unit and unchanged. No production, live keys, real money/messages or real buyer PII.

Normal branch push is authorized after the final evidence gate passes. Thread resolution and PR merging remain with the integrator. Own browser processes/fixtures exited; the browser-build lock was released. No rebase, force-push, deploy or foreign cleanup.
