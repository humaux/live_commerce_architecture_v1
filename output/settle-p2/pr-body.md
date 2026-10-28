Closes humaux/live_commerce_architecture_v1#216 humaux/live_commerce_architecture_v1#220

Fix all five #220 settlement gate follow-ups: use W2 Monday noon for PF16 payout and verify historical paid_at persists before closed_at; compare PF18a tenant/store/period/net/closed_at with the A/D fixtures; preserve distinct monetary argument keys and verify discovered SQL argument sets; test r0 UPDATE/DELETE immutability before removing the row for the legacy fixture; inspect all three close write surfaces through the still-open probe transaction.

RED E3: Gates 40642386557 / g09 job 122680364875 at head 4e447ea1d502f6de2799c7959978511043f9af4c, synthetic checkout 875994d3c1fe21194e74c954da5c74167b3c8344. All five items failed on their own assertions. Earlier monetary-key RED: 40628121605 / 122621731233 at caded567501aa169d47d0f3759e425167bc21bdb. The apparent matrix failures are expected child-scenario output, not a failing parent; integrator explicitly ruled to proceed.

#216 was already fixed on base 5365b5701a0e99ef9fd8fbb4e64a3a9f87aaf248: tests/foundation/platform_settlement_gate_test.go:180-181 maps returned_minor to amount, :735 supplies amount, migrations/0200_settlement_returns_recon.sql:427 declares p_returned_minor. This PR updates the input to returned_minor alongside the injective mapping. PF16a-j passed in the earlier RED run; current implementation GREEN remains pending. No new carry-filter mutation proof is claimed.

Probe calls use the real registrar role; only observation uses owner privileges. Snapshot comparison happens before the outer rollback. PostgreSQL automatically undoes failed-statement writes, so the probe does not claim visibility into transient writes erased by RAISE or savepoint recovery. The uncommitted fixture negative demonstrates observer visibility. Existing assertions are preserved.

Validation: gofmt and git diff --check exit 0 (static only). No import/package changes; no dependency-map regeneration required. Local tests/servers NOT_RUN by owner rule. Implementation GREEN, contractdrift 0-new and independent acceptance PENDING; exact run ids follow in PR comment. Evidence: output/settle-p2/red.log and raw filtered receipts. Humaux resumed with targeted lookup and locked handoff storage/canvas update.

Files outside card write paths: none. No migrations, frozen files or other modules. No merge/auto-merge. Reversal: revert this card's commits through the integrator; no production data or product code changes. GitHub workflow timeouts/notifications and integrator relay provide CI wakeup.

🤖 Generated with Codex (dispatched by Claude Code integrator).
