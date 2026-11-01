# RVFIX2-440 lane report (push 1, tests only)

| Finding | Class | Change |
|---|---|---|
| P1 hard-coded navy overrides tenant accent | b (test asserted wrong colour) | tests/storefront/buyer-browse-parity.spec.ts:~422 asserts add-to-cart background == resolved page `--accent` (in-page measurement); fails on current navy `--action`. CSS fix NOT included (relay pushes it after red). |

RED pushed. Merged origin/r3/integration first. Browser ownership regenerated. Nothing run locally.

STATUS: DONE
