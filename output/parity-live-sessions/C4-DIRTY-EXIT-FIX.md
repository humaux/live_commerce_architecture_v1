# C4 dirty metadata departure — bounded product fix 2/2

task_id: 63f62b4d-bb79-4b8f-b569-7dc4eea3af3d; own canvas: 63f62b4d-sub-1.
base_commit: 49337bad19983671edc8ecd4b58301f4a195b14c.
branch: codex/pr365-mock-balance; worktree: .worktrees/codex-pr365-mock-balance.
Requested worker: Sol/high UI writer; actual model/reasoning telemetry: UNKNOWN.
Root owns author lease and coord claim; no recursive delegation or independent claim.

## Authority and observed RED

Read repository AGENTS.md; architecture 0/1/2/26/27; tasks DAG T08/T09 and frozen contracts/studio-v1.md; invariants I01/I02/I06/I11/I14/I18/I21. Read Humaux d272d041-21c0-41f5-ac34-fa44f8d17e73 and d1e0b80a-d877-42be-a96d-81487608f044 originals, development detail 7bdb220b and ponytail. Rejected 11/11 inspection delegated by root was reused, not independently claimed.
Root SOURCE output/integrator/tools/logs/ci-evidence-1238/365-studio-source-1437.md (b1a9cee04a423f314fdcfe4783dc0624a1295e4ae611f027fac80a641ace6182) establishes the three original REAL_PG dirty failures at missing editor Back/Cancel, after successful owned-draft setup and fill; subsequent dialog/focus/zero-write assertions were not reached.
Frozen exact additions come only from root review-c4-backfix2-delta.md (fde0412dcbadc9e044ae19ef4d0335fd004aec376ea44322a7cc978a3b2f3ad8), adopted in d272. Existing dictionary values were byte-preserved; only the three approved metadata keep/discard/helper keys were added in each locale. Figma read/visual approval was root-provided; no independent Figma read or write is claimed here.

## Implementation

- Existing-session editor has Back labelled with the existing preparation label and existing Cancel label; both target the same locale/store/cursor/session preparation via original parent navigate.
- Parent stores only a confirmation identity snapshot and last focused field; original form/save/version/request/pending/recovery/CAS ownership remains in Studio. Original fields, IDs, date control, max/min/disabled rules, errors, version/read-only copy and Save logic were moved to one pure controlled view.
- Before opening and again on Discard, compare latest scope/scene/selected ID/form identity, authenticated boundary/cookie/current cookie, hidden/blocked/concealed/history recovery, busy, pending, uncertain and child copy-navigation guard. No UNKNOWN key replacement or discard exists.
- Discard passes that identity through original navigate/mayLeave, which still validates all its original non-dirty guards; only this validated local discard bypasses the native dirty question. Dirty/marker are not cleared before mayLeave accepts. Parent then restores form from persisted shown draft, clears local validation and returns to preparation. Clean exit uses original navigate without confirmation. No business mutation is added.
- Keep/Escape unmount the native shared ConfirmDialog (zero dialog nodes when closed), retain form and restore the last connected enabled input/select only while current identity/guards permit it. Scope/session/task changes, clear/logout and conceal invalidate confirmation. Shared Dialog implementation is unchanged.
- Other shell/copy/refresh mayLeave defaults and existing explicitDeparture effect are unchanged. No router/hash diagnosis or save-wait modification is included. Studio remains 791 lines without compression or raising its gate.

## Commands and evidence level

| Actual command | Exit / result |
| --- | --- |
| git status --short; git rev-parse HEAD (before edits) | 0; clean and exact base |
| pnpm --filter @live-commerce/admin exec tsc --noEmit --incremental false | 254; ERR_PNPM_RECURSIVE_EXEC_FIRST_FAIL, tsc not found; no node_modules in this worktree |
| git diff --check | 0 |
| Inline Python static scope/copy check: Studio <=800, removing only new locale rows equals base dictionary bytes, original three field IDs and Dialog callbacks exist, view has no useState/useEffect; SHA256 | 0; STATIC_SCOPE_AND_COPY_OK, 791 lines |

Read-only exploratory commands also encountered absent root-tree C4 paths and absent compiler symlink targets, and one tasks JSON assumed-object query exited1 (actual file is a list); corrected reads used the assigned worktree and list schema. No failure is represented as successful typecheck. No dependency install or cache mutation.
E1 structure/static only. All current-candidate typecheck/build, browser/Go/PG, real focus/navigation/zero-write/reload and independent acceptance are NOT_RUN. Existing source RED remains historical evidence, not candidate GREEN.

