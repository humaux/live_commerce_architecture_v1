# GEMINI-REPORT — Directed Fix of PR #403 (PARITY-NO-MARKET-FIX2)

Date: 2026-10-31
PR: #403 (branch: `unit/parity-no-market`)
Failing run: [41182542290](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41182542290)
Failing job: `124483922473` (`gate (--browser-identity)`)

## Summary of Changes

| Failing Job / Test | Class | What Changed (file:line) |
|---|---|---|
| `124483922473` / `TestBrowserSettingsWizardRealChain` | (b) product root cause | `apps/admin/lib/settings-copy.ts:13,25-32,102,112-119,180,190-197`: replaced merchant-visible market wording with delivery / setup copy across `en`, `zh-CN`, and `zh-TW`.<br>`apps/admin/components/SettingsWizard.tsx:143-165`: auto-resolve active market (`tw` preferred, fallback to oldest active same-currency market).<br>`apps/admin/components/SettingsWizard.tsx:402-458`: multi-page discovery in `loadBase` to locate `tw` market across cursors.<br>`apps/admin/components/SettingsWizard.tsx:739-810`: decouple `after` callback from lock release in `savePending` to allow sequential market-create and policy-save commands.<br>`apps/admin/components/SettingsWizard.tsx:832-845`: initialize `serviceObservation` baseline on market creation retry.<br>`apps/admin/components/SettingsWizard.tsx:966-1035`: automatically create default `tw` market (`{ code: "tw", name: "台灣", currency: store.currency }`) on zero-market policy save (AD-04/AD-05).<br>`apps/admin/components/SettingsWizard.tsx:1183`: `needsPolicySave` requires active market.<br>`apps/admin/components/SettingsWizard.tsx:1325-1345`: removed market selector and `settings-market-form` from Step 3; Step 3 is now "Delivery" section with real delivery controls.<br>`apps/admin/components/SettingsWizard.tsx:1575-1590,1680-1695`: update baseline guards for policy and service save.<br>`output/ci-select-backend-browser/browser-ownership.json:190`: updated hash for `tests/admin/settings-real.spec.ts`. |
| `124483922473` / `TestBrowserSettingsWizardRealChain` | (a) older test asserting replaced UI | `tests/admin/settings-real.spec.ts:151-200`: replaced assertions on removed `settings-market-form` with Step 3 delivery setup crash/retry test on unseeded market creation.<br>`tests/admin/settings-real.spec.ts:319,388,424`: replaced `getByRole("button", { name: "Market and delivery" })` with `getByRole("button", { name: "Delivery" })` matching signed Step 3 title. |

STATUS: DONE
