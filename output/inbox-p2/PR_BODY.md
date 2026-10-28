Closes humaux/live_commerce_architecture_v1#242

Opening a conversation under unread now removes its row after the successful mark-read request while preserving the open thread. INU01 asserts immediate removal and persistence after reload, and now marks a distinct conversation read through the real UI and excludes it from unread.

UR02 is relocated from internal/inbox/unread_test.go to its direct integration test tests/foundation/inbox_unread_trigger_test.go to reuse signed webhook/consumer fixtures. Both inbound messages commit real social.messages rows through the production trigger. Reader no-op, reply clear, other staff's shared view, sequence advancement and unread-total assertions remain covered. Its CI location changes from shard:unit to the foundation catch-all shard. No constraints/triggers are disabled.

The OC-12 delivery file records final historical run 40538883929 and jobs 122316890900 / 122316892730 (independently verified success), and distinguishes SQL RED from source-level proof of the pre-allowlist 400 path. These are historical receipts, not acceptance of this head.

RED (E3): tests-only head 8c12cec74644d3c1134ced0a3a8f4cd7b8820396, Gates run 40628056371, browser-inbox job 122622864648. INU01 failed its own new row-removal assertion at inbox-ui.spec.ts:135:5, expected 0 / received 1, after A10 returned 200. Synthetic merge dd3f3a1ca852b997031e1f2205eb06eee1a9c197. Filtered evidence: output/inbox-p2/red.log and red-assertion.log.

Acceptance gap: items 2/3 were not included in the inherited tests-only push and have no independent CI RED. The old allowlist/browser assertions also lack an observed mutation RED; the original SQL invalid_filter failure does not prove that. These gaps are explicit NOT_RUN and prevent strict per-item RED acceptance. No extra tests-only push was made, following the relay's instruction to continue implementation and the fleet's one-RED/one-implementation batching rule.

Static validation (E1): gofmt and go vet ./internal/inbox ./tests/foundation exit 0. No local tests or servers. Implementation GREEN, contractdrift 0-new and independent review are PENDING/NOT_RUN; current run IDs will be posted in a PR comment. Author self-report is E0.

Write paths: apps/admin/components/Inbox.tsx, tests/admin/inbox-ui.spec.ts, internal/inbox/unread_test.go, its direct test tests/foundation/inbox_unread_trigger_test.go, output/oc-12/DELIVERY.md and output/inbox-p2/**. Outside-card edits: none. No migrations, pin changes, frozen-file edits or existing gate assertion changes. Humaux tools unavailable; no store/canvas/coord action claimed. No merge or auto-merge.

🤖 Generated with Codex (dispatched by Claude Code integrator).
