# PARITY-CATALOG-PEOPLE-FIX2 — single batch prepared; CI pending

2026-10-31. Author assessment E0; static checks E1; original tests-only RED E3 failure evidence only. No GREEN or completed implementation claim.

## Current authority and action

- Latest integrator ruling (2026-10-31, dispatch message) authorizes only K2 Copy entry + confirmation/success, retaining the existing copy endpoint. Remaining K2 stays on PARITY-K2.
- Fresh fetch/merge: Merged origin/r3/integration at `acf2ebd2c6a19dd731aa98acce509b45629b1f77` (merge commit `c03a7b79b`); prior merge `ec39428e4360f09df042d073429b80d00a96f5dd` preserved. Base Gates 41107905131: no class (c) exemption.
- Live Fleet Bridge status one connected document. Read signed hub1469:140499: source `1469:140597`「複製商品」ON_CLICK NAVIGATE →1469:141623. Confirmation1469:141623/141703 and success1469:141754/141832 raw metadata in fix2-evidence/k2-*.json; inspected confirmation desktop and success mobile screenshots. No Figma writes.
- People correction verified live: signed entry1469:157628「標籤與紀錄」→1469:142986. Preserve the existing tag/note/history controls under that signed label rather than Advanced; native disclosure helpers still exercise them.
- Re-read native original-run G09 job124070683720 failed log: copy-ruling-g09.log1507–1516 exact DTO/never failures,1538 map[] want marketing=granted. Browser124070684239 failed log confirms TestBrowserCustomersBilling at2503. Original detailed browser artifact and every assertion table retained below.

## Implementation and same-coverage migrations

- ProductList removes unsigned Search/Advanced, applies signed chips/selection labels and keeps incumbent bulk commands and inventory navigation reachable. Product tasks → Copy product opens the signed source/version review. Cancel issues no command. Confirm uses existing fenced command and expected_version. Success displays only returned draft id/name and links exactly that id. Changed/denied guarded reads conceal scoped snapshots. Existing ambiguous-command recovery remains unchanged.
- Shared selectProductForCopy now clicks selection → Product tasks → Copy product → confirmation. product-review acceptance keeps its original response-loss/refused-retry/key/body/count assertions; settings-v2 calls the same helper for both competing-draft isolation scenarios. Product-editor recovery still requires Copy to be disabled after reload. No security assertion or test was removed/skipped.
- Class(a): catalog-core's incumbent inventory navigation opens Product tasks; former search fields used to scope isolated fixtures become read-only q URL preparation. Status filters, paging, row facts, Copy, bulk writes and reload assertions remain native clicks. No fixture rewritten, API write substituted, threshold or exemption altered. Existing signed Search/Advanced-absent tests stay intact.
- New additive Copy cases cover three locales × two widths: no source → disabled, cancellation → zero commands, confirmation → one keyed exact-version command, acknowledged new identity/name, refresh → persisted list row, and Chinese mobile review ≤12 blocks/100 characters. CI executes these; local runtime NOT_RUN.
- Earlier unpushed consent-state/privacy/editor root fixes remain included. Their original REDs and exact paths are below; their correctness is not certified by this static handoff.

## Checks and boundaries

|Command|Exit|Evidence/limit|
|---|---|---|
|git fetch + merge origin/r3/integration|0|Already up to date|
|node scripts/dev/derive-browser-ownership.mjs; --check|0|54 modes; generated file only|
|bash scripts/dev/depmap.sh|0|Generated dependency map unchanged|
|python3 scripts/check_packet.py|0|fix2-evidence/static-packet.log; packet structure only|
|go vet ./internal/customers|0|fix2-evidence/static-vet.log; no tests run|
|tsc --noEmit --strict --target ES2023 --module ESNext --moduleResolution bundler --allowImportingTsExtensions packages/i18n/src/catalog.ts packages/i18n/src/people.ts|0|fix2-evidence/tsc-copy.log; actual changed three-language copy|
|Full/scoped admin tsc attempts|1|Local dependency/alias setup unresolved; tsc-admin.log, tsc-scoped.log. NodeNext whole-i18n check also hits existing state-recovery.ts extension error. No source workaround or gate weakening; CI remains authority.|
|git diff --check|0|Whitespace only|

