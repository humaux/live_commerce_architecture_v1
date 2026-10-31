# PARITY-SHIPCENTER-FIX3 — Directed CI Repair & Integration Compatibility

PR #353 · Branch `unit/parity-shipcenter` · Target `r3/integration`
Base Gates: `41190019292` on `a9c3e65df`
Failing PR Run: [Gates 41181778625](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41181778625) on `d007cc6ed3bedb6ede0e7f3752a12409504ae4d9`

## 1. Merge `origin/r3/integration`

Merged `origin/r3/integration` (`a9c3e65df`), which brings in:
- PR #392 (`a9c3e65df`): `fix: enforce recent authentication on merchant CSV exports`
- PR #369 (`f927c3f5f`): `[PARITY-ENTRY] Implement signed auth and dashboard content`
- PR #356 upstream fix (`a122b3cd6`): resolves the `enterMessages` race in `tests/admin/inbox-browser-support.ts` where role-home redirect remounted the shell and timed out waiting for `Open navigation`.

Resolved merge conflicts during merge:
- `packages/i18n/src/index.ts`: Preserved both `shippingCopy` (`./shipping.ts`) from `PARITY-SHIPCENTER` and `entryParityCopy` (`./entry-parity.ts`) from `origin/r3/integration`. Updated header comment dependency line to list both dependencies, retaining all header comment invariants.
- `output/ci-select-backend-browser/browser-ownership.json`: Regenerated cleanly via `node scripts/dev/derive-browser-ownership.mjs` and verified with `node scripts/dev/derive-browser-ownership.mjs --check` (exit code 0, 54 modes).

## 2. Failing Jobs, Tests, Evidence, and Classification

| Failing job | Failing test / assertion | Evidence | Class & Action |
|---|---|---|---|
| `gate (bundle:06)` (`124481686118`) | `--- FAIL: TestBrowserInboxUIRealChain (487.06s)`<br>`bundle member --browser-inbox: failure` | `gh run view --job 124481686118 --log-failed`:<br>`browser_inbox_ui_test.go:355: Playwright has no case summary; inspect uploaded evidence`<br>`browser_inbox_ui_test.go:370: bundle credential evidence lookup failed`<br>Artifact `gate-browser-inbox` Playwright failure `inbox-ui.spec.ts:390:1` (INU04): `Error: locator.click: Test timeout of 30000ms exceeded. waiting for getByRole('button', { name: 'Open navigation', exact: true })` at `tests/admin/inbox-browser-support.ts:77`. | **(c) Base / known upstream issue**: As noted in card, root cause was a race in `tests/admin/inbox-browser-support.ts enterMessages` where role-home redirect remounted the shell. Fixed upstream in PR #356, which was merged into `origin/r3/integration` and brought into `unit/parity-shipcenter` via Step 1 merge. No manual edits to `inbox-browser-support.ts`. |
| `receipts` (`124486949073`) | `Every planned mode has one fresh PASS receipt with tests` | `matrix-receipts: {"mode":"--browser-inbox", ... "status":"FAIL"}`<br>`bundle member --browser-inbox: receipt receipt-browser-inbox.json is not PASS` | **(c) Downstream dependency failure**: Failed exclusively because `--browser-inbox` failed in `gate (bundle:06)`. Resolves when `bundle:06` passes. |
| `required` (`124486949072`) | Required status check aggregator | `gate (bundle:06)` and `receipts` exited 1 | **(c) Downstream aggregator failure**: Resolves when upstream gate jobs pass. |

## 3. Verification and Handoff

- `node scripts/dev/derive-browser-ownership.mjs` and `node scripts/dev/derive-browser-ownership.mjs --check` exited 0 (54 modes).
- `git diff --check` exited 0.
- `gofmt -l internal/ tests/` exited 0 (no formatting drift).
- Single atomic commit created locally with Co-Authored-By attribution for integrator push and CI verification.
