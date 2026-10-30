## INBOX-V2-IMPL / RVFIX5: retain all staff send obligations independently of local ACK

The latest independent REQUEST_CHANGES at head b9cc7e0c350f7353b1cfcbe211e28cd7987114e9 identifies a P1: local ACK A completing skips older queued/blocked B below newer sent C. B first seen while A is pending can also be lost if omitted when A completes. Both permit another reply before B is settled.

Current stage is **one batched tests-only RED push**, with no production implementation change. Add 24 Node regressions across zh-TW/zh-CN/en, both B states, both observation timings, and own SUCCEEDED/FAILED_FINAL; preserve exact-operation identity and rejected A original-text recovery. Add 12 signed native-click browser scenarios with omission/reappearance, reload/reopen, explicit B settlement and one-POST assertions. Strict discovery now requires 33 V2 scenarios plus all 13 legacy scenarios. Existing assertions/timeouts/fixtures/exemptions remain; added fixture switch defaults to prior behavior. No next-startup/gate-adapter edit.

Merge origin/r3/integration first: already current17328372d. Regenerate browser ownership last. Local node syntax, whitespace and mandated generation exit0 (E1). No local runtime verification; author acceptance E0. **New causal RED PENDING**, must reach these new assertions before implementation. Historical RVFIX4 RED Foundation40929100216/job123642440286 and GREEN Foundation40938647143/job123675809303, Gates40938647331/required123688180675 apply only to the earlier b9cc7e0c3 repair and do not accept the new conjunction. New exact native WAIT_RUN IDs follow in a PR comment; integrator relays completion within the dispatched CI-wait workflow. No evidence-only GREEN commit.

Evidence and four required UI process sections: output/inbox-v2-impl/rvfix5-294/DELIVERY.md, static.log, red.log, source-hashes.txt. Existing signed UI/copy/design remains; no new setting, layout, copy, API, permission, backend or design export. Native clicks and visible persisted MOCK facts are required; no LIVE claims. Rollback reference b9cc7e0c350f7353b1cfcbe211e28cd7987114e9. Pre-existing dirty/untracked handoffs preserved. Humaux plan store accepted; own child canvas updated under acquired lock; actual changed test files submitted to code index.

Outside initial card write paths for integrator approval (cumulative PR):
- playwright.config.ts — append V2 browser registration.
- scripts/dev/test-node.sh — register existing direct-consumer tests.
- tests/admin/inbox-unknown.test.ts — direct component regressions; modified this RVFIX5 batch under same-module scope pre-approval.
- tests/admin/inbox-window.test.ts — prior direct-consumer coverage.
- output/ci-select-backend-browser/browser-ownership.json — required generated artifact; regenerated this batch.

P2 for issue: timestamp lexical comparison remains dedicated existing PR comment6489297640; no duplicate or claim of repair. No contract/migration/OpenAPI/R2 pin/go.mod/lockfile/baseline/exemption changes. Depmap unnecessary: no Go dependency edge changed. No merge/automerge/status/deploy.

NOT_RUN: local tests/server/build/browser/PG/container/LIVE; new assertion RED, implementation-head GREEN, click/persistence/density runtime acceptance and independent acceptance pending CI relay. STOP for unrelated blockers or two targeted attempts on the same blocker.

🤖 Generated with Codex (dispatched by Claude Code integrator).
