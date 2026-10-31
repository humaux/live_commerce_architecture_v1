# PARITY-ENTRY-FIX2 — Directed Fix Resolution

## Directed FIX2 Resolution — 2026-10-31 (Relay 2 Head 6509e291)

Required first merge completed: integration `acf2ebd2c6a19dd731aa98acce509b45629b1f77` merged into local branch `unit/parity-entry` (already up to date).

Failing runs:
- `41170112825` (head `aaef95b97`): bundle:06 (`--browser-inbox`) and bundle:08 (`--browser-password-auth`) failed. Resolved in `a74dfa7c4`.
- `41173868839` (head `a74dfa7c4`): bundle:06 (`--browser-inbox`) PASSED. All 12 `reset/code/*` cases in `parity-entry.spec.ts` resolved in `6509e2919`.
- `41176542251` (head `6509e2919`): all 42 `parity-entry.spec.ts` cases PASSED; all click-sweep (10/10), visual-lint (4/4), foundation, bundles 01-07/09, and unit shards PASSED. Only `gate (bundle:08)` (`--browser-password-auth`, job 124466010545) and `gate (--browser-webkit)` (job 124466010533, same `TestBrowserPasswordAuth` step) failed on `tests/admin/password-auth.spec.ts:179:1` (`dashboardParity` at `tests/admin/parity-entry-dashboard.ts:112`).

Base Gates: `41167677841` on `acf2ebd2c` success (no class (c) base exemptions).

### Classification and Directed Fix Table

| Failing Job | Failing Test | Evidence | Class | What Changed & Root Cause |
|---|---|---|---|---|
| 124466010545 (`gate (bundle:08)` / `--browser-password-auth`) & 124466010533 (`gate (--browser-webkit)`) | `TestBrowserPasswordAuth` (`tests/admin/password-auth.spec.ts:179:1`, full chain -> `dashboardParity` at `tests/admin/parity-entry-dashboard.ts:112`) | `password-auth/playwright.log` line 46; `error-context.md` showing `Locator: getByTestId('dashboard-page').getByText('今日訂單', { exact: true })` timed out and page status `總覽暫時無法使用，請重試。` | (b) | Shared setup helper defect in `tests/admin/parity-entry-dashboard.ts:26-83`. `readDashboard` (`apps/admin/lib/customers-client.ts:43`) and `readMetaHealth` (`apps/admin/lib/orders-client.ts:40`) enforce that BFF JSON responses include `Cache-Control: private, no-store` (matching the real BFF routes in `apps/admin/app/api/stores/[store]/tools/[...resource]/route.ts:29` and `[...resource]/route.ts:482`). Because Playwright's `route.fulfill({ json: ... })` only sets `content-type: application/json` unless `headers` are explicitly supplied, `readJSON` and `read` threw `ReadError("unavailable")` / `OrderReadError("unavailable")`, causing `Dashboard` to render `unavailable` (`總覽暫時無法使用，請重試。`). Fixed by passing `headers: { "cache-control": "private, no-store" }` on the intercepted store BFF routes in `tests/admin/parity-entry-dashboard.ts`. |
| 124466010545 (`gate (bundle:08)` / `--browser-password-auth`) & 124466010533 (`gate (--browser-webkit)`) | `TestBrowserPasswordAuth` (`tests/admin/password-auth.spec.ts:603:1`, `matrix: locale copy differs per locale and the reset wording is the same for known and unknown addresses`) | `tests/admin/password-auth.spec.ts:620-626` vs `apps/admin/components/PasswordAuth.tsx:259` (serialized after test 43) | (a) | In `PARITY-ENTRY`, the signed reset flow splits step 1 acknowledgement into frame `82:1760` (`step === "sent"`, `"Check your email"`, which displays the masked email address) and frame `20:513` (`step === "code"`, `"Set a new password"`, entered by clicking `"Enter the confirmation code"`). `visible(email)` in `password-auth.spec.ts:620-626` clicked `"Enter the confirmation code"` before reading `innerText()` and asserting `expect(text).toContain(masked)`. Updated `visible(email)` to assert `expect(sentText).toContain(masked)` on the `"Check your email"` (`sent`) frame, then click `"Enter the confirmation code"` to reach the `"code"` frame and compare both frames' masked visible text between known and unknown addresses. |
| 124466010545 & 124466010533 (`receipts` / `required` / `aggregate`) | `matrix-receipts` & required gates | Derived: `receipt-browser-password-auth.json` and `receipt-browser-webkit.json` failed (`status: FAIL`) | Derived | Derived failure caused solely by `TestBrowserPasswordAuth` above; resolved by the fixes in `tests/admin/parity-entry-dashboard.ts` and `tests/admin/password-auth.spec.ts` and regenerating `output/ci-select-backend-browser/browser-ownership.json`. |

(c) Identical failure on base: None. Base Gates `41167677841` on `acf2ebd2c` passed.
