# Meaningful panel split for G-UI5

- task_id: 30cf3661-56e6-4c58-863f-057eca085eac
- base_commit: 99dfb1827f9e868c875e4f54cfc07fdcc83b540f
- worktree: /Volumes/data/live_commerce_architecture_v1/.worktrees/admin-visual-ads-settings
- branch: unit/admin-visual-ads-settings
- role: UI implementation; inherited GPT-6; exact runtime model/effort UNKNOWN

Root integrated source reported Ads 1302 lines and Attribution 813 lines exceeding the unchanged 800-line G-UI5 ceiling. Split by existing responsibilities, without new behavior: Ads retains command/journal/page orchestration; AdsConnection owns connect/pick presentation; AdsDraft owns draft editor/detail/status; AdsResults owns results/CAPI presentation; Attribution retains report loading/navigation; AttributionPanels owns complete report/table/empty-state presentation. Date formatting remains the unchanged page function, passed as a prop. Integer report counts use existing packages/format decimal(locale,n,0), not a duplicate Intl formatter. No thresholds, immutable basis or allowlist changed. SSR tests load the actual extracted AttributionPanels; all original domain assertions remain.

Changed paths: components/Ads.tsx, Attribution.tsx; new AdsConnection.tsx, AdsDraft.tsx, AdsResults.tsx, AttributionPanels.tsx; tests/admin/attribution.test.ts; own evidence.

Actual final commands:

| Command | Exit | Evidence |
| --- | --- | --- |
| pnpm exec prettier --write apps/admin/components/AdsConnection.tsx apps/admin/components/AdsDraft.tsx apps/admin/components/AdsResults.tsx apps/admin/components/AttributionPanels.tsx | 0 | tool output |
| pnpm typecheck:admin | 0 | split-typecheck.log |
| node --test tests/admin/shell-architecture.test.mjs | 0 | split-architecture.log: 2 pass, 0 fail |
| node --test --experimental-strip-types tests/admin/ads-model.test.ts tests/admin/ads-request.test.ts tests/admin/attribution.test.ts tests/admin/attribution-audience.test.ts tests/admin/attribution-format.test.ts | 0 | split-node.log: 61 pass, 0 fail, 0 skip |
| git diff --check | 0 | tool output |
| wc -l apps/admin/components/{Ads,Attribution,AdsConnection,AdsDraft,AdsResults,AttributionPanels}.tsx | 0 | 516 / 315 / 324 / 557 / 389 / 511 lines |

Initial split typecheck exit 1: moved PickStep used FormEvent without its type import. Added type FormEvent to AdsConnection; final exit 0. Initial full log NOT_SAVED; diagnostic was visible in tool output. No acceptance weakened.

Root DateControl relocation: 27baa044 is not cherry-picked because it edits other owned files and this worktree lacks its shared DateControl prerequisite. Parent must retain its existing attribution-from/to DateControl edits in Attribution.tsx; move its ads-f-starts/ends DateControl + presentationCopy imports to AdsDraft.tsx; move ads-report-from/to DateControl + presentationCopy imports to AdsResults.tsx. Local source retains native inputs/lang attributes from base. This is an explicit integration follow-up, not a date-localization PASS.

NOT_RUN: browser/build/PG/LIVE and independent review; root serializes visual/click gates. No processes or fixtures started by this unit.
