# followups-polish-1 delivery

- Branch/worktree: `unit/followups-polish-1`, `.worktrees/followups-polish-1`. Base: merged trunk `e5c228a552ccae72ed9d6b7cd40dc035ae251e01` (fast-forwarded the prepared worktree before edits). Final SHA is the commit containing this delivery.
- Author: Codex-3; inherited model/effort, exact runtime identifier not exposed. No delegated writers. Contract/API/dependency changes: none. Forbidden LiveConsole/BuyerPanel/CommentStream/Inbox/parcel/W3-U4 files are unchanged relative to the base.

## Four follow-ups

1. PR #3, 4212542044/4212542053: global desktop product-table layout/column percentages leaked into reports. Scope `table-layout: auto` and header `width: auto` to report tables; use a zero-minimum grid track and shrinkable tab panels. No global CSS changes. Real browser RED: desktop fixed tables had overflowing cells; mobile page widths were 1018px (products) and 1111px (manual orders). GREEN: page widths exactly 1440/390, zero overflowing cells, mobile scroll regions 358px wide. Products/channels/manual tables are covered; native ArrowRight scroll moves the region while window scrollX stays unchanged. Full report controls, monetary splits, exports and Go export-audit counts remain unchanged and pass.
2. PR #7, BankTransfer: normalize a missing resolved timezone to an empty string and render a generic device-timezone label in all three locales. Preserve the actual device-local input semantics and store-time echo. Three actual-component tests with an absent native Intl zone fail before and pass after; real shared formatter/copy imports remain intact.
3. PR #7, stableLocaleTarget: a local 10000ms deadline now bounds the held destination-head promise. Its error includes desktop/mobile, the GET route and current page URL. `finally` clears the timer on success/failure. No retries or changes to other timeouts. A registered Node test executes the actual probe function with a controlled browser/network edge and clock, proving missing-head rejection/message and completed-head timer cleanup. Existing --browser-order proves the real UI success path.
4. PR #5: remove the redundant ops-polish header paragraph, retaining one Purpose/Depends/Used block and the unique clock/call documentation. The executable body is byte-identical to the base.

## Validation

All final commands below exited **0**, with one frozen source digest and `source_unchanged=true` (see `gate-summary.json`). Browser modes ran sequentially:

- `bash scripts/dev/test-local.sh --browser-reports` — 1440/390 table geometry, screenshots, native inner scrolling, all existing report flow/permission/CSV assertions.
- `bash scripts/dev/test-local.sh --browser-order` — real locale/layout/order/closed-context chain, including the bounded-head success path.
- `bash scripts/dev/test-local.sh --browser-ops-polish` — existing OP1–OP4 gates; header-only change.
- `bash scripts/dev/test-node.sh` — 1148 passed, 0 failed/skipped (optional R04 external binary separately NOT_RUN).
- `bash scripts/dev/check-gates.sh`.
- `pnpm --dir apps/admin exec tsc --noEmit -p .`; storefront equivalent. Both changed browser specs also pass strict standalone tsc with the existing admin Node type root; staged check-gates passes.

Visual inspection: desktop product/manual columns are readable, and both 390px pages remain viewport-wide with deliberate table-local horizontal scrolling. Six final report screenshots plus geometry are in the archive. These are browser emulations, not physical-device acceptance.

## Evidence and scope

All paths in the archive are relative to the shared evidence root `output/followups-polish-1/` (main checkout), not ephemeral external directories. A portable copy is committed as `evidence.tar.gz`; `MANIFEST.json` verifies its members; `source-hashes.json` pins all eight changed source files.

- `reports-red-corrected/`: genuine pre-fix real-browser failure and 1440/390 artifacts.
- `unit-red.log`: five pre-fix failures. `deadline-red.log` recalibrates the actual pre-fix probe after correcting the unit clock synchronization. `unit-final-green.log`: 5/5 pass.
- Excluded attempts are retained, not counted as acceptance: initial report setup was cancelled because the added test tried to measure the table-free funnel chart; the corrected test covers all three table tabs. First `unit-green.log` had one clock-test synchronization failure: a single microtask did not drain cross-VM Promise adoption. Its shell continued to diff-check, so the outer exit0 was not a test pass. One full event-loop turn fixed the test synchronization, then pre-fix RED and fixed GREEN were rerun with unchanged assertions/budget. See `unit-classification.json`. The first optional standalone browser-tsc invocation lacked root Node types; the corrected invocation uses the already-installed `apps/admin/node_modules/@types` and passes (no dependency/source changes). Both invocation logs and their classification are retained.
- `final/`: final three browser modes and local gate logs/status; reports geometry/screenshots. Four tracked home-cod screenshots generated by ops-polish were preserved under shared `final/generated-home-cod/` and restored to baseline in this branch to avoid unrelated changes.

Evidence: **E3 / BROWSER + REAL_PG with signed MOCK IdP; component/clock tests are supplementary controlled-runtime evidence**. Full 52-mode PR CI matrix (`pr-modes.json`), G07, provider SANDBOX/LIVE and physical Safari/phone checks are NOT_RUN locally. All task-owned test commands have finished; evidence is retained. Integrator reviews and pushes; no push or GitHub replies by this author.
