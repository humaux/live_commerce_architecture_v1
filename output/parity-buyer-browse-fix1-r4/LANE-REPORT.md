# LANE-REPORT — PARITY-BUYER-BROWSE-FIX1-R4

Model: opencode-go/kimi-k2.7-code
Worktree: /Volumes/data/live_commerce_architecture_v1/.worktrees/parity-buyer-browse
Branch: unit/parity-buyer-browse
Base after merge: origin/r3/integration (4501b409c88542dc63182e619cffd819b194f928)
Inspected run: https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41252821638 (head 2a09e7ba5bb08df8c91ae282cf5c8f29179ed07e, merge sha 0bc92297509f80e5f7696e8bb7d7b1e0fcd24fb5)

| failing job | failing test / assertion | class | change (file:line) |
|---|---|---|---|
| `gate (bundle:02 --browser-storefront)` | `TestBrowserStorefront` / `checkBuyerBrowseParity` / `[782:4090] sort-asc-check` (zh-CN), `[782:4143] sort-desc-check` (en): `locator('a[data-sort="..."]').toBeVisible()` failed with strict-mode violation — resolved to 2 `<a data-sort="..." aria-current="true">` elements, one hidden. The signed mobile frames show the active option inside the visible sort-sheet dialog; an un-scoped locator also matched a transient hidden duplicate. | **(a)** | `tests/storefront/buyer-browse-parity.spec.ts:404-436` — mobile sort assertions now scope the active option to the visible `[role="dialog"][aria-label="..."]` (`a[data-sort="..."][aria-current="true"]`); desktop assertions continue to assert `<select name="sort">` carries the active value. |
| `receipts` | `matrix-receipts: RED (1 problem)` — `--browser-storefront` receipt status FAIL | downstream | Resolved by the class (a) fix above. |
| `required` | `gate=failure receipts=failure` | downstream | Resolved by the class (a) fix above. |

## Pre-push checks

- `node scripts/dev/derive-browser-ownership.mjs` → wrote `output/ci-select-backend-browser/browser-ownership.json` (54 modes).
- `node scripts/dev/derive-browser-ownership.mjs --check` → PASS (no output).
- `node scripts/dev/check-evidence-retention.mjs --base origin/r3/integration` → PASS (10 output/ files, 255281 bytes vs origin/r3/integration; ADR-20261031).
- No Go files changed; `gofmt -l` not needed.

No tests, builds, servers or browsers were started per the lane constraints.

## Class (c) known base issues left untouched

- `gate --browser-inbox` race in `tests/admin/inbox-browser-support.ts` — fixed by integrator on `origin/r3/integration` via `a122b3cd6` (PR #356).
- Evidence retention gate baseline from PR #426 / `b90c56c69` — local retention check passes; no raw CI artifacts committed.

## Commits in this batch

- `2a09e7ba5` — prior head (`fix(tests): desktop sort assertions target visible native select (PARITY-BUYER-BROWSE-FIX1-R3)`)
- (this batch) — `fix(tests): scope mobile sort assertions to visible sort-sheet dialog (PARITY-BUYER-BROWSE-FIX1-R4)`

STATUS: DONE