Imports/harness inventory: no test harness compiles ProductList.tsx by its literal name; CustomerDetail/Customers harnesses use existing CSS handling / AST extraction, no new runtime globals. Settings imports the existing native driver (no top-level registration). No added/removed browser spec file or route, ownership regenerated for modified source hashes anyway.

Outside initial card write paths carried from original RED/partial: `internal/customers/types.go`, `internal/customers/read.go`, `internal/customers/types_test.go`, `tests/foundation/customers_billing_http_test.go`, `tests/foundation/customers_billing_list_test.go` — integrator scope approval still required. Additional acceptance callers in this batch: `tests/admin/catalog-core.spec.ts`, `catalog-document-driver.ts`, `product-editor.acceptance.ts`, `settings-v2.spec.ts`; all listed in PR. No migrations, contracts/OpenAPI, lockfiles, R2 pin, contractdrift baseline, route dispatchers or next-startup gate assertions changed. No subagents, production actions, merge/automerge, force push or commit statuses.

NOT_RUN: local tests/servers(forbidden); implementation CI/GREEN pending push; browser clicks/persistence/parity/density, REAL_PG runtime, independent E4 and LIVE. Own temporary dependency links are removed before handoff; no process started.

Wake-up: record exact PR-head Gates/Foundation workflow ids after the one batch push; integrator resumes on completion/failure/cancellation. Do not poll >10min; queue >3h escalates. If the same test remains red after this directed batch, BLOCKED with exact assertion; no third approach.

## Every failed job → test → evidence → class

Original Gates run `41054099331`, tests-only head6e2cbdb1; synthetic merge `c99fdd96001375527e6ec1a03bffc7fb2ceee4e6`. Live `gh pr checks363` lists six failures (`fix2-evidence/pr-checks.log`). Original failures below do not certify local implementation.

|Job|Failing test|Evidence|Class / disposition|
|---|---|---|---|
|124070683720 shard:g09|CB03List / PARITY_consent_state_distinguishes_never_withdrawn_and_regranted|Fresh `fix2-evidence/job-124070683720-excerpt.log`: list_test.go187 got map[] want marketing=granted|(b) missing product read projection; prior local types/read implementation retained, current-head CI NOT_RUN.|
|124070683720 shard:g09|CB09HTTP / GET_customers permissions, grammar, shape, paging, method|Fresh log: http_test.go191/195 list exact key consent_state absent; absent events map[]|(b) same projection root; preserve exact-key/permission assertions.|
|124070683720 shard:g09|CB09HTTP / GET_customers/{id} shape, permissions, cross-store|Fresh log: http_test.go245 detail exact key consent_state absent|(b) same projection root; preserve cross-store coverage.|
|124070684122 shard:unit|TestParityConsentStateProjection / never, withdrawn, granted, regranted|`fix2-evidence/job-124070684122-retained-excerpt.log`: types_test.go388 four map[] failures|(b) same missing DTO/state root; no assertion change.|
|124070683762 bundle:02|TestBrowserCatalogCore / PE12-17-TWD; PARITY catalog signed controls zh-TW390|Retained original artifact `../PARITY-CATALOG-PEOPLE/fix1-evidence/editor/playwright/run.h2UveQ4r/catalog-core/20261030T132707.221775956/playwright.log:106`: unsigned search removed, Expected0 Received1|(b) signed product removal missing; fixing it exposes retained native Copy gate conflict below.|
|124070684239 bundle:05|TestBrowserCustomersBilling; --browser-customers-billing|Detailed 46 original browser assertions below|(b) missing signed controls/outcomes, actual read-only GET lookup, server consent fact and permission fence; prior partial repair unverified.|
|124100320890 receipts|unit, g09, customers-billing, product-editor receipts status FAIL|`fix2-evidence/job-124100320890-retained-excerpt.log`|Dependent on four failing jobs; no standalone missing browser-spec marker shown.|
|124103568945 required|gate=failure, receipts=failure; aggregate=success|`fix2-evidence/job-124103568945-retained-excerpt.log`|Dependent only; aggregate is successful.|

Fresh `gh run view --log-failed` calls returned502. Native jobs/logs API returned the G09 log successfully; browser jobs/logs returned503. Fresh browser artifact download failed503 “Egress is over the account limit”. The table explicitly identifies retained exact-run artifacts; they are original RED evidence, not current-state acceptance. No failed transport is treated as CI failure or success. Details in `fix2-evidence/transport.log`.

