# PARITY-ENTRY-FIX2 — Directed Fix Resolution

## Directed FIX2 Resolution — 2026-10-31 (Relay 1 Head a74dfa7c)

Required first merge completed: integration `acf2ebd2c6a19dd731aa98acce509b45629b1f77` merged into local branch `unit/parity-entry` (already up to date).

Failing runs:
- `41170112825` (earlier head `aaef95b97`): bundle:06 (`--browser-inbox`) and bundle:08 (`--browser-password-auth`) failed. Resolved in `a74dfa7c4`.
- `41173868839` (head `a74dfa7c4`): bundle:06 (`--browser-inbox`) PASSED. Only bundle:08 (`--browser-password-auth`, job 124458118570) failed.

Base Gates: `41167677841` on `acf2ebd2c` success (no class (c) base exemptions).

### Classification and Directed Fix Table

| Failing Job | Failing Test | Evidence | Class | What Changed & Root Cause |
|---|---|---|---|---|
| 124458118570 (bundle:08 / `--browser-password-auth`) | `TestBrowserPasswordAuth` (`tests/admin/parity-entry.spec.ts:69:3`, 12 tests in `reset/code/*`: `1198:14458`, `1198:14492`, `1207:7978`, `1207:8017`, `1253:18438`, `1253:18496`, `1198:14475`, `1198:14509`, `1207:7997`, `1207:8036`, `1253:18452`, `1253:18519`) | `password-auth/playwright.log` lines 34-40; `error-context.md` showing locator call log `waiting for getByLabel('新密碼', { exact: true })` and page snapshot `textbox "新密碼 至少 12 個字元，請使用沒有在其他地方用過的密碼。"` | (b) | Product JSX structure regression in `apps/admin/components/PasswordAuth.tsx:352`. In the `mode === "reset"` branch, `<small>{c.policyHint}</small>` was wrapped inside the `<label>` element. In WAI-ARIA / HTML, all descendant text inside a `<label>` contributes to the accessible name of the input, making its accessible name `"新密碼 至少 12 個字元，請使用沒有在其他地方用過的密碼。"` rather than exact `"新密碼"`. This caused all 12 `reset/code/*` tests to time out waiting 30000ms for `getByLabel('新密碼', { exact: true })`. Fixed by moving `<small>{c.policyHint}</small>` outside `<label>`, matching the pattern used by `confirm_password` in `mode === "signup"`. |
| 124458118570 (bundle:08 / `--browser-password-auth`) | `TestBrowserPasswordAuth` (`tests/admin/password-auth.spec.ts:179:1`, full chain: sign-up -> code -> onboarding -> logout -> sign-in -> code -> workspace; forgot -> reset -> signed in, old session revoked) | `password-auth/playwright.log` line 348; `error-context.md` showing `Locator: getByTestId('dashboard-page').getByText('今日訂單', { exact: true })` timed out, page snapshot showing error status `總覽暫時無法使用，請重試。` | (b) | Shared setup helper fixture defect in `tests/admin/parity-entry-dashboard.ts:27`. The mocked closed dashboard order fixture defined `created_at: "2026-10-30T01:00:00Z"` and `updated_at: "2026-10-30T01:00:00Z"` without microsecond precision. `apps/admin/lib/orders-model.ts:154` strictly validates timestamps against `/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{6}Z$/`, causing `date(v.created_at)` to return `false`, `parseOrderSummary` to throw `Error("unavailable")`, and the Dashboard component to render `unavailable` failure copy. Fixed by formatting both timestamps with 6-digit microseconds (`"2026-10-30T01:00:00.000000Z"`). Note: because `password-auth.spec.ts` configures `mode: "serial"`, tests 44–54 were marked did-not-run after test 43 failed; resolving test 43 unblocks the remainder of the suite. |
| 124458118570 (bundle:08 / `receipts`) | `matrix-receipts` | Derived: `receipt-browser-password-auth.json` failed (`status: FAIL`) | Derived | Derived failure caused solely by the single failing browser job above. Resolved by root fixes in both `PasswordAuth.tsx` and `parity-entry-dashboard.ts`. |
| 124458118570 (bundle:08 / `required` / `aggregate`) | Required gates | Derived: failed due to bundle:08 password-auth gate | Derived | Resolved by root fixes in bundle:08. |

(c) Identical failure on base: None. Base Gates `41167677841` on `acf2ebd2c` passed.
