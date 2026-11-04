# FEAT-BUYER-RMA FIX5 — directed implementation / CI handoff

2026-11-01; PR #395 into r3/integration; branch unit/feat-buyer-rma.
Role: dispatched implementer; no recursive delegation. Runtime model/effort identifiers
are not exposed. Only this worktree was written. No other branch was pushed.

## Authority and baseline

Read AGENTS.md, 架构.md §§0/1/2/26/27, contracts/tasks.json DAG,
contracts/returns-v1.md, current buyer HTTP middleware/writers and BFF callers.
Authority: the current human FIX5 ruling explicitly requires exact `no-store`
through the existing buyer response chain; it resolves the earlier FIX4 scope
and `private, no-store` conflict without changing the contract or helper.
Fetched and merged origin/r3/integration first: exit 0, already up to date,
base 4b8c04c7213025c173e260db6f9c81cb5b818c61.
Rollback point: pre-edit HEAD 46aba00f31f29fd54153564e5148ea1b3da2b65b.
Existing untracked evidence was preserved; no fixture, process, container or port started.
Humaux search/grep/get/store/canvas/coord and code-index tools are unavailable.
Repository evidence is not a shared-memory store or canvas update.

## Failure → root cause → change

| Failure | Root cause | Product change (file:line) |
| --- | --- | --- |
| BR01–BR08: missing safe HTTP response headers | New return-route branch overwrote shared middleware's exact Cache-Control value with `private, no-store`; the existing middleware already supplies no-store, nosniff and a server-owned 32-character request ID. | Removed override before dispatch at internal/buyerhttp/handler.go:334; retained shared httperror.Middleware at :80 and existing success/error writers, as used by /v1/buyer/orders. Removed now-unused classifier at internal/buyerhttp/return_requests.go:25. |
| G-UI3/G-UI5: buyer-server.ts 1370 > 1331 | New after-sales routing/validation was added to the ratcheted legacy module. | Extracted route matching (:35), mutation classification (:69), body validation (:73) and response validation (:79) to apps/storefront/lib/buyer-returns.ts with the standard header. Existing caller imports at apps/storefront/lib/buyer-server.ts:5 and delegates at :425/:633/:699/:1075/:1147/:1264. Removed redundant legacy header prose while preserving all four standard header lines. Gate line count is now 1331; no cap change. |
| BR05: 0 created / 0 refused | Both goroutines abort inside the safe-header helper before sending their responses to the channel; no independent transaction-race cause established. | Header root fixed above. No speculative SQL/locking/test edit; the unchanged BR05 CI assertion must establish one created/one refused after header admission succeeds. |

No test, helper, assertion, fixture, threshold, exemption, timeout, route freeze
pin, contract, migration, OpenAPI, go.mod/go.sum or lockfile change in FIX5.
Migration stays 0217; the R2 pin stays 143. No UI page, browser spec or visible copy changed.

## Evidence and verification

RED already observed at b6fd187b942b72164f6b0d6b5fb187afd8960067:
Gates 41275151042 / 124756434727 (BR01–BR08 header assertions and BR05
secondary count), / 124756434661 (legacy size assertion). Re-read raw filtered
failed-job logs; both commands exit 0. IDs and assertions appended to red.log.
Original tests-only causal RED remains Foundation 41093678360 / 124193777832,
head be932cbb748c3c88a1044ffcf242c6e04333a8b4 (404 vs 201/503).

Only permitted local static operations ran, all exit 0:
- gofmt -w internal/buyerhttp/handler.go internal/buyerhttp/return_requests.go
- go vet ./internal/buyerhttp
- go build ./internal/buyerhttp
- node --check apps/storefront/lib/buyer-server.ts
- node --check apps/storefront/lib/buyer-returns.ts
- python3 scripts/check_packet.py: PASS_PACKET_STRUCTURE_ONLY; restored only its timestamp residue after checking the remaining JSON was identical.
- node scripts/dev/derive-browser-ownership.mjs, then --check
- bash scripts/dev/depmap.sh, then --check
- git diff --check