### All browser assertion failures in job124070684239

Retained exact-run artifact: `output/PARITY-CATALOG-PEOPLE/fix1-evidence/customers/playwright/run.z5FBSQSR/customers-billing/20261030T133642.454918607/playwright.log`. Original artifact gate-browser-customers-billing id12982324428. Each row below comes from its assertion block; class(b) in every case.

|Test|Exact assertion / observed result|Artifact log line|Class|
|---|---|---|---|
|tests/admin/people-v2.spec.ts:272:3 › PARITY people signed controls zh-TW-1440|PARITY-PEOPLE: unsigned list disclosure removed; Expected: 0; Received: 1|84|(b)|
|tests/admin/people-v2.spec.ts:296:5 › PARITY people privacy none zh-TW-1440|PARITY-PEOPLE: signed dedicated outcome; Expected: visible|125|(b)|
|tests/admin/people-v2.spec.ts:296:5 › PARITY people privacy limit zh-TW-1440|PARITY-PEOPLE: signed dedicated outcome; Expected: visible|164|(b)|
|tests/admin/people-v2.spec.ts:296:5 › PARITY people privacy blocked zh-TW-1440|PARITY-PEOPLE: signed dedicated outcome; Expected: visible|203|(b)|
|tests/admin/people-v2.spec.ts:296:5 › PARITY people privacy read-export zh-TW-1440 › 查看待處理要求: read existing request without repeating POST|PARITY-PEOPLE: pending lookup actually reads server; Expected: > 1; Received:   1|242|(b)|
|tests/admin/people-v2.spec.ts:296:5 › PARITY people privacy read-erase zh-TW-1440 › 查看待處理要求: read existing request without repeating POST|PARITY-PEOPLE: pending lookup actually reads server; Expected: > 1; Received:   1|286|(b)|
|tests/admin/people-v2.spec.ts:296:5 › PARITY people privacy erase-success zh-TW-1440|PARITY-PEOPLE: signed dedicated outcome; Expected: visible|330|(b)|
|tests/admin/people-v2.spec.ts:272:3 › PARITY people signed controls zh-TW-390|PARITY-PEOPLE: unsigned list disclosure removed; Expected: 0; Received: 1|369|(b)|
|tests/admin/people-v2.spec.ts:296:5 › PARITY people privacy none zh-TW-390|PARITY-PEOPLE: signed dedicated outcome; Expected: visible|410|(b)|
|tests/admin/people-v2.spec.ts:296:5 › PARITY people privacy limit zh-TW-390|PARITY-PEOPLE: signed dedicated outcome; Expected: visible|449|(b)|
|tests/admin/people-v2.spec.ts:296:5 › PARITY people privacy blocked zh-TW-390|PARITY-PEOPLE: signed dedicated outcome; Expected: visible|488|(b)|
|tests/admin/people-v2.spec.ts:296:5 › PARITY people privacy read-export zh-TW-390 › 查看待處理要求: read existing request without repeating POST|PARITY-PEOPLE: pending lookup actually reads server; Expected: > 1; Received:   1|527|(b)|
|tests/admin/people-v2.spec.ts:296:5 › PARITY people privacy read-erase zh-TW-390 › 查看待處理要求: read existing request without repeating POST|PARITY-PEOPLE: pending lookup actually reads server; Expected: > 1; Received:   1|571|(b)|
|tests/admin/people-v2.spec.ts:296:5 › PARITY people privacy erase-success zh-TW-390|PARITY-PEOPLE: signed dedicated outcome; Expected: visible|615|(b)|
|tests/admin/people-v2.spec.ts:272:3 › PARITY people signed controls zh-CN-1440|PARITY-PEOPLE: unsigned list disclosure removed; Expected: 0; Received: 1|654|(b)|
|tests/admin/people-v2.spec.ts:296:5 › PARITY people privacy none zh-CN-1440|PARITY-PEOPLE: signed dedicated outcome; Expected: visible|695|(b)|
|tests/admin/people-v2.spec.ts:296:5 › PARITY people privacy limit zh-CN-1440|PARITY-PEOPLE: signed dedicated outcome; Expected: visible|734|(b)|
|tests/admin/people-v2.spec.ts:296:5 › PARITY people privacy blocked zh-CN-1440|PARITY-PEOPLE: signed dedicated outcome; Expected: visible|773|(b)|
|tests/admin/people-v2.spec.ts:296:5 › PARITY people privacy read-export zh-CN-1440 › 查看待处理申请: read existing request without repeating POST|PARITY-PEOPLE: pending lookup actually reads server; Expected: > 1; Received:   1|812|(b)|
|tests/admin/people-v2.spec.ts:296:5 › PARITY people privacy read-erase zh-CN-1440 › 查看待处理申请: read existing request without repeating POST|PARITY-PEOPLE: pending lookup actually reads server; Expected: > 1; Received:   1|856|(b)|
|tests/admin/people-v2.spec.ts:296:5 › PARITY people privacy erase-success zh-CN-1440|PARITY-PEOPLE: signed dedicated outcome; Expected: visible|900|(b)|
|tests/admin/people-v2.spec.ts:272:3 › PARITY people signed controls zh-CN-390|PARITY-PEOPLE: unsigned list disclosure removed; Expected: 0; Received: 1|939|(b)|
|tests/admin/people-v2.spec.ts:296:5 › PARITY people privacy none zh-CN-390|PARITY-PEOPLE: signed dedicated outcome; Expected: visible|980|(b)|
|tests/admin/people-v2.spec.ts:296:5 › PARITY people privacy limit zh-CN-390|PARITY-PEOPLE: signed dedicated outcome; Expected: visible|1019|(b)|
|tests/admin/people-v2.spec.ts:296:5 › PARITY people privacy blocked zh-CN-390|PARITY-PEOPLE: signed dedicated outcome; Expected: visible|1058|(b)|
|tests/admin/people-v2.spec.ts:296:5 › PARITY people privacy read-export zh-CN-390 › 查看待处理申请: read existing request without repeating POST|PARITY-PEOPLE: pending lookup actually reads server; Expected: > 1; Received:   1|1097|(b)|
|tests/admin/people-v2.spec.ts:296:5 › PARITY people privacy read-erase zh-CN-390 › 查看待处理申请: read existing request without repeating POST|PARITY-PEOPLE: pending lookup actually reads server; Expected: > 1; Received:   1|1141|(b)|
|tests/admin/people-v2.spec.ts:296:5 › PARITY people privacy erase-success zh-CN-390|PARITY-PEOPLE: signed dedicated outcome; Expected: visible|1185|(b)|
|tests/admin/people-v2.spec.ts:272:3 › PARITY people signed controls en-1440|PARITY-PEOPLE: unsigned list disclosure removed; Expected: 0; Received: 1|1224|(b)|
|tests/admin/people-v2.spec.ts:296:5 › PARITY people privacy none en-1440|PARITY-PEOPLE: signed dedicated outcome; Expected: visible|1265|(b)|
|tests/admin/people-v2.spec.ts:296:5 › PARITY people privacy limit en-1440|PARITY-PEOPLE: signed dedicated outcome; Expected: visible|1304|(b)|
|tests/admin/people-v2.spec.ts:296:5 › PARITY people privacy blocked en-1440|PARITY-PEOPLE: signed dedicated outcome; Expected: visible|1343|(b)|
|tests/admin/people-v2.spec.ts:296:5 › PARITY people privacy read-export en-1440 › View pending request: read existing request without repeating POST|PARITY-PEOPLE: pending lookup actually reads server; Expected: > 1; Received:   1|1382|(b)|
|tests/admin/people-v2.spec.ts:296:5 › PARITY people privacy read-erase en-1440 › View pending request: read existing request without repeating POST|PARITY-PEOPLE: pending lookup actually reads server; Expected: > 1; Received:   1|1426|(b)|
|tests/admin/people-v2.spec.ts:296:5 › PARITY people privacy erase-success en-1440|PARITY-PEOPLE: signed dedicated outcome; Expected: visible|1470|(b)|
|tests/admin/people-v2.spec.ts:272:3 › PARITY people signed controls en-390|PARITY-PEOPLE: unsigned list disclosure removed; Expected: 0; Received: 1|1509|(b)|
|tests/admin/people-v2.spec.ts:296:5 › PARITY people privacy none en-390|PARITY-PEOPLE: signed dedicated outcome; Expected: visible|1550|(b)|
|tests/admin/people-v2.spec.ts:296:5 › PARITY people privacy limit en-390|PARITY-PEOPLE: signed dedicated outcome; Expected: visible|1589|(b)|
|tests/admin/people-v2.spec.ts:296:5 › PARITY people privacy blocked en-390|PARITY-PEOPLE: signed dedicated outcome; Expected: visible|1628|(b)|
|tests/admin/people-v2.spec.ts:296:5 › PARITY people privacy read-export en-390 › View pending request: read existing request without repeating POST|PARITY-PEOPLE: pending lookup actually reads server; Expected: > 1; Received:   1|1667|(b)|
|tests/admin/people-v2.spec.ts:296:5 › PARITY people privacy read-erase en-390 › View pending request: read existing request without repeating POST|PARITY-PEOPLE: pending lookup actually reads server; Expected: > 1; Received:   1|1711|(b)|
|tests/admin/people-v2.spec.ts:296:5 › PARITY people privacy erase-success en-390|PARITY-PEOPLE: signed dedicated outcome; Expected: visible|1755|(b)|
|tests/admin/people-v2.spec.ts:346:3 › PARITY withdrawn list is a server fact 1440|PARITY-CONSENT-STATE: signed list distinguishes recorded withdrawal; |1794|(b)|
|tests/admin/people-v2.spec.ts:352:3 › PARITY privacy permission refusal 1440|PARITY-PEOPLE: denied privacy commands stay disabled; Expected: disabled; Received: enabled|1833|(b)|
|tests/admin/people-v2.spec.ts:346:3 › PARITY withdrawn list is a server fact 390|PARITY-CONSENT-STATE: signed list distinguishes recorded withdrawal; |1874|(b)|
|tests/admin/people-v2.spec.ts:352:3 › PARITY privacy permission refusal 390|PARITY-PEOPLE: denied privacy commands stay disabled; Expected: disabled; Received: enabled|1913|(b)|

