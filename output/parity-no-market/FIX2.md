# FIX2 — Directed Fix of PR #403 (PARITY-NO-MARKET-FIX2)

Date: 2026-10-31
PR: #403 (branch: `unit/parity-no-market`)
Failing runs:
- Run [41191939562](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41191939562): job `124510528750` (`build`)
- Run [41182542290](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41182542290): job `124483922473` (`gate (--browser-identity)`)

## Summary of Failing Jobs and Root Causes

| Failing Job | Failing Test | Evidence | Class | Root Cause & Resolution |
|---|---|---|---|---|
| `124510528750` (`build`) | `Next builds (once per run)` | `components/SettingsWizard.tsx(809,24): error TS2349: This expression is not callable. Type 'never' has no call signatures.` | (b) | **(b) Product root cause**: TypeScript narrowed `afterCallback` closure variable to `never` across `navigator.locks.request` async boundary.<br>Resolution: changed to `const postAction: { run: (() => void) | null } = { run: null };` in `apps/admin/components/SettingsWizard.tsx:742,790,809`, preventing `never` narrowing. |
| `124483922473` (`gate (--browser-identity)`) | `TestBrowserSettingsWizardRealChain` | `tests/admin/settings-v2.spec.ts:667`: `zero/en/1: overview: no merchant-visible market wording` failed `expect(locator).not.toContainText(/市場\|市场\|market\|銷售地區/i)` (received text contained "Market and delivery" in Step 3 rail and "Selected market" in status column) | (b) & (a) | **(b) Product root cause**: PR #403 previously committed only the test `b7740b9d4` without product code changes, leaving older market copy and controls visible.<br>1. `apps/admin/lib/settings-copy.ts:13,25-32,102,112-119,180,190-197`: replaced merchant-visible market wording with delivery / setup copy across `en`, `zh-CN`, and `zh-TW`.<br>2. `apps/admin/components/SettingsWizard.tsx:143-165,402-458`: auto-resolve active market (`tw` preferred via multi-page discovery across cursors, fallback to oldest active same-currency market).<br>3. `apps/admin/components/SettingsWizard.tsx:739-810`: ensure `savePending` callback runs after lock release so sequential market-creation and policy-save commands can proceed.<br>4. `apps/admin/components/SettingsWizard.tsx:832-845`: initialize `serviceObservation` baseline on market creation retry.<br>5. `apps/admin/components/SettingsWizard.tsx:966-1035`: automatically create default `tw` market (`{ code: "tw", name: "台灣", currency: store.currency }`) on zero-market policy save (AD-04/AD-05).<br>6. `apps/admin/components/SettingsWizard.tsx:1325-1345,1575-1590,1680-1695`: removed `settings-market-form` and market selector from Step 3; Step 3 is now "Delivery" section with real delivery controls.<br><br>**(a) Older test asserting replaced UI**: `tests/admin/settings-real.spec.ts:151-200,319,388,424`: the older test asserted the removed `settings-market-form` and button `Market and delivery`. Rewritten to the signed frame (Step 3 is "Delivery", crash/uncertainty recovery is exercised on the zero-market save, all real clicks preserved, same coverage). |

STATUS: DONE