Ownership and dependency-map generators produced no delta. New module was
added to the index before ownership generation so imported-source discovery
includes it. No component import/harness was changed. Local tsc is unavailable;
full typechecking and all executable behavior verification remain CI-only.

E1 static evidence bound to current source SHA256:
- buyer-server.ts: 7e9a321ae1e1a28a6f005428814da899a8c03300fc629c5b523777a9c3240418
- buyer-returns.ts: ada9b16e8f4635e3ef016ae1d196dec6b2c867d4de5bdda020cd09a3c698b710
- internal/buyerhttp/handler.go: be7b6f3e923292465e356c8a3da5e8cf873ad668c318c1c2eecd3354ecfebb60
- internal/buyerhttp/return_requests.go: 7ccfce474571d8d112897878f64f2ba49d812ca6d8d6f74b532a43e4bb5eb805

Author judgment E0; GREEN / current-head E3 and independent E4 acceptance pending.
One batched implementation push; no tests-only repeat. If CI reports dispatcher
freeze mismatch, STOP and report its new hash; integrator re-pins serially after
#394. No local scanner execution and no contracts/route-table-dispatchers.json edit.
No more than approximately ten minutes CI polling; integrator relays completion
in a fresh session using final WAIT_RUN IDs. No evidence-only push after GREEN.

Outside-card paths changed by this FIX5 implementation: none; buyer-returns.ts
and existing buyer-server.ts caller edits were explicitly granted. Card evidence:
output/FEAT-BUYER-RMA/{FIX5.md,red.log}. Generated files have no changes.
No merge/auto-merge/status write/force-push or production action.
NOT_RUN: local tests/servers (prohibited), local tsc, current-head CI GREEN,
BR05 post-header race verification, independent review, external SANDBOX/LIVE,
production deployment; Humaux/code-index operations (tools unavailable).

## Relay 1 — current-head CI failure, 2026-11-01

This section supersedes the pending-CI status above. Read-only GitHub receipts
confirm both runs completed with failure on the exact current head
`6bfb1e6422463096652295e6b9f3a7c6337a9879`:

- Foundation run `41279737932`, job `124769025188`.
- Gates run `41279737926`, plan job `124769025247`; required job
  `124770484451` failed downstream. A plan job is not behavioral verification.

| Failure | Root cause / observed evidence | Change or required action (file:line) |
| --- | --- | --- |
| Dispatcher freeze mismatch, Foundation `TestGoRepositoryInventory` and Gates contract drift | Removing the return-header override changed the frozen buyer dispatcher closure. CI reports actual `8cab516ac70487ed17c3c8eb90d88d822d0c0fe2f8d3e2e8445143b76e41287e`; pinned `aff6d24766226f455bea9be08b2de8abad3643a71951ad05ede717c28c127c05`. | Existing product repair: internal/buyerhttp/handler.go:334. STOP per FIX5; integrator reviews/re-pins serially after #394. No change to contracts/route-table-dispatchers.json:39. |
| Foundation `TestBFFLiveSettingsRealLeaves` / `TestBFFRealSourceMutation`; Gates `UNRESOLVED` | CI reports `unknown reached source binding buyerReturns` after the extraction introduced the namespace import at apps/storefront/lib/buyer-server.ts:5. Analyzer root cause beyond that diagnostic is not established. | No additional implementation or analyzer/test edit: dispatcher STOP applies first. This separate failure remains for the next directed fix. |

Receipt commands: `gh pr checks 395` (exit 1); `gh run view 41279737932
--log-failed --job 124769025188` and `gh run view 41279737926 --log-failed
--job 124769025247` (filtered to failure diagnostics); `gh run view` JSON
confirmed each head SHA and completed/failure conclusion. Diagnostic excerpts:

```text
--- FAIL: TestBFFLiveSettingsRealLeaves
bff_live_settings_test.go:63: real source unresolved: ... unknown reached source binding buyerReturns
--- FAIL: TestBFFRealSourceMutation
--- FAIL: TestGoRepositoryInventory
go_extract_test.go:350: buyer remains unresolved: ... route-table dispatcher changed or unpinned: buyerhttp.Routes
```

