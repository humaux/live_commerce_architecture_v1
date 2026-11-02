# PR365 Console pagination Phase A candidate

- task_id: `3a83424d-2e69-4dcf-acfe-9328c2c9941d`; own canvas `3a83424d-sub-1`.
- base_commit: `2bfa33066e0e2504cf8a2da36c8ca26b2965e293`.
- role: test worker `/root/console_pagination_red_1359`; actual model/reasoning: inherited runtime, UNKNOWN (not independently exposed).
- worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/codex-pr365-mock-balance`; parent owns author lease. No claim or recursive delegation.
- evidence level: E1 static candidate only. Runtime RED, GREEN and independent acceptance: NOT_RUN. This report does not establish a runtime failure or whole-PR completion.

## Authority and allowed changes

Contract `contracts/studio-v1.md:39-45,97` at base SHA (SHA256 `cb924b6fc15cb733caaa23a0d960de0620b37971962ee3f79fac5843603f0d70`) requires bounded store-scoped cursor pages. Real implementation `internal/live/studio.go:190-243` (SHA256 `04b9aef9694f1e26cd95ff79c662481e46571b6a4ed135d46a8b53c0253af61a`) uses limit+1 and a cursor from the last returned row. SOURCE card `output/integrator/tools/logs/ci-evidence-1238/365-pagination-source-1346.md` was read from the primary tree and its SHA256 verified as `f40523f96274a7753b26041db9e051d40589afc0f16d3b8b766a360f2d3e7212`; Humaux `af5add0f-0865-47de-80d1-a4cba6d9480e` was read in full. These SOURCE observations are not runtime proof.

Exactly three allowed paths:

1. New `tests/foundation/browser_live_console_pagination_test.go`: pure standard-library Recorder regression, build tag `browser`.
2. `tests/foundation/browser_live_console_test.go`: exactly one `t.Run("pagination-contract", testConsoleMockPaginationContract)` registration inside the existing RealChain parent.
3. This report.

The existing handler, structs, timestamps, copy/auth/stock/replay paths, old helpers, TS, parser20, i18n, registry, ownership and gates are untouched. A byte comparison against base confirms that removing the single registration reconstructs the original file exactly. No Phase B implementation is included.

## Regression behavior

The candidate constructs 26 complete existing-type same-store scenes and one foreign-store scene. Every scene has distinct ID, offer, SKU and warehouse with version, stock, generation, active/platform values, recommended data and a preexisting Reads entry. No future CreatedAt field is assumed. Receipts and replay responses are also preseeded. All 27 scenes remain in the map; no rows are reduced, truncated or discarded to satisfy the UI parser.

A direct call to the existing `consoleMock.serve` receives GET `/v1/admin/stores/<store>/live-sessions?limit=20`. The test first requires handled=true, status200 and decodable JSON, then independently asserts exactly20 items and nonempty next_cursor using nonfatal errors. Every returned ID must be unique, known and same-store; the foreign ID must never appear. Deferred checks compare deterministic JSON snapshots of every scene field, receipts, replay responses and authority counter, and independently require 26/1/27 retained rows even if another assertion fails. No replacement pagination algorithm is the subject of this test.

Static source prediction: the unchanged collection branch ignores query parameters, returns26 rows and an empty cursor, so both pagination assertions should fail. Actual runtime result: NOT_RUN; a compile error or environment failure cannot count as the required RED. This narrow Phase A candidate does not yet cover second-page traversal, canonical query/cursor rejection, tenant-bound cursors, real authentication or native UI navigation.

## Exact runtime selection (root only, NOT_RUN here)

```sh
GOTOOLCHAIN=go1.27.3 go test -race -tags browser -count=1 -run '^TestBrowserLiveConsoleRealChain/pagination-contract$' -v ./tests/foundation
```

The regexp selects the existing top-level parent and only its `pagination-contract` child. Its workspace and labels siblings are not selected, so `runBrowserLiveConsole` does not run. The existing TestMain calls m.Run and has only conditional cleanup; it does not eagerly create a fixture. The child directly uses NewRecorder/NewRequest; it starts no listener, server, PostgreSQL, OIDC, Next, browser or provider. Root must record a real compile-success assertion RED at a frozen SHA before separately authorizing Phase B. Root must preserve current immutable-head Console diagnosis before any push. Existing full Console, parity and affected Studio gates remain required later.

## Actual commands and exits

| Command/check actually executed | Exit/result |
| --- | --- |
| `git status --short` and `git rev-parse HEAD` | 0; initially clean at base |
| Targeted `rg`, `sed`, contract reads and source SHA verification | 0 after correcting path |
| Early combined read using absent worktree SOURCE path and absent `internal/studio` | 1; no runtime conclusion; corrected to primary-tree report and `internal/live/studio.go` |
| `gofmt -w tests/foundation/browser_live_console_pagination_test.go` | 0 |
| `git diff --check` | 0 |
| `GOTOOLCHAIN=go1.27.3 go vet -tags browser ./tests/foundation` | 0, 6.828s; compile/static analysis only, not test execution |
| Python base-byte comparison after deleting the one registration in memory | 0, REGISTER_ONLY_BYTE_PRESERVATION=PASS |
| `shasum` / Python SHA256 of SOURCE and changed tests | 0 |

Test SHA256s: new test `4eb15a3469a405eae1a14ee944cd3bc8bf7f48b01989b7e1e782b414399458a1`; original file plus registration `277a95e5e28c65a6fe84411a7f79383c9b38d6237c5deb45766d5c02a33ce1e1`.

NOT_RUN: all go test/runtime/browser/PG/provider commands, runtime RED/GREEN, new CI dispatch/rerun/cancel, push, ownership generation, Phase B, native UI and independent acceptance. No CI or server was started; the bounded static vet process exited normally. No temporary fixtures, containers or ports were created. Existing failures/evidence are retained. Unresolved: actual selected-subtest RED is pending root execution, then separate Phase B authorization; product behavior and full affected-mode acceptance remain UNKNOWN/NOT_RUN.
