# RELAY 3 — PR #403 head 40b599fe (run 41231545040)

Failing: gate (bundle:06) -> bundle member --browser-identity -> TestBrowserSettingsWizardRealChain; receipts/required follow from it. Every other job passed.
Playwright log (artifact gate-browser-identity, settings-real-*/playwright.log): settings-real.spec.ts:270 `a pending version read is not a saveable baseline` expected disabled, received enabled.

| finding | class | change |
|---|---|---|
| settings-real.spec.ts:270 Save enabled while the held home policy read is pending | b (test setup flaw, not a product regression: the page.reload added by PARITY-NO-MARKET restores the sessionStorage draft whose non-dirty tw:TW:home v0 baseline was already hydrated after the retry, so no read is pending) | tests/admin/settings-real.spec.ts:263-266 replaced reload with fill("home2") then fill("home"): update() on serviceCode nulls the observation and starts the held GET. Assertions untouched. |
| 40b599fe version-0 baseline (RVFIX2-403) | not implicated: it runs only in the POST success callback, which this test never reaches (lost reply, replayed via retryPending) | none |

Not run locally (owner rule). GREEN run id: pending CI on the next push.

STATUS: DONE
