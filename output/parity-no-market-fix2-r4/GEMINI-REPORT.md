# GEMINI-REPORT — Directed Fix of PR #403 (PARITY-NO-MARKET-FIX2-R4)

Date: 2026-10-31
PR: #403 (branch: `unit/parity-no-market`)
Failing run: [41196156208](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41196156208) (job `124523630229`, `gate (bundle:06)`)

## Summary of Failing Jobs and Changes

| Failing Job / Test | Class | What Changed (file:line) |
|---|---|---|
| `124523630229` (`gate (bundle:06)`) / `TestBrowserSettingsWizardRealChain` | (b) product root cause & (a) test asserting non-existent button | `apps/admin/components/SettingsWizard.tsx:1568`: `<small>{c.refHint}</small>` was nested inside `<label>` for `{c.ref}` without an explicit `aria-label` attribute on the input, so the accessible name calculation appended the hint text. This caused exact-label queries `getByLabel(labels.reference, { exact: true })` in `tests/admin/settings-v2.spec.ts:711` to time out across all three locales. Added `aria-label={c.ref}` to the input (and bounded `freeShipping` and `serviceMode` accessible names).<br>`tests/admin/settings-v2.spec.ts:725-728`: `noMarketAcceptance` asserted `await form.getByRole("button", { name: c.next, exact: true }).click()`. In the signed wizard implementation, submitting the delivery service form via `labels.save` ("Save delivery service") commits and automatically advances directly to Step 4 status (`settings-status`), unmounting `settings-service-form`. Rewrote to remove the non-existent Continue button click so Step 4 status is asserted directly upon service save.<br>`output/ci-select-backend-browser/browser-ownership.json:34`: regenerated browser ownership hash for `tests/admin/settings-v2.spec.ts`. |
| `124523630229` (`gate (bundle:06)`) / `--browser-inbox` | (c) identical failure on base | Known base issue in `tests/admin/inbox-browser-support.ts` (`enterMessages` shell remount race); documented class (c) riding on PR #356. No changes made in this PR. |

STATUS: DONE
