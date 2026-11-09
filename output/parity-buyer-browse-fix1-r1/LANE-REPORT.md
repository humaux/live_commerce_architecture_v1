# LANE-REPORT — PARITY-BUYER-BROWSE-FIX1-R1

Model: opencode-go/kimi-k2.7-code
Worktree: /Volumes/data/live_commerce_architecture_v1/.worktrees/parity-buyer-browse
Branch: unit/parity-buyer-browse
Base after merge: origin/r3/integration (includes integrator commit a122b3cd6 for inbox race and ADR-20261031 evidence-retention baseline).
Inspected run: https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41241504659

| failing job | failing test / assertion | class | change (file:line) |
|-------------|--------------------------|-------|--------------------|
| `gate (--browser-click-sweep@4/10)` | `NEW FAIL storefront /pages/contact mobile/zh-TW [page] "(coverage)" coverage-missing: no page-load row was written` | (b) | Added `/pages/contact` to `tests/ui/sweep-shard-lib.mjs:16`; updated storefront route count assertion to 18 in `tests/ui/sweep-shard-lib.test.mjs:52`. |
| `gate (--browser-click-sweep@7/10)` | `NEW FAIL storefront /pages/contact desktop/en [page] "(coverage)" coverage-missing` | (b) | Same `tests/ui/sweep-shard-lib.mjs:16` fix as shard 4/10. |
| `gate (--browser-click-sweep@8/10)` | `NEW FAIL storefront /pages/contact desktop/zh-TW [page] "(coverage)" coverage-missing` | (b) | Same `tests/ui/sweep-shard-lib.mjs:16` fix as shard 4/10. |
| `gate (--browser-webkit)` | `FAIL: order (go test exit=1)` — `ExpectError: getByTestId('cart-empty') expected "Loading…", received ""` | (a) | Rewrote loading-state assertion in `tests/storefront/order-gate.mjs:353` to check the signed-frame skeleton (`[aria-busy="true"]` with `aria-label=shopCopy.en.loading`); updated the BC02 measurement sampler (`tests/storefront/order-gate.mjs:287-296`) and the mount-phase click target (`tests/storefront/order-gate.mjs:363`) and post-failure assertion (`tests/storefront/order-gate.mjs:387`) to match the skeleton. |
| `gate (bundle:02)` | `--browser-order`: `TestBrowserBuyerOrderUI` FAIL — same cart-empty "Loading…" expectation as webkit order | (a) | Same `tests/storefront/order-gate.mjs` changes as above. |
| `gate (bundle:02)` | `--browser-storefront`: `ExpectError: getByRole('heading', { level: 1 }) expected "Tableware", received "Daily Series"` | (a) | Rewrote collection-page heading assertion in `tests/storefront/shop-real-gate.mjs:168` to expect `COPY.dailySeries` (the signed PARITY-BUYER-BROWSE series title). |
| `aggregate` | `G-UI8 click sweep aggregate: FAIL` — 3 missing contact page rows | (b) | Resolved by the `tests/ui/sweep-shard-lib.mjs:16` route fix above. |
| `receipts` | `matrix-receipts: RED (6 problem(s))` — click-sweep 4/7/8, order, storefront, webkit receipts are FAIL | downstream | Resolved by the class (a) and (b) fixes above. |
| `required` | `plan=success build=success gate=failure aggregate=failure receipts=failure` | downstream | Resolved by the class (a) and (b) fixes above. |

## Pre-push checks

- `node scripts/dev/derive-browser-ownership.mjs` → wrote `output/ci-select-backend-browser/browser-ownership.json` (54 modes).
- `node scripts/dev/derive-browser-ownership.mjs --check` → PASS.
- `node scripts/dev/check-evidence-retention.mjs --base origin/r3/integration` → PASS (6 output/ files, 242974 bytes).
- No Go files changed; `gofmt -l` not needed.

## Class (c) known base issues left untouched

- `gate --browser-inbox` race in `tests/admin/inbox-browser-support.ts` — fixed by integrator on `origin/r3/integration` via `a122b3cd6` (PR #356).
- Evidence retention gate baseline from PR #426 / `b90c56c69` — local retention check passes; no raw CI artifacts committed.

STATUS: DONE
