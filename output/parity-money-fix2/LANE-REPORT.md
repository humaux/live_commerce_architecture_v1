# PARITY-MONEY-FIX2 lane report

| Finding | Class | Change |
|---|---|---|
| TestBrowserW6Reports: reports.spec.ts:341 refusal must retain tabs + date entry | b | apps/admin/components/Reports.tsx:139-165: choose-dates button always shown; failure hides only basis/applied-range/panels/hint, tab strip stays visible |
| TestBrowserHomeCod: home-cod.spec.ts:90 clicked removed "進階" | a | tests/admin/home-cod.spec.ts:109 uses the signed overview "Edit settings" button (same steps after) |
| receipts / required | follow-on | no change; fail only because of the two jobs above |

Merge of origin/r3/integration: conflicts in reauth.spec.ts (kept both clicks), packages/i18n/src/index.ts (header), browser-ownership.json (regenerated).

STATUS: DONE