## Required acceptance and unresolved items

Root must run original three REAL_PG dirty cases to their end, preserving all Keep/Escape focus, Discard/re-entry/reload/clean Cancel/zero-write assertions; preserve old UNKNOWN/recovery/session-boundary and all browser gates required by shared i18n (not narrowed to these three). Save case104 final route remains UNKNOWN, with no wait/expectation change here. Console duplicate-fragment behavior is outside this card. Actual native focus, same-session transition, no extra writes and all guard race paths remain runtime-unverified. Candidate cannot be merged based on author E1.
No tests, fixtures, Go/CAS/router/SessionTasks/sharedUI/dependencies/global tokens, ownership helpers or gate thresholds were changed. No process/server/container/port/runtime/provider/CI/push/merge/deploy started; no task fixture/temp cleanup needed. Author lock remains with root for handoff; there is no third blind product repair authorized.

## Source hashes

- apps/admin/components/Studio.tsx: 1dd732eb88ebd57cee6eb05b81be4a67373e65b34fd46c0205d24f1e4eba80e3
- apps/admin/components/live-sessions/SessionMetadataEditor.tsx: 7172d42f2ceff548b6d4a304d0b37069f8a6e052f4c724a2b5d8b9851eb8b05d
- packages/i18n/src/live-sessions.ts: 6043434e95059a30efbcfd6a62beafb2fc21cc9a1b793d838431ad631e0ff892

## Compile blocker correction 1/2 — task cb0c8af9 (2026-11-02)

base_commit:37201d97cf2f5905acdd3603b1a51c954837cd25 (includes unchanged passive route diagnostic candidate). Root owns claim/author lock; own canvascb0c-sub-1. Requested Sol/high; actual telemetry UNKNOWN. This is the first repair of a distinct compile blocker, not another dirty-behavior attempt.
Root's raw005b Gbuild job125360319112, output/integrator/tools/logs/ci-evidence-1238/365-005b-build-1525.raw.log, was read at the exact error and independently hash-verified c64a0ec18a9f5c0037b7caf261d4a44bd7709eaeb7c5b2c2ce6f3e93c8154212. Lines481-519 identify SessionMetadataEditor.tsx:8: ConfirmDialog is absent from the target AppShell module in both client/SSR builds. Historical E1 did not establish compile success; this candidate has no GREEN build yet.
Root cause: I imported a real DS component from the wrong public package entry. packages/ui/package.json maps '.' to src/AppShell.tsx (legacy exports) and './ds' to src/ds/index.ts (ConfirmDialog re-export from Dialog). Existing ReauthDialog.tsx:17 demonstrates the supported DS import. Merely finding the Dialog symbol without following its public package entry was insufficient.
Correction is exactly one import replacement: retain DateControl/Field/FormRow from @live-commerce/ui, import ConfirmDialog from @live-commerce/ui/ds. No shared/global export, props, guard, UI behavior, product state, original test/fixture/gate or passive diagnostic file is changed.
All local named imports in this editor were checked through their actual entry/export/re-export: liveSessionsCopy/Locale via i18n index; legacy DateControl/Field/FormRow via AppShell to Presentation; ConfirmDialog via ds index to Dialog; presentationCopy/studioCopy/AspectRatio/StudioDetail/StudioErrorCode at their modules; displayTime via orders-model to packages/format; CSS row exists. ConfirmDialog open/title/description/cancelLabel/confirmLabel/onCancel/onConfirm match its actual props. ReactNode is the preexisting React public type, with local installed package unavailable; actual dependency type resolution remains NOT_RUN.
Actual commands: targeted rg/sed/cat and raw-log shasum exit0; inline Python package-entry/named-export/props and inverse-import-byte check exit0; git diff --check exit0. The inverse replacement reproduces the entire editor from37201d97 byte-for-byte, so no view/behavior statement changed. Current typecheck/build/browser/Go/PG/CI/independent acceptance NOT_RUN; local compiler remains absent as previously recorded, no install or repeat runtime attempted. Root must run the real build before downstream UI acceptance. No push/runtime/provider/process/fixture action; writer stops after localcommit, root retains author lock.
Corrected editor SHA256: ed15c4f60a3ea7fc0747cf6c7791b1f9d98dfc8efd3f5da2475bc97196f74727
