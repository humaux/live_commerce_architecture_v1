# PR365 C4 product repair 1/2 — static candidate

task_id: bb248283-737c-46ad-9981-c70e2c9522df / bb248283-sub-1.
base_commit: fb9b43a1678569398653ddf2409a99b0f0f964c6.
worktree: .worktrees/codex-pr365-mock-balance; branch: codex/pr365-mock-balance.
role: sole delegated UI writer; requested Sol/high; actual model/reasoning telemetry UNKNOWN.
Root retains the author lock. No separate claim, delegation, push, merge, runtime, build, browser or provider execution.

## Authority and observed RED

Human freeze: Humaux d272d041-21c0-41f5-ac34-fa44f8d17e73. SOURCE: d1e0b80a-d877-42be-a96d-81487608f044. Exact strings remain authoritative only in root's review-c4-backfix2-delta.md, SHA256 fde0412dcbadc9e044ae19ef4d0335fd004aec376ea44322a7cc978a3b2f3ad8. No new Figma design or token change.

Pre-repair own Gates41440918464 actually executed 27c2eaa31aac1f2b0f3d0600526312d744c8a58e, not its PR metadata SHA. Root diagnostic output/integrator/tools/logs/ci-evidence-1238/365-c4-failure-1225/REPORT.md SHA256 e35faf3988ce27278be620f56b5f4b4a4be63e914d5c578afc09427dac4fff08, case-matrix.json SHA256 a694907e2632fd9b77176e2b961c9de7702dd17464d31a604db685353d6b7545. These existing E3 failure receipts establish seven Console product REDs, five setup-blocked cases and three readonly PASS, zero formal skips. They are not repaired-candidate GREEN evidence.

## Separate commits and root causes

1. Setup-only ef03c041a: new C4 owned-draft helper expected201 although scopedAs internal/httpapi/handler.go387-420 returns HTTP.StatusOK and actual creation returned200. Change only exact201→200. UNKNOWN zhCN's unavailable nav-orders assumed a grant the fixture lacks; replace only that new departure attempt with native visible Live-group expansion and visible Studio click, retaining all departure/Retry/input/key/body/effect assertions. No permission/product/fixture change.
2. Product candidate: SessionTasks had no native Edit action, no frozen success statement/notice, and an unguarded Back anchor. Studio's copyNavigationGuard was never registered. Bridge the existing child command.blocked plus readiness through an identity-safe mount/cleanup registration and latest ref, and route Back through existing parent navigate/mayLeave. Separate native Edit in preparation and verified copied result uses the same existing parent navigation to current scene#edit. Success requires the parsed CopyResult matching the currently read new detail and a ready, unblocked command; hash-only/pending/UNKNOWN cannot produce the new success strings. Show exact unconfirmed label/help alongside original Retry and input. Existing View-settings/Open-preparation claims#preflight and returned copied ID remain intact.

Only four dictionary additions across three locales; existing labels and command/request/journal/API code unchanged. No metadata save/dirty-dialog repair is claimed before newly reachable runtime evidence. Parent owns can_manage/DRAFT, CAS, session boundary, store and selected identity. Child navigation observes the original journal, never clears it or creates a second ledger/key/body. Studio uses 800 lines under the actual source.split("\n").length guard (wc reports799); no extraction or threshold change.

## Changed paths

- tests/admin/studio-ui2-c4-metadata.spec.ts (setup commit only)
- tests/admin/live-console.spec.ts (setup commit only)
- apps/admin/components/live-sessions/SessionTasks.tsx
- apps/admin/components/Studio.tsx
- packages/i18n/src/live-sessions.ts
- output/parity-live-sessions/C4-PRODUCT-FIX1.md (this report)

## Executed static commands and exits

- pwd; git status --short; git rev-parse HEAD: exit0, clean at exact base initially.
- Targeted rg/sed/cat/git source reads and frozen Humaux get/rejected enumeration: shell exit0, rejected9/9 coverage1.0. One shell wildcard inventory reported no matches and was replaced by rg --files; no source conclusion relies on it.
- Python exact two-setup replacements: exit0; git diff --check: exit0.
- git add tests/admin/live-console.spec.ts tests/admin/studio-ui2-c4-metadata.spec.ts; git -c core.hooksPath=/dev/null commit -m 'test(live-sessions): correct C4 setup status and authorized shell target': exit0 (ef03c041a). Hooks deliberately disabled under root's no-runtime/no-build task card, not claimed as passed.
- Initial node parser using require("typescript-api"): exit1 MODULE_NOT_FOUND; no install attempted. Existing pinned Next16.3.5 bundled Babel parser fallback below: exit0 for all five changed TS/TSX sources, syntax only, no emit/type/runtime check.
- Python normalized byte comparisons against git show fb9b43a:<path>, retained fixture/command sources, Studio535 prefix, source SHA256 and actual800 line limit: exit0. This is source integrity evidence, not a runtime gate.
- git diff --check after product patch: exit0.

