Product/customer pages still expose unsigned controls and generic privacy outcomes. This PR starts with one tests-only RED batch against the real product/customer routes and adds signed-frame controls, three-locale coverage, per-frame screenshots and a read-only consent-state contract test. Implementation follows only after CI demonstrates assertion RED.

Base: r3/integration `3ea359f79`. Evidence E1 static checks, E0 author conclusion; RED/GREEN pending. MOCK browser fixtures; REAL_PG Go tests are CI-only, not yet run.

Integrator ruling: stored withdrawal events exist (0078/0139). RED covers additive `consent_state` (granted/withdrawn/never), not false-as-withdrawn. No migration picked. No source/API/contract implementation in the initial push.

Validation: Go vet customers/foundation, gofmt, diff check, packet structure checker, generated browser ownership and --check exit 0. Local tests/servers not run. tsc unavailable in the worktree (no dependencies); root compiler failed configuration/type resolution, so TypeScript compilation remains NOT_RUN.

Outside initial write paths (integrator review): `internal/customers/types_test.go`, `tests/foundation/customers_billing_http_test.go`, `tests/foundation/customers_billing_list_test.go` under the explicit consent ruling; generated `output/ci-select-backend-browser/browser-ownership.json`; evidence `output/PARITY-CATALOG-PEOPLE/**`. No migration, lockfile, protected baseline, other module or production change.

CI receipts: pending allocation; run/job IDs will be linked here/commented, not claimed from author checks. No merge or auto-merge.

Parity sheet (artifact paths describe captures produced by CI, currently NOT_RUN; DELIVERY paths are relative to each designated design worktree):

