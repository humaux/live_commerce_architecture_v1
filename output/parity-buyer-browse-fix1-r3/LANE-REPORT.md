# LANE-REPORT — PARITY-BUYER-BROWSE-FIX1-R3

Model: opencode-go/kimi-k2.7-code
Worktree: /Volumes/data/live_commerce_architecture_v1/.worktrees/parity-buyer-browse
Branch: unit/parity-buyer-browse
Base after merge: origin/r3/integration (4501b409c88542dc63182e619cffd819b194f928)
Inspected run: https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41249740186 (head fa84c559cc44418951b0f6ec504d2f498645283a)

| failing job | failing test / assertion | class | change (file:line) |
|---|---|---|---|
| `gate (bundle:02 --browser-storefront)` | `TestBrowserStorefront` / `checkBuyerBrowseParity` / `[782:3907] sort-asc-check`, `[782:3967] sort-desc-check`, `[782:4027] sort-title-check` (3 locales each = 9 FAILs): `locator('a[data-sort="..."]').toBeVisible()` received `hidden` on desktop because `.sf-sort-sheet` is `display: none` outside the mobile breakpoint | **(a)** | `tests/storefront/buyer-browse-parity.spec.ts:404-436` — desktop signed frames assert the visible native `<select name="sort">` carries the active sort value; mobile signed frames keep asserting the visible sheet option. |
| `receipts` | `matrix-receipts: RED (1 problem)` — `--browser-storefront` receipt status FAIL | downstream | Resolved by the class (a) fix above. |
| `required` | `gate=failure receipts=failure` | downstream | Resolved by the class (a) fix above. |

## Pre-push checks

- `node scripts/dev/derive-browser-ownership.mjs` → wrote `output/ci-select-backend-browser/browser-ownership.json` (54 modes).
- `node scripts/dev/derive-browser-ownership.mjs --check` → PASS (no output).
- `node scripts/dev/check-evidence-retention.mjs --base origin/r3/integration` → PASS (9 output/ files, 253428 bytes vs origin/r3/integration; ADR-20261031).
- No Go files changed; `gofmt -l` not needed.

No tests, builds, servers or browsers were started per the lane constraints.

## Class (c) known base issues left untouched

- `gate --browser-inbox` race in `tests/admin/inbox-browser-support.ts` — fixed by integrator on `origin/r3/integration` via `a122b3cd6` (PR #356).
- Evidence retention gate baseline from PR #426 / `b90c56c69` — local retention check passes; no raw CI artifacts committed.

## Commits

- `fa84c559c` — `fix(tests): align signed-frame assertions and coverage for PARITY-BUYER-BROWSE-FIX1-R2`
- (this batch) — `fix(tests): desktop sort assertions target visible native select (PARITY-BUYER-BROWSE-FIX1-R3)`

STATUS: DONE
