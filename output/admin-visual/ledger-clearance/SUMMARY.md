# Legacy ledger clearance and scrollbar contract source handoff

- task_id: `29b75773-01f5-4fb1-b307-56103f85cc51`
- base_commit: `1ded587749e5718f78c3fff1e2233a0ffd7321c7`
- branch/worktree: `unit/admin-visual-ledger-clearance`, `.worktrees/admin-visual-ledger-clearance`
- role: scoped UI implementation/test contract; agent `av_ads_settings`; inherited runtime model/reasoning identifier UNKNOWN.
- allowed source changes: inspector padding declarations in `apps/admin/app/globals.css`; exact scrollbar RGB expectation in `tests/admin/visual-states.spec.ts`.

## Audit and root cause

Reviewed the root legacy run log and failure screenshot first: `../admin-visual/output/playwright/admin-ledger-20261005T065802.813835000/playwright.log` and its inventory-tray failure PNG. The screenshot shows the initial inventory viewport; the offscreen tray overlap is established by the actual geometry assertion, not visually claimed from that PNG. Log: `button.close-details[] overlaps h2[Adjust inventory] (5x21)`. Existing inspector padding reserved 44px, while the 44px close target begins 5px inside the right edge. That requires at least 49px before adding a content gap.

Desktop and mobile inspector padding now reserve `calc(var(--ui-target) + 16px)` (60px at the frozen 44px target), covering the 5px inset and leaving 11px clearance. Close control dimensions and selectors are unchanged. No overlap threshold changed.

The same root run reports computed scrollbar `rgb(86, 97, 113) rgb(245, 246, 248)`, matching frozen `--ui-muted: #566171` and `--ui-page: #f5f6f8`. The test expectation now matches these approved tokens exactly. No source palette change. The thin scrollbar, overflowWidth > 0, actual wheel, scrollLeft > 0 and every other assertion are byte-identical to base, verified by a complete-file comparison permitting only this literal replacement.

Impeccable audit/craft-floor guidance used for bounded real geometry clearance and incumbent palette preservation; no visual-world redesign.

## Actual commands and exits

| Command | Exit | Evidence |
| --- | --- | --- |
| `git worktree add -b unit/admin-visual-ledger-clearance ... 1ded587749e5718f78c3fff1e2233a0ffd7321c7` | 0 | Tool output `405726` |
| `pnpm install --offline --frozen-lockfile` | 0 | Tool output `400d34`, reused 49/downloaded 0 |
| `pnpm typecheck:admin` | 0 | `typecheck.log` |
| `node --test tests/admin/shell-architecture.test.mjs` | 0 | `architecture.log`, 2 pass/0 fail/0 skip |
| Node assertion comparing complete visual-states file against base with only canonical RGB replacement | 0 | `assertion-preservation.log` |
| `git diff --check` | 0 | Tool output `c2947b` |

## Boundaries

Browser/build/PG/runtime geometry verification: NOT_RUN by author; root owns independent legacy rerun. No source/backend/receipt behavior changes beyond the two inspector clearance declarations. No services started. No existing test/gate loosened. This static handoff does not claim runtime GREEN.
