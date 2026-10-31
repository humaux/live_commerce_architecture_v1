# PARITY-ENTRY-FIX2 — Directed Fix Resolution

## Directed FIX2 Resolution — 2026-10-31

Required first merge completed: integration `acf2ebd2c6a19dd731aa98acce509b45629b1f77` merged into local branch `unit/parity-entry`.

Failing run: `41170112825` (PR #369). Base Gates: `41167677841` on `acf2ebd2c` success (no class (c) base exemptions).

### Classification and Directed Fix Table

| Failing Job | Failing Test | Evidence | Class | What Changed & Root Cause |
|---|---|---|---|---|
| 124446852658 (bundle:06 / `--browser-inbox`) | `TestBrowserInboxUIRealChain` (`inbox-ui.spec.ts:110:1 (INU01)`, `inbox-ui.spec.ts:590:3 (INU06 en)`, `inbox-v2-ui.spec.ts:733:3 (PARITY inbox zh-CN)`, `inbox-v2-ui.spec.ts:763:5 (PARITY inbox desktop closed A)`) | `inbox-ui/playwright.log`: locator.click timeout 30000ms waiting for `getByRole('button', { name: 'Open navigation', exact: true })` at `inbox-browser-support.ts:77` | (b) | Shared helper regression in `enterMessages` (`tests/admin/inbox-browser-support.ts:77`). At desktop viewports (`width > 1023`), the hamburger menu button has `display: none` in `packages/ui/src/AppShell.module.css`; checking `if (!await nav.isVisible())` during initial render/transition erroneously attempted to click the hidden menu button, timing out after 30s. At mobile viewports, the menu button is only named `"Open navigation"` in English. Fixed by querying responsive viewport width (`page.viewportSize ? page.viewportSize() ? page.viewportSize()!.width <= 1023 : false : !await nav.isVisible()`), preventing desktop test suites that walk the shell as setup from waiting on `display: none` mobile buttons while preserving mobile drawer behavior and `inbox-browser-source.test.ts` contracts. |
| 124446852816 (bundle:08 / `--browser-password-auth`) | `TestBrowserPasswordAuth` (`tests/admin/parity-entry.spec.ts:69-160`, 38 tests timed out; `tests/admin/password-auth.spec.ts:179:1` hit `net::ERR_CONNECTION_REFUSED`) | `password-auth/playwright.log` lines 60-120; `results/.../error-context.md` showing accessibility tree has `*電子郵件`, `*密碼` | (b) | Product styling regression. `apps/admin/app/globals.css:1331` defines `.entry-form label > span::before { content: "*"; }`, prepending `*` to input labels in the browser accessibility tree (`"*電子郵件"`, `"*密碼"`, `"*確認密碼"`). This contradicted signed Figma frame `20:443` (literal `電子郵件` without asterisk) and caused 30 tests in `parity-entry.spec.ts` to time out waiting for `getByRole('textbox', { name: '電子郵件', exact: true })` and `getByLabel(...)`, consuming 840s and hitting Go test deadline before `password-auth.spec.ts` could run. Fixed by resetting `.entry-parity .entry-form label > span::before { content: none; }` in `apps/admin/components/auth/entry.css:17`. |
| 124446852816 / 124446852658 (`receipts`) | `matrix-receipts` | Derived: `receipt-browser-inbox.json` and `receipt-browser-password-auth.json` failed | Derived | Derived failure caused solely by the two failing browser jobs above. Resolved by root fixes in both modes. |
| 124446852816 / 124446852658 (`required` / `aggregate`) | Required gates | Derived: failed due to inbox and password-auth jobs | Derived | Resolved by root fixes in both modes. |

(c) Identical failure on base: None. Base Gates `41167677841` on `acf2ebd2c` passed.
