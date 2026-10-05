# Mobile real targets and Ledger column access — source handoff

- task_id: `94b8f6e9-5176-469b-b538-5f57ffd05545`
- base_commit: `1ded587749e5718f78c3fff1e2233a0ffd7321c7`
- worktree/branch: `.worktrees/admin-visual-mobile-targets`, `unit/admin-visual-mobile-targets`
- agent/role: `av_ads_settings`, scoped UI implementation; inherited runtime model/reasoning identifier UNKNOWN.
- source path: `apps/admin/app/globals.css` only. No component, test, DOM, state or API edits.

## Measured before

Frozen audit: root `.worktrees/admin-visual/output/ui-visual-audit/20261005T063819Z/lint.json`, recorded source `5a895dea2c`. Direct JSON inspection confirms **40 admin R5 instances**, five unique shapes across four routes and three locales:

| Actual surface | Recorded height | Instances |
| --- | --- | --- |
| Ads CAPI label `.ads-check` | 24px | 3 |
| Customer order table anchor | 15px | 3 |
| Dashboard latest order table anchor | 16px | 28 |
| Dashboard All orders anchor | 15px | 3 |
| Settings storefront origin anchor | 16px | 3 |

Representative original crop files viewed: `crops/admin/ads/en-390x844-R5-1.png`, `crops/admin/home/en-390x844-R5-2.png`. Root-reviewed ADM-29 also identifies hidden inventory stock/status/action columns and duplicate SKU sublines. The currently available frozen inventory `shots/admin/inventory/zh-TW-390x844.png` was viewed: horizontal cue exists, but hidden columns are not recoverable through scrolling. The initially specified older audit worktree path is absent; its image could not be loaded, so no direct visual claim is made from it.

## Implementation

At <=680px, only the five exact clickable label/anchor shapes receive token-based 44px min-height/min-width on the actual interactive element. Text anchors use inline flex alignment; the storefront origin link retains full-row flex layout and inherited wrapping. No absolute/pseudo overlay, JS or noninteractive span styling.

At <=900px, Ledger's existing `.table-scroll` gets a table min-width of 760px and restores SKU, stock, status and action cells. The duplicate `.mobile-sku` subline is hidden. Existing TableFrame owns the actual horizontal scrollport and localized cue; DOM, projections, status and selection callbacks remain untouched. Other tables are not changed by the Ledger-specific selectors. Frozen ledger tests retain page-overflow/geometry checks; catalog-media retains attached state checks. No assertions or test fixtures changed. Parent separately owns status compatibility classes (`1b82e955`).

The independent inspector clearance commit `1f656977` is not included or altered. Removing just these two CSS additions restores the whole base globals file byte-for-byte.

Impeccable audit/craft-floor used to preserve the incumbent palette and expand real click surfaces rather than decorative hit areas. This is a source handoff, not a runtime lint-clear verdict.

## Actual commands

| Command | Exit | Evidence |
| --- | --- | --- |
| explicit `git worktree add -b unit/admin-visual-mobile-targets ... 1ded5877` | 0 | Tool `0d620e` |
| `pnpm install --offline --frozen-lockfile` | 0 | Tool `d09f94`, reused49/downloaded0 |
| `pnpm typecheck:admin` after both additions | 0 | `typecheck.log` |
| `node --test tests/admin/shell-architecture.test.mjs` after both additions | 0 | `architecture.log`, 2 pass/0 fail/0 skip |
| Node/Prettier CSS parse + complete-file removal/preservation comparison | 0 | `source-preservation.log` |
| `git diff --check` | 0 | Tool `480d7a` |

Targeted source reads initially referenced nonexistent `dashboard.css`/`MerchantTasks.css` (rg exit2), then corrected to actual Dashboard/source files. Initial exact `ADM29` search did not match `ADM-29` (exit1), then corrected. No install dependencies changed. No processes started.

## Pending independent acceptance

Root owns the frozen lint rerun and affected legacy/ads/customers/home/settings/catalog-media browser gates. Browser, build, PG and runtime column reachability/tap geometry: NOT_RUN by author. Risk: wider Ledger content and taller mobile table anchors require the existing no-page-overflow/row-consistency/real-click verification; no runtime PASS is asserted here.
