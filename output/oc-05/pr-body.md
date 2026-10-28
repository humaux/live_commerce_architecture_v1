OC-05 adds the server-owned quick-start marker for a store. Migration 0205 backfills existing stores as completed; new stores derive the name/shipping/product steps from real data and can complete or dismiss the checklist through the authenticated admin route. OC-06 UI is excluded.

Base: 79f95784c9dee419346fe3119d4d4f56720d4a3c. Target: r3/integration. No merge or auto-merge authorization for this worker.

RED first: this initial commit contains only the PostgreSQL/HTTP regression tests and the mandated generated package dependency entry. Expected failures are the missing GET/POST behavior and absent assigned migration. Run/job receipts will be recorded in output/oc-05/red.log before implementation is pushed.

Checks: no local tests or servers. Static formatting and dependency-map regeneration only. CI verification pending; independent review NOT_RUN; author self-assessment E0.

Write paths outside the assigned card: none. R2 pin 135 and API mount/contract changes are explicitly granted in the dispatch card.

Risk/rollback: a server-only additive table; no provider calls, money flow, customer data or UI changes. Revert the code through an integrator-reviewed PR; keep migrated marker data, with any schema reversal separately reviewed. No production database action.

🤖 Generated with Codex (dispatched by Claude Code integrator).
