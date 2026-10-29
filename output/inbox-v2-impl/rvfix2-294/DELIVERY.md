# RVFIX2 #294 — ACK identity repair / implementation attempt 1

## Authority, ownership and rollback

Integrator-dispatched single card. Review source: `output/integrator/tools/logs/cxf-rv-294.log`, final REQUEST_CHANGES for head `3e8fa84460b9f6824b6c11c5db365b44e9856efe`. Only P1 is the A12 queued ACK initialization gap; P2 timestamp comparison stays deferred to an integrator issue. Parent retains task ownership, independent acceptance and resumption; no delegation/claim/merge/automerge/deploy.

Assigned worktree `/Volumes/data/live_commerce_architecture_v1/.worktrees/inbox-v2-ui`, branch `unit/inbox-v2-ui`, existing PR294 into r3/integration. Role: dispatched UI implementation worker; model/effort not exposed to this runtime, UNKNOWN. Pre-change rollback reference: `3e8fa84460b9f6824b6c11c5db365b44e9856efe`; no rollback executed. The two existing untracked rvfix-294 relay files are preserved.

Fetched and merged integration `d8093bc43f16f8eb4e209eeb6c7b698efcd27b0e` in `aafd9a2e1`. Sole manual conflict resolution: i18n dependency header retains state-recovery, people and signed inbox copy; incoming people and inbox exports remain. Required browser ownership regenerated, never hand edited. Incoming integration files are merge provenance, not additional lane implementation.

## Root cause and acceptance gate

A12 ACK sets deliveryLock but leaves lockedOutbound null. The first A9 read selects the newest outbound indiscriminately; concurrent staff B=sent replaces the acknowledged A=queued and reopens the composer after a newer inbound. A9 has_unknown_outbound=false excludes UNKNOWN only. A9 display rows lack operation/outbound identity, whereas A12 supplies both; do not infer operation identity from the newest row. Existing operation detail read is a candidate authority only after verifying its scope/permissions and current contract; no API/backend/contract change is authorized by this card.

Six additive Node cases: actual A12 queued ACK, then immediate A9 with a different staff's later sent row plus newer inbound, own queued row visible or absent, across zh-TW/zh-CN/en. Assert disabled send, folded composer, exact queued result, no new receipt or POST through both current and previously captured submit handlers; repeat status query cannot release the lock.

Six additive browser cases: same three locales × visible/omitted initial own row, native fill + Send + two Check status clicks. Assert visible queued result, hidden composer, disabled controls and one POST throughout. MOCK transport fixtures are preparation only; no evaluate/DOM writes, direct API actions, force clicks or synthetic events replace user operation. Existing state reload/persistence/density and all real-chain assertions remain unchanged. Discovery now requires twelve signed INV2 scenarios and each new exact title once; legacy thirteen-case and calibration assertions remain.

Click ledger: PENDING implementation-head CI execution. Causal Node RED is confirmed in red.log (191 pass / 6 fail). Page messages; controls reply-text/reply-send/inbox-send-status; actions fill, click Send, click status twice; expected queued retained, composer hidden/disabled, one A12; actual NOT_RUN locally. Author runtime proof E0 until implementation CI. RED is a verified assertion failure; implementation began only after reading that receipt. RED/GREEN run/job IDs and tested SHA follow raw GitHub receipts; historical green is not acceptance.

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

Outside original card paths: `tests/admin/inbox-unknown.test.ts` (already disclosed direct consumer) and generated `output/ci-select-backend-browser/browser-ownership.json`. Header-only integration resolution in `packages/i18n/src/index.ts` is inside original paths. Implementation imports existing operations-client/model and the existing order read error; all three InboxThread importers were inspected. The shared host resolves real transitive sources generically, and its transport adds an optional operation-detail response; no production parser is mocked. No closed allowlist/injected global requires another change; no new/deleted/renamed spec/page/route; no Go graph change, depmap unnecessary. No migration/contract/OpenAPI/pin/baseline/lockfile/exemption/test weakening.

