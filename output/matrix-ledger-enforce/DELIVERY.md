# matrix-ledger-enforce delivery — PR28 round 3

- Branch: `unit/matrix-ledger-enforce`; base/pulled head `5e4e96b9bc5ce1e33185442d3173555ccec3dc79` (pull was a no-op).
- Source commits: `bcc24338c6ca9eff892368849468a682dbd778a7` (receiver/effect binding), `ec94cb0502c657892ad649dc2ad4e8384cb03860` (capture DOM precondition before opening the native dialog). Final runs bind **ec94cb05** and 16 source hashes; later changes are evidence/docs only.
- Author/ownership: Codex-4 `/root`, GPT-6 family (exact runtime variant/effort not exposed). Read-only explorer: `gpt-6.1-sol`, high. Parent writes only the matrix gate, acceptance spec, shared test declaration and unit evidence. Foreign `output/ext-agents/` is preserved and excluded from staging.

## Changes and review findings

The static gate previously recorded an outcome event without its receiver. A `product-name` fill could borrow `product-slug.toBeVisible()` and record PASS without a value assertion. `tests/ci/product-editor-matrix-ledger.test.mjs` now resolves each direct awaited outcome through the same inert locator slice as the operated receiver. It discharges only that step's pending action, using an exact operated receiver with the operation's matcher class, or an explicitly declared method-specific effect locator. Fill requires value readback; check/uncheck require checked-state readback. Non-pure supplemental assertions cannot certify an action or contaminate dependency resolution.

The receiver/method/Enter/Escape/row identity checks, required DOM reads, action/outcome/receipt order and nested-step isolation remain. Cross-control effects are declared in the **same shared 77-case table**; all 77 page/row/control/action/expected values are unchanged. No blanket ancestor/descendant matching was added. If/for-wrapped outcomes remain fail-closed, as requested.

Native archive acceptance proves a UI change that cannot exist before acceptance: the replacement row's `data-sku-id` is absent before opening the dialog; after acceptance and the existing successful save ACK, a retrying UI assertion requires its server-issued UUID and excludes the old IDs. The native ledger records `beforeId: null` and `replacementSKU`. Capturing the precondition before `window.confirm` avoids a DOM read blocked by the open dialog. Original reload, field, write-count and PG assertions/waits/timeouts are retained.

Contract/interface changes: **none in application/HTTP/SQL/public contracts**. The test-only declaration adds optional effect locator/matcher metadata, consumed by the Node gate; Go continues enforcing the original exact identity/expected/PASS set.

## Red → green and integrity evidence

[red-tests.json](round3/red-tests.json) binds the initial red gate/spec snapshots and log, plus the **final gate SHA256**, final spec SHA256 and each hashed mutation copy. Initial unrelated-locator and same-control visibility regressions both failed before the fix: **exit 1, two failed tests**. Final focused suite: **37 PASS / 0 FAIL / 0 SKIP**, exit 0.

The exact review mutation retains the action and observed return, replacing only the name's value assertion with `await expect(page.getByTestId("product-slug")).toBeVisible();`. On the final gate:

| Mutation / restoration | Exit | Actual result |
| --- | --- | --- |
| `LC_MATRIX_ACTION_MUTATION=unrelated-outcome` | 1 | Only product-name outcome failures, three locales |
| `LC_MATRIX_ACTION_MUTATION=tautological-outcome` | 1 | Same-control visibility cannot replace value readback |
| `LC_MATRIX_ACTION_MUTATION=nested-step` | 1 | Parent lacks its outcome; nested extra identity also rejected |
| `LC_MATRIX_ACTION_MUTATION=native-missing-outcome` | 1 | **Only native archive outcome failures**, three locales; no missing DOM-read findings |
| Restore original final spec, no mutation | 0 | Real-source mobile action gate green |

These use `node --test --test-name-pattern='every actual mobile' tests/ci/product-editor-matrix-ledger.test.mjs`; exact argv/env/exits and hashes are recorded in the JSON. Source snapshots use `.source.txt`, never runnable `.spec`/`.test` filenames.