No current-head GREEN exists. BR01–BR08 and BR05 post-header behavior remain
NOT_RUN on this head because the earlier gate failed. CI failure receipts are
SHA-bound E3 evidence of the blockers, not acceptance of the implementation;
author judgment remains E0. No new local tests, servers, generators, merge,
commit, push, PR comment or production action occurred in this relay. Only
this report changed; no additional outside-card paths. Earlier untracked
evidence was preserved. Humaux remains unavailable: no store/canvas/coord
operation is claimed. No task process or fixture was started.

NOT_RUN: local tests/servers (prohibited), current-head behavioral GREEN,
BR05 post-header race verification, independent E4 review, SANDBOX/LIVE,
production. WAIT_RUN: none pending; `41279737932` / `41279737926` completed FAIL.

## Relay 2 — verified STOP condition, 2026-11-01

Re-read the current human FIX5 card, repository rules and architecture sections,
then fetched origin/r3/integration (exit 0). Its tip is still
`4b8c04c7213025c173e260db6f9c81cb5b818c61`; `git merge --no-edit
origin/r3/integration` returned `Already up to date.` (exit 0). Head remains
`6bfb1e6422463096652295e6b9f3a7c6337a9879`, the rollback reference for
tracked source. Pre-existing modified report and untracked evidence were preserved.

`gh pr checks 395` and run JSON confirm Foundation `41279737932` / job
`124769025188` and Gates `41279737926` / plan job `124769025247` failed on
this exact head; required job `124770484451` failed downstream. Filtered raw
failed-job logs (receipt commands exit 0) reconfirm both blockers in the Relay 1
table above, including the actual dispatcher hash
`8cab516ac70487ed17c3c8eb90d88d822d0c0fe2f8d3e2e8445143b76e41287e`
versus the PR's pin
`aff6d24766226f455bea9be08b2de8abad3643a71951ad05ede717c28c127c05`.
The `buyerReturns` diagnostic is a separate unresolved source-binding failure;
its repair is deferred under the card's explicit dispatcher STOP instruction.

No new fix attempt, implementation change, local test/server, generator, commit,
push, PR comment, merge into the integration branch or pin edit occurred.
Only this in-card report was updated; no new outside-card paths. Humaux and
code-index tools remain unavailable, so no shared store/canvas/coord operation
is claimed. No process or fixture was started. CI receipts are E3 failure
evidence, not implementation acceptance; author judgment remains E0.

NOT_RUN: current-head GREEN, BR01–BR08/BR05 post-header behavioral verification,
independent E4 review, local tests/servers (prohibited), SANDBOX/LIVE, production.
WAIT_RUN: none — both current-head workflows completed FAIL; no new push.

## Relay 3 — current receipts and Humaux handoff, 2026-11-01

Fetched and merged origin/r3/integration first (both exit 0; already up to date).
Integration remains `4b8c04c7213025c173e260db6f9c81cb5b818c61`; this PR remains
`6bfb1e6422463096652295e6b9f3a7c6337a9879`. Existing report changes and untracked
evidence were preserved. No product source, pin, test or threshold was changed.

Re-read `gh pr checks 395`, workflow head/conclusion JSON and filtered raw
failed-job logs. Foundation `41279737932` / `124769025188` and Gates
`41279737926` / `124769025247` are completed FAIL on this exact head.
`TestGoRepositoryInventory` reports actual buyerhttp.Routes SHA256
`8cab516ac70487ed17c3c8eb90d88d822d0c0fe2f8d3e2e8445143b76e41287e`,
want `aff6d24766226f455bea9be08b2de8abad3643a71951ad05ede717c28c127c05`;
the plan log repeats the mismatch. This triggers FIX5's explicit STOP rule.
Integrator must review/re-pin serially after #394; this lane leaves
contracts/route-table-dispatchers.json:39 unchanged. The existing repair is at
internal/buyerhttp/handler.go:334. The separate `unknown reached source binding
buyerReturns` failure still points to apps/storefront/lib/buyer-server.ts:5;
no analyzer or import repair was attempted after the STOP condition.

