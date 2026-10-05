<!-- Purpose: Preserve admin-visual rebase recovery points and byte-preservation checks. -->
<!-- Depends on: local recovery branches, scoped stashes and explicit integration SHAs. -->
<!-- Used by: integrator review and recovery; no push or deployment authorization. -->
# Integration rebase checkpoint

## Machine-wide lock integration (2026-10-06)

Per explicit infra request, merged r3/integration **bb71f9666fee66f407d7a9db0d8683de43067f0e** into this branch with `git merge --no-ff r3/integration`; new merge commit **f1c5199adeb0c59cd74bb7b6e34a0cd0977bf2bf**, requested coauthor footer. Includes **15540a95** and upstream trunk-green/mock fix. No conflicts. New runner, test-focused, frozen detector and shared mock have no authored diff against bb71.

Recovery branch: `codex/admin-visual-before-global-lock-20261006` at4b9. Retained output-only stash **295c38613f24930e4755de5a16a862f45eba9206**;230saved paths restored byte-for-byte(diff0/0bytes). `merge-lock-output-backup.patch` remains local rollback material, excluded from final delivery. No untracked evidence removed.

Old4b9 batch had finished33modes(28zero,5nonzero) and no owned test processes remained before merge. Fresh shell syntax checks and install/node/adminTS/strictgates returned0. Next batch is strictly serial, requires155ancestor, waits7200s, stops on exit2/NOT_RUN. No old runner launched after merge.

## Latest explicit integration request

2026-10-05: pinned latest r3/integration to **1a6a917774672f297e13ce56ef070a3084a2a924**. Old runtime source **f0b93ca6** was safely stopped before changing source; the full sweep is labelled INTERRUPTED, not accepted. Recovery ref: `codex/admin-visual-before-latest-integration-20261005`.

Replayed48commits without conflicts. NewHEAD: **4b9d5dc85cc58047c4aa06875a2461cb2c349d49**. Scoped stash **495934f454b29cce74266618cf0ec68f4eaff58b** saved230 tracked generated outputs, applied successfully and retained; exact diff against all230saved paths returned0,0diffbytes. Untracked evidence untouched. Local `rebase-latest-output-backup.patch` and paths list are rollback material, not final delivery artifacts.

No differences in admin app, sharedUI, admin tests or frozen tests/ui between f0 and the rebased HEAD. New integration changes include0119inbox,0120kwc-v2,0121templates and0122lifecycle; imported backend code is not authored by this UI unit. Frozen clipping correction82e remains intact. Old source-bound receipts stay historical; final gates are rerun on4b9.

- Original branch head with copy repairs: `a6d37a5e`; recovery ref `codex/admin-visual-before-integration-20261005`.
- Target: `65902dbe` (current local `r3/integration` at the request, includes named `fd6ce9ab`).
- Rebased head before the new P2 repairs: `8fcb3745270b288d34d7e0b6a7208da8ffbddb30`.
- Replayed36commits. Two conflicts were only the ProductDocumentForm header/React import hunk: preserved upstream comments and each stage's required hooks. No commit skipped, no integration code discarded.
- Saved208 generated tracked output changes with scoped stash `ee18c544c961f2009fb00ab4361eb8a1862f08e1`. Applied after rebase, retained the stash. Independent byte comparison against every saved file:208checked,0mismatch, exit0. Original diff and path list remain local `rebase-output-backup.patch`/`rebase-output-paths.txt`.
- Untracked evidence was never stashed, deleted or overwritten by the rebase. Unrelated output files are not part of the UI source commits.
- Source repair `8ee1127c`: restored the distinct nonblocking3-image recommendation; zh-CN legal copy consistently uses评论收单. Red test1→green8; no permission, data model or write-command change.

Prior evidence pins remain historical. No push, deployment or live-console implementation.

## Approved harness-owner integration

The harness owner merged `82eebefb` through `3ed634be`: R6/R9 use ancestor-clipped visible rectangles, with unchanged thresholds and new pure/canary coverage. After the830b86f9 focused sweep completed (95/95 actual clicks), this branch rebased44commits onto `3ed634be`, without conflicts. New head: `0e2b5c55b7f01225c0f3fd3960ad37aa063c7fb1`.

Rollback ref: `codex/admin-visual-before-harness-rebase-20261005` (`830b86f9`). Scoped output stash `d30c6899ef0f653816ae69a69b554c2d86bf1bce` was applied and retained; all224 saved output files compare byte-identical (exit0). Binary backup and path inventory remain local. No UI-unit edits to the harness, Go or migrations appear relative to the new base.

Two task-owned evidence notes were found at the worktree root and relocated into this output directory; their contents were unchanged. The now-empty stray directory was removed. Nothing from another unit was deleted.
