# settle-p2 handoff

Evidence: E0 until the PR's own CI proves the expected assertion failures. No local tests or servers.

- Dispatch: integrator card in this session; no Humaux MCP/coord/code-index tools are exposed. No Humaux reads, stores, canvas writes or claims were possible.
- Role: implementation worker (Codex); exact runtime model/effort not exposed, UNKNOWN.
- Worktree: /Volumes/data/live_commerce_architecture_v1/.worktrees/settle-p2
- Branch: unit/issue-216-220-settlement-gate; base 5365b5701a0e99ef9fd8fbb4e64a3a9f87aaf248.
- Authority: GitHub issues #216 and #220 read on 2026-10-28; contracts/stripe-platform-account-v1.md; named SQL signatures in migrations 0150/0200 (read-only).
- Allowed changes: issue-named files and direct tests; output/settle-p2/**; generated dependency map only if forced. No migrations, frozen files, other worktrees or integration-branch writes.

## Item ledger

- #216 original missing returned_minor mapping: already fixed on base at tests/foundation/platform_settlement_gate_test.go:180-181, with its amount value at :735 and real SQL p_returned_minor at migrations/0200_settlement_returns_recon.sql:427. Skip re-fixing it. New argument coverage also protects the real spelling. No claim of a new mutation receipt or current PF16 business verification.
- #220.1 fixed Monday payout timestamp and pre-close paid_at: TODO; no implementation or RED yet.
- #220.2 PF18a exact fixture fields: TODO; no implementation or RED yet.
- #220.3 distinct monetary keys + discovered complete SQL argument sets: tests added; RED pending. After its assertion RED, remove the lossy amount mapping and give the PF16 return values the returned_minor key. Preserve real registrar calls. Then obtain GREEN on this PR.
- #220.4 r0 immutability before fixture deletion: TODO; no implementation or RED yet.
- #220.5 probe-transaction no-write observation: TODO; no implementation or RED yet. PostgreSQL statement errors abort/roll back the statement; design the observation around a savepoint without disguising that limitation.

## Verification and continuation

- gofmt on the changed Go test: exit 0.
- git diff --check: exit 0.
- No imports/packages changed; no dependency-map regeneration needed at this phase.
- No local test/build/server, provider call, production action, merge or auto-merge.
- Continue only after the identified RED job finishes. Read its failed log and bind the failure to the tested checkout SHA; an unrelated gate failure requires stopping/reporting.
- Each remaining item needs its own meaningful CI RED -> GREEN evidence (minimal mutations permitted by dispatch), without weakening existing assertions or changing frozen contracts.
- CI has workflow-level timeouts and GitHub completion notifications. Session handoff uses WAIT_JOB so the integrator relays the conclusion; no unattended local process remains.

## CI checkpoint (2026-10-28)

- Draft PR #261: https://github.com/humaux/live_commerce_architecture_v1/pull/261
- RED head: caded567501aa169d47d0f3759e425167bc21bdb.
- Gates run 40628121605 remains queued. WAIT_JOB 122617848131 is the plan job; g09 has not been allocated. After plan completes, retrieve gate (shard:g09)'s job id from this same run, then await/read its assertion result. New top-level tests go to catch_all g09, not the shard containing the old exact TestPlatformSettlement test name.
- Foundation run 40628121654 / job 122617848287 also queued.
- RED not observed; GREEN NOT_RUN. These appended remote receipts are local uncommitted handoff data, intentionally not pushed just to restart the queue. Commit with the implementation after the expected RED is observed.
- Saved PR body: output/settle-p2/pr-body.md (local untracked handoff asset).

## Resumed checkpoint — authoritative over the earlier pending/TODO entries

- Read issue #216/#220 full bodies and named frozen contract sections again. #216 original root cause remains already fixed on base.
- Confirmed E3 RED for #220.3: Gates 40628121605 / g09 122621731233; PR head caded567501aa169d47d0f3759e425167bc21bdb, checkout synthetic merge 749eff3a0baf71a33b32a593fb57c76eefd68f88. Exact assertions are in red.log and red-40628121605-assertions.log.
- Foundation 40628121654 / 122617848287 passed. All other Gates shards passed; receipts/required failed downstream of g09.
- Remaining four items now have one batched tests-only RED push. This extra RED push is necessary because the inherited first push covered only #220.3; it does not repeat that finished RED step or push item by item.
- #220.1: new Monday timestamp subtest fails existing Friday T; a real persisted paid_at/closed_at check also exercises historical payout acceptance.
- #220.2: shared payable validator retains existing weak checks in RED; all five bad-value negatives must fail. After RED, add exact fixture comparisons (closed_at by parsed instant) without removing existing assertions.
- #220.4: r0 forbidden UPDATE/DELETE guard is intentionally after the existing deletion for RED. After RED, move the entire guard before deletion, as the issue explicitly permits.
- #220.5: gateCloseCounts intentionally retains the separate-pool observation defect for RED. A rollback-only transaction changes one fixture statement; the observer must see it. After RED, use q and wire pre/post snapshots into closeProbeOut BEFORE outer rollback. Call real close under registrar role within an owner transaction, with a savepoint for SQL error recovery, RESET ROLE for count access, then inspect and roll back. PostgreSQL rolls back failed statements automatically: do not claim visibility into transient writes erased by a raised SQL exception.
- #220.3 after remaining RED: preserve exact monetary keys in gateKey, use returned_minor in the return values. Keep original argument-set assertions unchanged.
- No new imports or package dependencies; dependency-map regeneration is unnecessary. No frozen paths, migrations, other branches/worktrees or product code changed.
- Skill: ponytail read and applied; Humaux/coord/code-index unavailable, no shared-state claim/store fabricated. Repository-local evidence is the fallback, not a Humaux store.
- Static: gofmt and git diff --check exit 0. All local tests/servers NOT_RUN. No background processes started. CI timeout/completion notifications and integrator relay remain the wake mechanism.
