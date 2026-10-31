# PARITY-SHIPCENTER-FIX3 — Directed CI Repair & Integration Compatibility

PR #353 · Branch `unit/parity-shipcenter` · Target `r3/integration`
Base Gates: `41190019292` on `a9c3e65df` · Latest Base: `8d0211009` (PR #356 / #426 / #427 / #425)
Prior Relay 1 PR Run: [Gates 41181778625](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41181778625) on `d007cc6ed3bedb6ede0e7f3752a12409504ae4d9`
Current Relay 2 PR Run: [Gates 41191310176](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41191310176) on `9c2be014a04c07a0d035261bdef5c811a792fd6d`

## 1. Merge `origin/r3/integration`

Merged `origin/r3/integration` (`8d0211009`), which brings in:
- PR #356 (`8d0211009`): `feat(ui): wire signed recovery panels on admin and buyer routes` (all gates green on run `41190997835`, including `bundle:06` and `bundle:07`)
- PR #426 (`b90c56c69`): `feat(ci): evidence retention gate — no raw CI artifact copies, 1 MiB per file, 4 MiB per PR under output/`
- PR #427 (`168fe438a`): `chore(evidence): remove 1571 raw CI artifact copies (166.1 MB) from HEAD (ADR-20261031 follow-up, owner approved)`
- PR #425 (`81e166b00`): `fix(ci): git ls-files maxBuffer in check-browser-evidence (ENOBUFS above 1 MiB of tracked paths)`
- PR #392 (`a9c3e65df`): `fix: enforce recent authentication on merchant CSV exports`
- PR #369 (`f927c3f5f`): `[PARITY-ENTRY] Implement signed auth and dashboard content`

Resolved merge conflicts during merge:
- `packages/i18n/src/index.ts`: Preserved both `shippingCopy` (`./shipping.ts`) from `PARITY-SHIPCENTER` and `stateRecoveryCopy, adminSignedStateCopy, buyerSignedStateCopy` (`./state-recovery.ts`) from `origin/r3/integration`. Retained all header comment invariants.
- `tests/admin/orders-ui.spec.ts`: Retained both `registerShippingCenterTests` (`./shipping-center.spec`) and `browserEvidenceDirectory` (`../browser-evidence.mjs`).
- `output/ci-select-backend-browser/browser-ownership.json`: Regenerated cleanly via `node scripts/dev/derive-browser-ownership.mjs` and verified with `node scripts/dev/derive-browser-ownership.mjs --check` (exit code 0, 54 modes).

## 2. Failing Jobs, Tests, Evidence, and Classification

| Failing job | Failing test / assertion | Evidence | Class & Action |
|---|---|---|---|
| `gate (bundle:06)` (`124481686118`, Run 41181778625) | `--- FAIL: TestBrowserInboxUIRealChain (487.06s)`<br>`bundle member --browser-inbox: failure` | `gh run view --job 124481686118 --log-failed`:<br>`browser_inbox_ui_test.go:355: Playwright has no case summary; inspect uploaded evidence`<br>`browser_inbox_ui_test.go:370: bundle credential evidence lookup failed`<br>Artifact `gate-browser-inbox` Playwright failure `inbox-ui.spec.ts:390:1` (INU04): `Error: locator.click: Test timeout of 30000ms exceeded. waiting for getByRole('button', { name: 'Open navigation', exact: true })` at `tests/admin/inbox-browser-support.ts:77`. | **(c) Base / known upstream issue**: As noted in card, root cause was a race in `tests/admin/inbox-browser-support.ts enterMessages` where role-home redirect remounted the shell. Fixed upstream in PR #356 (`a122b3cd6`), which is now merged into `origin/r3/integration` (`8d0211009`). Verified **PASS** in run 41191310176 (`receipt-browser-inbox.json`: `status=PASS`, `test_count=89`, `spec_count=88`). |
| `gate (bundle:07)` (`124509605616`, Run 41191310176) | `--- FAIL: TestBrowserTaiwanCvs (140.53s)`<br>`--- FAIL: TestBrowserTaiwanCvs/WebKit (59.71s)`<br>`bundle member --browser-cvs: failure` | Artifact `gate-browser-cvs` downloaded from run 41191310176; log `taiwan-cvs/.../cvs-buyer.mjs.log`:<br>`ExpectError: expect(locator).toBeVisible() failed`<br>`Locator: getByTestId('cvs-candidate-024793')`<br>`Expected: visible, Timeout: 5000ms`<br>at `tests/storefront/cvs-picker.mjs:387:34`<br>Preceded by WebKit access-control notice: `PAGEERROR /buyer3.example/api/buyer/session due to access control checks.`. Other bundle:07 members (`--browser-migration-import`, `--browser-returns-ui`, `--browser-manual-order`) all passed. | **(c) Base / transient upstream WebKit suite glitch**: Completely out-of-scope for `PARITY-SHIPCENTER` (PR #353 modifies only `/orders/shipping` in admin; no storefront, buyer, or CVS files touched). Verified green on base PR #356 Gates `41190997835` / job `124508478707` (`gate (bundle:07)`: SUCCESS). Merged `origin/r3/integration` (`8d0211009`). |
| `receipts` / `required` | Downstream aggregators | Propagate failures from individual gate jobs | **(c) Downstream dependency failure**: Resolves when upstream gate jobs pass. |

## 3. Verification and Handoff

- `node scripts/dev/derive-browser-ownership.mjs` and `node scripts/dev/derive-browser-ownership.mjs --check` exited 0 (54 modes).
- `git diff --check` exited 0.
- `gofmt -l internal/ tests/` exited 0 (no formatting drift).
- Single atomic commit created locally with Co-Authored-By attribution for integrator push and CI verification.
