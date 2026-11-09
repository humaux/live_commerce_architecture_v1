<!-- Purpose: index LP2 member-read source and bounded evidence without committing raw archives.
Depends on: frozen LP2 source commits, official GitHub CI receipts, ADR-20261031 retention checker.
Used by: PR575 reviewers and integration acceptance; native completion remains explicit. -->
# LP2 member read slice delivery

PR: https://github.com/humaux/live_commerce_architecture_v1/pull/575 . Related #553; broader LP2 write scope remains open.

The slice reuses LP1 for private member search, list paging, member balance and ledger reads, with customer/member navigation, three locales and existing customer browser-mode registration. It adds no backend engine or write UI. Source snapshots: UI ef45dd8c5d5e72ee33e12a9c4909cef04ba2cde4, native registration 71113b341983868a04b50049655f7d0a53c01606, navigation regression c441534887b37976905cef83f526632dfda9e512. Accepted base 3d924d9cfbe56922be15b692da0ea45cbc5525f1 was normally merged; shared test-node retains both the member suite and accepted SU07 diagnostic suite.

## Evidence retained outside Git

The local test-log archive remains physically at `output/loyalty-points-admin-ui/read-slice-20261109/raw-logs.tar.gz` in the task worktree. It is 4,995 bytes, SHA256 `85b3f5762ae496dbcc6fcc7bcaa320d8de5cc6c3334f4479af3b291501077700`. It was removed only from the Git index under ADR-20261031; no bytes were deleted, moved or reencoded. Rollback/source custody: commit c441534887b37976905cef83f526632dfda9e512, blob de8e3ba0c79f09345598042b7b4b04aaa216588a. This is a local test-log archive, not a downloaded CI artifact; no CI artifact ID is asserted for it. Original historical manifests and failed logs remain unchanged.

The current c441 local canonical Node run completed with 2,353 tests, 2,353 pass, zero failure/skip; raw log SHA256 `e62adeed8825f0708aee1a810bbd2f2d2ad98de98b4ab7b23d43039f4fac4516`. Admin/storefront strict checks, pkgdocs, headers and generated ownership passed. These are local/OFFLINE results; Go browser compile executed zero tests. Historical per-stage bindings are in the retained read-slice manifest, not native acceptance.

## Official CI pointers and remaining acceptance

The original c441 pull_request attempt 1 Foundation run [42603254483](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/42603254483), job `129112642540`, failed evidence-retention on the tracked archive; raw job-log SHA256 `66aa913bf4b524a3668a6c3cd74f08b35233022f7d58b2ba7287dee0074335c4`. That failure is preserved. Concurrent Gates run [42603254571](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/42603254571) was still running at handoff; no native receipt was available then.

The corrected source will receive new automatic PR CI. Its exact run/attempt/tested-merge and artifact digests must be collected from the official API before claiming native completion. Full check-gates, REAL_PG customer/member journeys, actual click/persistence/visual/calibration and independent acceptance remain pending for this corrected union. No payment/provider LIVE action or deployment is included.
