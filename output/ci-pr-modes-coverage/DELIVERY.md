<!-- Purpose: Hand off both PR6 selector P1 repairs as one batch for integrator K3 pre-review and a new PR.
Depends on: ORCHESTRATION.md, pr6-b1300290 packet, real test-local registry, source e3857691 and Node evidence.
Used by: Codex-1 owner and integrator; no push or full runtime acceptance claim. -->
# CI PR modes coverage

- Branch/worktree: `unit/ci-pr-modes-coverage`, `/Volumes/data/live_commerce_architecture_v1/.worktrees/ci-pr-modes-coverage`.
- Base: **424f17ccd3fd8db00923d91d3d23ca8bfafc249f**. Source: **e385769110c037968c2711916f84bd4f52d42a75**. Final evidence commit changes no source.
- Owns `scripts/dev/pr-modes.mjs`, `tests/ci/pr-modes.test.mjs`, this evidence directory. No PR5 source changes. Exact model variant not exposed; one inherited-model read-only explorer at explicit medium reasoning, no parallel code writer.
- Assignment closes PR6 comments **4206797787**, **4206797789**. The packet's historical full-foundation timeout is not claimed fixed by this selector unit.

## Result

1. Browser-tagged foundation source no longer depends on filename. Every such edit adds existing CI browser modes **plus modes derived from the actual test-local dispatch branches using `-tags browser` and `./tests/foundation`**. No per-file/test-name mode table. Inventory has 47 tagged files; `account_process_test.go` requires browser-identity, and `studio_process_test.go` also requires **--studio-backend**, which does not contain “browser” in its name.
2. CLI reads source from the diff's **merge-base and explicit head**, and uses that head's registry. `--no-renames -z` retains old/new rename paths and unusual filename characters. Stdin considers worktree and HEAD; removed tags/deleted inputs cannot silently turn into backend-only selection. Missing source conservatively selects tagged runners. One base argument defaults head to HEAD, matching the new orchestration command.
3. Deploy selection retains `deploy/` and `scripts/deploy*`, adding deploy-smoke workflow (`.yml`/`.yaml`), `.github/scripts/` gate helpers and `tests/deploy/` executable smoke inputs. The current workflow calls smoke-verdict.py; smoke.sh executes deploy-prep tests.
4. Original browser-prefix/unknown-UI behavior and secret-dependent `--stripe-browser` exclusion remain. A frozen old-classifier oracle checks **6666 tracked paths**, plus synthetic unknown/removed paths: no old mode or true deploy flag is lost. Selection may conservatively add modes.

The runner parser follows the registry's current **if/elif dispatch with literal tag/package arguments**, including command arrays and backslash continuations. A future registry format change (e.g. case dispatch or computed tag variables) needs this parser and its acceptance cases updated together. This is deliberately not a general shell parser. Build-tag detection can over-select for a literal tag example inside a source file; it cannot hide a real tag behind a package-like comment.

## Local gates

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

Source SHA-256 evidence binding:

- `scripts/dev/pr-modes.mjs`: `d292edc97bf6e74cc8bcf714e513b4bc8aa771dcc20523326334ea312a3df312`
- `tests/ci/pr-modes.test.mjs`: `6180ba4241b58910a057fb0d5d655047ab6139bbce247fc5aeed3ab4e7c8c795`

Source inspection by a non-author explorer found no P0/P1; **integrator Kimi K3 pre-review is still pending** and its findings belong to this same batch. E3 is scoped to the automated selector/Node/static checks above. No new dependency, product runtime, Go/SQL or UI change.

## Handoff / remaining gates

- The assignment's three specific local gates are green. JS syntax checks also passed; no Go area or TypeScript source changed to require a focused Go/typecheck run.
- Repo-wide browser modes selected by the conservative script-change policy are recorded in `required-plan.json` and **NOT_RUN locally**; likewise full foundation and deploy-smoke execution. Docker capacity was observed as 6209777664 bytes; this is not a claim that RAM was unavailable. Local scope here is the explicitly assigned Node planner gate, not product browser acceptance.
- Optional R04 input runner is NOT_RUN (`COMMERCE_R04_LIVEKIT_BINARY` unset), excluded from 626 passes.
- No push/PR creation/deploy/provider action. Integrator runs K3 pre-review, then pushes and opens the new PR; its full required set remains pending. New CI run id not yet exists. Integrator review/result packet is the wake-up; no owned background process remains. Synthetic Git fixtures were removed by their test cleanup; logs preserved.
- PR5 stays separately owned/waiting on its completed-round packet. Follow the latest ORCHESTRATION lane queue, not older queued W2 assignments.
