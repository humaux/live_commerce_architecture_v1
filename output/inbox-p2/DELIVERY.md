# inbox-p2 — #242 follow-ups

Role: dispatched implementation worker (no delegation). Runtime: GPT-6 Codex;
exact model/effort not exposed by the runtime, UNKNOWN.
Base: 5365b5701a0e99ef9fd8fbb4e64a3a9f87aaf248.
Worktree: /Volumes/data/live_commerce_architecture_v1/.worktrees/inbox-p2.
Branch: unit/issue-242-inbox-p2. Target: r3/integration.
Authority: https://github.com/humaux/live_commerce_architecture_v1/issues/242
and contracts/live-console-v1.md §3.6 / A8–A10.

## State (E0, pending CI)
1. RED test staged: INU01 clicks an unread thread, observes A10 200, asserts
   immediate list-row removal and verifies absence after reload/reselect.
   Production Inbox.tsx intentionally unchanged until this assertion fails on CI.
2. Pending: seed/read a distinct conversation and exclude it under unread;
   establish a CI negative control for the new exclusion assertion.
3. Pending: replace the simulated UR02 inbound with a real social.messages
   insert through inbox.advance_conversation_state; CI RED→GREEN required.
4. Pending: read original CI receipts and replace OC-12 final evidence placeholder;
   explain/prove allowlist RED independently of the historical SQL invalid_filter.

Allowed writes: issue-named files and their direct tests, output/inbox-p2/**,
regenerated docs/engineering/dependency-map.md if CI requires it.
No files outside these paths changed. No migrations, pins or frozen files changed.

## Verification and handoff
Local tests/servers: NOT_RUN (owner CI-only policy).
CI: pending PR creation and RED run; see red.log for receipts when available.
Implementation, GREEN, contractdrift acceptance and independent review: NOT_RUN.
Humaux tools (search/store/canvas/coord/code index) unavailable; no memory or
canvas completion claimed. This committed handoff is the available trace.
No local processes, fixtures, containers or temporary directories started.
