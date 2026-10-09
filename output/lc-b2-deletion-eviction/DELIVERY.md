# LC-B2-DEL delivery — K3 P2 follow-up

- Branch/worktree: `unit/lc-b2-deletion-eviction`, `.worktrees/lc-b2-deletion-eviction`.
- Original handoff: `192d0922`; `git fetch origin && git merge origin/r3/integration` → exit0, conflict-free merge **1e069395** of trunk **30ddfb10**.
- Tested source: **13dd69eb13fa15c71141c74508d686c4d22af41f**; current source hashes in `source-hashes.json` and `k3-p2/source-hashes.json`. Final commit adds evidence only.
- Author: Codex-4 / GPT-6 family; exact runtime model/effort not exposed. Single writer; existing read-only wiring reviewer checked the bounded source/log diff, no recursive delegation. Task9d45a715-46af-4892-95c1-7eaf425a8c16.
- Parent K3 privacy review of192d0922: PASS with the two P2s below. Current follow-up independent source/log review: PASS, no new confirmedP0/P1/P2; runtime independently NOT_RUN.

## Changes

`comment_deletion.go:22,103`: a completed deletion batch that loses its source pointer or lease returns dedicated `errSourceReplaced`, not rate-budget denial. `pollOne` returns immediately on that sentinel after clearing `reading`, before any scheduling/status classification; ownership regained between the two mutex sections cannot make the old result a success or throttle event.

`comment_poll.go:496`: docstring accurately names either a forward after-cursor read or the §2.2 deletion batch. The existing shared rate budget, successful-response-only eviction, private backing-slot clearing and seq/epoch fences remain unchanged. No contract, SQL, HTTP/OpenAPI, UI or migration changes; dependency/toolchain changes are inherited from the requested trunk merge only.

New delayed MOCK Graph test covers both source replacement and lease loss: exact dedicated error, no budget error, retained rows/index/seq/epoch and no eviction of the current source. Red→green is recorded against Go1.27.2 from merged trunk.

## Commands / evidence

Exact commands, exit codes and SHA256 log hashes: `results.json` / `k3-p2/results.json`. All RED logs are plaintext and uncompressed.

| Gate | Exit | Result |
| --- | ---: | --- |
| `GOTOOLCHAIN=go1.27.2 go test -race -count=1 -v -run '^TestCommentDeletionLateBatchReportsCustodyFence$' ./internal/integrations/metareply` on old behavior | 1 | Both cases returned rate-budget error; `k3-p2/red.log` |
| Same focused command on fixed behavior | 0 | Both cases PASS; `k3-p2/green.log` |
| `GOTOOLCHAIN=go1.27.2 go test -race -count=1 -v ./internal/integrations/metareply ./internal/integrations/meta/oauth` | 0 | **59 PASS /0 FAIL**; `k3-p2/package.log` |
| `bash scripts/dev/test-focused.sh '^TestLiveConsoleLCN(01\|02\|04\|05)' ./tests/foundation` | 0 | **9 PASS /0 FAIL /0 SKIP**,40.697s; `k3-p2/pg.log` |
| `GOTOOLCHAIN=go1.27.2 go vet ./internal/integrations/metareply ./internal/integrations/meta/oauth ./cmd/claims-worker` | 0 | `k3-p2/vet.log` |
| `bash scripts/dev/test-node.sh` | 0 | **1148 PASS /0 FAIL**; `k3-p2/node.log` |
| `pnpm --filter @live-commerce/admin typecheck` | 0 | `k3-p2/typecheck.log` |
| `bash scripts/dev/check-gates.sh` | 0 |82 modes documented, tracked tests assigned, headers green |

**E3: tested MOCK Graph + REAL_PG**, bound to source13dd69eb. Package tests include deletion/error/budget/rotation and stale-custody cases; REAL_PG runs the actual lease/token custody, Console HTTP bridge and A2 service. Owned runners, loopback HTTP and PG fixture completed cleanup. No push, production action, live credentials, customer message or money.

Original unit RED→GREEN and diagnostic history remain in `initial-results.json`, `initial-source-hashes.json`, `initial-DELIVERY.md` and original plaintext logs. Those sourcef18eb487 results are historical; current manifests above bind this merged head.

## Bound / CI gates / NOT_RUN

For stable N eligible entries, rotation takes ceil(N/50) admitted checks plus shared-budget/provider delays. At2000 entries this is40 rounds (~40min), exceeding the30min eligibility window: no guarantee that every full-cap entry is checked before aging out. Older entries retain the existing maximum2h age/cap policy. Forward-polled FB sources are covered; IG webhook-copy behavior is unchanged. Browser-local reconciliation is separate PR18 client work.

CI needed: **foundation-shards**, including LCN01 deletion and metareply/oauth race suites, the current planner selects only `foundation-shards` (`node scripts/dev/pr-modes.mjs origin/r3/integration HEAD` → exit0, `k3-p2/pr-modes.json`). Integrator opens/pushes the PR and independently runs CI. The parent K3 PASS applies to192d0922; review/CI of this follow-up head are pending.

Optional real Graph deletion fixture in `--browser-live-console`: **NOT_RUN**, because that business-MOCK harness does not wire the actual poller/Graph chain; adding only a delete flag would fake backend acceptance. LCN01's synthetic deleteRef seam is reusable for a future real harness. Full foundation/browser modes, real provider SANDBOX/LIVE and production are NOT_RUN locally under the owner RAM/custody rule.

Only the final blank EOF line is trimmed from the committed typecheck review log; canonical raw stdout and both hashes are recorded. RED logs remain exact. Staged-evidence check-gates also exits0. Commit and stop; integrator pushes.
