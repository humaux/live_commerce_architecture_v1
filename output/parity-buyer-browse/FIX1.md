# PARITY-BUYER-BROWSE-FIX1 classification

Failing run: https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41240375377
Base gates run: 41238442379 on 4501b409c success (green base).

| failing job | failing test / evidence | class | what changed |
|-------------|------------------------|-------|--------------|
| `build` | `app/[locale]/search/page.tsx(31,38): error TS2339: Property 'searchQuery' does not exist on type 'Copy'.` | **(b)** | Added `searchQuery` to `apps/storefront/lib/shop-copy.ts` for all three locales (`en`: "Tote", `zh-TW`/`zh-CN`: "托特包"). |
| `foundation` | `G-UI8 storefront pages and the sweep's STOREFRONT_ROUTES are the same set`: actual includes `/pages/contact`, expected set does not. | **(b)** | Added `/pages/contact` to `tests/ui/sweep-shard-lib.mjs` `STOREFRONT_ROUTES`; updated assertion count from 17 to 18 storefront routes in `tests/ui/sweep-shard-lib.test.mjs`. |
| `aggregate` | `no click-sweep shard evidence found: every shard job failed before writing it` / `no visual-lint shard evidence found` | **(b)** | Downstream of `build` failure; should pass once build is green. |
| `receipts` | `matrix-receipts: RED (77 problem(s))` — all modes report `no receipt (job wrote none)` | **(b)** | Downstream of `build` failure and skipped gate; should pass once build/foundation are green. |
| `required` | `plan=success build=failure gate=skipped aggregate=failure receipts=failure` | **(b)** | Downstream of `build`/`aggregate`/`receipts` failures. |

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
