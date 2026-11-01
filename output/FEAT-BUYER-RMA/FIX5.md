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

STATUS: WAIT_CI — implementation prepared with E1 static evidence; no completion claim.