One tests-only RED push covers all cases. Poll at most about ten minutes; record native Foundation/Gates RUN IDs (not plan-job IDs) in WAIT-RUN.md and hand off. Integrator's completion relay is the wakeup, no local unattended watcher. No implementation until CI reaches the new assertions. No evidence-only push after GREEN.

NOT_RUN/PENDING: implementation-head GREEN, browser clicks/density/persistence, independent review, local runtime tests/build/server/browser/PG/container/LIVE. Local tsc unavailable. Syntax checks E1 only, author assessment E0. P2 for issue: mixed-fraction RFC3339 string ordering, integrator issue follow-up only.

## Relay 1 — implementation and recovery boundary

Authority: live-console-v1 §3.6/§4.4/§11; external-operation-v1 Amendment W6-05B (read model, exact integration:read permission); internal/httpapi/operations.go and existing operations-client/model. Current integration fetch/merge: already up to date at d8093bc43. No backend, contract, permission, API shape, copy, layout or setting changes.

A queued/blocked A12 ACK is now tracked independently from display-row metadata, from the moment it is accepted. The existing operation detail GET uses its validated operation ID, server store scope, matching conversation object, provider, purpose=service and action=meta.dm_send. Only that result updates the acknowledged reply. READY/DISPATCHING stay pending; SUCCEEDED confirms; FAILED_FINAL permits editing the original submitted text (never another staff member's display text); UNKNOWN/ACKNOWLEDGED stay uncertain and cannot resend. Existing operation parser rejects wrong IDs and malformed shapes. Fence checks after the bounded 8-second read prevent obsolete completion; 401 expires privacy scope, 403/404/failed reads leave the result locked. No operation query/retry/cancel POST is issued.

Honest boundary: A9 has no operation IDs. Missing/invalid ACK ID or absent integration:read permission cannot reconcile that ACK; the component stays locked rather than guessing from a display row. No grant or backend expansion is made. The ACK remains in transient component memory only; full navigation/reload uses the existing server A9 projection, not client persistence. Server history authority still has its documented 50-row/unknown limits; this card does not claim to expand that contract. Integrator should retain this limitation in acceptance.

The original six causal assertions remain verbatim and in order. They now also check exact-operation GETs and successful reconciliation without an additional POST. Thirteen new refusal/uncertainty variants cover 401/403/404, malformed/mismatched operation/conversation/action/provider/purpose, missing receipt ID, missing read permission, ACKNOWLEDGED and UNKNOWN. A rejection regression proves only our own original text is restored. Six browser cases retain their pending-lock assertions, then native status clicks observe own success and reopen the next composer. Existing browser states flow remains unchanged, with valid A12 IDs and an additive W6-05B transport fixture. All three visible locales remain present, with no new copy.

Click ledger additions (CI PENDING): Check status while pending → queued visible/composer hidden/one A12; Check status after own operation SUCCEEDED → composer visible and native fill enables Send/one A12. Existing main/state refresh persistence checks remain. This is MOCK transport browser evidence when executed, not LIVE delivery evidence.

Static-only: node --check on changed three TS test files and git diff --check exit0; required browser ownership generation exits0 (53 modes). No local tests, servers, build, browser, PG or containers. No Go graph change, depmap unnecessary. Only original InboxThread + permitted test host/spec and task evidence change; already-disclosed outside paths are tests/admin/inbox-unknown.test.ts and generated output/ci-select-backend-browser/browser-ownership.json.

Rollback reference: 9035853d86f13a672d0eba01f2bd8fb14a1f9093 before the isolated implementation commit. No destructive operation performed. P2 timestamp precision remains deferred in existing PR comment 6489297640, verified through GitHub; no duplicate issue or comment. Integrator receives current-head native run IDs after push and relays completion. No polling beyond the card cutoff; no merge/auto-merge/status/deployment/evidence-only GREEN push.
