# PARITY-BUYER-BROWSE-FIX1 classification

Failing runs for current HEAD:
- Main workflow: https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41249740186
- Foundation: https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41249740189

Base gates run: 41238442379 on 4501b409c success (green base).
Head at failure: `fa84c559cc44418951b0f6ec504d2f498645283a`.

| failing job | failing test / evidence | class | what changed |
|-------------|------------------------|-------|--------------|
| `gate (bundle:02 --browser-storefront)` | `PARITY-BUYER-BROWSE RED`: 9 signed-frame assertions failed — desktop sort-option locators (`a[data-sort="..."]`) resolved to hidden elements because the mobile sort sheet is `display: none` on desktop; the signed desktop frames expose the active sort through the visible native `<select>` | **(a)** | Rewrote desktop sort assertions in `tests/storefront/buyer-browse-parity.spec.ts:404-436` to assert the visible `<select name="sort">` has the active value; kept mobile assertions on the visible sheet option. |
| `receipts` | `matrix-receipts: RED (1 problem)` — `--browser-storefront` receipt is FAIL | downstream | Resolved by the class (a) fix above. |
| `required` | `plan=success build=success gate=failure aggregate=success receipts=failure` | downstream | Resolved by the class (a) fix above. |

## Pre-push checks (performed locally)

```sh
node scripts/dev/derive-browser-ownership.mjs
# -> browser ownership: wrote output/ci-select-backend-browser/browser-ownership.json (54 modes)
node scripts/dev/derive-browser-ownership.mjs --check
# -> (no output) PASS
node scripts/dev/check-evidence-retention.mjs --base origin/r3/integration
# -> evidence-retention: PASS (9 output/ files, 253428 bytes vs origin/r3/integration; ADR-20261031)
```

No tests, builds, servers or browsers were started locally per the lane constraints.

## Known base issues — class (c), NOT fixed in this PR

These are pre-existing on `origin/r3/integration` and must be left alone per integrator note (2026-10-31):

- `gate --browser-inbox`: `INU01..INU08` / `INV2` / `"PARITY inbox ... signed send state"` fail with
  `locator.click: Test timeout ... waiting for getByRole('button', { name: 'Open navigation' })`.
  Root cause is a race in `tests/admin/inbox-browser-support.ts` `enterMessages`; integrator fix `a122b3cd6`
  rides on PR #356 and is already in `origin/r3/integration`. Do not edit inbox support/source tests.

- Evidence retention gate (PR #426, `docs/adr/ADR-20261031-evidence-retention.md`): rejects raw CI artifact
  copies and large `output/` files. Fix is to `git rm -r --cached` raw paths and cite run id/job/artifact
  name + sha256 in `DELIVERY.md`. The pre-push evidence-retention check passes for this batch.
