# Ledger presentation extraction — static handoff

- task_id: `56b4f739-df8a-433b-8335-c6be07bf0eb9`
- base_commit: `de6430772c40ab973ae957de37a9f2cf17166cd1`
- branch: `unit/admin-visual-ledger`
- role: ui_worker responsibility; agent `av_ads_settings`; model/reasoning inherited (runtime identifier UNKNOWN).
- worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/admin-visual-ledger`
- changed source: `apps/admin/components/Ledger.tsx`, `apps/admin/components/LedgerTable.tsx`

## Implementation

The inventory table is now a pure component with explicit typed data and callback props. Ledger retains all fetching, state, operation, journal and version-fence logic. Product-name/edit selection calls the existing select callback only; the radio callback retains the existing delta/reason/error reset. The moved table preserves product photographs, names, status badges, empty/error/session presentation and selectors. `TableFrame` still receives `scrollClassName="table-scroll"`, which places that selector on the real overflow node. The existing radio-label markup is unchanged. Existing money formatting is reused; no formatter or inventory calculation was introduced.

The architecture line ceiling is fixed by a meaningful component boundary, not a threshold/allowlist change or compressed formatting. Current line counts: Ledger 756; LedgerTable 151.

## Actual commands

| Command | Exit | Evidence |
| --- | --- | --- |
| `pnpm install --offline --frozen-lockfile` | 0 | Tool output: reused 49, downloaded 0 |
| `pnpm exec prettier --write apps/admin/components/LedgerTable.tsx` | 0 | Tool output |
| `pnpm typecheck:admin` | 0 | `typecheck.log` |
| `node --test tests/admin/shell-architecture.test.mjs` | 0 | `architecture.log`: 2 pass, 0 fail, 0 skip |
| `git diff --check` | 0 | Tool output: clean |
| `wc -l apps/admin/components/Ledger.tsx apps/admin/components/LedgerTable.tsx` | 0 | Tool output: 756 + 151 |

## Boundaries

Browser, screenshot, build, PG and runtime interaction verification: NOT_RUN, reserved for root's serialized verification. No backend/API/write behavior changes, test edits or gate relaxations. Author-only static checks are not independent visual acceptance.
