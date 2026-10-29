# RVFIX3 #294 — completed ACK versus later unresolved send

> CURRENT relay 2 (2026-10-29): prior RVFIX3 causal-RED claim is RETRACTED. Both tests-only 9d4d9ed1c and implementation ad3d2b606 fail while locating a status button removed by the successful ACK, before observing the later send. Corrected actions plus a deliberate reversal of the ACK-handoff implementation are pushed for a valid RED. GREEN and P1 acceptance remain PENDING. The historical relay-1 implementation description below is not the current mutation source.

## Authority and ownership

Integrator dispatch: fix every P0/P1 in the final REQUEST_CHANGES section of `/Volumes/data/live_commerce_architecture_v1/output/integrator/tools/logs/cxf-rv-294.log`. Latest reviewed head is `3826d19c5336f672f81d85c4c63cadde9acb303f`. The log contains prior review findings and causal evidence for the omission/ACK identity repairs; those assertions remain. The only matching review log currently present is cxf-rv-294.log. P2 timestamp ordering remains deferred in existing PR comment 6489297640, for integrator issue handling.

Role: dispatched UI implementation worker; runtime model/effort UNKNOWN. Branch unit/inbox-v2-ui, assigned worktree only, existing PR294 into r3/integration. No delegation, duplicate task claim, other-worktree write, merge to integration, auto-merge, status posting or deployment. Initial head/rollback reference `3826d19c5`; original untracked handoffs preserved. Integration `d0e7331525fec046fbe9361906fa66367ae583c6` merged in `e975bf736`; sole conflict was generated browser ownership, resolved with its generator. Incoming integration changes are merge provenance.

Humaux/coord/code tools are absent from this session's tool catalog. Search/rejected enumeration/store/canvas/index/link NOT_RUN, no shared-state write claimed. Used repository symbols and direct consumers for navigation. Integrator retains task ownership, independent acceptance and CI completion relay.

## Root cause and gate

Completed acknowledged operation A remains authoritative on every first-page load, skipping projected result reconciliation. A later staff message B=queued/blocked plus newer buyer inbound is masked by A=SUCCEEDED; a valid new draft can enable another send. False UNKNOWN history authority does not resolve B. Correction must preserve exact-operation reconciliation, UNKNOWN latching, pending omission locks and original-text recovery.

INVALIDATED historical RED description (see current correction): Foundation run 40879608359 / job 123467284173 at tests-only SHA 9d4d9ed1c01a9e7ff01f15455ed78af95b1a24e8 (217 tests, 211 pass / six new assertions fail). Six Node cases (three locales × queued/blocked) perform an actual component A12, observe own SUCCEEDED, enter a valid next draft, then read later B and its subsequent omission/reappearance. Assert folded composer, disabled send, correct localized result, no new receipt and one POST through both current and previously captured submit handlers. Six native browser cases use the existing qualified signed-OIDC fixture, fill/click Send, click status to observe A success, fill a next draft, then click status with B visible/omitted/reappearing. Reload/reopen with B present, omit B on another native status click, and require locked controls and one POST. Fixture changes only prepare server transport facts; no DOM writes/direct API actions replace acceptance clicks. Discovery requires all 21 V2 cases while preserving legacy assertions and order.

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

Local syntax/whitespace checks and required ownership generation only: E1 receipts in static.log. Author runtime assessment E0. The earlier causal RED claim is retracted (red.log correction); corrected RED is PENDING; implementation-head GREEN/independent acceptance/LIVE NOT_RUN. No local tests/build/server/browser/PG/container. No owned process or temporary fixture started. After one batched push, record native Foundation/Gates run IDs and hand off WAIT_RUN; integrator relays completion into a fresh session. No polling beyond ten minutes and no implementation before causal RED.

## Relay 1 — implementation after causal RED

Re-read the final REQUEST_CHANGES and prior P1 rounds in the same authoritative review log; no additional cxf-rv-294*.log exists. Fetched origin/r3/integration d0e7331525fec046fbe9361906fa66367ae583c6, merge --no-edit returned already up to date before implementation. First implementation attempt for this P1. Rollback reference: 9d4d9ed1c01a9e7ff01f15455ed78af95b1a24e8; pre-existing untracked handoffs preserved.

InboxThread still reads the exact scoped acknowledged operation. Only a definitive sent/failed ACK allows projected pending/blocked evidence to take over the existing delivery lock. That handoff retires the completed ACK, so later omission cannot repeat its successful result and clear the new lock. Missing or refused operation evidence and unresolved own ACKs keep the original fail-closed path. Other projected terminal rows do not override the exact ACK or its rejected-text recovery. Existing UNKNOWN latches and current/captured send guards remain unchanged. No seller copy, import, UI layout or setting changed; all three languages use existing copy.

Acceptance is the unchanged batched RED regressions plus existing omission/ACK-identity/rejected-original/UNKNOWN suites and native-click cases in exact-head CI. No test or harness edits in this implementation push. All required UI process sections and click ledger above apply; actual click/persistence/density results remain PENDING CI. Component imports are unchanged, so no injected-global/import-allowlist change is required. Source header and dependency edges remain intact; no Go edge changes, so depmap is unchanged.

