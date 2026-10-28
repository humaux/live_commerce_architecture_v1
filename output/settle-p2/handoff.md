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
