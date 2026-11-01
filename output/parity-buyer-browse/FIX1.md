# PARITY-BUYER-BROWSE-FIX1 classification

Failing runs for current HEAD:
- Main workflow: https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41252821638
- Foundation: https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41252821649

Base gates run: 41238442379 on 4501b409c success (green base).
Head at failure: `2a09e7ba5bb08df8c91ae282cf5c8f29179ed07e`.
Merge SHA tested by CI: `0bc92297509f80e5f7696e8bb7d7b1e0fcd24fb5`.

| failing job | failing test / evidence | class | what changed |
|-------------|------------------------|-------|--------------|
| `gate (bundle:02 --browser-storefront)` | `TestBrowserStorefront` / `checkBuyerBrowseParity` / `[782:4090] sort-asc-check` (zh-CN), `[782:4143] sort-desc-check` (en): `locator('a[data-sort="..."]').toBeVisible()` failed with strict-mode violation (resolved to 2 elements, one hidden). The signed mobile frames expose the active sort option inside the visible sort-sheet dialog; a transient hidden duplicate (React streaming placeholder) was also matching the un-scoped locator. | **(a)** | Rewrote mobile sort assertions in `tests/storefront/buyer-browse-parity.spec.ts:404-436` to scope the active sort option to the visible `[role="dialog"][aria-label="..."]`; desktop assertions continue to assert the visible `<select name="sort">` value. |
| `receipts` | `matrix-receipts: RED (1 problem)` — `--browser-storefront` receipt status FAIL | downstream | Resolved by the class (a) fix above. |
| `required` | `plan=success build=success gate=failure aggregate=success receipts=failure` | downstream | Resolved by the class (a) fix above. |

## Pre-push checks (performed locally)

```sh
node scripts/dev/derive-browser-ownership.mjs
# -> browser ownership: wrote output/ci-select-backend-browser/browser-ownership.json (54 modes)
node scripts/dev/derive-browser-ownership.mjs --check
# -> (no output) PASS
node scripts/dev/check-evidence-retention.mjs --base origin/r3/integration
# -> evidence-retention: PASS (10 output/ files, 255281 bytes vs origin/r3/integration; ADR-20261031)
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
