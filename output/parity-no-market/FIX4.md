# FIX4 — PR #403 (PARITY-NO-MARKET-FIX4)

Merge origin/r3/integration: already up to date (faf6502b5).
Failing run 41222259982, job 124600415778 `gate (bundle:06)`, artifact `gate-browser-identity`
(playwright/settings-real-20261031T171413.910045893/playwright.log sha256 e334e412e405156e5f6ff9b7f232e7f1c01320e7082972702d5ec0d7ee01d702; not committed, ADR-20261031).

| Job | Failing test | Evidence | Class | Fix |
|---|---|---|---|---|
| 124600415778 gate (bundle:06) | TestBrowserSettingsWizardRealChain | `settings-v2.spec.ts:1019` strict-mode violation: `getByRole("alert")` matched the wizard error `div` AND Next's `#__next-route-announcer__` (2 elements); the wizard's own message was present | (a) test locator (RVFIX-added assertion) | scope to `wizard.getByRole("alert")`, same text assertion, same coverage (tests/admin/settings-v2.spec.ts:1019) |
| 124605454683 receipts, 124605540715 required | — | fail only because of the job above | derived | none |
