# FEAT-BUYER-RMA — implementation push / CI handoff

Evidence: **E1 structure ready; GREEN and E3/REAL_PG acceptance PENDING. Author judgment is E0.**
PR #395, base `r3/integration`; branch `unit/feat-buyer-rma`; current worktree only. Latest fetched integration base `2f61bf86673b3ebf4acbd3ee0ecacd54d37cf46f` is already an ancestor. No merge, force-push, production action or other worktree edit.
Role: dispatched backend implementer; no recursive delegation. Runtime model/effort identifiers are not exposed. Humaux tools unavailable: no shared memory store, canvas, rejected enumeration, code index or coord mutation was performed. This delivery file is repository evidence only.

## Authority and implementation

Read AGENTS.md; 架构.md §§0/1/2/26/27; returns-v1; buyer-order-history-v1; PROCESS; existing returns, buyer capability, HTTP/BFF and CI harness source. The integrator's 2026-10-31 ruling resolves the historical BLOCKED.md: separate applications, unchanged RMA parser/state machine, atomic approval via existing register_rma, 7-day collection / 14-day manual-shipment window.

Added contract amendment 0210 and history pointer; exact assigned migration 0210_buyer_return_requests.sql; full buyer create/list/detail/withdraw through existing BFF and capability; merchant Go list/detail/approve/reject; safe typed Go/TS projections. Independent request records use requested/approved/rejected/withdrawn and link to existing merchant RMA on approval. No refund or stock operation. Actor-scoped receipts, reauthorization, order-row serialization, CAS and scoped FORCE RLS. R2 pin 139 -> 140 (base count +1).

The frozen legacy route oracle remains unchanged. Its differential gate retains every previous case and now adds an independent four-endpoint request oracle; inventory is exactly 50 routes. No gate-adapter assertions, baselines, exemption lists, thresholds or fixtures were weakened.

## RED (observed, causal assertions)

Head `be932cbb748c3c88a1044ffcf242c6e04333a8b4`; Foundation run **41093678360**, job **124193777832**.
https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41093678360/job/124193777832
New BFF assertions failed at buyer-return-server.test.mjs:67 (404 vs 201 reachable create) and :141 (404 vs 503 unsafe-projection refusal). Node 502/504 passed; exactly 2 failed, none skipped. See red.log and red-foundation.log (raw tail retained locally). This is not a Go/REAL_PG RED claim; old Gates run 41093678473 was still queued when inspected.

## Static validation receipts

Executed only owner-permitted static operations, all exit 0:
- gofmt on changed Go files.
- go vet ./internal/returns ./internal/buyerhttp ./internal/httpapi ./tests/foundation
- node --check apps/storefront/lib/buyer-server.ts
- node --check apps/storefront/lib/return-request-contract.ts
- python3 scripts/check_packet.py: PASS_PACKET_STRUCTURE_ONLY; generated timestamp-only packet-check.json change restored to HEAD.
- node scripts/dev/derive-browser-ownership.mjs, then --check (generated ownership committed).
- bash scripts/dev/depmap.sh, then --check (generated map committed, never hand-edited).
- git diff --check.
No installed tsc was found; full TypeScript typechecking remains CI-only/NOT_RUN locally. No local tests, PG, containers, browser or server ran. No process remains.

## Design and limitations

Fleet Bridge status confirmed a connected document; read-only context 26:773 refreshed this relay. Previous raw context/metadata/screenshot receipts for all eight card frames are retained under design-reads. No Figma write. Only evidenced reason is damaged (商品有瑕疵). Other reason choices UNKNOWN. Photo upload DEFERRED: existing buyer media path is read-only. Human R-number uses UUID hex rather than inventing a store sequence. Lists return the newest 100; detail remains addressable.

Retention period is UNKNOWN in the existing privacy contract. This card adds no purge or retention duration; transactional notes and receipts remain with orders. Privacy-erasure adoption for new free-text fields and merchant admin UI/BFF adoption are explicitly recorded follow-ups, not claimed accepted or implemented. UI: pending PARITY-NEWFEAT. No UI strings/pages/assets were shipped, so visual/UI acceptance is NOT_RUN.

## Scope / handoff

Implementation paths: contracts/returns-v1.md, contracts/buyer-order-history-v1.md, migrations/0210_buyer_return_requests.sql, internal/returns/{returns,request}.go, internal/buyerhttp/{handler,routes,return_requests}.go, internal/httpapi/{returns,return_requests}.go, apps/storefront/lib/{buyer-server,return-request-contract}.ts, existing buyer route harnesses, assigned R2 test pin. Generated additions: docs/engineering/dependency-map.md and output/ci-select-backend-browser/browser-ownership.json (authorized). Card evidence: output/FEAT-BUYER-RMA/{red.log,DELIVERY.md}. No other app/backend package changed; no dependency/lockfile change. Integrator owns final migration/contract integration and independent review.

After the implementation push, current-head Foundation/Gates workflow IDs will be posted in a PR comment and final WAIT_RUN handoff; no evidence-only commit after GREEN. Integrator relay provides completion notification in a fresh session. Max two targeted attempts per blocker remains in force (zero implementation CI repair attempts used this relay).

NOT_RUN: local tests/servers (prohibited), implementation CI GREEN, REAL_PG acceptance, full G07, independent review, TypeScript typecheck locally, browser/UI acceptance, SANDBOX/LIVE providers, production rollout. No completion claim.
