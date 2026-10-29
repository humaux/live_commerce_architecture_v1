# RVFIX3 #294 — completed ACK versus later unresolved send

## Authority and ownership

Integrator dispatch: fix every P0/P1 in the final REQUEST_CHANGES section of `/Volumes/data/live_commerce_architecture_v1/output/integrator/tools/logs/cxf-rv-294.log`. Latest reviewed head is `3826d19c5336f672f81d85c4c63cadde9acb303f`. The log contains prior review findings and causal evidence for the omission/ACK identity repairs; those assertions remain. The only matching review log currently present is cxf-rv-294.log. P2 timestamp ordering remains deferred in existing PR comment 6489297640, for integrator issue handling.

Role: dispatched UI implementation worker; runtime model/effort UNKNOWN. Branch unit/inbox-v2-ui, assigned worktree only, existing PR294 into r3/integration. No delegation, duplicate task claim, other-worktree write, merge to integration, auto-merge, status posting or deployment. Initial head/rollback reference `3826d19c5`; original untracked handoffs preserved. Integration `d0e7331525fec046fbe9361906fa66367ae583c6` merged in `e975bf736`; sole conflict was generated browser ownership, resolved with its generator. Incoming integration changes are merge provenance.

Humaux/coord/code tools are absent from this session's tool catalog. Search/rejected enumeration/store/canvas/index/link NOT_RUN, no shared-state write claimed. Used repository symbols and direct consumers for navigation. Integrator retains task ownership, independent acceptance and CI completion relay.

## Root cause and gate

Completed acknowledged operation A remains authoritative on every first-page load, skipping projected result reconciliation. A later staff message B=queued/blocked plus newer buyer inbound is masked by A=SUCCEEDED; a valid new draft can enable another send. False UNKNOWN history authority does not resolve B. Correction must preserve exact-operation reconciliation, UNKNOWN latching, pending omission locks and original-text recovery.

This is the single batched tests-only RED stage. Six Node cases (three locales × queued/blocked) perform an actual component A12, observe own SUCCEEDED, enter a valid next draft, then read later B and its subsequent omission/reappearance. Assert folded composer, disabled send, correct localized result, no new receipt and one POST through both current and previously captured submit handlers. Six native browser cases use the existing qualified signed-OIDC fixture, fill/click Send, click status to observe A success, fill a next draft, then click status with B visible/omitted/reappearing. Reload/reopen with B present, omit B on another native status click, and require locked controls and one POST. Fixture changes only prepare server transport facts; no DOM writes/direct API actions replace acceptance clicks. Discovery requires all 21 V2 cases while preserving legacy assertions and order.

Acceptance: new Node assertions must fail on the tests-only head in this PR's CI before implementation. Record raw run/job/SHA in red.log. Implementation follows only after that receipt. GREEN is exact implementation-head Foundation/Gates plus independent integrator review. Existing thresholds, waits, fixtures, safety assertions, next-startup gate assertions, baseline and exemptions are unchanged. No local runtime tests, servers or browser discovery.

|Page/control|Operation|Expected|Actual|
|---|---|---|---|
|Messages / reply-text, reply-send|Native fill and Send|One acknowledged pending send|PENDING CI|
|Messages / status|Click after own success|Next draft available|PENDING CI|
|Messages / status|Click with later queued/blocked row, then omission|Composer hidden, Send disabled, one POST|PENDING CI|
|Messages / reload, conversation, status|Reload, native reopen, status after omission|Visible server pending result remains locked|PENDING CI|

## BOURDIEU

Q1 novice/expert means recent experience with this task. Q2 task is reply versus status check. Q3 concurrent staff activity must not erase unresolved sending evidence. Q4 no inferred delivery or identity. Q5 same three seller languages and existing controls. Q6 aesthetics unchanged; no new owner aesthetic verdict inferred.

## PROJECT UNDERSTANDING

Read AGENTS.md and architecture §§0/1/2/10/26/27: evidence environments stay distinct, Next/TypeScript consumes server authority, channel eligibility remains per binding, isolated worktree and independent acceptance apply. Read live-console-v1 A9/A12 and history-authority clauses: display rows do not prove exact ACK outcome; false UNKNOWN authority cannot settle queued/blocked results. Direct implementation/test authorities: InboxThread load/send/composer, inbox-review-host, inbox-unknown, inbox-v2-ui and discovery gate. Existing exact scoped operation-read permission stays intact.

## MOBBIN → EAGLE

Behavioral regression only; no new layout, visual pattern, asset or reference. Existing reference provenance remains in ../DELIVERY.md. No new Mobbin/Eagle observation or import claimed.

## FIGMA OFFICIAL GUIDANCE

No Figma write, visible string, component, variable, prototype or design handoff. Existing official guidance applications from output/ux-audit-ref/FIGMA-GUIDE.md and desktop/mobile/copy tables remain in ../DELIVERY.md. New design export/copy lint NOT_RUN for this behavior-only card.

## Scope, evidence and wakeup

Tests-only source changes: original card paths tests/admin/inbox-v2-ui.spec.ts and inbox-discovery.test.ts; previously disclosed outside-card direct consumer tests/admin/inbox-unknown.test.ts; required generated output/ci-select-backend-browser/browser-ownership.json. Task evidence stays under original output/inbox-v2-impl/**. List exceptions in PR body. No component import change or Go graph change; no harness adapter or depmap update needed. No migration/contract/OpenAPI/pin/go.mod/lockfile change.

Local syntax/whitespace checks and required ownership generation only: E1 receipts in static.log. Author runtime assessment E0. RED is PENDING; GREEN/independent acceptance/LIVE NOT_RUN. No local tests/build/server/browser/PG/container. No owned process or temporary fixture started. After one batched push, record native Foundation/Gates run IDs and hand off WAIT_RUN; integrator relays completion into a fresh session. No polling beyond ten minutes and no implementation before causal RED.
