PARITY-CATALOG-PEOPLE-FIX2 aligns the signed list/privacy states and restores native Copy coverage through the approved K2 entry. Integration acf2ebd2c was merged first. Only K2 Copy entry + confirmation/success are implemented; remaining K2 stays on PARITY-K2.

- Product tasks → Copy product → confirm sends the existing expected_version/idempotency-fenced endpoint; success opens only its returned new draft id. Advanced stays absent. Original Copy response-loss/refused-retry and competing-draft recovery assertions remain, via the shared real-click driver.
- Product parity removes the unsigned search form, adds signed chips/selection labels. Former search-only fixture scoping uses read-only q URL preparation; native filtering/paging/write/reload assertions remain. People uses signed Tags and records; retained tag/note controls remain reachable.
- Includes earlier unpushed consent-state projection, privacy outcome/pending readback, permission fences and editor parity repairs. No migrations, contracts/OpenAPI, lockfiles, protected gate baselines/dispatchers or R2 pin changed.

|Original tests-only RED41054099331 job|Failure|Class|
|---|---|---|
|124070683720|CB03List consent state; CB09HTTP exact list/detail DTO missing consent_state|(b) product read projection|
|124070684122|ConsentStateProjection map[] for never/withdrawn/granted/regranted|(b) same projection|
|124070684239|CustomersBilling 46 signed-control/outcome/pending/consent/permission assertions|(b) product parity|
|124070683762|Catalog unsigned search expected0 received1|(b) product parity; class(a) incumbent locator migration|
|124100320890|Four failed mode receipts|Dependent|
|124103568945|gate/receipts failure; aggregate success|Dependent|

Original RED head6e2cbdb1, synthetic mergec99fdd96001375527e6ec1a03bffc7fb2ceee4e6; original Foundation41054099291/job124057137224 success is partial only. Base41107905131 remains in_progress; no class(c) exemption. Full failing-job/test/assertion evidence table: output/parity-catalog-people/FIX2.md and red.log.

Signed source1469:140597 ON_CLICK→1469:141623; desktop/mobile confirmation1469:141623/141703, success1469:141754/141832. Live metadata and screenshots recorded without writes. New native Copy cases cover three locales × two widths, cancellation/no command, exact payload/key, returned identity, reload persistence and Chinese mobile density. Existing Copy security assertions preserved.

Static E1: packet structure, go vet customers, changed i18n TypeScript, ownership generation/--check and whitespace exit0. Admin full typecheck not established locally (dependency/alias setup); CI owns verification. Own PASS=E0. GREEN/browser clicks/parity/density/REAL_PG runtime/independentE4/LIVE NOT_RUN pending exact-head CI. No local tests or servers.

Outside initial UI paths requiring integrator scope review (carried original RED/partial): internal/customers/{types.go,read.go,types_test.go}; tests/foundation/customers_billing_{http,list}_test.go. Caller migrations: tests/admin/{catalog-core.spec.ts,catalog-document-driver.ts,product-editor.acceptance.ts,settings-v2.spec.ts}. Generated browser ownership is explicitly authorized; dependency-map regenerated unchanged. No additional outside-module product changes.

Rollback: revert this card's implementation commit and prior unpushed partial a128fe4cd; never alter shared integration directly. CI workflow ids will be posted as a PR comment without an evidence-only push. Integrator relays on completion/failure/cancellation; no local watcher or background process. Same-test RED after this batch → BLOCKED, no third approach.

🤖 Generated with Codex (dispatched by Claude Code integrator).
