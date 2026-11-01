# PARITY-TEAM ROUND 3 ruling implementation

Date: 2026-11-01. PR #476, `unit/parity-team` → `r3/integration`.
Role: integrator-dispatched UI implementation worker. Runtime identity: Codex/GPT-6; exact configured model/effort is not exposed. No delegation.
Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/parity-team` only.
Merged base: `87c67e33c329a43bb6fc0d51d9160ac4c9826b98`; local merge checkpoint: `3f00f1fe2ab5e86bfa3b09586f6032295f49d7af`.

## Authority and trail

- Human: ROUND 3 dispatch, two explicit rulings in this session. They supersede ROUND 2's pause-lift/outbox and no-op-role blockers.
- Signed copy: `output/parity-team/FRAME-COPY.md`, SHA-256 `fd1ff3d7c2f1eceb2a6aec157886925fd5bf31e2ea307a0e79f70bc97fee55c0`.
- Signed source: read-only ux-team DELIVERY, current 164-frame section, SPEC + three-language table; no other worktree was written.
- Read AGENTS.md, architecture §§0,1,2,26,27, original card, identity Q-TEAM contract, existing Team DTO/client/BFF and owner limits. New code uses only those DTOs/commands.
- Humaux and code-index tools are absent from this runtime. Targeted tool discovery returned no search/get/coord/canvas/store/code-index tools. Shared task/store/canvas/index/link writes are NOT_RUN; this report is a local handoff, not a substitute claimed as Humaux persistence.
- No production action, server, local test, invitation send, migration/contract/backend edit, merge to integration, force-push or auto-merge.

## Product behavior

- Signed member/pending tabs, identity-first cards, role/join/owner protection, email-derived label and null-email fallback. Five described role radios are the permissions view; server authorization remains authoritative.
- Review → submit once → result for reachable role changes, member removal and invitation withdrawal. Successful commands re-read server state; reload never replays a mutation.
- Unknown result → one read-only reconciliation → confirmed result or unresolved notice; no resubmit button. Identity/scope state unmounts when the existing guarded read closes.
- Invitation entry retains five role choices and email. Submit only opens signed unavailable copy; it never calls inviteMember or creates a success receipt. Q-TEAM pause and all backend entry points are unchanged.
- Continue is disabled for admin and unchanged roles; the unchanged-admin signed copy is shown. No admin confirmation/result route is built. Additive CI assertions cover promotion from viewer as well as the existing unchanged-admin assertions.
- zh-TW, zh-CN and en signed copy resides in `packages/i18n/src/team-parity.ts`. Legacy copy exports remain intact for existing mail-label contract tests.

## DEFERRED

| State | Desktop / mobile | Reason |
|---|---|---|
| invite-sent | `1468:75217` / `1468:75260` | staff invite outbox: separate backend card |
| invalid-email | `1468:75278` / `1468:75326` | staff invite outbox: separate backend card |
| already-member | `1468:75349` / `1468:75393` | staff invite outbox: separate backend card |
| invite-limit | `1468:75412` / `1468:75456` | staff invite outbox: separate backend card |
| owner-disabled | `1468:75475` / `1468:75519` | staff invite outbox: separate backend card |
| reauth | `1468:75538` / `1468:75582` | staff invite outbox: separate backend card |
| invite-checking | `1468:76405` / `1468:76453` | staff invite outbox: separate backend card |
| checking-unresolved | `1468:76689` / `1468:76734` | staff invite outbox: separate backend card |
| link | `1468:76880` / `1468:76892` | staff invite outbox: separate backend card |
| signin | `1468:76904` / `1468:76925` | staff invite outbox: separate backend card |
| signup | `1468:76946` / `1468:76968` | staff invite outbox: separate backend card |
| verify-signin | `1468:76990` / `1468:77006` | staff invite outbox: separate backend card |
| verify-signup | `1468:77022` / `1468:77038` | staff invite outbox: separate backend card |
| accept | `1468:77054` / `1468:77066` | staff invite outbox: separate backend card |
| accept-checking | `1468:77078` / `1468:77097` | staff invite outbox: separate backend card |
| joined | `1468:77116` / `1468:77134` | staff invite outbox: separate backend card |
| expired | `1468:77152` / `1468:77170` | staff invite outbox: separate backend card |
| revoked | `1468:77188` / `1468:77206` | staff invite outbox: separate backend card |
| used | `1468:77224` / `1468:77242` | staff invite outbox: separate backend card |
| wrong-account | `1468:77260` / `1468:77278` | staff invite outbox: separate backend card |
| invitee-member | `1468:77296` / `1468:77314` | staff invite outbox: separate backend card |
| invitee-off | `1468:77332` / `1468:77350` | staff invite outbox: separate backend card |
| invite-sent-owner | `1468:77368` / `1468:77411` | staff invite outbox: separate backend card |
| invite-sent-live | `1468:77496` / `1468:77539` | staff invite outbox: separate backend card |
| invite-sent-fulfilment | `1468:77624` / `1468:77667` | staff invite outbox: separate backend card |
| invite-sent-viewer | `1468:77752` / `1468:77795` | staff invite outbox: separate backend card |
| mail-failed send/history journey (unavailable content implemented) | `1468:75601` / `1468:75647` | staff invite outbox: separate backend card |
| role-confirm-admin | `1468:78293` / `1468:78333` | signed frame disables Continue |
| role-done-admin | `1468:78348` / `1468:78392` | signed frame disables Continue |
| reauth-signin | `1468:96313` / `1468:96358` | staff invite outbox: separate backend card |
| reauth-verify | `1468:96378` / `1468:96420` | staff invite outbox: separate backend card |

Entry frames `invite-owner/admin/live/fulfilment/viewer`, and the unavailable `mail-failed`/`password-off` content, are implemented. Existing pending invitations can still be read/withdrawn; that does not issue mail. Deferred frames remain in the frozen RED suite with all assertions intact; no skipped tests, oracle edits, threshold changes or exemption additions.

## Acceptance conflict requiring integrator reconciliation

The preserved tests `tests/admin/parity-team.mjs` still require invite writes, successful send/accept journeys, and reaching role-confirm-admin/role-done-admin after disabled Continue. These cannot pass while obeying this ruling. They were NOT rewritten, skipped or weakened. The two module-shell nav expectations also remain unchanged under the shared-shell exception. Do not classify these deferred paths as passed parity. Integrator must reconcile signed test scope separately; this lane cannot fabricate transitions or send mail for GREEN.

## Evidence

- RED (E3, failure only): head `89452970349f92219e1b1217868611f1a73057fe`; Gates run `41279053547`, browser job `124769729983`, missing signed 員工名單 at parity-team.mjs:171. Build and Foundation `41279053499` / `124767027990` passed on that head. Current source is not validated by these old receipts.
- E1 static: TypeScript strict `node node_modules/typescript/bin/tsc --project apps/admin/tsconfig.json --noEmit --incremental false`, exit 0; `node --check tests/admin/parity-team.mjs`, exit 0; `python3 scripts/check_packet.py`, exit 0 (structure only); depmap generation exit 0, no diff; browser ownership generation + --check exit 0.
- Tests-only RED assertions are preserved verbatim and in order. New ruling assertions are additive and await their first CI execution; no separate causal RED claim for these additions.
- CI supplies per-frame screenshots, SHA-bound manifest and real-click ledger. No local browser/screenshot/density parity or successful persistence result is claimed. Runtime response fixtures are MOCK, not Go/PG/provider acceptance.
- GREEN/independent review/density measurement/successful click acceptance: pending CI. SANDBOX/LIVE: NOT_RUN. Own PASS would be E0.

## Paths outside the original dedicated team surface

- `packages/i18n/src/team-parity.ts`: required shared three-locale copy module, consumed only by Team through team-copy; listed for integrator approval.
- `apps/admin/components/team.css`: scoped Team styles within the owned app/module (scope pre-approved).
- Generated `output/ci-select-backend-browser/browser-ownership.json`: expressly authorized. Depmap regenerated with no change.
- No migration, contract, OpenAPI, go.mod/sum, lockfile, baseline, dispatcher, R2 pin or other application/backend source change.

## Handoff

One implementation push follows the already observed tests-only RED; no evidence-only commit after CI. Gates/Foundation run IDs will be recorded in a PR comment once registered, and returned as WAIT_RUN. CI queue ownership returns to the integrator for re-dispatch, with no unattended local process.

STATUS: IMPLEMENTED_E1_CI_PENDING_SCOPE_CONFLICT_RETAINED
