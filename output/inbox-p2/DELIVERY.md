# inbox-p2 — #242 follow-ups

Role: dispatched implementation worker; no delegation. Runtime: GPT-6 Codex;
exact model/effort not exposed by this session, UNKNOWN.
Base: 5365b5701a0e99ef9fd8fbb4e64a3a9f87aaf248.
Worktree: /Volumes/data/live_commerce_architecture_v1/.worktrees/inbox-p2.
Branch: unit/issue-242-inbox-p2. PR #259 → r3/integration. No merge/auto-merge.
Authority: https://github.com/humaux/live_commerce_architecture_v1/issues/242,
contracts/live-console-v1.md §3.6 / A8–A10 and original CI receipts.

## Changes (author report E0; current-head CI pending)
1. Inbox.tsx onRead now removes read rows when filter=unread, retaining the open
   thread and existing unread-total calculation. All other filters keep their rows.
   Inherited INU01 checks immediate removal and absence after reload/reselect.
2. INU01 opens a distinct existing synthetic conversation through A9/A10, checks
   read status 200 and its content, then excludes it under unread, including after
   reload. An ignored filter returning all rows would now fail the absence check.
3. UR02 TestOC12UnreadStoreLevelAndReset moved from internal/inbox/unread_test.go
   to its direct integration test tests/foundation/inbox_unread_trigger_test.go.
   Reuses lbSetup/postDM: signed MOCK webhook → real consumer → committed
   social.messages INSERT → production inbox.advance_conversation_state trigger.
   Preserves reader no-op, reply-capable clear, another staff member's store-level
   view, newer inbound reopening and unread-total checks. Also verifies real message
   counts/sequences match state. Existing shared-store unread total is pinned and
   all pages are read, so prior scenarios do not hide the new row or change the
   expected total delta. No trigger/constraint bypass. Existing snapshot helper
   remains only for UR01/03/04 filtering fixtures. New test runs in foundation
   catch-all shard; it no longer runs in shard:unit.
4. output/oc-12/DELIVERY.md now records final historical head/run/jobs and explains
   exactly what the SQL RED establishes. Pre-allowlist 400 implies status=200
   assertions would fail (source-level counterfactual); independent CI mutation RED
   of the old allowlist/browser assertions is NOT_RUN, not borrowed from SQL RED.

## CI evidence
RED E3: tests-only head 8c12cec74644d3c1134ced0a3a8f4cd7b8820396;
Gates 40628056371 / browser-inbox job 122622864648, failure. Tested synthetic merge
SHA dd3f3a1ca852b997031e1f2205eb06eee1a9c197. INU01 failed its new own assertion:
marked-read row disappears immediately; expected count 0, received 1 (:135:5).
18 other Playwright cases passed. Raw filtered excerpt and source IDs: red.log,
red-assertion.log; original gate-browser-inbox artifact 12794009779.
Receipts/required failures cascade from that browser FAIL, not unrelated gates.
Historical OC-12: run 40538883929, head 52f3ae7adaa85b4ef8dd90ffc627352a05d30e12,
unit job 122316890900 and browser job 122316892730 independently read as success.
Historical results do not accept this PR's new head.
Implementation: one batched push; exact SHA and new Gates/Foundation WAIT_RUN IDs
will be recorded in a PR comment after allocation, without an evidence-only push.
Wake-up: integrator relays the run conclusion into a fresh worker session.

## Boundaries, checks and evidence gaps
All edits are within issue-named files, their direct tests and output/inbox-p2/**.
Direct-test addition: tests/foundation/inbox_unread_trigger_test.go. No outside-card
edits, migrations, shared contracts, pins, baselines, dispatchers or gate changes.
No production imports changed, so the Go dependency map does not require regeneration.
Local gofmt + go vet ./internal/inbox ./tests/foundation: exit 0 (E1); see static-vet.log.
No local tests, servers, browser, database, containers or fixtures were started.
Downloaded RED artifact was temporary; only the filtered failure excerpt is retained.
NOT_RUN/PENDING: implementation CI GREEN, contractdrift acceptance, independent
review, local tests/servers (policy), local tsc (no node_modules), LIVE Meta.
Items 2/3 were absent from the inherited tests-only push; no independent CI RED of
those test-hardening units is claimed. Item 4 allowlist/browser mutation RED also
remains NOT_RUN. Strict per-item RED acceptance is incomplete; integrator must not
mark the card accepted solely because implementation CI later turns green.
Humaux tools are unavailable (search/store/canvas/coord/code index); no store or
canvas action is claimed. These task-owned files are the available handoff trace.
