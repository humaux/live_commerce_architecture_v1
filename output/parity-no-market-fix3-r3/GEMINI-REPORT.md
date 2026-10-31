# GEMINI-REPORT — Directed Fix of PR #403 (PARITY-NO-MARKET-FIX3 Relay 3)

Date: 2026-10-31
PR: #403 (branch: `unit/parity-no-market`)
Failing run: [41206804253](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41206804253) (job `124554303544`, `gate (bundle:06)`)
Base Head: `3c8997eb3` (`origin/r3/integration`)

## Summary of Failing Jobs and Changes

| Failing Job / Test | Class | What Changed (file:line) |
|---|---|---|
| `124554303544` (`gate (bundle:06)`) / `TestBrowserSettingsWizardRealChain` | (b) product root cause | `apps/admin/components/SettingsWizard.tsx:1381-1403`: in delivery service `<select>` `onChange` handler, check whether the selected service matches the already active service (`draft.serviceCode === selected.code`) with an active loaded `policy` and baseline (`hasBaseline(draft.policyObservation, serviceTarget)`). If so, preserve `policy` and `policyObservation` via `saveDraft(hydrateService(draft, selected)); return;` instead of unconditionally wiping policy to null. If policy is not yet loaded or when switching services, invoke `void loadPolicy(next.marketID, next.country, selected.code)` so policy loading is triggered deterministically even if `draft.serviceCode` does not differ.<br>**Root cause**: In `SettingsWizard.tsx:1379-1396`, selecting an option in the delivery service dropdown previously unconditionally called `setPolicy(null)` and set `policyObservation: null`. When returning to Step 3 after reload, the active service `"home"` is already selected and its policy loaded. When Playwright triggers `selectOption("home")` (or a merchant re-selects the existing service), `event.target.value === "home"`. Because `draft.serviceCode` was already `"home"`, `draft.serviceCode` did not change, so React's `useEffect` for `loadPolicy` (`[ready, draft.marketID, draft.country, draft.serviceCode, loadPolicy]`) never fired. `needsPolicySave` became permanently `true` and `hasBaseline` evaluated to `false`, leaving the wizard stuck displaying `Loading saved configuration…` and unmounting `settings-service-form`. Consequently, `form.getByLabel(labels.enable, { exact: true })` at `tests/admin/settings-v2.spec.ts:737:73` timed out after 10000ms. |
| `124554303544` (`gate (bundle:06)`) / `--browser-inbox` | (c) base issue | Known base issue in `tests/admin/inbox-browser-support.ts` fixed by commit `a122b3cd6` on PR #356. Left as-is per card instructions. |

STATUS: DONE
