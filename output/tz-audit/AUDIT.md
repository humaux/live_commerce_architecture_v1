# Browser-local business date audit
Snapshot base 3807a3f8 (639c30ff already present); runtime candidates recorded in audit-candidates.json.
Coverage: 235 admin and 125 storefront TS/TSX/JS/JSX files; date patterns in 15 admin files/21 lines and 8 storefront files/12 lines on the final source snapshot. Expanded scan also checked local getFullYear/getMonth/setDate and Date.now consumers for day/range usage.

| Candidate | Finding / disposition |
|---|---|
| ads-model localToEpoch + epochToLocal; formOf and buildDraftInput callers | Business campaign start/end interval used browser wall time (old copy explicitly said local time). Owner's fixed Asia/Taipei policy now applies; both input and edit values use packages/format. UTC wire, 10-minute lead, 30-day cap unchanged. Three locale hints and real-click fixture updated. |
| AdsResults/defaultReportWindow | Already fixed on639c30ff; existing midnight/year-boundary TZ-cycle regression retained. |
| finance/page and customers-model financeDay | Already fixed UTC+8 day arithmetic; four-TZ midnight/leap/year 30-day default-range regression added without changing implementation. |
| ads/attribution/page | Converts current instant to Taipei day before UTC calendar arithmetic; correct. |
| Studio scheduled input | Existing Taipei helper plus calendar roundtrip; correct. |
| customers-request, orders-v2, ads-request/model validDate, attribution-model, studio-model, orders-model | UTC calendar/instant validation; does not derive the viewer's business day. |
| auth/password step1 | Cookie/time formatting input is an instant, not a business-day default. |
| Ads, Design, ManualOrder date displays | Absolute timestamp display; no today/business-day/range derivation. Display localization was kept outside this narrow audit. |
| storefront BankTransfer, OrderHistory, ClaimLink | Buyer-local proof-input/display of absolute instants; no store business-day/report window. No change. |
| storefront OrderFlow | Expiry display explicitly Asia/Taipei. |
| storefront payment-contract/ad-touch/buyer-server/purchase/order-payment | Absolute instants, UTC validation, token TTL/expiry/countdown; no business-day slicing. |
| storefront ShopChrome | Copyright year only, not a business day. |

Read-only storefront explorer (gpt-6-luna medium) reported the scoped no-business-day-bug finding; independent reviewer (gpt-6.1-sol high) checked the five changed files and reran 25 focused tests, exit0, no scope P0/P1/P2. BROWSER/PG/platform runtime is NOT_RUN.
