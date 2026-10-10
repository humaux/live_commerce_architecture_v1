# W3-U2 PR #31 post-PR36 delivery

- Branch/worktree: `unit/w3-u2-live-settings-ui`, `.worktrees/w3-u2-live-settings-ui`.
- Frozen tested source: **`7737cb87d186c5b17ef582142c7766136660a41b`**; trunk PR36: `287e08aa6c694030333dd00797c0d3bbac15abc6`, merged as `9e6282b7`. This final handoff changes evidence and documentation only. [SOURCE](pr36-integration/SOURCE.json) binds 61 current source/test files; its SHA256 is `07a69001c6c2f2cdf59a11be898768043cedcfc738265806834d69c8b9234e0d`.
- Author Codex-4; runtime model/effort unexposed. Isolated mapping worker owns five `scripts/dev/contractdrift/` files; its recorded actual model/effort are UNKNOWN. Read-only source/artifact reviewers own no product paths. Current task: `35a92743-066e-4fd9-8fb0-98eb3af4c539`.
- Evidence: **E3 in the tested BROWSER + MOCK business + REAL_PG authorization/adapter environment**. Source review `9806004a`, completed-subset artifact audit `e5d3a2ec`, and remaining-run audit `14008b1b` are **E1**, not new K3/CI approval.

## Changes and review resolution

The seven first-round fixes remain: preserve successful reminder receipts without inbox:read; keep same-address navigation alive; paginate scenes including scene 21; accept the contract's 120-rune template names; show sold-out sent badges only for a successful send; isolate positively proven optional-reminder absence; disable the selected scene's empty placeholder. Privacy-generation fences retain the original UNKNOWN command key and prevent discarded callbacks from borrowing fresh authority. Their red/green evidence remains under `pr31-r1/`.

OP4 keeps its exact navigation-array assertion in all three locales, with `nav-live-settings` immediately after `nav-claims`. The legitimate structure change passed both original full Ops runs; no containment assertion or timeout change.

The scoped-404 dependency is now resolved by **trunk PR36**, in `internal/platform/platform.go`: after a mid-transaction denial, roll back and re-resolve the signed scope within the bounded deadline; emit 404 only when store loss is proven. This also covers the dedicated settings adapter. This branch implements no second SCOPE-404 fix. The earlier shared BFF candidate `58510545` is historical; the W3-U3 revert `e4e48ec5` and source parity were checked read-only (E1).

Both original full Console runs now pass LCU2_404 all/private, without changing the original waits or later assertions. Four native REAL_PG receipts each have **status=404, privateRows=0, buyerPanel=0, sessionRetained=true, postLossReads=0**. The unit's legitimate restriction-check observer arm, local missing-bundle handling and effect-generation polling fence remain in place; they do not normalize a Go 403.

The new single mode registry is preserved. Commit `f9263012` declares `lc_browsers=chromium` in the existing live-settings registry block; the existing missing-declaration test was RED, then GREEN.

Commit `017c62b5` imports the source-derived CI-DRIFT mapping from worker `f60d71d4`. It derives dedicated leaf resources/methods from the actual tuple, native fetch path/options and guarded exported handlers. Unknown shapes remain UNRESOLVED; nested non-executing guards and computed method overrides are rejected. Thirteen mutation cases and the full scanner tests passed after actual red failures. No baseline relaxation or product route change. Uncompressed worker evidence: [mapping/evidence.json](pr36-integration/mapping/evidence.json), `red-tests.log`, `red-cli.log`, `red-guard-alias.log`, `red-review-mutants.log`, `green-review-focused.log`, `whole-review-tests.log`, `green-review-cli.log`.

Commit `7737cb87` renames three archived test-source snapshots to `.source.txt`, preserving bytes. They cannot be misclassified as runnable tests by check-gates. Red/green provenance is in `pr36-integration/red-archived-test-names/`.

## Contract / interface

No additional product contract, Go runtime endpoint, SQL/migration, permission, OpenAPI, dependency or lockfile change in this post-PR36 batch. The previously authorized sold-out GET/PUT adapter remains CAS/idempotent; focused REAL_PG tests cover it and PR36's scope recheck. CI-DRIFT changes are static tooling only.

## Actual commands and exits

All paths below are relative to this delivery directory. [RESULTS](pr36-integration/RESULTS.json) and `pr36-integration/runs/*.json` record exact argv, environment, UTC timestamps, timeout, PID, exit and actual log SHA256. Browser runs were serial, using Chromium and unique ignored evidence roots; `LC_TEST_LOCK_WAIT=1800` changes only resource queue waiting. No grep, calibration, Ops subset, browser assertion or test-wait change.

