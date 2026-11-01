# RVFIX-366 relay 1 lane report

| failing job / finding | class | change |
|---|---|---|
| CI a12f5391b run 41237371015: gate bundle:05 member `--browser-promotions` FAIL; receipts + required red as consequence | a (expected RED on new assertions) | apps/admin/components/Promotions.tsx:39-60 reads ads settings+drafts; :72-95 renders off frame only when server says off, drafts table when enabled, loading/error otherwise |
| P2-1 codes view lost on reload; Refresh re-read promotions | a | Promotions.tsx:39-41 view in URL via router; :94 Refresh -> ads.reload; app/[locale]/promotions/page.tsx accepts `view=codes` |

Not run locally (owner rule); GREEN pending next-head CI. WAIT_RUN
STATUS: DONE
