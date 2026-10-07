<!-- Purpose: Hand off both PR6 selector P1 repairs as one batch for integrator K3 pre-review and a new PR.
Depends on: ORCHESTRATION.md, pr6-b1300290 packet, Codex-2 cross-review, real test-local registry and Node evidence.
Used by: Codex-1 owner and integrator; no push or full runtime acceptance claim. -->
# CI PR modes coverage

- Branch/worktree: `unit/ci-pr-modes-coverage`, `/Volumes/data/live_commerce_architecture_v1/.worktrees/ci-pr-modes-coverage`.
- Original base: **424f17ccd3fd8db00923d91d3d23ca8bfafc249f**. Original source: **e385769110c037968c2711916f84bd4f52d42a75**. Review follow-up starts from **c1f3acf97ea66a57e0277beb96c623b9eff81a7c** after `git pull --ff-only` (already current), preserving trunk merge 9369640f and integrator P2/comment fixes.
- Owns `scripts/dev/pr-modes.mjs`, `tests/ci/pr-modes.test.mjs`, this evidence directory. No PR5 source changes. Exact model variant not exposed; one inherited-model read-only explorer at explicit medium reasoning, no parallel code writer.
- Assignment targets PR6 comments **4206797787**, **4206797789** plus Codex-2's CLI compatibility P1. The packet's historical full-foundation timeout is not claimed fixed by this selector unit.

## Result

1. Browser-tagged foundation source no longer depends on filename. Every such edit adds existing CI browser modes **plus modes derived from the actual test-local dispatch branches using `-tags browser` and `./tests/foundation`**. No per-file/test-name mode table. Inventory has 47 tagged files; `account_process_test.go` requires browser-identity, and `studio_process_test.go` also requires **--studio-backend**, which does not contain “browser” in its name.
2. CLI reads source from the diff's **merge-base and explicit head**, and uses that head's registry. `--no-renames -z` retains old/new rename paths and unusual filename characters. Stdin considers worktree and HEAD; removed tags/deleted inputs cannot silently turn into backend-only selection. Missing source conservatively selects tagged runners. One base argument defaults head to HEAD, matching the new orchestration command. The review repair additionally unions the legacy display/trim classifier (and its checkout registry); raw paths alone still own source lookup.
3. Deploy selection retains `deploy/` and `scripts/deploy*`, adding deploy-smoke workflow (`.yml`/`.yaml`), `.github/scripts/` gate helpers and `tests/deploy/` executable smoke inputs. The current workflow calls smoke-verdict.py; smoke.sh executes deploy-prep tests.
4. Original browser-prefix/unknown-UI behavior and secret-dependent `--stripe-browser` exclusion remain. The original frozen old-classifier oracle checked **6666 raw API paths** at that snapshot, plus synthetic unknown/removed paths. **That proved only API-level monotonicity, not real Git CLI compatibility.** Codex-2 found four real CLI path sets that lost modes/deploy selection because Git quoting and legacy trimming were absent from the oracle. The correction and real CLI evidence are below; the earlier broad no-loss claim was incorrect.

## Codex-2 P1 follow-up (current)

- Review: `output/integrator/cross-reviews/pr-modes-e385-codex2-eq4_nt8k/REVIEW.md`; its 33 real Git path sets had four regressions. No change to the integrator-owned P2 repair in c1f3acf9.
- Keep NUL-delimited raw paths for source/tag lookup. Separately obtain **the same `git diff --name-only base...head` display** the legacy CLI read, retain its per-line `trim()`, and union its mode/deploy selection. The legacy branch uses the checkout's registry, as the old CLI did even for an explicit non-checkout head. `--stdin` gets the same trim-compatibility union. No quoted/trimmed spelling is used to probe a source file.
- New real Git fixture cases cover `架构.md`, `internal/示例.go`, an actual tab in an untagged foundation path, and a leading-space deploy script, each with `core.quotePath=true` and `false`. An independent frozen legacy classifier consumes the actual Git display output. A stdin case also pins that legacy spelling must not trigger a phantom missing-file/tag lookup.
- **Red:** `review-cli-red.log`, exit 1, **17 PASS / 7 FAIL** on unchanged c1f3acf9 implementation. **Green:** `review-cli-green.log`, exit 0, **24 PASS / 0 FAIL**. Failures explicitly include lost browser modes and dropped deploy=true.
- Independent Codex-2 re-review remains pending; no push or merge.

### Current follow-up gates