Files in this push: apps/admin/components/InboxThread.tsx and output/inbox-v2-impl/rvfix3-294/{DELIVERY.md,red.log,red-assertions.log,relay1-static.log,relay1-source-sha256.txt}. Generated browser ownership is regenerated last before push; any resulting change is disclosed in the PR. No other source change, migration, contract, OpenAPI, pin, lockfile, baseline or exemption edit. Existing outside-card tests/admin/inbox-unknown.test.ts and generated ownership remain listed for integrator disposition; no new outside-card path in this implementation.

Static checks only, E1 receipts in relay1-static.log; local tsc unavailable (no installed compiler). Author acceptance E0. Exact implementation-head GREEN and independent review PENDING. No owned background process, server, fixture or container. Native Foundation/Gates run IDs are registered in the PR relay comment after push, and the integrator supplies completion wakeup. No merge/automerge/status/deploy or evidence-only GREEN commit. Humaux/coord/code tools remain unavailable: store/canvas/index/link NOT_RUN; file evidence is not represented as a Humaux receipt.

## Relay 2 — repair regression reachability and re-establish causal RED

Read PR294 checks and the exact failed Foundation job 40881614504 / 123474306476 at ad3d2b606, then re-read 40879608359 / 123467284173 at tests-only 9d4d9ed1c. Each has the same six failures at node() lookup on line 153, not the intended folded-composer assertion. Old red.log incorrectly described these as business-safety failures. Correction is recorded at its top, preserving the original receipt and raw excerpt as invalidated audit history. Filtered original evidence: relay2-reachability-failure.log and relay2-original-red-recheck.log.

Root cause: own operation SUCCEEDED plus newer inbound legitimately unfolds the next composer. The result section, including Check status, disappears. The next action therefore must use the already-present thread-header Refresh. Node cases now select that header button and additionally assert it is enabled; native Playwright cases click its exact existing localized accessible name (zh-TW 重新整理 / zh-CN 刷新 / en Refresh). Read-response/status/busy assertions remain. Every original safety assertion remains verbatim: hidden composer, disabled Send, localized queued/blocked result, current and captured handler refusal, null pending receipt, one POST, omission/reappearance and reload persistence. All existing status-query tests still use their original action. No force click, DOM write, API action, timeout, retry or permission adjustment.

Because the earlier RED never observed later B, it cannot validate the reviewed P1. This push deliberately restores only InboxThread.tsx from tests-only 9d4d9ed1c (reverses ad3d2b606's ACK handoff; imports/header/earlier omission and exact-ACK protections preserved). This is the card-authorized minimal mutation RED, not a finished fix or release candidate. The corrected batched six Node/six native-click cases must fail on their own locked-composer/result assertions after reading later queued/blocked B. NEW causal RED is PENDING. After integrator relays a valid failing receipt, restore the ACK-handoff implementation from ad3d2b606 in one implementation push, keeping the corrected tests, then require exact-head GREEN and independent review. If it fails again on this same blocker, stop and escalate under the two-attempt limit. This is the second targeted repair for RVFIX3; no third blind retry is authorized.

Fetched origin/r3/integration first; git merge --no-edit returned already up to date at d0e7331525fec046fbe9361906fa66367ae583c6. Rollback reference for this reversible RED mutation: ad3d2b606ec24093b83cbd682e72d6aa2a582b93. No rollback executed against another branch, untracked file or production state. No component import change; all three known harnesses still use the same adapter. No Go dependency edge change, so depmap is unchanged. Browser ownership is regenerated last before the push.

BOURDIEU: the six Q1–Q6 verdicts above still apply; this change only corrects a regression's user action. PROJECT UNDERSTANDING: §§0/1/2/10/26/27 plus live-console-v1 §§3.6/4.4/11 require server-authoritative concurrent delivery, no blind resend and isolated CI evidence. MOBBIN → EAGLE: no new layout/pattern/reference; existing provenance above remains applicable. FIGMA OFFICIAL GUIDANCE: no Figma/layout/copy/component change; existing guidance remains applicable, no new visual acceptance claimed.

Click ledger correction: Messages / header Refresh, native click after own success and with later row visible/omitted/reappearing; expected safety locks and one POST; actual PENDING CI. The existing final reload/status steps are retained. New static checks are syntax-only E1 in relay2-static.log; source binding in relay2-source-sha256.txt. Author runtime assessment E0. Local tests/build/server/browser/PG/container/LIVE NOT_RUN. Humaux/coord/code-index/link unavailable; no shared-state store or canvas update claimed. P2 remains existing PR comment 6489297640 for integrator issue handling.

Outside original card write paths in this relay: tests/admin/inbox-unknown.test.ts (already disclosed direct consumer), plus generated output/ci-select-backend-browser/browser-ownership.json if regeneration changes it; list in PR body. In-card: InboxThread.tsx, inbox-v2-ui.spec.ts and this evidence directory. No contract/migration/OpenAPI/pin/dependency/lockfile/baseline/exemption change. Pre-existing untracked handoffs preserved. No local process/fixture started. After push, register exact native Foundation/Gates run IDs and hand off WAIT_RUN to integrator; no more than ten minutes polling. No merge/automerge/status/deploy.
