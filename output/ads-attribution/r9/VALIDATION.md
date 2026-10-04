# R9 validation ledger

Product/test source: `492aaa30244326e58a8e8163e483409b8a3d975d`. All commands ran in `.worktrees/ads-attribution`. The following evidence-only commit does not alter that source. No remote provider mutation or deployment.

| Command | Exit | Evidence |
|---|---:|---|
| `go build ./...` | 0 | `build-final-source.log` |
| `go vet ./...` | 0 | `vet-final-source.log` |
| `git ls-files -z '*.go' \| xargs -0 gofmt -l` + assert empty | 0 | `gofmt-final-source.log` empty |
| `git diff --check` | 0 | command output, before generated sweep evidence |
| `bash scripts/dev/check-gates.sh` | 0 | `check-gates-final.log` |
| `bash scripts/dev/depmap.sh --check` | 0 | `depmap-final.log` (same Go/source graph; no dependency change afterward) |
| `bash scripts/dev/test-node.sh` | 0 | `node-final.log`: 359 PASS, 0 FAIL; optional media R04 suite NOT_RUN |
| `pnpm --filter admin exec tsc --noEmit` | 0 | `admin-tsc-final.log` |
| `pnpm --filter storefront exec tsc --noEmit` | 0 | `storefront-tsc-final.log` |
| `LC_RELEASE_GATE_OUT=output/ads-attribution/r9/g04-final bash scripts/dev/release-gate.sh --strict --only G04` | 0 | `g04-final/results.tsv`: PASS, commit492aaa30, dirty_files0 |
| `LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-ads-attribution` | 0 | `browser-attribution-accepted.log`: two Go browser tests PASS; see `BROWSER-EVIDENCE.md` |
| `LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-click-sweep` | 0 | `click-sweep.log`: 123 page cases, 0 load failures, 986 PASS/0 FAIL/24 SKIP controls, 18/18 journey steps; additional platform runner PASS30 page/15 SSR/16 tag cases,390 clicks,30 reloads |

The full click ledger is `output/ui-click-sweep/ledger.{json,md}` and `journeys.json`, generated 2026-10-03T23:07:08.706Z. Raw browser evidence: `output/playwright/click-sweep/20261003T225607.138309000/`. The attribution route is present in the ledger. Destructive actions stop at confirmation; skips have explicit reasons.

## Final PG still pending at this evidence checkpoint

An independent root focused run and the populated report run passed earlier, but final serial focused PG and full final-source G07 have **not yet completed**. A foreign integrator G07 is still running; no lock/process is forcibly removed. See `FOCUSED-INTERRUPTED.md` for the invalid interrupted run, whose misleading exit0 must never be counted as acceptance.

Historical red logs are retained. Trailing carriage returns/spaces on Next build progress lines in five logs were normalized for Git whitespace checks; test assertions, output text and results are unchanged. AT6 Events Manager SANDBOX and AT9 real live-video insights remain NOT_RUN per the owner prerequisites. The final SUMMARY will supersede this checkpoint after final PG execution.