| Command | Exit / result | Evidence |
| --- | --- | --- |
| `node --test tests/ci/pr-modes.test.mjs` before repair | 1; 17 PASS / 7 FAIL | `review-cli-red.log` |
| Same command after repair | **0; 24 PASS / 0 FAIL** | `review-cli-green.log` |
| `bash scripts/dev/test-node.sh` | **0; 636 PASS / 0 FAIL**, summed runners | `review-node.log` |
| `LC_HEADER_BASE=c1f3acf9 bash scripts/dev/check-gates.sh` | **0; 78 modes** | `review-check-gates.log` |
| Both modified JS files: `node --check`; `git diff --check` | 0 | author command receipts |

Current source SHA-256 (evidence binding before commit):

- `scripts/dev/pr-modes.mjs`: `0159d41cec8879749ab88fd4f7f0cf412fc49ad8b2baac03c3d0db950d9fbc55`
- `tests/ci/pr-modes.test.mjs`: `2a770db5884fae51cca90bea10b29f705e784d4201324c9280b6c4a03f98b9bc`

E3 is limited to these automated selector/Node/static gates. Optional R04 input runner remains NOT_RUN, excluded from 636. No browser/PG/full-foundation/deploy run in this follow-up; this repair changes only the CLI compatibility seam and its tests. Fixtures created by Node tests were cleaned by the test runner; no owned background process remains.

The runner parser follows the registry's current **if/elif dispatch with literal tag/package arguments**, including command arrays and backslash continuations. A future registry format change (e.g. case dispatch or computed tag variables) needs this parser and its acceptance cases updated together. This is deliberately not a general shell parser. Build-tag detection can over-select for a literal tag example inside a source file; it cannot hide a real tag behind a package-like comment.

## Original e3857691 local gates (historical, not current-source acceptance)

| Command | Exit / evidence | File |
| --- | --- | --- |
| `node --test tests/ci/pr-modes.test.mjs` before repair | **1**, 8 PASS / 5 FAIL (both P1s and revision/deletion paths) | `red.log` |
| Additional comment/header and explicit-head registry regressions before repair | **1**, 13 PASS / 2 FAIL | `edge-red.log` |
| `node --test tests/ci/pr-modes.test.mjs` final source | **0**, **15 PASS** | `green.log` |
| `bash scripts/dev/test-node.sh` | **0**, **626 PASS / 0 FAIL**, summed runners | `node.log` |
| `LC_HEADER_BASE=424f17cc bash scripts/dev/check-gates.sh` | **0**, 78 modes, header ratchet | `check-gates.log` |
| `node --check scripts/dev/pr-modes.mjs`; `node --check tests/ci/pr-modes.test.mjs`; `git diff --check` | **0** | author receipts |
| `node scripts/dev/pr-modes.mjs origin/r3/integration` at source commit | **0**, 48 modes, deploy=false | `required-plan.json`, `required-plan.log` |

Initial full Node/static attempts exited 1 because the new worktree had no node_modules (`typescript-api` absent); retained `setup-node-failed.log` and `setup-gates-failed.log`. `pnpm install --offline --frozen-lockfile` exited 0, reused 49 packages/downloaded 0, changed no lockfile. These setup failures are not behavior-red evidence.

Original source SHA-256 evidence binding:

- `scripts/dev/pr-modes.mjs`: `d292edc97bf6e74cc8bcf714e513b4bc8aa771dcc20523326334ea312a3df312`
- `tests/ci/pr-modes.test.mjs`: `6180ba4241b58910a057fb0d5d655047ab6139bbce247fc5aeed3ab4e7c8c795`

The initial explorer found no P0/P1, but the subsequent **Codex-2 cross-review found the P1 documented above**, so the original batch was not accepted. E3 for the historical rows is limited to those checks. Current re-review is pending. No new dependency, product runtime, Go/SQL or UI change in this repair.

## Handoff / remaining gates

- The current assignment requires the planner Node tests, test-node and check-gates; current results are separated from historical evidence. No Go area or TypeScript source changed in this repair.
- Repo-wide browser modes selected by the conservative script-change policy are recorded in `required-plan.json` and **NOT_RUN locally**; likewise full foundation and deploy-smoke execution. Docker capacity was observed as 6209777664 bytes; this is not a claim that RAM was unavailable. Local scope here is the explicitly assigned Node planner gate, not product browser acceptance.
- Optional R04 input runner is NOT_RUN (`COMMERCE_R04_LIVEKIT_BINARY` unset), excluded from 626 passes.
- No push/PR creation/deploy/provider action. **Stop after commit for Codex-2 re-review**, per current assignment; integrator owns the next push. No new CI run was started by this unit. Review/result packet is the wake-up; synthetic Git fixtures were removed by their test cleanup and logs preserved.
- No PR5 source or other queued unit was touched by this follow-up; follow the current integrator assignment.
