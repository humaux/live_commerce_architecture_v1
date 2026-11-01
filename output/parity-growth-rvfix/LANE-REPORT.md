# LANE-REPORT RVFIX-366 push 1 (tests only)

| finding | class | change |
|---|---|---|
| P1-1 | a (real defect, RED test added) | tests/storefront/promotions-gate.mjs `growthEnabledAndDeepLink`: enabled store + draft must show `ads-draft-<id>`, not g.off/g.noDrafts |
| P2-1 | a | same function: `/promotions?view=codes` shows the codes manager after goto and after reload |

Merged origin/r3/integration (ownership json conflict resolved by regenerating). No tests run (CI only).

STATUS: DONE RED pushed
