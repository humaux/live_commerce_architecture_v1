# GEMINI-REPORT — Directed Fix of PR #403 (PARITY-NO-MARKET-FIX3)

Date: 2026-10-31
PR: #403 (branch: `unit/parity-no-market`)
Failing run: [41198990173](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41198990173) (job `124531705344`, `gate (bundle:06)`)

## Summary of Failing Jobs and Changes

| Failing Job / Test | Class | What Changed (file:line) |
|---|---|---|
| `124531705344` (`gate (bundle:06)`) / `TestBrowserSettingsWizardRealChain` | (b) product root cause | `apps/admin/components/SettingsWizard.tsx:1010-1065`: in `policySubmit` for stores with zero markets (`!activeMarket`), removed the premature `saveDraft(updatedDraft)` before `command("PUT", policyResource, ...)` which caused `useEffect` to trigger `loadPolicy` while the policy was still in flight, resulting in a 404 response that wiped `draft.policyEnabled` to `false` and kept `needsPolicySave` true (preventing `settings-service-form` from rendering). In the PUT callback, advance `policyRead.current += 1` and `serviceRead.current += 1` to cancel stale reads, and atomically commit `setMarkets`, `setPolicy`, and `saveDraft` with synchronized fields (`policyEnabled: savedPolicy.enabled`, money conversions, and valid baselines for `policyObservation` and `serviceObservation: { target, version: 0, dirty: false }`). Also in `apps/admin/components/SettingsWizard.tsx:851-865`: synchronized `marketID` and `serviceObservation` baseline on policy retry in `retryPending`. |
| `124531705344` (`gate (bundle:06)`) / `--browser-inbox` | (c) base issue | Known base issue in `tests/admin/inbox-browser-support.ts` fixed by commit `a122b3cd6` on PR #356. Left as-is per card instructions. |

STATUS: DONE
