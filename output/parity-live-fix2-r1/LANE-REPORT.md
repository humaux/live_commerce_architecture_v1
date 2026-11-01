# PARITY-LIVE FIX2 relay 1 — lane report (head 0215362b, run 41247536261)

Merged origin/r3/integration first: already up to date (base 4501b409c).

| Failing job / test | Evidence | Class | What changed |
|---|---|---|---|
| gate (--browser-live-console) 124674733061, `TestBrowserLiveConsoleRealChain` | `panic: test timed out after 13m0s` at browser_live_console_test.go:624 (`cmd.Run()` of the Playwright child); the same job also failed at the older head 6c09cc83a (run 41110014965, ~14.5 min). Base PR #403 passes this job in 7m32s. This PR adds ~110 serial tests (PARITY-LIVE describe: 102 frame tests + 6 main + 3 block) to `live-console.spec.ts`, so the single `-timeout=780s` budget is exhausted. | (b) regression caused by this PR (runtime budget), not a product defect | Split the budget; no test removed or skipped, no timeout raised. `tests/foundation/browser_live_console_test.go`: the workspace run now passes `--grep-invert "PARITY-LIVE current Figma route parity"`; new `TestBrowserLiveConsoleParityRealChain` runs only that describe. `scripts/dev/test-local.sh`: new mode `--browser-live-console-parity` (same fixture, build and covers, own 780s go test). Ownership regenerated. |
| gate (bundle:07) 124674732627, member `--browser-identity`, `TestBrowserSettingsWizardRealChain` | Playwright log (artifact gate-browser-identity): `settings-v2.spec.ts:1085` in `marketSelectionAcceptance`: the `Save pricing policy` button stays `disabled` (SettingsWizard.tsx:1608, gate on `hasBaseline(policyObservation)` / `activeMarket.active`) for 10s. The settings wizard, settings-v2/settings-real specs and SettingsWizard are untouched by this PR (no settings file in `git diff origin/r3/integration...HEAD`); bundle:07 passed at 6c09cc83a and in base PR #403 CI. | (c) not caused by this PR; suspected load-order race in the base wizard (policy observation vs markets mock). Not proven identical on base: base push runs have no browser gates. | none. Rerun bundle:07; if it stays red it is a base defect to fix on r3/integration. |
| receipts 124680428673 | `matrix-receipts: RED (2 problem(s))` = the two jobs above | follows | none |
| required 124680494323 | aggregate of the jobs above | follows | none |

The new mode `--browser-live-console-parity` has no GATE_SECONDS entry, so CI gives it its own job (unmeasured-mode rule). Nothing was run locally (owner rule: all tests run in CI).

STATUS: DONE
