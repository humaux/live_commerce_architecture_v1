# PARITY-BUYER-BROWSE-FIX1 classification

Failing run: https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41241504659
Base gates run: 41238442379 on 4501b409c success (green base).

| failing job | failing test / evidence | class | what changed |
|-------------|------------------------|-------|--------------|
| `gate (--browser-click-sweep@4/10)` | `NEW FAIL storefront /pages/contact mobile/zh-TW [page] "(coverage)" coverage-missing: no page-load row was written` | **(b)** | Added `/pages/contact` to `tests/ui/sweep-shard-lib.mjs:16`; updated storefront route count assertion from 17 to 18 in `tests/ui/sweep-shard-lib.test.mjs:52`. |
| `gate (--browser-click-sweep@7/10)` | `NEW FAIL storefront /pages/contact desktop/en [page] "(coverage)" coverage-missing` | **(b)** | Same `tests/ui/sweep-shard-lib.mjs:16` route-registry fix. |
| `gate (--browser-click-sweep@8/10)` | `NEW FAIL storefront /pages/contact desktop/zh-TW [page] "(coverage)" coverage-missing` | **(b)** | Same `tests/ui/sweep-shard-lib.mjs:16` route-registry fix. |
| `gate (--browser-webkit)` | `FAIL: order` — `ExpectError: getByTestId('cart-empty') expected "Loading…", received ""` | **(a)** | Rewrote the pending-cart assertion in `tests/storefront/order-gate.mjs:353` to check the signed-frame skeleton (`[aria-busy="true"]` with `aria-label=shopCopy.en.loading`); updated the BC02 DOM sampler (`tests/storefront/order-gate.mjs:287-296`), mount click target (`tests/storefront/order-gate.mjs:363`) and post-failure assertion (`tests/storefront/order-gate.mjs:387`) to match the skeleton. |
| `gate (bundle:02)` | `--browser-order`: `TestBrowserBuyerOrderUI` FAIL — same cart-empty "Loading…" mismatch | **(a)** | Same `tests/storefront/order-gate.mjs` rewrite as above. |
| `gate (bundle:02)` | `--browser-storefront`: `ExpectError: getByRole('heading', { level: 1 }) expected "Tableware", received "Daily Series"` | **(a)** | Rewrote collection-page heading assertion in `tests/storefront/shop-real-gate.mjs:168` to expect `COPY.dailySeries` (signed PARITY-BUYER-BROWSE series title). |
| `aggregate` | `G-UI8 click sweep aggregate: FAIL` — 3 missing contact page rows | **(b)** | Downstream of the `tests/ui/sweep-shard-lib.mjs:16` route fix. |
| `receipts` | `matrix-receipts: RED (6 problem(s))` — click-sweep shards 4/7/8, order, storefront, webkit receipts FAIL | downstream | Downstream of the class (a) and (b) fixes above. |
| `required` | `plan=success build=success gate=failure aggregate=failure receipts=failure` | downstream | Downstream of the class (a) and (b) fixes above. |

## Known base issues — class (c), NOT fixed in this PR

These are pre-existing on `origin/r3/integration` and must be left alone per integrator note (2026-10-31):

- `gate --browser-inbox`: `INU01..INU08` / `INV2` / `"PARITY inbox ... signed send state"` fail with
  `locator.click: Test timeout ... waiting for getByRole('button', { name: 'Open navigation' })`.
  Root cause is a race in `tests/admin/inbox-browser-support.ts` `enterMessages`; integrator fix `a122b3cd6`
  rides on PR #356 and is already in `origin/r3/integration`. Do not edit inbox support/source tests.

- Evidence retention gate (PR #426, `docs/adr/ADR-20261031-evidence-retention.md`): rejects raw CI artifact
  copies and large `output/` files. Fix is to `git rm -r --cached` raw paths and cite run id/job/artifact
  name + sha256 in `DELIVERY.md`. Pre-push check `node scripts/dev/check-evidence-retention.mjs --base origin/r3/integration`
  passes locally for this batch.
