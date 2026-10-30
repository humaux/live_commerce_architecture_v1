## IMPL3: keep every independent inbox send locked until its own settlement

Completing local ACK A previously skipped older queued/blocked B beneath newer sent C, and B observed while A was pending could disappear from the next page. InboxThread now retains every observed obligation independently, including during pending ACK reads. Only exact A9 outbound_id equality with the A12 ACK can supersede A's stale projection after validated operation completion. Missing identities fail closed; display text/time/principal never establish identity. The rejected ACK keeps its original text for recovery after B settles. Fixtures add outbound IDs only, with distinct IDs for unrelated sends; assertions, copy, times and states are unchanged.

Consumes integrator seam commits 2947549dc and 10ec3a0f5. Pulled assigned branch and merged origin/r3/integration 21eaceb5c72ce6b55955607529daef31fe3163b7; resolved i18n header and regenerated browser ownership. No Go, contracts, migration, OpenAPI, lockfile, R2 pin, baseline, exemption or gate-adapter change by this dispatch. No PR merge/auto-merge/status/deploy.

Existing causal RED: tests-only head 5d66cd1b3d9da52a92f16384608b3d320d6f869a; Foundation 40943430245/job123692452595 reaches 24 new local-ACK assertions, including all FAILED_FINAL cases. Gates 40943430541/bundle:06 job123694392124 reports Inbox 12 failures/40 passes; per-assertion browser artifact independently confirmed by integrator, not downloaded this dispatch. RED is historical E3, not current implementation acceptance. One implementation push; exact head, native WAIT_RUN IDs and later GREEN receipts go in PR comments, without an evidence-only push.

Local E1: node --check on changed test fixtures and DTO; git diff --check; browser ownership regeneration followed by --check, all exit 0. No local tests, servers, browsers or builds. tsc NOT_RUN because local compiler/dependencies are unavailable. Implementation-head Foundation/Required-Gates, native clicks/persistence/three-locale acceptance and independent acceptance remain PENDING. Author acceptance E0.

Delivery: output/meta-inbox-v2/impl3-294/{DELIVERY.md,red.log,static.log,source-hashes.txt}. Four UI-process sections are in DELIVERY.md; this correction changes no visual design, visible copy, route or setting. Earlier card click/parity/design evidence remains historical and does not accept this head. Pre-existing dirty/untracked handoffs are preserved and excluded from commits. Humaux tools unavailable, shared-state store/canvas/index/link NOT_RUN; PR comment is the integrator relay handoff.

Outside original card paths (cumulative PR; integrator approval):
- apps/admin/lib/inbox-types.ts — explicitly authorized by IMPL3; optional outbound_id plus invariant header.
- packages/i18n/src/index.ts — merge-conflict header only this dispatch; both existing inbox and integration editor exports preserved.
- playwright.config.ts — earlier V2 browser registration; current integration edits merged without manual changes.
- scripts/dev/test-node.sh — earlier direct-consumer registrations.
- output/ci-select-backend-browser/browser-ownership.json — mandatory generated output, regenerated last and --check exit 0.
- Earlier output/inbox-v2-impl/** evidence remains cumulative PR history; new delivery is within output/meta-inbox-v2/**.

P2 for issue: existing timestamp display-order comparison, already recorded in PR comment 6489297640; unchanged.

NOT_RUN: local tests/server/browser/build/PG/container/LIVE, local tsc, current-head CI acceptance, native click/persistence/density acceptance, independent acceptance, Humaux operations. Integrator relays workflow completion after WAIT_RUN handoff; no local background process is left.

🤖 Generated with Codex (dispatched by Claude Code integrator).
