# PARITY-ENTRY-FIX2-R2 GEMINI REPORT

| Failing Job / Test | Class | What Changed | File:Line |
|---|---|---|---|
| 124466010545 (`gate (bundle:08)` / `--browser-password-auth`) & 124466010533 (`gate (--browser-webkit)`) / `TestBrowserPasswordAuth` (`password-auth.spec.ts:179:1` -> `parity-entry-dashboard.ts:112`) | (b) | Added `headers = { "cache-control": "private, no-store" }` to all intercepted BFF `route.fulfill` calls in `dashboardParity` so `customers-client.ts` (`readJSON`) and `orders-client.ts` (`read`) accept the mocked BFF responses instead of throwing `ReadError("unavailable")` | `tests/admin/parity-entry-dashboard.ts:26-83` |
| 124466010545 (`gate (bundle:08)` / `--browser-password-auth`) & 124466010533 (`gate (--browser-webkit)`) / `TestBrowserPasswordAuth` (`password-auth.spec.ts:603:1`) | (a) | Updated `visible(email)` in the known-vs-unknown reset wording test to read and verify the masked email on the signed `"Check your email"` (`82:1760`, `step === "sent"`) screen before clicking `"Enter the confirmation code"` (`20:513`, `step === "code"`), comparing both steps' masked text | `tests/admin/password-auth.spec.ts:620-627` |
| 124466010545 & 124466010533 (`receipts`, `required`, `aggregate`) | (b) | Regenerated browser ownership matrix via `node scripts/dev/derive-browser-ownership.mjs` and verified with `--check` | `output/ci-select-backend-browser/browser-ownership.json:152` |

STATUS: DONE