Syntax command actually executed (existing dependency only):

```sh
node <<'JS'
const fs = require('node:fs');
const parser = require('/Volumes/data/live_commerce_architecture_v1/node_modules/.pnpm/next@16.3.5_@types+node@24.13.6_react-dom@19.3.0_react@19.3.0__react@19.3.0/node_modules/next/dist/compiled/babel/parser.js');
for (const path of ['apps/admin/components/Studio.tsx', 'apps/admin/components/live-sessions/SessionTasks.tsx', 'packages/i18n/src/live-sessions.ts', 'tests/admin/live-console.spec.ts', 'tests/admin/studio-ui2-c4-metadata.spec.ts']) {
  parser.parse(fs.readFileSync(path, 'utf8'), {sourceType:'module', plugins:['typescript', ...(path.endsWith('.tsx') ? ['jsx'] : [])]});
  console.log('syntax OK', path);
}
JS
```

## Retained oracle integrity

The entire studio-ui2 C4 spec equals base byte-for-byte after reversing its single exact status replacement. The entire live-console spec equals base byte-for-byte after reversing only the new native shell-target block. Therefore every other existing assertion, key/body/Retry/effect/count oracle and prior case is unchanged. Original Studio535 prefix is the entire old Studio spec and remains byte-identical. No fixture,9/9/focused2/4/fault3, ownership metadata, helper2/2 or timeout edits.

| Unchanged source | SHA256 |
|---|---|
| tests/admin/studio-ui.spec.ts + original535 prefix | c53bd85544b1541f3bb259aab084ab0a39a12fa13acd94d1099de48bddee5217 |
| tests/foundation/browser_studio_ui_test.go | 97683286c4f1fd407c7eb9b8969975fd736bc9299238f59fd8d7d3ded0058332 |
| tests/foundation/browser_live_console_test.go | 530cdef17dc86912b9133fefe0c095660ea119129ed51d77815bbaa79ced6859 |
| apps/admin/src/features/live/use-live-workspace.ts | fcb3176a7398d70afdb4b21351d88dea9bead7b859b3c17e09aaf7d404fe2893 |
| apps/admin/src/features/live/command-journal.ts | 7e2e6ee6748a3e5d18a06fef6abe982309760be0f39048648aa452fb87f6c67c |
| apps/admin/src/features/live/console-client.ts | d4fe31b0c6d994a9398199a3500943d33d5d2d1fdfd480b1fd91b14a71ade444 |
| apps/admin/src/features/live/console-model.ts | e48d3b5eb1420882f6eb7c83755e7e65667c25b5b6c7f2788608775b97c50329 |

## Candidate file binding

| Source | SHA256 |
|---|---|
| apps/admin/components/Studio.tsx | 5858468e0a228092265c06a80dfbaaa07be86231fa2caef14b167cfe81b9b8dc |
| apps/admin/components/live-sessions/SessionTasks.tsx | 0c65ee4deadc5ae2f4598af08d87a0defea91b628386bbc4992f1b8cf3daa892 |
| packages/i18n/src/live-sessions.ts | dd787a8e5b8d01440dff878621df2cd7fcdd660b6726f90c7e1a2faddd8aa375 |
| tests/admin/live-console.spec.ts | 3adccb9b34e3dd9f1a732896d296eff3c9db5c3d1c5a42bfdb6db7196f24cd76 |
| tests/admin/studio-ui2-c4-metadata.spec.ts | da2d9489e579cb882b7b1d3c96a4196432cb84e89ea2207f5cc96d9198f20056 |

## Required acceptance and unresolved risks

E1 static syntax/source integrity only; product GREEN, full strict typecheck, architecture gate execution, metadata-save/dirty-dialog reachability, reload/history/recovery behavior and shell fixture reachability NOT_RUN. Whole PR acceptance requires same own-execution-bound SHA all selected CI, unchanged seven original product cases plus newly unblocked cases and all legacy cases; shared i18n must select all required native registry modes, not a narrowed Console subset. Root must perform derivation/ownership work and independent high-risk/cross-family/root review; this author does not approve their change. Figma native links/tokenDiff, provider SANDBOX/LIVE and production NOT_RUN.

No process, server, port, browser, fixture or container started. No temporary file or dependency cache created or removed. Failure evidence and all other task paths retained. No wait/background runtime remains. Product repair iteration1/2 candidate, not feature completion or release approval.
