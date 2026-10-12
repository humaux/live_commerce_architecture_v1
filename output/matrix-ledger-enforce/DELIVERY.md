# matrix-ledger-enforce delivery — PR28 canonical outcome addendum

- Branch: `unit/matrix-ledger-enforce`; pulled base `5e4e96b9bc5ce1e33185442d3173555ccec3dc79`.
- Tested source: **`05c2af77a6f90cef0fc797520b2f6361d1635527`**, with 18 Git/current hashes in [SOURCE.json](round3-canonical/SOURCE.json). The final evidence commit changes only unit documentation and receipts.
- Preserved commits: `bcc24338` receiver/effect identity; `ec94cb05` native-dialog precondition; `7f02bbea` preceding evidence. Author: Codex-4 `/root`, GPT-6 family; exact parent runtime variant/effort is not exposed. Read-only audit: `gpt-6.1-sol`, high, task `a480d297-sub-resume-audit`.
- Owned scope: matrix recorder, static Node gate/helpers, shared test declaration and unit evidence. Foreign `output/ext-agents/` and unrelated untracked files are excluded from staging.

## Result and review replies

The gate now checks each reached outcome against machine requirements declared alongside the case's canonical `expected` claim: exact locator, matcher, operand, polarity, action/reload position, write count and ACK source. Persistence must check the declared rows after reload, and immutable SKU codes and archived IDs must come from their actual producers. Repeating row/ID zero cannot certify every row. Supplemental or unused callbacks cannot donate proof.

The original operated-locator/effect identity check and the requirement for a post-action assertion inside the same step before its observed return remain. The source tracer uses a deterministic clock only in its inert evaluation; the real browser's clock, fixtures, waits and assertions are unchanged. Source extraction and claim validation have separate focused helpers. Runtime ledger rows keep the original fields without copying static proof metadata.

All **77 original page/row/control/action/expected declarations are unchanged**. New optional requirements/effects are test metadata; there are no application, HTTP, SQL or public contract changes. Native acceptance retains the absent-before-dialog to server-issued replacement UUID transition, successful ACK and old-ID exclusion from `ec94cb05`.

| Thread | Fix SHA | One-line reply for integrator |
| --- | --- | --- |
| `4246050140` | `bcc24338` + `05c2af77` | Every operated action requires a post-action outcome before its observed return, bound to the operated locator or declared effect; removed and unrelated outcomes are RED. |
| `4246713403` | `05c2af77` | Each outcome now validates its canonical expected claim, including exact values, write counts and persisted rows; dropped count/persistence assertions and unrelated visibility checks are RED. |

## Red → green

[red-tests.json](round3-canonical/red-tests.json) binds the producer gate, helper, declaration and spec hashes plus exact argv/env, mutation copies, raw logs and exits. Before implementation, five expected-claim regressions failed (exit 1); resumed row/code regressions failed twice, and repeated archived-ID regression failed once (exit 1). Current full Node includes **51 focused PASS / 0 FAIL / 0 SKIP**.

Final-source calibrations use `node --test --test-name-pattern='every actual mobile' tests/ci/product-editor-matrix-ledger.test.mjs` with the recorded `LC_MATRIX_ACTION_MUTATION`:

| Mutation / restoration | Exit | Proof |
| --- | --- | --- |
| `dropped-write-count` | 1 | Canonical local-write proof missing |
| `dropped-persistence` | 1 | Required post-reload value assertion missing |
| `unrelated-outcome` | 1 | Unrelated visible control cannot certify the action |
| `nested-step` | 1 | Nested outcome cannot certify its parent |
| `native-missing-outcome` | 1 | Only native outcome failures in three locales; no unrelated DOM-read failure |
| Restore original source, no mutation | 0 | Actual-source gate green |

Raw snapshots end in `.source.txt`; none are runnable test filenames. [Audit](round3-canonical/AUDIT.md) records source-bound peer checks: baseline green, ten negative probes RED and unchanged source hashes. This is scoped source/focused evidence, **not new K3/Claude or CI approval**.