Humaux is available in this relay, superseding the earlier availability notes
for this relay only. Searched task history, read development detail
7bdb220b-0798-4b1d-aff5-16ee94cae75f, enumerated rejected representative 5/9
(coverage 0.5556, not exhaustive), and read the task canvas/code map. This is a
dispatched subtask: no new claim or recursive delegation. Handoff problem and
own subtask canvas are recorded under advisory locks. No platform/local memory
files were used.

E3 failure evidence is bound to the head above; author PASS remains E0 and no
GREEN acceptance exists. No local tests/server, new fix attempt, generator,
commit, push, status, merge into integration or production action. Only this
in-card report changed in this relay; no task processes or fixtures started.
Original RED: Gates 41275151042 / jobs 124756434727 (headers/race secondary),
124756434661 (legacy line cap). Current failures supersede that older head.
NOT_RUN: GREEN, post-header BR01–BR08/BR05 verification, independent E4 review,
local tests/servers (prohibited), SANDBOX/LIVE, production.
WAIT_RUN: none — current-head workflows completed FAIL; integrator must relay
after serial re-pin and a separately authorized binding repair.

STATUS: BLOCKED_DISPATCHER_FREEZE — integrator serial review/re-pin after #394 required; buyerReturns analyzer failure also unresolved.

## Relay 4 — current-head STOP readback, 2026-11-01

Fetched origin/r3/integration and merged it first (exit 0 each; Already up to
date). Integration remains `4b8c04c7213025c173e260db6f9c81cb5b818c61`.
PR #395 and local HEAD are `6bfb1e6422463096652295e6b9f3a7c6337a9879`;
this is the existing FIX5 implementation commit, not the tests-only RED.
Pre-existing modified report and untracked evidence were preserved.

| Failure | Root cause / raw receipt | Change or handoff (file:line) |
| --- | --- | --- |
| Dispatcher freeze | Foundation `41279737932` / `124769025188`, `TestGoRepositoryInventory`, and Gates `41279737926` / `124769025247` report actual `8cab516ac70487ed17c3c8eb90d88d822d0c0fe2f8d3e2e8445143b76e41287e`, want `aff6d24766226f455bea9be08b2de8abad3643a71951ad05ede717c28c127c05`. Existing header repair changed the frozen closure. | STOP under FIX5 item 1. Existing repair internal/buyerhttp/handler.go:334; contracts/route-table-dispatchers.json:39 untouched. Integrator reviews/re-pins serially after #394. |
| BFF source binding | Same Foundation job: `TestBFFLiveSettingsRealLeaves` and `TestBFFRealSourceMutation`; Gates plan repeats `unknown reached source binding buyerReturns`. | Import at apps/storefront/lib/buyer-server.ts:5. Unresolved; no repair after the explicit dispatcher STOP. |

`gh pr checks 395` exited 1; both `gh run view --json` receipts bind completed
FAIL to the exact head above. Filtered `gh run view --log-failed --job` reads
exited 0. Required job `124770484451` failed downstream. E3 failure evidence
only; no GREEN acceptance and author judgment remains E0. The original directed
RED remains Gates `41275151042` / `124756434727` (headers and secondary race)
and `124756434661` (line cap), recorded in red.log.

No new implementation/fix attempt, pin/test/threshold edit, commit or push.
No generators rerun because no source changed and no final commit is being
made. `git diff --check` exited 0. Only this in-card report was appended;
outside-card files touched in this relay: none. No local tests/server or task
process/fixture started. Shared Humaux handoff is stored under the relay4
advisory lock; its own subtask canvas records this STOP, not completion.
NOT_RUN: current-head GREEN, BR01–BR08/BR05 post-header behavior, independent
E4 review, local tests/servers (prohibited), SANDBOX/LIVE, production.
WAIT_RUN: NONE — Foundation `41279737932` and Gates `41279737926` completed FAIL.

STATUS: BLOCKED_DISPATCHER_FREEZE — serial integrator re-pin required after #394; buyerReturns binding also unresolved; no further push.
