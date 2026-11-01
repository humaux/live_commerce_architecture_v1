# GEMINI-REPORT — Directed Fix of PR #403 (PARITY-NO-MARKET-FIX2-R1)

Date: 2026-10-31
PR: #403 (branch: `unit/parity-no-market`)
Failing runs:
- Run [41191939562](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41191939562): job `124510528750` (`build`)
- Run [41182542290](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41182542290): job `124483922473` (`gate (--browser-identity)`)

## Summary of Changes

| Failing Job / Test | Class | What Changed (file:line) |
|---|---|---|
| `124510528750` (`build`) / `Next builds (once per run)` | (b) product root cause | `apps/admin/components/SettingsWizard.tsx:742,790,809`: refactored `afterCallback` variable into `postAction: { run: (() => void) | null }` to prevent TypeScript control flow analysis from narrowing the closure variable to `never` after async `navigator.locks.request(...)`. Fixed `error TS2349: This expression is not callable. Type 'never' has no call signatures.`. |
| `124483922473` (`gate (--browser-identity)`) / `TestBrowserSettingsWizardRealChain` | (b) product root cause | `apps/admin/lib/settings-copy.ts:13,25-32,102,112-119,180,190-197`: replaced merchant-visible market wording with delivery / setup copy across `en`, `zh-CN`, and `zh-TW`.<br>`apps/admin/components/SettingsWizard.tsx:143-165,402-458`: auto-resolve store active market (`tw` preferred via multi-page discovery, fallback to oldest active same-currency market).<br>`apps/admin/components/SettingsWizard.tsx:966-1035`: automatically create default `tw` market on zero-market policy save.<br>`apps/admin/components/SettingsWizard.tsx:1325-1345,1575-1590,1680-1695`: removed `settings-market-form` and market selector from Step 3; Step 3 is now "Delivery" section with real delivery controls. |
| `124483922473` (`gate (--browser-identity)`) / `TestBrowserSettingsWizardRealChain` | (a) older test asserting replaced UI | `tests/admin/settings-real.spec.ts:151-200,319,388,424`: rewritten older test asserting removed `settings-market-form` and button `Market and delivery` to signed frame (`Delivery`, crash/uncertainty recovery on zero-market save, all real clicks preserved, same coverage). |

STATUS: DONE
