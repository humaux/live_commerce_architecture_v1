# inbox-p2 — #242 follow-ups

Role: dispatched implementation worker; no delegation. Runtime: GPT-6 Codex;
exact model/effort not exposed by this session, UNKNOWN.
Base: 5365b5701a0e99ef9fd8fbb4e64a3a9f87aaf248.
Worktree: /Volumes/data/live_commerce_architecture_v1/.worktrees/inbox-p2.
Branch: unit/issue-242-inbox-p2. PR #259 → r3/integration. No merge/auto-merge.
Authority: https://github.com/humaux/live_commerce_architecture_v1/issues/242,
contracts/live-console-v1.md §3.6 / A8–A10 and original CI receipts.

## Changes (author report E0; see latest RED continuation below)
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
The previous session had no Humaux tools. On 2026-10-28 continuation, tools became
discoverable: directed search, development rules, repo map, rejected/lesson subset,
locked decision store and own canvas update were performed. Rejected coverage 3/9;
lesson coverage 3/33; these subsets are not asserted to be complete inventories.

## 2026-10-28 RED continuation (author E0; new CI PENDING)

Prior implementation head 7f6ea56586ad9c5c90319b05c86b2821de193dd1 is now observed
GREEN: Foundation 40650271764 / 122688477621; Gates 40650271800 / receipts
122794721961 / required 122818633209. These are prior-head E3, not acceptance
of the intentional mutation or subsequent restoration. No failed jobs on either run.
This supersedes the previous implementation-CI-pending statement above.

Remaining item 2/3 RED evidence is combined in one minimal direct-test mutation
push. See RED-HANDOFF.md for exact mutations, expected own-assertion failures,
and the restoration instructions. Item 2 also gains precise response/loading
barriers and a positive unread row after reload, addressing the latest #242
incremental review: count=0 during an empty loading state is insufficient proof.
No assertions were removed or thresholds changed. The temporary mutations are
not production fixes and this mutation head must never be merged.
At most one restoration push remains after this batch. Item 4's historical
GREEN and source-counterfactual explanation remain in output/oc-12/DELIVERY.md;
its historical allowlist/browser mutation RED is not claimed.

## 2026-10-29 FIX-RED #259 restoration (E1; own-head CI PENDING)

Authority: current integrator FIX-RED card; issue #242; current repository
contracts/live-console-v1.md §3.6, A8-A10; architecture §§0,1,2,26,27 read.
Worktree/branch remain the assigned inbox-p2/unit/issue-242-inbox-p2 lane.
Integration fetched and merged before edits (original merge 054661a88; coauthor
trailer added before push). No delegation; runtime model/effort UNKNOWN.

Current remote RED head c4a02e25de0e082be7aa1d356ada05d9104a2c2a was checked
against GitHub run40690611280 attempt2. All four failing jobs were independently
read: g09/123014757576, bundle06/123014763947, receipts/123021219884 and
required/123021536575. UR02 fails its exact committed row/sequence assertion;
INU01 fails distinct-read exclusion after authoritative reload. See red.log and
item2-red.log. This supersedes earlier items2/3 RED-pending statements above.
The subsequent marked-read reload exclusion was not reached; no RED for it claimed.

Root cause: the previous head intentionally retained two sensitivity mutations.
Removed only the browser route that replaced unread data with filter=all, and
restored the second signed synthetic postDM instead of the direct sequence UPDATE.
All permanent tests and barriers stay intact. No test weakening, threshold changes,
exemption changes or frozen gate adapter changes. No app/source imports changed.

Current CardPayments is PASS67.79s (same bundle06 job), and current WebKit is PASS
(job123014763427); the previous unrelated WebKit STOP checkpoint is superseded.
No payment or WebKit changes are justified by current receipts.

Generated static browser ownership is refreshed with the repository generator:
it hashes both changed test files and records the new foundation direct test.
Outside original issue write paths: output/ci-select-backend-browser/browser-ownership.json
(generated; explicitly allowed by the FIX-RED preamble). No other scope expansion.
No dependency graph/import changes; depmap regeneration is unnecessary.

Local static checks: gofmt, go vet ./tests/foundation, python3 scripts/check_packet.py
all exit0. Packet output says PASS_PACKET_STRUCTURE_ONLY, not runtime acceptance.
Timestamp-only generated packet change was cleaned; static logs retained.
No tests/servers/browser/PG/containers run locally. No local fixtures/ports created.

Gate: this PR's own new Gates and Foundation workflows, bound to the restored head,
including real-click browser-inbox, UR02 REAL_PG, receipt/required aggregation and
ownership drift checks. Pending run IDs will be posted to the PR after the one
implementation push. Integrator relays completed run conclusions in a fresh session;
no long polling and no evidence-only push after GREEN.
NOT_RUN: new-head GREEN, independent acceptance, local tests/servers/browser/PG,
local tsc (dependencies absent), LIVE Meta and production. Author verdict remains E0.

Original artifact12835468093 playwright.log independently read in FIX-RED259 via bounded byte-range download; ZIP member CRC verified. Exact own assertion expected0/received1 at :171:131 is retained in fix-red-browser-assertion.log. Full download stalled and its partial temporary archive was removed.
