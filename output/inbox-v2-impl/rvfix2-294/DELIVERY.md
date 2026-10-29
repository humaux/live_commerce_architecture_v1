# RVFIX2 #294 — ACK initialization regression / tests-only handoff

## Authority, ownership and rollback

Integrator-dispatched single card. Review source: `output/integrator/tools/logs/cxf-rv-294.log`, final REQUEST_CHANGES for head `3e8fa84460b9f6824b6c11c5db365b44e9856efe`. Only P1 is the A12 queued ACK initialization gap; P2 timestamp comparison stays deferred to an integrator issue. Parent retains task ownership, independent acceptance and resumption; no delegation/claim/merge/automerge/deploy.

Assigned worktree `/Volumes/data/live_commerce_architecture_v1/.worktrees/inbox-v2-ui`, branch `unit/inbox-v2-ui`, existing PR294 into r3/integration. Role: dispatched UI implementation worker; model/effort not exposed to this runtime, UNKNOWN. Pre-change rollback reference: `3e8fa84460b9f6824b6c11c5db365b44e9856efe`; no rollback executed. The two existing untracked rvfix-294 relay files are preserved.

Fetched and merged integration `d8093bc43f16f8eb4e209eeb6c7b698efcd27b0e` in `aafd9a2e1`. Sole manual conflict resolution: i18n dependency header retains state-recovery, people and signed inbox copy; incoming people and inbox exports remain. Required browser ownership regenerated, never hand edited. Incoming integration files are merge provenance, not additional lane implementation.

## Root cause and acceptance gate

A12 ACK sets deliveryLock but leaves lockedOutbound null. The first A9 read selects the newest outbound indiscriminately; concurrent staff B=sent replaces the acknowledged A=queued and reopens the composer after a newer inbound. A9 has_unknown_outbound=false excludes UNKNOWN only. A9 display rows lack operation/outbound identity, whereas A12 supplies both; do not infer operation identity from the newest row. Existing operation detail read is a candidate authority only after verifying its scope/permissions and current contract; no API/backend/contract change is authorized by this card.

Six additive Node cases: actual A12 queued ACK, then immediate A9 with a different staff's later sent row plus newer inbound, own queued row visible or absent, across zh-TW/zh-CN/en. Assert disabled send, folded composer, exact queued result, no new receipt or POST through both current and previously captured submit handlers; repeat status query cannot release the lock.

Six additive browser cases: same three locales × visible/omitted initial own row, native fill + Send + two Check status clicks. Assert visible queued result, hidden composer, disabled controls and one POST throughout. MOCK transport fixtures are preparation only; no evaluate/DOM writes, direct API actions, force clicks or synthetic events replace user operation. Existing state reload/persistence/density and all real-chain assertions remain unchanged. Discovery now requires twelve signed INV2 scenarios and each new exact title once; legacy thirteen-case and calibration assertions remain.

Click ledger: PENDING CI execution. Page messages; controls reply-text/reply-send/inbox-send-status; actions fill, click Send, click status twice; expected queued retained, composer hidden/disabled, one A12; actual NOT_RUN locally. Author runtime proof E0 until CI. Causal RED must be an assertion failure, not collection/import/type failure. No implementation edits before causal RED. RED/GREEN run/job IDs and tested SHA follow raw GitHub receipts; historical green is not acceptance.

## BOURDIEU

Q1 novice/expert means recent experience with this reply task; Q2 keep send/status-query sequence; Q3 retain the seller's unresolved result during staff concurrency; Q4 no invented identity/delivery evidence; Q5 same controls and three locales; Q6 aesthetics unchanged, no owner aesthetic verdict inferred. No new visible strings or design frames; original signed copy/parity tables remain in `../DELIVERY.md`.

## PROJECT UNDERSTANDING

Read architecture §§0/1/2/26/27: separate evidence environments; preserve server authority, isolated worktree and independent acceptance. Contract live-console-v1 §3.6 explicitly allows other staff to send; §§3.4/4.4 define display projection and queued/final states; §11 A9/A12 defines the frozen responses. Current InboxThread load/send, actual hook host, browser transport, `internal/inbox/send.go`, `send_read.go` and MessageItem show the precise ACK-to-projection gap. Current card requires RED first and no local runtime verification.

## MOBBIN → EAGLE

Behavior regression only: no new layout, pattern, asset or reference import. Existing signed thread/recovery structure retained. Original reference provenance remains in `../DELIVERY.md`; no fresh Eagle/Mobbin observation or query claimed.

## FIGMA OFFICIAL GUIDANCE

No Figma/component/variant/variable/prototype/layout change. Existing guidance from `output/ux-audit-ref/FIGMA-GUIDE.md` and applied design process remain documented in `../DELIVERY.md`. New design copy lint/parity export NOT_RUN because no design delivery is made.

## Static evidence, scope and wakeup

Allowed local checks: node --check on inbox-unknown.test.ts, inbox-v2-ui.spec.ts, inbox-discovery.test.ts; required static browser ownership generator; git diff --check. Each exited 0 (E1); no local tests, builds, browser, server, PG or container. Source hashes in `source-sha256.txt`.

Outside original card paths: `tests/admin/inbox-unknown.test.ts` (already disclosed direct consumer) and generated `output/ci-select-backend-browser/browser-ownership.json`. Header-only integration resolution in `packages/i18n/src/index.ts` is inside original paths. No imports added/renamed, so harness allowlists unchanged; no new/deleted/renamed spec/page/route; no Go graph change, depmap unnecessary. No migration/contract/OpenAPI/pin/baseline/lockfile/exemption/test weakening.

One tests-only RED push covers all cases. Poll at most about ten minutes; record native Foundation/Gates RUN IDs (not plan-job IDs) in WAIT-RUN.md and hand off. Integrator's completion relay is the wakeup, no local unattended watcher. No implementation until CI reaches the new assertions. No evidence-only push after GREEN.

NOT_RUN/PENDING: causal RED, current-head GREEN, browser clicks/density/persistence, implementation, independent review, local runtime tests/build/server/browser/PG/container/LIVE. P2 for issue: mixed-fraction RFC3339 string ordering, integrator issue follow-up only.