## Actual gates on frozen 05c2af77

[Local receipts](round3-canonical/local-gates.json) and [browser receipt](round3-canonical/browser-run.json) record bounded runner PIDs/timeouts, UTC timestamps, argv, exits and log hashes.

| Command | Exit | Evidence |
| --- | --- | --- |
| `bash scripts/dev/test-node.sh` | 0 | [test-node.log](round3-canonical/test-node.log), 70.729 s; includes 51 matrix tests |
| `pnpm typecheck:admin` | 0 | [typecheck.log](round3-canonical/typecheck.log), 1.749 s |
| `go test ./tests/foundation -run '^TestProductEditorMatrixLedger' -count=1 -v` | 0 | [go-ledger.log](round3-canonical/go-ledger.log), 25.637 s; DB-free ledger corruption controls |
| `bash scripts/dev/check-gates.sh` | 0 | [check-gates.log](round3-canonical/check-gates.log), 258.145 s; [staged artifact recheck](round3-canonical/check-gates-staged.json) also exit 0; 82 modes documented, every tracked test run |
| `bash scripts/dev/test-local.sh --browser-product-editor` | 0 | [browser-product-editor.log](round3-canonical/browser-product-editor.log), 181.158 s; original complete mode |

The browser run used `output/playwright/pr28-canonical-20261012T142112Z` under the worktree, `LC_BROWSER_ENGINE=chromium` and bounded shared-PG lock waiting. No grep, calibration, retry or relaxed wait. **12 PE/review cases and two mandatory CC12 cases passed, zero skips.** Real Go/isolated PG validates **231 unique required PASS identities** at 390px: 77 each in zh-TW, zh-CN and en. The raw ledger contains 286 rows, including non-matrix coverage.

Current canonical copies are the actual [ledger](round3-canonical/browser/product-editor-click-ledger.json), [PE/review log](round3-canonical/browser/playwright.log), [CC12 log](round3-canonical/browser/cc12-playwright.log), last-run receipts and [manifest](round3-canonical/browser/browser-evidence.json). Copies preserve native bytes and SHA256. No screenshot delivery claim. All 18 executable source hashes remained unchanged throughout the gates.

## Historical evidence and limits

Earlier `round1`, `round2`, `round3` and root `browser/` evidence belongs to its recorded source, not this addendum. Round1 ran before the #23 evidence-path merge; its old source hash is not a current binding. The round2 native diagnostic included unrelated missing DOM reads and is not the isolated native RED above.

Intermediate `focused-green.log`, `resume-focused-green.log` and `implementation-diagnostic.log` under `round3-canonical` are diagnostic/history only. The stopped pre-freeze runner is explicitly `NOT_ACCEPTANCE`; its early Node log was replaced and was not retained. Only the completed `local-gates.json` and `browser-run.json` certify the final source. Forensic snapshots/logs preserve raw producer whitespace: the ancillary staged whitespace diagnostic exits 2; non-forensic files and source commits exit 0 ([receipts](round3-canonical/whitespace-checks.json)). No project gate is disabled.

Evidence: **E3 in the automated tested environment — BROWSER + isolated REAL_PG, synthetic MOCK identity/provider fixtures.** The source gate covers its declared grammar; unsupported wrapped assertions fail closed. It is not a general formal proof of arbitrary TypeScript. Browser assertions and PG readbacks provide runtime acceptance.

CI gates: foundation-shards and the PR planner's affected browser set including `--browser-product-editor`. New-head CI/external approval, full foundation, provider SANDBOX/LIVE and production/deployment are **NOT_RUN** here. No production, live keys, real money/messages or real buyer PII.

The owner authorized normal branch push for this batch after gates. Integrator retains review-thread replies/resolution and merging. No thread was resolved and no merge/rebase/deploy performed. Own runners and browser fixtures exited; shared browser-build lock released. No foreign directory or shared cache cleanup.
