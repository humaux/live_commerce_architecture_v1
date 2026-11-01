# RVFIX — PR #403 review P1-1 / P1-2

Evidence level: E0 (nothing run locally; CI is the only runner). Run ids: none yet (integrator pushes).

## Root cause
- P1-1: SettingsWizard sorted active same-currency markets by `created_at`, a field the Market DTO does not return, so the "oldest" pick degraded to discovery order (an arbitrary legacy market).
- P1-2: with no *active same-currency* market the zero-market path POSTed `code:"tw"` even if a `tw` row existed (inactive / other currency), violating UNIQUE(tenant_id,store_id,code).

## Change
- apps/admin/components/SettingsWizard.tsx: new `pickMarket` (draft market → active tw → sole active market → null), used at the render site and in `loadBase`; both sort heuristics deleted. Zero-market save refuses to POST when any `tw` row exists and sets error `market_code_taken`; `displayError` maps it to `c.twTaken`.
- apps/admin/lib/settings-copy.ts: `twTaken` in en / zh-CN / zh-TW.
- tests/admin/settings-v2.spec.ts: `marketSelectionAcceptance` (called from `settingsV2Acceptance`): b active tw, c single active, d ambiguous → create tw, inactive tw and USD tw → alert + zero writes. RED commit precedes the fix.
- output/ci-select-backend-browser/browser-ownership.json regenerated.
