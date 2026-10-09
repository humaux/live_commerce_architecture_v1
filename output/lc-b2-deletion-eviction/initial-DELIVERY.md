# LC-B2-DEL delivery

- Branch/worktree: `unit/lc-b2-deletion-eviction`, `.worktrees/lc-b2-deletion-eviction`.
- Base:85e77089 (unit brief on trunk); `git fetch origin && git merge origin/r3/integration` was a no-op before RED.
- Tested source: **f18eb487**, hashes in `source-hashes.json`. Final delivery commit adds evidence only.
- Author: Codex-4, explicitly authorized Go-only backend unit; exact parent runtime model/effort not exposed. Read-only wiring reviewer, assigned gpt-6.1-sol/high; no second writer or recursive delegation. Task773a2c87-f7d6-4b76-a3e6-4a141b97637e.

## Result

- `comment_deletion.go`: one<=50-id batch per source per60s, eligible entries younger than30min. Initial batch is newest-first; subsequent batches use oldest attempted/queued timestamp first. Successful HTTP200 must fully validate an exact dictionary of requested refs with matching sole id fields before any removal. Partial, malformed, duplicate, unknown, mismatched, token/error or transport responses evict nothing.
- Removal preserves source seq/epoch (no reset), verifies the current source pointer and entry seq, updates both ring/index, and clears discarded backing slots so removed text/name do not remain in the slice storage.
- `comment_rate.go`: forward polls, deletion, older pages and facts share one atomic asset_id pacing/backoff slot. Due deletion checks receive scheduling priority; budget denial does not count as a deletion attempt. Default pacing2s, failure ladder2s→60s, usage50/80%→5/15s. Backoff begins at response completion, not request start. Existing explicitly configured nondefault tunables remain.
- Existing code had **no shared asset budget** (only per-source delay). The common Console gate is the necessary scoped prerequisite. The existing OAuth Graph transport now exposes only the two non-secret usage-header strings; tokens still use Authorization only, no new transport or logging.
- Bridge budget waiting is bounded to3s within its existing deadline to preserve scoped-transaction availability. A confirmed single-comment404(code0/100 only) still spends a pacing slot but does not manufacture rate backoff; auth/quota errors remain failures. Existing IG Graph/webhook fallback assertions pass unchanged.
- Corrected the misleading age-only poller comment and foundation header; added a synthetic fake-Graph deleteRef seam to the LCN01 harness. No SQL, contract, HTTP/OpenAPI, UI, dependency lock or migration changes.

## Red → green / exact commands

**`results.json` contains exact commands, exit codes and plaintext log hashes.** All RED logs are uncompressed.

| Gate | Exit | Result |
| --- | ---: | --- |
| Actual poller deletion/error/budget/rotation cases before fix | 1 | `unit-red.log`; existing code never issues the batch |
| Real poller→bridge→A2 deletion before fix | 1 | `pg-red.log`; deleted comment remained after a full60s check window |
| Delayed429 and invalid-body completion-time backoff | 1→0 | `backoff-red.log`, `package-final.log` |
| Facts404 wrongly manufacturing backoff | 1→0 | `facts-404-red.log`, final package + PG gates |
| Contract default pacing regression | 1→0 | `defaults-red.log`, final package |
| `GOTOOLCHAIN=go1.27.1 go test -race -count=1 -v ./internal/integrations/metareply ./internal/integrations/meta/oauth` | **0** | **58 top-level PASS**,0 FAIL; package-final.log |
| `bash scripts/dev/test-focused.sh '^TestLiveConsoleLCN(01|02|04|05)' ./tests/foundation` | **0** | **9 top-level PASS**,0 FAIL/SKIP; pg-green-3.log |
| `GOTOOLCHAIN=go1.27.1 go vet ./internal/integrations/metareply ./internal/integrations/meta/oauth ./cmd/claims-worker` | **0** | vet.log |
| `bash scripts/dev/test-node.sh` | **0** | **1148 PASS**,0 FAIL; node.log |
| `pnpm --filter @live-commerce/admin typecheck` | **0** | typecheck.log |
| `bash scripts/dev/check-gates.sh` | **0** |82 modes documented, every tracked test assigned, headers green |

A new first-cadence test initially demanded a check at exactly60s even though a59s forward read had consumed the common slot. The corrected test retains the60s frequency assertion, explicitly proves the budget blocks that check, and requires it to run at slot reopening62s. The new foundation test checks nondecreasing page cursor/unchanged epoch; underlying seq remains exact in the unit test. Neither correction changes an existing assertion.

The package suite also covers 429/5xx, malformed/trailing/duplicate/unknown/null/partial/error rows, transport failure, no token URL, shared bridge budget, usage pacing,101-ref rotation, stale source replacement and cleared backing storage. The REAL_PG acceptance runs the actual lease/token custody, Console HTTP bridge and A2 service; existing LCN scope/cap/cursor/IG-facts/print/privacy tests are unchanged and pass.

**E3: tested MOCK Graph + REAL_PG**, bound to current source hashes. Independent read-only E1 review identified the completion-time P1, observed its exact RED/GREEN logs and confirmed it closed; final limited review reports no new confirmedP0/P1. It did not independently rerun tests and does not replace the integrator's K3 deep privacy review. Review memory5e928ec4-f845-4de6-8b2b-d77e0762d79a.

## Bound and limits

For a stable N eligible entries, one rotation takes ceil(N/50) admitted checks, plus shared-budget/provider delays. At the2000 ring cap this is40 rounds, roughly40min. The eligibility window is30min, so the contract's single50-id/60s budget **cannot guarantee every full-cap entry is checked before aging out**. New arrivals queue behind older attempt timestamps; uncertain attempted batches also rotate instead of starving other entries. Older-than30min entries are not checked; existing age/cap policy remains (maximum2h). No stronger capacity/privacy deadline is claimed.

The forward-polled FB source is covered. Instagram webhook-copy behavior is unchanged. API deletion is proved; browser-local buffer reconciliation is the separate PR18 client unit.

## CI / NOT_RUN / integrator

- CI needed: foundation-shards including LCN01 deletion and metareply/oauth race suites; K3 deep privacy review, then integrator push. Author commits and stops; **no push**.
- Optional `--browser-live-console` real Graph deletion fixture: **NOT_RUN**. Its current business-MOCK harness does not wire a real poller/Graph; adding only a delete flag would not prove backend eviction. A real harness wiring follow-up is required. The LCN01 fake Graph now has a reusable real deletion seam.
- Full foundation, browser modes, real provider SANDBOX/LIVE and production: NOT_RUN under owner RAM/custody rules.
- Original dependency-free worktree required `pnpm install --offline --frozen-lockfile --ignore-scripts` (exit0); lockfile unchanged. Diagnostic failed green attempts remain in results.json/history, not counted as final acceptance.
- Owned poller/PG/HTTP processes and fixture containers ended through cleanup. No live keys/money/messages, production action or unrelated worktree changes. PR20 K3 findings retain priority if delivered.

Evidence packaging: only typecheck stdout's final blank EOF line is trimmed in the committed review copy; exact raw stdout and both hashes remain recorded. All RED logs are exact uncompressed text. Packaged check-gates also exits0.
