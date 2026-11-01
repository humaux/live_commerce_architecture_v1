# RVFIX-370 lane report (push 1: tests only)

| Finding / job | Class | What changed | Where |
|---|---|---|---|
| Merge origin/r3/integration | — | Merged; conflicts in `packages/i18n/src/index.ts` (header comment, combined) and `browser-ownership.json` (took theirs, then regenerated) | merge commit 27e2fc1f9 |
| P1-1 card state in Advanced | RED test | Assertions that badge, platform badge, not-open/blocked notes, Enable/Disable are visible before opening Advanced | tests/admin/card-payments.spec.ts CPU5, CPU6, CPU6b, CPU7b |
| P1-2 reversed range dead end | RED test | New test: reversed-range URL shows range-invalid alert, no "not found", date editor opens by click, fix applies and survives reload | tests/admin/customers-billing.spec.ts (after CB11 finance) |
| P2-3 Settlements legend | RED test | Restored `settlements-legend`/`settlements-formula` assertions verbatim from origin/r3/integration (STL1 list, STL2 detail) | tests/admin/settlements.spec.ts STL1, STL2 |
| P2-7 `generated_at` optional | RED test | Unit case: summary without `generated_at` parses; bad value still fails | tests/admin/customers-model.test.ts |
| red.log | record | Expected failures recorded; run/job ids to be added by the relay after CI | output/parity-money/red.log |
| Ownership | derived | `derive-browser-ownership.mjs` run and `--check` clean | output/ci-select-backend-browser/browser-ownership.json |

Product code is intentionally unchanged (RED push). Implementation for P1-1, P1-2, P2-3, P2-7 is for the next push after CI shows red; planned changes are in `output/parity-money/RVFIX.md`.
Nothing was run locally (owner rule).

STATUS: DONE
