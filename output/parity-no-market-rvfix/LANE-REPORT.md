# RVFIX lane report

| Finding | Class | Change |
|---|---|---|
| P1-1 created_at "oldest" heuristic | a | SettingsWizard.tsx `pickMarket` replaces both sort blocks (render site + loadBase) |
| P1-2 POST tw over existing tw | a | SettingsWizard.tsx zero-market branch guard + `market_code_taken`; settings-copy.ts `twTaken` x3 locales |
| Tests per rule (b/c/d, inactive tw, usd tw) | a | tests/admin/settings-v2.spec.ts `marketSelectionAcceptance` (RED commit first) |
| Browser ownership | a | regenerated browser-ownership.json |

Nothing was run (E0). P2s untouched per card.

STATUS: DONE
