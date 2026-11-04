# PARITY-LIVE — integrator ruling implementation, 2026-11-01

Status: implementation prepared; PR #355 current-head CI GREEN **PENDING**. Author assessment E0; local architecture-packet structure E1 only. No completion or E4 acceptance claim.

## Authority and ownership

- Authority: integrator's 2026-11-01 ruling in this task, options 1 and 2; repository AGENTS.md; 架构.md §§0,1,2,26,27; live-console-v1 §13.2; current fixture code and original GitHub receipts.
- Assigned role: implementation worker dispatched by Claude integrator; no sub-agent. Runtime identifies GPT-6; exact model variant/effort is not exposed, UNKNOWN.
- Branch/worktree: `unit/parity-live`, `/Volumes/data/live_commerce_architecture_v1/.worktrees/parity-live`; PR https://github.com/humaux/live_commerce_architecture_v1/pull/355 targets `r3/integration`.
- Starting head: `1030a7e1a248c121aaadc310e89f24036043d93e`. First merged `origin/r3/integration` at `fde11c1310fce20b576fc98023bd3003c66c205b`, merge commit `4ae9439c081ebfb5b570e834cb90c288bdeb839c`.
- The merge's only conflict was generated browser ownership. Resolved by `node scripts/dev/derive-browser-ownership.mjs`, then regenerated again after the spec change. Integration's other changes are inherited, not authored by this card.
- Allowed source scope for this ruling: parity fixture/describe in `tests/admin/live-console.spec.ts`; required generated browser ownership; this report and existing `red.log`. No product change.

## Changes, file:line

| Ruling | Source | Result |
|---|---|---|
| Isolate the parity scene set | `tests/admin/live-console.spec.ts:882` | GET session-list requests now return only the same synthetic parity session used by console/detail/comments. Full nine-field Studio draft shape is checked with `satisfies StudioPage`; response retains the private no-store header and locale-specific title. The eight legacy scenes and other cases' fixtures are untouched. |
| Make only parity shardable | `tests/admin/live-console.spec.ts:932` | `test.describe.configure({ mode: "parallel" })` is confined to the parity describe. Existing eight shard invocations can partition these cases at test granularity; global `workers: 1`, global `fullyParallel: false`, legacy ordering and budgets stay unchanged. Actual partition counts await CI. |
| Preserve strict acceptance | Whole change | No assertion, expected value, timeout, threshold, exemption, product source, migration, R2 pin, contract, lockfile, next-startup baseline or route-dispatcher file changed. Existing 152 frame + 6 locale/viewport + 3 safety cases remain the 161-case acceptance target. |

Spec SHA-256: `b57ef70d63545feabf461d4adb83df5d3e6749a6348e88deac6a22214fe3d795`.
Unchanged `playwright.config.ts` SHA-256: `ad5ff56ccc1c4302c7a9daad01eb9e40c3ba02383e1b8692c9a7cf42f6c47c04`.
Unchanged Go harness SHA-256: `12a08781a89c04c6200c7ffd7c140c1701471aa1778f6fa0186f818d443ffe30`.

Optional diagnostics (option 3) omitted: the two authorized root-cause changes need no subprocess/budget change.

## Established RED (before this implementation)

- Original tests-only commit `898e71a829d7f4bce261eebc95ec5028693f8901`; Gates run **41050616128**, job **124051353169** failed. Existing `red.log` records recovered new exact-heading assertion failures (frames 1429:39502 and 1429:39683), before implementation; the outer job timed out. This ruling continues that existing RED→implementation cycle; no duplicate RED push.
- Immediate pre-ruling Gates run **41258492409**, head `1030a7e1a248c121aaadc310e89f24036043d93e`, parity job **124707559191** failed; legacy job **124707559131** passed. Re-read via `gh run view` and filtered failed-job log this session.
- Raw artifact **13065345268** from that parity run contains repeated failure of the unchanged `live-workspace` no-`MOCK` assertion at old spec line 938. Read original error contexts under `/tmp/diag-355/pr-parity`; their received text contains all eight `LC-U1 MOCK lane` titles. Existing `DIAG.md` lists the 60-context inventory. These are CI failure observations, E3, not successful acceptance.
- Latest failing job reports Go timeout after 780.135 seconds; downstream receipts **124714597637** and required **124714767067** fail. No timeout increase is implemented.

## Commands and evidence

| Command | Exit | Meaning |
|---|---|---|
| `git fetch origin r3/integration` | 0 | Refreshed integration before changes. |
| `git merge --no-edit origin/r3/integration` | 1, then resolved and committed | Generated ownership conflict only; generator used, no hand edit. |
| `node scripts/dev/derive-browser-ownership.mjs` | 0 | 56 modes; regenerated after final source edit. |
| `python3 scripts/check_packet.py` | 0 | `PASS_PACKET_STRUCTURE_ONLY`, E1; its incidental timestamp rewrite was restored. No product verification. |
| `git diff --check` | 0 | Whitespace only. |
| `command -v tsc` | 1 | Compiler absent; no installation, no local runtime attempted. |

CI acceptance command is the PR's existing `--browser-live-console-parity` mode (eight Go harness invocations), plus required Foundation/Gates checks. Success requires all **161** cases, all eight nonempty partitions and the PR's own checks green. Current replacement run/job IDs will be recorded in a PR comment and this report's local handoff after the single batch push; no evidence-only push.

## Scope, risks and NOT_RUN

- Outside source write paths: generated `output/ci-select-backend-browser/browser-ownership.json`; evidence `output/PARITY-LIVE/RULING-FIX.md` and `output/PARITY-LIVE/red.log`, all explicitly required by the task. Git preserves the existing uppercase directory spelling; on this filesystem it aliases requested `output/parity-live/RULING-FIX.md`.
- No Go dependency/header change; dependency map regeneration is unnecessary. No shared helpers, other-app source or backend package edited by this ruling.
- Risk: downstream product assertions have not yet run past the former fixture failure. A product-caused failure after this isolation/sharding is a STOP and report, not authority to change product code.
- NOT_RUN: local tests/servers, local TypeScript compilation (tool absent), replacement CI outcomes, all-161 pass/partition verification, independent review, SANDBOX/LIVE and production deployment.
- Humaux memory/store/canvas/coord/code-index tools are not exposed in this runtime; these operations are NOT_RUN, not represented by this file. Retained existing untracked evidence; no task-owned server/container/watcher started. Integrator resumes this lane on registered GitHub run completion.