The old round2 seven-test RED lacks a producer-gate hash, and its intermediate native diagnostic contains unrelated DOM-read failures. They are historical/diagnostic only and **are not this round's isolated red proof**. The regenerated native RED above uses the final gate. Intermediate emitter diagnostics are marked `NOT_ACCEPTANCE` in `round3/diagnostics/`; earlier bcc local successes are historical under `round3/before-modal-precondition/`.

Read-only peer audit `b5c7252b-6bc0-45b8-9154-87dfed0a2db1` independently ran 37 focused tests and eight receiver/effect/polarity/dependency probes: exit 0, no confirmed P0/P1 in scope. It predates only the precondition-read relocation; final ec94 source/artifact audit `cc05f37b-4331-4634-95bb-11952e1f1c2c` verifies the relocated precondition, all 16 Git/current/run hashes, exact mutation snapshots and canonical ledger. It did not rerun the browser. These are scoped source/focused evidence, **not new Claude/K3 or CI approval**.

## Gates run on frozen ec94cb05

[SOURCE](round3/SOURCE.json), [local gate receipts](round3/local-gates.json), [browser receipt](round3/browser-run.json) retain argv, bounded runner PIDs/timeouts, UTC times, exits and log hashes.

| Command | Exit | Current evidence |
| --- | --- | --- |
| `node --test tests/ci/product-editor-matrix-ledger.test.mjs` | 0 | `round3/focused-green.log`; 37/0/0 |
| `bash scripts/dev/test-node.sh` | 0 | `round3/test-node.log`; 31.220 s |
| `pnpm typecheck:admin` | 0 | `round3/typecheck.log`; 0.925 s |
| `go test ./tests/foundation -run '^TestProductEditorMatrixLedger' -count=1 -v` | 0 | `round3/go-ledger.log`; DB-free corruption controls |
| `bash scripts/dev/check-gates.sh` | 0 | `round3/check-gates.log`; staged-evidence recheck also exit 0 (`round3/check-gates-staged.json`), 82 modes documented and tracked tests run |
| `bash scripts/dev/test-local.sh --browser-product-editor` | 0 | `round3/browser-product-editor.log`; 114.044 s, original full mode |

Browser evidence used `LC_BROWSER_EVIDENCE_ROOT` under `output/playwright/pr28-r3-outcomes-20261012T110558Z`, resolved against the worktree; no grep, calibration, retry or relaxed wait. **12 PE/review cases plus two mandatory frozen CC12 cases passed, zero skips.** Real Go/isolated PG validates **231 unique required PASS rows**, 77 per zh-TW/zh-CN/en at 390px; raw ledger has 286 rows. All three native receipts show the absent-to-new-UUID transition.

Canonical actual [ledger](browser/product-editor-click-ledger.json), [PE/review result log](browser/playwright.log), [CC12 result log](browser/cc12-playwright.log) and [browser manifest](browser/browser-evidence.json) are committed and hash-bound to ec94. No screenshot delivery claim; auxiliary native paths remain only in original records. No source/hash changed during or after final runs. Artifact names are checked again by check-gates before handoff. Raw logs and exact source/mutation snapshots retain producer bytes: the ancillary full `git diff --cached --check` exits 2 for forensic whitespace; non-forensic staged files and both source commits pass that diagnostic (exit 0). No project gate is disabled; records are in `round3/whitespace-checks.json`.

## CI gates / limits / handoff

Evidence class: **E3 in the automated tested environment; BROWSER + REAL_PG with synthetic MOCK identity/provider fixtures.** Required integrator CI: foundation-shards (Node/check-gates and ledger controls), plus the PR planner's affected browser set including `--browser-product-editor`. Current-head remote CI and new external approval are **NOT_RUN** here; provider SANDBOX/LIVE, production/deployment and the full foundation suite are **NOT_RUN**.

Risk/limit: this bounded source gate proves receiver/effect identity, matcher class, ownership and order; it is not a formal equivalence proof of every matcher operand. Actual original browser assertions and PG readbacks provide runtime acceptance. Unsupported wrapped-outcome grammar fails closed.

Integrator to-do: review/push the batch, run required CI, reply to and resolve threads. Author commits only. Owned browser/Next/PG fixtures and runners exited; browser mutex released. No production/real money/messages/keys/buyer PII, provider actions, push/rebase/deploy, foreign directory edits or shared-cache cleanup.