|Signed frame|Real-route browser artifact|Signed DELIVERY PNG|
|---|---|---|
|1429:62828 (list)|`**/parity/1429-62828.png`|`ux-catalog/output/ux-catalog/shots-simple/get_screenshot-1429-62828-1793195992798-0.png`|
|1429:62906 (list)|`**/parity/1429-62906.png`|`ux-catalog/output/ux-catalog/shots-simple/get_screenshot-1429-62906-1793195993186-0.png`|
|1429:63218 (active)|`**/parity/1429-63218.png`|`ux-catalog/output/ux-catalog/shots-simple/get_screenshot-1429-63218-1793195993550-0.png`|
|1429:63287 (active)|`**/parity/1429-63287.png`|`ux-catalog/output/ux-catalog/shots-simple/get_screenshot-1429-63287-1793195993898-0.png`|
|1429:63348 (draft)|`**/parity/1429-63348.png`|`ux-catalog/output/ux-catalog/shots-simple/get_screenshot-1429-63348-1793195994253-0.png`|
|1429:63405 (draft)|`**/parity/1429-63405.png`|`ux-catalog/output/ux-catalog/shots-simple/get_screenshot-1429-63405-1793195994730-0.png`|
|1429:63453 (archived)|`**/parity/1429-63453.png`|`ux-catalog/output/ux-catalog/shots-simple/get_screenshot-1429-63453-1793195995096-0.png`|
|1429:63499 (archived)|`**/parity/1429-63499.png`|`ux-catalog/output/ux-catalog/shots-simple/get_screenshot-1429-63499-1793195995382-0.png`|
|1429:63540 (selected)|`**/parity/1429-63540.png`|`ux-catalog/output/ux-catalog/shots-simple/get_screenshot-1429-63540-1793195995729-0.png`|
|1429:63620 (selected)|`**/parity/1429-63620.png`|`ux-catalog/output/ux-catalog/shots-simple/get_screenshot-1429-63620-1793195996083-0.png`|
|1429:63693 (bulkReview)|`**/parity/1429-63693.png`|`ux-catalog/output/ux-catalog/shots-simple/get_screenshot-1429-63693-1793195996466-0.png`|
|1429:63740 (bulkReview)|`**/parity/1429-63740.png`|`ux-catalog/output/ux-catalog/shots-simple/get_screenshot-1429-63740-1793195996718-0.png`|
|1429:63782 (bulkResult)|`**/parity/1429-63782.png`|`ux-catalog/output/ux-catalog/shots-simple/get_screenshot-1429-63782-1793195997029-0.png`|
|1429:63827 (bulkResult)|`**/parity/1429-63827.png`|`ux-catalog/output/ux-catalog/shots-simple/get_screenshot-1429-63827-1793195997230-0.png`|
|1429:63867 (updated)|`**/parity/1429-63867.png`|`ux-catalog/output/ux-catalog/shots-simple/get_screenshot-1429-63867-1793195997486-0.png`|
|1429:63937 (updated)|`**/parity/1429-63937.png`|`ux-catalog/output/ux-catalog/shots-simple/get_screenshot-1429-63937-1793195997694-0.png`|
|1429:64000 (edit)|`**/parity/1429-64000.png`|`ux-catalog/output/ux-catalog/shots-simple/get_screenshot-1429-64000-1793195997935-0.png`|
|1429:64049 (edit)|`**/parity/1429-64049.png`|`ux-catalog/output/ux-catalog/shots-simple/get_screenshot-1429-64049-1793195998136-0.png`|
|1429:64093 (variants)|`**/parity/1429-64093.png`|`ux-catalog/output/ux-catalog/shots-simple/get_screenshot-1429-64093-1793195998403-0.png`|
|1429:64149 (variants)|`**/parity/1429-64149.png`|`ux-catalog/output/ux-catalog/shots-simple/get_screenshot-1429-64149-1793195998593-0.png`|
|1429:64200 (stock)|`**/parity/1429-64200.png`|`ux-catalog/output/ux-catalog/shots-simple/get_screenshot-1429-64200-1793195998853-0.png`|
|1429:64239 (stock)|`**/parity/1429-64239.png`|`ux-catalog/output/ux-catalog/shots-simple/get_screenshot-1429-64239-1793195999048-0.png`|
|1429:64273 (stockError)|`**/parity/1429-64273.png`|`ux-catalog/output/ux-catalog/shots-simple/get_screenshot-1429-64273-1793195999391-0.png`|
|1429:64314 (stockError)|`**/parity/1429-64314.png`|`ux-catalog/output/ux-catalog/shots-simple/get_screenshot-1429-64314-1793195999638-0.png`|
|1429:64350 (media)|`**/parity/1429-64350.png`|`ux-catalog/output/ux-catalog/shots-simple/get_screenshot-1429-64350-1793195999962-0.png`|
|1429:64391 (media)|`**/parity/1429-64391.png`|`ux-catalog/output/ux-catalog/shots-simple/get_screenshot-1429-64391-1793196000468-0.png`|
|1429:64427 (review)|`**/parity/1429-64427.png`|`ux-catalog/output/ux-catalog/shots-simple/get_screenshot-1429-64427-1793196000756-0.png`|
|1429:64474 (review)|`**/parity/1429-64474.png`|`ux-catalog/output/ux-catalog/shots-simple/get_screenshot-1429-64474-1793196000959-0.png`|
|1429:64516 (saved)|`**/parity/1429-64516.png`|`ux-catalog/output/ux-catalog/shots-simple/get_screenshot-1429-64516-1793196001211-0.png`|
|1429:64561 (saved)|`**/parity/1429-64561.png`|`ux-catalog/output/ux-catalog/shots-simple/get_screenshot-1429-64561-1793196001412-0.png`|
|1429:44136 (list)|`**/parity/1429-44136.png`|`ux-people/output/ux-people/shots-simple/1429-44136.png`|
|1429:44240 (list)|`**/parity/1429-44240.png`|`ux-people/output/ux-people/shots-simple/1429-44240.png`|
|1429:44287 (search)|`**/parity/1429-44287.png`|`ux-people/output/ux-people/shots-simple/1429-44287.png`|
|1429:44349 (search)|`**/parity/1429-44349.png`|`ux-people/output/ux-people/shots-simple/1429-44349.png`|
|1429:44396 (profile)|`**/parity/1429-44396.png`|`ux-people/output/ux-people/shots-simple/1429-44396.png`|
|1429:44485 (profile)|`**/parity/1429-44485.png`|`ux-people/output/ux-people/shots-simple/1429-44485.png`|
|1429:44564 (forbidden)|`**/parity/1429-44564.png`|`ux-people/output/ux-people/shots/forbidden-Desktop.png`|
|1429:44654 (forbidden)|`**/parity/1429-44654.png`|`ux-people/output/ux-people/shots/forbidden-Mobile.png`|
|1429:44734 (export-confirm)|`**/parity/1429-44734.png`|`ux-people/output/ux-people/shots-simple/1429-44734.png`|
|1429:44772 (export-confirm)|`**/parity/1429-44772.png`|`ux-people/output/ux-people/shots-simple/1429-44772.png`|
|1429:44801 (export-unknown)|`**/parity/1429-44801.png`|`ux-people/output/ux-people/shots-simple/1429-44801.png`|
|1429:44842 (export-unknown)|`**/parity/1429-44842.png`|`ux-people/output/ux-people/shots-simple/1429-44842.png`|
|1429:44874 (export-read-error)|`**/parity/1429-44874.png`|`ux-people/output/ux-people/shots-simple/1429-44874.png`|
|1429:44919 (export-read-error)|`**/parity/1429-44919.png`|`ux-people/output/ux-people/shots-simple/1429-44919.png`|
|1429:44955 (export-locked)|`**/parity/1429-44955.png`|`ux-people/output/ux-people/shots-simple/1429-44955.png`|
|1429:45011 (export-locked)|`**/parity/1429-45011.png`|`ux-people/output/ux-people/shots-simple/1429-45011.png`|
|1429:45058 (export-success)|`**/parity/1429-45058.png`|`ux-people/output/ux-people/shots/export-success-Desktop.png`|
|1429:45086 (export-success)|`**/parity/1429-45086.png`|`ux-people/output/ux-people/shots/export-success-Mobile.png`|
|1429:45105 (export-limit)|`**/parity/1429-45105.png`|`ux-people/output/ux-people/shots/export-limit-Desktop.png`|
|1429:45131 (export-limit)|`**/parity/1429-45131.png`|`ux-people/output/ux-people/shots/export-limit-Mobile.png`|
|1429:45148 (erase-confirm)|`**/parity/1429-45148.png`|`ux-people/output/ux-people/shots-simple/1429-45148.png`|
|1429:45193 (erase-confirm)|`**/parity/1429-45193.png`|`ux-people/output/ux-people/shots-simple/1429-45193.png`|
|1429:45229 (erase-ready)|`**/parity/1429-45229.png`|`ux-people/output/ux-people/shots-simple/1429-45229.png`|
|1429:45272 (erase-ready)|`**/parity/1429-45272.png`|`ux-people/output/ux-people/shots-simple/1429-45272.png`|
|1429:45306 (erase-unknown)|`**/parity/1429-45306.png`|`ux-people/output/ux-people/shots-simple/1429-45306.png`|
|1429:45347 (erase-unknown)|`**/parity/1429-45347.png`|`ux-people/output/ux-people/shots-simple/1429-45347.png`|
|1429:45379 (erase-read-error)|`**/parity/1429-45379.png`|`ux-people/output/ux-people/shots-simple/1429-45379.png`|
|1429:45424 (erase-read-error)|`**/parity/1429-45424.png`|`ux-people/output/ux-people/shots-simple/1429-45424.png`|
|1429:45460 (erase-locked)|`**/parity/1429-45460.png`|`ux-people/output/ux-people/shots-simple/1429-45460.png`|
|1429:45516 (erase-locked)|`**/parity/1429-45516.png`|`ux-people/output/ux-people/shots-simple/1429-45516.png`|
|1429:45563 (erase-blocked)|`**/parity/1429-45563.png`|`ux-people/output/ux-people/shots/erase-blocked-Desktop.png`|
|1429:45589 (erase-blocked)|`**/parity/1429-45589.png`|`ux-people/output/ux-people/shots/erase-blocked-Mobile.png`|
|1429:45606 (erase-success)|`**/parity/1429-45606.png`|`ux-people/output/ux-people/shots/erase-success-Desktop.png`|
|1429:45634 (erase-success)|`**/parity/1429-45634.png`|`ux-people/output/ux-people/shots/erase-success-Mobile.png`|
|1429:45653 (erased-profile)|`**/parity/1429-45653.png`|`ux-people/output/ux-people/shots/erased-profile-Desktop.png`|
|1429:45704 (erased-profile)|`**/parity/1429-45704.png`|`ux-people/output/ux-people/shots/erased-profile-Mobile.png`|

Click ledgers: per-test catalog-v2-click-ledger.json and people-v2-*-clicks.json. Automatic screenshot generation is not visual acceptance; independent comparison against the signed PNG is required, visible mismatch=P1.

NOT_RUN: implementation/GREEN, browser execution/persistence/captures, independent parity/E4, TypeScript compilation, Humaux store/canvas/coord tools (unavailable), all local tests/servers, LIVE/production. Full process/required-fact ledger: output/PARITY-CATALOG-PEOPLE/DELIVERY.md.

🤖 Generated with Codex (dispatched by Claude Code integrator).
