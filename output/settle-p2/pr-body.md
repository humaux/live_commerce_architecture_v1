Closes humaux/live_commerce_architecture_v1#216 humaux/live_commerce_architecture_v1#220

Settlement gate P2 follow-ups: distinguish monetary argument keys, align PF16 to the frozen Monday payout time, validate all PF18a fixture values, exercise r0 immutability before the legacy fixture removes it, and observe PF17c writes inside the probe transaction.

Current phase: tests-only RED, implementation pending. The inherited first RED push covered #220.3 only. Gates run 40628121605 / g09 job 122621731233 confirmed the new argument assertions fail on their own expectations at head caded567501aa169d47d0f3759e425167bc21bdb (synthetic checkout 749eff3a0baf71a33b32a593fb57c76eefd68f88). This continuation batches the remaining four RED vectors once before any implementation push. No GREEN claim.

The new negatives expose the existing Friday timestamp, shape-only payable validation, deleted r0 immutability subject, and separate-connection visibility gap. Test-only fixture mutations stay in the isolated CI database and roll back. Existing assertions remain intact; no production SQL or provider actions are changed.

#216 original missing-key defect was already fixed on base 5365b5701a0e99ef9fd8fbb4e64a3a9f87aaf248: platform_settlement_gate_test.go:180-181 maps returned_minor to amount, and :735 supplies amount. New distinct-key and full discovered SQL argument coverage protects the real spelling; PF16a-j passed in the first RED run. No new carry-filter mutation receipt is claimed.

Validation: gofmt and git diff --check exit 0. Local tests/servers NOT_RUN by owner rule. Remaining assertion RED, final GREEN, contractdrift 0-new and independent acceptance PENDING. Evidence: output/settle-p2/red.log and handoff.md. No imports/package dependency changes, so no dependency-map update is needed.

Files outside card write paths: none. No migrations, frozen files, other modules, merge or auto-merge. Humaux/coord/code-index tools unavailable; evidence is retained in the assigned output directory. Resume via integrator relay after the new Gates run completes; run IDs are recorded in the session handoff/comment.

🤖 Generated with Codex (dispatched by Claude Code integrator).
