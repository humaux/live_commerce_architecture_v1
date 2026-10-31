# GEMINI-REPORT — Directed Fix of PR #403 (PARITY-NO-MARKET-FIX3 Relay 1)

Date: 2026-10-31
PR: #403 (branch: `unit/parity-no-market`)
Failing run: [41202427277](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41202427277) (job `124541458023`, `gate (bundle:06)`)

## Summary of Failing Jobs and Changes

| Failing Job / Test | Class | What Changed (file:line) |
|---|---|---|
| `124541458023` (`gate (bundle:06)`) / `TestBrowserSettingsWizardRealChain` | (b) product root cause | `apps/admin/components/SettingsWizard.tsx:1376`: added `aria-label={c.service}` to delivery service `<select>`; line 1428: added `aria-label={c.deliveryKind}` to delivery kind `<select>`; line 1507: added `aria-label={c.taxMode}` to tax mode `<select>`; line 1527: added `aria-label={c.taxBasis}` to tax basis `<select>`.<br>**Root cause**: In `SettingsWizard.tsx`, the delivery service select element is wrapped in `<label>{c.service}<select ...></select></label>`. When the service list is reloaded and populated with active service `"home"`, W3C AccName step 2F computes the accessible name by including the selected child option text, evaluating to `"Delivery service home"` instead of `"Delivery service"`. Playwright's exact locator `wizard.getByLabel(..., { exact: true })` at `tests/admin/settings-v2.spec.ts:737` timed out waiting for an element named `"Delivery service"`. Adding `aria-label={c.service}` directly on the `<select>` satisfies AccName step 2A and guarantees that the computed accessible name exactly matches `c.service` across all supported locales ("Delivery service", "配送服務", "配送服务"). Explicit `aria-label` attributes were likewise added to `deliveryKind`, `taxMode`, and `taxBasis` `<select>` elements. |
| `124541458023` (`gate (bundle:06)`) / `--browser-inbox` | (c) base issue | Known base issue in `tests/admin/inbox-browser-support.ts` fixed by commit `a122b3cd6` on PR #356. Left as-is per card instructions. |

STATUS: DONE
