# FIX-COD-422 lane report

| Finding | Class | Change |
|---|---|---|
| Bar label shows 最終應付 under COD | a | CheckoutFlow.tsx:697 |
| Mode-save failure hidden | a | OrderFlow.tsx:186, :551 `setConfirmed(null)` |
| Misleading comment | a | OrderFlow.tsx:231-235 |

Nothing run locally (CI only). The RED test also wants a disabled submit with an aria-describedby reason after a save failure; the card does not cover that. If CI still fails on it, that is the next fix.

STATUS: DONE
