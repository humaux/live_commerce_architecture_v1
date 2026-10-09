# Browser date/time audit

Base: `35abffcadaa1240888d291391712e1411cb820a3`. Scope: all runtime files in `apps/admin` and `apps/storefront`; excludes dependencies, build output and test fixtures. `all-date-candidates.txt` records 142 instant/clock/serialization candidates; `candidates.txt` includes local getters and Intl calls. Follow-up search covered local setters/getters, locale date/time methods, date/datetime inputs and today/window defaults. `final-runtime-candidates.txt` records the final scan.

| Producer | Finding and result |
| --- | --- |
| admin `ads-model.ts:424,431` | Campaign wall inputs and saved schedules used browser-local getters/constructors. Now shared `taipeiToInstant` / `instantToTaipei`; round-trip rejects normalized invalid dates. Real `formFromDraft` and `buildDraftInput` are exercised with independent literal UTC instants. |
| admin `Ads.tsx:53` | Merchant draft timestamps omitted timeZone. Now shared `displayTime`. |
| admin `Design.tsx:688` | Published version timestamp omitted timeZone. Now shared `displayTime`. |
| admin `ManualOrder.tsx:159` | Bank-transfer expiry used local `toLocaleString`. Now shared `displayTime`. |
| storefront `OrderHistory.tsx:166` | Server order timestamp omitted timeZone. Now shared `displayTime`. |
| storefront `ClaimLink.tsx:342` | Claim expiry omitted timeZone. Now shared `displayTime`. |
| storefront `BankTransfer.tsx:215,230` | Transfer deadline and reported payment display omitted timeZone. Now shared `displayTime`. |
| storefront `ShopChrome.tsx:195` | Copyright year used host `getFullYear`. Now year from shared `instantToTaipei`. |
| admin `ads-copy.ts` and `tests/admin/ads.spec.ts` | Three locale hints explicitly identify Asia/Taipei (UTC+8); real browser fixture fills Taipei wall values. |

## Already correct or intentionally retained

- Admin report `defaultReportWindow`, attribution page date defaults: shared Taipei day, UTC calendar subtraction. Finance `financeDay` explicitly shifts UTC+8 before slicing a UTC date; tested across midnight, leap day, year end and the default 30-day window in both foreign TZs.
- Promotions and Studio scheduling use shared Taipei conversion; OrderFlow pins `timeZone: Asia/Taipei`; CheckoutFlow and other merchant/order timestamp consumers already use shared format.
- BankTransfer input `localInput` and `new Date(paidAt).toISOString()` intentionally represent the buyer's own wall time as a reported payment instant. The component explicitly documents that meaning, and storefront-v2 §C / bank-transfer-contract require an ISO instant. This is distinct from a store business-day/default window; its paired input conversion is retained. Its deadline/proof display now follows store time.
- Date-only validators (orders-v2, attribution, ads request/model, customer/card-payments request, settlement range labels) use UTC calendar getters or explicit UTC date arithmetic; they do not derive a store day from an instant.
- RFC3339 validators/serialization, cookie/session expiry, countdowns, polling windows, label validity and retry timers compare epoch instants or elapsed durations; browser timezone does not change their meaning.

## Regression evidence

Final regression files run against unchanged production source at base35abffca: `red-all-UTC.log` and `red-all-America-Los_Angeles.log`, exit1, each37 tests /28 pass /9 expected failures. These include two schedule/copy cases and seven actual TSX producer cases. Root fixed source: `green-all-UTC.log` and `green-all-America-Los_Angeles.log`, exit0, each37/37. A first supplemental baseline run lacked Next dependencies; `baseline-loader-error-*.log` is retained and excluded from product-red evidence; frozen offline install repaired that fixture.

The TSX fixtures load real component source and the real shared formatter. They simulate mounted hook/view state and presentation shells; they certify no HTTP/auth or browser-click behavior. No BFF/client route/helper implementation was replaced or changed. The pre-existing claim suite now imports the real format module for its TSX seam. PROCESS §2.4 remains binding for HTTP/auth/client tests.
