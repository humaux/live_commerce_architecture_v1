# LANE-REPORT — PARITY-BUYER-BROWSE-FIX1

Model: opencode-go/kimi-k2.7-code
Worktree: /Volumes/data/live_commerce_architecture_v1/.worktrees/parity-buyer-browse
Branch: unit/parity-buyer-browse
Base after merge: origin/r3/integration (includes integrator commit a122b3cd6 for inbox race and ADR-20261031 evidence retention baseline).

| failing job | failing test / assertion | class | change (file:line) |
|-------------|--------------------------|-------|--------------------|
| `build` | `Next builds` → `app/[locale]/search/page.tsx(31,38): error TS2339: Property 'searchQuery' does not exist on type 'Copy'.` | (b) | Added `searchQuery: "Tote"` to `apps/storefront/lib/shop-copy.ts:52`; added `searchQuery: "托特包"` to `apps/storefront/lib/shop-copy.ts:192` and `:331`. |
| `foundation` | `G-UI8 storefront pages and the sweep's STOREFRONT_ROUTES are the same set` — actual set contains `/pages/contact`, expected set does not. | (b) | Added `/pages/contact` to `tests/ui/sweep-shard-lib.mjs:16`; updated storefront route count assertion from 17 to 18 in `tests/ui/sweep-shard-lib.test.mjs:52`. |
| `aggregate` | `G-UI8 click sweep aggregate: FAIL` / `G-UI9 visual lint aggregate: FAIL` — no shard evidence because upstream build failed. | (b) | Resolved by `build` fix; no direct product change. |
| `receipts` | `matrix-receipts: RED (77 problem(s))` — every planned mode reports `no receipt` because gate was skipped after build failure. | (b) | Resolved by `build`/`foundation` fixes; no direct product change. |
| `required` | Required-job gate: `build=failure gate=skipped aggregate=failure receipts=failure`. | (b) | Resolved by `build`/`foundation` fixes; no direct product change. |

## Pre-push checks

- `node scripts/dev/derive-browser-ownership.mjs` → wrote `output/ci-select-backend-browser/browser-ownership.json` (54 modes).
- `node scripts/dev/derive-browser-ownership.mjs --check` → PASS.
- `node scripts/dev/check-evidence-retention.mjs --base origin/r3/integration` → PASS.
- `gofmt -l` on changed Go files → no output.

## Class (c) known base issues left untouched

- `gate --browser-inbox` race in `tests/admin/inbox-browser-support.ts` — fixed by integrator on `origin/r3/integration` via `a122b3cd6` (PR #356).
- Evidence retention gate baseline from PR #426 / `b90c56c69` — local retention check passes; no raw CI artifacts committed.

STATUS: DONE
