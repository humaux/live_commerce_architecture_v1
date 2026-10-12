<!-- Purpose: Qwen pre-push four-P2 closure, current-source verification and artifact layout evidence.
Depends on: appended unit brief, real evidence ratchet/shell/tool seams and production browser gates.
Used by: integrator PR creation/re-review; does not claim remote CI or external vision execution. -->
# Qwen pre-push P2 batch

- Base delivery `c47cbc63`; implementation `3145adfa`; merged origin/r3/integration `30ddfb10` (#19 Go 1.27.2 security upgrade) as `f3fab830cb2ce009b89b71a8607b8c6df3dce598`. Conflict resolution retained run-root destinations plus upstream toolchain pins. PR #22 was not present on fetched remote trunk.
- Four requested findings fixed; product code, historical summary files, assertions and thresholds unchanged. `output/ext-agents/` is integrator-owned untracked material and remains untouched/unstaged.

## Closures

1. GATES.md now names `<run-root>` / `<suite-evidence>` for visual lint/LATEST, click sweep/platform pass, legacy review shots and picklist, with the ignored/explicit-root convention defined once.
2. Ratchet covers writable open/openSync, writeSync descriptor paths, URL writer arguments, bound writer/path aliases and Go OpenFile create/write flags including named flag variables. Historical read-only opens remain legal. Added real checker regressions before the fix; seven current-producer injections each exit 1, restore byte-for-byte, then exit 0. Proof includes checker/producer SHA-256s and post-merge byte verification.
3. Ops-disk-guard allocates an ignored root; LC_OD_EVIDENCE governs both child output and tee's od-all.log. Existing committed summary stays history. Actual Bash allocation-seam test was RED then GREEN; real disk containers are NOT_RUN this small batch.
4. Visual-lint upload is separate/selective: gate log, lint.*, index.json and shots; crops, traces, facts and unrelated suites excluded. Visual review recursively discovers shots in old/nested artifact layouts and retains interrupted-run shots even when index is missing. Both upload-selection and real copy-loop tests were RED then GREEN. Local layout dry-run uses a real prior browser screenshot: five selected files, exact recovered PNG hash, index/lint copied and raw extras excluded. Remote CI artifact/paid vision calls are NOT_RUN.

The requested operator tool in the main checkout was synchronized under its file lock with a byte backup at `output/playwright/evidence-p2-validation/visual_review.original` in this worktree. The canonical updated script is committed at `output/integrator/tools/visual_review.sh`; sync hashes and dry-run tool hash are recorded. No other integrator tool was changed.

## Gates at f3fab830

| Command | Exit | Evidence |
|---|---:|---|
| `bash scripts/dev/test-node.sh` | 0 | `p2-evidence/node.log` |
| `bash scripts/dev/check-gates.sh` | 0 | `p2-evidence/gates.log` |
| `pnpm --dir apps/admin exec tsc --noEmit -p .` | 0 | `p2-evidence/admin-tsc.log` |
| `GOFLAGS=-p=1 go vet -tags browser ./tests/foundation` | 0 | `p2-evidence/browser-vet.log` |
| `bash scripts/dev/test-local.sh --browser-product-editor` | 0 | `p2-evidence/browser-product-editor.log` |
| `bash scripts/dev/test-local.sh --browser-admin-shell` | 0 | `p2-evidence/browser-admin-shell.log` |

Bounded status JSON binds current HEAD/source hashes and exits. Both browser modes run serially and require empty raw tracked status plus unchanged historical evidence hashes without restoration. Seven ratchet proofs and artifact selection/copy dry-run are in `p2-evidence/`; raw originals remain ignored under validation. Committed log copies normalize trailing whitespace only.

Read-only independent review executed 36 path/boundary tests at f3fab830 (36 PASS, no P1/P2 found); this is limited E3 evidence, not browser/CI acceptance. **E3 at f3fab830**: both requested browser modes passed (editor PE12–17/review plus frozen CC12; shell 24 matrix + 24 brand click/reload cases), with empty tracked status and unchanged historical hashes. All started runners exited. Failed filesystem fixtures were archived before reclaiming; raw screenshots/logs/rollback copy remain ignored. Final delivery commit adds evidence only. No push. All remaining CI modes, disk runtime, remote artifact listing, provider and external vision acceptance are NOT_RUN.
