# FIX1 — PR #439, run 41232180076 (merge of origin/r3/integration: already up to date)

| Job | Failing test | Evidence | Class |
|---|---|---|---|
| gate (bundle:03) job 124629547904 | TestBrowserHomeCod (buyer script ended before "placed") | home-cod-buyer.mjs.log: `locator.check: Timeout 30000ms` at modeSaveFailure (home-cod-buyer.mjs:442); orders A-D passed. Playwright `check()` asserts the radio ends checked, but the injected storage failure intentionally keeps it unchecked (OrderFlow onChange returns before setPaymentMode, option=null). | test misuse in this PR's new test; fixed with a real `.click()`; product unchanged |
| receipts 124634438892, required 124634532928 | — | consequence of the job above | follow-on |