## Relay 3 CI run 41173763994 on head 0c7f9b37d — failing job → test → evidence → class

|Job|Failing test|Evidence|Class / disposition|
|---|---|---|---|
|`124458076046` bundle:03|`TestBrowserCatalogCore/PE12-17-TWD` / `catalog-core.spec.ts >> review: copy UNKNOWN then client/401/403 preserves receipt fence after reload`|`playwright.log:71` (`locator.toBeDisabled: Error: element not found: getByTestId('products-copy-selected')`)|(a) Older test asserts UI that signed Figma frame intentionally relocated. Confirmation button `products-copy-selected` is inside modal `product-copy-review`. After reload to list, the disabled control is `products-copy-entry`. Updated `tests/admin/product-review.acceptance.ts:226`.|
|`124458077108` bundle:06|`TestBrowserInboxUIRealChain` (`tests/admin/inbox-ui.spec.ts`, tests 8, 10, 11, 14, 17, 18)|`playwright.log:127` (`locator.click: Test timeout of 30000ms exceeded waiting for getByRole('button', { name: 'Open navigation', exact: true })`)|(b) Shared browser test helper race. In `tests/admin/inbox-browser-support.ts:77`, `if (!await nav.isVisible())` evaluates before desktop DOM layout settles, falsely triggering mobile navigation opener which does not exist on desktop. Replaced with `try { await expect(nav).toBeVisible({ timeout: 500 }); } catch { await page.getByRole("button", { name: "Open navigation", exact: true }).click(); }`.|
|`124458077587` --browser-click-sweep@4/10|`journey J1 product editor -> storefront desktop/zh-TW [journey] "the merchant list shows the product (search + click)"`|`click-sweep.mjs.log:43` (`locator.fill: Timeout 30000ms exceeded waiting for getByTestId('products-search')`)|(a) Older test asserts unsigned search removed by signed Figma frame (`catalog-v2-simplified.acceptance.ts:218`). In `tests/ui/click-sweep.mjs:444-447`, replaced `products-search` fill/submit with real status tab click `products-tab-active`.|
|`124459434850` receipts|`bundle:03`, `bundle:06`, `click-sweep@4/10` status FAIL|`job/124459434850`|Dependent on the three failing jobs above.|
|`124459711685` required|`gate=failure`, `receipts=failure`|`job/124459711685`|Dependent aggregate job.|
|`124458078696` aggregate|Aggregate check failure|`job/124458078696`|Dependent aggregate job.|


