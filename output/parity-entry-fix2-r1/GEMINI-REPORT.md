# PARITY-ENTRY-FIX2-R1 GEMINI REPORT

| Failing Job / Test | Class | What Changed | File:Line |
|---|---|---|---|
| 124458118570 (`gate (bundle:08)` / `--browser-password-auth`) / `TestBrowserPasswordAuth` (`parity-entry.spec.ts:69:3`, 12 tests in `reset/code/*`) | (b) | Moved `<small>{c.policyHint}</small>` outside `<label>` in `PasswordAuth.tsx`, restoring the accessible name of `<input name="new_password">` to exact literal `新密碼` without the appended hint text in the accessibility tree, unblocking `getByLabel("新密碼", { exact: true })` | `apps/admin/components/PasswordAuth.tsx:352-368` |
| 124458118570 (`gate (bundle:08)` / `--browser-password-auth`) / `TestBrowserPasswordAuth` (`password-auth.spec.ts:179:1`) | (b) | Fixed mocked order fixture timestamp formatting in `dashboardParity` helper to include 6-digit microseconds (`.000000Z`), satisfying `orders-model.ts` strict timestamp validator (`/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{6}Z$/`) and preventing dashboard from falling back to `unavailable` error status | `tests/admin/parity-entry-dashboard.ts:27` |
| 124458118570 (`gate (bundle:08)` / `receipts`, `required`, `aggregate`) | (b) | Regenerated browser ownership matrix via `scripts/dev/derive-browser-ownership.mjs` and verified with `--check` | `output/ci-select-backend-browser/browser-ownership.json:152` |

STATUS: DONE
