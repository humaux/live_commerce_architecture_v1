# GEMINI-REPORT — Directed Fix of PR #403 (PARITY-NO-MARKET-FIX3 Relay 4)

Date: 2026-10-31
PR: #403 (branch: `unit/parity-no-market`)
Failing run: [41209651078](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41209651078) (job `124563465775`, `gate (bundle:06)`)
Base Head: `3c8997eb3` (`origin/r3/integration`)

## Summary of Failing Jobs and Changes

| Failing Job / Test | Class | What Changed (file:line) |
|---|---|---|
| `124563465775` (`gate (bundle:06)`) / `TestBrowserSettingsWizardRealChain` | (a) older test asserts UI replaced by signed no-market frame | `tests/admin/settings-real.spec.ts:198-214`: adapt post-reload command retry assertion barrier to poll for network replay arrival (`await expect.poll(() => marketRequests.length).toBe(2);`), verify idempotency key and payload identity, poll for command journal removal from `localStorage` (`commerce-settings-command:`), and confirm `pendingControls` hidden and `"Stable service code"` visible.<br>`apps/admin/components/SettingsWizard.tsx:837-854`: in `retryPending` for `resource === "markets"`, reference `draftRef.current` rather than captured `draft` state when invoking `commitDraft`.<br>**Root cause**: In `tests/admin/settings-real.spec.ts:198-206`, the test asserts replay of the zero-market creation POST command after crash/reload. Originally on base, market creation lived in an earlier wizard step, and `await expect(page.getByLabel("Stable service code", { exact: true })).toBeVisible()` served as the synchronization barrier waiting for the async replay to finish and transition the wizard into Step 3. Under the signed no-market frame, the separate market step was eliminated; Step 3 IS the delivery setup page, so `"Stable service code"` is already visible before clicking `"Retry pending command"`. Furthermore, when retry begins, `savePending` immediately sets `busy = true`, causing `pendingControls` (`{pending && !busy && ...}`) to hide immediately on the next tick. Therefore, lines 201-202 resolved synchronously in 0ms before the replayed HTTP POST arrived at the route handler, causing synchronous `expect(marketRequests.length).toBe(2)` to evaluate prematurely and fail with `Expected: 2, Received: 1`. |
| `124563465775` (`gate (bundle:06)`) / `--browser-inbox` | (c) base issue | Known base issue in `tests/admin/inbox-browser-support.ts` fixed by commit `a122b3cd6` on PR #356. Left as-is per card instructions. |

STATUS: DONE