| Command | Exit | Current source evidence |
| --- | ---: | --- |
| `bash scripts/dev/test-node.sh` | 0 | `pr36-integration/runs/test-node.log`; all suites green, no failed/skipped cases |
| `pnpm typecheck:admin` | 0 | `pr36-integration/runs/typecheck.log` |
| `bash scripts/dev/test-local.sh --list` | 0 | `pr36-integration/runs/registry-list.log`; single registry retained |
| `bash scripts/dev/check-gates.sh` | 0 | `pr36-integration/runs/check-gates.log`; 83 modes all documented, every tracked test run, header/dependency/format/shard checks green |
| `LC_TEST_LOCK_WAIT=1800 bash scripts/dev/test-focused.sh 'TestLiveSettingsHTTP\|TestScopeRevokeMidTransactionBecomesScopeNotFound\|TestScopeSupportGrantRevokedMidTransactionBecomesScopeNotFound\|TestScopeRecheckFailureKeepsDenial' ./tests/foundation ./internal/httpapi ./internal/platform` | 0 | `pr36-integration/runs/http-real-pg.log`; 5 top-level PASS, 0 FAIL/SKIP |
| `bash scripts/dev/test-local.sh --browser-live-console` ×2 | 0 / 0 | `pr36-integration/runs/console-{1,2}.log`; four original all/private scoped-404 cases pass |
| `bash scripts/dev/test-local.sh --browser-ops-polish` ×2 | 0 / 0 | `pr36-integration/runs/ops-{1,2}.log`; Storefront + Admin pass both times, Admin 8 tests each, OP4 en/zh-CN/zh-TW |
| `bash scripts/dev/test-local.sh --browser-live-settings` | 0 | `pr36-integration/runs/live-settings.log`; **22 Playwright tests passed, zero skips**, 22 per-test click ledgers plus native aggregate |
| `bash scripts/dev/test-local.sh --browser-live-claims` | 0 | `pr36-integration/runs/live-claims.log`; **one real-chain driver, 21 business cases, three locales, zero skips** |
| `go run ./scripts/dev/contractdrift` | 0 | `pr36-integration/runs/contractdrift-post.log`; **errors=0**, warnings=362 retained legacy. Started 07:47:08.942Z, after Claims ended 07:47:08.813Z and Settings ended 07:46:41.914Z |

After copying/staging artifacts, the final artifact-name/check-gates check **passed (exit 0)** and is recorded in `pr36-integration/final-artifact-check.json` and its log. It verifies the final tracked filenames; passed browser modes are not repeated. Native logs are retained byte-identical: the global `git diff --cached --check` returns 2 for log whitespace only; the exact non-log check returns 0 ([record](pr36-integration/artifact-style-check.json)).

## Committed canonical evidence

`pr36-integration/browser/` contains only needed native ledgers/results/logs, plus **12 synthetic masked settings screenshots** at 1440/390 in zh-TW/zh-CN/en. Settings masks private rows, drafts, preview and BuyerPanel; sold-out status screenshots mask comment author/text. No generated spec snapshots, traces, videos, credentials or real buyer data are copied.

Console receipts: `browser/console-1/live-console-3015325301/grant-revocation-{all,private}.json` and `browser/console-2/live-console-2879311949/grant-revocation-{all,private}.json`. Claims uses its actual fresh native root `output/playwright/live-claims-2725675277`; copied `browser/live-claims/result.json` retains the 21-case result and token hash only. The caller's unused Claims evidence-root value is not substituted for the real path.

[SOURCE](pr36-integration/SOURCE.json), [RESULTS](pr36-integration/RESULTS.json) and [MANIFEST](pr36-integration/MANIFEST.json) bind tested code, native results, logs and copied artifacts by SHA256. Runtime source and the four original browser specs are byte-identical throughout these runs; the specs also match `20417870`.

## Historical evidence / NOT_RUN

Old `b06df828` K3 PASS, `trunk-resume/`, `browser-before-extra/` and `pr31-r1/` runs belong to earlier source revisions. They are historical evidence, not current-head independent approval. The former pending-403 note is superseded by the actual PR36 integration and native receipts above; CI539f1cb8's first denied status remains UNKNOWN from its incomplete old artifact.

The first current-source Settings attempt never reached tests: another live owner's machine PG lock caused **NOT_RUN exit2 after 1800s**; Claims was not started in that chain. The holder was neither killed nor its lock removed. `pr36-integration/history/pg-queue-not-run.{json,log}` preserves this; the successful remaining-only runner 50173 is separately recorded. The interrupted initial check-gates metadata has no recorded exit and is **UNKNOWN**; only the completed retry is counted.

## CI gates / risks / handoff

Required next CI: **foundation-shards plus the current PR planner's selected required set**. Full foundation, other selected browser modes, iPhone WebKit, provider SANDBOX/LIVE, real messages and deployment are **NOT_RUN locally**. The four directly affected browser modes above passed on frozen source; current-head independent K3 and remote CI remain the integrator's gates. Scanner warnings362 are retained opaque/legacy provenance, not errors=0 proof of those routes.

Owned native browser/Next/PG runner processes finished; shared browser mutex released. Other tasks' processes/fixtures/caches were untouched. The heartbeat is paused after acceptance. **Commit only; integrator reviews and pushes.**
