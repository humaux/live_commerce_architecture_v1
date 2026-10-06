# Ads, attribution and settings presentation migration

- task_id: 564f02eb-56e2-4a12-9009-c999bc63f3cc (`av_ads_settings`)
- base_commit: 7e73a3a799431ff4ff87307b4642230b85fae95e
- branch: unit/admin-visual-ads-settings
- worktree: /Volumes/data/live_commerce_architecture_v1/.worktrees/admin-visual-ads-settings
- role: UI implementation; model: inherited GPT-6 (exact runtime model/effort UNKNOWN)
- contract: frozen Presentation.tsx and AdminPageHeader at base; domain contracts unchanged

Changed paths: apps/admin/components/{Ads,Attribution,SettingsWizard,StorefrontSettings,MetaConnect,CodSettings}.tsx; components/{ads,attribution,settings}.css; apps/admin/lib/attribution-copy.ts; tests/admin/{attribution.test.ts,settings-real.spec.ts}.

Audit evidence read: output/visual-review/REVIEW-admin.md ADM-01–06,11–12,14,16–19,22,24–25,30–31; actual en desktop Ads and Settings, en mobile Attribution and Settings screenshots from .worktrees/ui-visual-audit/output/ui-visual-audit/20261005T045559Z/shots/admin/. Read AGENTS.md, docs/delivery/PROCESS.md, DESIGN.md, architecture sections 0,1,2,7,15,26,27, tasks DAG and invariants. Impeccable audit-first preserved the incumbent Operate design and frozen truth boundaries.

Implementation: registry headings align with shell; header actions use shared slots; Fields/FormRows align date/report, draft schedule, CAPI, domain and COD fields; shared Badge/TableFrame replace local badge/scroll behavior; numeric amount/attempt columns align right; settings sections share white bordered surfaces, compact status layout, right grouped actions and semantic danger buttons. Ads safety notices share one frame; source-specific results remain distinct and keep original safety copy. Timeline has one empty message; wholly empty buyer breakdowns share one message without hiding summary facts. Domain handlers, permissions, journals, command keys and amounts remain unchanged. Tests retain original assertions and execute actual shared presentation JSX; three locale counterexamples cover registry title, field label, scroll region and consolidated empty messages.

Actual commands / exits:

| Command | Exit | Evidence |
| --- | --- | --- |
| node /Users/luolimo/.codex/skills/impeccable/scripts/context.mjs --target apps/admin/components/SettingsWizard.tsx | 0 | tool output, setup context read |
| pnpm install --offline --frozen-lockfile | 0 | tool output: reused 49, downloaded 0 |
| pnpm exec prettier --write [owned TSX/CSS/copy/test paths] | 0 | tool output; unrelated formatting subsequently reduced |
| pnpm typecheck:admin | 0 | typecheck.log |
| node --test --experimental-strip-types tests/admin/ads-model.test.ts tests/admin/ads-request.test.ts tests/admin/attribution.test.ts tests/admin/attribution-audience.test.ts tests/admin/attribution-format.test.ts | 1 initially, then 0 | INITIAL_FAILURE.md; final node-tests.log: 61 pass, 0 fail, 0 skip |
| git diff --check | 0 | tool output; no whitespace errors |

NOT_RUN: builds, browser/click ledger, PG, LIVE, provider, independent review. Parent serializes these gates. Native dates have locale attributes but browser-native localized rendering remains UNKNOWN; parent 07f00fd1 supplies DateControl for post-integration migration. Disabled reasons retain existing real states and hints; complete click/disabled matrix is NOT_RUN. No server, fixture, container or browser process was started by this unit.

This is a source/MOCK handoff, not a visual or release PASS. Main checkout paths and shared UI/global CSS were not edited.
