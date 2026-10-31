# FIX3 — Directed Fix of PR #403 (PARITY-NO-MARKET-FIX3)

Date: 2026-10-31
PR: #403 (branch: `unit/parity-no-market`)
Failing run: [41198990173](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41198990173) (job `124531705344`, `gate (bundle:06)`)

## Summary of Failing Jobs and Root Causes

| Failing Job | Failing Test | Evidence | Class | Root Cause & Resolution |
|---|---|---|---|---|
| `124531705344` (`gate (bundle:06)`) | `TestBrowserSettingsWizardRealChain` | Artifact `gate-browser-identity` (run `41198990173`, job `124531705344`, artifact `13041644991`, `playwright/settings-real-20261031T121431.138374010/playwright.log` sha256 `c8a405dc938d82e11e21c4ef6cfa74fc4dfa6c86eba20c15b8c775cdc1107fee`): `Error: expect(locator).toBeVisible() failed` on `Locator: getByTestId('settings-service-form')` with timeout 10000ms at `tests/admin/settings-v2.spec.ts:719:30` | (b) | **(b) Product root cause**: In `apps/admin/components/SettingsWizard.tsx` (`policySubmit`), saving a pricing policy in a store with zero markets (`!activeMarket`) previously called `saveDraft(updatedDraft)` with `marketID: savedMarket.id` before dispatching `PUT .../policy`. This triggered `useEffect` hooks for `loadPolicy` and `loadService` while the policy PUT was still in-flight. Because the policy was not yet saved on the server/mock, `loadPolicy` returned 404, which wiped `draft.policyEnabled` to `false` and set `policy = null`. When the policy PUT completed, `needsPolicySave` evaluated to `true` (`policy.enabled !== draft.policyEnabled`, `true !== false`), preventing `settings-service-form` (`{!needsPolicySave && ...}`) from ever rendering.<br>Resolution: in `apps/admin/components/SettingsWizard.tsx:1010-1065`, eliminated the premature `saveDraft(updatedDraft)` before `PUT .../policy`. In the PUT callback, advanced `policyRead.current` and `serviceRead.current` to cancel stale reads, and atomically set `setMarkets`, `setPolicy`, and `saveDraft` with synchronized fields (`policyEnabled: savedPolicy.enabled`, money conversions, and valid baselines for `policyObservation` and `serviceObservation: { target, version: 0, dirty: false }`). Also ensured `retryPending` for `resource.endsWith("/policy")` sets `marketID: saved.market_id` and service observation baseline. |
| `124531705344` (`gate (bundle:06)`) | `--browser-inbox` | Shell navigation button timeout on role-home redirect race | (c) | **(c) Base issue**: Documented known base issue in `tests/admin/inbox-browser-support.ts` fixed by commit `a122b3cd6` on PR #356. Left as-is per card instructions. |

STATUS: DONE
